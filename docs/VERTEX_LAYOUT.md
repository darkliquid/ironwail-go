# Vertex Layout — The "Three-Place Contract"

> **TL;DR**: When you add a new field to `WorldVertex`, you must update **three
> places** that must all agree on the byte layout: (1) the Go struct, (2) every
> function that packs vertices into bytes for GPU upload, and (3) the WebGPU
> pipeline's `VertexBufferLayout`. If any one disagrees, the GPU will read
> vertex data at the wrong offsets — textures, lighting, and geometry will appear
> scrambled in ways that are very hard to debug.

## Background: What is a vertex?

A "vertex" is a single point on a 3D surface. In addition to its position in
3D space, it carries extra information the GPU needs to draw that point
correctly — things like which part of a texture image to show, how much light
to apply, and which way the surface is facing.

The GPU doesn't understand Go structs. It reads vertices from a flat byte
buffer, one after another. The **stride** is the number of bytes between the
start of one vertex and the start of the next. Each piece of data within a
vertex sits at a fixed **offset** from the start of that vertex.

## The WorldVertex layout

Every world/brush/alias/sprite vertex in this engine uses the same 48-byte
layout:

```
Offset  Size  Field           Go type         WGSL type        Purpose
------  ----  -----           -------         ---------        -------
 0      12    Position        [3]float32      vec3<f32>        XYZ coordinates in world space
12       8    TexCoord        [2]float32      vec2<f32>        UV coordinates into the texture atlas
20       8    LightmapCoord   [2]float32      vec2<f32>        UV coordinates into the lightmap texture array
28      12    Normal          [3]float32      vec3<f32>        Surface direction (for lighting calculations)
40       4    LightmapLayer   float32         f32              Which layer (page) of the lightmap texture array to sample
44       4    MaterialID      uint32          u32              Which entry in the materials uniform array to use
--       --
48 bytes total (stride)
```

## The three places that must agree

### 1. The Go struct — `internal/renderer/world/types.go`

```go
type WorldVertex struct {
    Position      [3]float32   // 12 bytes
    TexCoord      [2]float32   //  8 bytes
    LightmapCoord [2]float32   //  8 bytes
    Normal        [3]float32   // 12 bytes
    LightmapLayer float32      //  4 bytes
    MaterialID    uint32       //  4 bytes
}
```

This is the authoritative definition. `unsafe.Sizeof(WorldVertex{})` is 48
because Go lays out struct fields in declaration order with natural alignment
(all fields here are 4-byte aligned, so there's no padding).

### 2. Every vertex-packing function

There are **four** functions that convert `WorldVertex` slices into flat byte
arrays for GPU upload. They must each write every field at the correct offset:

| Function | File | Used for |
|----------|------|----------|
| `CreateWorldVertexBuffer` | `world/gogpu/resources.go` | Static world geometry (BSP world) |
| `appendGoGPUWorldVertexBytes` | `renderer_gogpu_world_pipeline.go` | Brush entities (doors, platforms, triggers) |
| `VertexBytes` | `world/gogpu/buffer.go` | Sky brush entities |
| `AliasVertexBytesInto` | `world/gogpu/aliasbytes.go` | Alias models (enemies, items, weapons) |

### 3. The WebGPU pipeline vertex layout

Defined in `renderer_gogpu_world_pipelines.go` (and similar blocks in
`renderer_gogpu_world_alias.go` and `renderer_gogpu_world_sprite.go`). This tells the GPU how
to read the byte buffer:

```go
vertexBufferLayout := gputypes.VertexBufferLayout{
    ArrayStride: 48,  // <-- must match the byte size written by the packing functions
    Attributes: []gputypes.VertexAttribute{
        {Format: gputypes.VertexFormatFloat32x3, Offset: 0,  ShaderLocation: 0}, // Position
        {Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord
        {Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord
        {Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal
        {Format: gputypes.VertexFormatFloat32,   Offset: 40, ShaderLocation: 4}, // LightmapLayer
        {Format: gputypes.VertexFormatUint32,     Offset: 44, ShaderLocation: 5}, // MaterialID
    },
}
```

The WGSL shader's `VertexInput` struct must also match:

```wgsl
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) lightmapLayer: f32,
    @location(5) materialID: u32,
}
```

## What goes wrong when they disagree

### Symptom: Textures appear on the wrong surfaces

If the stride in a packing function is smaller than the pipeline's
`ArrayStride`, the GPU reads past the end of each vertex into the beginning
of the next. The `materialID` field (at offset 44) would read bytes from the
*next* vertex's `Position` field — a random-looking number that indexes into
a completely different texture in the atlas. Walls get water textures, floors
get sky textures, and the corruption shifts as the camera moves because
different vertices are processed in different orders.

### Symptom: Dark/coloured "shadow geometry" around moving brushes

This is the signature symptom of a stride mismatch in the **brush entity**
packing function specifically. Static world geometry renders correctly
(because it uses a different packing function), but brush entities — doors,
platforms, triggers, anything that moves or changes state — render with
garbage vertex data. The corrupted geometry appears as solid-coloured
triangles that cluster around where the brush entity is (or should be). The
brush itself may be invisible (its vertices are so scrambled the GPU culls or
misplaces them) while the surrounding area shows phantom triangles with
wrong textures.

### Symptom: Lighting artifacts and dark patches

If the `LightmapLayer` field is missing or at the wrong offset, the shader
samples the wrong page of the lightmap texture array. Faces that should be
bright get dark lighting, and vice versa. The artifacts may appear as
triangular dark patches that don't correspond to any actual geometry.

## How to diagnose a stride mismatch

1. **Check if the problem only affects moving brushes** (doors, platforms,
   triggers). If static world geometry looks fine but dynamic brushes don't,
   the bug is in `appendGoGPUWorldVertexBytes` or `VertexBytes`.

2. **Search for all places that hardcode a vertex stride**:
   ```
   grep -rn "Stride.*48\|stride.*48\|11 \* 4\|12 \* 4\|VertexStride" internal/renderer/
   ```
   Any `11 * 4` (44) is stale — it should be `12 * 4` (48).

3. **Compare the field writes in every packing function**. Each must write
   all six fields at the correct offsets (0, 12, 20, 28, 40, 44). A function
   that only writes four fields (Position, TexCoord, LightmapCoord, Normal)
   is missing `LightmapLayer` and `MaterialID`.

4. **Check the pipeline's `ArrayStride`** — it must equal the byte count
   written by the packing functions.

## Checklist: Adding a new field to WorldVertex

When adding a new field to `WorldVertex`:

- [ ] Add the field to the `WorldVertex` struct in `world/types.go`
- [ ] Add the field to the `VertexInput` struct in **every** WGSL shader
      (`renderer_gogpu_world_shaders.go`, `world/gogpu/shaders.go`)
- [ ] Update the `VertexOutput` struct and vertex shader's `vs_main` to pass
      the new field through to the fragment shader
- [ ] Update `ArrayStride` in **every** pipeline's `VertexBufferLayout`
      (`renderer_gogpu_world_pipelines.go`, `renderer_gogpu_world_alias.go`,
      `renderer_gogpu_world_sprite.go`)
- [ ] Update **all four** vertex-packing functions listed above
- [ ] Update the stride constant (`goGPUWorldVertexStrideBytes`,
      `worldVertexStrideBytes`, `aliasVertexStride`) in each file
- [ ] Run `go test -tags gogpu ./internal/renderer/...` — the SPIR-V
      validation test catches shader/pipeline mismatches but **not**
      packing-function mismatches
- [ ] Load a map with moving brush entities (e.g., `start` has a door and
      a platform) and verify they render correctly
