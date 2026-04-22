// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"fmt"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// sbar2MarginX / sbar2MarginY mirror the SBAR2_MARGIN_X / SBAR2_MARGIN_Y
// constants from C Ironwail's sbar.c — the inset (in logical pixels) that
// the modern HUD styles use when anchoring widgets to the canvas edges.
const (
	sbar2MarginX = 16
	sbar2MarginY = 10
)

// DrawModern renders the SBAR2-anchored "modern" HUD used by hud_style 1
// (centered 4x1 ammo strip) and 2 (side-anchored 2x2 ammo block). It mirrors
// the else-branch of C Ironwail's Sbar_Draw combined with Sbar_DrawInventory2,
// placing face+health in the lower-left, ammo in the lower-right, and the
// weapon/item column along the right edge.
//
// The caller is expected to switch to CanvasSbar2 before invoking DrawModern;
// the method reads canvas bounds via rc.Canvas() to anchor relative to the
// visible play-field edges.
func (sb *StatusBar) DrawModern(rc renderer.RenderContext, state State, sideAmmo bool) {
	if rc == nil {
		return
	}
	sb.trackPickups(state)

	canvas := rc.Canvas()
	left := int(canvas.Left)
	right := int(canvas.Right)
	bottom := int(canvas.Bottom)
	// Canvas bounds are reported as -half..+half-style NDC-ish values in some
	// configurations; guard against degenerate sizing.
	if right <= left || bottom <= int(canvas.Top) {
		return
	}

	viewSize := currentViewSize(sb.cvars)
	if state.GameType == 1 && state.MaxClients > 1 && (state.ShowScores || state.Health <= 0) {
		// Scoreboard takes precedence when requested or after death.
		sb.drawScoreboard(rc, state, (right+left)/2-160, bottom-48)
		return
	}
	if viewSize >= 120 {
		return
	}

	invuln := state.Items&cl.ItemInvulnerability != 0
	armor := armorValue(state)
	if invuln {
		armor = 666
	}
	alpha := sb.currentSbarAlpha()

	// ---- lower-left: face + health (+ armor above when present) ----
	x := left + sbar2MarginX
	y := bottom - sbar2MarginY - 48
	if pic := sb.facePic(state); pic != nil {
		rc.DrawPic(x, y, pic)
	}
	sb.drawBigNum(rc, x+32, y, state.Health, 3, state.Health <= 25)

	if armor > 0 {
		sb.drawBigNum(rc, x+32, y-24, armor, 3, invuln || armor <= 25)
		if pic := sb.armorPic(state); pic != nil {
			rc.DrawPic(x, y-24, pic)
		}
	}

	// ---- lower-right: ammo icon + main ammo count ----
	x = right - sbar2MarginX - 24
	if pic := sb.ammoPic(state); pic != nil {
		rc.DrawPic(x, y, pic)
		x -= 32
	}
	sb.drawBigNum(rc, x-48, y, state.Ammo, 3, state.Ammo <= 10)

	// ---- weapon / item column + ammo strip ----
	sb.drawModernInventory(rc, state, sideAmmo, alpha)

	if state.GameType == 1 && state.MaxClients > 1 {
		sb.drawModernFrags(rc, state)
	}
}

