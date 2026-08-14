package world

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

type RenderPass int

const (
	PassSky RenderPass = iota
	PassOpaque
	PassAlphaTest
	PassTranslucent
)

func FaceAlpha(flags int32, liquidAlpha LiquidAlphaSettings) float32 {
	if flags&model.SurfDrawTurb == 0 {
		return 1
	}
	switch {
	case flags&model.SurfDrawLava != 0:
		return liquidAlpha.Lava
	case flags&model.SurfDrawSlime != 0:
		return liquidAlpha.Slime
	case flags&model.SurfDrawTele != 0:
		return liquidAlpha.Tele
	default:
		return liquidAlpha.Water
	}
}

func FaceUsesTurb(flags int32) bool {
	return flags&model.SurfDrawTurb != 0 && flags&model.SurfDrawSky == 0
}

func FaceIsLiquid(flags int32) bool {
	return flags&(model.SurfDrawLava|model.SurfDrawSlime|model.SurfDrawTele|model.SurfDrawWater) != 0
}

func FacePass(flags int32, alpha float32) RenderPass {
	switch {
	case flags&model.SurfDrawSky != 0:
		return PassSky
	case flags&model.SurfDrawFence != 0:
		return PassAlphaTest
	case alpha < 1:
		return PassTranslucent
	default:
		return PassOpaque
	}
}

func FaceDistanceSq(center types.Vec3, cameraOrigin types.Vec3) float32 {
	dx := center.X - cameraOrigin.X
	dy := center.Y - cameraOrigin.Y
	dz := center.Z - cameraOrigin.Z
	return dx*dx + dy*dy + dz*dz
}

func BuildLeafFaceLookup(tree *bsp.Tree, faceLookup map[int]int) [][]int {
	if tree == nil || len(tree.Leafs) == 0 || len(tree.MarkSurfaces) == 0 || len(faceLookup) == 0 {
		return nil
	}
	leafFaces := make([][]int, len(tree.Leafs))
	for leafIndex, leaf := range tree.Leafs {
		start := int(leaf.FirstMarkSurface)
		count := int(leaf.NumMarkSurfaces)
		if start < 0 || count <= 0 || start >= len(tree.MarkSurfaces) {
			continue
		}
		end := minInt(start+count, len(tree.MarkSurfaces))
		seen := make(map[int]struct{}, end-start)
		for i := start; i < end; i++ {
			faceIndex := tree.MarkSurfaces[i]
			builtFaceIndex, ok := faceLookup[faceIndex]
			if !ok {
				continue
			}
			if _, exists := seen[builtFaceIndex]; exists {
				continue
			}
			leafFaces[leafIndex] = append(leafFaces[leafIndex], builtFaceIndex)
			seen[builtFaceIndex] = struct{}{}
		}
	}
	return leafFaces
}

func LeafVisibleInMask(mask []byte, leafBit int) bool {
	if leafBit < 0 {
		return false
	}
	byteIndex := leafBit >> 3
	if byteIndex >= len(mask) {
		return false
	}
	return mask[byteIndex]&(1<<uint(leafBit&7)) != 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
