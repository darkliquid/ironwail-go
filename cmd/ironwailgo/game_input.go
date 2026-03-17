package main

import (
	"fmt"
	"strings"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
)

func handleGameKeyEvent(event input.KeyEvent) {
	if gameInput == nil {
		return
	}

	switch gameInput.GetKeyDest() {
	case input.KeyConsole:
		handleConsoleKeyEvent(event)
		return
	case input.KeyGame:
	default:
		return
	}

	if event.Key == input.KEscape && event.Down {
		if gameMenu != nil {
			gameMenu.ToggleMenu()
		}
		syncGameplayInputMode()
		return
	}
	if event.Key == input.KEnter && event.Down {
		if mods := gameInput.GetModifierState(); mods.Alt {
			cvar.SetBool("vid_fullscreen", !cvar.BoolValue("vid_fullscreen"))
			return
		}
	}

	binding := strings.TrimSpace(gameInput.GetBinding(event.Key))
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
		if gameClient == nil {
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
	return gameHost != nil && gameHost.DemoState() != nil && gameHost.DemoState().Playback
}

func handleMenuKeyEvent(event input.KeyEvent) {
	if !event.Down || gameMenu == nil {
		return
	}
	gameMenu.M_Key(event.Key)
}

func handleMenuCharEvent(ch rune) {
	if gameInput == nil || gameInput.GetKeyDest() != input.KeyMenu || gameMenu == nil {
		return
	}
	gameMenu.M_Char(ch)
}

func handleGameCharEvent(ch rune) {
	if gameInput == nil || gameInput.GetKeyDest() != input.KeyConsole {
		return
	}
	if ch == '`' {
		return
	}
	console.AppendInputRune(ch)
}

func handleConsoleKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape, int('`'):
		console.ResetCompletion()
		gameInput.SetKeyDest(input.KeyGame)
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

func syncGameplayInputMode() {
	if gameInput == nil {
		return
	}

	menuActive := gameMenu != nil && gameMenu.IsActive()
	wantDest := gameInput.GetKeyDest()
	switch {
	case menuActive:
		wantDest = input.KeyMenu
	case wantDest == input.KeyMenu:
		wantDest = input.KeyGame
	case wantDest != input.KeyConsole:
		wantDest = input.KeyGame
	}
	if gameInput.GetKeyDest() != wantDest {
		gameInput.SetKeyDest(wantDest)
	}

	shouldGrab := !menuActive && wantDest == input.KeyGame
	if shouldGrab == gameMouseGrabbed {
		return
	}

	gameInput.SetMouseGrab(shouldGrab)
	gameInput.ClearState()
	if !shouldGrab {
		releaseGameplayButtons()
	}
	gameMouseGrabbed = shouldGrab
}

// applyMenuMouseMove forwards accumulated mouse Y movement to the menu manager
// when the menu is active. This implements the M_Mousemove() equivalent from
// C Ironwail, allowing mouse scrolling to drive menu cursor selection.
func applyMenuMouseMove() {
	if gameInput == nil || gameMenu == nil || !gameMenu.IsActive() {
		return
	}
	if gameInput.GetKeyDest() != input.KeyMenu {
		return
	}
	state := gameInput.GetState()
	if state.MouseDX != 0 || state.MouseDY != 0 {
		gameMenu.M_Mousemove(int(state.MouseDX), int(state.MouseDY))
	}
}

func applyGameplayMouseLook() {
	if gameInput == nil || gameClient == nil {
		return
	}
	if gameInput.GetKeyDest() != input.KeyGame {
		gameInput.ClearState()
		return
	}

	state := gameInput.GetState()
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
	mouseLook := gameClient.FreeLook || gameClient.InputMLook.State&1 != 0
	if state.MouseDX != 0 {
		gameClient.ViewAngles[1] -= float32(state.MouseDX) * yawScale
	}
	if state.MouseDY != 0 && mouseLook {
		gameClient.ViewAngles[0] += float32(state.MouseDY) * pitchScale
		if gameClient.ViewAngles[0] > gameClient.MaxPitch {
			gameClient.ViewAngles[0] = gameClient.MaxPitch
		}
		if gameClient.ViewAngles[0] < gameClient.MinPitch {
			gameClient.ViewAngles[0] = gameClient.MinPitch
		}
	}
	gameInput.ClearState()
}

func releaseGameplayButtons() {
	gameShowScores = false
	if gameClient == nil {
		return
	}
	buttons := []*cl.KButton{
		&gameClient.InputForward,
		&gameClient.InputBack,
		&gameClient.InputLeft,
		&gameClient.InputRight,
		&gameClient.InputUp,
		&gameClient.InputDown,
		&gameClient.InputLookUp,
		&gameClient.InputLookDown,
		&gameClient.InputMoveLeft,
		&gameClient.InputMoveRight,
		&gameClient.InputStrafe,
		&gameClient.InputSpeed,
		&gameClient.InputUse,
		&gameClient.InputJump,
		&gameClient.InputAttack,
		&gameClient.InputKLook,
		&gameClient.InputMLook,
	}
	for _, button := range buttons {
		gameClient.KeyUp(button, -1)
	}
}

func applyStartupGameplayInputMode() {
	if gameMenu != nil {
		gameMenu.HideMenu()
	}
	syncGameplayInputMode()
	if gameInput != nil {
		gameInput.ClearKeyStates()
	}
}
