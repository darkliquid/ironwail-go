// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"math/bits"
	"sort"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// StatusBar renders the Quake-style status bar at the bottom of the screen.
type StatusBar struct {
	drawManager *draw.Manager
	palette     []byte
	sbarPic     *image.QPic
	ibarPic     *image.QPic
	scorebarPic *image.QPic
	rankingPic  *image.QPic
	discPic     *image.QPic
	weaponPics  [7][7]*image.QPic
	ammoPics    [4]*image.QPic
	rogueAmmo   [3]*image.QPic
	armorPics   [3]*image.QPic
	itemPics    [6]*image.QPic
	hipItemPics [2]*image.QPic
	rogueItems  [2]*image.QPic
	sigilPics   [4]*image.QPic
	hipWeapons  [7][5]*image.QPic
	rogueInvBar [2]*image.QPic
	rogueWeps   [5]*image.QPic
	facePics    [5][2]*image.QPic
	faceInvis   *image.QPic
	faceInvuln  *image.QPic
	faceBoth    *image.QPic
	faceQuad    *image.QPic
	lastItems   uint32
	pickupTimes [32]float64
	pickupKnown uint32
}

const (
	hipLaserCannonBit = 23
	hipMjolnirBit     = 7
	hipProximityBit   = 16
	hipWetsuitBit     = 25
	hipEmpathyBit     = 26

	rogueLavaNailgun      = 1 << 12
	rogueLavaSuperNailgun = 1 << 13
	rogueMultiGrenade     = 1 << 14
	rogueMultiRocket      = 1 << 15
	roguePlasmaGun        = 1 << 16
	rogueLavaNails        = 1 << 26
	roguePlasmaAmmo       = 1 << 27
	rogueMultiRockets     = 1 << 28
	rogueShield           = 1 << 29
	rogueAntiGrav         = 1 << 30
)

// NewStatusBar creates a new status bar renderer.
func NewStatusBar(dm *draw.Manager) *StatusBar {
	sb := &StatusBar{drawManager: dm}
	if dm != nil {
		sb.palette = dm.Palette()
		sb.sbarPic = dm.GetPic("sbar")
		sb.ibarPic = dm.GetPic("ibar")
		sb.scorebarPic = dm.GetPic("scorebar")
		sb.rankingPic = dm.GetPic("gfx/ranking.lmp")
		sb.discPic = dm.GetPic("disc")
		baseWeaponNames := [...]string{"shotgun", "sshotgun", "nailgun", "snailgun", "rlaunch", "srlaunch", "lightng"}
		for i, name := range baseWeaponNames {
			sb.weaponPics[0][i] = dm.GetPic("inv_" + name)
			sb.weaponPics[1][i] = dm.GetPic("inv2_" + name)
			for flash := 0; flash < 5; flash++ {
				sb.weaponPics[2+flash][i] = dm.GetPic("inva" + string('1'+rune(flash)) + "_" + name)
			}
		}
		sb.ammoPics = [4]*image.QPic{
			dm.GetPic("sb_shells"),
			dm.GetPic("sb_nails"),
			dm.GetPic("sb_rocket"),
			dm.GetPic("sb_cells"),
		}
		sb.rogueAmmo = [3]*image.QPic{
			dm.GetPic("r_ammolava"),
			dm.GetPic("r_ammomulti"),
			dm.GetPic("r_ammoplasma"),
		}
		sb.armorPics = [3]*image.QPic{
			dm.GetPic("sb_armor1"),
			dm.GetPic("sb_armor2"),
			dm.GetPic("sb_armor3"),
		}
		sb.itemPics = [6]*image.QPic{
			dm.GetPic("sb_key1"),
			dm.GetPic("sb_key2"),
			dm.GetPic("sb_invis"),
			dm.GetPic("sb_invuln"),
			dm.GetPic("sb_suit"),
			dm.GetPic("sb_quad"),
		}
		sb.hipItemPics = [2]*image.QPic{
			dm.GetPic("sb_wsuit"),
			dm.GetPic("sb_eshld"),
		}
		sb.rogueItems = [2]*image.QPic{
			dm.GetPic("r_shield1"),
			dm.GetPic("r_agrav1"),
		}
		sb.sigilPics = [4]*image.QPic{
			dm.GetPic("sb_sigil1"),
			dm.GetPic("sb_sigil2"),
			dm.GetPic("sb_sigil3"),
			dm.GetPic("sb_sigil4"),
		}
		sb.facePics = [5][2]*image.QPic{
			{dm.GetPic("face5"), dm.GetPic("face_p5")},
			{dm.GetPic("face4"), dm.GetPic("face_p4")},
			{dm.GetPic("face3"), dm.GetPic("face_p3")},
			{dm.GetPic("face2"), dm.GetPic("face_p2")},
			{dm.GetPic("face1"), dm.GetPic("face_p1")},
		}
		sb.faceInvis = dm.GetPic("face_invis")
		sb.faceInvuln = dm.GetPic("face_invul2")
		sb.faceBoth = dm.GetPic("face_inv2")
		sb.faceQuad = dm.GetPic("face_quad")
		sb.rogueInvBar = [2]*image.QPic{
			dm.GetPic("r_invbar1"),
			dm.GetPic("r_invbar2"),
		}
		sb.rogueWeps = [5]*image.QPic{
			dm.GetPic("r_lava"),
			dm.GetPic("r_superlava"),
			dm.GetPic("r_gren"),
			dm.GetPic("r_multirock"),
			dm.GetPic("r_plasma"),
		}
		hipNames := [...]string{"laser", "mjolnir", "gren_prox", "prox_gren", "prox"}
		for i, name := range hipNames {
			sb.hipWeapons[0][i] = dm.GetPic("inv_" + name)
			sb.hipWeapons[1][i] = dm.GetPic("inv2_" + name)
			for flash := 0; flash < 5; flash++ {
				sb.hipWeapons[2+flash][i] = dm.GetPic("inva" + string('1'+rune(flash)) + "_" + name)
			}
		}
	}
	return sb
}

