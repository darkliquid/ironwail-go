package client

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/server"
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

func TestParseLiveServerEntityDatagrams(t *testing.T) {
	s := server.NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Time = 1.5
	s.ModelPrecache = make([]string, server.MaxModels)
	s.ModelPrecache[1] = "progs/player.mdl"
	s.ModelPrecache[2] = "progs/ogre.mdl"

	clientSlot := s.Static.Clients[0]
	clientSlot.Active = true
	clientSlot.Spawned = true
	clientSlot.Edict = s.EdictNum(1)
	if clientSlot.Edict == nil {
		t.Fatal("missing client edict")
	}
	clientSlot.Edict.Vars.Health = 100
	clientSlot.Edict.Vars.Origin = [3]float32{1, 2, 3}

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc entity")
	}
	ent.Vars.ModelIndex = 2
	ent.Vars.Origin = [3]float32{10, 20, 30}
	ent.Vars.Angles = [3]float32{0, 45, 0}
	ent.Vars.Frame = 4
	ent.Vars.Skin = 2
	ent.Vars.Effects = 8

	c := NewClient()
	p := NewParser(c)

	data := s.GetClientDatagram(0)
	if len(data) == 0 {
		t.Fatal("GetClientDatagram returned no data")
	}
	if data[len(data)-1] != 0xff {
		t.Fatalf("datagram terminator = 0x%02x, want 0xff", data[len(data)-1])
	}
	if err := p.ParseServerMessage(data); err != nil {
		t.Fatalf("ParseServerMessage first datagram: %v", err)
	}

	got := c.Entities[s.NumForEdict(ent)]
	if got.ModelIndex != 2 {
		t.Fatalf("entity modelindex = %d, want 2", got.ModelIndex)
	}
	if got.Frame != 4 {
		t.Fatalf("entity frame = %d, want 4", got.Frame)
	}
	if got.Origin != [3]float32{10, 20, 30} {
		t.Fatalf("entity origin = %v, want [10 20 30]", got.Origin)
	}
	if got.Angles[1] < 44.5 || got.Angles[1] > 45.5 {
		t.Fatalf("entity yaw = %f, want ~45", got.Angles[1])
	}

	s.Time = 1.6
	ent.Vars.Origin[0] = 42
	data = s.GetClientDatagram(0)
	if err := p.ParseServerMessage(data); err != nil {
		t.Fatalf("ParseServerMessage second datagram: %v", err)
	}

	got = c.Entities[s.NumForEdict(ent)]
	if got.ModelIndex != 2 {
		t.Fatalf("entity modelindex after delta = %d, want 2", got.ModelIndex)
	}
	if got.Frame != 4 {
		t.Fatalf("entity frame after delta = %d, want 4", got.Frame)
	}
	if got.Origin != [3]float32{42, 20, 30} {
		t.Fatalf("entity origin after delta = %v, want [42 20 30]", got.Origin)
	}

	s.FreeEdict(ent)
	s.Time = 1.7
	data = s.GetClientDatagram(0)
	if err := p.ParseServerMessage(data); err != nil {
		t.Fatalf("ParseServerMessage third datagram: %v", err)
	}
	if _, ok := c.Entities[s.NumForEdict(ent)]; ok {
		t.Fatalf("entity %d still present after retire update", s.NumForEdict(ent))
	}
}

