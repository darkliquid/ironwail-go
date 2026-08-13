package renderer

import (
	"log/slog"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

// WorldRuntime is the renderer-root contract that shared callers and tests use
// while backend-specific world code is being split into true subpackages.
type WorldRuntime interface {
	UploadWorld(tree *bsp.Tree) error
	ClearWorld()
	HasWorldData() bool
	WorldBounds() (min [3]float32, max [3]float32, ok bool)
	SetExternalSkybox(name string, loadFile func(string) ([]byte, error))
}

var _ WorldRuntime = (*Renderer)(nil)

// worldFogUniformDensity converts the fog density cvar value to the uniform value used in the shader's exponential fog formula.
func worldFogUniformDensity(density float32) float32 {
	return worldimpl.FogUniformDensity(density)
}

// blendFogStateTowards blends the previous fog state toward the target fog state by at most maxStep.
// It provides a deterministic one-step transition seam so abrupt fog changes can avoid hard pops.
func blendFogStateTowards(prevColor [3]float32, prevDensity float32, nextColor [3]float32, nextDensity float32, maxStep float32) ([3]float32, float32) {
	return worldimpl.BlendFogStateTowards(prevColor, prevDensity, nextColor, nextDensity, maxStep)
}

type worldRenderPass = worldimpl.RenderPass

const (
	worldPassSky         = worldimpl.PassSky
	worldPassOpaque      = worldimpl.PassOpaque
	worldPassAlphaTest   = worldimpl.PassAlphaTest
	worldPassTranslucent = worldimpl.PassTranslucent
)

func worldFaceAlpha(flags int32, liquidAlpha worldLiquidAlphaSettings) float32 {
	return worldimpl.FaceAlpha(flags, liquidAlpha.toWorld())
}



func worldFaceIsLiquid(flags int32) bool {
	return worldimpl.FaceIsLiquid(flags)
}

func worldFacePass(flags int32, alpha float32) worldRenderPass {
	return worldimpl.FacePass(flags, alpha)
}

func worldFaceDistanceSq(center [3]float32, camera CameraState) float32 {
	return worldimpl.FaceDistanceSq(center, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z})
}

func buildWorldLeafFaceLookup(tree *bsp.Tree, faceLookup map[int]int) [][]int {
	return worldimpl.BuildLeafFaceLookup(tree, faceLookup)
}

type worldVisibilityScratch struct {
	marks []uint32
	stamp uint32
	faces []WorldFace
}

func (s *worldVisibilityScratch) nextStamp(faceCount int) uint32 {
	if faceCount > len(s.marks) {
		s.marks = make([]uint32, faceCount)
	}
	s.stamp++
	if s.stamp == 0 {
		clear(s.marks)
		s.stamp = 1
	}
	return s.stamp
}

