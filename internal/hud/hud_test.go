// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"math"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

var testCV = cvar.NewCVarSystem()

func init() {
	testCV.Register("scr_sbaralpha", "0.75", cvar.FlagArchive, "")
}

func registerCenterprintTestCvars() {
	testCV.Register("scr_centertime", "2", 0, "test centerprint hold time")
	testCV.Register("scr_centerprintbg", "2", cvar.FlagArchive, "test centerprint background")
	testCV.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "test menu background alpha")
	testCV.Register("scr_printspeed", "8", 0, "test centerprint reveal speed")
	testCV.Register("con_notifyfade", "0", cvar.FlagArchive, "test centerprint fade enable")
	testCV.Register("con_notifyfadetime", "0.5", cvar.FlagArchive, "test centerprint fade duration")
	testCV.Register("crosshair", "0", cvar.FlagArchive, "test crosshair")
	testCV.Register("scr_viewsize", "100", cvar.FlagArchive, "test viewsize")
}

func setTestViewSize(t *testing.T, value string) {
	t.Helper()
	testCV.Set("scr_viewsize", value)
	t.Cleanup(func() {
		testCV.Set("scr_viewsize", "100")
	})
}

func setTestSbarAlpha(t *testing.T, value string) {
	t.Helper()
	testCV.Register("scr_sbaralpha", "0.75", cvar.FlagArchive, "")
	testCV.Set("scr_sbaralpha", value)
	t.Cleanup(func() {
		testCV.Set("scr_sbaralpha", "0.75")
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
	alphaPics []struct {
		x, y  int
		pic   *image.QPic
		alpha float32
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
func (m *mockRenderContext) DrawPicAlpha(x, y int, pic *image.QPic, alpha float32) {
	m.alphaPics = append(m.alphaPics, struct {
		x, y  int
		pic   *image.QPic
		alpha float32
	}{x, y, pic, alpha})
	m.DrawPic(x, y, pic)
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
	m.DrawFill(x, y, w, h, color)
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
	sb := NewStatusBar(nil, testCV)
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
	sb := NewStatusBar(nil, testCV)
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
	sb := NewStatusBar(nil, testCV)
	mock := &mockRenderContext{}
	setTestViewSize(t, "120")

	sb.Draw(mock, State{Health: 100}, 320, 48)

	if len(mock.fills) != 0 || len(mock.pics) != 0 || len(mock.characters) != 0 {
		t.Fatalf("expected no classic HUD output at scr_viewsize 120, got fills=%d pics=%d chars=%d", len(mock.fills), len(mock.pics), len(mock.characters))
	}
}

func TestStatusBarScoreboardOverridesHugeViewsize(t *testing.T) {
	sb := NewStatusBar(nil, testCV)
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
	setTestSbarAlpha(t, "1")
	sbar := &image.QPic{Width: 320, Height: 24}
	ibar := &image.QPic{Width: 320, Height: 24}
	armor := &image.QPic{Width: 24, Height: 24}
	face := &image.QPic{Width: 24, Height: 24}
	ammo := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{
		cvars:     testCV,
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

func TestStatusBarDrawUsesAlphaPicsForBarBackgrounds(t *testing.T) {
	setTestSbarAlpha(t, "0.75")
	sbar := &image.QPic{Width: 320, Height: 24}
	ibar := &image.QPic{Width: 320, Height: 24}
	sb := &StatusBar{cvars: testCV, sbarPic: sbar, ibarPic: ibar}
	mock := &mockRenderContext{}

	sb.Draw(mock, State{Health: 100}, 320, 48)

	if len(mock.alphaPics) != 2 {
		t.Fatalf("alpha pic count = %d, want 2", len(mock.alphaPics))
	}
	if got := mock.alphaPics[0]; got.x != 0 || got.y != 24 || got.pic != sbar || math.Abs(float64(got.alpha-0.75)) > 0.0001 {
		t.Fatalf("sbar alpha pic = %+v, want x=0 y=24 alpha=0.75", got)
	}
	if got := mock.alphaPics[1]; got.x != 0 || got.y != 0 || got.pic != ibar || math.Abs(float64(got.alpha-0.75)) > 0.0001 {
		t.Fatalf("ibar alpha pic = %+v, want x=0 y=0 alpha=0.75", got)
	}
}

func TestStatusBarScoreboardUsesAlphaBackground(t *testing.T) {
	setTestSbarAlpha(t, "0.5")
	scorebar := &image.QPic{Width: 320, Height: 24}
	sb := &StatusBar{cvars: testCV, scorebarPic: scorebar}
	mock := &mockRenderContext{}

	sb.drawScoreboard(mock, State{Scoreboard: []ScoreEntry{{Name: "p1", Frags: 1}}}, 0, 24)

	if len(mock.alphaPics) == 0 {
		t.Fatal("expected alpha scorebar background draw")
	}
	if got := mock.alphaPics[0]; got.pic != scorebar || math.Abs(float64(got.alpha-0.5)) > 0.0001 {
		t.Fatalf("scorebar alpha pic = %+v, want alpha=0.5", got)
	}
}

func TestStatusBarDrawBigNumUsesClassicPics(t *testing.T) {
	alt2 := &image.QPic{Width: 24, Height: 24}
	alt5 := &image.QPic{Width: 24, Height: 24}
	alt7 := &image.QPic{Width: 24, Height: 24}
	alt0 := &image.QPic{Width: 24, Height: 24}
	base9 := &image.QPic{Width: 24, Height: 24}
	sb := &StatusBar{cvars: testCV}
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
	sb := &StatusBar{cvars: testCV}
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
	hud := NewHUD(nil, testCV)
	mock := &mockRenderContext{}

	hud.SetScreenSize(1280, 720)
	hud.SetState(State{Health: 100, Armor: 75, Ammo: 50, ActiveWeapon: 1})
	hud.Draw(mock)

	// HUD should draw status bar elements
	if len(mock.characters) == 0 && len(mock.fills) == 0 {
		t.Error("HUD.Draw() drew nothing")
	}
}

func TestStatusBarDrawQuakeWorldAmmoBackgroundsUseAlpha(t *testing.T) {
	setTestSbarAlpha(t, "0.5")
	row0 := &image.QPic{Width: 42, Height: 11}
	row1 := &image.QPic{Width: 42, Height: 11}
	row2 := &image.QPic{Width: 42, Height: 11}
	row3 := &image.QPic{Width: 42, Height: 11}
	sb := &StatusBar{
		cvars:    testCV,
		qwAmmoBG: [4]*image.QPic{row0, row1, row2, row3},
	}

	mock := &mockRenderContext{}
	sb.DrawQuakeWorld(mock, State{
		Shells:       10,
		Nails:        20,
		Rockets:      30,
		Cells:        40,
		GameType:     0,
		MaxClients:   1,
		ActiveWeapon: int(cl.ItemShotgun),
	}, 320, 200)

	if len(mock.alphaPics) != 4 {
		t.Fatalf("quakeworld ammo bg alpha pics = %d, want 4", len(mock.alphaPics))
	}
	for i, got := range mock.alphaPics {
		wantY := -45 + i*11
		if got.x != 6 || got.y != wantY || math.Abs(float64(got.alpha)-0.5) > 0.0001 {
			t.Fatalf("quakeworld ammo bg %d = %+v, want x=6 y=%d alpha=0.5", i, got, wantY)
		}
	}
}

func TestStatusBarDrawQuakeWorldSigilBackgroundOnlyWhenNeeded(t *testing.T) {
	sigilBG := &image.QPic{Width: 32, Height: 16}
	sb := &StatusBar{
		cvars:     testCV,
		qwSigilBG: sigilBG,
		sigilPics: [4]*image.QPic{
			&image.QPic{Width: 8, Height: 16},
			&image.QPic{Width: 8, Height: 16},
			&image.QPic{Width: 8, Height: 16},
			&image.QPic{Width: 8, Height: 16},
		},
	}

	withSigil := &mockRenderContext{}
	sb.DrawQuakeWorld(withSigil, State{
		GameType:     0,
		MaxClients:   1,
		ActiveWeapon: int(cl.ItemShotgun),
		Items:        cl.ItemSigil1,
	}, 320, 200)

	foundBG := false
	for _, draw := range withSigil.pics {
		if draw.pic == sigilBG {
			foundBG = true
			if draw.x != 16 || draw.y != 8 {
				t.Fatalf("quakeworld sigil bg draw = (%d,%d), want (16,8)", draw.x, draw.y)
			}
		}
	}
	if !foundBG {
		t.Fatalf("expected quakeworld sigil background draw when sigil is owned")
	}

	withoutSigil := &mockRenderContext{}
	sb.DrawQuakeWorld(withoutSigil, State{
		GameType:     0,
		MaxClients:   1,
		ActiveWeapon: int(cl.ItemShotgun),
	}, 320, 200)

	for _, draw := range withoutSigil.pics {
		if draw.pic == sigilBG {
			t.Fatalf("unexpected quakeworld sigil background draw without sigils")
		}
	}
}

func TestRegularCenterprintYShiftsUpWhenCrosshairVisible(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("crosshair", "1")
	t.Cleanup(func() {
		testCV.Set("crosshair", "0")
	})

	setTestViewSize(t, "100")
	if got := (&Centerprint{cvars: testCV}).regularCenterprintY(200, "one\ntwo"); got != 62 {
		t.Fatalf("regular centerprint y with crosshair = %d, want 62", got)
	}

	setTestViewSize(t, "130")
	if got := (&Centerprint{cvars: testCV}).regularCenterprintY(200, "one\ntwo"); got != 70 {
		t.Fatalf("regular centerprint y at viewsize 130 = %d, want 70", got)
	}
}

func charactersToString(chars []struct{ x, y, num int }) string {
	out := strings.Builder{}
	for _, ch := range chars {
		out.WriteRune(rune(ch.num))
	}
	return out.String()
}
