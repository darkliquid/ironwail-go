package cmdsys

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func TestRegisterCvarCommandsIncludesParityHelpers(t *testing.T) {
	cs := NewCmdSystem()
	cs.RegisterCvarCommands()

	for _, name := range []string{
		"cvarlist",
		"toggle",
		"cycle",
		"cycleback",
		"inc",
		"reset",
		"resetall",
		"resetcfg",
	} {
		if !cs.Exists(name) {
			t.Fatalf("command %q not registered", name)
		}
	}
}

// TestToggle tests the toggle console command.
// It ensures that a cvar can be toggled between 0 and 1, which is a common Quake feature for boolean settings.
// Where in C: CV_Toggle_f in cvar.c
func TestToggle(t *testing.T) {
	cvar.Register("test_toggle", "0", cvar.FlagNone, "test")
	defer cvar.Set("test_toggle", "0")

	cmdToggle([]string{"test_toggle"})
	if cvar.IntValue("test_toggle") != 1 {
		t.Fatalf("after toggle from 0: got %d, want 1", cvar.IntValue("test_toggle"))
	}
	cmdToggle([]string{"test_toggle"})
	if cvar.IntValue("test_toggle") != 0 {
		t.Fatalf("after toggle from 1: got %d, want 0", cvar.IntValue("test_toggle"))
	}
}

// TestCycle tests the cycle console command.
// It ensures that a cvar can be cycled through a list of values, allowing for sequential setting changes.
// Where in C: CV_Cycle_f in cvar.c (common engine extension)
func TestCycle(t *testing.T) {
	cvar.Register("test_cycle", "a", cvar.FlagNone, "test")
	defer cvar.Set("test_cycle", "a")

	cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cvar.StringValue("test_cycle") != "b" {
		t.Fatalf("cycle from a: got %q, want b", cvar.StringValue("test_cycle"))
	}
	cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cvar.StringValue("test_cycle") != "c" {
		t.Fatalf("cycle from b: got %q, want c", cvar.StringValue("test_cycle"))
	}
	cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cvar.StringValue("test_cycle") != "a" {
		t.Fatalf("cycle from c: got %q, want a", cvar.StringValue("test_cycle"))
	}
}

// TestCycleBack tests the cycleback console command.
// It ensures that a cvar can be cycled backward through a list of values.
func TestCycleBack(t *testing.T) {
	cvar.Register("test_cycleback", "a", cvar.FlagNone, "test")
	defer cvar.Set("test_cycleback", "a")

	cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cvar.StringValue("test_cycleback") != "c" {
		t.Fatalf("cycleback from a: got %q, want c", cvar.StringValue("test_cycleback"))
	}
	cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cvar.StringValue("test_cycleback") != "b" {
		t.Fatalf("cycleback from c: got %q, want b", cvar.StringValue("test_cycleback"))
	}
	cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cvar.StringValue("test_cycleback") != "a" {
		t.Fatalf("cycleback from b: got %q, want a", cvar.StringValue("test_cycleback"))
	}
}

// TestInc tests the inc console command.
// It ensures that a cvar can be incremented by a specific amount, useful for adjustments like volume or sensitivity.
// Where in C: CV_Inc_f in cvar.c (common engine extension)
func TestInc(t *testing.T) {
	cvar.Register("test_inc", "5", cvar.FlagNone, "test")
	defer cvar.Set("test_inc", "5")

	cmdInc([]string{"test_inc"})
	if cvar.IntValue("test_inc") != 6 {
		t.Fatalf("inc default: got %d, want 6", cvar.IntValue("test_inc"))
	}
	cmdInc([]string{"test_inc", "10"})
	if cvar.IntValue("test_inc") != 16 {
		t.Fatalf("inc 10: got %d, want 16", cvar.IntValue("test_inc"))
	}
	cmdInc([]string{"test_inc", "-3"})
	if cvar.IntValue("test_inc") != 13 {
		t.Fatalf("inc -3: got %d, want 13", cvar.IntValue("test_inc"))
	}
}

// TestReset tests the reset console command for a single cvar.
// It ensures a cvar can be returned to its default value.
// Where in C: CV_Reset_f in cvar.c
func TestReset(t *testing.T) {
	cvar.Register("test_reset", "42", cvar.FlagNone, "test")
	cvar.Set("test_reset", "100")
	defer cvar.Set("test_reset", "42")

	cmdReset([]string{"test_reset"})
	if cvar.IntValue("test_reset") != 42 {
		t.Fatalf("reset: got %d, want 42", cvar.IntValue("test_reset"))
	}
}

// TestResetAll tests the resetall console command.
// It ensures all cvars can be reset to their default values simultaneously.
// Where in C: CV_ResetAll_f in cvar.c
func TestResetAll(t *testing.T) {
	cvar.Register("test_ra1", "10", cvar.FlagNone, "test")
	cvar.Register("test_ra2", "20", cvar.FlagNone, "test")
	cvar.Set("test_ra1", "99")
	cvar.Set("test_ra2", "99")
	defer cvar.Set("test_ra1", "10")
	defer cvar.Set("test_ra2", "20")

	cmdResetAll(nil)
	if cvar.IntValue("test_ra1") != 10 || cvar.IntValue("test_ra2") != 20 {
		t.Fatalf("resetall: ra1=%d ra2=%d, want 10,20",
			cvar.IntValue("test_ra1"), cvar.IntValue("test_ra2"))
	}
}

// TestResetCfg tests the resetcfg console command.
// It ensures only archived (config-stored) cvars are reset, preserving session-only settings.
// Where in C: CV_ResetCfg_f in cvar.c
func TestResetCfg(t *testing.T) {
	cvar.Register("test_arc", "5", cvar.FlagArchive, "archived")
	cvar.Register("test_noarc", "5", cvar.FlagNone, "not archived")
	cvar.Set("test_arc", "99")
	cvar.Set("test_noarc", "99")
	defer cvar.Set("test_arc", "5")
	defer cvar.Set("test_noarc", "5")

	cmdResetCfg(nil)
	if cvar.IntValue("test_arc") != 5 {
		t.Fatalf("resetcfg archived: got %d, want 5", cvar.IntValue("test_arc"))
	}
	if cvar.IntValue("test_noarc") != 99 {
		t.Fatalf("resetcfg non-archived: got %d, want 99 (unchanged)", cvar.IntValue("test_noarc"))
	}
}
