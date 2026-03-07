//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"github.com/ironwail/ironwail-go/internal/model"
)

// glSpriteModel holds OpenGL resources for a sprite model.
type glSpriteModel struct {
	modelID   string
	spriteType int
	frames    []glSpriteFrame
	maxWidth  int
	maxHeight int
	bounds    [3][2]float32 // min/max for each axis
}

// glSpriteFrame holds rendering data for a single sprite frame.
type glSpriteFrame struct {
	width  int
	height int
	up     float32
	down   float32
	left   float32
	right  float32
	sMax   float32
	tMax   float32
}

// uploadSpriteModel creates OpenGL resources for a sprite model.
// For sprites, we generate billboard quad geometry at render time, not here.
// This function primarily caches frame data for efficient rendering.
func uploadSpriteModel(modelID string, spr *model.MSprite) *glSpriteModel {
	if modelID == "" || spr == nil || spr.NumFrames == 0 {
		return nil
	}

	frames := make([]glSpriteFrame, 0, spr.NumFrames)
	minBounds := [3]float32{float32(spr.MaxWidth), float32(spr.MaxHeight), 0}
	maxBounds := [3]float32{-float32(spr.MaxWidth), -float32(spr.MaxHeight), 0}

	// Extract frame data from sprite frames (which can be individual or grouped)
	for _, frameDesc := range spr.Frames {
		switch frameDesc.Type {
		case model.SpriteFrameSingle:
			if frame, ok := frameDesc.FramePtr.(*model.MSpriteFrame); ok {
				frames = append(frames, glSpriteFrame{
					width:  frame.Width,
					height: frame.Height,
					up:     frame.Up,
					down:   frame.Down,
					left:   frame.Left,
					right:  frame.Right,
					sMax:   frame.SMax,
					tMax:   frame.TMax,
				})
				updateBounds(minBounds[:], maxBounds[:], frame)
			}
		case model.SpriteFrameGroup, model.SpriteFrameAngled:
			if group, ok := frameDesc.FramePtr.(*model.MSpriteGroup); ok {
				for _, frame := range group.Frames {
					if frame != nil {
						frames = append(frames, glSpriteFrame{
							width:  frame.Width,
							height: frame.Height,
							up:     frame.Up,
							down:   frame.Down,
							left:   frame.Left,
							right:  frame.Right,
							sMax:   frame.SMax,
							tMax:   frame.TMax,
						})
						updateBounds(minBounds[:], maxBounds[:], frame)
					}
				}
			}
		}
	}

	if len(frames) == 0 {
		return nil
	}

	return &glSpriteModel{
		modelID:    modelID,
		spriteType: spr.Type,
		frames:     frames,
		maxWidth:   spr.MaxWidth,
		maxHeight:  spr.MaxHeight,
		bounds: [3][2]float32{
			{minBounds[0], maxBounds[0]},
			{minBounds[1], maxBounds[1]},
			{minBounds[2], maxBounds[2]},
		},
	}
}

// updateBounds updates min/max bounds based on frame dimensions.
func updateBounds(minBounds, maxBounds []float32, frame *model.MSpriteFrame) {
	if minBounds == nil || maxBounds == nil || len(minBounds) < 3 || len(maxBounds) < 3 {
		return
	}
	// Update X (left/right)
	if frame.Left < minBounds[0] {
		minBounds[0] = frame.Left
	}
	if frame.Right > maxBounds[0] {
		maxBounds[0] = frame.Right
	}
	// Update Y (up/down)
	if frame.Down < minBounds[1] {
		minBounds[1] = frame.Down
	}
	if frame.Up > maxBounds[1] {
		maxBounds[1] = frame.Up
	}
	// Z is typically 0 for billboards
	minBounds[2] = 0
	maxBounds[2] = 0
}

// buildSpriteQuadVertices generates billboard quad vertices for a sprite.
// The quad is centered on the sprite origin and faces the camera.
func buildSpriteQuadVertices(sprite *glSpriteModel, frameIndex int, cameraForward [3]float32) []WorldVertex {
	if sprite == nil || frameIndex < 0 || frameIndex >= len(sprite.frames) {
		return nil
	}

	frame := sprite.frames[frameIndex]

	// For billboards, we need to build a quad facing the camera
	// We use the camera forward vector to determine orientation
	// The quad spans from left to right horizontally, and from down to up vertically

	// Normalize camera forward for billboard orientation
	// We'll use a simplified approach: create a quad in world space
	// The actual billboard rotation is done during rendering

	vertices := make([]WorldVertex, 4)

	// Billboard quad corners (before rotation)
	// The texture coordinates map the frame properly
	sMax := frame.sMax
	tMax := frame.tMax

	vertices[0] = WorldVertex{
		Position:      [3]float32{frame.left, frame.down, 0},
		TexCoord:      [2]float32{0, tMax},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[1] = WorldVertex{
		Position:      [3]float32{frame.right, frame.down, 0},
		TexCoord:      [2]float32{sMax, tMax},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[2] = WorldVertex{
		Position:      [3]float32{frame.right, frame.up, 0},
		TexCoord:      [2]float32{sMax, 0},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[3] = WorldVertex{
		Position:      [3]float32{frame.left, frame.up, 0},
		TexCoord:      [2]float32{0, 0},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}

	return vertices
}

// generateSpriteQuadIndices returns the index array for a billboard quad.
func generateSpriteQuadIndices() []uint32 {
	return []uint32{
		0, 1, 2,
		0, 2, 3,
	}
}

// billboardMatrix creates a transformation matrix that rotates a quad to face the camera.
// The sprite is positioned at origin and the camera is looking in the direction of cameraForward.
func billboardMatrix(origin [3]float32, cameraPos [3]float32, cameraForward [3]float32) [16]float32 {
	// Vector from sprite to camera
	toCamera := [3]float32{
		cameraPos[0] - origin[0],
		cameraPos[1] - origin[1],
		cameraPos[2] - origin[2],
	}

	// Normalize it
	dist := float32(0)
	dist += toCamera[0] * toCamera[0]
	dist += toCamera[1] * toCamera[1]
	dist += toCamera[2] * toCamera[2]
	if dist > 0 {
		dist = float32(1.0 / float32(len(toCamera)))
		toCamera[0] *= dist
		toCamera[1] *= dist
		toCamera[2] *= dist
	}

	// For now, use identity with translation
	// A more sophisticated approach would compute proper rotations
	// but for basic sprite rendering, we can apply rotations at render time
	// using the sprite's world position
	matrix := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		origin[0], origin[1], origin[2], 1,
	}
	return matrix
}

// In a future optimization, we could cache VAO/VBO for quads,
// but for now we build them per-frame since sprites have different sizes
