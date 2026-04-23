// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

// Svdbg emits server-side multiplayer and movement telemetry, matching
// the spirit of cl_debug_net on the client. The two cvars correspond to
// the split between multiplayer signalling (slist/listen/user connect)
// and move signalling (SV_Physics_Client input/output per frame):
//
//   - sv_debug_multiplayer — listen-server/slist/connect events.
//   - sv_debug_move        — physics-client pre/post-think state.
//
// Emission goes through svdbgEmit, a package-level var that's
// overridable from tests.

const (
	SvDebugMultiplayerCVarName = "sv_debug_multiplayer"
	SvDebugMoveCVarName        = "sv_debug_move"
)

var (
	svDebugMultiplayerCVar *cvar.CVar
	svDebugMoveCVar        *cvar.CVar
	// SvdbgEmit is the telemetry sink for svdbg lines.
	SvdbgEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterSvdbgCVars registers the svdbg cvars. Called alongside the
// existing RegisterDebugTelemetryCVars during host initialisation.
func RegisterSvdbgCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	svDebugMultiplayerCVar = cv.Register(SvDebugMultiplayerCVarName, "0", cvar.FlagNone,
		"Server multiplayer debug telemetry (0=off, 1=events, 2=verbose)")
	svDebugMoveCVar = cv.Register(SvDebugMoveCVarName, "0", cvar.FlagNone,
		"Server physics-client debug telemetry (0=off, 1=events, 2=verbose)")
	inet.SlistDebugHook = func(event, format string, args ...any) {
		SvdbgMultiplayerLogf("slist/"+event+" "+format, args...)
	}
}

func svDebugMultiplayerLevel() int {
	if svDebugMultiplayerCVar == nil {
		return 0
	}
	return svDebugMultiplayerCVar.Int
}

func svDebugMoveLevel() int {
	if svDebugMoveCVar == nil {
		return 0
	}
	return svDebugMoveCVar.Int
}

// SvdbgMultiplayerLogf emits a kind=multiplayer line at level>=1.
func SvdbgMultiplayerLogf(format string, args ...any) {
	if svDebugMultiplayerLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=multiplayer] " + fmt.Sprintf(format, args...))
}

// SvdbgMultiplayerLogfAt emits a kind=multiplayer line at level>=the
// given verbosity.
func SvdbgMultiplayerLogfAt(level int, format string, args ...any) {
	if svDebugMultiplayerLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=multiplayer] " + fmt.Sprintf(format, args...))
}

// SvdbgMoveLogf emits a kind=move line at level>=1.
func SvdbgMoveLogf(format string, args ...any) {
	if svDebugMoveLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=move] " + fmt.Sprintf(format, args...))
}

// SvdbgMoveLogfAt emits a kind=move line at level>=the given verbosity.
func SvdbgMoveLogfAt(level int, format string, args ...any) {
	if svDebugMoveLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=move] " + fmt.Sprintf(format, args...))
}
