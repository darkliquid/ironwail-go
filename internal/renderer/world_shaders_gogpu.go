package renderer

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/wgpu"
)

const worldUniformsWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    litWater: f32,
    skyWindPhase: f32,
    _padding0: f32,
    skyWindDir: vec3<f32>,
    skyWindEnabled: f32,
}
`

// worldVertexShaderWGSL is the WGSL source for world vertex shader
const worldVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
}
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
    @location(6) clipPos: vec4<f32>,
}

struct MaterialData {
    atlasBounds: vec4<f32>,
    layer: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    
    let worldPos = vec4<f32>(input.position, 1.0);
    let clipPos = uniforms.viewProjection * worldPos;
    output.clipPosition = clipPos;
    
    output.texCoord = input.texCoord;
    output.lightmapCoord = input.lightmapCoord;
    output.worldPos = input.position;
    output.normal = input.normal;
    output.clipPos = clipPos;
    output.lightmapLayer = input.lightmapLayer;
    output.materialID = input.materialID;
    
    return output;
}
`

// worldFragmentShaderWGSL is the WGSL source for the GoGPU world fragment shader.
// Keep its lightmap/fullbright/fog math aligned with the canonical world-shader
// behavior so BSP world surfaces look the same across renderer paths.
func buildWorldFragmentShaderWGSL(planeNormalExpr string, alphaTest bool) string {
	alphaDiscard := ""
	lightExpr := "mix(sampled.rgb, sampled.rgb * totalLight * 2.0, sampled.a) + fullbright.rgb * fullbright.a"
	if alphaTest {
		alphaDiscard = `
	if (sampled.a < 0.666) {
		discard;
	}`
		lightExpr = "sampled.rgb * totalLight * 2.0 + fullbright.rgb * fullbright.a"
	}
	return fmt.Sprintf(`
%s
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
    @location(6) clipPos: vec4<f32>,
}

struct MaterialData {
    atlasBounds: vec4<f32>,
    layer: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
}

struct DynamicLight {
    originRadius: vec4<f32>,
    colorMinLight: vec4<f32>,
}

struct DynamicLights {
    count: u32,
    lights: array<DynamicLight, %d>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(0) @binding(1)
var<uniform> materials: array<MaterialData, 256>;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@group(3) @binding(0)
var worldFullbrightSampler: sampler;

@group(3) @binding(1)
var worldFullbrightTexture: texture_2d<f32>;

@group(4) @binding(0)
var lightClusters: texture_3d<u32>;

@group(4) @binding(1)
var<storage, read> dynamicLights: DynamicLights;

fn accumulateDynamicLights(worldPos: vec3<f32>, planeNormalRaw: vec3<f32>, clipPos: vec4<f32>) -> vec3<f32> {
    let normalLenSq = dot(planeNormalRaw, planeNormalRaw);
    if (normalLenSq <= 0.000001) {
        return vec3<f32>(0.0);
    }
    let planeNormal = planeNormalRaw * inverseSqrt(normalLenSq);
    let planeW = dot(worldPos, planeNormal);
    var dynamicLight = vec3<f32>(0.0);

    let ndc = clipPos.xy / clipPos.w;
    let cx = clamp(i32((ndc.x * 0.5 + 0.5) * 32.0), 0, 31);
    let cy = clamp(i32((ndc.y * 0.5 + 0.5) * 16.0), 0, 15);
    // zLogScale = 32.0 / 12.0 = 2.6666667
    // zLogBias = -2.6666667 * 2.0 = -5.3333333
    let cz = clamp(i32(floor(log2(clipPos.w) * 2.6666667 - 5.3333333)), 0, 31);
    
    let mask = textureLoad(lightClusters, vec3<i32>(cx, cy, cz), 0);
    
    var m0 = mask.r;
    while (m0 != 0u) {
        let bit = firstTrailingBit(m0);
        m0 = m0 & ~(1u << bit);
        let i = bit;
        let light = dynamicLights.lights[i];
        
        var rad = light.originRadius.w;
        let planeDist = dot(light.originRadius.xyz, planeNormal) - planeW;
        rad = rad - abs(planeDist);
        let minLight = light.colorMinLight.w;
        if (rad >= minLight) {
            let localPos = light.originRadius.xyz - planeNormal * planeDist;
            let surfaceDist = length(worldPos - localPos);
            dynamicLight += clamp((rad - minLight - surfaceDist) / 16.0, 0.0, 1.0) * max(0.0, rad - surfaceDist) / 256.0 * light.colorMinLight.xyz;
        }
    }
    
    var m1 = mask.g;
    while (m1 != 0u) {
        let bit = firstTrailingBit(m1);
        m1 = m1 & ~(1u << bit);
        let i = bit + 32u;
        let light = dynamicLights.lights[i];
        
        var rad = light.originRadius.w;
        let planeDist = dot(light.originRadius.xyz, planeNormal) - planeW;
        rad = rad - abs(planeDist);
        let minLight = light.colorMinLight.w;
        if (rad >= minLight) {
            let localPos = light.originRadius.xyz - planeNormal * planeDist;
            let surfaceDist = length(worldPos - localPos);
            dynamicLight += clamp((rad - minLight - surfaceDist) / 16.0, 0.0, 1.0) * max(0.0, rad - surfaceDist) / 256.0 * light.colorMinLight.xyz;
        }
    }

    return dynamicLight;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let mat = materials[input.materialID];
    let localUV = fract(input.texCoord);
    let atlasUV = vec2<f32>(localUV * mat.atlasBounds.zw + mat.atlasBounds.xy);
    let atlasVOffset = mat.layer;
    let sampled = textureSampleLevel(worldTexture, worldSampler, vec2<f32>(atlasUV.x, atlasUV.y + atlasVOffset), 0.0);
    let fullbright = textureSampleLevel(worldFullbrightTexture, worldFullbrightSampler, vec2<f32>(atlasUV.x, atlasUV.y + atlasVOffset), 0.0);
	%s
    var totalLight = textureSample(worldLightmap, worldLightmapSampler, vec2<f32>(input.lightmapCoord.x, input.lightmapCoord.y + input.lightmapLayer)).rgb;
    let dynamicLight = accumulateDynamicLights(input.worldPos, %s, input.clipPos);
    totalLight += max(min(dynamicLight, vec3<f32>(1.0) - totalLight), vec3<f32>(0.0));
    let lit = %s;
    let fogPosition = input.worldPos - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
	let fogged = mix(uniforms.fogColor, lit, fog);
	return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`, worldUniformsWGSL, gogpuWorldDynamicLightBufferMax, alphaDiscard, planeNormalExpr, lightExpr)
}