// Draw renders a base-Quake style status bar and inventory strip.
func (sb *StatusBar) Draw(rc renderer.RenderContext, state State, screenWidth, screenHeight int) {
	if rc == nil {
		return
	}

	const sbarWidth = 320
	const sbarHeight = 24
	const inventoryHeight = 24

	sbarX := (screenWidth - sbarWidth) / 2
	sbarY := screenHeight - sbarHeight
	inventoryY := sbarY - inventoryHeight
	sb.trackPickups(state)

	if state.GameType == 1 && state.MaxClients > 1 && (state.ShowScores || state.Health <= 0) {
		sb.drawScoreboard(rc, state, sbarX, sbarY)
		return
	}

	if sb.sbarPic != nil {
		rc.DrawPic(sbarX, sbarY, sb.sbarPic)
	} else {
		rc.DrawFill(sbarX, sbarY, sbarWidth, sbarHeight, 4)
	}
	if sb.ibarPic != nil {
		rc.DrawPic(sbarX, inventoryY, sb.inventoryBarPic(state))
	} else {
		rc.DrawFill(sbarX, inventoryY, sbarWidth, inventoryHeight, 4)
	}
	sb.drawInventory(rc, sbarX, inventoryY, state)

	if pic := sb.armorPic(state.Items); pic != nil {
		rc.DrawPic(sbarX, sbarY, pic)
	}
	sb.drawBigNum(rc, sbarX+24, sbarY, armorValue(state), 3, armorValue(state) <= 25)

	if pic := sb.facePic(state); pic != nil {
		rc.DrawPic(sbarX+112, sbarY, pic)
	}
	sb.drawBigNum(rc, sbarX+136, sbarY, state.Health, 3, state.Health <= 25)

	if state.ModHipnotic {
		if state.Items&cl.ItemKey1 != 0 && sb.itemPics[0] != nil {
			rc.DrawPic(sbarX+209, sbarY+3, sb.itemPics[0])
		}
		if state.Items&cl.ItemKey2 != 0 && sb.itemPics[1] != nil {
			rc.DrawPic(sbarX+209, sbarY+12, sb.itemPics[1])
		}
	}

	if pic := sb.ammoPic(state); pic != nil {
		rc.DrawPic(sbarX+224, sbarY, pic)
	}
	sb.drawBigNum(rc, sbarX+248, sbarY, state.Ammo, 3, state.Ammo <= 10)

	if state.GameType == 1 && state.MaxClients > 1 {
		sb.drawMiniScoreboard(rc, state, sbarX, sbarY)
	}
}

