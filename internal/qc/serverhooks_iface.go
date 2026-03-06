package qc

// ServerHooks is a small interface that describes the server-side
// operations the QuakeC builtins call into. Exporting this interface
// allows the engine to provide a mock or a thin adapter in tests and
// keeps the `qc` package dependent only on a minimal contract.
//
// Typical implementations live in the `internal/server` package and
// provide entity allocation, lookup and movement helpers used by
// QuakeC builtins such as `spawn`, `find`, `walkmove` and others.
type ServerHooks interface {
	// Traceline performs a collision trace and returns the trace globals.
	Traceline(vm *VM, start, end [3]float32, noMonsters bool, passEnt int) BuiltinTraceResult

	// Spawn allocates and returns a new entity index. It may return
	// an error if allocation fails.
	Spawn(vm *VM) (int, error)

	// Remove deallocates entity `entNum` and returns nil on success.
	Remove(vm *VM, entNum int) error

	// Find searches for an entity starting after `startEnt` whose field
	// at `fieldOfs` matches `match`. Returns entity index or 0.
	Find(vm *VM, startEnt, fieldOfs int, match string) int

	// FindFloat searches for an entity with a float match in a field.
	FindFloat(vm *VM, startEnt, fieldOfs int, match float32) int

	// FindRadius finds an entity within `radius` of `org`.
	FindRadius(vm *VM, org [3]float32, radius float32) int

	// CheckClient returns a candidate client entity for AI targeting.
	CheckClient(vm *VM) int

	// NextEnt returns the next entity index after entNum or 0.
	NextEnt(vm *VM, entNum int) int

	// CheckBottom reports whether the entity is supported by solid ground.
	CheckBottom(vm *VM, entNum int) bool

	// PointContents returns BSP contents at a point.
	PointContents(vm *VM, point [3]float32) int

	// Aim returns an aim direction for the given entity.
	Aim(vm *VM, entNum int, missileSpeed float32) [3]float32

	// WalkMove moves the current entity forward by `dist` at `yaw`.
	// Returns true on success (moved) or false on collision.
	WalkMove(vm *VM, yaw, dist float32) bool

	// DropToFloor moves the current entity down to the floor.
	DropToFloor(vm *VM) bool

	// SetOrigin and SetSize update entity transform and bounding box.
	SetOrigin(vm *VM, entNum int, org [3]float32)
	SetSize(vm *VM, entNum int, mins, maxs [3]float32)

	// SetModel assigns a model name to an entity and updates related
	// model indices/animation state as needed by the server.
	SetModel(vm *VM, entNum int, modelName string)

	// PrecacheSound/PrecacheModel register resources for later lookup.
	PrecacheSound(vm *VM, sample string)
	PrecacheModel(vm *VM, modelName string)

	// Text output helpers.
	BroadcastPrint(vm *VM, msg string)
	ClientPrint(vm *VM, entNum int, msg string)
	DebugPrint(vm *VM, msg string)
	CenterPrint(vm *VM, entNum int, msg string)

	// Networked effects and client messaging helpers.
	Sound(vm *VM, entNum, channel int, sample string, volume int, attenuation float32)
	StuffCmd(vm *VM, entNum int, cmd string)
	LightStyle(vm *VM, style int, value string)
	Particle(vm *VM, org, dir [3]float32, color, count int)
	LocalSound(vm *VM, entNum int, sample string)
	WriteByte(vm *VM, dest, value int)
	WriteChar(vm *VM, dest, value int)
	WriteShort(vm *VM, dest, value int)
	WriteLong(vm *VM, dest int, value int32)
	WriteCoord(vm *VM, dest int, value float32)
	WriteAngle(vm *VM, dest int, value float32)
	WriteString(vm *VM, dest int, value string)
	WriteEntity(vm *VM, dest, entNum int)

	// Spawn parameter helpers for level transitions.
	SetSpawnParms(vm *VM, entNum int)

	// Static signon helpers.
	MakeStatic(vm *VM, entNum int)
	AmbientSound(vm *VM, org [3]float32, sample string, volume int, attenuation float32)

	// MoveToGoal/ChangeYaw are AI helpers invoked by QuakeC.
	MoveToGoal(vm *VM, dist float32)
	ChangeYaw(vm *VM)
}

