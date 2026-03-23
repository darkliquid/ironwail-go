package client

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/ironwail/ironwail-go/internal/common"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

const (
	Signons             = 4
	defaultMaxPitch     = 90.0
	defaultMinPitch     = -90.0
	defaultWheelPitch   = 5.0
	ItemShotgun         = 1 << 0
	ItemSuperShotgun    = 1 << 1
	ItemNailgun         = 1 << 2
	ItemSuperNailgun    = 1 << 3
	ItemGrenadeLauncher = 1 << 4
	ItemRocketLauncher  = 1 << 5
	ItemLightning       = 1 << 6
	ItemSuperLightning  = 1 << 7
	ItemShells          = 1 << 8
	ItemNails           = 1 << 9
	ItemRockets         = 1 << 10
	ItemCells           = 1 << 11
	ItemAxe             = 1 << 12
	ItemArmor1          = 1 << 13
	ItemArmor2          = 1 << 14
	ItemArmor3          = 1 << 15
	ItemKey1            = 1 << 17
	ItemKey2            = 1 << 18
	ItemInvisibility    = 1 << 19
	ItemInvulnerability = 1 << 20
	ItemSuit            = 1 << 21
	ItemQuad            = 1 << 22
	ItemSigil1          = 1 << 28
	ItemSigil2          = 1 << 29
	ItemSigil3          = 1 << 30
	ItemSigil4          = 1 << 31
)

type ClientState int

const (
	StateDisconnected ClientState = iota
	StateConnected
	StateActive
)

type KButton struct {
	Down  [2]int
	State int
}

type UserCmd struct {
	ViewAngles [3]float32
	Forward    float32
	Side       float32
	Up         float32
	Msec       uint8
	Buttons    int
	Impulse    int
}

type LightStyle struct {
	Length  int
	Map     string
	Average byte
	Peak    byte
}

type StaticSound struct {
	Origin      [3]float32
	SoundIndex  int
	Volume      int
	Attenuation float32
}

type SoundEvent struct {
	Entity      int
	Channel     int
	Origin      [3]float32
	SoundIndex  int
	SoundName   string
	Volume      int
	Attenuation float32
	Local       bool
}

type StopSoundEvent struct {
	Entity  int
	Channel int
}

type ParticleEvent struct {
	Origin [3]float32
	Dir    [3]float32
	Count  int
	Color  int
}

// TrailEvent represents a particle trail spawned by an entity each frame,
// based on its model flags (EF_ROCKET, EF_GRENADE, etc.). The renderer
// calls RocketTrail(Start, End, Type) for each event.
type TrailEvent struct {
	Start [3]float32 // Previous entity position (trail start)
	End   [3]float32 // Current entity position (trail end)
	Type  int        // Trail type: 0=rocket, 1=grenade smoke, 2=blood, 3=tracer, 4=slight blood, 5=tracer2, 6=voor trail
}

type TransientEvents struct {
	SoundEvents     []SoundEvent
	StopSoundEvents []StopSoundEvent
	ParticleEvents  []ParticleEvent
	TempEntities    []TempEntityEvent
	BeamSegments    []BeamSegment
	TrailEvents     []TrailEvent
}

