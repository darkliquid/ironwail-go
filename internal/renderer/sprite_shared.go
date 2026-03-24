package renderer

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

type spriteRenderModel struct {
	modelID    string
	spriteType int
	frames     []spriteRenderFrame
	maxWidth   int
	maxHeight  int
	bounds     [3][2]float32
}

type spriteRenderFrame struct {
	width  int
	height int
	up     float32
	down   float32
	left   float32
	right  float32
	sMax   float32
	tMax   float32
	pixels []byte
}

type spriteQuadVertex struct {
	Position [3]float32
	TexCoord [2]float32
}

func buildSpriteRenderModel(modelID string, spr *model.MSprite) *spriteRenderModel {
	if modelID == "" || spr == nil || spr.NumFrames == 0 {
		return nil
	}

	frames := make([]spriteRenderFrame, 0, spr.NumFrames)
	minBounds := [3]float32{float32(spr.MaxWidth), float32(spr.MaxHeight), 0}
	maxBounds := [3]float32{-float32(spr.MaxWidth), -float32(spr.MaxHeight), 0}

	for _, frameDesc := range spr.Frames {
		switch frameDesc.Type {
		case model.SpriteFrameSingle:
			if frame, ok := frameDesc.FramePtr.(*model.MSpriteFrame); ok {
				frames = append(frames, spriteRenderFrame{
					width:  frame.Width,
					height: frame.Height,
					up:     frame.Up,
					down:   frame.Down,
					left:   frame.Left,
					right:  frame.Right,
					sMax:   frame.SMax,
					tMax:   frame.TMax,
					pixels: append([]byte(nil), frame.Pixels...),
				})
				updateSpriteBounds(minBounds[:], maxBounds[:], frame)
			}
		case model.SpriteFrameGroup, model.SpriteFrameAngled:
			if group, ok := frameDesc.FramePtr.(*model.MSpriteGroup); ok {
				for _, frame := range group.Frames {
					if frame != nil {
						frames = append(frames, spriteRenderFrame{
							width:  frame.Width,
							height: frame.Height,
							up:     frame.Up,
							down:   frame.Down,
							left:   frame.Left,
							right:  frame.Right,
							sMax:   frame.SMax,
							tMax:   frame.TMax,
							pixels: append([]byte(nil), frame.Pixels...),
						})
						updateSpriteBounds(minBounds[:], maxBounds[:], frame)
					}
				}
			}
		}
	}

	if len(frames) == 0 {
		return nil
	}

	return &spriteRenderModel{
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

func spriteDataFromModel(mdl *model.Model) *model.MSprite {
	if mdl == nil || mdl.Type != model.ModSprite {
		return nil
	}
	if mdl.SpriteData != nil {
		return mdl.SpriteData
	}

	spr := &model.MSprite{
		Type:      int(mdl.Type),
		MaxWidth:  int(mdl.Maxs[0] - mdl.Mins[0]),
		MaxHeight: int(mdl.Maxs[2] - mdl.Mins[2]),
	}
	if mdl.Maxs[0] == 0 && mdl.Maxs[2] == 0 {
		spr.MaxWidth = 64
		spr.MaxHeight = 64
	}
	spr.NumFrames = 1
	spr.Frames = make([]model.MSpriteFrameDesc, 1)
	return spr
}

func spriteDataForEntity(entity SpriteEntity) *model.MSprite {
	if entity.SpriteData != nil {
		return entity.SpriteData
	}
	return spriteDataFromModel(entity.Model)
}

func updateSpriteBounds(minBounds, maxBounds []float32, frame *model.MSpriteFrame) {
	if minBounds == nil || maxBounds == nil || len(minBounds) < 3 || len(maxBounds) < 3 {
		return
	}
	if frame.Left < minBounds[0] {
		minBounds[0] = frame.Left
	}
	if frame.Right > maxBounds[0] {
		maxBounds[0] = frame.Right
	}
	if frame.Down < minBounds[1] {
		minBounds[1] = frame.Down
	}
	if frame.Up > maxBounds[1] {
		maxBounds[1] = frame.Up
	}
	minBounds[2] = 0
	maxBounds[2] = 0
}

func buildSpriteQuadVertices(sprite *spriteRenderModel, frameIndex int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp [3]float32, scale float32) []spriteQuadVertex {
	if sprite == nil || frameIndex < 0 || frameIndex >= len(sprite.frames) {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}

	frame := sprite.frames[frameIndex]
	sUp, sRight := spriteOrientationAxes(sprite.spriteType, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp)

	vertices := make([]spriteQuadVertex, 4)
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

	vertices[0] = spriteQuadVertex{Position: [3]float32{downLeft.X, downLeft.Y, downLeft.Z}, TexCoord: [2]float32{0, tMax}}
	vertices[1] = spriteQuadVertex{Position: [3]float32{upLeft.X, upLeft.Y, upLeft.Z}, TexCoord: [2]float32{0, 0}}
	vertices[2] = spriteQuadVertex{Position: [3]float32{upRight.X, upRight.Y, upRight.Z}, TexCoord: [2]float32{sMax, 0}}
	vertices[3] = spriteQuadVertex{Position: [3]float32{downRight.X, downRight.Y, downRight.Z}, TexCoord: [2]float32{sMax, tMax}}

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
	f, r, u := qtypes.AngleVectors(qtypes.Vec3{X: cameraAngles[0], Y: cameraAngles[1], Z: cameraAngles[2]})
	return [3]float32{f.X, f.Y, f.Z}, [3]float32{r.X, r.Y, r.Z}, [3]float32{u.X, u.Y, u.Z}
}

func spriteOrientationAxes(spriteType int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp [3]float32) (up, right [3]float32) {
	switch spriteType {
	case spriteTypeVPParallelUpright:
		up = [3]float32{0, 0, 1}
		right = spriteNormalize3(spriteCross3(cameraForward, up))
	case spriteTypeFacingUpright:
		toCamera := [3]float32{entityOrigin[0] - cameraOrigin[0], entityOrigin[1] - cameraOrigin[1], 0}
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
		_, r, u := qtypes.AngleVectors(qtypes.Vec3{X: entityAngles[0], Y: entityAngles[1], Z: entityAngles[2]})
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

func generateSpriteQuadIndices() []uint32 {
	return []uint32{0, 1, 2, 0, 2, 3}
}

func expandSpriteQuadVertices(vertices []spriteQuadVertex) []spriteQuadVertex {
	if len(vertices) < 4 {
		return nil
	}
	indices := generateSpriteQuadIndices()
	out := make([]spriteQuadVertex, 0, len(indices))
	for _, idx := range indices {
		if int(idx) >= len(vertices) {
			return nil
		}
		out = append(out, vertices[idx])
	}
	return out
}
