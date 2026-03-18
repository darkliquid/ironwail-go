// Package qc provides QuakeC built-in functions.
//
// This file implements the built-in functions that QuakeC code can call.
package qc

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

type BuiltinTraceResult struct {
	AllSolid    bool
	StartSolid  bool
	Fraction    float32
	EndPos      [3]float32
	PlaneNormal [3]float32
	PlaneDist   float32
	EntNum      int
	InOpen      bool
	InWater     bool
}

type ServerBuiltinHooks struct {
	Traceline      func(vm *VM, start, end [3]float32, noMonsters bool, passEnt int) BuiltinTraceResult
	Spawn          func(vm *VM) (int, error)
	Remove         func(vm *VM, entNum int) error
	Find           func(vm *VM, startEnt, fieldOfs int, match string) int
	FindFloat      func(vm *VM, startEnt, fieldOfs int, match float32) int
	FindRadius     func(vm *VM, org [3]float32, radius float32) int
	CheckClient    func(vm *VM) int
	NextEnt        func(vm *VM, entNum int) int
	CheckBottom    func(vm *VM, entNum int) bool
	PointContents  func(vm *VM, point [3]float32) int
	Aim            func(vm *VM, entNum int, missileSpeed float32) [3]float32
	WalkMove       func(vm *VM, yaw, dist float32) bool
	DropToFloor    func(vm *VM) bool
	SetOrigin      func(vm *VM, entNum int, org [3]float32)
	SetSize        func(vm *VM, entNum int, mins, maxs [3]float32)
	SetModel       func(vm *VM, entNum int, modelName string)
	PrecacheSound  func(vm *VM, sample string)
	PrecacheModel  func(vm *VM, modelName string)
	BroadcastPrint func(vm *VM, msg string)
	ClientPrint    func(vm *VM, entNum int, msg string)
	DebugPrint     func(vm *VM, msg string)
	CenterPrint    func(vm *VM, entNum int, msg string)
	Sound          func(vm *VM, entNum, channel int, sample string, volume int, attenuation float32)
	StuffCmd       func(vm *VM, entNum int, cmd string)
	LightStyle     func(vm *VM, style int, value string)
	Particle       func(vm *VM, org, dir [3]float32, color, count int)
	LocalSound     func(vm *VM, entNum int, sample string)
	WriteByte      func(vm *VM, dest, value int)
	WriteChar      func(vm *VM, dest, value int)
	WriteShort     func(vm *VM, dest, value int)
	WriteLong      func(vm *VM, dest int, value int32)
	WriteCoord     func(vm *VM, dest int, value float32)
	WriteAngle     func(vm *VM, dest int, value float32)
	WriteString    func(vm *VM, dest int, value string)
	WriteEntity    func(vm *VM, dest, entNum int)
	SetSpawnParms  func(vm *VM, entNum int)
	MakeStatic     func(vm *VM, entNum int)
	AmbientSound   func(vm *VM, org [3]float32, sample string, volume int, attenuation float32)
	MoveToGoal     func(vm *VM, dist float32)
	ChangeYaw      func(vm *VM)
}

var serverBuiltinHooks ServerBuiltinHooks

// SetServerBuiltinHooks sets the low-level function hooks consumed by
// the builtin implementations. Prefer `RegisterServerHooks` which
// accepts the typed `ServerHooks` interface defined in
// `internal/qc/serverhooks_iface.go` — this adapter keeps existing
// callers working while allowing testable, interface-based server
// implementations.
func SetServerBuiltinHooks(hooks ServerBuiltinHooks) {
	serverBuiltinHooks = hooks
}

