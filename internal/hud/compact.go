// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/renderer"
)

// CompactHUD renders an alternate, compact HUD overlay inspired by the Quake 64
// corner-based layout advertised in Ironwail's README.
//
// Instead of the classic bottom-strip status bar, the compact HUD places a
// minimal set of vital statistics in the screen corners so that the play-field
// is mostly unobstructed:
//
//   - Bottom-left:  Health (and armor when non-zero)
//   - Bottom-right: Current ammo
//   - Top-right:    Active weapon name (abbreviated)
//
// This mirrors the Q64 / non-classic overlay style from C Ironwail's sbar.c
// alternate rendering path and the hud_style feature listed in the README.
type CompactHUD struct{}

// NewCompactHUD returns a new compact HUD renderer.
func NewCompactHUD() *CompactHUD { return &CompactHUD{} }

const (
	compactMargin   = 4 // pixel gap from screen edge
	compactCharSize = 8 // Quake character glyph width/height
	compactScale    = 2 // scale factor for big corner numbers
)

// Draw renders the compact overlay onto rc.
func (c *CompactHUD) Draw(rc renderer.RenderContext, state State, screenWidth, screenHeight int) {
	if rc == nil {
		return
	}

	// ---- bottom-left: health (and armor) ----
	healthStr := fmt.Sprintf("%3d", state.Health)
	drawCompactString(rc, compactMargin, screenHeight-compactCharSize-compactMargin, healthStr)

	if state.Armor > 0 {
		armorStr := fmt.Sprintf("A:%d", state.Armor)
		drawCompactString(rc, compactMargin, screenHeight-compactCharSize*2-compactMargin*2, armorStr)
	}

	// ---- bottom-right: ammo ----
	ammo := currentAmmo(state)
	if ammo >= 0 {
		ammoStr := fmt.Sprintf("%3d", ammo)
		ammoX := screenWidth - len(ammoStr)*compactCharSize - compactMargin
		drawCompactString(rc, ammoX, screenHeight-compactCharSize-compactMargin, ammoStr)
	}

	// ---- top-right: weapon name ----
	weapon := compactWeaponName(state.ActiveWeapon)
	if weapon != "" {
		wX := screenWidth - len(weapon)*compactCharSize - compactMargin
		drawCompactString(rc, wX, compactMargin, weapon)
	}
}

// drawCompactString draws text using standard Quake character glyphs (no scale).
func drawCompactString(rc renderer.RenderContext, x, y int, s string) {
	for i, ch := range s {
		rc.DrawCharacter(x+i*compactCharSize, y, int(ch))
	}
}

// currentAmmo returns the ammo count for the active weapon, or -1 if unknown.
func currentAmmo(state State) int {
	switch state.ActiveWeapon {
	case 2, 3: // shotgun / super shotgun
		return state.Shells
	case 4, 5: // nailgun / super nailgun
		return state.Nails
	case 6, 7: // rocket launcher / grenade launcher
		return state.Rockets
	case 8: // lightning gun
		return state.Cells
	default:
		if state.Ammo > 0 {
			return state.Ammo
		}
		return -1
	}
}

// compactWeaponName returns a short uppercase name for the given weapon impulse
// number, matching the item bit layout used in sbar.c.
func compactWeaponName(weapon int) string {
	switch weapon {
	case 1:
		return "AXE"
	case 2:
		return "SG"
	case 3:
		return "SSG"
	case 4:
		return "NG"
	case 5:
		return "SNG"
	case 6:
		return "GL"
	case 7:
		return "RL"
	case 8:
		return "LG"
	default:
		return ""
	}
}
