package bsp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// Entity represents key-value attributes for a single map entity in a BSP file.
type Entity map[string]string

// String returns the string value for key. It checks both the literal key
// and its case-insensitive variant, as well as with/without a leading underscore
// (e.g. looking up "wateralpha" also checks "_wateralpha").
func (e Entity) String(key string) string {
	if val, ok := e.lookup(key); ok {
		return val
	}
	return ""
}

// Has reports whether the entity contains the specified key (case-insensitive,
// tolerating leading underscores).
func (e Entity) Has(key string) bool {
	_, ok := e.lookup(key)
	return ok
}

// Vec3 parses a 3D vector ("X Y Z") from the specified key.
func (e Entity) Vec3(key string) (types.Vec3, bool) {
	val, ok := e.lookup(key)
	if !ok {
		return types.Vec3{}, false
	}
	fields := strings.Fields(val)
	if len(fields) != 3 {
		return types.Vec3{}, false
	}
	x, err1 := strconv.ParseFloat(fields[0], 32)
	y, err2 := strconv.ParseFloat(fields[1], 32)
	z, err3 := strconv.ParseFloat(fields[2], 32)
	if err1 != nil || err2 != nil || err3 != nil {
		return types.Vec3{}, false
	}
	return types.Vec3{X: float32(x), Y: float32(y), Z: float32(z)}, true
}

// Float parses a float32 value from key, returning defaultVal if missing or invalid.
func (e Entity) Float(key string, defaultVal float32) float32 {
	val, ok := e.lookup(key)
	if !ok {
		return defaultVal
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 32)
	if err != nil {
		return defaultVal
	}
	return float32(f)
}

// Int parses an integer from key, returning defaultVal if missing or invalid.
func (e Entity) Int(key string, defaultVal int) int {
	val, ok := e.lookup(key)
	if !ok {
		return defaultVal
	}
	i, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		// Try parsing as float first (e.g. "300.0")
		if f, err2 := strconv.ParseFloat(strings.TrimSpace(val), 64); err2 == nil {
			return int(f)
		}
		return defaultVal
	}
	return i
}

// Bool parses a boolean value from key according to Quake engine conventions.
// Truthy values: "1", "true", "yes", "on", or non-zero numbers.
// Falsy values: "0", "false", "no", "off".
func (e Entity) Bool(key string, defaultVal bool) bool {
	if val, ok := e.BoolVal(key); ok {
		return val
	}
	return defaultVal
}

// FloatVal parses a float32 value from key, returning (value, true) if present and valid,
// or (0, false) if missing or not a valid number.
func (e Entity) FloatVal(key string) (float32, bool) {
	val, ok := e.lookup(key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 32)
	if err != nil {
		return 0, false
	}
	return float32(f), true
}

// IntVal parses an integer from key, returning (value, true) if present and valid,
// or (0, false) if missing or not a valid integer.
func (e Entity) IntVal(key string) (int, bool) {
	val, ok := e.lookup(key)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		if f, err2 := strconv.ParseFloat(strings.TrimSpace(val), 64); err2 == nil {
			return int(f), true
		}
		return 0, false
	}
	return i, true
}

// BoolVal parses a boolean value from key, returning (value, true) if present and recognized,
// or (false, false) if missing or unrecognized.
func (e Entity) BoolVal(key string) (bool, bool) {
	val, ok := e.lookup(key)
	if !ok {
		return false, false
	}
	val = strings.ToLower(strings.TrimSpace(val))
	switch val {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f != 0, true
	}
	return false, false
}

func (e Entity) lookup(key string) (string, bool) {
	if len(e) == 0 {
		return "", false
	}
	clean := strings.ToLower(strings.TrimPrefix(key, "_"))
	if val, ok := e[clean]; ok {
		return val, true
	}
	if val, ok := e["_"+clean]; ok {
		return val, true
	}
	// Direct literal lookup fallback
	if val, ok := e[key]; ok {
		return val, true
	}
	return "", false
}

// ParseEntities parses the text lump of Quake map entities into a slice of Entity objects.
func ParseEntities(entityLumpData string) ([]Entity, error) {
	var entities []Entity
	pos := 0
	length := len(entityLumpData)

	for pos < length {
		// Skip whitespace and comments
		pos = skipWhitespaceAndComments(entityLumpData, pos)
		if pos >= length {
			break
		}

		if entityLumpData[pos] != '{' {
			// Tolerate null byte terminator at end of lump
			if entityLumpData[pos] == 0 {
				break
			}
			return nil, fmt.Errorf("expected '{' at offset %d, got %q", pos, entityLumpData[pos])
		}
		pos++ // skip '{'

		ent := make(Entity)
		closed := false

		for pos < length {
			pos = skipWhitespaceAndComments(entityLumpData, pos)
			if pos >= length {
				break
			}

			if entityLumpData[pos] == '}' {
				pos++
				closed = true
				break
			}

			// Read key
			key, next, ok := nextQuotedString(entityLumpData, pos)
			if !ok {
				return nil, fmt.Errorf("expected quoted key at offset %d", pos)
			}
			pos = skipWhitespaceAndComments(entityLumpData, next)

			// Read value
			val, nextVal, ok := nextQuotedString(entityLumpData, pos)
			if !ok {
				return nil, fmt.Errorf("expected quoted value for key %q at offset %d", key, pos)
			}
			pos = nextVal

			cleanKey := strings.ToLower(strings.TrimSpace(key))
			// Store with both cleaned key (without _) and original lowercased key
			ent[cleanKey] = val
			cleanNoUnderscore := strings.TrimPrefix(cleanKey, "_")
			if cleanNoUnderscore != cleanKey {
				ent[cleanNoUnderscore] = val
			}
		}

		if !closed {
			return nil, fmt.Errorf("unclosed entity block (missing '}')")
		}

		entities = append(entities, ent)
	}

	return entities, nil
}

// ParseFirstEntity parses and returns the first entity block (usually worldspawn).
func ParseFirstEntity(entityLumpData string) (Entity, bool) {
	start := strings.IndexByte(entityLumpData, '{')
	if start < 0 {
		return nil, false
	}
	end := strings.IndexByte(entityLumpData[start+1:], '}')
	if end < 0 {
		return nil, false
	}
	block := entityLumpData[start : start+1+end+1]
	ents, err := ParseEntities(block)
	if err != nil || len(ents) == 0 {
		return nil, false
	}
	return ents[0], true
}

func skipWhitespaceAndComments(data string, pos int) int {
	length := len(data)
	for pos < length {
		c := data[pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == 0 {
			pos++
			continue
		}
		// Line comment //
		if c == '/' && pos+1 < length && data[pos+1] == '/' {
			pos += 2
			for pos < length && data[pos] != '\n' && data[pos] != '\r' {
				pos++
			}
			continue
		}
		break
	}
	return pos
}

func nextQuotedString(data string, pos int) (string, int, bool) {
	length := len(data)
	if pos >= length || data[pos] != '"' {
		return "", pos, false
	}
	pos++ // skip opening quote
	start := pos
	for pos < length && data[pos] != '"' {
		pos++
	}
	if pos >= length {
		return "", pos, false
	}
	val := data[start:pos]
	pos++ // skip closing quote
	return val, pos, true
}
