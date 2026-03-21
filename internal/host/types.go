// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"sync"
	"time"

	"github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/compatrand"
	"github.com/ironwail/ironwail-go/internal/menu"
)

const (
	Version        = 1.09
	MinEdicts      = 256
	MaxEdicts      = 32000
	MaxQPath       = 64
	MaxLightstyles = 64
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

	args      []string
	baseDir   string
	dedicated bool
	gameDir   string
	userDir   string
	lastSave  string

	aborted     bool
	abortReason string

	menu      *menu.Manager
	demoState *client.DemoState

	// Subs holds the subsystem container for this host instance.
	// Previously stored in a package-level sync.Map registry; now owned
	// directly by the Host for explicit lifetime and dependency management.
	Subs *Subsystems

	loadingPlaqueActive bool
	loadingPlaqueUntil  float64
	loadingPlaqueHeld   bool
	loadingPlaqueHoldTo float64

	// Demo loop state (for startup demos like demo1, demo2, demo3)
	demoList []string
	demoNum  int // current demo index, -1 means don't play demos

	// Version info
	versionMajor int
	versionMinor int
	versionPatch int

	compatRNG *compatrand.RNG

	autosave autosaveState
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
	return &Host{
		maxFPS:       250,
		netInterval:  1.0 / 72,
		maxClients:   1,
		currentSkill: 1,
		demoNum:      -1, // disabled until startdemos is called
		compatRNG:    compatrand.New(),
	}
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

func (h *Host) SetFramerate(fps float64) {
	h.framerate = fps
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

func (h *Host) GetMenu() *menu.Manager {
	return h.menu
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
