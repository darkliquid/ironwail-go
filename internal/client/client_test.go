package client

import (
	"bytes"
	"encoding/binary"
	"math"
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
	if c.State != StateActive {
		t.Fatalf("state = %d, want active", c.State)
	}
}

func TestParseClientDataEntityAndTempEntity(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)

	msg.WriteByte(byte(inet.SVCSpawnBaseline))
	writeShort(msg, 1)
	msg.WriteByte(5)
	msg.WriteByte(1)
	msg.WriteByte(2)
	msg.WriteByte(3)
	writeCoord(msg, 1)
	writeCoord(msg, 2)
	writeCoord(msg, 3)
	writeAngle(msg, 0)
	writeAngle(msg, 90)
	writeAngle(msg, 180)

	msg.WriteByte(byte(inet.SVCClientData))
	bits := inet.SU_VIEWHEIGHT | inet.SU_IDEALPITCH | inet.SU_PUNCH1 | inet.SU_VELOCITY1 |
		inet.SU_ITEMS | inet.SU_ONGROUND | inet.SU_WEAPONFRAME | inet.SU_ARMOR | inet.SU_WEAPON
	writeShort(msg, int(bits))
	msg.WriteByte(byte(int8(30)))
	msg.WriteByte(byte(int8(5)))
	msg.WriteByte(byte(int8(7)))
	msg.WriteByte(byte(int8(4)))
	writeLong(msg, 0x1234)
	msg.WriteByte(9)
	msg.WriteByte(10)
	msg.WriteByte(11)
	writeShort(msg, 100)
	msg.WriteByte(12)
	msg.WriteByte(13)
	msg.WriteByte(14)
	msg.WriteByte(15)
	msg.WriteByte(16)
	msg.WriteByte(2)

	msg.WriteByte(0x80 | byte(inet.U_FRAME|inet.U_ANGLE2|inet.U_ORIGIN1|inet.U_ORIGIN2|inet.U_ORIGIN3))
	msg.WriteByte(1)
	msg.WriteByte(4)
	writeCoord(msg, 10)
	writeCoord(msg, 20)
	writeCoord(msg, 30)
	writeAngle(msg, 45)

	msg.WriteByte(byte(inet.SVCTempEntity))
	msg.WriteByte(byte(inet.TE_EXPLOSION))
	writeCoord(msg, 100)
	writeCoord(msg, 200)
	writeCoord(msg, 300)

	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.EntityBaselines[1].ModelIndex; got != 5 {
		t.Fatalf("baseline model = %d, want 5", got)
	}
	if got := c.EntityBaselines[1].Origin; got != [3]float32{1, 2, 3} {
		t.Fatalf("baseline origin = %v, want [1 2 3]", got)
	}
	if got := c.Stats[statHealth]; got != 100 {
		t.Fatalf("health stat = %d, want 100", got)
	}
	if got := c.Stats[statArmor]; got != 10 {
		t.Fatalf("armor stat = %d, want 10", got)
	}
	if got := c.Stats[statWeapon]; got != 11 {
		t.Fatalf("weapon stat = %d, want 11", got)
	}
	if got := c.Items; got != 0x1234 {
		t.Fatalf("items = 0x%x, want 0x1234", got)
	}
	if !c.OnGround || c.InWater {
		t.Fatalf("onground/inwater = %v/%v, want true/false", c.OnGround, c.InWater)
	}
	if got := c.Velocity[0]; got != 64 {
		t.Fatalf("velocity[0] = %v, want 64", got)
	}

	ent := c.Entities[1]
	if got := ent.Frame; got != 4 {
		t.Fatalf("entity frame = %d, want 4", got)
	}
	if got := ent.Origin; got != [3]float32{10, 20, 30} {
		t.Fatalf("entity origin = %v, want [10 20 30]", got)
	}
	if got := ent.Angles[1]; got < 44.5 || got > 45.5 {
		t.Fatalf("entity yaw = %f, want ~45", got)
	}

	if len(c.TempEntities) != 1 {
		t.Fatalf("temp entities len = %d, want 1", len(c.TempEntities))
	}
	if got := c.TempEntities[0].Type; got != inet.TE_EXPLOSION {
		t.Fatalf("temp entity type = %d, want TE_EXPLOSION", got)
	}
	if got := c.TempEntities[0].Origin; got != [3]float32{100, 200, 300} {
		t.Fatalf("temp entity origin = %v, want [100 200 300]", got)
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

func writeShort(buf *bytes.Buffer, v int) {
	_ = binary.Write(buf, binary.LittleEndian, int16(v))
}

func writeLong(buf *bytes.Buffer, v int32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeCoord(buf *bytes.Buffer, v float32) {
	_ = binary.Write(buf, binary.LittleEndian, math.Float32bits(v))
}

func writeAngle(buf *bytes.Buffer, deg float32) {
	buf.WriteByte(byte(deg * 256.0 / 360.0))
}
