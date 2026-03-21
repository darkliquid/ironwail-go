package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/common"
	"github.com/ironwail/ironwail-go/internal/console"
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
var firstServerUpdateMsg = []byte{byte(inet.SVCTime), 0, 0, 0, 0, 0xff}

func TestParseServerSignOnSequence(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	for _, msg := range [][]byte{serverSignOnMsg1, serverSignOnMsg2, serverSignOnMsg3, firstServerUpdateMsg} {
		if err := p.ParseServerMessage(msg); err != nil {
			t.Fatalf("ParseServerMessage() error = %v", err)
		}
	}

	if c.Protocol != inet.PROTOCOL_FITZQUAKE {
		t.Fatalf("protocol = %d, want %d", c.Protocol, inet.PROTOCOL_FITZQUAKE)
	}
	if c.ProtocolFlags != 0 {
		t.Fatalf("protocol flags = %d, want 0 for FitzQuake serverinfo", c.ProtocolFlags)
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

func TestParseServerMessageAcknowledgesCommandOnServerTime(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.CommandCount = 2
	p := NewParser(c)

	if err := p.ParseServerMessage(firstServerUpdateMsg); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if c.CommandCount != 1 {
		t.Fatalf("CommandCount = %d, want 1 after svc_time", c.CommandCount)
	}
}

func TestParseServerMessageAcceptsNaturalEndOfBuffer(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	p := NewParser(c)

	msg := append([]byte(nil), firstServerUpdateMsg[:len(firstServerUpdateMsg)-1]...)
	if err := p.ParseServerMessage(msg); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.MTime[0]; got != 0 {
		t.Fatalf("server time = %v, want 0", got)
	}
}

func TestParseServerInfoRMQReadsProtocolFlags(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	flags := int32(inet.PRFL_FLOATCOORD | inet.PRFL_SHORTANGLE)
	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCServerInfo))
	writeLong(msg, inet.PROTOCOL_RMQ)
	writeLong(msg, flags)
	msg.WriteByte(0x04) // maxclients
	msg.WriteByte(0x00) // gametype
	msg.WriteString("RMQ Test Map")
	msg.WriteByte(0)
	msg.WriteString("maps/start.bsp")
	msg.WriteByte(0)
	msg.WriteByte(0) // model list terminator
	msg.WriteByte(0) // sound list terminator
	msg.WriteByte(0xff)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if c.Protocol != inet.PROTOCOL_RMQ {
		t.Fatalf("protocol = %d, want %d", c.Protocol, inet.PROTOCOL_RMQ)
	}
	if c.ProtocolFlags != uint32(flags) {
		t.Fatalf("protocol flags = %d, want %d", c.ProtocolFlags, uint32(flags))
	}
}

func TestParseClientDataEntityAndTempEntity(t *testing.T) {
	c := NewClient()
	c.Time = 2.5
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)

	msg.WriteByte(byte(inet.SVCSpawnBaseline))
	writeShort(msg, 1)
	msg.WriteByte(5)
	msg.WriteByte(1)
	msg.WriteByte(2)
	msg.WriteByte(3)
	// Origins and angles interleaved: O1, A1, O2, A2, O3, A3
	writeCoord(msg, 1)
	writeAngle(msg, 0)
	writeCoord(msg, 2)
	writeAngle(msg, 90)
	writeCoord(msg, 3)
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
	// Field order: FRAME, O1, O2, A2, O3 (interleaved)
	msg.WriteByte(4)
	writeCoord(msg, 10)
	writeCoord(msg, 20)
	writeAngle(msg, 45)
	writeCoord(msg, 30)

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
	if got := c.ViewHeight; got != 30 {
		t.Fatalf("viewheight = %v, want 30", got)
	}
	if got := c.PunchAngle; got != [3]float32{7, 0, 0} {
		t.Fatalf("punch angle = %v, want [7 0 0]", got)
	}
	if got := c.PunchAngles[0]; got != [3]float32{7, 0, 0} {
		t.Fatalf("current punch angles = %v, want [7 0 0]", got)
	}
	if got := c.PunchAngles[1]; got != [3]float32{} {
		t.Fatalf("previous punch angles = %v, want zero", got)
	}
	if got := c.PunchTime; got != 2.5 {
		t.Fatalf("punch time = %v, want 2.5", got)
	}

	ent := c.Entities[1]
	if got := ent.Frame; got != 4 {
		t.Fatalf("entity frame = %d, want 4", got)
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{10, 20, 30} {
		t.Fatalf("entity MsgOrigins[0] = %v, want [10 20 30]", got)
	}
	if got := ent.Origin; got != [3]float32{1, 2, 3} {
		t.Fatalf("entity origin = %v, want preserved live origin [1 2 3] until relink", got)
	}
	if got := ent.MsgAngles[0][1]; got < 44.5 || got > 45.5 {
		t.Fatalf("entity raw yaw = %f, want ~45", got)
	}
	if got := ent.Angles[1]; got != 90 {
		t.Fatalf("entity yaw = %f, want preserved live yaw 90 until relink", got)
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

func TestParseClientDataResetsViewHeightAndPunchWhenBitsOmitted(t *testing.T) {
	c := NewClient()
	c.Time = 1.5
	p := NewParser(c)

	first := bytes.NewBuffer(nil)
	first.WriteByte(byte(inet.SVCClientData))
	writeShort(first, int(inet.SU_VIEWHEIGHT|inet.SU_PUNCH1))
	first.WriteByte(byte(int8(30)))
	first.WriteByte(byte(int8(7)))
	writeLong(first, 0)
	writeShort(first, 100)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0xFF)

	if err := p.ParseServerMessage(first.Bytes()); err != nil {
		t.Fatalf("first ParseServerMessage() error = %v", err)
	}

	c.Time = 3.5
	second := bytes.NewBuffer(nil)
	second.WriteByte(byte(inet.SVCClientData))
	writeShort(second, 0)
	writeLong(second, 0)
	writeShort(second, 100)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0xFF)

	if err := p.ParseServerMessage(second.Bytes()); err != nil {
		t.Fatalf("second ParseServerMessage() error = %v", err)
	}

	if got := c.ViewHeight; got != inet.DEFAULT_VIEWHEIGHT {
		t.Fatalf("viewheight = %v, want %d", got, inet.DEFAULT_VIEWHEIGHT)
	}
	if got := c.PunchAngle; got != [3]float32{} {
		t.Fatalf("punch angle = %v, want zero", got)
	}
	if got := c.PunchAngles[0]; got != [3]float32{} {
		t.Fatalf("current punch angles = %v, want zero", got)
	}
	if got := c.PunchAngles[1]; got != [3]float32{7, 0, 0} {
		t.Fatalf("previous punch angles = %v, want [7 0 0]", got)
	}
	if got := c.PunchTime; got != 3.5 {
		t.Fatalf("punch time = %v, want 3.5", got)
	}
}

func TestParseClientDataResetsWeaponFrameWhenBitsOmitted(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	first := bytes.NewBuffer(nil)
	first.WriteByte(byte(inet.SVCClientData))
	writeShort(first, int(inet.SU_WEAPONFRAME))
	writeLong(first, 0)
	first.WriteByte(6)
	writeShort(first, 100)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0)
	first.WriteByte(0xFF)

	if err := p.ParseServerMessage(first.Bytes()); err != nil {
		t.Fatalf("first ParseServerMessage() error = %v", err)
	}
	if got := c.WeaponFrame(); got != 6 {
		t.Fatalf("weapon frame after first message = %d, want 6", got)
	}

	second := bytes.NewBuffer(nil)
	second.WriteByte(byte(inet.SVCClientData))
	writeShort(second, 0)
	writeLong(second, 0)
	writeShort(second, 100)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0)
	second.WriteByte(0xFF)

	if err := p.ParseServerMessage(second.Bytes()); err != nil {
		t.Fatalf("second ParseServerMessage() error = %v", err)
	}
	if got := c.WeaponFrame(); got != 0 {
		t.Fatalf("weapon frame after omitted bits = %d, want 0", got)
	}
}