func (sb *StatusBar) drawInventory(rc renderer.RenderContext, x, y int, state State) {
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
		flashOn := sb.weaponFlashIndex(state, bit, state.ActiveWeapon == int(bit))
		if pic := sb.weaponPic(i, flashOn); pic != nil {
			rc.DrawPic(x+i*24, y+8, pic)
		}
	}
	if state.ModHipnotic {
		sb.drawHipnoticWeapons(rc, x, y, state)
	}
	if state.ModRogue {
		sb.drawRogueWeapon(rc, x, y, state)
	}

	ammoCounts := []int{state.Shells, state.Nails, state.Rockets, state.Cells}
	for i, count := range ammoCounts {
		DrawNumber(rc, x+48*i+34, y, count, 3)
	}

	itemBits := []uint32{
		cl.ItemKey1,
		cl.ItemKey2,
		cl.ItemInvisibility,
		cl.ItemInvulnerability,
		cl.ItemSuit,
		cl.ItemQuad,
	}
	for i, bit := range itemBits {
		if state.ModHipnotic && i < 2 {
			continue
		}
		if state.Items&bit != 0 {
			if pic := sb.itemPics[i]; pic != nil {
				rc.DrawPic(x+192+i*16, y+8, pic)
			}
		}
	}
	if state.ModHipnotic {
		hipItemBits := []uint32{1 << hipWetsuitBit, 1 << hipEmpathyBit}
		for i, bit := range hipItemBits {
			if state.Items&bit == 0 {
				continue
			}
			if pic := sb.hipItemPics[i]; pic != nil {
				rc.DrawPic(x+288+i*16, y+8, pic)
			}
		}
	}
	if state.ModRogue {
		rogueBits := []uint32{rogueShield, rogueAntiGrav}
		for i, bit := range rogueBits {
			if state.Items&bit == 0 {
				continue
			}
			if pic := sb.rogueItems[i]; pic != nil {
				rc.DrawPic(x+288+i*16, y+8, pic)
			}
		}
		return
	}

	sigilBits := []uint32{cl.ItemSigil1, cl.ItemSigil2, cl.ItemSigil3, cl.ItemSigil4}
	for i, bit := range sigilBits {
		if state.Items&bit != 0 {
			if pic := sb.sigilPics[i]; pic != nil {
				rc.DrawPic(x+288+i*8, y+8, pic)
			}
		}
	}
}

func (sb *StatusBar) drawBigNum(rc renderer.RenderContext, x, y, value, digits int, alt bool) {
	DrawNumber(rc, x+digits*8, y, value, digits)
}

func (sb *StatusBar) facePic(state State) *image.QPic {
	items := state.Items
	if items&(cl.ItemInvisibility|cl.ItemInvulnerability) == (cl.ItemInvisibility | cl.ItemInvulnerability) {
		return sb.faceBoth
	}
	if items&cl.ItemQuad != 0 {
		return sb.faceQuad
	}
	if items&cl.ItemInvisibility != 0 {
		return sb.faceInvis
	}
	if items&cl.ItemInvulnerability != 0 {
		return sb.faceInvuln
	}
	bucket := state.Health / 20
	if state.Health >= 100 {
		bucket = 4
	}
	if bucket < 0 {
		bucket = 0
	}
	if bucket > 4 {
		bucket = 4
	}
	return sb.facePics[bucket][0]
}

func (sb *StatusBar) armorPic(items uint32) *image.QPic {
	if items&cl.ItemInvulnerability != 0 && sb.discPic != nil {
		return sb.discPic
	}
	switch {
	case items&cl.ItemArmor3 != 0:
		return sb.armorPics[2]
	case items&cl.ItemArmor2 != 0:
		return sb.armorPics[1]
	case items&cl.ItemArmor1 != 0:
		return sb.armorPics[0]
	default:
		return nil
	}
}

