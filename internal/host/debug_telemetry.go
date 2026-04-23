package host

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

const hostDebugSysCVarName = "host_debug_sys"

// Sysdbg emits telemetry for host lifecycle events (save/load, mod
// downloads, window title changes, manifest scans). It mirrors the style
// of internal/server/debug_telemetry.go and cmd/ironwailgo's cldbg
// plumbing so operators can diff host behaviour against reference
// ironwail by piping logs.
//
// All emission flows through hostDebugSysEmit, which defaults to
// fmt.Fprintln(os.Stderr, ...) and is overridable for tests.

var (
	hostDebugSysCVar *cvar.CVar
	hostDebugSysEmit = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// RegisterHostDebugTelemetryCVars registers host-lifecycle debug cvars.
// Safe to call once during init.
func RegisterHostDebugTelemetryCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	hostDebugSysCVar = cv.Register(hostDebugSysCVarName, "0", cvar.FlagNone,
		"Host system debug telemetry (0=off, 1=events, 2=verbose)")
}

// hostDebugSysLevel returns the current sysdbg verbosity; 0 means off.
func hostDebugSysLevel() int {
	if hostDebugSysCVar == nil {
		return 0
	}
	return hostDebugSysCVar.Int
}

// hostDebugSysEnabled reports whether sysdbg is enabled at a minimum level.
func hostDebugSysEnabled(level int) bool {
	return hostDebugSysLevel() >= level
}

// hostDebugSysLogf emits a formatted sysdbg line when the telemetry
// level is at least 1. Lines are prefixed with `[sysdbg kind=<kind>]`
// so they can be grepped alongside cldbg/svdbg output.
func hostDebugSysLogf(kind, format string, args ...any) {
	if !hostDebugSysEnabled(1) {
		return
	}
	hostDebugSysEmit(fmt.Sprintf("[sysdbg kind=%s] ", kind) + fmt.Sprintf(format, args...))
}

// hostDebugSysLogfAt emits a formatted sysdbg line when the telemetry
// level is at least the supplied level. Use level>=2 for verbose
// per-chunk detail such as mod download progress.
func hostDebugSysLogfAt(level int, kind, format string, args ...any) {
	if !hostDebugSysEnabled(level) {
		return
	}
	hostDebugSysEmit(fmt.Sprintf("[sysdbg kind=%s] ", kind) + fmt.Sprintf(format, args...))
}
