package server

import (
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestParseDebugEventMask(t *testing.T) {
	t.Run("named mask", func(t *testing.T) {
		got := parseDebugEventMask("trigger,qc|frame")
		want := debugEventMaskTrigger | debugEventMaskQC | debugEventMaskFrame
		if got != want {
			t.Fatalf("parseDebugEventMask() = %v, want %v", got, want)
		}
	})

	t.Run("numeric mask", func(t *testing.T) {
		if got := parseDebugEventMask("0x21"); got != debugEventMaskTrigger|debugEventMaskPhysics {
			t.Fatalf("parseDebugEventMask() = %v, want %v", got, debugEventMaskTrigger|debugEventMaskPhysics)
		}
	})

	t.Run("default all", func(t *testing.T) {
		if got := parseDebugEventMask(""); got != debugEventMaskAll {
			t.Fatalf("parseDebugEventMask() = %v, want %v", got, debugEventMaskAll)
		}
	})
}

func TestParseDebugEntityFilter(t *testing.T) {
	filter := parseDebugEntityFilter("1,4-6")
	for _, entNum := range []int{1, 4, 5, 6} {
		if !filter.Matches(entNum) {
			t.Fatalf("filter should match ent %d", entNum)
		}
	}
	for _, entNum := range []int{0, 2, 3, 7, -1} {
		if filter.Matches(entNum) {
			t.Fatalf("filter unexpectedly matched ent %d", entNum)
		}
	}
}

func TestMatchesClassnameFilter(t *testing.T) {
	if !matchesClassnameFilter("trigger_*", "trigger_multiple") {
		t.Fatal("glob filter did not match classname")
	}
	if matchesClassnameFilter("func_door", "trigger_multiple") {
		t.Fatal("exact classname filter should not match different classname")
	}
}

func TestDebugTelemetryLogEventHonorsFiltersAndFormatsSnapshot(t *testing.T) {
	vm := qc.NewVM()
	ent := &Edict{Vars: &EntVars{Origin: [3]float32{128, 64, 32}}}
	ent.Vars.ClassName = vm.AllocString("trigger_multiple")
	ent.Vars.TargetName = vm.AllocString("door1")
	ent.Vars.Target = vm.AllocString("torch1")
	ent.Vars.Model = vm.AllocString("*3")

	lines := make([]string, 0, 1)
	telemetry := NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:         true,
			EventMask:       debugEventMaskTrigger,
			ClassnameFilter: "trigger_*",
			EntityFilter:    parseDebugEntityFilter("7"),
			SummaryMode:     1,
			QCVerbosity:     1,
		}
	}, func(line string) {
		lines = append(lines, line)
	})

	telemetry.BeginFrame(1.25, 0.05)
	if !telemetry.LogEventf(DebugEventTrigger, vm, 7, ent, "opened %s", "door") {
		t.Fatal("LogEventf() returned false")
	}
	if telemetry.LogEventf(DebugEventUse, vm, 7, ent, "should be filtered") {
		t.Fatal("LogEventf() unexpectedly logged filtered event")
	}

	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1", len(lines))
	}
	line := lines[0]
	for _, want := range []string{
		"kind=trigger",
		`ent=7 classname="trigger_multiple" targetname="door1" target="torch1" model="*3" origin=(128.0 64.0 32.0)`,
		"opened door",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
}

func TestDebugTelemetrySummaryAndQCFormatting(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("monster_use"), FirstStatement: 42},
	}

	ent := &Edict{Vars: &EntVars{Origin: [3]float32{8, 16, 24}}}
	ent.Vars.ClassName = vm.AllocString("monster_ogre")
	ent.Vars.TargetName = vm.AllocString("ogre1")

	lines := make([]string, 0, 2)
	telemetry := NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:         true,
			EventMask:       debugEventMaskAll,
			ClassnameFilter: "monster_*",
			EntityFilter:    parseDebugEntityFilter("3"),
			SummaryMode:     1,
			QCTrace:         true,
			QCVerbosity:     2,
		}
	}, func(line string) {
		lines = append(lines, line)
	})

	telemetry.BeginFrame(2.5, 0.1)
	if !telemetry.LogQCEventf("enter", 2, 3, 1, vm, 3, ent, "trace") {
		t.Fatal("LogQCEventf() returned false")
	}
	telemetry.EndFrame()

	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "depth=3") || !strings.Contains(lines[0], "fn=monster_use[#1]") {
		t.Fatalf("qc line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "summary total=1 qc=1") || !strings.Contains(lines[1], "counts=qc=1") {
		t.Fatalf("summary line = %q", lines[1])
	}
}

func TestDebugTelemetrySummaryModeTwoLogsEmptyFrames(t *testing.T) {
	lines := make([]string, 0, 1)
	telemetry := NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskAll,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  2,
			QCVerbosity:  1,
		}
	}, func(line string) {
		lines = append(lines, line)
	})

	telemetry.BeginFrame(3, 0.1)
	telemetry.EndFrame()

	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "summary total=0 qc=0") {
		t.Fatalf("summary line = %q", lines[0])
	}
}
