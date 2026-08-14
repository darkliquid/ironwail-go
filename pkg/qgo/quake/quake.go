// Package quake provides the core types and primitives for the QuakeC Virtual Machine.
//
// This package is used as a foundation for writing QuakeC logic in Go.
// The QGo compiler recognizes these types and maps them directly to QCVM
// primitive types (ev_float, ev_vector, ev_string, etc.).
package quake

import "math"

// Vec3 represents a 3D vector, mapped to the QCVM 'ev_vector' type.
// In the QCVM, it is laid out as three consecutive float32 slots (X, Y, Z).
type Vec3 [3]float32

// Entity represents a handle to an entity in the game world, mapped to
// the QCVM 'ev_entity' type. Internally it is an index into the engine's
// edict (entity) table. Entity 0 is always the 'world' entity.
//
//qgo:entity
type Entity struct {
	// --- system fields (must match engine entvars_t layout) ---
	ModelIndex float32 `qgo:"modelindex"`
	AbsMin     Vec3    `qgo:"absmin"`
	AbsMax     Vec3    `qgo:"absmax"`

	LTime    float32 `qgo:"ltime"`
	MoveType float32 `qgo:"movetype"`
	Solid    float32 `qgo:"solid"`

	Origin    Vec3 `qgo:"origin"`
	OldOrigin Vec3 `qgo:"oldorigin"`
	Velocity  Vec3 `qgo:"velocity"`
	Angles    Vec3 `qgo:"angles"`
	AVelocity Vec3 `qgo:"avelocity"`

	PunchAngle Vec3 `qgo:"punchangle"`

	ClassName string  `qgo:"classname"`
	Model     string  `qgo:"model"`
	Frame     float32 `qgo:"frame"`
	Skin      float32 `qgo:"skin"`
	Effects   float32 `qgo:"effects"`

	Mins Vec3 `qgo:"mins"`
	Maxs Vec3 `qgo:"maxs"`
	Size Vec3 `qgo:"size"`

	Touch   Func `qgo:"touch"`
	Use     Func `qgo:"use"`
	Think   Func `qgo:"think"`
	Blocked Func `qgo:"blocked"`

	NextThink    float32 `qgo:"nextthink"`
	GroundEntity *Entity `qgo:"groundentity"`

	// stats
	Health      float32 `qgo:"health"`
	Frags       float32 `qgo:"frags"`
	Weapon      float32 `qgo:"weapon"`
	WeaponModel string  `qgo:"weaponmodel"`
	WeaponFrame float32 `qgo:"weaponframe"`
	CurrentAmmo float32 `qgo:"currentammo"`
	AmmoShells  float32 `qgo:"ammo_shells"`
	AmmoNails   float32 `qgo:"ammo_nails"`
	AmmoRockets float32 `qgo:"ammo_rockets"`
	AmmoCells   float32 `qgo:"ammo_cells"`

	Items float32 `qgo:"items"`

	TakeDamage float32 `qgo:"takedamage"`
	Chain      *Entity `qgo:"chain"`
	DeadFlag   float32 `qgo:"deadflag"`

	ViewOfs Vec3 `qgo:"view_ofs"`

	Button0 float32 `qgo:"button0"` // fire
	Button1 float32 `qgo:"button1"` // use
	Button2 float32 `qgo:"button2"` // jump

	Impulse float32 `qgo:"impulse"` // weapon changes

	FixAngle   float32 `qgo:"fixangle"`
	VAngle     Vec3    `qgo:"v_angle"`
	IdealPitch float32 `qgo:"idealpitch"`

	NetName string `qgo:"netname"`

	Enemy *Entity `qgo:"enemy"`

	Flags float32 `qgo:"flags"`

	ColorMap float32 `qgo:"colormap"`
	Team     float32 `qgo:"team"`

	MaxHealth float32 `qgo:"max_health"`

	TeleportTime float32 `qgo:"teleport_time"`

	ArmorType  float32 `qgo:"armortype"`
	ArmorValue float32 `qgo:"armorvalue"`

	WaterLevel float32 `qgo:"waterlevel"`
	WaterType  float32 `qgo:"watertype"`

	IdealYaw float32 `qgo:"ideal_yaw"`
	YawSpeed float32 `qgo:"yaw_speed"`

	AIMent *Entity `qgo:"aiment"`

	GoalEntity *Entity `qgo:"goalentity"`

	SpawnFlags float32 `qgo:"spawnflags"`

	Target     string `qgo:"target"`
	TargetName string `qgo:"targetname"`

	DmgTake      float32 `qgo:"dmg_take"`
	DmgSave      float32 `qgo:"dmg_save"`
	DmgInflictor *Entity `qgo:"dmg_inflictor"`

	InPain float32 `qgo:"inpain"`

	Owner   *Entity `qgo:"owner"`
	MoveDir Vec3    `qgo:"movedir"`

	Message string `qgo:"message"`

	Sounds float32 `qgo:"sounds"`

	Noise  string `qgo:"noise"`
	Noise1 string `qgo:"noise1"`
	Noise2 string `qgo:"noise2"`
	Noise3 string `qgo:"noise3"`

	// --- end of system fields ---

	// world fields
	Wad       string  `qgo:"wad"`
	Map       string  `qgo:"map"`
	WorldType float32 `qgo:"worldtype"`

	KillTarget string `qgo:"killtarget"`

	// quakeed fields
	LightLev float32 `qgo:"light_lev"`
	Style    float32 `qgo:"style"`

	// monster AI
	ThStand   Func     `qgo:"th_stand"`
	ThWalk    Func     `qgo:"th_walk"`
	ThRun     Func     `qgo:"th_run"`
	ThMissile Func     `qgo:"th_missile"`
	ThMelee   Func     `qgo:"th_melee"`
	ThPain    PainFunc `qgo:"th_pain"`
	ThDie     Func     `qgo:"th_die"`

	OldEnemy *Entity `qgo:"oldenemy"`

	Speed float32 `qgo:"speed"`

	Lefty float32 `qgo:"lefty"`

	SearchTime float32 `qgo:"search_time"`

	AttackState float32 `qgo:"attack_state"`

	AllowPathFind float32 `qgo:"allowPathFind"`

	CombatStyle float32 `qgo:"combat_style"`

	// player only fields
	WalkFrame float32 `qgo:"walkframe"`

	AttackFinished float32 `qgo:"attack_finished"`
	PainFinished   float32 `qgo:"pain_finished"`

	InvincibleFinished  float32 `qgo:"invincible_finished"`
	InvisibleFinished   float32 `qgo:"invisible_finished"`
	SuperDamageFinished float32 `qgo:"super_damage_finished"`
	RadSuitFinished     float32 `qgo:"radsuit_finished"`

	InvincibleTime  float32 `qgo:"invincible_time"`
	InvincibleSound float32 `qgo:"invincible_sound"`
	InvisibleTime   float32 `qgo:"invisible_time"`
	InvisibleSound  float32 `qgo:"invisible_sound"`
	SuperTime       float32 `qgo:"super_time"`
	SuperSound      float32 `qgo:"super_sound"`
	RadTime         float32 `qgo:"rad_time"`
	FlySound        float32 `qgo:"fly_sound"`

	HealthRotNextCheck float32 `qgo:"healthrot_nextcheck"`

	AxHitMe float32 `qgo:"axhitme"`

	ShowHostile float32 `qgo:"show_hostile"`
	JumpFlag    float32 `qgo:"jump_flag"`
	SwimFlag    float32 `qgo:"swim_flag"`
	AirFinished float32 `qgo:"air_finished"`
	BubbleCount float32 `qgo:"bubble_count"`
	DeathType   string  `qgo:"deathtype"`
	FiredWeapon float32 `qgo:"fired_weapon"`
	TookDamage  float32 `qgo:"took_damage"`

	// object stuff
	Mdl    string `qgo:"mdl"`
	Mangle Vec3   `qgo:"mangle"`

	TLength float32 `qgo:"t_length"`
	TWidth  float32 `qgo:"t_width"`

	// doors, etc
	Dest         Vec3    `qgo:"dest"`
	Dest1        Vec3    `qgo:"dest1"`
	Dest2        Vec3    `qgo:"dest2"`
	Wait         float32 `qgo:"wait"`
	Delay        float32 `qgo:"delay"`
	TriggerField *Entity `qgo:"trigger_field"`
	Noise4       string  `qgo:"noise4"`

	// monsters
	PauseTime  float32 `qgo:"pausetime"`
	MoveTarget *Entity `qgo:"movetarget"`

	// doors
	AFlag float32 `qgo:"aflag"`
	Dmg   float32 `qgo:"dmg"`

	// misc
	Cnt float32 `qgo:"cnt"`

	// subs
	Think1     Func `qgo:"think1"`
	FinalDest  Vec3 `qgo:"finaldest"`
	FinalAngle Vec3 `qgo:"finalangle"`

	// triggers
	Count float32 `qgo:"count"`

	// plats / doors / buttons
	Lip    float32 `qgo:"lip"`
	State  float32 `qgo:"state"`
	Pos1   Vec3    `qgo:"pos1"`
	Pos2   Vec3    `qgo:"pos2"`
	Height float32 `qgo:"height"`

	// sounds
	WaitMin  float32 `qgo:"waitmin"`
	WaitMax  float32 `qgo:"waitmax"`
	Distance float32 `qgo:"distance"`
	Volume   float32 `qgo:"volume"`

	KillString string `qgo:"killstring"`

	SpawnDeferred float32 `qgo:"spawn_deferred"`

	// health items
	HealAmount float32 `qgo:"healamount"`
	HealType   float32 `qgo:"healtype"`

	// water damage
	DmgTime float32 `qgo:"dmgtime"`
}

