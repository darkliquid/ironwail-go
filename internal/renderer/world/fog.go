package world

import "github.com/darkliquid/ironwail-go/pkg/types"

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
func BlendFogStateTowards(prevColor types.Vec3, prevDensity float32, nextColor types.Vec3, nextDensity float32, maxStep float32) (types.Vec3, float32) {
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

	return types.Vec3{
			X: blendChannel(prevColor.X, nextColor.X),
			Y: blendChannel(prevColor.Y, nextColor.Y),
			Z: blendChannel(prevColor.Z, nextColor.Z),
		},
		blendChannel(prevDensity, nextDensity)
}
