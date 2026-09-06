package game

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func (g *Game) pollRuntimeInputEvents() {
	if g.Input == nil {
		return
	}
	_ = g.Input.PollEvents()
}

func (g *Game) logRuntimeKeyDispatch(path string, event input.KeyEvent) {
	index := g.inputDispatchLogCount.Add(1)
	if index > 32 {
		return
	}
	keyName := input.KeyToString(event.Key)
	if keyName == "" {
		keyName = fmt.Sprintf("KEY%d", event.Key)
	}
	menuActive := g.Menu != nil && g.Menu.IsActive()
	keyDest := "none"
	if g.Input != nil {
		keyDest = g.keyDestName(g.Input.KeyDest())
	}
	slog.Debug("input dispatch", "path", path, "key", keyName, "key_code", event.Key, "down", event.Down, "key_dest", keyDest, "menu_active", menuActive, "event_index", index)
}

func (g *Game) handleGameKeyEvent(event input.KeyEvent) {
	if g.Input == nil {
		return
	}
	g.logRuntimeKeyDispatch("game", event)
	if event.Down && event.Key == input.KStart && g.Menu != nil {
		g.Menu.ToggleMenu()
		g.syncGameplayInputMode()
		return
	}

	if g.CSQC != nil && g.CSQC.IsLoaded() && g.CSQC.HasInputEvent() {
		evType := 0
		if !event.Down {
			evType = 1
		}
		ascii := event.Key
		if ascii > 255 {
			ascii = 0
		}
		handled, err := g.CSQC.CallInputEvent(evType, event.Key, ascii)
		if err != nil {
			slog.Error("CSQC_InputEvent failed", "error", err)
		} else if handled {
			return
		}
	}

	// C Quake keys.c:1357: key up events always generate button-release
	// commands if the key is bound to a button (+command), regardless of
	// key_dest (menu, console, message, game). This prevents actions started
	// before a mode switch or focus loss from getting stuck.
	binding := strings.TrimSpace(g.Input.Binding(event.Key))
	if !event.Down && strings.HasPrefix(binding, "+") {
		if g.Client != nil {
			command := "-" + binding[1:]
			g.Host.Cmd.ExecuteText(fmt.Sprintf("%s %d", command, event.Key))
		}
		return
	}

	switch g.Input.KeyDest() {
	case input.KeyConsole:
		g.handleConsoleKeyEvent(event)
		return
	case input.KeyMessage:
		g.handleMessageKeyEvent(event)
		return
	case input.KeyGame:
	default:
		return
	}

	if (event.Key == input.KEscape || event.Key == input.KStart) && event.Down {
		if g.Menu != nil {
			g.Menu.ToggleMenu()
		}
		g.syncGameplayInputMode()
		return
	}
	if event.Key == input.KEnter && event.Down {
		if mods := g.Input.ModifierState(); mods.Alt {
			g.Host.CVar.SetBool("vid_fullscreen", !g.Host.CVar.BoolValue("vid_fullscreen"))
			return
		}
	}
	if g.handleDemoPlaybackKeyEvent(event) {
		return
	}

	if binding == "" {
		if event.Down && event.Key >= input.KMouseBegin && !g.isDemoPlaybackActive() {
			keyName := input.KeyToString(event.Key)
			if keyName == "" {
				keyName = fmt.Sprintf("KEY%d", event.Key)
			}
			console.Printf("%s is unbound, use Options menu to set.\n", keyName)
		}
		return
	}
	if strings.HasPrefix(binding, "+") {
		if g.Client == nil {
			return
		}
		command := binding
		if !event.Down {
			command = "-" + binding[1:]
		}
		g.Host.Cmd.ExecuteText(fmt.Sprintf("%s %d", command, event.Key))
		return
	}
	if event.Down {
		g.Host.Cmd.ExecuteText(binding)
	}
}

func (g *Game) isDemoPlaybackActive() bool {
	return g.Host != nil && g.Host.DemoState() != nil && g.Host.DemoState().Playback
}

func (g *Game) currentDemoPlaybackState() *cl.DemoState {
	if g.Host == nil {
		return nil
	}
	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		return nil
	}
	return demo
}

