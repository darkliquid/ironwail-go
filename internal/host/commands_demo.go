// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"github.com/ironwail/ironwail-go/internal/client"
)

func (h *Host) CmdRecord(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Already recording a demo. Use 'stop' to end recording.\n")
		return
	}

	// Check if playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Cannot record during demo playback.\n")
		return
	}

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Get CD track (default to -1, meaning no forced track, matching C Ironwail)
	cdtrack := -1
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil && loopbackClient.CDTrack > 0 {
		cdtrack = loopbackClient.CDTrack
	}

	// Start recording
	if err := h.demoState.StartDemoRecording(filename, cdtrack); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start recording: %v\n", err))
		return
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil && loopbackClient.State != client.StateDisconnected && loopbackClient.Signon > 0 {
		if err := h.demoState.WriteInitialStateSnapshot(loopbackClient); err != nil {
			stopErr := h.demoState.StopRecording()
			if stopErr != nil {
				subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v (also failed to close demo: %v)\n", err, stopErr))
				return
			}
			subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v\n", err))
			return
		}
	}

	subs.Console.Print(fmt.Sprintf("Recording demo to %s\n", h.demoState.Filename))
}

func (h *Host) CmdStop(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Recording {
		subs.Console.Print("Not recording a demo.\n")
		return
	}

	var viewAngles [3]float32
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		viewAngles = loopbackClient.ViewAngles
	}

	trailerErr := h.demoState.WriteDisconnectTrailer(viewAngles)
	stopErr := h.demoState.StopRecording()
	if trailerErr != nil {
		if stopErr != nil {
			subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v (also failed to close demo: %v)\n", trailerErr, stopErr))
			return
		}
		subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v\n", trailerErr))
		return
	}
	if stopErr != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo: %v\n", stopErr))
		return
	}

	subs.Console.Print("Completed demo\n")
}

func (h *Host) CmdPlaydemo(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Already playing back a demo.\n")
		return
	}

	// Check if recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Cannot playback while recording.\n")
		return
	}

	// Disconnect from any current server
	if h.serverActive {
		h.ShutdownServer(subs)
	}
	h.clientState = caDisconnected

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Start playback
	if err := h.demoState.StartDemoPlayback(filename); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start playback: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Playing demo from %s\n", h.demoState.Filename))

	// Set client state to connected for demo playback
	h.clientState = caConnected

	// Reset the actual client so recorded serverinfo/signon frames can bootstrap playback.
	if clientState := LoopbackClientState(subs); clientState != nil {
		clientState.ClearState()
		clientState.State = cl.StateDisconnected
	}
}

func (h *Host) CmdStopdemo(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}

	if err := h.demoState.StopPlayback(); err != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		return
	}

	subs.Console.Print("Demo playback stopped.\n")
	h.clientState = caDisconnected
	h.SetDemoNum(-1)
}

// MaxDemos is the maximum number of demos in a startdemos playlist.
const MaxDemos = 8

// CmdStartdemos stores a list of demo names for attract-mode cycling.
// If no game is active it begins playback immediately.
func (h *Host) CmdStartdemos(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if len(args) == 0 {
		subs.Console.Print("usage: startdemos <demo1> [demo2] ...\n")
		return
	}

	count := len(args)
	if count > MaxDemos {
		count = MaxDemos
	}
	h.SetDemoList(args[:count])
	h.SetDemoNum(0)

	// If no game is in progress, start playing the first demo now.
	if h.clientState == caDisconnected && !h.serverActive {
		h.CmdDemos(subs)
	}
}

// CmdDemos restarts the demo loop from the current position.
func (h *Host) CmdDemos(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.DemoNum() < 0 {
		subs.Console.Print("No demo loop active.\n")
		return
	}

	demos := h.DemoList()
	if len(demos) == 0 {
		h.SetDemoNum(-1)
		return
	}

	num := h.DemoNum()
	// Wrap around when we reach the end.
	if num >= len(demos) || demos[num] == "" {
		num = 0
		h.SetDemoNum(num)
		if len(demos) == 0 || demos[0] == "" {
			h.SetDemoNum(-1)
			return
		}
	}

	h.CmdPlaydemo(demos[num], subs)
	h.SetDemoNum(num + 1)
}

func (h *Host) CmdTimedemo(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	h.CmdPlaydemo(filename, subs)
	if h.demoState == nil || !h.demoState.Playback {
		return
	}
	h.demoState.EnableTimeDemo()
	subs.Console.Print(fmt.Sprintf("Timing demo %s\n", h.demoState.Filename))
}

func (h *Host) CmdDemoSeek(frame int, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	if frame < 0 || frame >= len(h.demoState.Frames) {
		subs.Console.Print(fmt.Sprintf("Frame %d out of range (0-%d).\n", frame, len(h.demoState.Frames)))
		return
	}
	if err := h.seekDemoFrame(frame, subs); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to seek demo: %v\n", err))
		return
	}
	subs.Console.Print(fmt.Sprintf("Demo seeked to frame %d.\n", frame))
}

func (h *Host) CmdRewind(frames int, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	if frames <= 0 {
		frames = 1
	}
	target := h.demoState.FrameIndex - frames
	if target < 0 {
		target = 0
	}
	h.CmdDemoSeek(target, subs)
}

func (h *Host) seekDemoFrame(frame int, subs *Subsystems) error {
	if h.demoState == nil {
		return fmt.Errorf("demo state unavailable")
	}
	clientState := LoopbackClientState(subs)
	if clientState == nil {
		return fmt.Errorf("loopback client state unavailable")
	}
	if err := h.demoState.SeekFrame(0); err != nil {
		return err
	}
	clientState.ClearState()
	clientState.State = cl.StateDisconnected
	h.clientState = caConnected
	h.signOns = 0

	parser := cl.NewParser(clientState)
	for i := 0; i < frame; i++ {
		msgData, viewAngles, err := h.demoState.ReadDemoFrame()
		if err != nil {
			return fmt.Errorf("read frame %d: %w", i, err)
		}
		clientState.MViewAngles[1] = clientState.MViewAngles[0]
		clientState.MViewAngles[0] = viewAngles
		clientState.ViewAngles = viewAngles
		if err := parser.ParseServerMessage(msgData); err != nil {
			return fmt.Errorf("parse frame %d: %w", i, err)
		}
		DispatchLoopbackStuffText(subs)
	}
	return nil
}

func (h *Host) CmdDemoGoto(seconds float64, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	if seconds < 0 {
		seconds = 0
	}
	frame := h.demoState.FrameForTime(seconds)
	if err := h.seekDemoFrame(frame, subs); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to seek demo: %v\n", err))
		return
	}
	subs.Console.Print(fmt.Sprintf("Demo seeked to %.2fs (frame %d).\n", seconds, frame))
}

func (h *Host) CmdDemoPause(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	paused := h.demoState.TogglePause()
	if paused {
		subs.Console.Print("Demo paused.\n")
	} else {
		subs.Console.Print("Demo resumed.\n")
	}
}

func (h *Host) CmdDemoSpeed(speed float32, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	h.demoState.SetSpeed(speed)
	subs.Console.Print(fmt.Sprintf("Demo speed set to %.2f.\n", h.demoState.Speed))
}
