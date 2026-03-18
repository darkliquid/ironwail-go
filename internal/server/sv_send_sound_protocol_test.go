package server

import (
	"encoding/binary"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

func TestStartSoundNetQuakeDropsLargeSoundIndex(t *testing.T) {
	s := NewServer()
	s.Protocol = ProtocolNetQuake
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	const soundNum = 300
	s.SoundPrecache = make([]string, soundNum+1)
	s.SoundPrecache[soundNum] = "misc/large.wav"

	ent := &Edict{Vars: &EntVars{}}
	s.Edicts[1] = ent

	s.StartSound(ent, 1, "misc/large.wav", DefaultSoundVolume, DefaultSoundAttenuation)

	if got := s.Datagram.Len(); got != 0 {
		t.Fatalf("datagram len = %d, want 0", got)
	}
}

func TestStartSoundNetQuakeDropsLargeEntity(t *testing.T) {
	s := NewServer()
	s.Protocol = ProtocolNetQuake
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	const entNum = 8192
	s.Edicts = make([]*Edict, entNum+1)
	ent := &Edict{Vars: &EntVars{}}
	s.Edicts[entNum] = ent
	s.SoundPrecache[1] = "misc/small.wav"

	s.StartSound(ent, 1, "misc/small.wav", DefaultSoundVolume, DefaultSoundAttenuation)

	if got := s.Datagram.Len(); got != 0 {
		t.Fatalf("datagram len = %d, want 0", got)
	}
}

func TestStartSoundNetQuakeUsesLegacyEncoding(t *testing.T) {
	s := NewServer()
	s.Protocol = ProtocolNetQuake
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	const (
		entNum   = 100
		channel  = 3
		soundNum = 42
	)
	ent := &Edict{
		Vars: &EntVars{
			Origin: [3]float32{1, 2, 3},
		},
	}
	s.Edicts = make([]*Edict, entNum+1)
	s.Edicts[entNum] = ent
	s.SoundPrecache[soundNum] = "misc/small.wav"

	s.StartSound(ent, channel, "misc/small.wav", DefaultSoundVolume, DefaultSoundAttenuation)

	data := s.Datagram.Data[:s.Datagram.Len()]
	if got := len(data); got != 11 {
		t.Fatalf("datagram len = %d, want 11", got)
	}
	if got := data[0]; got != byte(inet.SVCSound) {
		t.Fatalf("svc = %d, want %d", got, inet.SVCSound)
	}
	if got := data[1]; got != 0 {
		t.Fatalf("field mask = 0x%02x, want 0x00", got)
	}
	packed := int(binary.LittleEndian.Uint16(data[2:4]))
	if want := entNum<<3 | channel; packed != want {
		t.Fatalf("packed entity/channel = %d, want %d", packed, want)
	}
	if got := int(data[4]); got != soundNum {
		t.Fatalf("sound = %d, want %d", got, soundNum)
	}
}

func TestLocalSoundNetQuakeDropsLargeSoundIndex(t *testing.T) {
	s := NewServer()
	s.Protocol = ProtocolNetQuake
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	client := &Client{Message: NewMessageBuffer(MaxDatagram)}

	const soundNum = 300
	s.SoundPrecache = make([]string, soundNum+1)
	s.SoundPrecache[soundNum] = "misc/large.wav"

	s.LocalSound(client, "misc/large.wav")

	if got := client.Message.Len(); got != 0 {
		t.Fatalf("message len = %d, want 0", got)
	}
}

func TestLocalSoundFitzQuakeUsesLargeSoundEncoding(t *testing.T) {
	s := NewServer()
	s.Protocol = ProtocolFitzQuake
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	client := &Client{Message: NewMessageBuffer(MaxDatagram)}

	const soundNum = 300
	s.SoundPrecache = make([]string, soundNum+1)
	s.SoundPrecache[soundNum] = "misc/large.wav"

	s.LocalSound(client, "misc/large.wav")

	data := client.Message.Data[:client.Message.Len()]
	if got := len(data); got != 4 {
		t.Fatalf("message len = %d, want 4", got)
	}
	if got := data[0]; got != byte(inet.SVCLocalSound) {
		t.Fatalf("svc = %d, want %d", got, inet.SVCLocalSound)
	}
	if got := data[1]; got != byte(inet.SND_LARGESOUND) {
		t.Fatalf("field mask = 0x%02x, want 0x%02x", got, byte(inet.SND_LARGESOUND))
	}
	if got := int(binary.LittleEndian.Uint16(data[2:4])); got != soundNum {
		t.Fatalf("sound = %d, want %d", got, soundNum)
	}
}
