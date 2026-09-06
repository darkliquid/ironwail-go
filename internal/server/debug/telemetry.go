// This file contains portable debug telemetry types and helper functions
// that do not depend on Server or Edict.
package debug

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// CVar names for debug telemetry.
const (
	DebugTelemetryEnableCVarName      = "sv_debug_telemetry"
	DebugTelemetryEventsCVarName      = "sv_debug_telemetry_events"
	DebugTelemetryClassnameCVarName   = "sv_debug_telemetry_classname"
	DebugTelemetryEntNumCVarName      = "sv_debug_telemetry_entnum"
	DebugTelemetrySummaryCVarName     = "sv_debug_telemetry_summary"
	DebugTelemetryQCTraceCVarName     = "sv_debug_qc_trace"
	DebugTelemetryQCVerbosityCVarName = "sv_debug_qc_trace_verbosity"
	DebugTriggerCVarName              = "sv_debug_trigger"
	QCDebugPortCVarName               = "qc_debug_port"
)

// Svdbg cvar names.
const (
	SvDebugMultiplayerCVarName = "sv_debug_multiplayer"
	SvDebugMoveCVarName        = "sv_debug_move"
	SvDebugPushCVarName        = "sv_debug_push"
	SvDebugCombatCVarName      = "sv_debug_combat"
)

// Emit is the default telemetry sink for debug telemetry lines.
var Emit = func(line string) {
	fmt.Fprintln(os.Stderr, line)
}

// SvdbgEmit is the telemetry sink for svdbg lines.
var SvdbgEmit = func(line string) {
	fmt.Fprintln(os.Stderr, line)
}

// DebugEventKind identifies a category of debug telemetry event.
type DebugEventKind string

const (
	DebugEventTrigger DebugEventKind = "trigger"
	DebugEventTouch   DebugEventKind = "touch"
	DebugEventUse     DebugEventKind = "use"
	DebugEventThink   DebugEventKind = "think"
	DebugEventBlocked DebugEventKind = "blocked"
	DebugEventPhysics DebugEventKind = "physics"
	DebugEventFrame   DebugEventKind = "frame"
	DebugEventQC      DebugEventKind = "qc"
)

// DebugEventMask is a bitmask of enabled debug event categories.
type DebugEventMask uint64

const (
	EventMaskTrigger DebugEventMask = 1 << iota
	EventMaskTouch
	EventMaskUse
	EventMaskThink
	EventMaskBlocked
	EventMaskPhysics
	EventMaskFrame
	EventMaskQC
)

// EventMaskAll is the OR of all event mask bits.
const EventMaskAll = EventMaskTrigger |
	EventMaskTouch |
	EventMaskUse |
	EventMaskThink |
	EventMaskBlocked |
	EventMaskPhysics |
	EventMaskFrame |
	EventMaskQC

// EventKindOrder defines a stable ordering for per-kind summary counts.
var EventKindOrder = []DebugEventKind{
	DebugEventFrame,
	DebugEventTrigger,
	DebugEventTouch,
	DebugEventUse,
	DebugEventThink,
	DebugEventBlocked,
	DebugEventPhysics,
	DebugEventQC,
}

// Mask returns the event mask bit for this event kind, or 0 if unknown.
func (k DebugEventKind) Mask() DebugEventMask {
	switch k {
	case DebugEventTrigger:
		return EventMaskTrigger
	case DebugEventTouch:
		return EventMaskTouch
	case DebugEventUse:
		return EventMaskUse
	case DebugEventThink:
		return EventMaskThink
	case DebugEventBlocked:
		return EventMaskBlocked
	case DebugEventPhysics:
		return EventMaskPhysics
	case DebugEventFrame:
		return EventMaskFrame
	case DebugEventQC:
		return EventMaskQC
	default:
		return 0
	}
}

// EntityFilter filters entities by entity number for debug telemetry.
type EntityFilter struct {
	All     bool
	Allowed map[int]struct{}
}

// Matches returns true if the given entity number passes the filter.
func (f EntityFilter) Matches(entNum int) bool {
	if f.All {
		return true
	}
	if entNum < 0 {
		return false
	}
	_, ok := f.Allowed[entNum]
	return ok
}

// TelemetryConfig holds the active debug telemetry configuration.
type TelemetryConfig struct {
	Enabled         bool
	EventMask       DebugEventMask
	ClassnameFilter string
	EntityFilter    EntityFilter
	SummaryMode     int
	QCTrace         bool
	QCVerbosity     int
}

