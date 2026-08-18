package theme

import (
	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

// Extension carries the Quake-specific conventions that the console, menu,
// and HUD widgets read from the theme (spec §4.3). It implements the
// gogpu/ui ThemeExtension contract so it can be merged, lerped, and copied
// like any other extension.
type Extension struct {
	// PromptGlyph is the console input prompt character (']').
	PromptGlyph rune
	// ScrollHintGlyph is the console scrollback indicator ('^').
	ScrollHintGlyph rune
	// BrightRow reports whether the high-bit conchars row (char + 128) is
	// the bright/white row. Quake draws bright text from the alternate
	// glyph row; when false the row is the bronze variant.
	BrightRow bool
	// MenuBgAlpha is the menu backdrop fade alpha (scr_menubgalpha).
	MenuBgAlpha float32
	// SbarAlpha is the status bar background alpha (scr_sbaralpha).
	SbarAlpha float32
	// Palette is the 768-byte Quake palette (palette.lmp) for exact
	// per-index color lookups.
	Palette []byte
	// PuaBase is the Private-Use-Area base for conchars glyph mapping
	// (ADR-0004): U+E000 + char for the normal row, U+E100 + char for
	// the bright row.
	PuaBase rune
}

// Name returns the stable extension identifier.
func (e *Extension) Name() string { return ExtensionName }

// Merge adopts the other extension when it is the same type (child
// overrides parent), otherwise keeps this one.
func (e *Extension) Merge(other theme.ThemeExtension) theme.ThemeExtension {
	if o, ok := other.(*Extension); ok {
		return o
	}
	return e
}

// Lerp interpolates the numeric fields and keeps the non-interpolable
// fields from this side (ADR-0004: glyphs and palette do not animate).
func (e *Extension) Lerp(other theme.ThemeExtension, t float32) theme.ThemeExtension {
	if o, ok := other.(*Extension); ok {
		return &Extension{
			PromptGlyph:     rune(theme.LerpString(string(e.PromptGlyph), string(o.PromptGlyph), t)[0]),
			ScrollHintGlyph: rune(theme.LerpString(string(e.ScrollHintGlyph), string(o.ScrollHintGlyph), t)[0]),
			BrightRow:       e.BrightRow,
			MenuBgAlpha:     theme.LerpFloat32(e.MenuBgAlpha, o.MenuBgAlpha, t),
			SbarAlpha:       theme.LerpFloat32(e.SbarAlpha, o.SbarAlpha, t),
			Palette:         e.Palette,
			PuaBase:         e.PuaBase,
		}
	}
	return e
}

// CopyWith returns a copy with the named overrides applied. Supported keys:
// "menu_bg_alpha" (float32) and "sbar_alpha" (float32).
func (e *Extension) CopyWith(overrides map[string]any) theme.ThemeExtension {
	copy := *e
	if v, ok := overrides["menu_bg_alpha"].(float32); ok {
		copy.MenuBgAlpha = v
	}
	if v, ok := overrides["sbar_alpha"].(float32); ok {
		copy.SbarAlpha = v
	}
	return &copy
}

// PuaForChar returns the Private-Use-Area rune for a conchars character.
// When bright is true the rune lands in the bright row (U+E100 + char).
func (e *Extension) PuaForChar(c byte, bright bool) rune {
	base := e.PuaBase
	if bright {
		base += 0x100
	}
	return base + rune(c)
}

var _ theme.ThemeExtension = (*Extension)(nil)

// ColorFromPalette converts a palette index to a widget.Color using the
// extension's palette. Out-of-range palettes fall back to opaque gray.
func ColorFromPalette(pal []byte, idx byte) widget.Color {
	if len(pal) < 768 {
		return widget.RGB8(idx, idx, idx)
	}
	off := int(idx) * 3
	return widget.RGB8(pal[off], pal[off+1], pal[off+2])
}
