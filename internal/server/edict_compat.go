// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.

// Package server implements Quake server simulation.
//
// This file now only re-exports the edict-manager surface that moved to
// internal/server/edict (Plan 16b step 16+2.7). The full EntityManager
// implementation, string/parse helpers, and field mapping live in the
// subpackage; root callers use the edict.NewManager constructor with the
// injected QCVM-clear and default-offset dependencies.
package server

import (
	"hash/fnv"
	"strconv"
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
)

// stringEntFieldNames lists entity fields whose int32 values are indices
// into the QuakeC VM string table rather than raw numeric values. Kept in
// root for savegame.go and server_qc_sync.go; the edict subpackage has its
// own copy to stay import-cycle-free.
var stringEntFieldNames = map[string]struct{}{
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

var (
	normalizedNamesMu    sync.RWMutex
	normalizedNamesCache = make(map[string]string, 512)
)

// normalizeFieldName strips underscores and lowercases the input string to
// produce a canonical form suitable for case-insensitive,
// underscore-insensitive field name matching. Shared by root entvar offset
// tables (server_qc_sync.go) and savegame serialization.
func normalizeFieldName(name string) string {
	normalizedNamesMu.RLock()
	if n, ok := normalizedNamesCache[name]; ok {
		normalizedNamesMu.RUnlock()
		return n
	}
	normalizedNamesMu.RUnlock()

	n := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	normalizedNamesMu.Lock()
	normalizedNamesCache[name] = n
	normalizedNamesMu.Unlock()
	return n
}

// The remaining parse helpers (parseVec3, parseFloat32, parseInt32,
// parseStringFallbackInt32) are used by savegame.go and tests; they were
// moved with the edict subpackage and are kept here for those root callers.
// Their implementations were not deleted from the root to avoid churning
// savegame.go and walkable_point_diagnostics_test.go.

// parseStringFallbackInt32 computes an FNV-1a hash of the input string and
// returns it as an int32. Root copy (used by savegame serialization).
func parseStringFallbackInt32(raw string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(raw))
	return int32(h.Sum32())
}

var vec3Replacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

// parseVec3 parses a space-separated "x y z" string into a [3]float32
// vector (root copy, used by savegame/testing).
func parseVec3(raw string) ([3]float32, error) {
	var out [3]float32
	normalized := vec3Replacer.Replace(strings.TrimSpace(raw))
	if normalized == "" {
		return out, nil
	}
	parts := strings.Split(normalized, " ")
	component := 0
	for _, part := range parts {
		if component >= len(out) {
			break
		}
		if part == "" {
			component++
			continue
		}
		v, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return [3]float32{}, err
		}
		out[component] = float32(v)
		component++
	}
	return out, nil
}

// parseFloat32 parses a string token into a float32, trimming whitespace
// (root copy, used by savegame/testing).
func parseFloat32(raw string) (float32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return 0, err
	}
	return float32(v), nil
}

// parseInt32 parses a string into base-10 int32, trimming whitespace
// (root copy, used by savegame/testing).
func parseInt32(raw string) (int32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// Keep the qc + types imports referenced even though the original
// EntityManager moved out; these types are part of the server root surface.
var (
	_ = qc.EntFieldModel
	_ = types.Edict{}
)
