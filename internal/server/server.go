// server.go — Core server-side game state for the Ironwail Go Quake engine port.
//
// This file defines the primary data structures that make up the running game
// server: the [Server] struct (per-level state), [ServerStatic] (cross-level
// persistent state), the [Client] struct (per-connected-player state), and the
// [AreaNode] spatial-partitioning tree used for entity collision queries.
//
// It also contains all QuakeC builtin hook registrations. The Quake engine
// executes game logic via a bytecode VM (the QuakeC VM, or QCVM). The engine
// exposes "builtins" — native Go functions — that QuakeC scripts call to
// interact with the world: spawning entities, performing traces, precaching
// resources, sending network messages, etc. The [NewServer] constructor wires
// these builtins so that QC code drives the authoritative server state defined
// here.
//
// A central design challenge is the "dual-representation" problem: every entity
// (edict) exists both as a typed Go struct ([Edict]/[EntVars]) and as a flat
// byte array inside the QC VM's memory. The sync* family of functions in this
// file bridges these two representations, copying data between them at key
// boundaries so that Go physics/networking code and QuakeC game logic always
// operate on consistent values.
package server

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Server holds the state for the current running game.
type Server struct {
	Active             bool
	Paused             bool
	LoadGame           bool
	PreserveSpawnParms bool

	State ServerState

	Name      string
	ModelName string

	WorldModel CollisionModel
	WorldTree  *bsp.Tree // BSP tree retained for rendering

	// Physics settings
	Gravity     float32
	MaxVelocity float32
	Friction    float32
	StopSpeed   float32

	// Timing
	Time      float32
	FrameTime float32

	// Entity management
	Edicts     []*Edict
	NumEdicts  int
	MaxEdicts  int
	peakEdicts int // Dev stats: peak active edict count
	devStats   DevStats
	devPeak    DevStats

	// QuakeC VM integration
	QCVM *qc.VM
	// Static data (persists across levels)
	Static *ServerStatic

	// Area nodes for spatial partitioning
	Areanodes    []AreaNode
	numAreaNodes int

	// Network messaging
	Datagram         *MessageBuffer
	ReliableDatagram *MessageBuffer

	// Signon buffer system - shared initial game state sent to connecting clients.
	// Populated during SpawnServer with precache lists, static entities, and sounds.
	SignonBuffers []*MessageBuffer
	Signon        *MessageBuffer // Current signon buffer being written to

	// Precached resources
	SoundPrecache  []string
	ModelPrecache  []string
	StaticEntities []EntityState
	StaticSounds   []StaticSound
	LightStyles    [64]string
	FileSystem     modelAssetFileSystem

	// Protocol version (15=NetQuake, 666=FitzQuake, 999=RMQ)
	Protocol int

	// Cached QC field offsets for alpha/scale/items2 (populated once per progs.dat load).
	// -1 means the field doesn't exist in the loaded progs.
	QCFieldAlpha   int
	QCFieldScale   int
	QCFieldGravity int
	QCFieldItems2  int

	// EffectsMask filters EntVars.Effects before serializing entity updates.
	// Defaults to 0xFF (all low 8 bits allowed) and can be narrowed when loaded
	// progs.dat does not define specific effect bits (e.g. QEX-only bits).
	EffectsMask int

	// Game rules
	Coop       bool
	Deathmatch bool

	DebugTelemetry *DebugTelemetry

	acceptConnection func() *inet.Socket

	checkClientSlot int
	checkClientTime float32
	checkClientPVS  []byte

	impactFrameActive bool
	impactFrameSeen   map[impactTouchKey]struct{}
	suppressTouchQC   bool

	compatRNG *compatrand.RNG

	modelCache map[string]cachedModelInfo
}

type entFieldKind uint8

const (
	entFieldFloat32 entFieldKind = iota
	entFieldInt32
	entFieldVec3
)

type entFieldBinding struct {
	fieldIndex int
	ofs        int
	kind       entFieldKind
}

type qcSyncCache struct {
	fieldOffsets   map[string]int
	entVarBindings []entFieldBinding
	modelIndexOfs  int
}

