// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

// Entity update write tests split from sv_send_test.go.

import (
	"bytes"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestWriteEntityUpdate_OriginTolerance(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	newServerTestVM(s, 8)
	baseline := EntityState{
		Origin: [3]float32{100, 200, 300},
		Scale:  16,
	}

	tests := []struct {
		name       string
		originX    float32
		wantUpdate bool
		wantBit    uint32
	}{
		{
			name:       "within tolerance still emits visible baseline entity",
			originX:    100.1,
			wantUpdate: true,
		},
		{
			name:       "beyond tolerance sets origin1",
			originX:    100.1001,
			wantUpdate: true,
			wantBit:    inet.U_ORIGIN1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := baseline
			state.Origin[0] = tc.originX
			msg := NewMessageBuffer(256)
			gotUpdate := s.writeEntityUpdate(msg, 1, state, baseline, false, 0, 0, false)
			if gotUpdate != tc.wantUpdate {
				t.Fatalf("writeEntityUpdate update=%v, want %v", gotUpdate, tc.wantUpdate)
			}
			bits, _ := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
			if tc.wantBit == 0 {
				if bits != 0 {
					t.Fatalf("bits=%#x, want zero-delta visible entity header", bits)
				}
				return
			}
			if bits&tc.wantBit == 0 {
				t.Fatalf("bits=%#x missing expected bit %#x", bits, tc.wantBit)
			}
		})
	}
}

func TestWriteEntityUpdate_SetsUStepForStepMoveType(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	newServerTestVM(s, 8)
	state := EntityState{Scale: 16}
	baseline := state

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, baseline, false, float32(MoveTypeStep), 0, false) {
		t.Fatal("writeEntityUpdate returned false; expected U_STEP-only update")
	}

	bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
	if bits&inet.U_STEP == 0 {
		t.Fatalf("bits=%#x missing U_STEP", bits)
	}
	if len(payload) != 0 {
		t.Fatalf("U_STEP-only update wrote unexpected payload bytes: %v", payload)
	}
}

func TestWriteEntitiesToClient_UsesBaselineNotPreviousState(t *testing.T) {
	t.Parallel()

	ent := &Edict{Num: 1}

	client := &Client{
		Edict:        ent,
		EntityStates: make(map[int]EntityState),
	}
	s := &Server{
		Protocol:  ProtocolFitzQuake,
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{}, ent},
		NumEdicts: 2,
	}
	newServerTestVM(s, 8)
	ent.SetOrigin(s, [3]float32{64, 0, 0})

	currentState, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	ent.Baseline = currentState
	ent.Baseline.Origin[0] = 0
	client.EntityStates[1] = currentState

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)
	if msg.Len() == 0 {
		t.Fatal("writeEntitiesToClient wrote no update; expected baseline-relative delta")
	}

	bits, _ := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
	if bits&inet.U_ORIGIN1 == 0 {
		t.Fatalf("bits=%#x missing U_ORIGIN1 baseline delta", bits)
	}
}

func TestWriteEntitiesToClient_OmitsLerpFinishWithoutSendInterval(t *testing.T) {
	t.Parallel()

	ent := &Edict{
				NumLeafs: 1,
	}
	ent.LeafNums[0] = 0

	client := &Client{
		FatPVS:       []byte{0x01},
		EntityStates: make(map[int]EntityState),
	}
	s := &Server{
		Time:          10,
		Protocol:      ProtocolFitzQuake,
		Static:        &ServerStatic{MaxClients: 0},
		ModelPrecache: []string{"", "*1"},
		Edicts:        []*Edict{{}, ent},
		NumEdicts:     2,
	}
	newServerTestVM(s, 8)
	ent.SetModelIndex(s, 1)
	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false for visible brush entity")
	}
	ent.Baseline = state

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)
	if msg.Len() == 0 {
		t.Fatal("writeEntitiesToClient wrote no visible baseline-equal brush entity update")
	}

	bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
	if bits&inet.U_LERPFINISH != 0 {
		t.Fatalf("bits=%#x unexpectedly included U_LERPFINISH", bits)
	}
	if len(payload) != 0 {
		t.Fatalf("unexpected lerpfinish payload bytes: %v", payload)
	}
}

