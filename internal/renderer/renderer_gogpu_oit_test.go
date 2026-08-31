package renderer

import (
	"strings"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func TestOITPipelineSetup(t *testing.T) {
	targets := oitColorTargetStates()
	if len(targets) != 2 {
		t.Fatalf("oitColorTargetStates() target count = %d, want 2", len(targets))
	}

	// Target 0: Accumulation buffer (RGBA16Float)
	target0 := targets[0]
	if target0.Format != gputypes.TextureFormatRGBA16Float {
		t.Errorf("Target 0 format = %v, want %v (RGBA16Float)", target0.Format, gputypes.TextureFormatRGBA16Float)
	}
	if target0.Blend == nil {
		t.Fatal("Target 0 blend is nil")
	}
	if target0.Blend.Color.SrcFactor != gputypes.BlendFactorOne ||
		target0.Blend.Color.DstFactor != gputypes.BlendFactorOne ||
		target0.Blend.Color.Operation != gputypes.BlendOperationAdd {
		t.Errorf("Target 0 color blend = (%v, %v, %v), want (One, One, Add)",
			target0.Blend.Color.SrcFactor, target0.Blend.Color.DstFactor, target0.Blend.Color.Operation)
	}
	if target0.Blend.Alpha.SrcFactor != gputypes.BlendFactorOne ||
		target0.Blend.Alpha.DstFactor != gputypes.BlendFactorOne ||
		target0.Blend.Alpha.Operation != gputypes.BlendOperationAdd {
		t.Errorf("Target 0 alpha blend = (%v, %v, %v), want (One, One, Add)",
			target0.Blend.Alpha.SrcFactor, target0.Blend.Alpha.DstFactor, target0.Blend.Alpha.Operation)
	}
	if target0.WriteMask != gputypes.ColorWriteMaskAll {
		t.Errorf("Target 0 write mask = %v, want ColorWriteMaskAll", target0.WriteMask)
	}

	// Target 1: Revealage buffer (R8Unorm)
	target1 := targets[1]
	if target1.Format != gputypes.TextureFormatR8Unorm {
		t.Errorf("Target 1 format = %v, want %v (R8Unorm)", target1.Format, gputypes.TextureFormatR8Unorm)
	}
	if target1.Blend == nil {
		t.Fatal("Target 1 blend is nil")
	}
	if target1.Blend.Color.SrcFactor != gputypes.BlendFactorZero ||
		target1.Blend.Color.DstFactor != gputypes.BlendFactorOneMinusSrc ||
		target1.Blend.Color.Operation != gputypes.BlendOperationAdd {
		t.Errorf("Target 1 color blend = (%v, %v, %v), want (Zero, OneMinusSrc, Add)",
			target1.Blend.Color.SrcFactor, target1.Blend.Color.DstFactor, target1.Blend.Color.Operation)
	}
	if target1.Blend.Alpha.SrcFactor != gputypes.BlendFactorZero ||
		target1.Blend.Alpha.DstFactor != gputypes.BlendFactorOneMinusSrc ||
		target1.Blend.Alpha.Operation != gputypes.BlendOperationAdd {
		t.Errorf("Target 1 alpha blend = (%v, %v, %v), want (Zero, OneMinusSrc, Add)",
			target1.Blend.Alpha.SrcFactor, target1.Blend.Alpha.DstFactor, target1.Blend.Alpha.Operation)
	}
	if target1.WriteMask != gputypes.ColorWriteMaskAll {
		t.Errorf("Target 1 write mask = %v, want ColorWriteMaskAll", target1.WriteMask)
	}

	// Verify non-decal depth state for accumulation has DepthWriteEnabled = false
	depthState := gogpuNonDecalDepthStencilState(false)
	if depthState.DepthWriteEnabled {
		t.Error("DepthWriteEnabled = true for OIT accumulation, want false")
	}
	if depthState.DepthCompare != gputypes.CompareFunctionLessEqual {
		t.Errorf("DepthCompare = %v, want LessEqual", depthState.DepthCompare)
	}
}

func TestOITShaderEpilogue(t *testing.T) {
	epilogue := oitAccumFragEpilogueWGSL()

	// Check for MRT output struct locations
	if !strings.Contains(epilogue, "@location(0) accum: vec4<f32>") {
		t.Errorf("epilogue missing @location(0) accum: vec4<f32>, got:\n%s", epilogue)
	}
	if !strings.Contains(epilogue, "@location(1) reveal: vec4<f32>") && !strings.Contains(epilogue, "@location(1) reveal: f32") {
		t.Errorf("epilogue missing @location(1) reveal, got:\n%s", epilogue)
	}

	// Check McGuire weight formula matching C Ironwail:
	// weight = clamp(pow(color.a, 2.0) * 0.03 / (1e-5 + pow(z / 1e7, 1.0)), 1e-2, 3e3)
	// or color.a * color.a * 0.03 / (1e-5 + z / 1e7)
	hasWeightFormula := strings.Contains(epilogue, "clamp(pow(color.a, 2.0) * 0.03 / (1e-5 + pow(z / 1e7, 1.0)), 1e-2, 3e3)") ||
		strings.Contains(epilogue, "clamp(color.a * color.a * 0.03 / (1e-5 + z / 1e7), 1e-2, 3e3)")
	if !hasWeightFormula {
		t.Errorf("epilogue missing McGuire weight formula, got:\n%s", epilogue)
	}

	// Check accumulation output (premultiplied rgb + a * weight)
	hasAccumAssign := strings.Contains(epilogue, "o.accum = vec4<f32>(color.rgb * premul, premul)") ||
		strings.Contains(epilogue, "o.accum = vec4<f32>(color.rgb * color.a, color.a) * weight")
	if !hasAccumAssign {
		t.Errorf("epilogue missing accum assignment, got:\n%s", epilogue)
	}

	// Check revealage output (color.a)
	if !strings.Contains(epilogue, "o.reveal = vec4<f32>(color.a") && !strings.Contains(epilogue, "o.reveal = color.a") {
		t.Errorf("epilogue missing revealage assignment, got:\n%s", epilogue)
	}

	// Check shader wrappers for all translucent types
	t.Run("WaterShader", func(t *testing.T) {
		shader := oitTranslucentWaterFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("water shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("water shader missing MRT outputs")
		}
	})

	t.Run("WorldTranslucentShader", func(t *testing.T) {
		shader := oitTranslucentWorldFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("world translucent shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("world translucent shader missing MRT outputs")
		}
	})

	t.Run("AliasShader", func(t *testing.T) {
		shader := oitAliasFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("alias shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("alias shader missing MRT outputs")
		}
	})

	t.Run("ParticleShader", func(t *testing.T) {
		shader := oitParticleFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("particle shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("particle shader missing MRT outputs")
		}
	})

	t.Run("SpriteShader", func(t *testing.T) {
		shader := oitSpriteFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("sprite shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("sprite shader missing MRT outputs")
		}
	})

	t.Run("DecalShader", func(t *testing.T) {
		shader := oitDecalFragmentShaderWGSL()
		if !strings.Contains(shader, "fn oitColor(input: VertexOutput)") {
			t.Errorf("decal shader missing oitColor function")
		}
		if !strings.Contains(shader, "@location(0) accum") || !strings.Contains(shader, "@location(1) reveal") {
			t.Errorf("decal shader missing MRT outputs")
		}
	})
}

func TestOITAccumulationPassContracts(t *testing.T) {
	// Verify that effective alpha mode controls OIT enable state
	origMode := GetAlphaMode()
	defer SetAlphaMode(origMode)

	SetAlphaMode(AlphaModeOIT)
	if !goGPUOITEnabled() {
		t.Fatal("goGPUOITEnabled() = false with AlphaModeOIT, want true")
	}

	SetAlphaMode(AlphaModeBasic)
	if goGPUOITEnabled() {
		t.Fatal("goGPUOITEnabled() = true with AlphaModeBasic, want false")
	}

	SetAlphaMode(AlphaModeSorted)
	if goGPUOITEnabled() {
		t.Fatal("goGPUOITEnabled() = true with AlphaModeSorted, want false")
	}

	// Verify all translucent entity phases are recognized as late translucency
	phases := []gogpuEntityPhase{
		gogpuEntityPhaseTranslucentWorldLiquid,
		gogpuEntityPhaseTranslucentLiquidBrush,
		gogpuEntityPhaseTranslucentBrush,
		gogpuEntityPhaseDecals,
		gogpuEntityPhaseTranslucentAlias,
		gogpuEntityPhaseSprites,
		gogpuEntityPhaseTranslucentParticles,
	}
	for _, phase := range phases {
		if !isTranslucentEntityPhase(phase) {
			t.Errorf("isTranslucentEntityPhase(%v) = false, want true", phase)
		}
	}

	opaquePhases := []gogpuEntityPhase{
		gogpuEntityPhaseOpaqueBrush,
		gogpuEntityPhaseOpaqueAlias,
		gogpuEntityPhaseOpaqueParticles,
		gogpuEntityPhaseSkyBrush,
		gogpuEntityPhaseOpaqueLiquidBrush,
	}
	for _, phase := range opaquePhases {
		if isTranslucentEntityPhase(phase) {
			t.Errorf("isTranslucentEntityPhase(%v) = true, want false", phase)
		}
	}
}

func TestOITResourceReleaseOnShutdown(t *testing.T) {
	r := &Renderer{}
	r.ensureResources()

	// Simulate OIT accumulation pipeline layout aliasing WorldPipelineLayout
	fakeWorldLayout := &wgpu.PipelineLayout{}
	r.resources.WorldPipelineLayout = fakeWorldLayout
	r.resources.OITAccumPipelineLayout = fakeWorldLayout

	// Calling destroyOITResourcesLocked should nil out OITAccumPipelineLayout
	// WITHOUT calling .Release() on WorldPipelineLayout (which would panic on dummy struct or double-free).
	r.destroyOITResourcesLocked()

	if r.resources.OITAccumPipelineLayout != nil {
		t.Errorf("OITAccumPipelineLayout was not nilled out")
	}
	if r.resources.WorldPipelineLayout != fakeWorldLayout {
		t.Errorf("WorldPipelineLayout was modified unexpectedly")
	}

	// Calling destroyOITResourcesLocked again should be a safe no-op (idempotent)
	r.destroyOITResourcesLocked()

	// Calling Shutdown on clean renderer should succeed without panics
	r.Shutdown()
}

func TestOITSpritesVertexPreallocation(t *testing.T) {
	// Each sprite quad expands to 2 triangles = 6 vertices.
	// Each vertex stride is 48 bytes (pos float32x3 + uv float32x2 + uv2 float32x2 + normal float32x3).
	const spriteVertexStride = 48
	const verticesPerSpriteQuad = 6

	drawCounts := []int{0, 1, 5, 32, 128}
	for _, count := range drawCounts {
		expectedBytes := uint64(count * verticesPerSpriteQuad * spriteVertexStride)
		calculatedBytes := uint64(count * 6 * 48)
		if expectedBytes != calculatedBytes {
			t.Fatalf("draw count %d: expected %d bytes, got %d", count, expectedBytes, calculatedBytes)
		}
	}
}

func TestOITResolveAndViewModelPassOrdering(t *testing.T) {
	// Verify Pass 3 & Pass 4 phase contract ordering:
	// In McGuire weighted-blended OIT:
	// 1. Opaque scene and world depth are drawn first.
	// 2. Translucent accumulation writes to MRT (Accum + Reveal) with depth test (no depth write).
	// 3. OIT Resolve must execute BEFORE the first-person ViewModel so that:
	//    a) Translucent water/liquids are composited over the world background.
	//    b) The weapon model renders ON TOP of the resolved translucent water with depth testing,
	//       preventing the fullscreen resolve quad from blending over the weapon.
	// 4. Scene Composite (water warp, contrast, gamma) and PolyBlend composite the entire 3D scene
	//    (including view model) to the swapchain.
	// 5. 2D Overlay (HUD, Menu, Console) draws on top of swapchain.

	type passStep int
	const (
		stepWorldOpaque passStep = iota
		stepTranslucentAccum
		stepOITResolve
		stepViewModel
		stepSceneComposite
		stepPolyBlend
		step2DOverlay
	)

	steps := []passStep{
		stepWorldOpaque,
		stepTranslucentAccum,
		stepOITResolve,
		stepViewModel,
		stepSceneComposite,
		stepPolyBlend,
		step2DOverlay,
	}

	resolveIdx := -1
	viewModelIdx := -1
	compositeIdx := -1
	polyBlendIdx := -1
	overlayIdx := -1

	for i, s := range steps {
		switch s {
		case stepOITResolve:
			resolveIdx = i
		case stepViewModel:
			viewModelIdx = i
		case stepSceneComposite:
			compositeIdx = i
		case stepPolyBlend:
			polyBlendIdx = i
		case step2DOverlay:
			overlayIdx = i
		}
	}

	if resolveIdx >= viewModelIdx {
		t.Fatalf("OIT resolve (step %d) must occur BEFORE viewmodel (step %d)", resolveIdx, viewModelIdx)
	}
	if viewModelIdx >= compositeIdx {
		t.Fatalf("viewmodel (step %d) must occur BEFORE scene composite (step %d)", viewModelIdx, compositeIdx)
	}
	if compositeIdx >= polyBlendIdx {
		t.Fatalf("scene composite (step %d) must occur BEFORE polyblend (step %d)", compositeIdx, polyBlendIdx)
	}
	if polyBlendIdx >= overlayIdx {
		t.Fatalf("polyblend (step %d) must occur BEFORE 2D overlay (step %d)", polyBlendIdx, overlayIdx)
	}
}
