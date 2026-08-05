// This file contains the svdbg multiplayer/move/push logging functions
// and cvar registration. These are standalone functions with no dependency
// on Server or Edict.
package debug

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

// Svdbg cvar variables (set during cvar registration).
var (
	svDebugMultiplayerCVar *cvar.CVar
	svDebugMoveCVar        *cvar.CVar
	svDebugPushCVar        *cvar.CVar
	// SvDebugPushTriggerDumpDone ensures the trigger entity dump is only
	// emitted once per session (on the first touchLinks call after
	// sv_debug_push is enabled).
	SvDebugPushTriggerDumpDone bool
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

func SvDebugMultiplayerLevel() int {
	if svDebugMultiplayerCVar == nil {
		return 0
	}
	return svDebugMultiplayerCVar.Int
}

func SvDebugMoveLevel() int {
	if svDebugMoveCVar == nil {
		return 0
	}
	return svDebugMoveCVar.Int
}

func SvDebugPushLevel() int {
	if svDebugPushCVar == nil {
		return 0
	}
	return svDebugPushCVar.Int
}

// SvDebugPushEnabled reports whether any sv_debug_push logging is active.
// Hot-loop call sites use it to skip varargs formatting and interface-boxing
// when the cvar is 0; without it every SvdbgPushLogf* call in the per-edict
// PushMove loop allocates a []any even when diagnostics are off.
func SvDebugPushEnabled() bool {
	return svDebugPushCVar != nil && svDebugPushCVar.Int > 0
}

// ResetSvdbgCVars clears the svdbg cvar references. Used by tests to
// ensure a clean state between test cases.
func ResetSvdbgCVars() {
	svDebugMultiplayerCVar = nil
	svDebugMoveCVar = nil
	svDebugPushCVar = nil
	SvDebugPushTriggerDumpDone = false
}

// SvdbgMultiplayerLogf emits a kind=multiplayer line at level>=1.
func SvdbgMultiplayerLogf(format string, args ...any) {
	if SvDebugMultiplayerLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=multiplayer] " + fmt.Sprintf(format, args...))
}

// SvdbgMultiplayerLogfAt emits a kind=multiplayer line at level>=the
// given verbosity.
func SvdbgMultiplayerLogfAt(level int, format string, args ...any) {
	if SvDebugMultiplayerLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=multiplayer] " + fmt.Sprintf(format, args...))
}

// SvdbgMoveLogf emits a kind=move line at level>=1.
func SvdbgMoveLogf(format string, args ...any) {
	if SvDebugMoveLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=move] " + fmt.Sprintf(format, args...))
}

// SvdbgMoveLogfAt emits a kind=move line at level>=the given verbosity.
func SvdbgMoveLogfAt(level int, format string, args ...any) {
	if SvDebugMoveLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=move] " + fmt.Sprintf(format, args...))
}

// SvdbgPushLogf emits a kind=push line at level>=1. Used for PushMove
// riding detection and touchLinks telemetry when diagnosing lift/plat
// trigger firing issues.
func SvdbgPushLogf(format string, args ...any) {
	if SvDebugPushLevel() < 1 {
		return
	}
	SvdbgEmit("[svdbg kind=push] " + fmt.Sprintf(format, args...))
}

// SvdbgPushLogfAt emits a kind=push line at level>=the given verbosity.
func SvdbgPushLogfAt(level int, format string, args ...any) {
	if SvDebugPushLevel() < level {
		return
	}
	SvdbgEmit("[svdbg kind=push] " + fmt.Sprintf(format, args...))
}
