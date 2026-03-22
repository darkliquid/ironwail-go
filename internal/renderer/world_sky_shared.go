package renderer

import "strings"

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
