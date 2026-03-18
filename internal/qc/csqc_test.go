package qc

import (
	"bytes"
	"testing"
)

func TestNewCSQCCreatesValidInstance(t *testing.T) {
	csqc := NewCSQC()
	if csqc == nil {
		t.Fatal("NewCSQC() returned nil")
	}
	if csqc.VM == nil {
		t.Fatal("NewCSQC().VM is nil")
	}
	if csqc.IsLoaded() {
		t.Fatal("new CSQC instance should not be loaded")
	}
}

func TestCSQCLoadFailsWithInvalidData(t *testing.T) {
	csqc := NewCSQC()

	err := csqc.Load(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("Load() expected error for invalid data")
	}
	if csqc.IsLoaded() {
		t.Fatal("CSQC should not be marked loaded after failed Load()")
	}
}

func TestCSQCIsLoadedFalseBeforeLoad(t *testing.T) {
	csqc := NewCSQC()
	if csqc.IsLoaded() {
		t.Fatal("IsLoaded() = true, want false before Load()")
	}
}

func TestCSQCFunctionIndicesStartAtMinusOne(t *testing.T) {
	csqc := NewCSQC()

	if csqc.initFunc != -1 {
		t.Fatalf("initFunc = %d, want -1", csqc.initFunc)
	}
	if csqc.shutdownFunc != -1 {
		t.Fatalf("shutdownFunc = %d, want -1", csqc.shutdownFunc)
	}
	if csqc.drawHudFunc != -1 {
		t.Fatalf("drawHudFunc = %d, want -1", csqc.drawHudFunc)
	}
	if csqc.drawScoresFunc != -1 {
		t.Fatalf("drawScoresFunc = %d, want -1", csqc.drawScoresFunc)
	}
}

func TestCSQCGlobalsStartAtMinusOne(t *testing.T) {
	csqc := NewCSQC()

	if csqc.globals.cltime != -1 {
		t.Fatalf("globals.cltime = %d, want -1", csqc.globals.cltime)
	}
	if csqc.globals.clframetime != -1 {
		t.Fatalf("globals.clframetime = %d, want -1", csqc.globals.clframetime)
	}
	if csqc.globals.maxclients != -1 {
		t.Fatalf("globals.maxclients = %d, want -1", csqc.globals.maxclients)
	}
	if csqc.globals.intermission != -1 {
		t.Fatalf("globals.intermission = %d, want -1", csqc.globals.intermission)
	}
	if csqc.globals.intermissionTime != -1 {
		t.Fatalf("globals.intermissionTime = %d, want -1", csqc.globals.intermissionTime)
	}
	if csqc.globals.playerLocalNum != -1 {
		t.Fatalf("globals.playerLocalNum = %d, want -1", csqc.globals.playerLocalNum)
	}
	if csqc.globals.playerLocalEntNum != -1 {
		t.Fatalf("globals.playerLocalEntNum = %d, want -1", csqc.globals.playerLocalEntNum)
	}
	if csqc.globals.viewAngles != -1 {
		t.Fatalf("globals.viewAngles = %d, want -1", csqc.globals.viewAngles)
	}
	if csqc.globals.clientCommandFrame != -1 {
		t.Fatalf("globals.clientCommandFrame = %d, want -1", csqc.globals.clientCommandFrame)
	}
	if csqc.globals.serverCommandFrame != -1 {
		t.Fatalf("globals.serverCommandFrame = %d, want -1", csqc.globals.serverCommandFrame)
	}
}

func TestCSQCSyncGlobalsUnloadedNoPanic(t *testing.T) {
	csqc := NewCSQC()
	csqc.SyncGlobals(CSQCFrameState{})
}

func TestCSQCFrameStateZeroValueSafe(t *testing.T) {
	var state CSQCFrameState
	if state.Time != 0 {
		t.Fatalf("state.Time = %f, want 0", state.Time)
	}
}
