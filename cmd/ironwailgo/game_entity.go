package main

import (
	"bytes"
	"log/slog"
	"math"
	"strconv"
	"strings"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

func collectBrushEntities() []renderer.BrushEntity {
	if g.Client == nil || g.Server == nil || g.Server.WorldTree == nil || len(g.Server.WorldTree.Models) <= 1 {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.BrushEntity, bool) {
		if state.ModelIndex <= 1 {
			return renderer.BrushEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(g.Client.ModelPrecache) {
			return renderer.BrushEntity{}, false
		}
		modelName := g.Client.ModelPrecache[precacheIndex]
		if len(modelName) < 2 || modelName[0] != '*' {
			return renderer.BrushEntity{}, false
		}
		submodelIndex, err := strconv.Atoi(modelName[1:])
		if err != nil || submodelIndex <= 0 || submodelIndex >= len(g.Server.WorldTree.Models) {
			return renderer.BrushEntity{}, false
		}
		return renderer.BrushEntity{
			SubmodelIndex: submodelIndex,
			Frame:         int(state.Frame),
			Origin:        state.Origin,
			Angles:        state.Angles,
			Alpha:         entityStateAlpha(state),
			Scale:         entityStateScale(state),
		}, true
	}

	brushEntities := make([]renderer.BrushEntity, 0, len(g.Client.Entities)+len(g.Client.StaticEntities))
	for entityNum, state := range g.Client.Entities {
		if entityNum == g.Client.ViewEntity {
			continue
		}
		if brushEntity, ok := resolve(state); ok {
			brushEntities = append(brushEntities, brushEntity)
		}
	}
	for _, state := range g.Client.StaticEntities {
		if brushEntity, ok := resolve(state); ok {
			brushEntities = append(brushEntities, brushEntity)
		}
	}

	return brushEntities
}

func loadAliasModel(modelName string) (*model.Model, bool) {
	if modelName == "" || g.Subs == nil || g.Subs.Files == nil {
		return nil, false
	}
	if g.AliasModelCache == nil {
		g.AliasModelCache = make(map[string]*model.Model)
	}
	if mdl, ok := g.AliasModelCache[modelName]; ok {
		return mdl, mdl != nil
	}

	data, err := g.Subs.Files.LoadFile(modelName)
	if err != nil {
		slog.Debug("alias model load skipped", "model", modelName, "error", err)
		g.AliasModelCache[modelName] = nil
		return nil, false
	}
	loaded, err := model.LoadAliasModel(bytes.NewReader(data))
	if err != nil {
		slog.Debug("alias model parse skipped", "model", modelName, "error", err)
		g.AliasModelCache[modelName] = nil
		return nil, false
	}
	loaded.Name = modelName
	g.AliasModelCache[modelName] = loaded
	return loaded, true
}

func loadSpriteModel(modelName string) (*runtimeSpriteModel, bool) {
	if g.Subs == nil || g.Subs.Files == nil || modelName == "" {
		return nil, false
	}
	if g.SpriteModelCache == nil {
		g.SpriteModelCache = make(map[string]*runtimeSpriteModel)
	}
	if entry, ok := g.SpriteModelCache[modelName]; ok {
		return entry, entry != nil
	}

	data, err := g.Subs.Files.LoadFile(modelName)
	if err != nil {
		slog.Debug("sprite model load skipped", "model", modelName, "error", err)
		g.SpriteModelCache[modelName] = nil
		return nil, false
	}
	loaded, err := model.LoadSprite(bytes.NewReader(data))
	if err != nil {
		slog.Debug("sprite model parse skipped", "model", modelName, "error", err)
		g.SpriteModelCache[modelName] = nil
		return nil, false
	}

	halfWidth := float32(loaded.MaxWidth) * 0.5
	halfHeight := float32(loaded.MaxHeight) * 0.5
	entry := &runtimeSpriteModel{
		model: &model.Model{
			Name:      modelName,
			Type:      model.ModSprite,
			NumFrames: loaded.NumFrames,
			Mins:      [3]float32{-halfWidth, -halfWidth, -halfHeight},
			Maxs:      [3]float32{halfWidth, halfWidth, halfHeight},
		},
		sprite: loaded,
	}
	g.SpriteModelCache[modelName] = entry
	return entry, true
}

func collectAliasEntities() []renderer.AliasModelEntity {
	if g.Client == nil || g.Subs == nil || g.Subs.Files == nil {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.AliasModelEntity, bool) {
		if state.ModelIndex == 0 {
			return renderer.AliasModelEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(g.Client.ModelPrecache) {
			return renderer.AliasModelEntity{}, false
		}
		modelName := g.Client.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
			return renderer.AliasModelEntity{}, false
		}

		mdl, _ := loadAliasModel(modelName)
		if mdl == nil || mdl.Type != model.ModAlias || mdl.AliasHeader == nil || len(mdl.AliasHeader.Poses) == 0 {
			return renderer.AliasModelEntity{}, false
		}

		frame := int(state.Frame)
		if frame < 0 || frame >= mdl.AliasHeader.NumFrames {
			frame = 0
		}

		return renderer.AliasModelEntity{
			ModelID: modelName,
			Model:   mdl,
			Frame:   frame,
			SkinNum: int(state.Skin),
			Origin:  state.Origin,
			Angles:  state.Angles,
			Alpha:   entityStateAlpha(state),
			Scale:   entityStateScale(state),
		}, true
	}

	aliasEntities := make([]renderer.AliasModelEntity, 0, len(g.Client.Entities)+len(g.Client.StaticEntities))
	for entityNum, state := range g.Client.Entities {
		if entityNum == g.Client.ViewEntity {
			continue
		}
		if aliasEntity, ok := resolve(state); ok {
			aliasEntities = append(aliasEntities, aliasEntity)
		}
	}
	for _, state := range g.Client.StaticEntities {
		if aliasEntity, ok := resolve(state); ok {
			aliasEntities = append(aliasEntities, aliasEntity)
		}
	}

	return aliasEntities
}

func collectEntityEffectSources() []renderer.EntityEffectSource {
	if g.Client == nil {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.EntityEffectSource, bool) {
		if state.Effects == 0 || state.ModelIndex == 0 {
			return renderer.EntityEffectSource{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(g.Client.ModelPrecache) {
			return renderer.EntityEffectSource{}, false
		}
		modelName := g.Client.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
			return renderer.EntityEffectSource{}, false
		}
		return renderer.EntityEffectSource{
			Origin:  state.Origin,
			Angles:  state.Angles,
			Effects: state.Effects,
		}, true
	}

	sources := make([]renderer.EntityEffectSource, 0, len(g.Client.Entities)+len(g.Client.StaticEntities))
	for entNum, state := range g.Client.Entities {
		if source, ok := resolve(state); ok {
			source.EntityNum = entNum
			sources = append(sources, source)
		}
	}
	for _, state := range g.Client.StaticEntities {
		if source, ok := resolve(state); ok {
			sources = append(sources, source)
		}
	}

	return sources
}

func collectSpriteEntities() []renderer.SpriteEntity {
	if g.Client == nil || g.Subs == nil || g.Subs.Files == nil {
		return nil
	}

	viewForward, viewRight, _ := runtimeAngleVectors(g.Client.ViewAngles)
	resolve := func(state inet.EntityState) (renderer.SpriteEntity, bool) {
		if state.ModelIndex == 0 {
			return renderer.SpriteEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(g.Client.ModelPrecache) {
			return renderer.SpriteEntity{}, false
		}
		modelName := g.Client.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".spr") {
			return renderer.SpriteEntity{}, false
		}

		entry, _ := loadSpriteModel(modelName)
		if entry == nil || entry.model == nil || entry.model.Type != model.ModSprite || entry.sprite == nil || entry.sprite.NumFrames == 0 {
			return renderer.SpriteEntity{}, false
		}

		frame := resolveRuntimeSpriteFrame(entry.sprite, int(state.Frame), state.Angles, viewForward, viewRight, g.Client.Time)

		return renderer.SpriteEntity{
			ModelID:    modelName,
			Model:      entry.model,
			Frame:      frame,
			Origin:     state.Origin,
			Angles:     state.Angles,
			Alpha:      entityStateAlpha(state),
			Scale:      entityStateScale(state),
			SpriteData: entry.sprite,
		}, true
	}

	spriteEntities := make([]renderer.SpriteEntity, 0, len(g.Client.Entities)+len(g.Client.StaticEntities))
	for entityNum, state := range g.Client.Entities {
		if entityNum == g.Client.ViewEntity {
			continue
		}
		if spriteEntity, ok := resolve(state); ok {
			spriteEntities = append(spriteEntities, spriteEntity)
		}
	}
	for _, state := range g.Client.StaticEntities {
		if spriteEntity, ok := resolve(state); ok {
			spriteEntities = append(spriteEntities, spriteEntity)
		}
	}

	return spriteEntities
}

func resolveRuntimeSpriteFrame(sprite *model.MSprite, frame int, entityAngles [3]float32, viewForward, viewRight [3]float32, clientTime float64) int {
	if sprite == nil || sprite.NumFrames == 0 || len(sprite.Frames) == 0 {
		return 0
	}
	if frame < 0 || frame >= sprite.NumFrames || frame >= len(sprite.Frames) {
		frame = 0
	}

	flatOffset := spriteFlatFrameOffset(sprite, frame)
	frameDesc := sprite.Frames[frame]
	switch frameDesc.Type {
	case model.SpriteFrameGroup:
		return flatOffset + resolveRuntimeSpriteGroupSubframe(frameDesc.FramePtr, clientTime)
	case model.SpriteFrameAngled:
		return flatOffset + resolveRuntimeSpriteAngledSubframe(frameDesc.FramePtr, entityAngles, viewForward, viewRight)
	default:
		return flatOffset
	}
}

func resolveRuntimeSpriteGroupSubframe(framePtr interface{}, clientTime float64) int {
	group, ok := framePtr.(*model.MSpriteGroup)
	if !ok || group == nil || group.NumFrames <= 0 || len(group.Intervals) == 0 {
		return 0
	}
	lastInterval := group.Intervals[len(group.Intervals)-1]
	if lastInterval <= 0 {
		return 0
	}

	targetTime := float32(math.Mod(clientTime, float64(lastInterval)))
	if targetTime < 0 {
		targetTime += lastInterval
	}
	for subframe := 0; subframe < group.NumFrames && subframe < len(group.Intervals); subframe++ {
		if targetTime < group.Intervals[subframe] {
			return subframe
		}
	}
	return 0
}

func resolveRuntimeSpriteAngledSubframe(framePtr interface{}, entityAngles [3]float32, viewForward, viewRight [3]float32) int {
	group, ok := framePtr.(*model.MSpriteGroup)
	if !ok || group == nil || group.NumFrames <= 0 || len(group.Frames) == 0 {
		return 0
	}

	frameCount := group.NumFrames
	if len(group.Frames) < frameCount {
		frameCount = len(group.Frames)
	}
	if frameCount <= 0 {
		return 0
	}

	entityForward, _, _ := runtimeAngleVectors(entityAngles)
	forwardDot := qtypes.Vec3Dot(
		qtypes.Vec3{X: viewForward[0], Y: viewForward[1], Z: viewForward[2]},
		qtypes.Vec3{X: entityForward[0], Y: entityForward[1], Z: entityForward[2]},
	)
	rightDot := qtypes.Vec3Dot(
		qtypes.Vec3{X: viewRight[0], Y: viewRight[1], Z: viewRight[2]},
		qtypes.Vec3{X: entityForward[0], Y: entityForward[1], Z: entityForward[2]},
	)

	dir := int((math.Atan2(float64(rightDot), float64(forwardDot)) + 1.125*math.Pi) * (4.0 / math.Pi))
	dir %= frameCount
	if dir < 0 {
		dir += frameCount
	}
	return dir
}

func spriteFlatFrameOffset(sprite *model.MSprite, frame int) int {
	if sprite == nil || frame <= 0 {
		return 0
	}
	maxFrame := frame
	if maxFrame > len(sprite.Frames) {
		maxFrame = len(sprite.Frames)
	}
	offset := 0
	for i := 0; i < maxFrame; i++ {
		offset += spriteFrameSpan(sprite.Frames[i])
	}
	return offset
}

func spriteFrameSpan(frameDesc model.MSpriteFrameDesc) int {
	switch frameDesc.Type {
	case model.SpriteFrameGroup, model.SpriteFrameAngled:
		group, ok := frameDesc.FramePtr.(*model.MSpriteGroup)
		if !ok || group == nil || group.NumFrames <= 0 {
			return 1
		}
		return group.NumFrames
	default:
		return 1
	}
}

func buildRuntimeRenderFrameState(brushEntities []renderer.BrushEntity, aliasEntities []renderer.AliasModelEntity, spriteEntities []renderer.SpriteEntity, viewModel *renderer.AliasModelEntity) *renderer.RenderFrameState {
	state := renderer.DefaultRenderFrameState()
	state.ClearColor = [4]float32{0, 0, 0, 1}
	state.DrawWorld = g.Renderer != nil && g.Renderer.HasWorldData()
	state.DrawEntities = len(brushEntities) > 0 || len(aliasEntities) > 0 || len(spriteEntities) > 0 || viewModel != nil
	state.BrushEntities = brushEntities
	state.AliasEntities = aliasEntities
	state.SpriteEntities = spriteEntities
	state.ViewModel = viewModel
	state.DrawParticles = g.Particles != nil && g.Particles.ActiveCount() > 0
	state.Draw2DOverlay = true
	state.MenuActive = g.Menu != nil && g.Menu.IsActive()
	state.Particles = g.Particles
	if g.DecalMarks != nil {
		state.DecalMarks = g.DecalMarks.ActiveMarks()
	}
	if g.Client != nil {
		state.LightStyles = g.Client.LightStyleValues()
		state.FogDensity, state.FogColor = g.Client.CurrentFog()
	}
	if g.Draw != nil {
		state.Palette = g.Draw.Palette()
	}
	// Set underwater visual warp state (r_waterwarp).
	// WaterWarp (r_waterwarp == 1): screen-space sinusoidal post-process.
	// ForceUnderwater: menu is previewing the waterwarp option.
	// WaterwarpFOV is applied via CameraState.WaterwarpFOV in UpdateCamera.
	waterWarp, _, warpTime := runtimeWaterwarpState()
	state.WaterWarp = waterWarp
	state.WaterWarpTime = warpTime
	state.ForceUnderwater = g.Menu != nil && g.Menu.ForcedUnderwater()

	// Compute v_blend (polyblend) screen tint from client color shifts.
	// Mirrors C Ironwail: view.c V_CalcBlend() → V_PolyBlend().
	// Only apply when gl_polyblend is enabled.
	if g.Client != nil && g.Client.State == cl.StateActive {
		polyblendEnabled := true
		if cv := cvar.Get("gl_polyblend"); cv != nil {
			polyblendEnabled = cv.Float32() != 0
		}
		if polyblendEnabled {
			globalPct := float32(100)
			if cv := cvar.Get("gl_cshiftpercent"); cv != nil {
				globalPct = cv.Float32()
			}
			g.Client.SetContentsColor(g.CameraLeafContents)
			state.VBlend = g.Client.CalcBlend(globalPct)
		}
	}
	return state
}

func entityStateAlpha(state inet.EntityState) float32 {
	return inet.ENTALPHA_DECODE(state.Alpha)
}

func entityStateScale(state inet.EntityState) float32 {
	scale := inet.ENTSCALE_DECODE(state.Scale)
	if scale <= 0 {
		return 1
	}
	return scale
}

func collectViewModelEntity() *renderer.AliasModelEntity {
	if !runtimeViewModelVisible() {
		return nil
	}

	modelIndex := g.Client.WeaponModelIndex()
	if modelIndex <= 0 {
		return nil
	}
	precacheIndex := modelIndex - 1
	if precacheIndex < 0 || precacheIndex >= len(g.Client.ModelPrecache) {
		return nil
	}

	modelName := g.Client.ModelPrecache[precacheIndex]
	if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
		return nil
	}
	mdl, ok := loadAliasModel(modelName)
	if !ok || mdl == nil || mdl.AliasHeader == nil || mdl.AliasHeader.NumFrames == 0 {
		return nil
	}

	frame := g.Client.WeaponFrame()
	if frame < 0 || frame >= mdl.AliasHeader.NumFrames {
		frame = 0
	}
	origin, _ := runtimeViewState()
	viewAngles := g.Client.ViewAngles

	// CalcGunAngle: rate-limited drift + idle sway on the weapon model.
	frameTime := 0.0
	if g.Host != nil {
		frameTime = g.Host.FrameTime()
	}
	angles := viewCalcGunAngle(&globalViewCalc, viewAngles, g.Client.Time, frameTime)

	// Apply view bob to weapon origin (V_CalcRefdef: forward*bob*0.4 + Z bob).
	bob := viewCalcBob(g.Client.Time, g.Client.Velocity)
	if bob != 0 {
		forward, _, _ := runtimeAngleVectors(viewAngles)
		origin = viewApplyBobToOrigin(origin, forward, bob)
	}

	// r_viewmodel_quake origin fudge.
	scrViewSize := 100.0
	if cv := cvar.Get("scr_viewsize"); cv != nil {
		scrViewSize = cv.Float
	}
	origin = viewApplyViewmodelQuakeFudge(origin, scrViewSize)

	// Apply stair step smoothing to weapon origin.
	// Mirrors C Ironwail V_CalcRefdef: view->origin[2] += oldz - ent->origin[2].
	// Note: globalViewCalc.oldZ was already updated by runtimeCameraState, so we just
	// need to apply the offset. However, since we don't have the offset stored, we need
	// to recompute it. But we can't call viewStairSmoothOffset again because it modifies
	// state. Instead, we'll compute the offset directly from globalViewCalc.oldZ.
	if entityOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
		if globalViewCalc.oldZInit {
			offset := globalViewCalc.oldZ - entityOrigin[2]
			origin[2] += offset
		}
	}

	return &renderer.AliasModelEntity{
		ModelID: modelName,
		Model:   mdl,
		Frame:   frame,
		SkinNum: 0,
		Origin:  origin,
		Angles:  angles,
		Alpha:   1,
		Scale:   1,
	}
}
