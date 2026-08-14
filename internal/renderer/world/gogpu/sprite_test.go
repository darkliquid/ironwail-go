package gogpu

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestSpriteUniformBytes(t *testing.T) {
	vp := types.Mat4{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}
	cameraOrigin := types.Vec3{X: 10, Y: 20, Z: 30}
	alpha := float32(0.25)
	fogColor := types.Vec3{X: 0.1, Y: 0.2, Z: 0.3}
	fogDensity := float32(0.4)

	data := SpriteUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
	if len(data) != SpriteUniformBufferSize {
		t.Fatalf("len(data) = %d, want %d", len(data), SpriteUniformBufferSize)
	}

	matrixBytes := types.Mat4ToBytes(vp)
	if !bytes.Equal(data[:64], matrixBytes[:]) {
		t.Fatalf("viewProjection matrix bytes mismatch")
	}

	camArr := [3]float32{cameraOrigin.X, cameraOrigin.Y, cameraOrigin.Z}
	for i, want := range camArr {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4:]))
		if got != want {
			t.Fatalf("cameraOrigin[%d] = %v, want %v", i, got, want)
		}
	}

	if got := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80])); got != worldimpl.FogUniformDensity(fogDensity) {
		t.Fatalf("fogDensity = %v, want %v", got, worldimpl.FogUniformDensity(fogDensity))
	}

	fogArr := [3]float32{fogColor.X, fogColor.Y, fogColor.Z}
	for i, want := range fogArr {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[80+i*4:]))
		if got != want {
			t.Fatalf("fogColor[%d] = %v, want %v", i, got, want)
		}
	}

	if got := math.Float32frombits(binary.LittleEndian.Uint32(data[92:96])); got != alpha {
		t.Fatalf("alpha = %v, want %v", got, alpha)
	}
}

func TestSpriteQuadVerticesToWorldVertices(t *testing.T) {
	input := []SpriteQuadVertex{
		{Position: types.Vec3{X: 1, Y: 2, Z: 3}, TexCoord: [2]float32{0.25, 0.5}},
		{Position: types.Vec3{X: -4, Y: 5, Z: -6}, TexCoord: [2]float32{0.75, 1}},
	}

	got := SpriteQuadVerticesToWorldVertices(input)
	if len(got) != len(input) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(input))
	}

	for i := range input {
		if got[i].Position != input[i].Position {
			t.Fatalf("Position[%d] = %v, want %v", i, got[i].Position, input[i].Position)
		}
		if got[i].TexCoord != input[i].TexCoord {
			t.Fatalf("TexCoord[%d] = %v, want %v", i, got[i].TexCoord, input[i].TexCoord)
		}
		if got[i].LightmapCoord != ([2]float32{}) {
			t.Fatalf("LightmapCoord[%d] = %v, want zero", i, got[i].LightmapCoord)
		}
		if got[i].Normal != (types.Vec3{X: 0, Y: 0, Z: 1}) {
			t.Fatalf("Normal[%d] = %v, want [0 0 1]", i, got[i].Normal)
		}
	}
}

func TestProjectSpriteQuadVerticesToWorldVertices(t *testing.T) {
	type rootQuadVertex struct {
		pos types.Vec3
		uv  [2]float32
	}
	input := []rootQuadVertex{
		{pos: types.Vec3{X: 7, Y: 8, Z: 9}, uv: [2]float32{0.1, 0.2}},
		{pos: types.Vec3{X: -1, Y: -2, Z: -3}, uv: [2]float32{0.3, 0.4}},
	}

	got := ProjectSpriteQuadVerticesToWorldVertices(input, func(vertex rootQuadVertex) SpriteQuadVertex {
		return SpriteQuadVertex{Position: vertex.pos, TexCoord: vertex.uv}
	})

	if len(got) != len(input) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(input))
	}
	for i := range input {
		if got[i].Position != input[i].pos {
			t.Fatalf("Position[%d] = %v, want %v", i, got[i].Position, input[i].pos)
		}
		if got[i].TexCoord != input[i].uv {
			t.Fatalf("TexCoord[%d] = %v, want %v", i, got[i].TexCoord, input[i].uv)
		}
	}
}

