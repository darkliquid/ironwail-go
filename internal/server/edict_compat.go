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
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// parseVec3 is used by walkable_point_diagnostics_test.go for test vectors.
var vec3Replacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

// parseVec3 parses a space-separated "x y z" string into a types.Vec3
// vector (root copy, used by savegame/testing).
func parseVec3(raw string) (qtypes.Vec3, error) {
	var out qtypes.Vec3
	normalized := vec3Replacer.Replace(strings.TrimSpace(raw))
	if normalized == "" {
		return out, nil
	}
	parts := strings.Split(normalized, " ")
	component := 0
	for _, part := range parts {
		if component >= 3 {
			break
		}
		if part == "" {
			component++
			continue
		}
		v, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return qtypes.Vec3{}, err
		}
		switch component {
		case 0:
			out.X = float32(v)
		case 1:
			out.Y = float32(v)
		case 2:
			out.Z = float32(v)
		}
		component++
	}
	return out, nil
}

// Keep the qc + types imports referenced even though the original
// EntityManager moved out; these types are part of the server root surface.
var (
	_ = qc.EntFieldModel
	_ = types.Edict{}
)