func TestParseClientDataZeroesMissingVelocityBitsAndAdvancesHistory(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	buildClientDataMsg := func(bits uint32, velocity [3]int8) []byte {
		msg := bytes.NewBuffer(nil)
		msg.WriteByte(byte(inet.SVCClientData))
		writeShort(msg, int(bits))
		for i := 0; i < 3; i++ {
			if bits&(inet.SU_VELOCITY1<<uint(i)) != 0 {
				msg.WriteByte(byte(velocity[i]))
			}
		}
		writeLong(msg, 0)
		writeShort(msg, 100)
		msg.WriteByte(0)
		msg.WriteByte(0)
		msg.WriteByte(0)
		msg.WriteByte(0)
		msg.WriteByte(0)
		msg.WriteByte(0)
		msg.WriteByte(0xFF)
		return msg.Bytes()
	}

	if err := p.ParseServerMessage(buildClientDataMsg(inet.SU_VELOCITY1, [3]int8{4, 0, 0})); err != nil {
		t.Fatalf("first ParseServerMessage() error = %v", err)
	}
	if got := c.Velocity; got != [3]float32{64, 0, 0} {
		t.Fatalf("Velocity = %v, want [64 0 0]", got)
	}
	if got := c.MVelocity[0]; got != [3]float32{64, 0, 0} {
		t.Fatalf("current velocity = %v, want [64 0 0]", got)
	}
	if got := c.MVelocity[1]; got != [3]float32{} {
		t.Fatalf("previous velocity = %v, want zero", got)
	}

	if err := p.ParseServerMessage(buildClientDataMsg(0, [3]int8{})); err != nil {
		t.Fatalf("second ParseServerMessage() error = %v", err)
	}
	if got := c.Velocity; got != [3]float32{} {
		t.Fatalf("Velocity = %v, want zero when SU_VELOCITY bits are absent", got)
	}
	if got := c.MVelocity[0]; got != [3]float32{} {
		t.Fatalf("current velocity = %v, want zeroed current sample", got)
	}
	if got := c.MVelocity[1]; got != [3]float32{64, 0, 0} {
		t.Fatalf("previous velocity = %v, want prior sample [64 0 0]", got)
	}
}

func TestParseEntityUpdateUsesBaselineForOmittedPartialDeltaFields(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{11, 22, 33},
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{999, 999, 999}, // rendered/interpolated value, not raw network snapshot
		Angles:     [3]float32{90, 0, 0},
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{5, 6, 7},
			{8, 9, 10},
		},
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ANGLE2))
	msg.WriteByte(1) // entity num
	writeAngle(msg, 45)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.MsgOrigins[1]; got != [3]float32{10, 20, 30} {
		t.Fatalf("MsgOrigins[1] = %v, want prior raw snapshot [10 20 30]", got)
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{1, 2, 3} {
		t.Fatalf("MsgOrigins[0] = %v, want baseline origin [1 2 3] for omitted fields", got)
	}
	if got := ent.MsgAngles[1]; got != [3]float32{5, 6, 7} {
		t.Fatalf("MsgAngles[1] = %v, want prior raw snapshot [5 6 7]", got)
	}
	if got := ent.MsgAngles[0][0]; got != 11 {
		t.Fatalf("MsgAngles[0][0] = %v, want baseline pitch 11", got)
	}
	if got := ent.MsgAngles[0][1]; got < 44.5 || got > 45.5 {
		t.Fatalf("MsgAngles[0][1] = %v, want updated yaw ~45", got)
	}
	if got := ent.MsgAngles[0][2]; got != 33 {
		t.Fatalf("MsgAngles[0][2] = %v, want baseline roll 33", got)
	}
	if got := ent.Origin; got != [3]float32{999, 999, 999} {
		t.Fatalf("render Origin = %v, want preserved live origin [999 999 999] until relink", got)
	}
	if got := ent.Angles; got != [3]float32{90, 0, 0} {
		t.Fatalf("render Angles = %v, want preserved live angles [90 0 0] until relink", got)
	}
}

func TestParseEntityUpdatePreservesSpriteRuntimeStateAcrossCarryForward(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{11, 22, 33},
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex:           1,
		Frame:                4,
		SpriteSyncBase:       0.75,
		SpriteSyncFrame:      4,
		SpriteSyncModelIndex: 1,
		Origin:               [3]float32{999, 999, 999},
		Angles:               [3]float32{90, 0, 0},
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{5, 6, 7},
			{8, 9, 10},
		},
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ANGLE2))
	msg.WriteByte(1)
	writeAngle(msg, 45)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.SpriteSyncBase; got != 0.75 {
		t.Fatalf("SpriteSyncBase = %v, want 0.75", got)
	}
	if got := ent.SpriteSyncFrame; got != 4 {
		t.Fatalf("SpriteSyncFrame = %d, want 4", got)
	}
	if got := ent.SpriteSyncModelIndex; got != 1 {
		t.Fatalf("SpriteSyncModelIndex = %d, want 1", got)
	}
}

func TestParseEntityUpdateKeepsLiveOriginUntilRelink(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{999, 888, 777},
		Angles:     [3]float32{10, 20, 30},
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{4, 5, 6},
			{7, 8, 9},
		},
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_MOREBITS | inet.U_ORIGIN1 | inet.U_ORIGIN2 | inet.U_ORIGIN3 | inet.U_ANGLE2))
	msg.WriteByte(byte(inet.U_ANGLE1 >> 8))
	msg.WriteByte(1)
	writeCoord(msg, 40)
	writeAngle(msg, 15)
	writeCoord(msg, 50)
	writeAngle(msg, 25)
	writeCoord(msg, 60)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if ent.ForceLink {
		t.Fatal("ForceLink = true, want false for normal delta with fresh previous frame")
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{40, 50, 60} {
		t.Fatalf("MsgOrigins[0] = %v, want latest raw origin [40 50 60]", got)
	}
	if got := ent.MsgOrigins[1]; got != [3]float32{10, 20, 30} {
		t.Fatalf("MsgOrigins[1] = %v, want prior raw origin [10 20 30]", got)
	}
	if got := ent.MsgAngles[0]; got[0] < 13.5 || got[0] > 14.5 || got[1] < 23.5 || got[1] > 24.5 || got[2] != 0 {
		t.Fatalf("MsgAngles[0] = %v, want updated raw angles [~14 ~24 0]", got)
	}
	if got := ent.Origin; got != [3]float32{999, 888, 777} {
		t.Fatalf("Origin = %v, want preserved live origin [999 888 777] until relink", got)
	}
	if got := ent.Angles; got != [3]float32{10, 20, 30} {
		t.Fatalf("Angles = %v, want preserved live angles [10 20 30] until relink", got)
	}
}

func TestParseEntityUpdateForceLinksFirstPartialDeltaWithoutPreviousFrame(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	p := NewParser(c)

	baseline := bytes.NewBuffer(nil)
	baseline.WriteByte(byte(inet.SVCSpawnBaseline))
	writeShort(baseline, 1)
	baseline.WriteByte(5)
	baseline.WriteByte(0)
	baseline.WriteByte(0)
	baseline.WriteByte(0)
	writeCoord(baseline, 1)
	writeAngle(baseline, 10)
	writeCoord(baseline, 2)
	writeAngle(baseline, 20)
	writeCoord(baseline, 3)
	writeAngle(baseline, 30)
	baseline.WriteByte(0xFF)

	if err := p.ParseServerMessage(baseline.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage(baseline) error = %v", err)
	}
	wantBaselineOrigin := c.EntityBaselines[1].Origin
	wantBaselineAngles := c.EntityBaselines[1].Angles

	update := bytes.NewBuffer(nil)
	update.WriteByte(byte(0x80 | inet.U_ORIGIN1 | inet.U_ANGLE2))
	update.WriteByte(1)
	writeCoord(update, 11)
	writeAngle(update, 45)
	update.WriteByte(0xFF)

	if err := p.ParseServerMessage(update.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage(update) error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.MsgOrigins[0]; got != [3]float32{11, wantBaselineOrigin[1], wantBaselineOrigin[2]} {
		t.Fatalf("MsgOrigins[0] = %v, want partial delta over baseline [%v %v %v]", got, float32(11), wantBaselineOrigin[1], wantBaselineOrigin[2])
	}
	if got := ent.MsgOrigins[1]; got != ent.MsgOrigins[0] {
		t.Fatalf("MsgOrigins[1] = %v, want snapped previous origin %v", got, ent.MsgOrigins[0])
	}
	wantAngles := [3]float32{wantBaselineAngles[0], 45, wantBaselineAngles[2]}
	if got := ent.MsgAngles[0]; got != wantAngles {
		t.Fatalf("MsgAngles[0] = %v, want partial delta over baseline %v", got, wantAngles)
	}
	if got := ent.MsgAngles[1]; got != ent.MsgAngles[0] {
		t.Fatalf("MsgAngles[1] = %v, want snapped previous angles %v", got, ent.MsgAngles[0])
	}
	if !ent.ForceLink {
		t.Fatal("ForceLink = false, want true on first partial delta without previous frame")
	}
	if got := ent.Origin; got != ent.MsgOrigins[0] {
		t.Fatalf("render Origin = %v, want raw snapshot %v", got, ent.MsgOrigins[0])
	}
	if got := ent.Angles; got != ent.MsgAngles[0] {
		t.Fatalf("render Angles = %v, want raw snapshot %v", got, ent.MsgAngles[0])
	}
}

func TestParseEntityUpdateForceLinksWhenPreviousFrameMissing(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{111, 222, 333},
		Angles:     [3]float32{1, 2, 3},
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{5, 6, 7},
			{8, 9, 10},
		},
		MsgTime: 1.7,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ORIGIN1 | inet.U_ORIGIN2 | inet.U_ORIGIN3))
	msg.WriteByte(1)
	writeCoord(msg, 40)
	writeCoord(msg, 50)
	writeCoord(msg, 60)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if !ent.ForceLink {
		t.Fatal("ForceLink = false, want true when previous message time is stale")
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{40, 50, 60} {
		t.Fatalf("MsgOrigins[0] = %v, want latest raw origin [40 50 60]", got)
	}
	if got := ent.MsgOrigins[1]; got != ent.MsgOrigins[0] {
		t.Fatalf("MsgOrigins[1] = %v, want snapped previous origin %v", got, ent.MsgOrigins[0])
	}
	if got := ent.Origin; got != ent.MsgOrigins[0] {
		t.Fatalf("Origin = %v, want snapped origin %v", got, ent.MsgOrigins[0])
	}
}

