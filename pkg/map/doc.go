// Package mapfile reads and writes Quake .map files (QuakeEd and Valve 220
// texture formats), shared by the qbsp compiler and the bspdec decompiler.
//
// # Original C lineage
//
// The parser is a Go port of ericw-tools common/mapfile.cc (tokenizer,
// entity/brush grammar, both texture-definition forms). The writer emits
// Valve 220 face lines.
//
// The vector and plane types alias pkg/types (Vec3d/Plane64) so geometry
// math stays in one place across the engine, toolchain, and decompiler.
package mapfile

import "github.com/darkliquid/ironwail-go/pkg/types"

// Vec3 is a double-precision 3D vector (alias of types.Vec3d).
type Vec3 = types.Vec3d

// Plane is a double-precision plane (alias of types.Plane64).
type Plane = types.Plane64