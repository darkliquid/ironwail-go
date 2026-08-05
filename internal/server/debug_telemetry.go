// This file belongs to the Debug subsystem: debug telemetry, trigger touch debugging, and multiplayer debug logging.
//
// Portable types (DebugEventKind, DebugEventMask, TelemetryConfig, EntityFilter,
// EntitySnapshot) and helper functions have been moved to
// internal/server/debug. The DebugTelemetry engine and its methods remain
// here because they reference Edict and the QC VM.
package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
)

// Type aliases for debug types moved to the debug sub-package.
type (
	DebugEventKind       = srvdebug.DebugEventKind
	DebugEventMask       = srvdebug.DebugEventMask
	DebugEntityFilter    = srvdebug.EntityFilter
	DebugTelemetryConfig = srvdebug.TelemetryConfig
	DebugEntitySnapshot  = srvdebug.EntitySnapshot
)

// Debug event kind constants re-exported from the debug sub-package.
const (
	DebugEventTrigger = srvdebug.DebugEventTrigger
	DebugEventTouch   = srvdebug.DebugEventTouch
	DebugEventUse     = srvdebug.DebugEventUse
	DebugEventThink   = srvdebug.DebugEventThink
	DebugEventBlocked = srvdebug.DebugEventBlocked
	DebugEventPhysics = srvdebug.DebugEventPhysics
	DebugEventFrame   = srvdebug.DebugEventFrame
	DebugEventQC      = srvdebug.DebugEventQC
)

// Debug event mask constants re-exported from the debug sub-package.
const (
	debugEventMaskTrigger = srvdebug.EventMaskTrigger
	debugEventMaskTouch   = srvdebug.EventMaskTouch
	debugEventMaskUse     = srvdebug.EventMaskUse
	debugEventMaskThink   = srvdebug.EventMaskThink
	debugEventMaskBlocked = srvdebug.EventMaskBlocked
	debugEventMaskPhysics = srvdebug.EventMaskPhysics
	debugEventMaskFrame   = srvdebug.EventMaskFrame
	debugEventMaskQC      = srvdebug.EventMaskQC
	debugEventMaskAll     = srvdebug.EventMaskAll
)

var (
	debugEventKindOrder = srvdebug.EventKindOrder
)

// CVar variables for debug telemetry (set during registration).
var (
	debugTelemetryEnableCVar      *cvar.CVar
	debugTelemetryEventsCVar      *cvar.CVar
	debugTelemetryClassnameCVar   *cvar.CVar
	debugTelemetryEntNumCVar      *cvar.CVar
	debugTelemetrySummaryCVar     *cvar.CVar
	debugTelemetryQCTraceCVar     *cvar.CVar
	debugTelemetryQCVerbosityCVar *cvar.CVar
	debugTriggerCVar              *cvar.CVar
	debugTelemetryEmit            = func(line string) {
		fmt.Fprintln(os.Stderr, line)
	}
)

// CVar name constants re-exported from the debug sub-package.
const (
	debugTelemetryEnableCVarName      = srvdebug.DebugTelemetryEnableCVarName
	debugTelemetryEventsCVarName      = srvdebug.DebugTelemetryEventsCVarName
	debugTelemetryClassnameCVarName   = srvdebug.DebugTelemetryClassnameCVarName
	debugTelemetryEntNumCVarName      = srvdebug.DebugTelemetryEntNumCVarName
	debugTelemetrySummaryCVarName     = srvdebug.DebugTelemetrySummaryCVarName
	debugTelemetryQCTraceCVarName     = srvdebug.DebugTelemetryQCTraceCVarName
	debugTelemetryQCVerbosityCVarName = srvdebug.DebugTelemetryQCVerbosityCVarName
	debugTriggerCVarName              = srvdebug.DebugTriggerCVarName
)

// RegisterDebugTelemetryCVars registers the server-side debug telemetry control
// surface. The cvars live with host initialization so later instrumentation can
// safely assume these names exist before a server starts running.
func RegisterDebugTelemetryCVars(cv *cvar.CVarSystem) {
	debugTelemetryEnableCVar = cv.Register(debugTelemetryEnableCVarName, "0", cvar.FlagNone, "Enable server debug telemetry")
	debugTelemetryEventsCVar = cv.Register(debugTelemetryEventsCVarName, "all", cvar.FlagNone, "Telemetry event mask (all, none, numeric mask, or comma-separated names)")
	debugTelemetryClassnameCVar = cv.Register(debugTelemetryClassnameCVarName, "", cvar.FlagNone, "Optional classname filter (supports glob patterns like trigger_*)")
	debugTelemetryEntNumCVar = cv.Register(debugTelemetryEntNumCVarName, "-1", cvar.FlagNone, "Optional entity number filter (-1=all, or comma/range list like 1,4-6)")
	debugTelemetrySummaryCVar = cv.Register(debugTelemetrySummaryCVarName, "1", cvar.FlagNone, "Per-frame summary mode (0=off, 1=frames with events, 2=all frames)")
	debugTelemetryQCTraceCVar = cv.Register(debugTelemetryQCTraceCVarName, "0", cvar.FlagNone, "Enable QuakeC debug trace output")
	debugTelemetryQCVerbosityCVar = cv.Register(debugTelemetryQCVerbosityCVarName, "1", cvar.FlagNone, "QuakeC trace verbosity ceiling")
	debugTriggerCVar = cv.Register(debugTriggerCVarName, "0", cvar.FlagNone, "Print trigger/entity activation info to console")
}

