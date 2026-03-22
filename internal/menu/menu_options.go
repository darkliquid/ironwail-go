package menu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// optionsKey routes keyboard input on the Options menu page.
// Items: 0 = Controls, 1 = Video, 2 = Audio, 3 = VSync toggle, 4 = Back.
func (m *Manager) optionsKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.optionsCursor--
		if m.optionsCursor < 0 {
			m.optionsCursor = optionsItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.optionsCursor++
		if m.optionsCursor >= optionsItems {
			m.optionsCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.optionsCursor {
		case 0:
			m.controlsCursor = 0
			m.controlsRebinding = false
			m.state = MenuControls
		case 1:
			m.videoCursor = 0
			m.state = MenuVideo
		case 2:
			m.audioCursor = 0
			m.state = MenuAudio
		case 3:
			cvar.SetBool("vid_vsync", !cvar.BoolValue("vid_vsync"))
		case 4:
			m.state = MenuMain
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	}
}

// controlsKey routes keyboard input on the Controls menu page. It handles two
// modes: normal navigation (up/down/left/right/enter) and rebinding mode,
// where the next key press is captured as the new binding for the selected
// action.
func (m *Manager) controlsKey(key int) {
	if m.controlsRebinding {
		switch key {
		case input.KEscape, input.KMouse2:
			m.controlsRebinding = false
			m.playMenuSound(menuSoundCancel)
		default:
			m.setControlBinding(m.controlsCursor, key)
			m.controlsRebinding = false
			m.playMenuSound(menuSoundSelect)
		}
		return
	}

	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.controlsCursor--
		if m.controlsCursor < 0 {
			m.controlsCursor = controlsItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.controlsCursor++
		if m.controlsCursor >= controlsItems {
			m.controlsCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(-1)
			m.playMenuSound(menuSoundNavigate)
			return
		}
		m.clearControlBinding(m.controlsCursor)
		m.playMenuSound(menuSoundCancel)
	case input.KBackspace:
		if m.controlsCursor < controlsBindingStart || m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		m.clearControlBinding(m.controlsCursor)
		m.playMenuSound(menuSoundCancel)
	case input.KRightArrow:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(1)
			m.playMenuSound(menuSoundNavigate)
			return
		}
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEnter, input.KSpace, input.KMouse1:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundSelect)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(1)
			m.playMenuSound(menuSoundSelect)
			return
		}
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

// adjustControlSetting modifies the slider/toggle control setting at the
// current cursor position by the given delta (+1 or -1). Only applies to
// the non-binding items (mouse speed, invert mouse, always run, freelook).
func (m *Manager) adjustControlSetting(delta int) {
	switch m.controlsCursor {
	case controlItemMouseSpeed:
		speed := cvar.FloatValue("sensitivity") + 0.5*float64(delta)
		speed = clampFloat(speed, 1, 11)
		cvar.SetFloat("sensitivity", roundToTenth(speed))
	case controlItemInvertMouse:
		pitch := cvar.FloatValue("m_pitch")
		if pitch == 0 {
			pitch = 0.0176
		}
		cvar.SetFloat("m_pitch", -pitch)
	case controlItemAlwaysRun:
		cvar.SetBool("cl_alwaysrun", !cvar.BoolValue("cl_alwaysrun"))
	case controlItemFreeLook:
		cvar.SetBool("freelook", !cvar.BoolValue("freelook"))
	}
}

