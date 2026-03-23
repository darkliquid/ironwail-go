package renderer

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

type worldRenderPass int

const (
	worldPassSky worldRenderPass = iota
	worldPassOpaque
	worldPassAlphaTest
	worldPassTranslucent
)

func worldFaceAlpha(flags int32, liquidAlpha worldLiquidAlphaSettings) float32 {
	if flags&model.SurfDrawTurb == 0 {
		return 1
	}
	if flags&model.SurfDrawLava != 0 {
		return liquidAlpha.lava
	}
	if flags&model.SurfDrawSlime != 0 {
		return liquidAlpha.slime
	}
	if flags&model.SurfDrawTele != 0 {
		return liquidAlpha.tele
	}
	if flags&model.SurfDrawWater != 0 {
		return liquidAlpha.water
	}
	return 1
}

func worldFaceUsesTurb(flags int32) bool {
	return flags&model.SurfDrawTurb != 0 && flags&model.SurfDrawSky == 0
}

func worldFaceIsLiquid(flags int32) bool {
	return flags&(model.SurfDrawLava|model.SurfDrawSlime|model.SurfDrawTele|model.SurfDrawWater) != 0
}

func worldFacePass(flags int32, alpha float32) worldRenderPass {
	switch {
	case flags&model.SurfDrawSky != 0:
		return worldPassSky
	case flags&model.SurfDrawFence != 0:
		return worldPassAlphaTest
	case alpha < 1:
		return worldPassTranslucent
	default:
		return worldPassOpaque
	}
}

func worldFaceDistanceSq(center [3]float32, camera CameraState) float32 {
	dx := center[0] - camera.Origin.X
	dy := center[1] - camera.Origin.Y
	dz := center[2] - camera.Origin.Z
	return dx*dx + dy*dy + dz*dz
}

func buildWorldLeafFaceLookup(tree *bsp.Tree, faceLookup map[int]int) [][]int {
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
		end := min(start+count, len(tree.MarkSurfaces))
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

func selectVisibleWorldFaces(tree *bsp.Tree, allFaces []WorldFace, leafFaces [][]int, cameraOrigin [3]float32) []WorldFace {
	if len(allFaces) == 0 {
		return nil
	}
	if tree == nil || len(tree.Leafs) <= 1 || len(leafFaces) == 0 {
		return allFaces
	}

	cameraLeaf := tree.PointInLeaf(cameraOrigin)
	if cameraLeaf == nil {
		return allFaces
	}
	cameraLeafIndex := -1
	for i := range tree.Leafs {
		if &tree.Leafs[i] == cameraLeaf {
			cameraLeafIndex = i
			break
		}
	}
	pvs := tree.LeafPVS(cameraLeaf)
	if len(pvs) == 0 {
		return allFaces
	}

	visible := make([]bool, len(allFaces))
	visibleCount := 0
	for leafIndex := 1; leafIndex < len(tree.Leafs) && leafIndex < len(leafFaces); leafIndex++ {
		if leafIndex != cameraLeafIndex && !leafVisibleInMask(pvs, leafIndex-1) {
			continue
		}
		for _, faceIndex := range leafFaces[leafIndex] {
			if faceIndex < 0 || faceIndex >= len(allFaces) || visible[faceIndex] {
				continue
			}
			visible[faceIndex] = true
			visibleCount++
		}
	}

	if visibleCount == 0 {
		return allFaces
	}
	faces := make([]WorldFace, 0, visibleCount)
	for faceIndex, ok := range visible {
		if ok {
			faces = append(faces, allFaces[faceIndex])
		}
	}
	return faces
}

func leafVisibleInMask(mask []byte, leafBit int) bool {
	if leafBit < 0 {
		return false
	}
	byteIndex := leafBit >> 3
	if byteIndex >= len(mask) {
		return false
	}
	return mask[byteIndex]&(1<<uint(leafBit&7)) != 0
}
