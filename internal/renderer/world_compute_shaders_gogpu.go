package renderer

import "fmt"

const worldClusterComputeShaderWGSL = `
struct ComputeUniforms {
    view: mat4x4<f32>,
    transposedProj: mat4x4<f32>,
    zLogScale: f32,
    zLogBias: f32,
    numLights: u32,
    _padding: vec3<f32>,
}

struct DynamicLight {
    originRadius: vec4<f32>,
    colorMinLight: vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: ComputeUniforms;
@group(0) @binding(1) var<storage, read> lights: array<DynamicLight>;
@group(0) @binding(2) var lightClusters: texture_storage_3d<rg32uint, write>;

const LIGHT_TILES_X: u32 = 32u;
const LIGHT_TILES_Y: u32 = 16u;
const LIGHT_TILES_Z: u32 = 32u;

var<workgroup> local_lights: array<vec4<f32>, 64>;

fn ExtractFrustumPlane(axis: u32, ndcval: f32, side: f32) -> vec4<f32> {
    let plane = uniforms.transposedProj[axis] - ndcval * uniforms.transposedProj[3];
    return inverseSqrt(dot(plane.xyz, plane.xyz)) * side * plane;
}

fn IntersectDepthPlane(dir: vec3<f32>, depth: f32) -> vec3<f32> {
    return vec3<f32>(depth, (depth / dir.x) * dir.yz);
}

@compute @workgroup_size(8, 8, 1)
fn cs_main(
    @builtin(global_invocation_id) gid: vec3<u32>,
    @builtin(local_invocation_index) lid: u32,
) {
    if (gid.x >= LIGHT_TILES_X || gid.y >= LIGHT_TILES_Y || gid.z >= LIGHT_TILES_Z) {
        return;
    }
    
    let numlights = uniforms.numLights;
    if (numlights == 0u) {
        textureStore(lightClusters, vec3<i32>(gid), vec4<u32>(0u, 0u, 0u, 0u));
        return;
    }
    
    let groupsize = 64u;
    let numpasses = (numlights + (groupsize - 1u)) / groupsize;
    
    for (var i: u32 = 0u; i < numpasses; i = i + 1u) {
        let index = lid + i * groupsize;
        if (index < numlights) {
            let l = lights[index];
            let viewPos = uniforms.view * vec4<f32>(l.originRadius.xyz, 1.0);
            local_lights[index] = vec4<f32>(viewPos.xyz, l.originRadius.w);
        }
    }
    
    workgroupBarrier();
    
    let TileSizeX = 2.0 / f32(LIGHT_TILES_X);
    let TileSizeY = 2.0 / f32(LIGHT_TILES_Y);
    let x0 = -1.0 + f32(gid.x) * TileSizeX;
    let y0 = -1.0 + f32(gid.y) * TileSizeY;
    let z0 = exp2((f32(gid.z) - uniforms.zLogBias) / uniforms.zLogScale);
    
    var cluster_planes: array<vec4<f32>, 6>;
    cluster_planes[0] = ExtractFrustumPlane(0u, x0,             -1.0);
    cluster_planes[1] = ExtractFrustumPlane(0u, x0 + TileSizeX,  1.0);
    cluster_planes[2] = ExtractFrustumPlane(1u, y0,             -1.0);
    cluster_planes[3] = ExtractFrustumPlane(1u, y0 + TileSizeY,  1.0);
    cluster_planes[4] = vec4<f32>(-1.0, 0.0, 0.0,  z0);
    cluster_planes[5] = vec4<f32>( 1.0, 0.0, 0.0, -z0 * exp2(1.0 / uniforms.zLogScale));

    let bl = cross(cluster_planes[2].xyz, cluster_planes[0].xyz);
    let tr = cross(cluster_planes[3].xyz, cluster_planes[1].xyz);
    let depth_near = cluster_planes[4].w;
    let depth_far = -cluster_planes[5].w;
    
    let p0 = IntersectDepthPlane(bl, depth_near);
    let p1 = IntersectDepthPlane(bl, depth_far);
    let p2 = IntersectDepthPlane(tr, depth_near);
    let p3 = IntersectDepthPlane(tr, depth_far);
    
    let min_yz = min(min(p0.yz, p1.yz), min(p2.yz, p3.yz));
    let max_yz = max(max(p0.yz, p1.yz), max(p2.yz, p3.yz));
    
    let cluster_mins = vec3<f32>(depth_near, min_yz.x, min_yz.y);
    let cluster_maxs = vec3<f32>(depth_far,  max_yz.x, max_yz.y);
    
    let cluster_center = (cluster_mins + cluster_maxs) * 0.5;
    let cluster_half_size = (cluster_maxs - cluster_mins) * 0.5;
    
    var clustermask = array<u32, 2>(0u, 0u);
    for (var i: u32 = 0u; i < numlights; i = i + 1u) {
        let l = local_lights[i];
        let delta = max(abs(l.xyz - cluster_center) - cluster_half_size, vec3<f32>(0.0));
        if (dot(delta, delta) < l.w * l.w) {
            clustermask[i >> 5u] = clustermask[i >> 5u] | (1u << (i & 31u));
        }
    }
    
    textureStore(lightClusters, vec3<i32>(gid), vec4<u32>(clustermask[0], clustermask[1], 0u, 0u));
}
`
