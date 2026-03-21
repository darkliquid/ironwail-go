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
