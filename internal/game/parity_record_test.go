// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestRenderRecordHashDeterministic(t *testing.T) {
	rec1 := RenderRecord{
		ViewLeaf: 10,
		ViewOrg:  [3]int32{100, 200, 300},
		Entities: []RenderRecordEntity{
			{ModelIndex: 1, Frame: 0, Skin: 0, Effects: 0, OriginX: 10, OriginY: 20, OriginZ: 30},
			{ModelIndex: 2, Frame: 1, Skin: 0, Effects: 0, OriginX: 40, OriginY: 50, OriginZ: 60},
		},
	}

	// Permuted entity insertion order
	rec2 := RenderRecord{
		ViewLeaf: 10,
		ViewOrg:  [3]int32{100, 200, 300},
		Entities: []RenderRecordEntity{
			{ModelIndex: 2, Frame: 1, Skin: 0, Effects: 0, OriginX: 40, OriginY: 50, OriginZ: 60},
			{ModelIndex: 1, Frame: 0, Skin: 0, Effects: 0, OriginX: 10, OriginY: 20, OriginZ: 30},
		},
	}

	hash1 := rec1.Hash()
	hash2 := rec2.Hash()

	if hash1 == "" || hash2 == "" {
		t.Fatalf("hashes should not be empty")
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash invariant to entity ordering: %s != %s", hash1, hash2)
	}

	// Perturbation test: different frame
	rec3 := RenderRecord{
		ViewLeaf: 10,
		ViewOrg:  [3]int32{100, 200, 300},
		Entities: []RenderRecordEntity{
			{ModelIndex: 1, Frame: 2, Skin: 0, Effects: 0, OriginX: 10, OriginY: 20, OriginZ: 30},
			{ModelIndex: 2, Frame: 1, Skin: 0, Effects: 0, OriginX: 40, OriginY: 50, OriginZ: 60},
		},
	}
	hash3 := rec3.Hash()
	if hash3 == hash1 {
		t.Fatalf("expected hash to change upon state perturbation")
	}

	// Perturbation test: different viewleaf
	rec4 := rec1
	rec4.ViewLeaf = 11
	if rec4.Hash() == hash1 {
		t.Fatalf("expected hash to change upon viewleaf change")
	}
}

func TestRenderRecordFromDumpState(t *testing.T) {
	dump := DumpFrameState{
		Frame:    1,
		ViewOrg:  types.Vec3{X: 100.125, Y: 200.25, Z: 300.5},
		ViewLeaf: 5,
		Visedicts: []DumpEdict{
			{
				ModelIndex: 3,
				Frame:      1,
				Skin:       0,
				Origin:     types.Vec3{X: 10.0, Y: 20.0, Z: 30.0},
				Angles:     types.Vec3{X: 0.0, Y: 90.0, Z: 0.0},
			},
		},
	}

	rec := ComputeRenderRecordFromDump(dump)
	if rec.ViewLeaf != 5 {
		t.Fatalf("expected ViewLeaf=5, got %d", rec.ViewLeaf)
	}
	if rec.ViewOrg[0] != 801 || rec.ViewOrg[1] != 1602 || rec.ViewOrg[2] != 2404 {
		t.Fatalf("unexpected quantized ViewOrg: %v", rec.ViewOrg)
	}
	if len(rec.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(rec.Entities))
	}
	if rec.Entities[0].AngleYaw != 720 { // 90 * 8
		t.Fatalf("expected AngleYaw=720, got %d", rec.Entities[0].AngleYaw)
	}
	if rec.Hash() == "" {
		t.Fatalf("expected non-empty hash")
	}
}
