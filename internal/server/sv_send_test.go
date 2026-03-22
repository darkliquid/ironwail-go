package server

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestEncodeAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "zero", in: 0.0, want: inet.ENTALPHA_DEFAULT},
		{name: "half", in: 0.5, want: 128},
		{name: "one", in: 1.0, want: inet.ENTALPHA_ONE},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := inet.ENTALPHA_ENCODE(tc.in); got != tc.want {
				t.Fatalf("ENTALPHA_ENCODE(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEncodeScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "one", in: 1.0, want: 16},
		{name: "two", in: 2.0, want: 32},
		{name: "zero", in: 0.0, want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encodeScale(tc.in); got != tc.want {
				t.Fatalf("encodeScale(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEntityStateForClient_AlphaScaleDefaultsWhenFieldsMissing(t *testing.T) {
	t.Parallel()

	s := &Server{
		QCVM:         newTestQCVM(),
		QCFieldAlpha: -1,
		QCFieldScale: -1,
	}
	ent := &Edict{
		Vars:  &EntVars{},
		Alpha: 77,
		Scale: 99,
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 0 {
		t.Fatalf("state.Alpha = %d, want 0", state.Alpha)
	}
	if state.Scale != 16 {
		t.Fatalf("state.Scale = %d, want 16", state.Scale)
	}
}

func TestEntityStateForClient_ReadsQCAlphaScale(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.SetEFloat(1, 0, 0.5) // alpha
	vm.SetEFloat(1, 1, 2.0) // scale

	s := &Server{
		QCVM:         vm,
		QCFieldAlpha: 0,
		QCFieldScale: 1,
	}
	ent := &Edict{
		Vars: &EntVars{},
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 128 {
		t.Fatalf("state.Alpha = %d, want 128", state.Alpha)
	}
	if state.Scale != 32 {
		t.Fatalf("state.Scale = %d, want 32", state.Scale)
	}
}

func TestEntityStateForClient_AppliesEffectsMask(t *testing.T) {
	t.Parallel()

	s := &Server{
		EffectsMask: 0x0f,
	}
	ent := &Edict{
		Vars: &EntVars{
			Effects: float32(EffectMuzzleFlash | EffectPentaLight),
		},
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Effects != EffectMuzzleFlash {
		t.Fatalf("state.Effects = %#x, want %#x", state.Effects, EffectMuzzleFlash)
	}
}

func newTestQCVM() *qc.VM {
	vm := &qc.VM{
		NumEdicts: 2,
		EdictSize: 28 + 8, // prefix + 2 float fields
	}
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	return vm
}

func TestEncodeLerpFinish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nextThink float32
		time      float32
		want      byte
		ok        bool
	}{
		{name: "zero delta omitted", nextThink: 10.0, time: 10.0, want: 0, ok: false},
		{name: "half second", nextThink: 10.5, time: 10.0, want: 128, ok: true},
		{name: "clamped to one second", nextThink: 12.0, time: 10.0, want: 255, ok: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := encodeLerpFinish(tc.nextThink, tc.time)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("encodeLerpFinish(%v, %v) = (%d, %v), want (%d, %v)", tc.nextThink, tc.time, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestWriteEntityUpdate_FieldOrderMatchesCProtocol(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		Origin:     [3]float32{1.25, 2.5, 3.75},
		Angles:     [3]float32{10, 20, 30},
		ModelIndex: 0x123,
		Frame:      0x234,
		Colormap:   4,
		Skin:       5,
		Effects:    6,
		Alpha:      7,
		Scale:      8,
	}

	msg := NewMessageBuffer(512)
	if !s.writeEntityUpdate(msg, 1, state, EntityState{}, true, 0, 200, true) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	want := NewMessageBuffer(512)
	flags := uint32(s.ProtocolFlags())
	want.WriteByte(byte(state.ModelIndex))
	want.WriteByte(byte(state.Frame))
	want.WriteByte(byte(state.Colormap))
	want.WriteByte(byte(state.Skin))
	want.WriteByte(byte(state.Effects))
	want.WriteCoord(state.Origin[0], flags)
	want.WriteAngle(state.Angles[0], flags)
	want.WriteCoord(state.Origin[1], flags)
	want.WriteAngle(state.Angles[1], flags)
	want.WriteCoord(state.Origin[2], flags)
	want.WriteAngle(state.Angles[2], flags)
	want.WriteByte(state.Alpha)
	want.WriteByte(state.Scale)
	want.WriteByte(byte(state.Frame >> 8))
	want.WriteByte(byte(state.ModelIndex >> 8))
	want.WriteByte(200)

	if !bytes.Equal(payload, want.Data[:want.Len()]) {
		t.Fatalf("payload order mismatch:\n got: %v\nwant: %v", payload, want.Data[:want.Len()])
	}
}

func TestBuildClientDatagramUsesEyePositionForFatPVS(t *testing.T) {
	s := &Server{
		Datagram: NewMessageBuffer(MaxDatagram),
		WorldTree: &bsp.Tree{
			Planes: []bsp.DPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}},
			Nodes: []bsp.TreeNode{{
				PlaneNum: 0,
				Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 2}},
			}},
			Leafs: []bsp.TreeLeaf{
				{Contents: bsp.ContentsSolid, VisOfs: -1},
				{Contents: 0, VisOfs: 0},
				{Contents: 0, VisOfs: 1},
			},
			Visibility: []byte{0x01, 0x02},
			Models:     []bsp.DModel{{VisLeafs: 2}},
		},
	}
	client := &Client{Edict: &Edict{Vars: &EntVars{
		Origin:  [3]float32{-64, 0, 0},
		ViewOfs: [3]float32{128, 0, 0},
	}}}
	msg := NewMessageBuffer(128)

	s.buildClientDatagram(client, msg)

	if len(client.FatPVS) == 0 || client.FatPVS[0] != 0x01 {
		t.Fatalf("FatPVS = %v, want visibility from eye position leaf", client.FatPVS)
	}
}

func TestUpdateToReliableMessagesQueuesNonClientStatsAndUnderwaterOverride(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	client := s.Static.Clients[0]
	client.Active = true
	client.Message.Clear()
	client.Stats[inet.StatSecrets] = 7
	client.Edict.ForceWater = true
	client.Edict.SendForceWater = true

	s.UpdateToReliableMessages()

	data := client.Message.Data[:client.Message.Len()]
	if !bytes.Contains(data, []byte{byte(inet.SVCUpdateStat), byte(inet.StatSecrets)}) {
		t.Fatalf("reliable message missing StatSecrets update: %v", data)
	}
	if !bytes.Contains(data, []byte("//v_water 1\n")) {
		t.Fatalf("reliable message missing underwater override: %q", string(data))
	}
	if client.Edict.SendForceWater {
		t.Fatal("SendForceWater should be cleared after reliable override write")
	}
}

func TestBuildClientDatagramOmitsReliableStatUpdates(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	client := s.Static.Clients[0]
	client.Spawned = true
	client.Stats[inet.StatSecrets] = 7

	msg := NewMessageBuffer(MaxDatagram)
	s.buildClientDatagram(client, msg)

	data := msg.Data[:msg.Len()]
	if bytes.Contains(data, []byte{byte(inet.SVCUpdateStat), byte(inet.StatSecrets)}) {
		t.Fatalf("client datagram unexpectedly contains reliable stat update: %v", data)
	}
}

func TestWriteEntitiesToClientCullOtherPlayersByPVS(t *testing.T) {
	s := &Server{
		Datagram:      NewMessageBuffer(MaxDatagram),
		Static:        &ServerStatic{MaxClients: 2},
		ModelPrecache: []string{"", "progs/player.mdl"},
		NumEdicts:     3,
		Edicts: []*Edict{
			{Vars: &EntVars{}},
			{Vars: &EntVars{Origin: [3]float32{0, 0, 0}, ModelIndex: 1}},
			{Vars: &EntVars{Origin: [3]float32{64, 0, 0}, ModelIndex: 1}},
		},
	}
	s.Static.Clients = []*Client{{Edict: s.Edicts[1], FatPVS: []byte{0x01}, EntityStates: map[int]EntityState{}}}
	s.Edicts[2].NumLeafs = 1
	s.Edicts[2].LeafNums[0] = 1
	msg := NewMessageBuffer(256)

	s.writeEntitiesToClient(s.Static.Clients[0], msg)

	if _, ok := s.Static.Clients[0].EntityStates[2]; ok {
		t.Fatalf("other player outside PVS was still transmitted")
	}
}

func TestBuildClientDatagramSkipsDatagramWhenRemoteMTUWouldOverflow(t *testing.T) {
	s := &Server{Datagram: NewMessageBuffer(MaxDatagram)}
	client := &Client{Edict: &Edict{Vars: &EntVars{}}}
	base := NewMessageBuffer(MaxDatagram)
	base.MaxSize = DatagramMTU
	s.buildClientDatagram(client, base)
	baseLen := base.Len()
	if baseLen == 0 {
		t.Fatal("expected base datagram payload")
	}

	s.Datagram = NewMessageBuffer(MaxDatagram)
	for i := 0; i < DatagramMTU-baseLen; i++ {
		s.Datagram.WriteByte(0x42)
	}

	msg := NewMessageBuffer(MaxDatagram)
	msg.MaxSize = DatagramMTU
	s.buildClientDatagram(client, msg)

	if got := msg.Len(); got != baseLen {
		t.Fatalf("remote datagram len = %d, want %d (base payload only)", got, baseLen)
	}
}

func TestWriteEntityUpdate_OriginsAnglesInterleaved(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		Origin: [3]float32{10, 20, 30},
		Angles: [3]float32{40, 50, 60},
	}
	prev := state
	prev.Origin = [3]float32{}
	prev.Angles = [3]float32{}

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, 0, false) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	want := NewMessageBuffer(256)
	flags := uint32(s.ProtocolFlags())
	want.WriteCoord(state.Origin[0], flags)
	want.WriteAngle(state.Angles[0], flags)
	want.WriteCoord(state.Origin[1], flags)
	want.WriteAngle(state.Angles[1], flags)
	want.WriteCoord(state.Origin[2], flags)
	want.WriteAngle(state.Angles[2], flags)

	if !bytes.Equal(payload, want.Data[:want.Len()]) {
		t.Fatalf("origin/angle interleave mismatch:\n got: %v\nwant: %v", payload, want.Data[:want.Len()])
	}
}

func TestWriteEntityUpdate_Frame2Model2AfterAlphaScale(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		ModelIndex: 0x345,
		Frame:      0x267,
		Alpha:      0x89,
		Scale:      0x9a,
	}
	prev := EntityState{
		ModelIndex: 1,
		Frame:      1,
		Alpha:      0,
		Scale:      16,
	}

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, 0, false) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	// Byte fields only in this test: MODEL, FRAME, ALPHA, SCALE, FRAME2, MODEL2
	want := []byte{
		byte(state.ModelIndex),
		byte(state.Frame),
		state.Alpha,
		state.Scale,
		byte(state.Frame >> 8),
		byte(state.ModelIndex >> 8),
	}

	if !bytes.Equal(payload, want) {
		t.Fatalf("FRAME2/MODEL2 order mismatch:\n got: %v\nwant: %v", payload, want)
	}
}