// videoKey routes keyboard input on the Video settings menu page. Left/Right
// adjust the current setting; Enter cycles it forward; Escape returns to
// the Options parent menu.
func (m *Manager) videoKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.videoCursor--
		if m.videoCursor < 0 {
			m.videoCursor = videoItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.videoCursor++
		if m.videoCursor >= videoItems {
			m.videoCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		m.adjustVideoSetting(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustVideoSetting(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		if m.videoCursor == videoItemBack {
			m.state = MenuOptions
			return
		}
		m.adjustVideoSetting(1)
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

// audioKey routes keyboard input on the Audio settings menu page.
// Currently only volume is adjustable via left/right arrows.
func (m *Manager) audioKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.audioCursor--
		if m.audioCursor < 0 {
			m.audioCursor = audioItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.audioCursor++
		if m.audioCursor >= audioItems {
			m.audioCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		m.adjustAudioSetting(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustAudioSetting(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		if m.audioCursor == audioItemBack {
			m.state = MenuOptions
			return
		}
		m.adjustAudioSetting(1)
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

// adjustVideoSetting modifies the video cvar at the current cursor position
// by the given delta. Each item maps to a specific cvar: resolution cycles
// through videoResolutions, fullscreen/vsync/viewmodel/showfps are toggles,
// maxFPS cycles through maxFPSValues, gamma is a float slider, waterwarp
// cycles 0/1/2, and hud_style cycles 0/1/2.
func (m *Manager) adjustVideoSetting(delta int) {
	switch m.videoCursor {
	case videoItemResolution:
		index := m.currentResolutionIndex()
		index = wrapIndex(index+delta, len(videoResolutions))
		selected := videoResolutions[index]
		cvar.SetInt("vid_width", selected.width)
		cvar.SetInt("vid_height", selected.height)
	case videoItemFullscreen:
		cvar.SetBool("vid_fullscreen", !cvar.BoolValue("vid_fullscreen"))
	case videoItemVSync:
		cvar.SetBool("vid_vsync", !cvar.BoolValue("vid_vsync"))
	case videoItemMaxFPS:
		index := nearestMaxFPSIndex(cvar.IntValue("host_maxfps"))
		index = wrapIndex(index+delta, len(maxFPSValues))
		cvar.SetInt("host_maxfps", maxFPSValues[index])
	case videoItemGamma:
		gamma := cvar.FloatValue("r_gamma") + 0.1*float64(delta)
		gamma = clampFloat(gamma, 0.5, 1.5)
		cvar.SetFloat("r_gamma", roundToTenth(gamma))
	case videoItemViewModel:
		cvar.SetBool("r_drawviewmodel", !cvar.BoolValue("r_drawviewmodel"))
	case videoItemWaterwarp:
		// Cycle through 0=off, 1=screen warp, 2=FOV warp.
		next := (cvar.IntValue("r_waterwarp") + delta + 3) % 3
		cvar.SetInt("r_waterwarp", next)
	case videoItemHUDStyle:
		next := (cvar.IntValue("hud_style") + delta + 3) % 3
		cvar.SetInt("hud_style", next)
	case videoItemShowFPS:
		cvar.SetBool("scr_showfps", cvar.FloatValue("scr_showfps") == 0)
	}
}

// adjustAudioSetting modifies the audio cvar at the current cursor position
// by the given delta. Currently only s_volume is supported, clamped to [0, 1].
func (m *Manager) adjustAudioSetting(delta int) {
	if m.audioCursor != audioItemVolume {
		return
	}

	volume := cvar.FloatValue("s_volume") + 0.1*float64(delta)
	volume = clampFloat(volume, 0, 1)
	cvar.SetFloat("s_volume", roundToTenth(volume))
}

// controlBindingLabel returns the display string for the key bound to the
// action at the given Controls-menu index. Returns "UNBOUND" if no key is
// assigned, or "KEY +N" if multiple keys are bound.
func (m *Manager) controlBindingLabel(index int) string {
	command, ok := m.controlCommand(index)
	if !ok {
		return ""
	}
	keys := m.keysForBinding(command)
	if len(keys) == 0 {
		return "UNBOUND"
	}
	if len(keys) == 1 {
		return keys[0]
	}
	return fmt.Sprintf("%s +%d", keys[0], len(keys)-1)
}

// controlCommand maps a Controls-menu cursor index to the console command
// string for that binding row. Returns ("", false) if the index is outside
// the binding range.
func (m *Manager) controlCommand(index int) (string, bool) {
	bindingIndex := index - controlsBindingStart
	if bindingIndex < 0 || bindingIndex >= len(controlBindings) {
		return "", false
	}
	return controlBindings[bindingIndex].command, true
}

// setControlBinding clears the old binding for the action at index, then
// assigns the given key to the action's console command.
func (m *Manager) setControlBinding(index, key int) {
	command, ok := m.controlCommand(index)
	if !ok || m.inputSystem == nil || key < 0 || key >= input.NumKeycode {
		return
	}
	m.clearControlBinding(index)
	m.inputSystem.SetBinding(key, command)
}

// clearControlBinding removes all key bindings for the action at the given
// Controls-menu index by scanning every keycode and clearing matches.
func (m *Manager) clearControlBinding(index int) {
	command, ok := m.controlCommand(index)
	if !ok || m.inputSystem == nil {
		return
	}
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(m.inputSystem.GetBinding(key)) == command {
			m.inputSystem.SetBinding(key, "")
		}
	}
}

// keysForBinding scans all keycodes and returns the human-readable names of
// any keys currently bound to the given console command string.
func (m *Manager) keysForBinding(command string) []string {
	if m.inputSystem == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(m.inputSystem.GetBinding(key)) != command {
			continue
		}
		name := input.KeyToString(key)
		if name == "" {
			name = strconv.Itoa(key)
		}
		keys = append(keys, name)
	}
	return keys
}

// currentResolutionIndex returns the videoResolutions index that matches the
// current vid_width/vid_height cvars, or the nearest higher resolution if
// no exact match is found.
func (m *Manager) currentResolutionIndex() int {
	width := cvar.IntValue("vid_width")
	height := cvar.IntValue("vid_height")
	for i, mode := range videoResolutions {
		if mode.width == width && mode.height == height {
			return i
		}
	}
	return nearestResolutionIndex(width, height)
}

// nearestResolutionIndex returns the index of the first videoResolution whose
// width and height are >= the given values, or the last index if none qualifies.
func nearestResolutionIndex(width, height int) int {
	for i, mode := range videoResolutions {
		if mode.width >= width && mode.height >= height {
			return i
		}
	}
	return len(videoResolutions) - 1
}

// nearestMaxFPSIndex returns the index of the first maxFPSValues entry >= value,
// or the last index if all entries are below value.
func nearestMaxFPSIndex(value int) int {
	for i, maxFPS := range maxFPSValues {
		if maxFPS >= value {
			return i
		}
	}
	return len(maxFPSValues) - 1
}

// drawOptions renders the Options sub-menu with its five items: Controls,
// Video, Audio, VSync shortcut, and Back.
func (m *Manager) drawOptions(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	m.drawText(dc, 84, 32, "CONTROLS", true)
	m.drawText(dc, 84, 52, "VIDEO", true)
	m.drawText(dc, 84, 72, "AUDIO", true)
	m.drawText(dc, 84, 92, "VSYNC", true)
	m.drawText(dc, 84, 112, "BACK", true)

	m.drawCursor(dc, 54, 32+m.optionsCursor*20)
}

// waterwarpLabel returns a human-readable label for the r_waterwarp cvar value:
// 0 → "OFF", 1 → "SCREEN WARP", 2 → "FOV WARP".
func waterwarpLabel(v int) string {
	switch v {
	case 1:
		return "SCREEN WARP"
	case 2:
		return "FOV WARP"
	default:
		return "OFF"
	}
}

// hudStyleLabel returns a human-readable label for the hud_style cvar value:
// 0 → "CLASSIC", 1 → "COMPACT", 2 → "QUAKEWORLD".
func hudStyleLabel(v int) string {
	switch v {
	case 1:
		return "COMPACT"
	case 2:
		return "QUAKEWORLD"
	default:
		return "CLASSIC"
	}
}

// drawVideo renders the Video settings menu showing resolution, fullscreen,
// vsync, max FPS, gamma, viewmodel, waterwarp, HUD style, an FPS counter
// toggle, and a Back item. Each row displays the label on the left and the
// current cvar value on the right.
func (m *Manager) drawVideo(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	mode := videoResolutions[m.currentResolutionIndex()]
	m.drawText(dc, 56, 32, "RESOLUTION", true)
	m.drawText(dc, 184, 32, fmt.Sprintf("%dx%d", mode.width, mode.height), true)
	m.drawText(dc, 56, 48, "FULLSCREEN", true)
	m.drawText(dc, 184, 48, boolLabel(cvar.BoolValue("vid_fullscreen")), true)
	m.drawText(dc, 56, 64, "VSYNC", true)
	m.drawText(dc, 184, 64, boolLabel(cvar.BoolValue("vid_vsync")), true)
	m.drawText(dc, 56, 80, "MAX FPS", true)
	m.drawText(dc, 184, 80, fmt.Sprintf("%d", cvar.IntValue("host_maxfps")), true)
	m.drawText(dc, 56, 96, "GAMMA", true)
	m.drawText(dc, 184, 96, fmt.Sprintf("%.1f", cvar.FloatValue("r_gamma")), true)
	m.drawText(dc, 56, 112, "VIEWMODEL", true)
	m.drawText(dc, 184, 112, boolLabel(cvar.BoolValue("r_drawviewmodel")), true)
	m.drawText(dc, 56, 128, "WATERWARP", true)
	m.drawText(dc, 184, 128, waterwarpLabel(cvar.IntValue("r_waterwarp")), true)
	m.drawText(dc, 56, 144, "HUD STYLE", true)
	m.drawText(dc, 184, 144, hudStyleLabel(cvar.IntValue("hud_style")), true)
	m.drawText(dc, 56, 160, "SHOW FPS", true)
	m.drawText(dc, 184, 160, boolLabel(cvar.FloatValue("scr_showfps") != 0), true)
	m.drawText(dc, 56, 176, "BACK", true)

	m.drawArrowCursor(dc, 40, 32+m.videoCursor*16)
	m.drawText(dc, 40, 192, "VIDEO CHANGES ARE SAVED TO CONFIG", true)
}

// drawControls renders the Controls settings menu with sliders (mouse speed),
// toggles (invert mouse, always run, freelook), key-binding rows, and a Back
// item. A status line at the bottom shows context-sensitive help.
func (m *Manager) drawControls(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	m.drawText(dc, 32, controlRowY(controlItemMouseSpeed), "MOUSE SPEED", true)
	m.drawText(dc, 208, controlRowY(controlItemMouseSpeed), fmt.Sprintf("%.1f", cvar.FloatValue("sensitivity")), true)
	m.drawText(dc, 32, controlRowY(controlItemInvertMouse), "INVERT MOUSE", true)
	m.drawText(dc, 208, controlRowY(controlItemInvertMouse), boolLabel(cvar.FloatValue("m_pitch") < 0), true)
	m.drawText(dc, 32, controlRowY(controlItemAlwaysRun), "ALWAYS RUN", true)
	m.drawText(dc, 208, controlRowY(controlItemAlwaysRun), boolLabel(cvar.BoolValue("cl_alwaysrun")), true)
	m.drawText(dc, 32, controlRowY(controlItemFreeLook), "MOUSE LOOK", true)
	m.drawText(dc, 208, controlRowY(controlItemFreeLook), boolLabel(cvar.BoolValue("freelook")), true)

	for i, binding := range controlBindings {
		y := controlRowY(controlsBindingStart + i)
		m.drawText(dc, 40, y, binding.label, true)
		m.drawText(dc, 200, y, m.controlBindingLabel(controlsBindingStart+i), true)
	}
	m.drawText(dc, 40, controlRowY(controlItemBack), "BACK", true)

	m.drawArrowCursor(dc, 24, controlRowY(m.controlsCursor))
	if m.controlsRebinding {
		m.drawText(dc, 24, 176, "PRESS A KEY OR ESC TO CANCEL", true)
		return
	}
	if m.controlsCursor < controlsBindingStart {
		m.drawText(dc, 24, 176, "LEFT/RIGHT/ENTER CHANGE, ESC BACK", true)
		return
	}
	m.drawText(dc, 24, 176, "ENTER/RIGHT BIND LEFT/BKSP CLEAR", true)
}

// drawAudio renders the Audio settings menu with a volume percentage bar and
// a Back item.
func (m *Manager) drawAudio(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	volumePercent := int(clampFloat(cvar.FloatValue("s_volume"), 0, 1)*100 + 0.5)
	m.drawText(dc, 72, 56, "SOUND VOLUME", true)
	m.drawText(dc, 200, 56, fmt.Sprintf("%d%%", volumePercent), true)
	m.drawText(dc, 72, 88, "BACK", true)

	m.drawArrowCursor(dc, 56, 56+m.audioCursor*32)
}
