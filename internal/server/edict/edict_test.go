package edict

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
)

// newTestVM returns a minimal QCVM with known field definitions at fixed
// offsets: classname (EvString @8), health (EvFloat @24), mins/maxs/size
// (EvVector @64/68/72). It must have EdictSize large enough to hold them.
func newTestVM() *qc.VM {
	vm := qc.NewVM()
	vm.EdictSize = 128
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*4)
	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: 8, Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvFloat), Ofs: 24, Name: vm.AllocString("health")},
		{Type: uint16(qc.EvVector), Ofs: 64, Name: vm.AllocString("mins")},
		{Type: uint16(qc.EvVector), Ofs: 68, Name: vm.AllocString("maxs")},
		{Type: uint16(qc.EvVector), Ofs: 72, Name: vm.AllocString("size")},
	}
	return vm
}

func TestEDAllocGrowsPool(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	for i := 0; i < 4; i++ {
		n, err := m.ED_Alloc()
		if err != nil {
			t.Fatalf("ED_Alloc(%d) = %v, want nil", i, err)
		}
		if n != i {
			t.Fatalf("ED_Alloc(%d) = %d, want %d", i, n, i)
		}
	}
	if _, err := m.ED_Alloc(); err == nil {
		t.Fatal("ED_Alloc on full pool = nil, want error")
	}
}

func TestEDFreeThenAllocAtLateTimeKeepsCooldown(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	// Allocate all 4 so the pool is full; reuse must come from the free list.
	ids := make([]int, 4)
	for i := range ids {
		ids[i], _ = m.ED_Alloc()
	}
	m.SetCurrentTime(3) // absolute time >= 2 → cooldown applies
	if err := m.ED_Free(ids[0]); err != nil {
		t.Fatalf("ED_Free = %v", err)
	}
	// Freed at t=3, cooldown window is 0.5s; still at t=3 → the freed slot is
	// held, and with a full pool no fresh slot exists → ED_Alloc errors.
	if _, err := m.ED_Alloc(); err == nil {
		t.Fatal("ED_Alloc = nil, want error (freed slot within cooldown, pool full)")
	}
}

func TestEDFreeReuseAfterCooldown(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	ids := make([]int, 4)
	for i := range ids {
		ids[i], _ = m.ED_Alloc()
	}
	m.SetCurrentTime(3)
	_ = m.ED_Free(ids[0])
	m.SetCurrentTime(4) // 1s later → past 0.5s cooldown
	next, err := m.ED_Alloc()
	if err != nil {
		t.Fatalf("ED_Alloc = %v", err)
	}
	if next != ids[0] {
		t.Fatalf("expected reuse of %d after cooldown, got %d", ids[0], next)
	}
}

func TestEDParseEdictSimple(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	raw := `{"classname" "monster_ogre" "origin" "0 0 0"}`
	remainder, err := m.ED_ParseEdict(raw, 1)
	if err != nil {
		t.Fatalf("ED_ParseEdict = %v", err)
	}
	if remainder != "" {
		t.Fatalf("remainder = %q, want empty", remainder)
	}
	// ED_ParseEdict initializes the edict slot but does not bump numEdicts
	// (only ED_Alloc grows the pool); slot 1 must be addressable.
	if e := m.Edict(1); e == nil {
		t.Fatal("Edict(1) = nil after parse, want allocated")
	}
}

func TestEDParseStructBlocks(t *testing.T) {
	m := NewEmptyManager(8, 0, nil, nil)
	raw := `{"classname" "worldspawn"}
{"classname" "info_player_start"}`
	remaining := raw
	var count int
	for remaining != "" {
		var err error
		remaining, err = m.ED_ParseEdict(remaining, count+1)
		if err != nil {
			t.Fatalf("ED_ParseEdict block %d = %v (remaining %q)", count, err, remaining)
		}
		count++
		if count > 10 {
			t.Fatal("parse loop did not terminate")
		}
	}
	if count != 2 {
		t.Fatalf("parsed %d entities, want 2", count)
	}
}