type Client struct {
	State  ClientState
	Signon int

	Protocol      int32
	ProtocolFlags uint32

	MTime   [2]float64
	Time    float64
	OldTime float64

	ViewAngles  [3]float32
	MViewAngles [2][3]float32
	PunchAngle  [3]float32
	PunchAngles [2][3]float32
	PunchTime   float64
	MVelocity   [2][3]float32
	Velocity    [3]float32
	ViewHeight  float32
	IdealPitch  float32
	DriftMove   float32
	PitchVel    float32
	LastStop    float64
	NoDrift     bool

	FixAngle bool

	MoveMessages int
	Cmd          UserCmd
	PendingCmd   UserCmd

	InImpulse int

	MaxClients int
	GameType   int

	LevelName string
	MapName   string

	ModelPrecache []string
	SoundPrecache []string

	ViewEntity   int
	ViewEntAlpha byte
	CDTrack      int
	LoopTrack    int

	Intermission  int
	CompletedTime float64
	Paused        bool
	CenterPrint   string
	CenterPrintAt float64
	FaceAnimUntil float64

	Stats  [32]int
	StatsF [32]float32
	Items  uint32
	Frags  map[int]int

	KillCount   int
	SecretCount int

	PlayerNames  map[int]string
	PlayerColors map[int]byte

	SkyboxName    string
	FogDensity    byte
	FogColor      [3]byte
	FogTime       float32
	fogOldDensity float32
	fogOldColor   [3]float32
	fogFadeDone   float64
	fogFadeTime   float32

	OnGround bool
	InWater  bool

	EntityBaselines map[int]inet.EntityState
	Entities        map[int]inet.EntityState
	StaticEntities  []inet.EntityState
	StaticSounds    []StaticSound
	SoundEvents     []SoundEvent
	StopSoundEvents []StopSoundEvent
	ParticleEvents  []ParticleEvent
	TrailEvents     []TrailEvent
	TempEntities    []TempEntityEvent
	BeamSegments    []BeamSegment
	beams           [maxBeams]beamState
	DamageTaken     int
	DamageSaved     int
	DamageOrigin    [3]float32
	// Damage kick state - roll/pitch angles and time remaining.
	// Computed by CalculateDamageKick() and consumed by view calculation.
	DamageKickRoll  float32
	DamageKickPitch float32
	DamageKickTime  float32

	// CShifts holds the four color-shift blend channels used to compute the
	// v_blend polyblend screen tint each frame.  Mirrors C cl.cshifts[].
	CShifts [numCShifts]ColorShift

	StuffCmdBuf string

	// LerpPoint bypass flags — set by the host each frame.
	// When any is true, LerpPoint returns 1.0 (no interpolation).
	TimeDemoActive  bool // cls.timedemo equivalent
	LocalServerFast bool // sv.active && !host_netinterval equivalent
	NoLerp          bool // cl_nolerp cvar equivalent
	DemoPlayback    bool // cls.demoplayback — enables view angle interpolation

	ForwardSpeed float32
	BackSpeed    float32
	SideSpeed    float32
	UpSpeed      float32

	YawSpeed      float32
	PitchSpeed    float32
	AngleSpeedKey float32
	MoveSpeedKey  float32

	AlwaysRun  bool
	FreeLook   bool
	LookSpring bool

	MaxPitch   float32
	MinPitch   float32
	WheelPitch float32

	InputForward     KButton
	InputBack        KButton
	InputLeft        KButton
	InputRight       KButton
	InputUp          KButton
	InputDown        KButton
	InputLookUp      KButton
	InputLookDown    KButton
	InputMoveLeft    KButton
	InputMoveRight   KButton
	InputStrafe      KButton
	InputSpeed       KButton
	InputUse         KButton
	InputJump        KButton
	InputAttack      KButton
	InputKLook       KButton
	InputMLook       KButton
	MouseSideMove    float32
	MouseForwardMove float32
	MouseUpMove      float32

	LightStyles [64]LightStyle

	// Movement prediction state
	PredictedOrigin               [3]float32                // Predicted player position
	PredictedVelocity             [3]float32                // Predicted player velocity
	LastServerOrigin              [3]float32                // Last known server position
	PredictionError               [3]float32                // Error to correct over time
	PredictionValid               bool                      // Current-frame prediction is valid for camera use
	PredictionEntityNum           int                       // Entity number predicted on the current frame
	PredictionFrameTime           float64                   // Client time for the current prediction snapshot
	LocalViewTeleport             bool                      // True for the current relink frame when the local player hard-snapped
	CommandBuffer                 [256]UserCmd              // Queue of user commands for prediction
	CommandCount                  int                       // Number of unacknowledged commands in buffer
	CommandSequence               int                       // Total number of queued commands
	LastLerpTelemetry             LerpTelemetry             // Last LerpPoint evaluation snapshot
	LastPredictionReplayTelemetry PredictionReplayTelemetry // Last prediction replay snapshot

	// Prediction tuning parameters
	PredictionFriction  float32 // Ground friction coefficient
	PredictionAccel     float32 // Acceleration coefficient
	PredictionStopSpeed float32 // Ground friction minimum control speed
	PredictionGravity   float32 // Downward acceleration when airborne
	PredictionMaxSpeed  float32 // Maximum predicted speed
	PredictionErrorLerp float32 // Error correction speed (0.1-0.5)

	// ModelFlagsFunc returns model flags for a given model name.
	// Set by the host to allow RelinkEntities to check EF_ROTATE etc.
	ModelFlagsFunc func(modelName string) int
}

