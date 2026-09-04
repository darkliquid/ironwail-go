// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"sync"
	"time"

	"github.com/darkliquid/ironwail-go/internal/async"
	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/menu"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

const (
	Version        = 1.09
	MinEdicts      = 256
	MaxEdicts      = 32000
	MaxQPath       = 64
	MaxLightstyles = 256
	MaxModels      = 4096
	MaxSounds      = 2048
	MaxScoreboard  = 16
	SoundChannels  = 8
	NumSpawnParms  = 16
)

type ClientState int

const (
	caDisconnected ClientState = iota
	caConnecting
	caConnected
	caActive
)

type Host struct {
	mu sync.Mutex

	initialized bool
	frameCount  int

	realtime     float64
	oldrealtime  float64
	frameTime    float64
	rawFrameTime float64
	simFrameTime float64
	netInterval  float64
	accumTime    float64

	serverActive bool
	serverPaused bool
	maxClients   int
	currentSkill int

	clientState ClientState
	signOns     int

	timeScale float64
	maxFPS    float64
	framerate float64

	args        []string
	baseDir     string
	baseGameDir string
	dedicated   bool
	gameDir     string
	userDir     string
	userFS      UserFS
	lastSave    string
	spawnArgs   string

	aborted     bool
	abortReason string

	menu                   *menu.Manager
	demoState              *client.DemoState
	gameDirChangedCallback func(subs *Subsystems, changed *fs.FileSystem) error

	// Subs holds the subsystem container for this host instance.
	// Previously stored in a package-level sync.Map registry; now owned
	// directly by the Host for explicit lifetime and dependency management.
	Subs *Subsystems

	loadingPlaqueActive bool
	loadingPlaqueUntil  float64
	loadingPlaqueHeld   bool
	loadingPlaqueHoldTo float64
	savingIconUntil     float64

	// Demo loop state (for startup demos like demo1, demo2, demo3)
	demoList []string
	demoNum  int // current demo index, -1 means don't play demos

	// Version info
	versionMajor int
	versionMinor int
	versionPatch int

	compatRNG *compatrand.RNG

	autosave autosaveState

	netStats *inet.NetStats

	saveWorker saveWorker

	title windowTitleState

	modsDL modsDownloaderState
	// Cmd is the host-owned command system instance. All host/game/server
	// command registrations and command-buffer execution flow through this
	// instance instead of the previous package-level singleton.
	Cmd *cmdsys.CmdSystem

	// CVar is the host-owned console variable registry. All cvar
	// registrations and lookups flow through this instance instead of
	// the previous package-level singleton.
	CVar *cvar.CVarSystem

	// Net is the host-owned networking subsystem. All connect/listen/
	// send/receive and IP-ban operations flow through this instance.
	// Currently its methods still delegate to internal/net package-level
	// state (preserved for backward compatibility during the phased DI
	// migration); per-instance isolation lands in a later phase.
	Net *inet.Network

	// mainThreadQueue marshals work from background goroutines (save
	// worker, mod downloader, etc) onto the main Host.Frame thread.
	// Drained once per frame during Host.Frame.
	mainThreadQueue *async.Queue

	RemoteClientFactory  func(address string) (Client, error)
	ServerBrowserFactory func() serverBrowser
}

type autosaveState struct {
	lastTime    float64
	cheatTime   float64
	hurtTime    float64
	shootTime   float64
	prevHealth  float64
	prevSecrets float64
	secretBoost float64
}

func NewHost() *Host {
	network := inet.NewNetwork()
	h := &Host{
		maxFPS:               250,
		netInterval:          1.0 / 72,
		maxClients:           1,
		currentSkill:         1,
		demoNum:              -1, // disabled until startdemos is called
		compatRNG:            compatrand.New(),
		netStats:             &inet.NetStats{},
		Cmd:                  cmdsys.NewCmdSystem(),
		CVar:                 cvar.NewCVarSystem(),
		Net:                  network,
		mainThreadQueue:      async.NewQueue(1024),
		ServerBrowserFactory: defaultServerBrowserFactory,
	}
	h.RemoteClientFactory = func(address string) (Client, error) {
		return defaultRemoteClientFactory(h.Net, address)
	}
	h.ServerBrowserFactory = h.defaultServerBrowserFactory
	h.Cmd.CVar = h.CVar
	return h
}

