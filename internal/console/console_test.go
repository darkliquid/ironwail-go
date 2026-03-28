package console

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

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

func TestConsoleWordWrapAtWordBoundary(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	c.Resize(10)
	c.Printf("12345 67890")
	if got := strings.TrimSpace(c.GetLine(c.CurrentLine() - 1)); got != "12345" {
		t.Fatalf("first wrapped line = %q, want %q", got, "12345")
	}
	if got := strings.TrimSpace(c.GetLine(c.CurrentLine())); got != "67890" {
		t.Fatalf("second wrapped line = %q, want %q", got, "67890")
	}
}

func TestQuakeBarSuppressesNewlineAtFullWidth(t *testing.T) {
	if err := InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	Resize(8)
	bar := QuakeBar(8)
	if strings.HasSuffix(bar, "\n") {
		t.Fatalf("QuakeBar(8) should not end with newline at full width")
	}
}

func TestDPrintf2RequiresDeveloperLevel2(t *testing.T) {
	cvar.Register("developer", "0", 0, "developer mode")
	if err := InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	Clear()
	cvar.Set("developer", "1")
	DPrintf2("hidden\n")
	if strings.Contains(GetLine(CurrentLine()), "hidden") {
		t.Fatalf("DPrintf2 printed with developer=1")
	}
	cvar.Set("developer", "2")
	DPrintf2("visible\n")
	if !strings.Contains(GetLine(CurrentLine()), "visible") {
		t.Fatalf("DPrintf2 did not print with developer=2")
	}
}

func TestLogCenterPrintDedupesAndGatesByGameType(t *testing.T) {
	cvar.Register("con_logcenterprint", "1", cvar.FlagArchive, "centerprint logging mode")
	if err := InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	Clear()
	LogCenterPrint(1, "coop message")
	if strings.Contains(GetLine(CurrentLine()), "coop message") {
		t.Fatalf("message should be gated in multiplayer for mode 1")
	}

	cvar.Set("con_logcenterprint", "2")
	LogCenterPrint(1, "dm message")
	foundDM := false
	foundMessage := false
	for line := CurrentLine() - 10; line <= CurrentLine(); line++ {
		text := GetLine(line)
		foundDM = foundDM || strings.Contains(text, "dm")
		foundMessage = foundMessage || strings.Contains(text, "message")
	}
	if !foundDM || !foundMessage {
		t.Fatalf("expected centerprint message in console output")
	}
	before := CurrentLine()
	LogCenterPrint(1, "dm message")
	if CurrentLine() != before {
		t.Fatalf("duplicate centerprint should not append output")
	}
}

func TestConsoleCursorEditingAndHistoryRestore(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	c.SetInputLine("hello world")
	c.DeleteWordLeft()
	if got := c.InputLine(); got != "hello " {
		t.Fatalf("DeleteWordLeft result = %q, want %q", got, "hello ")
	}

	c.SetInputLine("first")
	c.CommitInput()
	c.SetInputLine("draft")
	if got := c.PreviousHistory(); got != "first" {
		t.Fatalf("PreviousHistory = %q, want first", got)
	}
	if got := c.NextHistory(); got != "draft" {
		t.Fatalf("NextHistory restore = %q, want draft", got)
	}
}
