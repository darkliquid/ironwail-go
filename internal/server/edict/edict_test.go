package edict

import (
	"testing"

	types "github.com/darkliquid/ironwail-go/internal/server/types"
)

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