// EntityFlags represents bitflags stored in Entity.Flags.
//
//go:generate stringer -type=EntityFlags
type EntityFlags uint32

const (
	FlagFly EntityFlags = 1 << iota
	FlagSwim
	_
	FlagClient
	FlagInWater
	FlagMonster
	FlagGodMode
	FlagNoTarget
	FlagItem
	FlagOnGround
	FlagPartialGround
	FlagWaterJump
	FlagJumpReleased
	FlagIsBot
	FlagNoPlayers
	FlagNoMonsters
	FlagNoBots
	FlagObjective
)

// EntityFlagsFromFloat converts QC-style float storage into typed flag bits.
func EntityFlagsFromFloat(v float32) EntityFlags {
	return EntityFlags(uint32(v))
}

// Float32 converts typed flags into QC-style float storage.
func (f EntityFlags) Float32() float32 {
	return float32(uint32(f))
}

// Has reports whether all bits in mask are set.
func (f EntityFlags) Has(mask EntityFlags) bool {
	return f&mask == mask
}

// With returns flags with mask bits set.
func (f EntityFlags) With(mask EntityFlags) EntityFlags {
	return f | mask
}

// Without returns flags with mask bits cleared.
func (f EntityFlags) Without(mask EntityFlags) EntityFlags {
	return f &^ mask
}

