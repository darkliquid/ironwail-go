//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
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

func TestBuildSpriteDrawLockedUsesModelSpriteDataWhenEntitySpriteDataNil(t *testing.T) {
	r := &Renderer{
		spriteModels: map[string]*glSpriteModel{},
	}
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
				Up:     4,
				Down:   -4,
				Left:   -4,
				Right:  4,
				SMax:   1,
				TMax:   1,
				Pixels: []byte{5, 6, 7},
			},
		}},
	}
	entity := SpriteEntity{
		ModelID: "progs/flame.spr",
		Model: &model.Model{
			Type:       model.ModSprite,
			SpriteData: modelSprite,
		},
		Frame: 0,
		Alpha: 1,
		Scale: 1,
	}

	draw := r.buildSpriteDrawLocked(entity)
	if draw == nil {
		t.Fatal("buildSpriteDrawLocked returned nil")
	}
	if draw.sprite == nil {
		t.Fatal("buildSpriteDrawLocked should upload sprite from Model.SpriteData")
	}
	if len(draw.sprite.frames) != 1 {
		t.Fatalf("sprite frames = %d, want 1", len(draw.sprite.frames))
	}
	if got := draw.sprite.frames[0].pixels; len(got) != 3 || got[0] != 5 || got[2] != 7 {
		t.Fatalf("sprite frame pixels = %v, want copied Model.SpriteData payload", got)
	}
}

func spriteTestModel(spriteType int) *glSpriteModel {
	return &glSpriteModel{
		modelID:    "test",
		spriteType: spriteType,
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
}

func quadAxes(vertices []spriteQuadVertex) (up, right [3]float32) {
	up = spriteNormalize3([3]float32{
		vertices[1].Position[0] - vertices[0].Position[0],
		vertices[1].Position[1] - vertices[0].Position[1],
		vertices[1].Position[2] - vertices[0].Position[2],
	})
	right = spriteNormalize3([3]float32{
		vertices[2].Position[0] - vertices[1].Position[0],
		vertices[2].Position[1] - vertices[1].Position[1],
		vertices[2].Position[2] - vertices[1].Position[2],
	})
	return up, right
}

func assertVecClose(t *testing.T, got, want [3]float32) {
	t.Helper()
	const epsilon = 1e-5
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > epsilon {
			t.Fatalf("vec[%d]=%v, want %v (got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}

func TestBuildSpriteQuadVerticesVPParallelUpright(t *testing.T) {
	spr := spriteTestModel(spriteTypeVPParallelUpright)
	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{10, 0, 0},
		[3]float32{},
		[3]float32{1, 0, 0},
		[3]float32{0, -1, 0},
		[3]float32{0, 0, 1},
		1,
	)
	if len(vertices) != 4 {
		t.Fatalf("vertex count = %d, want 4", len(vertices))
	}
	up, right := quadAxes(vertices)
	assertVecClose(t, up, [3]float32{0, 0, 1})
	assertVecClose(t, right, [3]float32{0, -1, 0})
}

func TestBuildSpriteQuadVerticesFacingUpright(t *testing.T) {
	spr := spriteTestModel(spriteTypeFacingUpright)
	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{10, 0, 4},
		[3]float32{},
		[3]float32{1, 0, 0},
		[3]float32{0, -1, 0},
		[3]float32{0, 0, 1},
		1,
	)
	up, right := quadAxes(vertices)
	assertVecClose(t, up, [3]float32{0, 0, 1})
	assertVecClose(t, right, [3]float32{0, -1, 0})
}

func TestBuildSpriteQuadVerticesVPParallelUsesCameraBasis(t *testing.T) {
	spr := spriteTestModel(spriteTypeVPParallel)
	cameraForward, cameraRight, cameraUp := spriteCameraBasis([3]float32{20, 35, 15})
	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{0, 0, 0},
		[3]float32{},
		cameraForward, cameraRight, cameraUp,
		1,
	)
	up, right := quadAxes(vertices)
	assertVecClose(t, up, spriteNormalize3(cameraUp))
	assertVecClose(t, right, spriteNormalize3(cameraRight))
}

func TestBuildSpriteQuadVerticesOrientedUsesEntityAngles(t *testing.T) {
	spr := spriteTestModel(spriteTypeOriented)
	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{0, 0, 0},
		[3]float32{0, 90, 0},
		[3]float32{1, 0, 0},
		[3]float32{0, -1, 0},
		[3]float32{0, 0, 1},
		1,
	)
	up, right := quadAxes(vertices)
	assertVecClose(t, up, [3]float32{0, 0, 1})
	assertVecClose(t, right, [3]float32{1, 0, 0})
}

