package game

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
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

// Game consolidates all top-level engine state into a single struct.
// Previously these were scattered package-level variables; grouping them
// here makes ownership, lifetime, and dependencies explicit.
type Game struct {
	Host       *host.Host
	Server     *server.Server
	QC         *qc.VM
	CSQC       *qc.CSQC // Client-side QuakeC VM (nil when not loaded)
	Renderer   Renderer
	Subs       *host.Subsystems
	Client     *client.Client
	Particles  *renderer.ParticleSystem
	DecalMarks *renderer.DecalMarkSystem

	ParticleRNG  *rand.Rand
	ParticleTime float32
	RuntimeBeams []client.BeamSegment

	Menu  *menu.Manager
	Input *input.System
	Draw  *draw.Manager
	HUD   *hud.HUD
	Audio *audio.AudioAdapter

	MouseGrabbed     bool
	AliasModelCache  map[string]*model.Model
	SpriteModelCache map[string]*SpriteModel
	BrushModelCache  map[string]*bsp.Tree
	SoundSFXByIndex  map[int]*audio.SFX
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
	TextEditRepeat       TextEditRepeatState
	FPSOverlay           FPSOverlay
	SpeedOverlay         SpeedOverlay
	DemoOverlay          DemoOverlay
	TurtleOverlayCount   int
	LastServerMessageAt  float64

	// Private state for renderer asset queuing
	pendingRendererAssets *PendingRendererAssets

	// processClientPhase tracks which half of the client frame step we are
	// in ("send" or "read"); set by the gameCallbacks adapter.
	processClientPhase string

	// runtimeMu serializes runtime state access between the game update
	// callback and the renderer draw callback.
	runtimeMu sync.Mutex

	// inputDispatchLogCount caps the verbose input-dispatch log stream.
	inputDispatchLogCount atomic.Uint32

	// cpuProfile tracks the active CPU profile capture state.
	cpuProfile cpuProfileState

	// loadDemoWorldTree is a test-swappable hook that loads the demo
	// playback world (.bsp + .lit). Production code uses the default
	// implementation set in New().
	loadDemoWorldTree func(files host.Filesystem, worldModel string) (*bsp.Tree, error)

	// viewCalc holds per-frame view calculation state (bob, roll, gun
	// angles, damage kick, origin-select latch). Mirrors C Ironwail's
	// V_* globals scoped to the active gameplay session.
	viewCalc viewCalcState

	// chatBuffer holds the in-progress chat line while the "messagemode"
	// input destination is active. chatTeam selects global ("say") vs.
	// team ("say_team") broadcast on send.
	chatBuffer string
	chatTeam   bool

	// debugView holds client-side view debug telemetry state (coalescing
	// ring buffers, last-seen origins, etc.) guarded by the
	// cl_debug_view cvar. Moved from a package-level singleton so each
	// Game instance has an isolated trace.
	debugView              debugViewTelemetryState
	debugViewTelemetryCVar *cvar.CVar
	debugViewTelemetryEmit func(line string)
}

type cpuProfileState struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// PendingRendererAssets holds queued renderer assets to be applied.
// This encapsulates the asset queuing logic and reduces global state.
type PendingRendererAssets struct {
	mu         sync.Mutex
	Palette    []byte
	Conchars   []byte
	HasPending bool
	ClearWorld bool
}

// New creates a new Game instance with initialized caches.
func New() *Game {
	return &Game{
		Host:                  host.NewHost(),
		AliasModelCache:       make(map[string]*model.Model),
		SpriteModelCache:      make(map[string]*SpriteModel),
		SoundSFXByIndex:       make(map[int]*audio.SFX),
		pendingRendererAssets: &PendingRendererAssets{},
		loadDemoWorldTree:     defaultLoadDemoWorldTree,
		debugViewTelemetryEmit: func(line string) {
			fmt.Fprintln(os.Stderr, line)
		},
	}
}
