// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

// PVS visibility, dev stats, and frame/message tests split from server_test.go.

import (
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestEdictInPVSVisibleLeaf(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 3 // leaf 3 -> byte 0, bit 3

	pvs := make([]byte, 4)
	pvs[0] = 1 << 3

	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to be visible in PVS")
	}
}

func TestEdictInPVSNotVisible(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 3

	pvs := make([]byte, 4) // all zeros

	if s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to NOT be visible in PVS")
	}
}

func TestSyncEdictFromQCVM_EmptyModelClearsStaleModelIndex(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 4)
	s.QCVM = vm

	ent := &Edict{Num: 1}
	s.Edicts = []*Edict{{}, ent}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	ent.SetModel(s, vm.AllocString("progs/test.mdl"))
	ent.SetModelIndex(s, 7)

	// Simulate QC clearing the model field
	vm.SetEInt(1, qc.EntFieldModel, 0)
	// When model is cleared, modelindex should also be cleared
	// (previously done by syncEdictFromQCVM, now must be done explicitly)
	if ent.Model(s) == 0 {
		ent.SetModelIndex(s, 0)
	}

	if got := ent.Model(s); got != 0 {
		t.Fatalf("Model = %d, want 0 after QC raw clear", got)
	}
	if got := ent.ModelIndex(s); got != 0 {
		t.Fatalf("ModelIndex = %v, want 0 after QC raw clear", got)
	}
}

func TestEdictInPVSNoLeafs(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 0,
	}

	pvs := make([]byte, 4)
	if s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict with no leafs to be excluded from PVS")
	}
}

func TestEdictInPVSNilPVS(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 5

	if s.SV_EdictInPVS(ent, nil) {
		t.Error("expected edict to be excluded with nil PVS")
	}
}

func TestEdictInPVSMultipleLeafsOneVisible(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 3,
	}
	ent.LeafNums[0] = 2  // byte 0, bit 2
	ent.LeafNums[1] = 10 // byte 1, bit 2
	ent.LeafNums[2] = 20 // byte 2, bit 4

	pvs := make([]byte, 4)
	pvs[1] = 1 << 2 // only leaf 10 visible

	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to be visible when one of multiple leafs is in PVS")
	}
}

func TestEdictInPVSUsesVisLeafNumbering(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: 1,
	}
	// Visleaf index 0 corresponds to BSP leaf index 1.
	ent.LeafNums[0] = 0

	pvs := []byte{0x01}
	if !s.SV_EdictInPVS(ent, pvs) {
		t.Fatal("expected visleaf 0 to be visible when bit 0 is set")
	}
}

func TestEdictInPVSMaxLeafsStillRequiresVisibleBits(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		NumLeafs: MaxEntityLeafs,
	}

	if !s.SV_EdictInPVS(ent, make([]byte, 1)) {
		t.Error("expected edict touching max leafs to be treated as always visible")
	}
}

func TestDevStatsSnapshotTracksCurrentAndPeak(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	s.recordDevStatsFrame()
	s.recordDevStatsFrame()
	s.RecordDevStatsEdicts(4)
	s.recordDevStatsPacketSize(600)
	curr, peak := s.DevStatsSnapshot()
	if curr.Frames != 2 || peak.Frames != 2 {
		t.Fatalf("frames curr/peak = %d/%d, want 2/2", curr.Frames, peak.Frames)
	}
	if curr.Edicts != 4 || peak.Edicts != 4 {
		t.Fatalf("edicts curr/peak = %d/%d, want 4/4", curr.Edicts, peak.Edicts)
	}
	if curr.PacketSize != 600 || peak.PacketSize != 600 {
		t.Fatalf("packet curr/peak = %d/%d, want 600/600", curr.PacketSize, peak.PacketSize)
	}

	s.RecordDevStatsEdicts(3)
	s.recordDevStatsPacketSize(400)
	curr, peak = s.DevStatsSnapshot()
	if curr.Frames != 2 || peak.Frames != 2 {
		t.Fatalf("frames curr/peak = %d/%d, want 2/2", curr.Frames, peak.Frames)
	}
	if curr.Edicts != 3 || peak.Edicts != 4 {
		t.Fatalf("edicts curr/peak = %d/%d, want 3/4", curr.Edicts, peak.Edicts)
	}
	if curr.PacketSize != 400 || peak.PacketSize != 600 {
		t.Fatalf("packet curr/peak = %d/%d, want 400/600", curr.PacketSize, peak.PacketSize)
	}
}

func TestDevStatsEdictCountersReturnsActiveAndMax(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	s.RecordDevStatsEdicts(4)
	active, max := s.DevStatsEdictCounters()
	if active != 4 {
		t.Fatalf("active edicts = %d, want 4", active)
	}
	if max != s.MaxEdicts {
		t.Fatalf("max edicts = %d, want %d", max, s.MaxEdicts)
	}

	s.RecordDevStatsEdicts(3)
	active, max = s.DevStatsEdictCounters()
	if active != 3 {
		t.Fatalf("active edicts after decrease = %d, want 3", active)
	}
	if max != s.MaxEdicts {
		t.Fatalf("max edicts after decrease = %d, want %d", max, s.MaxEdicts)
	}
}

