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
	SvDebugPushCVarName        = "sv_debug_push"
)

var (
	svDebugMultiplayerCVar *cvar.CVar
	svDebugMoveCVar        *cvar.CVar
	svDebugPushCVar        *cvar.CVar
	// svDebugPushTriggerDumpDone ensures the trigger entity dump is only
	// emitted once per session (on the first touchLinks call after
	// sv_debug_push is enabled).
	svDebugPushTriggerDumpDone bool
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
	svDebugPushCVar = cv.Register(SvDebugPushCVarName, "0", cvar.FlagNone,
		"Debug PushMove riding/push detection: logs pusher, riding check, AABB overlap, and touchLinks results for MOVETYPE_PUSH entities (0=off, 1=summary, 2=verbose)")
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

func svDebugPushLevel() int {
	if svDebugPushCVar == nil {
		return 0
	}
	return svDebugPushCVar.Int
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

// SvdbgPushLogf emits a kind=push line at level>=1. Used for PushMove
// riding detection and touchLinks telemetry when diagnosing lift/plat
// trigger firing issues.
func SvdbgPushLogf(format string, args ...any) {
	if svDebugPushLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=push] " + fmt.Sprintf(format, args...))
}

// SvdbgPushLogfAt emits a kind=push line at level>=the given verbosity.
func SvdbgPushLogfAt(level int, format string, args ...any) {
	if svDebugPushLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=push] " + fmt.Sprintf(format, args...))
}

// svDebugPushDumpTriggersOnce dumps all SOLID_TRIGGER entities once per
// session so we can see what triggers exist and where they are positioned.
// This helps diagnose cases where touchLinks reports candidates=0 because
// no trigger entities overlap the player's bbox.
func svDebugPushDumpTriggersOnce(s *Server) {
	if svDebugPushTriggerDumpDone {
		return
	}
	svDebugPushTriggerDumpDone = true
	for i := 1; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e == nil || e.Free {
			continue
		}
		if int(e.Vars.Solid) != int(SolidTrigger) {
			continue
		}
		cn := qcString(s.QCVM, e.Vars.ClassName)
		SvdbgPushLogf("trigger_dump edict=%d classname=%q touch=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f) origin=(%.1f %.1f %.1f)",
			i, cn, e.Vars.Touch,
			e.Vars.AbsMin[0], e.Vars.AbsMin[1], e.Vars.AbsMin[2],
			e.Vars.AbsMax[0], e.Vars.AbsMax[1], e.Vars.AbsMax[2],
			e.Vars.Origin[0], e.Vars.Origin[1], e.Vars.Origin[2])
	}
}
