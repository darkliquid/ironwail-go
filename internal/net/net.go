// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// net.go is the top-level networking facade for the Quake engine. It implements
// the "dispatcher" pattern from original Quake's net_main.c: every public
// networking operation (Connect, GetMessage, SendMessage, etc.) first checks
// which driver owns the socket (loopback vs. datagram/UDP) and delegates to
// the appropriate implementation.
//
// Quake's networking uses a layered architecture:
//   - net.go (this file): connection-level abstraction and driver dispatch
//   - datagram.go: reliable/unreliable message protocol over UDP
//   - loopback.go: in-memory "network" for single-player (no real I/O)
//   - udp.go: raw UDP socket operations via Go's net.UDPConn
//   - protocol.go: wire format constants (svc_*, clc_*, entity update flags)
//   - types.go: shared types (Socket, packet flags, size limits)
//   - slist.go: LAN server discovery/browsing
//
// In single-player, the client and server run in the same process and
// communicate through a loopback driver — packets are just memory copies.
// In multiplayer, the datagram driver sends packets over UDP with its own
// reliability layer (sequence numbers, acknowledgments, fragmentation).

import (
	"time"
)

// startTime records the process start instant. All Quake networking timestamps
// are measured as seconds elapsed since this moment, providing a monotonic
// clock for timeout detection, packet pacing, and retransmission logic.
var (
	startTime = time.Now()
)

// NetTime returns the elapsed time in seconds since the networking subsystem
// was initialized. This is Quake's internal clock for the network layer,
// used for retransmission timeouts, keep-alive intervals, and connection
// timeout detection. It corresponds to Sys_FloatTime() in the original C code.
func NetTime() float64 {
	return time.Since(startTime).Seconds()
}

// Init initializes the networking subsystem by bringing up all available
// transport drivers. Currently this means UDP only; the loopback driver
// requires no initialization as it is created on demand. This corresponds
// to NET_Init() in net_main.c. Returns an error if any required driver
// fails to start.
func Init() error {
	UDPInit()
	return nil
}

// Shutdown tears down the networking subsystem and releases any resources
// held by active drivers. Called during engine exit. Corresponds to
// NET_Shutdown() in net_main.c.
func Shutdown() {
	Listen(false)
	for _, sock := range acceptedServerSockets {
		if sock == nil || sock.udpConn == nil {
			continue
		}
		UDPCloseSocket(sock.udpConn)
		sock.udpConn = nil
	}
	acceptedServerSockets = nil
	if loopback != nil {
		loopback.Shutdown()
		loopback = nil
	}
	serverInfoProvider = nil
}

// Connect establishes a network connection to the given host. If the host
// is "local" or "localhost", a loopback connection is created for
// single-player — this avoids real network I/O by routing packets through
// shared memory buffers. Otherwise, the datagram driver initiates a UDP
// connection handshake with the remote server. Returns nil on failure.
// This is the engine's NET_Connect() equivalent from net_main.c.
func Connect(host string) *Socket {
	if host == "local" || host == "localhost" {
		// Loopback
		l := NewLoopback()
		l.Init()
		sock := l.Connect()
		sock.driver = DriverLoopback
		return sock
	}
	return DatagramConnect(host)
}

// GetMessage polls a socket for incoming data. The return value indicates
// the message type: 0 = no message, 1 = reliable message, 2 = unreliable
// message, 3 = control message. The byte slice contains the message payload.
// The driver dispatch here mirrors NET_GetMessage() in net_main.c: loopback
// sockets read from shared memory, while datagram sockets read from UDP and
// process the reliability protocol (ACKs, sequencing, fragmentation).
func GetMessage(sock *Socket) (int, []byte) {
	if sock.driver == DriverLoopback {
		return GetMessageLoopback(sock, nil)
	}
	return DatagramGetMessage(sock)
}

// SendMessage sends a reliable message over the given socket. Reliable
// messages are guaranteed to arrive in order — the datagram layer handles
// sequencing, acknowledgment, and retransmission. For loopback sockets,
// data is simply copied into the peer's receive buffer. Returns 1 on
// success, -1 on failure. Corresponds to NET_SendMessage() in net_main.c.
func SendMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendMessageLoopback(sock, data)
	}
	return DatagramSendMessage(sock, data)
}

// SendUnreliableMessage sends a fire-and-forget message over the given
// socket. Unreliable messages may be lost or arrive out of order, but they
// have lower overhead — ideal for rapidly-changing state like entity
// positions. For loopback sockets, data is copied directly. Returns 1 on
// success, -1 on failure. Corresponds to NET_SendUnreliableMessage().
func SendUnreliableMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendUnreliableMessageLoopback(sock, data)
	}
	return DatagramSendUnreliableMessage(sock, data)
}

// CanSendMessage reports whether the socket is ready to accept a new
// reliable message. The datagram driver can only have one reliable message
// in flight at a time (stop-and-wait ARQ); this returns false while
// waiting for an ACK from the remote end. Loopback sockets are always
// ready. Corresponds to NET_CanSendMessage() in net_main.c.
func CanSendMessage(sock *Socket) bool {
	if sock.driver == DriverLoopback {
		return true
	}
	return DatagramCanSendMessage(sock)
}

// CanSendUnreliableMessage reports whether the socket can accept an unreliable
// message right now. This mirrors NET_CanSendUnreliableMessage(); datagram and
// loopback transports always return true because unreliable messages bypass the
// reliable stop-and-wait flow control.
func CanSendUnreliableMessage(sock *Socket) bool {
	if sock == nil {
		return false
	}
	return sock.CanSendUnreliable()
}

// Close shuts down a network connection, releasing the underlying
// transport resources. For loopback sockets this disconnects the
// client/server peer link; for datagram sockets it closes the UDP
// connection. Corresponds to NET_Close() in net_main.c.
func Close(sock *Socket) {
	if sock == nil {
		return
	}

	if sock.driver == DriverLoopback {
		CloseLoopback(sock)
	} else {
		untrackAcceptedServerSocket(sock)
		UDPCloseSocket(sock.udpConn)
	}
}

// loopback holds the active loopback driver instance (if any). It is
// created when a single-player game starts and allows the server side
// to detect a pending local connection via CheckNewConnections.
//
// listening indicates whether the server is accepting new connections.
// When true, the datagram driver polls its accept socket for incoming
// connection requests from remote clients.
var (
	loopback  *Loopback
	listening bool
)

// Listen toggles the server's willingness to accept new connections.
// When enabled, it opens (or keeps open) a UDP socket on the host port
// (default 26000) to receive connection-request control packets from
// clients. When disabled, the accept socket is closed. This corresponds
// to NET_Listen() in net_main.c.
func Listen(state bool) {
	listening = state
	if listening {
		if acceptSocket == nil {
			var err error
			acceptSocket, err = UDPOpenSocket(netHostPort)
			if err != nil {
				// Handle error
			}
		}
	} else {
		if acceptSocket != nil {
			UDPCloseSocket(acceptSocket)
			acceptSocket = nil
		}
	}
}

// CheckNewConnections polls all drivers for pending incoming connections.
// The loopback driver is checked first (for single-player), then the
// datagram driver (for multiplayer UDP clients). Returns a fully
// initialized Socket if a new connection is ready, or nil otherwise.
// The server calls this once per frame. Corresponds to
// NET_CheckNewConnections() in net_main.c.
func CheckNewConnections() *Socket {
	if loopback != nil {
		if sock := loopback.CheckNewConnections(); sock != nil {
			sock.driver = DriverLoopback
			return sock
		}
	}

	if listening {
		return DatagramCheckNewConnections()
	}

	return nil
}
