// What: Core client-side state machine and network message tests.
// Why: Ensures connection handshake, sign-on progress, and state management work correctly.
// Where in C: cl_main.c, cl_parse.c

package client

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
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

// TestParseServerSignOnSequence verifies the standard sequence of sign-on messages from the server.
// Why: Ensures the connection handshake and initial state synchronization (map, models, sounds) follow the Quake protocol.
// Where in C: cl_parse.c, CL_ParseServerMessage.
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

// TestParseServerMessageAcknowledgesCommandOnServerTime verifies that receiving a time update from the server acknowledges outstanding client commands.
// Why: Part of the network flow control to prevent the client from getting too far ahead of the server.
// Where in C: cl_parse.c, CL_ParseServerMessage.
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

// TestParseServerMessageAcceptsNaturalEndOfBuffer ensures the parser correctly handles server messages that end exactly at the buffer boundary.
// Why: Robustness against varied packet sizes and protocol edge cases.
// Where in C: cl_parse.c, CL_ParseServerMessage.
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

// TestParseServerInfoRMQReadsProtocolFlags verifies that the client correctly parses extended protocol flags for RMQ servers.
// Why: Compatibility with modern Quake engine extensions that support improved precision for coordinates and angles.
// Where in C: cl_parse.c, CL_ParseServerInfo.
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

// TestParseClientDataEntityAndTempEntity verifies that SVC_ClientData and SVC_TempEntity messages correctly update local state.
// Why: These messages provide essential updates for the local player's state (health, items, velocity) and transient effects.
// Where in C: cl_parse.c, CL_ParseClientData, CL_ParseTEnt.
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
	if got := ent.Origin; got != [3]float32{10, 20, 30} {
		t.Fatalf("entity origin = %v, want initial forced-link origin [10 20 30]", got)
	}
	if got := ent.MsgAngles[0][1]; got < 44.5 || got > 45.5 {
		t.Fatalf("entity raw yaw = %f, want ~45", got)
	}
	if got := ent.Angles[1]; got < 44.5 || got > 45.5 {
		t.Fatalf("entity yaw = %f, want initial forced-link yaw ~45", got)
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

// TestParseClientDataResetsViewHeightAndPunchWhenBitsOmitted ensures that view height and punch angles return to defaults if not present in a clientdata update.
// Why: Quake's delta compression assumes that omitted fields should be reset to their canonical baseline values.
// Where in C: cl_parse.c, CL_ParseClientData.
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

// TestParseClientDataResetsWeaponFrameWhenBitsOmitted ensures the local weapon animation frame resets if omitted from the update.
// Why: Prevents weapon animations from getting stuck when no new frame data is sent.
// Where in C: cl_parse.c, CL_ParseClientData.
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

// TestParseClientDataZeroesMissingVelocityBitsAndAdvancesHistory verifies that velocity is correctly tracked and zeroed when bits are omitted.
// Why: Precise velocity history is required for accurate client-side movement prediction.
// Where in C: cl_parse.c, CL_ParseClientData.
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

// TestParseEntityUpdateUsesBaselineForOmittedPartialDeltaFields verifies that entity updates use baseline values for fields not present in a partial delta.
// Why: Core part of Quake's network efficiency; only changed fields are sent, others are filled from a known baseline.
// Where in C: cl_parse.c, CL_ParsePacketEntities.
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
	if _, ok := c.Entities[1]; ok {
		t.Fatal("spawn baseline seeded live entity state, want baselines-only until first packet update")
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

func TestParseEntityUpdateForceLinksWhenPreviousStateWasRetired(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 0,
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
		MsgTime: 1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_MOREBITS | inet.U_ORIGIN1 | inet.U_ORIGIN2 | inet.U_ORIGIN3))
	msg.WriteByte(byte(inet.U_MODEL >> 8))
	msg.WriteByte(1)
	msg.WriteByte(2)
	writeCoord(msg, 40)
	writeCoord(msg, 50)
	writeCoord(msg, 60)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if !ent.ForceLink {
		t.Fatal("ForceLink = false, want true when previous state was retired with ModelIndex 0")
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