func (s *worldVisibilityScratch) selectVisibleWorldFaces(tree *bsp.Tree, allFaces []WorldFace, leafFaces [][]int, cameraOrigin [3]float32) []WorldFace {
	if len(allFaces) == 0 {
		s.faces = s.faces[:0]
		return nil
	}
	if tree == nil || len(tree.Leafs) <= 1 || len(leafFaces) == 0 {
		s.faces = s.faces[:0]
		return allFaces
	}

	cameraLeaf := tree.PointInLeaf(cameraOrigin)
	if cameraLeaf == nil {
		s.faces = s.faces[:0]
		return allFaces
	}
	cameraLeafIndex := -1
	for i := range tree.Leafs {
		if &tree.Leafs[i] == cameraLeaf {
			cameraLeafIndex = i
			break
		}
	}
	nearWaterPortal := false
	if cameraLeafIndex >= 0 {
		if cameraLeafIndex < len(leafFaces) {
			for _, faceIdx := range leafFaces[cameraLeafIndex] {
				if faceIdx >= 0 && faceIdx < len(allFaces) {
					if allFaces[faceIdx].Flags&model.SurfDrawTurb != 0 {
						nearWaterPortal = true
						break
					}
				}
			}
		}
		if !nearWaterPortal {
			fatPVS := tree.FatPVS(cameraOrigin)
			for leafIdx := range tree.Leafs {
				byteIdx := leafIdx / 8
				bitIdx := uint(leafIdx % 8)
				if byteIdx < len(fatPVS) && (fatPVS[byteIdx]&(1<<bitIdx)) != 0 {
					if leafIdx < len(leafFaces) {
						for _, faceIdx := range leafFaces[leafIdx] {
							if faceIdx >= 0 && faceIdx < len(allFaces) {
								if allFaces[faceIdx].Flags&model.SurfDrawTurb != 0 {
									nearWaterPortal = true
									break
								}
							}
						}
					}
				}
				if nearWaterPortal {
					break
				}
			}
		}
	}

	var pvs []byte
	if nearWaterPortal {
		pvs = tree.FatPVS(cameraOrigin)
	} else {
		pvs = tree.LeafPVS(cameraLeaf)
	}

	if rDebugWaterEnabled() {
		slog.Debug("[rwater] PVS selection",
			"near_water_portal", nearWaterPortal,
			"camera_leaf_index", cameraLeafIndex,
			"pvs_len", len(pvs),
		)
	}
	if len(pvs) == 0 {
		s.faces = s.faces[:0]
		return allFaces
	}

	stamp := s.nextStamp(len(allFaces))
	visibleMarks := s.marks[:len(allFaces)]
	visibleCount := 0
	for leafIndex := 1; leafIndex < len(tree.Leafs) && leafIndex < len(leafFaces); leafIndex++ {
		if leafIndex != cameraLeafIndex && !leafVisibleInMask(pvs, leafIndex-1) {
			continue
		}
		for _, faceIndex := range leafFaces[leafIndex] {
			if faceIndex < 0 || faceIndex >= len(allFaces) || visibleMarks[faceIndex] == stamp {
				continue
			}
			visibleMarks[faceIndex] = stamp
			visibleCount++
		}
	}

	if visibleCount == 0 {
		s.faces = s.faces[:0]
		return allFaces
	}

	faces := s.faces[:0]
	for faceIndex, mark := range visibleMarks {
		if mark == stamp {
			faces = append(faces, allFaces[faceIndex])
		}
	}
	s.faces = faces
	return faces
}

func selectVisibleWorldFaces(tree *bsp.Tree, allFaces []WorldFace, leafFaces [][]int, cameraOrigin [3]float32) []WorldFace {
	var scratch worldVisibilityScratch
	return scratch.selectVisibleWorldFaces(tree, allFaces, leafFaces, cameraOrigin)
}

func leafVisibleInMask(mask []byte, leafBit int) bool {
	return worldimpl.LeafVisibleInMask(mask, leafBit)
}

// worldTextureMeta holds parsed texture metadata (name, dimensions, classified type)
// from the BSP miptex lump entries.
type worldTextureMeta struct {
	Width  int
	Height int
	Name   string
	Type   model.TextureType
}

// parseWorldTextureMeta parses the BSP miptex lump to extract texture names and dimensions.
func parseWorldTextureMeta(tree *bsp.Tree) []worldTextureMeta {
	parsed := worldimpl.ParseTextureMeta(tree)
	out := make([]worldTextureMeta, len(parsed))
	for i, t := range parsed {
		out[i] = worldTextureMeta{Width: t.Width, Height: t.Height, Name: t.Name, Type: t.Type}
	}
	return out
}

// classifyWorldTextureName classifies a texture by its name prefix convention.
func classifyWorldTextureName(name string) model.TextureType {
	return worldimpl.ClassifyTextureName(name)
}

// deriveWorldFaceFlags converts texture type and texinfo flags into surface rendering flags.
func deriveWorldFaceFlags(textureType model.TextureType, texinfoFlags int32) int32 {
	return worldimpl.DeriveFaceFlags(textureType, texinfoFlags)
}



// worldLiquidAlphaSettings stores per-liquid-type alpha values read from console
// variables (r_wateralpha, r_lavaalpha, r_slimealpha, r_telealpha). These control
// the transparency of water, lava, slime, and teleporter surfaces during the world
// liquid render passes, allowing mappers and players to configure liquid visibility.
type worldLiquidAlphaSettings struct {
	water float32
	lava  float32
	slime float32
	tele  float32
}

type worldLiquidAlphaOverrides struct {
	hasWater bool
	water    float32
	hasLava  bool
	lava     float32
	hasSlime bool
	slime    float32
	hasTele  bool
	tele     float32
}