func TestWriteEntityUpdate_NetQuakeOmitsFitzExtensions(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolNetQuake}
	state := EntityState{
		ModelIndex: 0x345,
		Frame:      0x267,
		Alpha:      0x89,
		Scale:      0x9a,
	}
	prev := EntityState{
		ModelIndex: 1,
		Frame:      1,
		Alpha:      0,
		Scale:      16,
	}

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, 200, true) {
		t.Fatal("writeEntityUpdate returned false")
	}

	bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	if bits&inet.U_ALPHA != 0 || bits&inet.U_SCALE != 0 || bits&inet.U_FRAME2 != 0 || bits&inet.U_MODEL2 != 0 || bits&inet.U_LERPFINISH != 0 {
		t.Fatalf("netquake unexpectedly set extension bits: %#x", bits)
	}
	if bits&inet.U_EXTEND1 != 0 || bits&inet.U_EXTEND2 != 0 {
		t.Fatalf("netquake unexpectedly set extension header bits: %#x", bits)
	}

	want := []byte{
		byte(state.ModelIndex),
		byte(state.Frame),
	}
	if !bytes.Equal(payload, want) {
		t.Fatalf("netquake payload contains unexpected extension bytes:\n got: %v\nwant: %v", payload, want)
	}
}

func TestWriteEntityUpdate_NonNetQuakeSetsFitzExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		protocol int
	}{
		{name: "fitzquake", protocol: ProtocolFitzQuake},
		{name: "rmq", protocol: ProtocolRMQ},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := &Server{Protocol: tc.protocol}
			state := EntityState{
				ModelIndex: 0x345,
				Frame:      0x267,
				Alpha:      0x89,
				Scale:      0x9a,
			}
			prev := EntityState{
				ModelIndex: 1,
				Frame:      1,
				Alpha:      0,
				Scale:      16,
			}

			msg := NewMessageBuffer(256)
			if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, 200, true) {
				t.Fatal("writeEntityUpdate returned false")
			}

			bits, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

			required := uint32(inet.U_ALPHA | inet.U_SCALE | inet.U_FRAME2 | inet.U_MODEL2 | inet.U_LERPFINISH)
			if bits&required != required {
				t.Fatalf("%s missing extension bits: bits=%#x required=%#x", tc.name, bits, required)
			}
			if bits&inet.U_EXTEND1 == 0 {
				t.Fatalf("%s missing U_EXTEND1 for extension bits: %#x", tc.name, bits)
			}

			want := []byte{
				byte(state.ModelIndex),
				byte(state.Frame),
				state.Alpha,
				state.Scale,
				byte(state.Frame >> 8),
				byte(state.ModelIndex >> 8),
				200,
			}
			if !bytes.Equal(payload, want) {
				t.Fatalf("%s payload mismatch:\n got: %v\nwant: %v", tc.name, payload, want)
			}
		})
	}
}

