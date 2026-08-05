package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/renderer/decal"
)

// decalDraw is a mark paired with its squared distance to the camera, used by
// the HAL draw prep to sort far-to-near.
type decalDraw struct {
	mark       DecalMarkEntity
	distanceSq float32
}

func generateDecalAtlasData() []byte {
	return decal.AtlasData()
}

func prepareDecalDraws(marks []DecalMarkEntity, camera CameraState) []decalDraw {
	entities := make([]decal.MarkEntity, 0, len(marks))
	for i := range marks {
		entities = append(entities, marks[i])
	}
	prepared := decal.PrepareDraws(entities, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z})
	out := make([]decalDraw, 0, len(prepared))
	for _, draw := range prepared {
		out = append(out, decalDraw{mark: draw.Mark.(DecalMarkEntity), distanceSq: draw.DistanceSq})
	}
	return out
}

func buildDecalQuad(mark DecalMarkEntity) ([4][3]float32, bool) {
	return decal.BuildQuad(mark)
}

func buildDecalBasis(normal [3]float32, rotation float32) (tangent [3]float32, bitangent [3]float32) {
	return decal.BuildBasis(normal, rotation)
}

func decalNormalize3(v [3]float32) ([3]float32, bool) {
	return decal.Normalize3(v)
}
