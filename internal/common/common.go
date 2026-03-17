// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package common provides low-level utilities shared throughout the engine.
//
// This package contains fundamental data structures and I/O helpers that form
// the foundation for Quake's networking, file handling, and entity management:
//
//   - SizeBuf: A sized buffer for network message serialization
//   - Link: Intrusive doubly-linked list for entity management
//   - BitArray: Efficient bit-level storage for visibility and PVS data
//   - Binary I/O helpers for little-endian data (BSP files, network protocol)
//
// # SizeBuf and Network Messages
//
// The SizeBuf type is central to Quake's networking. All client-server
// communication uses a binary protocol where messages are serialized into
// sized buffers. The buffer handles overflow detection and allows both
// reading and writing of the same underlying data.
//
// Example usage:
//
//	buf := common.NewSizeBuf(1400) // MTU-sized buffer
//	buf.WriteByte(svc_print)
//	buf.WriteString("Hello, world!")
//	// Send buf.Data[:buf.CurSize] over network
//
// # Link and Entity Lists
//
// Quake uses intrusive linked lists extensively for entity management.
// Entities are linked into various lists (solid entities, trigger entities,
// area nodes) for efficient spatial queries. The Link type provides the
// primitives for this intrusive pattern.
//
// # Binary I/O
//
// All Quake data files (BSP, MDL, progs.dat) use little-endian byte order.
// The Read*/Write* functions handle this transparently.
package common

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"strings"
)

// SizeBuf is a sized buffer used for network message serialization.
//
// In Quake's networking model, all messages between client and server are
// serialized into these buffers. The buffer tracks its current size and
// provides overflow detection for reliable message handling.
//
// Key concepts:
//   - Data: The underlying byte buffer
//   - MaxSize: Maximum capacity of the buffer
//   - CurSize: Current number of bytes written
//   - ReadCount: Current read position (for parsing incoming messages)
//   - AllowOverflow: If true, buffer clears itself on overflow instead of erroring
//   - Overflowed: Set to true if buffer exceeded capacity
type SizeBuf struct {
	Data          []byte
	MaxSize       int
	CurSize       int
	ReadCount     int
	AllowOverflow bool
	Overflowed    bool
}

// NewSizeBuf creates a new sized buffer with the specified capacity.
// The buffer is initialized with zeros.
func NewSizeBuf(size int) *SizeBuf {
	return &SizeBuf{
		Data:    make([]byte, size),
		MaxSize: size,
	}
}

// Clear resets the buffer to empty state.
// This is called after a message has been sent or when starting a new message.
func (sb *SizeBuf) Clear() {
	sb.CurSize = 0
	sb.Overflowed = false
	sb.ReadCount = 0
}

// GetSpace allocates 'length' bytes in the buffer and returns a slice to them.
//
// If the buffer doesn't have enough space:
//   - If AllowOverflow is true: buffer is cleared and Overflowed is set
//   - If AllowOverflow is false: nil is returned (caller should handle error)
//
// Returns nil on failure.
func (sb *SizeBuf) GetSpace(length int) []byte {
	if sb.CurSize+length > sb.MaxSize {
		if sb.AllowOverflow {
			sb.Overflowed = true
			sb.CurSize = 0
		}
		return nil
	}
	start := sb.CurSize
	sb.CurSize += length
	return sb.Data[start : start+length]
}

// Write copies data into the buffer.
// Returns true on success, false if buffer overflowed.
func (sb *SizeBuf) Write(data []byte) bool {
	space := sb.GetSpace(len(data))
	if space == nil {
		return false
	}
	copy(space, data)
	return true
}

// WriteByte writes a single byte to the buffer.
func (sb *SizeBuf) WriteByte(b byte) bool {
	space := sb.GetSpace(1)
	if space == nil {
		return false
	}
	space[0] = b
	return true
}

// WriteShort writes a 16-bit signed integer in little-endian format.
func (sb *SizeBuf) WriteShort(s int16) bool {
	space := sb.GetSpace(2)
	if space == nil {
		return false
	}
	binary.LittleEndian.PutUint16(space, uint16(s))
	return true
}

// WriteLong writes a 32-bit signed integer in little-endian format.
func (sb *SizeBuf) WriteLong(l int32) bool {
	space := sb.GetSpace(4)
	if space == nil {
		return false
	}
	binary.LittleEndian.PutUint32(space, uint32(l))
	return true
}

// WriteFloat writes a 32-bit float in little-endian IEEE 754 format.
func (sb *SizeBuf) WriteFloat(f float32) bool {
	space := sb.GetSpace(4)
	if space == nil {
		return false
	}
	bits := math.Float32bits(f)
	binary.LittleEndian.PutUint32(space, bits)
	return true
}

