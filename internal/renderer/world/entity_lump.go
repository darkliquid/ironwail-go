package world

import (
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/bsp"
)

// ReadAlphaCvar reads a clamped alpha cvar value with a fallback default.
func ReadAlphaCvar(name string, fallback float32) float32 {
	if pkgCVars == nil {
		return clamp01(fallback)
	}
	cv := pkgCVars.Get(name)
	if cv == nil {
		return clamp01(fallback)
	}
	return clamp01(cv.Float32())
}

// ParseEntityAlphaField parses a floating-point alpha value from an entity key-value field.
func ParseEntityAlphaField(fields map[string]string, key string) (float32, bool) {
	return bsp.Entity(fields).FloatVal(key)
}

// ParseEntityBoolField parses a boolean entity field using Quake's convention.
func ParseEntityBoolField(fields map[string]string, key string) (bool, bool) {
	return bsp.Entity(fields).BoolVal(key)
}

// ParseEntityFields parses key-value pairs from a Quake entity definition string into a map.
func ParseEntityFields(data string) map[string]string {
	clean := strings.TrimSpace(data)
	if !strings.HasPrefix(clean, "{") {
		clean = "{\n" + clean + "\n}"
	}
	ents, err := bsp.ParseEntities(clean)
	if err == nil && len(ents) > 0 {
		return ents[0]
	}
	return make(map[string]string)
}

// FirstEntityLumpObject extracts the first entity block (the worldspawn) from the BSP entity lump.
func FirstEntityLumpObject(data string) (string, bool) {
	start := strings.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(data[start+1:], '}')
	if end < 0 {
		return "", false
	}
	return data[start+1 : start+1+end], true
}
