// Package gameconfig centralises every Quake-specific identity, default, and
// gate that a standalone mod or total conversion may need to override.
//
// The zero value of Config is intentionally empty: use Default() for a
// Config that reproduces stock Quake / Ironwail-Go behaviour exactly. The
// resolve() method fills zero-value fields from the defaults so a mod can
// override only the fields it cares about.
//
// This package is imported by the engine (internal/game, internal/fs,
// internal/net, internal/host) and by standalone mod binaries. It must not
// import any other project package.
package gameconfig

import "strings"

// ProtocolNumbers holds the QCVM protocol version constants a build may use.
// Stock Quake uses NetQuake (15); FitzQuake and RMQ are extensions.
type ProtocolNumbers struct {
	NetQuake  int
	FitzQuake int
	RMQ       int
}

// Features controls which engine affordances are available. All default to
// true; a standalone mod can disable menus, console, or commands it does
// not want players to access. (Not yet threaded — see spec §11.6.)
type Features struct {
	Console      bool
	SinglePlayer bool
	Multiplayer  bool
	DemoPlayback bool
	SaveLoad     bool
	Cheats       bool
}

// Config is the game's identity, defaults, and gates. Created at startup
// and threaded to the subsystems that read it. Never mutated after init.
type Config struct {
	// Identity.
	GameName    string // window title, console title, CSQC init name
	BaseGameDir string // required base data directory (stock: "id1")
	UserDirName string // user config dir under $HOME (stock: ".ironwail")

	// Licensing / registration gate.
	RequireRegistered     bool   // when true, non-base mods require `registered 1`
	DefaultRegistered     bool   // initial value of the `registered` cvar
	RegisteredMessage     string // "Playing registered version.\n"
	SharewareMessage      string // "Playing shareware version.\n"
	ModRequiresRegistered string // error when mods are blocked
	RegistrationCVarName  string // "registered"

	// Network identity. ProtocolMagic is the connection handshake identity
	// (stock: "QUAKE\x00"); ProtocolVer is the version byte in handshake
	// and server info responses (stock: 3).
	ProtocolMagic []byte
	ProtocolVer   int
	ProtocolNums  ProtocolNumbers

	// Default cvar values (stock: skill 1, deathmatch 0, coop 0, teamplay 0).
	DefaultSkill      int
	DefaultDeathmatch int
	DefaultCoop       int
	DefaultTeamplay   int

	// Menu strings. Empty string hides the menu entry.
	ModDirMenuLabel string // "QUAKE DIRECTORY"
	NetOptionLabel  string // "QUAKEWORLD"

	// Feature toggles. Zero value (all false) resolves to all-true.
	Features Features

	// Engine name used for CSQC init (stock: "Ironwail").
	CSQCInitName string
}

// Default returns a Config that reproduces stock Quake / Ironwail-Go
// behaviour exactly. Every field matches the current hard-coded literals.
func Default() Config {
	return Config{
		GameName:    "Ironwail-Go",
		BaseGameDir: "id1",
		UserDirName: ".ironwail",

		RequireRegistered:     true,
		DefaultRegistered:     false,
		RegisteredMessage:     "Playing registered version.\n",
		SharewareMessage:      "Playing shareware version.\n",
		ModRequiresRegistered: "you must have the registered version to use modified games",
		RegistrationCVarName:  "registered",

		ProtocolMagic: append([]byte(nil), "QUAKE\x00"...),
		ProtocolVer:   3,
		ProtocolNums: ProtocolNumbers{
			NetQuake:  15,
			FitzQuake: 666,
			RMQ:       999,
		},

		DefaultSkill:      1,
		DefaultDeathmatch: 0,
		DefaultCoop:       0,
		DefaultTeamplay:   0,

		ModDirMenuLabel: "QUAKE DIRECTORY",
		NetOptionLabel:  "QUAKEWORLD",

		CSQCInitName: "Ironwail",

		Features: Features{
			Console:      true,
			SinglePlayer: true,
			Multiplayer:  true,
			DemoPlayback: true,
			SaveLoad:     true,
			Cheats:       true,
		},
	}
}

// resolve returns a Config with zero-value fields filled from Default().
// A standalone mod constructs a Config with only the fields it cares about;
// resolve fills in the rest so downstream code never sees a zero value.
func (c Config) resolve() Config {
	def := Default()
	if c.GameName == "" {
		c.GameName = def.GameName
	}
	if c.BaseGameDir == "" {
		c.BaseGameDir = def.BaseGameDir
	}
	if c.UserDirName == "" {
		c.UserDirName = def.UserDirName
	}
	if c.ProtocolMagic == nil {
		c.ProtocolMagic = def.ProtocolMagic
	}
	if c.ProtocolVer == 0 {
		c.ProtocolVer = def.ProtocolVer
	}
	if c.ProtocolNums == (ProtocolNumbers{}) {
		c.ProtocolNums = def.ProtocolNums
	}
	if c.RegistrationCVarName == "" {
		c.RegistrationCVarName = def.RegistrationCVarName
	}
	if c.RegisteredMessage == "" {
		c.RegisteredMessage = def.RegisteredMessage
	}
	if c.SharewareMessage == "" {
		c.SharewareMessage = def.SharewareMessage
	}
	if c.ModRequiresRegistered == "" {
		c.ModRequiresRegistered = def.ModRequiresRegistered
	}
	if c.ModDirMenuLabel == "" {
		c.ModDirMenuLabel = def.ModDirMenuLabel
	}
	if c.NetOptionLabel == "" {
		c.NetOptionLabel = def.NetOptionLabel
	}
	if c.CSQCInitName == "" {
		c.CSQCInitName = def.CSQCInitName
	}
	// Bool fields: false is a meaningful value for the gate fields (e.g.
	// RequireRegistered=false means "don't gate"), so they are NOT resolved
	// from defaults. A mod that wants the gate must set it explicitly.
	// Features: all-false resolves to all-true (backward compatibility).
	if !c.Features.Console && !c.Features.SinglePlayer && !c.Features.Multiplayer &&
		!c.Features.DemoPlayback && !c.Features.SaveLoad && !c.Features.Cheats {
		c.Features = def.Features
	}
	return c
}

// DefaultRegisteredInt returns the initial `registered` cvar value as a
// string ("0" or "1"), for cvar.Register.
func (c Config) DefaultRegisteredInt() string {
	if c.DefaultRegistered {
		return "1"
	}
	return "0"
}

// BaseGameDirLower returns the lowercased base game dir for comparisons
// (matching the existing strings.ToLower(strings.TrimSpace(...)) pattern).
func (c Config) BaseGameDirLower() string {
	return strings.ToLower(strings.TrimSpace(c.BaseGameDir))
}
