//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

// TestUploadSpriteModel tests sprite model upload.
func TestUploadSpriteModel(t *testing.T) {
	spr := &model.MSprite{
		Type:      0,
		MaxWidth:  64,
		MaxHeight: 64,
		NumFrames: 1,
		Frames:    make([]model.MSpriteFrameDesc, 1),
	}

	// Create a test sprite frame
	frame := &model.MSpriteFrame{
		Width:  64,
		Height: 64,
		Up:     32,
		Down:   -32,
		Left:   -32,
		Right:  32,
		SMax:   1.0,
		TMax:   1.0,
	}
	spr.Frames[0] = model.MSpriteFrameDesc{
		Type:     model.SpriteFrameSingle,
		FramePtr: frame,
	}

	glsprite := uploadSpriteModel("test_sprite", spr)
	if glsprite == nil {
		t.Fatalf("uploadSpriteModel returned nil")
	}

	if glsprite.modelID != "test_sprite" {
		t.Errorf("modelID = %q, want %q", glsprite.modelID, "test_sprite")
	}

	if glsprite.maxWidth != 64 || glsprite.maxHeight != 64 {
		t.Errorf("dimensions = %dx%d, want 64x64", glsprite.maxWidth, glsprite.maxHeight)
	}

	if len(glsprite.frames) != 1 {
		t.Errorf("frame count = %d, want 1", len(glsprite.frames))
	}

	frame0 := glsprite.frames[0]
	if frame0.width != 64 || frame0.height != 64 {
		t.Errorf("frame 0 dimensions = %dx%d, want 64x64", frame0.width, frame0.height)
	}

	if frame0.up != 32 || frame0.down != -32 {
		t.Errorf("frame 0 vertical bounds = [%v, %v], want [32, -32]", frame0.up, frame0.down)
	}
}

// TestUploadSpriteModelWithGroup tests sprite model upload with grouped frames.
func TestUploadSpriteModelWithGroup(t *testing.T) {
	spr := &model.MSprite{
		Type:      0,
		MaxWidth:  32,
		MaxHeight: 32,
		NumFrames: 1,
		Frames:    make([]model.MSpriteFrameDesc, 1),
	}

	// Create a sprite group with multiple frames
	frames := make([]*model.MSpriteFrame, 2)
	for i := 0; i < 2; i++ {
		frames[i] = &model.MSpriteFrame{
			Width:  32,
			Height: 32,
			Up:     16,
			Down:   -16,
			Left:   -16,
			Right:  16,
			SMax:   1.0,
			TMax:   1.0,
		}
	}

	group := &model.MSpriteGroup{
		NumFrames: 2,
		Frames:    frames,
		Intervals: []float32{0.1, 0.1},
	}

	spr.Frames[0] = model.MSpriteFrameDesc{
		Type:     model.SpriteFrameGroup,
		FramePtr: group,
	}

	glsprite := uploadSpriteModel("test_group_sprite", spr)
	if glsprite == nil {
		t.Fatalf("uploadSpriteModel returned nil for grouped sprite")
	}

	if len(glsprite.frames) != 2 {
		t.Errorf("frame count = %d, want 2", len(glsprite.frames))
	}

	for i, frame := range glsprite.frames {
		if frame.width != 32 || frame.height != 32 {
			t.Errorf("frame %d dimensions = %dx%d, want 32x32", i, frame.width, frame.height)
		}
	}
}

// TestBuildSpriteQuadVertices tests billboard quad vertex generation.
func TestBuildSpriteQuadVertices(t *testing.T) {
	spr := &glSpriteModel{
		modelID:    "test",
		spriteType: 0,
		maxWidth:   64,
		maxHeight:  64,
		frames: []glSpriteFrame{
			{
				width:  64,
				height: 64,
				up:     32,
				down:   -32,
				left:   -32,
				right:  32,
				sMax:   1.0,
				tMax:   1.0,
			},
		},
	}

	vertices := buildSpriteQuadVertices(spr, 0, [3]float32{0, 0, 1}, 1)
	if vertices == nil {
		t.Fatalf("buildSpriteQuadVertices returned nil")
	}

	if len(vertices) != 4 {
		t.Errorf("vertex count = %d, want 4", len(vertices))
	}

	// Verify quad corners
	if vertices[0].Position[0] != -32 || vertices[0].Position[1] != -32 {
		t.Errorf("quad[0] position = [%.1f, %.1f], want [-32, -32]",
			vertices[0].Position[0], vertices[0].Position[1])
	}

	if vertices[2].Position[0] != 32 || vertices[2].Position[1] != 32 {
		t.Errorf("quad[2] position = [%.1f, %.1f], want [32, 32]",
			vertices[2].Position[0], vertices[2].Position[1])
	}

	// Verify texture coordinates are within expected bounds
	for i, v := range vertices {
		if v.TexCoord[0] < 0 || v.TexCoord[0] > 1.0 {
			t.Errorf("vertex %d u coord = %.2f, want 0-1", i, v.TexCoord[0])
		}
		if v.TexCoord[1] < 0 || v.TexCoord[1] > 1.0 {
			t.Errorf("vertex %d v coord = %.2f, want 0-1", i, v.TexCoord[1])
		}
	}
}

// TestGenerateSpriteQuadIndices tests quad index array generation.
func TestGenerateSpriteQuadIndices(t *testing.T) {
	indices := generateSpriteQuadIndices()
	if indices == nil {
		t.Fatalf("generateSpriteQuadIndices returned nil")
	}

	if len(indices) != 6 {
		t.Errorf("index count = %d, want 6", len(indices))
	}

	// Verify first triangle
	if indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Errorf("triangle 1 = [%d, %d, %d], want [0, 1, 2]",
			indices[0], indices[1], indices[2])
	}

	// Verify second triangle
	if indices[3] != 0 || indices[4] != 2 || indices[5] != 3 {
		t.Errorf("triangle 2 = [%d, %d, %d], want [0, 2, 3]",
			indices[3], indices[4], indices[5])
	}
}

// TestBillboardMatrix tests billboard transformation matrix generation.
func TestBillboardMatrix(t *testing.T) {
	origin := [3]float32{10, 20, 30}
	cameraPos := [3]float32{10, 20, 35} // Camera 5 units in front
	cameraForward := [3]float32{0, 0, -1}

	matrix := billboardMatrix(origin, cameraPos, cameraForward)

	// Verify it's a valid 4x4 matrix (check last row)
	if matrix[15] != 1 {
		t.Errorf("matrix[15] = %.1f, want 1", matrix[15])
	}

	// Verify translation part
	if matrix[12] != origin[0] || matrix[13] != origin[1] || matrix[14] != origin[2] {
		t.Errorf("translation = [%.1f, %.1f, %.1f], want [%.1f, %.1f, %.1f]",
			matrix[12], matrix[13], matrix[14], origin[0], origin[1], origin[2])
	}
}
