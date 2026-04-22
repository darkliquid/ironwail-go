// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// network.go declares *Network, the type that owns every piece of mutable
// networking state. Each Host constructs its own *Network via NewNetwork,
// and a process-wide defaultNet singleton backs the package-level free
// functions (Listen, SetIPBan, SetHostPort, Connect, etc.) so legacy
// callers that have not yet been threaded through a Host-owned Network
// continue to work.

import (
	stdnet "net"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// Network encapsulates every piece of mutable networking state — the IP
// ban list, host port configuration, listen/accept sockets, the loopback
// driver, and server-info plumbing — behind a single value that can be
// instantiated per Host for test isolation.
type Network struct {
	// cvars is the cvar system consulted by LAN/WAN server-info responses
	// for hostname lookups and server-info iteration.
	cvars *cvar.CVarSystem

	// siProvider supplies live server state for LAN browser queries.
	siProvider *ServerInfoProvider

	// ipBan holds the single active server IP ban.
	ipBan IPBan

	// hostPort is the UDP port on which the server listens for incoming
	// connections. Defaults to 26000.
	hostPort int

	// defaultHostPort records the originally-configured host port so that
	// the engine can restore it after a command-line override.
	defaultHostPort int

	// tcpipAvailable reports whether UDPInit successfully discovered a
	// usable IPv4 address.
	tcpipAvailable bool

	// myTCPIPAddress is the local machine's IPv4 address as a string,
	// advertised in server-info responses.
	myTCPIPAddress string

	// startTime anchors NetTime() — seconds since the networking
	// subsystem was initialised.
	startTime time.Time

	// loopback is the active single-player loopback driver.
	loopback *Loopback

	// listening indicates whether the server is accepting new
	// connections.
	listening bool

	// acceptSocket is the UDP socket on which the server listens for
	// incoming connection requests.
	acceptSocket *stdnet.UDPConn

	// accepted tracks currently-accepted datagram sockets so reconnects
	// from the same remote endpoint can close stale sockets before
	// creating a new one.
	accepted []*Socket
}

// NewNetwork constructs a Network with Quake-standard defaults:
//   - hostPort 26000
//   - startTime set to time.Now()
//   - all other fields zero-valued
//
// The returned Network is ready to use but holds no live sockets; Init
// brings up the UDP transport layer.
func NewNetwork() *Network {
	return &Network{
		hostPort:        26000,
		defaultHostPort: 26000,
		startTime:       time.Now(),
	}
}

// defaultNet is the process-wide Network instance that package-level free
// functions (SetIPBan, Connect, Listen, etc.) delegate to. Host-owned
// Networks (constructed via NewNetwork) are independent of this singleton;
// defaultNet exists for legacy callers that have not yet been threaded
// through a *Network.
var defaultNet = NewNetwork()

// DefaultNetwork returns the process-wide Network singleton. Callers that
// want true per-instance isolation should construct their own *Network
// via NewNetwork instead.
func DefaultNetwork() *Network {
	return defaultNet
}