// NetStats returns the host-owned network statistics counters.
func (h *Host) NetStats() *inet.NetStats {
	return h.netStats
}

func (h *Host) IsInitialized() bool {
	return h.initialized
}

func (h *Host) FrameCount() int {
	return h.frameCount
}

func (h *Host) FrameTime() float64 {
	return h.frameTime
}

// RawFrameTime returns the real wall-clock frame delta, unaffected by the
// net-tick batching that temporarily overwrites h.frameTime in the "send"
// phase of Host.Frame. Callers driving simulation clocks that must advance
// at real time (e.g. demo playback's cl.time) should use this instead of
// FrameTime to avoid running faster than real time when host_maxfps > 72.
// Mirrors C Quake's use of host_rawframetime for CL_AdjustAngles / demo
// timing while host_frametime is the (possibly-capped) server tick step.
func (h *Host) RawFrameTime() float64 {
	return h.rawFrameTime
}

// SimFrameTime returns the canonical simulation frame delta established by
// advanceTime at the start of each Host.Frame. Unlike FrameTime, it is not
// temporarily mutated by the net-tick "send" block, so callers that advance
// long-running simulation clocks (e.g. demo playback's cl.time) see the same
// value during both the send and read phases of a host frame. This mirrors C
// Quake's host_frametime value as seen outside the send/CL_SendMove block.
func (h *Host) SimFrameTime() float64 {
	return h.simFrameTime
}

// SetFrameTime overrides the current frame delta. Intended for tests that
// exercise code paths depending on Host.FrameTime.
func (h *Host) SetFrameTime(dt float64) {
	h.frameTime = dt
}

func (h *Host) RealTime() float64 {
	return h.realtime
}

func (h *Host) ServerActive() bool {
	return h.serverActive
}

func (h *Host) SetServerActive(active bool) {
	h.serverActive = active
}

func (h *Host) ServerPaused() bool {
	return h.serverPaused
}

func (h *Host) SetServerPaused(paused bool) {
	h.serverPaused = paused
}

func (h *Host) MaxClients() int {
	return h.maxClients
}

func (h *Host) ClientState() ClientState {
	return h.clientState
}

func (h *Host) ClientSessionActive() bool {
	return h.clientState != caDisconnected
}

func (h *Host) SetClientState(state ClientState) {
	h.clientState = state
}

func (h *Host) SignOns() int {
	return h.signOns
}

func (h *Host) SetSignOns(count int) {
	h.signOns = count
	if count >= client.Signons {
		h.EndLoadingPlaque(0)
	}
}

func (h *Host) CurrentSkill() int {
	return h.currentSkill
}

func (h *Host) SetCurrentSkill(skill int) {
	h.currentSkill = skill
}

func (h *Host) Args() []string {
	return h.args
}

func (h *Host) SetArgs(args []string) {
	h.args = args
}

func (h *Host) BaseDir() string {
	return h.baseDir
}

func (h *Host) SetBaseDir(dir string) {
	h.baseDir = dir
}

func (h *Host) UserDir() string {
	return h.userDir
}

func (h *Host) SetUserDir(dir string) {
	h.userDir = dir
}

func (h *Host) SetTimeScale(scale float64) {
	h.timeScale = scale
}

func (h *Host) SetMaxFPS(fps float64) {
	h.maxFPS = fps
	if fps > 72 || fps <= 0 {
		h.netInterval = 1.0 / 72
	} else {
		h.netInterval = 0
	}
}

func (h *Host) NetInterval() float64 {
	return h.netInterval
}

