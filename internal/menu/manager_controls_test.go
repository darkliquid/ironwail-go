package menu

// Help, options, controls, video, and audio menu tests split from manager_test.go.

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/input"
)

func TestHelpNavigation(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.ShowMenu()
	mgr.state = MenuHelp
	mgr.helpPage = 0

	mgr.M_Key(input.KRightArrow)
	if mgr.helpPage != 1 {
		t.Fatalf("expected help page 1, got %d", mgr.helpPage)
	}

	mgr.helpPage = helpPages - 1
	mgr.M_Key(input.KRightArrow)
	if mgr.helpPage != 0 {
		t.Fatalf("expected help page wrap to 0, got %d", mgr.helpPage)
	}

	mgr.M_Key(input.KEscape)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected return to main menu, got %v", mgr.GetState())
	}
}

func TestOptionsNavigationAndAction(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.cvars.Register("vid_vsync", "1", cvar.FlagArchive, "Vertical sync")
	mgr.cvars.Set("vid_vsync", "1")

	mgr.ShowMenu()
	mgr.state = MenuOptions
	mgr.optionsCursor = 0 // CONTROLS
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuControls {
		t.Fatalf("expected controls menu, got %v", got)
	}

	mgr.controlsCursor = controlItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from controls, got %v", got)
	}

	mgr.optionsCursor = 1 // VIDEO
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuVideo {
		t.Fatalf("expected video menu, got %v", got)
	}

	mgr.M_Key(input.KEscape)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from video, got %v", got)
	}

	mgr.optionsCursor = 2 // AUDIO
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuAudio {
		t.Fatalf("expected audio menu, got %v", got)
	}

	mgr.M_Key(input.KBackspace)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from audio, got %v", got)
	}

	mgr.optionsCursor = 3 // VSYNC
	mgr.M_Key(input.KEnter)
	if mgr.cvars.BoolValue("vid_vsync") {
		t.Fatal("expected options vsync toggle to set cvar off")
	}

	mgr.optionsCursor = 4 // Back
	mgr.M_Key(input.KEnter)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected back to main menu, got %v", mgr.GetState())
	}
}

func TestControlsMenuRebindingAndClearing(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	inputSys.SetBinding(int('w'), "+forward")
	inputSys.SetBinding(input.KUpArrow, "+forward")

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemForward
	mgr.M_Key(input.KEnter)
	if !mgr.controlsRebinding {
		t.Fatal("expected controls menu to enter rebinding mode")
	}
	mgr.M_Key(int('i'))
	if mgr.controlsRebinding {
		t.Fatal("expected controls menu to exit rebinding mode after key selection")
	}
	if got := inputSys.GetBinding(int('i')); got != "+forward" {
		t.Fatalf("binding for i = %q, want +forward", got)
	}
	if got := inputSys.GetBinding(int('w')); got != "" {
		t.Fatalf("binding for w should be cleared by menu rebind, got %q", got)
	}
	if got := inputSys.GetBinding(input.KUpArrow); got != "" {
		t.Fatalf("binding for UPARROW should be cleared by menu rebind, got %q", got)
	}

	mgr.M_Key(input.KLeftArrow)
	if got := inputSys.GetBinding(int('i')); got != "" {
		t.Fatalf("binding for i should be cleared by menu clear action, got %q", got)
	}
}

func TestControlsMenuCancelRebinding(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	inputSys.SetBinding(input.KMouse1, "+attack")

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemAttack
	mgr.M_Key(input.KEnter)
	mgr.M_Key(input.KEscape)
	if mgr.controlsRebinding {
		t.Fatal("expected rebinding mode to cancel on escape")
	}
	if got := inputSys.GetBinding(input.KMouse1); got != "+attack" {
		t.Fatalf("attack binding should be unchanged after cancel, got %q", got)
	}
}

func TestControlsMenuCanBindBackquote(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.state = MenuControls
	mgr.active = true
	mgr.controlsCursor = controlItemToggleConsole
	mgr.M_Key(input.KEnter)
	if !mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls menu to enter rebinding mode")
	}
	mgr.M_Key(int('`'))
	if mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls menu to exit rebinding mode after backquote selection")
	}
	if got := inputSys.GetBinding(int('`')); got != "toggleconsole" {
		t.Fatalf("binding for backquote = %q, want toggleconsole", got)
	}
}