// drawModernInventory ports Sbar_DrawInventory2: the weapon column on the
// right edge, the shared ammo strip (centered 4x1 or side-anchored 2x2), and
// the item column (keys, powerups) on the right. Hipnotic / Rogue weapon and
// item additions are honoured alongside the base set.
func (sb *StatusBar) drawModernInventory(rc renderer.RenderContext, state State, sideAmmo bool, alpha float32) {
	canvas := rc.Canvas()
	left := int(canvas.Left)
	right := int(canvas.Right)
	top := int(canvas.Top)
	bottom := int(canvas.Bottom)

	viewSize := currentViewSize(sb.cvars)

	// Weapon column along the right edge (visible only at viewSize < 110,
	// matching C sbar.c's guard).
	if viewSize < 110 {
		const rowHeight = 16
		wx := right + 1
		wy := (top+bottom-148)/2 + rowHeight*7/2
		if state.ModHipnotic {
			wy += 12
		}
		weaponBits := []uint32{
			cl.ItemShotgun,
			cl.ItemSuperShotgun,
			cl.ItemNailgun,
			cl.ItemSuperNailgun,
			cl.ItemGrenadeLauncher,
			cl.ItemRocketLauncher,
			cl.ItemLightning,
		}
		for i, bit := range weaponBits {
			if state.Items&bit == 0 {
				continue
			}
			active := state.ActiveWeapon == int(bit)
			flashOn := sb.weaponFlashIndex(state, bit, active)
			if pic := sb.weaponPic(i, flashOn); pic != nil {
				dx := wx - 18
				if active {
					dx = wx - 24
				}
				rc.DrawPic(dx, wy-rowHeight*i, pic)
			}
		}
	}

	// Ammo strip.
	if viewSize < 110 {
		bar := sb.inventoryBarPic(state)
		ammoCounts := [4]int{state.Shells, state.Nails, state.Rockets, state.Cells}
		if sideAmmo {
			// 2x2 block anchored to the lower-right, above the main ammo.
			const itemWidth = 52
			ax := right - sbar2MarginX - itemWidth*2
			ay := bottom - sbar2MarginY - 60
			if bar != nil {
				// Replicate Draw_SubPic's (i * 2*48/320, 0, 2*48/320, 10/24) source
				// window by slicing twice (one slice per row pair).
				for i := 0; i < 2; i++ {
					srcX := i * (2 * 48)
					if sub := bar.SubPic(srcX, 0, 2*48, 10); sub != nil {
						sb.drawPicAlpha(rc, ax, ay+24-10*i, sub, alpha)
					}
				}
			}
			for i, count := range ammoCounts {
				sb.drawSmallNum(rc, ax+11+itemWidth*(i&1), ay-10*(i>>1), count)
			}
		} else {
			// 4x1 centered strip near the bottom.
			ax := (right+left)/2 - 96
			ay := bottom - 9
			if bar != nil {
				if sub := bar.SubPic(0, 0, 192, 10); sub != nil {
					sb.drawPicAlpha(rc, ax, ay, sub, alpha)
				}
			}
			for i, count := range ammoCounts {
				sb.drawSmallNum(rc, ax+10+48*i, ay-24, count)
			}
		}
	}

	// Item column (keys, powerups).
	var ix, iy int
	if viewSize < 110 && sideAmmo {
		ix = right - sbar2MarginX - 16
		iy = bottom - sbar2MarginY - 68 - 20
	} else {
		ix = right - sbar2MarginX - 20
		iy = bottom - sbar2MarginY - 68
	}

	itemBits := []uint32{
		cl.ItemKey1,
		cl.ItemKey2,
		cl.ItemInvisibility,
		cl.ItemInvulnerability,
		cl.ItemSuit,
		cl.ItemQuad,
	}
	for i := 0; i < 6; i++ {
		if i == 2 {
			if viewSize >= 110 {
				break // mini-HUD: only keys
			}
			ix = left + sbar2MarginX + 4
			iy = bottom - sbar2MarginY - 66
			if state.Items&cl.ItemInvulnerability != 0 || state.Armor > 0 {
				iy -= 24 // armor row takes space
			}
		}
		bit := itemBits[i]
		// Hipnotic replaces the first two key slots with its own key icons
		// (drawn separately below).
		if state.ModHipnotic && i < 2 {
			continue
		}
		if state.Items&bit != 0 && sb.itemPics[i] != nil {
			rc.DrawPic(ix, iy, sb.itemPics[i])
			iy -= 16
		}
	}

	if state.ModHipnotic && viewSize < 110 {
		hipItemBits := []uint32{1 << hipWetsuitBit, 1 << hipEmpathyBit}
		for i, bit := range hipItemBits {
			if state.Items&bit == 0 || sb.hipItemPics[i] == nil {
				continue
			}
			rc.DrawPic(ix, iy, sb.hipItemPics[i])
			iy -= 16
		}
	}
	if state.ModRogue && viewSize < 110 {
		rogueBits := []uint32{rogueShield, rogueAntiGrav}
		for i, bit := range rogueBits {
			if state.Items&bit == 0 || sb.rogueItems[i] == nil {
				continue
			}
			rc.DrawPic(ix, iy, sb.rogueItems[i])
			iy -= 16
		}
	}
}

// drawSmallNum draws a small 3-digit number using the character glyphs
// 18..27 (yellow numerals), matching C Ironwail's Sbar_DrawSmallNum.
func (sb *StatusBar) drawSmallNum(rc renderer.RenderContext, x, y, num int) {
	if rc == nil {
		return
	}
	if num < 0 {
		num = 0
	}
	if num > 999 {
		num = 999
	}
	str := fmt.Sprintf("%3d", num)
	for i := 0; i < 3; i++ {
		ch := str[i]
		if ch == ' ' {
			continue
		}
		rc.DrawCharacter(x+i*8, y, 18+int(ch-'0'))
	}
}

// drawModernFrags renders the deathmatch frag strip along the bottom edge.
// This is a simplified port of Sbar_DrawFrags2 — it shows the top-N players'
// frag counts as small numerals, anchored to the bottom center.
func (sb *StatusBar) drawModernFrags(rc renderer.RenderContext, state State) {
	if len(state.Scoreboard) == 0 {
		return
	}
	canvas := rc.Canvas()
	left := int(canvas.Left)
	right := int(canvas.Right)
	bottom := int(canvas.Bottom)

	// Draw up to 4 top frag entries as small numerals centered near the
	// bottom, leaving room for the ammo strip above.
	const maxEntries = 4
	limit := len(state.Scoreboard)
	if limit > maxEntries {
		limit = maxEntries
	}
	totalWidth := limit * 32
	x := (right+left)/2 - totalWidth/2
	y := bottom - sbar2MarginY - 58
	for i := 0; i < limit; i++ {
		entry := state.Scoreboard[i]
		sb.drawSmallNum(rc, x+i*32, y, entry.Frags)
	}
}