func NewClient() *Client {
	return &Client{
		State:           StateDisconnected,
		ForwardSpeed:    200,
		BackSpeed:       200,
		SideSpeed:       350,
		UpSpeed:         200,
		YawSpeed:        140,
		PitchSpeed:      150,
		AngleSpeedKey:   1.5,
		MoveSpeedKey:    2.0,
		AlwaysRun:       true,
		FreeLook:        true,
		ViewHeight:      inet.DEFAULT_VIEWHEIGHT,
		MaxPitch:        defaultMaxPitch,
		MinPitch:        defaultMinPitch,
		WheelPitch:      defaultWheelPitch,
		EntityBaselines: make(map[int]inet.EntityState),
		Entities:        make(map[int]inet.EntityState),
		Frags:           make(map[int]int),
		PlayerNames:     make(map[int]string),
		PlayerColors:    make(map[int]byte),
		// Prediction defaults (Quake movement-like tuning)
		PredictionFriction:  4.0,
		PredictionAccel:     10.0,
		PredictionStopSpeed: 100.0,
		PredictionGravity:   800.0,
		PredictionMaxSpeed:  320.0,
		PredictionErrorLerp: 0.3,
	}
}

func (c *Client) ClearState() {
	c.Signon = 0
	c.Protocol = 0
	c.ProtocolFlags = 0
	c.MTime = [2]float64{}
	c.Time = 0
	c.OldTime = 0
	c.PunchAngle = [3]float32{}
	c.PunchAngles = [2][3]float32{}
	c.PunchTime = 0
	c.ViewHeight = inet.DEFAULT_VIEWHEIGHT
	c.IdealPitch = 0
	c.DriftMove = 0
	c.PitchVel = 0
	c.LastStop = 0
	c.NoDrift = false
	c.ViewEntity = 0
	c.CDTrack = 0
	c.LoopTrack = 0
	c.LevelName = ""
	c.MapName = ""
	c.ModelPrecache = nil
	c.SoundPrecache = nil
	c.StuffCmdBuf = ""
	c.Intermission = 0
	c.CompletedTime = 0
	c.Paused = false
	c.CenterPrint = ""
	c.CenterPrintAt = 0
	c.FaceAnimUntil = 0
	c.FixAngle = false
	c.MoveMessages = 0
	c.InImpulse = 0
	c.PendingCmd = UserCmd{}
	c.Cmd = UserCmd{}
	c.ViewAngles = [3]float32{}
	c.MViewAngles = [2][3]float32{}
	c.MouseSideMove = 0
	c.MouseForwardMove = 0
	c.MouseUpMove = 0
	c.Stats = [32]int{}
	c.StatsF = [32]float32{}
	c.Items = 0
	c.DamageTaken = 0
	c.DamageSaved = 0
	c.DamageOrigin = [3]float32{}
	c.CShifts = [numCShifts]ColorShift{}
	c.OnGround = false
	c.InWater = false
	c.Velocity = [3]float32{}
	c.MVelocity = [2][3]float32{}
	c.KillCount = 0
	c.SecretCount = 0
	c.SkyboxName = ""
	c.FogDensity = 0
	c.FogColor = [3]byte{}
	c.FogTime = 0
	c.fogOldDensity = 0
	c.fogOldColor = [3]float32{}
	c.fogFadeDone = 0
	c.fogFadeTime = 0
	c.SoundEvents = nil
	c.StopSoundEvents = nil
	c.ParticleEvents = nil
	c.TempEntities = nil
	c.BeamSegments = nil
	c.beams = [maxBeams]beamState{}
	c.StaticEntities = nil
	c.StaticSounds = nil
	if c.Frags == nil {
		c.Frags = make(map[int]int)
	} else {
		clear(c.Frags)
	}
	if c.PlayerNames == nil {
		c.PlayerNames = make(map[int]string)
	} else {
		clear(c.PlayerNames)
	}
	if c.PlayerColors == nil {
		c.PlayerColors = make(map[int]byte)
	} else {
		clear(c.PlayerColors)
	}
	if c.EntityBaselines == nil {
		c.EntityBaselines = make(map[int]inet.EntityState)
	} else {
		clear(c.EntityBaselines)
	}
	if c.Entities == nil {
		c.Entities = make(map[int]inet.EntityState)
	} else {
		clear(c.Entities)
	}

	// Reset prediction state
	c.PredictedOrigin = [3]float32{}
	c.PredictedVelocity = [3]float32{}
	c.LastServerOrigin = [3]float32{}
	c.PredictionError = [3]float32{}
	c.PredictionValid = false
	c.PredictionEntityNum = 0
	c.PredictionFrameTime = 0
	c.LocalViewTeleport = false
	c.CommandCount = 0
	c.CommandSequence = 0
	c.LastLerpTelemetry = LerpTelemetry{}
	c.LastPredictionReplayTelemetry = PredictionReplayTelemetry{}
}