func TestParseEntityUpdateUsesBaselineForOmittedFitzFields(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 513,
		Frame:      514,
		Colormap:   3,
		Skin:       4,
		Effects:    5,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{10, 20, 30},
		Alpha:      200,
		Scale:      190,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1024,
		Frame:      1025,
		Colormap:   8,
		Skin:       9,
		Effects:    10,
		Origin:     [3]float32{40, 50, 60},
		Angles:     [3]float32{70, 80, 90},
		MsgOrigins: [2][3]float32{
			{40, 50, 60},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{70, 80, 90},
			{10, 20, 30},
		},
		MsgTime: 1.9,
		Alpha:   111,
		Scale:   112,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ORIGIN1))
	msg.WriteByte(1)
	writeCoord(msg, 9)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.ModelIndex; got != 513 {
		t.Fatalf("ModelIndex = %d, want baseline 513", got)
	}
	if got := ent.Frame; got != 514 {
		t.Fatalf("Frame = %d, want baseline 514", got)
	}
	if got := ent.Colormap; got != 3 {
		t.Fatalf("Colormap = %d, want baseline 3", got)
	}
	if got := ent.Skin; got != 4 {
		t.Fatalf("Skin = %d, want baseline 4", got)
	}
	if got := ent.Effects; got != 5 {
		t.Fatalf("Effects = %d, want baseline 5", got)
	}
	if got := ent.Alpha; got != 200 {
		t.Fatalf("Alpha = %d, want baseline 200", got)
	}
	if got := ent.Scale; got != 190 {
		t.Fatalf("Scale = %d, want baseline 190", got)
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{9, 2, 3} {
		t.Fatalf("MsgOrigins[0] = %v, want baseline-relative [9 2 3]", got)
	}
	if got := ent.MsgAngles[0]; got != [3]float32{10, 20, 30} {
		t.Fatalf("MsgAngles[0] = %v, want baseline angles [10 20 30]", got)
	}
}

func TestParseEntityUpdateNetQuakeResetsAlphaAndScaleToBaselineWhenTransAbsent(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      200,
		Scale:      190,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      111,
		Scale:      112,
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{5, 6, 7},
			{8, 9, 10},
		},
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ORIGIN1))
	msg.WriteByte(1)
	writeCoord(msg, 9)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.Alpha; got != 200 {
		t.Fatalf("Alpha = %d, want baseline 200", got)
	}
	if got := ent.Scale; got != 190 {
		t.Fatalf("Scale = %d, want baseline 190", got)
	}
}

func TestParseEntityUpdateStepMovePreservesHistoryWithoutForceLink(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		Origin:     [3]float32{10, 20, 30},
		Angles:     [3]float32{0, 0, 0},
		MsgOrigins: [2][3]float32{
			{10, 20, 30},
			{1, 2, 3},
		},
		MsgAngles: [2][3]float32{
			{0, 45, 0},
			{0, 30, 0},
		},
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_ORIGIN1 | inet.U_STEP))
	msg.WriteByte(1)
	writeCoord(msg, 24)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if ent.ForceLink {
		t.Fatal("ForceLink = true, want false for ordinary U_STEP updates with a fresh previous frame")
	}
	if ent.LerpFlags&inet.LerpMoveStep == 0 {
		t.Fatal("LerpFlags missing LerpMoveStep for U_STEP update")
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{24, 0, 0} {
		t.Fatalf("MsgOrigins[0] = %v, want latest raw origin [24 0 0]", got)
	}
	if got := ent.MsgOrigins[1]; got != [3]float32{10, 20, 30} {
		t.Fatalf("MsgOrigins[1] = %v, want preserved previous origin [10 20 30]", got)
	}
	if got := ent.Origin; got != [3]float32{10, 20, 30} {
		t.Fatalf("Origin = %v, want live origin preserved until relink", got)
	}
}

func TestHUDAccessorsExposeParsedStats(t *testing.T) {
	c := NewClient()
	c.Stats[inet.StatHealth] = 81
	c.Stats[inet.StatArmor] = 27
	c.Stats[inet.StatAmmo] = 14
	c.Stats[inet.StatWeapon] = 6
	c.Stats[inet.StatActiveWeapon] = ItemLightning
	c.Stats[inet.StatShells] = 11
	c.Stats[inet.StatNails] = 22
	c.Stats[inet.StatRockets] = 33
	c.Stats[inet.StatCells] = 44

	if got := c.Health(); got != 81 {
		t.Fatalf("Health() = %d, want 81", got)
	}
	if got := c.Armor(); got != 27 {
		t.Fatalf("Armor() = %d, want 27", got)
	}
	if got := c.Ammo(); got != 14 {
		t.Fatalf("Ammo() = %d, want 14", got)
	}
	if got := c.WeaponModelIndex(); got != 6 {
		t.Fatalf("WeaponModelIndex() = %d, want 6", got)
	}
	if got := c.ActiveWeapon(); got != ItemLightning {
		t.Fatalf("ActiveWeapon() = %d, want %d", got, ItemLightning)
	}
	s, n, r, ce := c.AmmoCounts()
	if s != 11 || n != 22 || r != 33 || ce != 44 {
		t.Fatalf("AmmoCounts() = (%d,%d,%d,%d), want (11,22,33,44)", s, n, r, ce)
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
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Type: 0, Dist: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{
				{IsLeaf: true, Index: 1},
				{IsLeaf: true, Index: 1},
			},
		}},
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid, VisOfs: -1},
			{VisOfs: -1},
		},
	}

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
	ent.NumLeafs = 1
	ent.LeafNums[0] = 0

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
	if got.MsgOrigins[0] != [3]float32{10, 20, 30} {
		t.Fatalf("entity MsgOrigins[0] = %v, want [10 20 30]", got.MsgOrigins[0])
	}
	if got.Origin != [3]float32{10, 20, 30} {
		t.Fatalf("entity origin = %v, want initial forced-link origin [10 20 30]", got.Origin)
	}
	if got.MsgAngles[0][1] < 44.5 || got.MsgAngles[0][1] > 45.5 {
		t.Fatalf("entity raw yaw = %f, want ~45", got.MsgAngles[0][1])
	}
	if got.Angles[1] < 44.5 || got.Angles[1] > 45.5 {
		t.Fatalf("entity yaw = %f, want initial forced-link yaw ~45", got.Angles[1])
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
	if got.MsgOrigins[0] != [3]float32{42, 20, 30} {
		t.Fatalf("entity MsgOrigins[0] after delta = %v, want [42 20 30]", got.MsgOrigins[0])
	}
	if got.Origin != [3]float32{10, 20, 30} {
		t.Fatalf("entity origin after delta = %v, want preserved live origin [10 20 30] until relink", got.Origin)
	}

	s.FreeEdict(ent)
	s.Time = 1.7
	data = s.GetClientDatagram(0)
	if err := p.ParseServerMessage(data); err != nil {
		t.Fatalf("ParseServerMessage third datagram: %v", err)
	}
	// After the server retires the entity (ModelIndex → 0), the client keeps
	// the slot in the map with ModelIndex==0, matching C Quake's fixed-size
	// entity array where slots persist. The renderer skips ModelIndex==0.
	if state, ok := c.Entities[s.NumForEdict(ent)]; ok {
		if state.ModelIndex != 0 {
			t.Fatalf("retired entity %d has ModelIndex=%d, want 0", s.NumForEdict(ent), state.ModelIndex)
		}
		c.RelinkEntities()
		state = c.Entities[s.NumForEdict(ent)]
		if state.ModelIndex != 0 {
			t.Fatalf("retired entity %d changed ModelIndex=%d after relink, want 0", s.NumForEdict(ent), state.ModelIndex)
		}
		if state.Origin != [3]float32{10, 20, 30} {
			t.Fatalf("retired entity %d origin = %v, want preserved last render origin", s.NumForEdict(ent), state.Origin)
		}
	} else {
		t.Fatalf("entity %d should still be in map (with ModelIndex==0) after retire, but was deleted", s.NumForEdict(ent))
	}
}

