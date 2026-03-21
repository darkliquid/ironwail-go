package cmdsys

import (
	"reflect"
	"testing"
)

func TestParseCommandPreservesEscapedQuotesAndBackslashes(t *testing.T) {
	args := parseCommand(`bind t "say He said \"hello\" \\world\nnext\tline"`)
	want := []string{"bind", "t", "say He said \"hello\" \\world\nnext\tline"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("parseCommand returned %v, want %v", args, want)
	}
}

func TestCommandTakesPrecedenceOverAlias(t *testing.T) {
	c := NewCmdSystem()

	commandCalled := false
	aliasCalled := false
	c.AddCommand("foo", func(args []string) {
		commandCalled = true
	}, "")
	c.AddCommand("alias_target", func(args []string) {
		aliasCalled = true
	}, "")
	c.AddAlias("foo", "alias_target\n")

	c.ExecuteText("foo")

	if !commandCalled {
		t.Fatal("expected command handler to run")
	}
	if aliasCalled {
		t.Fatal("expected alias not to run when command exists")
	}
}

func TestAliasExecutesUnderlyingCommandText(t *testing.T) {
	c := NewCmdSystem()

	var gotArgs []string
	c.AddCommand("alias_target", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "")

	c.AddAlias("foo", "alias_target bar baz\n")
	c.ExecuteText("foo")

	want := []string{"bar", "baz"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("alias execution args = %v, want %v", gotArgs, want)
	}
}

func TestExecuteTextSplitsSemicolonSeparatedCommands(t *testing.T) {
	c := NewCmdSystem()

	var got []string
	c.AddCommand("first", func(args []string) {
		got = append(got, "first")
	}, "")
	c.AddCommand("second", func(args []string) {
		got = append(got, "second")
	}, "")

	c.ExecuteText("first; second")

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("executed commands = %v, want %v", got, want)
	}
}

func TestAliasExecutesSemicolonSeparatedCommandText(t *testing.T) {
	c := NewCmdSystem()

	var got []string
	c.AddCommand("first", func(args []string) {
		got = append(got, "first")
	}, "")
	c.AddCommand("second", func(args []string) {
		got = append(got, "second")
	}, "")

	c.AddAlias("combo", "first; second\n")
	c.ExecuteText("combo")

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("alias execution order = %v, want %v", got, want)
	}
}

func TestRecursiveAliasStopsAtActiveExpansion(t *testing.T) {
	c := NewCmdSystem()

	calls := 0
	c.AddCommand("mark", func(args []string) {
		calls++
	}, "")
	c.AddAlias("loop", "mark; loop\n")

	c.ExecuteText("loop")

	if calls != 1 {
		t.Fatalf("recursive alias mark calls = %d, want 1", calls)
	}
}

func TestWaitCommandDefersRemainingCommands(t *testing.T) {
	c := NewCmdSystem()

	var executed []string
	c.AddCommand("first", func(args []string) {
		executed = append(executed, "first")
	}, "")
	c.AddCommand("second", func(args []string) {
		executed = append(executed, "second")
	}, "")
	c.AddCommand("third", func(args []string) {
		executed = append(executed, "third")
	}, "")

	// Add commands to buffer: first; wait; second; third
	c.AddText("first; wait; second; third")

	// First Execute() should run 'first' then stop at 'wait'
	c.Execute()
	want := []string{"first"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("after first Execute: executed = %v, want %v", executed, want)
	}

	// Second Execute() should run 'second' and 'third'
	c.Execute()
	want = []string{"first", "second", "third"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("after second Execute: executed = %v, want %v", executed, want)
	}
}

func TestWaitCommandWithExistingBufferContent(t *testing.T) {
	c := NewCmdSystem()

	var executed []string
	c.AddCommand("cmd", func(args []string) {
		executed = append(executed, args[0])
	}, "")

	// Add initial content to buffer
	c.AddText("cmd A; wait; cmd B")
	// Add more content (should go after deferred commands)
	c.AddText("cmd C")

	// First Execute() runs "cmd A" and defers "cmd B"
	c.Execute()
	if !reflect.DeepEqual(executed, []string{"A"}) {
		t.Fatalf("after first Execute: executed = %v, want [A]", executed)
	}

	// Second Execute() should run "cmd B" then "cmd C"
	c.Execute()
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("after second Execute: executed = %v, want %v", executed, want)
	}
}

func TestWaitCommandAtEnd(t *testing.T) {
	c := NewCmdSystem()

	var executed []string
	c.AddCommand("first", func(args []string) {
		executed = append(executed, "first")
	}, "")

	// Wait at end with no remaining commands
	c.AddText("first; wait")
	c.Execute()

	want := []string{"first"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("executed = %v, want %v", executed, want)
	}

	// Second Execute() should do nothing (no deferred commands)
	c.Execute()
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("after second Execute: executed = %v, want %v (unchanged)", executed, want)
	}
}