// FlagsValue returns Entity.Flags as strongly typed bits.
func (e *Entity) FlagsValue() EntityFlags {
	if e == nil {
		return 0
	}
	return EntityFlagsFromFloat(e.Flags)
}

// SetFlagsValue writes strongly typed bits into Entity.Flags.
func (e *Entity) SetFlagsValue(flags EntityFlags) {
	if e == nil {
		return
	}
	e.Flags = flags.Float32()
}

// HasFlags reports whether all bits in mask are set in Entity.Flags.
func (e *Entity) HasFlags(mask EntityFlags) bool {
	if e == nil {
		return false
	}
	return e.FlagsValue().Has(mask)
}

// AddFlags sets bits in Entity.Flags.
func (e *Entity) AddFlags(mask EntityFlags) {
	if e == nil {
		return
	}
	e.SetFlagsValue(e.FlagsValue().With(mask))
}

// ClearFlags clears bits in Entity.Flags.
func (e *Entity) ClearFlags(mask EntityFlags) {
	if e == nil {
		return
	}
	e.SetFlagsValue(e.FlagsValue().Without(mask))
}

// SpawnFlagsValue returns Entity.SpawnFlags as strongly typed bits.
func (e *Entity) SpawnFlagsValue() EntityFlags {
	if e == nil {
		return 0
	}
	return EntityFlagsFromFloat(e.SpawnFlags)
}