func (c *Client) LocalViewTeleportActive() bool {
	return c != nil && c.LocalViewTeleport
}

func (c *Client) LerpTelemetrySnapshot() LerpTelemetry {
	if c == nil {
		return LerpTelemetry{}
	}
	return c.LastLerpTelemetry
}

func (c *Client) PredictionReplayTelemetrySnapshot() PredictionReplayTelemetry {
	if c == nil {
		return PredictionReplayTelemetry{}
	}
	return c.LastPredictionReplayTelemetry
}

func (c *Client) HasFreshPredictionForCurrentEntity() bool {
	if c == nil || !c.PredictionValid {
		return false
	}
	if c.PredictionFrameTime != c.Time {
		return false
	}
	return c.PredictionEntityNum == c.predictionEntityNum()
}

func (c *Client) enqueueCommand(cmd UserCmd) {
	if c == nil {
		return
	}
	if c.CommandCount >= len(c.CommandBuffer) {
		// Drop the oldest unacknowledged command when the ring is full.
		c.CommandCount = len(c.CommandBuffer) - 1
	}
	idx := wrapBufferIndex(c.CommandSequence, len(c.CommandBuffer))
	c.CommandBuffer[idx] = cmd
	c.CommandSequence++
	if c.CommandCount < len(c.CommandBuffer) {
		c.CommandCount++
	}
}

func (c *Client) RecordSentCmd(cmd UserCmd) {
	if c == nil {
		return
	}
	c.enqueueCommand(cmd)
}

func (c *Client) bufferedCommands() []UserCmd {
	if c == nil || c.CommandCount == 0 {
		return nil
	}
	count := c.CommandCount
	start := c.CommandSequence - count
	commands := make([]UserCmd, 0, count)
	for i := 0; i < count; i++ {
		idx := wrapBufferIndex(start+i, len(c.CommandBuffer))
		commands = append(commands, c.CommandBuffer[idx])
	}
	return commands
}

func (c *Client) acknowledgeCommand() {
	if c == nil || c.CommandCount == 0 {
		return
	}
	c.CommandCount--
}

func (c *Client) newestBufferedCommand() (UserCmd, bool) {
	if c == nil || c.CommandCount == 0 {
		return UserCmd{}, false
	}
	idx := wrapBufferIndex(c.CommandSequence-1, len(c.CommandBuffer))
	return c.CommandBuffer[idx], true
}

func (c *Client) clearCommandReplayBacklog() {
	if c == nil || c.CommandCount == 0 {
		return
	}
	c.CommandCount = 0
}

func wrapBufferIndex(idx, size int) int {
	if size <= 0 {
		return 0
	}
	idx %= size
	if idx < 0 {
		idx += size
	}
	return idx
}

func (c *Client) ConsumeParticleEvents() []ParticleEvent {
	if c == nil || len(c.ParticleEvents) == 0 {
		return nil
	}
	events := c.ParticleEvents
	c.ParticleEvents = nil
	return events
}

func (c *Client) ConsumeSoundEvents() []SoundEvent {
	if c == nil || len(c.SoundEvents) == 0 {
		return nil
	}
	events := c.SoundEvents
	c.SoundEvents = nil
	return events
}

func (c *Client) ConsumeStopSoundEvents() []StopSoundEvent {
	if c == nil || len(c.StopSoundEvents) == 0 {
		return nil
	}
	events := c.StopSoundEvents
	c.StopSoundEvents = nil
	return events
}

func (c *Client) ConsumeTempEntities() []TempEntityEvent {
	if c == nil || len(c.TempEntities) == 0 {
		return nil
	}
	events := c.TempEntities
	c.TempEntities = nil
	return events
}

func (c *Client) ConsumeBeamSegments() []BeamSegment {
	if c == nil || len(c.BeamSegments) == 0 {
		return nil
	}
	segments := c.BeamSegments
	c.BeamSegments = nil
	return segments
}

