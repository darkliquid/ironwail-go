package renderer

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

type spriteRenderModel struct {
	modelID    string
	spriteType int
	frames     []spriteRenderFrame
	maxWidth   int
	maxHeight  int
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
	Position qtypes.Vec3
	TexCoord [2]float32
}

func buildSpriteRenderModel(modelID string, spr *model.MSprite) *spriteRenderModel {
	if modelID == "" || spr == nil || spr.NumFrames == 0 {
		return nil
	}

	frames := make([]spriteRenderFrame, 0, spr.NumFrames)

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
		MaxWidth:  int(mdl.Maxs.X - mdl.Mins.X),
		MaxHeight: int(mdl.Maxs.Z - mdl.Mins.Z),
	}
	if mdl.Maxs.X == 0 && mdl.Maxs.Z == 0 {
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

func buildSpriteQuadVertices(sprite *spriteRenderModel, frameIndex int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp qtypes.Vec3, scale float32) []spriteQuadVertex {
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
		qtypes.Vec3Scale(sUp, frame.down*scale),
		qtypes.Vec3Scale(sRight, frame.left*scale),
	)
	upLeft := qtypes.Vec3Add(
		downLeft,
		qtypes.Vec3Scale(sUp, (frame.up-frame.down)*scale),
	)
	upRight := qtypes.Vec3Add(
		upLeft,
		qtypes.Vec3Scale(sRight, (frame.right-frame.left)*scale),
	)
	downRight := qtypes.Vec3Add(
		upRight,
		qtypes.Vec3Scale(sUp, (frame.down-frame.up)*scale),
	)

	vertices[0] = spriteQuadVertex{Position: downLeft, TexCoord: [2]float32{0, tMax}}
	vertices[1] = spriteQuadVertex{Position: upLeft, TexCoord: [2]float32{0, 0}}
	vertices[2] = spriteQuadVertex{Position: upRight, TexCoord: [2]float32{sMax, 0}}
	vertices[3] = spriteQuadVertex{Position: downRight, TexCoord: [2]float32{sMax, tMax}}

	return vertices
}

func spriteNeedsDepthOffset(spriteType int) bool {
	return spriteType == spriteTypeOriented
}

func spriteUsesOpaqueCutout(spriteType int, alpha float32) bool {
	return spriteNeedsDepthOffset(spriteType) && isFullyOpaqueAlpha(clamp01(alpha))
}

const (
	spriteTypeVPParallelUpright  = 0
	spriteTypeFacingUpright      = 1
	spriteTypeVPParallel         = 2
	spriteTypeOriented           = 3
	spriteTypeVPParallelOriented = 4
)

func spriteCameraBasis(cameraAngles qtypes.Vec3) (forward, right, up qtypes.Vec3) {
	return qtypes.AngleVectors(cameraAngles)
}

func spriteOrientationAxes(spriteType int, cameraOrigin, entityOrigin, entityAngles, cameraForward, cameraRight, cameraUp qtypes.Vec3) (up, right qtypes.Vec3) {
	switch spriteType {
	case spriteTypeVPParallelUpright:
		up = qtypes.Vec3{X: 0, Y: 0, Z: 1}
		right = spriteNormalize(qtypes.Vec3Cross(cameraForward, up))
	case spriteTypeFacingUpright:
		toCamera := qtypes.Vec3{X: entityOrigin.X - cameraOrigin.X, Y: entityOrigin.Y - cameraOrigin.Y, Z: 0}
		forward := spriteNormalize(toCamera)
		if forward.Len() == 0 {
			forward = spriteNormalize(qtypes.Vec3{X: cameraForward.X, Y: cameraForward.Y, Z: 0})
		}
		right = qtypes.Vec3{X: forward.Y, Y: -forward.X, Z: 0}
		up = qtypes.Vec3{X: 0, Y: 0, Z: 1}
	case spriteTypeVPParallel:
		up = spriteNormalize(cameraUp)
		right = spriteNormalize(cameraRight)
	case spriteTypeOriented:
		_, r, u := qtypes.AngleVectors(entityAngles)
		up = u
		right = r
	case spriteTypeVPParallelOriented:
		rollRad := entityAngles.Z * (float32(math.Pi) / 180)
		sr := float32(math.Sin(float64(rollRad)))
		cr := float32(math.Cos(float64(rollRad)))
		right = qtypes.Vec3{
			X: cameraRight.X*cr + cameraUp.X*sr,
			Y: cameraRight.Y*cr + cameraUp.Y*sr,
			Z: cameraRight.Z*cr + cameraUp.Z*sr,
		}
		up = qtypes.Vec3{
			X: cameraRight.X*-sr + cameraUp.X*cr,
			Y: cameraRight.Y*-sr + cameraUp.Y*cr,
			Z: cameraRight.Z*-sr + cameraUp.Z*cr,
		}
	default:
		up = spriteNormalize(cameraUp)
		right = spriteNormalize(cameraRight)
	}

	if right.Len() == 0 {
		right = qtypes.Vec3{X: 1, Y: 0, Z: 0}
	}
	if up.Len() == 0 {
		up = qtypes.Vec3{X: 0, Y: 0, Z: 1}
	}
	return spriteNormalize(up), spriteNormalize(right)
}

func spriteNormalize(v qtypes.Vec3) qtypes.Vec3 {
	length := v.Len()
	if length == 0 {
		return v
	}
	return qtypes.Vec3{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
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