func TestLightStyleValues(t *testing.T) {
	c := NewClient()
	if err := c.SetLightStyle(0, "az"); err != nil {
		t.Fatalf("SetLightStyle error: %v", err)
	}
	if err := c.SetLightStyle(1, "m"); err != nil {
		t.Fatalf("SetLightStyle error: %v", err)
	}

	c.Time = 0.0
	values := c.LightStyleValues()
	if values[0] != 0 {
		t.Fatalf("style[0] at t=0 = %f, want 0", values[0])
	}
	if math.Abs(float64(values[1]-1.0)) > 1e-6 {
		t.Fatalf("style[1] at t=0 = %f, want 1", values[1])
	}

	c.Time = 0.1
	values = c.LightStyleValues()
	if values[0] <= 2.0 {
		t.Fatalf("style[0] at t=0.1 = %f, want > 2", values[0])
	}
}

func TestEvalLightStyleInterpolation(t *testing.T) {
	// "mm" is constant normal brightness — no interpolation needed.
	style := LightStyle{Map: "mm", Length: 2, Average: 'm', Peak: 'm'}
	cfg := DefaultLightStyleConfig()
	cfg.LerpLightStyles = 2 // always lerp

	// At any time, brightness should be 1.0 (normal).
	for _, tm := range []float64{0, 0.05, 0.1, 0.15} {
		v := evalLightStyleValue(style, tm, cfg)
		if math.Abs(float64(v-1.0)) > 1e-5 {
			t.Errorf("constant 'mm' at t=%f = %f, want 1.0", tm, v)
		}
	}

	// "mn" (12, 13): small change, should interpolate smoothly.
	style = LightStyle{Map: "mn", Length: 2, Average: 'm', Peak: 'n'}
	// At t=0.0: idx=0 ('m'=12), next=1 ('n'=13), frac=0 → 12/12 = 1.0
	v0 := evalLightStyleValue(style, 0.0, cfg)
	if math.Abs(float64(v0-1.0)) > 1e-5 {
		t.Errorf("'mn' at t=0.0 = %f, want 1.0", v0)
	}
	// At t=0.05: frac=0.5 → lerp(12, 13, 0.5) = 12.5/12 ≈ 1.0417
	v05 := evalLightStyleValue(style, 0.05, cfg)
	if v05 <= 1.0 || v05 >= 13.0/12.0+0.01 {
		t.Errorf("'mn' at t=0.05 = %f, want ~1.04", v05)
	}
	// At t=0.1: idx=1 ('n'=13), frac=0 → 13/12 ≈ 1.0833
	v1 := evalLightStyleValue(style, 0.1, cfg)
	expected := float32(13.0) / 12.0
	if math.Abs(float64(v1-expected)) > 1e-5 {
		t.Errorf("'mn' at t=0.1 = %f, want %f", v1, expected)
	}
}

func TestEvalLightStyleAbruptChangeSkip(t *testing.T) {
	// "az" has a large brightness jump (0 to 25).
	style := LightStyle{Map: "az", Length: 2, Average: 'm', Peak: 'z'}

	// With LerpLightStyles=1 (default): abrupt changes are NOT interpolated.
	cfg := DefaultLightStyleConfig()
	// At t=0.0, idx=0 ('a'=0), next=1 ('z'=25), diff=25 >= 6 → snap.
	v := evalLightStyleValue(style, 0.0, cfg)
	if v != 0 {
		t.Errorf("abrupt skip at t=0: got %f, want 0", v)
	}
	// At midframe t=0.05, should still snap (no interpolation).
	v05 := evalLightStyleValue(style, 0.05, cfg)
	if v05 != 0 {
		t.Errorf("abrupt skip at t=0.05: got %f, want 0 (no lerp)", v05)
	}

	// With LerpLightStyles=2 (always lerp): should interpolate even abrupt changes.
	cfg.LerpLightStyles = 2
	v05lerp := evalLightStyleValue(style, 0.05, cfg)
	if v05lerp <= 0 {
		t.Errorf("forced lerp at t=0.05: got %f, want > 0", v05lerp)
	}
}

func TestEvalLightStyleFlatModes(t *testing.T) {
	// "azaz" pattern with average ≈ 'm' and peak = 'z'.
	style := LightStyle{Map: "azaz", Length: 4}
	// Manually compute average and peak.
	style.Peak = 'z'
	style.Average = byte((0+25+0+25)/4) + 'a' // 12 + 'a' = 'm'

	// FlatLightStyles=1: use average.
	cfg := DefaultLightStyleConfig()
	cfg.FlatLightStyles = 1
	v := evalLightStyleValue(style, 0.0, cfg)
	expected := float32(style.Average-'a') / 12.0
	if math.Abs(float64(v-expected)) > 1e-5 {
		t.Errorf("flat=1 average: got %f, want %f", v, expected)
	}
	// Should be same at any time (static).
	v2 := evalLightStyleValue(style, 0.35, cfg)
	if v != v2 {
		t.Errorf("flat=1 should be time-independent: t=0 %f, t=0.35 %f", v, v2)
	}

	// FlatLightStyles=2: use peak.
	cfg.FlatLightStyles = 2
	vPeak := evalLightStyleValue(style, 0.0, cfg)
	expectedPeak := float32(style.Peak-'a') / 12.0
	if math.Abs(float64(vPeak-expectedPeak)) > 1e-5 {
		t.Errorf("flat=2 peak: got %f, want %f", vPeak, expectedPeak)
	}
}

func TestEvalLightStyleDynamicLightsOff(t *testing.T) {
	style := LightStyle{Map: "azaz", Length: 4, Average: 'm', Peak: 'z'}
	cfg := DefaultLightStyleConfig()
	cfg.DynamicLights = false
	// Should use average, matching r_dynamic=0 in C.
	v := evalLightStyleValue(style, 0.0, cfg)
	expected := float32(style.Average-'a') / 12.0
	if math.Abs(float64(v-expected)) > 1e-5 {
		t.Errorf("dynamic off: got %f, want %f (average)", v, expected)
	}
}

func TestEvalLightStyleNoLerpMode(t *testing.T) {
	// "mn" small change, should snap with LerpLightStyles=0.
	style := LightStyle{Map: "mn", Length: 2, Average: 'm', Peak: 'n'}
	cfg := DefaultLightStyleConfig()
	cfg.LerpLightStyles = 0

	// At t=0.05 (mid-frame): should snap to frame 0 value, no interpolation.
	v := evalLightStyleValue(style, 0.05, cfg)
	expected := float32(12.0) / 12.0 // 'm'=12
	if math.Abs(float64(v-expected)) > 1e-5 {
		t.Errorf("no-lerp at t=0.05: got %f, want %f (snapped)", v, expected)
	}
}

func TestLightStyleValuesWithConfig(t *testing.T) {
	c := NewClient()
	_ = c.SetLightStyle(0, "m") // normal
	_ = c.SetLightStyle(1, "a") // dark
	c.Time = 0.0

	cfg := DefaultLightStyleConfig()
	values := c.LightStyleValuesWithConfig(cfg)
	if math.Abs(float64(values[0]-1.0)) > 1e-5 {
		t.Errorf("style 0 = %f, want 1.0", values[0])
	}
	if values[1] != 0 {
		t.Errorf("style 1 = %f, want 0.0", values[1])
	}
	// Unset styles default to 1.0.
	if values[2] != 1.0 {
		t.Errorf("style 2 (unset) = %f, want 1.0", values[2])
	}
}

