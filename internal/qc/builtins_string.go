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
	vm.SetGString(OFSReturn, fmt.Sprintf("entity %d", e))
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
	argc := vm.ArgC
	if argc <= 0 {
		argc = 2
	}
	var b strings.Builder
	for i := 0; i < argc; i++ {
		b.WriteString(vm.GString(OFSParm0 + i*3))
	}
	vm.SetGString(OFSReturn, b.String())
}

// substringBuiltin extracts a substring.
// QuakeC signature: string(string s, float start, float length) substring
func substringBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	start := int(vm.GFloat(OFSParm0 + 3))
	length := int(vm.GFloat(OFSParm0 + 6))
	slen := len(s)

	if start < 0 {
		start = slen + start
	}
	if length < 0 {
		length = slen - start + (length + 1)
	}
	if start < 0 {
		start = 0
	}
	if start >= slen || length <= 0 {
		vm.SetGString(OFSReturn, "")
		return
	}

	remaining := slen - start
	if length > remaining {
		length = remaining
	}
	vm.SetGString(OFSReturn, s[start:start+length])
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
	argc := vm.ArgC
	if argc <= 0 {
		argc = 1
	}
	var b strings.Builder
	for i := 0; i < argc; i++ {
		b.WriteString(vm.GString(OFSParm0 + i*3))
	}
	vm.SetGString(OFSReturn, b.String())
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
	if idx < 0 {
		idx = len(s) + idx
	}
	if idx < 0 || idx >= len(s) {
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	vm.SetGFloat(OFSReturn, float32(s[idx]))
}

// chr2strBuiltin converts an ASCII code to a single-character string.
// QuakeC signature: string(float c) chr2str
func chr2strBuiltin(vm *VM) {
	argc := vm.ArgC
	if argc <= 0 {
		argc = 1
	}
	var b strings.Builder
	for i := 0; i < argc; i++ {
		c := int(vm.GFloat(OFSParm0 + i*3))
		if c < 0 || c > 255 {
			c = '?'
		}
		b.WriteByte(byte(c))
	}
	vm.SetGString(OFSReturn, b.String())
}
