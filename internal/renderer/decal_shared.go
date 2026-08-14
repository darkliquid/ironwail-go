package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/renderer/decal"
	"github.com/darkliquid/ironwail-go/pkg/types"
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
	prepared := decal.PrepareDraws(entities, camera.Origin)
	out := make([]decalDraw, 0, len(prepared))
	for _, draw := range prepared {
		out = append(out, decalDraw{mark: draw.Mark.(DecalMarkEntity), distanceSq: draw.DistanceSq})
	}
	return out
}

func buildDecalQuad(mark DecalMarkEntity) ([4]types.Vec3, bool) {
	return decal.BuildQuad(mark)
}