func (g *Game) handleDemoPlaybackKeyEvent(event input.KeyEvent) bool {
	if g.Input == nil || g.Input.KeyDest() != input.KeyGame {
		return false
	}
	demo := g.currentDemoPlaybackState()
	if demo == nil {
		return false
	}

	switch event.Key {
	case input.KSpace, input.KYButton:
		if event.Down {
			demo.TogglePause()
			g.refreshDemoPlaybackSpeed()
		}
		return true

	case input.KUpArrow, input.KDpadUp:
		if event.Down {
			demo.IncreaseBaseSpeed()
			g.refreshDemoPlaybackSpeed()
		}
		return true

	case input.KDownArrow, input.KDpadDown:
		if event.Down {
			demo.DecreaseBaseSpeed()
			g.refreshDemoPlaybackSpeed()
		}
		return true

	case input.KLeftArrow, input.KRightArrow, input.KDpadLeft, input.KDpadRight, input.KShift, input.KCtrl:
		g.refreshDemoPlaybackSpeed()
		return true
	}

	return false
}

func (g *Game) backspaceChatInput() {
	if len(g.chatBuffer) > 0 {
		g.chatBuffer = g.chatBuffer[:len(g.chatBuffer)-1]
	}
}

func (g *Game) armRuntimeTextEditRepeat(key int) {
	g.TextEditRepeat = TextEditRepeatState{
		Key:       key,
		NextDelay: 0.45,
	}
}

func (g *Game) refreshDemoPlaybackSpeed() {
	if g.Input == nil {
		return
	}
	demo := g.currentDemoPlaybackState()
	if demo == nil {
		return
	}
	leftHeld := g.Input.IsKeyDown(input.KLeftArrow) || g.Input.IsKeyDown(input.KDpadLeft)
	rightHeld := g.Input.IsKeyDown(input.KRightArrow) || g.Input.IsKeyDown(input.KDpadRight)
	slowHeld := g.Input.IsKeyDown(input.KShift) || g.Input.IsKeyDown(input.KCtrl)
	demo.UpdatePlaybackSpeed(g.Input.KeyDest() == input.KeyGame, leftHeld, rightHeld, slowHeld)
}

func (g *Game) handleMenuKeyEvent(event input.KeyEvent) {
	if !event.Down || g.Menu == nil {
		return
	}
	g.logRuntimeKeyDispatch("menu", event)
	if event.Key == int('`') && !g.Menu.WaitingForKeyBinding() {
		g.cmdToggleConsole(nil)
		return
	}
	g.Menu.M_Key(event.Key)
	if g.Input != nil && !g.Menu.IsActive() {
		g.syncGameplayInputMode()
		if g.Input.KeyDest() == input.KeyGame {
			g.Input.ClearKeyStates()
		}
	}
}

func (g *Game) handleMenuCharEvent(ch rune) {
	if g.Input == nil || g.Input.KeyDest() != input.KeyMenu || g.Menu == nil {
		return
	}
	g.Menu.M_Char(ch)
}

func (g *Game) handleGameCharEvent(ch rune) {
	if g.Input == nil {
		return
	}

	switch g.Input.KeyDest() {
	case input.KeyConsole:
		if ch == '`' {
			return
		}
		console.AppendInputRune(ch)
	case input.KeyMessage:
		// Basic ASCII/Latin filtering, matching Quake's limited text support
		if ch >= 32 && ch < 127 {
			if len(g.chatBuffer) < 31 { // MAX_SAY
				g.chatBuffer += string(ch)
			}
		}
	}
}

func (g *Game) handleConsoleKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape, int('`'):
		console.ResetCompletion()
		if g.runtimeConsoleForcedUp() && g.Menu != nil {
			g.showRuntimeMenuState(menu.MenuMain)
			g.syncGameplayInputMode()
			return
		}
		g.Input.SetKeyDest(input.KeyGame)
		g.syncGameplayInputMode()
	case input.KEnter:
		line := strings.TrimSpace(console.CommitInput())
		console.ResetCompletion()
		if line == "" {
			return
		}
		console.Printf("]%s\n", line)
		g.Host.Cmd.ExecuteText(line)
	case input.KTab, input.KBack:
		line := console.InputLine()
		forward := !g.Input.ModifierState().Shift
		completed, matches := console.CompleteInput(line, forward)
		if len(matches) == 0 {
			return
		}
		console.SetInputLine(completed)
	case input.KBackspace:
		g.armRuntimeTextEditRepeat(input.KBackspace)
		if g.Input.ModifierState().Ctrl {
			console.DeleteWordLeft()
		} else {
			console.BackspaceInput()
		}
	case input.KDel:
		if g.Input.ModifierState().Ctrl {
			console.DeleteWordRight()
		} else {
			console.DeleteInput()
		}
	case input.KUpArrow:
		console.PreviousHistory()
	case input.KDownArrow:
		console.NextHistory()
	case input.KLeftArrow:
		console.MoveCursorLeft(g.Input.ModifierState().Ctrl)
	case input.KRightArrow:
		console.MoveCursorRight(g.Input.ModifierState().Ctrl)
	case input.KIns:
		console.ToggleInsertMode()
	case input.KPgUp:
		console.Scroll(2)
	case input.KPgDn:
		console.Scroll(-2)
	case input.KHome:
		if g.Input.ModifierState().Ctrl {
			console.Scroll(console.TotalLines())
		} else {
			console.MoveCursorStart()
		}
	case input.KEnd:
		if g.Input.ModifierState().Ctrl {
			console.Scroll(-console.TotalLines())
		} else {
			console.MoveCursorEnd()
		}
	}
}

