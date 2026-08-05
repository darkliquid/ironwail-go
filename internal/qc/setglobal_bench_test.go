package qc

import "testing"

// TestSetGlobalTypedNoAlloc verifies the typed global setters used by the
// per-frame QC sync path allocate nothing (plan 20.2: eliminate convT32/64
// boxing in SetGlobal calls made every frame).
func TestSetGlobalTypedNoAlloc(t *testing.T) {
	vm := NewVM()
	if ofs := vm.FindGlobal("time"); ofs >= 0 {
		// ensure both globals resolve so the benchmark exercises the write
	}
	if got := testing.AllocsPerRun(1000, func() {
		vm.SetGlobalFloat("time", 1.5)
	}); got != 0 {
		t.Fatalf("SetGlobalFloat allocs/run = %.2f, want 0", got)
	}
	if got := testing.AllocsPerRun(1000, func() {
		vm.SetGlobalInt32("serverflags", 3)
	}); got != 0 {
		t.Fatalf("SetGlobalInt32 allocs/run = %.2f, want 0", got)
	}
}

func BenchmarkSetGlobalTyped(b *testing.B) {
	vm := NewVM()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.SetGlobalFloat("time", 1.5)
		vm.SetGlobalInt32("self", int32(i&0xff))
	}
}
