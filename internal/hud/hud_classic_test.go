// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

// Classic status bar, rogue, compact, and HUD style tests split from hud_test.go.

import (
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func TestStatusBarDrawsClassicIconsFromState(t *testing.T) {
	weaponOwned := &image.QPic{Width: 24, Height: 16}
	weaponActive := &image.QPic{Width: 24, Height: 16}
	itemPic := &image.QPic{Width: 16, Height: 16}
	sigilPic := &image.QPic{Width: 8, Height: 16}
	facePic := &image.QPic{Width: 24, Height: 24}
	armorPic := &image.QPic{Width: 24, Height: 24}
	ammoPic := &image.QPic{Width: 24, Height: 24}
	sbarPic := &image.QPic{Width: 320, Height: 24}
	ibarPic := &image.QPic{Width: 320, Height: 24}

	sb := &StatusBar{
		cvars:      testCV,
		sbarPic:    sbarPic,
		ibarPic:    ibarPic,
		weaponPics: [7][7]*image.QPic{{weaponActive}, {weaponOwned}},
		itemPics:   [6]*image.QPic{itemPic},
		sigilPics:  [4]*image.QPic{sigilPic},
		facePics:   [5][2]*image.QPic{{facePic}, {facePic}, {facePic}, {facePic}, {facePic}},
		armorPics:  [3]*image.QPic{armorPic},
		ammoPics:   [4]*image.QPic{ammoPic},
	}
	mock := &mockRenderContext{}
	state := State{
		Health:       100,
		Armor:        40,
		Ammo:         20,
		ActiveWeapon: int(cl.ItemShotgun),
		Shells:       20,
		Nails:        30,
		Rockets:      40,
		Cells:        50,
		Items:        1 | (1 << 8) | (1 << 13) | (1 << 17) | (1 << 28),
	}
	sb.Draw(&mockRenderContext{}, state, 320, 200)
	state.Time = 2.2

	sb.Draw(mock, state, 320, 200)

	if len(mock.pics) < 7 {
		t.Fatalf("expected several icon pic draws, got %d", len(mock.pics))
	}

	var sawWeapon, sawActiveWeapon, sawItem, sawSigil, sawFace, sawArmor, sawAmmo bool
	for _, draw := range mock.pics {
		switch draw.pic {
		case weaponOwned:
			sawWeapon = true
		case weaponActive:
			sawWeapon = true
			sawActiveWeapon = true
		case itemPic:
			sawItem = true
		case sigilPic:
			sawSigil = true
		case facePic:
			sawFace = true
		case armorPic:
			sawArmor = true
		case ammoPic:
			sawAmmo = true
		}
	}
	if !sawWeapon || !sawActiveWeapon || !sawItem || !sawSigil || !sawFace || !sawArmor || !sawAmmo {
		t.Fatalf("missing expected draws: weapon=%v activeWeapon=%v item=%v sigil=%v face=%v armor=%v ammo=%v", sawWeapon, sawActiveWeapon, sawItem, sawSigil, sawFace, sawArmor, sawAmmo)
	}
}

func TestStatusBarRogueItemsReplaceSigils(t *testing.T) {
	rogueShieldPic := &image.QPic{Width: 16, Height: 16}
	rogueAntiPic := &image.QPic{Width: 16, Height: 16}
	sigilPic := &image.QPic{Width: 8, Height: 16}
	sb := &StatusBar{
		cvars:      testCV,
		sbarPic:    &image.QPic{Width: 320, Height: 24},
		ibarPic:    &image.QPic{Width: 320, Height: 24},
		rogueItems: [2]*image.QPic{rogueShieldPic, rogueAntiPic},
		sigilPics:  [4]*image.QPic{sigilPic, sigilPic, sigilPic, sigilPic},
	}
	mock := &mockRenderContext{}
	sb.Draw(mock, State{
		Health:     100,
		Ammo:       20,
		ModRogue:   true,
		Items:      rogueShield | rogueAntiGrav,
		Time:       10,
		GameType:   0,
		MaxClients: 1,
	}, 320, 200)
	var sawRogue, sawSigil bool
	for _, draw := range mock.pics {
		if draw.pic == rogueShieldPic || draw.pic == rogueAntiPic {
			sawRogue = true
		}
		if draw.pic == sigilPic {
			sawSigil = true
		}
	}
	if !sawRogue {
		t.Fatal("expected rogue expansion item icons")
	}
	if sawSigil {
		t.Fatal("expected rogue item path to suppress sigils")
	}
}

func TestStatusBarRogueArmorUsesRogueArmorBits(t *testing.T) {
	greenArmor := &image.QPic{Width: 24, Height: 24}
	yellowArmor := &image.QPic{Width: 24, Height: 24}
	redArmor := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{
		cvars:     testCV,
		sbarPic:   &image.QPic{Width: 320, Height: 24},
		ibarPic:   &image.QPic{Width: 320, Height: 24},
		armorPics: [3]*image.QPic{greenArmor, yellowArmor, redArmor},
	}
	mock := &mockRenderContext{}

	sb.Draw(mock, State{
		Health:   100,
		Armor:    150,
		Ammo:     20,
		ModRogue: true,
		Items:    rogueArmor2 | cl.ItemShells,
	}, 320, 200)

	var sawYellow bool
	for _, draw := range mock.pics {
		if draw.pic == yellowArmor {
			sawYellow = true
			break
		}
	}
	if !sawYellow {
		t.Fatal("expected rogue armor bits to select the matching armor icon")
	}
}

func TestStatusBarWeaponPickupFlashTiming(t *testing.T) {
	active := &image.QPic{Width: 24, Height: 16}
	owned := &image.QPic{Width: 24, Height: 16}
	flash := &image.QPic{Width: 24, Height: 16}
	sb := &StatusBar{
		cvars:   testCV,
		sbarPic: &image.QPic{Width: 320, Height: 24},
		ibarPic: &image.QPic{Width: 320, Height: 24},
		weaponPics: [7][7]*image.QPic{
			{active},
			{owned},
			{flash},
			{flash},
			{flash},
			{flash},
			{flash},
		},
	}
	sb.Draw(&mockRenderContext{}, State{Time: 1}, 320, 200)
	flashFrame := &mockRenderContext{}
	sb.Draw(flashFrame, State{
		Time:  1.1,
		Items: cl.ItemShotgun,
	}, 320, 200)
	var sawFlash bool
	for _, draw := range flashFrame.pics {
		if draw.pic == flash {
			sawFlash = true
		}
	}
	if !sawFlash {
		t.Fatal("expected flashing weapon frame right after pickup")
	}

	steady := &mockRenderContext{}
	sb.Draw(steady, State{
		Time:  2.2,
		Items: cl.ItemShotgun,
	}, 320, 200)
	var sawOwned bool
	for _, draw := range steady.pics {
		if draw.pic == owned {
			sawOwned = true
		}
	}
	if !sawOwned {
		t.Fatal("expected non-flashing owned weapon frame after flash window")
	}
}

func TestStatusBarDrawMiniScoreboardForDeathmatch(t *testing.T) {
	sb := NewStatusBar(nil, testCV)
	mock := &mockRenderContext{}
	const screenWidth = 320
	const screenHeight = 200
	const sbarY = screenHeight - 24
	sb.Draw(mock, State{
		Health:     100,
		Armor:      50,
		Ammo:       30,
		GameType:   1,
		MaxClients: 4,
		Scoreboard: []ScoreEntry{
			{Name: "alpha", Frags: 2, Colors: 0x1f},
			{Name: "bravo", Frags: 9, Colors: 0x2e, IsCurrent: true},
		},
	}, screenWidth, screenHeight)
	if len(mock.fills) < 6 {
		t.Fatalf("expected status bar and mini scoreboard fills, got %d", len(mock.fills))
	}
	var sawMiniTop, sawMiniBottom, sawTopAnchoredMini bool
	for _, f := range mock.fills {
		if f.x == 194 && f.w == 28 && f.h == 4 && f.y == sbarY+1 {
			sawMiniTop = true
		}
		if f.x == 194 && f.w == 28 && f.h == 3 && f.y == sbarY+5 {
			sawMiniBottom = true
		}
		if f.x == 194 && (f.y == 1 || f.y == 5) {
			sawTopAnchoredMini = true
		}
	}
	if !sawMiniTop || !sawMiniBottom {
		t.Fatalf("expected mini scoreboard fills anchored to status bar y=%d; top=%v bottom=%v", sbarY, sawMiniTop, sawMiniBottom)
	}
	if sawTopAnchoredMini {
		t.Fatalf("mini scoreboard still appears top-anchored")
	}
}

func TestStatusBarFaceUsesPainFrameDuringDamageWindow(t *testing.T) {
	idleFace := &image.QPic{Width: 24, Height: 24}
	painFace := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{
		cvars:   testCV,
		sbarPic: &image.QPic{Width: 320, Height: 24},
		ibarPic: &image.QPic{Width: 320, Height: 24},
	}
	sb.facePics[3][0] = idleFace
	sb.facePics[3][1] = painFace

	mock := &mockRenderContext{}
	sb.Draw(mock, State{
		Health:        70,
		Time:          5,
		FaceAnimUntil: 5.2,
	}, 320, 200)

	var sawPain bool
	for _, draw := range mock.pics {
		if draw.pic == painFace {
			sawPain = true
		}
		if draw.pic == idleFace {
			t.Fatal("expected pain frame during damage animation window")
		}
	}
	if !sawPain {
		t.Fatal("expected pain face draw during damage animation window")
	}
}

func TestStatusBarFacePowerupOverridesPainFrame(t *testing.T) {
	painFace := &image.QPic{Width: 24, Height: 24}
	quadFace := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{
		cvars:    testCV,
		sbarPic:  &image.QPic{Width: 320, Height: 24},
		ibarPic:  &image.QPic{Width: 320, Height: 24},
		faceQuad: quadFace,
	}
	sb.facePics[3][1] = painFace

	mock := &mockRenderContext{}
	sb.Draw(mock, State{
		Health:        70,
		Items:         cl.ItemQuad,
		Time:          5,
		FaceAnimUntil: 5.2,
	}, 320, 200)

	var sawQuad bool
	for _, draw := range mock.pics {
		if draw.pic == quadFace {
			sawQuad = true
		}
		if draw.pic == painFace {
			t.Fatal("expected quad face to override pain frame")
		}
	}
	if !sawQuad {
		t.Fatal("expected quad face draw")
	}
}

func TestStatusBarDrawScoreboardOverlayWhenHeld(t *testing.T) {
	sb := NewStatusBar(nil, testCV)
	mock := &mockRenderContext{}
	sb.Draw(mock, State{
		Health:     100,
		GameType:   1,
		MaxClients: 2,
		ShowScores: true,
		Scoreboard: []ScoreEntry{
			{Name: "alpha", Frags: 2, Colors: 0x1f},
			{Name: "bravo", Frags: 9, Colors: 0x2e, IsCurrent: true},
		},
	}, 320, 200)
	if got := charactersToString(mock.characters); !strings.Contains(got, "bravo") {
		t.Fatalf("expected scoreboard name draw, got %q", got)
	}
}

// ---- CompactHUD tests ----

// TestCompactHUDDrawsHealthBottomLeft verifies that the compact HUD renders the
// player health in the bottom-left corner using DrawCharacter calls.
func TestCompactHUDDrawsHealthBottomLeft(t *testing.T) {
	c := NewCompactHUD()
	rc := &mockRenderContext{}

	state := State{Health: 75, ActiveWeapon: 2, Shells: 30}
	c.Draw(rc, state, 640, 480)

	if len(rc.characters) == 0 {
		t.Fatal("expected DrawCharacter calls for compact HUD, got none")
	}

	// Health is drawn at the bottom-left; the first chars should be at low X, near bottom.
	bottomY := 480 - compactCharSize - compactMargin
	found := false
	for _, ch := range rc.characters {
		if ch.y == bottomY {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a character drawn at y=%d (bottom-left health), none found; chars: %v", bottomY, rc.characters)
	}
}

// TestCompactHUDDrawsAmmoBottomRight verifies that ammo is drawn near the right
// side of the screen.
func TestCompactHUDDrawsAmmoBottomRight(t *testing.T) {
	c := NewCompactHUD()
	rc := &mockRenderContext{}

	state := State{Health: 100, ActiveWeapon: 4, Nails: 50}
	c.Draw(rc, state, 640, 480)

	// The ammo string "  50" (right-aligned, 3 chars wide) starts at
	// x = 640 - 3*8 - 4 = 612. Verify some char is near the right edge.
	rightX := 640 - 4*compactCharSize - compactMargin
	found := false
	for _, ch := range rc.characters {
		if ch.x >= rightX {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a character near right edge (x>=%d) for ammo, none found", rightX)
	}
}

// TestCompactHUDNilRenderContextNoPanic verifies that Draw with a nil context does
// not panic.
func TestCompactHUDNilRenderContextNoPanic(t *testing.T) {
	c := NewCompactHUD()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked with nil RenderContext: %v", r)
		}
	}()
	c.Draw(nil, State{Health: 50}, 640, 480)
}

func TestCompactHUDSupportsBitmaskActiveWeapon(t *testing.T) {
	state := State{
		ActiveWeapon: int(cl.ItemRocketLauncher),
		Rockets:      12,
	}
	if got := currentAmmo(state); got != 12 {
		t.Fatalf("currentAmmo(bitmask RL) = %d, want 12", got)
	}
	if got := compactWeaponName(int(cl.ItemRocketLauncher)); got != "RL" {
		t.Fatalf("compactWeaponName(bitmask RL) = %q, want RL", got)
	}
}

// TestHUDStyleSwitchesRenderer verifies that hud.Draw dispatches to the compact
// renderer when hud_style=1.
func TestHUDStyleSwitchesRenderer(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 42, ActiveWeapon: 2, Shells: 10})

	rc := &mockRenderContext{}

	// Classic style: status bar draws pics (from sbar.go); with nil draw manager
	// it falls through to DrawFill calls. Reset and count.
	_ = strings.Contains // keep import
	_ = cl.Client{}      // keep import
	testCV.Set("hud_style", "0")
	h.Draw(rc)
	classicCalls := len(rc.characters) + len(rc.fills)

	// Compact style: only DrawCharacter calls.
	rc2 := &mockRenderContext{}
	testCV.Set("hud_style", "1")
	h.Draw(rc2)
	compactCalls := len(rc2.characters)

	// Both should produce output.
	if classicCalls == 0 && compactCalls == 0 {
		t.Fatal("both styles produced no output")
	}
	// Compact should not use fills (no status bar background).
	if len(rc2.fills) != 0 {
		t.Errorf("compact HUD should not use DrawFill, got %d calls", len(rc2.fills))
	}
}

