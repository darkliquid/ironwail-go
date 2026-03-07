package console

import "testing"

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
