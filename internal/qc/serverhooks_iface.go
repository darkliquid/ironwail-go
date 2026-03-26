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
	WriteByteTo(vm *VM, dest, value int)
	WriteCharTo(vm *VM, dest, value int)
	WriteShortTo(vm *VM, dest, value int)
	WriteLongTo(vm *VM, dest int, value int32)
	WriteCoordTo(vm *VM, dest int, value float32)
	WriteAngleTo(vm *VM, dest int, value float32)
	WriteStringTo(vm *VM, dest int, value string)
	WriteEntityTo(vm *VM, dest, entNum int)

	// Spawn parameter helpers for level transitions.
	SetSpawnParms(vm *VM, entNum int)

	// Static signon helpers.
	MakeStatic(vm *VM, entNum int)
	AmbientSound(vm *VM, org [3]float32, sample string, volume int, attenuation float32)

	// MoveToGoal/ChangeYaw are AI helpers invoked by QuakeC.
	MoveToGoal(vm *VM, dist float32)
	ChangeYaw(vm *VM)

	// IssueChangeLevel requests a map transition command and returns true if accepted.
	IssueChangeLevel(vm *VM, level string) bool
}

type serverBuiltinHooksAdapter struct {
	hooks ServerBuiltinHooks
}

// AdaptServerBuiltinHooks wraps a legacy `ServerBuiltinHooks` table in the
// typed `ServerHooks` interface so registration sites can migrate without
// rewriting every callback at once.
func AdaptServerBuiltinHooks(h ServerBuiltinHooks) ServerHooks {
	return serverBuiltinHooksAdapter{hooks: h}
}

func (a serverBuiltinHooksAdapter) Traceline(vm *VM, start, end [3]float32, noMonsters bool, passEnt int) BuiltinTraceResult {
	if a.hooks.Traceline == nil {
		return BuiltinTraceResult{}
	}
	return a.hooks.Traceline(vm, start, end, noMonsters, passEnt)
}

func (a serverBuiltinHooksAdapter) Spawn(vm *VM) (int, error) {
	if a.hooks.Spawn == nil {
		return 0, nil
	}
	return a.hooks.Spawn(vm)
}

func (a serverBuiltinHooksAdapter) Remove(vm *VM, entNum int) error {
	if a.hooks.Remove == nil {
		return nil
	}
	return a.hooks.Remove(vm, entNum)
}

func (a serverBuiltinHooksAdapter) Find(vm *VM, startEnt, fieldOfs int, match string) int {
	if a.hooks.Find == nil {
		return 0
	}
	return a.hooks.Find(vm, startEnt, fieldOfs, match)
}

func (a serverBuiltinHooksAdapter) FindFloat(vm *VM, startEnt, fieldOfs int, match float32) int {
	if a.hooks.FindFloat == nil {
		return 0
	}
	return a.hooks.FindFloat(vm, startEnt, fieldOfs, match)
}

func (a serverBuiltinHooksAdapter) FindRadius(vm *VM, org [3]float32, radius float32) int {
	if a.hooks.FindRadius == nil {
		return 0
	}
	return a.hooks.FindRadius(vm, org, radius)
}

func (a serverBuiltinHooksAdapter) CheckClient(vm *VM) int {
	if a.hooks.CheckClient == nil {
		return 0
	}
	return a.hooks.CheckClient(vm)
}

func (a serverBuiltinHooksAdapter) NextEnt(vm *VM, entNum int) int {
	if a.hooks.NextEnt == nil {
		return 0
	}
	return a.hooks.NextEnt(vm, entNum)
}

func (a serverBuiltinHooksAdapter) CheckBottom(vm *VM, entNum int) bool {
	if a.hooks.CheckBottom == nil {
		return false
	}
	return a.hooks.CheckBottom(vm, entNum)
}

func (a serverBuiltinHooksAdapter) PointContents(vm *VM, point [3]float32) int {
	if a.hooks.PointContents == nil {
		return 0
	}
	return a.hooks.PointContents(vm, point)
}

