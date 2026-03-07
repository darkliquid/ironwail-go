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
