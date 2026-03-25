package server

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestSendServerInfoFitzQuakeOmitsProtocolFlags(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Protocol = ProtocolFitzQuake

	client := &Client{
		Edict:   s.EdictNum(0),
		Message: NewMessageBuffer(MaxDatagram),
	}
	s.SendServerInfo(client)

	data := client.Message.Data[:client.Message.Len()]
	idx := bytes.IndexByte(data, byte(inet.SVCServerInfo))
	if idx < 0 {
		t.Fatalf("SVCServerInfo not found in message")
	}
	if len(data) < idx+6 {
		t.Fatalf("short message after SVCServerInfo: len=%d idx=%d", len(data), idx)
	}

	protocol := int32(binary.LittleEndian.Uint32(data[idx+1 : idx+5]))
	if protocol != ProtocolFitzQuake {
		t.Fatalf("protocol = %d, want %d", protocol, ProtocolFitzQuake)
	}
	if got := data[idx+5]; got != byte(s.Static.MaxClients) {
		t.Fatalf("byte after protocol = %d, want maxclients %d (no protocolflags for FitzQuake)", got, s.Static.MaxClients)
	}
}

func TestSendServerInfoRMQIncludesProtocolFlags(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Protocol = ProtocolRMQ

	client := &Client{
		Edict:   s.EdictNum(0),
		Message: NewMessageBuffer(MaxDatagram),
	}
	s.SendServerInfo(client)

	data := client.Message.Data[:client.Message.Len()]
	idx := bytes.IndexByte(data, byte(inet.SVCServerInfo))
	if idx < 0 {
		t.Fatalf("SVCServerInfo not found in message")
	}
	if len(data) < idx+10 {
		t.Fatalf("short message after SVCServerInfo: len=%d idx=%d", len(data), idx)
	}

	protocol := int32(binary.LittleEndian.Uint32(data[idx+1 : idx+5]))
	if protocol != ProtocolRMQ {
		t.Fatalf("protocol = %d, want %d", protocol, ProtocolRMQ)
	}

	flags := int32(binary.LittleEndian.Uint32(data[idx+5 : idx+9]))
	wantFlags := int32(ProtocolFlagInt32Coord | ProtocolFlagShortAngle)
	if flags != wantFlags {
		t.Fatalf("protocol flags = %d, want %d", flags, wantFlags)
	}
	if got := data[idx+9]; got != byte(s.Static.MaxClients) {
		t.Fatalf("byte after protocolflags = %d, want maxclients %d", got, s.Static.MaxClients)
	}
}

func TestSendServerInfoNetQuakeCapsPrecaches(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Protocol = ProtocolNetQuake
	s.ModelPrecache = make([]string, 258)
	s.SoundPrecache = make([]string, 258)
	for i := 1; i < len(s.ModelPrecache); i++ {
		s.ModelPrecache[i] = "model"
		s.SoundPrecache[i] = "sound"
	}
	s.ModelPrecache[255] = "mdl255"
	s.ModelPrecache[256] = "mdl256"
	s.SoundPrecache[255] = "snd255"
	s.SoundPrecache[256] = "snd256"

	client := &Client{
		Edict:   s.EdictNum(0),
		Message: NewMessageBuffer(MaxDatagram * 4),
	}
	s.SendServerInfo(client)

	payload := string(client.Message.Data[:client.Message.Len()])
	if !strings.Contains(payload, "mdl255\x00") || !strings.Contains(payload, "snd255\x00") {
		t.Fatalf("serverinfo missing capped NetQuake precache entries: %q", payload)
	}
	if strings.Contains(payload, "mdl256\x00") || strings.Contains(payload, "snd256\x00") {
		t.Fatalf("serverinfo included overflow NetQuake precache entries: %q", payload)
	}
}

func TestBuildSignonBuffers_WritesSpawnBaselines(t *testing.T) {
	s := &Server{
		Protocol: ProtocolFitzQuake,
		Static: &ServerStatic{
			MaxClients: 1,
		},
		NumEdicts: 2,
		Edicts: []*Edict{
			{Free: true, Vars: &EntVars{}}, // skipped
			{
				Vars: &EntVars{
					ModelIndex: 2,
					Frame:      3,
				},
			},
		},
		ModelPrecache: []string{"", "progs/player.mdl"},
	}

	if err := s.buildSignonBuffers(); err != nil {
		t.Fatalf("buildSignonBuffers: %v", err)
	}
	if len(s.SignonBuffers) == 0 {
		t.Fatal("expected signon buffers")
	}

	data := s.SignonBuffers[0].Data[:s.SignonBuffers[0].Len()]
	if len(data) == 0 {
		t.Fatal("expected signon data")
	}
	if data[0] != byte(inet.SVCSpawnBaseline) {
		t.Fatalf("first signon command = %d, want SVCSpawnBaseline(%d)", data[0], inet.SVCSpawnBaseline)
	}

	// Ensure the baseline command is for entity #1.
	if len(data) < 4 {
		t.Fatalf("short baseline message: %v", data)
	}
	entNum := binary.LittleEndian.Uint16(data[1:3])
	if entNum != 1 {
		t.Fatalf("baseline entity = %d, want 1", entNum)
	}
}