func (sb *StatusBar) ammoPic(state State) *image.QPic {
	items := state.Items
	if state.ModRogue {
		switch {
		case items&cl.ItemShells != 0:
			return sb.ammoPics[0]
		case items&cl.ItemNails != 0:
			return sb.ammoPics[1]
		case items&cl.ItemRockets != 0:
			return sb.ammoPics[2]
		case items&cl.ItemCells != 0:
			return sb.ammoPics[3]
		case items&rogueLavaNails != 0:
			return sb.rogueAmmo[0]
		case items&roguePlasmaAmmo != 0:
			return sb.rogueAmmo[1]
		case items&rogueMultiRockets != 0:
			return sb.rogueAmmo[2]
		default:
			return nil
		}
	}
	switch {
	case items&cl.ItemShells != 0:
		return sb.ammoPics[0]
	case items&cl.ItemNails != 0:
		return sb.ammoPics[1]
	case items&cl.ItemRockets != 0:
		return sb.ammoPics[2]
	case items&cl.ItemCells != 0:
		return sb.ammoPics[3]
	default:
		return nil
	}
}

func (sb *StatusBar) inventoryBarPic(state State) *image.QPic {
	if state.ModRogue {
		if state.ActiveWeapon < rogueLavaNailgun {
			if sb.rogueInvBar[1] != nil {
				return sb.rogueInvBar[1]
			}
		} else if sb.rogueInvBar[0] != nil {
			return sb.rogueInvBar[0]
		}
	}
	return sb.ibarPic
}

func (sb *StatusBar) weaponPic(slot, flashOn int) *image.QPic {
	if flashOn < 0 || flashOn >= len(sb.weaponPics) {
		flashOn = 1
	}
	if pic := sb.weaponPics[flashOn][slot]; pic != nil {
		return pic
	}
	if flashOn > 1 {
		if pic := sb.weaponPics[1][slot]; pic != nil {
			return pic
		}
	}
	if pic := sb.weaponPics[0][slot]; pic != nil {
		return pic
	}
	return sb.weaponPics[1][slot]
}

func (sb *StatusBar) weaponFlashIndex(state State, bit uint32, active bool) int {
	if !sb.pickedUp(bit) {
		if active {
			return 0
		}
		return 1
	}
	delta := state.Time - sb.pickupTimes[bits.TrailingZeros32(bit)]
	if delta >= 0 && delta < 1 {
		return (int(delta*10) % 5) + 2
	}
	if active {
		return 0
	}
	return 1
}

func (sb *StatusBar) trackPickups(state State) {
	added := state.Items &^ sb.lastItems
	removed := sb.lastItems &^ state.Items
	for i := 0; i < 32; i++ {
		bit := uint32(1) << i
		if added&bit != 0 {
			sb.pickupTimes[i] = state.Time
			sb.pickupKnown |= bit
		}
		if removed&bit != 0 {
			sb.pickupKnown &^= bit
			sb.pickupTimes[i] = 0
		}
	}
	sb.lastItems = state.Items
}

func (sb *StatusBar) pickedUp(bit uint32) bool {
	return sb.pickupKnown&bit != 0
}

func (sb *StatusBar) drawHipnoticWeapons(rc renderer.RenderContext, x, y int, state State) {
	hipBits := []uint32{1 << hipLaserCannonBit, 1 << hipMjolnirBit, cl.ItemGrenadeLauncher, 1 << hipProximityBit}
	grenadeFlashing := false
	for i, bit := range hipBits {
		if state.Items&bit == 0 {
			continue
		}
		flashOn := sb.weaponFlashIndex(state, bit, state.ActiveWeapon == int(bit))
		switch i {
		case 2:
			if state.Items&(1<<hipProximityBit) != 0 && flashOn > 1 {
				grenadeFlashing = true
				if pic := sb.hipWeapons[flashOn][2]; pic != nil {
					rc.DrawPic(x+96, y+8, pic)
				}
			}
		case 3:
			if state.Items&cl.ItemGrenadeLauncher != 0 {
				if !grenadeFlashing {
					idx := flashOn
					if idx == 0 {
						idx = 1
					}
					if pic := sb.hipWeapons[idx][3]; pic != nil {
						rc.DrawPic(x+96, y+8, pic)
					}
				}
			} else if pic := sb.hipWeapons[flashOn][4]; pic != nil {
				rc.DrawPic(x+96, y+8, pic)
			}
		default:
			if pic := sb.hipWeapons[flashOn][i]; pic != nil {
				rc.DrawPic(x+176+i*24, y+8, pic)
			}
		}
	}
}

