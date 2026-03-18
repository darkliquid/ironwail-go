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
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
)

// Server holds the state for the current running game.
type Server struct {
	Active   bool
	Paused   bool
	LoadGame bool

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
	Edicts    []*Edict
	NumEdicts int
	MaxEdicts int

	// QuakeC VM integration
	QCVM *qc.VM
	// Static data (persists across levels)
	Static *ServerStatic

	// Area nodes for spatial partitioning
	Areanodes    []AreaNode
	numAreaNodes int

	// Network messaging
	Datagram *MessageBuffer

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

	// Protocol version (15=NetQuake, 666=FitzQuake, 999=RMQ)
	Protocol int

	// Cached QC field offsets for alpha/scale/items2 (populated once per progs.dat load).
	// -1 means the field doesn't exist in the loaded progs.
	QCFieldAlpha  int
	QCFieldScale  int
	QCFieldItems2 int

	// Game rules
	Coop       bool
	Deathmatch bool
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
func syncEdictToQCVM(vm *qc.VM, entNum int, ent *Edict) {
	if vm == nil || ent == nil || ent.Vars == nil || entNum < 0 || entNum >= vm.NumEdicts {
		return
	}
	fieldOffsets := qcFieldOffsets(vm)
	syncEntVarsToQC(vm, entNum, ent.Vars, fieldOffsets)
	if ent.Free {
		if modelIndexOfs, ok := fieldOffsets[normalizeFieldName("ModelIndex")]; ok {
			vm.SetEFloat(entNum, modelIndexOfs, 0)
		}
	}
}

// syncEdictFromQCVM pulls one VM edict's fields back into the Go Edict struct.
// It is used after QC mutates fields so server physics/network code can continue
// from the updated authoritative values produced by QuakeC logic.
func syncEdictFromQCVM(vm *qc.VM, entNum int, ent *Edict) {
	if vm == nil || ent == nil || ent.Vars == nil || entNum < 0 || entNum >= vm.NumEdicts {
		return
	}
	fieldOffsets := qcFieldOffsets(vm)
	syncEntVarsFromQC(vm, entNum, ent.Vars, fieldOffsets)
}

// qcFieldOffsets builds a normalized field-name → VM offset table for entvars.
// QuakeC field layouts are data-driven from progs.dat; this lookup lets reflection
// code map Go struct field names onto runtime VM offsets safely.
func qcFieldOffsets(vm *qc.VM) map[string]int {
	offsets := make(map[string]int, len(defaultEntFieldOffsets)+len(vm.FieldDefs))
	for key, ofs := range defaultEntFieldOffsets {
		offsets[key] = ofs
	}
	for _, def := range vm.FieldDefs {
		name := vm.GetString(def.Name)
		if name == "" {
			continue
		}
		offsets[normalizeFieldName(name)] = int(def.Ofs)
	}
	return offsets
}

var defaultEntFieldOffsets = map[string]int{
	normalizeFieldName("ModelIndex"):   qc.EntFieldModelIndex,
	normalizeFieldName("AbsMin"):       qc.EntFieldAbsMin,
	normalizeFieldName("AbsMax"):       qc.EntFieldAbsMax,
	normalizeFieldName("LTime"):        qc.EntFieldLTime,
	normalizeFieldName("MoveType"):     qc.EntFieldMoveType,
	normalizeFieldName("Solid"):        qc.EntFieldSolid,
	normalizeFieldName("Origin"):       qc.EntFieldOrigin,
	normalizeFieldName("OldOrigin"):    qc.EntFieldOldOrigin,
	normalizeFieldName("Velocity"):     qc.EntFieldVelocity,
	normalizeFieldName("Angles"):       qc.EntFieldAngles,
	normalizeFieldName("AVelocity"):    qc.EntFieldAVelocity,
	normalizeFieldName("PunchAngle"):   qc.EntFieldPunchAngle,
	normalizeFieldName("ClassName"):    qc.EntFieldClassName,
	normalizeFieldName("Model"):        qc.EntFieldModel,
	normalizeFieldName("Frame"):        qc.EntFieldFrame,
	normalizeFieldName("Skin"):         qc.EntFieldSkin,
	normalizeFieldName("Effects"):      qc.EntFieldEffects,
	normalizeFieldName("Mins"):         qc.EntFieldMins,
	normalizeFieldName("Maxs"):         qc.EntFieldMaxs,
	normalizeFieldName("Size"):         qc.EntFieldSize,
	normalizeFieldName("Touch"):        qc.EntFieldTouch,
	normalizeFieldName("Use"):          qc.EntFieldUse,
	normalizeFieldName("Think"):        qc.EntFieldThink,
	normalizeFieldName("Blocked"):      qc.EntFieldBlocked,
	normalizeFieldName("NextThink"):    qc.EntFieldNextThink,
	normalizeFieldName("GroundEntity"): qc.EntFieldGroundEnt,
	normalizeFieldName("Health"):       qc.EntFieldHealth,
	normalizeFieldName("Frags"):        qc.EntFieldFrags,
	normalizeFieldName("Weapon"):       qc.EntFieldWeapon,
	normalizeFieldName("WeaponModel"):  qc.EntFieldWeaponModel,
	normalizeFieldName("WeaponFrame"):  qc.EntFieldWeaponFrame,
	normalizeFieldName("CurrentAmmo"):  qc.EntFieldCurrentAmmo,
	normalizeFieldName("AmmoShells"):   qc.EntFieldAmmoShells,
	normalizeFieldName("AmmoNails"):    qc.EntFieldAmmoNails,
	normalizeFieldName("AmmoRockets"):  qc.EntFieldAmmoRockets,
	normalizeFieldName("AmmoCells"):    qc.EntFieldAmmoCells,
	normalizeFieldName("Items"):        qc.EntFieldItems,
	normalizeFieldName("TakeDamage"):   qc.EntFieldTakeDamage,
	normalizeFieldName("Chain"):        qc.EntFieldChain,
	normalizeFieldName("DeadFlag"):     qc.EntFieldDeadFlag,
	normalizeFieldName("ViewOfs"):      qc.EntFieldViewOfs,
	normalizeFieldName("Button0"):      qc.EntFieldButton0,
	normalizeFieldName("Button1"):      qc.EntFieldButton1,
	normalizeFieldName("Button2"):      qc.EntFieldButton2,
	normalizeFieldName("Impulse"):      qc.EntFieldImpulse,
	normalizeFieldName("FixAngle"):     qc.EntFieldFixAngle,
	normalizeFieldName("VAngle"):       qc.EntFieldVAngle,
	normalizeFieldName("IdealPitch"):   qc.EntFieldIdealPitch,
	normalizeFieldName("NetName"):      qc.EntFieldNetName,
	normalizeFieldName("Enemy"):        qc.EntFieldEnemy,
	normalizeFieldName("Flags"):        qc.EntFieldFlags,
	normalizeFieldName("Colormap"):     qc.EntFieldColormap,
	normalizeFieldName("Team"):         qc.EntFieldTeam,
	normalizeFieldName("MaxHealth"):    qc.EntFieldMaxHealth,
	normalizeFieldName("TeleportTime"): qc.EntFieldTeleportTime,
	normalizeFieldName("ArmorType"):    qc.EntFieldArmorType,
	normalizeFieldName("ArmorValue"):   qc.EntFieldArmorValue,
	normalizeFieldName("WaterLevel"):   qc.EntFieldWaterLevel,
	normalizeFieldName("WaterType"):    qc.EntFieldWaterType,
	normalizeFieldName("IdealYaw"):     qc.EntFieldIdealYaw,
	normalizeFieldName("YawSpeed"):     qc.EntFieldYawSpeed,
	normalizeFieldName("AimEnt"):       qc.EntFieldAimEnt,
	normalizeFieldName("GoalEntity"):   qc.EntFieldGoalEntity,
	normalizeFieldName("SpawnFlags"):   qc.EntFieldSpawnFlags,
	normalizeFieldName("Target"):       qc.EntFieldTarget,
	normalizeFieldName("TargetName"):   qc.EntFieldTargetName,
	normalizeFieldName("DmgTake"):      qc.EntFieldDmgTake,
	normalizeFieldName("DmgSave"):      qc.EntFieldDmgSave,
	normalizeFieldName("DmgInflictor"): qc.EntFieldDmgInflictor,
	normalizeFieldName("Owner"):        qc.EntFieldOwner,
	normalizeFieldName("MoveDir"):      qc.EntFieldMoveDir,
	normalizeFieldName("Message"):      qc.EntFieldMessage,
	normalizeFieldName("Sounds"):       qc.EntFieldSounds,
	normalizeFieldName("Noise"):        qc.EntFieldNoise,
	normalizeFieldName("Noise1"):       qc.EntFieldNoise1,
	normalizeFieldName("Noise2"):       qc.EntFieldNoise2,
	normalizeFieldName("Noise3"):       qc.EntFieldNoise3,
}

// syncEntVarsToQC reflects over EntVars and writes each mapped field into VM edict memory.
// This generic reflection pass avoids hand-writing dozens of assignments and keeps
// the Go-side EntVars schema synchronized with QuakeC-visible entity fields.
func syncEntVarsToQC(vm *qc.VM, entNum int, vars *EntVars, fieldOffsets map[string]int) {
	if vm == nil || vars == nil {
		return
	}
	rv := reflect.ValueOf(vars).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		ofs, ok := fieldOffsets[normalizeFieldName(field.Name)]
		if !ok {
			continue
		}
		value := rv.Field(i)
		switch value.Kind() {
		case reflect.Float32:
			vm.SetEFloat(entNum, ofs, float32(value.Float()))
		case reflect.Int32:
			vm.SetEInt(entNum, ofs, int32(value.Int()))
		case reflect.Array:
			if value.Len() != 3 || value.Type().Elem().Kind() != reflect.Float32 {
				continue
			}
			vm.SetEVector(entNum, ofs, [3]float32{
				float32(value.Index(0).Float()),
				float32(value.Index(1).Float()),
				float32(value.Index(2).Float()),
			})
		}
	}
}

