package theme

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

// TestQuakeThemeTokenMapping asserts the Quake palette token mapping from
// spec §4.3: palette[0] becomes Background, palette[4] becomes Surface, and
// the bright text row (palette[254], white) is reachable via the extension.
func TestQuakeThemeTokenMapping(t *testing.T) {
	th := QuakeTheme()

	// Background = palette index 0 (black opaque).
	if th.Colors.Background != widget.RGB8(0x00, 0x00, 0x00) {
		t.Fatalf("Background = %v, want palette[0] (black)", th.Colors.Background)
	}
	// Surface = palette index 4 (0x3f3f3f).
	if th.Colors.Surface != widget.RGB8(0x3f, 0x3f, 0x3f) {
		t.Fatalf("Surface = %v, want palette[4] (0x3f3f3f)", th.Colors.Surface)
	}

	ext, ok := theme.ExtensionAs[*Extension](th, ExtensionName)
	if !ok {
		t.Fatal("quakeui ThemeExtension not registered")
	}
	// Bright row flag true; bright text color is palette[254] (white).
	if !ext.BrightRow {
		t.Fatal("BrightRow = false, want true (high-bit conchars row is bright)")
	}
	if ext.Palette == nil || len(ext.Palette) != 768 {
		t.Fatalf("Palette length = %d, want 768", len(ext.Palette))
	}
}

// TestQuakeThemeExtensionDefaults asserts the extension carries the engine
// cvar defaults for menu/sbar alpha and the conchars glyph conventions.
func TestQuakeThemeExtensionDefaults(t *testing.T) {
	th := QuakeTheme()
	ext, ok := theme.ExtensionAs[*Extension](th, ExtensionName)
	if !ok {
		t.Fatal("quakeui ThemeExtension not registered")
	}

	if ext.PromptGlyph != ']' {
		t.Fatalf("PromptGlyph = %q, want ']'", ext.PromptGlyph)
	}
	if ext.ScrollHintGlyph != '^' {
		t.Fatalf("ScrollHintGlyph = %q, want '^'", ext.ScrollHintGlyph)
	}
	if ext.MenuBgAlpha != 0.7 {
		t.Fatalf("MenuBgAlpha = %v, want 0.7 (scr_menubgalpha default)", ext.MenuBgAlpha)
	}
	if ext.SbarAlpha != 0.75 {
		t.Fatalf("SbarAlpha = %v, want 0.75 (scr_sbaralpha default)", ext.SbarAlpha)
	}
	if ext.PuaBase != PuaBase {
		t.Fatalf("PuaBase = %#x, want %#x", ext.PuaBase, PuaBase)
	}
}

// TestQuakeThemeMergeLerp asserts the extension satisfies the ThemeExtension
// contract: Merge takes the other side, Lerp interpolates colors.
func TestQuakeThemeMergeLerp(t *testing.T) {
	th := QuakeTheme()
	ext, ok := theme.ExtensionAs[*Extension](th, ExtensionName)
	if !ok {
		t.Fatal("quakeui ThemeExtension not registered")
	}

	other := &Extension{
		PromptGlyph:     '>',
		ScrollHintGlyph: 'v',
		BrightRow:       false,
		MenuBgAlpha:     0.5,
		SbarAlpha:       0.5,
		Palette:         []byte{1, 2, 3},
		PuaBase:         0xE200,
	}
	merged, ok := ext.Merge(other).(*Extension)
	if !ok {
		t.Fatal("Merge returned non-Extension")
	}
	if merged.PromptGlyph != '>' || merged.MenuBgAlpha != 0.5 {
		t.Fatalf("Merge did not adopt other: %+v", merged)
	}

	lerped := ext.Lerp(other, 0.25).(*Extension)
	if lerped.PromptGlyph != ']' {
		t.Fatalf("Lerp changed non-interpolable PromptGlyph: %q", lerped.PromptGlyph)
	}
	if lerped.MenuBgAlpha != 0.65 {
		t.Fatalf("Lerp MenuBgAlpha = %v, want 0.65", lerped.MenuBgAlpha)
	}
}

// TestQuakeThemeCopyWith asserts CopyWith applies named overrides and keeps
// the rest.
func TestQuakeThemeCopyWith(t *testing.T) {
	th := QuakeTheme()
	ext, _ := theme.ExtensionAs[*Extension](th, ExtensionName)

	copied := ext.CopyWith(map[string]any{"menu_bg_alpha": float32(0.3)}).(*Extension)
	if copied.MenuBgAlpha != 0.3 {
		t.Fatalf("CopyWith menu_bg_alpha = %v, want 0.3", copied.MenuBgAlpha)
	}
	if copied.SbarAlpha != ext.SbarAlpha {
		t.Fatalf("CopyWith changed untouched SbarAlpha: %v", copied.SbarAlpha)
	}
}

// TestQPicToImage asserts the pic bridge converts a palette-indexed QPic to
// an RGBA image with palette index 255 transparent (Quake convention).
func TestQPicToImage(t *testing.T) {
	pic := &qimage.QPic{
		Width:  2,
		Height: 2,
		Pixels: []byte{0, 4, 255, 254},
	}
	img := QPicToImage(pic, draw.DefaultQuakePalette())

	if img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", img.Bounds())
	}
	// palette[0] = black opaque.
	if r, g, b, a := img.At(0, 0).RGBA(); r != 0 || g != 0 || b != 0 || a != 0xffff {
		t.Fatalf("pixel(0,0) = %d,%d,%d,%d, want opaque black", r, g, b, a)
	}
	// palette[4] = 0x3f3f3f opaque.
	if r, g, b, _ := img.At(1, 0).RGBA(); r>>8 != 0x3f || g>>8 != 0x3f || b>>8 != 0x3f {
		t.Fatalf("pixel(1,0) = %d,%d,%d, want 0x3f3f3f", r>>8, g>>8, b>>8)
	}
	// palette[255] = transparent.
	if _, _, _, a := img.At(0, 1).RGBA(); a != 0 {
		t.Fatalf("pixel(0,1) alpha = %d, want 0 (transparent index 255)", a)
	}
	// palette[254] = white opaque.
	if r, g, b, a := img.At(1, 1).RGBA(); r>>8 != 0xff || g>>8 != 0xff || b>>8 != 0xff || a != 0xffff {
		t.Fatalf("pixel(1,1) = %d,%d,%d,%d, want white opaque", r>>8, g>>8, b>>8, a)
	}
}

// TestQPicToImageNilPalette asserts a nil palette falls back to the standard
// Quake palette without panicking.
func TestQPicToImageNilPalette(t *testing.T) {
	pic := &qimage.QPic{
		Width:  1,
		Height: 1,
		Pixels: []byte{0},
	}
	img := QPicToImage(pic, nil)
	if img == nil || img.Bounds() != image.Rect(0, 0, 1, 1) {
		t.Fatalf("nil palette produced %v", img)
	}
}
