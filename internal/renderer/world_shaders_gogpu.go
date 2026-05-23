package renderer

import (
	"fmt"

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
}
` + worldUniformsWGSL + `

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
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
    @location(4) clipPos: vec4<f32>,
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
	let sampled = textureSample(worldTexture, worldSampler, input.texCoord);
	%s
	var totalLight = textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb;
	let fullbright = textureSample(worldFullbrightTexture, worldFullbrightSampler, input.texCoord);
	let dynamicLight = accumulateDynamicLights(input.worldPos, %s);
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

const worldSkyVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
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
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
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
    @location(4) clipPos: vec4<f32>,
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
    let uv = input.texCoord * 2.0 + 0.125 * sin(input.texCoord.yx * (3.14159265 * 2.0) + vec2<f32>(uniforms.time, uniforms.time));
    let sampled = textureSample(worldTexture, worldSampler, uv);
    let fullbright = textureSample(worldFullbrightTexture, worldFullbrightSampler, uv);
    var totalLight = vec3<f32>(0.5);
    if (uniforms.litWater > 0.5) {
        totalLight = textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb;
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
