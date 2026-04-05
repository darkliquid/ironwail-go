package gogpu

const AliasVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct AliasUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: AliasUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.normal = input.normal;
    output.worldPosition = input.position;
    return output;
}
`

const AliasFragmentShaderWGSL = `
struct AliasUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: AliasUniforms;

@group(1) @binding(0)
var skinSampler: sampler;

@group(1) @binding(1)
var skinTexture: texture_2d<f32>;

@group(1) @binding(2)
var fullbrightTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let sampled = textureSample(skinTexture, skinSampler, input.texCoord);
    if (sampled.a < 0.01) {
        discard;
    }
    let fullbright = textureSample(fullbrightTexture, skinSampler, input.texCoord);
    let lit = sampled.rgb + fullbright.rgb * fullbright.a;
    let fogPosition = input.worldPosition - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
    return vec4<f32>(mix(uniforms.fogColor, lit, vec3<f32>(fog)), sampled.a * uniforms.alpha);
}
`

const SpriteVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct SpriteUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: SpriteUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.worldPosition = input.position;
    return output;
}
`

const SpriteFragmentShaderWGSL = `
struct SpriteUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: SpriteUniforms;

@group(1) @binding(0)
var spriteSampler: sampler;

@group(1) @binding(1)
var spriteTexture: texture_2d<f32>;

@group(1) @binding(2)
var unusedTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let sampled = textureSample(spriteTexture, spriteSampler, input.texCoord);
    if (sampled.a < 0.666) {
        discard;
    }
    let fogPosition = input.worldPosition - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
    let fogged = mix(uniforms.fogColor, sampled.rgb, vec3<f32>(fog));
    return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`

const DecalVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) color: vec4<f32>,
}

struct DecalUniforms {
    viewProjection: mat4x4<f32>,
    alpha: f32,
    _pad0: vec3<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: DecalUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.color = input.color;
    return output;
}
`

const DecalFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@group(1) @binding(0)
var decalSampler: sampler;

@group(1) @binding(1)
var decalTexture: texture_2d<f32>;

@group(1) @binding(2)
var unusedTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
     let sampled = textureSample(decalTexture, decalSampler, input.texCoord);
     if (sampled.a < 0.01) {
         discard;
     }
     let p = input.texCoord * 2.0 - vec2<f32>(1.0, 1.0);
     let d2 = dot(p, p);
     if (d2 > 1.0) {
         discard;
     }
     let edge = smoothstep(1.0, 0.7, d2);
     return vec4<f32>(input.color.rgb * sampled.rgb, input.color.a * edge * sampled.a);
}
`
