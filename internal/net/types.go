// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

const (
	MaxMessage     = 64000
	MaxDatagram    = 1400
	HeaderSize     = 8
	FlagData       = 1 << 24
	FlagEOM        = 1 << 25
	FlagUnreliable = 1 << 26
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
	peer                 *Socket
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
