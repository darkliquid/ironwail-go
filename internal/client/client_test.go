package client

import (
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

var serverSignOnMsg1 = []byte{
	byte(inet.SVCServerInfo),
	0x9a, 0x02, 0x00, 0x00,
	0x04,
	0x00,
	'U', 'n', 'i', 't', ' ', 'T', 'e', 's', 't', ' ', 'M', 'a', 'p', 0,
	'm', 'a', 'p', 's', '/', 's', 't', 'a', 'r', 't', '.', 'b', 's', 'p', 0,
	'p', 'r', 'o', 'g', 's', '/', 'p', 'l', 'a', 'y', 'e', 'r', '.', 'm', 'd', 'l', 0,
	0,
	'm', 'i', 's', 'c', '/', 'n', 'u', 'l', 'l', '.', 'w', 'a', 'v', 0,
	0,
	byte(inet.SVCCDTrack), 0x02, 0x02,
	byte(inet.SVCSetView), 0x01, 0x00,
	byte(inet.SVCSignOnNum), 0x01,
	0xff,
}

var serverSignOnMsg2 = []byte{byte(inet.SVCSignOnNum), 0x02, 0xff}
var serverSignOnMsg3 = []byte{byte(inet.SVCSignOnNum), 0x03, 0xff}
var serverSignOnMsg4 = []byte{byte(inet.SVCSignOnNum), 0x04, 0xff}

func TestParseServerSignOnSequence(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	for _, msg := range [][]byte{serverSignOnMsg1, serverSignOnMsg2, serverSignOnMsg3, serverSignOnMsg4} {
		if err := p.ParseServerMessage(msg); err != nil {
			t.Fatalf("ParseServerMessage() error = %v", err)
		}
	}

	if c.Protocol != inet.PROTOCOL_FITZQUAKE {
		t.Fatalf("protocol = %d, want %d", c.Protocol, inet.PROTOCOL_FITZQUAKE)
	}
	if c.MaxClients != 4 {
		t.Fatalf("maxclients = %d, want 4", c.MaxClients)
	}
	if c.LevelName != "Unit Test Map" {
		t.Fatalf("levelname = %q", c.LevelName)
	}
	if c.MapName != "start" {
		t.Fatalf("mapname = %q, want start", c.MapName)
	}
	if got := len(c.ModelPrecache); got != 2 {
		t.Fatalf("model precache count = %d, want 2", got)
	}
	if got := len(c.SoundPrecache); got != 1 {
		t.Fatalf("sound precache count = %d, want 1", got)
	}
	if c.ViewEntity != 1 {
		t.Fatalf("viewentity = %d, want 1", c.ViewEntity)
	}
	if c.CDTrack != 2 || c.LoopTrack != 2 {
		t.Fatalf("cd/loop track = %d/%d, want 2/2", c.CDTrack, c.LoopTrack)
	}
	if c.Signon != 4 {
		t.Fatalf("signon = %d, want 4", c.Signon)
	}
	if c.State != StateConnected {
		t.Fatalf("state = %d, want connected", c.State)
	}
}

func TestLerpPointClampsAndInterpolates(t *testing.T) {
	c := NewClient()
	c.MTime[1] = 1.0
	c.MTime[0] = 1.1
	c.Time = 1.05

	frac := c.LerpPoint()
	if frac < 0.49 || frac > 0.51 {
		t.Fatalf("lerp frac = %f, want ~0.5", frac)
	}

	c.Time = 2.0
	frac = c.LerpPoint()
	if frac != 1 {
		t.Fatalf("clamped lerp frac = %f, want 1", frac)
	}
}
