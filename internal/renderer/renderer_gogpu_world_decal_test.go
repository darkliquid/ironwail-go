package renderer

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestGoGPUDecalMarkParamsPreserveGeometry(t *testing.T) {
	mark := DecalMarkEntity{
		Origin:   types.Vec3{X: 1, Y: 2, Z: 3},
		Normal:   types.Vec3{X: 0, Y: 0, Z: 1},
		Size:     16,
		Rotation: 0.25,
		Variant:  DecalVariantScorch,
	}

	got := gogpuDecalMarkParams(mark)
	want := worldgogpu.DecalMarkParams{
		Origin:   mark.Origin,
		Normal:   mark.Normal,
		Size:     mark.Size,
		Rotation: mark.Rotation,
		Variant:  int(mark.Variant),
	}
	if got != want {
		t.Fatalf("gogpuDecalMarkParams() = %+v, want %+v", got, want)
	}
}

func TestGoGPUPreparedDecalMarkClampsFinalColorAndAlpha(t *testing.T) {
	draw := decalDraw{
		mark: DecalMarkEntity{
			Origin:   types.Vec3{X: 1, Y: 2, Z: 3},
			Normal:   types.Vec3{X: 0, Y: 0, Z: 1},
			Size:     12,
			Rotation: 0.5,
			Alpha:    1.5,
			Color:    types.Vec3{X: -0.25, Y: 0.4, Z: 2.0},
			Variant:  DecalVariantMagic,
		},
	}

	got, ok := gogpuDecalPreparedMark(draw)
	if !ok {
		t.Fatal("gogpuDecalPreparedMark() = !ok, want ok")
	}
	if got.Params != gogpuDecalMarkParams(draw.mark) {
		t.Fatalf("Params = %+v, want %+v", got.Params, gogpuDecalMarkParams(draw.mark))
	}
	wantColor := [4]float32{0, 0.4, 1, 1}
	if got.Color != wantColor {
		t.Fatalf("Color = %v, want %v", got.Color, wantColor)
	}
}

func TestGoGPUDecalQuadUsesRootBuilder(t *testing.T) {
	params := worldgogpu.DecalMarkParams{
		Origin:   types.Vec3{X: 100, Y: 200, Z: 300},
		Normal:   types.Vec3{X: 0, Y: 0, Z: 1},
		Size:     16,
		Rotation: 0.75,
		Variant:  int(DecalVariantChip),
	}

	got, gotOK := gogpuDecalQuad(params)
	want, wantOK := buildDecalQuad(DecalMarkEntity{
		Origin:   params.Origin,
		Normal:   params.Normal,
		Size:     params.Size,
		Rotation: params.Rotation,
	})
	if gotOK != wantOK {
		t.Fatalf("ok = %v, want %v", gotOK, wantOK)
	}
	if got != want {
		t.Fatalf("quad = %v, want %v", got, want)
	}
}

func TestPrepareGoGPUDecalHALDrawsUsesRootAdapterSeam(t *testing.T) {
	draws := []decalDraw{{
		mark: DecalMarkEntity{
			Origin:   types.Vec3{X: 10, Y: 20, Z: 30},
			Normal:   types.Vec3{X: 0, Y: 0, Z: 1},
			Size:     8,
			Rotation: 0.5,
			Alpha:    1.5,
			Color:    types.Vec3{X: -0.25, Y: 0.4, Z: 2.0},
			Variant:  DecalVariantScorch,
		},
	}}

	got := prepareGoGPUDecalHALDraws(draws)
	if len(got) != 1 {
		t.Fatalf("len(prepareGoGPUDecalHALDraws()) = %d, want 1", len(got))
	}
	if got[0].VertexCount != 6 {
		t.Fatalf("VertexCount = %d, want 6", got[0].VertexCount)
	}
	if len(got[0].VertexBytes) != 6*36 {
		t.Fatalf("len(VertexBytes) = %d, want %d", len(got[0].VertexBytes), 6*36)
	}

	firstU := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[12:16]))
	firstV := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[16:20]))
	if firstU != 0 || firstV != 0.5 {
		t.Fatalf("first uv = (%v,%v), want (0,0.5)", firstU, firstV)
	}

	firstR := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[20:24]))
	firstG := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[24:28]))
	firstB := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[28:32]))
	firstA := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[32:36]))
	if firstR != 0 || firstG != 0.4 || firstB != 1 || firstA != 1 {
		t.Fatalf("first color = (%v,%v,%v,%v), want (0,0.4,1,1)", firstR, firstG, firstB, firstA)
	}
}

func TestAliasVertexBytesIntoReusesBuffer(t *testing.T) {
	vertices := []WorldVertex{
		{
			Position:      types.Vec3{X: 1, Y: 2, Z: 3},
			TexCoord:      [2]float32{0.25, 0.75},
			LightmapCoord: [2]float32{0.5, 0.125},
			Normal:        types.Vec3{X: 0, Y: 0, Z: 1},
		},
	}
	scratch := make([]byte, 0, len(vertices)*aliasVertexStride)
	before := &scratch[:cap(scratch)][0]

	got := aliasVertexBytesInto(scratch, vertices)
	if len(got) != len(vertices)*aliasVertexStride {
		t.Fatalf("len(aliasVertexBytesInto()) = %d, want %d", len(got), len(vertices)*aliasVertexStride)
	}
	after := &got[:cap(got)][0]
	if before != after {
		t.Fatal("expected aliasVertexBytesInto to reuse caller buffer")
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = aliasVertexBytesInto(scratch, vertices)
	})
	if allocs != 0 {
		t.Fatalf("aliasVertexBytesInto allocated %.2f times per run, want 0", allocs)
	}
}

func TestBuildAliasVerticesInterpolatedIntoReusesBuffer(t *testing.T) {
	alias := &gpuAliasModel{
		poses: [][]model.TriVertX{
			{{V: [3]byte{1, 0, 0}, LightNormalIndex: 0}},
			{{V: [3]byte{3, 0, 0}, LightNormalIndex: 0}},
		},
		refs: []aliasimpl.MeshRef{{VertexIndex: 0, TexCoord: [2]float32{0.25, 0.75}}},
	}
	mdl := &model.Model{
		AliasHeader: &model.AliasHeader{
			Scale:       types.Vec3{X: 1, Y: 1, Z: 1},
			ScaleOrigin: types.Vec3{},
		},
	}
	input := make([]WorldVertex, 0, 4)
	input = append(input, WorldVertex{})
	input = input[:0]
	before := &input[:cap(input)][0]

	got := buildAliasVerticesInterpolatedInto(input, alias, mdl, 0, 1, 0.5, types.Vec3{X: 10, Y: 20, Z: 30}, types.Vec3{X: 0, Y: 90, Z: 0}, 2, false)
	if len(got) != 1 {
		t.Fatalf("len(buildAliasVerticesInterpolatedInto()) = %d, want 1", len(got))
	}
	after := &got[:cap(got)][0]
	if before != after {
		t.Fatal("expected buildAliasVerticesInterpolatedInto to reuse caller buffer")
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = buildAliasVerticesInterpolatedInto(input, alias, mdl, 0, 1, 0.5, types.Vec3{X: 10, Y: 20, Z: 30}, types.Vec3{X: 0, Y: 90, Z: 0}, 2, false)
	})
	if allocs != 0 {
		t.Fatalf("buildAliasVerticesInterpolatedInto allocated %.2f times per run, want 0", allocs)
	}
}
