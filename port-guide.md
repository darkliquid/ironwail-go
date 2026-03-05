# ironwail-go Port Guide

Comparison of the original C Quake source (`/home/darkliquid/Projects/ironwail/Quake`) against
the in-progress Go port. Documents key differences, missing features, and implementation
details that **must not diverge** (especially file-format-related code).

---

## Quick Status Summary

| Subsystem | C Files | Go Package(s) | Status |
|-----------|---------|---------------|--------|
| BSP Format | `bspfile.h`, `gl_model.c` | `internal/bsp/` | ✓ Complete |
| PAK Format | `common.h` | `internal/fs/` | ✓ Complete |
| WAD Format | `wad.c` | `internal/image/` | ✓ Complete |
| MDL Format | `gl_model.h`, `gl_model.c` | `internal/model/mdl.go` | ✓ Complete |
| SPR Format | `r_sprite.c` | `internal/model/sprite.go` | ✓ Complete |
| Sound (WAV) | `snd_wave.c` | `internal/audio/wav.go` | ✓ Complete |
| QuakeC VM | `pr_exec.c`, `pr_cmds.c` | `internal/qc/` | ✓ Substantial |
| Server | `sv_*.c`, `world.c` | `internal/server/` | ✓ Substantial |
| Console | `console.c` | `internal/console/` | ✓ Substantial |
| Entity/World | `pr_edict.c`, `world.c` | `internal/server/{edict,world}.go` | ✓ Substantial |
| Physics | `sv_phys.c`, `sv_move.c` | `internal/server/physics*.go` | ✓ Substantial |
| Client | `cl_*.c` | `internal/client/` | ◐ Partial |
| Rendering | `gl_*.c`, `r_*.c` | `internal/renderer/` | ◐ M4 (2D done, 3D stub) |
| Network | `net_*.c` | `internal/net/` | ◐ Partial (protocol done, UDP stub) |
| Audio | `snd_*.c` | `internal/audio/` | ◐ Partial (mix framework; no device) |
| Menu | `menu.c`, `sbar.c` | `internal/menu/`, `internal/hud/` | ◐ M4 (render done, input pending) |
| Input | `in_sdl.c`, `keys.c` | `internal/input/` | ◐ Types only |
| Demo | `cl_demo.c` | `internal/client/demo.go` | ◐ Stub |
| Save/Load | (in `cl_main.c`, `common.c`) | — | ✗ Not started |
| 3D Rendering | `gl_rmain.c`, `r_*.c` | — | ✗ Not started |
| Audio Backend | `snd_sdl.c` | — | ✗ Stub only |
| Full Network | `net_udp.c`, `net_dgrm.c` | `internal/net/udp.go` | ✗ Stub only |

---

## File Formats — Must Match Exactly

These are binary formats read from game data files. Any deviation breaks compatibility.

### BSP (Map Format)

**Source**: `bspfile.h`
**Go**: `internal/bsp/bsp.go`, `internal/bsp/loader.go`

- **Magic version**: `29` (standard Quake), `0x32505342` (BSP2_2PSB), `0x42535032` (BSP2), `0x34` (Quake64)
- **15 lumps** (exact order):
  ```
  0: ENTITIES    1: PLANES      2: TEXTURES    3: VERTEXES
  4: VISIBILITY  5: NODES       6: TEXINFO     7: FACES
  8: LIGHTING    9: CLIPNODES   10: LEAFS      11: MARKSURFACES
  12: EDGES      13: SURFEDGES  14: MODELS
  ```
- **Content types** (leaf contents, must be exact for collision):
  ```
  EMPTY=-1, SOLID=-2, WATER=-3, SLIME=-4, LAVA=-5, SKY=-6
  ORIGIN=-7, CLIP=-8
  CURRENT_0=-9, CURRENT_90=-10, CURRENT_180=-11, CURRENT_270=-12
  CURRENT_UP=-13, CURRENT_DOWN=-14
  ```
- **Plane types**: PLANE_X/Y/Z (0-2, axial), PLANE_ANYX/Y/Z (3-5, non-axial)
- **BSP2 differences**: `dsnode_t` becomes `dl1node_t`/`dl2node_t` with int32 instead of int16 children;
  `dsleaf_t` becomes `dl1leaf_t`/`dl2leaf_t` with int32 marksurf counts.
