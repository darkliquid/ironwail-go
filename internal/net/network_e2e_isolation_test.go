// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"bytes"
	stdnet "net"
	"testing"
	"time"
)

// TestNetworkSendServerInfoResponseIsolation verifies end-to-end that
// two concurrently-configured *Network instances produce distinct
// CCRepServerInfo payloads reflecting their own siProvider state. This
// is the e2e complement to TestNetworkIsolation, which only checks
// field-level isolation. Here we actually call sendServerInfoResponse
// on each Network, read the bytes back through a real UDP loopback,
// and parse them to prove the state never crosses instances.
func TestNetworkSendServerInfoResponseIsolation(t *testing.T) {
	// Client socket (receives both responses).
	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("open client socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()
	clientAddr := clientConn.LocalAddr().(*stdnet.UDPAddr)

	// Separate "server" sockets for each Network. In production these
	// would be each Network's own acceptSocket; here we just need
	// distinct sending sources so each response can be attributed.
	serverConnA, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("open server A socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(serverConnA) }()
	serverConnB, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("open server B socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(serverConnB) }()

	netA := NewNetwork()
	netA.SetServerInfoProvider(&ServerInfoProvider{
		Hostname: func() string { return "alpha-host" },
		MapName:  func() string { return "e1m1" },
		Address:  func() string { return "10.0.0.1:26000" },
		Players:  func() int { return 1 },
		MaxPlayers: func() int {
			return 4
		},
	})

	netB := NewNetwork()
	netB.SetServerInfoProvider(&ServerInfoProvider{
		Hostname: func() string { return "beta-host" },
		MapName:  func() string { return "e4m6" },
		Address:  func() string { return "10.0.0.2:27000" },
		Players:  func() int { return 7 },
		MaxPlayers: func() int {
			return 8
		},
	})

	netA.sendServerInfoResponse(serverConnA, clientAddr)
	netB.sendServerInfoResponse(serverConnB, clientAddr)

	got := make(map[int][]byte, 2) // key = sender's source port
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer func() { _ = clientConn.SetReadDeadline(time.Time{}) }()
	for i := 0; i < 2; i++ {
		buf := make([]byte, 1024)
		n, src, err := UDPRead(clientConn, buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		got[src.Port] = buf[:n]
	}

	portA := serverConnA.LocalAddr().(*stdnet.UDPAddr).Port
	portB := serverConnB.LocalAddr().(*stdnet.UDPAddr).Port

	bytesA, okA := got[portA]
	if !okA {
		t.Fatalf("no response received from netA (port %d); got keys %v", portA, mapKeys(got))
	}
	bytesB, okB := got[portB]
	if !okB {
		t.Fatalf("no response received from netB (port %d); got keys %v", portB, mapKeys(got))
	}

	addrA, hostA, mapA := parseServerInfoPayload(t, bytesA)
	addrB, hostB, mapB := parseServerInfoPayload(t, bytesB)

	if hostA != "alpha-host" {
		t.Errorf("netA hostname = %q, want alpha-host", hostA)
	}
	if mapA != "e1m1" {
		t.Errorf("netA map = %q, want e1m1", mapA)
	}
	if addrA != "10.0.0.1:26000" {
		t.Errorf("netA address = %q, want 10.0.0.1:26000", addrA)
	}

	if hostB != "beta-host" {
		t.Errorf("netB hostname = %q, want beta-host", hostB)
	}
	if mapB != "e4m6" {
		t.Errorf("netB map = %q, want e4m6", mapB)
	}
	if addrB != "10.0.0.2:27000" {
		t.Errorf("netB address = %q, want 10.0.0.2:27000", addrB)
	}

	// Sanity: the two payloads must differ; if sendServerInfoResponse
	// ever reintroduced a shared global, the second response would
	// echo the first.
	if bytes.Equal(bytesA, bytesB) {
		t.Fatalf("netA and netB produced identical responses — DI regression")
	}
}

func parseServerInfoPayload(t *testing.T, pkt []byte) (addr, host, mapname string) {
	t.Helper()
	if len(pkt) < HeaderSize+1 {
		t.Fatalf("packet too short: %d bytes", len(pkt))
	}
	if pkt[HeaderSize] != CCRepServerInfo {
		t.Fatalf("cmd = %d, want CCRepServerInfo (%d)", pkt[HeaderSize], CCRepServerInfo)
	}
	body := pkt[HeaderSize+1:]

	parts := make([]string, 0, 3)
	for len(body) > 0 && len(parts) < 3 {
		idx := bytes.IndexByte(body, 0)
		if idx < 0 {
			t.Fatalf("malformed server-info payload: no NUL terminator")
		}
		parts = append(parts, string(body[:idx]))
		body = body[idx+1:]
	}
	if len(parts) != 3 {
		t.Fatalf("parsed %d strings, want 3 (addr, host, map)", len(parts))
	}
	return parts[0], parts[1], parts[2]
}

func mapKeys(m map[int][]byte) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
