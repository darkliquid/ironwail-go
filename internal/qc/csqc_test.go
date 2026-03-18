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