- **MAX limits** must match for array sizing:
  ```
  MAX_MAP_HULLS=4, MAX_MAP_MODELS=256, MAX_MAP_ENTITIES=1024
  MAX_MAP_PLANES=32767, MAX_MAP_NODES=32767 (65535 for BSP2)
  MAX_MAP_CLIPNODES=32767 (65535 for BSP2), MAX_MAP_LEAFS=8192 (524288 for BSP2)
  MAX_MAP_VERTS=65535, MAX_MAP_FACES=65535, MAX_MAP_MARKSURFACES=65535
  MAX_MAP_TEXINFO=8192, MAX_MAP_EDGES=256000, MAX_MAP_SURFEDGES=512000
  MAX_MAP_MIPTEX=512, MAX_MAP_LIGHTING=0x300000, MAX_MAP_VISIBILITY=0x200000
  ```

**Status**: ✓ Go implementation has exact match including BSP2 variants.

---

### PAK (Package Archive)

**Source**: `common.h`, `common.c` (`COM_LoadPackFile`)
**Go**: `internal/fs/fs.go`

Binary layout:
```
Header (12 bytes):
  [0-3]  char[4]  "PACK"
  [4-7]  int32    directory offset
  [8-11] int32    directory length

File entry (64 bytes each):
  [0-55]  char[56]  filename (null-padded)
  [56-59] int32     file offset in pak
  [60-63] int32     file length
```

- `numFiles = DirLen / 64`
- Files loaded in order: `pak0.pak`, `pak1.pak`, etc. (later paks override earlier)
- Filesystem search order: direct file → pak files (in pak-load order)
- **Case-insensitive** path matching required on case-sensitive filesystems

**Status**: ✓ Correct binary layout, search semantics.

---

### WAD (Texture/Pic Archive)

**Source**: `wad.c`, `wad.h`
**Go**: `internal/image/wad.go`

Binary layout:
```
Header (12 bytes):
  [0-3]  char[4]  "WAD2"
  [4-7]  int32    num lumps
  [8-11] int32    info table offset

Lump entry (32 bytes each):
  [0-3]   int32  file position
  [4-7]   int32  disk size (compressed)
  [8-11]  int32  uncompressed size
  [12]    int8   type
  [13]    int8   compression (always 0)
  [14-15] int8   padding
  [16-31] char[16] name (uppercased, zero-padded)

QPic (inline):
  [0-3]  int32  width
  [4-7]  int32  height
  [8+]   byte[] pixels (width*height palette indices)
```

**Lump types**:
```
0x40 (64) = Palette / Lumpy
0x41 (65) = QTex (miptex in WAD)
0x42 (66) = QPic (2D picture, used for HUD/menu)
0x43 (67) = Sound
0x44 (68) = MipTex (world textures)
0x45 (69) = ConsolePic (console background)
```

**Name cleanup** (`W_CleanupName` / `CleanupName`):
- Uppercase input, copy max 15 chars, zero-pad to 16
- The C implementation does `toupper()` on each char

**Status**: ✓ Exact match. The lump type constants and name cleanup are identical.

---

### MDL (Alias Model)

**Source**: `gl_model.h`, `gl_model.c`
**Go**: `internal/model/mdl.go`

```
Magic: 0x4F504449 ("IDPO" in little-endian, i.e. bytes I,D,P,O)
Version: 6

Header (84 bytes):
  ident       int32    = 0x4F504449
  version     int32    = 6
  scale       vec3     (3x float32)
  scaleorigin vec3     (3x float32)
  boundrad    float32
  eyepos      vec3     (3x float32)
  numskins    int32
  skinwidth   int32    (must be multiple of 4)
  skinheight  int32
  numverts    int32
  numtris     int32
  numframes   int32
  synctype    int32    (0=sync, 1=rand)
  flags       int32
  size        float32  (average triangle area * numtris)

Skin: [type int32][data...]
  type=0: single skin: skinwidth*skinheight bytes of palette indices
  type=1: group skin: [numskins int32][intervals float32*n][frames...data]

STVert (per-vertex texture coords, 12 bytes each):
  onseam  int32   (nonzero if vertex is on seam)
  s       int32   (texture S coordinate)
  t       int32   (texture T coordinate)

Triangle (12 bytes each):
  facesfront int32  (nonzero = faces front)
  v[3]       int32  (indices into stvert array)

Frame:
  type=0 (single):
    [type int32=0]
    [bboxmin TriVertX]  (3 bytes + normal index)
    [bboxmax TriVertX]
    [name char[16]]
    [vertices TriVertX * numverts]

  type=1 (group):
    [type int32=1]
    [bboxmin TriVertX]
    [bboxmax TriVertX]
    [numframes int32]
    [intervals float32 * numframes]
    [frames DAliasFrame * numframes]

TriVertX (4 bytes):
  v[0] byte  (X * scale.x + origin.x)
  v[1] byte  (Y * scale.y + origin.y)
  v[2] byte  (Z * scale.z + origin.z)
  lightnormalindex byte  (index into anorms table, 162 normals)
```

