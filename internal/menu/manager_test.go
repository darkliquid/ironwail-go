package menu

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
)

// mockDrawManager is a mock implementation of DrawManager for testing.
type mockDrawManager struct{}

func (m *mockDrawManager) GetPic(name string) *image.QPic {
	return nil
}

func TestNewManager(t *testing.T) {
	drawMgr := &mockDrawManager{}
	inputSys := input.NewSystem(nil)
	mgr := NewManager(drawMgr, inputSys)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.IsActive() {
		t.Error("Menu should not be active initially")
	}

	if mgr.GetState() != MenuNone {
		t.Error("Initial state should be MenuNone")
	}
}

func TestToggleMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	// Toggle menu on
	mgr.ToggleMenu()
	if !mgr.IsActive() {
		t.Error("Menu should be active after toggle")
	}
	if mgr.GetState() != MenuMain {
		t.Error("State should be MenuMain after toggle")
	}

	// Toggle menu off
	mgr.ToggleMenu()
	if mgr.IsActive() {
		t.Error("Menu should not be active after second toggle")
	}
	if mgr.GetState() != MenuNone {
		t.Error("State should be MenuNone after second toggle")
	}
}

func TestShowHideMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	// Show menu
	mgr.ShowMenu()
	if !mgr.IsActive() {
		t.Error("Menu should be active after ShowMenu")
	}

	// Hide menu
	mgr.HideMenu()
	if mgr.IsActive() {
		t.Error("Menu should not be active after HideMenu")
	}
}

func TestMainMenuKey(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	mgr.ShowMenu()

	// Test up arrow

	// Test up arrow
	// Test up arrow
	mgr.M_Key(input.KUpArrow)
	if mgr.mainCursor != 5 { // Should wrap to last item
		t.Error("Up arrow should wrap cursor to end")
	}

	// Test down arrow
	mgr.M_Key(input.KDownArrow)
	if mgr.mainCursor != 0 { // Should wrap to first item
		t.Error("Down arrow should wrap cursor to start")
	}

	// Test escape to close
	mgr.M_Key(input.KEscape)
	if mgr.IsActive() {
		t.Error("Escape should hide menu")
	}
}

func TestQuitMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	mgr.ShowMenu()

	// Navigate to quit (item 5)

	// Navigate to quit (item 5)
	mgr.M_Key(input.KDownArrow) // Item 1
	mgr.M_Key(input.KDownArrow) // Item 2
	mgr.M_Key(input.KDownArrow) // Item 3
	mgr.M_Key(input.KDownArrow) // Item 4
	mgr.M_Key(input.KDownArrow) // Item 5 (Quit)
	mgr.M_Key(input.KEnter)     // Enter to select quit

	if mgr.GetState() != MenuQuit {
		t.Error("State should be MenuQuit after selecting quit")
	}

	// Test N to cancel
	// Test Backspace to cancel
	mgr.M_Key(input.KBackspace)
	if mgr.GetState() != MenuMain {
		t.Error("Backspace should return to main menu")
	}
}
