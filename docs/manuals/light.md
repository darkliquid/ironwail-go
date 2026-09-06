# light Manual

light bakes the lightmaps of a compiled map. A lightmap is a small image per surface that stores how much light falls on it. The renderer multiplies each texture by its lightmap, so walls with no light appear dark.

Run qbsp and vis before light. The tool reads the finished BSP, casts rays from every light in the map, and writes the results back into the BSP.

```text
map.map -> qbsp -> map.bsp + map.prt -> vis -> map.bsp -> light -> map.bsp + map.lit
```

The tool also writes a colored sidecar file when you ask for it. That file is the `.lit` file, and it holds colored light for surfaces where white light is not enough.

## Getting light

Build it from the repository root:

```sh
go build -o light ./cmd/light
```

Or run `mise run build-light`.

## The shortest usable command

```sh
./light mymap.bsp
```

The tool reads the BSP in place, bakes the light, and overwrites the same file. A map with no `light` entities gets no lightmap, and the tool says so.

Work on a copy when you experiment:

```sh
./light -o mymap-lit.bsp mymap.bsp
```

## Command line

```text
light [-o out.bsp] [-lit] [-sun] [-bounce n] [-extra n] [-phong deg] map.bsp
```

| Flag | What it does |
| --- | --- |
| `-o <path>` | Writes the output BSP to this path. The default overwrites the input file. |
| `-lit` | Writes the colored `.lit` sidecar file next to the output. |
| `-sun` | Enables sunlight. See the sun section. |
| `-bounce <n>` | Sets the number of light bounces. Zero means direct light only. |
| `-extra <n>` | Sets the supersample factor. Two equals the classic `-extra`, four equals `-extra4`. |
| `-phong <deg>` | Enables phong shading up to this angle, in degrees. Zero disables it. |

## How lighting works

The map author places `light` entities in the editor. Each one has an origin and a brightness. The tool samples every lit surface at fixed points, called luxels. One luxel covers a square of 16 units.

For every luxel, the tool adds the light from every light entity. The formula is the light brightness divided by the distance squared, then scaled by the angle of the surface. The sum is clamped to a maximum of 255.

The brightness of a light uses the `light` key. A light can carry extra keys:

| Key | What it does |
| --- | --- |
| `origin` | The position of the light in the map. |
| `light` | The brightness of the light. |
| `style` | The light animation style, from 0 to 31. A separate lightmap is kept for each style. |
| `_color` | The color of the light, as three numbers for red, green, and blue. |

Each surface can carry several lightmaps, one per style. Styles make lights flicker or pulse in the game.

Darkness comes from shadows. Before a light adds to a luxel, the tool casts a ray from the light to the surface point. If geometry blocks the ray, the light does not reach that point.

## Sky faces

Surfaces with sky textures get no lightmap. The sky covers them instead. This is the classic Quake rule, and the tool follows it.

## Sunlight

Outdoor maps get a directional light from the sun. Enable it with `-sun`.

The sun direction comes from either a `sun` entity or from the worldspawn keys. The worldspawn keys are `sunlight` for brightness, `sun_mangle` for direction, and `sunlight_color` for color.

Sunlight lights every non-sky surface from one direction. It casts the same shadow rays as point lights.

## Bounce light

Bounce is the simplest radiosity. When a surface has light, it re-emits some of that light onto its neighbors. The tool repeats this for the number of bounces you ask for.

One bounce is the default for most maps. It brightens walls that no light can reach directly. Each bounced ray casts a shadow trace, like the direct rays do.

## Supersampling and phong

The `-extra` flag computes several samples inside every luxel and averages them. The result has softer edges between light and shadow. It costs more time.

The `-phong` flag smooths the lighting across flat surfaces. The tool blends the surface normals at shared corners instead of using one normal per surface. The angle you pass sets the limit: corners sharper than this stay hard-edged, corners softer than this blend. A common value is 89 degrees.

## The .lit sidecar

The BSP lightmap is monochrome: one brightness byte per luxel. Colored light looks wrong through it.

The `-lit` flag writes a sidecar file with the same light in color, as red, green, and blue triplets. The engine loads it when it finds it next to the BSP. Keep the sidecar file with the map when you ship a map that uses colored light.

## Output messages

The tool summarizes its work:

```text
light: wrote 41280 lightmap samples (1234 lit faces) to mymap.bsp
light: wrote mymap.lit
```

When no surface can be lit, it prints a hint instead:

```text
light: no lightable faces (add light entities)
```

## Input formats

The tool reads and patches two BSP versions. BSP29 with 20-byte faces, and BSP2 with 28-byte faces.

## Known limits

A few lighting features are not done yet. High dynamic range lightmaps, the `-lit2` format, and the lightgrid for entities are follow-up work, and the tool does not claim them.