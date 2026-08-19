package console

import (
	"testing"
)

func TestConsoleAccessors_SnapshotFull(t *testing.T) {
	c := &Console{
		Title: "Test Console",
	}
	_ = c.Init(1024)
	c.Printf("Line 1\n")
	c.Printf("Line 2\n")
	c.Printf("Line 3\n")
	c.SetInputLine("test command")

	snap := c.SnapshotFull(5)
	if snap.InputLine != "test command" {
		t.Fatalf("SnapshotFull.InputLine = %q, want test command", snap.InputLine)
	}
	if snap.Title != "Test Console" {
		t.Fatalf("SnapshotFull.Title = %q, want Test Console", snap.Title)
	}
	if len(snap.Lines) != 5 {
		t.Fatalf("SnapshotFull.Lines length = %d, want 5", len(snap.Lines))
	}
}

func TestConsoleAccessors_SnapshotNotify(t *testing.T) {
	c := &Console{}
	_ = c.Init(1024)
	c.Printf("Notify 1\n")
	c.Printf("Notify 2\n")

	notifies := c.SnapshotNotify()
	if len(notifies) == 0 {
		t.Fatal("expected active notify lines in SnapshotNotify")
	}
	if notifies[len(notifies)-1].Alpha <= 0 {
		t.Fatalf("expected positive alpha, got %f", notifies[len(notifies)-1].Alpha)
	}
}
