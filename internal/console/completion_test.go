package console

import "testing"

// TestExtractPartialSingleToken tests extraction of the current token for completion.
// It providing the foundation for tab-completion by identifying what the user is currently typing.
// Where in C: Con_CompleteCommand in console.c (or similar input handling)
func TestExtractPartialSingleToken(t *testing.T) {
	if got := extractPartial("tog"); got != "tog" {
		t.Fatalf("extractPartial(%q) = %q, want %q", "tog", got, "tog")
	}
}

// TestTabCompleterCompletesCurrentToken tests command tab-completion.
// It providing a standard Quake UX where users can complete command names by pressing Tab.
// Where in C: Con_CompleteCommand in console.c
func TestTabCompleterCompletesCurrentToken(t *testing.T) {
	tc := NewTabCompleter()
	tc.SetCommandProvider(func(partial string) []string {
		if partial == "tog" {
			return []string{"toggleconsole"}
		}
		return nil
	})

	got, matches := tc.Complete("tog", true)
	if got != "toggleconsole" {
		t.Fatalf("Complete(%q) = %q, want %q", "tog", got, "toggleconsole")
	}
	if len(matches) != 1 || matches[0] != "toggleconsole (command)" {
		t.Fatalf("matches = %v, want [toggleconsole (command)]", matches)
	}
}

// TestTabCompleterIncludesAliases tests that aliases are included in tab-completion.
// It ensuring user-defined shortcuts are as discoverable as built-in commands.
// Where in C: Con_CompleteCommand in console.c
func TestTabCompleterIncludesAliases(t *testing.T) {
	tc := NewTabCompleter()
	tc.SetAliasProvider(func(partial string) []string {
		if partial == "qa" {
			return []string{"qalias"}
		}
		return nil
	})

	got, matches := tc.Complete("qa", true)
	if got != "qalias" {
		t.Fatalf("Complete(%q) = %q, want %q", "qa", got, "qalias")
	}
	if len(matches) != 1 || matches[0] != "qalias (alias)" {
		t.Fatalf("matches = %v, want [qalias (alias)]", matches)
	}
}

func TestTabCompleterCompletesMapArgument(t *testing.T) {
	tc := NewTabCompleter()
	var gotPattern string
	tc.SetFileProvider(func(pattern string) []string {
		gotPattern = pattern
		return []string{"maps/e1m1.bsp"}
	})

	got, matches := tc.Complete("map e1", true)
	if gotPattern != "maps/*.bsp" {
		t.Fatalf("file provider pattern = %q, want %q", gotPattern, "maps/*.bsp")
	}
	if got != "map e1m1" {
		t.Fatalf("Complete(%q) = %q, want %q", "map e1", got, "map e1m1")
	}
	if len(matches) != 1 || matches[0] != "e1m1 (map)" {
		t.Fatalf("matches = %v, want [e1m1 (map)]", matches)
	}
}

func TestTabCompleterCompletesExecArgument(t *testing.T) {
	tc := NewTabCompleter()
	tc.SetFileProvider(func(pattern string) []string {
		if pattern != "*.cfg" {
			t.Fatalf("file provider pattern = %q, want %q", pattern, "*.cfg")
		}
		return []string{"autoexec.cfg"}
	})

	got, matches := tc.Complete("exec auto", true)
	if got != "exec autoexec.cfg" {
		t.Fatalf("Complete(%q) = %q, want %q", "exec auto", got, "exec autoexec.cfg")
	}
	if len(matches) != 1 || matches[0] != "autoexec.cfg (config)" {
		t.Fatalf("matches = %v, want [autoexec.cfg (config)]", matches)
	}
}
