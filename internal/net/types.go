// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	stdnet "net"
)

const (
	MaxMessage     = 64000
	MaxDatagram    = 1400
	HeaderSize     = 8
	FlagData       = 1 << 16 // 0x00010000
	FlagAck        = 1 << 17 // 0x00020000
	FlagNak        = 1 << 18 // 0x00040000
	FlagEOM        = 1 << 19 // 0x00080000
	FlagUnreliable = 1 << 20 // 0x00100000
	FlagCtl        = 1 << 31 // 0x80000000
	LengthMask     = 0xffff
)

const (
	DriverLoopback = 0
	DriverDatagram = 1
)

type Socket struct {
	address              string
	sendMessage          []byte
	sendMessageLength    int
	receiveMessage       []byte
	receiveMessageLength int
	canSend              bool
	sendSequence         uint32
	recvSequence         uint32
	unreliableSendSeq    uint32
	unreliableRecvSeq    uint32
	ackSequence          uint32
	lastSendTime         float64
	lastMessageTime      float64
	connectTime          float64
	disconnected         bool
	sendNext             bool

	// Driver specific
	driver     int
	peer       *Socket
	udpConn    *stdnet.UDPConn
	remoteAddr *stdnet.UDPAddr
}

func NewSocket(address string) *Socket {
	return &Socket{
		address:        address,
		sendMessage:    make([]byte, MaxMessage),
		receiveMessage: make([]byte, MaxMessage),
		canSend:        true,
	}
}

func (s *Socket) Address() string {
	return s.address
}

func (s *Socket) CanSendMessage() bool {
	return s.canSend
}

func (s *Socket) CanSendUnreliable() bool {
	return true
}