// debugEntityFilter is aliased to srvdebug.EntityFilter for backward compat.
type debugEntityFilter = srvdebug.EntityFilter

// parseDebugEntityFilter wraps the debug sub-package's ParseEntityFilter.
func parseDebugEntityFilter(raw string) debugEntityFilter {
	return srvdebug.ParseEntityFilter(raw)
}

// matchesClassnameFilter wraps the debug sub-package's MatchesClassnameFilter.
func matchesClassnameFilter(raw, classname string) bool {
	return srvdebug.MatchesClassnameFilter(raw, classname)
}

// formatDebugMessage wraps the debug sub-package's FormatMessage.
func formatDebugMessage(format string, args ...any) string {
	return srvdebug.FormatMessage(format, args...)
}

// clampSummaryMode wraps the debug sub-package's ClampSummaryMode.
func clampSummaryMode(mode int) int {
	return srvdebug.ClampSummaryMode(mode)
}

type DebugTelemetry struct {
	emit           func(string)
	configProvider func() DebugTelemetryConfig
	batchOutput    bool
	pendingLines   []string
	currentConfig  DebugTelemetryConfig
	configLoaded   bool

	frameIndex  uint64
	serverTime  float32
	frameTime   float32
	frameEvents int
	frameQC     int
	perKind     map[DebugEventKind]int

	coalesceKey   string
	coalesceKind  DebugEventKind
	coalesceCount int
}

func NewDebugTelemetry() *DebugTelemetry {
	t := NewDebugTelemetryWithConfig(readDebugTelemetryConfig, debugTelemetryEmit)
	t.batchOutput = true
	return t
}

func NewDebugTelemetryWithConfig(configProvider func() DebugTelemetryConfig, emit func(string)) *DebugTelemetry {
	if configProvider == nil {
		configProvider = readDebugTelemetryConfig
	}
	if emit == nil {
		emit = func(string) {}
	}
	return &DebugTelemetry{
		emit:           emit,
		pendingLines:   make([]string, 0, 16),
		configProvider: configProvider,
		perKind:        make(map[DebugEventKind]int, len(debugEventKindOrder)),
	}
}

func (t *DebugTelemetry) BeginFrame(serverTime, frameTime float32) {
	t.flushCoalescedRepeats()
	t.flushPendingLines()
	t.reloadConfig()
	t.frameIndex++
	t.serverTime = serverTime
	t.frameTime = frameTime
	t.frameEvents = 0
	t.frameQC = 0
	clear(t.perKind)
}

func (t *DebugTelemetry) EndFrame() {
	t.flushCoalescedRepeats()
	cfg := t.currentDebugConfig()
	if !cfg.AnyEnabled() || cfg.SummaryMode == 0 {
		t.flushPendingLines()
		return
	}

	total := t.frameEvents + t.frameQC
	if cfg.SummaryMode == 1 && total == 0 {
		t.flushPendingLines()
		return
	}

	counts := make([]string, 0, len(debugEventKindOrder))
	for _, kind := range debugEventKindOrder {
		if count := t.perKind[kind]; count > 0 {
			counts = append(counts, fmt.Sprintf("%s=%d", kind, count))
		}
	}

	line := fmt.Sprintf("[svdbg frame=%d time=%.3f dt=%.3f] summary total=%d qc=%d",
		t.frameIndex, t.serverTime, t.frameTime, total, t.frameQC)
	if len(counts) > 0 {
		line += " counts=" + strings.Join(counts, ",")
	}
	t.emitLine(line)
	t.flushPendingLines()
}

func (t *DebugTelemetry) EventsEnabled() bool {
	if t == nil {
		return false
	}
	if debugTelemetryEnableCVar != nil {
		return debugTelemetryEnableCVar.Bool()
	}
	return t.currentDebugConfig().Enabled
}