**Critical**: Vertex decompression:
```
worldX = v[0] * header.Scale[0] + header.ScaleOrigin[0]
worldY = v[1] * header.Scale[1] + header.ScaleOrigin[1]
worldZ = v[2] * header.Scale[2] + header.ScaleOrigin[2]
```

**Seam handling**: If triangle `facesfront=0` and stvert `onseam!=0`, add `skinwidth/2` to S coord.

**Status**: ✓ Exact format match.

---

### SPR (Sprite Model)

**Source**: `r_sprite.c`, `r_part.c`
**Go**: `internal/model/sprite.go`

```
Magic: 0x50534449 ("IDSP" little-endian, bytes I,D,S,P)
Version: 1

Header (40 bytes):
  ident       int32 = 0x50534449
  version     int32 = 1
  type        int32  (orientation: 0=VP_PARALLEL_UPRIGHT, 1=FACING_UPRIGHT, 2=VP_PARALLEL, 3=ORIENTED, 4=VP_PARALLEL_ORIENTED)
  boundrad    float32
  width       int32  (max frame width)
  height      int32  (max frame height)
  numframes   int32
  beamlength  float32
  synctype    int32  (0=sync, 1=rand)

Frame entry:
  type  int32  (0=single, 1=group, 2=angled)

  Single frame (type=0):
    [origin[0] int32]  (left offset)
    [origin[1] int32]  (down offset)
    [width int32]
    [height int32]
    [pixels byte * width*height]

  Group frame (type=1):
    [numframes int32]
    [intervals float32 * numframes]
    [frames... (single frame structures)]
```

**Status**: ✓ Exact format match.

---

### Colormap (`gfx/colormap.lmp`)

**Source**: `gl_texmgr.c`, various
**Go**: `internal/draw/manager.go`, `internal/renderer/`

```
Size: 16385 bytes = 256 shades * 64 light levels + 1 fullbright sentinel
Layout: [shade 0, all light levels][shade 1, all light levels]...[fullbright marker]

Usage:
  index = colormap[paletteIndex * 64 + lightLevel]
  lightLevel 0 = darkest, 63 = brightest
  Last byte (index 16384) = 255 (fullbright sentinel)
```

Translucency: colormap entries map palette indices under lighting to the "best" palette entry
for that shade/brightness combination. The engine uses this for software rendering and
for correct palette-based transparency.

**Status**: Loaded from pak; used in software renderer and palette-indexed texture uploads.

---

### Palette (`gfx/palette.lmp`)

**Source**: Loaded in `gl_vidsdl.c` / `gl_texmgr.c`
**Go**: `internal/draw/manager.go`

```
Size: 768 bytes = 256 colors * 3 (R, G, B)
Each entry: [R byte][G byte][B byte]
No alpha in the palette file; index 255 is "transparent" by convention
```

**Special entries**:
- Index 255: transparent (not rendered in sprites/models)
- Indices 0-223: normal colors
- Indices 224-231: fullbright colors (not affected by lighting)
- Indices 232-255: more fullbright/special use

**Status**: ✓ Loaded correctly from pak.

---

## Network Protocol — Must Match Exactly

Quake is a live network game. Protocol deviations cause disconnects or corrupted game state.

### Protocol Versions

```go
PROTOCOL_NETQUAKE  = 15    // Standard id Quake
PROTOCOL_FITZQUAKE = 666   // FitzQuake extensions (alpha, fog, skybox)
PROTOCOL_RMQ      = 999    // RMQ (scale, etc.)
```

### Server→Client Message Types (`svc_*`)

