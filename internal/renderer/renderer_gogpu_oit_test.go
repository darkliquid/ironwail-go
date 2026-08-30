package renderer

import (
	"strings"
	"testing"

	"github.com/gogpu/gputypes"
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
	if !strings.Contains(epilogue, "@location(1) reveal: f32") {
		t.Errorf("epilogue missing @location(1) reveal: f32, got:\n%s", epilogue)
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
	if !strings.Contains(epilogue, "o.reveal = color.a") {
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
