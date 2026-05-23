// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package hud implements the Quake heads-up display rendering.
// It renders the status bar, centerprint messages, and other 2D overlays.
package hud

import (
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// HUDStyle selects the active HUD presentation. Values match C Ironwail's
// hudstyle_t enum: 0=classic, 1=modern center-ammo, 2=modern side-ammo,
// 3=QuakeWorld.
type HUDStyle int

const (
	// HUDStyleClassic is the original Quake status-bar strip (default).
	HUDStyleClassic HUDStyle = 0
	// HUDStyleModernCenterAmmo is the SBAR2-based "Modern 1" layout: big
	// corner face/health/armor and ammo pair, with a centered 4x1 ammo strip.
	HUDStyleModernCenterAmmo HUDStyle = 1
	// HUDStyleModernSideAmmo is the SBAR2-based "Modern 2" layout, identical
	// to Modern 1 but with a 2x2 ammo block tucked into the right side.
	HUDStyleModernSideAmmo HUDStyle = 2
	// HUDStyleQuakeWorld mirrors Ironwail's QuakeWorld status-bar layout, with
	// the main strip on the left and inventory/frag widgets on the right.
	HUDStyleQuakeWorld HUDStyle = 3

	// HUDStyleCompact is a backwards-compatible alias for
	// HUDStyleModernCenterAmmo.
	HUDStyleCompact HUDStyle = HUDStyleModernCenterAmmo
)

// hudStyleCVar is the console variable name that selects between the classic
// full-width status bar (0), compact corner overlay (1), and QuakeWorld (2)
// HUD layouts. The value is read each frame via cvar.IntValue so changes take
// effect immediately.
const hudStyleCVar = "hud_style"

// HUD manages the heads-up display rendering.
type HUD struct {
	drawManager *draw.Manager
	cvars       *cvar.CVarSystem
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

	Paused                  bool
	InCutscene              bool
	Intermission            int
	HideIntermissionOverlay bool
	CompletedTime           float64
	Time                    float64
	CenterPrint             string
	CenterPrintAt           float64
	FaceAnimUntil           float64
	CenterPrintHold         float64
	LevelName               string
	Secrets                 int
	TotalSecrets            int
	Monsters                int
	TotalMonsters           int
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
func NewHUD(dm *draw.Manager, cv *cvar.CVarSystem) *HUD {
	if cv == nil {
		cv = cvar.NewCVarSystem()
	}
	hudStyleCVarObj := cv.Register(hudStyleCVar, "0", cvar.FlagArchive, "HUD presentation style: 0=classic status bar, 1=modern center-ammo, 2=modern side-ammo, 3=QuakeWorld status bar")
	hudstyleLegacyObj := cv.Register("hudstyle", "0", cvar.FlagArchive, "HUD presentation style (legacy alias)")

	hudStyleCVarObj.Callback = func(c *cvar.CVar) {
		if legacy := cv.Get("hudstyle"); legacy != nil && legacy.String != c.String {
			cv.Set("hudstyle", c.String)
		}
	}
	hudstyleLegacyObj.Callback = func(c *cvar.CVar) {
		if style := cv.Get(hudStyleCVar); style != nil && style.String != c.String {
			cv.Set(hudStyleCVar, c.String)
		}
	}

	return &HUD{
		drawManager: dm,
		cvars:       cv,
		status:      NewStatusBar(dm, cv),
		compact:     NewCompactHUD(),
		crosshair:   Crosshair{cvars: cv},
		centerprint: NewCenterprint(dm, cv),
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
	return HUDStyle(h.cvars.IntValue(hudStyleCVar))
}

// Draw renders the complete HUD overlay.
func (h *HUD) Draw(rc renderer.RenderContext) {
	if rc == nil {
		return
	}

	setHUDCanvasParams(h.cvars, rc, h.Style(), h.state, h.screenWidth, h.screenHeight)

	if h.state.Intermission == 0 {
		switch h.Style() {
		case HUDStyleModernCenterAmmo, HUDStyleModernSideAmmo:
			if currentViewSize(h.cvars) < 120 {
				rc.SetCanvas(renderer.CanvasSbar2)
				sideAmmo := h.Style() == HUDStyleModernSideAmmo
				h.status.DrawModern(rc, h.state, sideAmmo)
			}
		case HUDStyleQuakeWorld:
			rc.SetCanvas(renderer.CanvasSbar)
			// CanvasSbar is a fixed 320x48 logical coordinate system, centered
			// at the bottom of the screen by the canvas transform. Pass those
			// intrinsic dimensions directly rather than the full-screen clip
			// bounds returned by canvasDimensions (which would place the bar
			// off-screen because StatusBar.Draw treats them as screen coords).
			h.status.DrawQuakeWorld(rc, h.state, 320, 48)
		default: // HUDStyleClassic
			rc.SetCanvas(renderer.CanvasSbar)
			h.status.Draw(rc, h.state, 320, 48)
		}
	}
	rc.SetCanvas(renderer.CanvasCrosshair)
	h.crosshair.Draw(rc, h.state, h.screenWidth, h.screenHeight)
	rc.SetCanvas(renderer.CanvasDefault)
	h.centerprint.Draw(rc, h.state, h.screenWidth, h.screenHeight)
}

type canvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}

func setHUDCanvasParams(cv *cvar.CVarSystem, rc renderer.RenderContext, style HUDStyle, state State, screenWidth, screenHeight int) {
	setter, ok := rc.(canvasParamSetter)
	if !ok || screenWidth <= 0 || screenHeight <= 0 {
		return
	}

	sbarScale := float32(cv.FloatValue("scr_sbarscale"))
	if sbarScale <= 0 {
		sbarScale = 1
	}
	menuScale := float32(cv.FloatValue("scr_menuscale"))
	if menuScale <= 0 {
		menuScale = 1
	}
	crosshairScale := float32(cv.FloatValue("scr_crosshairscale"))
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
		VRect: renderer.ViewRect{
			X:      0,
			Y:      0,
			Width:  screenWidth,
			Height: screenHeight,
		},
		GameType: state.GameType,
		HudStyle: int(style),
	})
}

func currentViewSize(cv *cvar.CVarSystem) float64 {
	if c := cv.Get("viewsize"); c != nil && c.Float > 0 {
		return c.Float
	}
	if c := cv.Get("scr_viewsize"); c != nil && c.Float > 0 {
		return c.Float
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
