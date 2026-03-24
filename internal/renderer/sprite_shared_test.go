package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

func TestBuildSpriteRenderModelRetainsFramePixels(t *testing.T) {
	spr := &model.MSprite{
		Type:      spriteTypeVPParallel,
		MaxWidth:  16,
		MaxHeight: 8,
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{{
			Type: model.SpriteFrameSingle,
			FramePtr: &model.MSpriteFrame{
				Width:  16,
				Height: 8,
				Up:     4,
				Down:   -4,
				Left:   -8,
				Right:  8,
				SMax:   1,
				TMax:   1,
				Pixels: []byte{1, 2, 3, 4, 5},
			},
		}},
	}

	renderModel := buildSpriteRenderModel("progs/flame.spr", spr)
	if renderModel == nil {
		t.Fatal("buildSpriteRenderModel returned nil")
	}
	if len(renderModel.frames) != 1 {
		t.Fatalf("frame count = %d, want 1", len(renderModel.frames))
	}
	if got := renderModel.frames[0].pixels; len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Fatalf("pixels = %v, want copied frame pixels", got)
	}
	sprFrame := spr.Frames[0].FramePtr.(*model.MSpriteFrame)
	sprFrame.Pixels[0] = 99
	if renderModel.frames[0].pixels[0] != 1 {
		t.Fatal("buildSpriteRenderModel should copy frame pixels")
	}
}

func TestBuildSpriteQuadVerticesUsesFrameBounds(t *testing.T) {
	sprite := &spriteRenderModel{
		spriteType: spriteTypeVPParallel,
		frames: []spriteRenderFrame{{
			width:  4,
			height: 8,
			up:     4,
			down:   -4,
			left:   -2,
			right:  2,
			sMax:   1,
			tMax:   1,
		}},
	}

	verts := buildSpriteQuadVertices(
		sprite,
		0,
		[3]float32{0, 0, 0},
		[3]float32{10, 20, 30},
		[3]float32{0, 0, 0},
		[3]float32{1, 0, 0},
		[3]float32{0, 1, 0},
		[3]float32{0, 0, 1},
		1,
	)
	if len(verts) != 4 {
		t.Fatalf("vertex count = %d, want 4", len(verts))
	}
	if verts[0].TexCoord != [2]float32{0, 1} || verts[2].TexCoord != [2]float32{1, 0} {
		t.Fatalf("texcoords = %v %v, want sprite quad mapping", verts[0].TexCoord, verts[2].TexCoord)
	}
}

func TestSpriteDataForEntityPrefersExplicitSpriteData(t *testing.T) {
	explicit := &model.MSprite{Type: spriteTypeVPParallel, MaxWidth: 12, MaxHeight: 34, NumFrames: 1}
	entity := SpriteEntity{
		Model: &model.Model{
			Type: model.ModSprite,
			Mins: [3]float32{-8, -8, -8},
			Maxs: [3]float32{8, 8, 8},
		},
		SpriteData: explicit,
	}

	got := spriteDataForEntity(entity)
	if got != explicit {
		t.Fatal("spriteDataForEntity should prefer explicit SpriteData")
	}
}

func TestSpriteDataForEntityFallsBackToModel(t *testing.T) {
	entity := SpriteEntity{
		Model: &model.Model{
			Type: model.ModSprite,
			Mins: [3]float32{-16, -4, -6},
			Maxs: [3]float32{16, 4, 10},
		},
	}

	got := spriteDataForEntity(entity)
	if got == nil {
		t.Fatal("spriteDataForEntity returned nil")
	}
	if got.MaxWidth != 32 {
		t.Fatalf("MaxWidth = %d, want 32", got.MaxWidth)
	}
	if got.MaxHeight != 16 {
		t.Fatalf("MaxHeight = %d, want 16", got.MaxHeight)
	}
	if got.NumFrames != 1 || len(got.Frames) != 1 {
		t.Fatalf("fallback frames = (%d, %d), want (1, 1)", got.NumFrames, len(got.Frames))
	}
}

func TestSpriteDataForEntityFallsBackToModelSpriteDataPayload(t *testing.T) {
	modelSprite := &model.MSprite{
		Type:      spriteTypeVPParallel,
		MaxWidth:  8,
		MaxHeight: 8,
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{{
			Type: model.SpriteFrameSingle,
			FramePtr: &model.MSpriteFrame{
				Width:  8,
				Height: 8,
				Pixels: []byte{7, 8, 9},
			},
		}},
	}
	entity := SpriteEntity{
		Model: &model.Model{
			Type:       model.ModSprite,
			SpriteData: modelSprite,
		},
	}

	got := spriteDataForEntity(entity)
	if got != modelSprite {
		t.Fatal("spriteDataForEntity should prefer Model.SpriteData when explicit SpriteData is nil")
	}
}

func TestSpriteDataFromModelUsesFallbackDimensions(t *testing.T) {
	got := spriteDataFromModel(&model.Model{Type: model.ModSprite})
	if got == nil {
		t.Fatal("spriteDataFromModel returned nil")
	}
	if got.MaxWidth != 64 || got.MaxHeight != 64 {
		t.Fatalf("fallback size = %dx%d, want 64x64", got.MaxWidth, got.MaxHeight)
	}
}
