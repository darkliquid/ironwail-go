package main

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/input"
)

type quitPollingBackend struct {
	pollResult    bool
	shutdownCalls int
}

func (b *quitPollingBackend) Init() error                            { return nil }
func (b *quitPollingBackend) Shutdown()                              { b.shutdownCalls++ }
func (b *quitPollingBackend) PollEvents() bool                       { return b.pollResult }
func (b *quitPollingBackend) GetMouseDelta() (int32, int32)          { return 0, 0 }
func (b *quitPollingBackend) GetMousePosition() (int32, int32, bool) { return 0, 0, false }
func (b *quitPollingBackend) GetModifierState() input.ModifierState  { return input.ModifierState{} }
func (b *quitPollingBackend) SetTextMode(input.TextMode)             {}
func (b *quitPollingBackend) SetCursorMode(input.CursorMode)         {}
func (b *quitPollingBackend) ShowKeyboard(bool)                      {}
func (b *quitPollingBackend) GetGamepadState(int) input.GamepadState {
	return input.GamepadState{}
}
func (b *quitPollingBackend) IsGamepadConnected(int) bool { return false }
func (b *quitPollingBackend) SetMouseGrab(bool)           {}
func (b *quitPollingBackend) SetWindow(interface{})       {}

func TestPollRuntimeInputEventsRequestsQuitOnWindowClose(t *testing.T) {
	prev := g
	t.Cleanup(func() { g = prev })

	g.Host = host.NewHost()
	g.Input = input.NewSystem(&quitPollingBackend{pollResult: false})

	pollRuntimeInputEvents()

	if !g.Host.IsAborted() {
		t.Fatal("pollRuntimeInputEvents did not request quit after backend close event")
	}
}
