package draw

import (
	"github.com/darkliquid/ironwail-go/pkg/wad"
)

// StandardQuakePaletteHex is the exact 768-byte RGB palette from Quake's gfx/palette.lmp.
const StandardQuakePaletteHex = wad.StandardQuakePaletteHex

// DefaultQuakePalette returns the decoded 768-byte RGB Quake palette.
func DefaultQuakePalette() []byte {
	return wad.DefaultQuakePalette()
}
