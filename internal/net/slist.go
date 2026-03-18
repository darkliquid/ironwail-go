// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// slist.go implements the LAN server browser for Quake multiplayer.
// It discovers servers on the local network by broadcasting
// CCReqServerInfo control packets and collecting CCRepServerInfo
// responses. This mirrors the "slist" functionality from net_main.c
// in the original C engine.
//
// The discovery process follows the original Quake timing:
//   - T=0ms: first broadcast query sent to 255.255.255.255:26000
//   - T=500ms: retry broadcast (in case the first was lost)
//   - T=1500ms: search complete, results available
//
// Servers that receive the query respond with their hostname, current
// map, player count, max players, and protocol version. The browser
// deduplicates responses by address and presents the results to the
// player for server selection.
//
// The ServerBrowser type is safe for concurrent use. The search runs
// in a background goroutine, and Results() can be called at any time
// to get the current snapshot of discovered servers.

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	stdnet "net"
	"sort"
	"sync"
	"time"
)

// HostCacheEntry holds information about a discovered LAN server.
type HostCacheEntry struct {
	Name       string // Server hostname
	Map        string // Current map name
	Players    int    // Current player count
	MaxPlayers int    // Maximum player count
	Address    string // Network address (ip:port)
}

// String returns a one-line summary suitable for console display.
func (e HostCacheEntry) String() string {
	return fmt.Sprintf("%-15s %-8s %d/%d %s", e.Name, e.Map, e.Players, e.MaxPlayers, e.Address)
}

// ServerBrowser discovers LAN Quake servers by broadcasting query packets
// and collecting responses. This mirrors the C Ironwail Slist functionality
// from net_main.c.
type ServerBrowser struct {
	mu        sync.Mutex
	entries   []HostCacheEntry
	searching bool
	done      chan struct{}
}

// NewServerBrowser creates a ServerBrowser ready for use.
func NewServerBrowser() *ServerBrowser {
	return &ServerBrowser{}
}

// Start initiates an asynchronous LAN server search.
// The search broadcasts query packets and collects responses over ~1.5 seconds.
// Call Results() after IsSearching() returns false to retrieve discovered servers.
func (sb *ServerBrowser) Start() {
	sb.mu.Lock()
	if sb.searching {
		sb.mu.Unlock()
		return
	}
	sb.entries = nil
	sb.searching = true
	sb.done = make(chan struct{})
	sb.mu.Unlock()

	go sb.run()
}

// Results returns the current list of discovered servers, sorted by name.
// Matches C NET_SlistSort() which sorts the host cache alphabetically.
func (sb *ServerBrowser) Results() []HostCacheEntry {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	out := make([]HostCacheEntry, len(sb.entries))
	copy(out, sb.entries)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// IsSearching reports whether a search is currently in progress.
func (sb *ServerBrowser) IsSearching() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.searching
}

// Wait blocks until the current search completes. If no search is active
// it returns immediately.
func (sb *ServerBrowser) Wait() {
	sb.mu.Lock()
	ch := sb.done
	sb.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

// run executes the broadcast-poll cycle matching the C Ironwail timing:
//
//	T=0ms   — first broadcast
//	T=500ms — second broadcast (retry)
//	T=1500ms — stop
func (sb *ServerBrowser) run() {
	defer func() {
		sb.mu.Lock()
		sb.searching = false
		close(sb.done)
		sb.mu.Unlock()
	}()

	conn, err := stdnet.ListenPacket("udp4", ":0")
	if err != nil {
		slog.Error("slist: failed to open socket", "err", err)
		return
	}
	defer conn.Close()

	query := buildServerInfoQuery()

	// First broadcast
	sb.broadcast(conn, query)

	deadline := time.After(1500 * time.Millisecond)
	retryAt := time.After(500 * time.Millisecond)
	retried := false

	buf := make([]byte, 1024)
	for {
		select {
		case <-deadline:
			return
		case <-retryAt:
			if !retried {
				sb.broadcast(conn, query)
				retried = true
			}
		default:
		}

		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if entry, ok := parseServerInfoResponse(buf[:n], addr); ok {
			sb.addEntry(entry)
		}
	}
}

// broadcast sends the query packet to the broadcast address on the Quake port.
func (sb *ServerBrowser) broadcast(conn stdnet.PacketConn, query []byte) {
	broadcastAddr := &stdnet.UDPAddr{
		IP:   stdnet.IPv4bcast,
		Port: defaultNetHostPort,
	}
	if _, err := conn.WriteTo(query, broadcastAddr); err != nil {
		slog.Debug("slist: broadcast failed, trying localhost", "err", err)
		// Fallback: try localhost (works in loopback-only environments)
		localhost := &stdnet.UDPAddr{
			IP:   stdnet.IPv4(127, 0, 0, 1),
			Port: defaultNetHostPort,
		}
		conn.WriteTo(query, localhost)
	}
}

// buildServerInfoQuery constructs a CCReqServerInfo control packet.
func buildServerInfoQuery() []byte {
	// FlagCtl packet: 4 bytes header + 4 bytes sequence (0xffffffff) + 1 byte command
	const packetLen = HeaderSize + 1
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen)|FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = CCReqServerInfo
	return buf
}

// parseServerInfoResponse decodes a CCRepServerInfo control packet into a
// HostCacheEntry. Returns false if the packet is not a valid response.
func parseServerInfoResponse(data []byte, addr stdnet.Addr) (HostCacheEntry, bool) {
	if len(data) < HeaderSize+1 {
		return HostCacheEntry{}, false
	}
	header := binary.BigEndian.Uint32(data[0:])
	if header&FlagCtl == 0 {
		return HostCacheEntry{}, false
	}
	if data[8] != CCRepServerInfo {
		return HostCacheEntry{}, false
	}

	// Payload starts at offset 9: null-terminated strings and byte fields.
	// Format: address\0 hostname\0 mapname\0 players maxplayers protocol
	payload := data[9:]

	readString := func() string {
		for i, b := range payload {
			if b == 0 {
				s := string(payload[:i])
				payload = payload[i+1:]
				return s
			}
		}
		s := string(payload)
		payload = nil
		return s
	}

	address := readString()
	hostname := readString()
	mapName := readString()

	if len(payload) < 3 {
		return HostCacheEntry{}, false
	}
	players := int(payload[0])
	maxPlayers := int(payload[1])
	// payload[2] is protocol version — we accept any

	// Prefer the address from the actual packet source if the embedded
	// address looks like a placeholder.
	if address == "" || address == "localhost:26000" || address == "UNNAMED" {
		address = addr.String()
	}

	return HostCacheEntry{
		Name:       hostname,
		Map:        mapName,
		Players:    players,
		MaxPlayers: maxPlayers,
		Address:    address,
	}, true
}

// addEntry appends an entry if no duplicate address is already present.
func (sb *ServerBrowser) addEntry(entry HostCacheEntry) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for _, e := range sb.entries {
		if e.Address == entry.Address {
			return
		}
	}
	sb.entries = append(sb.entries, entry)
}
