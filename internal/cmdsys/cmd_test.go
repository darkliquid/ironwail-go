package cmdsys

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// TestParseCommandPreservesEscapedQuotesAndBackslashes tests command line tokenization with escape characters.
// It ensures that complex command strings (like those in bind or alias) are parsed correctly.
// Where in C: Cmd_TokenizeString in cmd.c
func TestParseCommandPreservesEscapedQuotesAndBackslashes(t *testing.T) {
	args := parseCommand(`bind t "say He said \"hello\" \\world\nnext\tline"`)
	want := []string{"bind", "t", "say He said \"hello\" \\world\nnext\tline"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("parseCommand returned %v, want %v", args, want)
	}
}

func TestParseCommandTreatsTabsAsArgumentSeparators(t *testing.T) {
	args := parseCommand("bind\tPAUSE\tpause")
	want := []string{"bind", "PAUSE", "pause"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("parseCommand returned %v, want %v", args, want)
	}
}

// TestCommandTakesPrecedenceOverAlias tests that built-in commands take precedence over aliases with the same name.
// It maintains the standard command resolution order where engine commands cannot be overridden by user aliases.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestAliasExecutesUnderlyingCommandText tests that an alias correctly expands to its defined command text.
// It ensures basic alias functionality works as expected for user-defined shortcuts.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestExecuteTextSplitsSemicolonSeparatedCommands tests that semicolons are correctly used as command separators.
// It allows multiple commands to be executed from a single input line.
// Where in C: Cmd_ExecuteString in cmd.c
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

func TestExecuteTextPreservesSemicolonsInsideQuotedArguments(t *testing.T) {
	c := NewCmdSystem()

	var (
		bindings [][]string
		waitRan  bool
		saveRan  bool
	)
	c.AddCommand("bind", func(args []string) {
		bindings = append(bindings, append([]string(nil), args...))
	}, "")
	c.AddCommand("wait", func(args []string) {
		waitRan = true
	}, "")
	c.AddCommand("save", func(args []string) {
		saveRan = true
	}, "")

	c.ExecuteText("bind F6 \"echo Quicksaving...; wait; save quick\"\nbind F10 quit\n")

	want := [][]string{
		{"F6", "echo Quicksaving...; wait; save quick"},
		{"F10", "quit"},
	}
	if !reflect.DeepEqual(bindings, want) {
		t.Fatalf("bind commands = %v, want %v", bindings, want)
	}
	if waitRan {
		t.Fatal("quoted semicolon unexpectedly executed wait command")
	}
	if saveRan {
		t.Fatal("quoted semicolon unexpectedly executed save command")
	}
}

// TestAliasExecutesSemicolonSeparatedCommandText tests that aliases containing multiple semicolon-separated commands are executed correctly.
// It supports complex multi-command aliases.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestRecursiveAliasStopsAtActiveExpansion tests recursion depth protection for aliases.
// It prevents infinite loops and engine crashes from recursive alias definitions.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestWaitCommandDefersRemainingCommands tests the wait command's ability to pause command execution.
// It allows scripts to delay execution until the next frame/execution cycle.
// Where in C: Cmd_ExecuteString and Cmd_Wait_f in cmd.c
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

// TestWaitCommandWithExistingBufferContent tests wait behavior when additional commands are added to the buffer.
// It ensures command ordering is preserved when execution is deferred.
// Where in C: Cbuf_AddText and Cbuf_Execute in cmd.c
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

// TestWaitCommandAtEnd tests wait at the end of a command string.
// It ensures it doesn't cause issues when there are no more commands to defer.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestCommandSourceDefaultsToSrcCommand tests the default command execution source.
// It ensures that commands originate from the console/config by default.
// Where in C: cmd_source global in cmd.c
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

// TestExecuteTextWithSourceSetsSourceForHandler tests that the command source is correctly passed to the command handler.
// It allows handlers to distinguish between local, client, and server sources for security and logic.
// Where in C: Cmd_ExecuteString and cmd_source in cmd.c
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