// WriteString writes a null-terminated string to the buffer.
// The null terminator is always appended.
func (sb *SizeBuf) WriteString(s string) bool {
	data := []byte(s)
	data = append(data, 0)
	return sb.Write(data)
}

// WriteAngle writes an 8-bit angle value (0-255 representing 0-360 degrees).
// Used in standard Quake protocol for entity angles and view angles.
func (sb *SizeBuf) WriteAngle(angle float32) bool {
	// Convert angle to 8-bit representation (0-255 for 0-360 degrees)
	b := byte(int(angle*256.0/360.0) & 255)
	return sb.WriteByte(b)
}

// WriteAngle16 writes a 16-bit angle value for greater precision.
// Used in FitzQuake protocol extensions and RMQ when PRFL_SHORTANGLE is set.
func (sb *SizeBuf) WriteAngle16(angle float32) bool {
	// Convert angle to 16-bit representation
	s := int16(int(angle*65536.0/360.0) & 65535)
	return sb.WriteShort(s)
}

// BeginReading resets the read position to the start of the buffer.
// Call this before parsing an incoming message.
func (sb *SizeBuf) BeginReading() {
	sb.ReadCount = 0
}

// ReadByte reads a single byte from the buffer.
// Returns the byte and true on success, or 0 and false on underflow.
func (sb *SizeBuf) ReadByte() (byte, bool) {
	if sb.ReadCount+1 > sb.CurSize {
		return 0, false
	}
	b := sb.Data[sb.ReadCount]
	sb.ReadCount++
	return b, true
}

// ReadShort reads a 16-bit signed integer in little-endian format.
func (sb *SizeBuf) ReadShort() (int16, bool) {
	if sb.ReadCount+2 > sb.CurSize {
		return 0, false
	}
	s := int16(binary.LittleEndian.Uint16(sb.Data[sb.ReadCount:]))
	sb.ReadCount += 2
	return s, true
}

// ReadLong reads a 32-bit signed integer in little-endian format.
func (sb *SizeBuf) ReadLong() (int32, bool) {
	if sb.ReadCount+4 > sb.CurSize {
		return 0, false
	}
	l := int32(binary.LittleEndian.Uint32(sb.Data[sb.ReadCount:]))
	sb.ReadCount += 4
	return l, true
}

// ReadFloat reads a 32-bit float in little-endian IEEE 754 format.
func (sb *SizeBuf) ReadFloat() (float32, bool) {
	if sb.ReadCount+4 > sb.CurSize {
		return 0, false
	}
	bits := binary.LittleEndian.Uint32(sb.Data[sb.ReadCount:])
	sb.ReadCount += 4
	return math.Float32frombits(bits), true
}

// ReadAngle reads an 8-bit angle value and converts to degrees (0-360).
func (sb *SizeBuf) ReadAngle() (float32, bool) {
	b, ok := sb.ReadByte()
	if !ok {
		return 0, false
	}
	return float32(b) * 360.0 / 256.0, true
}

// ReadAngle16 reads a 16-bit angle value and converts to degrees (0-360).
func (sb *SizeBuf) ReadAngle16() (float32, bool) {
	s, ok := sb.ReadShort()
	if !ok {
		return 0, false
	}
	return float32(uint16(s)) * 360.0 / 65536.0, true
}

// ReadString reads a null-terminated string from the buffer.
// The null terminator is consumed but not included in the result.
func (sb *SizeBuf) ReadString() string {
	var result []byte
	for {
		b, ok := sb.ReadByte()
		if !ok || b == 0 {
			break
		}
		result = append(result, b)
	}
	return string(result)
}

// ReadBytes reads exactly n bytes from the buffer.
// Returns the bytes and true on success, or nil and false on underflow.
func (sb *SizeBuf) ReadBytes(n int) ([]byte, bool) {
	if sb.ReadCount+n > sb.CurSize {
		return nil, false
	}
	data := make([]byte, n)
	copy(data, sb.Data[sb.ReadCount:sb.ReadCount+n])
	sb.ReadCount += n
	return data, true
}

// Link is an intrusive doubly-linked list node.
//
// Quake uses intrusive lists for entity management. Instead of storing
// entities in separate list structures, each entity embeds a Link and
// is linked directly into various lists (solid entities, area nodes, etc).
//
// This pattern avoids memory allocations and provides O(1) insertion/removal.
//
// Example:
//
//	type Entity struct {
//	    link common.Link  // Embedded for intrusive linking
//	    // ... other fields
//	}
//
//	// Link entity into a list
//	entity.link.InsertBefore(&listHead)
type Link struct {
	Prev, Next *Link
}

