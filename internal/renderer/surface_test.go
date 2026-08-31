package renderer

import (
	"errors"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/pkg/types"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

func TestTextureAnimation(t *testing.T) {
	t1 := &surfacepkg.SurfaceTexture{AnimTotal: 20, AnimMin: 0, AnimMax: 10}
	t2 := &surfacepkg.SurfaceTexture{AnimTotal: 20, AnimMin: 10, AnimMax: 20}
	t1.AnimNext = t2
	t2.AnimNext = t1

	tex, err := surfacepkg.TextureAnimation(t1, 0, 1.5)
	if err != nil {
		t.Fatalf("surfacepkg.TextureAnimation error: %v", err)
	}
	if tex != t2 {
		t.Fatalf("surfacepkg.TextureAnimation selected wrong frame")
	}

	alt := &surfacepkg.SurfaceTexture{AnimTotal: 0}
	t1.AlternateAnims = alt
	tex, err = surfacepkg.TextureAnimation(t1, 1, 0.0)
	if err != nil {
		t.Fatalf("surfacepkg.TextureAnimation alternate error: %v", err)
	}
	if tex != alt {
		t.Fatalf("surfacepkg.TextureAnimation alternate frame mismatch")
	}
}

func TestTextureAnimationBrokenCycle(t *testing.T) {
	base := &surfacepkg.SurfaceTexture{AnimTotal: 10, AnimMin: 9, AnimMax: 10, AnimNext: nil}
	_, err := surfacepkg.TextureAnimation(base, 0, 0.0)
	if !errors.Is(err, surfacepkg.ErrBrokenTextureAnimationCycle) {
		t.Fatalf("error = %v, want %v", err, surfacepkg.ErrBrokenTextureAnimationCycle)
	}
}

func TestBuildTextureAnimationsLinksPrimaryAndAlternateChains(t *testing.T) {
	textures, err := surfacepkg.BuildTextureAnimations([]string{"+0lava", "+1lava", "+Alava", "+Blava", "stone"})
	if err != nil {
		t.Fatalf("surfacepkg.BuildTextureAnimations error: %v", err)
	}

	primary, err := surfacepkg.TextureAnimation(textures[0], 0, 0.3)
	if err != nil {
		t.Fatalf("surfacepkg.TextureAnimation(primary) error: %v", err)
	}
	if primary != textures[1] {
		t.Fatalf("surfacepkg.TextureAnimation(primary) = %#v, want frame 1", primary)
	}

	alternate, err := surfacepkg.TextureAnimation(textures[0], 1, 0.0)
	if err != nil {
		t.Fatalf("surfacepkg.TextureAnimation(alternate) error: %v", err)
	}
	if alternate != textures[2] {
		t.Fatalf("surfacepkg.TextureAnimation(alternate) = %#v, want alternate frame A", alternate)
	}
	if alternate.TextureIndex != 2 {
		t.Fatalf("alternate TextureIndex = %d, want 2", alternate.TextureIndex)
	}
}

func TestBuildTextureAnimationsRejectsMissingFrame(t *testing.T) {
	_, err := surfacepkg.BuildTextureAnimations([]string{"+0lava", "+2lava"})
	if err == nil || err.Error() != "missing frame 1 of +0lava" {
		t.Fatalf("surfacepkg.BuildTextureAnimations error = %v, want missing frame error", err)
	}
}

func TestBuildTextureAnimationsRejectsInvalidFrameToken(t *testing.T) {
	_, err := surfacepkg.BuildTextureAnimations([]string{"+"})
	if err == nil || err.Error() != "bad animating texture \"+\"" {
		t.Fatalf("surfacepkg.BuildTextureAnimations error = %v, want bad animating texture error", err)
	}
}

func TestBlendFogStateTowards(t *testing.T) {
	prevColor := types.Vec3{X: 0.1, Y: 0.2, Z: 0.3}
	prevDensity := float32(0.1)
	nextColor := types.Vec3{X: 0.9, Y: 0.05, Z: 0.6}
	nextDensity := float32(0.7)

	gotColor, gotDensity := blendFogStateTowards(prevColor, prevDensity, nextColor, nextDensity, 0.2)
	wantColor := types.Vec3{X: 0.3, Y: 0.05, Z: 0.5}
	wantDensity := float32(0.3)
	if gotColor != wantColor {
		t.Fatalf("blendFogStateTowards color = %v, want %v", gotColor, wantColor)
	}
	if gotDensity != wantDensity {
		t.Fatalf("blendFogStateTowards density = %v, want %v", gotDensity, wantDensity)
	}
}

func TestBlendFogStateTowardsImmediateWhenStepDisabled(t *testing.T) {
	prevColor := types.Vec3{X: 0.1, Y: 0.2, Z: 0.3}
	prevDensity := float32(0.1)
	nextColor := types.Vec3{X: 0.9, Y: 0.8, Z: 0.7}
	nextDensity := float32(0.6)

	gotColor, gotDensity := blendFogStateTowards(prevColor, prevDensity, nextColor, nextDensity, 0)
	if gotColor != nextColor {
		t.Fatalf("blendFogStateTowards color = %v, want %v", gotColor, nextColor)
	}
	if gotDensity != nextDensity {
		t.Fatalf("blendFogStateTowards density = %v, want %v", gotDensity, nextDensity)
	}
}

func TestChartAddSerpentine(t *testing.T) {
	var c surfacepkg.Chart
	if err := c.Init(8, 4); err != nil {
		t.Fatalf("surfacepkg.Chart.Init error: %v", err)
	}

	x, y, ok, err := c.Add(3, 2)
	if err != nil || !ok || x != 0 || y != 0 {
		t.Fatalf("first add = (%d,%d,%v,%v), want (0,0,true,nil)", x, y, ok, err)
	}

	x, y, ok, err = c.Add(3, 2)
	if err != nil || !ok || x != 3 || y != 0 {
		t.Fatalf("second add = (%d,%d,%v,%v), want (3,0,true,nil)", x, y, ok, err)
	}

	x, y, ok, err = c.Add(3, 2)
	if err != nil || !ok || x != 5 || y != 2 {
		t.Fatalf("third add = (%d,%d,%v,%v), want (5,2,true,nil)", x, y, ok, err)
	}
}

func TestLightmapAllocatorReserveFirstTexel(t *testing.T) {
	a, err := surfacepkg.NewLightmapAllocator(8, 8, true)
	if err != nil {
		t.Fatalf("surfacepkg.NewLightmapAllocator error: %v", err)
	}

	tex, x, y, err := a.AllocBlock(1, 1)
	if err != nil {
		t.Fatalf("AllocBlock error: %v", err)
	}
	if tex != 0 || x != 1 || y != 0 {
		t.Fatalf("AllocBlock = (%d,%d,%d), want (0,1,0)", tex, x, y)
	}
}

func TestFillSurfaceLightmapSingleStyle(t *testing.T) {
	in := surfacepkg.SurfaceLightmapInput{
		Styles:  [4]byte{0, 255, 255, 255},
		Extents: [2]int16{16, 16},
		Samples: []byte{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
			10, 11, 12,
		},
	}

	dst := make([]uint32, 16)
	if err := surfacepkg.FillSurfaceLightmap(in, surfacepkg.Lightmap{}, 4, dst); err != nil {
		t.Fatalf("surfacepkg.FillSurfaceLightmap error: %v", err)
	}

	want := []uint32{
		0xff030201, 0xff060504,
		0xff090807, 0xff0c0b0a,
	}
	got := []uint32{dst[0], dst[1], dst[4], dst[5]}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("single-style pixel %d = 0x%08x, want 0x%08x", i, got[i], want[i])
		}
	}
}

