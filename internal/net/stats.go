package net

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// NetStats tracks network-level counters for diagnostics.
// Mirrors C net_dgrm.c global packet counters.
type NetStats struct {
	UnreliableSent     atomic.Int64
	UnreliableReceived atomic.Int64
	ReliableSent       atomic.Int64
	ReliableReceived   atomic.Int64
	PacketsSent        atomic.Int64
	PacketsResent      atomic.Int64
	PacketsReceived    atomic.Int64
	DuplicateCount     atomic.Int64
	ShortPacketCount   atomic.Int64
	DroppedDatagrams   atomic.Int64
}

// String returns a formatted summary of all network counters.
func (s *NetStats) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "unreliable messages sent   = %d\n", s.UnreliableSent.Load())
	fmt.Fprintf(&b, "unreliable messages recv   = %d\n", s.UnreliableReceived.Load())
	fmt.Fprintf(&b, "reliable messages sent     = %d\n", s.ReliableSent.Load())
	fmt.Fprintf(&b, "reliable messages received = %d\n", s.ReliableReceived.Load())
	fmt.Fprintf(&b, "packetsSent                = %d\n", s.PacketsSent.Load())
	fmt.Fprintf(&b, "packetsReSent              = %d\n", s.PacketsResent.Load())
	fmt.Fprintf(&b, "packetsReceived            = %d\n", s.PacketsReceived.Load())
	fmt.Fprintf(&b, "receivedDuplicateCount     = %d\n", s.DuplicateCount.Load())
	fmt.Fprintf(&b, "shortPacketCount           = %d\n", s.ShortPacketCount.Load())
	fmt.Fprintf(&b, "droppedDatagrams           = %d\n", s.DroppedDatagrams.Load())
	return b.String()
}

// Reset zeroes all counters.
func (s *NetStats) Reset() {
	s.UnreliableSent.Store(0)
	s.UnreliableReceived.Store(0)
	s.ReliableSent.Store(0)
	s.ReliableReceived.Store(0)
	s.PacketsSent.Store(0)
	s.PacketsResent.Store(0)
	s.PacketsReceived.Store(0)
	s.DuplicateCount.Store(0)
	s.ShortPacketCount.Store(0)
	s.DroppedDatagrams.Store(0)
}

// GlobalStats is the package-level network statistics counter set.
var GlobalStats NetStats
