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
	"io"
	"math"
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

// Little-endian binary I/O helpers.
// Quake data files and network protocol use little-endian byte order.

// LittleShort reads a 16-bit signed integer from a 2-byte slice.
func LittleShort(data []byte) int16 {
	return int16(binary.LittleEndian.Uint16(data))
}

// LittleLong reads a 32-bit signed integer from a 4-byte slice.
func LittleLong(data []byte) int32 {
	return int32(binary.LittleEndian.Uint32(data))
}

// LittleFloat reads a 32-bit float from a 4-byte slice in little-endian IEEE 754 format.
func LittleFloat(data []byte) float32 {
	bits := binary.LittleEndian.Uint32(data)
	return math.Float32frombits(bits)
}

// BigShort reads a 16-bit signed integer in big-endian format.
func BigShort(data []byte) int16 {
	return int16(binary.BigEndian.Uint16(data))
}

// BigLong reads a 32-bit signed integer in big-endian format.
func BigLong(data []byte) int32 {
	return int32(binary.BigEndian.Uint32(data))
}

// BigFloat reads a 32-bit float in big-endian IEEE 754 format.
func BigFloat(data []byte) float32 {
	bits := binary.BigEndian.Uint32(data)
	return math.Float32frombits(bits)
}

// WriteLittleShort writes a 16-bit signed integer to a writer in little-endian format.
func WriteLittleShort(w io.Writer, v int16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(v))
	_, err := w.Write(buf[:])
	return err
}

// WriteLittleLong writes a 32-bit signed integer to a writer in little-endian format.
func WriteLittleLong(w io.Writer, v int32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	_, err := w.Write(buf[:])
	return err
}

// WriteLittleFloat writes a 32-bit float to a writer in little-endian IEEE 754 format.
func WriteLittleFloat(w io.Writer, v float32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadLittleShort reads a 16-bit signed integer from a reader in little-endian format.
func ReadLittleShort(r io.Reader) (int16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf[:])), nil
}

// ReadLittleLong reads a 32-bit signed integer from a reader in little-endian format.
func ReadLittleLong(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

// ReadLittleFloat reads a 32-bit float from a reader in little-endian IEEE 754 format.
func ReadLittleFloat(r io.Reader) (float32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(buf[:])), nil
}