func (a serverBuiltinHooksAdapter) Aim(vm *VM, entNum int, missileSpeed float32) [3]float32 {
	if a.hooks.Aim == nil {
		return [3]float32{}
	}
	return a.hooks.Aim(vm, entNum, missileSpeed)
}

func (a serverBuiltinHooksAdapter) WalkMove(vm *VM, yaw, dist float32) bool {
	if a.hooks.WalkMove == nil {
		return false
	}
	return a.hooks.WalkMove(vm, yaw, dist)
}

func (a serverBuiltinHooksAdapter) DropToFloor(vm *VM) bool {
	if a.hooks.DropToFloor == nil {
		return false
	}
	return a.hooks.DropToFloor(vm)
}

func (a serverBuiltinHooksAdapter) SetOrigin(vm *VM, entNum int, org [3]float32) {
	if a.hooks.SetOrigin != nil {
		a.hooks.SetOrigin(vm, entNum, org)
	}
}

func (a serverBuiltinHooksAdapter) SetSize(vm *VM, entNum int, mins, maxs [3]float32) {
	if a.hooks.SetSize != nil {
		a.hooks.SetSize(vm, entNum, mins, maxs)
	}
}

func (a serverBuiltinHooksAdapter) SetModel(vm *VM, entNum int, modelName string) {
	if a.hooks.SetModel != nil {
		a.hooks.SetModel(vm, entNum, modelName)
	}
}

func (a serverBuiltinHooksAdapter) PrecacheSound(vm *VM, sample string) {
	if a.hooks.PrecacheSound != nil {
		a.hooks.PrecacheSound(vm, sample)
	}
}

func (a serverBuiltinHooksAdapter) PrecacheModel(vm *VM, modelName string) {
	if a.hooks.PrecacheModel != nil {
		a.hooks.PrecacheModel(vm, modelName)
	}
}

func (a serverBuiltinHooksAdapter) BroadcastPrint(vm *VM, msg string) {
	if a.hooks.BroadcastPrint != nil {
		a.hooks.BroadcastPrint(vm, msg)
	}
}

func (a serverBuiltinHooksAdapter) ClientPrint(vm *VM, entNum int, msg string) {
	if a.hooks.ClientPrint != nil {
		a.hooks.ClientPrint(vm, entNum, msg)
	}
}

func (a serverBuiltinHooksAdapter) DebugPrint(vm *VM, msg string) {
	if a.hooks.DebugPrint != nil {
		a.hooks.DebugPrint(vm, msg)
	}
}

func (a serverBuiltinHooksAdapter) CenterPrint(vm *VM, entNum int, msg string) {
	if a.hooks.CenterPrint != nil {
		a.hooks.CenterPrint(vm, entNum, msg)
	}
}

func (a serverBuiltinHooksAdapter) Sound(vm *VM, entNum, channel int, sample string, volume int, attenuation float32) {
	if a.hooks.Sound != nil {
		a.hooks.Sound(vm, entNum, channel, sample, volume, attenuation)
	}
}

func (a serverBuiltinHooksAdapter) StuffCmd(vm *VM, entNum int, cmd string) {
	if a.hooks.StuffCmd != nil {
		a.hooks.StuffCmd(vm, entNum, cmd)
	}
}

func (a serverBuiltinHooksAdapter) LightStyle(vm *VM, style int, value string) {
	if a.hooks.LightStyle != nil {
		a.hooks.LightStyle(vm, style, value)
	}
}

func (a serverBuiltinHooksAdapter) Particle(vm *VM, org, dir [3]float32, color, count int) {
	if a.hooks.Particle != nil {
		a.hooks.Particle(vm, org, dir, color, count)
	}
}

func (a serverBuiltinHooksAdapter) LocalSound(vm *VM, entNum int, sample string) {
	if a.hooks.LocalSound != nil {
		a.hooks.LocalSound(vm, entNum, sample)
	}
}