func TestExecuteTextPrintsQueriedCVarValue(t *testing.T) {
	c := NewCmdSystem()
	cv := cvar.Register("test_cmdsys_query_cvar", "42", cvar.FlagNone, "test")
	defer cvar.Set(cv.Name, cv.DefaultValue)

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText(cv.Name)

	if printed != "\"test_cmdsys_query_cvar\" is \"42\"\n" {
		t.Fatalf("printed = %q, want queried cvar output", printed)
	}
}

func TestExecuteTextPrintsUnknownCommandWithoutForwarder(t *testing.T) {
	c := NewCmdSystem()

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("definitely_unknown_command")

	if printed != "Unknown command \"definitely_unknown_command\"\n" {
		t.Fatalf("printed = %q, want unknown command output", printed)
	}
}

// TestExecuteWithSourceUsesProvidedSource tests ExecuteWithSource with buffered commands.
// It ensures the source is correctly tracked through the command buffer.
// Where in C: Cbuf_Execute and cmd_source in cmd.c
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

// TestInsertTextDuringExecutePreemptsRemainingCommands verifies Quake-style
// command buffer semantics: InsertText from a running command executes before
// later lines from the current buffer.
func TestInsertTextDuringExecutePreemptsRemainingCommands(t *testing.T) {
	c := NewCmdSystem()

	var executed []string
	c.AddCommand("stuffcmds", func(args []string) {
		executed = append(executed, "stuffcmds")
		c.InsertText("map e2m2")
	}, "")
	c.AddCommand("map", func(args []string) {
		executed = append(executed, "map "+args[0])
	}, "")
	c.AddCommand("startdemos", func(args []string) {
		executed = append(executed, "startdemos "+args[0])
	}, "")

	c.AddText("stuffcmds\nstartdemos demo1\n")
	c.Execute()

	want := []string{"stuffcmds", "map e2m2", "startdemos demo1"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("execution order = %v, want %v", executed, want)
	}
}

// TestParseCommandStripsComments tests that comments (starting with //) are stripped from command lines.
// It allows users to add comments to their config files without affecting execution.
// Where in C: Cmd_TokenizeString in cmd.c
func TestParseCommandStripsComments(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"sv_gravity 800 // normal gravity", []string{"sv_gravity", "800"}},
		{"// full line comment", nil},
		{"/* full line block comment */", nil},
		{"echo \"hello // world\"", []string{"echo", "hello // world"}},
		{"echo \"hello /* world */\"", []string{"echo", "hello /* world */"}},
		{"cmd arg1 arg2//attached", []string{"cmd", "arg1", "arg2"}},
		{"cmd arg1 /* block */ arg2", []string{"cmd", "arg1", "arg2"}},
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

// TestExecuteTextDoesNotSplitSemicolonsInsideComments verifies that once a //
// comment begins, semicolons are ignored until the next newline.
// Where in C: Cbuf_Execute and Cmd_TokenizeString in cmd.c
func TestExecuteTextDoesNotSplitSemicolonsInsideComments(t *testing.T) {
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

	c.ExecuteText("first // comment; second\nthird")

	want := []string{"first", "third"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("executed = %v, want %v", executed, want)
	}
}

func TestExecuteTextDoesNotSplitSemicolonsInsideBlockComments(t *testing.T) {
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

	c.ExecuteText("first /* comment; second */\nthird")

	want := []string{"first", "third"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("executed = %v, want %v", executed, want)
	}
}

// TestForwardFuncCalledForUnknownCommands tests the command forwarding mechanism for unrecognized commands.
// It allows the engine to pass unknown commands to other systems (like the server) for handling.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestForwardFuncNotCalledForKnownCommands tests that known commands are not forwarded.
// It ensures local commands are handled correctly before attempting to forward.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestClientSourceOnlyExecutesClientCommands tests source-based execution filtering for client commands.
// It restricts certain commands to specific sources for security and protocol adherence.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestServerSourceOnlyExecutesServerCommands tests source-based execution filtering for server commands.
// It prevents clients from executing restricted server-side commands.
// Where in C: Cmd_ExecuteString in cmd.c
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

