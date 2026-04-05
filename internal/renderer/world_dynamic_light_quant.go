package renderer

import "math"

const (
	goGPUWorldDynamicLightQuantStep = float32(1.0 / 32.0)
	goGPUWorldDynamicLightEpsilon   = goGPUWorldDynamicLightQuantStep * 0.25
)

func quantizeGoGPUWorldDynamicLight(light [3]float32) [3]float32 {
	for i := range light {
		value := light[i]
		if math.Abs(float64(value)) < float64(goGPUWorldDynamicLightEpsilon) {
			light[i] = 0
			continue
		}
		light[i] = float32(math.Round(float64(value/goGPUWorldDynamicLightQuantStep))) * goGPUWorldDynamicLightQuantStep
	}
	return light
}
