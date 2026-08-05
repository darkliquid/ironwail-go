package edict

import (
	"fmt"
	"log/slog"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
)

// SV_UnlinkEdict removes an entity from the spatial partitioning
// doubly-linked list (the "area chain") that the engine uses for
// broad-phase collision and trigger queries. It splices the edict out by
// patching up the prev/next pointers of its neighbours, then sets AreaPrev
// and AreaNext to nil and NumLeafs to 0. After this call the entity is
// invisible to SV_TouchLinks, SV_ClipMoveToEntity, and similar world
// queries until SV_LinkEdict inserts it again at its new position.
func (em *Manager) SV_UnlinkEdict(entNum int) {
	if entNum < 0 || entNum >= em.numEdicts {
		return
	}

	edict := em.edicts[entNum]
	if edict == nil {
		return
	}

	if edict.AreaPrev != nil {
		edict.AreaPrev.AreaNext = edict.AreaNext
	}
	if edict.AreaNext != nil {
		edict.AreaNext.AreaPrev = edict.AreaPrev
	}

	edict.AreaPrev = nil
	edict.AreaNext = nil
	edict.NumLeafs = 0
}

// ED_ParseGlobals parses global variable key-value pairs from map/savegame data.
//
// This function parses the global variables section of a map file or
// savegame, setting values in the VM's globals array.
//
// The data format is a series of "key" "value" pairs enclosed in braces.
//
// Returns the data pointer after parsing, or an error.
func (em *Manager) ED_ParseGlobals(data string, vm any) (string, error) {
	// Find opening brace
	pos := 0
	for pos < len(data) && data[pos] != '{' {
		pos++
	}
	if pos >= len(data) {
		return "", fmt.Errorf("ED_ParseGlobals: no opening brace")
	}
	pos++ // Skip opening brace

	// Parse key-value pairs until closing brace
	for pos < len(data) {
		// Skip whitespace
		for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n') {
			pos++
		}

		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseGlobals: EOF without closing brace")
		}

		if data[pos] == '}' {
			// Done with this section
			return data[pos+1:], nil
		}

		// Parse key name (quoted string)
		if data[pos] != '"' {
			return "", fmt.Errorf("ED_ParseGlobals: expected quoted key at position %d", pos)
		}
		pos++ // Skip opening quote

		keyStart := pos
		for pos < len(data) && data[pos] != '"' {
			pos++
		}
		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseGlobals: EOF in key name")
		}
		keyName := data[keyStart:pos]
		pos++ // Skip closing quote

		// Skip whitespace
		for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n') {
			pos++
		}

		// Parse value (quoted string)
		if pos >= len(data) || data[pos] != '"' {
			return "", fmt.Errorf("ED_ParseGlobals: expected quoted value for key %s", keyName)
		}
		pos++ // Skip opening quote

		valStart := pos
		for pos < len(data) && data[pos] != '"' {
			pos++
		}
		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseGlobals: EOF in value for key %s", keyName)
		}
		value := data[valStart:pos]
		pos++ // Skip closing quote

		if qvm, ok := vm.(*qc.VM); ok {
			em.parseGlobalValue(qvm, keyName, value)
		}
	}

	return data[pos:], nil
}

