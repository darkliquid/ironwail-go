// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// loopback.go implements the loopback network driver, which enables
// single-player Quake by creating an in-memory "network" connection
// between the client and server running in the same process.
//
// Instead of sending packets over a real network, the loopback driver
// simply copies data between two paired Socket instances (client and
// server). Each socket writes into its peer's receive buffer, making
// message delivery instantaneous and zero-copy from the network's
// perspective.
//
// The loopback message format uses a simple 4-byte header per message:
//
//	byte 0: message type (1 = reliable, 2 = unreliable)
//	byte 1-2: payload length (little-endian 16-bit)
//	byte 3: padding (for 4-byte alignment)
//
// Messages are packed contiguously in the receive buffer and aligned
// to 4-byte boundaries, matching the original C Quake implementation
// where int-alignment was important for performance on 1990s hardware.
//
// This driver corresponds to net_loop.c in the original Quake source.

import "encoding/binary"

// Loopback represents a paired client/server loopback connection.
// It holds references to both endpoint sockets and tracks whether
// a client connection is pending acceptance by the server side.
// In single-player, exactly one Loopback instance exists, created
// when the player starts a new game or loads a save.
type Loopback struct {
	client  *Socket
	server  *Socket
	pending bool
}

// NewLoopback creates a new loopback driver with pre-allocated client
// and server sockets. The client socket is addressed as "localhost"
// and the server socket as "LOCAL". The sockets are not yet linked —
// call Init() to establish the peer relationship.
func NewLoopback() *Loopback {
	return &Loopback{
		client:  NewSocket("localhost"),
		server:  NewSocket("LOCAL"),
		pending: false,
	}
}

// Init establishes the bidirectional peer link between the client
// and server sockets. After Init, writing to one socket's send path
// will deliver data to the other socket's receive buffer.
func (l *Loopback) Init() error {
	l.client.peer = l.server
	l.server.peer = l.client
	return nil
}

// Shutdown destroys the loopback connection by releasing both socket
// references and clearing the pending flag.
func (l *Loopback) Shutdown() {
	l.client = nil
	l.server = nil
	l.pending = false
}

// Connect initiates the client side of a loopback connection. It sets
// the pending flag (so the server side can detect the new connection
// via CheckNewConnections) and resets both sockets' message buffers.
// Returns the client-side socket.
func (l *Loopback) Connect() *Socket {
	l.pending = true
	l.client.sendMessageLength = 0
	l.client.receiveMessageLength = 0
	l.client.canSend = true
	l.server.sendMessageLength = 0
	l.server.receiveMessageLength = 0
	l.server.canSend = true
	return l.client
}

// CheckNewConnections is called by the server side to detect a pending
// loopback connection. If a client has called Connect(), this returns
// the server-side socket (consuming the pending flag).
func (l *Loopback) CheckNewConnections() *Socket {
	if !l.pending {
		return nil
	}
	l.pending = false
	l.server.sendMessageLength = 0
	l.server.receiveMessageLength = 0
	l.server.canSend = true
	l.client.sendMessageLength = 0
	l.client.receiveMessageLength = 0
	l.client.canSend = true
	return l.server
}

// Client returns the client-side socket of the loopback pair.
func (l *Loopback) Client() *Socket {
	return l.client
}

// Server returns the server-side socket of the loopback pair.
func (l *Loopback) Server() *Socket {
	return l.server
}

// intAlign rounds a byte offset up to the next 4-byte boundary.
// This matches the int-alignment used in original Quake's loopback
// message packing, where each message header + payload is padded
// so the next message starts at a 32-bit aligned offset.
func intAlign(value int) int {
	return (value + 3) & ^3
}

// GetMessageLoopback reads the next message from a loopback socket's
// receive buffer. The buffer may contain multiple packed messages
// (each with a 4-byte header: type, length-lo, length-hi, pad).
// After extracting one message, the remaining data is shifted down.
//
// Return values: message type (0=none, 1=reliable, 2=unreliable)
// and the message payload. If a reliable message is read, the peer's
// canSend flag is set to true, completing the flow-control handshake.
func GetMessageLoopback(sock *Socket, dest []byte) (int, []byte) {
	if sock.receiveMessageLength == 0 {
		return 0, nil
	}

	msgType := sock.receiveMessage[0]
	length := int(sock.receiveMessage[1]) | (int(sock.receiveMessage[2]) << 8)

	if len(dest) < length {
		dest = make([]byte, length)
	}
	copy(dest, sock.receiveMessage[4:4+length])

	alignedLen := intAlign(length + 4)
	sock.receiveMessageLength -= alignedLen

	if sock.receiveMessageLength > 0 {
		copy(sock.receiveMessage, sock.receiveMessage[alignedLen:])
	}

	if sock.peer != nil && msgType == 1 {
		sock.peer.canSend = true
	}

	return int(msgType), dest[:length]
}

// SendMessageLoopback writes a reliable message into the peer socket's
// receive buffer. The message is prefixed with a 4-byte header
// (type=1, 16-bit length, padding) and 4-byte aligned. The sender's
// canSend flag is cleared — the sender cannot send another reliable
// message until the receiver has read this one.
// Returns 1 on success, -1 if the peer is nil or the buffer would overflow.
func SendMessageLoopback(sock *Socket, data []byte) int {
	if sock.peer == nil {
		return -1
	}

	totalLen := sock.peer.receiveMessageLength + len(data) + 4
	if totalLen > MaxMessage {
		return -1
	}

	offset := sock.peer.receiveMessageLength
	buf := sock.peer.receiveMessage

	buf[offset] = 1
	buf[offset+1] = byte(len(data) & 0xff)
	buf[offset+2] = byte((len(data) >> 8) & 0xff)
	buf[offset+3] = 0
	copy(buf[offset+4:], data)

	sock.peer.receiveMessageLength = intAlign(sock.peer.receiveMessageLength + len(data) + 4)
	sock.canSend = false

	return 1
}

