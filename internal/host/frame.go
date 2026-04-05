// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"log/slog"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func elapsedMilliseconds(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}

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

type processClientPhaseAware interface {
	SetProcessClientPhase(phase string)
}

func setProcessClientPhase(cb FrameCallbacks, phase string) {
	if aware, ok := cb.(processClientPhaseAware); ok {
		aware.SetProcessClientPhase(phase)
	}
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
	} else {
		h.frameTime = clamp(h.frameTime, 0.0001, 0.1)
	}
}

func (h *Host) Frame(dt float64, cb FrameCallbacks) error {
	frameStart := time.Now()
	hostSpeeds := cvar.BoolValue("host_speeds")
	var frameStats FrameStats
	var eventMS float64
	var consoleMS float64
	var clientSendMS float64
	var clientReadMS float64
	if h.aborted {
		return nil
	}

	if h.compatRNG != nil {
		h.compatRNG.Int()
	}

	h.advanceTime(dt)
	frameStats.GameTime = h.frameTime * 1000.0

	if cb != nil {
		phaseStart := time.Time{}
		if hostSpeeds {
			phaseStart = time.Now()
		}
		cb.GetEvents()
		if hostSpeeds {
			eventMS = elapsedMilliseconds(phaseStart)
			phaseStart = time.Now()
		}
		cb.ProcessConsoleCommands()
		if hostSpeeds {
			consoleMS = elapsedMilliseconds(phaseStart)
		}

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

			setProcessClientPhase(cb, "send")
			if hostSpeeds {
				phaseStart = time.Now()
			}
			cb.ProcessClient() // This should be CL_SendCmd
			if hostSpeeds {
				clientSendMS = elapsedMilliseconds(phaseStart)
			}
			setProcessClientPhase(cb, "")

			if h.serverActive {
				if hostSpeeds {
					phaseStart = time.Now()
				}
				cb.ProcessServer()
				h.checkAutosave(h.Subs)
				if hostSpeeds {
					frameStats.Server = elapsedMilliseconds(phaseStart)
				}
			}
			h.frameTime = realFrameTime
		}

		if h.clientState == caConnected || h.clientState == caActive {
			setProcessClientPhase(cb, "read")
			if hostSpeeds {
				phaseStart = time.Now()
			}
			cb.ProcessClient() // This should be CL_ReadFromServer
			if hostSpeeds {
				clientReadMS = elapsedMilliseconds(phaseStart)
			}
			setProcessClientPhase(cb, "")
		}

		if hostSpeeds {
			phaseStart = time.Now()
		}
		cb.UpdateScreen()
		if hostSpeeds {
			frameStats.Render = elapsedMilliseconds(phaseStart)
			phaseStart = time.Now()
		}

		if h.signOns >= 4 {
			cb.UpdateAudio([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})
		} else {
			cb.UpdateAudio([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})
		}
		if hostSpeeds {
			frameStats.Audio = elapsedMilliseconds(phaseStart)
		}
	}

	h.frameCount++
	frameStats.Client = clientSendMS + clientReadMS
	frameStats.Total = elapsedMilliseconds(frameStart)

	if hostSpeeds {
		slog.Info("host_speeds",
			"frame", h.frameCount,
			"game_ms", frameStats.GameTime,
			"events_ms", eventMS,
			"console_ms", consoleMS,
			"client_send_ms", clientSendMS,
			"server_ms", frameStats.Server,
			"client_read_ms", clientReadMS,
			"render_ms", frameStats.Render,
			"audio_ms", frameStats.Audio,
			"total_ms", frameStats.Total,
		)
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