// ED_ParseEdict parses a single entity's key-value pairs from map data.
//
// This function parses one entity definition from a map file,
// populating the entity's fields. The world entity (entity 0)
// is never skipped.
//
// Returns the data pointer after parsing, or an error.
func (em *Manager) ED_ParseEdict(data string, entNum int) (string, error) {
	if entNum < 0 || entNum >= len(em.edicts) {
		return "", fmt.Errorf("ED_ParseEdict: invalid entity number %d", entNum)
	}

	edict := em.edicts[entNum]
	if edict == nil {
		edict = &types.Edict{}
		em.edicts[entNum] = edict
	}

	// Don't clear entity 0 (worldspawn)
	if entNum > 0 {
		// Zero all fields (in Go, we just replace with new struct)
		// Match C Quake's "clear before parse" behavior for non-world edicts so
		// reused VM slots don't leak QC-only fields into the next entity. Known
		// fields will be mirrored back into QC before spawn; QC-defined fields
		// parsed below are written directly into VM edict storage.
		if em.clearFunc != nil {
			em.clearFunc(em.vm, entNum)
		}
	}

	// Find opening brace
	pos := 0
	for pos < len(data) && data[pos] != '{' {
		pos++
	}
	if pos >= len(data) {
		return "", fmt.Errorf("ED_ParseEdict: no opening brace")
	}
	pos++ // Skip opening brace

	hasData := false

	// Parse key-value pairs until closing brace
	for pos < len(data) {
		// Skip whitespace
		for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n') {
			pos++
		}

		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseEdict: EOF without closing brace")
		}

		if data[pos] == '}' {
			// Done with this entity
			if !hasData {
				// Empty entity - not valid
				if err := em.ED_Free(entNum); err != nil {
					slog.Warn("server: failed to free empty edict", "entNum", entNum, "err", err)
				}
			}
			return data[pos+1:], nil
		}

		// Parse key name (quoted string)
		if data[pos] != '"' {
			return "", fmt.Errorf("ED_ParseEdict: expected quoted key at position %d", pos)
		}
		pos++ // Skip opening quote

		keyStart := pos
		for pos < len(data) && data[pos] != '"' {
			pos++
		}
		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseEdict: EOF in key name")
		}
		keyName := data[keyStart:pos]
		pos++ // Skip closing quote

		// Skip whitespace
		for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t' || data[pos] == '\n') {
			pos++
		}

		// Parse value (quoted string)
		if pos >= len(data) || data[pos] != '"' {
			return "", fmt.Errorf("ED_ParseEdict: expected quoted value for key %s", keyName)
		}
		pos++ // Skip opening quote

		valStart := pos
		for pos < len(data) && data[pos] != '"' {
			pos++
		}
		if pos >= len(data) {
			return "", fmt.Errorf("ED_ParseEdict: EOF in value for key %s", keyName)
		}
		value := data[valStart:pos]
		pos++ // Skip closing quote

		hasData = true

		if keyName == "alpha" {
			if f, err := parseFloat32(value); err == nil {
				edict.Alpha = inet.ENTALPHA_ENCODE(f)
			}
		}

		// QuakeEd compatibility hacks — early map editors used slightly
		// different key names than the engine's EntVars fields:
		//
		//   "angle" → "angles": QuakeEd stored a single yaw rotation as
		//     the key "angle". The engine expects a three-component vector
		//     "pitch yaw roll" in the "angles" field, so the scalar value
		//     is wrapped as "0 <yaw> 0".
		//
		//   "light" → "light_lev": Some editors used "light" for the
		//     brightness key, but the engine field is "light_lev".
		//
		// These rewrites ensure maps authored with different editors load
		// correctly without requiring map authors to update key names.
		finalKeyName := keyName
		switch keyName {
		case "angle":
			finalKeyName = "angles"
			// Wrap scalar in vector format: "0 angle 0"
			value = fmt.Sprintf("0 %s 0", value)
		case "light":
			finalKeyName = "light_lev"
		}

		// Keys that start with an underscore are editor-private metadata
		// (e.g. "_color", "_tb_type"). They carry no game-relevant
		// information and are silently skipped.
		if len(finalKeyName) > 0 && finalKeyName[0] == '_' {
			continue
		}

		if err := em.parseEdictFieldValue(edict, entNum, finalKeyName, value); err != nil {
			return "", fmt.Errorf("ED_ParseEdict: parse field %s: %w", finalKeyName, err)
		}
	}

	return "", fmt.Errorf("ED_ParseEdict: unexpected end of data")
}

// SetCurrentTime sets the server time for free-time calculations.
func (em *Manager) SetCurrentTime(time float32) {
	em.currentTime = time
}

// SetEdicts replaces the backing edict array (used by map loading when the
// server grows its pool via AllocEdict between parses).
func (em *Manager) SetEdicts(edicts []*types.Edict) {
	em.edicts = edicts
}

// SetNumEdicts sets the current allocated edict count (used by map loading
// after the server grows the pool).
func (em *Manager) SetNumEdicts(n int) {
	em.numEdicts = n
}

// Edict returns the entity at the given index.
func (em *Manager) Edict(entNum int) *types.Edict {
	if entNum < 0 || entNum >= len(em.edicts) {
		return nil
	}
	return em.edicts[entNum]
}

// ActiveCount returns the number of active (non-free) entities.
func (em *Manager) ActiveCount() int {
	return em.numEdicts - len(em.freeList)
}

// NumEdicts returns the current number of allocated edict slots.
func (em *Manager) NumEdicts() int {
	return em.numEdicts
}
