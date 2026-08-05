// Package binary provides low-level binary I/O readers and little-endian buffer helpers.
//
// # Purpose
//
// Read and write primitive byte data types (int16, uint16, int32, float32, fixed-point angles/coords)
// matching Quake's wire protocol and file format layouts.
//
// # Original C lineage
//
// Mirrored from original Quake / Ironwail C sources:
//   - common.c: Byte swapping, little-endian read/write helpers.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/common/binary -count=1
package binary
