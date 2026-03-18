// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// types.go defines the shared networking types and constants used across
// all layers of Quake's network stack. This includes:
//   - Packet size limits (MaxMessage, MaxDatagram)
//   - Wire protocol flags that encode packet type in the header
//   - Driver identifiers (loopback vs. datagram/UDP)
//   - The Socket type, which is the central connection abstraction
//
// The Socket type is intentionally driver-agnostic: it contains fields
// for both the loopback driver (peer pointer) and the datagram/UDP
// driver (udpConn, remoteAddr). The 'driver' field determines which
// set of fields is active, and the net.go dispatcher uses it to route
// operations to the correct driver implementation.

import (
	stdnet "net"
)

// Packet size limits and wire protocol header flags.
//
// MaxMessage is the maximum size of a complete reliable message after
// reassembly from fragments. MaxDatagram is the maximum payload per
// individual UDP packet (chosen to stay well under typical MTU limits
// to avoid IP fragmentation).
//
// HeaderSize is the 8-byte packet header: 4 bytes for flags+length
// (upper bits are flags, lower 16 bits are packet length) and 4 bytes
// for the sequence number.
//
// The Flag* constants are bitmasks applied to the first 4 bytes of
// every packet header. They identify the packet type:
//   - FlagData: reliable data packet (may be one fragment of a larger message)
//   - FlagAck: acknowledgment of a reliable data packet
//   - FlagNak: negative acknowledgment (reserved, not actively used)
//   - FlagEOM: end-of-message marker for the last fragment of a reliable send
//   - FlagUnreliable: unreliable (fire-and-forget) data packet
//   - FlagCtl: control packet (connection handshake, server queries)
//
// LengthMask extracts the packet length from the header's first uint32.
const (
	MaxMessage     = 65535 // NET_MAXMESSAGE in C (was 32000, raised by ericw)
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

// Driver identifiers stored in Socket.driver to select the correct
// transport implementation. DriverLoopback routes through in-memory
// buffers (single-player); DriverDatagram routes through UDP (multiplayer).
const (
	DriverLoopback = 0
	DriverDatagram = 1
)

// Socket represents a single network connection in the Quake engine.
// It is the central abstraction that unifies loopback (single-player)
// and datagram/UDP (multiplayer) connections behind a common interface.
//
// Key fields for the reliability protocol (datagram driver):
//   - sendSequence / recvSequence: sequence counters for reliable messages
//   - unreliableSendSeq / unreliableRecvSeq: sequence counters for unreliable messages
//   - ackSequence: tracks the last acknowledged reliable sequence
//   - canSend: false while a reliable message is in flight (stop-and-wait ARQ)
//   - sendNext: true when the next fragment of a multi-part message is ready
//   - sendMessage / receiveMessage: buffers for message fragmentation/reassembly
//
// Key fields for the loopback driver:
//   - peer: pointer to the other end of the loopback pair
//
// Key fields for UDP transport:
//   - udpConn: the Go net.UDPConn for sending/receiving packets
//   - remoteAddr: the remote endpoint's UDP address
//
// This corresponds to qsocket_t in the original C Quake engine.
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
	rejectionReason      string

	// Driver specific
	driver     int
	peer       *Socket
	udpConn    *stdnet.UDPConn
	remoteAddr *stdnet.UDPAddr
}

// NewSocket creates a new Socket with pre-allocated send and receive
// buffers (MaxMessage bytes each). The socket starts in a "ready to send"
// state (canSend = true). The caller must set driver-specific fields
// (driver, peer, udpConn, remoteAddr) after creation.
func NewSocket(address string) *Socket {
	return &Socket{
		address:        address,
		sendMessage:    make([]byte, MaxMessage),
		receiveMessage: make([]byte, MaxMessage),
		canSend:        true,
	}
}

// Address returns the human-readable address string for this connection
// (e.g., "localhost" for loopback, or "192.168.1.5:26000" for UDP).
func (s *Socket) Address() string {
	return s.address
}

// CanSendMessage reports whether the socket is ready to accept a new
// reliable message. Returns false when a reliable send is in progress
// and awaiting acknowledgment.
func (s *Socket) CanSendMessage() bool {
	return s.canSend
}

// CanSendUnreliable reports whether the socket can send an unreliable
// message. Always returns true because unreliable messages have no
// flow control — they are sent immediately without waiting for ACKs.
func (s *Socket) CanSendUnreliable() bool {
	return true
}

// Error returns the rejection reason if the connection was refused by
// the server during the handshake. Empty string means no error.
func (s *Socket) Error() string {
	return s.rejectionReason
}
