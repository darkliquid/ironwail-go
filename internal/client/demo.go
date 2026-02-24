package client

import "os"

type DemoFrame struct {
	FileOffset       int64
	Intermission     int
	ForceUnderwater  bool
	SerializedEvents int
}

type DemoState struct {
	File       *os.File
	Playback   bool
	Recording  bool
	Paused     bool
	Speed      float32
	BaseSpeed  float32
	TimeDemo   bool
	FrameIndex int

	Frames []DemoFrame
}

func (c *Client) ClearSignons() {
	c.Signon = 0
}

func (c *Client) AdvanceTime(demo *DemoState, frametime float64) {
	c.OldTime = c.Time
	if demo != nil && demo.Playback {
		speed := float64(demo.Speed)
		if demo.Paused {
			speed = 0
		}
		c.Time += speed * frametime
		return
	}
	c.Time += frametime
}

func (c *Client) FinishDemoFrame() {
}

func (c *Client) StopPlayback(demo *DemoState) {
	if demo == nil || !demo.Playback {
		return
	}
	if demo.File != nil {
		_ = demo.File.Close()
	}
	demo.File = nil
	demo.Playback = false
	demo.Paused = false
	demo.Speed = 1
	demo.Frames = nil
	demo.FrameIndex = 0
	c.State = StateDisconnected
}