// syncEntVarsFromQC reflects over EntVars and imports values from VM edict memory.
// It is the inverse of syncEntVarsToQC and keeps engine systems (physics, networking,
// savegames) in lockstep with whatever game DLL logic changed in QuakeC this frame.
func syncEntVarsFromQC(vm *qc.VM, entNum int, vars *EntVars, fieldOffsets map[string]int) {
	if vm == nil || vars == nil {
		return
	}
	rv := reflect.ValueOf(vars).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		ofs, ok := fieldOffsets[normalizeFieldName(field.Name)]
		if !ok {
			continue
		}
		value := rv.Field(i)
		switch value.Kind() {
		case reflect.Float32:
			value.SetFloat(float64(vm.EFloat(entNum, ofs)))
		case reflect.Int32:
			value.SetInt(int64(vm.EInt(entNum, ofs)))
		case reflect.Array:
			if value.Len() != 3 || value.Type().Elem().Kind() != reflect.Float32 {
				continue
			}
			vec := vm.EVector(entNum, ofs)
			value.Index(0).SetFloat(float64(vec[0]))
			value.Index(1).SetFloat(float64(vec[1]))
			value.Index(2).SetFloat(float64(vec[2]))
		}
	}
}

// ensureQCVMEdictStorage grows VM edict backing storage to match server edict capacity.
// QuakeC addresses entities by index into a flat byte block; this guarantees indexes
// the server hands to QC are always valid before any builtin or script executes.
func (s *Server) ensureQCVMEdictStorage() {
	if s.QCVM == nil || s.QCVM.EdictSize <= 0 {
		return
	}
	maxEdicts := s.MaxEdicts
	if maxEdicts < s.NumEdicts {
		maxEdicts = s.NumEdicts
	}
	if maxEdicts <= 0 {
		maxEdicts = s.NumEdicts
	}
	if s.QCVM.MaxEdicts < maxEdicts {
		s.QCVM.MaxEdicts = maxEdicts
	}
	needed := s.QCVM.EdictSize * s.QCVM.MaxEdicts
	if len(s.QCVM.Edicts) < needed {
		s.QCVM.Edicts = make([]byte, needed)
	}
	if s.QCVM.NumEdicts < s.NumEdicts {
		s.QCVM.NumEdicts = s.NumEdicts
	}
}