func (g *Game) handleMessageKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape:
		g.Input.SetKeyDest(input.KeyGame)
		g.syncGameplayInputMode()
	case input.KEnter:
		g.Input.SetKeyDest(input.KeyGame)
		g.syncGameplayInputMode()
		if g.chatBuffer != "" {
			cmd := "say"
			if g.chatTeam {
				cmd = "say_team"
			}
			// Escape quotes in the message
			msg := strings.ReplaceAll(g.chatBuffer, "\"", "'")
			if g.Client != nil {
				if _, err := g.Client.SendStringCmd(fmt.Sprintf("%s \"%s\"", cmd, msg)); err != nil {
					slog.Warn("game: failed to send chat command", "cmd", cmd, "err", err)
				}
			}
		}
	case input.KBackspace:
		g.armRuntimeTextEditRepeat(input.KBackspace)
		g.backspaceChatInput()
	}
}

func (g *Game) updateRuntimeTextEditRepeat(dt float64) {
	if g.Input == nil || dt <= 0 {
		g.TextEditRepeat = TextEditRepeatState{}
		return
	}

	activeKey := 0
	var repeatAction func()
	switch g.Input.KeyDest() {
	case input.KeyConsole:
		if g.Input.IsKeyDown(input.KBackspace) {
			activeKey = input.KBackspace
			repeatAction = console.BackspaceInput
		}
	case input.KeyMessage:
		if g.Input.IsKeyDown(input.KBackspace) {
			activeKey = input.KBackspace
			repeatAction = g.backspaceChatInput
		}
	}

	if activeKey == 0 || repeatAction == nil {
		g.TextEditRepeat = TextEditRepeatState{}
		return
	}
	if g.TextEditRepeat.Key != activeKey {
		g.TextEditRepeat.Key = activeKey
		g.TextEditRepeat.NextDelay = 0.45
		return
	}

	g.TextEditRepeat.NextDelay -= dt
	for g.TextEditRepeat.NextDelay <= 0 {
		repeatAction()
		g.TextEditRepeat.NextDelay += 0.05
	}
}

func (g *Game) syncGameplayInputMode() {
	if g.Input == nil {
		return
	}

	menuActive := g.Menu != nil && g.Menu.IsActive()
	wantDest := g.Input.KeyDest()
	switch {
	case menuActive:
		wantDest = input.KeyMenu
	case g.runtimeConsoleForcedUp():
		wantDest = input.KeyConsole
	case wantDest == input.KeyMenu:
		wantDest = input.KeyGame
	case wantDest != input.KeyConsole && wantDest != input.KeyMessage:
		wantDest = input.KeyGame
	}
	if g.Input.KeyDest() != wantDest {
		g.Input.SetKeyDest(wantDest)
		slog.Debug("input mode updated", "key_dest", g.keyDestName(wantDest), "menu_active", menuActive)
	}

	// Leaving active gameplay (menu/console/message mode, or a fresh start in a
	// non-game dest) must always release latched gameplay buttons and forget the
	// physical key states, since the matching key-up events will be routed to
	// (and swallowed by) the menu/console handler. Doing this only when the
	// cursor-grab state changes is not enough: if the cursor was never grabbed
	// (or grab state is unchanged) the early return below would otherwise leave
	// +forward/+attack/etc. latched, and re-pressing the key after closing the
	// menu would be filtered as a key-repeat.
	if wantDest != input.KeyGame {
		if g.Client != nil {
			g.releaseGameplayButtons()
		}
		g.Input.ClearKeyStates()
	}

	shouldGrab := !menuActive && wantDest == input.KeyGame
	if shouldGrab == g.MouseGrabbed {
		return
	}

	g.Input.SetMouseGrab(shouldGrab)
	g.Input.ClearState()
	if !shouldGrab {
		g.releaseGameplayButtons()
	}
	g.MouseGrabbed = shouldGrab
}

