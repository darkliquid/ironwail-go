// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"math"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

func registerCenterprintTestCvars() {
	cvar.Register("scr_centerprintbg", "2", cvar.FlagArchive, "test centerprint background")
	cvar.Register("con_notifyfade", "0", cvar.FlagArchive, "test centerprint fade enable")
	cvar.Register("con_notifyfadetime", "0.5", cvar.FlagArchive, "test centerprint fade duration")
}

func setTestViewSize(t *testing.T, value string) {
	t.Helper()
	cvar.Set("scr_viewsize", value)
	t.Cleanup(func() {
		cvar.Set("scr_viewsize", "100")
	})
}

// mockRenderContext is a test double for renderer.RenderContext
type mockRenderContext struct {
	characters      []struct{ x, y, num int }
	alphaCharacters []struct {
		x, y, num int
		alpha     float32
	}
	pics []struct {
		x, y int
		pic  *image.QPic
	}
	menuPics []struct {
		x, y int
		pic  *image.QPic
	}
	fills []struct {
		x, y, w, h int
		color      byte
	}
	alphaFills []struct {
		x, y, w, h int
		color      byte
		alpha      float32
	}
	canvas       renderer.CanvasState
	canvasSwitch []renderer.CanvasType
	canvasParams renderer.CanvasTransformParams
}

func (m *mockRenderContext) Clear(r, g, b, a float32)        {}
func (m *mockRenderContext) DrawTriangle(r, g, b, a float32) {}
func (m *mockRenderContext) SurfaceView() interface{}        { return nil }
func (m *mockRenderContext) Gamma() float32                  { return 1.0 }
func (m *mockRenderContext) DrawPic(x, y int, pic *image.QPic) {
	m.pics = append(m.pics, struct {
		x, y int
		pic  *image.QPic
	}{x, y, pic})
}
func (m *mockRenderContext) DrawMenuPic(x, y int, pic *image.QPic) {
	m.menuPics = append(m.menuPics, struct {
		x, y int
		pic  *image.QPic
	}{x, y, pic})
}
func (m *mockRenderContext) DrawFill(x, y, w, h int, color byte) {
	m.fills = append(m.fills, struct {
		x, y, w, h int
		color      byte
	}{x, y, w, h, color})
}
func (m *mockRenderContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	m.alphaFills = append(m.alphaFills, struct {
		x, y, w, h int
		color      byte
		alpha      float32
	}{x, y, w, h, color, alpha})
}
func (m *mockRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockRenderContext) DrawCharacterAlpha(x, y int, num int, alpha float32) {
	m.alphaCharacters = append(m.alphaCharacters, struct {
		x, y, num int
		alpha     float32
	}{x, y, num, alpha})
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.DrawCharacter(x, y, num)
}
func (m *mockRenderContext) SetCanvas(ct renderer.CanvasType) {
	m.canvas.Type = ct
	m.canvasSwitch = append(m.canvasSwitch, ct)
}
func (m *mockRenderContext) Canvas() renderer.CanvasState { return m.canvas }
func (m *mockRenderContext) SetCanvasParams(p renderer.CanvasTransformParams) {
	m.canvasParams = p
}

func TestDrawNumber(t *testing.T) {
	tests := []struct {
		name   string
		num    int
		digits int
		want   string
	}{
		{"zero", 0, 1, "0"},
		{"single digit", 5, 1, "5"},
		{"two digits", 42, 2, "42"},
		{"padded", 7, 3, "7"}, // Spaces are not drawn, only visible chars
		{"negative", -10, 2, "-10"},
		{"large number", 999, 3, "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRenderContext{}
			DrawNumber(mock, 100, 50, tt.num, tt.digits)

			// Verify characters were drawn
			if len(mock.characters) == 0 {
				t.Error("No characters drawn")
				return
			}

			// Build the drawn string (only visible characters, spaces are skipped)
			drawn := ""
			for _, ch := range mock.characters {
				drawn += string(rune(ch.num))
			}

			if drawn != tt.want {
				t.Errorf("DrawNumber(%d, %d) = %q, want %q", tt.num, tt.digits, drawn, tt.want)
			}
		})
	}
}