// TestUnknownClientSourceDoesNotExpandAliasOrForward tests that unknown client sources have restricted capabilities.
// It prevents unauthorized alias expansion or command forwarding from external sources.
// Where in C: Cmd_ExecuteString in cmd.c
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

func TestCmdListListsVisibleCommandsByPrefix(t *testing.T) {
	c := NewCmdSystem()
	c.AddCommand("alpha", func(args []string) {}, "alpha command")
	c.AddServerCommand("alphaserver", func(args []string) {}, "server only")
	c.AddCommand("__alphareserved", func(args []string) {}, "reserved")

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("cmdlist alpha")

	if !strings.Contains(printed, "   alpha\n") {
		t.Fatalf("cmdlist output missing visible command:\n%s", printed)
	}
	if strings.Contains(printed, "alphaserver") {
		t.Fatalf("cmdlist should not show server-only commands:\n%s", printed)
	}
	if strings.Contains(printed, "__alphareserved") {
		t.Fatalf("cmdlist should not show reserved commands:\n%s", printed)
	}
	if !strings.Contains(printed, "1 commands beginning with \"alpha\"\n") {
		t.Fatalf("cmdlist summary mismatch:\n%s", printed)
	}
}

func TestFindSearchesCommandsAndCvars(t *testing.T) {
	c := NewCmdSystem()
	c.AddCommand("zoommode", func(args []string) {}, "camera size controls")
	cv := cvar.Register("crosshair_size_test", "3", cvar.FlagNone, "crosshair size")
	t.Cleanup(func() {
		cvar.Set(cv.Name, cv.DefaultValue)
	})

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("find size")

	if !strings.Contains(printed, "   zoommode\n") {
		t.Fatalf("find output missing command hit:\n%s", printed)
	}
	if !strings.Contains(printed, "   crosshair_size_test (current value: \"3\")\n") {
		t.Fatalf("find output missing cvar hit:\n%s", printed)
	}
	if !strings.Contains(printed, "2 cvars/commands containing \"size\"\n") {
		t.Fatalf("find summary mismatch:\n%s", printed)
	}
}

func TestAproposPrintsUsageWithoutSubstring(t *testing.T) {
	c := NewCmdSystem()

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("apropos")

	if printed != "apropos <substring> : search through commands and cvars for the given substring\n" {
		t.Fatalf("apropos usage = %q", printed)
	}
}

func TestAliasListPrintsZeroCountWhenEmpty(t *testing.T) {
	c := NewCmdSystem()

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("aliaslist")

	if printed != "0 aliases\n" {
		t.Fatalf("aliaslist empty output = %q", printed)
	}
}

func TestAliasListPrintsAliasesAlphabetically(t *testing.T) {
	c := NewCmdSystem()
	c.AddAlias("zoom", "fov 110")
	c.AddAlias("attack", "+attack")
	c.AddAlias("rocketjump", "+jump;wait;+attack")

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("aliaslist")

	wantOrder := []string{
		"   attack : +attack\n",
		"   rocketjump : +jump;wait;+attack\n",
		"   zoom : fov 110\n",
	}
	pos := 0
	for _, want := range wantOrder {
		idx := strings.Index(printed[pos:], want)
		if idx < 0 {
			t.Fatalf("aliaslist output missing %q in:\n%s", want, printed)
		}
		pos += idx + len(want)
	}
}

func TestAliasListCountMatchesDefinedAliases(t *testing.T) {
	c := NewCmdSystem()
	c.AddAlias("foo", "echo foo")
	c.AddAlias("bar", "echo bar")

	var printed string
	SetPrintCallback(func(msg string) {
		printed += msg
	})
	t.Cleanup(func() {
		SetPrintCallback(nil)
	})

	c.ExecuteText("aliaslist")

	if !strings.HasSuffix(printed, "2 aliases\n") {
		t.Fatalf("aliaslist count mismatch:\n%s", printed)
	}
}
