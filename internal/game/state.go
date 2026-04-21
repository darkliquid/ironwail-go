package game

import (
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// SpriteModel holds a sprite model reference.
type SpriteModel struct {
	Model  *model.Model
	Sprite *model.MSprite
}

// TextEditRepeatState tracks text editing key repeat state.
type TextEditRepeatState struct {
	Key       int
	NextDelay float64
}

// FPSOverlay tracks FPS display state.
type FPSOverlay struct {
	OldTime       float64
	LastFPS       float64
	OldFrameCount int
}

// SpeedOverlay tracks speed display state.
type SpeedOverlay struct {
	MaxSpeed     float32
	DisplaySpeed float32
	LastRealTime float64
}

// DemoOverlay tracks demo playback overlay state.
type DemoOverlay struct {
	PrevSpeed     float32
	PrevBaseSpeed float32
	ShowTime      float64
}

// TelemetryState holds telemetry information for debugging/display.
type TelemetryState struct {
	RealTime        float64
	FrameCount      int
	FrameTime       float64
	ViewSize        float32
	HUDStyle        int
	ShowFPS         float32
	ShowClock       int
	ShowSpeed       bool
	ShowTurtle      bool
	ShowSpeedOfs    float32
	ClientTime      float64
	Intermission    int
	InCutscene      bool
	DemoPlayback    bool
	DemoSpeed       float32
	DemoBaseSpeed   float32
	DemoProgress    float64
	DemoName        string
	DemoBarTimeout  float32
	ClientActive    bool
	Velocity        [3]float32
	ConsoleForced   bool
	LastServerMsgAt float64
	SavingActive    bool
	ViewRect        renderer.ViewRect
}
