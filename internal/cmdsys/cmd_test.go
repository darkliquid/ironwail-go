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

