package main

import (
	"os"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/game"
)

// TestMain ensures the package-level `g` *game.Game pointer is initialised
// before any cmd-package tests run, matching the production main() setup.
func TestMain(m *testing.M) {
	g = game.New()
	os.Exit(m.Run())
}

// markCurrentPredictionFresh marks the current predicted frame as valid for
// the client's view entity. Used by cmd-level tests that exercise telemetry
// and view-calculation code paths that rely on prediction state.
func markCurrentPredictionFresh(c *cl.Client) {
	if c == nil {
		return
	}
	entNum := c.ViewEntity
	if entNum == 0 {
		if _, ok := c.Entities[1]; ok {
			entNum = 1
		}
	}
	c.PredictionValid = true
	c.PredictionEntityNum = entNum
	c.PredictionFrameTime = c.Time
}
