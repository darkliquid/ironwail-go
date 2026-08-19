package quakui

import (
	"github.com/darkliquid/ironwail-go/internal/draw"
	uitheme "github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

// ThemeExtensionName is the stable name under which the quakui
// ThemeExtension is registered on the Quake theme.
const ThemeExtensionName = "quakui"

// QuakeTheme returns a dark gogpu/ui theme with the Quake palette tokens
// applied (spec §4.3, ADR-0008):
//
//   - Background: palette index 0 (black opaque).
//   - Surface:    palette index 4 (0x3f3f3f) — the status-bar panel color.
//   - OnSurface:  palette index 254 (white) — bright text row.
//   - Primary:    palette index 250 (0xd70000) — bright red accent.
//   - OnBackground: palette index 253 (0xfff7c7) — warm off-white.
//
// The quakui ThemeExtension is registered with the glyph conventions and the
// raw 768-byte palette for exact per-index lookups.
func QuakeTheme() *uitheme.Theme {
	t := uitheme.DefaultDark()
	t.Name = "quake"

	t.Colors.Background = widget.RGB8(0x00, 0x00, 0x00)    // palette[0]
	t.Colors.Surface = widget.RGB8(0x3f, 0x3f, 0x3f)       // palette[4]
	t.Colors.OnSurface = widget.RGB8(0xff, 0xff, 0xff)      // palette[254]
	t.Colors.Primary = widget.RGB8(0xd7, 0x00, 0x00)        // palette[250]
	t.Colors.OnBackground = widget.RGB8(0xff, 0xf7, 0xc7)   // palette[253]

	t.RegisterExtension(DefaultThemeExtension())
	return t
}

// ThemeExtension carries the Quake-specific conventions the widgets read
// from the theme (spec §4.3).
type ThemeExtension struct {
	// PromptGlyph is the console input prompt character (']').
	PromptGlyph rune
	// ScrollHintGlyph is the console scrollback indicator ('^').
	ScrollHintGlyph rune
	// BrightRow reports whether the high-bit conchars row (char + 128) is
	// the bright/white row.
	BrightRow bool
	// MenuBgAlpha is the menu backdrop fade alpha (scr_menubgalpha default).
	MenuBgAlpha float32
	// SbarAlpha is the status bar background alpha (scr_sbaralpha default).
	SbarAlpha float32
	// Palette is the 768-byte Quake palette for exact per-index lookups.
	Palette []byte
}

// DefaultThemeExtension returns the extension with engine cvar defaults.
func DefaultThemeExtension() *ThemeExtension {
	return &ThemeExtension{
		PromptGlyph:     ']',
		ScrollHintGlyph: '^',
		BrightRow:       true,
		MenuBgAlpha:     0.7,  // scr_menubgalpha default
		SbarAlpha:       0.75, // scr_sbaralpha default
		Palette:         draw.DefaultQuakePalette(),
	}
}

// Name returns the stable extension identifier.
func (e *ThemeExtension) Name() string { return ThemeExtensionName }

// Merge adopts the other extension when it is the same type, else keeps this.
func (e *ThemeExtension) Merge(other uitheme.ThemeExtension) uitheme.ThemeExtension {
	if o, ok := other.(*ThemeExtension); ok {
		return o
	}
	return e
}

// Lerp interpolates the numeric fields and keeps the non-numeric ones.
func (e *ThemeExtension) Lerp(other uitheme.ThemeExtension, t float32) uitheme.ThemeExtension {
	if o, ok := other.(*ThemeExtension); ok {
		return &ThemeExtension{
			PromptGlyph:     rune(uitheme.LerpString(string(e.PromptGlyph), string(o.PromptGlyph), t)[0]),
			ScrollHintGlyph: rune(uitheme.LerpString(string(e.ScrollHintGlyph), string(o.ScrollHintGlyph), t)[0]),
			BrightRow:       e.BrightRow,
			MenuBgAlpha:     uitheme.LerpFloat32(e.MenuBgAlpha, o.MenuBgAlpha, t),
			SbarAlpha:       uitheme.LerpFloat32(e.SbarAlpha, o.SbarAlpha, t),
			Palette:         e.Palette,
		}
	}
	return e
}

// CopyWith returns a copy with the named overrides applied.
func (e *ThemeExtension) CopyWith(overrides map[string]any) uitheme.ThemeExtension {
	copy := *e
	if v, ok := overrides["menu_bg_alpha"].(float32); ok {
		copy.MenuBgAlpha = v
	}
	if v, ok := overrides["sbar_alpha"].(float32); ok {
		copy.SbarAlpha = v
	}
	return &copy
}

var _ uitheme.ThemeExtension = (*ThemeExtension)(nil)
