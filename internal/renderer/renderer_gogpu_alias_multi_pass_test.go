package renderer

import (
	"bytes"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/wgpu"
)

func createDummyAliasDraw(vCount int) gpuAliasDraw {
	verts := make([]model.TriVertX, vCount)
	refs := make([]aliasimpl.MeshRef, vCount)
	for i := range verts {
		verts[i] = model.TriVertX{V: [3]byte{byte(i + 1), 2, 3}}
		refs[i] = aliasimpl.MeshRef{VertexIndex: i, TexCoord: [2]float32{0, 0}}
	}
	hdr := &model.AliasHeader{
		Scale:  types.Vec3{X: 1, Y: 1, Z: 1},
		Frames: []model.AliasFrameDesc{{NumPoses: 1}},
	}
	mdl := &model.Model{AliasHeader: hdr}
	alias := &gpuAliasModel{
		poses: [][]model.TriVertX{verts},
		refs:  refs,
	}
	return gpuAliasDraw{
		skin:   &gpuAliasSkin{bindGroup: &wgpu.BindGroup{}},
		model:  mdl,
		alias:  alias,
		scale:  1.0,
		alpha:  1.0,
		full:   true,
	}
}

// TestPrepareAliasDrawsMultiPassAccumulation verifies that consecutive alias draw
// passes in the same frame (e.g. opaque entities followed by viewmodel) accumulate
// their vertex and uniform data with disjoint offsets instead of overwriting offset 0.
func TestPrepareAliasDrawsMultiPassAccumulation(t *testing.T) {
	dc := &DrawContext{}

	// --- Pass 1: Opaque alias entities (e.g. 2 entities: 3 verts + 6 verts) ---
	draw1 := createDummyAliasDraw(3)
	draw2 := createDummyAliasDraw(6)
	vp := types.IdentityMatrix()
	cam := types.Vec3{X: 10, Y: 20, Z: 30}
	fogColor := types.Vec3{X: 0.1, Y: 0.2, Z: 0.3}

	start1 := dc.prepareAliasDraws([]gpuAliasDraw{draw1, draw2}, vp, cam, fogColor, 0.0)
	if start1 != 0 {
		t.Fatalf("pass 1 startDrawIndex = %d, want 0", start1)
	}
	if len(dc.aliasPreparedScratch) != 2 {
		t.Fatalf("pass 1 prepared count = %d, want 2", len(dc.aliasPreparedScratch))
	}
	if dc.aliasVertexOffsets[0] != 0 {
		t.Errorf("draw 0 vertex offset = %d, want 0", dc.aliasVertexOffsets[0])
	}
	if dc.aliasVertexCounts[0] != 3 {
		t.Errorf("draw 0 vertex count = %d, want 3", dc.aliasVertexCounts[0])
	}
	expectedDraw1VertexOffset := uint64(3 * aliasVertexStride)
	if dc.aliasVertexOffsets[1] != expectedDraw1VertexOffset {
		t.Errorf("draw 1 vertex offset = %d, want %d", dc.aliasVertexOffsets[1], expectedDraw1VertexOffset)
	}
	if dc.aliasVertexCounts[1] != 6 {
		t.Errorf("draw 1 vertex count = %d, want 6", dc.aliasVertexCounts[1])
	}
	if dc.aliasUniformOffsets[0] != 0 {
		t.Errorf("draw 0 uniform offset = %d, want 0", dc.aliasUniformOffsets[0])
	}
	if dc.aliasUniformOffsets[1] != worldUniformAlign {
		t.Errorf("draw 1 uniform offset = %d, want %d", dc.aliasUniformOffsets[1], worldUniformAlign)
	}

	totalPass1Vertices := uint64((3 + 6) * aliasVertexStride)
	if uint64(len(dc.aliasBulkVertexData)) != totalPass1Vertices {
		t.Fatalf("pass 1 vertex data len = %d, want %d", len(dc.aliasBulkVertexData), totalPass1Vertices)
	}
	totalPass1Uniforms := 2 * worldUniformAlign
	if len(dc.aliasBulkUniformData) != int(totalPass1Uniforms) {
		t.Fatalf("pass 1 uniform data len = %d, want %d", len(dc.aliasBulkUniformData), totalPass1Uniforms)
	}

	// Snapshot pass 1 data
	pass1VertexSnapshot := append([]byte(nil), dc.aliasBulkVertexData...)
	pass1UniformSnapshot := append([]byte(nil), dc.aliasBulkUniformData...)

	// --- Pass 2: Viewmodel pass in same frame (1 entity: 4 verts) ---
	drawVM := createDummyAliasDraw(4)
	start2 := dc.prepareAliasDraws([]gpuAliasDraw{drawVM}, vp, cam, fogColor, 0.0)
	if start2 != 2 {
		t.Fatalf("pass 2 startDrawIndex = %d, want 2", start2)
	}
	if len(dc.aliasPreparedScratch) != 3 {
		t.Fatalf("pass 2 total prepared count = %d, want 3", len(dc.aliasPreparedScratch))
	}

	// Verify Pass 2 draw offset is positioned AFTER Pass 1
	if dc.aliasVertexOffsets[2] != totalPass1Vertices {
		t.Errorf("pass 2 (draw 2) vertex offset = %d, want %d (must not overwrite pass 1 at offset 0!)",
			dc.aliasVertexOffsets[2], totalPass1Vertices)
	}
	if dc.aliasVertexCounts[2] != 4 {
		t.Errorf("pass 2 vertex count = %d, want 4", dc.aliasVertexCounts[2])
	}
	expectedVMUniformOffset := uint32(2 * worldUniformAlign)
	if dc.aliasUniformOffsets[2] != expectedVMUniformOffset {
		t.Errorf("pass 2 uniform offset = %d, want %d (must not overwrite pass 1 at offset 0!)",
			dc.aliasUniformOffsets[2], expectedVMUniformOffset)
	}

	// Verify Pass 1 data was NOT overwritten
	if !bytes.Equal(dc.aliasBulkVertexData[:len(pass1VertexSnapshot)], pass1VertexSnapshot) {
		t.Fatal("pass 2 overwrote pass 1 vertex data!")
	}
	if !bytes.Equal(dc.aliasBulkUniformData[:len(pass1UniformSnapshot)], pass1UniformSnapshot) {
		t.Fatal("pass 2 overwrote pass 1 uniform data!")
	}

	// Verify total accumulated size
	expectedTotalVertexLen := int(totalPass1Vertices + 4*aliasVertexStride)
	if len(dc.aliasBulkVertexData) != expectedTotalVertexLen {
		t.Errorf("total vertex data len = %d, want %d", len(dc.aliasBulkVertexData), expectedTotalVertexLen)
	}
	expectedTotalUniformLen := 3 * worldUniformAlign
	if len(dc.aliasBulkUniformData) != int(expectedTotalUniformLen) {
		t.Errorf("total uniform data len = %d, want %d", len(dc.aliasBulkUniformData), expectedTotalUniformLen)
	}

	// Snapshot pass 1 + 2 data
	pass2VertexSnapshot := append([]byte(nil), dc.aliasBulkVertexData...)
	pass2UniformSnapshot := append([]byte(nil), dc.aliasBulkUniformData...)

	// --- Pass 3: Translucent alias models via OIT (1 entity: 5 verts) ---
	drawOIT := createDummyAliasDraw(5)
	start3 := dc.prepareAliasDraws([]gpuAliasDraw{drawOIT}, vp, cam, fogColor, 0.0)
	if start3 != 3 {
		t.Fatalf("pass 3 startDrawIndex = %d, want 3", start3)
	}
	if len(dc.aliasPreparedScratch) != 4 {
		t.Fatalf("pass 3 total prepared count = %d, want 4", len(dc.aliasPreparedScratch))
	}
	expectedOITVertexOffset := uint64(expectedTotalVertexLen)
	if dc.aliasVertexOffsets[3] != expectedOITVertexOffset {
		t.Errorf("pass 3 (draw 3) vertex offset = %d, want %d", dc.aliasVertexOffsets[3], expectedOITVertexOffset)
	}
	if dc.aliasVertexCounts[3] != 5 {
		t.Errorf("pass 3 vertex count = %d, want 5", dc.aliasVertexCounts[3])
	}
	expectedOITUniformOffset := uint32(3 * worldUniformAlign)
	if dc.aliasUniformOffsets[3] != expectedOITUniformOffset {
		t.Errorf("pass 3 uniform offset = %d, want %d", dc.aliasUniformOffsets[3], expectedOITUniformOffset)
	}
	if !bytes.Equal(dc.aliasBulkVertexData[:len(pass2VertexSnapshot)], pass2VertexSnapshot) {
		t.Fatal("pass 3 overwrote pass 1 or 2 vertex data!")
	}
	if !bytes.Equal(dc.aliasBulkUniformData[:len(pass2UniformSnapshot)], pass2UniformSnapshot) {
		t.Fatal("pass 3 overwrote pass 1 or 2 uniform data!")
	}

	// --- Test empty / invalid draw skipping ---
	invalidDrawNilSkin := createDummyAliasDraw(3)
	invalidDrawNilSkin.skin = nil
	invalidDrawNilBindGroup := createDummyAliasDraw(3)
	invalidDrawNilBindGroup.skin.bindGroup = nil
	emptyStart := dc.prepareAliasDraws([]gpuAliasDraw{invalidDrawNilSkin, invalidDrawNilBindGroup}, vp, cam, fogColor, 0.0)
	if emptyStart != 4 {
		t.Fatalf("invalid draws startDrawIndex = %d, want 4", emptyStart)
	}
	if len(dc.aliasPreparedScratch) != 4 {
		t.Fatalf("invalid draws should not have added prepared draws, got %d", len(dc.aliasPreparedScratch))
	}

	// --- Frame End / Reset ---
	dc.resetAliasBuffers()
	if len(dc.aliasPreparedScratch) != 0 {
		t.Errorf("after reset: aliasPreparedScratch len = %d, want 0", len(dc.aliasPreparedScratch))
	}
	if len(dc.aliasBulkVertexData) != 0 {
		t.Errorf("after reset: aliasBulkVertexData len = %d, want 0", len(dc.aliasBulkVertexData))
	}
	if len(dc.aliasBulkUniformData) != 0 {
		t.Errorf("after reset: aliasBulkUniformData len = %d, want 0", len(dc.aliasBulkUniformData))
	}
	if len(dc.aliasVertexOffsets) != 0 {
		t.Errorf("after reset: aliasVertexOffsets len = %d, want 0", len(dc.aliasVertexOffsets))
	}
	if len(dc.aliasVertexCounts) != 0 {
		t.Errorf("after reset: aliasVertexCounts len = %d, want 0", len(dc.aliasVertexCounts))
	}
	if len(dc.aliasUniformOffsets) != 0 {
		t.Errorf("after reset: aliasUniformOffsets len = %d, want 0", len(dc.aliasUniformOffsets))
	}
}
