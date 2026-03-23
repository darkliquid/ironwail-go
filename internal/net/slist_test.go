// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"encoding/binary"
	stdnet "net"
	"strings"
	"testing"
	"time"
)

func TestBuildServerInfoQuery(t *testing.T) {
	q := buildServerInfoQuery()
	if len(q) != HeaderSize+1 {
		t.Fatalf("query length = %d, want %d", len(q), HeaderSize+1)
	}
	header := binary.BigEndian.Uint32(q[0:])
	if header&FlagCtl == 0 {
		t.Fatal("FlagCtl not set")
	}
	seq := binary.BigEndian.Uint32(q[4:])
	if seq != 0xffffffff {
		t.Fatalf("sequence = %08x, want ffffffff", seq)
	}
	if q[8] != CCReqServerInfo {
		t.Fatalf("command = %02x, want %02x", q[8], CCReqServerInfo)
	}
}

func buildTestServerInfoResponse(address, hostname, mapName string, players, maxPlayers, proto byte) []byte {
	var payload []byte
	payload = append(payload, []byte(address)...)
	payload = append(payload, 0)
	payload = append(payload, []byte(hostname)...)
	payload = append(payload, 0)
	payload = append(payload, []byte(mapName)...)
	payload = append(payload, 0)
	payload = append(payload, players, maxPlayers, proto)

	pktLen := HeaderSize + 1 + len(payload)
	buf := make([]byte, pktLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(pktLen)|FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = CCRepServerInfo
	copy(buf[9:], payload)
	return buf
}

func TestParseServerInfoResponse(t *testing.T) {
	resp := buildTestServerInfoResponse("192.168.1.5:26000", "Test Server", "dm4", 3, 8, 3)
	addr := &stdnet.UDPAddr{IP: stdnet.IPv4(192, 168, 1, 5), Port: 26000}

	entry, ok := parseServerInfoResponse(resp, addr)
	if !ok {
		t.Fatal("parseServerInfoResponse returned false")
	}
	if entry.Name != "Test Server" {
		t.Errorf("Name = %q, want %q", entry.Name, "Test Server")
	}
	if entry.Map != "dm4" {
		t.Errorf("Map = %q, want %q", entry.Map, "dm4")
	}
	if entry.Players != 3 {
		t.Errorf("Players = %d, want 3", entry.Players)
	}
	if entry.MaxPlayers != 8 {
		t.Errorf("MaxPlayers = %d, want 8", entry.MaxPlayers)
	}
	if entry.Address != "192.168.1.5:26000" {
		t.Errorf("Address = %q, want %q", entry.Address, "192.168.1.5:26000")
	}
}

func TestParseServerInfoResponseFallbackAddress(t *testing.T) {
	resp := buildTestServerInfoResponse("localhost:26000", "MyServer", "e1m1", 0, 8, 3)
	addr := &stdnet.UDPAddr{IP: stdnet.IPv4(10, 0, 0, 5), Port: 26000}

	entry, ok := parseServerInfoResponse(resp, addr)
	if !ok {
		t.Fatal("parseServerInfoResponse returned false")
	}
	if !strings.Contains(entry.Address, "10.0.0.5") {
		t.Errorf("Address = %q, expected fallback to packet source", entry.Address)
	}
}

func TestParseServerInfoResponseRejectsNonCtl(t *testing.T) {
	buf := make([]byte, HeaderSize+10)
	binary.BigEndian.PutUint32(buf[0:], uint32(HeaderSize+10)|FlagData)
	buf[8] = CCRepServerInfo
	_, ok := parseServerInfoResponse(buf, &stdnet.UDPAddr{})
	if ok {
		t.Fatal("expected rejection of non-CTL packet")
	}
}

func TestParseServerInfoResponseRejectsWrongCommand(t *testing.T) {
	buf := make([]byte, HeaderSize+10)
	binary.BigEndian.PutUint32(buf[0:], uint32(HeaderSize+10)|FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = CCRepAccept
	_, ok := parseServerInfoResponse(buf, &stdnet.UDPAddr{})
	if ok {
		t.Fatal("expected rejection of wrong command")
	}
}

func TestParseServerInfoResponseTruncated(t *testing.T) {
	_, ok := parseServerInfoResponse([]byte{1, 2, 3}, &stdnet.UDPAddr{})
	if ok {
		t.Fatal("expected rejection of truncated packet")
	}
}

func TestServerBrowserStateTransitions(t *testing.T) {
	sb := NewServerBrowser()
	if sb.IsSearching() {
		t.Fatal("expected not searching initially")
	}
	if results := sb.Results(); len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

func TestServerBrowserAddEntryDedup(t *testing.T) {
	sb := NewServerBrowser()
	sb.addEntry(HostCacheEntry{Name: "A", Address: "1.2.3.4:26000"})
	sb.addEntry(HostCacheEntry{Name: "B", Address: "1.2.3.4:26000"})
	sb.addEntry(HostCacheEntry{Name: "C", Address: "5.6.7.8:26000"})
	results := sb.Results()
	if len(results) != 2 {
		t.Fatalf("expected 2 entries after dedup, got %d", len(results))
	}
}

func TestHostCacheEntryString(t *testing.T) {
	e := HostCacheEntry{
		Name:       "MyServer",
		Map:        "dm4",
		Players:    3,
		MaxPlayers: 8,
		Address:    "192.168.1.5:26000",
	}
	s := e.String()
	if !strings.Contains(s, "MyServer") || !strings.Contains(s, "dm4") || !strings.Contains(s, "3/8") {
		t.Errorf("String() = %q, missing expected fields", s)
	}
}

// TestServerBrowserWithLocalServer spins up a fake server that responds to
// CCReqServerInfo queries, then verifies the browser discovers it.
func TestServerBrowserWithLocalServer(t *testing.T) {
	// Start a fake server on a random port
	serverConn, err := stdnet.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer serverConn.Close()

	serverAddr := serverConn.LocalAddr().(*stdnet.UDPAddr)

	// Run fake server in background
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < HeaderSize+1 {
				continue
			}
			header := binary.BigEndian.Uint32(buf[0:])
			if header&FlagCtl == 0 || buf[8] != CCReqServerInfo {
				continue
			}
			resp := buildTestServerInfoResponse(
				serverAddr.String(), "LAN Test", "e1m1", 2, 8, 3,
			)
			serverConn.WriteTo(resp, addr)
		}
	}()

	// Create a browser and manually send to the known server port
	sb := NewServerBrowser()
	conn, err := stdnet.ListenPacket("udp4", ":0")
	if err != nil {
		t.Fatalf("failed to open client socket: %v", err)
	}
	defer conn.Close()

	query := buildServerInfoQuery()
	conn.WriteTo(query, serverAddr)

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, raddr, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("no response from fake server: %v", err)
	}

	entry, ok := parseServerInfoResponse(buf[:n], raddr)
	if !ok {
		t.Fatal("failed to parse response from fake server")
	}
	sb.addEntry(entry)

	results := sb.Results()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "LAN Test" {
		t.Errorf("Name = %q, want %q", results[0].Name, "LAN Test")
	}
	if results[0].Map != "e1m1" {
		t.Errorf("Map = %q, want %q", results[0].Map, "e1m1")
	}
	if results[0].Players != 2 {
		t.Errorf("Players = %d, want 2", results[0].Players)
	}
}

func TestServerBrowserStartAndWait(t *testing.T) {
	sb := NewServerBrowser()
	sb.Start()
	if !sb.IsSearching() {
		t.Fatal("expected searching after Start")
	}

	// Double-start should be a no-op
	sb.Start()

	sb.Wait()
	if sb.IsSearching() {
		t.Fatal("expected not searching after Wait")
	}
}

func TestServerBrowserRetryDelayMatchesCParity(t *testing.T) {
	if slistRetryDelay != 750*time.Millisecond {
		t.Fatalf("slistRetryDelay = %s, want %s", slistRetryDelay, 750*time.Millisecond)
	}
	if slistStopAfter != 1500*time.Millisecond {
		t.Fatalf("slistStopAfter = %s, want %s", slistStopAfter, 1500*time.Millisecond)
	}
	if slistRetryDelay >= slistStopAfter {
		t.Fatalf("retry delay %s must be before stop window %s", slistRetryDelay, slistStopAfter)
	}
}
