package gogpu

const AliasVertexShaderWGSL = `
struct VertexInput {
    @location(0) vertexIndex: f32,
    @location(1) texCoord: vec2<f32>,
}

struct AliasUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct AliasInstance {
    frame1: f32,
    frame2: f32,
    blend: f32,
    entityScale: f32,
    scale: vec3<f32>,
    scaleOrigin: vec3<f32>,
    origin: vec3<f32>,
    angles: vec3<f32>,
    fullAngles: f32,
    numVerts: f32,
    _pad: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: AliasUniforms;

@group(2) @binding(0)
var<uniform> instance: AliasInstance;

@group(2) @binding(1)
var<storage, read> poseData: array<u32>;

let normals = array<vec3<f32>, 162>(
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.525731, 0.000000, 0.850651),
    vec3<f32>(-0.442863, -0.238856, 0.864188),
    vec3<f32>(-0.295242, 0.000000, 0.955423),
    vec3<f32>(-0.309017, 0.500000, 0.809017),
    vec3<f32>(-0.162460, 0.262866, 0.951056),
    vec3<f32>(0.000000, 0.000000, 1.000000),
    vec3<f32>(0.000000, 0.850651, 0.525731),
    vec3<f32>(-0.147621, 0.716567, 0.681718),
    vec3<f32>(0.147621, 0.716567, 0.681718),
    vec3<f32>(0.000000, 0.525731, 0.850651),
    vec3<f32>(0.309017, 0.500000, 0.809017),
    vec3<f32>(0.525731, 0.000000, 0.850651),
    vec3<f32>(0.295242, 0.000000, 0.955423),
    vec3<f32>(0.442863, 0.238856, 0.864188),
    vec3<f32>(-0.309017, -0.500000, 0.809017),
    vec3<f32>(-0.162460, -0.262866, 0.951056),
    vec3<f32>(0.000000, -0.850651, 0.525731),
    vec3<f32>(0.147621, -0.716567, 0.681718),
    vec3<f32>(0.000000, -0.525731, 0.850651),
    vec3<f32>(0.309017, -0.500000, 0.809017),
    vec3<f32>(0.442863, -0.238856, 0.864188),
);

fn rotateYaw(v: vec3<f32>, yawDegrees: f32) -> vec3<f32> {
    if (yawDegrees == 0.0) { return v; }
    let yaw = 3.14159265358979 * yawDegrees / 180.0;
    let s = sin(yaw);
    let c = cos(yaw);
    return vec3<f32>(v.x * c - v.y * s, v.x * s + v.y * c, v.z);
}

fn rotatePitch(v: vec3<f32>, pitchDegrees: f32) -> vec3<f32> {
    if (pitchDegrees == 0.0) { return v; }
    let pitch = 3.14159265358979 * pitchDegrees / 180.0;
    let s = sin(pitch);
    let c = cos(pitch);
    return vec3<f32>(v.x * c - v.z * s, v.y, v.x * s + v.z * c);
}

fn rotateRoll(v: vec3<f32>, rollDegrees: f32) -> vec3<f32> {
    if (rollDegrees == 0.0) { return v; }
    let roll = 3.14159265358979 * rollDegrees / 180.0;
    let s = sin(roll);
    let c = cos(roll);
    return vec3<f32>(v.x, v.y * c - v.z * s, v.y * s + v.z * c);
}

fn rotateAngles(v: vec3<f32>, angles: vec3<f32>) -> vec3<f32> {
    var r = rotateRoll(v, angles.z);
    r = rotatePitch(r, angles.x);
    r = rotateYaw(r, angles.y);
    return r;
}

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;

    let vIdx = u32(input.vertexIndex);
    let nv = u32(instance.numVerts);
    let f1 = u32(instance.frame1);
    let f2 = u32(instance.frame2);

    let pose1Word = poseData[f1 * nv + vIdx];
    let pose2Word = poseData[f2 * nv + vIdx];

    let v1x = f32((pose1Word >> 0u) & 0xFFu);
    let v1y = f32((pose1Word >> 8u) & 0xFFu);
    let v1z = f32((pose1Word >> 16u) & 0xFFu);
    let v2x = f32((pose2Word >> 0u) & 0xFFu);
    let v2y = f32((pose2Word >> 8u) & 0xFFu);
    let v2z = f32((pose2Word >> 16u) & 0xFFu);
    let normalIdx = (pose1Word >> 24u) & 0xFFu;

    var pos1 = instance.scaleOrigin + vec3<f32>(v1x, v1y, v1z) * instance.scale;
    var pos2 = instance.scaleOrigin + vec3<f32>(v2x, v2y, v2z) * instance.scale;
    var pos = mix(pos1, pos2, instance.blend);
    pos = pos * instance.entityScale;

    var nrm = normals[normalIdx];
    if (instance.fullAngles > 0.5) {
        pos = rotateAngles(pos, instance.angles);
        nrm = rotateAngles(nrm, instance.angles);
    } else {
        pos = rotateYaw(pos, instance.angles.y);
        nrm = rotateYaw(nrm, instance.angles.y);
    }

    pos = pos + instance.origin;

    output.clipPosition = uniforms.viewProjection * vec4<f32>(pos, 1.0);
    output.texCoord = input.texCoord;
    output.normal = nrm;
    output.worldPosition = pos;
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
    if (sampled.a < 0.666) {
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
     if (sampled.a < 0.666) {
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
