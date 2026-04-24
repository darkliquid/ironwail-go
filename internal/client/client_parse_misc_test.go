package client

// Parse entity update / fog / sound / set-angle / client-data misc tests split from client_test.go.

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/common"
	"github.com/darkliquid/ironwail-go/internal/console"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

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

func TestParseEntityUpdateCarriesLerpFinish(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	c.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		MsgOrigins: [2][3]float32{{10, 20, 30}, {1, 2, 3}},
		MsgAngles:  [2][3]float32{{0, 45, 0}, {0, 30, 0}},
		MsgTime:    1.9,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(0x80 | inet.U_MOREBITS))
	msg.WriteByte(byte(inet.U_EXTEND1 >> 8))
	msg.WriteByte(byte(inet.U_LERPFINISH >> 16))
	msg.WriteByte(1)
	msg.WriteByte(0x80)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if ent.LerpFlags&inet.LerpFinish == 0 {
		t.Fatal("LerpFlags missing LerpFinish")
	}
	if want := c.MTime[0] + float64(0x80)/255.0; ent.LerpFinish != want {
		t.Fatalf("LerpFinish = %v, want %v", ent.LerpFinish, want)
	}
}

func TestParseServerMessageDoesNotTreat0xFFEntityUpdateAsTerminator(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.MTime = [2]float64{2.0, 1.9}
	c.EntityBaselines[1] = inet.EntityState{
		ModelIndex: 1,
		Frame:      2,
		Colormap:   3,
		Skin:       4,
		Effects:    5,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{10, 20, 30},
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	// 0xFF is a valid fast-update command byte: U_SIGNAL plus all low 7 update bits.
	// Follow with morebits that set ANGLE1/3, MODEL, COLORMAP, SKIN, EFFECTS.
	msg.WriteByte(0xFF)
	msg.WriteByte(0x3f)
	msg.WriteByte(1) // entity number
	msg.WriteByte(9) // model
	msg.WriteByte(8) // frame
	msg.WriteByte(7) // colormap
	msg.WriteByte(6) // skin
	msg.WriteByte(5) // effects
	writeCoord(msg, 40)
	writeAngle(msg, 11)
	writeCoord(msg, 50)
	writeAngle(msg, 22)
	writeCoord(msg, 60)
	writeAngle(msg, 33)
	msg.WriteByte(0xFF) // packet terminator

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	ent := c.Entities[1]
	if got := ent.ModelIndex; got != 9 {
		t.Fatalf("ModelIndex = %d, want 9", got)
	}
	if got := ent.Frame; got != 8 {
		t.Fatalf("Frame = %d, want 8", got)
	}
	if got := ent.Colormap; got != 7 {
		t.Fatalf("Colormap = %d, want 7", got)
	}
	if got := ent.Skin; got != 6 {
		t.Fatalf("Skin = %d, want 6", got)
	}
	if got := ent.Effects; got != 5 {
		t.Fatalf("Effects = %d, want 5", got)
	}
	if got := ent.MsgOrigins[0]; got != [3]float32{40, 50, 60} {
		t.Fatalf("MsgOrigins[0] = %v, want [40 50 60]", got)
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

	data := s.ClientDatagram(0)
	if len(data) == 0 {
		t.Fatal("ClientDatagram returned no data")
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
	data = s.ClientDatagram(0)
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
	data = s.ClientDatagram(0)
	if err := p.ParseServerMessage(data); err != nil {
		t.Fatalf("ParseServerMessage third datagram: %v", err)
	}
	// C omits missing packet entities and lets RelinkEntities stale-clear them
	// locally on the client; there is no explicit server-side retire update.
	if state, ok := c.Entities[s.NumForEdict(ent)]; ok {
		if state.ModelIndex != 2 {
			t.Fatalf("omitted entity %d has ModelIndex=%d before relink, want preserved previous model 2", s.NumForEdict(ent), state.ModelIndex)
		}
		c.RelinkEntities()
		state = c.Entities[s.NumForEdict(ent)]
		if state.ModelIndex != 0 {
			t.Fatalf("stale-cleared entity %d has ModelIndex=%d after relink, want 0", s.NumForEdict(ent), state.ModelIndex)
		}
		if state.Origin != [3]float32{10, 20, 30} {
			t.Fatalf("stale-cleared entity %d origin = %v, want preserved last render origin", s.NumForEdict(ent), state.Origin)
		}
	} else {
		t.Fatalf("entity %d should still be in map after omission/relink stale-clear, but was deleted", s.NumForEdict(ent))
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

func TestClientCurrentFogDensityInterpolationNotQuantized(t *testing.T) {
	c := NewClient()
	c.Time = 5
	c.FogDensity = 255
	c.FogColor = [3]byte{0, 0, 0}
	c.fogOldDensity = 0
	c.fogOldColor = [3]float32{}
	c.fogFadeTime = 3
	c.fogFadeDone = 7

	density, _ := c.CurrentFog()
	if math.Abs(float64(density-float32(1.0/3.0))) > 0.0001 {
		t.Fatalf("density = %v, want %v", density, float32(1.0/3.0))
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
	writeShort(msg, 200)
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

func TestApplyWorldspawnFogDefaultsParsesFogKey(t *testing.T) {
	c := NewClient()
	c.ApplyWorldspawnFogDefaults([]byte(`{"classname" "worldspawn" "fog" "0.5 0.25 0.5 0.75"}`))

	if got := c.FogDensity; got != 128 {
		t.Fatalf("FogDensity = %d, want 128", got)
	}
	if got := c.FogColor; got != [3]byte{64, 128, 191} {
		t.Fatalf("FogColor = %v, want [64 128 191]", got)
	}
	if got := c.fogOldDensity; math.Abs(float64(got-float32(128)/255.0)) > 0.0001 {
		t.Fatalf("fogOldDensity = %v, want %v", got, float32(128)/255.0)
	}
	if got := c.fogOldColor; got != [3]float32{64.0 / 255.0, 128.0 / 255.0, 191.0 / 255.0} {
		t.Fatalf("fogOldColor = %v, want [64/255 128/255 191/255]", got)
	}
	if !c.fogConfigured {
		t.Fatal("fogConfigured = false, want true")
	}
}

func TestApplyWorldspawnFogDefaultsDoesNotOverrideConfiguredFog(t *testing.T) {
	c := NewClient()
	c.SetFogState(32, [3]byte{16, 32, 48}, 0)
	c.ApplyWorldspawnFogDefaults([]byte(`{"classname" "worldspawn" "fog" "0.5 0.25 0.5 0.75"}`))

	if got := c.FogDensity; got != 32 {
		t.Fatalf("FogDensity = %d, want 32", got)
	}
	if got := c.FogColor; got != [3]byte{16, 32, 48} {
		t.Fatalf("FogColor = %v, want [16 32 48]", got)
	}
}

func TestApplyWorldspawnFogDefaultsUsesCGrayWhenFogMissing(t *testing.T) {
	c := NewClient()
	c.ApplyWorldspawnFogDefaults([]byte(`{"classname" "worldspawn" "message" "start"}`))

	if got := c.FogDensity; got != 0 {
		t.Fatalf("FogDensity = %d, want 0", got)
	}
	if got := c.FogColor; got != [3]byte{77, 77, 77} {
		t.Fatalf("FogColor = %v, want [77 77 77]", got)
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
	msg.PutByte(byte(inet.SVCSetAngle))
	msg.WriteAngle16(90)
	msg.WriteAngle16(180)
	msg.WriteAngle16(270)
	msg.PutByte(0xFF)

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
	msg.PutByte(byte(inet.SVCSetAngle))
	msg.WriteFloat(12.5)
	msg.WriteFloat(181.25)
	msg.WriteFloat(-45.75)
	msg.PutByte(0xFF)

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
	msg.PutByte(byte(0x80 | inet.U_MOREBITS | inet.U_ORIGIN1))
	msg.PutByte(byte(inet.U_ANGLE1 >> 8))
	msg.PutByte(1)
	msg.WriteFloat(10.25)
	msg.WriteFloat(12.5)
	msg.PutByte(0xFF)

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

func TestParseClientDataActiveWeaponOverflowUpdatestatWins(t *testing.T) {
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
	msg.WriteByte(0) // truncated active weapon byte in clientdata

	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(byte(inet.StatActiveWeapon))
	writeLong(msg, int32(1<<8)) // full active weapon bitmask from Alkaline compat path
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	if got, want := c.ActiveWeapon(), 1<<8; got != want {
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
