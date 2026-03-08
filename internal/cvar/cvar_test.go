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
