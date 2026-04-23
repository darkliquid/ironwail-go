// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package renderer

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// M5 parity telemetry for renderer pass counts. The cvar gates `rendbg`
// lines that surface per-frame alpha-pass / batch counts for diffing
// against reference ironwail gl_rmain.c / gl_warp.c behaviour.

const RDebugPassesCVarName = "r_debug_passes"

var (
	rDebugPassesCVar *cvar.CVar
	// RendbgEmit is the telemetry sink; overridable in tests.
	RendbgEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterRendbgCVars registers r_debug_passes.
func RegisterRendbgCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	rDebugPassesCVar = cv.Register(RDebugPassesCVarName, "0", cvar.FlagNone,
		"Renderer pass debug telemetry (0=off, 1=per-frame summary, 2=per-call)")
}

func rendbgLevel() int {
	if rDebugPassesCVar == nil {
		return 0
	}
	return rDebugPassesCVar.Int
}

// RendbgLogf emits at level>=1.
func RendbgLogf(format string, args ...any) {
	if rendbgLevel() < 1 {
		return
	}
	RendbgEmit("[rendbg] " + fmt.Sprintf(format, args...))
}

// RendbgLogfAt emits at level>=the given level.
func RendbgLogfAt(level int, format string, args ...any) {
	if rendbgLevel() < level {
		return
	}
	RendbgEmit("[rendbg] " + fmt.Sprintf(format, args...))
}