func (c *Client) ConsumeTransientEvents() TransientEvents {
	if c == nil {
		return TransientEvents{}
	}
	return TransientEvents{
		SoundEvents:     c.ConsumeSoundEvents(),
		StopSoundEvents: c.ConsumeStopSoundEvents(),
		ParticleEvents:  c.ConsumeParticleEvents(),
		TempEntities:    c.ConsumeTempEntities(),
		BeamSegments:    c.ConsumeBeamSegments(),
	}
}

func (c *Client) ConsumeStuffCommands() string {
	if c == nil || c.StuffCmdBuf == "" {
		return ""
	}
	idx := strings.LastIndexByte(c.StuffCmdBuf, '\n')
	if idx < 0 {
		return ""
	}
	cmds := c.StuffCmdBuf[:idx+1]
	c.StuffCmdBuf = c.StuffCmdBuf[idx+1:]
	return cmds
}

// WeaponModelIndex returns the current first-person weapon model index.
func (c *Client) WeaponModelIndex() int {
	if c == nil {
		return 0
	}
	return c.Stats[statWeapon]
}

// WeaponFrame returns the current first-person weapon animation frame.
func (c *Client) WeaponFrame() int {
	if c == nil {
		return 0
	}
	return c.Stats[statWeaponFrame]
}

// Health returns the current player health stat.
func (c *Client) Health() int {
	if c == nil {
		return 0
	}
	return c.Stats[statHealth]
}

// Armor returns the current player armor stat.
func (c *Client) Armor() int {
	if c == nil {
		return 0
	}
	return c.Stats[statArmor]
}

// Ammo returns the current player ammo stat.
func (c *Client) Ammo() int {
	if c == nil {
		return 0
	}
	return c.Stats[statAmmo]
}

// ActiveWeapon returns the current active weapon bit flag.
func (c *Client) ActiveWeapon() int {
	if c == nil {
		return 0
	}
	return c.Stats[statActiveWeapon]
}

// AmmoCounts returns the shells, nails, rockets and cells counts.
func (c *Client) AmmoCounts() (int, int, int, int) {
	if c == nil {
		return 0, 0, 0, 0
	}
	return c.Stats[statShells], c.Stats[statNails], c.Stats[statRockets], c.Stats[statCells]
}

// LightStyleConfig controls lightstyle animation behavior. These correspond
// to the C Ironwail cvars r_flatlightstyles and r_lerplightstyles.
type LightStyleConfig struct {
	// FlatLightStyles controls flattening:
	//   0 = dynamic animation (default)
	//   1 = use average brightness (static)
	//   2 = use peak brightness (static)
	FlatLightStyles int
	// LerpLightStyles controls interpolation between animation frames:
	//   0 = no interpolation (snap to frame)
	//   1 = interpolate, but skip abrupt changes (default)
	//   2 = always interpolate
	LerpLightStyles int
	// DynamicLights enables dynamic light animation. When false, lightstyles
	// use their average value (equivalent to r_flatlightstyles=1).
	DynamicLights bool
}

// DefaultLightStyleConfig returns the default configuration matching
// C Ironwail's default cvar values.
func DefaultLightStyleConfig() LightStyleConfig {
	return LightStyleConfig{
		FlatLightStyles: 0,
		LerpLightStyles: 1,
		DynamicLights:   true,
	}
}

// LightStyleValues evaluates the current lightstyle scalars for the client clock.
func (c *Client) LightStyleValues() [64]float32 {
	return c.LightStyleValuesWithConfig(DefaultLightStyleConfig())
}

// LightStyleValuesWithConfig evaluates lightstyle brightness with the given
// animation configuration, matching C R_AnimateLight() behavior.
func (c *Client) LightStyleValuesWithConfig(cfg LightStyleConfig) [64]float32 {
	var out [64]float32
	for i := range out {
		out[i] = 1
	}
	if c == nil {
		return out
	}
	for i, style := range c.LightStyles {
		out[i] = evalLightStyleValue(style, c.Time, cfg)
	}
	return out
}

