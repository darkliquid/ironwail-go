// Package server implements Quake entity management.
//
// This file provides entity allocation, deallocation, and management
// for the Quake server. Entities (edicts) represent game objects
// such as players, monsters, items, and triggers.
//
// # Key Concepts
//
// - Entity 0 is always worldspawn (the level itself)
// - Free entities are tracked for reuse with a 500ms delay
// - Entities have both engine fields and QuakeC-visible fields
package server

import (
	"fmt"
)

// EntityManager manages the entity pool for a Quake server.
// It provides allocation, deallocation, and tracking of game entities.
type EntityManager struct {
	// edicts is the array of all entities
	edicts []*Edict

	// maxEdicts is the maximum number of entities
	maxEdicts int

	// numEdicts is the current number of active entities
	numEdicts int

	// freeList is a list of free entity indices for reuse
	freeList []int

	// freeTime tracks when each entity was freed
	// Used to prevent immediate reuse that can cause client glitches
	freeTime []float32

	// currentTime is the server time for free-time calculations
	currentTime float32

	// maxClients is the number of client slots that should never be freed
	maxClients int
}

// NewEntityManager creates a new entity manager with the given capacity.
func NewEntityManager(maxEdicts, maxClients int) *EntityManager {
	return &EntityManager{
		edicts:     make([]*Edict, maxEdicts),
		maxEdicts:  maxEdicts,
		maxClients: maxClients,
		freeTime:   make([]float32, maxEdicts),
	}
}

// ED_Alloc allocates a new entity, reusing freed ones when possible.
//
// It tries to avoid reusing entities that were recently freed (< 500ms ago)
// to prevent client interpolation glitches where an entity appears to morph.
//
// Returns the allocated entity index, or error if no entities available.
func (em *EntityManager) ED_Alloc() (int, error) {
	// Try to reuse from free list first
	for len(em.freeList) > 0 {
		entNum := em.freeList[0]
		em.freeList = em.freeList[1:]

		// Check if we can reuse this entity (500ms delay)
		freeTime := em.freeTime[entNum]
		if freeTime < 2 || em.currentTime-freeTime > 0.5 {
			// Clear and return the entity
			em.ED_ClearEdict(entNum)
			return entNum, nil
		}

		// This entity was freed too recently, try another one
	}

	// No suitable free entity found, allocate new one
	if em.numEdicts >= em.maxEdicts {
		return 0, fmt.Errorf("ED_Alloc: no free edicts (max_edicts is %d)", em.maxEdicts)
	}

	entNum := em.numEdicts
	em.numEdicts++

	// Initialize new entity
	if em.edicts[entNum] == nil {
		em.edicts[entNum] = &Edict{}
	}

	// Set default scale for new entities
	em.edicts[entNum].Scale = 16 // ENTSCALE_DEFAULT

	return entNum, nil
}

// ED_Free marks an entity as free for reuse.
//
// It unlinks the entity from the world, clears key fields,
// and adds it to the free list with the current time.
func (em *EntityManager) ED_Free(entNum int) error {
	if entNum < 0 || entNum >= em.numEdicts {
		return fmt.Errorf("ED_Free: invalid entity number %d", entNum)
	}

	edict := em.edicts[entNum]
	if edict == nil {
		return fmt.Errorf("ED_Free: nil entity at index %d", entNum)
	}

	// TODO: SV_UnlinkEdict(ed) - unlink from world BSP
	// This requires access to the server's world/bsp data

	// Don't add client slots (0 to maxClients-1) to free list
	if entNum >= em.maxClients {
		// Clear key fields
		if edict.Vars != nil {
			edict.Vars = &EntVars{}
		}
		edict.Vars.Model = 0
		edict.Vars.TakeDamage = 0
		edict.Vars.Frame = 0
		edict.Vars.Origin = [3]float32{}
		edict.Vars.Angles = [3]float32{}
		edict.Vars.NextThink = -1
		edict.Vars.Solid = 0

		// Reset alpha and scale to defaults
		edict.Alpha = 0  // ENTALPHA_DEFAULT
		edict.Scale = 16 // ENTSCALE_DEFAULT

		// Mark as free and record time
		edict.Free = true
		em.freeTime[entNum] = em.currentTime

		// Add to free list
		em.freeList = append(em.freeList, entNum)
	}

	return nil
}

