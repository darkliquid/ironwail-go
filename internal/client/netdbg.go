// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// NetDebugCVarName is the name of the netdbg verbosity cvar. Mirrors
// C Ironwail's cl_debug_net (0=off, 1=events, 2=verbose field-level).
const NetDebugCVarName = "cl_debug_net"

var (
	netDebugCVar *cvar.CVar
	// NetDebugEmit is the sink for netdbg lines. Overridable from tests.
	NetDebugEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterNetDebugCVars registers the client-side net telemetry cvars.
// Safe to call once during host CVar registration.
func RegisterNetDebugCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	netDebugCVar = cv.Register(NetDebugCVarName, "0", cvar.FlagNone,
		"Client network parse debug telemetry (0=off, 1=events, 2=verbose)")
}

// netDebugLevel returns the current verbosity, 0 when disabled.
func netDebugLevel() int {
	if netDebugCVar == nil {
		return 0
	}
	return netDebugCVar.Int
}

// netDebugEnabled reports whether netdbg is at least the supplied level.
func netDebugEnabled(level int) bool { return netDebugLevel() >= level }

// netDebugLogf emits a netdbg line at level>=1.
func netDebugLogf(kind, format string, args ...any) {
	if !netDebugEnabled(1) {
		return
	}
	NetDebugEmit(fmt.Sprintf("[netdbg kind=%s] ", kind) + fmt.Sprintf(format, args...))
}

// netDebugLogfAt emits a netdbg line at level>=the supplied level.
func netDebugLogfAt(level int, kind, format string, args ...any) {
	if !netDebugEnabled(level) {
		return
	}
	NetDebugEmit(fmt.Sprintf("[netdbg kind=%s] ", kind) + fmt.Sprintf(format, args...))
}