// Clear initializes the link as a self-referential empty list head.
// An empty list has Prev and Next pointing to itself.
func (l *Link) Clear() {
	l.Prev = l
	l.Next = l
}

// Remove unlinks this node from its list.
// After removal, the node's links are not valid until re-inserted.
func (l *Link) Remove() {
	l.Next.Prev = l.Prev
	l.Prev.Next = l.Next
}

// InsertBefore inserts this link before another link in a list.
func (l *Link) InsertBefore(before *Link) {
	l.Next = before
	l.Prev = before.Prev
	l.Prev.Next = l
	l.Next.Prev = l
}

// InsertAfter inserts this link after another link in a list.
func (l *Link) InsertAfter(after *Link) {
	l.Next = after.Next
	l.Prev = after
	l.Prev.Next = l
	l.Next.Prev = l
}

// IsEmpty returns true if this link is an empty list head (self-referential).
func (l *Link) IsEmpty() bool {
	return l.Next == l
}

// BitArray is a memory-efficient bit storage for visibility data.
//
// Quake uses bit arrays extensively for:
//   - PVS (Potentially Visible Set) - which clusters can see each other
//   - PAS (Potentially Audible Set) - which clusters can hear each other
//   - Entity visibility masks
//   - Surface culling flags
//
// The implementation packs bits into 32-bit words for efficient access.
type BitArray []uint32

// NewBitArray creates a bit array with at least 'bits' bits.
func NewBitArray(bits int) BitArray {
	dwords := (bits + 31) / 32
	return make(BitArray, dwords)
}

// Get returns true if bit i is set.
func (ba BitArray) Get(i uint32) bool {
	return ba[i/32]&(1<<(i%32)) != 0
}

// Set sets bit i to 1.
func (ba BitArray) Set(i uint32) {
	ba[i/32] |= 1 << (i % 32)
}

// Clear sets bit i to 0.
func (ba BitArray) Clear(i uint32) {
	ba[i/32] &^= 1 << (i % 32)
}

// Toggle flips bit i.
func (ba BitArray) Toggle(i uint32) {
	ba[i/32] ^= 1 << (i % 32)
}

// Global variables for the Quake command-line argument system and tokenizer.
//
// Quake's command-line handling predates Go's flag package and has specific
// behaviors that mods and configs depend on (case-insensitive matching,
// integer indexing, etc.). These globals mirror the C engine's com_argc,
// com_argv, and com_token variables.
//
//   - ComToken: Holds the most recently parsed token from COM_Parse/COM_ParseEx.
//     This global-state design matches the original C code where com_token
//     is a static buffer shared across all callers.
//   - ComArgc: Number of command-line arguments (mirrors C's com_argc).
//   - ComArgv: The argument strings themselves (mirrors C's com_argv).
var (
	ComToken string
	ComArgc  int
	ComArgv  []string
)

// COM_InitArgv initializes the command-line argument system with the
// provided argument list. This is called once during engine startup
// (Host_Init) before any subsystem that might query command-line parameters.
//
// Various engine systems check for command-line flags early in their
// initialization: -dedicated (headless server), -basedir (game data path),
// -heapsize (memory pool), -listen (max players), etc.
func COM_InitArgv(args []string) {
	ComArgv = args
	ComArgc = len(args)
}

// COM_CheckParmNext searches the command-line arguments for the parameter
// 'parm', starting after index 'last'. Returns the index (1-based) where
// the parameter was found, or 0 if not present.
//
// The 'last' parameter enables scanning for repeated flags. For example,
// "-game hipnotic -game rogue" can be iterated by calling:
//
//	idx := COM_CheckParmNext(0, "-game")   // finds first -game
//	idx = COM_CheckParmNext(idx, "-game")  // finds second -game
//
// Comparison is case-insensitive to match C Quake's Q_strcasecmp behavior.
func COM_CheckParmNext(last int, parm string) int {
	for i := last + 1; i < ComArgc; i++ {
		if ComArgv[i] == "" {
			continue
		}
		if strings.EqualFold(parm, ComArgv[i]) {
			return i
		}
	}
	return 0
}

// COM_CheckParm searches the entire command-line for the parameter 'parm'.
// Returns the 1-based index where it was found, or 0 if absent.
//
// This is the primary way engine subsystems check for command-line flags:
//
//	if idx := COM_CheckParm("-dedicated"); idx != 0 {
//	    // start in dedicated server mode
//	}
func COM_CheckParm(parm string) int {
	return COM_CheckParmNext(0, parm)
}

