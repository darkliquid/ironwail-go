//go:build gogpu && !cgo
// +build gogpu,!cgo

package bugs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const particleVertexShaderWGSL = `
struct ParticleInstance {
    @location(0) position: vec3<f32>,
    @location(1) color: vec4<f32>,
}

struct ParticleUniforms {
    viewProjection: mat4x4<f32>,
    projScale: vec2<f32>,
    uvScale: f32,
    _pad0: f32,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    _pad1: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) color: vec4<f32>,
    @location(2) fogPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: ParticleUniforms;

@vertex
fn vs_main(instance: ParticleInstance, @builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var corners = array<vec2<f32>, 4>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(-1.0,  1.0),
        vec2<f32>( 1.0, -1.0),
        vec2<f32>( 1.0,  1.0),
    );
    let corner = corners[vertexIndex & 3u];
    var clipPosition = uniforms.viewProjection * vec4<f32>(instance.position, 1.0);
    let depthScale = max(1.0 + clipPosition.w * 0.004, 1.08);
    let clipOffset = uniforms.projScale * corner * depthScale;
    clipPosition = vec4<f32>(
        clipPosition.x + clipOffset.x,
        clipPosition.y + clipOffset.y,
        clipPosition.z,
        clipPosition.w,
    );

    var output: VertexOutput;
    output.clipPosition = clipPosition;
    output.uv = corner * uniforms.uvScale;
    output.color = instance.color;
    output.fogPosition = instance.position - uniforms.cameraOrigin;
    return output;
}
`

const sceneCompositeVertexShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var uvs = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.0),
        vec2<f32>(2.0, 0.0),
        vec2<f32>(0.0, 2.0),
    );

    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}
`

const sceneCompositeCrashFragmentShaderWGSL = `
struct SceneCompositeUniforms {
    uvScaleWarpTime: vec4<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var sceneSampler: sampler;

@group(0) @binding(1)
var sceneTexture: texture_2d<f32>;

@group(0) @binding(2)
var<uniform> uniforms: SceneCompositeUniforms;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    var uv = input.uv;
    let uvScale = uniforms.uvScaleWarpTime.xy;
    let warpAmp = uniforms.uvScaleWarpTime.z;
    let time = uniforms.uvScaleWarpTime.w;

    let textureSizeVec = vec2<f32>(textureDimensions(sceneTexture));
    let ddx = dpdx(uv.x) * textureSizeVec.x;
    let ddy = dpdy(uv.y) * textureSizeVec.y;

    if (warpAmp > 0.0) {
        let aspect = abs(ddy) / max(abs(ddx), 0.0001);
        let amp = vec2<f32>(warpAmp, warpAmp * aspect);
        uv = amp + uv * (1.0 - 2.0 * amp);
        uv += amp * sin(vec2<f32>(uv.y / max(aspect, 0.0001), uv.x) * (3.14159265 * 8.0) + time);
    }