var qcSyncCaches sync.Map

type modelAssetFileSystem interface {
	OpenFile(filename string) (io.ReadSeekCloser, int64, error)
}

type impactTouchKey struct {
	self  int
	other int
	fn    int32
}

// DevStats mirrors C Ironwail's per-frame developer counters.
// Some fields are populated by server runtime while others are currently
// renderer/client-owned and may remain zero on the server side.
type DevStats struct {
	Frames     int
	PacketSize int
	Edicts     int
	Visedicts  int
	Efrags     int
	Tempents   int
	Beams      int
	DLights    int
	GPUUpload  int
}

// ServerStatic holds state that persists across level changes.
type ServerStatic struct {
	MaxClients        int
	MaxClientsLimit   int
	Clients           []*Client
	ServerFlags       int
	ChangeLevelIssued bool
}

// Client represents a connected player.
type Client struct {
	Active        bool
	Spawned       bool
	DropASAP      bool
	SendSignon    SignonStage
	Loopback      bool
	NetConnection *inet.Socket // Per-client network socket

	LastMessage float64

	Name  string
	Color int

	Edict *Edict

	PingTimes [16]float32
	NumPings  int

	SpawnParms [16]float32
	// Client input state
	LastCmd            UserCmd
	LoopbackCmdPending bool
	Message            *MessageBuffer
	SignonIdx          int
	OldFrags           int // Previous frags count for reliable message updates
	EntityStates       map[int]EntityState
	RespawnTime        float32
	FatPVS             []byte
	Stats              [32]int32
	OldStats           [32]int32
}

// AreaNode is a node in the spatial partitioning tree for entity collision.
type AreaNode struct {
	Axis          int
	Dist          float32
	Children      [2]*AreaNode
	TriggerEdicts Edict
	SolidEdicts   Edict
}