func (sb *StatusBar) drawRogueWeapon(rc renderer.RenderContext, x, y int, state State) {
	if state.ActiveWeapon < rogueLavaNailgun {
		return
	}
	rogueActive := []int{
		rogueLavaNailgun,
		rogueLavaSuperNailgun,
		rogueMultiGrenade,
		rogueMultiRocket,
		roguePlasmaGun,
	}
	for i, weapon := range rogueActive {
		if state.ActiveWeapon == weapon && sb.rogueWeps[i] != nil {
			rc.DrawPic(x+(i+2)*24, y+8, sb.rogueWeps[i])
		}
	}
}

func armorValue(state State) int {
	if state.Items&cl.ItemInvulnerability != 0 {
		return 666
	}
	return state.Armor
}

func (sb *StatusBar) drawScoreboard(rc renderer.RenderContext, state State, sbarX, sbarY int) {
	const scorebarHeight = 24
	if sb.scorebarPic != nil {
		rc.DrawPic(sbarX, sbarY, sb.scorebarPic)
	} else {
		rc.DrawFill(sbarX, sbarY, 320, scorebarHeight, 4)
	}

	if sb.rankingPic != nil {
		rc.DrawPic(sbarX+(320-int(sb.rankingPic.Width))/2, 8, sb.rankingPic)
	}

	rows := sortedScoreboard(state.Scoreboard)
	y := 40
	for _, row := range rows {
		top := colorForMap(int(row.Colors & 0xf0))
		bottom := colorForMap(int((row.Colors & 0x0f) << 4))
		rowX := sbarX + 80
		rc.DrawFill(rowX, y, 40, 4, top)
		rc.DrawFill(rowX, y+4, 40, 4, bottom)
		DrawNumber(rc, rowX+32, y, row.Frags, 3)
		if row.IsCurrent {
			rc.DrawCharacter(rowX-8, y, int('>'))
		}
		DrawString(rc, rowX+64, y, row.Name)
		y += 10
	}
}

func (sb *StatusBar) drawMiniScoreboard(rc renderer.RenderContext, state State, sbarX, sbarY int) {
	rows := sortedScoreboard(state.Scoreboard)
	if len(rows) > 4 {
		rows = rows[:4]
	}
	x := sbarX + 184
	for _, row := range rows {
		top := colorForMap(int(row.Colors & 0xf0))
		bottom := colorForMap(int((row.Colors & 0x0f) << 4))
		rc.DrawFill(x+10, sbarY+1, 28, 4, top)
		rc.DrawFill(x+10, sbarY+5, 28, 3, bottom)
		DrawNumber(rc, x+28, sbarY, row.Frags, 3)
		if row.IsCurrent {
			rc.DrawCharacter(x+6, sbarY, 16)
			rc.DrawCharacter(x+32, sbarY, 17)
		}
		x += 32
	}
}

func sortedScoreboard(rows []ScoreEntry) []ScoreEntry {
	sorted := make([]ScoreEntry, 0, len(rows))
	for _, row := range rows {
		if row.Name == "" {
			continue
		}
		sorted = append(sorted, row)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Frags == sorted[j].Frags {
			return sorted[i].ClientIndex < sorted[j].ClientIndex
		}
		return sorted[i].Frags > sorted[j].Frags
	})
	return sorted
}

func colorForMap(m int) byte {
	return byte(m + 8)
}