// syncQCVMState publishes core server globals and all live edicts into the QC VM.
// This is called at key boundaries (e.g. map spawn/load) so QuakeC starts from an
// accurate world snapshot before executing functions like worldspawn or client logic.
func (s *Server) syncQCVMState() {
	if s.QCVM == nil {
		return
	}
	skill := 1
	if skillCV := cvar.Get("skill"); skillCV != nil {
		skill = int(skillCV.Float + 0.5)
		if skill < 0 {
			skill = 0
		} else if skill > 3 {
			skill = 3
		}
		cvar.Set("skill", strconv.Itoa(skill))
	}
	s.ensureQCVMEdictStorage()
	s.QCVM.SetGlobal("world", 0)
	s.QCVM.SetGlobal("mapname", s.QCVM.AllocString(s.Name))
	s.QCVM.SetGlobal("skill", skill)
	s.QCVM.SetGlobal("time", s.Time)

	// C Ironwail sets coop/deathmatch globals before ED_LoadFromFile so
	// QC spawn functions can branch on game mode.
	coopVal := cvar.FloatValue("coop")
	dmVal := cvar.FloatValue("deathmatch")
	if coopVal != 0 {
		s.QCVM.SetGlobal("coop", float32(coopVal))
	} else {
		s.QCVM.SetGlobal("deathmatch", float32(dmVal))
	}

	for entNum := 0; entNum < s.NumEdicts; entNum++ {
		syncEdictToQCVM(s.QCVM, entNum, s.EdictNum(entNum))
	}
}

