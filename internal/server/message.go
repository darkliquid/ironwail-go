// Package server implements the Quake server physics and game logic.
//
// message.go provides the MessageBuffer type for reading and writing
// network messages. It supports both directions:
//   - Client → Server: reading movement, commands, impulses
//   - Server → Client: writing game state, sounds, entities
package server

import (
	"encoding/binary"
	"math"
)

// MessageBuffer provides methods for reading and writing network messages.
// It maintains separate read and write positions to allow bidirectional use.
// The buffer is used for:
//   - Client input: reading movement commands, angles, impulses
//   - Server output: writing entity updates, sounds, particles
type MessageBuffer struct {
	Data     []byte // Raw message data
	ReadPos  int    // Current read position
	writePos int    // Current write position
	BadRead  bool   // Set if a read operation failed (past end of buffer)
}

// NewMessageBuffer creates a new message buffer with the given capacity.
func NewMessageBuffer(size int) *MessageBuffer {
	return &MessageBuffer{
		Data: make([]byte, size),
	}
}

// Len returns the current length of written data.
func (m *MessageBuffer) Len() int {
	return m.writePos
}

// Clear resets the buffer for reuse.
func (m *MessageBuffer) Clear() {
	m.writePos = 0
	m.ReadPos = 0
	m.BadRead = false
}

// ============================================================================
// WRITE METHODS (Server → Client)
// ============================================================================

// WriteByte writes a single byte to the buffer.
func (m *MessageBuffer) WriteByte(b byte) {
	if m.writePos < len(m.Data) {
		m.Data[m.writePos] = b
		m.writePos++
	}
}

// WriteChar writes a signed 8-bit character.
func (m *MessageBuffer) WriteChar(c int8) {
	m.WriteByte(byte(c))
}

// WriteShort writes a 16-bit signed integer (little-endian).
func (m *MessageBuffer) WriteShort(s int16) {
	if m.writePos+2 <= len(m.Data) {
		binary.LittleEndian.PutUint16(m.Data[m.writePos:], uint16(s))
		m.writePos += 2
	}
}

// WriteLong writes a 32-bit signed integer (little-endian).
func (m *MessageBuffer) WriteLong(l int32) {
	if m.writePos+4 <= len(m.Data) {
		binary.LittleEndian.PutUint32(m.Data[m.writePos:], uint32(l))
		m.writePos += 4
	}
}

// WriteFloat writes a 32-bit float (little-endian).
func (m *MessageBuffer) WriteFloat(f float32) {
	if m.writePos+4 <= len(m.Data) {
		binary.LittleEndian.PutUint32(m.Data[m.writePos:], math.Float32bits(f))
		m.writePos += 4
	}
}

// WriteCoord writes a coordinate (float32 in FitzQuake protocol).
func (m *MessageBuffer) WriteCoord(c float32) {
	m.WriteFloat(c)
}

// WriteAngle writes an angle as a single byte (0-255 maps to 0-360 degrees).
func (m *MessageBuffer) WriteAngle(a float32) {
	m.WriteByte(byte(a * 256.0 / 360.0))
}

// WriteString writes a null-terminated string.
func (m *MessageBuffer) WriteString(s string) {
	for i := 0; i < len(s) && m.writePos < len(m.Data); i++ {
		m.Data[m.writePos] = s[i]
		m.writePos++
	}
	if m.writePos < len(m.Data) {
		m.Data[m.writePos] = 0
		m.writePos++
	}
}

// Write appends raw bytes to the buffer.
func (m *MessageBuffer) Write(data []byte) {
	for _, b := range data {
		if m.writePos >= len(m.Data) {
			break
		}
		m.Data[m.writePos] = b
		m.writePos++
	}
}

// ============================================================================
// READ METHODS (Client → Server)
// ============================================================================

// ReadByte reads a single byte from the buffer.
func (m *MessageBuffer) ReadByte() byte {
	if m.ReadPos >= len(m.Data) {
		m.BadRead = true
		return 0
	}
	val := m.Data[m.ReadPos]
	m.ReadPos++
	return val
}

// ReadShort reads a 16-bit signed integer (little-endian).
func (m *MessageBuffer) ReadShort() int16 {
	if m.ReadPos+2 > len(m.Data) {
		m.BadRead = true
		return 0
	}
	val := int16(binary.LittleEndian.Uint16(m.Data[m.ReadPos : m.ReadPos+2]))
	m.ReadPos += 2
	return val
}

// ReadFloat reads a 32-bit float (little-endian).
func (m *MessageBuffer) ReadFloat() float32 {
	if m.ReadPos+4 > len(m.Data) {
		m.BadRead = true
		return 0
	}
	val := math.Float32frombits(binary.LittleEndian.Uint32(m.Data[m.ReadPos : m.ReadPos+4]))
	m.ReadPos += 4
	return val
}

// ReadAngle16 reads a 16-bit angle (0-65535 maps to 0-360 degrees).
func (m *MessageBuffer) ReadAngle16() float32 {
	val := float32(m.ReadShort())
	return val * (360.0 / 65536.0)
}

// ReadString reads a null-terminated string.
func (m *MessageBuffer) ReadString() string {
	start := m.ReadPos
	for m.ReadPos < len(m.Data) && m.Data[m.ReadPos] != 0 {
		m.ReadPos++
	}
	str := string(m.Data[start:m.ReadPos])
	if m.ReadPos < len(m.Data) {
		m.ReadPos++ // Skip null terminator
	}
	return str
}

// ReadLong reads a 32-bit signed integer (little-endian).
func (m *MessageBuffer) ReadLong() int32 {
	if m.ReadPos+4 > len(m.Data) {
		m.BadRead = true
		return 0
	}
	val := int32(binary.LittleEndian.Uint32(m.Data[m.ReadPos : m.ReadPos+4]))
	m.ReadPos += 4
	return val
}

// ReadChar reads a signed 8-bit character.
func (m *MessageBuffer) ReadChar() int8 {
	return int8(m.ReadByte())
}

// ReadCoord reads a coordinate (float32 in FitzQuake protocol).
func (m *MessageBuffer) ReadCoord() float32 {
	return m.ReadFloat()
}

// ReadAngle reads an angle as a single byte (0-255 maps to 0-360 degrees).
func (m *MessageBuffer) ReadAngle() float32 {
	return float32(m.ReadByte()) * (360.0 / 256.0)
}
