package main

import (
	"log/slog"

	inet "github.com/ironwail/ironwail-go/internal/net"
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

func releaseRuntimeRenderer() {
	if g.Renderer != nil {
		g.Renderer.Shutdown()
		g.Renderer = nil
	}
	if g.Subs != nil {
		g.Subs.Renderer = nil
	}
}

func shutdownEngine() {
	if g.Host == nil {
		return
	}

	g.Host.PrepareForShutdown(g.Subs)

	if g.CSQC != nil && g.CSQC.IsLoaded() {
		if err := g.CSQC.CallShutdown(); err != nil {
			slog.Error("CSQC_Shutdown failed", "error", err)
		}
		g.CSQC.Unload()
	}

	inet.Shutdown()
	g.Host.Shutdown(g.Subs)
	slog.Info("Engine shutdown complete")
}