func TestCommandSourceDefaultsToSrcCommand(t *testing.T) {
	c := NewCmdSystem()

	if got := c.Source(); got != SrcCommand {
		t.Fatalf("default source = %v, want %v", got, SrcCommand)
	}

	var sourceInHandler CommandSource
	c.AddCommand("capture", func(args []string) {
		sourceInHandler = c.Source()
	}, "")

	c.ExecuteText("capture")

	if sourceInHandler != SrcCommand {
		t.Fatalf("source in handler = %v, want %v", sourceInHandler, SrcCommand)
	}
}

func TestExecuteTextWithSourceSetsSourceForHandler(t *testing.T) {
	c := NewCmdSystem()

	var seen []CommandSource
	c.AddClientCommand("capture", func(args []string) {
		seen = append(seen, c.Source())
	}, "")
	c.AddServerCommand("servercapture", func(args []string) {
		seen = append(seen, c.Source())
	}, "")

	c.ExecuteTextWithSource("capture", SrcClient)
	c.ExecuteTextWithSource("servercapture", SrcServer)

	want := []CommandSource{SrcClient, SrcServer}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("seen sources = %v, want %v", seen, want)
	}
	if got := c.Source(); got != SrcCommand {
		t.Fatalf("source after execution = %v, want %v", got, SrcCommand)
	}
}

func TestExecuteWithSourceUsesProvidedSource(t *testing.T) {
	c := NewCmdSystem()

	var seen CommandSource
	c.AddClientCommand("capture", func(args []string) {
		seen = c.Source()
	}, "")

	c.AddText("capture")
	c.ExecuteWithSource(SrcClient)

	if seen != SrcClient {
		t.Fatalf("source in buffered execute = %v, want %v", seen, SrcClient)
	}
}

func TestParseCommandStripsComments(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"sv_gravity 800 // normal gravity", []string{"sv_gravity", "800"}},
		{"// full line comment", nil},
		{"echo \"hello // world\"", []string{"echo", "hello // world"}},
		{"cmd arg1 arg2//attached", []string{"cmd", "arg1", "arg2"}},
	}
	for _, tc := range tests {
		got := parseCommand(tc.line)
		if len(got) != len(tc.want) {
			t.Errorf("parseCommand(%q) = %v, want %v", tc.line, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseCommand(%q)[%d] = %q, want %q", tc.line, i, got[i], tc.want[i])
			}
		}
	}
}

func TestForwardFuncCalledForUnknownCommands(t *testing.T) {
	cs := NewCmdSystem()
	var forwarded string
	cs.ForwardFunc = func(line string) {
		forwarded = line
	}
	cs.ExecuteText("unknowncmd arg1 arg2")
	if forwarded != "unknowncmd arg1 arg2" {
		t.Fatalf("ForwardFunc got %q, want %q", forwarded, "unknowncmd arg1 arg2")
	}
}

func TestForwardFuncNotCalledForKnownCommands(t *testing.T) {
	cs := NewCmdSystem()
	called := false
	cs.ForwardFunc = func(line string) {
		called = true
	}
	cs.AddCommand("known", func(args []string) {}, "test")
	cs.ExecuteText("known arg1")
	if called {
		t.Fatal("ForwardFunc should not be called for known commands")
	}
}

func TestClientSourceOnlyExecutesClientCommands(t *testing.T) {
	c := NewCmdSystem()
	executed := false
	c.AddCommand("localonly", func(args []string) {
		executed = true
	}, "")

	c.ExecuteTextWithSource("localonly", SrcClient)

	if executed {
		t.Fatal("src_client should not execute regular console commands")
	}
}

func TestServerSourceOnlyExecutesServerCommands(t *testing.T) {
	c := NewCmdSystem()
	executed := false
	c.AddClientCommand("clientonly", func(args []string) {
		executed = true
	}, "")

	c.ExecuteTextWithSource("clientonly", SrcServer)

	if executed {
		t.Fatal("src_server should not execute client commands")
	}
}

func TestUnknownClientSourceDoesNotExpandAliasOrForward(t *testing.T) {
	c := NewCmdSystem()
	c.AddAlias("boom", "echo no")

	expanded := false
	c.AddCommand("echo", func(args []string) {
		expanded = true
	}, "")
	defer c.RemoveCommand("echo")

	forwarded := false
	c.ForwardFunc = func(line string) {
		forwarded = true
	}

	c.ExecuteTextWithSource("boom", SrcClient)

	if expanded {
		t.Fatal("src_client should not expand aliases")
	}
	if forwarded {
		t.Fatal("src_client should not forward unknown commands")
	}
}
