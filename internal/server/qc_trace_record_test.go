package server

import (
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// newQCTraceTestServer boots the server with pak0.pak start map + in-memory progs
// and returns a server ready to execute QC (the inspector's observer is wired
// into executeQCFunction).
func newQCTraceTestServer(t *testing.T) *Server {
	t.Helper()
	pakPath := testutil.SkipIfNoPak0(t)
	vfs := fs.NewFileSystem()
	baseDir := filepath.Dir(filepath.Dir(pakPath))
	if err := vfs.AddGameDirectory(filepath.Join(baseDir, "id1")); err != nil {
		t.Fatalf("AddGameDirectory(%q): %v", baseDir, err)
	}
	srv := NewServer()
	qc.RegisterBuiltins(srv.QCVM)
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init server: %v", err)
	}
	if err := srv.SpawnServer("start", vfs); err != nil {
		t.Fatalf("SpawnServer(\"start\"): %v", err)
	}
	t.Cleanup(func() { vfs.Close() })
	return srv
}

// TestQCTraceObserverRecordsCalls verifies the walkthrough inspector's QuakeC
// layer data source: executing real QC fills the retained observer ring and
// counts calls. Uses the no-assets synthetic map + in-memory progs, so this
// passes without Quake data or a C binary.
func TestQCTraceObserverRecordsCalls(t *testing.T) {
	srv := newQCTraceTestServer(t)

	idx := srv.QCVM.FindFunction("ClientConnect")
	if idx < 0 {
		t.Fatalf("ClientConnect not found in compiled progs")
	}
	srv.QCVM.SetGlobal("self", 1)
	if err := srv.executeQCFunction(idx); err != nil {
		t.Fatalf("executeQCFunction(ClientConnect): %v", err)
	}

	events, counts := srv.QCTraceSnapshot()
	if len(events) == 0 {
		t.Fatalf("QC trace ring empty after executing a function; observer not wired")
	}
	total := int32(0)
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		t.Fatalf("no call counts recorded")
	}
	foundCall := false
	for _, e := range events {
		if e.Phase == "enter" && e.Function != "" {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Fatalf("no 'call' events in ring: %+v", events)
	}
}

// TestQCTraceObserverRingBounded verifies the ring never grows past its cap
// even after many QC calls (the inspector panel reads a fixed-size buffer).
func TestQCTraceObserverRingBounded(t *testing.T) {
	srv := newQCTraceTestServer(t)

	idx := srv.QCVM.FindFunction("ClientConnect")
	if idx < 0 {
		t.Fatalf("ClientConnect not found")
	}
	for i := 0; i < maxQCObservedEvents*4; i++ {
		srv.QCVM.SetGlobal("self", 1)
		if err := srv.executeQCFunction(idx); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	events, _ := srv.QCTraceSnapshot()
	if len(events) > maxQCObservedEvents {
		t.Fatalf("ring exceeds cap: len=%d cap=%d", len(events), maxQCObservedEvents)
	}
	if len(events) == 0 {
		t.Fatalf("ring empty after many calls")
	}
}