func TestWriteEntityUpdate_OriginTolerance(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
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

	ent := &Edict{
		Vars: &EntVars{
			Origin: [3]float32{10, 0, 0},
		},
	}
	client := &Client{
		Edict:        ent,
		EntityStates: make(map[int]EntityState),
	}
	s := &Server{
		Protocol:  ProtocolFitzQuake,
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{Vars: &EntVars{}}, ent},
		NumEdicts: 2,
	}

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

func TestWriteEntitiesToClient_EmitsVisibleBaselineEqualBrushEntity(t *testing.T) {
	t.Parallel()

	ent := &Edict{
		Vars: &EntVars{
			Model:      1,
			ModelIndex: 1,
			Origin:     [3]float32{0, 0, 0},
		},
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
		Edicts:        []*Edict{{Vars: &EntVars{}}, ent},
		NumEdicts:     2,
	}
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

func TestWriteClientDataToMessage_NetQuakeOmitsExtensions(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolNetQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	ent := &Edict{
		Vars: &EntVars{
			WeaponModel: weaponModel,
			WeaponFrame: 0x234,
			ArmorValue:  0x345,
			Health:      100,
			CurrentAmmo: 0x456,
			AmmoShells:  0x567,
			AmmoNails:   0x678,
			AmmoRockets: 0x789,
			AmmoCells:   0x89a,
		},
		Alpha: 0x7f,
	}

	msg := NewMessageBuffer(512)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	extBits := uint32(
		inet.SU_EXTEND1 | inet.SU_EXTEND2 |
			inet.SU_WEAPON2 | inet.SU_ARMOR2 | inet.SU_AMMO2 |
			inet.SU_SHELLS2 | inet.SU_NAILS2 | inet.SU_ROCKETS2 | inet.SU_CELLS2 |
			inet.SU_WEAPONFRAME2 | inet.SU_WEAPONALPHA,
	)
	if bits&extBits != 0 {
		t.Fatalf("netquake unexpectedly set extension bits: %#x", bits&extBits)
	}

	// NetQuake payload ends after base fields only.
	if len(payload) != 16 {
		t.Fatalf("netquake payload length = %d, want 16; payload=%v", len(payload), payload)
	}
}

func TestWriteClientDataToMessage_FitzSendsWeapon2(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolFitzQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	ent := &Edict{
		Vars: &EntVars{
			WeaponModel: weaponModel,
			Health:      100,
		},
	}

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	if bits&inet.SU_WEAPON2 == 0 {
		t.Fatalf("missing SU_WEAPON2 bit: %#x", bits)
	}
	if bits&inet.SU_EXTEND1 == 0 {
		t.Fatalf("missing SU_EXTEND1 bit for SU_WEAPON2: %#x", bits)
	}
	if bits&inet.SU_EXTEND2 != 0 {
		t.Fatalf("unexpected SU_EXTEND2 bit: %#x", bits)
	}

	if got, want := payload[len(payload)-1], byte(0x01); got != want {
		t.Fatalf("weapon2 high byte = %#x, want %#x; payload=%v", got, want, payload)
	}
}

func TestWriteClientDataToMessage_FitzExtensionsPayloadOrder(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolFitzQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	ent := &Edict{
		Vars: &EntVars{
			WeaponModel: weaponModel,
			WeaponFrame: 0x234,
			ArmorValue:  0x345,
			Health:      100,
			CurrentAmmo: 0x456,
			AmmoShells:  0x567,
			AmmoNails:   0x678,
			AmmoRockets: 0x789,
			AmmoCells:   0x89a,
		},
		Alpha: 0x7f,
	}

	msg := NewMessageBuffer(512)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	required := uint32(
		inet.SU_EXTEND1 | inet.SU_EXTEND2 |
			inet.SU_WEAPON2 | inet.SU_ARMOR2 | inet.SU_AMMO2 |
			inet.SU_SHELLS2 | inet.SU_NAILS2 | inet.SU_ROCKETS2 | inet.SU_CELLS2 |
			inet.SU_WEAPONFRAME2 | inet.SU_WEAPONALPHA,
	)
	if bits&required != required {
		t.Fatalf("missing extension bits: bits=%#x required=%#x", bits, required)
	}

	got := payload[len(payload)-9:]
	want := []byte{
		0x01, // SU_WEAPON2
		0x03, // SU_ARMOR2
		0x04, // SU_AMMO2
		0x05, // SU_SHELLS2
		0x06, // SU_NAILS2
		0x07, // SU_ROCKETS2
		0x08, // SU_CELLS2
		0x02, // SU_WEAPONFRAME2
		0x7f, // SU_WEAPONALPHA
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extension payload order mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestWriteClientDataToMessage_FixAngleUsesVAngle(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.FixAngle = 1
	ent.Vars.Angles = [3]float32{10, 20, 30}
	ent.Vars.VAngle = [3]float32{90, 180, 270}

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	data := msg.Data[:msg.Len()]
	if len(data) < 4 {
		t.Fatalf("short message: %v", data)
	}
	if got, want := data[0], byte(inet.SVCSetAngle); got != want {
		t.Fatalf("message[0] = %d, want %d", got, want)
	}

	want := NewMessageBuffer(16)
	flags := uint32(s.ProtocolFlags())
	want.WriteAngle(ent.Vars.VAngle[0], flags)
	want.WriteAngle(ent.Vars.VAngle[1], flags)
	want.WriteAngle(ent.Vars.VAngle[2], flags)
	if got := data[1:4]; !bytes.Equal(got, want.Data[:want.Len()]) {
		t.Fatalf("setangle payload = %v, want %v", got, want.Data[:want.Len()])
	}
	if ent.Vars.FixAngle != 0 {
		t.Fatalf("FixAngle = %v, want 0", ent.Vars.FixAngle)
	}
}

func TestWriteClientDataToMessage_SendsBaseWeaponBitmask(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolNetQuake}
	ent := &Edict{
		Vars: &EntVars{
			Weapon:      1 << 5,
			Health:      100,
			CurrentAmmo: 5,
		},
	}

	msg := NewMessageBuffer(128)
	s.WriteClientDataToMessage(ent, msg)

	_, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])
	if got, want := payload[len(payload)-1], byte(1<<5); got != want {
		t.Fatalf("active weapon byte = %#x, want %#x; payload=%v", got, want, payload)
	}
}