func TestDrawString(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"single char", "A"},
		{"word", "Hello"},
		{"sentence", "Testing 123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRenderContext{}
			DrawString(mock, 10, 20, tt.text)

			if len(mock.characters) != len(tt.text) {
				t.Errorf("DrawString(%q) drew %d chars, want %d", tt.text, len(mock.characters), len(tt.text))
				return
			}

			// Verify each character
			for i, ch := range tt.text {
				if mock.characters[i].num != int(ch) {
					t.Errorf("Character %d: got %c, want %c", i, rune(mock.characters[i].num), ch)
				}
			}
		})
	}
}

func TestStatusBarDraw(t *testing.T) {
	sb := NewStatusBar(nil)
	mock := &mockRenderContext{}
	setTestViewSize(t, "100")

	// Draw with typical values
	sb.Draw(mock, State{Health: 100, Armor: 50, Ammo: 30}, 1280, 720)

	// Should have drawn numeric values (health, armor, ammo and inventory counts)
	if len(mock.characters) == 0 {
		t.Error("StatusBar.Draw() drew no characters")
	}

	// Should have drawn some rectangles (bars or background)
	if len(mock.fills) == 0 {
		t.Error("StatusBar.Draw() drew no rectangles")
	}
}

func TestStatusBarDrawHidesInventoryAtLargeViewsize(t *testing.T) {
	sb := NewStatusBar(nil)
	mock := &mockRenderContext{}
	setTestViewSize(t, "110")

	sb.Draw(mock, State{Health: 100}, 320, 48)

	if len(mock.fills) != 1 {
		t.Fatalf("fills = %d, want 1 status-bar fill without inventory strip", len(mock.fills))
	}
	if mock.fills[0].y != 24 {
		t.Fatalf("status-bar fill y = %d, want 24", mock.fills[0].y)
	}
}

func TestStatusBarDrawHidesMainBarAtHugeViewsize(t *testing.T) {
	sb := NewStatusBar(nil)
	mock := &mockRenderContext{}
	setTestViewSize(t, "120")

	sb.Draw(mock, State{Health: 100}, 320, 48)

	if len(mock.fills) != 0 || len(mock.pics) != 0 || len(mock.characters) != 0 {
		t.Fatalf("expected no classic HUD output at scr_viewsize 120, got fills=%d pics=%d chars=%d", len(mock.fills), len(mock.pics), len(mock.characters))
	}
}

func TestStatusBarScoreboardOverridesHugeViewsize(t *testing.T) {
	sb := NewStatusBar(nil)
	mock := &mockRenderContext{}
	setTestViewSize(t, "120")

	sb.Draw(mock, State{
		Health:     100,
		GameType:   1,
		MaxClients: 2,
		ShowScores: true,
		Scoreboard: []ScoreEntry{{Name: "p1", Frags: 5}},
	}, 320, 48)

	if len(mock.characters) == 0 && len(mock.fills) == 0 && len(mock.pics) == 0 {
		t.Fatal("scoreboard overlay should still draw at scr_viewsize 120")
	}
}

func TestStatusBarDrawUsesScreenSpacePicCoordinates(t *testing.T) {
	sbar := &image.QPic{Width: 320, Height: 24}
	ibar := &image.QPic{Width: 320, Height: 24}
	armor := &image.QPic{Width: 24, Height: 24}
	face := &image.QPic{Width: 24, Height: 24}
	ammo := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{
		sbarPic:   sbar,
		ibarPic:   ibar,
		armorPics: [3]*image.QPic{armor},
		ammoPics:  [4]*image.QPic{ammo},
	}
	sb.facePics[4][0] = face

	mock := &mockRenderContext{}
	sb.Draw(mock, State{
		Health: 100,
		Armor:  50,
		Ammo:   30,
		Items:  cl.ItemArmor1 | cl.ItemShells,
	}, 1280, 720)

	if len(mock.menuPics) != 0 {
		t.Fatalf("expected HUD status bar to avoid menu-space pic draws, got %d", len(mock.menuPics))
	}

	want := []struct {
		x, y int
		pic  *image.QPic
	}{
		{x: 480, y: 696, pic: sbar},
		{x: 480, y: 672, pic: ibar},
		{x: 480, y: 696, pic: armor},
		{x: 592, y: 696, pic: face},
		{x: 704, y: 696, pic: ammo},
	}
	if len(mock.pics) != len(want) {
		t.Fatalf("pic draw count = %d, want %d", len(mock.pics), len(want))
	}
	for i, expected := range want {
		got := mock.pics[i]
		if got.x != expected.x || got.y != expected.y || got.pic != expected.pic {
			t.Fatalf("pic draw %d = %+v, want %+v", i, got, expected)
		}
	}
}

