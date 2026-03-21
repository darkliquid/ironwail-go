// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package hud implements the Quake heads-up display rendering.
// It renders the status bar, centerprint messages, and other 2D overlays.
package hud

import (
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// HUDStyle selects the active HUD presentation.
type HUDStyle int

const (
	// HUDStyleClassic is the original Quake status-bar strip (default).
	HUDStyleClassic HUDStyle = 0
	// HUDStyleCompact is a minimal corner-overlay inspired by the Q64 layout
	// and the alternate HUD styles advertised in Ironwail's README.
	HUDStyleCompact HUDStyle = 1
	// HUDStyleQuakeWorld mirrors Ironwail's QuakeWorld status-bar layout, with
	// the main strip on the left and inventory/frag widgets on the right.
	HUDStyleQuakeWorld HUDStyle = 2
)

// hudStyleCVar is the console variable name that selects between the classic
// full-width status bar (0), compact corner overlay (1), and QuakeWorld (2)
// HUD layouts. The value is read each frame via cvar.IntValue so changes take
// effect immediately.
const hudStyleCVar = "hud_style"

// HUD manages the heads-up display rendering.
type HUD struct {
	drawManager *draw.Manager
	status      *StatusBar
	compact     *CompactHUD
	crosshair   Crosshair
	centerprint *Centerprint

	// Player state
	state State

	// Screen dimensions
	screenWidth  int
	screenHeight int
}

// State is the subset of client state required to render the classic status bar.
type State struct {
	Health       int
	Armor        int
	Ammo         int
	WeaponModel  int
	ActiveWeapon int
	Shells       int
	Nails        int
	Rockets      int
	Cells        int
	Items        uint32
	ModHipnotic  bool
	ModRogue     bool
	GameType     int
	MaxClients   int
	ShowScores   bool
	Scoreboard   []ScoreEntry

	Intermission    int
	CompletedTime   float64
	Time            float64
	CenterPrint     string
	CenterPrintAt   float64
	CenterPrintHold float64
	LevelName       string
	Secrets         int
	TotalSecrets    int
	Monsters        int
	TotalMonsters   int
}

// ScoreEntry is a single player row in the multiplayer scoreboard.
type ScoreEntry struct {
	ClientIndex int
	Name        string
	Frags       int
	Colors      byte
	IsCurrent   bool
}

// NewHUD creates a new HUD instance.
func NewHUD(dm *draw.Manager) *HUD {
	cvar.Register(hudStyleCVar, "0", cvar.FlagArchive, "HUD presentation style: 0=classic status bar, 1=compact Q64-style overlay, 2=QuakeWorld status bar")
	return &HUD{
		drawManager: dm,
		status:      NewStatusBar(dm),
		compact:     NewCompactHUD(),
		crosshair:   Crosshair{},
		centerprint: NewCenterprint(dm),
	}
}

// SetScreenSize updates the screen dimensions for layout.
func (h *HUD) SetScreenSize(width, height int) {
	h.screenWidth = width
	h.screenHeight = height
}

// SetState updates the HUD values from player/client state.
func (h *HUD) SetState(state State) {
	h.state = state
}

// State returns the latest HUD state snapshot.
func (h *HUD) State() State {
	return h.state
}

// Style returns the currently configured HUD style.
func (h *HUD) Style() HUDStyle {
	return HUDStyle(cvar.IntValue(hudStyleCVar))
}

// Draw renders the complete HUD overlay.
func (h *HUD) Draw(rc renderer.RenderContext) {
	if rc == nil {
		return
	}

	setHUDCanvasParams(rc, h.Style(), h.state, h.screenWidth, h.screenHeight)

	if h.state.Intermission == 0 {
		switch h.Style() {
		case HUDStyleCompact:
			if currentViewSize() < 120 {
				rc.SetCanvas(renderer.CanvasSbar2)
				width, height := canvasDimensions(rc, h.screenWidth, h.screenHeight)
				h.compact.Draw(rc, h.state, width, height)
			}
		case HUDStyleQuakeWorld:
			rc.SetCanvas(renderer.CanvasSbar)
			width, height := canvasDimensions(rc, h.screenWidth, h.screenHeight)
			h.status.DrawQuakeWorld(rc, h.state, width, height)
		default: // HUDStyleClassic
			rc.SetCanvas(renderer.CanvasSbar)
			width, height := canvasDimensions(rc, h.screenWidth, h.screenHeight)
			h.status.Draw(rc, h.state, width, height)
		}
	}
	rc.SetCanvas(renderer.CanvasDefault)
	h.crosshair.Draw(rc, h.state, h.screenWidth, h.screenHeight)
	h.centerprint.Draw(rc, h.state, h.screenWidth, h.screenHeight)
}

type canvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}

func setHUDCanvasParams(rc renderer.RenderContext, style HUDStyle, state State, screenWidth, screenHeight int) {
	setter, ok := rc.(canvasParamSetter)
	if !ok || screenWidth <= 0 || screenHeight <= 0 {
		return
	}

	sbarScale := float32(cvar.FloatValue("scr_sbarscale"))
	if sbarScale <= 0 {
		sbarScale = 1
	}
	menuScale := float32(cvar.FloatValue("scr_menuscale"))
	if menuScale <= 0 {
		menuScale = 1
	}
	crosshairScale := float32(cvar.FloatValue("scr_crosshairscale"))
	if crosshairScale <= 0 {
		crosshairScale = 1
	}

	setter.SetCanvasParams(renderer.CanvasTransformParams{
		GUIWidth:       float32(screenWidth),
		GUIHeight:      float32(screenHeight),
		GLWidth:        float32(screenWidth),
		GLHeight:       float32(screenHeight),
		ConWidth:       float32(screenWidth),
		ConHeight:      float32(screenHeight),
		SbarScale:      sbarScale,
		MenuScale:      menuScale,
		CrosshairScale: crosshairScale,
		GameType:       state.GameType,
		HudStyle:       int(style),
	})
}

func canvasDimensions(rc renderer.RenderContext, fallbackWidth, fallbackHeight int) (int, int) {
	canvas := rc.Canvas()
	width := int(canvas.Right - canvas.Left)
	height := int(canvas.Bottom - canvas.Top)
	if width <= 0 {
		width = fallbackWidth
	}
	if height <= 0 {
		height = fallbackHeight
	}
	return width, height
}

func currentViewSize() float64 {
	if cv := cvar.Get("scr_viewsize"); cv != nil && cv.Float > 0 {
		return cv.Float
	}
	return 100
}

// UpdateCrosshair updates the crosshair glyph from the crosshair cvar value.
func (h *HUD) UpdateCrosshair(crosshairValue float64) {
	h.crosshair.UpdateCvar(crosshairValue)
}

// SetCenterprint displays a centered message for the specified duration.
func (h *HUD) SetCenterprint(message string, duration time.Duration) {
	h.centerprint.SetMessage(message, duration)
}

// ClearCenterprint removes any active centerprint message.
func (h *HUD) ClearCenterprint() {
	h.centerprint.Clear()
}

// IsActive returns true if the HUD has any active elements.
func (h *HUD) IsActive() bool {
	return h.centerprint.IsActive()
}
