package quakeui

import (
	"testing"

	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

// TestQuakeThemeTokenMapping asserts the Quake palette token mapping
// (spec §4.3): palette[0] = Background, palette[4] = Surface, and the
// extension carries the bright-row flag and the raw palette.
func TestQuakeThemeTokenMapping(t *testing.T) {
	th := QuakeTheme()
	if th == nil {
		t.Fatal("QuakeTheme = nil")
	}

	if th.Colors.Background != widget.RGBA(0x00, 0x00, 0x00, 0x00) {
		t.Fatalf("Background = %v, want transparent black", th.Colors.Background)
	}
	if th.Colors.Surface != widget.RGB8(0x3f, 0x3f, 0x3f) {
		t.Fatalf("Surface = %v, want palette[4] (0x3f3f3f)", th.Colors.Surface)
	}
	if th.Colors.OnSurface != widget.RGB8(0xff, 0xff, 0xff) {
		t.Fatalf("OnSurface = %v, want palette[254] (white)", th.Colors.OnSurface)
	}

	ext, ok := theme.ExtensionAs[*ThemeExtension](th, ThemeExtensionName)
	if !ok {
		t.Fatal("ThemeExtension not registered")
	}
	if !ext.BrightRow {
		t.Fatal("BrightRow = false, want true")
	}
	if len(ext.Palette) != 768 {
		t.Fatalf("Palette length = %d, want 768", len(ext.Palette))
	}
}

// TestThemeExtensionGlyphs asserts the extension carries the conchars glyph
// conventions (prompt ']', scroll '^').
func TestThemeExtensionGlyphs(t *testing.T) {
	ext := DefaultThemeExtension()
	if ext.PromptGlyph != ']' {
		t.Fatalf("PromptGlyph = %q, want ']'", ext.PromptGlyph)
	}
	if ext.ScrollHintGlyph != '^' {
		t.Fatalf("ScrollHintGlyph = %q, want '^'", ext.ScrollHintGlyph)
	}
}

// TestThemeExtensionMergeLerp asserts the extension satisfies the
// ThemeExtension contract (Merge adopts other, Lerp interpolates).
func TestThemeExtensionMergeLerp(t *testing.T) {
	ext := DefaultThemeExtension()
	other := &ThemeExtension{PromptGlyph: '>', BrightRow: false, MenuBgAlpha: 0.5, SbarAlpha: 0.5}

	merged, ok := ext.Merge(other).(*ThemeExtension)
	if !ok || merged.PromptGlyph != '>' {
		t.Fatal("Merge did not adopt other")
	}
	lerped := ext.Lerp(other, 0.25).(*ThemeExtension)
	if lerped.MenuBgAlpha != 0.65 {
		t.Fatalf("Lerp MenuBgAlpha = %v, want 0.65", lerped.MenuBgAlpha)
	}
	if lerped.PromptGlyph != ']' {
		t.Fatalf("Lerp changed non-interpolable PromptGlyph: %q", lerped.PromptGlyph)
	}
}
