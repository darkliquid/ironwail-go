// Package qc provides QuakeC built-in functions.
//
// This file implements string manipulation QuakeC built-ins (etos, strlen,
// strcat, substring, stov, strzone, strunzone) matching C pr_cmds.c.
package qc

import (
	"fmt"
	"strings"
)

// etosBuiltin converts an entity number to a string "entity N".
// QuakeC signature: string(entity e) etos
func etosBuiltin(vm *VM) {
	e := int(vm.GInt(OFSParm0))
	vm.SetGString(OFSReturn, fmt.Sprintf("%d", e))
}

// strlenBuiltin returns the length of a string.
// QuakeC signature: float(string s) strlen
func strlenBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(len(s)))
}

// strcatBuiltin concatenates two strings.
// QuakeC signature: string(string s1, string s2) strcat
func strcatBuiltin(vm *VM) {
	s1 := vm.GString(OFSParm0)
	s2 := vm.GString(OFSParm1)
	vm.SetGString(OFSReturn, s1+s2)
}

// substringBuiltin extracts a substring.
// QuakeC signature: string(string s, float start, float length) substring
func substringBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	start := int(vm.GFloat(OFSParm0 + 3))
	length := int(vm.GFloat(OFSParm0 + 6))

	if start < 0 {
		start = 0
	}
	if start >= len(s) {
		vm.SetGString(OFSReturn, "")
		return
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	if end < start {
		vm.SetGString(OFSReturn, "")
		return
	}
	vm.SetGString(OFSReturn, s[start:end])
}

// stovBuiltin converts a string like "'1 2 3'" to a vector.
// QuakeC signature: vector(string s) stov
func stovBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	// Strip surrounding quotes/apostrophes that QuakeC vectors use.
	s = strings.Trim(s, "' \"")
	var v [3]float32
	fmt.Sscanf(s, "%f %f %f", &v[0], &v[1], &v[2])
	vm.SetGVector(OFSReturn, v)
}

// strzoneBuiltin allocates a permanent copy of a string.
// In Go, strings are immutable and GC'd, so this is effectively a no-op
// that just returns the same string index.
// QuakeC signature: string(string s) strzone
func strzoneBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	vm.SetGString(OFSReturn, s)
}

// strunzoneBuiltin frees a zoned string. No-op in Go (GC handles it).
// QuakeC signature: void(string s) strunzone
func strunzoneBuiltin(vm *VM) {
	// No-op: Go's garbage collector handles string deallocation.
}

// str2chrBuiltin returns the ASCII code of the first character.
// QuakeC signature: float(string s, float index) str2chr
func str2chrBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	idx := int(vm.GFloat(OFSParm0 + 3))
	if idx < 0 || idx >= len(s) {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	vm.SetGFloat(OFSReturn, float32(s[idx]))
}

// chr2strBuiltin converts an ASCII code to a single-character string.
// QuakeC signature: string(float c) chr2str
func chr2strBuiltin(vm *VM) {
	c := int(vm.GFloat(OFSParm0))
	if c < 0 || c > 255 {
		vm.SetGString(OFSReturn, "")
		return
	}
	vm.SetGString(OFSReturn, string(rune(c)))
}
