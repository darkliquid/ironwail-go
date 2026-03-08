// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/image"
)

// mockRenderContext is a test double for renderer.RenderContext
type mockRenderContext struct {
	characters []struct{ x, y, num int }
	pics       []struct {
		x, y int
		pic  *image.QPic
	}
	fills []struct {
		x, y, w, h int
		color      byte
	}
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
func (m *mockRenderContext) DrawFill(x, y, w, h int, color byte) {
	m.fills = append(m.fills, struct {
		x, y, w, h int
		color      byte
	}{x, y, w, h, color})
}
func (m *mockRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.DrawCharacter(x, y, num)
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
		weaponPics: [2][7]*image.QPic{{weaponOwned}, {weaponActive}},
		itemPics:   [6]*image.QPic{itemPic},
		sigilPics:  [4]*image.QPic{sigilPic},
		facePics:   [5][2]*image.QPic{{facePic}, {facePic}, {facePic}, {facePic}, {facePic}},
		armorPics:  [3]*image.QPic{armorPic},
		ammoPics:   [4]*image.QPic{ammoPic},
	}
	mock := &mockRenderContext{}

	sb.Draw(mock, State{
		Health:       100,
		Armor:        40,
		Ammo:         20,
		ActiveWeapon: 1,
		Shells:       20,
		Nails:        30,
		Rockets:      40,
		Cells:        50,
		Items:        1 | (1 << 8) | (1 << 13) | (1 << 17) | (1 << 28),
	}, 320, 200)

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