func TestWriteEntitiesToClient_EmitsLerpFinishOnlyWhenSendIntervalSet(t *testing.T) {
	t.Parallel()

	ent := &Edict{
		Num:          1,
		SendInterval: true,
		NumLeafs:     1,
	}
	ent.LeafNums[0] = 0

	client := &Client{
		FatPVS:       []byte{0x01},
		EntityStates: make(map[int]EntityState),
	}
	s := &Server{
		Time:          10,
		Protocol:      ProtocolFitzQuake,
		Static:        &ServerStatic{MaxClients: 0},
		ModelPrecache: []string{"", "*1"},
		Edicts:        []*Edict{{}, ent},
		NumEdicts:     2,
	}
	newServerTestVM(s, 8)
	ent.SetModelIndex(s, 1)
	ent.SetNextThink(s, 10.5)
	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false for visible brush entity")
	}
	ent.Baseline = state

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)
	if msg.Len() == 0 {
		t.Fatal("writeEntitiesToClient wrote no visible baseline-equal brush entity update")
	}

	bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
	if bits&inet.U_LERPFINISH == 0 {
		t.Fatalf("bits=%#x missing U_LERPFINISH", bits)
	}
	if !bytes.Equal(payload, []byte{128}) {
		t.Fatalf("payload=%v, want [128]", payload)
	}
}

func TestWriteEntitiesToClient_EmitsVisibleBaselineEqualBrushEntity(t *testing.T) {
	t.Parallel()

	ent := &Edict{
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 0

	client := &Client{
		FatPVS:       []byte{0x01},
		EntityStates: make(map[int]EntityState),
	}
	s := &Server{
		Protocol:      ProtocolFitzQuake,
		Static:        &ServerStatic{MaxClients: 0},
		ModelPrecache: []string{"", "*1"},
		Edicts:        []*Edict{{}, ent},
		NumEdicts:     2,
	}
	newServerTestVM(s, 8)
	ent.SetModelIndex(s, 1)
	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false for visible brush entity")
	}
	ent.Baseline = state

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)
	if msg.Len() == 0 {
		t.Fatal("writeEntitiesToClient wrote no visible baseline-equal brush entity update")
	}

	bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])
	if bits != 0 {
		t.Fatalf("bits=%#x, want zero-delta visible entity header", bits)
	}
	if len(payload) != 0 {
		t.Fatalf("zero-delta visible entity wrote unexpected payload bytes: %v", payload)
	}
	if state, ok := client.EntityStates[1]; !ok {
		t.Fatal("baseline-equal brush entity was not tracked for the client")
	} else if state.ModelIndex != 1 {
		t.Fatalf("tracked ModelIndex=%d, want 1", state.ModelIndex)
	}
}

func TestWriteEntitiesToClient_PrioritizesCloserVisibleEntitiesWhenPacketBudgetTight(t *testing.T) {
	t.Parallel()

	s := newPhysicsTestServer()

	far := &Edict{Num: 2}
	near := &Edict{Num: 3}
	clientEdict := &Edict{Num: 4}
	s.Edicts = []*Edict{s.Edicts[0], nil, far, near, clientEdict}
	s.NumEdicts = 5
	s.ensureQCVMEdictStorage()

	far.SetModelIndex(s, 5)
	far.SetOrigin(s, [3]float32{1000, 0, 0})
	far.SetAbsMin(s, [3]float32{999, -1, -1})
	far.SetAbsMax(s, [3]float32{1001, 1, 1})
	far.NumLeafs = 1
	far.LeafNums[0] = 0
	far.Baseline = EntityState{ModelIndex: 5, Origin: far.Origin(s), Scale: inet.ENTSCALE_DEFAULT}

	near.SetModelIndex(s, 6)
	near.SetOrigin(s, [3]float32{10, 0, 0})
	near.SetAbsMin(s, [3]float32{9, -1, -1})
	near.SetAbsMax(s, [3]float32{11, 1, 1})
	near.NumLeafs = 1
	near.LeafNums[0] = 0
	near.Baseline = EntityState{ModelIndex: 6, Origin: near.Origin(s), Scale: inet.ENTSCALE_DEFAULT}


	client := &Client{
		Edict:        clientEdict,
		FatPVS:       []byte{0x01},
		EntityStates: make(map[int]EntityState),
	}
	s.ModelPrecache = []string{"", "progs/player.mdl", "unused", "unused", "unused", "progs/far.mdl", "progs/near.mdl"}

	msg := NewMessageBuffer(41)
	s.writeEntitiesToClient(client, msg)

	if msg.Overflowed {
		t.Fatal("writeEntitiesToClient overflowed instead of stopping before packet budget was exhausted")
	}
	if got := msg.Len(); got != 2 {
		t.Fatalf("writeEntitiesToClient wrote %d bytes, want exactly one minimal entity update", got)
	}
	if got := msg.Data[1]; got != 3 {
		t.Fatalf("first transmitted entnum = %d, want nearer entity 3 before farther entity 2", got)
	}
	if _, ok := client.EntityStates[3]; !ok {
		t.Fatal("nearer entity was not tracked after transmission")
	}
	if _, ok := client.EntityStates[2]; ok {
		t.Fatal("farther entity should not have been transmitted once the packet budget was exhausted")
	}
}