func TestCreateBaselineRMQUsesQCScaleAndWritesSpawnBaseline2(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Protocol = ProtocolRMQ
	s.QCVM = newTestQCVM()
	s.QCFieldScale = 0
	s.ModelPrecache = []string{"", "progs/ogre.mdl", "progs/player.mdl"}

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	ent.Vars.ModelIndex = 1
	s.QCVM.SetEFloat(entNum, s.QCFieldScale, 2.0)

	s.CreateBaseline()

	if got := ent.Baseline.Scale; got != 32 {
		t.Fatalf("baseline scale = %d, want 32", got)
	}

	if err := s.AddSignonBuffer(); err != nil {
		t.Fatalf("AddSignonBuffer() error = %v", err)
	}
	if err := s.writeSpawnBaselineToSignon(entNum, ent.Baseline); err != nil {
		t.Fatalf("writeSpawnBaselineToSignon() error = %v", err)
	}

	data := s.Signon.Data[:s.Signon.Len()]
	if len(data) == 0 {
		t.Fatal("expected signon data")
	}
	if got := data[0]; got != byte(inet.SVCSpawnBaseline2) {
		t.Fatalf("signon command = %d, want %d", got, inet.SVCSpawnBaseline2)
	}
	if got := data[len(data)-1]; got != 32 {
		t.Fatalf("encoded baseline scale byte = %d, want 32", got)
	}
}

