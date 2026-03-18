package server

import (
	"bytes"
	"encoding/binary"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
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
	if flags != int32(s.ProtocolFlags()) {
		t.Fatalf("protocol flags = %d, want %d", flags, s.ProtocolFlags())
	}
	if got := data[idx+9]; got != byte(s.Static.MaxClients) {
		t.Fatalf("byte after protocolflags = %d, want maxclients %d", got, s.Static.MaxClients)
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
