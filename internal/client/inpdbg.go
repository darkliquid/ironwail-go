// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// M4 parity telemetry for input processing. The cvar gates `inpdbg`
// lines that surface raw accumulator values vs processed view angles
// for cross-checking against reference ironwail in_*.c behaviour.

const InDebugCVarName = "in_debug"

var (
	inDebugCVar *cvar.CVar
	// InpdbgEmit is the telemetry sink; overridable in tests.
	InpdbgEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterInDebugCVars registers in_debug.
func RegisterInDebugCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	inDebugCVar = cv.Register(InDebugCVarName, "0", cvar.FlagNone,
		"Input debug telemetry (0=off, 1=events, 2=verbose)")
}

func InDebugLevel() int {
	return inpdbgLevel()
}

func inpdbgLevel() int {
	if inDebugCVar == nil {
		return 0
	}
	return inDebugCVar.Int
}

// InpdbgLogf emits at level>=1.
func InpdbgLogf(format string, args ...any) {
	if inpdbgLevel() < 1 {
		return
	}
	InpdbgEmit("[inpdbg] " + fmt.Sprintf(format, args...))
}

// InpdbgLogfAt emits at level>=the given level.
func InpdbgLogfAt(level int, format string, args ...any) {
	if inpdbgLevel() < level {
		return
	}
	InpdbgEmit("[inpdbg] " + fmt.Sprintf(format, args...))
}
