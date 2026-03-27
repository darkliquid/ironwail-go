package world

// ExpandLightmapSamples returns RGB triplets for a face's lightmap samples. It
// preserves explicit RGB lighting from .lit sidecars and expands legacy
// monochrome BSP lightmaps to RGB so the rest of the renderer can treat both
// formats uniformly.
func ExpandLightmapSamples(lighting []byte, lightingRGB bool, lightOfs, sampleCount int) []byte {
	if lightOfs < 0 || sampleCount < 0 {
		return nil
	}
	if lightingRGB {
		end := lightOfs + sampleCount*3
		if end > len(lighting) {
			return nil
		}
		return append([]byte(nil), lighting[lightOfs:end]...)
	}

	end := lightOfs + sampleCount
	if end > len(lighting) {
		return nil
	}
	rawSamples := lighting[lightOfs:end]
	samples := make([]byte, sampleCount*3)
	for i, val := range rawSamples {
		samples[i*3] = val
		samples[i*3+1] = val
		samples[i*3+2] = val
	}
	return samples
}