// SetSpawnFlagsValue writes strongly typed bits into Entity.SpawnFlags.
func (e *Entity) SetSpawnFlagsValue(flags EntityFlags) {
	if e == nil {
		return
	}
	e.SpawnFlags = flags.Float32()
}

// Func represents a function pointer or index in the QCVM function table,
// mapped to the 'ev_function' type. It is used for callback fields like
// .think, .touch, and .use.
type Func func()

// PainFunc represents a function called when an entity takes pain.
type PainFunc func(attacker *Entity, damage float32)

// FieldOffset represents an offset into the entity field data, mapped to
// the 'ev_field' type. It is used to dynamically access fields on entities.
type FieldOffset any

// FieldFloat reads a float32 entity field using a runtime field offset.
// qgo lowers this helper as a compiler intrinsic (OP_LOAD_F).
func FieldFloat(entity *Entity, field FieldOffset) float32 {
	if entity == nil {
		return 0
	}
	return entity.FieldFloat(field)
}

// FieldFloat reads a float32 entity field using a runtime field offset.
// qgo lowers this helper as a compiler intrinsic (OP_LOAD_F).
func (e *Entity) FieldFloat(field FieldOffset) float32 {
	if e == nil {
		return 0
	}
	_ = field
	return 0
}

// SetFieldFloat writes a float32 entity field using a runtime field offset.
// qgo lowers this helper as a compiler intrinsic (OP_ADDRESS + OP_STOREP_F).
func SetFieldFloat(entity *Entity, field FieldOffset, value float32) {
	if entity == nil {
		return
	}
	entity.SetFieldFloat(field, value)
}

// SetFieldFloat writes a float32 entity field using a runtime field offset.
// qgo lowers this helper as a compiler intrinsic (OP_ADDRESS + OP_STOREP_F).
func (e *Entity) SetFieldFloat(field FieldOffset, value float32) {
	if e == nil {
		return
	}
	_, _ = field, value
}

// Heal applies Quake-style healing rules to an entity.
// Returns 1 when healing was applied, or 0 when no change occurred.
func (e *Entity) Heal(healAmount, ignore float32) float32 {
	if e == nil || e.Health <= 0 {
		return 0
	}
	if ignore == 0 && e.Health >= e.MaxHealth {
		return 0
	}

	healAmount = float32(math.Ceil(float64(healAmount)))
	e.Health += healAmount

	if ignore == 0 && e.Health >= e.MaxHealth {
		e.Health = e.MaxHealth
	}
	if e.Health > 250 {
		e.Health = 250
	}

	return 1
}

// Void is a marker type used for functions that do not return a value.
// In QGo, a function returning Void maps to a QCVM 'ev_void' return type.
type Void struct{}

// MakeVec3 constructs a Vec3 from three float32 values.
// This is a compiler-known helper that allows creating vector literals.
func MakeVec3(x, y, z float32) Vec3 {
	return Vec3{x, y, z}
}

// Sprintf performs string formatting and interpolation at compile time.
// The QGo compiler expands this into a sequence of 'ftos' and 'strcat'
// builtin calls. Only a subset of standard Go Sprintf features are supported.
func Sprintf(format string, args ...any) string {
	return ""
}

// Mul returns the vector scaled by s.
func (v Vec3) Mul(s float32) Vec3 {
	return Vec3{v[0] * s, v[1] * s, v[2] * s}
}

// Div returns the vector divided by s.
func (v Vec3) Div(s float32) Vec3 {
	return Vec3{v[0] / s, v[1] / s, v[2] / s}
}

// Add returns the sum of v and o.
func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{v[0] + o[0], v[1] + o[1], v[2] + o[2]}
}