// NewServer creates a new server instance.
func NewServer() *Server {
	s := &Server{
		Gravity:      800,
		MaxVelocity:  2000,
		Friction:     4,
		StopSpeed:    100,
		MaxEdicts:    1024,
		Protocol:     ProtocolFitzQuake,
		QCFieldAlpha: -1,
		QCFieldScale: -1,
		QCVM:         qc.NewVM(),
	}

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
	ensurePrecache := func(cache *[]string, value string) {
		if value == "" {
			return
		}
		if len(*cache) == 0 {
			*cache = make([]string, 2)
			(*cache)[0] = ""
		}
		for _, existing := range *cache {
			if existing == value {
				return
			}
		}
		for i := 1; i < len(*cache); i++ {
			if (*cache)[i] == "" {
				(*cache)[i] = value
				return
			}
		}
		*cache = append(*cache, value)
	}
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
		case 2, 3:
			if s.Static == nil {
				return nil
			}
			buffers := make([]*MessageBuffer, 0, len(s.Static.Clients))
			for _, client := range s.Static.Clients {
				if client != nil && client.Message != nil {
					buffers = append(buffers, client.Message)
				}
			}
			return buffers
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
				InOpen:      s.PointContents(trace.EndPos) == bsp.ContentsEmpty,
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
			for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
				if vm.GetString(vm.EString(entNum, fieldOfs)) == match {
					return entNum
				}
			}
			return 0
		},
		FindFloat: func(vm *qc.VM, startEnt, fieldOfs int, match float32) int {
			for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
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
			for entNum := 1; entNum < vm.NumEdicts; entNum++ {
				entOrg := vm.EVector(entNum, qc.EntFieldOrigin)
				dx := entOrg[0] - org[0]
				dy := entOrg[1] - org[1]
				dz := entOrg[2] - org[2]
				if dx*dx+dy*dy+dz*dz <= radSq {
					return entNum
				}
			}
			return 0
		},
		CheckClient: func(vm *qc.VM) int {
			if s.Static == nil {
				return 0
			}
			self := int(vm.GInt(qc.OFSSelf))
			for _, client := range s.Static.Clients {
				if client == nil || !client.Active || client.Edict == nil || client.Edict.Free {
					continue
				}
				entNum := s.NumForEdict(client.Edict)
				if entNum > 0 && entNum != self {
					return entNum
				}
			}
			return 0
		},
		NextEnt: func(vm *qc.VM, entNum int) int {
			if entNum+1 > 0 && entNum+1 < vm.NumEdicts {
				return entNum + 1
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
			start[2] += ent.Vars.ViewOfs[2]
			targetNum := int(ent.Vars.AimEnt)
			if targetNum == 0 {
				targetNum = int(ent.Vars.Enemy)
			}
			if target := s.EdictNum(targetNum); target != nil && target.Vars != nil {
				end := target.Vars.Origin
				end[2] += 0.5 * (target.Vars.Mins[2] + target.Vars.Maxs[2])
				dir := [3]float32{end[0] - start[0], end[1] - start[1], end[2] - start[2]}
				if l := vm.VectorLength(dir); l > 0 {
					return vm.VectorNormalize(dir)
				}
			}
			return vm.GVector(qc.OFSGlobalVForward)
		},
		WalkMove: func(vm *qc.VM, yaw, dist float32) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			// Prefer server-side movement helpers which perform traces
			// and collision resolution. If the server has an Edict for
			// the entity, ask it to step in the given direction.
			if e := s.EdictNum(self); e != nil {
				ok := s.StepDirection(e, yaw, dist)
				// Mirror server-side origin back into the VM fields so
				// QuakeC sees the authoritative position.
				vm.SetEVector(self, qc.EntFieldOrigin, e.Vars.Origin)
				return ok
			}

			// Fallback: use a simple translation (should be rare).
			rad := float64(yaw) * math.Pi / 180.0
			dx := dist * float32(math.Cos(rad))
			dy := dist * float32(math.Sin(rad))
			org := vm.EVector(self, qc.EntFieldOrigin)
			newOrg := [3]float32{org[0] + dx, org[1] + dy, org[2]}
			vm.SetEVector(self, qc.EntFieldOrigin, newOrg)
			return true
		},
		DropToFloor: func(vm *qc.VM) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			// If the server has an Edict, run a downward trace using the
			// server Move helpers to land on the floor properly.
			if e := s.EdictNum(self); e != nil && e.Vars != nil {
				start := e.Vars.Origin
				end := start
				end[2] -= 1024 // large drop to find floor
				trace := s.SV_Move(start, e.Vars.Mins, e.Vars.Maxs, end, MoveType(MoveNormal), e)
				if trace.Fraction == 1 {
					return false
				}
				// Place entity on top of the surface found.
				newOrg := trace.EndPos
				vm.SetEVector(self, qc.EntFieldOrigin, newOrg)
				e.Vars.Origin = newOrg
				e.Vars.AbsMin = [3]float32{newOrg[0] + e.Vars.Mins[0], newOrg[1] + e.Vars.Mins[1], newOrg[2] + e.Vars.Mins[2]}
				e.Vars.AbsMax = [3]float32{newOrg[0] + e.Vars.Maxs[0], newOrg[1] + e.Vars.Maxs[1], newOrg[2] + e.Vars.Maxs[2]}
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
					panic("no precache: " + modelName)
				}
			}

			modelString := int32(0)
			if modelName != "" {
				modelString = vm.AllocString(modelName)
			}

			vm.SetEInt(entNum, qc.EntFieldModel, modelString)
			vm.SetEFloat(entNum, qc.EntFieldModelIndex, float32(modelIndex))

			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				e.Vars.Model = modelString
				e.Vars.ModelIndex = float32(modelIndex)
				if mins, maxs, ok := s.modelBounds(modelName); ok {
					e.Vars.Mins = mins
					e.Vars.Maxs = maxs
				} else {
					e.Vars.Mins = [3]float32{}
					e.Vars.Maxs = [3]float32{}
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
			ensurePrecache(&s.SoundPrecache, sample)
		},
		PrecacheModel: func(vm *qc.VM, modelName string) {
			ensurePrecache(&s.ModelPrecache, modelName)
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
				client.Message.WriteByte(byte(SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		ClientPrint: func(vm *qc.VM, entNum int, msg string) {
			console.Printf("%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		DebugPrint: func(vm *qc.VM, msg string) {
			console.Printf("%s", msg)
		},
		CenterPrint: func(vm *qc.VM, entNum int, msg string) {
			console.CenterPrintf(40, "%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(SVCCenterPrint))
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
				client.Message.WriteByte(byte(SVCStuffText))
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
				client.Message.WriteByte(byte(SVCLightStyle))
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
				Effects:    int(ent.Vars.Effects),
				Alpha:      ent.Alpha,
				Scale:      ent.Scale,
			}
			if state.Scale == 0 {
				state.Scale = 16
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
			s.MoveToGoal(ent, dist)
			syncEdictToQCVM(vm, entNum, ent)
		},
		ChangeYaw: func(vm *qc.VM) {
			entNum := int(vm.GInt(qc.OFSSelf))
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			s.changeYaw(ent)
			syncEdictToQCVM(vm, entNum, ent)
		},
	}))
	return s
}

// AllocEdict allocates a new entity.
func (s *Server) AllocEdict() *Edict {
	for i, e := range s.Edicts {
		if e.Free {
			s.NumEdicts = max(s.NumEdicts, i+1)
			s.ensureQCVMEdictStorage()
			syncEdictToQCVM(s.QCVM, i, e)
			return e
		}
	}

	if len(s.Edicts) >= s.MaxEdicts {
		return nil
	}

	e := &Edict{Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, e)
	s.NumEdicts = len(s.Edicts)
	s.ensureQCVMEdictStorage()
	syncEdictToQCVM(s.QCVM, s.NumEdicts-1, e)
	return e
}

// FreeEdict marks an entity as free.
func (s *Server) FreeEdict(e *Edict) {
	e.Free = true
	e.FreeTime = s.Time
}

// EdictNum returns the entity at the given index.
func (s *Server) EdictNum(n int) *Edict {
	if n < 0 || n >= len(s.Edicts) {
		return nil
	}
	return s.Edicts[n]
}

// NumForEdict returns the index for the given entity.
func (s *Server) NumForEdict(e *Edict) int {
	for i, ent := range s.Edicts {
		if ent == e {
			return i
		}
	}
	return -1
}

// GetMaxClients returns configured client slot count from persistent server static state.
func (s *Server) GetMaxClients() int {
	if s.Static == nil {
		return 0
	}
	return s.Static.MaxClients
}

// IsClientActive reports whether a client slot is currently occupied by an active connection.
func (s *Server) IsClientActive(clientNum int) bool {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return false
	}
	client := s.Static.Clients[clientNum]
	return client != nil && client.Active
}

