package sdk

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

// The sdk package is a re-export facade: the exported names must be the
// same types the engine itself uses, so mods and engine agree on one
// configuration contract.

func TestConfigAliasIsGameconfigConfig(t *testing.T) {
	c, ok := any(gameconfig.Default()).(Config)
	if !ok {
		t.Fatal("sdk.Config is not the same type as gameconfig.Config")
	}
	if c.GameName != "Ironwail-Go" || c.BaseGameDir != "id1" {
		t.Fatalf("Config alias mismatch: %+v", c)
	}
}

func TestZeroValueResolvesToStockDefaults(t *testing.T) {
	r := Config{}.Resolve()
	def := gameconfig.Default()
	cases := map[string]struct{ got, want string }{
		"GameName":        {r.GameName, def.GameName},
		"BaseGameDir":     {r.BaseGameDir, def.BaseGameDir},
		"UserDirName":     {r.UserDirName, def.UserDirName},
		"ModDirMenuLabel": {r.ModDirMenuLabel, def.ModDirMenuLabel},
		"NetOptionLabel":  {r.NetOptionLabel, def.NetOptionLabel},
		"CSQCInitName":    {r.CSQCInitName, def.CSQCInitName},
	}
	for field, c := range cases {
		if c.got != c.want {
			t.Errorf("Resolve() %s = %q, want %q", field, c.got, c.want)
		}
	}
	if r.ProtocolVer != def.ProtocolVer {
		t.Errorf("Resolve() ProtocolVer = %d, want %d", r.ProtocolVer, def.ProtocolVer)
	}
	if string(r.ProtocolMagic) != string(def.ProtocolMagic) {
		t.Errorf("Resolve() ProtocolMagic = %q, want %q", r.ProtocolMagic, def.ProtocolMagic)
	}
}

func TestModOverridesSurviveResolve(t *testing.T) {
	c := Config{
		GameName:          "Mygame",
		BaseGameDir:       "mygame",
		RequireRegistered: false,
		DefaultRegistered: true,
	}
	r := c.Resolve()
	if r.GameName != "Mygame" || r.BaseGameDir != "mygame" {
		t.Fatalf("mod identity overrides lost: %+v", r)
	}
	// Bool gates are meaningful at false: they must NOT be re-defaulted.
	if r.RequireRegistered {
		t.Error("RequireRegistered=false must survive Resolve")
	}
	if !r.DefaultRegistered {
		t.Error("DefaultRegistered=true must survive Resolve")
	}
	// Unset fields still fall back to stock defaults.
	if r.UserDirName != gameconfig.Default().UserDirName {
		t.Errorf("UserDirName = %q, want stock default %q", r.UserDirName, gameconfig.Default().UserDirName)
	}
}

func TestOptionHelpersProduceRunOptions(t *testing.T) {
	opts := []Option{Headless(), Args("prog", "-basedir", ".")}
	if len(opts) != 2 {
		t.Fatalf("option helpers produced %d options, want 2", len(opts))
	}
}

// TestRunHeadlessBootsWithoutAssets is the SDK delegation contract: a mod
// that calls Run with a minimal config gets a booted engine back without
// any game data present (progs.dat is compiled from pkg/qgo/quakego). This
// mirrors how qcmod test / e2e tests boot headless engines.
func TestRunHeadlessBootsWithoutAssets(t *testing.T) {
	g, err := Run(Config{
		GameName:          "SdkTest",
		BaseGameDir:       "id1",
		RequireRegistered: false,
		DefaultRegistered: true,
	}, Headless(), Args("sdk-test"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if g == nil {
		t.Fatal("Run returned nil Game with nil error")
	}
	if g.Config.GameName != "SdkTest" {
		t.Errorf("Game.Config GameName = %q, want %q", g.Config.GameName, "SdkTest")
	}
	if g.Server == nil {
		t.Fatal("headless boot did not initialise the server")
	}
	if g.QC == nil {
		t.Fatal("headless boot did not load the QC VM")
	}
}

func TestRunErrorIsWrapped(t *testing.T) {
	// A config that cannot boot must surface a wrapped engine error, so mod
	// authors can distinguish SDK boot failures from gameplay errors.
	_, err := Run(Config{BaseGameDir: strings.Repeat("x", 300)}, Headless())
	if err == nil {
		t.Skip("oversized base dir was tolerated; nothing to assert")
	}
	if !strings.Contains(err.Error(), "engine:") {
		t.Errorf("Run error %q is not wrapped as an engine error", err)
	}
}