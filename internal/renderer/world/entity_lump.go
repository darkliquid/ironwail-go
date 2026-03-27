package world

import (
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// ReadAlphaCvar reads a clamped alpha cvar value with a fallback default.
func ReadAlphaCvar(name string, fallback float32) float32 {
	cv := cvar.Get(name)
	if cv == nil {
		return clamp01(fallback)
	}
	return clamp01(cv.Float32())
}

// ParseEntityAlphaField parses a floating-point alpha value from an entity key-value field.
func ParseEntityAlphaField(fields map[string]string, key string) (float32, bool) {
	value, ok := fields[key]
	if !ok {
		value, ok = fields["_"+key]
		if !ok {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, false
	}
	return float32(f), true
}

// ParseEntityBoolField parses a boolean entity field using Quake's convention.
func ParseEntityBoolField(fields map[string]string, key string) (bool, bool) {
	value, ok := fields[key]
	if !ok {
		value, ok = fields["_"+key]
		if !ok {
			return false, false
		}
	}
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return false, false
	}
	return f != 0, true
}

// ParseEntityFields parses key-value pairs from a Quake entity definition string into a map.
func ParseEntityFields(data string) map[string]string {
	fields := make(map[string]string)
	pos := 0
	for {
		key, next, ok := nextQuotedEntityToken(data, pos)
		if !ok {
			break
		}
		value, nextValue, ok := nextQuotedEntityToken(data, next)
		if !ok {
			break
		}
		fields[strings.ToLower(key)] = value
		pos = nextValue
	}
	return fields
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

// nextQuotedEntityToken extracts the next double-quoted string token from Quake entity lump data.
func nextQuotedEntityToken(data string, pos int) (string, int, bool) {
	start := strings.IndexByte(data[pos:], '"')
	if start < 0 {
		return "", pos, false
	}
	start += pos
	end := strings.IndexByte(data[start+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += start + 1
	return data[start+1 : end], end + 1, true
}
