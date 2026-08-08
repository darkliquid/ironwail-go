package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestREPLHostsDebuggerSession drives the headless REPL with a scripted
// session: functions list, break+run a function, inspect an edict, quit.
// It proves Phase D (headless REPL over the In-VM debugger) end-to-end.
func TestREPLHostsDebuggerSession(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	script := strings.Join([]string{
		"functions Start",
		"break SUB_Null",
		"run SUB_Null 1 0 1.0",
		"inspect 1",
		"quit",
	}, "\n") + "\n"

	var out bytes.Buffer
	code := runREPL(strings.NewReader(script), &out)
	if code != 0 {
		t.Fatalf("runREPL exit = %d, want 0\noutput:\n%s", code, out.String())
	}
	s := out.String()
	for _, want := range []string{
		"StartFrame",
		"break set: SUB_Null",
		"paused: break at function",
		"edict 1:",
		"movetype = 0",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("REPL output missing %q\noutput:\n%s", want, s)
		}
	}
}
