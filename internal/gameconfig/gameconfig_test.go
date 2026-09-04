package gameconfig

import (
	"testing"
)

func TestDefaultMatchesCurrentLiterals(t *testing.T) {
	def := Default()

	if def.GameName != "Ironwail-Go" {
		t.Errorf("GameName = %q, want %q", def.GameName, "Ironwail-Go")
	}
	if def.BaseGameDir != "id1" {
		t.Errorf("BaseGameDir = %q, want %q", def.BaseGameDir, "id1")
	}
	if def.UserDirName != ".ironwail" {
		t.Errorf("UserDirName = %q, want %q", def.UserDirName, ".ironwail")
	}
	if def.RequireRegistered != true {
		t.Errorf("RequireRegistered = %v, want true", def.RequireRegistered)
	}
	if def.DefaultRegistered != false {
		t.Errorf("DefaultRegistered = %v, want false", def.DefaultRegistered)
	}
	if def.RegistrationCVarName != "registered" {
		t.Errorf("RegistrationCVarName = %q, want %q", def.RegistrationCVarName, "registered")
	}
	if def.RegisteredMessage != "Playing registered version.\n" {
		t.Errorf("RegisteredMessage = %q", def.RegisteredMessage)
	}
	if def.SharewareMessage != "Playing shareware version.\n" {
		t.Errorf("SharewareMessage = %q", def.SharewareMessage)
	}
	if def.ModRequiresRegistered != "you must have the registered version to use modified games" {
		t.Errorf("ModRequiresRegistered = %q", def.ModRequiresRegistered)
	}
	if string(def.ProtocolMagic) != "QUAKE\x00" {
		t.Errorf("ProtocolMagic = %q, want %q", def.ProtocolMagic, "QUAKE\x00")
	}
	if def.ProtocolVer != 3 {
		t.Errorf("ProtocolVer = %d, want 3", def.ProtocolVer)
	}
	if def.ProtocolNums.NetQuake != 15 {
		t.Errorf("ProtocolNums.NetQuake = %d, want 15", def.ProtocolNums.NetQuake)
	}
	if def.ProtocolNums.FitzQuake != 666 {
		t.Errorf("ProtocolNums.FitzQuake = %d, want 666", def.ProtocolNums.FitzQuake)
	}
	if def.ProtocolNums.RMQ != 999 {
		t.Errorf("ProtocolNums.RMQ = %d, want 999", def.ProtocolNums.RMQ)
	}
	if def.DefaultSkill != 1 {
		t.Errorf("DefaultSkill = %d, want 1", def.DefaultSkill)
	}
	if def.DefaultDeathmatch != 0 {
		t.Errorf("DefaultDeathmatch = %d, want 0", def.DefaultDeathmatch)
	}
	if def.DefaultCoop != 0 {
		t.Errorf("DefaultCoop = %d, want 0", def.DefaultCoop)
	}
	if def.DefaultTeamplay != 0 {
		t.Errorf("DefaultTeamplay = %d, want 0", def.DefaultTeamplay)
	}
	if def.ModDirMenuLabel != "QUAKE DIRECTORY" {
		t.Errorf("ModDirMenuLabel = %q", def.ModDirMenuLabel)
	}
	if def.NetOptionLabel != "QUAKEWORLD" {
		t.Errorf("NetOptionLabel = %q", def.NetOptionLabel)
	}
	if def.CSQCInitName != "Ironwail" {
		t.Errorf("CSQCInitName = %q", def.CSQCInitName)
	}
}

func TestResolveFillsZeroValues(t *testing.T) {
	// A mod overrides only the name and base dir; everything else must
	// fall back to Default().
	c := Config{GameName: "MyGame", BaseGameDir: "mydata"}.resolve()

	if c.GameName != "MyGame" {
		t.Errorf("GameName = %q, want %q", c.GameName, "MyGame")
	}
	if c.BaseGameDir != "mydata" {
		t.Errorf("BaseGameDir = %q, want %q", c.BaseGameDir, "mydata")
	}
	if c.UserDirName != ".ironwail" {
		t.Errorf("UserDirName = %q, want %q (resolved from default)", c.UserDirName, ".ironwail")
	}
	if c.ProtocolVer != 3 {
		t.Errorf("ProtocolVer = %d, want 3 (resolved)", c.ProtocolVer)
	}
	if !c.RequireRegistered {
		t.Logf("RequireRegistered = false (correct: standalone mods want no gate; bool fields are NOT resolved)")
	}
}

func TestResolvePreservesFalseGate(t *testing.T) {
	// A standalone mod explicitly disables the registered gate. The
	// zero-value false must NOT be replaced by Default()'s true.
	c := Config{RequireRegistered: false}.resolve()
	if c.RequireRegistered != false {
		t.Errorf("RequireRegistered = %v, want false (explicitly disabled)", c.RequireRegistered)
	}
	// Other fields still resolve.
	if c.GameName != "Ironwail-Go" {
		t.Errorf("GameName = %q, want resolved default", c.GameName)
	}
}

func TestResolveFeaturesDefaultToAllEnabled(t *testing.T) {
	c := Config{}.resolve()
	if !c.Features.Console {
		t.Error("Features.Console should resolve to true (all-false falls back to Default)")
	}
	if !c.Features.SinglePlayer {
		t.Error("Features.SinglePlayer should resolve to true")
	}
	if !c.Features.Cheats {
		t.Error("Features.Cheats should resolve to true")
	}
}

func TestResolveFeaturesPreservesExplicitDisable(t *testing.T) {
	c := Config{Features: Features{Console: true}}.resolve()
	// At least one feature is explicitly set (Console=true), so the
	// zero-value fallback must NOT kick in. Unset features stay false.
	if c.Features.SinglePlayer {
		t.Error("SinglePlayer should stay false when Features is partially set")
	}
	if !c.Features.Console {
		t.Error("Console should be true")
	}
}

func TestDefaultRegisteredInt(t *testing.T) {
	if Default().DefaultRegisteredInt() != "0" {
		t.Errorf("Default registered int = %q, want %q", Default().DefaultRegisteredInt(), "0")
	}
	mod := Config{DefaultRegistered: true}.resolve()
	if mod.DefaultRegisteredInt() != "1" {
		t.Errorf("Mod registered int = %q, want %q", mod.DefaultRegisteredInt(), "1")
	}
}

func TestBaseGameDirLower(t *testing.T) {
	if Default().BaseGameDirLower() != "id1" {
		t.Errorf("BaseGameDirLower = %q, want %q", Default().BaseGameDirLower(), "id1")
	}
	c := Config{BaseGameDir: "MyData"}.resolve()
	if c.BaseGameDirLower() != "mydata" {
		t.Errorf("BaseGameDirLower = %q, want %q", c.BaseGameDirLower(), "mydata")
	}
}