func TestWriteClientDataToMessage_LogsPhysicsTelemetry(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 4)
	s := &Server{
		Protocol: ProtocolNetQuake,
		DebugTelemetry: NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
			return DebugTelemetryConfig{
				Enabled:      true,
				EventMask:    debugEventMaskPhysics,
				EntityFilter: debugEntityFilter{all: true},
				SummaryMode:  0,
			}
		}, func(line string) {
			lines = append(lines, line)
		}),
	}
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_clientdata", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	ent := &Edict{
		Vars: &EntVars{
			Flags:        FlagPartialGround,
			ViewOfs:      [3]float32{0, 0, 22},
			IdealPitch:   7,
			Velocity:     [3]float32{0, 1840, 0},
			PunchAngle:   [3]float32{105, 0, 32},
			FixAngle:     1,
			GroundEntity: 99,
			TeleportTime: 1.25,
			Health:       100,
		},
	}
	s.Edicts = []*Edict{{Vars: &EntVars{}}, ent}

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"kind=physics",
		"clientdata serialize bits=",
		"onground=false",
		"vel=(0.0 1840.0 0.0)",
		"punch=(105.0 0.0 32.0)",
		"fixangle_sent=true",
		"ground=99",
		"teleport=1.250",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}

func TestWriteEntitiesToClient_SkipsEntAlphaZero(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.SetEFloat(1, 0, 0.001) // tiny positive alpha rounds to ENTALPHA_ZERO

	ent := &Edict{
		Vars: &EntVars{},
	}
	client := &Client{
		Edict: ent,
	}
	s := &Server{
		Protocol:     ProtocolFitzQuake,
		Static:       &ServerStatic{MaxClients: 1},
		QCVM:         vm,
		QCFieldAlpha: 0,
		QCFieldScale: -1,
		Edicts:       []*Edict{{}, ent},
		NumEdicts:    2,
	}

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)

	if got := msg.Len(); got != 0 {
		t.Fatalf("writeEntitiesToClient wrote %d bytes for ENTALPHA_ZERO entity, want 0", got)
	}
	if _, ok := client.EntityStates[1]; ok {
		t.Fatal("ENTALPHA_ZERO entity should not be tracked in client.EntityStates")
	}
}

