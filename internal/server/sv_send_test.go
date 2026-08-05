// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"bytes"
	"io"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
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
	newServerTestVM(s, 8)
	ent := &Edict{
		Alpha: 77,
		Scale: 99,
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 77 {
		t.Fatalf("state.Alpha = %d, want 77", state.Alpha)
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
	ent := &Edict{Num: 1}

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
	newServerTestVM(s, 8)
	ent := &Edict{Num: 1}
	ent.SetEffects(s, float32(EffectMuzzleFlash))

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
		EdictSize: 28 + 128*4, // prefix + 128 float fields
	}
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	return vm
}

type testGameDirFS struct {
	gameDir string
}

func (fs testGameDirFS) OpenFile(filename string) (io.ReadSeekCloser, int64, error) {
	return nil, 0, nil
}

func (fs testGameDirFS) GameDir() string {
	return fs.gameDir
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
	newServerTestVM(s, 8)
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
	want.PutByte(byte(state.ModelIndex))
	want.PutByte(byte(state.Frame))
	want.PutByte(byte(state.Colormap))
	want.PutByte(byte(state.Skin))
	want.PutByte(byte(state.Effects))
	want.WriteCoord(state.Origin[0], flags)
	want.WriteAngle(state.Angles[0], flags)
	want.WriteCoord(state.Origin[1], flags)
	want.WriteAngle(state.Angles[1], flags)
	want.WriteCoord(state.Origin[2], flags)
	want.WriteAngle(state.Angles[2], flags)
	want.PutByte(state.Alpha)
	want.PutByte(state.Scale)
	want.PutByte(byte(state.Frame >> 8))
	want.PutByte(byte(state.ModelIndex >> 8))
	want.PutByte(200)

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
	newServerTestVM(s, 8)
	client := &Client{Edict: &Edict{Num: 1}}
	client.Edict.SetViewOfs(s, [3]float32{128, 0, 0})
	msg := NewMessageBuffer(128)

	s.buildClientDatagram(client, msg)

	if len(client.FatPVS) == 0 || client.FatPVS[0] != 0x01 {
		t.Fatalf("FatPVS = %v, want visibility from eye position leaf", client.FatPVS)
	}
}

func TestUpdateToReliableMessagesQueuesNonClientStatsAndUnderwaterOverride(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
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

func TestUpdateToReliableMessagesQueuesQCGlobalStats(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	vm := newTestQCVM()
	vm.Globals = make([]float32, 16)
	vm.StringTable = make(map[int32]string)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: 1, Name: vm.AllocString("total_secrets")},
		{Type: uint16(qc.EvFloat), Ofs: 2, Name: vm.AllocString("total_monsters")},
		{Type: uint16(qc.EvFloat), Ofs: 3, Name: vm.AllocString("found_secrets")},
		{Type: uint16(qc.EvFloat), Ofs: 4, Name: vm.AllocString("killed_monsters")},
	}
	vm.Globals[1] = 9
	vm.Globals[2] = 66
	vm.Globals[3] = 3
	vm.Globals[4] = 12
	s.QCVM = vm

	client := s.Static.Clients[0]
	client.Active = true
	client.Message.Clear()

	s.UpdateToReliableMessages()

	data := client.Message.Data[:client.Message.Len()]
	for _, tc := range []struct {
		stat byte
		want int32
	}{
		{stat: byte(inet.StatTotalSecrets), want: 9},
		{stat: byte(inet.StatTotalMonsters), want: 66},
		{stat: byte(inet.StatSecrets), want: 3},
		{stat: byte(inet.StatMonsters), want: 12},
	} {
		prefix := []byte{byte(inet.SVCUpdateStat), tc.stat}
		if !bytes.Contains(data, prefix) {
			t.Fatalf("reliable message missing stat update %d: %v", tc.stat, data)
		}
	}
	if got := client.Stats[inet.StatSecrets]; got != 3 {
		t.Fatalf("client StatSecrets = %d, want 3", got)
	}
	if got := client.Stats[inet.StatMonsters]; got != 12 {
		t.Fatalf("client StatMonsters = %d, want 12", got)
	}
}

func TestBuildClientDatagramOmitsReliableStatUpdates(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
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
			{},
			{Num: 1},
			{Num: 2},
		},
	}
	newServerTestVM(s, 8)
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
	newServerTestVM(s, 8)
	client := &Client{Edict: &Edict{}}
	base := NewMessageBuffer(MaxDatagram)
	base.MaxSize = DatagramMTU
	s.buildClientDatagram(client, base)
	baseLen := base.Len()
	if baseLen == 0 {
		t.Fatal("expected base datagram payload")
	}

	s.Datagram = NewMessageBuffer(MaxDatagram)
	for i := 0; i < DatagramMTU-baseLen; i++ {
		s.Datagram.PutByte(0x42)
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
	newServerTestVM(s, 8)
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
	newServerTestVM(s, 8)
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
	newServerTestVM(s, 8)
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
			newServerTestVM(s, 8)
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