// AnyEnabled returns true if telemetry or QC tracing is active.
func (c TelemetryConfig) AnyEnabled() bool {
	return c.Enabled || c.QCTrace
}

// ShouldLog returns true if an event of the given kind, entity number, and
// classname should be logged under the current configuration.
func (c TelemetryConfig) ShouldLog(kind DebugEventKind, entNum int, classname string) bool {
	mask := kind.Mask()
	if mask == 0 || c.EventMask&mask == 0 {
		return false
	}
	if !c.EntityFilter.Matches(entNum) {
		return false
	}
	return MatchesClassnameFilter(c.ClassnameFilter, classname)
}

// EntitySnapshot captures a snapshot of an entity for debug logging.
type EntitySnapshot struct {
	EntNum     int
	ClassName  string
	TargetName string
	Target     string
	Model      string
	Origin     qtypes.Vec3
}

// ParseEventMask parses a debug event mask from a string.
func ParseEventMask(raw string) DebugEventMask {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" || raw == "all" {
		return EventMaskAll
	}
	if raw == "none" {
		return 0
	}
	if value, err := strconv.ParseUint(raw, 0, 64); err == nil {
		return DebugEventMask(value)
	}

	var mask DebugEventMask
	for _, token := range splitDebugFilterTokens(raw) {
		switch token {
		case "all":
			mask |= EventMaskAll
		case "trigger":
			mask |= EventMaskTrigger
		case "touch":
			mask |= EventMaskTouch
		case "use":
			mask |= EventMaskUse
		case "think":
			mask |= EventMaskThink
		case "blocked":
			mask |= EventMaskBlocked
		case "physics":
			mask |= EventMaskPhysics
		case "frame":
			mask |= EventMaskFrame
		case "qc":
			mask |= EventMaskQC
		}
	}
	return mask
}

// ParseEntityFilter parses an entity number filter from a string.
func ParseEntityFilter(raw string) EntityFilter {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" || raw == "all" || raw == "-1" {
		return EntityFilter{All: true}
	}

	filter := EntityFilter{Allowed: make(map[int]struct{})}
	for _, token := range splitDebugFilterTokens(raw) {
		if token == "" {
			continue
		}
		if start, end, ok := parseDebugEntityRange(token); ok {
			for entNum := start; entNum <= end; entNum++ {
				filter.Allowed[entNum] = struct{}{}
			}
			continue
		}
		if entNum, err := strconv.Atoi(token); err == nil {
			filter.Allowed[entNum] = struct{}{}
		}
	}
	if len(filter.Allowed) == 0 {
		return EntityFilter{}
	}
	return filter
}

func parseDebugEntityRange(token string) (int, int, bool) {
	if strings.Count(token, "-") != 1 || strings.HasPrefix(token, "-") {
		return 0, 0, false
	}
	parts := strings.SplitN(token, "-", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil || end < start {
		return 0, 0, false
	}
	return start, end, true
}

// MatchesClassnameFilter checks if a classname matches a glob/comma filter.
func MatchesClassnameFilter(raw, classname string) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" {
		return true
	}
	classname = strings.ToLower(classname)
	if classname == "" {
		return false
	}
	for _, token := range splitDebugFilterTokens(raw) {
		if token == "" {
			continue
		}
		if strings.ContainsAny(token, "*?[") {
			matched, err := path.Match(token, classname)
			if err == nil && matched {
				return true
			}
			continue
		}
		if token == classname {
			return true
		}
	}
	return false
}

func splitDebugFilterTokens(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '|', '+', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
}

// FormatMessage formats a debug message, returning "" if format is empty.
func FormatMessage(format string, args ...any) string {
	if format == "" {
		return ""
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// ClampSummaryMode clamps a summary mode value to the range [0, 2].
func ClampSummaryMode(mode int) int {
	if mode < 0 {
		return 0
	}
	if mode > 2 {
		return 2
	}
	return mode
}

// FormatEntitySnapshot formats an EntitySnapshot as a log-ready string.
func FormatEntitySnapshot(snapshot EntitySnapshot) string {
	return fmt.Sprintf("ent=%d classname=%q targetname=%q target=%q model=%q origin=(%.1f %.1f %.1f)",
		snapshot.EntNum,
		snapshot.ClassName,
		snapshot.TargetName,
		snapshot.Target,
		snapshot.Model,
		snapshot.Origin.X,
		snapshot.Origin.Y,
		snapshot.Origin.Z,
	)
}
