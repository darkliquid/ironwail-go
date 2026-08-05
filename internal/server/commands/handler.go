// Package commands implements server-side user command handlers and cheat controls.
package commands

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// Handler manages server console commands and user cheat requests.
type Handler struct {
	cvars *cvar.CVarSystem
}

// NewHandler creates a new server user command handler wrapping the cvar registry.
func NewHandler(cvars *cvar.CVarSystem) *Handler {
	return &Handler{
		cvars: cvars,
	}
}

// AllowedCommands returns the list of valid user commands accepted by the server.
func AllowedCommands() []string {
	return []string{
		"status", "god", "notarget", "fly", "name", "noclip", "setpos",
		"say", "say_team", "tell", "color", "kill", "pause", "spawn",
		"begin", "prespawn", "kick", "ping", "give", "ban",
	}
}
