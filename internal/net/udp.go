// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// udp.go is the lowest layer of Quake's network stack — the actual
// network I/O layer using Go's net.UDPConn. It provides thin wrappers
// around Go's standard library UDP operations, abstracting platform
// differences behind a consistent API that the datagram layer calls.
//
// This file corresponds to net_udp.c / net_wins.c in the original
// Quake source (Unix and Windows UDP implementations respectively).
// In the Go port, the standard library handles cross-platform concerns,
// so this layer is intentionally minimal.
//
// Quake uses UDP (not TCP) because game networking requires low-latency
// delivery of small packets. TCP's in-order delivery guarantee and
// congestion control add unacceptable latency for real-time games.
// Instead, Quake implements its own reliability protocol in datagram.go
// on top of raw UDP.

import (
	"fmt"
	stdnet "net"
	"strings"
)

// Network configuration variables. These mirror the C Quake globals
// from net_udp.c / net_wins.c.
//
// netHostPort is the UDP port the server listens on (default 26000, the
// classic Quake port). Clients connect to this port for the initial
// handshake, though the server may redirect them to a different port.
//
// defaultNetHostPort preserves the original default for reset purposes.
//
// tcpipAvailable indicates whether UDP networking was successfully
// initialized. In the original C code, this could be false if Winsock
// failed to start; in Go, UDP is always available.
//
// myTCPIPAddress stores the local machine's IP address string, used
// in server info responses so LAN browser clients know how to connect.
var (
	netHostPort        = 26000
	defaultNetHostPort = 26000
	tcpipAvailable     = false
	myTCPIPAddress     string
)

// UDPInit initializes the UDP transport layer. In the original C engine,
// this performed Winsock initialization (Windows) or socket setup (Unix).
// In Go, the standard library handles these details, so this discovers
// the local IPv4 address and marks UDP as available.
// Corresponds to UDP_Init() in net_udp.c.
func UDPInit() error {
	// Discover local IPv4 address, matching C UDP_Init behavior.
	// Default to loopback (same as C: myAddr = htonl(INADDR_LOOPBACK)).
	myTCPIPAddress = "127.0.0.1"

	addrs, err := stdnet.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			ipNet, ok := a.(*stdnet.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}
			myTCPIPAddress = ip4.String()
			break
		}
	}

	tcpipAvailable = true
	return nil
}

// UDPOpenSocket opens a UDP socket bound to the specified port. Pass 0
// to let the OS assign a random ephemeral port (used by clients). The
// server calls this with netHostPort (default 26000) to create its
// accept socket. The socket is IPv4-only ("udp4") to match original
// Quake behavior. Corresponds to UDP_OpenSocket() in net_udp.c.
func UDPOpenSocket(port int) (*stdnet.UDPConn, error) {
	addr, err := stdnet.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := stdnet.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// UDPCloseSocket closes a UDP connection and releases the associated
// file descriptor / socket handle. Safely handles nil connections.
// Corresponds to UDP_CloseSocket() in net_udp.c.
func UDPCloseSocket(conn *stdnet.UDPConn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// UDPRead performs a non-blocking (or deadline-limited) read from a UDP
// socket, returning the number of bytes read, the sender's address, and
// any error. The caller must set read deadlines on the connection before
// calling if non-blocking behavior is desired. Wraps Go's ReadFromUDP.
// Corresponds to UDP_Read() in net_udp.c.
func UDPRead(conn *stdnet.UDPConn, buf []byte) (int, *stdnet.UDPAddr, error) {
	return conn.ReadFromUDP(buf)
}

// UDPWrite sends a UDP packet to the specified address. Returns the
// number of bytes written and any error. This is the final step in
// packet transmission — the datagram layer has already constructed
// the packet header and payload. Wraps Go's WriteToUDP.
// Corresponds to UDP_Write() in net_udp.c.
func UDPWrite(conn *stdnet.UDPConn, buf []byte, addr *stdnet.UDPAddr) (int, error) {
	return conn.WriteToUDP(buf, addr)
}

// UDPAddrToString converts a UDP address to its human-readable string
// form (e.g., "192.168.1.5:26000"). Returns empty string for nil
// addresses. Corresponds to UDP_AddrToString() in net_udp.c.
func UDPAddrToString(addr *stdnet.UDPAddr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

// UDPStringToAddr resolves a host:port string into a UDP address,
// performing DNS resolution if necessary. Used by DatagramConnect
// to turn user-provided server addresses into network endpoints.
// Corresponds to UDP_StringToAddr() in net_udp.c.
func UDPStringToAddr(address string) (*stdnet.UDPAddr, error) {
	if !strings.Contains(address, ":") {
		address = fmt.Sprintf("%s:%d", address, netHostPort)
	}
	if len(address) > 0 && address[0] >= '0' && address[0] <= '9' {
		expanded, err := PartialIPAddress(address, stdnet.ParseIP(myTCPIPAddress), netHostPort)
		if err != nil {
			return nil, err
		}
		address = expanded
	}
	return stdnet.ResolveUDPAddr("udp4", address)
}