// RegisterServerHooks adapts a ServerHooks implementation to the
// legacy `ServerBuiltinHooks` struct used by existing builtins. This
// helper enables code that already calls `SetServerBuiltinHooks` to
// keep working while allowing new code to supply a typed interface.
func RegisterServerHooks(h ServerHooks) {
	if h == nil {
		SetServerBuiltinHooks(ServerBuiltinHooks{})
		return
	}

	SetServerBuiltinHooks(ServerBuiltinHooks{
		Traceline: func(vm *VM, start, end [3]float32, noMonsters bool, passEnt int) BuiltinTraceResult {
			return h.Traceline(vm, start, end, noMonsters, passEnt)
		},
		Spawn:  func(vm *VM) (int, error) { return h.Spawn(vm) },
		Remove: func(vm *VM, entNum int) error { return h.Remove(vm, entNum) },
		Find:   func(vm *VM, startEnt, fieldOfs int, match string) int { return h.Find(vm, startEnt, fieldOfs, match) },
		FindFloat: func(vm *VM, startEnt, fieldOfs int, match float32) int {
			return h.FindFloat(vm, startEnt, fieldOfs, match)
		},
		FindRadius:  func(vm *VM, org [3]float32, radius float32) int { return h.FindRadius(vm, org, radius) },
		CheckClient: func(vm *VM) int { return h.CheckClient(vm) },
		NextEnt:     func(vm *VM, entNum int) int { return h.NextEnt(vm, entNum) },
		CheckBottom: func(vm *VM, entNum int) bool { return h.CheckBottom(vm, entNum) },
		PointContents: func(vm *VM, point [3]float32) int {
			return h.PointContents(vm, point)
		},
		Aim: func(vm *VM, entNum int, missileSpeed float32) [3]float32 {
			return h.Aim(vm, entNum, missileSpeed)
		},
		WalkMove:       func(vm *VM, yaw, dist float32) bool { return h.WalkMove(vm, yaw, dist) },
		DropToFloor:    func(vm *VM) bool { return h.DropToFloor(vm) },
		SetOrigin:      func(vm *VM, entNum int, org [3]float32) { h.SetOrigin(vm, entNum, org) },
		SetSize:        func(vm *VM, entNum int, mins, maxs [3]float32) { h.SetSize(vm, entNum, mins, maxs) },
		SetModel:       func(vm *VM, entNum int, modelName string) { h.SetModel(vm, entNum, modelName) },
		PrecacheSound:  func(vm *VM, sample string) { h.PrecacheSound(vm, sample) },
		PrecacheModel:  func(vm *VM, modelName string) { h.PrecacheModel(vm, modelName) },
		BroadcastPrint: func(vm *VM, msg string) { h.BroadcastPrint(vm, msg) },
		ClientPrint:    func(vm *VM, entNum int, msg string) { h.ClientPrint(vm, entNum, msg) },
		DebugPrint:     func(vm *VM, msg string) { h.DebugPrint(vm, msg) },
		CenterPrint:    func(vm *VM, entNum int, msg string) { h.CenterPrint(vm, entNum, msg) },
		Sound: func(vm *VM, entNum, channel int, sample string, volume int, attenuation float32) {
			h.Sound(vm, entNum, channel, sample, volume, attenuation)
		},
		StuffCmd:   func(vm *VM, entNum int, cmd string) { h.StuffCmd(vm, entNum, cmd) },
		LightStyle: func(vm *VM, style int, value string) { h.LightStyle(vm, style, value) },
		Particle: func(vm *VM, org, dir [3]float32, color, count int) {
			h.Particle(vm, org, dir, color, count)
		},
		LocalSound:    func(vm *VM, entNum int, sample string) { h.LocalSound(vm, entNum, sample) },
		WriteByte:     func(vm *VM, dest, value int) { h.WriteByte(vm, dest, value) },
		WriteChar:     func(vm *VM, dest, value int) { h.WriteChar(vm, dest, value) },
		WriteShort:    func(vm *VM, dest, value int) { h.WriteShort(vm, dest, value) },
		WriteLong:     func(vm *VM, dest int, value int32) { h.WriteLong(vm, dest, value) },
		WriteCoord:    func(vm *VM, dest int, value float32) { h.WriteCoord(vm, dest, value) },
		WriteAngle:    func(vm *VM, dest int, value float32) { h.WriteAngle(vm, dest, value) },
		WriteString:   func(vm *VM, dest int, value string) { h.WriteString(vm, dest, value) },
		WriteEntity:   func(vm *VM, dest, entNum int) { h.WriteEntity(vm, dest, entNum) },
		SetSpawnParms: func(vm *VM, entNum int) { h.SetSpawnParms(vm, entNum) },
		MakeStatic:    func(vm *VM, entNum int) { h.MakeStatic(vm, entNum) },
		AmbientSound: func(vm *VM, org [3]float32, sample string, volume int, attenuation float32) {
			h.AmbientSound(vm, org, sample, volume, attenuation)
		},
		MoveToGoal: func(vm *VM, dist float32) { h.MoveToGoal(vm, dist) },
		ChangeYaw:  func(vm *VM) { h.ChangeYaw(vm) },
	})
}
