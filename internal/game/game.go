package game

import (
	"math/rand"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/client"
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

var runtimeStateMu sync.Mutex

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
	TextEditRepeat       TextEditRepeatState
	FPSOverlay           FPSOverlay
	SpeedOverlay         SpeedOverlay
	DemoOverlay          DemoOverlay
	TurtleOverlayCount   int
	LastServerMessageAt  float64

	// Private state for renderer asset queuing
	pendingRendererAssets *PendingRendererAssets
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
		AliasModelCache:       make(map[string]*model.Model),
		SpriteModelCache:      make(map[string]*SpriteModel),
		SoundSFXByIndex:       make(map[int]*audio.SFX),
		MenuSFXByName:         make(map[string]*audio.SFX),
		pendingRendererAssets: &PendingRendererAssets{},
	}
}