func (a serverBuiltinHooksAdapter) WriteByteTo(vm *VM, dest, value int) {
	if a.hooks.WriteByte != nil {
		a.hooks.WriteByte(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteCharTo(vm *VM, dest, value int) {
	if a.hooks.WriteChar != nil {
		a.hooks.WriteChar(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteShortTo(vm *VM, dest, value int) {
	if a.hooks.WriteShort != nil {
		a.hooks.WriteShort(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteLongTo(vm *VM, dest int, value int32) {
	if a.hooks.WriteLong != nil {
		a.hooks.WriteLong(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteCoordTo(vm *VM, dest int, value float32) {
	if a.hooks.WriteCoord != nil {
		a.hooks.WriteCoord(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteAngleTo(vm *VM, dest int, value float32) {
	if a.hooks.WriteAngle != nil {
		a.hooks.WriteAngle(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteStringTo(vm *VM, dest int, value string) {
	if a.hooks.WriteString != nil {
		a.hooks.WriteString(vm, dest, value)
	}
}

func (a serverBuiltinHooksAdapter) WriteEntityTo(vm *VM, dest, entNum int) {
	if a.hooks.WriteEntity != nil {
		a.hooks.WriteEntity(vm, dest, entNum)
	}
}

func (a serverBuiltinHooksAdapter) SetSpawnParms(vm *VM, entNum int) {
	if a.hooks.SetSpawnParms != nil {
		a.hooks.SetSpawnParms(vm, entNum)
	}
}

func (a serverBuiltinHooksAdapter) MakeStatic(vm *VM, entNum int) {
	if a.hooks.MakeStatic != nil {
		a.hooks.MakeStatic(vm, entNum)
	}
}

func (a serverBuiltinHooksAdapter) AmbientSound(vm *VM, org [3]float32, sample string, volume int, attenuation float32) {
	if a.hooks.AmbientSound != nil {
		a.hooks.AmbientSound(vm, org, sample, volume, attenuation)
	}
}

func (a serverBuiltinHooksAdapter) MoveToGoal(vm *VM, dist float32) {
	if a.hooks.MoveToGoal != nil {
		a.hooks.MoveToGoal(vm, dist)
	}
}

func (a serverBuiltinHooksAdapter) ChangeYaw(vm *VM) {
	if a.hooks.ChangeYaw != nil {
		a.hooks.ChangeYaw(vm)
	}
}

func (a serverBuiltinHooksAdapter) IssueChangeLevel(vm *VM, level string) bool {
	if a.hooks.IssueChangeLevel == nil {
		return false
	}
	return a.hooks.IssueChangeLevel(vm, level)
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
		WriteByte:     func(vm *VM, dest, value int) { h.WriteByteTo(vm, dest, value) },
		WriteChar:     func(vm *VM, dest, value int) { h.WriteCharTo(vm, dest, value) },
		WriteShort:    func(vm *VM, dest, value int) { h.WriteShortTo(vm, dest, value) },
		WriteLong:     func(vm *VM, dest int, value int32) { h.WriteLongTo(vm, dest, value) },
		WriteCoord:    func(vm *VM, dest int, value float32) { h.WriteCoordTo(vm, dest, value) },
		WriteAngle:    func(vm *VM, dest int, value float32) { h.WriteAngleTo(vm, dest, value) },
		WriteString:   func(vm *VM, dest int, value string) { h.WriteStringTo(vm, dest, value) },
		WriteEntity:   func(vm *VM, dest, entNum int) { h.WriteEntityTo(vm, dest, entNum) },
		SetSpawnParms: func(vm *VM, entNum int) { h.SetSpawnParms(vm, entNum) },
		MakeStatic:    func(vm *VM, entNum int) { h.MakeStatic(vm, entNum) },
		AmbientSound: func(vm *VM, org [3]float32, sample string, volume int, attenuation float32) {
			h.AmbientSound(vm, org, sample, volume, attenuation)
		},
		MoveToGoal:       func(vm *VM, dist float32) { h.MoveToGoal(vm, dist) },
		ChangeYaw:        func(vm *VM) { h.ChangeYaw(vm) },
		IssueChangeLevel: func(vm *VM, level string) bool { return h.IssueChangeLevel(vm, level) },
	})
}