func TestControlsMenuCursorWrapWithExpandedBindings(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemBack
	mgr.M_Key(input.KDownArrow)
	if got := mgr.controlsCursor; got != controlItemMouseSpeed {
		t.Fatalf("down from back should wrap to first row, got %d", got)
	}

	mgr.controlsCursor = controlItemMouseSpeed
	mgr.M_Key(input.KUpArrow)
	if got := mgr.controlsCursor; got != controlItemBack {
		t.Fatalf("up from first row should wrap to back, got %d", got)
	}
}

func TestControlsMenuLabelForNewCommand(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	if got := mgr.controlBindingLabel(controlItemCenterView); got != "UNBOUND" {
		t.Fatalf("centerview label when unbound = %q, want UNBOUND", got)
	}

	inputSys.SetBinding(int('v'), "centerview")
	if got := mgr.controlBindingLabel(controlItemCenterView); got != "v" {
		t.Fatalf("centerview label with one key = %q, want v", got)
	}
}

func TestControlsMenuRebindAndClearNewCommand(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	inputSys.SetBinding(int('z'), "centerview")

	mgr.state = MenuControls
	mgr.active = true
	mgr.controlsCursor = controlItemCenterView
	mgr.M_Key(input.KEnter)
	if !mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls menu to enter rebinding mode")
	}
	mgr.M_Key(int('v'))
	if mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls menu to exit rebinding mode after key selection")
	}
	if got := inputSys.GetBinding(int('v')); got != "centerview" {
		t.Fatalf("binding for v = %q, want centerview", got)
	}
	if got := inputSys.GetBinding(int('z')); got != "" {
		t.Fatalf("binding for z should be cleared by menu rebind, got %q", got)
	}

	mgr.M_Key(input.KLeftArrow)
	if got := inputSys.GetBinding(int('v')); got != "" {
		t.Fatalf("binding for v should be cleared by menu clear action, got %q", got)
	}
}

func TestControlsMenuAdjustsLiveControlCvars(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.cvars.Register("sensitivity", "6.8", cvar.FlagArchive, "Mouse sensitivity")
	mgr.cvars.Register("m_pitch", "0.0176", cvar.FlagArchive, "Mouse pitch scale")
	mgr.cvars.Register("cl_alwaysrun", "1", cvar.FlagArchive, "Always run")
	mgr.cvars.Register("freelook", "1", cvar.FlagArchive, "Freelook")
	mgr.cvars.Set("sensitivity", "6.8")
	mgr.cvars.Set("m_pitch", "0.0176")
	mgr.cvars.Set("cl_alwaysrun", "1")
	mgr.cvars.Set("freelook", "1")

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemMouseSpeed
	mgr.M_Key(input.KRightArrow)
	if got := mgr.cvars.FloatValue("sensitivity"); math.Abs(got-7.3) > 0.001 {
		t.Fatalf("sensitivity = %.1f, want 7.3", got)
	}

	mgr.controlsCursor = controlItemInvertMouse
	mgr.M_Key(input.KEnter)
	if got := mgr.cvars.FloatValue("m_pitch"); math.Abs(got-(-0.0176)) > 0.0001 {
		t.Fatalf("m_pitch = %.4f, want -0.0176", got)
	}

	mgr.controlsCursor = controlItemAlwaysRun
	mgr.M_Key(input.KEnter)
	if mgr.cvars.BoolValue("cl_alwaysrun") {
		t.Fatalf("expected cl_alwaysrun toggled off")
	}

	mgr.controlsCursor = controlItemFreeLook
	mgr.M_Key(input.KLeftArrow)
	if mgr.cvars.BoolValue("freelook") {
		t.Fatalf("expected freelook toggled off")
	}

	if mgr.controlsRebinding {
		t.Fatalf("settings rows should not enter rebinding mode")
	}

	mgr.controlsCursor = controlItemMouseSpeed
	mgr.M_Key(input.KBackspace)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("settings-row backspace should return to options, got %v", got)
	}
}

