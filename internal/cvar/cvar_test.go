package cvar

import (
	"fmt"
	"testing"
	"time"
)

func TestSetCallbackCanReadUpdatedValue(t *testing.T) {
	sys := NewCVarSystem()
	cv := sys.Register("test_callback", "0", FlagArchive, "callback test")

	callbackErr := make(chan error, 1)
	cv.Callback = func(got *CVar) {
		if got.Int != 7 {
			callbackErr <- fmt.Errorf("callback value = %d, want 7", got.Int)
			return
		}
		if current := sys.IntValue("test_callback"); current != 7 {
			callbackErr <- fmt.Errorf("callback readback = %d, want 7", current)
			return
		}
		callbackErr <- nil
	}

	setDone := make(chan struct{})
	go func() {
		sys.Set("test_callback", "7")
		close(setDone)
	}()

	select {
	case <-setDone:
	case <-time.After(time.Second):
		t.Fatal("Set blocked while running callback")
	}

	select {
	case err := <-callbackErr:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("expected callback to run")
	}
}

func TestFlagROM(t *testing.T) {
	sys := NewCVarSystem()
	cv := sys.Register("test_rom", "42", FlagROM, "read-only test")

	if cv.Int != 42 {
		t.Errorf("initial value = %d, want 42", cv.Int)
	}

	sys.Set("test_rom", "100")

	if cv.Int != 42 {
		t.Errorf("ROM cvar was modified: got %d, want 42", cv.Int)
	}

	if sys.IntValue("test_rom") != 42 {
		t.Errorf("ROM cvar readback = %d, want 42", sys.IntValue("test_rom"))
	}
}

func TestLockedCvarRejectsSet(t *testing.T) {
	sys := NewCVarSystem()
	sys.Register("test_lock", "10", FlagNone, "lockable cvar")

	sys.LockVar("test_lock")
	sys.Set("test_lock", "20")
	if sys.StringValue("test_lock") != "10" {
		t.Fatalf("locked cvar changed to %q, want 10", sys.StringValue("test_lock"))
	}

	sys.UnlockVar("test_lock")
	sys.Set("test_lock", "20")
	if sys.StringValue("test_lock") != "20" {
		t.Fatalf("unlocked cvar = %q, want 20", sys.StringValue("test_lock"))
	}
}

func TestAutoCvarCallback(t *testing.T) {
	sys := NewCVarSystem()
	sys.Register("sv_gravity", "800", FlagAutoCvar, "gravity")

	var calledWith string
	sys.AutoCvarChanged = func(cv *CVar) {
		calledWith = cv.String
	}

	sys.Set("sv_gravity", "400")
	if calledWith != "400" {
		t.Fatalf("autocvar callback got %q, want 400", calledWith)
	}

	// Non-autocvar cvar should not trigger the callback.
	sys.Register("sv_speed", "320", 0, "speed")
	calledWith = ""
	sys.Set("sv_speed", "640")
	if calledWith != "" {
		t.Fatalf("non-autocvar triggered callback with %q", calledWith)
	}
}