// applyMenuMouseMove forwards accumulated mouse Y movement to the menu manager
// when the menu is active. This implements the M_Mousemove() equivalent from
// C Ironwail, allowing mouse scrolling to drive menu cursor selection.
func (g *Game) applyMenuMouseMove() {
	if g.Input == nil || g.Menu == nil || !g.Menu.IsActive() {
		return
	}
	if g.Input.KeyDest() != input.KeyMenu {
		return
	}
	state := g.Input.State()
	if state.MouseValid {
		if mx, my, ok := g.screenToMenuCoords(int(state.MouseX), int(state.MouseY)); ok {
			g.Menu.M_MousemoveAbsolute(mx, my)
			return
		}
	}
	if state.MouseDX != 0 || state.MouseDY != 0 {
		g.Menu.M_Mousemove(int(state.MouseDX), int(state.MouseDY))
	}
}

func (g *Game) screenToMenuCoords(screenX, screenY int) (menuX, menuY int, ok bool) {
	screenW, screenH := 320, 200
	if g.Renderer != nil {
		if w, h := g.Renderer.Size(); w > 0 && h > 0 {
			screenW, screenH = w, h
		}
	}
	params := g.runtimeOverlayCanvasParams(screenW, screenH)
	if params.GLWidth <= 0 || params.GLHeight <= 0 {
		return 0, 0, false
	}
	transform := renderer.GetCanvasTransform(renderer.CanvasMenu, params)
	if transform.Scale[0] == 0 || transform.Scale[1] == 0 {
		return 0, 0, false
	}
	ndcX := (float32(screenX)+0.5)*2/params.GLWidth - 1
	ndcY := 1 - (float32(screenY)+0.5)*2/params.GLHeight
	menuXF := (ndcX - transform.Offset[0]) / transform.Scale[0]
	menuYF := (ndcY - transform.Offset[1]) / transform.Scale[1]
	if menuXF < 0 || menuXF >= 320 || menuYF < 0 || menuYF >= 200 {
		return 0, 0, false
	}
	return int(math.Floor(float64(menuXF))), int(math.Floor(float64(menuYF))), true
}

func (g *Game) applyGameplayMouseLook() {
	if g.Input == nil || g.Client == nil {
		return
	}
	if g.Input.KeyDest() != input.KeyGame {
		g.Client.MouseSideMove = 0
		g.Client.MouseForwardMove = 0
		g.Client.MouseUpMove = 0
		g.Input.ClearState()
		return
	}
	if g.Client.InCutscene() {
		g.Client.MouseSideMove = 0
		g.Client.MouseForwardMove = 0
		g.Client.MouseUpMove = 0
		g.Input.ClearState()
		return
	}

	state := g.Input.State()
	sensitivity := float32(g.Host.CVar.FloatValue("sensitivity"))
	if sensitivity <= 0 {
		sensitivity = 1
	}
	yawScale := sensitivity * float32(g.Host.CVar.FloatValue("m_yaw"))
	if yawScale == 0 {
		yawScale = 0.15
	}
	pitchScale := sensitivity * float32(g.Host.CVar.FloatValue("m_pitch"))
	if pitchScale == 0 {
		pitchScale = 0.12
	}
	sideScale := sensitivity * float32(g.Host.CVar.FloatValue("m_side"))
	if sideScale == 0 {
		sideScale = 0.8
	}
	forwardScale := sensitivity * float32(g.Host.CVar.FloatValue("m_forward"))
	if forwardScale == 0 {
		forwardScale = 1
	}
	mouseLook := g.Client.FreeLook || g.Client.InputMLook.State&1 != 0
	lookStrafe := g.Host.CVar.BoolValue("lookstrafe")
	g.Client.MouseSideMove = 0
	g.Client.MouseForwardMove = 0
	g.Client.MouseUpMove = 0
	if state.MouseDX != 0 {
		if g.Client.InputStrafe.State&1 != 0 || (lookStrafe && mouseLook) {
			g.Client.MouseSideMove += float32(state.MouseDX) * sideScale
		} else {
			g.Client.ViewAngles.Y -= float32(state.MouseDX) * yawScale
		}
	}
	if mouseLook && (state.MouseDX != 0 || state.MouseDY != 0) {
		g.Client.StopPitchDrift()
	}
	if state.MouseDY != 0 {
		if mouseLook && g.Client.InputStrafe.State&1 == 0 {
			g.Client.ViewAngles.X += float32(state.MouseDY) * pitchScale
			if g.Client.ViewAngles.X > g.Client.MaxPitch {
				g.Client.ViewAngles.X = g.Client.MaxPitch
			}
			if g.Client.ViewAngles.X < g.Client.MinPitch {
				g.Client.ViewAngles.X = g.Client.MinPitch
			}
		} else {
			g.Client.MouseForwardMove -= float32(state.MouseDY) * forwardScale
		}
	}
	if g.Input.IsGamepadConnected(0) && g.Host.CVar.BoolValue("joy_look") {
		gamepad := g.Input.GamepadState(0)
		gamepadYawScale := float32(g.Host.CVar.FloatValue("joy_looksensitivity_yaw"))
		if gamepadYawScale == 0 {
			gamepadYawScale = 4
		}
		gamepadPitchScale := float32(g.Host.CVar.FloatValue("joy_looksensitivity_pitch"))
		if gamepadPitchScale == 0 {
			gamepadPitchScale = 4
		}
		yawDelta := gamepad.RightX * gamepadYawScale
		pitchDelta := gamepad.RightY * gamepadPitchScale
		if g.Host.CVar.BoolValue("joy_gyro_look") {
			yawDelta += gamepad.GyroYawDelta * float32(g.Host.CVar.FloatValue("joy_gyro_yaw_scale"))
			pitchDelta += gamepad.GyroPitchDelta * float32(g.Host.CVar.FloatValue("joy_gyro_pitch_scale"))
		}
		if yawDelta != 0 {
			g.Client.ViewAngles.Y -= yawDelta
		}
		if pitchDelta != 0 && g.Client.InputStrafe.State&1 == 0 {
			g.Client.ViewAngles.X += pitchDelta
			if g.Client.ViewAngles.X > g.Client.MaxPitch {
				g.Client.ViewAngles.X = g.Client.MaxPitch
			}
			if g.Client.ViewAngles.X < g.Client.MinPitch {
				g.Client.ViewAngles.X = g.Client.MinPitch
			}
		}
		if yawDelta != 0 || pitchDelta != 0 {
			g.Client.StopPitchDrift()
		}
	}
	if !mouseLook && g.Client.LookSpring {
		g.Client.StartPitchDrift()
	}
	g.Input.ClearState()
}