func TestWriteSpawnStaticToSignon_MatchesDirectSpawnStaticEncoding(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake}
	ent := EntityState{
		ModelIndex: 5,
		Frame:      7,
		Colormap:   2,
		Skin:       3,
		Origin:     [3]float32{10, 20, 30},
		Angles:     [3]float32{45, 90, 180},
	}

	direct := NewMessageBuffer(64)
	s.writeSpawnStaticMessage(direct, ent)
	if err := s.writeSpawnStaticToSignon(ent); err != nil {
		t.Fatalf("writeSpawnStaticToSignon: %v", err)
	}

	got := s.Signon.Data[:s.Signon.Len()]
	want := direct.Data[:direct.Len()]
	if !bytes.Equal(got, want) {
		t.Fatalf("signon static encoding mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestWriteSpawnStaticToSignon_MatchesDirectSpawnStaticEncodingExtended(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake}
	ent := EntityState{
		ModelIndex: 300,
		Frame:      400,
		Colormap:   1,
		Skin:       4,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{10, 20, 30},
		Alpha:      200,
		Scale:      24,
	}

	direct := NewMessageBuffer(64)
	s.writeSpawnStaticMessage(direct, ent)
	if err := s.writeSpawnStaticToSignon(ent); err != nil {
		t.Fatalf("writeSpawnStaticToSignon: %v", err)
	}

	got := s.Signon.Data[:s.Signon.Len()]
	want := direct.Data[:direct.Len()]
	if !bytes.Equal(got, want) {
		t.Fatalf("extended signon static encoding mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestUpdateToReliableMessages_BroadcastsChangedPlayerFragsToAllActiveClients(t *testing.T) {
	s := &Server{
		Static: &ServerStatic{
			Clients: make([]*Client, 4),
		},
	}

	s.Static.Clients[0] = &Client{
		Active:   true,
		Edict:    &Edict{Vars: &EntVars{Frags: 5}},
		OldFrags: 0, // changed
		Message:  NewMessageBuffer(MaxDatagram),
	}
	s.Static.Clients[1] = &Client{
		Active:   true,
		Edict:    &Edict{Vars: &EntVars{Frags: 10}},
		OldFrags: 10, // unchanged
		Message:  NewMessageBuffer(MaxDatagram),
	}
	s.Static.Clients[2] = &Client{
		Active:   true,
		Edict:    &Edict{Vars: &EntVars{Frags: -2}},
		OldFrags: -2, // unchanged
		Message:  NewMessageBuffer(MaxDatagram),
	}
	s.Static.Clients[3] = &Client{
		Active:   false,
		Edict:    &Edict{Vars: &EntVars{Frags: 99}},
		OldFrags: 0,
		Message:  NewMessageBuffer(MaxDatagram),
	}

	s.UpdateToReliableMessages()

	want := []byte{byte(inet.SVCUpdateFrags), 0, 5, 0}
	for i := 0; i < 3; i++ {
		got := s.Static.Clients[i].Message.Data[:s.Static.Clients[i].Message.Len()]
		if !bytes.Equal(got, want) {
			t.Fatalf("client %d message = %v, want %v", i, got, want)
		}
	}

	if got := s.Static.Clients[3].Message.Len(); got != 0 {
		t.Fatalf("inactive client received frag update, message len = %d, want 0", got)
	}

	if got := s.Static.Clients[0].OldFrags; got != 5 {
		t.Fatalf("changed client OldFrags = %d, want 5", got)
	}
	if got := s.Static.Clients[1].OldFrags; got != 10 {
		t.Fatalf("unchanged client 1 OldFrags = %d, want 10", got)
	}
	if got := s.Static.Clients[2].OldFrags; got != -2 {
		t.Fatalf("unchanged client 2 OldFrags = %d, want -2", got)
	}
}

func TestUpdateToReliableMessagesFansOutSharedReliableDatagram(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	for _, client := range s.Static.Clients[:2] {
		client.Active = true
		client.Message.Clear()
	}
	s.ReliableDatagram.WriteByte(byte(inet.SVCPrint))
	s.ReliableDatagram.WriteString("shared\n")

	s.UpdateToReliableMessages()

	for i, client := range s.Static.Clients[:2] {
		if got := string(client.Message.Data[:client.Message.Len()]); !strings.Contains(got, "shared\n") {
			t.Fatalf("client %d missing shared reliable payload: %q", i, got)
		}
	}
	if got := s.ReliableDatagram.Len(); got != 0 {
		t.Fatalf("ReliableDatagram len = %d, want 0 after fanout", got)
	}
}

func TestSaveSpawnParmsCopiesServerFlagsFromQC(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	vm := qc.NewVM()
	vm.Globals = make([]float32, 16)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: 1, Name: vm.AllocString("serverflags")},
	}
	vm.SetGInt(1, 13)
	s.QCVM = vm

	s.SaveSpawnParms()

	if got := s.Static.ServerFlags; got != 13 {
		t.Fatalf("Static.ServerFlags = %d, want 13", got)
	}
}

func TestDropClientCrashClosesAndClearsRemoteConnection(t *testing.T) {
	s := NewServer()
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
	defer inet.Close(clientSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.NetConnection = serverSock

	s.DropClient(client, true)

	if client.NetConnection != nil {
		t.Fatal("crash drop should clear client net connection")
	}
	if got := inet.SendMessage(clientSock, []byte{0x01}); got != -1 {
		t.Fatalf("send from peer after server close = %d, want -1", got)
	}
}

func TestSendClientMessagesCrashDropOnReliableSendFailureClosesConnection(t *testing.T) {
	s := NewServer()
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
	defer inet.Close(clientSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.SendSignon = SignonPrespawn
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString("force send path")
	inet.Close(clientSock)

	s.SendClientMessages()

	if client.Active {
		t.Fatal("client should be dropped on reliable send failure")
	}
	if client.NetConnection != nil {
		t.Fatal("crash drop should clear net connection after reliable send failure")
	}
}

func TestSendClientMessagesCrashDropOnOverflowClosesConnection(t *testing.T) {
	s := NewServer()
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
	defer inet.Close(clientSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.SendSignon = SignonFlush
	client.Message.Overflowed = true

	s.SendClientMessages()

	if client.Active {
		t.Fatal("client should be dropped on message overflow")
	}
	if client.NetConnection != nil {
		t.Fatal("crash drop should clear net connection after overflow drop")
	}
	if client.Message.Overflowed {
		t.Fatal("overflowed message should be cleared after drop")
	}
}

func TestQueuePendingSignonTreatsLOCALSocketAsLocalClient(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	bufA := NewMessageBuffer(MaxDatagram)
	bufA.WriteByte(0x11)
	bufB := NewMessageBuffer(MaxDatagram)
	bufB.WriteByte(0x22)
	s.SignonBuffers = []*MessageBuffer{bufA, bufB}

	client := s.Static.Clients[0]
	client.SendSignon = SignonPrespawn
	client.SignonIdx = 0
	client.Loopback = false
	client.NetConnection = inet.NewSocket("LOCAL")

	s.queuePendingSignon(client)

	if got := client.SignonIdx; got != 2 {
		t.Fatalf("SignonIdx = %d, want 2 for LOCAL socket", got)
	}
	if client.SendSignon != SignonSignonBufs {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonSignonBufs)
	}

	got := client.Message.Data[:client.Message.Len()]
	want := []byte{0x11, 0x22, byte(inet.SVCSignOnNum), 2}
	if !bytes.Equal(got, want) {
		t.Fatalf("message = %v, want %v", got, want)
	}
}