func TestHUDDrawUsesParityCanvases(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 100})
	setTestViewSize(t, "100")

	classic := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 320, Bottom: 48},
	}
	testCV.Set("hud_style", "0")
	h.Draw(classic)
	if len(classic.canvasSwitch) == 0 || classic.canvasSwitch[0] != renderer.CanvasSbar {
		t.Fatalf("classic HUD first canvas = %v, want %v", classic.canvasSwitch, renderer.CanvasSbar)
	}
	if len(classic.canvasSwitch) < 2 || classic.canvasSwitch[len(classic.canvasSwitch)-2] != renderer.CanvasCrosshair {
		t.Fatalf("classic HUD canvas switches = %v, want penultimate %v", classic.canvasSwitch, renderer.CanvasCrosshair)
	}
	if classic.canvas.Type != renderer.CanvasDefault {
		t.Fatalf("final classic canvas = %v, want %v", classic.canvas.Type, renderer.CanvasDefault)
	}
	if len(classic.fills) == 0 {
		t.Fatal("classic HUD drew nothing")
	}

	compact := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 400, Bottom: 225},
	}
	testCV.Set("hud_style", "1")
	h.Draw(compact)
	if len(compact.canvasSwitch) == 0 || compact.canvasSwitch[0] != renderer.CanvasSbar2 {
		t.Fatalf("compact HUD first canvas = %v, want %v", compact.canvasSwitch, renderer.CanvasSbar2)
	}
	if len(compact.canvasSwitch) < 2 || compact.canvasSwitch[len(compact.canvasSwitch)-2] != renderer.CanvasCrosshair {
		t.Fatalf("compact HUD canvas switches = %v, want penultimate %v", compact.canvasSwitch, renderer.CanvasCrosshair)
	}
	if compact.canvas.Type != renderer.CanvasDefault {
		t.Fatalf("final compact canvas = %v, want %v", compact.canvas.Type, renderer.CanvasDefault)
	}
	if len(compact.characters) == 0 {
		t.Fatal("compact HUD drew nothing")
	}

	quakeWorld := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 320, Bottom: 48},
	}
	testCV.Set("hud_style", "2")
	h.SetState(State{
		Health:       100,
		Armor:        50,
		Ammo:         30,
		Shells:       20,
		Nails:        40,
		Rockets:      10,
		Cells:        5,
		Items:        cl.ItemShotgun | cl.ItemQuad,
		ActiveWeapon: int(cl.ItemShotgun),
		GameType:     1,
		MaxClients:   2,
		Scoreboard: []ScoreEntry{
			{Name: "alpha", Frags: 2, Colors: 0x1f},
			{Name: "bravo", Frags: 9, Colors: 0x2e, IsCurrent: true},
		},
	})
	h.Draw(quakeWorld)
	if len(quakeWorld.canvasSwitch) < 4 {
		t.Fatalf("quakeworld HUD canvas switches = %v, want QW inventory/frag canvases", quakeWorld.canvasSwitch)
	}
	if quakeWorld.canvasSwitch[0] != renderer.CanvasSbar {
		t.Fatalf("quakeworld HUD first canvas = %v, want %v", quakeWorld.canvasSwitch, renderer.CanvasSbar)
	}
	var sawQWInv bool
	for _, ct := range quakeWorld.canvasSwitch {
		if ct == renderer.CanvasSbarQWInv {
			sawQWInv = true
			break
		}
	}
	if !sawQWInv {
		t.Fatalf("quakeworld HUD never switched to %v: %v", renderer.CanvasSbarQWInv, quakeWorld.canvasSwitch)
	}
	if quakeWorld.canvasParams.HudStyle != int(HUDStyleQuakeWorld) {
		t.Fatalf("quakeworld HUD canvas params style = %d, want %d", quakeWorld.canvasParams.HudStyle, HUDStyleQuakeWorld)
	}
	if quakeWorld.canvasParams.GameType != 1 {
		t.Fatalf("quakeworld HUD canvas params gametype = %d, want 1", quakeWorld.canvasParams.GameType)
	}
}

