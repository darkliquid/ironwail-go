// stepframe.go implements the per-frame physics loop (SV_Physics orchestration)
// in the physics subpackage so it can be unit-tested against injected mocks
// without a live server.
package physics

import (
	"log/slog"
	"math"
	"time"

	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// MovetypeDispatch abstracts the per-movetype physics leaf dispatchers. The
// leaf algorithms (PhysicsWalk, PhysicsToss, ...) remain on *server.Server;
// this narrow interface lets the frame loop live here.
type MovetypeDispatch interface {
	PhysicsPusher(ent *srvtypes.Edict)
	PhysicsNone(ent *srvtypes.Edict)
	PhysicsNoClip(ent *srvtypes.Edict)
	PhysicsStep(ent *srvtypes.Edict)
	PhysicsToss(ent *srvtypes.Edict)
	PhysicsWalk(ent *srvtypes.Edict)
}

// StepFrame runs one server physics frame: QC StartFrame, per-edict movetype
// dispatch with client pre/post think, SendInterval bookkeeping, force_retouch
// decay, and srvTime advance. It returns the advanced server time.
func (s *System) StepFrame(
	driver srvtypes.FrameDriver,
	dispatch MovetypeDispatch,
	srvTime float32,
	frameTime float32,
) float32 {
	physicsStart := time.Now()
	hostSpeeds := driver.BoolValue("host_speeds")
	phaseStart := time.Time{}
	measureEnabled := func() bool { return hostSpeeds }
	phaseBegin := func() {
		if measureEnabled() {
			phaseStart = time.Now()
		}
	}
	phaseEnd := func(total *float64) {
		if measureEnabled() {
			*total += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		}
	}
	var startFrameMS, forceRetouchMS, preThinkMS, postThinkMS, bookkeepingMS float64
	var devStatsMS, physicsPushMS, physicsNoneMS, physicsNoClipMS, physicsStepMS, physicsTossMS, physicsWalkMS float64
	var physicsPushCount, physicsNoneCount, physicsNoClipCount, physicsStepCount, physicsTossCount, physicsWalkCount int

	telemetryActive := driver.EventsEnabled()
	if telemetryActive {
		driver.BeginFrame(srvTime, frameTime)
		driver.LogEventf(srvdebug.DebugEventFrame, driver.GetVM(), 0, s.store.EdictNum(0),
			"physics begin edicts=%d", s.store.GetNumEdicts())
		defer func() {
			driver.LogEventf(srvdebug.DebugEventFrame, driver.GetVM(), 0, s.store.EdictNum(0),
				"physics end edicts=%d", s.store.GetNumEdicts())
			driver.EndFrame()
		}()
	}

	phaseBegin()
	if vm := driver.GetVM(); vm != nil {
		// Mirror C: StartFrame runs against the authoritative edict state updated by
		// SV_RunClients. Keep the QC VM snapshot in sync so intermission/QC frame
		// logic observes fresh button presses immediately.
		driver.SyncQCVMGlobals()
		if startFrame := vm.FindFunction("StartFrame"); startFrame >= 0 {
			if telemetryActive {
				driver.LogEventf(srvdebug.DebugEventFrame, vm, 0, s.store.EdictNum(0),
					"startframe begin function=%d", startFrame)
			}
			vm.SetGlobal("self", 0)
			vm.SetGlobal("other", 0)
			driver.SetQCTimeGlobal(srvTime)
			_ = driver.ExecuteQCFunction(startFrame)
			if telemetryActive {
				driver.LogEventf(srvdebug.DebugEventFrame, vm, 0, s.store.EdictNum(0),
					"startframe end function=%d", startFrame)
			}
		}
	}
	phaseEnd(&startFrameMS)

	freezeNonClients := false
	if cv := driver.Get("sv_freezenonclients"); cv != nil && cv.Bool() {
		freezeNonClients = true
	}

	entityCap := s.store.GetNumEdicts()
	sh := s.sh
	if freezeNonClients {
		clients := driver.MaxClients() + 1
		if clients > entityCap {
			clients = entityCap
		}
		entityCap = clients
	}

	forceRetouch := float32(0)
	if vm := driver.GetVM(); vm != nil {
		forceRetouch = vm.GlobalFloat("force_retouch")
	}

	for i := 0; i < entityCap; i++ {
		ent := s.store.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}

		if forceRetouch != 0 {
			phaseBegin()
			s.col.LinkEdict(ent, true)
			phaseEnd(&forceRetouchMS)
		}

		// Mirror C SV_Physics: client-slot entities (i=1..maxclients) go through
		// SV_Physics_Client which calls PlayerPreThink and PlayerPostThink regardless
		// of movetype. PhysicsWalk already handles MoveTypeWalk; for all other
		// movetypes (especially MoveTypeNone during intermission) wrap here so
		// IntermissionThink in QC fires during intermission.
		var clientForPostThink *srvtypes.Client
		if srvtypes.MoveType(ent.MoveType(sh)) != srvtypes.MoveTypeWalk {
			if pc := driver.PlayerClient(ent); pc != nil {
				phaseBegin()
				driver.RunClientQCThinkWithMode(pc, "PlayerPreThink", false)
				phaseEnd(&preThinkMS)
				if ent.Free {
					// Entity freed by PreThink (e.g. ClientDisconnect during think).
					continue
				}
				clientForPostThink = pc
			}
		}

		moveType := srvtypes.MoveType(ent.MoveType(sh))
		switch moveType {
		case srvtypes.MoveTypePush:
			physicsPushCount++
			phaseBegin()
			dispatch.PhysicsPusher(ent)
			phaseEnd(&physicsPushMS)
		case srvtypes.MoveTypeNone:
			physicsNoneCount++
			phaseBegin()
			dispatch.PhysicsNone(ent)
			phaseEnd(&physicsNoneMS)
		case srvtypes.MoveTypeNoClip:
			physicsNoClipCount++
			phaseBegin()
			dispatch.PhysicsNoClip(ent)
			phaseEnd(&physicsNoClipMS)
		case srvtypes.MoveTypeStep:
			physicsStepCount++
			phaseBegin()
			dispatch.PhysicsStep(ent)
			phaseEnd(&physicsStepMS)
		case srvtypes.MoveTypeToss, srvtypes.MoveTypeGib, srvtypes.MoveTypeBounce,
			srvtypes.MoveTypeFly, srvtypes.MoveTypeFlyMissile:
			physicsTossCount++
			phaseBegin()
			dispatch.PhysicsToss(ent)
			phaseEnd(&physicsTossMS)
		case srvtypes.MoveTypeWalk:
			physicsWalkCount++
			phaseBegin()
			dispatch.PhysicsWalk(ent)
			phaseEnd(&physicsWalkMS)
		default:
			slog.Warn("server physics: bad movetype; skipping entity", "movetype", int(ent.MoveType(sh)), "edict", i)
			continue
		}

		// C's SV_Physics_Client unconditionally calls SV_LinkEdict(ent, true) after the
		// movetype switch, before PlayerPostThink. PhysicsWalk handles this internally,
		// but non-WALK client paths (e.g. MOVETYPE_NONE during intermission) need it here
		// so trigger touches fire even for stationary clients.
		if clientForPostThink != nil && !ent.Free {
			if srvtypes.MoveType(ent.MoveType(sh)) != srvtypes.MoveTypeWalk {
				phaseBegin()
				s.col.LinkEdict(ent, true)
				phaseEnd(&forceRetouchMS)
			}
			phaseBegin()
			driver.RunClientQCThinkWithMode(clientForPostThink, "PlayerPostThink", false)
			phaseEnd(&postThinkMS)
		}

		phaseBegin()
		ent.SendInterval = false
		nextThink := ent.NextThink(sh)
		frame := ent.Frame(sh)
		if !ent.Free && nextThink > srvTime &&
			(moveType == srvtypes.MoveTypeStep || moveType == srvtypes.MoveTypeWalk || frame != ent.OldFrame) {
			// Encode the interval to next think as a byte (0-255). Values 25 and 26
			// are close enough to 0.1 (the client default) that sending them would be
			// redundant.
			j := int(math.Round(float64((nextThink - ent.OldThinkTime) * 255)))
			if j >= 0 && j < 256 && j != 25 && j != 26 {
				ent.SendInterval = true
			}
		}
		phaseEnd(&bookkeepingMS)
	}

	if vm := driver.GetVM(); vm != nil {
		if forceRetouch := vm.GlobalFloat("force_retouch"); forceRetouch > 0 {
			next := forceRetouch - 1
			if next < 0 {
				next = 0
			}
			vm.SetGlobal("force_retouch", next)
		}
	}

	if !freezeNonClients {
		srvTime += frameTime
	}

	// Track active edict count and warn if exceeding the standard limit of 600.
	// Matches C host.c dev_stats/dev_peakstats tracking.
	phaseBegin()
	active := 0
	for i := 0; i < s.store.GetNumEdicts(); i++ {
		if ent := s.store.EdictNum(i); ent != nil && !ent.Free {
			active++
		}
	}
	driver.RecordDevStatsEdicts(active)
	phaseEnd(&devStatsMS)

	if hostSpeeds {
		slog.Debug("physics_speeds",
			"srvTime", srvTime,
			"startframe_ms", startFrameMS,
			"force_retouch_ms", forceRetouchMS,
			"prethink_ms", preThinkMS,
			"postthink_ms", postThinkMS,
			"push_ms", physicsPushMS,
			"push_count", physicsPushCount,
			"none_ms", physicsNoneMS,
			"none_count", physicsNoneCount,
			"noclip_ms", physicsNoClipMS,
			"noclip_count", physicsNoClipCount,
			"step_ms", physicsStepMS,
			"step_count", physicsStepCount,
			"toss_ms", physicsTossMS,
			"toss_count", physicsTossCount,
			"walk_ms", physicsWalkMS,
			"walk_count", physicsWalkCount,
			"bookkeeping_ms", bookkeepingMS,
			"devstats_ms", devStatsMS,
			"total_ms", float64(time.Since(physicsStart))/float64(time.Millisecond),
		)
	}
	return srvTime
}
