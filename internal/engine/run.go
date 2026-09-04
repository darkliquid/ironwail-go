// Package engine provides the SDK entry point for standalone mods and
// total conversions. A mod binary imports this package, provides a
// gameconfig.Config and gameplay hooks, and gets a runnable game.
package engine

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

// RunOption configures the engine bootstrap.
type RunOption func(*runOpts)

type runOpts struct {
	headless bool
	args     []string
}

// Headless runs without rendering (dedicated server or automated testing).
func Headless() RunOption { return func(o *runOpts) { o.headless = true } }

// Args passes command-line arguments (same format as the engine binary).
func Args(args ...string) RunOption { return func(o *runOpts) { o.args = args } }

// Run boots the engine with the given configuration and returns the
// initialised Game. The caller is responsible for the game loop (or can
// use RunMain for the full bootstrap including the renderer loop).
//
// This is the SDK entry point for standalone mods: a mod binary's main()
// constructs a gameconfig.Config, calls engine.Run, and the engine handles
// the rest. The Config determines the game's identity, data paths, and
// feature gates.
func Run(config gameconfig.Config, opts ...RunOption) (*game.Game, error) {
	o := &runOpts{}
	for _, opt := range opts {
		opt(o)
	}

	config = config.Resolve()

	g := game.New()
	g.Config = config

	baseDir := "."
	args := o.args
	if len(args) > 0 {
		args = args[1:] // strip program name
	}

	headless := o.headless
	if err := g.InitSubsystems(headless, false, 1, baseDir, config.BaseGameDir, args); err != nil {
		return nil, fmt.Errorf("engine: init subsystems: %w", err)
	}
	return g, nil
}
