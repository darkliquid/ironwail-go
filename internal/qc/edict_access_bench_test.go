package qc

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// BenchmarkEdictFieldAccess is the plan 27.1 baseline for the QCVM edict
// field access hot path (O1). It mirrors qbj3-class load: ~2k edicts, ~20
// scalar/vector accessors each per frame (origin, velocity, angles, mins,
// maxs, solid, movetype, flags, nextthink, health, ...).
//
// The engine calls these ~40k+ times per server frame on busy maps; the
// accessors currently re-derive EdictData (NumEdicts/EdictSize bounds) + do a
// manual little-endian unpack on every call. This benchmark is the baseline
// gate for plan 27.2's per-VM precomputed offset table.
func BenchmarkEdictFieldAccess(b *testing.B) {
	const numEdicts = 2048
	vm := NewVM()
	vm.EdictSize = 28 + 128*4
	vm.MaxEdicts = numEdicts
	vm.NumEdicts = numEdicts
	vm.Edicts = make([]byte, vm.EdictSize*numEdicts)
	for i := 0; i < numEdicts; i++ {
		buf := vm.Edicts[i*vm.EdictSize+28 : i*vm.EdictSize+28+64]
		for j := range buf {
			buf[j] = byte(i + j)
		}
	}

	// Simulate one "physics read-modify-write" pass over the edict table:
	// read origin/velocity/flags/nextthink/health, write back a float/vector.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for e := 1; e < numEdicts; e++ {
			org := vm.EVector(e, EntFieldOrigin)
			vel := vm.EVector(e, EntFieldVelocity)
			flags := vm.EFloat(e, EntFieldFlags)
			next := vm.EFloat(e, EntFieldNextThink)
			health := vm.EFloat(e, EntFieldHealth)
			_ = org.X + vel.Y + flags + next + health
			vm.SetEFloat(e, EntFieldHealth, 100)
			vm.SetEVector(e, EntFieldOrigin, types.Vec3{X: org.X + 1, Y: org.Y, Z: org.Z})
		}
	}
}

// BenchmarkEdictFieldAccessSingle reads/writes one field per edict — the
// lowest-cost unit, useful for isolating per-call overhead of the bounds
// checks + LE decode.
func BenchmarkEdictFieldAccessSingle(b *testing.B) {
	const numEdicts = 2048
	vm := NewVM()
	vm.EdictSize = 28 + 128*4
	vm.MaxEdicts = numEdicts
	vm.NumEdicts = numEdicts
	vm.Edicts = make([]byte, vm.EdictSize*numEdicts)
	for i := 0; i < numEdicts; i++ {
		bits := math.Float32bits(float32(i) * 0.5)
		buf := vm.Edicts[i*vm.EdictSize+28+EntFieldHealth*4 : i*vm.EdictSize+28+EntFieldHealth*4+4]
		buf[0] = byte(bits)
		buf[1] = byte(bits >> 8)
		buf[2] = byte(bits >> 16)
		buf[3] = byte(bits >> 24)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for e := 1; e < numEdicts; e++ {
			h := vm.EFloat(e, EntFieldHealth)
			vm.SetEFloat(e, EntFieldHealth, h+1)
		}
	}
}
