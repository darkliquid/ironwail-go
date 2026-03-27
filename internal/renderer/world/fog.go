package world

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// FogUniformDensity converts fog density cvar values to the shader uniform scale.
func FogUniformDensity(density float32) float32 {
	const (
		expAdjustment       = 1.20112241
		sphericalCorrection = 0.85
		densityScale        = expAdjustment * sphericalCorrection / 64.0
	)
	density = clamp01(density) * densityScale
	return density * density
}

// BlendFogStateTowards blends previous fog state toward a target by maxStep.
func BlendFogStateTowards(prevColor [3]float32, prevDensity float32, nextColor [3]float32, nextDensity float32, maxStep float32) ([3]float32, float32) {
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