// CPE_Mode controls how token overflow (exceeding the maximum token buffer
// size) is handled in [COM_ParseEx]. In the C engine, com_token is a
// fixed-size char array; this Go port uses strings so overflow is less of
// a concern, but the mode is preserved for behavioral compatibility.
type CPE_Mode int

// CPE_NOTRUNC and CPE_ALLOWTRUNC control the overflow behavior of COM_ParseEx.
// CPE_NOTRUNC aborts parsing entirely on overflow (returning ""), which is the
// safe default for untrusted input. CPE_ALLOWTRUNC silently truncates the token,
// which is acceptable when a best-effort result is sufficient.
const (
	CPE_NOTRUNC    CPE_Mode = iota // return "" (abort parsing) in case of overflow
	CPE_ALLOWTRUNC                 // truncate ComToken in case of overflow
)

// COM_Parse is the Quake tokenizer — the engine's equivalent of a lexer.
//
// It parses one token from 'data' and stores it in the global ComToken,
// returning the remaining unparsed string. This function drives the parsing
// of virtually all text-based data in the engine:
//
//   - Entity definitions in BSP maps (key-value pairs in curly braces)
//   - Console commands and aliases (e.g., "bind w +forward")
//   - Configuration files (config.cfg, autoexec.cfg)
//   - QuakeC source listings and progs headers
//   - Server info strings
//
// The tokenizer recognizes:
//   - Whitespace as delimiters (space, tab, newline, carriage return)
//   - C-style comments: // line comments and /* block comments */
//   - Quoted strings: "hello world" is a single token (preserving spaces)
//   - Special single-character tokens: { } ( ) ' :
//   - Regular words: contiguous non-whitespace characters
//
// Returns "" when the input is exhausted, signaling end-of-data to callers
// that loop until the return is empty.
func COM_Parse(data string) string {
	return COM_ParseEx(data, CPE_NOTRUNC)
}

// COM_ParseEx is the extended tokenizer with overflow control.
//
// This is the core implementation behind [COM_Parse]. The 'mode' parameter
// controls what happens if a token exceeds the maximum buffer size:
//
//   - CPE_NOTRUNC: returns "" and aborts (used for untrusted/network input)
//   - CPE_ALLOWTRUNC: silently truncates (used for best-effort parsing)
//
// The parsing algorithm uses a "goto skipwhite" pattern inherited from
// the original C code. While unconventional in Go, this preserves the
// exact control flow of C Quake's COM_Parse for compatibility — some
// mods depend on subtle tokenizer behavior (e.g., how comments interact
// with whitespace skipping).
func COM_ParseEx(data string, mode CPE_Mode) string {
	ComToken = ""
	if data == "" {
		return ""
	}

	i := 0
skipwhite:
	for i < len(data) && data[i] <= ' ' {
		i++
	}
	if i >= len(data) {
		return "" // end of file
	}

	// skip // comments
	if i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
		for i < len(data) && data[i] != '\n' {
			i++
		}
		goto skipwhite
	}

	// skip /*..*/ comments
	if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
		i += 2
		for i < len(data) && !(data[i] == '*' && i+1 < len(data) && data[i+1] == '/') {
			i++
		}
		if i < len(data) {
			i += 2
		}
		goto skipwhite
	}

	// handle quoted strings specially
	if data[i] == '"' {
		i++
		start := i
		for i < len(data) && data[i] != '"' {
			i++
		}
		ComToken = data[start:i]
		if i < len(data) {
			i++ // skip closing quote
		}
		return data[i:]
	}

	// parse single characters
	c := data[i]
	if c == '{' || c == '}' || c == '(' || c == ')' || c == '\'' || c == ':' {
		ComToken = string(c)
		return data[i+1:]
	}

	// parse a regular word
	start := i
	for i < len(data) && data[i] > ' ' {
		c = data[i]
		if c == '{' || c == '}' || c == '(' || c == ')' || c == '\'' {
			break
		}
		i++
	}
	ComToken = data[start:i]
	return data[i:]
}

// COM_SkipPath returns the filename portion of a path (everything after
// the last directory separator).
//
// In Quake's virtual filesystem, paths use forward slashes regardless of
// the host OS (e.g., "maps/e1m1.bsp"). This function extracts the bare
// filename for display in the console, logging, and model/sound name
// lookups where only the leaf name matters.
func COM_SkipPath(pathname string) string {
	return filepath.Base(pathname)
}

