package renderer_test

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// syntheticTransparencyWorld contains procedural geometry and model data
// simulating a four-quadrant transparency test scene:
// 1. Ground floor with opaque texture and lightmap (z = 0)
// 2. Recessed water pool (SURF_DRAWTURB) with wateralpha = 0.6 (z = -16 to -64)
// 3. Recessed slime / lava / tele pools with varying alphas
// 4. Semi-transparent glass brush entity with alpha = 0.5 (z = 16)
// 5. Submerged prop (opaque brush) placed inside the water pool (z = -48)
type syntheticTransparencyWorld struct {
	Model    *model.Model
	Tree     *bsp.Tree
	Geometry *worldimpl.WorldGeometry
}

func buildSyntheticTransparencyWorld() *syntheticTransparencyWorld {
	m := &model.Model{
		Type: model.ModBrush,
		Name: "maps/synthetic_transparency.bsp",
		Mins: types.Vec3{X: -256, Y: -256, Z: -128},
		Maxs: types.Vec3{X: 256, Y: 256, Z: 256},
	}

	// Textures: ground, water, slime, lava, tele, fence, glass
	texGround := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeDefault}
	copy(texGround.Name[:], "ground")

	texWater := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeWater}
	copy(texWater.Name[:], "*water")

	texSlime := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeSlime}
	copy(texSlime.Name[:], "*slime")

	texLava := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeLava}
	copy(texLava.Name[:], "*lava")

	texTele := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeTele}
	copy(texTele.Name[:], "*tele")

	texFence := &model.Texture{Width: 64, Height: 64, Type: model.TexTypeCutout}
	copy(texFence.Name[:], "{fence")

	m.Textures = []*model.Texture{texGround, texWater, texSlime, texLava, texTele, texFence}
	m.NumTextures = len(m.Textures)

	// Planes
	planeZ0 := model.MPlane{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0, Type: 2}
	planeZNeg16 := model.MPlane{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -16, Type: 2}
	planeZNeg64 := model.MPlane{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: 2}
	planeVert := model.MPlane{Normal: types.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0}
	m.Planes = []model.MPlane{planeZ0, planeZNeg16, planeZNeg64, planeVert}
	m.NumPlanes = len(m.Planes)

	// TexInfos
	texInfoGround := model.MTexInfo{TexNum: 0, Flags: 0}
	texInfoWater := model.MTexInfo{TexNum: 1, Flags: model.SurfDrawTurb | model.SurfDrawWater}
	texInfoSlime := model.MTexInfo{TexNum: 2, Flags: model.SurfDrawTurb | model.SurfDrawSlime}
	texInfoLava := model.MTexInfo{TexNum: 3, Flags: model.SurfDrawTurb | model.SurfDrawLava}
	texInfoTele := model.MTexInfo{TexNum: 4, Flags: model.SurfDrawTurb | model.SurfDrawTele}
	texInfoFence := model.MTexInfo{TexNum: 5, Flags: model.SurfDrawFence}
	m.TexInfo = []model.MTexInfo{texInfoGround, texInfoWater, texInfoSlime, texInfoLava, texInfoTele, texInfoFence}
	m.NumTexInfo = len(m.TexInfo)

	// Surfaces:
	// 0: Opaque ground floor (z = 0)
	// 1: Recessed pool floor (z = -64, opaque)
	// 2: Recessed water surface (z = -16, turb water)
	// 3: Recessed slime surface (z = -16, turb slime)
	// 4: Recessed lava surface (z = -16, turb lava)
	// 5: Teleporter surface (turb tele)
	// 6: Fence / grate surface (alpha test fence)
	m.Surfaces = []model.MSurface{
		{
			Plane:        &m.Planes[0],
			Flags:        0,
			TexInfo:      &m.TexInfo[0],
			Mins:         types.Vec3{X: -256, Y: -256, Z: 0},
			Maxs:         types.Vec3{X: 256, Y: 256, Z: 0},
			VBOFirstVert: 0,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[2],
			Flags:        0,
			TexInfo:      &m.TexInfo[0],
			Mins:         types.Vec3{X: -128, Y: -128, Z: -64},
			Maxs:         types.Vec3{X: 0, Y: 0, Z: -64},
			VBOFirstVert: 4,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[1],
			Flags:        model.SurfDrawTurb | model.SurfDrawWater,
			TexInfo:      &m.TexInfo[1],
			Mins:         types.Vec3{X: -128, Y: -128, Z: -16},
			Maxs:         types.Vec3{X: 0, Y: 0, Z: -16},
			VBOFirstVert: 8,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[1],
			Flags:        model.SurfDrawTurb | model.SurfDrawSlime,
			TexInfo:      &m.TexInfo[2],
			Mins:         types.Vec3{X: 0, Y: -128, Z: -16},
			Maxs:         types.Vec3{X: 128, Y: 0, Z: -16},
			VBOFirstVert: 12,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[1],
			Flags:        model.SurfDrawTurb | model.SurfDrawLava,
			TexInfo:      &m.TexInfo[3],
			Mins:         types.Vec3{X: -128, Y: 0, Z: -16},
			Maxs:         types.Vec3{X: 0, Y: 128, Z: -16},
			VBOFirstVert: 16,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[1],
			Flags:        model.SurfDrawTurb | model.SurfDrawTele,
			TexInfo:      &m.TexInfo[4],
			Mins:         types.Vec3{X: 0, Y: 0, Z: -16},
			Maxs:         types.Vec3{X: 128, Y: 128, Z: -16},
			VBOFirstVert: 20,
			NumEdges:     4,
		},
		{
			Plane:        &m.Planes[3],
			Flags:        model.SurfDrawFence,
			TexInfo:      &m.TexInfo[5],
			Mins:         types.Vec3{X: 0, Y: -128, Z: 0},
			Maxs:         types.Vec3{X: 0, Y: 128, Z: 64},
			VBOFirstVert: 24,
			NumEdges:     4,
		},
	}
	m.NumSurfaces = len(m.Surfaces)

	// Build corresponding worldimpl.WorldGeometry representation
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{
			// Ground (opaque)
			{Position: types.Vec3{X: -256, Y: -256, Z: 0}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 256, Y: -256, Z: 0}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 256, Y: 256, Z: 0}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: -256, Y: 256, Z: 0}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Pool floor (opaque)
			{Position: types.Vec3{X: -128, Y: -128, Z: -64}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: -128, Z: -64}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 0, Z: -64}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: -128, Y: 0, Z: -64}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Water (translucent liquid)
			{Position: types.Vec3{X: -128, Y: -128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: -128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: -128, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Slime
			{Position: types.Vec3{X: 0, Y: -128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 128, Y: -128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 128, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Lava
			{Position: types.Vec3{X: -128, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: -128, Y: 128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Tele
			{Position: types.Vec3{X: 0, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 128, Y: 0, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 128, Y: 128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			{Position: types.Vec3{X: 0, Y: 128, Z: -16}, Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
			// Fence
			{Position: types.Vec3{X: 0, Y: -128, Z: 0}, Normal: types.Vec3{X: 1, Y: 0, Z: 0}},
			{Position: types.Vec3{X: 0, Y: 128, Z: 0}, Normal: types.Vec3{X: 1, Y: 0, Z: 0}},
			{Position: types.Vec3{X: 0, Y: 128, Z: 64}, Normal: types.Vec3{X: 1, Y: 0, Z: 0}},
			{Position: types.Vec3{X: 0, Y: -128, Z: 64}, Normal: types.Vec3{X: 1, Y: 0, Z: 0}},
		},
		Indices: []uint32{
			0, 1, 2, 0, 2, 3, // Ground
			4, 5, 6, 4, 6, 7, // Pool floor
			8, 9, 10, 8, 10, 11, // Water
			12, 13, 14, 12, 14, 15, // Slime
			16, 17, 18, 16, 18, 19, // Lava
			20, 21, 22, 20, 22, 23, // Tele
			24, 25, 26, 24, 26, 27, // Fence
		},
		Faces: []worldimpl.WorldFace{
			{FirstIndex: 0, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: 0, Center: types.Vec3{X: 0, Y: 0, Z: 0}},
			{FirstIndex: 6, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: 0, Center: types.Vec3{X: -64, Y: -64, Z: -64}},
			{FirstIndex: 12, NumIndices: 6, TextureIndex: 1, LightmapIndex: -1, Flags: model.SurfDrawTurb | model.SurfDrawWater, Center: types.Vec3{X: -64, Y: -64, Z: -16}},
			{FirstIndex: 18, NumIndices: 6, TextureIndex: 2, LightmapIndex: -1, Flags: model.SurfDrawTurb | model.SurfDrawSlime, Center: types.Vec3{X: 64, Y: -64, Z: -16}},
			{FirstIndex: 24, NumIndices: 6, TextureIndex: 3, LightmapIndex: -1, Flags: model.SurfDrawTurb | model.SurfDrawLava, Center: types.Vec3{X: -64, Y: 64, Z: -16}},
			{FirstIndex: 30, NumIndices: 6, TextureIndex: 4, LightmapIndex: -1, Flags: model.SurfDrawTurb | model.SurfDrawTele, Center: types.Vec3{X: 64, Y: 64, Z: -16}},
			{FirstIndex: 36, NumIndices: 6, TextureIndex: 5, LightmapIndex: 0, Flags: model.SurfDrawFence, Center: types.Vec3{X: 0, Y: 0, Z: 32}},
		},
		LiquidFaceTypes: model.SurfDrawWater | model.SurfDrawSlime | model.SurfDrawLava | model.SurfDrawTele,
	}

	return &syntheticTransparencyWorld{
		Model:    m,
		Geometry: geom,
	}
}

func TestSyntheticMultiLiquidTransparencyParity(t *testing.T) {
	world := buildSyntheticTransparencyWorld()
	if world == nil || world.Model == nil || world.Geometry == nil {
		t.Fatal("failed to initialize synthetic transparency world")
	}

	t.Run("WorldModelStructure", func(t *testing.T) {
		m := world.Model
		if m.Type != model.ModBrush {
			t.Fatalf("model.Type = %v, want %v", m.Type, model.ModBrush)
		}
		if len(m.Textures) != 6 {
			t.Fatalf("len(Textures) = %d, want 6", len(m.Textures))
		}
		if len(m.Surfaces) != 7 {
			t.Fatalf("len(Surfaces) = %d, want 7", len(m.Surfaces))
		}
	})

	t.Run("FacePassClassificationUnderLiquidSettings", func(t *testing.T) {
		liquidSettings := worldimpl.LiquidAlphaSettings{
			Water: 0.6,
			Slime: 0.7,
			Lava:  1.0, // Lava opaque by default
			Tele:  0.9,
		}

		faces := world.Geometry.Faces

		// Face 0: Ground (opaque)
		alpha0 := worldimpl.FaceAlpha(faces[0].Flags, liquidSettings)
		if alpha0 != 1.0 {
			t.Errorf("ground alpha = %v, want 1.0", alpha0)
		}
		if pass := worldimpl.FacePass(faces[0].Flags, alpha0); pass != worldimpl.PassOpaque {
			t.Errorf("ground pass = %v, want PassOpaque", pass)
		}

		// Face 1: Pool floor (opaque)
		alpha1 := worldimpl.FaceAlpha(faces[1].Flags, liquidSettings)
		if alpha1 != 1.0 {
			t.Errorf("pool floor alpha = %v, want 1.0", alpha1)
		}
		if pass := worldimpl.FacePass(faces[1].Flags, alpha1); pass != worldimpl.PassOpaque {
			t.Errorf("pool floor pass = %v, want PassOpaque", pass)
		}

		// Face 2: Water (wateralpha = 0.6 -> translucent)
		alpha2 := worldimpl.FaceAlpha(faces[2].Flags, liquidSettings)
		if alpha2 != 0.6 {
			t.Errorf("water alpha = %v, want 0.6", alpha2)
		}
		if pass := worldimpl.FacePass(faces[2].Flags, alpha2); pass != worldimpl.PassTranslucent {
			t.Errorf("water pass = %v, want PassTranslucent", pass)
		}

		// Face 3: Slime (slimealpha = 0.7 -> translucent)
		alpha3 := worldimpl.FaceAlpha(faces[3].Flags, liquidSettings)
		if alpha3 != 0.7 {
			t.Errorf("slime alpha = %v, want 0.7", alpha3)
		}
		if pass := worldimpl.FacePass(faces[3].Flags, alpha3); pass != worldimpl.PassTranslucent {
			t.Errorf("slime pass = %v, want PassTranslucent", pass)
		}

		// Face 4: Lava (lavaalpha = 1.0 -> opaque)
		alpha4 := worldimpl.FaceAlpha(faces[4].Flags, liquidSettings)
		if alpha4 != 1.0 {
			t.Errorf("lava alpha = %v, want 1.0", alpha4)
		}
		if pass := worldimpl.FacePass(faces[4].Flags, alpha4); pass != worldimpl.PassOpaque {
			t.Errorf("lava (alpha 1.0) pass = %v, want PassOpaque", pass)
		}

		// Face 5: Tele (telealpha = 0.9 -> translucent)
		alpha5 := worldimpl.FaceAlpha(faces[5].Flags, liquidSettings)
		if alpha5 != 0.9 {
			t.Errorf("tele alpha = %v, want 0.9", alpha5)
		}
		if pass := worldimpl.FacePass(faces[5].Flags, alpha5); pass != worldimpl.PassTranslucent {
			t.Errorf("tele pass = %v, want PassTranslucent", pass)
		}

		// Face 6: Fence (alpha test fence)
		alpha6 := worldimpl.FaceAlpha(faces[6].Flags, liquidSettings)
		if pass := worldimpl.FacePass(faces[6].Flags, alpha6); pass != worldimpl.PassAlphaTest {
			t.Errorf("fence pass = %v, want PassAlphaTest", pass)
		}
	})

	t.Run("LavaTranslucencyOverride", func(t *testing.T) {
		// When lavaalpha is configured < 1.0 (e.g. 0.8), lava must become PassTranslucent
		liquidSettings := worldimpl.LiquidAlphaSettings{
			Water: 0.6,
			Slime: 0.7,
			Lava:  0.8,
			Tele:  0.9,
		}
		lavaFace := world.Geometry.Faces[4]
		alpha := worldimpl.FaceAlpha(lavaFace.Flags, liquidSettings)
		if alpha != 0.8 {
			t.Errorf("lava alpha = %v, want 0.8", alpha)
		}
		if pass := worldimpl.FacePass(lavaFace.Flags, alpha); pass != worldimpl.PassTranslucent {
			t.Errorf("lava (alpha 0.8) pass = %v, want PassTranslucent", pass)
		}
	})

	t.Run("TranslucentGlassBrushEntityPlanning", func(t *testing.T) {
		// Semi-transparent glass brush entity with alpha = 0.5 placed at z = 16
		glassGeom := &worldimpl.WorldGeometry{
			Vertices: []worldimpl.WorldVertex{
				{Position: types.Vec3{X: -64, Y: -64, Z: 0}},
				{Position: types.Vec3{X: 64, Y: -64, Z: 0}},
				{Position: types.Vec3{X: 64, Y: 64, Z: 0}},
				{Position: types.Vec3{X: -64, Y: 64, Z: 0}},
			},
			Indices: []uint32{0, 1, 2, 0, 2, 3},
			Faces: []worldimpl.WorldFace{
				{FirstIndex: 0, NumIndices: 6, Flags: 0, Center: types.Vec3{X: 0, Y: 0, Z: 0}},
			},
		}

		glassEntity := worldgogpu.BrushEntityParams{
			Alpha:  0.5,
			Origin: types.Vec3{X: 0, Y: 0, Z: 16},
			Scale:  1.0,
		}

		draw := worldgogpu.BuildTranslucentBrushEntityDraw(glassEntity, glassGeom,
			func(face worldimpl.WorldFace, entityAlpha float32) (worldgogpu.TranslucentFacePlan, bool) {
				return worldgogpu.TranslucentFacePlan{
					Pass:   worldgogpu.TranslucentFacePassTranslucent,
					Alpha:  entityAlpha,
					Liquid: false,
				}, true
			},
			func(center types.Vec3) float32 {
				return center.Z
			},
		)

		if draw == nil {
			t.Fatal("BuildTranslucentBrushEntityDraw returned nil for glass entity")
		}
		if len(draw.TranslucentFaces) != 1 {
			t.Fatalf("len(TranslucentFaces) = %d, want 1", len(draw.TranslucentFaces))
		}
		if draw.TranslucentFaces[0].Alpha != 0.5 {
			t.Fatalf("glass face alpha = %v, want 0.5", draw.TranslucentFaces[0].Alpha)
		}
		if draw.TranslucentFaces[0].Center.Z != 16 {
			t.Fatalf("glass face center Z = %v, want 16", draw.TranslucentFaces[0].Center.Z)
		}
	})

	t.Run("SubmergedPropOpaqueOrderingParity", func(t *testing.T) {
		// Submerged prop (e.g. ammo box or pillar) placed at z = -48 in the water pool.
		// Its alpha is 1.0 (opaque), so it should be drawn during the opaque pass
		// with depth write enabled BEFORE the translucent water accumulation pass.
		propGeom := &worldimpl.WorldGeometry{
			Vertices: []worldimpl.WorldVertex{
				{Position: types.Vec3{X: -48, Y: -48, Z: -64}},
				{Position: types.Vec3{X: -16, Y: -48, Z: -64}},
				{Position: types.Vec3{X: -16, Y: -16, Z: -48}},
				{Position: types.Vec3{X: -48, Y: -16, Z: -48}},
			},
			Indices: []uint32{0, 1, 2, 0, 2, 3},
			Faces: []worldimpl.WorldFace{
				{FirstIndex: 0, NumIndices: 6, Flags: 0, Center: types.Vec3{X: -32, Y: -32, Z: -56}},
			},
		}

		propEntity := worldgogpu.BrushEntityParams{
			Alpha:  1.0,
			Origin: types.Vec3{X: -32, Y: -32, Z: -48},
			Scale:  1.0,
		}

		// A fully opaque brush entity (alpha = 1.0) must be rejected by translucent brush builders
		// because it belongs to the opaque scene pass.
		drawTranslucent := worldgogpu.BuildTranslucentBrushEntityDraw(propEntity, propGeom,
			func(face worldimpl.WorldFace, entityAlpha float32) (worldgogpu.TranslucentFacePlan, bool) {
				return worldgogpu.TranslucentFacePlan{Pass: worldgogpu.TranslucentFacePassTranslucent, Alpha: 1.0}, true
			},
			func(center types.Vec3) float32 { return 0 },
		)
		if drawTranslucent != nil {
			t.Fatalf("opaque submerged prop was incorrectly planned into translucent brush pass: %+v", drawTranslucent)
		}

		// Ensure PassWorldOpaque and PassTranslucentLiquids masks are distinct
		if renderer.PassWorldOpaque&renderer.PassTranslucentLiquids != 0 {
			t.Fatalf("PassWorldOpaque (0x%x) and PassTranslucentLiquids (0x%x) must not overlap",
				renderer.PassWorldOpaque, renderer.PassTranslucentLiquids)
		}
	})
}