// CurrentFog evaluates the client's active fog state at the current client clock.
func (c *Client) CurrentFog() (density float32, color [3]float32) {
	if c == nil {
		return 0, [3]float32{}
	}

	targetDensity := float32(c.FogDensity) / 255
	targetColor := [3]float32{
		float32(c.FogColor[0]) / 255,
		float32(c.FogColor[1]) / 255,
		float32(c.FogColor[2]) / 255,
	}
	if c.fogFadeDone > c.Time && c.fogFadeTime > 0 {
		f := float32((c.fogFadeDone - c.Time) / float64(c.fogFadeTime))
		if f < 0 {
			f = 0
		}
		if f > 1 {
			f = 1
		}
		density = f*c.fogOldDensity + (1-f)*targetDensity
		for i := range color {
			color[i] = f*c.fogOldColor[i] + (1-f)*targetColor[i]
		}
	} else {
		density = targetDensity
		color = targetColor
	}

	for i := range color {
		if color[i] < 0 {
			color[i] = 0
		}
		if color[i] > 1 {
			color[i] = 1
		}
		color[i] = float32(math.Round(float64(color[i]*255))) / 255
	}
	if density < 0 {
		density = 0
	}
	if density > 1 {
		density = 1
	}
	return density, color
}

// lightstyleNormalBrightness is the brightness of character 'm' (normal light).
const lightstyleNormalBrightness = float32('m' - 'a') // 12

func evalLightStyleValue(style LightStyle, timeSeconds float64, cfg LightStyleConfig) float32 {
	if style.Length <= 0 || style.Map == "" {
		return 1
	}

	// Flat lightstyle modes use precomputed average/peak.
	if cfg.FlatLightStyles == 2 {
		return float32(style.Peak-'a') / lightstyleNormalBrightness
	}
	if cfg.FlatLightStyles == 1 || !cfg.DynamicLights {
		return float32(style.Average-'a') / lightstyleNormalBrightness
	}

	// Dynamic animation: 10 frames per second, matching C cl.time * 10.0.
	f := timeSeconds * 10.0
	base := math.Floor(f)
	frac := float32(f - base)
	if cfg.LerpLightStyles == 0 {
		frac = 0
	}

	idx := int(base) % style.Length
	if idx < 0 {
		idx += style.Length
	}
	next := (idx + 1) % style.Length

	k := float32(style.Map[idx] - 'a')
	n := float32(style.Map[next] - 'a')

	// Skip interpolation for abrupt changes (e.g. flickering) unless
	// r_lerplightstyles >= 2, matching C behavior.
	if cfg.LerpLightStyles < 2 {
		abruptThreshold := lightstyleNormalBrightness / 2 // 6
		diff := k - n
		if diff < 0 {
			diff = -diff
		}
		if diff >= abruptThreshold {
			n = k
		}
	}

	return (k + (n-k)*frac) / lightstyleNormalBrightness
}

func (c *Client) SetLightStyle(i int, style string) error {
	if i < 0 || i >= len(c.LightStyles) {
		return errors.New("lightstyle index out of range")
	}
	ls := &c.LightStyles[i]
	ls.Map = style
	ls.Length = len(style)
	if ls.Length == 0 {
		ls.Average = 'm'
		ls.Peak = 'm'
		return nil
	}
	total := 0
	peak := byte('a')
	for j := 0; j < len(style); j++ {
		ch := style[j]
		total += int(ch - 'a')
		if ch > peak {
			peak = ch
		}
	}
	ls.Peak = peak
	ls.Average = byte(total/ls.Length) + 'a'
	return nil
}