```
0  SVCBad            (should never occur)
1  SVCNop
2  SVCDisconnect
3  SVCUpdateStat     [byte stat][int32 value]
4  SVCVersion        [int32 version]
5  SVCSetView        [int16 viewent]
6  SVCSound          [...]  (complex: flags, vol, atten, channel, origin, sound index)
7  SVCTime           [float32 time]
8  SVCPrint          [string]
9  SVCStuffText      [string]  (execute as console command on client)
10 SVCSetAngle       [angle3]  (3 × byte-encoded angles)
11 SVCServerInfo     [int32 proto][int32 spawncount][string mapname][string* models][string* sounds]
12 SVCLightStyle     [byte index][string pattern]
13 SVCUpdateName     [byte player][string name]
14 SVCUpdateFrags    [byte player][int16 frags]
15 SVCClientData     [flags...] (health, armor, ammo, items, weapon, etc.)
16 SVCStopSound      [int16 channel_entity]
17 SVCUpdateColors   [byte player][byte colors]
18 SVCParticle       [coord3 origin][byte3 dir][byte count][byte color]
19 SVCDamage         [byte armor][byte blood][coord3 origin]
20 SVCSpawnStatic    [byte modelindex][byte frame][byte colormap][byte skin][coord3 origin][angle3 angles]
21 SVCSpawnBinary    (unused)
22 SVCSpawnBaseline  [int16 entnum][byte modelindex][byte frame][byte colormap][byte skin][coord3 origin][angle3 angles]
23 SVCTempEntity     [byte type][...type-specific data...]
24 SVCSetPause       [byte paused]
25 SVCSignOnNum      [byte num]
26 SVCCenterPrint    [string]
27 SVCKillMonster
28 SVCFoundSecret
29 SVCSpawnStaticSound [coord3][byte sound][byte vol][byte atten]
30 SVCIntermission
31 SVCFinale         [string text]
32 SVCCDTrack        [byte track][byte loop]
33 SVCSellScreen
34 SVCCutScene       [string text]

-- FitzQuake extensions (protocol 666+) --
37 SVCSkyBox         [string skyname]
41 SVCFog            [byte density_byte][byte r][byte g][byte b][float32 time]
42 SVCSpawnBaseline2 [int16 entnum][int16 modelindex][int16 frame][byte colormap][byte skin][coord3 origin][angle3 angles][byte alpha]
43 SVCSpawnStatic2   (same fields as SpawnBaseline2 but for static ents)
44 SVCSpawnStaticSound2 [coord3][int16 sound][byte vol][byte atten]
```

### Client→Server Message Types (`clc_*`)

```
0  CLCBad
1  CLCNop
2  CLCDisconnect
3  CLCMove    [byte msec][angle3 viewangles][int16 forwardmove][int16 sidemove][int16 upmove][byte buttons][byte impulse]
4  CLCStringCmd [string]
```

### Entity Update Flags (`U_*`)

These are bitfield flags in SVCUpdate messages:

```
Byte 1:
  bit 0: U_MOREBITS    (another flags byte follows)
  bit 1: U_ORIGIN1     (X coordinate)
  bit 2: U_ORIGIN2     (Y coordinate)
  bit 3: U_ORIGIN3     (Z coordinate)
  bit 4: U_ANGLE2      (pitch)
  bit 5: U_STEP       (step-up, play land sound)
  bit 6: U_FRAME       (frame changed)
  bit 7: U_SIGNAL      (unused)

Byte 2 (if U_MOREBITS):
  bit 0: U_ANGLE1      (yaw)
  bit 1: U_ANGLE3      (roll)
  bit 2: U_MODEL       (model index)
  bit 3: U_COLORMAP    (colormap)
  bit 4: U_SKIN        (skin number)
  bit 5: U_EFFECTS     (effects flags)
  bit 6: U_LONGENTITY  (entity number is int16, not byte)
  bit 7: U_EXTEND1     (3rd flags byte for FitzQuake)

Byte 3 (FitzQuake, if U_EXTEND1):
  bit 0: U_ALPHA       (alpha value)
  bit 1: U_FRAME2      (high byte of frame)
  bit 2: U_MODEL2      (high byte of model)
  bit 3: U_LERPFINISH  (lerp end time)
  bit 4: U_SCALE       (scale, RMQ only)
  bit 5: U_UNUSED5
  bit 6: U_UNUSED6
  bit 7: U_EXTEND2     (4th flags byte)
```

### Client Data Flags (`SU_*`)

```
bit 0:  SU_VIEWHEIGHT  [byte]  (view height, default 22)
bit 1:  SU_IDEALPITCH  [byte]  (ideal pitch for swimming)
bit 2:  SU_PUNCH1      [byte signed]  (punch angle X)
bit 3:  SU_PUNCH2      [byte signed]  (punch angle Y)
bit 4:  SU_PUNCH3      [byte signed]  (punch angle Z)
bit 5:  SU_VELOCITY1   [byte signed]  (velocity X / 16)
bit 6:  SU_VELOCITY2   [byte signed]  (velocity Y / 16)
bit 7:  SU_VELOCITY3   [byte signed]  (velocity Z / 16)
bit 9:  SU_ITEMS       [int32]  (item bitflags)
bit 10: SU_ONGROUND    (no data)
bit 11: SU_INWATER     (no data)
bit 12: SU_WEAPONFRAME [byte]
bit 13: SU_ARMOR       [byte]
bit 14: SU_WEAPON      [byte]  (model index)
bit 15: SU_EXTEND1     (more flags)

FitzQuake extended (bit 15 set):
bit 16: SU_WEAPON2     (high byte of weapon model)
bit 17: SU_ARMOR2      (high byte? unused in practice)
bit 18: SU_AMMO2       (high byte? unused in practice)
bit 19: SU_SHELLS2     (high byte? unused in practice)
bit 20: SU_NAILS2
bit 21: SU_ROCKETS2
bit 22: SU_CELLS2
bit 23: SU_WEAPONFRAME2
bit 24: SU_WEAPONALPHA [byte]
bit 25-31: SU_EXTEND2
```