var worldFragmentShaderWGSL = buildWorldFragmentShaderWGSL("cross(dpdx(input.worldPos), dpdy(input.worldPos))", false)

var worldAlphaTestFragmentShaderWGSL = buildWorldFragmentShaderWGSL("input.normal", true)

// buildWorldDebugFragmentShaderWGSL builds a fragment shader that outputs
// diagnostic colors instead of sampled textures, so the operator can see
// material assignment problems without pixel comparisons.
//
// mode 1: encodes materialID as R=(id%256)/255, G=(id/256)/255, B=0
//   - Each material gets a distinct color. Out-of-range IDs (>256) will
//     show as varying green intensities instead of the expected low-R range.
// mode 2: encodes atlas layer as grayscale layer/maxLayers
//   - Faces on the wrong layer will have wrong brightness.
// mode 3: encodes atlas UV as R=u, G=v, B=layer/maxLayers
//   - Shows the atlas remapping. Wrong atlas bounds produce wrong colors.
// mode 4: samples the texture array at the material's layer (mat.layer)
//   - Shows the actual texture the shader would sample. If this looks
//     wrong compared to the atlas dump PNGs, the texture array data is
//     corrupted or the wrong layer is being sampled.
// mode 5: samples the texture array at layer 0 regardless of mat.layer
//   - Forces all faces to sample layer 0. If this looks the same as the
//     normal render, it means the layer value is being ignored or is
//     always 0. If this looks different (some textures correct), it
//     confirms the layer selection is the problem.
// mode 6: samples the texture array at layer 1 regardless of mat.layer
//   - Forces all faces to sample layer 1. Comparison with mode 5 reveals
//     whether multi-layer sampling works at all.
func buildWorldDebugFragmentShaderWGSL(mode int) string {
	var body string
	switch mode {
	case 1:
		body = `
    let mat = materials[input.materialID];
    let id = f32(input.materialID);
    return vec4<f32>(fract(id / 256.0), floor(id / 256.0) / 256.0, 0.0, 1.0);
`
	case 2:
		body = `
    let mat = materials[input.materialID];
    let maxLayer = 16.0;
    return vec4<f32>(vec3<f32>(mat.layer / maxLayer), 1.0);
`
	case 3:
		body = `
    let mat = materials[input.materialID];
    let localUV = fract(input.texCoord);
    let atlasUV = localUV * mat.atlasBounds.zw + mat.atlasBounds.xy;
    let maxLayer = 16.0;
    return vec4<f32>(atlasUV.x, atlasUV.y, mat.layer / maxLayer, 1.0);
`
	case 4:
		body = `
    let mat = materials[input.materialID];
    let localUV = fract(input.texCoord);
    let atlasUV = localUV * mat.atlasBounds.zw + mat.atlasBounds.xy;
    let sampled = textureSampleLevel(worldTexture, worldSampler, vec2<f32>(atlasUV.x, atlasUV.y + mat.layer), 0.0);
    return vec4<f32>(sampled.rgb, 1.0);
`
	case 5:
		body = `
    let mat = materials[input.materialID];
    let localUV = fract(input.texCoord);
    let atlasUV = localUV * mat.atlasBounds.zw + mat.atlasBounds.xy;
    let sampled = textureSampleLevel(worldTexture, worldSampler, atlasUV, 0.0);
    return vec4<f32>(sampled.rgb, 1.0);
`
	case 6:
		body = `
    let mat = materials[input.materialID];
    let localUV = fract(input.texCoord);
    let atlasUV = localUV * mat.atlasBounds.zw + mat.atlasBounds.xy;
    let sampled = textureSampleLevel(worldTexture, worldSampler, vec2<f32>(atlasUV.x, atlasUV.y + 1.0 / 3.0), 0.0);
    return vec4<f32>(sampled.rgb, 1.0);
`
	default:
		body = `
    return vec4<f32>(1.0, 0.0, 1.0, 1.0);
`
	}
	return fmt.Sprintf(`
%s
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
    @location(6) clipPos: vec4<f32>,
}

struct MaterialData {
    atlasBounds: vec4<f32>,
    layer: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(0) @binding(1)
var<uniform> materials: array<MaterialData, 256>;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@group(3) @binding(0)
var worldFullbrightSampler: sampler;

@group(3) @binding(1)
var worldFullbrightTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
%s
}
`, worldUniformsWGSL, body)
}