func (c *Client) LerpPoint() float64 {
	telemetry := LerpTelemetry{
		TimeBefore:   c.Time,
		TimeAfter:    c.Time,
		OldTime:      c.OldTime,
		MTime0Before: c.MTime[0],
		MTime1Before: c.MTime[1],
		MTime0After:  c.MTime[0],
		MTime1After:  c.MTime[1],
		Reason:       LerpTelemetryReasonNormal,
	}
	f := c.MTime[0] - c.MTime[1]
	telemetry.FrameDeltaBefore = f
	telemetry.FrameDeltaAfter = f
	// C: if (!f || cls.timedemo || (sv.active && !host_netinterval))
	switch {
	case f == 0:
		telemetry.Reason = LerpTelemetryReasonF0
	case c.TimeDemoActive:
		telemetry.Reason = LerpTelemetryReasonTimeDemo
	case c.LocalServerFast:
		telemetry.Reason = LerpTelemetryReasonFastServer
	case c.NoLerp:
		telemetry.Reason = LerpTelemetryReasonNoLerp
	}
	if telemetry.Reason != LerpTelemetryReasonNormal {
		c.Time = c.MTime[0]
		telemetry.TimeAfter = c.Time
		telemetry.Frac = 1
		c.LastLerpTelemetry = telemetry
		return 1
	}
	if f > 0.1 {
		c.MTime[1] = c.MTime[0] - 0.1
		f = 0.1
		telemetry.GapClamped = true
		telemetry.Reason = LerpTelemetryReasonGapClamp
	}
	telemetry.MTime1After = c.MTime[1]
	telemetry.FrameDeltaAfter = f
	frac := (c.Time - c.MTime[1]) / f
	telemetry.HasRawFrac = true
	telemetry.RawFrac = frac
	if frac < 0 {
		if frac < -0.01 {
			c.Time = c.MTime[1]
			telemetry.TimeSnapped = true
		}
		telemetry.TimeAfter = c.Time
		telemetry.Frac = 0
		telemetry.Reason = LerpTelemetryReasonFracLT0
		c.LastLerpTelemetry = telemetry
		return 0
	}
	if frac > 1 {
		if frac > 1.01 {
			c.Time = c.MTime[0]
			telemetry.TimeSnapped = true
		}
		telemetry.TimeAfter = c.Time
		telemetry.Frac = 1
		telemetry.Reason = LerpTelemetryReasonFracGT1
		c.LastLerpTelemetry = telemetry
		return 1
	}
	telemetry.TimeAfter = c.Time
	telemetry.Frac = frac
	c.LastLerpTelemetry = telemetry
	return frac
}

func (c *Client) SignonReply() {
	if c.Signon < 1 || c.Signon > Signons {
		return
	}
	if c.Signon == 4 {
		c.setState(StateActive)
	}
}

func (c *Client) setState(next ClientState) {
	if c.State == next {
		return
	}
	c.State = next
	log.Printf("Client state changed to %s", stateLogLabel(next))
}

func stateLogLabel(state ClientState) string {
	switch state {
	case StateDisconnected:
		return "Disconnected"
	case StateConnected:
		return "Connected"
	case StateActive:
		return "Active"
	default:
		return "Unknown"
	}
}

func (c *Client) HandleServerInfo() error {
	if c.State != StateDisconnected {
		return fmt.Errorf("serverinfo requires disconnected state, got %s", c.State)
	}
	c.setState(StateConnected)
	return nil
}

func (c *Client) HandleSignonReply(command string) error {
	if c.State != StateConnected {
		return fmt.Errorf("%s requires connected state, got %s", command, c.State)
	}

	switch signonReplyVerb(command) {
	case "prespawn":
		if c.Signon != 0 {
			return fmt.Errorf("prespawn requires signon 0, got %d", c.Signon)
		}
		c.Signon = 1
	case "spawn":
		if c.Signon != 1 {
			return fmt.Errorf("spawn requires signon 1, got %d", c.Signon)
		}
		c.Signon = 2
	case "begin":
		if c.Signon != 2 {
			return fmt.Errorf("begin requires signon 2, got %d", c.Signon)
		}
		c.Signon = 4
		c.setState(StateActive)
	default:
		return fmt.Errorf("unsupported signon reply %q", command)
	}

	return nil
}

