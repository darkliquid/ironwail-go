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