// ED_ClearEdict resets an entity to empty state.
//
// If the entity is in use, it unlinks from the world.
// If it's in the free list, it removes it from there.
// All QuakeC-visible fields are zeroed.
func (em *EntityManager) ED_ClearEdict(entNum int) {
	if entNum < 0 || entNum >= em.numEdicts {
		return
	}

	edict := em.edicts[entNum]
	if edict == nil {
		return
	}

	// TODO: If not free, SV_UnlinkEdict - unlink from world BSP

	// If in free list, remove from it
	if edict.Free {
		// Remove from free list
		for i, idx := range em.freeList {
			if idx == entNum {
				em.freeList = append(em.freeList[:i], em.freeList[i+1:]...)
				break
			}
		}
	}

	// Mark as in use
	edict.Free = false

	// Zero all QuakeC fields
	if edict.Vars != nil {
		edict.Vars = &EntVars{}
	}
	// In Go, the EntVars struct is already zero-initialized on creation

	// Reset rendering state
	edict.Alpha = 0  // ENTALPHA_DEFAULT
	edict.Scale = 16 // ENTSCALE_DEFAULT
}

// ED_ParseGlobals parses global variable key-value pairs from map/savegame data.
//
// This function parses the global variables section of a map file or
// savegame, setting values in the VM's globals array.
//
// The data format is a series of "key" "value" pairs enclosed in braces.
//
// Returns the data pointer after parsing, or an error.
func (em *EntityManager) ED_ParseGlobals(data string, vm interface{}) (string, error) {
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

		// Parse value (quoted string)
		if pos >= len(data) || data[pos] != '\"' {
			return 0, fmt.Errorf("expected quoted value at position %d", pos)
		}
		pos++

		valueStart := pos
		for pos < len(data) && data[pos] != '\"' {
			pos++
		}

		if pos >= len(data) {
			return 0, fmt.Errorf("EOF in value at position %d", valueStart)
		}
		valueStr := data[valueStart:pos]
		pos++

		// TODO: Look up field by name and set appropriate value
		// This requires access to VM's field definitions
		// Common fields to implement:
		// - model, origin, angles, classname, spawnflags
		// TODO: Set value based on field type (float32, vec3, string index)
		fmt.Printf("Parsed: %s = %s\n", keyName, valueStr)

		return pos, nil
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
func (em *EntityManager) ED_ParseEdict(data string, entNum int) (string, error) {
	if entNum < 0 || entNum >= len(em.edicts) {
		return "", fmt.Errorf("ED_ParseEdict: invalid entity number %d", entNum)
	}

	edict := em.edicts[entNum]
	if edict == nil {
		edict = &Edict{}
		em.edicts[entNum] = edict
	}

	// Don't clear entity 0 (worldspawn)
	if entNum > 0 {
		if edict.Vars == nil {
			edict.Vars = &EntVars{}
		}
		// Zero all fields (in Go, we just replace with new struct)
		edict.Vars = &EntVars{}
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
				em.ED_Free(entNum)
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

		// Handle QuakeEd compatibility hacks
		// "angle" -> "angles" (scalar to vector)
		// "light" -> "light_lev"
		finalKeyName := keyName
		if keyName == "angle" {
			finalKeyName = "angles"
			// Wrap scalar in vector format: "0 angle 0"
			value = fmt.Sprintf("0 %s 0", value)
		} else if keyName == "light" {
			finalKeyName = "light_lev"
		}

		// Skip underscore keys (comments)
		if len(finalKeyName) > 0 && finalKeyName[0] == '_' {
			continue
		}

		// TODO: Parse the value into entity fields
		// This requires ED_ParseEpair and field lookup
		fmt.Printf("TODO: Parse entity field %s = %s\n", finalKeyName, value)
	}

	return "", fmt.Errorf("ED_ParseEdict: unexpected end of data")
}

// SetCurrentTime sets the server time for free-time calculations.
func (em *EntityManager) SetCurrentTime(time float32) {
	em.currentTime = time
}

// GetEdict returns the entity at the given index.
func (em *EntityManager) GetEdict(entNum int) *Edict {
	if entNum < 0 || entNum >= len(em.edicts) {
		return nil
	}
	return em.edicts[entNum]
}

// ActiveCount returns the number of active (non-free) entities.
func (em *EntityManager) ActiveCount() int {
	return em.numEdicts - len(em.freeList)
}