func signonReplyVerb(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseMapNameFromWorldModel(worldModel string) string {
	if worldModel == "" {
		return ""
	}
	return common.COM_StripExtension(common.COM_SkipPath(worldModel))
}

func supportedProtocol(version int32) bool {
	return version == inet.PROTOCOL_NETQUAKE || version == inet.PROTOCOL_FITZQUAKE || version == inet.PROTOCOL_RMQ
}

func trimNUL(s string) string {
	return strings.TrimRight(s, "\x00")
}

// SendMove serializes a user command into a CLCMove message and sends it
// via unreliable channel. This is the low-level network transmission function.
//
// Message format (CLCMove):
//   - Byte: CLCMove opcode (3)
//   - Float: client time for ping calculation
//   - 3×Angle: view angles (angle8 or angle16 depending on protocol)
//   - Short: forward movement
//   - Short: side movement
//   - Short: up movement
//   - Byte: button bits (1=attack, 2=jump)
//   - Byte: impulse command
//
// Returns the serialized message bytes and any error.
// The caller is responsible for sending via socket.
func (c *Client) SendMove(cmd *UserCmd) ([]byte, error) {
	if c == nil || cmd == nil {
		return nil, nil
	}

	// Create message buffer (max size for move command is ~30 bytes)
	buf := common.NewSizeBuf(64)

	// Write CLCMove opcode
	if !buf.WriteByte(byte(inet.CLCMove)) {
		return nil, fmt.Errorf("failed to write CLCMove opcode")
	}

	// Write client time for ping calculation
	if !buf.WriteFloat(float32(c.Time)) {
		return nil, fmt.Errorf("failed to write client time")
	}

	// Write view angles (protocol-dependent encoding)
	// FitzQuake and RMQ always use 16-bit angles for CLC_MOVE,
	// matching C Ironwail CL_SendMove behavior. Only NetQuake uses 8-bit.
	useShortAngles := c.Protocol != inet.PROTOCOL_NETQUAKE

	for i := 0; i < 3; i++ {
		if useShortAngles {
			if !buf.WriteAngle16(cmd.ViewAngles[i]) {
				return nil, fmt.Errorf("failed to write view angle %d", i)
			}
		} else {
			if !buf.WriteAngle(cmd.ViewAngles[i]) {
				return nil, fmt.Errorf("failed to write view angle %d", i)
			}
		}
	}

	// Write movement values (convert float32 to int16)
	if !buf.WriteShort(int16(cmd.Forward)) {
		return nil, fmt.Errorf("failed to write forward movement")
	}
	if !buf.WriteShort(int16(cmd.Side)) {
		return nil, fmt.Errorf("failed to write side movement")
	}
	if !buf.WriteShort(int16(cmd.Up)) {
		return nil, fmt.Errorf("failed to write up movement")
	}

	// Write button bits
	if !buf.WriteByte(byte(cmd.Buttons)) {
		return nil, fmt.Errorf("failed to write buttons")
	}

	// Write impulse
	if !buf.WriteByte(byte(cmd.Impulse)) {
		return nil, fmt.Errorf("failed to write impulse")
	}

	// Return serialized message
	return buf.Data[:buf.CurSize], nil
}

func (c *Client) SendStringCmd(command string) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, nil
	}

	buf := common.NewSizeBuf(128 + len(command))
	if !buf.WriteByte(byte(inet.CLCStringCmd)) {
		return nil, fmt.Errorf("failed to write CLCStringCmd opcode")
	}
	if !buf.WriteString(command) {
		return nil, fmt.Errorf("failed to write CLCStringCmd payload")
	}
	return buf.Data[:buf.CurSize], nil
}

// SendCmd is the top-level function called each frame to send client commands
// to the server. It handles state checking, input gathering, and message sending.
//
// This should be called from the host frame loop after input processing but
// before rendering. For network games, pass the UDP socket. For loopback,
// use the loopback SendCommand implementation instead.
//
// Returns error if network transmission fails, but client should continue.
func (c *Client) SendCmd(sendFunc func([]byte) error) error {
	if c == nil {
		return nil
	}

	// Only send commands when connected (loopback handles this differently)
	if c.State != StateConnected && c.State != StateActive {
		return nil
	}

	// Skip first 2 move messages to avoid stale input on connection
	// (Quake convention to allow time for initial setup)
	if c.MoveMessages < 2 {
		c.MoveMessages++
		return nil
	}

	// Prepare command to send
	var cmd *UserCmd
	if c.Signon >= Signons {
		// Signon complete: sample one-shot button/impulse bits at send time.
		pending := c.BuildPendingMove()
		cmd = &pending
	} else {
		// Still in signon: send empty command to keep server in sync
		emptyCmd := UserCmd{
			ViewAngles: c.ViewAngles,
		}
		cmd = &emptyCmd
	}

	// Serialize command to network message
	msgData, err := c.SendMove(cmd)
	if err != nil {
		return fmt.Errorf("send move: %w", err)
	}

	// Send via provided send function (unreliable channel)
	if sendFunc != nil && len(msgData) > 0 {
		if err := sendFunc(msgData); err != nil {
			// Network error - log but continue (unreliable channel)
			log.Printf("SendCmd: network send failed: %v", err)
			return err
		}
	}

	// Update command state
	if c.Signon >= Signons {
		c.Cmd = *cmd
		c.RecordSentCmd(*cmd)
	}

	return nil
}