// RegisterBuiltins registers all QuakeC built-in functions with the VM.
func RegisterBuiltins(vm *VM) {
	// Builtin numbers match standard Quake numbering

	// Group 1-10: Vector/Math Operations
	vm.Builtins[1] = makevectors
	// Canonical mapping (per pr_cmds.c):
	// 2 = setorigin, 3 = setmodel, 4 = setsize
	vm.Builtins[2] = setorigin
	vm.Builtins[3] = setmodel
	vm.Builtins[4] = setsize
	// 6 = break
	vm.Builtins[6] = breakBuiltin
	// 7 = random, 9 = normalize, 12 = vlen, 13 = vectoyaw
	vm.Builtins[7] = random
	vm.Builtins[8] = sound
	vm.Builtins[9] = normalize
	vm.Builtins[10] = errorBuiltin
	vm.Builtins[11] = objerrorBuiltin
	vm.Builtins[12] = vlen
	vm.Builtins[13] = vectoyaw

	// Group 11-20: Entity Management (canonical indices)
	vm.Builtins[14] = spawn
	vm.Builtins[15] = remove
	vm.Builtins[16] = traceline
	vm.Builtins[17] = checkclient
	vm.Builtins[18] = find
	vm.Builtins[19] = precacheSound
	vm.Builtins[20] = precacheModel
	vm.Builtins[21] = stuffcmd
	vm.Builtins[22] = findradius
	vm.Builtins[23] = bprint
	vm.Builtins[24] = sprint
	vm.Builtins[25] = dprint
	vm.Builtins[26] = ftosBuiltin
	vm.Builtins[27] = vtosBuiltin
	vm.Builtins[28] = noopBuiltin
	vm.Builtins[29] = noopBuiltin
	vm.Builtins[30] = noopBuiltin
	vm.Builtins[31] = eprint

	// Group 21-40: Physics/Movement (canonical indices)
	vm.Builtins[32] = walkmove
	vm.Builtins[34] = droptofloor
	vm.Builtins[35] = lightstyle
	vm.Builtins[36] = rintBuiltin
	vm.Builtins[37] = floorBuiltin
	vm.Builtins[38] = ceilBuiltin
	vm.Builtins[40] = checkbottom
	vm.Builtins[41] = pointcontents
	vm.Builtins[43] = fabsBuiltin
	vm.Builtins[44] = aimBuiltin
	vm.Builtins[45] = cvarBuiltin
	vm.Builtins[46] = localcmd
	// nextent canonical index is 47
	vm.Builtins[47] = nextent
	vm.Builtins[48] = particle
	// ChangeYaw / movetogoal / vectoangles have canonical indices 49, 67, 51
	vm.Builtins[49] = changeyaw
	vm.Builtins[51] = vectoangles
	vm.Builtins[52] = writeByteBuiltin
	vm.Builtins[53] = writeCharBuiltin
	vm.Builtins[54] = writeShortBuiltin
	vm.Builtins[55] = writeLongBuiltin
	vm.Builtins[56] = writeCoordBuiltin
	vm.Builtins[57] = writeAngleBuiltin
	vm.Builtins[58] = writeStringBuiltin
	vm.Builtins[59] = writeEntityBuiltin
	vm.Builtins[67] = movetogoal
	vm.Builtins[68] = precacheFile
	vm.Builtins[69] = makestatic
	vm.Builtins[70] = changelevel
	vm.Builtins[72] = cvarSetBuiltin
	vm.Builtins[73] = centerprint
	vm.Builtins[74] = ambientsound
	vm.Builtins[78] = setspawnparms
	vm.Builtins[79] = finaleFinished
	vm.Builtins[80] = localsound

	// Non-canonical helpers we still want available: place them at high indices
	vm.Builtins[1000] = findfloat

	// Set NumBuiltins to cover the canonical builtin table (highest standard index used)
	vm.NumBuiltins = 627
}
func noopBuiltin(vm *VM) {
	vm.SetGFloat(OFSReturn, 0)
}

func breakBuiltin(vm *VM) {
	vm.SetGFloat(OFSReturn, 0)
}

func errorBuiltin(vm *VM) {
	console.Printf("QC error: %s", vm.GString(OFSParm0))
}

func objerrorBuiltin(vm *VM) {
	console.Printf("QC objerror: %s", vm.GString(OFSParm0))
}
// ftosBuiltin converts a float to a string. If the float is an integer value,
// formats as "%d", otherwise as "%5.1f". Matches C PF_ftos behavior exactly.
func ftosBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	var s string
	if v == float32(int(v)) {
		s = fmt.Sprintf("%d", int(v))
	} else {
		s = fmt.Sprintf("%5.1f", v)
	}
	vm.SetGString(OFSReturn, s)
}

// vtosBuiltin converts a vector to a string in the format "'%5.1f %5.1f %5.1f'".
// Matches C PF_vtos behavior exactly.
func vtosBuiltin(vm *VM) {
	vec := vm.GVector(OFSParm0)
	s := fmt.Sprintf("'%5.1f %5.1f %5.1f'", vec[0], vec[1], vec[2])
	vm.SetGString(OFSReturn, s)
}

func cvarBuiltin(vm *VM) {
	vm.SetGFloat(OFSReturn, float32(cvar.FloatValue(vm.GString(OFSParm0))))
}

func cvarSetBuiltin(vm *VM) {
	cvar.Set(vm.GString(OFSParm0), vm.GString(OFSParm1))
	vm.SetGFloat(OFSReturn, 0)
}

func localcmd(vm *VM) {
	cmdsys.AddText(vm.GString(OFSParm0))
	vm.SetGFloat(OFSReturn, 0)
}

func writeByteBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteByte != nil {
		serverBuiltinHooks.WriteByte(vm, int(vm.GFloat(OFSParm0)), int(vm.GFloat(OFSParm1)))
	}
}

func writeCharBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteChar != nil {
		serverBuiltinHooks.WriteChar(vm, int(vm.GFloat(OFSParm0)), int(vm.GFloat(OFSParm1)))
	}
}

func writeShortBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteShort != nil {
		serverBuiltinHooks.WriteShort(vm, int(vm.GFloat(OFSParm0)), int(vm.GFloat(OFSParm1)))
	}
}

func writeLongBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteLong != nil {
		serverBuiltinHooks.WriteLong(vm, int(vm.GFloat(OFSParm0)), int32(vm.GFloat(OFSParm1)))
	}
}

func writeCoordBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteCoord != nil {
		serverBuiltinHooks.WriteCoord(vm, int(vm.GFloat(OFSParm0)), vm.GFloat(OFSParm1))
	}
}

func writeAngleBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteAngle != nil {
		serverBuiltinHooks.WriteAngle(vm, int(vm.GFloat(OFSParm0)), vm.GFloat(OFSParm1))
	}
}

func writeStringBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteString != nil {
		serverBuiltinHooks.WriteString(vm, int(vm.GFloat(OFSParm0)), vm.GString(OFSParm1))
	}
}

func writeEntityBuiltin(vm *VM) {
	if serverBuiltinHooks.WriteEntity != nil {
		serverBuiltinHooks.WriteEntity(vm, int(vm.GFloat(OFSParm0)), int(vm.GFloat(OFSParm1)))
	}
}

func setTraceGlobals(vm *VM, trace BuiltinTraceResult) {
	if trace.AllSolid {
		vm.SetGFloat(OFSTraceAllSolid, 1)
	} else {
		vm.SetGFloat(OFSTraceAllSolid, 0)
	}
	if trace.StartSolid {
		vm.SetGFloat(OFSTraceStartSolid, 1)
	} else {
		vm.SetGFloat(OFSTraceStartSolid, 0)
	}
	vm.SetGFloat(OFSTraceFraction, trace.Fraction)
	vm.SetGVector(OFSTraceEndPos, trace.EndPos)
	vm.SetGVector(OFSTracePlaneNormal, trace.PlaneNormal)
	vm.SetGFloat(OFSTracePlaneDist, trace.PlaneDist)
	vm.SetGFloat(OFSTraceEnt, float32(trace.EntNum))
	if trace.InOpen {
		vm.SetGFloat(OFSTraceInOpen, 1)
	} else {
		vm.SetGFloat(OFSTraceInOpen, 0)
	}
	if trace.InWater {
		vm.SetGFloat(OFSTraceInWater, 1)
	} else {
		vm.SetGFloat(OFSTraceInWater, 0)
	}
}
func precacheSound(vm *VM) {
	sample := vm.GString(OFSParm0)
	if serverBuiltinHooks.PrecacheSound != nil {
		serverBuiltinHooks.PrecacheSound(vm, sample)
	}
	vm.SetGInt(OFSReturn, vm.GInt(OFSParm0))
}

// precacheModel records a model resource for later lookup.
func precacheModel(vm *VM) {
	modelName := vm.GString(OFSParm0)
	if serverBuiltinHooks.PrecacheModel != nil {
		serverBuiltinHooks.PrecacheModel(vm, modelName)
	}
	vm.SetGInt(OFSReturn, vm.GInt(OFSParm0))
}

func stuffcmd(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	cmd := vm.GString(OFSParm1)
	if serverBuiltinHooks.StuffCmd != nil {
		serverBuiltinHooks.StuffCmd(vm, entNum, cmd)
		return
	}
	cmdsys.AddText(cmd)
}

// bprint prints a broadcast message.
func bprint(vm *VM) {
	msg := vm.GString(OFSParm0)
	if serverBuiltinHooks.BroadcastPrint != nil {
		serverBuiltinHooks.BroadcastPrint(vm, msg)
		return
	}
	console.Printf("%s", msg)
}

// sprint prints a message intended for one client.
func sprint(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	msg := vm.GString(OFSParm1)
	if serverBuiltinHooks.ClientPrint != nil {
		serverBuiltinHooks.ClientPrint(vm, entNum, msg)
		return
	}
	console.Printf("%s", msg)
}

// dprint prints a developer/debug message.
func dprint(vm *VM) {
	msg := vm.GString(OFSParm0)
	if serverBuiltinHooks.DebugPrint != nil {
		serverBuiltinHooks.DebugPrint(vm, msg)
		return
	}
	console.Printf("%s", msg)
}

func eprint(vm *VM) {
	console.Printf("entity %d", int(vm.GFloat(OFSParm0)))
}
func precacheFile(vm *VM) {
	vm.SetGInt(OFSReturn, vm.GInt(OFSParm0))
}
func changelevel(vm *VM) {
	cmdsys.AddText("changelevel " + vm.GString(OFSParm0) + "\n")
	vm.SetGFloat(OFSReturn, 0)
}
func finaleFinished(vm *VM) {
	vm.SetGFloat(OFSReturn, 0)
}