// worldDebugFragmentShaderWGSL is the debug viz fragment shader, rebuilt
// when the viz mode env var is read. nil when debug viz is off.
var worldDebugFragmentShaderWGSL string

// shouldUseDebugFragmentShader returns true when IRONWAIL_DEBUG_MATERIAL_VIZ
// is set to a nonzero mode, indicating that debug shader variants should be
// used instead of the normal texture-sampling fragment shaders.
func shouldUseDebugFragmentShader() bool {
	return debugMaterialVizMode() != 0
}

// initDebugShaders builds the debug fragment shader WGSL if the env var is
// set. Called from world_upload_gogpu.go during UploadWorld.
func initDebugShaders() {
	mode := debugMaterialVizMode()
	if mode != 0 {
		worldDebugFragmentShaderWGSL = buildWorldDebugFragmentShaderWGSL(mode)
		slog.Info("Debug material visualization shader enabled",
			"viz_mode", mode,
			"description", debugVizModeDescription(mode),
		)
	}
}

func debugVizModeDescription(mode int) string {
	switch mode {
	case 1:
		return "materialID as color (R=id%256, G=id/256)"
	case 2:
		return "atlas layer as grayscale"
	case 3:
		return "atlas UV as color (R=u, G=v, B=layer)"
	case 4:
		return "sample texture at mat.layer (shows actual sampled texture)"
	case 5:
		return "sample texture at layer 0 (forces layer 0 for all faces)"
	case 6:
		return "sample texture at layer 1 (forces layer 1 for all faces)"
	default:
		return "unknown"
	}
}

const worldSkyVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
}
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    let worldPos = vec4<f32>(input.position, 1.0);
    let clipPos = uniforms.viewProjection * worldPos;
    output.clipPosition = vec4<f32>(clipPos.xy, clipPos.w, clipPos.w);
    output.dir = vec3<f32>(
        input.position.x - uniforms.cameraOrigin.x,
        input.position.y - uniforms.cameraOrigin.y,
        (input.position.z - uniforms.cameraOrigin.z) * 3.0,
    );
    return output;
}
`

const worldSkyMaskVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
}
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    let clipPos = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.clipPosition = vec4<f32>(clipPos.xy, clipPos.w, clipPos.w);
    output.dir = vec3<f32>(
        input.position.x - uniforms.cameraOrigin.x,
        input.position.y - uniforms.cameraOrigin.y,
        (input.position.z - uniforms.cameraOrigin.z) * 3.0,
    );
    return output;
}
`

var worldTurbulentFragmentShaderWGSL = fmt.Sprintf(`
%s
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
    @location(6) clipPos: vec4<f32>,
}

struct MaterialData {
    atlasBounds: vec4<f32>,
    layer: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
}

struct DynamicLight {
    originRadius: vec4<f32>,
    colorMinLight: vec4<f32>,
}

struct DynamicLights {
    count: u32,
    lights: array<DynamicLight, %d>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(0) @binding(1)
var<uniform> materials: array<MaterialData, 256>;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@group(3) @binding(0)
var worldFullbrightSampler: sampler;

@group(3) @binding(1)
var worldFullbrightTexture: texture_2d<f32>;

@group(4) @binding(0)
var<storage, read> dynamicLights: DynamicLights;

fn accumulateDynamicLights(worldPos: vec3<f32>, planeNormalRaw: vec3<f32>) -> vec3<f32> {
    let normalLenSq = dot(planeNormalRaw, planeNormalRaw);
    if (normalLenSq <= 0.000001) {
        return vec3<f32>(0.0);
    }
    let planeNormal = planeNormalRaw * inverseSqrt(normalLenSq);
    let planeW = dot(worldPos, planeNormal);
    var dynamicLight = vec3<f32>(0.0);
    for (var i: u32 = 0u; i < dynamicLights.count; i = i + 1u) {
        let light = dynamicLights.lights[i];
        var rad = light.originRadius.w;
        let planeDist = dot(light.originRadius.xyz, planeNormal) - planeW;
        rad = rad - abs(planeDist);
        let minLight = light.colorMinLight.w;
        if (rad < minLight) {
            continue;
        }
        let localPos = light.originRadius.xyz - planeNormal * planeDist;
        let surfaceDist = length(worldPos - localPos);
        dynamicLight += clamp((rad - minLight - surfaceDist) / 16.0, 0.0, 1.0) * max(0.0, rad - surfaceDist) / 256.0 * light.colorMinLight.xyz;
    }
    return dynamicLight;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let mat = materials[input.materialID];
    let uv = fract(input.texCoord * 2.0 + 0.125 * sin(input.texCoord.yx * (3.14159265 * 2.0) + vec2<f32>(uniforms.time, uniforms.time)));
    let atlasUV = uv * mat.atlasBounds.zw + mat.atlasBounds.xy;
    let sampled = textureSampleLevel(worldTexture, worldSampler, vec2<f32>(atlasUV.x, atlasUV.y + mat.layer), 0.0);
    let fullbright = textureSampleLevel(worldFullbrightTexture, worldFullbrightSampler, vec2<f32>(atlasUV.x, atlasUV.y + mat.layer), 0.0);
    var totalLight = vec3<f32>(0.5);
    if (uniforms.litWater > 0.5) {
        totalLight = textureSample(worldLightmap, worldLightmapSampler, vec2<f32>(input.lightmapCoord.x, input.lightmapCoord.y + input.lightmapLayer)).rgb;
    }
    let dynamicLight = accumulateDynamicLights(input.worldPos, cross(dpdx(input.worldPos), dpdy(input.worldPos)));
    totalLight += max(min(dynamicLight, vec3<f32>(1.0) - totalLight), vec3<f32>(0.0));
    let lit = sampled.rgb * totalLight * 2.0 + fullbright.rgb * fullbright.a;
    let fogPosition = input.worldPos - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
    let fogged = mix(uniforms.fogColor, lit, fog);
    return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`, worldUniformsWGSL, gogpuWorldDynamicLightBufferMax)

