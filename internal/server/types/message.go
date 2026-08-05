// message.go provides the MessageBuffer type for network message encoding and decoding.
package types

import (
	"encoding/binary"
	"math"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// MessageBuffer provides methods for reading and writing network messages.
type MessageBuffer struct {
	Data       []byte        // Raw message data
	ReadPos    int           // Current read position
	writePos   int           // Current write position
	MaxSize    int           // Optional write limit; defaults to len(Data) when zero
	BadRead    bool          // Set if a read operation failed
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

// Limit returns the effective maximum write capacity of the buffer.
func (m *MessageBuffer) Limit() int {
	if m == nil {
		return 0
	}
	if m.MaxSize > 0 && m.MaxSize <= len(m.Data) {
		return m.MaxSize
	}
	return len(m.Data)
}

// Clear resets the buffer for reuse.
func (m *MessageBuffer) Clear() {
	m.writePos = 0
	m.ReadPos = 0
	m.BadRead = false
	m.Overflowed = false
}

// PutByte writes a single byte to the buffer.
func (m *MessageBuffer) PutByte(b byte) {
	if m == nil {
		return
	}
	if m.writePos >= m.Limit() {
		m.Overflowed = true
		return
	}
	m.Data[m.writePos] = b
	m.writePos++
}

// WriteChar writes a signed 8-bit character.
func (m *MessageBuffer) WriteChar(c int8) {
	m.PutByte(byte(c))
}

// WriteShort writes a 16-bit signed integer (little-endian).
func (m *MessageBuffer) WriteShort(s int16) {
	if m == nil {
		return
	}
	if m.writePos+2 > m.Limit() {
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
	if m.writePos+4 > m.Limit() {
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
	if m.writePos+4 > m.Limit() {
		m.Overflowed = true
		return
	}
	binary.LittleEndian.PutUint32(m.Data[m.writePos:], math.Float32bits(f))
	m.writePos += 4
}

// CoordWireSize returns encoded byte size for one coordinate value under flags.
func CoordWireSize(flags uint32) int {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 || flags&uint32(ProtocolFlagInt32Coord) != 0 {
		return 4
	}
	if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		return 3
	}
	return 2
}

// AngleWireSize returns encoded byte size for one angle value under flags.
func AngleWireSize(flags uint32) int {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		return 4
	}
	if flags&uint32(ProtocolFlagShortAngle) != 0 {
		return 2
	}
	return 1
}

// WriteCoord writes a coordinate using precision dictated by protocol flags.
func (m *MessageBuffer) WriteCoord(c float32, flags uint32) {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 {
		m.WriteFloat(c)
	} else if flags&uint32(ProtocolFlagInt32Coord) != 0 {
		m.WriteLong(int32(qtypes.QRint(c * 16)))
	} else if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		m.WriteShort(int16(c))
		m.PutByte(byte(int(c*255) % 255))
	} else {
		m.WriteShort(int16(qtypes.QRint(c * 8)))
	}
}

// WriteAngle writes an angle using precision dictated by protocol flags.
func (m *MessageBuffer) WriteAngle(a float32, flags uint32) {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		m.WriteFloat(a)
	} else if flags&uint32(ProtocolFlagShortAngle) != 0 {
		m.WriteShort(int16(qtypes.QRint(a*65536.0/360.0) & 65535))
	} else {
		m.PutByte(byte(qtypes.QRint(a*256.0/360.0) & 255))
	}
}

// WriteString writes a null-terminated string.
func (m *MessageBuffer) WriteString(s string) {
	if m == nil {
		return
	}
	for i := 0; i < len(s) && m.writePos < m.Limit(); i++ {
		m.Data[m.writePos] = s[i]
		m.writePos++
	}
	if m.writePos >= m.Limit() {
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
		if m.writePos >= m.Limit() {
			m.Overflowed = true
			break
		}
		m.Data[m.writePos] = b
		m.writePos++
	}
}

// Byte reads a single byte from the buffer.
func (m *MessageBuffer) Byte() byte {
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
		m.ReadPos++
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
	return int8(m.Byte())
}

// ReadCoord reads a coordinate using precision dictated by protocol flags.
func (m *MessageBuffer) ReadCoord(flags uint32) float32 {
	if flags&uint32(ProtocolFlagFloatCoord) != 0 {
		return m.ReadFloat()
	} else if flags&uint32(ProtocolFlagInt32Coord) != 0 {
		return float32(m.ReadLong()) / 16.0
	} else if flags&uint32(ProtocolFlag24BitCoord) != 0 {
		whole := float32(m.ReadShort())
		frac := float32(m.Byte()) / 255.0
		return whole + frac
	}
	return float32(m.ReadShort()) / 8.0
}

// ReadAngle reads an angle using precision dictated by protocol flags.
func (m *MessageBuffer) ReadAngle(flags uint32) float32 {
	if flags&uint32(ProtocolFlagFloatAngle) != 0 {
		return m.ReadFloat()
	} else if flags&uint32(ProtocolFlagShortAngle) != 0 {
		return float32(m.ReadShort()) * (360.0 / 65536.0)
	}
	return float32(int8(m.Byte())) * (360.0 / 256.0)
}
