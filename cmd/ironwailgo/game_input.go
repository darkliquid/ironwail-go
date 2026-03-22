package main

import (
	"fmt"
	"math"
	"strings"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

func handleGameKeyEvent(event input.KeyEvent) {
	if g.Input == nil {
		return
	}

	switch g.Input.GetKeyDest() {
	case input.KeyConsole:
		handleConsoleKeyEvent(event)
		return
	case input.KeyMessage:
		handleMessageKeyEvent(event)
		return
	case input.KeyGame:
	default:
		return
	}

	if event.Key == input.KEscape && event.Down {
		if g.Menu != nil {
			g.Menu.ToggleMenu()
		}
		syncGameplayInputMode()
		return
	}
	if event.Key == input.KEnter && event.Down {
		if mods := g.Input.GetModifierState(); mods.Alt {
			cvar.SetBool("vid_fullscreen", !cvar.BoolValue("vid_fullscreen"))
			return
		}
	}
	if handleDemoPlaybackKeyEvent(event) {
		return
	}

	binding := strings.TrimSpace(g.Input.GetBinding(event.Key))
	if binding == "" {
		if event.Down && event.Key >= input.KMouseBegin && !isDemoPlaybackActive() {
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
		cmdsys.ExecuteText(fmt.Sprintf("%s %d", command, event.Key))
		return
	}
	if event.Down {
		cmdsys.ExecuteText(binding)
	}
}

func isDemoPlaybackActive() bool {
	return g.Host != nil && g.Host.DemoState() != nil && g.Host.DemoState().Playback
}

func currentDemoPlaybackState() *cl.DemoState {
	if g.Host == nil {
		return nil
	}
	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		return nil
	}
	return demo
}

func handleDemoPlaybackKeyEvent(event input.KeyEvent) bool {
	if g.Input == nil || g.Input.GetKeyDest() != input.KeyGame {
		return false
	}
	demo := currentDemoPlaybackState()
	if demo == nil {
		return false
	}

	switch event.Key {
	case input.KSpace, input.KYButton:
		if event.Down {
			demo.TogglePause()
			refreshDemoPlaybackSpeed()
		}
		return true

	case input.KUpArrow, input.KDpadUp:
		if event.Down {
			demo.IncreaseBaseSpeed()
			refreshDemoPlaybackSpeed()
		}
		return true

	case input.KDownArrow, input.KDpadDown:
		if event.Down {
			demo.DecreaseBaseSpeed()
			refreshDemoPlaybackSpeed()
		}
		return true

	case input.KLeftArrow, input.KRightArrow, input.KDpadLeft, input.KDpadRight, input.KShift, input.KCtrl:
		refreshDemoPlaybackSpeed()
		return true
	}

	return false
}

func backspaceChatInput() {
	if len(chatBuffer) > 0 {
		chatBuffer = chatBuffer[:len(chatBuffer)-1]
	}
}

func armRuntimeTextEditRepeat(key int) {
	g.TextEditRepeat = runtimeTextEditRepeatState{
		key:       key,
		nextDelay: 0.45,
	}
}

func refreshDemoPlaybackSpeed() {
	if g.Input == nil {
		return
	}
	demo := currentDemoPlaybackState()
	if demo == nil {
		return
	}
	leftHeld := g.Input.IsKeyDown(input.KLeftArrow) || g.Input.IsKeyDown(input.KDpadLeft)
	rightHeld := g.Input.IsKeyDown(input.KRightArrow) || g.Input.IsKeyDown(input.KDpadRight)
	slowHeld := g.Input.IsKeyDown(input.KShift) || g.Input.IsKeyDown(input.KCtrl)
	demo.UpdatePlaybackSpeed(g.Input.GetKeyDest() == input.KeyGame, leftHeld, rightHeld, slowHeld)
}

func handleMenuKeyEvent(event input.KeyEvent) {
	if !event.Down || g.Menu == nil {
		return
	}
	if event.Key == int('`') && !g.Menu.WaitingForKeyBinding() {
		cmdToggleConsole(nil)
		return
	}
	g.Menu.M_Key(event.Key)
	if g.Input != nil && !g.Menu.IsActive() {
		syncGameplayInputMode()
		if g.Input.GetKeyDest() == input.KeyGame {
			g.Input.ClearKeyStates()
		}
	}
}

func handleMenuCharEvent(ch rune) {
	if g.Input == nil || g.Input.GetKeyDest() != input.KeyMenu || g.Menu == nil {
		return
	}
	g.Menu.M_Char(ch)
}

func handleGameCharEvent(ch rune) {
	if g.Input == nil {
		return
	}

	switch g.Input.GetKeyDest() {
	case input.KeyConsole:
		if ch == '`' {
			return
		}
		console.AppendInputRune(ch)
	case input.KeyMessage:
		// Basic ASCII/Latin filtering, matching Quake's limited text support
		if ch >= 32 && ch < 127 {
			if len(chatBuffer) < 31 { // MAX_SAY
				chatBuffer += string(ch)
			}
		}
	}
}

func handleConsoleKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape, int('`'):
		console.ResetCompletion()
		g.Input.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
	case input.KEnter:
		line := strings.TrimSpace(console.CommitInput())
		console.ResetCompletion()
		if line == "" {
			return
		}
		console.Printf("]%s\n", line)
		cmdsys.ExecuteText(line)
	case input.KTab:
		line := console.InputLine()
		completed, matches := console.CompleteInput(line, true)
		if len(matches) == 0 {
			return
		}
		console.SetInputLine(completed)
	case input.KBackspace:
		armRuntimeTextEditRepeat(input.KBackspace)
		console.BackspaceInput()
	case input.KUpArrow:
		console.PreviousHistory()
	case input.KDownArrow:
		console.NextHistory()
	case input.KPgUp:
		console.Scroll(2)
	case input.KPgDn:
		console.Scroll(-2)
	case input.KHome:
		console.Scroll(console.TotalLines())
	case input.KEnd:
		console.Scroll(-console.TotalLines())
	}
}

func handleMessageKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape:
		g.Input.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
	case input.KEnter:
		g.Input.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
		if chatBuffer != "" {
			cmd := "say"
			if chatTeam {
				cmd = "say_team"
			}
			// Escape quotes in the message
			msg := strings.ReplaceAll(chatBuffer, "\"", "'")
			if g.Client != nil {
				g.Client.SendStringCmd(fmt.Sprintf("%s \"%s\"", cmd, msg))
			}
		}
	case input.KBackspace:
		armRuntimeTextEditRepeat(input.KBackspace)
		backspaceChatInput()
	}
}

func updateRuntimeTextEditRepeat(dt float64) {
	if g.Input == nil || dt <= 0 {
		g.TextEditRepeat = runtimeTextEditRepeatState{}
		return
	}

	activeKey := 0
	var repeatAction func()
	switch g.Input.GetKeyDest() {
	case input.KeyConsole:
		if g.Input.IsKeyDown(input.KBackspace) {
			activeKey = input.KBackspace
			repeatAction = console.BackspaceInput
		}
	case input.KeyMessage:
		if g.Input.IsKeyDown(input.KBackspace) {
			activeKey = input.KBackspace
			repeatAction = backspaceChatInput
		}
	}

	if activeKey == 0 || repeatAction == nil {
		g.TextEditRepeat = runtimeTextEditRepeatState{}
		return
	}
	if g.TextEditRepeat.key != activeKey {
		g.TextEditRepeat.key = activeKey
		g.TextEditRepeat.nextDelay = 0.45
		return
	}

	g.TextEditRepeat.nextDelay -= dt
	for g.TextEditRepeat.nextDelay <= 0 {
		repeatAction()
		g.TextEditRepeat.nextDelay += 0.05
	}
}

func syncGameplayInputMode() {
	if g.Input == nil {
		return
	}

	menuActive := g.Menu != nil && g.Menu.IsActive()
	wantDest := g.Input.GetKeyDest()
	switch {
	case menuActive:
		wantDest = input.KeyMenu
	case wantDest == input.KeyMenu:
		wantDest = input.KeyGame
	case wantDest != input.KeyConsole && wantDest != input.KeyMessage:
		wantDest = input.KeyGame
	}
	if g.Input.GetKeyDest() != wantDest {
		g.Input.SetKeyDest(wantDest)
	}

	shouldGrab := !menuActive && wantDest == input.KeyGame
	if shouldGrab == g.MouseGrabbed {
		return
	}

	g.Input.SetMouseGrab(shouldGrab)
	g.Input.ClearState()
	if !shouldGrab {
		releaseGameplayButtons()
	}
	g.MouseGrabbed = shouldGrab
}

