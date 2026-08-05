// network_manager.go re-exports srvnet.NetworkManager for package server.
package server

import (
	srvnet "github.com/darkliquid/ironwail-go/internal/server/net"
)

// NetworkManager encapsulates network message encoding, datagram dispatching, and sound emission.
type NetworkManager = srvnet.NetworkManager

// NewNetworkManager creates a new NetworkManager instance.
func NewNetworkManager(s *Server) *NetworkManager {
	return srvnet.NewNetworkManager(s)
}
