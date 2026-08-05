package renderer

import "math"

const (
	goGPUWorldDynamicLightQuantStep = float32(1.0 / 32.0)
	goGPUWorldDynamicLightEpsilon   = goGPUWorldDynamicLightQuantStep * 0.25
)

func quantizeGoGPUWorldDynamicLightScalar(value float32) float32 {
	if math.Abs(float64(value)) < float64(goGPUWorldDynamicLightEpsilon) {
		return 0
	}
	return float32(math.Round(float64(value/goGPUWorldDynamicLightQuantStep))) * goGPUWorldDynamicLightQuantStep
}

func quantizeGoGPUWorldDynamicLight(light [3]float32) [3]float32 {
	for i := range light {
		light[i] = quantizeGoGPUWorldDynamicLightScalar(light[i])
	}
	return light
}
