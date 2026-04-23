// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// M4 parity telemetry for the mixer. The cvar gates `snddbg` lines
// that surface per-channel spatialisation + filter state so it can be
// diffed against reference ironwail snd_dma.c.

const SndDebugMixerCVarName = "snd_debug_mixer"

var (
	sndDebugMixerCVar *cvar.CVar
	// SnddbgEmit is the telemetry sink; overridable in tests.
	SnddbgEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterSnddbgCVars registers snd_debug_mixer.
func RegisterSnddbgCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	sndDebugMixerCVar = cv.Register(SndDebugMixerCVarName, "0", cvar.FlagNone,
		"Audio mixer debug telemetry (0=off, 1=events, 2=verbose)")
}

func snddbgLevel() int {
	if sndDebugMixerCVar == nil {
		return 0
	}
	return sndDebugMixerCVar.Int
}

// SnddbgLogf emits at level>=1.
func SnddbgLogf(format string, args ...any) {
	if snddbgLevel() < 1 {
		return
	}
	SnddbgEmit("[snddbg] " + fmt.Sprintf(format, args...))
}

// SnddbgLogfAt emits at level>=the given level.
func SnddbgLogfAt(level int, format string, args ...any) {
	if snddbgLevel() < level {
		return
	}
	SnddbgEmit("[snddbg] " + fmt.Sprintf(format, args...))
}