func (h *Host) LocalServerFast() bool {
	return h.serverActive && h.netInterval == 0
}

// SetFramerate pins the simulation timestep to a fixed number of seconds per
// frame. Zero restores normal wall-clock-driven timing. Matches C Ironwail's
// host_framerate cvar: the value is seconds-per-frame, not FPS.
func (h *Host) SetFramerate(secondsPerFrame float64) {
	h.framerate = secondsPerFrame
}

func (h *Host) Abort(reason string) {
	h.aborted = true
	h.abortReason = reason
}

func (h *Host) ClearAbort() {
	h.aborted = false
	h.abortReason = ""
}

func (h *Host) IsAborted() bool {
	return h.aborted
}

func (h *Host) AbortReason() string {
	return h.abortReason
}

func (h *Host) SetMenu(menu *menu.Manager) {
	h.menu = menu
}

func (h *Host) Menu() *menu.Manager {
	return h.menu
}

func (h *Host) SetGameDirChangedCallback(cb func(subs *Subsystems, changed *fs.FileSystem) error) {
	h.gameDirChangedCallback = cb
}

func (h *Host) DemoState() *client.DemoState {
	return h.demoState
}

func (h *Host) SetDemoState(ds *client.DemoState) {
	h.demoState = ds
}

func (h *Host) DemoList() []string {
	return h.demoList
}

func (h *Host) SetDemoList(demos []string) {
	h.demoList = demos
}

func (h *Host) DemoNum() int {
	return h.demoNum
}

func (h *Host) SetDemoNum(num int) {
	h.demoNum = num
}

const (
	loadingPlaqueMinDuration  = 0.2
	loadingPlaqueHoldDuration = 60.0
)

func (h *Host) BeginLoadingPlaque(now float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if now <= 0 {
		now = currentTime()
	}
	h.loadingPlaqueActive = true
	h.loadingPlaqueUntil = now + loadingPlaqueMinDuration
	h.loadingPlaqueHeld = false
	h.loadingPlaqueHoldTo = 0
}

func (h *Host) BeginLoadingTransitionPlaque(now float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if now <= 0 {
		now = currentTime()
	}
	h.loadingPlaqueActive = true
	h.loadingPlaqueUntil = now + loadingPlaqueMinDuration
	h.loadingPlaqueHeld = true
	h.loadingPlaqueHoldTo = now + loadingPlaqueHoldDuration
}

func (h *Host) EndLoadingPlaque(now float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.loadingPlaqueHeld = false
	h.loadingPlaqueHoldTo = 0
	if now > 0 && now > h.loadingPlaqueUntil {
		h.loadingPlaqueActive = false
		h.loadingPlaqueUntil = 0
	}
}

func (h *Host) LoadingPlaqueActive(now float64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.loadingPlaqueActive {
		return false
	}
	if now <= 0 {
		now = currentTime()
	}
	if now <= h.loadingPlaqueUntil {
		return true
	}
	if h.loadingPlaqueHeld {
		if now <= h.loadingPlaqueHoldTo {
			return true
		}
		h.loadingPlaqueHeld = false
		h.loadingPlaqueHoldTo = 0
	}
	h.loadingPlaqueActive = false
	h.loadingPlaqueUntil = 0
	return false
}

const savingIconMinDuration = 0.2

func (h *Host) BeginSavingIndicator(now float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if now <= 0 {
		now = h.realtime
		if now <= 0 {
			now = currentTime()
		}
	}
	h.savingIconUntil = now + savingIconMinDuration
}

func (h *Host) SavingIndicatorActive(now float64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.savingIconUntil <= 0 {
		return false
	}
	if now <= 0 {
		now = h.realtime
		if now <= 0 {
			now = currentTime()
		}
	}
	if now <= h.savingIconUntil {
		return true
	}
	h.savingIconUntil = 0
	return false
}

func (h *Host) Lock() {
	h.mu.Lock()
}

func (h *Host) Unlock() {
	h.mu.Unlock()
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func currentTime() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}
