// Package theme builds the Quake visual theme for the gogpu/ui widget tree
// (IRONWAIL-SPEC-001 §4.3, ADR-0001). It derives theme.Theme tokens from the
// Quake palette and conchars conventions, and carries a quakeui
// ThemeExtension with the glyph/alpha conventions the console, menu, and HUD
// widgets need.
//
// Token mapping (derived from research 0001 §3-5 and palette.lmp):
//
//   - Background: palette index 0 (black opaque) — Quake's DrawFill(0) fill.
//   - Surface:    palette index 4 (0x3f3f3f) — the status-bar panel color.
//   - OnSurface:  palette index 254 (white) — bright text row.
//   - Primary:    palette index 250 (0xd70000) — Quake's bright red accent.
//   - OnBackground: palette index 253 (0xfff7c7) — warm off-white text.
//
// The remaining semantic tokens keep the DefaultDark() values; the Quake
// palette itself is always available verbatim via the extension's Palette
// field for exact per-index color lookups.
package theme

import (
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

// ExtensionName is the stable name under which the quakeui ThemeExtension is
// registered on the Quake theme.
const ExtensionName = "quakeui"

// PuaBase is the Private-Use-Area base for conchars glyph mapping
// (ADR-0004): rune U+E000 + char for the normal (bronze) row and
// U+E100 + char for the bright row.
const PuaBase = 0xE000

// QuakeTheme returns a dark theme derived from theme.DefaultDark() with the
// Quake palette tokens applied (spec §4.3). The quakeui ThemeExtension is
// registered so widgets can read glyph/alpha/palette conventions.
func QuakeTheme() *theme.Theme {
	t := theme.DefaultDark()
	t.Name = "quake"

	t.Colors.Background = widget.RGB8(0x00, 0x00, 0x00) // palette[0]
	t.Colors.Surface = widget.RGB8(0x3f, 0x3f, 0x3f)    // palette[4]
	t.Colors.OnSurface = widget.RGB8(0xff, 0xff, 0xff)  // palette[254]
	t.Colors.Primary = widget.RGB8(0xd7, 0x00, 0x00)    // palette[250]
	t.Colors.OnBackground = widget.RGB8(0xff, 0xf7, 0xc7) // palette[253]

	t.RegisterExtension(DefaultExtension())
	return t
}

// DefaultExtension returns the quakeui ThemeExtension with the engine's
// cvar defaults and the conchars glyph conventions.
func DefaultExtension() *Extension {
	return &Extension{
		PromptGlyph:     ']',
		ScrollHintGlyph: '^',
		BrightRow:       true,
		MenuBgAlpha:     0.7,  // scr_menubgalpha default
		SbarAlpha:       0.75, // scr_sbaralpha default
		Palette:         draw.DefaultQuakePalette(),
		PuaBase:         PuaBase,
	}
}
