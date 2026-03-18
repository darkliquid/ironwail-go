package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestEncodeAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "zero", in: 0.0, want: 1},
		{name: "half", in: 0.5, want: 128},
		{name: "one", in: 1.0, want: 255},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encodeAlpha(tc.in); got != tc.want {
				t.Fatalf("encodeAlpha(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEncodeScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "one", in: 1.0, want: 16},
		{name: "two", in: 2.0, want: 32},
		{name: "zero", in: 0.0, want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encodeScale(tc.in); got != tc.want {
				t.Fatalf("encodeScale(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEntityStateForClient_AlphaScaleDefaultsWhenFieldsMissing(t *testing.T) {
	t.Parallel()

	s := &Server{
		QCVM:         newTestQCVM(),
		QCFieldAlpha: -1,
		QCFieldScale: -1,
	}
	ent := &Edict{
		Vars:  &EntVars{},
		Alpha: 77,
		Scale: 99,
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 0 {
		t.Fatalf("state.Alpha = %d, want 0", state.Alpha)
	}
	if state.Scale != 16 {
		t.Fatalf("state.Scale = %d, want 16", state.Scale)
	}
}

func TestEntityStateForClient_ReadsQCAlphaScale(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.SetEFloat(1, 0, 0.5) // alpha
	vm.SetEFloat(1, 1, 2.0) // scale

	s := &Server{
		QCVM:         vm,
		QCFieldAlpha: 0,
		QCFieldScale: 1,
	}
	ent := &Edict{
		Vars: &EntVars{},
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 128 {
		t.Fatalf("state.Alpha = %d, want 128", state.Alpha)
	}
	if state.Scale != 32 {
		t.Fatalf("state.Scale = %d, want 32", state.Scale)
	}
}

func newTestQCVM() *qc.VM {
	vm := &qc.VM{
		NumEdicts: 2,
		EdictSize: 28 + 8, // prefix + 2 float fields
	}
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	return vm
}
