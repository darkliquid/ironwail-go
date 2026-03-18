package cmdsys

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

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

func TestReset(t *testing.T) {
	cvar.Register("test_reset", "42", cvar.FlagNone, "test")
	cvar.Set("test_reset", "100")
	defer cvar.Set("test_reset", "42")

	cmdReset([]string{"test_reset"})
	if cvar.IntValue("test_reset") != 42 {
		t.Fatalf("reset: got %d, want 42", cvar.IntValue("test_reset"))
	}
}

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