func TestBuildSpriteDrawClampsFrameAndAlpha(t *testing.T) {
	sprite := &struct{ name string }{"test"}
	spriteData := &model.MSprite{}
	var gotModelID string
	var gotSpriteData *model.MSprite
	draw, ok := BuildSpriteDraw(SpriteDrawParams{
		ModelID:    "progs/flame.spr",
		SpriteData: spriteData,
		Frame:      7,
		Origin:     types.Vec3{X: 1, Y: 2, Z: 3},
		Angles:     types.Vec3{X: 4, Y: 5, Z: 6},
		Alpha:      2,
		Scale:      1.5,
	}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[*struct{ name string }], bool) {
		gotModelID = modelID
		gotSpriteData = resolved
		return ResolvedSpriteModel[*struct{ name string }]{Handle: sprite, FrameCount: 2}, true
	})
	if !ok {
		t.Fatal("BuildSpriteDraw returned !ok")
	}
	if gotModelID != "progs/flame.spr" {
		t.Fatalf("modelID = %q, want progs/flame.spr", gotModelID)
	}
	if gotSpriteData != spriteData {
		t.Fatal("BuildSpriteDraw should pass sprite data to resolver")
	}
	if draw.Sprite != sprite {
		t.Fatal("BuildSpriteDraw should preserve sprite handle")
	}
	if draw.Frame != 0 {
		t.Fatalf("Frame = %d, want 0", draw.Frame)
	}
	if draw.Alpha != 1 {
		t.Fatalf("Alpha = %v, want 1", draw.Alpha)
	}
	if draw.Origin != (types.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("Origin = %v", draw.Origin)
	}
	if draw.Angles != (types.Vec3{X: 4, Y: 5, Z: 6}) {
		t.Fatalf("Angles = %v", draw.Angles)
	}
	if draw.Scale != 1.5 {
		t.Fatalf("Scale = %v, want 1.5", draw.Scale)
	}
}

func TestBuildSpriteDrawRejectsInvisibleOrEmptySprites(t *testing.T) {
	sprite := struct{}{}
	spriteData := &model.MSprite{}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", SpriteData: spriteData, Alpha: 1}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{Handle: sprite, FrameCount: 0}, true
	}); ok {
		t.Fatal("BuildSpriteDraw should reject zero-frame resolved sprites")
	}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", SpriteData: spriteData, Alpha: 0}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{Handle: sprite, FrameCount: 1}, true
	}); ok {
		t.Fatal("BuildSpriteDraw should reject alpha 0")
	}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", SpriteData: spriteData, Alpha: -1}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{Handle: sprite, FrameCount: 1}, true
	}); ok {
		t.Fatal("BuildSpriteDraw should reject negative alpha")
	}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "", SpriteData: spriteData, Alpha: 1}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{Handle: sprite, FrameCount: 1}, true
	}); ok {
		t.Fatal("BuildSpriteDraw should reject empty model ids")
	}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", Alpha: 1}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{Handle: sprite, FrameCount: 1}, true
	}); ok {
		t.Fatal("BuildSpriteDraw should reject nil sprite data")
	}
	var nilResolver func(string, *model.MSprite) (ResolvedSpriteModel[struct{}], bool)
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", SpriteData: spriteData, Alpha: 1}, nilResolver); ok {
		t.Fatal("BuildSpriteDraw should reject nil resolvers")
	}
	if _, ok := BuildSpriteDraw(SpriteDrawParams{ModelID: "sprite", SpriteData: spriteData, Alpha: 1}, func(modelID string, resolved *model.MSprite) (ResolvedSpriteModel[struct{}], bool) {
		return ResolvedSpriteModel[struct{}]{}, false
	}); ok {
		t.Fatal("BuildSpriteDraw should reject resolver misses")
	}
}
