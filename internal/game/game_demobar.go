package game

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/quakeui/demobar"
)

// gameDemoBarState adapts the engine's demo playback state to the display-only
// demo bar widget (ADR-0015). It mirrors the legacy drawRuntimeDemoControls
// show-time logic: the bar shows while a demo plays for scr_demobar_timeout
// seconds after any speed change (0 = always, <0 = never).
type gameDemoBarState struct {
	g *Game
}

// DemoBarState returns the per-frame snapshot for the demo bar widget.
func (s *gameDemoBarState) DemoBarState() demobar.DemoBarState {
	if s == nil || s.g == nil || s.g.Host == nil {
		return demobar.DemoBarState{}
	}
	demo := s.g.Host.DemoState()
	if demo == nil || !demo.Playback {
		return demobar.DemoBarState{}
	}

	timeout := float32(0)
	if s.g.Host.CVar != nil {
		timeout = float32(s.g.Host.CVar.FloatValue("scr_demobar_timeout"))
	}
	if timeout < 0 {
		return demobar.DemoBarState{Playback: true, Show: false}
	}

	show := false
	ov := &s.g.DemoOverlay
	if demo.Speed != ov.PrevSpeed ||
		demo.BaseSpeed != ov.PrevBaseSpeed ||
		math.Abs(float64(demo.Speed)) > math.Abs(float64(demo.BaseSpeed)) ||
		timeout == 0 {
		ov.PrevSpeed = demo.Speed
		ov.PrevBaseSpeed = demo.BaseSpeed
		ov.ShowTime = 1
		if timeout > 0 {
			ov.ShowTime = float64(timeout)
		}
		show = true
	} else {
		ov.ShowTime -= s.g.Host.FrameTime()
		if ov.ShowTime < 0 {
			ov.ShowTime = 0
		} else {
			show = true
		}
	}

	clientTime := 0.0
	if s.g.Client != nil {
		clientTime = s.g.Client.Time
	}
	return demobar.DemoBarState{
		Playback:   true,
		Show:       show,
		Speed:      demo.Speed,
		BaseSpeed:  demo.BaseSpeed,
		Progress:   demo.Progress(),
		Name:       s.g.runtimeDemoName(demo.Filename),
		ClientTime: clientTime,
	}
}