func TestWriteEntitiesToClient_RetiresBaselineOnlyEntity(t *testing.T) {
	t.Parallel()

	ent := &Edict{
		Vars:     &EntVars{},
		Baseline: EntityState{ModelIndex: 5, Scale: inet.ENTSCALE_DEFAULT},
	}
	client := &Client{}
	s := &Server{
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{}, ent},
		NumEdicts: 2,
	}

	s.seedClientEntityStatesFromBaselines(client)
	if _, ok := client.EntityStates[1]; !ok {
		t.Fatal("expected baseline entity state to be seeded for retire tracking")
	}

	ent.Free = true
	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)

	if got := msg.Len(); got == 0 {
		t.Fatal("writeEntitiesToClient wrote no retire update for baseline-only entity")
	}
	if state, ok := client.EntityStates[1]; !ok {
		t.Fatal("baseline-only entity should stay tracked for sticky retire updates")
	} else if state.ModelIndex != 0 {
		t.Fatalf("sticky retire tracked ModelIndex=%d, want 0", state.ModelIndex)
	}
}

func TestWriteEntitiesToClient_RepeatsRetireUntilOverwritten(t *testing.T) {
	t.Parallel()

	ent := &Edict{
		Vars:     &EntVars{},
		Baseline: EntityState{ModelIndex: 5, Scale: inet.ENTSCALE_DEFAULT},
	}
	client := &Client{}
	s := &Server{
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{}, ent},
		NumEdicts: 2,
	}

	s.seedClientEntityStatesFromBaselines(client)
	ent.Free = true

	first := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, first)
	if got := first.Len(); got == 0 {
		t.Fatal("first retire update was empty")
	}

	second := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, second)
	if got := second.Len(); got == 0 {
		t.Fatal("second retire update was empty; want sticky re-send")
	}
	if state := client.EntityStates[1]; state.ModelIndex != 0 {
		t.Fatalf("sticky retire ModelIndex=%d, want 0", state.ModelIndex)
	}
}