// syncEdictToQCVM copies one Go edict's EntVars into the QuakeC VM edict table.
// This is part of the engine↔QC bridge: before QC runs, the authoritative Go state
// is mirrored so QC builtins and scripts read the same fields (origin, health, etc.).
func NewServer() *Server {
	compatRNG := compatrand.New()
	vm := qc.NewVM()
	vm.SetCompatRNG(compatRNG)
	cvar.Register("sv_aim", "0.93", cvar.FlagNone, "Auto-aim cosine threshold")
	cvar.Register("teamplay", "0", cvar.FlagServerInfo, "Teamplay rules")

	s := &Server{
		Gravity:          800,
		MaxVelocity:      2000,
		Friction:         4,
		StopSpeed:        100,
		MaxEdicts:        1024,
		Protocol:         ProtocolFitzQuake,
		QCFieldAlpha:     -1,
		QCFieldScale:     -1,
		QCFieldGravity:   -1,
		EffectsMask:      defaultEffectsMask,
		QCVM:             vm,
		DebugTelemetry:   NewDebugTelemetry(),
		acceptConnection: inet.CheckNewConnections,
		impactFrameSeen:  make(map[impactTouchKey]struct{}),
		compatRNG:        compatRNG,
	}
	vm.IsServerActive = func() bool { return s.State == ServerStateActive }

	// Ensure entity 0 (worldspawn) exists so subsequent allocations
	// return entity indices starting at 1, matching the VM's
	// expectations where entity 0 is the level itself.
	world := &Edict{Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, world)
	s.NumEdicts = 1

	// Register a minimal set of builtin hooks so QuakeC code that calls
	// simple entity APIs (spawn/remove/nextent) will be routed to the
	// server-side implementations. This is intentionally conservative
	// — more sophisticated behaviors (walkmove, findradius, etc.) will
	// be implemented incrementally during the port.
	clientForEntNum := func(entNum int) *Client {
		if s.Static == nil {
			return nil
		}
		ent := s.EdictNum(entNum)
		if ent == nil {
			return nil
		}
		for _, client := range s.Static.Clients {
			if client != nil && client.Edict == ent {
				return client
			}
		}
		return nil
	}
	writeBuffers := func(vm *qc.VM, dest int) []*MessageBuffer {
		switch dest {
		case 0:
			if s.Datagram != nil {
				return []*MessageBuffer{s.Datagram}
			}
		case 1:
			msgEntityOfs := vm.FindGlobal("msg_entity")
			if msgEntityOfs < 0 {
				msgEntityOfs = qc.OFSMsgEntity
			}
			if client := clientForEntNum(int(vm.GInt(msgEntityOfs))); client != nil && client.Message != nil {
				return []*MessageBuffer{client.Message}
			}
		case 2:
			if s.ReliableDatagram != nil {
				return []*MessageBuffer{s.ReliableDatagram}
			}
		case 3:
			if s.Signon != nil {
				return []*MessageBuffer{s.Signon}
			}
		}
		return nil
	}
	// Register the engine-side implementations of QuakeC builtins.
	//
	// QC source calls builtins like:
	//   sound(self, CHAN_AUTO, "misc/hit.wav", 1, ATTN_NORM);
	// The VM places arguments on its parameter stack (OFS_PARM* globals), and
	// AdaptServerBuiltinHooks decodes those stack slots into typed Go arguments
	// for each closure below. These closures then bridge script intent into
	// authoritative engine systems: entity allocation, traces, networking,
	// precache lists, and world mutation.
	qc.RegisterServerHooks(qc.AdaptServerBuiltinHooks(qc.ServerBuiltinHooks{
		Traceline: func(vm *qc.VM, start, end [3]float32, noMonsters bool, passEnt int) qc.BuiltinTraceResult {
			moveType := MoveType(MoveNormal)
			if noMonsters {
				moveType = MoveType(MoveNoMonsters)
			}
			var pass *Edict
			if passEnt > 0 {
				pass = s.EdictNum(passEnt)
			}
			trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, moveType, pass)
			res := qc.BuiltinTraceResult{
				AllSolid:    trace.AllSolid,
				StartSolid:  trace.StartSolid,
				Fraction:    trace.Fraction,
				EndPos:      trace.EndPos,
				PlaneNormal: trace.PlaneNormal,
				PlaneDist:   trace.PlaneDist,
				InOpen:      trace.InOpen,
				InWater:     trace.InWater,
			}
			if trace.Entity != nil {
				res.EntNum = s.NumForEdict(trace.Entity)
			}
			return res
		},
		Spawn: func(vm *qc.VM) (int, error) {
			e := s.AllocEdict()
			if e == nil {
				return 0, errors.New("no free edict")
			}
			return s.NumForEdict(e), nil
		},
		Remove: func(vm *qc.VM, entNum int) error {
			e := s.EdictNum(entNum)
			if e == nil {
				return nil
			}
			s.FreeEdict(e)
			return nil
		},
		Find: func(vm *qc.VM, startEnt, fieldOfs int, match string) int {
			for entNum := startEnt + 1; entNum < s.NumEdicts && entNum < vm.NumEdicts; entNum++ {
				ent := s.EdictNum(entNum)
				if ent == nil || ent.Free {
					continue
				}
				if vm.GetString(vm.EString(entNum, fieldOfs)) == match {
					return entNum
				}
			}
			return 0
		},
		FindFloat: func(vm *qc.VM, startEnt, fieldOfs int, match float32) int {
			for entNum := startEnt + 1; entNum < s.NumEdicts && entNum < vm.NumEdicts; entNum++ {
				ent := s.EdictNum(entNum)
				if ent == nil || ent.Free {
					continue
				}
				if vm.EFloat(entNum, fieldOfs) == match {
					return entNum
				}
			}
			return 0
		},
		FindRadius: func(vm *qc.VM, org [3]float32, radius float32) int {
			if radius < 0 {
				return 0
			}
			radSq := radius * radius
			chain := 0
			for entNum := 1; entNum < vm.NumEdicts; entNum++ {
				if vm.EFloat(entNum, qc.EntFieldSolid) == float32(SolidNot) {
					continue
				}
				entOrg := vm.EVector(entNum, qc.EntFieldOrigin)
				mins := vm.EVector(entNum, qc.EntFieldMins)
				maxs := vm.EVector(entNum, qc.EntFieldMaxs)
				center := [3]float32{
					entOrg[0] + 0.5*(mins[0]+maxs[0]),
					entOrg[1] + 0.5*(mins[1]+maxs[1]),
					entOrg[2] + 0.5*(mins[2]+maxs[2]),
				}
				dx := center[0] - org[0]
				dy := center[1] - org[1]
				dz := center[2] - org[2]
				if dx*dx+dy*dy+dz*dz <= radSq {
					vm.SetEInt(entNum, qc.EntFieldChain, int32(chain))
					if ent := s.EdictNum(entNum); ent != nil && ent.Vars != nil {
						ent.Vars.Chain = int32(chain)
					}
					chain = entNum
				}
			}
			return chain
		},
		CheckClient: func(vm *qc.VM) int {
			if s.Static == nil {
				return 0
			}
			self := int(vm.GInt(qc.OFSSelf))
			if self > 0 && self < vm.NumEdicts {
				if selfEnt := s.EdictNum(self); selfEnt != nil && selfEnt.Vars != nil && !selfEnt.Free {
					syncEdictFromQCVM(vm, self, selfEnt)
				}
			}
			if s.checkClientSlot == 0 || s.Time-s.checkClientTime >= 0.1 {
				_ = s.newCheckClient()
				s.checkClientTime = s.Time
			}
			slot := s.checkClientSlot
			if slot <= 0 || slot > len(s.Static.Clients) {
				return 0
			}
			client := s.Static.Clients[slot-1]
			if client == nil || !client.Active || client.Edict == nil || client.Edict.Free || client.Edict.Vars.Health <= 0 {
				return 0
			}
			entNum := s.NumForEdict(client.Edict)
			if entNum <= 0 || entNum == self {
				return 0
			}
			if s.WorldTree != nil && len(s.WorldTree.Nodes) > 0 && len(s.checkClientPVS) > 0 {
				selfEnt := s.EdictNum(self)
				if selfEnt == nil || selfEnt.Vars == nil {
					return 0
				}
				view := [3]float32{
					selfEnt.Vars.Origin[0] + selfEnt.Vars.ViewOfs[0],
					selfEnt.Vars.Origin[1] + selfEnt.Vars.ViewOfs[1],
					selfEnt.Vars.Origin[2] + selfEnt.Vars.ViewOfs[2],
				}
				leaf := s.WorldTree.PointInLeaf(view)
				leafIdx := s.worldLeafIndex(leaf)
				if leafIdx < 0 {
					return 0
				}
				byteIdx := leafIdx >> 3
				if byteIdx >= len(s.checkClientPVS) || (s.checkClientPVS[byteIdx]&(1<<(uint(leafIdx)&7))) == 0 {
					return 0
				}
			}
			return entNum
		},
		NextEnt: func(vm *qc.VM, entNum int) int {
			for next := entNum + 1; next < s.NumEdicts && next < vm.NumEdicts; next++ {
				ent := s.EdictNum(next)
				if ent == nil || ent.Free {
					continue
				}
				return next
			}
			return 0
		},
		CheckBottom: func(vm *qc.VM, entNum int) bool {
			if entNum <= 0 {
				return false
			}
			e := s.EdictNum(entNum)
			if e == nil || e.Vars == nil {
				return false
			}
			syncEdictFromQCVM(vm, entNum, e)
			return s.CheckBottom(e)
		},
		PointContents: func(vm *qc.VM, point [3]float32) int {
			return s.PointContents(point)
		},
		Aim: func(vm *qc.VM, entNum int, missileSpeed float32) [3]float32 {
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil {
				return vm.GVector(qc.OFSGlobalVForward)
			}
			start := ent.Vars.Origin
			start[2] += 20
			bestDir := vm.GVector(qc.OFSGlobalVForward)
			end := VecAdd(start, VecScale(bestDir, 2048))
			trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNormal), ent)
			teamplay := cvar.FloatValue("teamplay") != 0
			if trace.Entity != nil && trace.Entity.Vars != nil &&
				TakeDamage(int(trace.Entity.Vars.TakeDamage)) == DamageAim &&
				(!teamplay || ent.Vars.Team <= 0 || ent.Vars.Team != trace.Entity.Vars.Team) {
				return bestDir
			}

			bestDist := float32(cvar.FloatValue("sv_aim"))
			var bestEnt *Edict
			for i := 1; i < s.NumEdicts && i < vm.NumEdicts; i++ {
				check := s.EdictNum(i)
				if check == nil || check.Free || check.Vars == nil || check == ent {
					continue
				}
				if TakeDamage(int(check.Vars.TakeDamage)) != DamageAim {
					continue
				}
				if teamplay && ent.Vars.Team > 0 && ent.Vars.Team == check.Vars.Team {
					continue
				}
				targetCenter := [3]float32{
					check.Vars.Origin[0] + 0.5*(check.Vars.Mins[0]+check.Vars.Maxs[0]),
					check.Vars.Origin[1] + 0.5*(check.Vars.Mins[1]+check.Vars.Maxs[1]),
					check.Vars.Origin[2] + 0.5*(check.Vars.Mins[2]+check.Vars.Maxs[2]),
				}
				dir := VecSub(targetCenter, start)
				if VecNormalize(&dir) == 0 {
					continue
				}
				dist := VecDot(dir, bestDir)
				if dist < bestDist {
					continue
				}
				trace = s.SV_Move(start, [3]float32{}, [3]float32{}, targetCenter, MoveType(MoveNormal), ent)
				if trace.Entity == check {
					bestDist = dist
					bestEnt = check
				}
			}
			if bestEnt == nil {
				return bestDir
			}
			dir := VecSub(bestEnt.Vars.Origin, ent.Vars.Origin)
			dist := VecDot(dir, bestDir)
			end = VecScale(bestDir, dist)
			end[2] = dir[2]
			VecNormalize(&end)
			return end
		},
		WalkMove: func(vm *qc.VM, yaw, dist float32) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			e := s.EdictNum(self)
			if e == nil || e.Vars == nil || e.Free {
				return false
			}
			syncEdictFromQCVM(vm, self, e)
			flags := uint32(e.Vars.Flags)
			if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
				return false
			}

			oldSelf := vm.GInt(qc.OFSSelf)
			oldOther := vm.GInt(qc.OFSOther)
			oldXFunction := vm.XFunction
			oldXFunctionIndex := vm.XFunctionIndex

			rad := float64(yaw) * math.Pi / 180.0
			move := [3]float32{
				dist * float32(math.Cos(rad)),
				dist * float32(math.Sin(rad)),
				0,
			}
			ok := s.MoveStep(e, move, true)
			vm.SetGInt(qc.OFSSelf, oldSelf)
			vm.SetGInt(qc.OFSOther, oldOther)
			vm.XFunction = oldXFunction
			vm.XFunctionIndex = oldXFunctionIndex
			syncEdictToQCVM(vm, self, e)
			return ok
		},
		DropToFloor: func(vm *qc.VM) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			// If the server has an Edict, run a downward trace using the
			// server Move helpers to land on the floor properly.
			if e := s.EdictNum(self); e != nil && e.Vars != nil {
				syncEdictFromQCVM(vm, self, e)
				start := e.Vars.Origin
				end := start
				end[2] -= 256
				trace := s.SV_Move(start, e.Vars.Mins, e.Vars.Maxs, end, MoveType(MoveNormal), e)
				if trace.Fraction == 1 || trace.AllSolid {
					return false
				}
				newOrg := trace.EndPos
				e.Vars.Origin = newOrg
				e.Vars.Flags = float32(uint32(e.Vars.Flags) | FlagOnGround)
				if trace.Entity != nil {
					e.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
				} else {
					e.Vars.GroundEntity = 0
				}
				s.LinkEdict(e, false)
				syncEdictToQCVM(vm, self, e)
				return true
			}

			// Fallback: naive placement as before.
			mins := vm.EVector(self, qc.EntFieldMins)
			org := vm.EVector(self, qc.EntFieldOrigin)
			org[2] = -mins[2]
			vm.SetEVector(self, qc.EntFieldOrigin, org)
			return true
		},
		SetOrigin: func(vm *qc.VM, entNum int, org [3]float32) {
			vm.SetEVector(entNum, qc.EntFieldOrigin, org)
			mins := vm.EVector(entNum, qc.EntFieldMins)
			maxs := vm.EVector(entNum, qc.EntFieldMaxs)
			vm.SetEVector(entNum, qc.EntFieldAbsMin, [3]float32{org[0] + mins[0], org[1] + mins[1], org[2] + mins[2]})
			vm.SetEVector(entNum, qc.EntFieldAbsMax, [3]float32{org[0] + maxs[0], org[1] + maxs[1], org[2] + maxs[2]})
			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				syncEdictFromQCVM(vm, entNum, e)
				e.Vars.Origin = org
				e.Vars.AbsMin = [3]float32{org[0] + e.Vars.Mins[0], org[1] + e.Vars.Mins[1], org[2] + e.Vars.Mins[2]}
				e.Vars.AbsMax = [3]float32{org[0] + e.Vars.Maxs[0], org[1] + e.Vars.Maxs[1], org[2] + e.Vars.Maxs[2]}
				s.LinkEdict(e, false)
			}
		},
		SetSize: func(vm *qc.VM, entNum int, mins, maxs [3]float32) {
			vm.SetEVector(entNum, qc.EntFieldMins, mins)
			vm.SetEVector(entNum, qc.EntFieldMaxs, maxs)
			size := [3]float32{maxs[0] - mins[0], maxs[1] - mins[1], maxs[2] - mins[2]}
			vm.SetEVector(entNum, qc.EntFieldSize, size)
			origin := vm.EVector(entNum, qc.EntFieldOrigin)
			vm.SetEVector(entNum, qc.EntFieldAbsMin, [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]})
			vm.SetEVector(entNum, qc.EntFieldAbsMax, [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]})
			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				syncEdictFromQCVM(vm, entNum, e)
				e.Vars.Mins = mins
				e.Vars.Maxs = maxs
				e.Vars.Size = size
				e.Vars.AbsMin = [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]}
				e.Vars.AbsMax = [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]}
				// C's SetMinMaxSize calls SV_LinkEdict(e, false) to update
				// the spatial partition after bounds change.
				s.LinkEdict(e, false)
			}
		},
		SetModel: func(vm *qc.VM, entNum int, modelName string) {
			modelIndex := 0
			if modelName != "" {
				modelIndex = s.FindModel(modelName)
				if modelIndex == 0 {
					s.raiseQCRuntimeError(vm, "no precache: %s", modelName)
					return
				}
			}

			modelString := int32(0)
			if modelName != "" {
				modelString = vm.AllocString(modelName)
			}

			vm.SetEInt(entNum, qc.EntFieldModel, modelString)
			vm.SetEFloat(entNum, qc.EntFieldModelIndex, float32(modelIndex))

			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				syncEdictFromQCVM(vm, entNum, e)
				e.Vars.Model = modelString
				e.Vars.ModelIndex = float32(modelIndex)
				if mins, maxs, ok := s.modelBounds(modelName); ok {
					e.Vars.Mins = mins
					e.Vars.Maxs = maxs
				} else {
					if info, err := s.cacheModelInfo(modelName); err == nil && info.known {
						e.Vars.Mins = info.mins
						e.Vars.Maxs = info.maxs
					} else {
						e.Vars.Mins = [3]float32{}
						e.Vars.Maxs = [3]float32{}
					}
				}
				e.Vars.Size = [3]float32{
					e.Vars.Maxs[0] - e.Vars.Mins[0],
					e.Vars.Maxs[1] - e.Vars.Mins[1],
					e.Vars.Maxs[2] - e.Vars.Mins[2],
				}
				s.LinkEdict(e, false)

				// Only push the fields SetModel actually modified back to
				// the QCVM. A full syncEdictToQCVM here would clobber any
				// QC-set fields (e.g. solid, touch, movetype) that were
				// changed between builtins within the same QC function.
				vm.SetEVector(entNum, qc.EntFieldMins, e.Vars.Mins)
				vm.SetEVector(entNum, qc.EntFieldMaxs, e.Vars.Maxs)
				vm.SetEVector(entNum, qc.EntFieldSize, e.Vars.Size)
				// LinkEdict updates AbsMin/AbsMax (including fat-1 for BSP
				// entities); push those back so QC sees consistent bounds.
				vm.SetEVector(entNum, qc.EntFieldAbsMin, e.Vars.AbsMin)
				vm.SetEVector(entNum, qc.EntFieldAbsMax, e.Vars.AbsMax)
			}
		},
		PrecacheSound: func(vm *qc.VM, sample string) {
			if err := s.precacheSound(sample); err != nil {
				s.raiseQCRuntimeError(vm, "%v", err)
			}
		},
		PrecacheModel: func(vm *qc.VM, modelName string) {
			if err := s.precacheModel(modelName); err != nil {
				s.raiseQCRuntimeError(vm, "%v", err)
			}
		},
		BroadcastPrint: func(vm *qc.VM, msg string) {
			console.Printf("%s", msg)
			if s.Static == nil {
				return
			}
			for _, client := range s.Static.Clients {
				if client == nil || client.Message == nil {
					continue
				}
				client.Message.WriteByte(byte(inet.SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		ClientPrint: func(vm *qc.VM, entNum int, msg string) {
			console.Printf("%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(inet.SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		DebugPrint: func(vm *qc.VM, msg string) {
			console.Printf("%s", msg)
		},
		CenterPrint: func(vm *qc.VM, entNum int, msg string) {
			console.CenterPrintf(40, "%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(inet.SVCCenterPrint))
				client.Message.WriteString(msg)
			}
		},
		Sound: func(vm *qc.VM, entNum, channel int, sample string, volume int, attenuation float32) {
			if ent := s.EdictNum(entNum); ent != nil {
				s.StartSound(ent, channel, sample, volume, attenuation)
			}
		},
		StuffCmd: func(vm *qc.VM, entNum int, cmd string) {
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(inet.SVCStuffText))
				client.Message.WriteString(cmd)
			}
		},
		LightStyle: func(vm *qc.VM, style int, value string) {
			if s.Static == nil || style < 0 || style >= len(s.LightStyles) {
				return
			}
			s.LightStyles[style] = value
			for _, client := range s.Static.Clients {
				if client == nil || client.Message == nil {
					continue
				}
				client.Message.WriteByte(byte(inet.SVCLightStyle))
				client.Message.WriteByte(byte(style))
				client.Message.WriteString(value)
			}
		},
		Particle: func(vm *qc.VM, org, dir [3]float32, color, count int) {
			s.StartParticle(org, dir, color, count)
		},
		LocalSound: func(vm *qc.VM, entNum int, sample string) {
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				s.LocalSound(client, sample)
			}
		},
		WriteByte: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteByte(byte(value))
			}
		},
		WriteChar: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteChar(int8(value))
			}
		},
		WriteShort: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteShort(int16(value))
			}
		},
		WriteLong: func(vm *qc.VM, dest int, value int32) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteLong(value)
			}
		},
		WriteCoord: func(vm *qc.VM, dest int, value float32) {
			flags := uint32(s.ProtocolFlags())
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteCoord(value, flags)
			}
		},
		WriteAngle: func(vm *qc.VM, dest int, value float32) {
			flags := uint32(s.ProtocolFlags())
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteAngle(value, flags)
			}
		},
		WriteString: func(vm *qc.VM, dest int, value string) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteString(value)
			}
		},
		WriteEntity: func(vm *qc.VM, dest, entNum int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteShort(int16(entNum))
			}
		},
		SetSpawnParms: func(vm *qc.VM, entNum int) {
			if s.Static == nil {
				return
			}
			ent := s.EdictNum(entNum)
			if ent == nil {
				return
			}
			for _, client := range s.Static.Clients {
				if client == nil || client.Edict != ent {
					continue
				}
				for i := 0; i < len(client.SpawnParms); i++ {
					parmOfs := vm.FindGlobal("parm" + strconv.Itoa(i+1))
					if parmOfs >= 0 {
						vm.SetGFloat(parmOfs, client.SpawnParms[i])
						continue
					}
					if qc.OFSParmStart+i < len(vm.Globals) {
						vm.SetGFloat(qc.OFSParmStart+i, client.SpawnParms[i])
					}
				}
				return
			}
		},
		MakeStatic: func(vm *qc.VM, entNum int) {
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			state := EntityState{
				Origin:     ent.Vars.Origin,
				Angles:     ent.Vars.Angles,
				ModelIndex: int(ent.Vars.ModelIndex),
				Frame:      int(ent.Vars.Frame),
				Colormap:   int(ent.Vars.Colormap),
				Skin:       int(ent.Vars.Skin),
				Effects:    int(ent.Vars.Effects) & s.effectsMask(),
				Alpha:      ent.Alpha,
				Scale:      ent.Scale,
			}
			if state.Scale == 0 {
				state.Scale = 16
			}
			if state.Alpha == inet.ENTALPHA_ZERO {
				UnlinkEdict(ent)
				s.FreeEdict(ent)
				return
			}
			if s.Protocol == ProtocolNetQuake && (state.ModelIndex > 255 || state.Frame > 255) {
				UnlinkEdict(ent)
				s.FreeEdict(ent)
				return
			}
			s.StaticEntities = append(s.StaticEntities, state)
			if s.Static != nil {
				for _, client := range s.Static.Clients {
					if client == nil || !client.Active || client.Message == nil {
						continue
					}
					s.writeSpawnStaticMessage(client.Message, state)
				}
			}
			UnlinkEdict(ent)
			s.FreeEdict(ent)
		},
		AmbientSound: func(vm *qc.VM, org [3]float32, sample string, volume int, attenuation float32) {
			soundIndex := s.FindSound(sample)
			if soundIndex < 0 {
				return
			}
			st := StaticSound{
				Origin:      org,
				SoundIndex:  soundIndex,
				Volume:      volume,
				Attenuation: attenuation,
			}
			s.StaticSounds = append(s.StaticSounds, st)
			if s.Static != nil {
				for _, client := range s.Static.Clients {
					if client == nil || !client.Active || client.Message == nil {
						continue
					}
					s.writeSpawnStaticSoundMessage(client.Message, st)
				}
			}
		},
		MoveToGoal: func(vm *qc.VM, dist float32) {
			entNum := int(vm.GInt(qc.OFSSelf))
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			syncEdictFromQCVM(vm, entNum, ent)
			if goalNum := int(ent.Vars.GoalEntity); goalNum > 0 {
				if goal := s.EdictNum(goalNum); goal != nil && goal.Vars != nil && !goal.Free {
					syncEdictFromQCVM(vm, goalNum, goal)
				}
			}
			oldSelf := vm.GInt(qc.OFSSelf)
			oldOther := vm.GInt(qc.OFSOther)
			oldXFunction := vm.XFunction
			oldXFunctionIndex := vm.XFunctionIndex
			s.MoveToGoal(ent, dist)
			vm.SetGInt(qc.OFSSelf, oldSelf)
			vm.SetGInt(qc.OFSOther, oldOther)
			vm.XFunction = oldXFunction
			vm.XFunctionIndex = oldXFunctionIndex
			syncEdictToQCVM(vm, entNum, ent)
		},
		ChangeYaw: func(vm *qc.VM) {
			entNum := int(vm.GInt(qc.OFSSelf))
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			syncEdictFromQCVM(vm, entNum, ent)
			s.changeYaw(ent)
			syncEdictToQCVM(vm, entNum, ent)
		},
		IssueChangeLevel: func(vm *qc.VM, level string) bool {
			level = strings.TrimSpace(level)
			if level == "" || s.Static == nil || s.Static.ChangeLevelIssued {
				return false
			}
			s.Static.ChangeLevelIssued = true
			cmdsys.AddText("changelevel " + level + "\n")
			return true
		},
	}))
	return s
}
