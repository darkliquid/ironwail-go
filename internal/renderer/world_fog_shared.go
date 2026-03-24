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

// blendFogStateTowards blends the previous fog state toward the target fog state by at most maxStep.
// It provides a deterministic one-step transition seam so abrupt fog changes can avoid hard pops.
func blendFogStateTowards(prevColor [3]float32, prevDensity float32, nextColor [3]float32, nextDensity float32, maxStep float32) ([3]float32, float32) {
	if maxStep <= 0 {
		return nextColor, nextDensity
	}

	blendChannel := func(prev, next float32) float32 {
		delta := next - prev
		if delta > maxStep {
			return prev + maxStep
		}
		if delta < -maxStep {
			return prev - maxStep
		}
		return next
	}

	return [3]float32{
			blendChannel(prevColor[0], nextColor[0]),
			blendChannel(prevColor[1], nextColor[1]),
			blendChannel(prevColor[2], nextColor[2]),
		},
		blendChannel(prevDensity, nextDensity)
}