// applyMenuMouseMove forwards accumulated mouse Y movement to the menu manager
// when the menu is active. This implements the M_Mousemove() equivalent from
// C Ironwail, allowing mouse scrolling to drive menu cursor selection.
func applyMenuMouseMove() {
	if g.Input == nil || g.Menu == nil || !g.Menu.IsActive() {
		return
	}
	if g.Input.GetKeyDest() != input.KeyMenu {
		return
	}
	state := g.Input.GetState()
	if state.MouseValid {
		if mx, my, ok := screenToMenuCoords(int(state.MouseX), int(state.MouseY)); ok {
			g.Menu.M_MousemoveAbsolute(mx, my)
			return
		}
	}
	if state.MouseDX != 0 || state.MouseDY != 0 {
		g.Menu.M_Mousemove(int(state.MouseDX), int(state.MouseDY))
	}
}

func screenToMenuCoords(screenX, screenY int) (menuX, menuY int, ok bool) {
	screenW, screenH := 320, 200
	if g.Renderer != nil {
		if w, h := g.Renderer.Size(); w > 0 && h > 0 {
			screenW, screenH = w, h
		}
	}
	params := runtimeOverlayCanvasParams(screenW, screenH)
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

func applyGameplayMouseLook() {
	if g.Input == nil || g.Client == nil {
		return
	}
	if g.Input.GetKeyDest() != input.KeyGame {
		g.Client.MouseSideMove = 0
		g.Client.MouseForwardMove = 0
		g.Client.MouseUpMove = 0
		g.Input.ClearState()
		return
	}

	state := g.Input.GetState()
	sensitivity := float32(cvar.FloatValue("sensitivity"))
	if sensitivity <= 0 {
		sensitivity = 1
	}
	yawScale := sensitivity * float32(cvar.FloatValue("m_yaw"))
	if yawScale == 0 {
		yawScale = 0.15
	}
	pitchScale := sensitivity * float32(cvar.FloatValue("m_pitch"))
	if pitchScale == 0 {
		pitchScale = 0.12
	}
	sideScale := sensitivity * float32(cvar.FloatValue("m_side"))
	if sideScale == 0 {
		sideScale = 0.8
	}
	forwardScale := sensitivity * float32(cvar.FloatValue("m_forward"))
	if forwardScale == 0 {
		forwardScale = 1
	}
	mouseLook := g.Client.FreeLook || g.Client.InputMLook.State&1 != 0
	lookStrafe := cvar.BoolValue("lookstrafe")
	g.Client.MouseSideMove = 0
	g.Client.MouseForwardMove = 0
	g.Client.MouseUpMove = 0
	if state.MouseDX != 0 {
		if g.Client.InputStrafe.State&1 != 0 || (lookStrafe && mouseLook) {
			g.Client.MouseSideMove += float32(state.MouseDX) * sideScale
		} else {
			g.Client.ViewAngles[1] -= float32(state.MouseDX) * yawScale
		}
	}
	if mouseLook && (state.MouseDX != 0 || state.MouseDY != 0) {
		g.Client.StopPitchDrift()
	}
	if state.MouseDY != 0 {
		if mouseLook && g.Client.InputStrafe.State&1 == 0 {
			g.Client.ViewAngles[0] += float32(state.MouseDY) * pitchScale
			if g.Client.ViewAngles[0] > g.Client.MaxPitch {
				g.Client.ViewAngles[0] = g.Client.MaxPitch
			}
			if g.Client.ViewAngles[0] < g.Client.MinPitch {
				g.Client.ViewAngles[0] = g.Client.MinPitch
			}
		} else {
			g.Client.MouseForwardMove -= float32(state.MouseDY) * forwardScale
		}
	}
	if !mouseLook && g.Client.LookSpring {
		g.Client.StartPitchDrift()
	}
	g.Input.ClearState()
}

func releaseGameplayButtons() {
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

func applyStartupGameplayInputMode() {
	if g.Menu != nil {
		g.Menu.HideMenu()
	}
	syncGameplayInputMode()
	if g.Input != nil {
		g.Input.ClearKeyStates()
	}
}