func TestStatusBarDrawBigNumUsesClassicPics(t *testing.T) {
	alt2 := &image.QPic{Width: 24, Height: 24}
	alt5 := &image.QPic{Width: 24, Height: 24}
	alt7 := &image.QPic{Width: 24, Height: 24}
	alt0 := &image.QPic{Width: 24, Height: 24}
	base9 := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{}
	sb.numPics[0][9] = base9
	sb.numPics[1][0] = alt0
	sb.numPics[1][2] = alt2
	sb.numPics[1][5] = alt5
	sb.numPics[1][7] = alt7

	mock := &mockRenderContext{}
	sb.drawBigNum(mock, 24, 0, 25, 3, true)
	sb.drawBigNum(mock, 136, 0, 70, 3, true)
	sb.drawBigNum(mock, 248, 0, 1007, 3, false)

	if len(mock.characters) != 0 {
		t.Fatalf("expected classic pics, got %d character draws", len(mock.characters))
	}
	if len(mock.pics) != 7 {
		t.Fatalf("pic draw count = %d, want 7", len(mock.pics))
	}

	want := []struct {
		x   int
		pic *image.QPic
	}{
		{48, alt2},
		{72, alt5},
		{160, alt7},
		{184, alt0},
		{248, base9},
		{272, base9},
		{296, base9},
	}
	for i, expected := range want {
		got := mock.pics[i]
		if got.x != expected.x || got.y != 0 || got.pic != expected.pic {
			t.Fatalf("pic draw %d = %+v, want x=%d y=0 pic=%p", i, got, expected.x, expected.pic)
		}
	}
}

func TestStatusBarDrawBigNumFallsBackWithoutPics(t *testing.T) {
	sb := &StatusBar{}
	mock := &mockRenderContext{}

	sb.drawBigNum(mock, 24, 0, 25, 3, true)

	if len(mock.pics) != 0 {
		t.Fatalf("expected no pic draws without numeral assets, got %d", len(mock.pics))
	}
	drawn := ""
	for _, ch := range mock.characters {
		drawn += string(rune(ch.num))
	}
	if drawn != "25" {
		t.Fatalf("fallback characters = %q, want %q", drawn, "25")
	}
}

func TestHUDDraw(t *testing.T) {
	hud := NewHUD(nil)
	mock := &mockRenderContext{}

	hud.SetScreenSize(1280, 720)
	hud.SetState(State{Health: 100, Armor: 75, Ammo: 50, ActiveWeapon: 1})
	hud.Draw(mock)

	// HUD should draw status bar elements
	if len(mock.characters) == 0 && len(mock.fills) == 0 {
		t.Error("HUD.Draw() drew nothing")
	}
}

func TestHUDDrawCenterprintTimeoutFromClientTime(t *testing.T) {
	registerCenterprintTestCvars()
	cvar.Set("scr_centerprintbg", "2")
	cvar.Set("con_notifyfade", "0")
	h := NewHUD(nil)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          11,
	})
	active := &mockRenderContext{}
	h.Draw(active)
	if len(active.fills) <= 2 {
		t.Fatalf("expected centerprint background fills in addition to status bar, got %d fills", len(active.fills))
	}

	h.SetState(State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          13.1,
	})
	expired := &mockRenderContext{}
	h.Draw(expired)
	if len(expired.fills) != 2 {
		t.Fatalf("expected only status bar fills after centerprint expiry, got %d", len(expired.fills))
	}
}

func TestHUDCenterprintFadeTailExtendsLifetime(t *testing.T) {
	registerCenterprintTestCvars()
	cvar.Set("scr_centerprintbg", "0")
	cvar.Set("con_notifyfade", "1")
	cvar.Set("con_notifyfadetime", "0.5")

	cp := NewCenterprint(nil)
	active := &mockRenderContext{}
	cp.Draw(active, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.05,
	}, 320, 200)
	if got := charactersToString(active.characters); got != "message" {
		t.Fatalf("centerprint during fade tail = %q, want %q", got, "message")
	}

	expired := &mockRenderContext{}
	cp.Draw(expired, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.6,
	}, 320, 200)
	if got := charactersToString(expired.characters); got != "" {
		t.Fatalf("centerprint after fade tail = %q, want empty", got)
	}
}