// Sub returns the difference of v and o.
func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{v[0] - o[0], v[1] - o[1], v[2] - o[2]}
}

// Dot returns the dot product of v and o.
func (v Vec3) Dot(o Vec3) float32 {
	return v[0]*o[0] + v[1]*o[1] + v[2]*o[2]
}

// Neg returns the negated vector.
func (v Vec3) Neg() Vec3 {
	return Vec3{-v[0], -v[1], -v[2]}
}

// Cross returns the cross product of v and o.
func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		v[1]*o[2] - v[2]*o[1],
		v[2]*o[0] - v[0]*o[2],
		v[0]*o[1] - v[1]*o[0],
	}
}

// Lerp linearly interpolates between v and o by t.
func (v Vec3) Lerp(o Vec3, t float32) Vec3 {
	return v.Add(o.Sub(v).Mul(t))
}

// Scale returns the vector scaled by s. Alias for Mul.
func (v Vec3) Scale(s float32) Vec3 {
	return v.Mul(s)
}

// Negate returns the negated vector. Alias for Neg.
func (v Vec3) Negate() Vec3 {
	return v.Neg()
}

// Len returns the Euclidean length of the vector.
func (v Vec3) Len() float32 {
	return float32(math.Sqrt(float64(v.Dot(v))))
}

// Length returns the Euclidean length of the vector. Alias for Len.
func (v Vec3) Length() float32 {
	return v.Len()
}

// LenSq returns the squared length of the vector (v[0]² + v[1]² + v[2]²).
func (v Vec3) LenSq() float32 {
	return v.Dot(v)
}

// LengthSq returns the squared length of the vector. Alias for LenSq.
func (v Vec3) LengthSq() float32 {
	return v.LenSq()
}

// Distance returns the Euclidean distance between v and other.
func (v Vec3) Distance(other Vec3) float32 {
	return other.Sub(v).Len()
}

// Dist returns the Euclidean distance between v and other. Alias for Distance.
func (v Vec3) Dist(other Vec3) float32 {
	return v.Distance(other)
}

// DistanceSq returns the squared Euclidean distance between v and other.
func (v Vec3) DistanceSq(other Vec3) float32 {
	return other.Sub(v).LenSq()
}

// Normalize returns a unit-length vector pointing in the same direction.
func (v Vec3) Normalize() Vec3 {
	l := v.Len()
	if l > 0 {
		return v.Mul(1.0 / l)
	}
	return v
}

// MA performs fused Multiply-Add: v + b * scale.
func (v Vec3) MA(scale float32, b Vec3) Vec3 {
	return v.Add(b.Mul(scale))
}

// MultiplyAdd performs fused Multiply-Add: v + b * scale. Alias for MA.
func (v Vec3) MultiplyAdd(scale float32, b Vec3) Vec3 {
	return v.MA(scale, b)
}

// X returns the first component of the vector.
func (v Vec3) X() float32 { return v[0] }

// Y returns the second component of the vector.
func (v Vec3) Y() float32 { return v[1] }

// Z returns the third component of the vector.
func (v Vec3) Z() float32 { return v[2] }

// Set mutates the vector components in-place.
func (v *Vec3) Set(x, y, z float32) {
	v[0] = x
	v[1] = y
	v[2] = z
}

// Equals returns true if all vector components match exactly.
func (v Vec3) Equals(other Vec3) bool {
	return v[0] == other[0] && v[1] == other[1] && v[2] == other[2]
}

// ApproxEqual returns true if all components are within epsilon.
func (v Vec3) ApproxEqual(other Vec3, eps float32) bool {
	return float32(math.Abs(float64(v[0]-other[0]))) <= eps &&
		float32(math.Abs(float64(v[1]-other[1]))) <= eps &&
		float32(math.Abs(float64(v[2]-other[2]))) <= eps
}

// Box3 represents an axis-aligned bounding box defined by Min and Max bounds.
type Box3 struct {
	Min Vec3
	Max Vec3
}

// NewBox3 creates a Box3 from minimum and maximum bounds.
func NewBox3(min, max Vec3) Box3 {
	return Box3{Min: min, Max: max}
}