### Wire Encoding

```
Coord (16-bit fixed-point): int16 / 8.0  →  float32   (1/8 unit precision)
Angle (byte): byte * (360.0/256.0)       →  float32 degrees
Short: int16 little-endian
Long: int32 little-endian
Float: float32 little-endian
String: null-terminated bytes
```

**Alpha encoding** (FitzQuake):
```
ENTALPHA_DEFAULT = 0    // Use default (fully opaque)
ENTALPHA_ZERO    = 1    // Invisible
ENTALPHA_ONE     = 255  // Fully opaque

// encode: byte(clamp(alpha, 0, 1) * 254 + 1)
// decode: float32(a-1) / 254.0  (a==0 means default/1.0)
```

**Scale encoding** (RMQ):
```
ENTSCALE_DEFAULT = 16   // 1.0 scale
// encode: byte(scale * 16)
// decode: float32(a) / 16.0
```

---

## QuakeC VM — Must Match Exactly

The `.progs.dat` file is loaded and executed. The VM ABI must be exact.

### Progs File Format

```
Header (60 bytes):
  version       int32 = 6
  crc           int32 (checksum of header)
  ofs_statements int32 (file offset)
  numstatements  int32
  ofs_globaldefs int32
  numglobaldefs  int32
  ofs_fielddefs  int32
  numfielddefs   int32
  ofs_functions  int32
  numfunctions   int32
  ofs_strings    int32
  numstrings     int32
  ofs_globals    int32
  numglobals     int32
  entityfields   int32 (number of int32 words per entity)

Statement (8 bytes each):
  op   uint16
  a    int16
  b    int16
  c    int16

Def (8 bytes each, for globaldefs and fielddefs):
  type int16   (etype_t)
  ofs  int16   (offset into globals or entity fields)
  s_name int32 (offset into string table)

Function (36 bytes each):
  first_statement int32   (negative = builtin ID)
  parm_start      int32   (start of locals in globals)
  locals          int32   (number of local words)
  profile         int32   (not used at runtime, can be 0)
  s_name          int32   (string table offset)
  s_file          int32   (source filename, for debug)
  numparms        int32
  parm_size[8]    byte[8] (size of each parm in words)
```

### Reserved Global Offsets

```
OFS_NULL    = 0   (always 0)
OFS_RETURN  = 1   (return value, 3 words for vector)
OFS_PARM0   = 4   (parameter 0, 3 words)
OFS_PARM1   = 7
OFS_PARM2   = 10
OFS_PARM3   = 13
OFS_PARM4   = 16
OFS_PARM5   = 19
OFS_PARM6   = 22
OFS_PARM7   = 25
             28   (GlobalVars struct starts here)
```

### GlobalVars Layout (starting at offset 28)

These field names and offsets are fixed by the progs compiler; client code references them by index.

```
self           entity   (current executing entity)
other          entity   (touched entity in touch functions)
world          entity   (always entity 0 = world)
time           float    (server time)
frametime      float    (time since last frame)
force_retouch  float    (force bmodels to touchlinks next frame)
mapname        string
deathmatch     float    (0=coop, 1=dm, 2=deathmatch2)
coop           float
teamplay       float
serverflags    float    (serverflags global)
total_secrets  float
total_monsters float
found_secrets  float
killed_monsters float
parm1-parm16   float    (user parameters, e.g. skill level)
v_forward      vector   (view forward)
v_up           vector   (view up)
v_right        vector   (view right)
trace_allsolid float    (result from traceline)
trace_startsolid float
trace_fraction float
trace_endpos   vector
trace_plane_normal vector
trace_plane_dist float
trace_ent      entity
trace_inopen   float
trace_inwater  float
msg_entity     entity   (target for WriteByte etc.)
```

### Opcodes (all 56 must be implemented)