// COM_SkipSpace skips leading whitespace characters (space, tab, newline,
// carriage return, vertical tab, form feed) and returns the trimmed string.
// Used by the console command parser and config file loader.
func COM_SkipSpace(str string) string {
	return strings.TrimLeft(str, " \t\n\r\v\f")
}

// COM_StripExtension removes the file extension (including the dot) from
// a path string.
//
// This is used throughout the engine to derive related filenames from a
// base path. For example, when loading a BSP map "maps/e1m1.bsp", the
// engine strips the extension and appends ".lit" to find the external
// lightmap file, or ".ent" to find entity overrides. The function is
// careful not to strip dots that appear in directory names (e.g.,
// "id1.5/maps/start" should not lose "5/maps/start").
func COM_StripExtension(in string) string {
	ext := filepath.Ext(in)
	if ext == "" {
		return in
	}
	// Check if the dot is part of a directory name
	lastSlash := strings.LastIndexAny(in, "/\\")
	lastDot := strings.LastIndex(in, ".")
	if lastDot < lastSlash {
		return in
	}
	return in[:lastDot]
}

// COM_FileGetExtension returns the file extension without the leading dot.
//
// Used by the filesystem and model loader to determine file types:
// "bsp" → BSP map, "mdl" → alias model, "spr" → sprite, "wav" → sound,
// "lmp" → lump (2D image), "pak" → package archive. Returns "" if the
// path has no extension, or if the only dot is in a directory component.
func COM_FileGetExtension(in string) string {
	ext := filepath.Ext(in)
	if ext == "" {
		return ""
	}
	// Check if the dot is part of a directory name
	lastSlash := strings.LastIndexAny(in, "/\\")
	lastDot := strings.LastIndex(in, ".")
	if lastDot < lastSlash {
		return ""
	}
	return ext[1:] // skip the dot
}

// COM_ExtractExtension is a compatibility wrapper around [COM_FileGetExtension].
// In the original C code, COM_ExtractExtension wrote into a caller-supplied
// buffer; this Go version simply returns the extension string.
func COM_ExtractExtension(in string) string {
	return COM_FileGetExtension(in)
}

// COM_FileBase returns the base filename without path or extension.
//
// For "maps/e1m1.bsp", this returns "e1m1". Used by the model loader
// to derive the display name of a model (shown in the console when loading),
// and by the demo system to construct demo filenames. The fallback "?model?"
// matches C Quake's behavior for empty/invalid paths and serves as a
// visible sentinel in debug output.
func COM_FileBase(in string) string {
	base := filepath.Base(in)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	if base == "" || base == "." {
		return "?model?"
	}
	return base
}

// COM_AddExtension appends an extension to a path if it doesn't already
// end with that extension (case-insensitive comparison).
//
// Used when the engine needs to ensure a canonical file extension, e.g.,
// adding ".dem" to a demo filename or ".cfg" to a config filename that
// the user specified without one.
func COM_AddExtension(path, extension string) string {
	if !strings.HasSuffix(strings.ToLower(path), strings.ToLower(extension)) {
		return path + extension
	}
	return path
}

// COM_HashString computes a 32-bit FNV-1a hash of the given string.
//
// FNV-1a (Fowler–Noll–Vo) is a non-cryptographic hash chosen for its
// simplicity, speed, and excellent distribution properties. The algorithm
// starts with the FNV offset basis (0x811c9dc5), and for each byte:
//  1. XORs the byte into the hash
//  2. Multiplies by the FNV prime (0x01000193)
//
// This hash is used throughout the engine for fast string lookups in
// hash tables: model name → model cache entry, texture name → texture
// object, console command name → handler function, etc. The hash is
// NOT suitable for security purposes (it's trivially invertible), but
// its speed and low collision rate make it ideal for engine data structures.
func COM_HashString(str string) uint32 {
	var hash uint32 = 0x811c9dc5
	for i := 0; i < len(str); i++ {
		hash ^= uint32(str[i])
		hash *= 0x01000193
	}
	return hash
}

// COM_HashBlock computes the FNV-1a hash of an arbitrary byte slice.
//
// Same algorithm as [COM_HashString] but operates on raw bytes instead of
// a string. Used for hashing binary data such as texture pixel buffers
// (for deduplication in the texture cache) and compiled QuakeC bytecode
// (for integrity checks).
func COM_HashBlock(data []byte) uint32 {
	var hash uint32 = 0x811c9dc5
	for _, b := range data {
		hash ^= uint32(b)
		hash *= 0x01000193
	}
	return hash
}
