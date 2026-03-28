package renderer

import worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"

func expandLightmapSamples(lighting []byte, lightingRGB bool, lightOfs, sampleCount int) []byte {
	return worldimpl.ExpandLightmapSamples(lighting, lightingRGB, lightOfs, sampleCount)
}