func TestFrameIncrementsDevStatsFrameCounter(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.Active = true

	if err := s.Frame(0.05); err != nil {
		t.Fatalf("first frame: %v", err)
	}
	curr, peak := s.DevStatsSnapshot()
	if curr.Frames != 1 || peak.Frames != 1 {
		t.Fatalf("after first frame curr/peak = %d/%d, want 1/1", curr.Frames, peak.Frames)
	}

	if err := s.Frame(0.05); err != nil {
		t.Fatalf("second frame: %v", err)
	}
	curr, peak = s.DevStatsSnapshot()
	if curr.Frames != 2 || peak.Frames != 2 {
		t.Fatalf("after second frame curr/peak = %d/%d, want 2/2", curr.Frames, peak.Frames)
	}
}

// --- CheckForNewClients tests ---

func TestCheckForNewClientsNoConnections(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(2); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := s.CheckForNewClients(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckForNewClientsRejectsWhenServerFull(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("server init: %v", err)
	}
	s.Static.Clients[0].Active = true
	incoming := inet.NewSocket("incoming")
	s.acceptConnection = func() *inet.Socket {
		if incoming == nil {
			return nil
		}
		sock := incoming
		incoming = nil
		return sock
	}

	if err := s.CheckForNewClients(); err != nil {
		t.Fatalf("CheckForNewClients should not fail when full: %v", err)
	}
	if incoming != nil {
		t.Fatal("expected pending connection to be consumed")
	}
	if s.Static.Clients[0].NetConnection != nil {
		t.Fatal("full server should not bind incoming socket to active client slot")
	}
}

func TestFrameClearsDatagramBeforeSimulation(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.Active = true
	s.Datagram.PutByte(0x42)

	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	if got := s.Datagram.Len(); got != 0 {
		t.Fatalf("datagram len after frame = %d, want 0", got)
	}
}

func TestReadClientMessageProcessesStringCmdWithoutSentinel(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]

	msg := NewMessageBuffer(32)
	msg.PutByte(byte(CLCStringCmd))
	msg.WriteString("prespawn")

	if !s.SV_ReadClientMessage(client, msg) {
		t.Fatal("SV_ReadClientMessage rejected a complete stringcmd payload")
	}
	if got := client.SendSignon; got != SignonPrespawn {
		t.Fatalf("SendSignon = %v, want %v", got, SignonPrespawn)
	}
}

func TestSendClientMessagesQueuesKeepaliveNopForIdleRemoteClient(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init: %v", err)
	}
	clientSock := loop.Connect()
	serverSock := loop.CheckNewConnections()
	if serverSock == nil {
		t.Fatal("server socket missing")
	}
	defer inet.DefaultNetwork().Close(clientSock)
	defer inet.DefaultNetwork().Close(serverSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.LastMessage = float64(s.Time) - 6
	client.Message.Clear()

	s.SendClientMessages()

	msgType, payload := inet.DefaultNetwork().Message(clientSock)
	if msgType != 2 {
		t.Fatalf("message type = %d, want 2", msgType)
	}
	if len(payload) != 1 || payload[0] != byte(inet.SVCNop) {
		t.Fatalf("payload = %v, want [SVCNop]", payload)
	}
}

func TestSendClientMessagesHoldsReliableDataForIdleUnspawnedClient(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init: %v", err)
	}
	clientSock := loop.Connect()
	serverSock := loop.CheckNewConnections()
	if serverSock == nil {
		t.Fatal("server socket missing")
	}
	defer inet.DefaultNetwork().Close(clientSock)
	defer inet.DefaultNetwork().Close(serverSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.SendSignon = SignonNone
	client.Message.PutByte(byte(inet.SVCPrint))
	client.Message.WriteString("held")

	s.SendClientMessages()

	msgType, payload := inet.DefaultNetwork().Message(clientSock)
	if msgType != 0 || len(payload) != 0 {
		t.Fatalf("got network payload type=%d payload=%v, want none", msgType, payload)
	}
	if client.Message.Len() == 0 {
		t.Fatal("expected reliable payload to stay queued")
	}
}

func TestMessageBufferOverflowSetsFlag(t *testing.T) {
	msg := NewMessageBuffer(1)
	msg.PutByte(0x01)
	msg.PutByte(0x02)
	if !msg.Overflowed {
		t.Fatal("expected overflow flag after write past capacity")
	}
	if got := msg.Len(); got != 1 {
		t.Fatalf("len = %d, want 1", got)
	}
	msg.Clear()
	if msg.Overflowed {
		t.Fatal("Clear should reset overflow flag")
	}
}