func TestParseStaticEntityAndSoundMessages(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)

	msg.WriteByte(byte(inet.SVCSpawnStatic))
	msg.WriteByte(5) // model
	msg.WriteByte(1) // frame
	msg.WriteByte(2) // colormap
	msg.WriteByte(3) // skin
	// Interleaved origins and angles: O1, A1, O2, A2, O3, A3
	writeCoord(msg, 10)
	writeAngle(msg, 45)
	writeCoord(msg, 20)
	writeAngle(msg, 90)
	writeCoord(msg, 30)
	writeAngle(msg, 180)

	msg.WriteByte(byte(inet.SVCSpawnStatic2))
	msg.WriteByte(byte(inet.BLARGEMODEL | inet.BLARGEFRAME | inet.BALPHA | inet.BSCALE))
	writeShort(msg, 300) // model (large)
	writeShort(msg, 400) // frame (large)
	msg.WriteByte(0)     // colormap
	msg.WriteByte(7)     // skin
	// Interleaved origins and angles
	writeCoord(msg, 1)
	writeAngle(msg, 0)
	writeCoord(msg, 2)
	writeAngle(msg, 10)
	writeCoord(msg, 3)
	writeAngle(msg, 20)
	msg.WriteByte(200) // alpha
	msg.WriteByte(24)  // scale

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
	if c.CenterPrintAt != c.Time {
		t.Fatalf("centerprint at = %f, want %f", c.CenterPrintAt, c.Time)
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

func TestParseFinaleCutScenePreservesCenterText(t *testing.T) {
	c := NewClient()
	c.Time = 12.5
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCFinale))
	msg.WriteString("Finale text")
	msg.WriteByte(0)
	msg.WriteByte(byte(inet.SVCCutScene))
	msg.WriteString("Cutscene text")
	msg.WriteByte(0)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if c.Intermission != 3 {
		t.Fatalf("intermission = %d, want 3", c.Intermission)
	}
	if c.CenterPrint != "Cutscene text" {
		t.Fatalf("centerprint = %q, want cutscene text", c.CenterPrint)
	}
	if c.CenterPrintAt != 12.5 {
		t.Fatalf("centerprint at = %f, want 12.5", c.CenterPrintAt)
	}
	if c.CompletedTime != 12.5 {
		t.Fatalf("completed time = %f, want 12.5", c.CompletedTime)
	}
}

func TestParseFinaleCutSceneRefreshesRevealStartTime(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	c.Time = 3
	finale := bytes.NewBuffer(nil)
	finale.WriteByte(byte(inet.SVCFinale))
	finale.WriteString("Finale")
	finale.WriteByte(0)
	finale.WriteByte(0xFF)
	if err := p.ParseServerMessage(finale.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage(finale) error = %v", err)
	}
	if c.CenterPrintAt != 3 || c.CompletedTime != 3 {
		t.Fatalf("finale timing = center %f completed %f, want 3/3", c.CenterPrintAt, c.CompletedTime)
	}

	c.Time = 7.25
	cutscene := bytes.NewBuffer(nil)
	cutscene.WriteByte(byte(inet.SVCCutScene))
	cutscene.WriteString("Cutscene")
	cutscene.WriteByte(0)
	cutscene.WriteByte(0xFF)
	if err := p.ParseServerMessage(cutscene.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage(cutscene) error = %v", err)
	}
	if c.CenterPrintAt != 7.25 || c.CompletedTime != 7.25 {
		t.Fatalf("cutscene timing = center %f completed %f, want 7.25/7.25", c.CenterPrintAt, c.CompletedTime)
	}
}

func TestConsumeTransientEffectsClearsBuffers(t *testing.T) {
	c := NewClient()
	c.SoundEvents = []SoundEvent{{Entity: 1, Channel: 2, SoundIndex: 3}}
	c.StopSoundEvents = []StopSoundEvent{{Entity: 4, Channel: 5}}
	c.ParticleEvents = []ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	c.TempEntities = []TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{4, 5, 6}}}

	events := c.ConsumeTransientEvents()
	if len(events.SoundEvents) != 1 || len(events.StopSoundEvents) != 1 || len(events.ParticleEvents) != 1 || len(events.TempEntities) != 1 {
		t.Fatalf("consumed = %d sounds, %d stops, %d particles, %d temps; want 1,1,1,1", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
	if len(c.SoundEvents) != 0 || len(c.StopSoundEvents) != 0 || len(c.ParticleEvents) != 0 || len(c.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d sounds %d stops %d particles %d temps", len(c.SoundEvents), len(c.StopSoundEvents), len(c.ParticleEvents), len(c.TempEntities))
	}
	if second := c.ConsumeTransientEvents(); len(second.SoundEvents)+len(second.StopSoundEvents)+len(second.ParticleEvents)+len(second.TempEntities) != 0 {
		t.Fatalf("second consume returned %d events, want 0", len(second.SoundEvents)+len(second.StopSoundEvents)+len(second.ParticleEvents)+len(second.TempEntities))
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

func TestLerpPointBypassConditions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		set        func(c *Client)
		wantReason LerpTelemetryReason
	}{
		{"TimeDemoActive", func(c *Client) { c.TimeDemoActive = true }, LerpTelemetryReasonTimeDemo},
		{"LocalServerFast", func(c *Client) { c.LocalServerFast = true }, LerpTelemetryReasonFastServer},
		{"NoLerp", func(c *Client) { c.NoLerp = true }, LerpTelemetryReasonNoLerp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient()
			c.MTime[1] = 1.0
			c.MTime[0] = 1.1
			c.Time = 1.05
			tc.set(c)
			frac := c.LerpPoint()
			if frac != 1 {
				t.Fatalf("LerpPoint() = %f, want 1 (bypass)", frac)
			}
			if c.Time != c.MTime[0] {
				t.Fatalf("Time = %f, want MTime[0] = %f", c.Time, c.MTime[0])
			}
			if telemetry := c.LerpTelemetrySnapshot(); telemetry.Reason != tc.wantReason {
				t.Fatalf("LerpTelemetrySnapshot().Reason = %s, want %s", telemetry.Reason, tc.wantReason)
			}
		})
	}
}

func TestLerpPointTelemetryCapturesNormalAndGapClamp(t *testing.T) {
	c := NewClient()
	c.MTime[1] = 1.0
	c.MTime[0] = 1.08
	c.Time = 1.04
	c.OldTime = 1.0

	if got := c.LerpPoint(); got < 0.49 || got > 0.51 {
		t.Fatalf("LerpPoint() = %f, want ~0.5", got)
	}
	telemetry := c.LerpTelemetrySnapshot()
	if telemetry.Reason != LerpTelemetryReasonNormal {
		t.Fatalf("normal telemetry reason = %s, want %s", telemetry.Reason, LerpTelemetryReasonNormal)
	}
	if !telemetry.HasRawFrac || telemetry.RawFrac < 0.49 || telemetry.RawFrac > 0.51 {
		t.Fatalf("normal telemetry raw frac = %f (valid=%t), want ~0.5", telemetry.RawFrac, telemetry.HasRawFrac)
	}
	if telemetry.FrameDeltaBefore >= 0.1 || telemetry.FrameDeltaAfter >= 0.1 {
		t.Fatalf("normal frame delta = %f->%f, want unclamped delta below 0.1", telemetry.FrameDeltaBefore, telemetry.FrameDeltaAfter)
	}

	c.MTime[1] = 1.0
	c.MTime[0] = 1.5
	c.Time = 1.45
	c.OldTime = 1.4

	if got := c.LerpPoint(); got < 0.49 || got > 0.51 {
		t.Fatalf("gap-clamped LerpPoint() = %f, want ~0.5", got)
	}
	telemetry = c.LerpTelemetrySnapshot()
	if telemetry.Reason != LerpTelemetryReasonGapClamp {
		t.Fatalf("gap clamp telemetry reason = %s, want %s", telemetry.Reason, LerpTelemetryReasonGapClamp)
	}
	if !telemetry.GapClamped {
		t.Fatal("gap clamp telemetry did not record GapClamped")
	}
	if telemetry.MTime1After != 1.4 {
		t.Fatalf("gap clamp MTime1After = %f, want 1.4", telemetry.MTime1After)
	}
}

func TestLerpPointTelemetryCapturesFractionClampReasons(t *testing.T) {
	c := NewClient()
	c.MTime[1] = 1.0
	c.MTime[0] = 1.1
	c.Time = 0.98

	if got := c.LerpPoint(); got != 0 {
		t.Fatalf("LerpPoint() low clamp = %f, want 0", got)
	}
	telemetry := c.LerpTelemetrySnapshot()
	if telemetry.Reason != LerpTelemetryReasonFracLT0 {
		t.Fatalf("low clamp telemetry reason = %s, want %s", telemetry.Reason, LerpTelemetryReasonFracLT0)
	}
	if !telemetry.TimeSnapped || telemetry.TimeAfter != c.MTime[1] {
		t.Fatalf("low clamp telemetry snap = %t time_after=%f, want snapped to %f", telemetry.TimeSnapped, telemetry.TimeAfter, c.MTime[1])
	}

	c.MTime[1] = 1.0
	c.MTime[0] = 1.1
	c.Time = 1.12

	if got := c.LerpPoint(); got != 1 {
		t.Fatalf("LerpPoint() high clamp = %f, want 1", got)
	}
	telemetry = c.LerpTelemetrySnapshot()
	if telemetry.Reason != LerpTelemetryReasonFracGT1 {
		t.Fatalf("high clamp telemetry reason = %s, want %s", telemetry.Reason, LerpTelemetryReasonFracGT1)
	}
	if !telemetry.TimeSnapped || telemetry.TimeAfter != c.MTime[0] {
		t.Fatalf("high clamp telemetry snap = %t time_after=%f, want snapped to %f", telemetry.TimeSnapped, telemetry.TimeAfter, c.MTime[0])
	}
}