func TestEDParseGlobalsNoBrace(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	if _, err := m.ED_ParseGlobals("no brace", nil); err == nil {
		t.Fatal("ED_ParseGlobals without brace = nil, want error")
	}
}

func TestEDParseGlobalsSkipsUnknown(t *testing.T) {
	m := NewEmptyManager(4, 0, nil, nil)
	if _, err := m.ED_ParseGlobals(`{"unknown_key" "1"}`, nil); err != nil {
		t.Fatalf("ED_ParseGlobals = %v", err)
	}
}

func TestEdictAccessor(t *testing.T) {
	m := NewEmptyManager(2, 0, nil, nil)
	n, _ := m.ED_Alloc()
	e := m.Edict(n)
	if e == nil {
		t.Fatal("Edict(n) = nil")
	}
	if e.Scale != 16 {
		t.Fatalf("Scale = %d, want 16 (ENTSCALE_DEFAULT)", e.Scale)
	}
	if m.Edict(-1) != nil || m.Edict(100) != nil {
		t.Fatal("Edict out of range = non-nil")
	}
}

func TestManagerUsesTypesEdict(t *testing.T) {
	// Construction with a types-package pool (compile-time + runtime check).
	pool := make([]*types.Edict, 2)
	m := NewManager(pool, nil, 2, 0, 0, make([]float32, 2), nil, nil)
	if m.NumEdicts() != 0 {
		t.Fatalf("NumEdicts = %d, want 0", m.NumEdicts())
	}
}

func TestFieldDefWithVMDefs(t *testing.T) {
	vm := newTestVM()
	em := NewEmptyManager(4, 0, nil, nil)
	em.vm = vm
	if em.fieldDefMap != nil {
		t.Fatal("fieldDefMap should start nil (lazy init)")
	}
	ofs, etype, ok := em.fieldDef("classname")
	if !ok {
		t.Fatal("fieldDef(classname) = !ok with injected FieldDefs")
	}
	if ofs != 8 {
		t.Fatalf("fieldDef(classname) ofs = %d, want 8", ofs)
	}
	if etype != qc.EvString {
		t.Fatalf("fieldDef(classname) type = %v, want EvString", etype)
	}
}

func TestParseEdictWithVMWritesFields(t *testing.T) {
	vm := newTestVM()
	em := NewEmptyManager(4, 0, nil, nil)
	em.vm = vm
	raw := `{"classname" "monster" "health" "50"}`
	if _, err := em.ED_ParseEdict(raw, 1); err != nil {
		t.Fatalf("ED_ParseEdict = %v", err)
	}
	// health is EvFloat at 24 via injected defs; classname is a string idx.
	ofs, etype, _ := em.fieldDef("health")
	if etype != qc.EvFloat {
		t.Fatalf("health type = %v, want EvFloat", etype)
	}
	if got := vm.EFloat(1, ofs); got != 50 {
		t.Fatalf("vm.EFloat(health) = %v, want 50", got)
	}
	if got := vm.EString(1, 8); vm.String(got) != "monster" {
		t.Fatalf("vm.String(EString(classname)) = %q, want %q", vm.String(got), "monster")
	}
}

func TestParseEdictRecalculatesSize(t *testing.T) {
	vm := newTestVM()
	em := NewEmptyManager(4, 0, nil, nil)
	em.vm = vm
	raw := `{"mins" "0 0 0" "maxs" "64 32 16"}`
	if _, err := em.ED_ParseEdict(raw, 1); err != nil {
		t.Fatalf("ED_ParseEdict = %v", err)
	}
	minsOfs, _, _ := em.fieldDef("mins")
	maxsOfs, _, _ := em.fieldDef("maxs")
	sizeOfs, _, _ := em.fieldDef("size")
	mins := vm.EVector(1, minsOfs)
	maxs := vm.EVector(1, maxsOfs)
	size := vm.EVector(1, sizeOfs)
	if size != maxs.Sub(mins) {
		t.Fatalf("size = %v, want (64,32,16)", size)
	}
}