func TestFillSurfaceLightmapTwoStyles(t *testing.T) {
	in := surfacepkg.SurfaceLightmapInput{
		Styles:  [4]byte{0, 1, 255, 255},
		Extents: [2]int16{16, 16},
		Samples: []byte{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
			10, 11, 12,

			13, 14, 15,
			16, 17, 18,
			19, 20, 21,
			22, 23, 24,
		},
	}

	dst := make([]uint32, 32)
	if err := surfacepkg.FillSurfaceLightmap(in, surfacepkg.Lightmap{}, 8, dst); err != nil {
		t.Fatalf("surfacepkg.FillSurfaceLightmap error: %v", err)
	}

	if dst[0] != 0xff030201 || dst[1] != 0xff060504 || dst[2] != 0xff0f0e0d || dst[3] != 0xff121110 {
		t.Fatalf("unexpected first row values: %08x %08x %08x %08x", dst[0], dst[1], dst[2], dst[3])
	}
}

func TestFillSurfaceLightmapPackedRGB(t *testing.T) {
	in := surfacepkg.SurfaceLightmapInput{
		Styles:  [4]byte{0, 1, 2, 255},
		Extents: [2]int16{0, 0},
		Samples: []byte{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
	}

	dst := make([]uint32, 3)
	if err := surfacepkg.FillSurfaceLightmap(in, surfacepkg.Lightmap{}, 3, dst); err != nil {
		t.Fatalf("surfacepkg.FillSurfaceLightmap error: %v", err)
	}

	if dst[0] != 0x00070401 {
		t.Fatalf("packed r = 0x%08x, want 0x00070401", dst[0])
	}
	if dst[1] != 0x00080502 {
		t.Fatalf("packed g = 0x%08x, want 0x00080502", dst[1])
	}
	if dst[2] != 0x00090603 {
		t.Fatalf("packed b = 0x%08x, want 0x00090603", dst[2])
	}
}

func TestSetupAliasFrameAndTransform(t *testing.T) {
	hdr := &AliasHeader{
		NumFrames: 2,
		Frames: []AliasFrame{
			{FirstPose: 0, NumPoses: 1, Interval: 0.1},
			{FirstPose: 1, NumPoses: 2, Interval: 0.2},
		},
	}

	ent := &AliasEntity{Frame: 1, CurrentPose: 0, PreviousPose: 0}

	lerp, err := SetupAliasFrame(ent, hdr, 0.2, true, false, 1)
	if err != nil {
		t.Fatalf("SetupAliasFrame error: %v", err)
	}
	if lerp.Pose1 != 0 || lerp.Pose2 != 2 {
		t.Fatalf("poses = (%d,%d), want (0,2)", lerp.Pose1, lerp.Pose2)
	}

	ent.LerpFlags = LerpMoveStep
	ent.PreviousOrigin = types.Vec3{X: 0, Y: 0, Z: 0}
	ent.CurrentOrigin = types.Vec3{X: 10, Y: 0, Z: 0}
	ent.Origin = ent.CurrentOrigin
	ent.PreviousAngles = types.Vec3{X: 0, Y: 0, Z: 0}
	ent.CurrentAngles = types.Vec3{X: 0, Y: 90, Z: 0}
	ent.Angles = ent.CurrentAngles
	ent.MoveLerpStart = 0

	origin, angles := SetupEntityTransform(ent, 0.05, true, false, false, false, 1)
	if origin.X < 4.99 || origin.X > 5.01 {
		t.Fatalf("origin.x = %f, want ~5", origin.X)
	}
	if angles.Y < 44.99 || angles.Y > 45.01 {
		t.Fatalf("angles.y = %f, want ~45", angles.Y)
	}
}

func TestRendererAliasStateInterpolatesFrameAndTransformAcrossUpdates(t *testing.T) {
	r := &Renderer{}
	hdr := &model.AliasHeader{
		NumFrames: 2,
		Frames: []model.AliasFrameDesc{
			{FirstPose: 0, NumPoses: 1, Interval: 0.1},
			{FirstPose: 1, NumPoses: 1, Interval: 0.1},
		},
	}

	first := r.ensureAliasStateLocked(AliasModelEntity{
		ModelID:     "progs/ogre.mdl",
		EntityKey:   7,
		Frame:       0,
		TimeSeconds: 1.0,
		Origin:      types.Vec3{X: 0, Y: 0, Z: 0},
		Angles:      types.Vec3{X: 0, Y: 0, Z: 0},
	})
	lerp, err := SetupAliasFrame(first, aliasHeaderFromModel(hdr), 1.0, true, false, 1)
	if err != nil {
		t.Fatalf("SetupAliasFrame(first) error: %v", err)
	}
	if lerp.Pose1 != 0 || lerp.Pose2 != 0 || lerp.Blend != 1 {
		t.Fatalf("first lerp = %#v, want pose snap to 0", lerp)
	}
	origin, angles := SetupEntityTransform(first, 1.0, true, false, false, false, 1)
	if origin != (types.Vec3{}) || angles != (types.Vec3{}) {
		t.Fatalf("first transform = (%v,%v), want origin/angles at zero", origin, angles)
	}

	second := r.ensureAliasStateLocked(AliasModelEntity{
		ModelID:     "progs/ogre.mdl",
		EntityKey:   7,
		Frame:       1,
		TimeSeconds: 1.05,
		LerpFlags:   LerpMoveStep,
		Origin:      types.Vec3{X: 10, Y: 0, Z: 0},
		Angles:      types.Vec3{X: 0, Y: 90, Z: 0},
	})
	lerp, err = SetupAliasFrame(second, aliasHeaderFromModel(hdr), 1.05, true, false, 1)
	if err != nil {
		t.Fatalf("SetupAliasFrame(second) error: %v", err)
	}
	if lerp.Pose1 != 0 || lerp.Pose2 != 1 {
		t.Fatalf("second poses = (%d,%d), want (0,1)", lerp.Pose1, lerp.Pose2)
	}
	if lerp.Blend != 0 {
		t.Fatalf("second blend = %f, want 0 on frame change", lerp.Blend)
	}
	origin, angles = SetupEntityTransform(second, 1.05, true, false, false, false, 1)
	if origin != (types.Vec3{}) || angles != (types.Vec3{}) {
		t.Fatalf("second transform = (%v,%v), want interpolation start", origin, angles)
	}

	third := r.ensureAliasStateLocked(AliasModelEntity{
		ModelID:     "progs/ogre.mdl",
		EntityKey:   7,
		Frame:       1,
		TimeSeconds: 1.10,
		LerpFlags:   LerpMoveStep,
		Origin:      types.Vec3{X: 10, Y: 0, Z: 0},
		Angles:      types.Vec3{X: 0, Y: 90, Z: 0},
	})
	lerp, err = SetupAliasFrame(third, aliasHeaderFromModel(hdr), 1.10, true, false, 1)
	if err != nil {
		t.Fatalf("SetupAliasFrame(third) error: %v", err)
	}
	if lerp.Pose1 != 0 || lerp.Pose2 != 1 {
		t.Fatalf("third poses = (%d,%d), want (0,1)", lerp.Pose1, lerp.Pose2)
	}
	if lerp.Blend < 0.49 || lerp.Blend > 0.51 {
		t.Fatalf("third blend = %f, want ~0.5", lerp.Blend)
	}
	origin, angles = SetupEntityTransform(third, 1.10, true, false, false, false, 1)
	if origin.X < 4.99 || origin.X > 5.01 {
		t.Fatalf("origin.x = %f, want ~5", origin.X)
	}
	if angles.Y < 44.99 || angles.Y > 45.01 {
		t.Fatalf("angles.y = %f, want ~45", angles.Y)
	}
}

func TestRendererPruneAliasStatesDropsRetiredWorldEntities(t *testing.T) {
	r := &Renderer{
		aliasRendererState: aliasRendererState{
			aliasEntityStates: map[int]*AliasEntity{
				1: {ModelID: "progs/ogre.mdl"},
				2: {ModelID: "progs/knight.mdl"},
			},
			viewModelAliasState: &AliasEntity{ModelID: "progs/v_axe.mdl"},
		},
	}

	r.pruneAliasStatesLocked([]AliasModelEntity{{EntityKey: 2}})

	if _, ok := r.aliasEntityStates[1]; ok {
		t.Fatal("expected retired world alias state to be pruned")
	}
	if _, ok := r.aliasEntityStates[2]; !ok {
		t.Fatal("expected active world alias state to be preserved")
	}
	if r.viewModelAliasState == nil {
		t.Fatal("expected viewmodel alias state to remain untouched")
	}
}