```
OP_DONE=0
OP_MUL_F=1, OP_MUL_V=2, OP_MUL_FV=3, OP_MUL_VF=4
OP_DIV_F=5
OP_ADD_F=6, OP_ADD_V=7
OP_SUB_F=8, OP_SUB_V=9
OP_EQ_F=10, OP_EQ_V=11, OP_EQ_S=12, OP_EQ_E=13, OP_EQ_FNC=14
OP_NE_F=15, OP_NE_V=16, OP_NE_S=17, OP_NE_E=18, OP_NE_FNC=19
OP_LE=20, OP_GE=21, OP_LT=22, OP_GT=23
OP_LOAD_F=24, OP_LOAD_V=25, OP_LOAD_S=26, OP_LOAD_ENT=27, OP_LOAD_FLD=28, OP_LOAD_FNC=29
OP_ADDRESS=30
OP_STORE_F=31, OP_STORE_V=32, OP_STORE_S=33, OP_STORE_ENT=34, OP_STORE_FLD=35, OP_STORE_FNC=36
OP_STOREP_F=37, OP_STOREP_V=38, OP_STOREP_S=39, OP_STOREP_ENT=40, OP_STOREP_FLD=41, OP_STOREP_FNC=42
OP_RETURN=43
OP_NOT_F=44, OP_NOT_V=45, OP_NOT_S=46, OP_NOT_ENT=47, OP_NOT_FNC=48
OP_IF=49, OP_IFNOT=50
OP_CALL0=51, OP_CALL1=52, OP_CALL2=53, OP_CALL3=54, OP_CALL4=55,
OP_CALL5=56, OP_CALL6=57, OP_CALL7=58, OP_CALL8=59
OP_STATE=60
OP_GOTO=61
OP_AND=62, OP_OR=63
OP_BITAND=64, OP_BITOR=65
```

### Entity Field Layout

The entity field layout is defined by the progs file's `fielddefs`. The C `entvars_t` structure
must match what `progs.dat` from standard Quake expects. Key fields:

```c
// From pr_comp.h / pr_edict.c
typedef struct {
    float   modelindex;           // model precache index
    vec3_t  absmin, absmax;      // world bounding box (auto-set by engine)
    float   ltime;               // local time (for bmodels)
    float   movetype;            // MOVETYPE_*
    float   solid;               // SOLID_*
    vec3_t  origin, oldorigin;
    vec3_t  velocity, avelocity; // linear and angular velocity
    vec3_t  angles, fixedangle;
    vec3_t  v_angle;             // player view angle
    float   idealpitch;
    string_t classname;
    string_t model;
    float   frame, skin, effects;
    vec3_t  mins, maxs, size;    // relative bounding box
    func_t  touch, use, think, blocked;
    float   nextthink;
    int     groundentity;        // entity on ground
    float   health, frags, weapon, ammo_shells, ammo_nails, ammo_rockets, ammo_cells;
    float   items;               // item flags
    float   takedamage;
    int     chain;               // for find() chains
    float   deadflag;
    vec3_t  view_ofs;
    float   button0, button1, button2;
    float   impulse;
    float   fixangle;
    vec3_t  v_angle;
    float   idealpitch;
    string_t netname;
    int     enemy;
    float   flags;               // FL_* flags
    float   colormap, team, max_health;
    float   teleport_time;
    float   armortype, armorvalue;
    float   waterlevel, watertype;
    float   ideal_yaw, yaw_speed;
    int     aiment;
    int     goalentity;
    float   spawnflags;
    string_t target, targetname;
    float   dmg_take, dmg_save;
    int     dmg_inflictor, owner;
    vec3_t  movedir;
    string_t message;
    float   sounds;
    string_t noise, noise1, noise2, noise3;
} entvars_t;
```

The Go `EntVars` struct in `internal/server/types.go` must match this exactly.

### Builtin Functions (engine-side, by number)

These are called from QuakeC as negative function indices:

```
#1  print(string)
#2  bprint(string)   — broadcast to all clients
#3  sprint(entity, string)  — send to specific client
#4  normalize(vector) vector
#5  vlen(vector) float
#6  vectoyaw(vector) float
#7  spawn() entity
#8  remove(entity)
#9  traceline(v1, v2, nomonsters, ent)
#10 checkclient() entity
#11 find(start, field, match) entity
#12 precache_sound(string) string
#13 precache_model(string) string
#14 stuffcmd(entity, string)  — send console command to client
#15 findradius(origin, radius) entity
#16 bprint2(string)
#17 sprint2(entity, string)
#18 dprint(string)   — debug print
#19 ftos(float) string
#20 vtos(vector) string
#21 coredump()
#22 traceon()
#23 traceoff()
#24 eprint(entity)   — debug entity print
#25 walkmove(yaw, dist) float
#26 (unused)
#27 droptofloor() float
#28 lightstyle(style, string)
#29 rint(float) float
#30 floor(float) float
#31 ceil(float) float
#32 (unused)
#33 checkbottom(entity) float
#34 pointcontents(vector) float
#35 (unused)
#36 fabs(float) float
#37 aim(entity, speed) vector
#38 cvar(string) float
#39 localcmd(string)
#40 nextent(entity) entity
#41 particle(origin, dir, color, count)
#42 ChangeYaw()
#43 (unused)
#44 vectoangles(vector) vector
#45 WriteByte(dest, value)
#46 WriteChar(dest, value)
#47 WriteShort(dest, value)
#48 WriteLong(dest, value)
#49 WriteCoord(dest, value)
#50 WriteAngle(dest, value)
#51 WriteString(dest, value)
#52 WriteEntity(dest, entity)
#59 movetogoal(dist)
#60 precache_file(string) string
#61 makestatic(entity)
#62 changelevel(string)
#63 (unused)
#64 cvar_set(string, string)
#65 centerprint(entity, string)
#66 ambientsound(origin, sample, vol, atten)
#67 precache_model2(string) string   — same as #13
#68 precache_sound2(string) string   — same as #12
#69 precache_file2(string) string    — same as #60
#70 setspawnparms(entity)
```