func TestHUDCenterprintFadeTailUsesCharacterAlphaDuringLateFade(t *testing.T) {
	registerCenterprintTestCvars()
	cvar.Set("scr_centerprintbg", "2")
	cvar.Set("con_notifyfade", "1")
	cvar.Set("con_notifyfadetime", "0.5")

	cp := NewCenterprint(nil)
	fading := &mockRenderContext{}
	cp.Draw(fading, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.25,
	}, 320, 200)

	if got := charactersToString(fading.characters); got != "message" {
		t.Fatalf("late fade text = %q, want full message", got)
	}
	if len(fading.alphaCharacters) != len("message") {
		t.Fatalf("late fade alpha characters = %d, want %d", len(fading.alphaCharacters), len("message"))
	}
	for _, ch := range fading.alphaCharacters {
		if math.Abs(float64(ch.alpha)-0.5) > 0.0001 {
			t.Fatalf("late fade alpha character = %+v, want alpha=0.5", ch)
		}
	}
	if len(fading.alphaFills) != 1 {
		t.Fatalf("late fade alpha background fills = %d, want 1", len(fading.alphaFills))
	}
	if got := fading.alphaFills[0]; got.color != 0 || math.Abs(float64(got.alpha)-0.5) > 0.0001 {
		t.Fatalf("late fade alpha background fill = %+v, want color=0 alpha=0.5", got)
	}
}

func TestHUDIntermissionOverlaySuppressesStatusBar(t *testing.T) {
	h := NewHUD(nil)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  1,
		CompletedTime: 125,
		LevelName:     "Unit Test Map",
		Secrets:       2,
		TotalSecrets:  4,
		Monsters:      5,
		TotalMonsters: 8,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if len(mock.fills) != 0 {
		t.Fatalf("expected no status-bar fill draws during intermission, got %d", len(mock.fills))
	}
	if len(mock.characters) == 0 {
		t.Fatal("expected intermission overlay text draw")
	}
}

func TestCenterprintIntermissionUsesMenuSpaceOverlayCoordinates(t *testing.T) {
	complete := &image.QPic{Width: 100, Height: 20}
	inter := &image.QPic{Width: 64, Height: 24}
	cp := &Centerprint{
		completePic: complete,
		interPic:    inter,
	}
	mock := &mockRenderContext{}
	cp.Draw(mock, State{Intermission: 1}, 1280, 720)

	if len(mock.pics) != 2 {
		t.Fatalf("screen-space pic draw count = %d, want 2 menu-space-aware pic draws", len(mock.pics))
	}

	want := []struct {
		x, y int
		pic  *image.QPic
	}{
		{x: 110, y: 8, pic: complete},
		{x: 128, y: 56, pic: inter},
	}
	if len(mock.pics) < len(want) {
		t.Fatalf("pic draw count = %d, want at least %d", len(mock.pics), len(want))
	}
	for i, expected := range want {
		got := mock.pics[i]
		if got.x != expected.x || got.y != expected.y || got.pic != expected.pic {
			t.Fatalf("pic draw %d = %+v, want %+v", i, got, expected)
		}
	}
	if len(mock.canvasSwitch) == 0 || mock.canvasSwitch[0] != renderer.CanvasMenu {
		t.Fatalf("canvas switches = %v, want first switch to CanvasMenu", mock.canvasSwitch)
	}
}

func TestHUDFinaleOverlayShowsCenterTextWithoutTimeout(t *testing.T) {
	h := NewHUD(nil)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  2,
		CenterPrint:   "Finale line",
		CenterPrintAt: 1,
		Time:          100,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if len(mock.characters) == 0 {
		t.Fatal("expected finale center text draw")
	}
}

func TestHUDFinaleOverlayRevealsCenterTextOverTime(t *testing.T) {
	h := NewHUD(nil)
	h.SetScreenSize(320, 200)
	base := State{
		Intermission:  2,
		CenterPrint:   "ABCD",
		CenterPrintAt: 1,
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.1
		return s
	}())
	initial := &mockRenderContext{}
	h.Draw(initial)
	if got := charactersToString(initial.characters); got != "" {
		t.Fatalf("initial finale reveal = %q, want empty", got)
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.26
		return s
	}())
	partial := &mockRenderContext{}
	h.Draw(partial)
	if got := charactersToString(partial.characters); got != "AB" {
		t.Fatalf("partial finale reveal = %q, want AB", got)
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.6
		return s
	}())
	full := &mockRenderContext{}
	h.Draw(full)
	if got := charactersToString(full.characters); got != "ABCD" {
		t.Fatalf("full finale reveal = %q, want ABCD", got)
	}
}