func (t *DebugTelemetry) ShouldLogEvent(kind DebugEventKind, vm *qc.VM, entNum int, ent *Edict) bool {
	cfg := t.currentDebugConfig()
	if !cfg.Enabled {
		return false
	}
	return cfg.ShouldLog(kind, entNum, entityClassname(vm, ent))
}

func (t *DebugTelemetry) ShouldLogQCEvent(vm *qc.VM, entNum int, ent *Edict, verbosity int) bool {
	cfg := t.currentDebugConfig()
	if !cfg.QCTrace || verbosity > cfg.QCVerbosity {
		return false
	}
	return cfg.ShouldLog(DebugEventQC, entNum, entityClassname(vm, ent))
}

func (t *DebugTelemetry) QCTraceVerbosityEnabled(verbosity int) bool {
	if t == nil {
		return false
	}
	cfg := t.currentDebugConfig()
	return cfg.QCTrace && cfg.EventMask&debugEventMaskQC != 0 && verbosity <= cfg.QCVerbosity
}

func (t *DebugTelemetry) LogEventf(kind DebugEventKind, vm *qc.VM, entNum int, ent *Edict, format string, args ...any) bool {
	if !t.ShouldLogEvent(kind, vm, entNum, ent) {
		return false
	}

	t.frameEvents++
	t.perKind[kind]++

	snapshot := t.FormatEntitySnapshot(t.EntitySnapshot(vm, entNum, ent))
	msg := formatDebugMessage(format, args...)
	line := fmt.Sprintf("[svdbg frame=%d time=%.3f kind=%s] %s",
		t.frameIndex, t.serverTime, kind, snapshot)
	if msg != "" {
		line += " " + msg
	}
	key := t.coalesceEventKey(kind, snapshot, msg)
	t.emitCoalescedLine(kind, key, line)
	return true
}

func (t *DebugTelemetry) LogQCEventf(phase string, verbosity int, depth int, functionIndex int32, vm *qc.VM, entNum int, ent *Edict, format string, args ...any) bool {
	if !t.ShouldLogQCEvent(vm, entNum, ent, verbosity) {
		return false
	}

	t.frameQC++
	t.perKind[DebugEventQC]++

	fn := t.FormatQCFunction(vm, functionIndex)
	snapshot := t.FormatEntitySnapshot(t.EntitySnapshot(vm, entNum, ent))
	msg := formatDebugMessage(format, args...)
	line := fmt.Sprintf("[svdbg frame=%d time=%.3f kind=qc depth=%d phase=%s fn=%s] %s",
		t.frameIndex, t.serverTime, depth, phase, fn, snapshot)
	if msg != "" {
		line += " " + msg
	}
	key := t.coalesceQCEventKey(depth, phase, fn, snapshot, msg)
	t.emitCoalescedLine(DebugEventQC, key, line)
	return true
}

func (t *DebugTelemetry) emitCoalescedLine(kind DebugEventKind, key, line string) {
	if t.coalesceKey == "" {
		t.coalesceKey = key
		t.coalesceKind = kind
		t.coalesceCount = 0
		t.emitLine(line)
		return
	}
	if key == t.coalesceKey {
		t.coalesceCount++
		return
	}
	t.flushCoalescedRepeats()
	t.coalesceKey = key
	t.coalesceKind = kind
	t.coalesceCount = 0
	t.emitLine(line)
}

func (t *DebugTelemetry) flushCoalescedRepeats() {
	if t.coalesceCount > 0 {
		t.emitLine(fmt.Sprintf("[svdbg frame=%d time=%.3f kind=%s] repeated x%d",
			t.frameIndex, t.serverTime, t.coalesceKind, t.coalesceCount))
	}
	t.coalesceKey = ""
	t.coalesceKind = ""
	t.coalesceCount = 0
}

func (t *DebugTelemetry) emitLine(line string) {
	if t == nil || line == "" {
		return
	}
	if t.batchOutput {
		t.pendingLines = append(t.pendingLines, line)
		return
	}
	t.emit(line)
}

func (t *DebugTelemetry) flushPendingLines() {
	if t == nil || len(t.pendingLines) == 0 {
		return
	}
	t.emit(strings.Join(t.pendingLines, "\n"))
	t.pendingLines = t.pendingLines[:0]
}

func (t *DebugTelemetry) coalesceEventKey(kind DebugEventKind, snapshot, msg string) string {
	if msg == "" {
		return fmt.Sprintf("event kind=%s %s", kind, snapshot)
	}
	return fmt.Sprintf("event kind=%s %s %s", kind, snapshot, msg)
}

func (t *DebugTelemetry) coalesceQCEventKey(depth int, phase, fn, snapshot, msg string) string {
	if msg == "" {
		return fmt.Sprintf("qc depth=%d phase=%s fn=%s %s", depth, phase, fn, snapshot)
	}
	return fmt.Sprintf("qc depth=%d phase=%s fn=%s %s %s", depth, phase, fn, snapshot, msg)
}

