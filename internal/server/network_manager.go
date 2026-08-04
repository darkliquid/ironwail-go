// network_manager.go implements the NetworkBroadcaster interface as a standalone component.

// This file belongs to the Network/Protocol subsystem: server-to-client message encoding, client management, and protocol types.
package server

// NetworkManager encapsulates network message encoding, datagram dispatching, and sound emission.
type NetworkManager struct {
	server *Server
}

// NewNetworkManager creates a new NetworkManager wrapping network operations.
func NewNetworkManager(s *Server) *NetworkManager {
	return &NetworkManager{
		server: s,
	}
}

// StartParticle broadcasts a particle effect event to connected clients.
func (n *NetworkManager) StartParticle(org, dir [3]float32, color, count int) {
	n.server.StartParticle(org, dir, color, count)
}

// StartSound broadcasts a sound emission event for an entity to connected clients.
func (n *NetworkManager) StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32) {
	n.server.StartSound(ent, channel, sample, volume, attenuation)
}
