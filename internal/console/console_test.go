package console

import "testing"

// TestConsoleInputHistory tests the console's input buffer and history management.
// It ensures users can navigate through previous commands using up/down keys, matching standard Quake console behavior.
// Where in C: Con_Init and related input handling in console.c
func TestConsoleInputHistory(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.AppendInputRune('t')
	c.AppendInputRune('e')
	c.AppendInputRune('s')
	c.AppendInputRune('t')
	if got := c.InputLine(); got != "test" {
		t.Fatalf("InputLine() = %q, want %q", got, "test")
	}

	if got := c.CommitInput(); got != "test" {
		t.Fatalf("CommitInput() = %q, want %q", got, "test")
	}
	if got := c.InputLine(); got != "" {
		t.Fatalf("InputLine() after commit = %q, want empty", got)
	}

	c.AppendInputRune('n')
	c.AppendInputRune('e')
	c.AppendInputRune('x')
	c.AppendInputRune('t')
	c.CommitInput()

	if got := c.PreviousHistory(); got != "next" {
		t.Fatalf("PreviousHistory() first = %q, want %q", got, "next")
	}
	if got := c.PreviousHistory(); got != "test" {
		t.Fatalf("PreviousHistory() second = %q, want %q", got, "test")
	}
	if got := c.NextHistory(); got != "next" {
		t.Fatalf("NextHistory() = %q, want %q", got, "next")
	}
	if got := c.NextHistory(); got != "" {
		t.Fatalf("NextHistory() at end = %q, want empty", got)
	}
}

// TestConsoleBackspaceInput tests backspace functionality in the console.
// It providing basic text editing capabilities in the console input line.
// Where in C: Con_Key or similar in console.c
func TestConsoleBackspaceInput(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.AppendInputRune('g')
	c.AppendInputRune('o')
	c.BackspaceInput()
	if got := c.InputLine(); got != "g" {
		t.Fatalf("InputLine() after backspace = %q, want %q", got, "g")
	}
}