func (t *DebugTelemetry) EntitySnapshot(vm *qc.VM, entNum int, ent *Edict) DebugEntitySnapshot {
	snapshot := DebugEntitySnapshot{EntNum: entNum}
	if ent == nil {
		return snapshot
	}
	if vm != nil && vm.EdictSize > 28 {
		snapshot.ClassName = qcString(vm, vm.EInt(entNum, qc.EntFieldClassName))
		snapshot.TargetName = qcString(vm, vm.EInt(entNum, qc.EntFieldTargetName))
		snapshot.Target = qcString(vm, vm.EInt(entNum, qc.EntFieldTarget))
		snapshot.Model = qcString(vm, vm.EInt(entNum, qc.EntFieldModel))
		snapshot.Origin = vm.EVector(entNum, qc.EntFieldOrigin)
	}
	return snapshot
}

func (t *DebugTelemetry) FormatEntitySnapshot(snapshot DebugEntitySnapshot) string {
	return fmt.Sprintf("ent=%d classname=%q targetname=%q target=%q model=%q origin=(%.1f %.1f %.1f)",
		snapshot.EntNum,
		snapshot.ClassName,
		snapshot.TargetName,
		snapshot.Target,
		snapshot.Model,
		snapshot.Origin[0],
		snapshot.Origin[1],
		snapshot.Origin[2],
	)
}

func (t *DebugTelemetry) FormatQCFunction(vm *qc.VM, functionIndex int32) string {
	if vm == nil || functionIndex < 0 || int(functionIndex) >= len(vm.Functions) {
		return fmt.Sprintf("#%d", functionIndex)
	}

	fn := vm.Functions[functionIndex]
	name := vm.String(fn.Name)
	if name == "" {
		name = fmt.Sprintf("#%d", functionIndex)
	}
	if fn.FirstStatement < 0 {
		return fmt.Sprintf("%s[#%d builtin=%d]", name, functionIndex, -fn.FirstStatement)
	}
	return fmt.Sprintf("%s[#%d]", name, functionIndex)
}

func (t *DebugTelemetry) currentDebugConfig() DebugTelemetryConfig {
	if t == nil {
		return DebugTelemetryConfig{}
	}
	if !t.configLoaded {
		t.reloadConfig()
	}
	return t.currentConfig
}

func (t *DebugTelemetry) reloadConfig() {
	if t == nil {
		return
	}
	t.currentConfig = t.configProvider()
	t.configLoaded = true
}

func readDebugTelemetryConfig() DebugTelemetryConfig {
	cfg := DebugTelemetryConfig{
		EventMask:    debugEventMaskAll,
		EntityFilter: debugEntityFilter{All: true},
		SummaryMode:  1,
		QCVerbosity:  1,
	}
	if debugTelemetryEnableCVar != nil {
		cfg.Enabled = debugTelemetryEnableCVar.Bool()
	}
	if debugTelemetryQCTraceCVar != nil {
		cfg.QCTrace = debugTelemetryQCTraceCVar.Bool()
	}
	if !cfg.AnyEnabled() {
		return cfg
	}
	if debugTelemetryEventsCVar != nil {
		cfg.EventMask = parseDebugEventMask(debugTelemetryEventsCVar.String)
	}
	if debugTelemetryClassnameCVar != nil {
		cfg.ClassnameFilter = debugTelemetryClassnameCVar.String
	}
	if debugTelemetryEntNumCVar != nil {
		cfg.EntityFilter = parseDebugEntityFilter(debugTelemetryEntNumCVar.String)
	}
	if debugTelemetrySummaryCVar != nil {
		cfg.SummaryMode = clampSummaryMode(debugTelemetrySummaryCVar.Int)
	}
	if debugTelemetryQCVerbosityCVar != nil {
		cfg.QCVerbosity = debugTelemetryQCVerbosityCVar.Int
		if cfg.QCVerbosity < 0 {
			cfg.QCVerbosity = 0
		}
	}
	return cfg
}

// parseDebugEventMask wraps the debug sub-package's ParseEventMask.
func parseDebugEventMask(raw string) DebugEventMask {
	return srvdebug.ParseEventMask(raw)
}

func entityClassname(vm *qc.VM, ent *Edict) string {
	if ent == nil {
		return ""
	}
	if vm != nil && vm.EdictSize > 28 {
		return qcString(vm, vm.EInt(ent.Num, qc.EntFieldClassName))
	}
	return ""
}

func qcString(vm *qc.VM, idx int32) string {
	if vm == nil || idx == 0 {
		return ""
	}
	return vm.String(idx)
}
