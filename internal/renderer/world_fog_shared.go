package renderer

// worldFogUniformDensity converts the fog density cvar value to the uniform value used in the shader's exponential fog formula.
func worldFogUniformDensity(density float32) float32 {
	const (
		expAdjustment       = 1.20112241
		sphericalCorrection = 0.85
		densityScale        = expAdjustment * sphericalCorrection / 64.0
	)
	density = clamp01(density) * densityScale
	return density * density
}