func (g *Game) releaseGameplayButtons() {
	g.ShowScores = false
	if g.Client == nil {
		return
	}
	buttons := []*cl.KButton{
		&g.Client.InputForward,
		&g.Client.InputBack,
		&g.Client.InputLeft,
		&g.Client.InputRight,
		&g.Client.InputUp,
		&g.Client.InputDown,
		&g.Client.InputLookUp,
		&g.Client.InputLookDown,
		&g.Client.InputMoveLeft,
		&g.Client.InputMoveRight,
		&g.Client.InputStrafe,
		&g.Client.InputSpeed,
		&g.Client.InputUse,
		&g.Client.InputJump,
		&g.Client.InputAttack,
		&g.Client.InputKLook,
		&g.Client.InputMLook,
	}
	for _, button := range buttons {
		g.Client.KeyUp(button, -1)
	}
}

func (g *Game) showRuntimeMenuState(state menu.MenuState) {
	if g.Menu == nil {
		return
	}
	g.Menu.ShowState(state)
	g.syncGameplayInputMode()
}

func (g *Game) ApplyStartupGameplayInputMode() {
	// Mirror C Ironwail: entering active gameplay does NOT force-close the
	// main menu. In particular, attract-mode demo playback (startdemos) runs
	// behind an open menu_main overlay; hiding the menu here would diverge
	// from C's behavior and cause the parity harness to desync. Only hide
	// the menu when we are NOT playing back an attract-mode demo.
	wasMenuActive := g.Menu != nil && g.Menu.IsActive()
	if g.Menu != nil {
		demo := g.Host.DemoState()
		attract := demo != nil && demo.Playback && g.Host.DemoNum() >= 0
		if !attract {
			g.Menu.HideMenu()
		}
	}
	wasConsole := g.Input != nil && (g.Input.KeyDest() == input.KeyConsole || g.Input.KeyDest() == input.KeyMessage)
	g.syncGameplayInputMode()
	// Reset the physical key state only when this call actually lifted the
	// game out of a menu/console/message mode. Calling ClearKeyStates on an
	// already-active frame drops the release of any key held during gameplay,
	// latching the button until escape resets it (C only clears key states on
	// input-grab begin/end, not every frame).
	if g.Input != nil && (wasMenuActive || wasConsole) {
		g.Input.ClearKeyStates()
	}
}