func (s worldLiquidAlphaSettings) toWorld() worldimpl.LiquidAlphaSettings {
	return worldimpl.LiquidAlphaSettings{
		Water: s.water,
		Lava:  s.lava,
		Slime: s.slime,
		Tele:  s.tele,
	}
}

func worldLiquidAlphaSettingsFromWorld(s worldimpl.LiquidAlphaSettings) worldLiquidAlphaSettings {
	return worldLiquidAlphaSettings{
		water: s.Water,
		lava:  s.Lava,
		slime: s.Slime,
		tele:  s.Tele,
	}
}

func (o worldLiquidAlphaOverrides) toWorld() worldimpl.LiquidAlphaOverrides {
	return worldimpl.LiquidAlphaOverrides{
		HasWater: o.hasWater,
		Water:    o.water,
		HasLava:  o.hasLava,
		Lava:     o.lava,
		HasSlime: o.hasSlime,
		Slime:    o.slime,
		HasTele:  o.hasTele,
		Tele:     o.tele,
	}
}

func worldLiquidAlphaOverridesFromWorld(o worldimpl.LiquidAlphaOverrides) worldLiquidAlphaOverrides {
	return worldLiquidAlphaOverrides{
		hasWater: o.HasWater,
		water:    o.Water,
		hasLava:  o.HasLava,
		lava:     o.Lava,
		hasSlime: o.HasSlime,
		slime:    o.Slime,
		hasTele:  o.HasTele,
		tele:     o.Tele,
	}
}

func worldLiquidAlphaSettingsFromCvars(overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	return worldLiquidAlphaSettingsFromWorld(worldimpl.ReadLiquidAlphaSettings(overrides.toWorld(), tree))
}

func resolveWorldLiquidAlphaSettings(cvarWater, cvarLava, cvarSlime, cvarTele float32, overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	return worldLiquidAlphaSettingsFromWorld(worldimpl.ResolveLiquidAlphaSettings(cvarWater, cvarLava, cvarSlime, cvarTele, overrides.toWorld(), tree))
}

type worldSkyFogOverride struct {
	hasValue bool
	value    float32
}

// parseWorldspawnSkyFogOverride parses the worldspawn entity for sky fog override values.
func parseWorldspawnSkyFogOverride(entities []byte) worldSkyFogOverride {
	if len(entities) == 0 {
		return worldSkyFogOverride{}
	}

	entity, ok := worldimpl.FirstEntityLumpObject(string(entities))
	if !ok {
		return worldSkyFogOverride{}
	}

	fields := worldimpl.ParseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return worldSkyFogOverride{}
	}

	value, ok := worldimpl.ParseEntityAlphaField(fields, "skyfog")
	if !ok {
		return worldSkyFogOverride{}
	}

	return worldSkyFogOverride{hasValue: true, value: value}
}

// readWorldSkyFogCvar reads the r_skyfog cvar value with a fallback default.
func readWorldSkyFogCvar(fallback float32) float32 {
	return worldimpl.ReadAlphaCvar(CvarRSkyFog, fallback)
}

func proceduralSkyGradientColors() (horizon, zenith [3]float32) {
	return worldimpl.ProceduralSkyGradientColors()
}

func shouldUseProceduralSky(fastSky, proceduralSkyEnabled bool, externalSkyMode externalSkyboxRenderMode) bool {
	return worldimpl.ShouldUseProceduralSky(fastSky, proceduralSkyEnabled, externalSkyMode == externalSkyboxRenderEmbedded)
}

// resolveWorldSkyFogMix resolves the final sky fog mix factor from the cvar value, worldspawn override, and fog density.
func resolveWorldSkyFogMix(cvarValue float32, override worldSkyFogOverride, fogDensity float32) float32 {
	return worldimpl.ResolveSkyFogMix(cvarValue, override.hasValue, override.value, fogDensity)
}

func gogpuWorldSkyFogDensity(worldEntities []byte, fogDensity float32) float32 {
	return resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), parseWorldspawnSkyFogOverride(worldEntities), fogDensity)
}

func resolveWorldSkyTextureIndex(face WorldFace, textureAnimations []*surfacepkg.SurfaceTexture, frame int, timeSeconds float64) int32 {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := surfacepkg.TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}
	return textureIndex
}