func TestHUDCutsceneOverlayUsesTimedReveal(t *testing.T) {
	h := NewHUD(nil)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  3,
		CenterPrint:   "A\nB",
		CenterPrintAt: 4,
		Time:          4.26,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if got := charactersToString(mock.characters); got != "AB" {
		t.Fatalf("cutscene reveal = %q, want AB", got)
	}
}

func TestCenterprintBackgroundModeThreeUsesFullWidthStrip(t *testing.T) {
	registerCenterprintTestCvars()
	cvar.Set("scr_centerprintbg", "3")

	cp := NewCenterprint(nil)
	mock := &mockRenderContext{}
	cp.Draw(mock, State{
		CenterPrint:   "HELLO",
		CenterPrintAt: 1,
		Time:          1.1,
	}, 320, 200)

	if len(mock.fills) != 1 {
		t.Fatalf("fill count = %d, want 1", len(mock.fills))
	}
	if got := mock.fills[0]; got.x != 0 || got.w != 320 {
		t.Fatalf("strip fill = %+v, want full-width strip", got)
	}
}

func TestCenterprintYMatchesCanonicalBranches(t *testing.T) {
	registerCenterprintTestCvars()
	cvar.Set("con_notifyfade", "1")
	cvar.Set("con_notifyfadetime", "0.5")

	if got := centerprintY(200, "one\ntwo"); got != 70 {
		t.Fatalf("short centerprint y = %d, want 70", got)
	}
	if got := centerprintY(200, "1\n2\n3\n4\n5"); got != 48 {
		t.Fatalf("long centerprint y = %d, want 48", got)
	}
	if got := centerprintFadeTail(); math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("centerprint fade tail = %.2f, want 0.50", got)
	}
	if got := centerprintVisualAlpha(State{CenterPrintAt: 10, Time: 12.25}); math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("centerprint visual alpha = %.2f, want 0.50", got)
	}
}

func charactersToString(chars []struct{ x, y, num int }) string {
	out := strings.Builder{}
	for _, ch := range chars {
		out.WriteRune(rune(ch.num))
	}
	return out.String()
}

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

func TestStatusBarWeaponPickupFlashTiming(t *testing.T) {
	active := &image.QPic{Width: 24, Height: 16}
	owned := &image.QPic{Width: 24, Height: 16}
	flash := &image.QPic{Width: 24, Height: 16}
	sb := &StatusBar{
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
	sb := NewStatusBar(nil)
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

func TestStatusBarDrawScoreboardOverlayWhenHeld(t *testing.T) {
	sb := NewStatusBar(nil)
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
	h := NewHUD(nil)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 42, ActiveWeapon: 2, Shells: 10})

	rc := &mockRenderContext{}

	// Classic style: status bar draws pics (from sbar.go); with nil draw manager
	// it falls through to DrawFill calls. Reset and count.
	_ = strings.Contains // keep import
	_ = cl.Client{}      // keep import
	cvar.Set("hud_style", "0")
	h.Draw(rc)
	classicCalls := len(rc.characters) + len(rc.fills)

	// Compact style: only DrawCharacter calls.
	rc2 := &mockRenderContext{}
	cvar.Set("hud_style", "1")
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
	h := NewHUD(nil)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 100})
	setTestViewSize(t, "100")

	classic := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 320, Bottom: 48},
	}
	cvar.Set("hud_style", "0")
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
	cvar.Set("hud_style", "1")
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
	cvar.Set("hud_style", "2")
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
	sb := NewStatusBar(nil)
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
	h := NewHUD(nil)
	h.SetScreenSize(640, 480)
	h.SetState(State{Health: 100, ActiveWeapon: 2, Shells: 10})

	rc := &mockRenderContext{
		canvas: renderer.CanvasState{Left: 0, Top: 0, Right: 400, Bottom: 225},
	}
	cvar.Set("hud_style", "1")
	setTestViewSize(t, "120")
	h.Draw(rc)

	if len(rc.characters) != 0 || len(rc.fills) != 0 || len(rc.pics) != 0 {
		t.Fatalf("compact HUD should hide at scr_viewsize 120, got chars=%d fills=%d pics=%d", len(rc.characters), len(rc.fills), len(rc.pics))
	}
}
