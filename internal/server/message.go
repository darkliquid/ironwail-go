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
	Data       []byte        // Raw message data
	ReadPos    int           // Current read position
	writePos   int           // Current write position
	MaxSize    int           // Optional write limit; defaults to len(Data) when zero
	BadRead    bool          // Set if a read operation failed (past end of buffer)
	Overflowed bool          // Set if a write operation exceeded buffer capacity
	ProtoFlags ProtocolFlags // Protocol flags controlling coord/angle precision
}

// NewMessageBuffer creates a new message buffer with the given capacity.
func NewMessageBuffer(size int) *MessageBuffer {
	return &MessageBuffer{
		Data:    make([]byte, size),
		MaxSize: size,
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
	m.Overflowed = false
}

func (m *MessageBuffer) limit() int {
	if m == nil {
		return 0
	}
	if m.MaxSize > 0 && m.MaxSize <= len(m.Data) {
		return m.MaxSize
	}
	return len(m.Data)
}

// ============================================================================
// WRITE METHODS (Server → Client)
// ============================================================================

// WriteByte writes a single byte to the buffer.
func (m *MessageBuffer) WriteByte(b byte) {
	if m == nil {
		return
	}
	if m.writePos >= m.limit() {
		m.Overflowed = true
		return
	}
	m.Data[m.writePos] = b
	m.writePos++
}

// WriteChar writes a signed 8-bit character.
func (m *MessageBuffer) WriteChar(c int8) {
	m.WriteByte(byte(c))
}

// WriteShort writes a 16-bit signed integer (little-endian).
func (m *MessageBuffer) WriteShort(s int16) {
	if m == nil {
		return
	}
	if m.writePos+2 > m.limit() {
		m.Overflowed = true
		return
	}
	binary.LittleEndian.PutUint16(m.Data[m.writePos:], uint16(s))
	m.writePos += 2
}

// WriteLong writes a 32-bit signed integer (little-endian).
func (m *MessageBuffer) WriteLong(l int32) {
	if m == nil {
		return
	}
	if m.writePos+4 > m.limit() {
		m.Overflowed = true
		return
	}
	binary.LittleEndian.PutUint32(m.Data[m.writePos:], uint32(l))
	m.writePos += 4
}

// WriteFloat writes a 32-bit float (little-endian).
func (m *MessageBuffer) WriteFloat(f float32) {
	if m == nil {
		return
	}
	if m.writePos+4 > m.limit() {
		m.Overflowed = true
		return
	}
	binary.LittleEndian.PutUint32(m.Data[m.writePos:], math.Float32bits(f))
	m.writePos += 4
}

// coordWireSize returns encoded byte size for one coordinate value under flags.
func coordWireSize(flags uint32) int {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 || flags&uint32(ProtocolFlagInt32Coord) != 0 {
		return 4
	}
	if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		return 3
	}
	return 2
}

// angleWireSize returns encoded byte size for one angle value under flags.
func angleWireSize(flags uint32) int {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		return 4
	}
	if flags&uint32(ProtocolFlagShortAngle) != 0 {
		return 2
	}
	return 1
}

// WriteCoord writes a coordinate using the precision dictated by protocol flags.
// C reference: MSG_WriteCoord in common.c:782-791
//   - PRFL_FLOATCOORD:  32-bit float (4 bytes)
//   - PRFL_INT32COORD:  32-bit int, value * 16 (4 bytes)
//   - PRFL_24BITCOORD:  16-bit int + 8-bit frac (3 bytes)
//   - default:          16-bit fixed-point, value * 8 (2 bytes)
func (m *MessageBuffer) WriteCoord(c float32, flags uint32) {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 {
		m.WriteFloat(c)
	} else if flags&uint32(ProtocolFlagInt32Coord) != 0 {
		m.WriteLong(int32(math.Round(float64(c) * 16)))
	} else if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		m.WriteShort(int16(c))
		m.WriteByte(byte(int(c*255) % 255))
	} else {
		// Default: 16-bit fixed-point (2 bytes). This is the standard for
		// FitzQuake protocol 666 where protocolflags == 0.
		m.WriteShort(int16(math.Round(float64(c) * 8)))
	}
}

// WriteAngle writes an angle using the precision dictated by protocol flags.
// C reference: MSG_WriteAngle in common.c:793-800
//   - PRFL_FLOATANGLE: 32-bit float (4 bytes)
//   - PRFL_SHORTANGLE: 16-bit (2 bytes)
//   - default:         8-bit (1 byte, 0-255 maps to 0-360 degrees)
func (m *MessageBuffer) WriteAngle(a float32, flags uint32) {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		m.WriteFloat(a)
	} else if flags&uint32(ProtocolFlagShortAngle) != 0 {
		m.WriteShort(int16(int(math.Round(float64(a)*65536.0/360.0)) & 65535))
	} else {
		m.WriteByte(byte(int(math.Round(float64(a)*256.0/360.0)) & 255))
	}
}

// WriteString writes a null-terminated string.
func (m *MessageBuffer) WriteString(s string) {
	if m == nil {
		return
	}
	for i := 0; i < len(s) && m.writePos < m.limit(); i++ {
		m.Data[m.writePos] = s[i]
		m.writePos++
	}
	if m.writePos >= m.limit() {
		m.Overflowed = true
		return
	}
	m.Data[m.writePos] = 0
	m.writePos++
}

// Write appends raw bytes to the buffer.
func (m *MessageBuffer) Write(data []byte) {
	if m == nil {
		return
	}
	for _, b := range data {
		if m.writePos >= m.limit() {
			m.Overflowed = true
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

// ReadCoord reads a coordinate using the precision dictated by protocol flags.
// Mirror of WriteCoord — must match the encoding format.
func (m *MessageBuffer) ReadCoord(flags uint32) float32 {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 {
		return m.ReadFloat()
	} else if flags&uint32(ProtocolFlagInt32Coord) != 0 {
		return float32(m.ReadLong()) / 16.0
	} else if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		whole := float32(m.ReadShort())
		frac := float32(m.ReadByte()) / 255.0
		return whole + frac
	}
	// Default: 16-bit fixed-point
	return float32(m.ReadShort()) / 8.0
}

// ReadAngle reads an angle using the precision dictated by protocol flags.
// Mirror of WriteAngle — must match the encoding format.
func (m *MessageBuffer) ReadAngle(flags uint32) float32 {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		return m.ReadFloat()
	} else if flags&uint32(ProtocolFlagShortAngle) != 0 {
		return float32(m.ReadShort()) * (360.0 / 65536.0)
	}
	return float32(m.ReadByte()) * (360.0 / 256.0)
}
