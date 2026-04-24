package main

import (
	"log/slog"
)

func pollRuntimeInputEvents() {
	if g.Input == nil {
		return
	}
	if g.Input.PollEvents() {
		return
	}
	if g.Host != nil && !g.Host.IsAborted() {
		g.Host.CmdQuit()
	}
}

func shutdownEngine() {
	if g.Host == nil {
		return
	}

	if path, active, err := g.StopCPUProfile(); active {
		if err != nil {
			slog.Error("Failed to close active CPU profile during shutdown", "path", path, "error", err)
		} else {
			slog.Info("Stopped active CPU profile during shutdown", "path", path)
		}
	}

	g.Host.PrepareForShutdown(g.Subs)

	if g.CSQC != nil && g.CSQC.IsLoaded() {
		if err := g.CSQC.CallShutdown(); err != nil {
			slog.Error("CSQC_Shutdown failed", "error", err)
		}
		g.CSQC.Unload()
	}

	g.Host.Net.Shutdown()
	g.Host.Shutdown(g.Subs)
	slog.Info("Engine shutdown complete")
}