func TestVideoMenuAdjustmentsWriteCvars(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.cvars.Register("vid_width", "1280", cvar.FlagArchive, "Video width")
	mgr.cvars.Register("vid_height", "720", cvar.FlagArchive, "Video height")
	mgr.cvars.Register("vid_fullscreen", "0", cvar.FlagArchive, "Fullscreen mode")
	mgr.cvars.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum frames per second")
	mgr.cvars.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	mgr.cvars.Register("r_drawviewmodel", "1", cvar.FlagArchive, "Draw first-person viewmodel")
	mgr.cvars.Register("scr_showfps", "0", cvar.FlagArchive, "Show FPS counter")
	mgr.cvars.Register("scr_showspeed", "0", cvar.FlagArchive, "Show speed overlay")
	mgr.cvars.Register("scr_clock", "0", cvar.FlagArchive, "Show level clock")
	mgr.cvars.Set("vid_width", "1280")
	mgr.cvars.Set("vid_height", "720")
	mgr.cvars.Set("vid_fullscreen", "0")
	mgr.cvars.Set("host_maxfps", "250")
	mgr.cvars.Set("r_gamma", "1.0")
	mgr.cvars.Set("r_drawviewmodel", "1")
	mgr.cvars.Set("scr_showfps", "0")
	mgr.cvars.Set("scr_showspeed", "0")
	mgr.cvars.Set("scr_clock", "0")

	mgr.state = MenuVideo
	mgr.videoCursor = videoItemResolution
	mgr.M_Key(input.KRightArrow)
	if gotW, gotH := mgr.cvars.IntValue("vid_width"), mgr.cvars.IntValue("vid_height"); gotW != 1366 || gotH != 768 {
		t.Fatalf("resolution cvars = %dx%d, want 1366x768", gotW, gotH)
	}

	mgr.videoCursor = videoItemFullscreen
	mgr.M_Key(input.KEnter)
	if !mgr.cvars.BoolValue("vid_fullscreen") {
		t.Fatal("fullscreen toggle did not update cvar")
	}

	mgr.videoCursor = videoItemMaxFPS
	mgr.M_Key(input.KLeftArrow)
	if got := mgr.cvars.IntValue("host_maxfps"); got != 240 {
		t.Fatalf("host_maxfps = %d, want 240", got)
	}

	mgr.videoCursor = videoItemGamma
	mgr.M_Key(input.KRightArrow)
	if got := mgr.cvars.FloatValue("r_gamma"); got != 1.1 {
		t.Fatalf("r_gamma = %.1f, want 1.1", got)
	}

	mgr.videoCursor = videoItemViewModel
	mgr.M_Key(input.KEnter)
	if mgr.cvars.BoolValue("r_drawviewmodel") {
		t.Fatal("viewmodel toggle did not update cvar")
	}

	mgr.videoCursor = videoItemShowFPS
	mgr.M_Key(input.KEnter)
	if !mgr.cvars.BoolValue("scr_showfps") {
		t.Fatal("showfps toggle did not update cvar")
	}

	mgr.videoCursor = videoItemShowSpeed
	mgr.M_Key(input.KEnter)
	if !mgr.cvars.BoolValue("scr_showspeed") {
		t.Fatal("showspeed toggle did not update cvar")
	}

	mgr.videoCursor = videoItemShowTime
	mgr.M_Key(input.KEnter)
	if !mgr.cvars.BoolValue("scr_clock") {
		t.Fatal("showtime toggle did not update cvar")
	}

	mgr.videoCursor = videoItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("video back should return to options, got %v", got)
	}
}

func TestAudioMenuVolumeAdjustment(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.cvars.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	mgr.cvars.Set("s_volume", "0.7")

	mgr.state = MenuAudio
	mgr.audioCursor = audioItemVolume
	mgr.M_Key(input.KRightArrow)
	if got := mgr.cvars.FloatValue("s_volume"); got != 0.8 {
		t.Fatalf("s_volume after right = %.1f, want 0.8", got)
	}

	mgr.M_Key(input.KLeftArrow)
	if got := mgr.cvars.FloatValue("s_volume"); got != 0.7 {
		t.Fatalf("s_volume after left = %.1f, want 0.7", got)
	}

	mgr.audioCursor = audioItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("audio back should return to options, got %v", got)
	}
}