func TestSVCUpdateName(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCUpdateName))
	msg.WriteByte(1)             // player index
	msg.WriteString("PlayerOne") // player name
	msg.WriteByte(0)             // null terminator
	msg.WriteByte(0xFF)          // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.PlayerNames[1]; got != "PlayerOne" {
		t.Fatalf("player name = %q, want %q", got, "PlayerOne")
	}
}

func TestSVCUpdateColors(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCUpdateColors))
	msg.WriteByte(2)    // player index
	msg.WriteByte(0x42) // colors
	msg.WriteByte(0xFF) // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.PlayerColors[2]; got != 0x42 {
		t.Fatalf("player colors = 0x%02x, want 0x42", got)
	}
}

func TestSVCStopSound(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCStopSound))
	// C encodes entity and channel in one short: (entity << 3) | channel
	writeShort(msg, (10<<3)|3)
	msg.WriteByte(0xFF) // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	if got := len(c.StopSoundEvents); got != 1 {
		t.Fatalf("stop sound events len = %d, want 1", got)
	}
	if got := c.StopSoundEvents[0]; got.Entity != 10 || got.Channel != 3 {
		t.Fatalf("stop sound event = %+v", got)
	}
}

func TestSVCKillMonster(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	if c.KillCount != 0 {
		t.Fatalf("initial kill count = %d, want 0", c.KillCount)
	}

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCKillMonster))
	msg.WriteByte(0xFF) // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.KillCount; got != 1 {
		t.Fatalf("kill count = %d, want 1", got)
	}

	// Parse again to verify increment
	msg = bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCKillMonster))
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.KillCount; got != 2 {
		t.Fatalf("kill count after second = %d, want 2", got)
	}
}

func TestSVCFoundSecret(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	if c.SecretCount != 0 {
		t.Fatalf("initial secret count = %d, want 0", c.SecretCount)
	}

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCFoundSecret))
	msg.WriteByte(0xFF) // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.SecretCount; got != 1 {
		t.Fatalf("secret count = %d, want 1", got)
	}

	// Parse again to verify increment
	msg = bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCFoundSecret))
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.SecretCount; got != 2 {
		t.Fatalf("secret count after second = %d, want 2", got)
	}
}

func TestSVCSkyBox(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCSkyBox))
	msg.WriteString("env/plasma") // skybox name
	msg.WriteByte(0)              // null terminator
	msg.WriteByte(0xFF)           // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.SkyboxName; got != "env/plasma" {
		t.Fatalf("skybox name = %q, want %q", got, "env/plasma")
	}
}

func TestSVCFog(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCFog))
	msg.WriteByte(128)                                       // density
	msg.WriteByte(192)                                       // red
	msg.WriteByte(144)                                       // green
	msg.WriteByte(100)                                       // blue
	_ = binary.Write(msg, binary.LittleEndian, float32(2.5)) // time
	msg.WriteByte(0xFF)                                      // frame terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got := c.FogDensity; got != 128 {
		t.Fatalf("fog density = %d, want 128", got)
	}
	if got := c.FogColor; got != [3]byte{192, 144, 100} {
		t.Fatalf("fog color = %v, want [192 144 100]", got)
	}
	if got := c.FogTime; got < 2.49 || got > 2.51 {
		t.Fatalf("fog time = %f, want ~2.5", got)
	}
}

func TestClientCurrentFogInterpolatesFade(t *testing.T) {
	c := NewClient()
	c.Time = 4
	c.FogDensity = 255
	c.FogColor = [3]byte{255, 128, 0}
	c.fogOldDensity = 0
	c.fogOldColor = [3]float32{}
	c.fogFadeTime = 4
	c.fogFadeDone = 6

	density, color := c.CurrentFog()
	if math.Abs(float64(density-0.5)) > 0.0001 {
		t.Fatalf("density = %v, want 0.5", density)
	}
	want := [3]float32{128.0 / 255.0, 64.0 / 255.0, 0}
	for i := range want {
		if math.Abs(float64(color[i]-want[i])) > 0.0001 {
			t.Fatalf("color[%d] = %v, want %v", i, color[i], want[i])
		}
	}
}

func TestSVCFogStartsFadeFromCurrentValue(t *testing.T) {
	c := NewClient()
	c.Time = 4
	c.FogDensity = 255
	c.FogColor = [3]byte{255, 128, 0}
	c.fogOldDensity = 0
	c.fogOldColor = [3]float32{}
	c.fogFadeTime = 4
	c.fogFadeDone = 6
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCFog))
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	_ = binary.Write(msg, binary.LittleEndian, float32(2.0))
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	if math.Abs(float64(c.fogOldDensity-0.5)) > 0.0001 {
		t.Fatalf("fogOldDensity = %v, want 0.5", c.fogOldDensity)
	}
	want := [3]float32{128.0 / 255.0, 64.0 / 255.0, 0}
	for i := range want {
		if math.Abs(float64(c.fogOldColor[i]-want[i])) > 0.0001 {
			t.Fatalf("fogOldColor[%d] = %v, want %v", i, c.fogOldColor[i], want[i])
		}
	}
	if c.fogFadeDone != 6 {
		t.Fatalf("fogFadeDone = %v, want 6", c.fogFadeDone)
	}
}

func TestParseSoundSupportsExtendedEntityChannelAndSoundIndex(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCSound))
	msg.WriteByte(byte(inet.SND_VOLUME | inet.SND_ATTENUATION | inet.SND_LARGEENTITY | inet.SND_LARGESOUND))
	msg.WriteByte(200)
	msg.WriteByte(byte(0.5 * 64))
	writeShort(msg, 8192)
	msg.WriteByte(17)
	writeShort(msg, 300)
	writeCoord(msg, 10)
	writeCoord(msg, 20)
	writeCoord(msg, 30)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	if len(c.SoundEvents) != 1 {
		t.Fatalf("SoundEvents len = %d, want 1", len(c.SoundEvents))
	}
	ev := c.SoundEvents[0]
	if ev.Entity != 8192 {
		t.Fatalf("entity = %d, want 8192", ev.Entity)
	}
	if ev.Channel != 17 {
		t.Fatalf("channel = %d, want 17", ev.Channel)
	}
	if ev.SoundIndex != 300 {
		t.Fatalf("sound index = %d, want 300", ev.SoundIndex)
	}
	if ev.Volume != 200 {
		t.Fatalf("volume = %d, want 200", ev.Volume)
	}
	if ev.Attenuation != 0.5 {
		t.Fatalf("attenuation = %v, want 0.5", ev.Attenuation)
	}
	if ev.Origin != [3]float32{10, 20, 30} {
		t.Fatalf("origin = %v, want [10 20 30]", ev.Origin)
	}
}

func TestParseSetAngleSnapsViewAngleHistory(t *testing.T) {
	c := NewClient()
	c.MViewAngles[1] = [3]float32{1, 2, 3}
	c.MViewAngles[0] = [3]float32{4, 5, 6}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCSetAngle))
	msg.WriteByte(64)
	msg.WriteByte(128)
	msg.WriteByte(192)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	want := [3]float32{90, 180, 270}
	if c.ViewAngles != want {
		t.Fatalf("ViewAngles = %v, want %v", c.ViewAngles, want)
	}
	if c.MViewAngles[0] != want {
		t.Fatalf("MViewAngles[0] = %v, want %v", c.MViewAngles[0], want)
	}
	if c.MViewAngles[1] != want {
		t.Fatalf("MViewAngles[1] = %v, want %v", c.MViewAngles[1], want)
	}
	if !c.FixAngle {
		t.Fatal("FixAngle = false, want true")
	}
}

func TestParseSetAngleUsesProtocolShortAngles(t *testing.T) {
	c := NewClient()
	c.ProtocolFlags = inet.PRFL_SHORTANGLE
	p := NewParser(c)

	msg := common.NewSizeBuf(32)
	msg.WriteByte(byte(inet.SVCSetAngle))
	msg.WriteAngle16(90)
	msg.WriteAngle16(180)
	msg.WriteAngle16(270)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Data); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	want := [3]float32{90, 180, 270}
	if c.ViewAngles != want {
		t.Fatalf("ViewAngles = %v, want %v", c.ViewAngles, want)
	}
	if c.MViewAngles[0] != want || c.MViewAngles[1] != want {
		t.Fatalf("MViewAngles = %v / %v, want both %v", c.MViewAngles[0], c.MViewAngles[1], want)
	}
}

