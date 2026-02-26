// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"sync"
	"time"

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

	args    []string
	baseDir string
	gameDir string
	userDir string

	aborted     bool
	abortReason string

	menu *menu.Manager
}

func NewHost() *Host {
	return &Host{
		maxFPS:       250,
		netInterval:  1.0 / 72,
		maxClients:   1,
		currentSkill: 1,
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
