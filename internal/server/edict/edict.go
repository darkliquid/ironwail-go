// Package edict implements Quake entity (edict) allocation, parsing, and
// management for the server.
//
// # Purpose
//
// This package owns the EntityManager that server and savegame loading use
// to manage the edict pool: allocation with a 500ms free-reuse cooldown,
// clearing, spatial unlink, and parsing of map/savegame entity text into
// both the Go Edict struct and the QuakeC VM edict storage.
//
// The package is deliberately dependency-injected: it takes the two root
// server helpers it needs (QCVM edict clearing, default entvar field
// offsets) as constructor parameters, keeping it importable by
// internal/server without a cycle.
//
// # Original C lineage
//
// ED_Alloc / ED_Free / ED_ClearEdict / ED_ParseEdict / ED_ParseGlobals in
// C Ironwail's edict.c and world.c.
package edict

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
)

// StringEntFieldNames lists entity fields whose int32 values are indices
// into the QuakeC VM string table rather than raw numeric values. When
// parsing map entity data, if the key matches one of these names (after
// normalisation) the value string is allocated in the VM via AllocString and
// the returned negative index is stored in the int32 field. Fields not in
// this set are parsed as numeric integers (or hashed via FNV fallback).
var StringEntFieldNames = map[string]struct{}{
	"classname":   {},
	"map":         {},
	"message":     {},
	"model":       {},
	"netname":     {},
	"noise":       {},
	"noise1":      {},
	"noise2":      {},
	"noise3":      {},
	"target":      {},
	"targetname":  {},
	"weaponmodel": {},
}

// ClearQCVMFunc clears the QCVM edict data for a given entity number. It is
// injected from the parent server package (clearQCVMEdictData).
type ClearQCVMFunc func(vm *qc.VM, entNum int)

// Manager manages the entity pool for a Quake server.
// It provides allocation, deallocation, and tracking of game entities.
type Manager struct {
	// edicts is the array of all entities
	edicts []*types.Edict

	// vm is used to resolve QuakeC field types when parsing entities.
	vm *qc.VM

	// fieldDefMap caches O(1) field lookups for parsed entity keys.
	fieldDefMap map[string]fieldDefInfo

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

	// clearFunc clears QCVM edict storage (injected from parent server).
	clearFunc ClearQCVMFunc

	// defaultOffsets maps normalized entvar names to default field offsets
	// (injected from parent server).
	defaultOffsets map[string]int
}

// NewManager creates an entity manager backed by the given edict array.
// clearFunc and defaultOffsets are the root server helpers this package is
// deliberately injected with to avoid an import cycle.
func NewManager(edicts []*types.Edict, vm *qc.VM, maxEdicts, numEdicts, maxClients int, freeTime []float32, clearFunc ClearQCVMFunc, defaultOffsets map[string]int) *Manager {
	return &Manager{
		edicts:         edicts,
		vm:             vm,
		maxEdicts:      maxEdicts,
		numEdicts:      numEdicts,
		maxClients:     maxClients,
		freeTime:       freeTime,
		clearFunc:      clearFunc,
		defaultOffsets: defaultOffsets,
	}
}

// NewEmptyManager creates an entity manager with a fresh edict pool of the
// given capacity, for standalone use (tests, tools).
func NewEmptyManager(maxEdicts, maxClients int, clearFunc ClearQCVMFunc, defaultOffsets map[string]int) *Manager {
	return &Manager{
		edicts:         make([]*types.Edict, maxEdicts),
		maxEdicts:      maxEdicts,
		maxClients:     maxClients,
		freeTime:       make([]float32, maxEdicts),
		clearFunc:      clearFunc,
		defaultOffsets: defaultOffsets,
	}
}

// ED_Alloc allocates a new entity, reusing freed ones when possible.
//
// It tries to avoid reusing entities that were recently freed (< 500ms ago)
// to prevent client interpolation glitches where an entity appears to morph.
//
// Returns the allocated entity index, or error if no entities available.
func (em *Manager) ED_Alloc() (int, error) {
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
		em.edicts[entNum] = &types.Edict{}
	}

	// Set default scale for new entities
	em.edicts[entNum].Scale = 16 // ENTSCALE_DEFAULT

	return entNum, nil
}

// ED_Free marks an entity as free for reuse.
//
// It unlinks the entity from the world, clears key fields,
// and adds it to the free list with the current time.
func (em *Manager) ED_Free(entNum int) error {
	if entNum < 0 || entNum >= em.numEdicts {
		return fmt.Errorf("ED_Free: invalid entity number %d", entNum)
	}

	edict := em.edicts[entNum]
	if edict == nil {
		return fmt.Errorf("ED_Free: nil entity at index %d", entNum)
	}

	em.SV_UnlinkEdict(entNum)

	// Don't add client slots (0 to maxClients-1) to free list
	if entNum >= em.maxClients {
		// Clear key fields
		if em.vm != nil && em.vm.EdictSize > 28 {
			em.vm.SetEInt(entNum, qc.EntFieldModel, 0)
			em.vm.SetEFloat(entNum, qc.EntFieldTakeDamage, 0)
			em.vm.SetEFloat(entNum, qc.EntFieldFrame, 0)
			em.vm.SetEVector(entNum, qc.EntFieldOrigin, [3]float32{})
			em.vm.SetEVector(entNum, qc.EntFieldAngles, [3]float32{})
			em.vm.SetEFloat(entNum, qc.EntFieldNextThink, -1)
			em.vm.SetEFloat(entNum, qc.EntFieldSolid, 0)
		}

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
func (em *Manager) ED_ClearEdict(entNum int) {
	if entNum < 0 || entNum >= em.numEdicts {
		return
	}

	edict := em.edicts[entNum]
	if edict == nil {
		return
	}

	if !edict.Free {
		em.SV_UnlinkEdict(entNum)
	}

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
	if em.vm != nil && em.vm.EdictSize > 28 {
		for i := 0; i < em.vm.EdictSize; i += 4 {
			em.vm.SetEFloat(entNum, i, 0)
		}
	}

	// Reset rendering state
	edict.Alpha = 0  // ENTALPHA_DEFAULT
	edict.Scale = 16 // ENTSCALE_DEFAULT
}
