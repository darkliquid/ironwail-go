package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0

	runtimeMaxPredictedXYOffset = 4.0

	csqcPicFlagAuto   uint32 = 0
	csqcPicFlagBlock  uint32 = 1 << 9
	csqcPicFlagNoLoad uint32 = 1 << 31
)

// Game consolidates all top-level engine state into a single struct.
// Previously these were scattered package-level variables; grouping them
// here makes ownership, lifetime, and dependencies explicit.
type Game struct {
	Host       *host.Host
	Server     *server.Server
	QC         *qc.VM
	CSQC       *qc.CSQC // Client-side QuakeC VM (nil when not loaded)
	Renderer   gameRenderer
	Subs       *host.Subsystems
	Client     *cl.Client
	Particles  *renderer.ParticleSystem
	DecalMarks *renderer.DecalMarkSystem

	ParticleRNG  *rand.Rand
	ParticleTime float32
	RuntimeBeams []cl.BeamSegment

	Menu  *menu.Manager
	Input *input.System
	Draw  *draw.Manager
	HUD   *hud.HUD
	Audio *audio.AudioAdapter

	MouseGrabbed     bool
	AliasModelCache  map[string]*model.Model
	SpriteModelCache map[string]*runtimeSpriteModel
	SoundSFXByIndex  map[int]*audio.SFX
	MenuSFXByName    map[string]*audio.SFX
	AmbientSFX       [audio.NumAmbients]*audio.SFX
	SoundPrecacheKey string
	StaticSoundKey   string
	MusicTrackKey    string
	SkyboxNameKey    string
	WorldUploadKey   string
	ShowScores       bool
	ModDir           string

	CameraInLiquid     bool
	CameraLeafContents int32

	// Scope zoom state, updated each frame via renderer.UpdateZoom.
	Zoom    float32
	ZoomDir float32

	ConsoleSlideFraction float32
	TextEditRepeat       runtimeTextEditRepeatState
	FPSOverlay           runtimeFPSOverlay
	SpeedOverlay         runtimeSpeedOverlay
	DemoOverlay          runtimeDemoOverlay
	TurtleOverlayCount   int
	LastServerMessageAt  float64
}

type gameRendererFrameLoop interface {
	OnDraw(func(renderer.RenderContext))
	OnUpdate(func(dt float64))
	Size() (width, height int)
	SetConfig(renderer.Config)
	Run() error
	Stop()
	Shutdown()
}

type gameRendererAssets interface {
	SetPalette([]byte)
	SetConchars([]byte)
	SetExternalSkybox(string, func(string) ([]byte, error))
}

type gameRendererWorld interface {
	UpdateCamera(renderer.CameraState, float32, float32)
	UploadWorld(*bsp.Tree) error
	HasWorldData() bool
	GetWorldBounds() (min [3]float32, max [3]float32, ok bool)
}

type gameRendererLights interface {
	SpawnDynamicLight(renderer.DynamicLight) bool
	SpawnKeyedDynamicLight(renderer.DynamicLight) bool
	UpdateLights(float32)
	ClearDynamicLights()
}

type gameRendererInput interface {
	InputBackendForSystem(*input.System) input.Backend
}

type gameRenderer interface {
	gameRendererFrameLoop
	gameRendererAssets
	gameRendererWorld
	gameRendererLights
	gameRendererInput
}

var g Game
var runtimeNow = time.Now
var runtimeStateMu sync.Mutex

var (
	pendingRendererAssetsMu      sync.Mutex
	pendingRendererPalette       []byte
	pendingRendererConchars      []byte
	pendingRendererAssetsPending bool
	pendingRendererWorldClear    bool
)

type canvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}

func queueRuntimeRendererAssets(palette []byte, conchars []byte) {
	pendingRendererAssetsMu.Lock()
	defer pendingRendererAssetsMu.Unlock()

	pendingRendererPalette = append(pendingRendererPalette[:0], palette...)
	pendingRendererConchars = append(pendingRendererConchars[:0], conchars...)
	pendingRendererAssetsPending = true
}

func queueRuntimeRendererWorldClear() {
	pendingRendererAssetsMu.Lock()
	defer pendingRendererAssetsMu.Unlock()

	pendingRendererWorldClear = true
}