func TestParseSetAngleUsesProtocolFloatAngles(t *testing.T) {
	c := NewClient()
	c.ProtocolFlags = inet.PRFL_FLOATANGLE
	p := NewParser(c)

	msg := common.NewSizeBuf(32)
	msg.WriteByte(byte(inet.SVCSetAngle))
	msg.WriteFloat(12.5)
	msg.WriteFloat(181.25)
	msg.WriteFloat(-45.75)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Data); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	want := [3]float32{12.5, 181.25, -45.75}
	if c.ViewAngles != want {
		t.Fatalf("ViewAngles = %v, want %v", c.ViewAngles, want)
	}
	if c.MViewAngles[0] != want || c.MViewAngles[1] != want {
		t.Fatalf("MViewAngles = %v / %v, want both %v", c.MViewAngles[0], c.MViewAngles[1], want)
	}
}

func TestParseEntityUpdateUsesRMQFloatCoordsAndAngles(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_RMQ
	c.ProtocolFlags = inet.PRFL_FLOATCOORD | inet.PRFL_FLOATANGLE
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	p := NewParser(c)

	msg := common.NewSizeBuf(32)
	msg.WriteByte(byte(0x80 | inet.U_MOREBITS | inet.U_ORIGIN1))
	msg.WriteByte(byte(inet.U_ANGLE1 >> 8))
	msg.WriteByte(1)
	msg.WriteFloat(10.25)
	msg.WriteFloat(12.5)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Data[:msg.CurSize]); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	wantOrigin := [3]float32{10.25, 0, 0}
	wantAngles := [3]float32{12.5, 0, 0}
	if ent.MsgOrigins[0] != wantOrigin {
		t.Fatalf("MsgOrigins[0] = %v, want %v", ent.MsgOrigins[0], wantOrigin)
	}
	if ent.MsgAngles[0] != wantAngles {
		t.Fatalf("MsgAngles[0] = %v, want %v", ent.MsgAngles[0], wantAngles)
	}
	if ent.Origin != wantOrigin {
		t.Fatalf("Origin = %v, want %v", ent.Origin, wantOrigin)
	}
	if ent.Angles != wantAngles {
		t.Fatalf("Angles = %v, want %v", ent.Angles, wantAngles)
	}
}

func TestParseClientDataNormalizesIndexedActiveWeapon(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCClientData))
	writeShort(msg, 0)
	writeLong(msg, 0)
	writeShort(msg, 100)
	msg.WriteByte(20)
	msg.WriteByte(5)
	msg.WriteByte(6)
	msg.WriteByte(7)
	msg.WriteByte(8)
	msg.WriteByte(5)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got, want := c.ActiveWeapon(), ItemRocketLauncher; got != want {
		t.Fatalf("ActiveWeapon() = %d, want %d", got, want)
	}
}

