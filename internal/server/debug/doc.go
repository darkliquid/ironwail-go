// Package debug contains server-side debug telemetry types, cvar registration,
// and logging functions. The DebugTelemetry engine (which references Edict
// and the QC VM) remains in the server package because Go does not allow
// methods on non-local types.
//
// Types and constants re-exported via aliases from the server package so
// existing code continues to work unchanged.
//
// # Original C lineage
//
// Debug telemetry and svdbg logging are Ironwail-Go additions with no
// direct C counterpart; the cvar names and output format follow the
// conventions of the original engine's developer diagnostics.
package debug
