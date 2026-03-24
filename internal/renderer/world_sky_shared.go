package renderer

import (
	"strings"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

type worldSkyFogOverride struct {
	hasValue bool
	value    float32
}

// parseWorldspawnSkyFogOverride parses the worldspawn entity for sky fog override values.
func parseWorldspawnSkyFogOverride(entities []byte) worldSkyFogOverride {
	if len(entities) == 0 {
		return worldSkyFogOverride{}
	}

	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return worldSkyFogOverride{}
	}

	fields := parseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return worldSkyFogOverride{}
	}

	value, ok := parseEntityAlphaField(fields, "skyfog")
	if !ok {
		return worldSkyFogOverride{}
	}

	return worldSkyFogOverride{hasValue: true, value: value}
}

// readWorldSkyFogCvar reads the r_skyfog cvar value with a fallback default.
func readWorldSkyFogCvar(fallback float32) float32 {
	return readWorldAlphaCvar(CvarRSkyFog, fallback)
}

func readWorldFastSkyEnabled() bool {
	return cvar.BoolValue(CvarRFastSky)
}

// resolveWorldSkyFogMix resolves the final sky fog mix factor from the cvar value, worldspawn override, and fog density.
func resolveWorldSkyFogMix(cvarValue float32, override worldSkyFogOverride, fogDensity float32) float32 {
	if fogDensity <= 0 {
		return 0
	}
	skyFog := clamp01(cvarValue)
	if override.hasValue {
		skyFog = clamp01(override.value)
	}
	return skyFog
}

func gogpuWorldSkyFogDensity(worldEntities []byte, fogDensity float32) float32 {
	return resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), parseWorldspawnSkyFogOverride(worldEntities), fogDensity)
}

func resolveWorldSkyTextureIndex(face WorldFace, textureAnimations []*SurfaceTexture, frame int, timeSeconds float64) int32 {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}
	return textureIndex
}

func buildSkyFlatRGBA(alphaLayer []byte) [4]byte {
	var out [4]byte
	if len(alphaLayer) < 4 {
		out[3] = 255
		return out
	}
	var (
		sumR uint64
		sumG uint64
		sumB uint64
		n    uint64
	)
	for i := 0; i+3 < len(alphaLayer); i += 4 {
		if alphaLayer[i+3] == 0 {
			continue
		}
		sumR += uint64(alphaLayer[i+0])
		sumG += uint64(alphaLayer[i+1])
		sumB += uint64(alphaLayer[i+2])
		n++
	}
	if n == 0 {
		out[3] = 255
		return out
	}
	out[0] = byte(sumR / n)
	out[1] = byte(sumG / n)
	out[2] = byte(sumB / n)
	out[3] = 255
	return out
}
