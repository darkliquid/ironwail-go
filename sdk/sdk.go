// Package sdk is the public entry point for standalone mods and total
// conversions built on the Ironwail-Go engine. It is the importable face of
// the engine SDK (SPEC-006): a mod's main() constructs a Config, calls Run,
// and gets a runnable game.
//
// The package is intentionally a thin re-export of internal packages. Mod
// binaries live in their own Go modules, which Go's internal-package rule
// forbids from importing github.com/darkliquid/ironwail-go/internal/...;
// this public facade is what makes the SDK importable from outside the
// engine module.
//
// The surface grows as the SPEC-006 §11 extensibility architecture lands
// (subsystem registry, command restriction, post-FX hooks). Only stable,
// mod-facing API belongs here.
package sdk

import (
	"github.com/darkliquid/ironwail-go/internal/engine"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

// Config is a mod's identity and feature configuration. It is the
// gameconfig.Config struct re-exported for mod binaries; zero-value fields
// resolve to stock Quake defaults via Resolve.
type Config = gameconfig.Config

// Features toggles built-in engine subsystems on and off.
type Features = gameconfig.Features

// ProtocolNumbers selects the network protocol identities served by the
// engine (stock: NetQuake 15, FitzQuake 666, RMQ 999).
type ProtocolNumbers = gameconfig.ProtocolNumbers

// Option configures the engine bootstrap.
type Option = engine.RunOption

// Headless runs without rendering (dedicated server or automated testing).
func Headless() Option { return engine.Headless() }

// Args passes command-line arguments (same format as the engine binary).
func Args(args ...string) Option { return engine.Args(args...) }

// Run boots the engine with the given configuration and returns the
// initialised Game. The caller owns the game loop afterwards.
func Run(config Config, opts ...Option) (*game.Game, error) {
	return engine.Run(config, opts...)
}