**Dest values for Write***: `MSG_BROADCAST=1`, `MSG_ONE=2`, `MSG_ALL=3`, `MSG_INIT=4`

---

## Physics Constants — Must Match Exactly

Quake's feel depends on these. Any deviation changes movement feel.

```
sv_gravity      = 800       (default, cvar-overridable)
sv_maxvelocity  = 2000      (velocity clamp, hardcoded)
sv_friction     = 4.0       (ground friction)
sv_edgefriction = 2.0       (extra friction near edges, sv_user.c)
sv_stopspeed    = 100.0     (speed at which friction fully stops entity)
sv_stepheight   = 18.0      (max step-up height, stepsize)
sv_maxspeed     = 320.0     (player max speed, cvar-overridable)
sv_accelerate   = 10.0      (acceleration from zero)
sv_airaccelerate = 0.7      (acceleration while airborne)
sv_wateraccelerate = 10.0   (acceleration in water)
sv_waterfriction = 1.0      (friction in water)
sv_nostep       = 0         (cvar; disable step-up)
pushepsilon     = 0.01      (epsilon for push collision)
stopepsilon     = 0.1       (epsilon for stopping)
```

### Coordinate / Angle Conventions

- Angles stored in degrees (float32), not radians
- Pitch = rotation around Y axis (look up/down), range [-70, 80] clamped for players
- Yaw = rotation around Z axis (turn left/right), range [-180, 180]
- Roll = rotation around X axis (lean), rarely used except for chasecam

- Velocity is in **Quake units per second**
- 1 Quake unit ≈ 1/32 of a foot (approximately)
- Player capsule: mins=(-16,-16,-24), maxs=(16,16,32), eyepos=(0,0,22)

### ClipVelocity (Critical for movement feel)

```c
// In sv_move.c / sv_phys.c
// overbounce = 1.0 for normal collision, 1.01 for grounding
vec3_t SV_ClipVelocity(vec3_t in, vec3_t normal, float overbounce) {
    float backoff = DotProduct(in, normal) * overbounce;
    vec3_t out;
    for (i = 0; i < 3; i++)
        out[i] = in[i] - normal[i] * backoff;
    return out;
}
```

---

## 2D Drawing Conventions — Coordinate System

This is important for correct HUD/menu rendering.

- **Virtual space**: 320 units wide, proportional height (usually 200 for 4:3)
- **Scale factor**: `screenPixelWidth / 320.0`
- `DrawPic(x, y, name)`: x/y are in **virtual 320-space** (scaled to pixels internally)
- `DrawFill(x, y, w, h, color)`: x/y/w/h are in **virtual 320-space** as well (scaled internally)
- `DrawCharacter(x, y, char)`: x/y are virtual; character cell is 8×8 virtual units

**HUD/menu layout** (virtual coordinates):
- Screen width: 320, height: 200 (virtual)
- Main menu: centered, at virtual (80, 68) approximately
- qplaque: at virtual (16, 4)
- mainmenu graphic: at virtual (72, 4)

---

## Known Quirks and Gotchas

### Entities

- **Entity 0** = world entity (special: static BSP brushes, always exists)
- **Entity 1** = first player (in single-player)
- Edict numbers start at 1 for progs purposes; 0 = null entity
- Free edicts are recycled after `sv_edgetimeout` seconds (default 5.0)
- Max edicts: 600 (standard Quake), up to 32768 (with protocol extensions)

### Texture and Model Precaching