const worldSkyFragmentShaderWGSL = `
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var skySolidSampler: sampler;

@group(1) @binding(1)
var skySolidTexture: texture_2d<f32>;

@group(2) @binding(0)
var skyAlphaSampler: sampler;

@group(2) @binding(1)
var skyAlphaTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let dir = normalize(input.dir);
    let uv = dir.xy * (189.0 / 64.0);
    var result = textureSample(skySolidTexture, skySolidSampler, uv + vec2<f32>(uniforms.time / 16.0, uniforms.time / 16.0));
    let layer = textureSample(skyAlphaTexture, skyAlphaSampler, uv + vec2<f32>(uniforms.time / 8.0, uniforms.time / 8.0));
    result = vec4<f32>(mix(result.rgb, layer.rgb, vec3<f32>(layer.a)), 1.0);
    result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), 1.0);
    return result;
}
`

const worldSkyExternalFaceFragmentShaderWGSL = `
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var skySampler: sampler;

@group(1) @binding(1)
var skyRT: texture_2d<f32>;

@group(1) @binding(2)
var skyBK: texture_2d<f32>;

@group(1) @binding(3)
var skyLF: texture_2d<f32>;

@group(1) @binding(4)
var skyFT: texture_2d<f32>;

@group(1) @binding(5)
var skyUP: texture_2d<f32>;

@group(1) @binding(6)
var skyDN: texture_2d<f32>;

fn sampleExternalSky(dir: vec3<f32>) -> vec4<f32> {
    let absDir = abs(dir);
    var ma: f32;
    var uv: vec2<f32>;
    if (absDir.x >= absDir.y && absDir.x >= absDir.z) {
        ma = max(absDir.x, 0.000001);
        if (dir.x > 0.0) {
            uv = vec2<f32>((-dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
            return textureSample(skyFT, skySampler, uv);
        }
        uv = vec2<f32>((dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
        return textureSample(skyBK, skySampler, uv);
    }
    if (absDir.y >= absDir.x && absDir.y >= absDir.z) {
        ma = max(absDir.y, 0.000001);
        if (dir.y > 0.0) {
            uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (dir.z / ma + 1.0) * 0.5);
            return textureSample(skyUP, skySampler, uv);
        }
        uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (-dir.z / ma + 1.0) * 0.5);
        return textureSample(skyDN, skySampler, uv);
    }
    ma = max(absDir.z, 0.000001);
    if (dir.z > 0.0) {
        uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
        return textureSample(skyRT, skySampler, uv);
    }
    uv = vec2<f32>((-dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
    return textureSample(skyLF, skySampler, uv);
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    // C Ironwail's sky-cubemap shader converts Quake world axes to cubemap
    // axes as (-Y, Z, X). The shared sky vertex shader's Z is stretched for
    // classic scrolling skies, so undo that stretch for external cubemap faces.
    let dir = normalize(vec3<f32>(-input.dir.y, input.dir.z / 3.0, input.dir.x) + vec3<f32>(0.0, 0.000001, 0.0));
    var result = sampleExternalSky(dir);
    if (uniforms.skyWindEnabled > 0.5) {
        let t1 = uniforms.skyWindPhase;
        let t2 = fract(t1) - 0.5;
        let blend = abs(t1 * 2.0);
        var layer1 = sampleExternalSky(dir + t1 * uniforms.skyWindDir);
        var layer2 = sampleExternalSky(dir + t2 * uniforms.skyWindDir);
        layer1 = vec4<f32>(layer1.rgb * layer1.a * (1.0 - blend), layer1.a * (1.0 - blend));
        layer2 = vec4<f32>(layer2.rgb * layer2.a * blend, layer2.a * blend);
        let combined = layer1 + layer2;
        result = vec4<f32>(result.rgb * (1.0 - combined.a) + combined.rgb, 1.0);
    }
    result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), result.a);
    return result;
}
`

// createWorldShaderModule creates a HAL shader module from WGSL source
func createWorldShaderModule(device *wgpu.Device, wgslSource string, label string) (*wgpu.ShaderModule, error) {
	shaderModule, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: label,
		WGSL:  wgslSource,
	})
	if err != nil {
		return nil, fmt.Errorf("create shader module: %w", err)
	}

	return shaderModule, nil
}