func TestSVCPrintWritesToConsole(t *testing.T) {
	var printed []string
	console.SetPrintCallback(func(msg string) {
		printed = append(printed, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCPrint))
	msg.WriteString("hello from server")
	msg.WriteByte(0)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	if len(printed) != 1 || printed[0] != "hello from server" {
		t.Fatalf("printed = %v, want [hello from server]", printed)
	}
}

func TestParseClientDataLogsSuspiciousPacketTrace(t *testing.T) {
	var printed []string
	console.SetPrintCallback(func(msg string) {
		printed = append(printed, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCStuffText))
	msg.WriteString("echo test")
	msg.WriteByte(0)
	msg.WriteByte(byte(inet.SVCClientData))
	writeShort(msg, int(inet.SU_PUNCH1|inet.SU_PUNCH3|inet.SU_VELOCITY2))
	msg.WriteByte(byte(int8(105)))
	msg.WriteByte(byte(int8(32)))
	msg.WriteByte(byte(int8(115)))
	writeLong(msg, 0)
	writeShort(msg, 100)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	joined := strings.Join(printed, "\n")
	if !strings.Contains(joined, "client packet anomaly:") {
		t.Fatalf("console output missing anomaly log: %q", joined)
	}
	if !strings.Contains(joined, "current=svc_clientdata") {
		t.Fatalf("console output missing clientdata offsets: %q", joined)
	}
	if !strings.Contains(joined, "recent=svc_stufftext") {
		t.Fatalf("console output missing prior svc trace: %q", joined)
	}
	if !strings.Contains(joined, "0f 54 00") {
		t.Fatalf("console output missing raw clientdata bytes: %q", joined)
	}
}

func writeShort(buf *bytes.Buffer, v int) {
	_ = binary.Write(buf, binary.LittleEndian, int16(v))
}

func writeLong(buf *bytes.Buffer, v int32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

// writeCoord writes a coordinate as 16-bit fixed-point (default FitzQuake encoding).
func writeCoord(buf *bytes.Buffer, v float32) {
	_ = binary.Write(buf, binary.LittleEndian, int16(math.RoundToEven(float64(v)*8)))
}

func writeAngle(buf *bytes.Buffer, deg float32) {
	buf.WriteByte(byte(deg * 256.0 / 360.0))
}

// Tests for SendMove and SendCmd

func TestSendMoveNotConnected(t *testing.T) {
	c := NewClient()
	c.State = StateDisconnected

	var sent []byte
	sendFunc := func(data []byte) error {
		sent = data
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v, want nil", err)
	}
	if sent != nil {
		t.Fatalf("SendCmd sent data while disconnected")
	}
}

func TestSendStringCmdEncodesOpcodeAndPayload(t *testing.T) {
	c := NewClient()

	msg, err := c.SendStringCmd("prespawn")
	if err != nil {
		t.Fatalf("SendStringCmd error = %v", err)
	}
	if len(msg) < 2 {
		t.Fatalf("message too short: %d", len(msg))
	}
	if msg[0] != byte(inet.CLCStringCmd) {
		t.Fatalf("opcode = %d, want %d", msg[0], inet.CLCStringCmd)
	}
	if got := string(msg[1:]); got != "prespawn\x00" {
		t.Fatalf("payload = %q, want %q", got, "prespawn\\x00")
	}
}

func TestSendMovePacking(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.Time = 1.234

	cmd := &UserCmd{
		ViewAngles: [3]float32{10.0, 45.0, 0.0},
		Forward:    200.0,
		Side:       50.0,
		Up:         0.0,
		Buttons:    3, // attack + jump
		Impulse:    7,
	}

	data, err := c.SendMove(cmd)
	if err != nil {
		t.Fatalf("SendMove() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("SendMove() returned empty data")
	}

	// Parse the message back
	if data[0] != byte(inet.CLCMove) {
		t.Fatalf("first byte = %d, want %d (CLCMove)", data[0], inet.CLCMove)
	}

	// Verify we can parse the message
	buf := &bytes.Buffer{}
	buf.Write(data)

	// Read opcode
	opcode, _ := buf.ReadByte()
	if opcode != byte(inet.CLCMove) {
		t.Fatalf("opcode = %d, want %d", opcode, inet.CLCMove)
	}

	// Read time
	var timeVal float32
	binary.Read(buf, binary.LittleEndian, &timeVal)
	if math.Abs(float64(timeVal-1.234)) > 0.001 {
		t.Fatalf("time = %f, want 1.234", timeVal)
	}

	// Read angles (8-bit for NetQuake)
	angle0, _ := buf.ReadByte()
	angle1, _ := buf.ReadByte()
	angle2, _ := buf.ReadByte()

	// Convert back to degrees
	gotAngle0 := float32(angle0) * 360.0 / 256.0
	gotAngle1 := float32(angle1) * 360.0 / 256.0
	gotAngle2 := float32(angle2) * 360.0 / 256.0

	if math.Abs(float64(gotAngle0-10.0)) > 2.0 {
		t.Fatalf("angle[0] = %f, want ~10.0", gotAngle0)
	}
	if math.Abs(float64(gotAngle1-45.0)) > 2.0 {
		t.Fatalf("angle[1] = %f, want ~45.0", gotAngle1)
	}
	if math.Abs(float64(gotAngle2)) > 2.0 {
		t.Fatalf("angle[2] = %f, want ~0.0", gotAngle2)
	}

	// Read movement
	var forward, side, up int16
	binary.Read(buf, binary.LittleEndian, &forward)
	binary.Read(buf, binary.LittleEndian, &side)
	binary.Read(buf, binary.LittleEndian, &up)

	if forward != 200 {
		t.Fatalf("forward = %d, want 200", forward)
	}
	if side != 50 {
		t.Fatalf("side = %d, want 50", side)
	}
	if up != 0 {
		t.Fatalf("up = %d, want 0", up)
	}

	// Read buttons and impulse
	buttons, _ := buf.ReadByte()
	impulse, _ := buf.ReadByte()

	if buttons != 3 {
		t.Fatalf("buttons = %d, want 3", buttons)
	}
	if impulse != 7 {
		t.Fatalf("impulse = %d, want 7", impulse)
	}
}

func TestSendMoveWithShortAngles(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.ProtocolFlags = inet.PRFL_SHORTANGLE
	c.Time = 2.5

	cmd := &UserCmd{
		ViewAngles: [3]float32{15.5, 180.25, 5.0},
		Forward:    150.0,
		Side:       -75.0,
		Up:         10.0,
		Buttons:    1, // attack only
		Impulse:    0,
	}

	data, err := c.SendMove(cmd)
	if err != nil {
		t.Fatalf("SendMove() error = %v", err)
	}

	// Verify message is longer (16-bit angles take more space)
	// NetQuake: 1 + 4 + 3 + 6 + 2 = 16 bytes
	// FitzQuake with short angles: 1 + 4 + 6 + 6 + 2 = 19 bytes
	if len(data) < 19 {
		t.Fatalf("message length = %d, want >= 19 for 16-bit angles", len(data))
	}
}

func TestSendCmdDuringSignOn(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.State = StateConnected
	c.Signon = 2 // Not yet complete
	c.MoveMessages = 2
	c.ViewAngles = [3]float32{0, 90, 0}

	var sentData []byte
	sendFunc := func(data []byte) error {
		sentData = make([]byte, len(data))
		copy(sentData, data)
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	if len(sentData) == 0 {
		t.Fatal("SendCmd() did not send data during signon")
	}

	// Should send empty move (no movement values)
	// Parse and verify it's mostly zeros except angles
	buf := bytes.NewBuffer(sentData)
	buf.ReadByte() // opcode
	var timeVal float32
	binary.Read(buf, binary.LittleEndian, &timeVal)
	buf.ReadByte() // angle 0
	buf.ReadByte() // angle 1
	buf.ReadByte() // angle 2

	var forward, side, up int16
	binary.Read(buf, binary.LittleEndian, &forward)
	binary.Read(buf, binary.LittleEndian, &side)
	binary.Read(buf, binary.LittleEndian, &up)

	if forward != 0 || side != 0 || up != 0 {
		t.Fatalf("movement during signon = (%d,%d,%d), want (0,0,0)", forward, side, up)
	}
}

func TestSendCmdAfterSignOn(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.State = StateActive
	c.Signon = Signons // Complete
	c.MoveMessages = 2
	c.Time = 5.0

	// Simulate accumulated input
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{5.0, 270.0, 0.0},
		Forward:    300.0,
		Side:       -100.0,
		Up:         0.0,
		Buttons:    3,
		Impulse:    10,
	}

	var sentData []byte
	sendFunc := func(data []byte) error {
		sentData = make([]byte, len(data))
		copy(sentData, data)
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	if len(sentData) == 0 {
		t.Fatal("SendCmd() did not send data after signon")
	}
	if c.CommandCount != 1 {
		t.Fatalf("CommandCount = %d, want 1 after sending one command", c.CommandCount)
	}

	// Verify real command was sent
	buf := bytes.NewBuffer(sentData)
	buf.ReadByte() // opcode
	var timeVal float32
	binary.Read(buf, binary.LittleEndian, &timeVal)
	buf.ReadByte() // angles
	buf.ReadByte()
	buf.ReadByte()

	var forward, side, up int16
	binary.Read(buf, binary.LittleEndian, &forward)
	binary.Read(buf, binary.LittleEndian, &side)
	binary.Read(buf, binary.LittleEndian, &up)

	if forward != 300 {
		t.Fatalf("forward = %d, want 300", forward)
	}
	if side != -100 {
		t.Fatalf("side = %d, want -100", side)
	}

	buttons, _ := buf.ReadByte()
	impulse, _ := buf.ReadByte()

	if buttons != 3 {
		t.Fatalf("buttons = %d, want 3", buttons)
	}
	if impulse != 10 {
		t.Fatalf("impulse = %d, want 10", impulse)
	}

	// Verify Cmd was updated
	if c.Cmd.Forward != 300 {
		t.Fatalf("c.Cmd.Forward = %f, want 300", c.Cmd.Forward)
	}
}

func TestSendCmdRateLimit(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.MoveMessages = 0 // Should skip first 2

	sendCount := 0
	sendFunc := func(data []byte) error {
		sendCount++
		return nil
	}

	// First call - should skip (MoveMessages = 0)
	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(1) error = %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("sent on first call, want skip")
	}
	if c.MoveMessages != 1 {
		t.Fatalf("MoveMessages = %d, want 1", c.MoveMessages)
	}

	// Second call - should skip (MoveMessages = 1)
	err = c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(2) error = %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("sent on second call, want skip")
	}
	if c.MoveMessages != 2 {
		t.Fatalf("MoveMessages = %d, want 2", c.MoveMessages)
	}

	// Third call - should send
	c.PendingCmd.Forward = 100
	err = c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(3) error = %v", err)
	}
	if sendCount != 1 {
		t.Fatalf("sendCount = %d, want 1 after third call", sendCount)
	}
}

func TestSendMoveNilClient(t *testing.T) {
	var c *Client
	data, err := c.SendMove(&UserCmd{})
	if err != nil {
		t.Fatalf("SendMove(nil client) error = %v, want nil", err)
	}
	if data != nil {
		t.Fatalf("SendMove(nil client) returned data")
	}
}

func TestSendCmdNetworkError(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.MoveMessages = 2

	expectedErr := fmt.Errorf("network down")
	sendFunc := func(data []byte) error {
		return expectedErr
	}

	err := c.SendCmd(sendFunc)
	if err == nil {
		t.Fatal("SendCmd() error = nil, want error")
	}
	// Should return the error but not panic
}

// Integration test: SendCmd with mock network socket
func TestSendCmdIntegrationWithSocket(t *testing.T) {
	// Setup client in active state
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.MoveMessages = 2
	c.Time = 3.5

	// Setup input command
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{10.0, 90.0, 0.0},
		Forward:    250.0,
		Side:       -50.0,
		Up:         0.0,
		Buttons:    1,
		Impulse:    5,
	}

	// Mock network send function that captures data
	var sentMessages [][]byte
	sendFunc := func(data []byte) error {
		captured := make([]byte, len(data))
		copy(captured, data)
		sentMessages = append(sentMessages, captured)
		return nil
	}

	// Send command
	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	// Verify exactly one message was sent
	if len(sentMessages) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sentMessages))
	}

	// Parse the message
	msg := sentMessages[0]
	if len(msg) < 16 {
		t.Fatalf("message too short: %d bytes", len(msg))
	}

	// Verify it's a CLCMove message
	if msg[0] != byte(inet.CLCMove) {
		t.Fatalf("message type = %d, want %d (CLCMove)", msg[0], inet.CLCMove)
	}

	// Parse time
	timeBytes := msg[1:5]
	timeBits := binary.LittleEndian.Uint32(timeBytes)
	timeVal := math.Float32frombits(timeBits)
	if math.Abs(float64(timeVal-3.5)) > 0.01 {
		t.Fatalf("time = %f, want 3.5", timeVal)
	}

	// Parse angles (8-bit for NetQuake)
	angle0 := float32(msg[5]) * 360.0 / 256.0
	angle1 := float32(msg[6]) * 360.0 / 256.0
	_ = msg[7] // angle2 (roll), not checked in this test

	if math.Abs(float64(angle0-10.0)) > 2.0 {
		t.Errorf("angle0 = %f, want ~10.0", angle0)
	}
	if math.Abs(float64(angle1-90.0)) > 2.0 {
		t.Errorf("angle1 = %f, want ~90.0", angle1)
	}

	// Parse movement
	forward := int16(binary.LittleEndian.Uint16(msg[8:10]))
	side := int16(binary.LittleEndian.Uint16(msg[10:12]))
	up := int16(binary.LittleEndian.Uint16(msg[12:14]))

	if forward != 250 {
		t.Errorf("forward = %d, want 250", forward)
	}
	if side != -50 {
		t.Errorf("side = %d, want -50", side)
	}
	if up != 0 {
		t.Errorf("up = %d, want 0", up)
	}

	// Parse buttons and impulse
	buttons := msg[14]
	impulse := msg[15]

	if buttons != 1 {
		t.Errorf("buttons = %d, want 1", buttons)
	}
	if impulse != 5 {
		t.Errorf("impulse = %d, want 5", impulse)
	}

	// Verify client command was updated
	if c.Cmd.Forward != 250.0 {
		t.Errorf("client Cmd not updated")
	}
}

func TestAccumulateCmdSetsPerCommandMsec(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons

	c.AccumulateCmd(0.016)
	if c.PendingCmd.Msec != 16 {
		t.Fatalf("PendingCmd.Msec = %d, want 16", c.PendingCmd.Msec)
	}

	c.AccumulateCmd(1.0)
	if c.PendingCmd.Msec != 255 {
		t.Fatalf("PendingCmd.Msec clamp = %d, want 255", c.PendingCmd.Msec)
	}
	if c.CommandCount != 0 {
		t.Fatalf("CommandCount = %d, want 0 until a command is actually sent", c.CommandCount)
	}
}
