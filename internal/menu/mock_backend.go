package menu

import (
	"github.com/darkliquid/ironwail-go/internal/input"
)

// mockInputBackend is a mock implementation of input.Backend for testing.
type mockInputBackend struct{}

func (m *mockInputBackend) Init() error {
	return nil
}

func (m *mockInputBackend) Shutdown() {
}

func (m *mockInputBackend) PollEvents() bool {
	return true
}

func (m *mockInputBackend) GetMouseDelta() (dx, dy int32) {
	return 0, 0
}

func (m *mockInputBackend) GetMousePosition() (x, y int32, valid bool) {
	return 0, 0, false
}

func (m *mockInputBackend) GetModifierState() input.ModifierState {
	return input.ModifierState{}
}

func (m *mockInputBackend) SetTextMode(mode input.TextMode) {
}

func (m *mockInputBackend) SetCursorMode(mode input.CursorMode) {
}

func (m *mockInputBackend) ShowKeyboard(show bool) {
}

func (m *mockInputBackend) GetGamepadState(player int) input.GamepadState {
	return input.GamepadState{}
}

func (m *mockInputBackend) IsGamepadConnected(player int) bool {
	return false
}

func (m *mockInputBackend) SetMouseGrab(grabbed bool) {
}

func (m *mockInputBackend) SetWindow(win interface{}) {}