func TestQuakeWorldHUDHidesFragStripAtLargeViewsize(t *testing.T) {
	sb := NewStatusBar(nil, testCV)
	mock := &mockRenderContext{}
	setTestViewSize(t, "115")
	sb.DrawQuakeWorld(mock, State{
		Health:       100,
		Armor:        50,
		Ammo:         30,
		Shells:       20,
		Items:        cl.ItemShotgun,
		ActiveWeapon: int(cl.ItemShotgun),
		GameType:     1,
		MaxClients:   2,
		Scoreboard: []ScoreEntry{
			{Name: "alpha", Frags: 2, Colors: 0x1f},
			{Name: "bravo", Frags: 9, Colors: 0x2e, IsCurrent: true},
		},
	}, 320, 200)

	for _, f := range mock.fills {
		if f.y == 0 || f.y == 4 {
			t.Fatalf("unexpected QW frag-strip fill at scr_viewsize 115: %+v", f)
		}
	}
}

func TestCompactHUDHidesAtHugeViewsize(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 100, ActiveWeapon: 2, Shells: 10})

	rc := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 400, Bottom: 225},
	}
	testCV.Set("hud_style", "1")
	setTestViewSize(t, "120")
	h.Draw(rc)

	if len(rc.characters) != 0 || len(rc.fills) != 0 || len(rc.pics) != 0 {
		t.Fatalf("compact HUD should hide at scr_viewsize 120, got chars=%d fills=%d pics=%d", len(rc.characters), len(rc.fills), len(rc.pics))
	}
}