func TestBuildSpriteQuadVerticesVPParallelOrientedAppliesEntityRoll(t *testing.T) {
	spr := spriteTestModel(spriteTypeVPParallelOriented)
	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{0, 0, 0},
		[3]float32{0, 0, 90},
		[3]float32{1, 0, 0},
		[3]float32{0, -1, 0},
		[3]float32{0, 0, 1},
		1,
	)
	up, right := quadAxes(vertices)
	assertVecClose(t, right, [3]float32{0, 0, 1})
	assertVecClose(t, up, [3]float32{0, 1, 0})
}

// TestBuildSpriteQuadVertices tests C corner ordering and UV mapping.
func TestBuildSpriteQuadVertices(t *testing.T) {
	spr := &glSpriteModel{
		modelID:    "test",
		spriteType: spriteTypeVPParallel,
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

	vertices := buildSpriteQuadVertices(
		spr, 0,
		[3]float32{0, 0, 0},
		[3]float32{0, 0, 0},
		[3]float32{},
		[3]float32{1, 0, 0},
		[3]float32{1, 0, 0},
		[3]float32{0, 1, 0},
		1,
	)
	if vertices == nil {
		t.Fatalf("buildSpriteQuadVertices returned nil")
	}

	if len(vertices) != 4 {
		t.Errorf("vertex count = %d, want 4", len(vertices))
	}

	if vertices[0].Position != [3]float32{-32, -32, 0} {
		t.Fatalf("quad[0] position = %v, want [-32 -32 0]", vertices[0].Position)
	}
	if vertices[1].Position != [3]float32{-32, 32, 0} {
		t.Fatalf("quad[1] position = %v, want [-32 32 0]", vertices[1].Position)
	}
	if vertices[2].Position != [3]float32{32, 32, 0} {
		t.Fatalf("quad[2] position = %v, want [32 32 0]", vertices[2].Position)
	}
	if vertices[3].Position != [3]float32{32, -32, 0} {
		t.Fatalf("quad[3] position = %v, want [32 -32 0]", vertices[3].Position)
	}

	wantUVs := [][2]float32{{0, 1}, {0, 0}, {1, 0}, {1, 1}}
	for i, v := range vertices {
		if v.TexCoord != wantUVs[i] {
			t.Fatalf("vertex %d uv = %v, want %v", i, v.TexCoord, wantUVs[i])
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

func TestExpandSpriteQuadVertices(t *testing.T) {
	vertices := []spriteQuadVertex{
		{Position: [3]float32{0, 0, 0}},
		{Position: [3]float32{1, 0, 0}},
		{Position: [3]float32{1, 1, 0}},
		{Position: [3]float32{0, 1, 0}},
	}

	got := expandSpriteQuadVertices(vertices)
	if len(got) != 6 {
		t.Fatalf("expanded vertex count = %d, want 6", len(got))
	}

	want := [][3]float32{
		{0, 0, 0},
		{1, 0, 0},
		{1, 1, 0},
		{0, 0, 0},
		{1, 1, 0},
		{0, 1, 0},
	}
	for i := range want {
		if got[i].Position != want[i] {
			t.Fatalf("expanded vertex %d = %v, want %v", i, got[i].Position, want[i])
		}
	}
}