// GetClientName returns the user-visible name for a connected client slot.
func (s *Server) GetClientName(clientNum int) string {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return ""
	}
	if s.Static.Clients[clientNum] == nil {
		return ""
	}
	return s.Static.Clients[clientNum].Name
}

// SetClientName updates a client's display name used by chat, scoreboards, and prints.
func (s *Server) SetClientName(clientNum int, name string) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	if s.Static.Clients[clientNum] == nil {
		return
	}
	s.Static.Clients[clientNum].Name = name
}

// GetClientColor returns the top/bottom shirt color bits for the given client slot.
func (s *Server) GetClientColor(clientNum int) int {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	if s.Static.Clients[clientNum] == nil {
		return 0
	}
	return s.Static.Clients[clientNum].Color
}

// SetClientColor updates a client's color setting used by player model colormap rendering.
func (s *Server) SetClientColor(clientNum int, color int) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	if s.Static.Clients[clientNum] == nil {
		return
	}
	s.Static.Clients[clientNum].Color = color
}

// GetClientPing returns average ping (ms) from the client's rolling network sample window.
func (s *Server) GetClientPing(clientNum int) float32 {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	c := s.Static.Clients[clientNum]
	if c == nil || c.NumPings == 0 {
		return 0
	}

	var total float32
	count := min(c.NumPings, 16)
	for i := 0; i < count; i++ {
		total += c.PingTimes[i]
	}
	return total / float32(count) * 1000
}

