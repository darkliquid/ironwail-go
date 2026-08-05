package net

import (
	"testing"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type mockBroadcaster struct {
	particleCalled bool
	soundCalled    bool
}

func (m *mockBroadcaster) StartParticle(org, dir [3]float32, color, count int) {
	m.particleCalled = true
}

func (m *mockBroadcaster) StartSound(ent *srvtypes.Edict, channel int, sample string, volume int, attenuation float32) {
	m.soundCalled = true
}

func TestNetworkManager_Delegation(t *testing.T) {
	b := &mockBroadcaster{}
	nm := NewNetworkManager(b)

	nm.StartParticle([3]float32{0, 0, 0}, [3]float32{0, 0, 1}, 1, 10)
	if !b.particleCalled {
		t.Error("StartParticle was not delegated")
	}

	nm.StartSound(&srvtypes.Edict{Num: 1}, 1, "test.wav", 255, 1.0)
	if !b.soundCalled {
		t.Error("StartSound was not delegated")
	}
}
