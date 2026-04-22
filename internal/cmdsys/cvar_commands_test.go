package cmdsys

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func newTestCmdSys() (*CmdSystem, *cvar.CVarSystem) {
	cs := NewCmdSystem()
	cs.CVar = cvar.NewCVarSystem()
	cs.RegisterCvarCommands()
	return cs, cs.CVar
}

func TestRegisterCvarCommandsIncludesParityHelpers(t *testing.T) {
	cs, _ := newTestCmdSys()

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

func TestCvarListPrintsCStyleListingAndPrefixSummary(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_cvarlist_archived", "1", cvar.FlagArchive, "archived")
	cv.Register("test_cvarlist_notify", "2", cvar.FlagNotify, "notify")
	cv.Register("test_cvarlist_other", "3", cvar.FlagNone, "other")

	var printed string
	cs.SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		cs.SetPrintCallback(nil)
	})

	cs.ExecuteText("cvarlist test_cvarlist_")

	if !strings.Contains(printed, "*  test_cvarlist_archived \"1\"\n") {
		t.Fatalf("cvarlist missing archived marker:\n%s", printed)
	}
	if !strings.Contains(printed, " s test_cvarlist_notify \"2\"\n") {
		t.Fatalf("cvarlist missing notify marker:\n%s", printed)
	}
	if !strings.Contains(printed, "   test_cvarlist_other \"3\"\n") {
		t.Fatalf("cvarlist missing plain entry:\n%s", printed)
	}
	if !strings.Contains(printed, "3 cvars beginning with \"test_cvarlist_\"\n") {
		t.Fatalf("cvarlist summary mismatch:\n%s", printed)
	}
}

func TestToggle(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_toggle", "0", cvar.FlagNone, "test")

	cs.cmdToggle([]string{"test_toggle"})
	if cv.IntValue("test_toggle") != 1 {
		t.Fatalf("after toggle from 0: got %d, want 1", cv.IntValue("test_toggle"))
	}
	cs.cmdToggle([]string{"test_toggle"})
	if cv.IntValue("test_toggle") != 0 {
		t.Fatalf("after toggle from 1: got %d, want 0", cv.IntValue("test_toggle"))
	}
}

func TestCycle(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_cycle", "a", cvar.FlagNone, "test")

	cs.cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cv.StringValue("test_cycle") != "b" {
		t.Fatalf("cycle from a: got %q, want b", cv.StringValue("test_cycle"))
	}
	cs.cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cv.StringValue("test_cycle") != "c" {
		t.Fatalf("cycle from b: got %q, want c", cv.StringValue("test_cycle"))
	}
	cs.cmdCycle([]string{"test_cycle", "a", "b", "c"})
	if cv.StringValue("test_cycle") != "a" {
		t.Fatalf("cycle from c: got %q, want a", cv.StringValue("test_cycle"))
	}
}

func TestCycleMatchesNumericEquivalentValue(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_cycle_numeric", "1.0", cvar.FlagNone, "test")

	cs.cmdCycle([]string{"test_cycle_numeric", "0", "1", "2"})
	if cv.StringValue("test_cycle_numeric") != "2" {
		t.Fatalf("cycle from 1.0 through numeric list: got %q, want 2", cv.StringValue("test_cycle_numeric"))
	}
}

func TestCycleRejectsUnknownCvar(t *testing.T) {
	cs, cv := newTestCmdSys()
	const name = "test_cycle_unknown"

	cs.cmdCycle([]string{name, "a", "b"})
	if got := cv.Get(name); got != nil {
		t.Fatalf("cycle created unknown cvar %q with value %q", name, got.String)
	}
}

func TestCycleBack(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_cycleback", "a", cvar.FlagNone, "test")

	cs.cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cv.StringValue("test_cycleback") != "c" {
		t.Fatalf("cycleback from a: got %q, want c", cv.StringValue("test_cycleback"))
	}
	cs.cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cv.StringValue("test_cycleback") != "b" {
		t.Fatalf("cycleback from c: got %q, want b", cv.StringValue("test_cycleback"))
	}
	cs.cmdCycleBack([]string{"test_cycleback", "a", "b", "c"})
	if cv.StringValue("test_cycleback") != "a" {
		t.Fatalf("cycleback from b: got %q, want a", cv.StringValue("test_cycleback"))
	}
}

func TestCycleBackMatchesNumericEquivalentValue(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_cycleback_numeric", "1.0", cvar.FlagNone, "test")

	cs.cmdCycleBack([]string{"test_cycleback_numeric", "0", "1", "2"})
	if cv.StringValue("test_cycleback_numeric") != "0" {
		t.Fatalf("cycleback from 1.0 through numeric list: got %q, want 0", cv.StringValue("test_cycleback_numeric"))
	}
}

func TestInc(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_inc", "5", cvar.FlagNone, "test")

	cs.cmdInc([]string{"test_inc"})
	if cv.IntValue("test_inc") != 6 {
		t.Fatalf("inc default: got %d, want 6", cv.IntValue("test_inc"))
	}
	cs.cmdInc([]string{"test_inc", "10"})
	if cv.IntValue("test_inc") != 16 {
		t.Fatalf("inc 10: got %d, want 16", cv.IntValue("test_inc"))
	}
	cs.cmdInc([]string{"test_inc", "-3"})
	if cv.IntValue("test_inc") != 13 {
		t.Fatalf("inc -3: got %d, want 13", cv.IntValue("test_inc"))
	}
}

func TestIncRejectsUnknownCvar(t *testing.T) {
	cs, cv := newTestCmdSys()
	const name = "test_inc_unknown"

	cs.cmdInc([]string{name})
	if got := cv.Get(name); got != nil {
		t.Fatalf("inc created unknown cvar %q with value %q", name, got.String)
	}
}

func TestReset(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_reset", "42", cvar.FlagNone, "test")
	cv.Set("test_reset", "100")

	cs.cmdReset([]string{"test_reset"})
	if cv.IntValue("test_reset") != 42 {
		t.Fatalf("reset: got %d, want 42", cv.IntValue("test_reset"))
	}
}

func TestResetAll(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_ra1", "10", cvar.FlagNone, "test")
	cv.Register("test_ra2", "20", cvar.FlagNone, "test")
	cv.Set("test_ra1", "99")
	cv.Set("test_ra2", "99")

	cs.cmdResetAll(nil)
	if cv.IntValue("test_ra1") != 10 || cv.IntValue("test_ra2") != 20 {
		t.Fatalf("resetall: ra1=%d ra2=%d, want 10,20",
			cv.IntValue("test_ra1"), cv.IntValue("test_ra2"))
	}
}

func TestResetCfg(t *testing.T) {
	cs, cv := newTestCmdSys()
	cv.Register("test_arc", "5", cvar.FlagArchive, "archived")
	cv.Register("test_noarc", "5", cvar.FlagNone, "not archived")
	cv.Set("test_arc", "99")
	cv.Set("test_noarc", "99")

	cs.cmdResetCfg(nil)
	if cv.IntValue("test_arc") != 5 {
		t.Fatalf("resetcfg archived: got %d, want 5", cv.IntValue("test_arc"))
	}
	if cv.IntValue("test_noarc") != 99 {
		t.Fatalf("resetcfg non-archived: got %d, want 99 (unchanged)", cv.IntValue("test_noarc"))
	}
}
