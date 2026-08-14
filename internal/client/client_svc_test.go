package client

// Static-entity, runtime server messages, finale/cut-scene, and SVC* update tests split from client_test.go.

import (
	"bytes"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

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
	if got := c.StaticEntities[0].Origin; got != (types.Vec3{X: 10, Y: 20, Z: 30}) {
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
	if c.FaceAnimUntil != c.Time+0.2 {
		t.Fatalf("face anim until = %f, want %f", c.FaceAnimUntil, c.Time+0.2)
	}
	if c.DamageOrigin != (types.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("damage origin = %v, want [1 2 3]", c.DamageOrigin)
	}

	if got := c.SoundEvents[0]; got.Entity != 1 || got.Channel != 2 || got.SoundIndex != 9 || got.Volume != 200 || got.Attenuation != 0.5 || got.Origin != (types.Vec3{X: 10, Y: 20, Z: 30}) || got.Local {
		t.Fatalf("sound event = %+v", got)
	}
	if got := c.ParticleEvents[0]; got.Origin != (types.Vec3{X: 4, Y: 5, Z: 6}) || got.Dir != (types.Vec3{X: 1, Y: -1, Z: 0.5}) || got.Count != 1024 || got.Color != 99 {
		t.Fatalf("particle event = %+v", got)
	}

	if got := len(c.ParticleEvents); got != 1 {
		t.Fatalf("particle events len = %d, want 1", got)
	}
	if got := c.ParticleEvents[0]; got.Origin != (types.Vec3{X: 4, Y: 5, Z: 6}) || got.Dir != (types.Vec3{X: 1, Y: -1, Z: 0.5}) || got.Count != 1024 || got.Color != 99 {
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
	c.ParticleEvents = []ParticleEvent{{Origin: types.Vec3{X: 1, Y: 2, Z: 3}, Count: 12, Color: 4}}
	c.TempEntities = []TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: types.Vec3{X: 4, Y: 5, Z: 6}}}

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

	// NoLerp is applied after frac computation and clamping (matching C),
	// so Time is not snapped to MTime[0] when frac is in the normal range.
	t.Run("NoLerp", func(t *testing.T) {
		c := NewClient()
		c.MTime[1] = 1.0
		c.MTime[0] = 1.1
		c.Time = 1.05
		c.NoLerp = true
		frac := c.LerpPoint()
		if frac != 1 {
			t.Fatalf("LerpPoint() = %f, want 1 (bypass)", frac)
		}
		// Time is NOT snapped because nolerp fires after frac is in [0,1].
		if c.Time != 1.05 {
			t.Fatalf("Time = %f, want 1.05 (no snap)", c.Time)
		}
		if telemetry := c.LerpTelemetrySnapshot(); telemetry.Reason != LerpTelemetryReasonNoLerp {
			t.Fatalf("LerpTelemetrySnapshot().Reason = %s, want %s", telemetry.Reason, LerpTelemetryReasonNoLerp)
		}
	})
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
	if got := c.Stats[inet.StatMonsters]; got != 2 {
		t.Fatalf("StatMonsters = %d, want 2", got)
	}
	if got := c.StatsF[inet.StatMonsters]; got != 2 {
		t.Fatalf("StatMonstersF = %v, want 2", got)
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
	if got := c.Stats[inet.StatSecrets]; got != 2 {
		t.Fatalf("StatSecrets = %d, want 2", got)
	}
	if got := c.StatsF[inet.StatSecrets]; got != 2 {
		t.Fatalf("StatSecretsF = %v, want 2", got)
	}
}

func TestSVCLevelCompletedAndBackToLobbyAreAccepted(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCLevelCompleted))
	msg.WriteByte(byte(inet.SVCBackToLobby))
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
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
	msg.WriteByte(128)   // density
	msg.WriteByte(192)   // red
	msg.WriteByte(144)   // green
	msg.WriteByte(100)   // blue
	writeShort(msg, 250) // time (2.50s in C wire format)
	msg.WriteByte(0xFF)  // frame terminator

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

func TestSVCBFTriggersBonusFlash(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCBF))
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	shift := c.CShifts[CShiftBonus]
	if shift.Percent != 50 || shift.R != 215 || shift.G != 186 || shift.B != 69 {
		t.Fatalf("bonus shift = %+v, want R215 G186 B69 P50", shift)
	}
}

func TestSVCDisconnectClearsClientRuntimeState(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.LevelName = "start"
	c.MapName = "start"
	c.Stats[0] = 99
	p := NewParser(c)

	msg := []byte{byte(inet.SVCDisconnect), 0xFF}
	if err := p.ParseServerMessage(msg); err == nil {
		t.Fatal("ParseServerMessage() error = nil, want disconnect error")
	}
	if c.State != StateDisconnected {
		t.Fatalf("state = %v, want disconnected", c.State)
	}
	if c.Signon != 0 {
		t.Fatalf("signon = %d, want 0", c.Signon)
	}
	if c.LevelName != "" || c.MapName != "" {
		t.Fatalf("level/map = %q/%q, want cleared", c.LevelName, c.MapName)
	}
	if c.Stats[0] != 0 {
		t.Fatalf("stats[0] = %d, want 0", c.Stats[0])
	}
}