func applyQueuedRuntimeRendererAssets(target gameRenderer) {
	if target == nil {
		return
	}

	pendingRendererAssetsMu.Lock()
	if !pendingRendererAssetsPending && !pendingRendererWorldClear {
		pendingRendererAssetsMu.Unlock()
		return
	}
	clearWorld := pendingRendererWorldClear
	palette := append([]byte(nil), pendingRendererPalette...)
	conchars := append([]byte(nil), pendingRendererConchars...)
	pendingRendererPalette = pendingRendererPalette[:0]
	pendingRendererConchars = pendingRendererConchars[:0]
	pendingRendererAssetsPending = false
	pendingRendererWorldClear = false
	pendingRendererAssetsMu.Unlock()

	if clearWorld {
		if clearer, ok := any(target).(interface{ ClearWorld() }); ok {
			clearer.ClearWorld()
		}
	}
	if len(palette) >= 768 {
		target.SetPalette(palette)
	}
	if len(conchars) >= 128*128 {
		target.SetConchars(conchars)
	}
}

type defaultBinding struct {
	key     int
	command string
}

type runtimeFPSOverlay struct {
	oldTime       float64
	lastFPS       float64
	oldFrameCount int
}

type runtimeTextEditRepeatState struct {
	key       int
	nextDelay float64
}

type runtimeSpeedOverlay struct {
	maxSpeed     float32
	displaySpeed float32
	lastRealTime float64
}

type runtimeDemoOverlay struct {
	prevSpeed     float32
	prevBaseSpeed float32
	showTime      float64
}

type runtimeTelemetryState struct {
	RealTime        float64
	FrameCount      int
	FrameTime       float64
	ViewSize        float32
	HUDStyle        int
	ShowFPS         float32
	ShowClock       int
	ShowSpeed       bool
	ShowTurtle      bool
	ShowSpeedOfs    float32
	ClientTime      float64
	Intermission    int
	InCutscene      bool
	DemoPlayback    bool
	DemoSpeed       float32
	DemoBaseSpeed   float32
	DemoProgress    float64
	DemoName        string
	DemoBarTimeout  float32
	ClientActive    bool
	Velocity        [3]float32
	ConsoleForced   bool
	LastServerMsgAt float64
	SavingActive    bool
	ViewRect        renderer.ViewRect
}

type runtimeSpriteModel struct {
	model  *model.Model
	sprite *model.MSprite
}

var gameplayDefaultBindings = []defaultBinding{
	{key: int('`'), command: "toggleconsole"},
	{key: int('w'), command: "+forward"},
	{key: input.KUpArrow, command: "+forward"},
	{key: int('s'), command: "+back"},
	{key: input.KDownArrow, command: "+back"},
	{key: int('a'), command: "+moveleft"},
	{key: int('d'), command: "+moveright"},
	{key: input.KLeftArrow, command: "+left"},
	{key: input.KRightArrow, command: "+right"},
	{key: input.KShift, command: "+speed"},
	{key: input.KAlt, command: "+strafe"},
	{key: input.KTab, command: "+showscores"},
	{key: input.KCtrl, command: "+attack"},
	{key: input.KMouse1, command: "+attack"},
	{key: input.KSpace, command: "+jump"},
	{key: input.KMouse2, command: "+jump"},
	{key: int('e'), command: "+use"},
	{key: input.KMouse3, command: "+mlook"},
	{key: input.KMWheelUp, command: "impulse 10"},
	{key: input.KMWheelDown, command: "impulse 12"},
}

var essentialFallbackBindings = []defaultBinding{
	{key: input.KEscape, command: "togglemenu"},
	{key: int('`'), command: "toggleconsole"},
}

type globalCommandBuffer struct{}

func (globalCommandBuffer) Init()    {}
func (globalCommandBuffer) Execute() { cmdsys.Execute() }
func (globalCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	cmdsys.ExecuteWithSource(source)
}
func (globalCommandBuffer) ExecuteTextWithSource(text string, source cmdsys.CommandSource) {
	cmdsys.ExecuteTextWithSource(text, source)
}
func (globalCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalCommandBuffer) Shutdown() {}
