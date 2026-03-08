//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

// glSpriteModel holds OpenGL resources for a sprite model.
type glSpriteModel struct {
	modelID    string
	spriteType int
	frames     []glSpriteFrame
	maxWidth   int
	maxHeight  int
	bounds     [3][2]float32 // min/max for each axis
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

// buildSpriteQuadVertices generates sprite quad vertices with C-like orientation rules.
// Corner ordering matches C R_DrawSpriteModel_Real: down-left, up-left, up-right, down-right.
func buildSpriteQuadVertices(sprite *glSpriteModel, frameIndex int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp [3]float32, scale float32) []WorldVertex {
	if sprite == nil || frameIndex < 0 || frameIndex >= len(sprite.frames) {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}

	frame := sprite.frames[frameIndex]
	sUp, sRight := spriteOrientationAxes(sprite.spriteType, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp)

	vertices := make([]WorldVertex, 4)
	sMax := frame.sMax
	tMax := frame.tMax

	downLeft := qtypes.Vec3Add(
		qtypes.Vec3Scale(vec3FromArray(sUp), frame.down*scale),
		qtypes.Vec3Scale(vec3FromArray(sRight), frame.left*scale),
	)
	upLeft := qtypes.Vec3Add(
		downLeft,
		qtypes.Vec3Scale(vec3FromArray(sUp), (frame.up-frame.down)*scale),
	)
	upRight := qtypes.Vec3Add(
		upLeft,
		qtypes.Vec3Scale(vec3FromArray(sRight), (frame.right-frame.left)*scale),
	)
	downRight := qtypes.Vec3Add(
		upRight,
		qtypes.Vec3Scale(vec3FromArray(sUp), (frame.down-frame.up)*scale),
	)

	vertices[0] = WorldVertex{
		Position:      [3]float32{downLeft.X, downLeft.Y, downLeft.Z},
		TexCoord:      [2]float32{0, tMax},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[1] = WorldVertex{
		Position:      [3]float32{upLeft.X, upLeft.Y, upLeft.Z},
		TexCoord:      [2]float32{0, 0},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[2] = WorldVertex{
		Position:      [3]float32{upRight.X, upRight.Y, upRight.Z},
		TexCoord:      [2]float32{sMax, 0},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices[3] = WorldVertex{
		Position:      [3]float32{downRight.X, downRight.Y, downRight.Z},
		TexCoord:      [2]float32{sMax, tMax},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}

	return vertices
}

const (
	spriteTypeVPParallelUpright  = 0
	spriteTypeFacingUpright      = 1
	spriteTypeVPParallel         = 2
	spriteTypeOriented           = 3
	spriteTypeVPParallelOriented = 4
)

func spriteCameraBasis(cameraAngles [3]float32) (forward, right, up [3]float32) {
	f, r, u := qtypes.AngleVectors(qtypes.Vec3{
		X: cameraAngles[0],
		Y: cameraAngles[1],
		Z: cameraAngles[2],
	})
	return [3]float32{f.X, f.Y, f.Z}, [3]float32{r.X, r.Y, r.Z}, [3]float32{u.X, u.Y, u.Z}
}

func spriteOrientationAxes(spriteType int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp [3]float32) (up, right [3]float32) {
	switch spriteType {
	case spriteTypeVPParallelUpright:
		up = [3]float32{0, 0, 1}
		right = spriteNormalize3(spriteCross3(cameraForward, up))
	case spriteTypeFacingUpright:
		toCamera := [3]float32{
			entityOrigin[0] - cameraOrigin[0],
			entityOrigin[1] - cameraOrigin[1],
			0,
		}
		forward := spriteNormalize3(toCamera)
		if spriteVecLen3(forward) == 0 {
			forward = spriteNormalize3([3]float32{cameraForward[0], cameraForward[1], 0})
		}
		right = [3]float32{forward[1], -forward[0], 0}
		up = [3]float32{0, 0, 1}
	case spriteTypeVPParallel:
		up = spriteNormalize3(cameraUp)
		right = spriteNormalize3(cameraRight)
	case spriteTypeOriented:
		_, r, u := qtypes.AngleVectors(qtypes.Vec3{
			X: entityAngles[0],
			Y: entityAngles[1],
			Z: entityAngles[2],
		})
		up = [3]float32{u.X, u.Y, u.Z}
		right = [3]float32{r.X, r.Y, r.Z}
	case spriteTypeVPParallelOriented:
		rollRad := entityAngles[2] * (float32(math.Pi) / 180)
		sr := float32(math.Sin(float64(rollRad)))
		cr := float32(math.Cos(float64(rollRad)))
		right = [3]float32{
			cameraRight[0]*cr + cameraUp[0]*sr,
			cameraRight[1]*cr + cameraUp[1]*sr,
			cameraRight[2]*cr + cameraUp[2]*sr,
		}
		up = [3]float32{
			cameraRight[0]*-sr + cameraUp[0]*cr,
			cameraRight[1]*-sr + cameraUp[1]*cr,
			cameraRight[2]*-sr + cameraUp[2]*cr,
		}
	default:
		up = spriteNormalize3(cameraUp)
		right = spriteNormalize3(cameraRight)
	}

	if spriteVecLen3(right) == 0 {
		right = [3]float32{1, 0, 0}
	}
	if spriteVecLen3(up) == 0 {
		up = [3]float32{0, 0, 1}
	}
	return spriteNormalize3(up), spriteNormalize3(right)
}

func spriteCross3(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func spriteVecLen3(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

func spriteNormalize3(v [3]float32) [3]float32 {
	length := spriteVecLen3(v)
	if length == 0 {
		return v
	}
	return [3]float32{v[0] / length, v[1] / length, v[2] / length}
}

func vec3FromArray(v [3]float32) qtypes.Vec3 {
	return qtypes.Vec3{X: v[0], Y: v[1], Z: v[2]}
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