    return textureSample(sceneTexture, sceneSampler, uv * uvScale);
}
`

const sceneCompositeUniformBufferSize = 16

func TestParticleShaderNagaCompileRegression(t *testing.T) {
	output, err := runStandaloneGoGPURepro(t, "particle")
	if err != nil {
		t.Fatalf("particle repro subprocess failed: %v\n%s", err, output)
	}
	if !bytes.Contains(output, []byte("particle shader compiled")) {
		t.Fatalf("standalone repro output missing particle shader success marker:\n%s", output)
	}
	t.Log(string(output))
}

func TestSceneCompositePipelineCrashRepro(t *testing.T) {
	output, err := runStandaloneGoGPURepro(t, "scene-composite-crash")
	if err == nil {
		t.Fatalf("expected standalone scene-composite repro to crash, got success:\n%s", output)
	}
	if !bytes.Contains(output, []byte("about to create standalone scene composite repro pipeline")) {
		t.Fatalf("standalone repro output missing scene composite pipeline marker:\n%s", output)
	}
	if !bytes.Contains(output, []byte("SIGSEGV")) && !bytes.Contains(output, []byte("segmentation violation")) {
		t.Fatalf("standalone repro output missing expected crash marker:\n%s", output)
	}
	t.Log(string(output))
	t.Fail()
}

func TestStandaloneGoGPUReproHelper(t *testing.T) {
	mode := os.Getenv("IW_STANDALONE_GOGPU_REPRO")
	if mode == "" {
		t.Skip("helper process only")
	}
	os.Exit(runStandaloneGoGPUReproProcess(mode))
}

func runStandaloneGoGPURepro(t *testing.T, mode string) ([]byte, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStandaloneGoGPUReproHelper$")
	cmd.Env = append(os.Environ(), "IW_STANDALONE_GOGPU_REPRO="+mode)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("standalone repro %q timed out:\n%s", mode, output)
	}
	return output, err
}

func runStandaloneGoGPUReproProcess(mode string) int {
	cfg := gogpu.DefaultConfig()
	cfg.Title = "Standalone GoGPU Repro"
	cfg.Width = 64
	cfg.Height = 64
	cfg = cfg.WithContinuousRender(true)
	cfg = cfg.WithBackend(gogpu.BackendGo)

	app := gogpu.NewApp(cfg)
	done := false

	app.OnDraw(func(*gogpu.Context) {
		if done {
			return
		}
		provider := app.DeviceProvider()
		if provider == nil {
			return
		}
		done = true

		device := provider.Device()
		switch mode {
		case "particle":
			_, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
				Label: "Standalone Particle Vertex Shader Repro",
				WGSL:  particleVertexShaderWGSL,
			})
			if err == nil {
				fmt.Fprintln(os.Stdout, "particle shader compiled")
				os.Exit(0)
			} else {
				fmt.Fprintln(os.Stdout, "particle shader error:", err)
				os.Exit(2)
			}
		case "scene-composite-crash":
			vertexShader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
				Label: "Standalone Scene Composite Vertex Shader Repro",
				WGSL:  sceneCompositeVertexShaderWGSL,
			})
			if err != nil {
				fmt.Fprintln(os.Stdout, "vertex shader error:", err)
				os.Exit(3)
			}
			defer vertexShader.Release()

			fragmentShader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
				Label: "Standalone Scene Composite Fragment Shader Crash Repro",
				WGSL:  sceneCompositeCrashFragmentShaderWGSL,
			})
			if err != nil {
				fmt.Fprintln(os.Stdout, "fragment shader error:", err)
				os.Exit(4)
			}
			defer fragmentShader.Release()

			bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
				Label: "Standalone Scene Composite Repro BGL",
				Entries: []gputypes.BindGroupLayoutEntry{
					{
						Binding:    0,
						Visibility: gputypes.ShaderStageFragment,
						Sampler: &gputypes.SamplerBindingLayout{
							Type: gputypes.SamplerBindingTypeFiltering,
						},
					},
					{
						Binding:    1,
						Visibility: gputypes.ShaderStageFragment,
						Texture: &gputypes.TextureBindingLayout{
							SampleType:    gputypes.TextureSampleTypeFloat,
							ViewDimension: gputypes.TextureViewDimension2D,
							Multisampled:  false,
						},
					},
					{
						Binding:    2,
						Visibility: gputypes.ShaderStageFragment,
						Buffer: &gputypes.BufferBindingLayout{
							Type:             gputypes.BufferBindingTypeUniform,
							HasDynamicOffset: false,
							MinBindingSize:   sceneCompositeUniformBufferSize,
						},
					},
				},
			})
			if err != nil {
				fmt.Fprintln(os.Stdout, "bind group layout error:", err)
				os.Exit(5)
			}
			defer bindGroupLayout.Release()

			pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
				Label:            "Standalone Scene Composite Repro Pipeline Layout",
				BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
			})
			if err != nil {
				fmt.Fprintln(os.Stdout, "pipeline layout error:", err)
				os.Exit(6)
			}
			defer pipelineLayout.Release()

			fmt.Fprintln(os.Stdout, "about to create standalone scene composite repro pipeline")
			_, err = device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
				Label:  "Standalone Scene Composite Crash Repro Pipeline",
				Layout: pipelineLayout,
				Vertex: wgpu.VertexState{
					Module:     vertexShader,
					EntryPoint: "vs_main",
				},
				Primitive: gputypes.PrimitiveState{
					Topology:  gputypes.PrimitiveTopologyTriangleList,
					FrontFace: gputypes.FrontFaceCCW,
					CullMode:  gputypes.CullModeNone,
				},
				Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
				Fragment: &wgpu.FragmentState{
					Module:     fragmentShader,
					EntryPoint: "fs_main",
					Targets: []gputypes.ColorTargetState{{
						Format:    provider.SurfaceFormat(),
						WriteMask: gputypes.ColorWriteMaskAll,
					}},
				},
			})
			if err != nil {
				fmt.Fprintln(os.Stdout, "CreateRenderPipeline returned error:", err)
				os.Exit(7)
			} else {
				fmt.Fprintln(os.Stdout, "CreateRenderPipeline returned success")
				os.Exit(8)
			}
		default:
			fmt.Fprintln(os.Stdout, "unknown repro mode:", mode)
			os.Exit(10)
		}
	})

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stdout, "app run error:", err)
		return 11
	}
	return 12
}
