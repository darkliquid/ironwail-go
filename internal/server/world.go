// world.go provides server-side delegates to the collision subsystem.
package server

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/server/collision"
)

// GetWorldModel implements collision.WorldProvider.
func (s *Server) GetWorldModel() CollisionModel {
	if s == nil {
		return nil
	}
	return s.WorldModel
}

// GetWorldTree implements collision.WorldProvider.
func (s *Server) GetWorldTree() *bsp.Tree {
	if s == nil {
		return nil
	}
	return s.WorldTree
}

// TouchLinks implements collision.TouchProvider.
func (s *Server) TouchLinks(ent *Edict) {
	s.touchLinks(ent)
}

func (s *Server) ensureCollisionSys() {
	if s.CollisionSys == nil {
		s.CollisionSys = NewCollisionSystem(s)
		s.Areanodes = s.CollisionSys.Areanodes()
		s.numAreaNodes = cNumAreaNodes(s.CollisionSys)
	}
}

func cNumAreaNodes(sys *CollisionSystem) int {
	if sys == nil {
		return 0
	}
	return sys.NumAreaNodes()
}

// ClearWorld initializes spatial nodes for a map.
func (s *Server) ClearWorld() {
	s.ensureCollisionSys()
	s.CollisionSys.ClearWorld()
	s.Areanodes = s.CollisionSys.Areanodes()
	s.numAreaNodes = cNumAreaNodes(s.CollisionSys)
}

// UnlinkEdict removes an edict from the spatial grid.
func UnlinkEdict(ent *Edict) {
	collision.UnlinkEdict(ent)
}

// LinkEdict adds an edict to the spatial grid.
func (s *Server) LinkEdict(ent *Edict, touchTriggers bool) {
	s.ensureCollisionSys()
	s.CollisionSys.LinkEdict(ent, touchTriggers)
	s.Areanodes = s.CollisionSys.Areanodes()
	s.numAreaNodes = cNumAreaNodes(s.CollisionSys)
}

// PointContents returns the content flags at a 3D point.
func (s *Server) PointContents(p [3]float32) int {
	s.ensureCollisionSys()
	return s.CollisionSys.PointContents(p)
}

// Move performs a sweep trace through the BSP world and entities.
func (s *Server) Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult {
	s.ensureCollisionSys()
	return s.CollisionSys.Move(start, mins, maxs, end, moveType, passedict)
}

// TestEntityPosition tests if an entity is stuck in solid.
func (s *Server) TestEntityPosition(ent *Edict) *Edict {
	s.ensureCollisionSys()
	return s.CollisionSys.TestEntityPosition(ent)
}

func (s *Server) clipMoveToEntity(ent *Edict, start, mins, maxs, end [3]float32) TraceResult {
	s.ensureCollisionSys()
	return s.CollisionSys.ClipMoveToEntity(ent, start, mins, maxs, end)
}

func (s *Server) hullForEntity(ent *Edict, mins, maxs [3]float32, offset *[3]float32) *model.Hull {
	s.ensureCollisionSys()
	h, off := s.CollisionSys.SV_HullForEntity(ent, mins, maxs)
	*offset = off
	return h
}

func (s *Server) findTouchedLeafs(ent *Edict, child bsp.TreeChild) {
	s.ensureCollisionSys()
	s.CollisionSys.FindTouchedLeafs(ent, child)
}

func recursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 [3]float32, trace *TraceResult) bool {
	return collision.RecursiveHullCheck(hull, num, p1f, p2f, p1, p2, trace)
}

func hullPointContents(hull *model.Hull, num int, p [3]float32) int {
	return collision.HullPointContents(hull, num, p)
}