// KickClient sends a kick reason to a client then drops the connection from the server.
func (s *Server) KickClient(clientNum int, who, reason string) bool {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return false
	}
	client := s.Static.Clients[clientNum]
	if client == nil || !client.Active {
		return false
	}
	if who == "" {
		who = "Console"
	}

	if client.Message != nil {
		message := "Kicked by " + who
		if reason != "" {
			message += ": " + reason
		}
		client.Message.WriteByte(byte(SVCPrint))
		client.Message.WriteString(message + "\n")
	}

	s.DropClient(client, false)
	return true
}

// GetMapName returns the currently loaded map short name (without maps/ path or .bsp suffix).
func (s *Server) GetMapName() string {
	return s.Name
}

// SV_BroadcastPrintf prints formatted text to server console and all active clients reliably.
func (s *Server) SV_BroadcastPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	console.Printf("%s", msg)
	if s.Static == nil {
		return
	}
	for _, client := range s.Static.Clients {
		if client == nil || !client.Active || client.Message == nil {
			continue
		}
		client.Message.WriteByte(byte(SVCPrint))
		client.Message.WriteString(msg)
	}
}

// SV_ClientPrintf queues a formatted print message to a single client's reliable stream.
func (s *Server) SV_ClientPrintf(client *Client, format string, args ...any) {
	if client == nil || !client.Active || client.Message == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	client.Message.WriteByte(byte(SVCPrint))
	client.Message.WriteString(msg)
}
