package renderer

import (
	"github.com/ironwail/ironwail-go/internal/cvar"
	warpscaleimpl "github.com/ironwail/ironwail-go/internal/renderer/warpscale"
)

// readWaterwarpCvar returns the current r_waterwarp value (0, 1, or >1).
func readWaterwarpCvar() float32 {
	cv := cvar.Get(CvarRWaterwarp)
	if cv == nil {
		return 0
	}
	return cv.Float32()
}

// WaterwarpFOV reports whether the FOV-oscillation underwater warp is active
// (r_waterwarp > 1 and the given underwater flag is true).
func WaterwarpFOV(underwaterOrForced bool) bool {
	return underwaterOrForced && readWaterwarpCvar() > 1.0
}

// WaterwarpFOVScale computes the horizontal FOV scale factor for r_waterwarp > 1.
func WaterwarpFOVScale(t float32) float32 {
	return warpscaleimpl.WaterwarpFOVScale(t)
}

// ApplyWaterwarpFOV returns the FOV (in degrees) after applying the r_waterwarp > 1
// sinusoidal modulation.
func ApplyWaterwarpFOV(baseFOV, t float32) float32 {
	return warpscaleimpl.ApplyWaterwarpFOV(baseFOV, t)
}
