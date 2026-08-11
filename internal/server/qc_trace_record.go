// qc_trace_record.go — QuakeC call observation for the walkthrough inspector.
//
// A bounded ring of the most recent QC function enter/leave events plus
// per-function call counters, filled for every VM call regardless of the
// sv_debug_qc_trace telemetry cvar. The browser's QuakeC layer panel reads
// this through the inspector bridge; it is tiny and never grows.
package server

import (
	"strconv"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// maxQCObservedEvents bounds the retained QC trace ring.
const maxQCObservedEvents = 64

// QCTraceRecord is one observed QuakeC call event.
type QCTraceRecord struct {
	Phase    string // "call" / "return" / builtin phase name
	Function string // resolved function name
	Index    int32  // function table index
	Depth    int
	Self     int // edict number of $self at the event
}

// qcTraceMu guards the observed ring + counters. QC runs on the server (main)
// goroutine while the inspector reads from the JS dispatcher goroutine.
var qcTraceMu sync.Mutex

// recordQCTraceEvent appends an event to the inspector ring. Called from the
// VM TraceCallFunc hook in executeQCFunction (always installed).
func (s *Server) recordQCTraceEvent(vm *qc.VM, event qc.TraceCallEvent) {
	if s == nil || vm == nil {
		return
	}
	name := ""
	if event.FunctionIndex >= 0 && int(event.FunctionIndex) < len(vm.Functions) {
		name = vm.String(vm.Functions[event.FunctionIndex].Name)
		if name == "" {
			name = "#" + itoa32(event.FunctionIndex)
		}
	} else {
		name = "#" + itoa32(event.FunctionIndex)
	}

	rec := QCTraceRecord{
		Phase:    event.Phase,
		Function: name,
		Index:    event.FunctionIndex,
		Depth:    event.Depth,
		Self:     int(vm.GInt(qc.OFSSelf)),
	}

	qcTraceMu.Lock()
	defer qcTraceMu.Unlock()
	if s.qcObservedEvents == nil {
		s.qcObservedEvents = make([]QCTraceRecord, 0, maxQCObservedEvents)
		s.qcCallCounts = make(map[string]int32)
	}
	if len(s.qcObservedEvents) >= maxQCObservedEvents {
		copy(s.qcObservedEvents, s.qcObservedEvents[1:])
		s.qcObservedEvents = s.qcObservedEvents[:maxQCObservedEvents-1]
	}
	s.qcObservedEvents = append(s.qcObservedEvents, rec)
	if event.Phase == "enter" {
		s.qcCallCounts[name]++
	}
}

// QCTraceSnapshot returns a copy of the recent QC events + call counters for
// the inspector. Safe for concurrent readers.
func (s *Server) QCTraceSnapshot() ([]QCTraceRecord, map[string]int32) {
	qcTraceMu.Lock()
	defer qcTraceMu.Unlock()
	events := make([]QCTraceRecord, len(s.qcObservedEvents))
	copy(events, s.qcObservedEvents)
	counts := make(map[string]int32, len(s.qcCallCounts))
	for k, v := range s.qcCallCounts {
		counts[k] = v
	}
	return events, counts
}

func itoa32(v int32) string { return strconv.Itoa(int(v)) }