func TestParseStaticEntityAndSoundMessages(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)

	msg.WriteByte(byte(inet.SVCSpawnStatic))
	msg.WriteByte(5)
	msg.WriteByte(1)
	msg.WriteByte(2)
	msg.WriteByte(3)
	writeCoord(msg, 10)
	writeCoord(msg, 20)
	writeCoord(msg, 30)
	writeAngle(msg, 45)
	writeAngle(msg, 90)
	writeAngle(msg, 180)

	msg.WriteByte(byte(inet.SVCSpawnStatic2))
	msg.WriteByte(byte(inet.BLARGEMODEL | inet.BLARGEFRAME | inet.BALPHA | inet.BSCALE))
	writeShort(msg, 300)
	writeShort(msg, 400)
	msg.WriteByte(0)
	msg.WriteByte(7)
	writeCoord(msg, 1)
	writeCoord(msg, 2)
	writeCoord(msg, 3)
	writeAngle(msg, 0)
	writeAngle(msg, 10)
	writeAngle(msg, 20)
	msg.WriteByte(200)
	msg.WriteByte(24)

	msg.WriteByte(byte(inet.SVCSpawnStaticSound))
	writeCoord(msg, 4)
	writeCoord(msg, 5)
	writeCoord(msg, 6)
	msg.WriteByte(9)
	msg.WriteByte(255)
	msg.WriteByte(64)

	msg.WriteByte(byte(inet.SVCSpawnStaticSound2))
	writeCoord(msg, 7)
	writeCoord(msg, 8)
	writeCoord(msg, 9)
	writeShort(msg, 300)
	msg.WriteByte(128)
	msg.WriteByte(32)

	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := len(c.StaticEntities); got != 2 {
		t.Fatalf("static entities len = %d, want 2", got)
	}
	if got := c.StaticEntities[0].Origin; got != [3]float32{10, 20, 30} {
		t.Fatalf("static entity origin = %v, want [10 20 30]", got)
	}
	if got := c.StaticEntities[1].ModelIndex; got != 300 {
		t.Fatalf("static entity model = %d, want 300", got)
	}
	if got := c.StaticEntities[1].Frame; got != 400 {
		t.Fatalf("static entity frame = %d, want 400", got)
	}
	if got := c.StaticEntities[1].Alpha; got != 200 {
		t.Fatalf("static entity alpha = %d, want 200", got)
	}
	if got := c.StaticEntities[1].Scale; got != 24 {
		t.Fatalf("static entity scale = %d, want 24", got)
	}

	if got := len(c.StaticSounds); got != 2 {
		t.Fatalf("static sounds len = %d, want 2", got)
	}
	if got := c.StaticSounds[0].SoundIndex; got != 9 {
		t.Fatalf("static sound index = %d, want 9", got)
	}
	if got := c.StaticSounds[0].Attenuation; got != 1 {
		t.Fatalf("static sound attenuation = %v, want 1", got)
	}
	if got := c.StaticSounds[1].SoundIndex; got != 300 {
		t.Fatalf("static sound2 index = %d, want 300", got)
	}
	if got := c.StaticSounds[1].Attenuation; got != 0.5 {
		t.Fatalf("static sound2 attenuation = %v, want 0.5", got)
	}
}

