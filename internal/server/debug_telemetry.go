package server

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/qc"
)

const (
	debugTelemetryEnableCVarName      = "sv_debug_telemetry"
	debugTelemetryEventsCVarName      = "sv_debug_telemetry_events"
	debugTelemetryClassnameCVarName   = "sv_debug_telemetry_classname"
	debugTelemetryEntNumCVarName      = "sv_debug_telemetry_entnum"
	debugTelemetrySummaryCVarName     = "sv_debug_telemetry_summary"
	debugTelemetryQCTraceCVarName     = "sv_debug_qc_trace"
	debugTelemetryQCVerbosityCVarName = "sv_debug_qc_trace_verbosity"
)

var (
	debugTelemetryEnableCVar      *cvar.CVar
	debugTelemetryEventsCVar      *cvar.CVar
	debugTelemetryClassnameCVar   *cvar.CVar
	debugTelemetryEntNumCVar      *cvar.CVar
	debugTelemetrySummaryCVar     *cvar.CVar
	debugTelemetryQCTraceCVar     *cvar.CVar
	debugTelemetryQCVerbosityCVar *cvar.CVar
)

// RegisterDebugTelemetryCVars registers the server-side debug telemetry control
// surface. The cvars live with host initialization so later instrumentation can
// safely assume these names exist before a server starts running.
func RegisterDebugTelemetryCVars() {
	debugTelemetryEnableCVar = cvar.Register(debugTelemetryEnableCVarName, "0", cvar.FlagNone, "Enable server debug telemetry")
	debugTelemetryEventsCVar = cvar.Register(debugTelemetryEventsCVarName, "all", cvar.FlagNone, "Telemetry event mask (all, none, numeric mask, or comma-separated names)")
	debugTelemetryClassnameCVar = cvar.Register(debugTelemetryClassnameCVarName, "", cvar.FlagNone, "Optional classname filter (supports glob patterns like trigger_*)")
	debugTelemetryEntNumCVar = cvar.Register(debugTelemetryEntNumCVarName, "-1", cvar.FlagNone, "Optional entity number filter (-1=all, or comma/range list like 1,4-6)")
	debugTelemetrySummaryCVar = cvar.Register(debugTelemetrySummaryCVarName, "1", cvar.FlagNone, "Per-frame summary mode (0=off, 1=frames with events, 2=all frames)")
	debugTelemetryQCTraceCVar = cvar.Register(debugTelemetryQCTraceCVarName, "0", cvar.FlagNone, "Enable QuakeC debug trace output")
	debugTelemetryQCVerbosityCVar = cvar.Register(debugTelemetryQCVerbosityCVarName, "1", cvar.FlagNone, "QuakeC trace verbosity ceiling")
}

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

type DebugEventMask uint64

const (
	debugEventMaskTrigger DebugEventMask = 1 << iota
	debugEventMaskTouch
	debugEventMaskUse
	debugEventMaskThink
	debugEventMaskBlocked
	debugEventMaskPhysics
	debugEventMaskFrame
	debugEventMaskQC
)

const debugEventMaskAll = debugEventMaskTrigger |
	debugEventMaskTouch |
	debugEventMaskUse |
	debugEventMaskThink |
	debugEventMaskBlocked |
	debugEventMaskPhysics |
	debugEventMaskFrame |
	debugEventMaskQC

var debugEventKindOrder = []DebugEventKind{
	DebugEventFrame,
	DebugEventTrigger,
	DebugEventTouch,
	DebugEventUse,
	DebugEventThink,
	DebugEventBlocked,
	DebugEventPhysics,
	DebugEventQC,
}

func (k DebugEventKind) mask() DebugEventMask {
	switch k {
	case DebugEventTrigger:
		return debugEventMaskTrigger
	case DebugEventTouch:
		return debugEventMaskTouch
	case DebugEventUse:
		return debugEventMaskUse
	case DebugEventThink:
		return debugEventMaskThink
	case DebugEventBlocked:
		return debugEventMaskBlocked
	case DebugEventPhysics:
		return debugEventMaskPhysics
	case DebugEventFrame:
		return debugEventMaskFrame
	case DebugEventQC:
		return debugEventMaskQC
	default:
		return 0
	}
}

type debugEntityFilter struct {
	all     bool
	allowed map[int]struct{}
}

func (f debugEntityFilter) Matches(entNum int) bool {
	if f.all {
		return true
	}
	if entNum < 0 {
		return false
	}
	_, ok := f.allowed[entNum]
	return ok
}

type DebugTelemetryConfig struct {
	Enabled         bool
	EventMask       DebugEventMask
	ClassnameFilter string
	EntityFilter    debugEntityFilter
	SummaryMode     int
	QCTrace         bool
	QCVerbosity     int
}

func (c DebugTelemetryConfig) AnyEnabled() bool {
	return c.Enabled || c.QCTrace
}

func (c DebugTelemetryConfig) ShouldLog(kind DebugEventKind, entNum int, classname string) bool {
	mask := kind.mask()
	if mask == 0 || c.EventMask&mask == 0 {
		return false
	}
	if !c.EntityFilter.Matches(entNum) {
		return false
	}
	return matchesClassnameFilter(c.ClassnameFilter, classname)
}

type DebugEntitySnapshot struct {
	EntNum     int
	ClassName  string
	TargetName string
	Target     string
	Model      string
	Origin     [3]float32
}

