package main

import (
	"os"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/game"
)

// TestMain ensures the package-level `g` *game.Game pointer is initialised
// before any cmd-package tests run, matching the production main() setup.
func TestMain(m *testing.M) {
	g = game.New()
	os.Exit(m.Run())
}