// touchLinks executes QC touch callbacks for triggers overlapping an edict.
func (s *Server) touchLinks(ent *Edict) {
	if s == nil || s.QCVM == nil || s.suppressTouchQC {
		return
	}

	entNum := s.NumForEdict(ent)
	moverClassName := ent.ClassNameString(s)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	entAbsMin := ent.AbsMin(s)
	entAbsMax := ent.AbsMax(s)
	entTouchFn := ent.Touch(s)
	entSolid := ent.Solid(s)

	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks begin mover_classname=%q touchfn=%d solid=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			moverClassName, entTouchFn, int(entSolid),
			entAbsMin[0], entAbsMin[1], entAbsMin[2],
			entAbsMax[0], entAbsMax[1], entAbsMax[2])
		defer s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks end mover_classname=%q solid=%d touchfn=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			moverClassName, int(entSolid), entTouchFn,
			entAbsMin[0], entAbsMin[1], entAbsMin[2],
			entAbsMax[0], entAbsMax[1], entAbsMax[2])
	}

	// Reuse the per-server trigger-candidate scratch slice across calls so
	// touchLinks (per-trigger per-frame) does not allocate a fresh
	// NumEdicts-sized slice every time (plan 20.4; mirrors PushMove 0afc2db).
	touches := s.touchLinkScratch[:0]
	s.ensureCollisionSys()
	var rootNode *collision.AreaNode
	if len(s.CollisionSys.Areanodes()) > 0 {
		rootNode = &s.CollisionSys.Areanodes()[0]
	}
	s.CollisionSys.AreaTriggerEdicts(ent, rootNode, &touches, s.NumEdicts)
	s.touchLinkScratch = touches

	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks candidates=%d mover_classname=%q", len(touches), moverClassName)
	}
	if svDebugPushLevel() >= 1 {
		svDebugPushDumpTriggersOnce(s)
		SvdbgPushLogf("touchlinks ent=%d classname=%q candidates=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			entNum, moverClassName, len(touches),
			entAbsMin[0], entAbsMin[1], entAbsMin[2],
			entAbsMax[0], entAbsMax[1], entAbsMax[2])
	}

	for _, touch := range touches {
		touchNum := s.NumForEdict(touch)
		touchClassName := touch.ClassNameString(s)
		if touch == ent {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
					"touchlinks scan skip-self candidate=%d classname=%q", touchNum, touchClassName)
			}
			continue
		}
		touchTouchFn := touch.Touch(s)
		touchSolid := touch.Solid(s)
		if touchTouchFn == 0 {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks scan reject candidate=%d other=%d reason=no-touch classname=%q solid=%d",
					touchNum, entNum, touchClassName, int(touchSolid))
			}
			continue
		}
		if int(touchSolid) != int(SolidTrigger) {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks scan reject candidate=%d other=%d reason=not-trigger classname=%q solid=%d",
					touchNum, entNum, touchClassName, int(touchSolid))
			}
			continue
		}
		touchAbsMin := touch.AbsMin(s)
		touchAbsMax := touch.AbsMax(s)
		if entAbsMin[0] > touchAbsMax[0] || entAbsMin[1] > touchAbsMax[1] || entAbsMin[2] > touchAbsMax[2] ||
			entAbsMax[0] < touchAbsMin[0] || entAbsMax[1] < touchAbsMin[1] || entAbsMax[2] < touchAbsMin[2] {
			if telemetryEnabled {
				reason := "axis2"
				if entAbsMin[0] > touchAbsMax[0] || entAbsMax[0] < touchAbsMin[0] {
					reason = "axis0"
				} else if entAbsMin[1] > touchAbsMax[1] || entAbsMax[1] < touchAbsMin[1] {
					reason = "axis1"
				}
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks overlap-reject candidate=%d other=%d reason=%s candidate_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f) other_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f)",
					touchNum, entNum, reason,
					touchAbsMin[0], touchAbsMin[0], touchAbsMin[2],
					touchAbsMax[0], touchAbsMax[1], touchAbsMax[2],
					entAbsMin[0], entAbsMin[1], entAbsMin[2],
					entAbsMax[0], entAbsMax[1], entAbsMax[2])
			}
			continue
		}

		if telemetryEnabled {
			ef := ent.Flags(s)
			eg := ent.GroundEntity(s)
			ev := ent.Velocity(s)
			ep := ent.PunchAngle(s)
			eo := ent.Origin(s)
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
				"touchlinks callback begin self=%d(%q) other=%d(%q) fn=%d self_solid=%d other_solid=%d other_flags=%#x other_ground=%d other_vel=(%.1f %.1f %.1f) other_punch=(%.1f %.1f %.1f) other_fixangle=%d other_teleport=%.3f self_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f) other_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f) other_origin=(%.1f %.1f %.1f)",
				touchNum, touchClassName, entNum, moverClassName, touchTouchFn, int(touchSolid), int(entSolid),
				uint32(ef), int(eg), ev[0], ev[1], ev[2], ep[0], ep[1], ep[2],
				int(ent.FixAngle(s)), ent.TeleportTime(s),
				touchAbsMin[0], touchAbsMin[1], touchAbsMin[2], touchAbsMax[0], touchAbsMax[1], touchAbsMax[2],
				entAbsMin[0], entAbsMin[1], entAbsMin[2], entAbsMax[0], entAbsMax[1], entAbsMax[2],
				eo[0], eo[1], eo[2])
		}

		s.debugTriggerTouch("touchlinks", touch, ent)
		SvdbgPushLogf("touchlinks ent=%d candidate=%d classname=%q FIRING touchfn=%d",
			entNum, touchNum, touchClassName, touchTouchFn)

		ctx := captureQCExecutionContext(s.QCVM)
		s.QCVM.SetGlobalInt32("self", int32(touchNum))
		s.QCVM.SetGlobalInt32("other", int32(entNum))
		s.SetQCTimeGlobal(s.Time)
		prevNumEdicts := s.NumEdicts
		if err := s.executeQCFunction(int(touchTouchFn)); err != nil {
			slog.Warn("touchlinks callback failed", "self", touchNum, "other", entNum, "func", touchTouchFn, "err", err)
		} else {
			s.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		restoreQCExecutionContext(s.QCVM, ctx)

		if telemetryEnabled {
			linkState := "linked"
			if touch.AreaPrev == nil {
				linkState = "unlinked"
			}
			ef := ent.Flags(s)
			eg := ent.GroundEntity(s)
			ev := ent.Velocity(s)
			ep := ent.PunchAngle(s)
			to := touch.Origin(s)
			eo := ent.Origin(s)
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
				"touchlinks callback end self=%d(%q) other=%d(%q) fn=%d self_solid=%d other_solid=%d self_link=%s other_flags=%#x other_ground=%d other_vel=(%.1f %.1f %.1f) other_punch=(%.1f %.1f %.1f) other_fixangle=%d other_teleport=%.3f self_origin=(%.1f %.1f %.1f) other_origin=(%.1f %.1f %.1f)",
				touchNum, touchClassName, entNum, moverClassName, touchTouchFn, int(touchSolid), int(entSolid), linkState,
				uint32(ef), int(eg), ev[0], ev[1], ev[2], ep[0], ep[1], ep[2],
				int(ent.FixAngle(s)), ent.TeleportTime(s),
				to[0], to[1], to[2], eo[0], eo[1], eo[2])
		}
	}
}

func (s *Server) areaTriggerEdicts(ent *Edict, node *AreaNode, list *[]*Edict, listCap int) {
	s.ensureCollisionSys()
	if node == nil && len(s.CollisionSys.Areanodes()) > 0 {
		node = &s.CollisionSys.Areanodes()[0]
	}
	s.CollisionSys.AreaTriggerEdicts(ent, node, list, listCap)
}
