// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import "encoding/binary"

type Loopback struct {
	client  *Socket
	server  *Socket
	pending bool
}

func NewLoopback() *Loopback {
	return &Loopback{
		client:  NewSocket("localhost"),
		server:  NewSocket("LOCAL"),
		pending: false,
	}
}

func (l *Loopback) Init() error {
	l.client.peer = l.server
	l.server.peer = l.client
	return nil
}

func (l *Loopback) Shutdown() {
	l.client = nil
	l.server = nil
	l.pending = false
}

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

func (l *Loopback) Client() *Socket {
	return l.client
}

func (l *Loopback) Server() *Socket {
	return l.server
}

func intAlign(value int) int {
	return (value + 3) & ^3
}

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

func CloseLoopback(sock *Socket) {
	if sock.peer != nil {
		sock.peer.peer = nil
	}
	sock.receiveMessageLength = 0
	sock.sendMessageLength = 0
	sock.canSend = true
}

type Buffer struct {
	data       []byte
	cursor     int
	maxsize    int
	overflowed bool
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		data:    make([]byte, size),
		maxsize: size,
	}
}

func (b *Buffer) Clear() {
	b.cursor = 0
	b.overflowed = false
}

func (b *Buffer) Size() int {
	return b.cursor
}

func (b *Buffer) Data() []byte {
	return b.data[:b.cursor]
}

func (b *Buffer) WriteByte(val byte) {
	if b.cursor >= b.maxsize {
		b.overflowed = true
		return
	}
	b.data[b.cursor] = val
	b.cursor++
}

func (b *Buffer) WriteShort(val int16) {
	if b.cursor+2 > b.maxsize {
		b.overflowed = true
		return
	}
	binary.LittleEndian.PutUint16(b.data[b.cursor:], uint16(val))
	b.cursor += 2
}

func (b *Buffer) WriteLong(val int32) {
	if b.cursor+4 > b.maxsize {
		b.overflowed = true
		return
	}
	binary.LittleEndian.PutUint32(b.data[b.cursor:], uint32(val))
	b.cursor += 4
}

func (b *Buffer) WriteFloat(val float32) {
	if b.cursor+4 > b.maxsize {
		b.overflowed = true
		return
	}
	bits := uint32(val)
	binary.LittleEndian.PutUint32(b.data[b.cursor:], bits)
	b.cursor += 4
}

func (b *Buffer) WriteString(val string) {
	for i := 0; i < len(val); i++ {
		b.WriteByte(val[i])
	}
	b.WriteByte(0)
}

func (b *Buffer) Write(data []byte) {
	if b.cursor+len(data) > b.maxsize {
		b.overflowed = true
		return
	}
	copy(b.data[b.cursor:], data)
	b.cursor += len(data)
}

func (b *Buffer) ReadByte() byte {
	if b.cursor >= b.maxsize {
		return 0
	}
	val := b.data[b.cursor]
	b.cursor++
	return val
}

func (b *Buffer) ReadShort() int16 {
	if b.cursor+2 > b.maxsize {
		return 0
	}
	val := int16(binary.LittleEndian.Uint16(b.data[b.cursor:]))
	b.cursor += 2
	return val
}

func (b *Buffer) ReadLong() int32 {
	if b.cursor+4 > b.maxsize {
		return 0
	}
	val := int32(binary.LittleEndian.Uint32(b.data[b.cursor:]))
	b.cursor += 4
	return val
}

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
