// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"log/slog"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

type FrameStats struct {
	GameTime float64
	Server   float64
	Client   float64
	Render   float64
	Audio    float64
	Total    float64
}

type FrameCallbacks interface {
	GetEvents()
	ProcessConsoleCommands()
	ProcessServer()
	ProcessClient()
	UpdateScreen()
	UpdateAudio(origin, forward, right, up [3]float32)
}

func (h *Host) GetFrameInterval() float64 {
	if h.maxFPS > 0 || h.clientState == caDisconnected {
		maxfps := h.maxFPS
		if h.clientState == caDisconnected {
			maxfps = 60
			if h.maxFPS > 0 && h.maxFPS < maxfps {
				maxfps = h.maxFPS
			}
		}
		maxfps = clamp(maxfps, 10, 1000)
		return 1.0 / maxfps
	}
	return 0.0
}

func (h *Host) advanceTime(dt float64) {
	h.realtime += dt
	h.frameTime = dt
	h.rawFrameTime = dt

	if h.timeScale > 0 {
		h.frameTime *= h.timeScale
	} else if h.framerate > 0 {
		h.frameTime = 1.0 / h.framerate
	} else if h.maxFPS > 0 {
		h.frameTime = clamp(h.frameTime, 0.0001, 0.1)
	}
}

func (h *Host) Frame(dt float64, cb FrameCallbacks) error {
	frameStart := time.Now()
	if h.aborted {
		return nil
	}

	h.advanceTime(dt)

	if cb != nil {
		cb.GetEvents()
		cb.ProcessConsoleCommands()

		if h.netInterval > 0 {
			h.accumTime += clamp(dt, 0, 0.2)
		}

		if h.accumTime >= h.netInterval {
			realFrameTime := h.frameTime
			if h.netInterval > 0 {
				h.frameTime = h.accumTime
				if h.frameTime < h.netInterval {
					h.frameTime = h.netInterval
				}
				h.accumTime -= h.frameTime
				if h.timeScale > 0 {
					h.frameTime *= h.timeScale
				} else if h.framerate > 0 {
					h.frameTime = 1.0 / h.framerate
				}
			} else {
				h.accumTime -= h.netInterval
			}

			cb.ProcessClient() // This should be CL_SendCmd

			if h.serverActive {
				cb.ProcessServer()
			}
			h.frameTime = realFrameTime
		}

		if h.clientState == caConnected || h.clientState == caActive {
			cb.ProcessClient() // This should be CL_ReadFromServer
		}

		cb.UpdateScreen()

		if h.signOns >= 4 {
			cb.UpdateAudio([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})
		} else {
			cb.UpdateAudio([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})
		}
	}

	h.frameCount++

	if cvar.BoolValue("host_speeds") {
		elapsed := time.Since(frameStart)
		slog.Debug("frame timing", "ms", elapsed.Milliseconds(), "frame", h.frameCount)
	}

	return nil
}

func (h *Host) FrameLoop(targetFPS float64, cb FrameCallbacks, shouldQuit func() bool) error {
	frameInterval := 1.0 / targetFPS
	lastTime := currentTime()

	for !shouldQuit() && !h.aborted {
		now := currentTime()
		dt := now - lastTime

		frameTarget := h.GetFrameInterval()
		if frameTarget <= 0 {
			frameTarget = frameInterval
		}

		if dt < frameTarget {
			sleepTime := frameTarget - dt
			if sleepTime > 0.001 {
				time.Sleep(time.Duration(sleepTime * float64(time.Second)))
			}
			now = currentTime()
			dt = now - lastTime
		}

		lastTime = now

		if err := h.Frame(dt, cb); err != nil {
			return err
		}
	}

	if h.aborted {
		return &HostError{Message: h.abortReason}
	}

	return nil
}

type HostError struct {
	Message string
}

func (e *HostError) Error() string {
	return e.Message
}

func HostEndGame(h *Host, format string, args ...interface{}) {
	h.Abort(format)
}