// Center returns the center of the bounding box.
func (b Box3) Center() Vec3 {
	return Vec3{
		(b.Min[0] + b.Max[0]) * 0.5,
		(b.Min[1] + b.Max[1]) * 0.5,
		(b.Min[2] + b.Max[2]) * 0.5,
	}
}

// Size returns the width, depth, height dimensions of the box.
func (b Box3) Size() Vec3 {
	return Vec3{
		b.Max[0] - b.Min[0],
		b.Max[1] - b.Min[1],
		b.Max[2] - b.Min[2],
	}
}

// ContainsPoint returns true if point p is inside or on the box bounds.
func (b Box3) ContainsPoint(p Vec3) bool {
	return p[0] >= b.Min[0] && p[0] <= b.Max[0] &&
		p[1] >= b.Min[1] && p[1] <= b.Max[1] &&
		p[2] >= b.Min[2] && p[2] <= b.Max[2]
}

// Intersects returns true if b and other overlap.
func (b Box3) Intersects(other Box3) bool {
	if b.Max[0] < other.Min[0] || b.Min[0] > other.Max[0] {
		return false
	}
	if b.Max[1] < other.Min[1] || b.Min[1] > other.Max[1] {
		return false
	}
	if b.Max[2] < other.Min[2] || b.Min[2] > other.Max[2] {
		return false
	}
	return true
}

// Expand returns a box expanded uniformly by delta in all directions.
func (b Box3) Expand(delta float32) Box3 {
	return Box3{
		Min: Vec3{b.Min[0] - delta, b.Min[1] - delta, b.Min[2] - delta},
		Max: Vec3{b.Max[0] + delta, b.Max[1] + delta, b.Max[2] + delta},
	}
}

// Plane represents an infinite 2D plane in 3D space: Normal · P - Dist = 0.
type Plane struct {
	Normal Vec3
	Dist   float32
}

// NewPlane creates a Plane with a unit normal and distance.
func NewPlane(normal Vec3, dist float32) Plane {
	return Plane{
		Normal: normal.Normalize(),
		Dist:   dist,
	}
}

// DistanceToPoint returns the signed distance from the plane to point p.
func (pl Plane) DistanceToPoint(p Vec3) float32 {
	return pl.Normal.Dot(p) - pl.Dist
}

// PointOnSide classifies point p relative to the plane (+1 front, -1 back, 0 on plane).
func (pl Plane) PointOnSide(p Vec3, epsilon float32) int {
	d := pl.DistanceToPoint(p)
	if d > epsilon {
		return 1
	}
	if d < -epsilon {
		return -1
	}
	return 0
}

// Angles represents Euler angles (pitch, yaw, roll) in degrees.
type Angles struct {
	Pitch float32
	Yaw   float32
	Roll  float32
}

// ToVec3 converts Angles to a Vec3 (X=Pitch, Y=Yaw, Z=Roll).
func (a Angles) ToVec3() Vec3 {
	return Vec3{a.Pitch, a.Yaw, a.Roll}
}

// OpAddVV emulates QC vector addition: a + b.
func OpAddVV(a, b Vec3) Vec3 {
	return a.Add(b)
}

// OpSubVV emulates QC vector subtraction: a - b.
func OpSubVV(a, b Vec3) Vec3 {
	return a.Sub(b)
}

// OpMulVF emulates QC vector-scalar multiply: a * s.
func OpMulVF(a Vec3, s float32) Vec3 {
	return a.Mul(s)
}

// OpMulFV emulates QC scalar-vector multiply: s * a.
func OpMulFV(s float32, a Vec3) Vec3 {
	return a.Mul(s)
}

// OpMulVV emulates QC vector multiply (dot product): a * b.
func OpMulVV(a, b Vec3) float32 {
	return a.Dot(b)
}

// OpDivVF emulates QC vector-scalar divide: a / s.
func OpDivVF(a Vec3, s float32) Vec3 {
	return a.Div(s)
}

// OpNegV emulates QC unary vector negation: -a.
func OpNegV(a Vec3) Vec3 {
	return a.Neg()
}
