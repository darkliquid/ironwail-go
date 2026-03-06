// Package server implements the Quake server physics and game logic.
//
// The server handles:
//   - Entity physics simulation (movement, collision)
//   - QuakeC think/touch/blocked callbacks
//   - Client state management
//   - World state (map, models, sounds)
//
// Physics is driven by SV_Physics which iterates all entities each frame
// and dispatches to specialized physics functions based on movetype.
package server

import "math"

// MoveType defines how an entity moves through the world.
type MoveType int

const (
	MoveTypeNone        MoveType = iota // Never moves
	MoveTypeAngleNoClip                 // Moves with angles, no clipping
	MoveTypeAngleClip                   // Moves with angles, with clipping
	MoveTypeWalk                        // Player walking with gravity
	MoveTypeStep                        // Monster stepping with gravity
	MoveTypeFly                         // Flying (no gravity)
	MoveTypeToss                        // Tossed with gravity
	MoveTypePush                        // Pusher (doors, plats)
	MoveTypeNoClip                      // No clipping
	MoveTypeFlyMissile                  // Missile flying
	MoveTypeBounce                      // Bouncing
	MoveTypeGib                         // Gib parts (2021 rerelease)
)

// SolidType defines how an entity collides with others.
type SolidType int

const (
	SolidNot      SolidType = iota // No interaction
	SolidTrigger                   // Touch on edge, not blocking
	SolidBBox                      // Touch on edge, blocks
	SolidSlideBox                  // Touch on edge, no onground
	SolidBSP                       // BSP clip, blocks
)

// DeadFlag defines the death state of an entity.
type DeadFlag int

const (
	DeadNo DeadFlag = iota
	DeadDying
	DeadDead
	DeadRespawnable
)

// TakeDamage defines how an entity receives damage.
type TakeDamage int

const (
	DamageNo TakeDamage = iota
	DamageYes
	DamageAim
)

// EntityFlags define entity behavior flags.
const (
	FlagFly = 1 << iota
	FlagSwim
	FlagConveyor
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
)

// EntityEffects define visual effects for entities.
const (
	EffectBrightField = 1 << iota
	EffectMuzzleFlash
	EffectBrightLight
	EffectDimLight
	EffectQuadLight // 2021 rerelease
	EffectPentaLight
	EffectCandleLight
)

// ServerState defines the current state of the server.
type ServerState int

const (
	ServerStateLoading ServerState = iota
	ServerStateActive
)

// Physics constants.
const (
	MoveEpsilon = 0.01
	StopEpsilon = 0.1
)

// EntityState represents the baseline state sent to clients.
type EntityState struct {
	Origin     [3]float32
	Angles     [3]float32
	ModelIndex int
	Frame      int
	Colormap   int
	Skin       int
	Effects    int
	Alpha      uint8
	Scale      uint8
}

// StaticSound represents a persistent ambient sound in the world signon state.
type StaticSound struct {
	Origin      [3]float32
	SoundIndex  int
	Volume      int
	Attenuation float32
}

// Edict represents a game entity.
type Edict struct {
	Free bool

	// Area linkage for spatial partitioning
	AreaPrev *Edict
	AreaNext *Edict

	// Leaf visibility
	NumLeafs int
	LeafNums [32]int

	// Client state
	Baseline EntityState
	Alpha    uint8
	Scale    uint8

	// Physics state
	ForceWater     bool
	SendForceWater bool
	SendInterval   bool
	OldFrame       float32
	OldThinkTime   float32

	// Timing
	FreeTime float32

	// Entity variables (from QuakeC)
	Vars *EntVars
}

// EntVars contains the QuakeC-exported entity fields.
type EntVars struct {
	ModelIndex   float32
	AbsMin       [3]float32
	AbsMax       [3]float32
	LTime        float32
	MoveType     float32
	Solid        float32
	Origin       [3]float32
	OldOrigin    [3]float32
	Velocity     [3]float32
	Angles       [3]float32
	AVelocity    [3]float32
	PunchAngle   [3]float32
	ClassName    int32
	Model        int32
	Frame        float32
	Skin         float32
	Effects      float32
	Mins         [3]float32
	Maxs         [3]float32
	Size         [3]float32
	Touch        int32
	Use          int32
	Think        int32
	Blocked      int32
	NextThink    float32
	GroundEntity int32
	Health       float32
	Frags        float32
	Weapon       float32
	WeaponModel  int32
	WeaponFrame  float32
	CurrentAmmo  float32
	AmmoShells   float32
	AmmoNails    float32
	AmmoRockets  float32
	AmmoCells    float32
	Items        float32
	TakeDamage   float32
	Chain        int32
	DeadFlag     float32
	ViewOfs      [3]float32
	Button0      float32
	Button1      float32
	Button2      float32
	Impulse      float32
	FixAngle     float32
	VAngle       [3]float32
	IdealPitch   float32
	NetName      int32
	Enemy        int32
	Flags        float32
	Colormap     float32
	Team         float32
	MaxHealth    float32
	TeleportTime float32
	ArmorType    float32
	ArmorValue   float32
	WaterLevel   float32
	WaterType    float32
	IdealYaw     float32
	YawSpeed     float32
	AimEnt       int32
	GoalEntity   int32
	SpawnFlags   float32
	Target       int32
	TargetName   int32
	DmgTake      float32
	DmgSave      float32
	DmgInflictor int32
	Owner        int32
	MoveDir      [3]float32
	Message      int32
	Sounds       float32
	Noise        int32
	Noise1       int32
	Noise2       int32
	Noise3       int32
}

