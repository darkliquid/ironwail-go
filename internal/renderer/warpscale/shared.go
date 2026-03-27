package warpscale

import "math"

// WaterwarpFOVScale computes the horizontal FOV scale factor for r_waterwarp > 1.
func WaterwarpFOVScale(t float32) float32 {
	return float32(0.97 + math.Sin(float64(t)*1.5)*0.03)
}

// ApplyWaterwarpFOV returns the FOV (in degrees) after applying sinusoidal modulation.
func ApplyWaterwarpFOV(baseFOV, t float32) float32 {
	scale := WaterwarpFOVScale(t)
	halfTan := float32(math.Tan(float64(baseFOV) * math.Pi / 360.0))
	return float32(math.Atan(float64(halfTan)*float64(scale))) * 360.0 / math.Pi
}