func TestParseRuntimeServerMessages(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)

	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(3)
	writeLong(msg, 77)

	msg.WriteByte(byte(inet.SVCUpdateFrags))
	msg.WriteByte(2)
	writeShort(msg, 15)

	msg.WriteByte(byte(inet.SVCCenterPrint))
	msg.WriteString("centered")
	msg.WriteByte(0)

	msg.WriteByte(byte(inet.SVCSetPause))
	msg.WriteByte(1)

	msg.WriteByte(byte(inet.SVCDamage))
	msg.WriteByte(5)
	msg.WriteByte(7)
	writeCoord(msg, 1)
	writeCoord(msg, 2)
	writeCoord(msg, 3)

	msg.WriteByte(byte(inet.SVCSound))
	msg.WriteByte(byte(inet.SND_VOLUME | inet.SND_ATTENUATION))
	msg.WriteByte(200)
	msg.WriteByte(32)
	writeShort(msg, (1<<3)|2)
	msg.WriteByte(9)
	writeCoord(msg, 10)
	writeCoord(msg, 20)
	writeCoord(msg, 30)

	msg.WriteByte(byte(inet.SVCLocalSound))
	msg.WriteByte(0)
	msg.WriteByte(4)

	msg.WriteByte(byte(inet.SVCParticle))
	writeCoord(msg, 4)
	writeCoord(msg, 5)
	writeCoord(msg, 6)
	msg.WriteByte(byte(int8(16)))
	msg.WriteByte(240)
	msg.WriteByte(byte(int8(8)))
	msg.WriteByte(255)
	msg.WriteByte(99)

	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.Stats[3]; got != 77 {
		t.Fatalf("stat[3] = %d, want 77", got)
	}
	if got := c.Frags[2]; got != 15 {
		t.Fatalf("frags[2] = %d, want 15", got)
	}
	if c.CenterPrint != "centered" {
		t.Fatalf("centerprint = %q, want centered", c.CenterPrint)
	}
	if !c.Paused {
		t.Fatal("paused = false, want true")
	}
	if c.DamageSaved != 5 || c.DamageTaken != 7 {
		t.Fatalf("damage save/take = %d/%d, want 5/7", c.DamageSaved, c.DamageTaken)
	}
	if c.DamageOrigin != [3]float32{1, 2, 3} {
		t.Fatalf("damage origin = %v, want [1 2 3]", c.DamageOrigin)
	}

	if got := len(c.SoundEvents); got != 2 {
		t.Fatalf("sound events len = %d, want 2", got)
	}
	if got := c.SoundEvents[0]; got.Entity != 1 || got.Channel != 2 || got.SoundIndex != 9 || got.Volume != 200 || got.Attenuation != 0.5 || got.Origin != [3]float32{10, 20, 30} || got.Local {
		t.Fatalf("sound event[0] = %+v", got)
	}
	if got := c.SoundEvents[1]; got.SoundIndex != 4 || !got.Local {
		t.Fatalf("sound event[1] = %+v", got)
	}

	if got := len(c.ParticleEvents); got != 1 {
		t.Fatalf("particle events len = %d, want 1", got)
	}
	if got := c.ParticleEvents[0]; got.Origin != [3]float32{4, 5, 6} || got.Dir != [3]float32{1, -1, 0.5} || got.Count != 1024 || got.Color != 99 {
		t.Fatalf("particle event = %+v", got)
	}
}

func TestConsumeTransientEffectsClearsBuffers(t *testing.T) {
	c := NewClient()
	c.ParticleEvents = []ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	c.TempEntities = []TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{4, 5, 6}}}

	particles := c.ConsumeParticleEvents()
	temps := c.ConsumeTempEntities()
	if len(particles) != 1 || len(temps) != 1 {
		t.Fatalf("consumed = %d particles, %d temps; want 1,1", len(particles), len(temps))
	}
	if len(c.ParticleEvents) != 0 || len(c.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d particles %d temps", len(c.ParticleEvents), len(c.TempEntities))
	}
	if got := len(c.ConsumeParticleEvents()) + len(c.ConsumeTempEntities()); got != 0 {
		t.Fatalf("second consume returned %d events, want 0", got)
	}
}

func TestConsumeStuffCommandsKeepsPartialLine(t *testing.T) {
	c := NewClient()
	c.StuffCmdBuf = "bf\nrecon"

	if got := c.ConsumeStuffCommands(); got != "bf\n" {
		t.Fatalf("ConsumeStuffCommands = %q, want %q", got, "bf\n")
	}
	if got := c.StuffCmdBuf; got != "recon" {
		t.Fatalf("StuffCmdBuf remainder = %q, want %q", got, "recon")
	}

	c.StuffCmdBuf += "nect\n"
	if got := c.ConsumeStuffCommands(); got != "reconnect\n" {
		t.Fatalf("ConsumeStuffCommands second = %q, want %q", got, "reconnect\n")
	}
	if got := c.ConsumeStuffCommands(); got != "" {
		t.Fatalf("ConsumeStuffCommands third = %q, want empty", got)
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