type DebugTelemetry struct {
	emit           func(string)
	configProvider func() DebugTelemetryConfig

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
	return NewDebugTelemetryWithConfig(readDebugTelemetryConfig, func(line string) {
		console.Printf("%s\n", line)
	})
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
		configProvider: configProvider,
		perKind:        make(map[DebugEventKind]int, len(debugEventKindOrder)),
	}
}

func (t *DebugTelemetry) BeginFrame(serverTime, frameTime float32) {
	t.flushCoalescedRepeats()
	t.frameIndex++
	t.serverTime = serverTime
	t.frameTime = frameTime
	t.frameEvents = 0
	t.frameQC = 0
	clear(t.perKind)
}

func (t *DebugTelemetry) EndFrame() {
	t.flushCoalescedRepeats()
	cfg := t.configProvider()
	if !cfg.AnyEnabled() || cfg.SummaryMode == 0 {
		return
	}

	total := t.frameEvents + t.frameQC
	if cfg.SummaryMode == 1 && total == 0 {
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
	t.emit(line)
}

func (t *DebugTelemetry) EventsEnabled() bool {
	if t == nil {
		return false
	}
	if debugTelemetryEnableCVar != nil {
		return debugTelemetryEnableCVar.Bool()
	}
	return t.configProvider().Enabled
}

func (t *DebugTelemetry) ShouldLogEvent(kind DebugEventKind, vm *qc.VM, entNum int, ent *Edict) bool {
	cfg := t.configProvider()
	if !cfg.Enabled {
		return false
	}
	return cfg.ShouldLog(kind, entNum, entityClassname(vm, ent))
}

func (t *DebugTelemetry) ShouldLogQCEvent(vm *qc.VM, entNum int, ent *Edict, verbosity int) bool {
	cfg := t.configProvider()
	if !cfg.QCTrace || verbosity > cfg.QCVerbosity {
		return false
	}
	return cfg.ShouldLog(DebugEventQC, entNum, entityClassname(vm, ent))
}

func (t *DebugTelemetry) QCTraceVerbosityEnabled(verbosity int) bool {
	if t == nil {
		return false
	}
	cfg := t.configProvider()
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
		t.emit(line)
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
	t.emit(line)
}

func (t *DebugTelemetry) flushCoalescedRepeats() {
	if t.coalesceCount > 0 {
		t.emit(fmt.Sprintf("[svdbg frame=%d time=%.3f kind=%s] repeated x%d",
			t.frameIndex, t.serverTime, t.coalesceKind, t.coalesceCount))
	}
	t.coalesceKey = ""
	t.coalesceKind = ""
	t.coalesceCount = 0
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
	if ent == nil || ent.Vars == nil {
		return snapshot
	}
	snapshot.ClassName = qcString(vm, ent.Vars.ClassName)
	snapshot.TargetName = qcString(vm, ent.Vars.TargetName)
	snapshot.Target = qcString(vm, ent.Vars.Target)
	snapshot.Model = qcString(vm, ent.Vars.Model)
	snapshot.Origin = ent.Vars.Origin
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
	name := vm.GetString(fn.Name)
	if name == "" {
		name = fmt.Sprintf("#%d", functionIndex)
	}
	if fn.FirstStatement < 0 {
		return fmt.Sprintf("%s[#%d builtin=%d]", name, functionIndex, -fn.FirstStatement)
	}
	return fmt.Sprintf("%s[#%d]", name, functionIndex)
}

func readDebugTelemetryConfig() DebugTelemetryConfig {
	cfg := DebugTelemetryConfig{
		EventMask:    debugEventMaskAll,
		EntityFilter: debugEntityFilter{all: true},
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

func parseDebugEventMask(raw string) DebugEventMask {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" || raw == "all" {
		return debugEventMaskAll
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
			mask |= debugEventMaskAll
		case "trigger":
			mask |= debugEventMaskTrigger
		case "touch":
			mask |= debugEventMaskTouch
		case "use":
			mask |= debugEventMaskUse
		case "think":
			mask |= debugEventMaskThink
		case "blocked":
			mask |= debugEventMaskBlocked
		case "physics":
			mask |= debugEventMaskPhysics
		case "frame":
			mask |= debugEventMaskFrame
		case "qc":
			mask |= debugEventMaskQC
		}
	}
	return mask
}

func parseDebugEntityFilter(raw string) debugEntityFilter {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" || raw == "all" || raw == "-1" {
		return debugEntityFilter{all: true}
	}

	filter := debugEntityFilter{allowed: make(map[int]struct{})}
	for _, token := range splitDebugFilterTokens(raw) {
		if token == "" {
			continue
		}
		if start, end, ok := parseDebugEntityRange(token); ok {
			for entNum := start; entNum <= end; entNum++ {
				filter.allowed[entNum] = struct{}{}
			}
			continue
		}
		if entNum, err := strconv.Atoi(token); err == nil {
			filter.allowed[entNum] = struct{}{}
		}
	}
	if len(filter.allowed) == 0 {
		return debugEntityFilter{}
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

func matchesClassnameFilter(raw, classname string) bool {
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

func entityClassname(vm *qc.VM, ent *Edict) string {
	if ent == nil || ent.Vars == nil {
		return ""
	}
	return qcString(vm, ent.Vars.ClassName)
}

func qcString(vm *qc.VM, idx int32) string {
	if vm == nil || idx == 0 {
		return ""
	}
	return vm.GetString(idx)
}

func formatDebugMessage(format string, args ...any) string {
	if format == "" {
		return ""
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

func clampSummaryMode(mode int) int {
	if mode < 0 {
		return 0
	}
	if mode > 2 {
		return 2
	}
	return mode
}