func decodeClientDataBitsAndPayload(t *testing.T, data []byte) (uint32, []byte) {
	t.Helper()
	if len(data) < 3 {
		t.Fatalf("short clientdata message: %v", data)
	}
	if got, want := data[0], byte(inet.SVCClientData); got != want {
		t.Fatalf("message type = %d, want %d", got, want)
	}

	i := 1
	bits := uint32(data[i]) | uint32(data[i+1])<<8
	i += 2
	if bits&inet.SU_EXTEND1 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend1 byte in %v", data)
		}
		bits |= uint32(data[i]) << 16
		i++
	}
	if bits&inet.SU_EXTEND2 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend2 byte in %v", data)
		}
		bits |= uint32(data[i]) << 24
		i++
	}
	return bits, data[i:]
}

func decodeEntityUpdateBitsAndPayload(t *testing.T, data []byte) (uint32, []byte) {
	t.Helper()
	if len(data) < 2 {
		t.Fatalf("short entity update: %v", data)
	}
	i := 0
	first := data[i]
	i++
	bits := uint32(first & 0x7f)
	if bits&inet.U_MOREBITS != 0 {
		if i >= len(data) {
			t.Fatalf("missing morebits byte in %v", data)
		}
		bits |= uint32(data[i]) << 8
		i++
	}
	if bits&inet.U_EXTEND1 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend1 byte in %v", data)
		}
		bits |= uint32(data[i]) << 16
		i++
	}
	if bits&inet.U_EXTEND2 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend2 byte in %v", data)
		}
		bits |= uint32(data[i]) << 24
		i++
	}
	if bits&inet.U_LONGENTITY != 0 {
		i += 2
	} else {
		i++
	}
	if i > len(data) {
		t.Fatalf("invalid entity header in %v", data)
	}
	return bits, data[i:]
}
