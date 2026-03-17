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
	if g.Input == nil {
		return
	}

	switch g.Input.GetKeyDest() {
	case input.KeyConsole:
		handleConsoleKeyEvent(event)
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

func handleMenuKeyEvent(event input.KeyEvent) {
	if !event.Down || g.Menu == nil {
		return
	}
	g.Menu.M_Key(event.Key)
}

func handleMenuCharEvent(ch rune) {
	if g.Input == nil || g.Input.GetKeyDest() != input.KeyMenu || g.Menu == nil {
		return
	}
	g.Menu.M_Char(ch)
}

func handleGameCharEvent(ch rune) {
	if g.Input == nil || g.Input.GetKeyDest() != input.KeyConsole {
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
	case wantDest != input.KeyConsole:
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
	if state.MouseDX != 0 || state.MouseDY != 0 {
		g.Menu.M_Mousemove(int(state.MouseDX), int(state.MouseDY))
	}
}

func applyGameplayMouseLook() {
	if g.Input == nil || g.Client == nil {
		return
	}
	if g.Input.GetKeyDest() != input.KeyGame {
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
	mouseLook := g.Client.FreeLook || g.Client.InputMLook.State&1 != 0
	if state.MouseDX != 0 {
		g.Client.ViewAngles[1] -= float32(state.MouseDX) * yawScale
	}
	if state.MouseDY != 0 && mouseLook {
		g.Client.ViewAngles[0] += float32(state.MouseDY) * pitchScale
		if g.Client.ViewAngles[0] > g.Client.MaxPitch {
			g.Client.ViewAngles[0] = g.Client.MaxPitch
		}
		if g.Client.ViewAngles[0] < g.Client.MinPitch {
			g.Client.ViewAngles[0] = g.Client.MinPitch
		}
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
