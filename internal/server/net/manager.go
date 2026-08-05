// manager.go implements the NetworkManager component for server network broadcasting.
package net

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Broadcaster handles particle and sound event emissions.
type Broadcaster interface {
	StartParticle(org, dir [3]float32, color, count int)
	StartSound(ent *srvtypes.Edict, channel int, sample string, volume int, attenuation float32)
}

// NetworkManager encapsulates network message encoding, datagram dispatching, and sound emission.
type NetworkManager struct {
	b Broadcaster
}

// NewNetworkManager creates a new NetworkManager wrapping network operations.
func NewNetworkManager(b Broadcaster) *NetworkManager {
	return &NetworkManager{
		b: b,
	}
}

// StartParticle broadcasts a particle effect event to connected clients.
func (n *NetworkManager) StartParticle(org, dir [3]float32, color, count int) {
	if n != nil && n.b != nil {
		n.b.StartParticle(org, dir, color, count)
	}
}

// StartSound broadcasts a sound emission event for an entity to connected clients.
func (n *NetworkManager) StartSound(ent *srvtypes.Edict, channel int, sample string, volume int, attenuation float32) {
	if n != nil && n.b != nil {
		n.b.StartSound(ent, channel, sample, volume, attenuation)
	}
}
