package console

import "testing"

func TestExtractPartialSingleToken(t *testing.T) {
	if got := extractPartial("tog"); got != "tog" {
		t.Fatalf("extractPartial(%q) = %q, want %q", "tog", got, "tog")
	}
}

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