// SendUnreliableMessageLoopback writes an unreliable message into the
// peer socket's receive buffer. Works like SendMessageLoopback but
// with message type 2 and without clearing canSend — unreliable
// messages do not participate in flow control. If the buffer would
// overflow, the message is silently dropped (returns 0).
func SendUnreliableMessageLoopback(sock *Socket, data []byte) int {
	if sock.peer == nil {
		return -1
	}

	totalLen := sock.peer.receiveMessageLength + len(data) + 4
	if totalLen > MaxMessage {
		return 0
	}

	offset := sock.peer.receiveMessageLength
	buf := sock.peer.receiveMessage

	buf[offset] = 2
	buf[offset+1] = byte(len(data) & 0xff)
	buf[offset+2] = byte((len(data) >> 8) & 0xff)
	buf[offset+3] = 0
	copy(buf[offset+4:], data)

	sock.peer.receiveMessageLength = intAlign(sock.peer.receiveMessageLength + len(data) + 4)

	return 1
}

// CloseLoopback tears down one side of a loopback connection.
// The peer link is severed and the socket's buffers are reset.
func CloseLoopback(sock *Socket) {
	if sock.peer != nil {
		sock.peer.peer = nil
	}
	sock.receiveMessageLength = 0
	sock.sendMessageLength = 0
	sock.canSend = true
}

// Buffer is a simple sequential read/write byte buffer used for
// constructing and parsing Quake network messages. It mirrors the
// sizebuf_t / MSG_Write* / MSG_Read* API from the original C engine.
// Data is written in Quake's native wire format: little-endian byte
// order for multi-byte integers, null-terminated strings.
type Buffer struct {
	data       []byte
	cursor     int
	maxsize    int
	overflowed bool
}

// NewBuffer creates a Buffer with the given capacity in bytes.
func NewBuffer(size int) *Buffer {
	return &Buffer{
		data:    make([]byte, size),
		maxsize: size,
	}
}

// Clear resets the buffer to empty, allowing reuse without reallocating.
func (b *Buffer) Clear() {
	b.cursor = 0
	b.overflowed = false
}

// Size returns the number of bytes currently written to the buffer.
func (b *Buffer) Size() int {
	return b.cursor
}

// Data returns the buffer's contents up to the current write position.
func (b *Buffer) Data() []byte {
	return b.data[:b.cursor]
}

// WriteByte appends a single byte to the buffer.
func (b *Buffer) WriteByte(val byte) {
	if b.cursor >= b.maxsize {
		b.overflowed = true
		return
	}
	b.data[b.cursor] = val
	b.cursor++
}

// WriteShort appends a 16-bit integer in little-endian byte order.
func (b *Buffer) WriteShort(val int16) {
	if b.cursor+2 > b.maxsize {
		b.overflowed = true
		return
	}
	binary.LittleEndian.PutUint16(b.data[b.cursor:], uint16(val))
	b.cursor += 2
}

// WriteLong appends a 32-bit integer in little-endian byte order.
func (b *Buffer) WriteLong(val int32) {
	if b.cursor+4 > b.maxsize {
		b.overflowed = true
		return
	}
	binary.LittleEndian.PutUint32(b.data[b.cursor:], uint32(val))
	b.cursor += 4
}

// WriteFloat appends a 32-bit float in little-endian byte order.
func (b *Buffer) WriteFloat(val float32) {
	if b.cursor+4 > b.maxsize {
		b.overflowed = true
		return
	}
	bits := uint32(val)
	binary.LittleEndian.PutUint32(b.data[b.cursor:], bits)
	b.cursor += 4
}

// WriteString appends a null-terminated string to the buffer.
// Quake's wire protocol uses C-style null-terminated strings.
func (b *Buffer) WriteString(val string) {
	for i := 0; i < len(val); i++ {
		b.WriteByte(val[i])
	}
	b.WriteByte(0)
}

// Write appends a raw byte slice to the buffer.
func (b *Buffer) Write(data []byte) {
	if b.cursor+len(data) > b.maxsize {
		b.overflowed = true
		return
	}
	copy(b.data[b.cursor:], data)
	b.cursor += len(data)
}

// ReadByte reads and returns a single byte, advancing the cursor.
func (b *Buffer) ReadByte() byte {
	if b.cursor >= b.maxsize {
		return 0
	}
	val := b.data[b.cursor]
	b.cursor++
	return val
}

// ReadShort reads a 16-bit little-endian integer, advancing by 2 bytes.
func (b *Buffer) ReadShort() int16 {
	if b.cursor+2 > b.maxsize {
		return 0
	}
	val := int16(binary.LittleEndian.Uint16(b.data[b.cursor:]))
	b.cursor += 2
	return val
}

// ReadLong reads a 32-bit little-endian integer, advancing by 4 bytes.
func (b *Buffer) ReadLong() int32 {
	if b.cursor+4 > b.maxsize {
		return 0
	}
	val := int32(binary.LittleEndian.Uint32(b.data[b.cursor:]))
	b.cursor += 4
	return val
}

// ReadString reads bytes until a null terminator, returning as a string.
func (b *Buffer) ReadString() string {
	var result []byte
	for b.cursor < b.maxsize {
		c := b.ReadByte()
		if c == 0 {
			break
		}
		result = append(result, c)
	}
	return string(result)
}