// UserCmd represents client input commands sent to the server.
// Contains movement, angles, and impulse values for a single frame.
type UserCmd struct {
	ViewAngles  [3]float32 // Client view angles (pitch, yaw, roll)
	ForwardMove float32    // Forward/backward movement (-back, +forward)
	SideMove    float32    // Strafe movement (-left, +right)
	UpMove      float32    // Vertical movement (jump/swim)
	Buttons     uint8      // Button state (attack, jump, etc.)
	Impulse     uint8      // Weapon/impulse command
}

// TraceResult contains the result of a collision trace.
type TraceResult struct {
	AllSolid    bool       // Entire trace is in solid
	StartSolid  bool       // Trace started in solid
	Fraction    float32    // Fraction of path completed (0-1)
	EndPos      [3]float32 // Final position of trace
	PlaneNormal [3]float32 // Normal of impact plane
	Entity      *Edict     // Entity that was hit (if any)
}

// ClientState tracks the current state of a connected client.
type ClientState int

const (
	ClientStateDisconnected ClientState = iota
	ClientStateConnected
	ClientStateSpawned
)

// SignonStage tracks the connection handshake progress.
type SignonStage int

const (
	SignonNone SignonStage = iota
	SignonPrespawn
	SignonSignonBufs
	SignonSignonMsg
	SignonFlush
	SignonDone
)

// NetMessageType defines client-to-server message types.
type NetMessageType int

const (
	CLCNop NetMessageType = iota
	CLCDisconnect
	CLCMove
	CLCStringCmd
)

// ServerNetMessage defines server-to-client message types.
type ServerNetMessage int

const (
	SVCNop ServerNetMessage = iota
	SVCDamage
	SVCDisplayDisconnect
	SVCLevelName
	SVCLoaded
	SVCMove
	SVCEnterServer
	SVCSound
	SVCPrint
	SVCSinglePrecisionFrame
	SVCDoublePrecisionFrame
	SVCCreateBaseline
	SVCCreateBaseline2
	SVCLightStyle
	SVCTempEntity
	SVCCenterPrint
	SVCKillMonster
	SVCSpawnBaseline
	SVCSpawnBaseline2
	SVCSpawnStatic
	SVCSpawnStatic2
	SVCSpawnStaticSound
	SVCSpawnStaticSound2
	SVCClientData
	SVCDownload
	SVCUpdatePing
	SVCUpdateFrags
	SVCUpdateStat
	SVCParticle
	SVCCDTrack
	SVCLocalSound
	SVCSetAngle
	SVCSetView
	SVCUpdateUserInfo
	SVCSignOnNum
	SVCStuffText
	SVCTime
	SVCSetInfo
	SVCServerInfo
	SVCUpdateEnt
	SVCLocalSound2
)

// Max constants for server limits.
const (
	MaxClients       = 16
	MaxModels        = 2048
	MaxSounds        = 2048
	MaxEdicts        = 8192
	MaxDatagram      = 32000
	MaxSignonBuffers = 16
	MaxEntityLeafs   = 32
)

// Default physics/sound constants.
const (
	DefaultSoundVolume      = 255
	DefaultSoundAttenuation = 1.0
	ViewHeight              = 22
	OneEpsilon              = 0.01
)

// Vector math helper functions.

// VecAdd adds two vectors.
func VecAdd(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// VecSub subtracts two vectors.
func VecSub(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

// VecScale scales a vector by a scalar.
func VecScale(v [3]float32, s float32) [3]float32 {
	return [3]float32{v[0] * s, v[1] * s, v[2] * s}
}

// VecLen returns the length of a vector.
func VecLen(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// VecNormalize normalizes a vector, returning its original length.
func VecNormalize(v *[3]float32) float32 {
	len := VecLen(*v)
	if len > 0 {
		v[0] /= len
		v[1] /= len
		v[2] /= len
	}
	return len
}

// VecDot returns the dot product of two vectors.
func VecDot(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// VecCopy copies a vector.
func VecCopy(src [3]float32, dst *[3]float32) {
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
}

// VecCross returns the cross product of two vectors.
func VecCross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
