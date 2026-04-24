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
	"fmt"
	"time"
)

// startTime is now a field on the Network struct; defaultNet.startTime is
// used for the process-wide NetTime() entry point.

// NetTime returns the elapsed time in seconds since this Network was
// constructed (or since Init was called). Mirrors Sys_FloatTime() used by
// Quake's retransmission timers and timeout detection.
func (n *Network) NetTime() float64 {
	return time.Since(n.startTime).Seconds()
}

// NetTime delegates to the process-wide defaultNet's startTime anchor.
// Prefer Network.NetTime on a Host-owned instance.
func NetTime() float64 {
	return time.Since(defaultNet.startTime).Seconds()
}

// SetHostPort configures the UDP port this Network listens on for incoming
// connections. Values outside the valid 1..65534 range are ignored.
func (n *Network) SetHostPort(port int) {
	if port < 1 || port > 65534 {
		return
	}
	n.hostPort = port
	n.defaultHostPort = port
}

// HostPort returns the currently configured listen port.
func (n *Network) HostPort() int {
	return n.hostPort
}

// IsListening reports whether the server is currently accepting new
// connections.
func (n *Network) IsListening() bool {
	return n.listening
}

// SetHostPort configures the process-wide listen port. Values outside
// the valid 1..65534 range are ignored. Per-Host callers should use
// Network.SetHostPort on their own instance.
func SetHostPort(port int) {
	defaultNet.SetHostPort(port)
}

// HostPort returns the process-wide configured listen port.
func HostPort() int {
	return defaultNet.HostPort()
}

// IsListening reports whether the process-wide server is accepting new
// connections.
func IsListening() bool {
	return defaultNet.IsListening()
}

// Init brings up the transport drivers for this Network instance.
func (n *Network) Init() error {
	return n.UDPInit()
}

// Shutdown tears down the transport drivers for this Network instance.
func (n *Network) Shutdown() {
	_ = n.Listen(false)
	for _, sock := range n.accepted {
		if sock == nil || sock.udpConn == nil {
			continue
		}
		_ = UDPCloseSocket(sock.udpConn)
		sock.udpConn = nil
	}
	n.accepted = nil
	_ = n.SetIPBan("", "")
	if n.loopback != nil {
		n.loopback.Shutdown()
		n.loopback = nil
	}
	n.siProvider = nil
}

// Init initializes the process-wide networking subsystem. Delegates to
// defaultNet.Init().
func Init() error {
	return defaultNet.Init()
}

// Shutdown tears down the process-wide networking subsystem. Delegates to
// defaultNet.Shutdown().
func Shutdown() {
	defaultNet.Shutdown()
}

// Connect establishes a network connection to the given host on this
// Network. Loopback hosts produce a loopback driver socket; all other
// hosts dispatch through this Network's DatagramConnect.
func (n *Network) Connect(host string) *Socket {
	if host == "local" || host == "localhost" {
		l := NewLoopback()
		if err := l.Init(); err != nil {
			return nil
		}
		sock := l.Connect()
		sock.driver = DriverLoopback
		return sock
	}
	return n.DatagramConnect(host)
}

// Connect establishes a network connection on the process-wide defaultNet.
func Connect(host string) *Socket {
	return defaultNet.Connect(host)
}

// GetMessage polls a socket for incoming data. The return value indicates
// the message type: 0 = no message, 1 = reliable message, 2 = unreliable
// message, 3 = control message.
func (n *Network) GetMessage(sock *Socket) (int, []byte) {
	if sock.driver == DriverLoopback {
		return GetMessageLoopback(sock, nil)
	}
	return DatagramGetMessage(sock)
}

// GetMessage polls a socket on the process-wide defaultNet.
func GetMessage(sock *Socket) (int, []byte) {
	return defaultNet.GetMessage(sock)
}

// SendMessage sends a reliable message over the given socket.
func (n *Network) SendMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendMessageLoopback(sock, data)
	}
	return DatagramSendMessage(sock, data)
}

// SendMessage dispatches through the process-wide defaultNet.
func SendMessage(sock *Socket, data []byte) int {
	return defaultNet.SendMessage(sock, data)
}

// SendUnreliableMessage sends a fire-and-forget message on this Network.
func (n *Network) SendUnreliableMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendUnreliableMessageLoopback(sock, data)
	}
	return DatagramSendUnreliableMessage(sock, data)
}

// SendUnreliableMessage dispatches through the process-wide defaultNet.
func SendUnreliableMessage(sock *Socket, data []byte) int {
	return defaultNet.SendUnreliableMessage(sock, data)
}

// CanSendMessage reports whether sock can accept a new reliable message.
func (n *Network) CanSendMessage(sock *Socket) bool {
	if sock.driver == DriverLoopback {
		return true
	}
	return DatagramCanSendMessage(sock)
}

// CanSendMessage dispatches through the process-wide defaultNet.
func CanSendMessage(sock *Socket) bool {
	return defaultNet.CanSendMessage(sock)
}

// CanSendUnreliableMessage reports whether sock can accept an unreliable
// message right now.
func (n *Network) CanSendUnreliableMessage(sock *Socket) bool {
	if sock == nil {
		return false
	}
	return sock.CanSendUnreliable()
}

// CanSendUnreliableMessage dispatches through the process-wide defaultNet.
func CanSendUnreliableMessage(sock *Socket) bool {
	return defaultNet.CanSendUnreliableMessage(sock)
}

// Close shuts down a network connection on this Network.
func (n *Network) Close(sock *Socket) {
	if sock == nil {
		return
	}
	if sock.driver == DriverLoopback {
		CloseLoopback(sock)
	} else {
		n.untrackAcceptedServerSocket(sock)
		_ = UDPCloseSocket(sock.udpConn)
	}
}

// Close dispatches through the process-wide defaultNet.
func Close(sock *Socket) {
	defaultNet.Close(sock)
}

// Listen toggles this Network's willingness to accept new connections.
func (n *Network) Listen(state bool) error {
	if state {
		if n.acceptSocket == nil {
			sock, err := UDPOpenSocket(n.hostPort)
			if err != nil {
				n.listening = false
				return fmt.Errorf("listen on %d: %w", n.hostPort, err)
			}
			n.acceptSocket = sock
		}
		n.listening = true
		return nil
	}

	n.listening = false
	if n.acceptSocket != nil {
		if err := UDPCloseSocket(n.acceptSocket); err != nil {
			return fmt.Errorf("close listen socket on %d: %w", n.hostPort, err)
		}
		n.acceptSocket = nil
	}
	return nil
}

// Listen toggles the process-wide defaultNet's willingness to accept new
// connections.
func Listen(state bool) error {
	return defaultNet.Listen(state)
}

// CheckNewConnections polls all drivers on this Network for pending
// incoming connections.
func (n *Network) CheckNewConnections() *Socket {
	if n.loopback != nil {
		if sock := n.loopback.CheckNewConnections(); sock != nil {
			sock.driver = DriverLoopback
			return sock
		}
	}
	if n.listening {
		return n.DatagramCheckNewConnections()
	}
	return nil
}

// CheckNewConnections dispatches through the process-wide defaultNet.
func CheckNewConnections() *Socket {
	return defaultNet.CheckNewConnections()
}