- Models/sounds must be precached by QuakeC before use
- Precache lists sent to client in SVCServerInfo
- Model index 0 = none; valid models start at index 1
- Sound index 0 = none; valid sounds start at index 1
- `precache_model` / `precache_sound` in QC map to `sv.model_precache[]` / `sv.sound_precache[]`

### Angle Encoding on Wire

```
// 1 byte encodes 360/256 = 1.40625 degrees per unit
encode: byte(angle * 256.0 / 360.0)  →  int & 0xFF
decode: float(byte) * 360.0 / 256.0
```

### Coord Encoding on Wire

```
// 16-bit signed, 1/8 unit precision, range ±4096 units
encode: int16(coord * 8.0)
decode: float32(int16) / 8.0
```

### Light Style Strings

- Characters `'a'` through `'z'` = light levels 0-25 (0=off, 12='m'=normal)
- Strings looped continuously at 10Hz
- Default normal light = "m" (single char, constant)
- Animated lights use sequences like "mmnmmommommnonmmonqnmmo"

### Sound Attenuation

```
ATTN_NONE  = 0  (full volume everywhere)
ATTN_NORM  = 1  (normal world attenuation)
ATTN_IDLE  = 2  (quiet, short range)
ATTN_STATIC = 3 (ambient, very short range)
```

### Temp Entity Types (`TE_*`)

```
TE_SPIKE         = 0   [coord3 origin]  — spike impact
TE_SUPERSPIKE    = 1   [coord3 origin]  — super nailgun impact
TE_GUNSHOT       = 2   [coord3 origin]  — shotgun impact
TE_EXPLOSION     = 3   [coord3 origin]  — rocket explosion
TE_TAREXPLOSION  = 4   [coord3 origin]  — tarbaby explosion
TE_LIGHTNING1    = 5   [entity ent][coord3 start][coord3 end]
TE_LIGHTNING2    = 6   [entity ent][coord3 start][coord3 end]
TE_WIZSPIKE      = 7   [coord3 origin]
TE_KNIGHTSPIKE   = 8   [coord3 origin]
TE_LIGHTNING3    = 9   [entity ent][coord3 start][coord3 end]
TE_LAVASPLASH    = 10  [coord3 origin]
TE_TELEPORT      = 11  [coord3 origin]
TE_EXPLOSION2    = 12  [coord3 origin][byte color][byte count]
TE_BEAM          = 13  [entity ent][coord3 start][coord3 end]  — grappling hook
```

### Signon Sequence

Client must complete 4 signon phases before game starts:

```
Signon 1: client receives SVCSignOnNum(1) — received server info, loading assets
Signon 2: client receives SVCSignOnNum(2) — assets loaded, spawning
Signon 3: client receives SVCSignOnNum(3) — entity baselines received
Signon 4: client receives SVCSignOnNum(4) — connected, ready to play
```

Client sends `CLCStringCmd("new")`, `CLCStringCmd("spawn")`, `CLCStringCmd("begin")` during handshake.

---

## What's Left To Port (Priority Order)

### High Priority (for playable game)
1. **3D World Rendering** — BSP face extraction, lightmaps, brush geometry drawing
2. **Full Input Event Loop** — keyboard/mouse events from window → client commands
3. **Audio Backend** — SDL2 or other audio device for mixing output
4. **Network Sockets (UDP)** — real network connection to servers
5. **Demo Playback** — read `.dem` files for testing without a server

### Medium Priority (for complete single-player)
6. **Save/Load** — game state serialization (savegames)
7. **Full Console Rendering** — draw console text buffer on screen
8. **Dynamic Lighting** — `dlights` from explosions/torches
9. **Alias Model Rendering** — draw player and enemy models (MDL)
10. **Particle System** — explosions, blood, sparks

### Lower Priority (polish / multiplayer)
11. **CD Audio** — track selection from SVCCDTrack (or OGG replacements)
12. **Multiple Codec Support** — Vorbis, FLAC, MP3 for music
13. **Demo Recording** — write `.dem` files
14. **Screenshot/Video** — screenshot command
15. **FitzQuake Extensions** — fog, skybox, alpha, scale in full

### Quake-Specific Trivia That Affects Correctness

- `v_punch` (punch angles) decay at 4x/s in `V_CalcBlend`
- View bob uses `cl.onground` and speed; formula in `V_CalcViewRoll`
- `r_norefresh` cvar disables all rendering (useful for profiling)
- The HUD is drawn at virtual 320×200 regardless of window size
- `scr_crosshair` draws a "+" at screen center (character 43 from conchars)
- Quake's sky is a scrolling 128×128 texture (two layers at different speeds)
- Underwater view uses a ripple warp shader/effect on the framebuffer
- `r_waterwarp` controls this warp effect (0 = off, 1 = on)
