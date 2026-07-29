# Chapter 1: How the Quake Engine Actually Works

Before we can understand what `ironwail-go` changes, we need to understand
what it is changing. This chapter is an architectural tour of the original
Quake engine — not the software renderer's scanline rasterizer, not the
Win32 platform glue, but the *core engine*: the simulation, the scripting
VM, the world representation, the networking, and the rendering contract.
It draws on two sources: Zachary Hickman's academic analysis of the Quake
engine, written for Northeastern University [#Hickman](#hickman), and the
formal behavior specification in `docs/QUAKE_SPECIFICATION.md`
[#Spec](#spec), cross-referenced against the C source in `ironwail/Quake/`.

---

## The game loop: time is real, and the server owns truth

Hickman opens with the observation that *"defining what sort of time the
game loop is based on [...] is critical, as all sub-processes of the engine
are related to the selection and definition of time."* [#Hickman](#hickman)
Quake's answer is unambiguous: **time is real time.** The engine measures
wall-clock seconds via `Sys_DoubleTime()`, computes a delta, and passes it
to `Host_Frame`. Here is the C Ironwail main loop, simplified from
`host.c:1199`:

```c
void _Host_Frame (double time)
{
    // decide the simulation time
    accumtime += host_netinterval ? CLAMP(0.0, time, 0.2) : 0.0;
    Host_AdvanceTime (time);

    // input
    Sys_SendKeyEvents ();
    IN_Commands ();
    Host_GetConsoleCommands ();

    // console commands
    Cbuf_Execute ();

    // server runs at a fixed rate, separate from the renderer
    if (accumtime >= host_netinterval)
    {
        CL_SendCmd ();
        if (sv.active) {
            PR_SwitchQCVM(&sv.qcvm);
            Host_ServerFrame ();
            PR_SwitchQCVM(NULL);
        }
        accumtime -= host_netinterval;
    }

    // client reads results from server
    if (cls.state == ca_connected)
        CL_ReadFromServer ();

    // prediction
    CL_SetUpPlayerPrediction(false);
    CL_PredictMove;
    CL_SetUpPlayerPrediction(true);

    // render
    SCR_UpdateScreen ();

    host_framecount++;
}
```

Hickman summarized this as three phases: *"Network, Prediction/Collision,
and Rendition."* [#Hickman](#hickman) But there is a subtlety he did not
emphasize: **the server and the renderer run at different rates.** The
renderer can run at `host_maxfps` (default 250 in Ironwail), but the server
ticks at a fixed `host_netinterval` (72 Hz for network play). The
`accumtime` variable buffers real time and releases it in fixed-size chunks
to the server, so the simulation is deterministic regardless of frame rate.
This is the same pattern the Go port preserves — the `internal/host`
package's `Frame()` method in `frame.go` implements the same timing logic,
and the `FrameCallbacks` interface (`GetEvents`, `ProcessConsoleCommands`,
`ProcessServer`, `ProcessClient`, `UpdateScreen`, `UpdateAudio`) mirrors
the C call sequence. [#HostDocs](#hostdocs)

---

## Resource management: the Hunk, the Zone, and the Cache

Quake manages memory through three mechanisms, each with a different
lifecycle. Hickman covers all three [#Hickman](#hickman); the C code lives
in `common.c`.

### The Hunk

The Hunk is a single large block of memory allocated at startup. It is
the engine's general-purpose arena — all map geometry, models, textures,
and most runtime structures live here. The Hunk provides four operations:

- `Hunk_AllocName(size, name)` — bump-allocate from the low end.
- `Hunk_Alloc()` — shorthand for `Hunk_AllocName` with a generic name.
- `Hunk_HighAllocName(size, name)` — allocate from the *high* end (for
  short-lived large allocations that can be freed together).
- `Hunk_TempAlloc(size)` — allocate from the high end, freed on the next
  temp alloc. Used for transient buffers.

The Hunk is never freed piecemeal. When a map changes, the entire Hunk is
wiped and rebuilt. This is brutally simple and avoids fragmentation — you
allocate linearly, then nuke everything and start over.

### The Zone

The Zone is a smaller heap allocator for objects with individual lifetimes:
strings, small structures, configuration data. It supports `Z_Malloc`,
`Z_Free`, and `Z_CheckHeap`. The Zone is a traditional free-list allocator
inside the Hunk.

### The Cache

The Cache is for resources that can be evicted under memory pressure:
sounds, model data, textures. It supports `Cache_Alloc`, `Cache_Free`,
`Cache_Flush`, and `Cache_TryAlloc`. When the cache is full and a new
resource is needed, the least-recently-used cached resource is evicted
automatically.

The key architectural point: **Quake's memory model assumes the engine owns
a single contiguous block of memory and manages it by hand with pointer
arithmetic.** There is no GC, no per-object free, no safety. The Hunk is
reset by changing a single offset. This is fast and deterministic, and it
is the first thing the Go port has to replace — Chapter 2 covers that
collision.

---

## The client-server architecture

This is the single most important architectural fact about Quake, and the
`ironwail-go` learning guide states it plainly:

> The most important thing to understand about the Quake engine (and thus
> Ironwail Go) is that it is fundamentally a **client-server application**,
> even when playing single-player.
> [#LearningGuide](#learningguide)

### The server is the source of truth

The server runs the physics, executes the QuakeC game logic, and decides
what happens in the world. In C, `SV_Physics` in `sv_phys.c:1226` is the
heart of the server. Each frame it:

1. Calls the QuakeC `StartFrame` function to let game logic know a new
   frame has begun.
2. Iterates every entity (`for (i=0; i<entity_cap; i++, ent=NEXT_EDICT(ent))`)
   and dispatches physics based on `movetype`:

   ```c
   if (i > 0 && i <= svs.maxclients)
       SV_Physics_Client (ent, i);
   else if (ent->v.movetype == MOVETYPE_PUSH)
       SV_Physics_Pusher (ent);
   else if (ent->v.movetype == MOVETYPE_NONE)
       SV_Physics_None (ent);
   else if (ent->v.movetype == MOVETYPE_NOCLIP)
       SV_Physics_Noclip (ent);
   else if (ent->v.movetype == MOVETYPE_STEP)
       SV_Physics_Step (ent);
   else if (ent->v.movetype == MOVETYPE_TOSS
       || ent->v.movetype == MOVETYPE_GIB
       || ent->v.movetype == MOVETYPE_BOUNCE
       || ent->v.movetype == MOVETYPE_FLY
       || ent->v.movetype == MOVETYPE_FLYMISSILE)
       SV_Physics_Toss (ent);
   ```

3. Advances `qcvm->time` by `host_frametime`.

The movetypes are:

| Movetype | Physics handler | Used for |
| --- | --- | --- |
| `MOVETYPE_PUSH` | `SV_Physics_Pusher` | Doors, platforms, trains (entities that push others) |
| `MOVETYPE_NONE` | `SV_Physics_None` | Stationary entities |
| `MOVETYPE_NOCLIP` | `SV_Physics_Noclip` | Entities that ignore collision |
| `MOVETYPE_STEP` | `SV_Physics_Step` | Monsters (step up stairs, gravity) |
| `MOVETYPE_TOSS`/`GIB`/`BOUNCE`/`FLY`/`FLYMISSILE` | `SV_Physics_Toss` | Projectiles, gibs, flying entities |
| (clients) | `SV_Physics_Client` → `SV_WalkMove` | Players |

`SV_WalkMove` is the player movement function — friction, acceleration,
stair-stepping. `SV_FlyMove` (`sv_phys.c:231`) is the "basic solid body
movement clip that slides along multiple planes" — the core collision
response for non-walking entities. `SV_PushMove` (`sv_phys.c:434`) moves
pusher entities and carries anything riding on them. `SV_PushEntity`
(`sv_phys.c:403`) moves a single entity and returns a trace with the
collision result.

### The client is a predictive terminal

The client gathers input, sends it to the server as a `UserCmd`, and
renders the state updates it receives back. As the learning guide's client
package doc explains, the client does five things each frame:

1. **Sample input** — `KButton` states and mouse movement are combined
   into a `UserCmd` (view angles, movement, buttons, impulse).
2. **Send command** — `SendCmd` serializes the `UserCmd` into a `CLCMove`
   message.
3. **Parse server messages** — `parse.go` reads `svc_update` messages,
   delta-decompresses entity positions, and updates stats.
4. **Predict** — `prediction.go` runs the *same* physics code as the server
   locally, so the player's view responds instantly rather than waiting
   for a server round-trip.
5. **Interpolate** — `LerpPoint` computes a 0.0–1.0 fraction to smoothly
   interpolate entity positions between server updates (which arrive at
   20 Hz, while the renderer may run at 250 Hz). [#ClientDocs](#clientdocs)

### The signon sequence

Even in single-player, the client connects to the server through a
multi-stage "signon" sequence. The formal specification
(`QUAKE_SPECIFICATION.md` §3.1) defines it:

| Stage | Name | What happens |
| --- | --- | --- |
| 0 | `SignonNone` | Initial state |
| 1 | `SignonPrespawn` | Server info and precaches |
| 2 | `SignonClientInfo` | Client sends its info |
| 3 | `SignonBegin` | Loading the map |
| 4 | `SignonDone` | Fully connected and active |

The walkthrough doc puts it bluntly: *"single-player is not a shortcut
around the network model. It is the same conceptual client/server
lifecycle, just connected in-process"* via a loopback socket.
[#WalkSP](#walksp) In multiplayer, the same protocol runs over UDP.

### Entity snapshots and delta compression

The server does not send the full state of every entity every frame. It
sends **delta-compressed snapshots**: the client maintains the previous
frame's state, and the server sends only what changed. If a frame is
missed, the client must "force link" (snap) to the new position. The
specification notes: *"the client maintains the previous frame's state to
interpolate positions and angles. If a frame is missed, the client must
'force link' (snap) to the new position to prevent visual glitches."*
[#Spec](#spec) In C, this lives in `cl_parse.c`'s `CL_ParseDelta`.

---

## The filesystem: PAK files and search paths

Quake's asset system is a virtual filesystem. The formal specification
(§1) defines the search-path precedence, highest to lowest:

1. **Mod loose files** — files in the active mod directory (e.g.,
   `hipnotic/config.cfg`).
2. **Mod PAK files** — `pak%d.pak` in the mod directory, descending
   numeric order (`pak1.pak` overrides `pak0.pak`).
3. **Base loose files** — files in `id1/`.
4. **Base PAK files** — `pak%d.pak` in `id1/`, descending numeric order.
5. **Engine PAK** — `ironwail.pak` in the application root.

The PAK format is simple: a `PACK` header (4 bytes), a directory offset
(int32), and a directory length (int32). Each directory entry is a 56-byte
null-terminated filename, a position (int32), and a length (int32).
Lookups are case-insensitive. [#Spec](#spec) In C, this lives in
`common.c` (`COM_InitFilesystem`, `COM_AddGameDirectory`,
`COM_LoadPackFile`). The Go port's `internal/fs` package mirrors this
exactly, including path-sanitization security checks against directory
traversal. [#FSDocs](#fsdocs)

---

## The BSP world: geometry, visibility, and collision

Quake maps use **Binary Space Partitioning** (BSP). The BSP package doc
explains:

> A `.bsp` file is a list of vertices, edges, faces, planes, and leaves.
> You do not need to parse it yourself [...] but you must understand that
> the world is "one big mesh with extra metadata". [#BSPDocs](#bspdocs)

### What BSP gives you

A BSP file organizes world geometry into a tree of splitting planes. The
leaves of the tree are convex "rooms" — the empty spaces the player walks
through. Each leaf stores:

- **Contents** — `CONTENTS_EMPTY`, `CONTENTS_WATER`, `CONTENTS_SLIME`,
  etc.
- **A PVS bitmask** — the **Potentially Visible Set**. For each leaf,
  the PVS is a bitmap saying which *other* leaves can be seen from this
  one. This is the single most important optimization in a Quake renderer:
  before drawing, the engine finds the leaf containing the camera, looks
  up the PVS, and only draws faces in leaves that are in the PVS. Huge
  maps render at 60 FPS because most of the world is never drawn.
- **Cluster** — for the PVS lookup.

The tree also supports **collision detection**. Quake uses three fixed
**hull** sizes (point, player, large), each precomputed as a separate set
of clip nodes. A trace is a recursive walk down the clip node tree,
splitting the move at each plane. The formal specification (§4.1)
defines them:

| Hull | Bounds | Used for |
| --- | --- | --- |
| 0 | Point (0x0x0) | Projectiles, small objects |
| 1 | Player (-16x-16x-24 to 16x16x32) | Player, most monsters |
| 2 | Large (-32x-32x-24 to 32x32x64) | Shambler, large monsters |

### The three trees

A Quake BSP has three parallel trees:

- **Nodes/Leafs** — the visibility and rendering tree. Faces are attached
  to leaves.
- **Clipnodes** — the collision tree. Hull 0/1/2 each have their own
  clipnode root.
- **Areanodes** — a runtime-built spatial partitioning tree (not in the
  BSP file) used by the server to quickly find entities near each other
  for trigger/collision queries. `SV_AreaTriggerEdicts` (`world.c:287`)
  walks this tree.

`SV_FindTouchedLeafs` (`world.c:389`) recursively determines which BSP
leaves an entity overlaps, which is used for visibility and for deciding
which triggers should fire. `SV_LinkEdict` (`world.c:467`) inserts an
entity into the areanode tree and calls `SV_FindTouchedLeafs`.
`SV_TouchLinks` (`world.c:336`) walks the areanode tree to find and fire
trigger entities overlapping a given entity.

---

## The QuakeC VM: game logic as bytecode

Hickman notes that Quake's scripting system allowed modders to *"change
the game without having the C source code"* and that scripts are
*"processed by the exec command and sent to cmd.h to be run."*
[#Hickman](#hickman) But the scripting system is far more sophisticated
than that summary suggests.

### What QuakeC is

QuakeC is a compiled domain-specific language. Gameplay logic — how the
shotgun works, how monsters think, how doors open, how triggers fire — is
not in the C engine. It is compiled into a `progs.dat` bytecode file that
the engine loads at map start. The engine provides **builtins** (native
functions like `traceline`, `spawn`, `sound`, `setorigin`) that the
QuakeC code calls. The engine calls into QuakeC at specific dispatch
points: `StartFrame`, `PlayerPreThink`, `PlayerPostThink`, `touch`,
`think`, `use`, `blocked`, and the client lifecycle functions
(`PutClientInServer`, `ClientConnect`, etc.).

### The VM architecture

The C `qcvm_t` struct (`progs.h:203`) holds:

- `progs` — the parsed header of `progs.dat`.
- `functions` — the function table.
- `statements` — the bytecode instruction array.
- `globals` — the global variable array (floats, accessed by offset).
- `fielddefs` — field definitions (for reflection on entity fields).
- `edicts` — a `byte *` pointing to the entity array.
- `edict_size` — bytes per entity.
- `builtins[MAX_BUILTINS]` — the builtin function table.

### The shared-memory model

This is the critical architectural fact. In C, an `edict_t` is:

```c
typedef struct edict_s
{
    // engine-side fields (free, area chain, baseline, alpha, scale, ...)
    qboolean    free;
    link_t      area;
    entity_state_t baseline;
    unsigned char alpha;
    // ...
    entvars_t   v;          /* C exported fields from progs */
    /* other fields from progs come immediately after */
} edict_t;
```

The `entvars_t v` field is embedded directly in the struct. QC bytecode's
`OP_LOAD_*` and `OP_STORE_*` instructions read and write
`&ed->v + field_offset` — the **exact same memory** the C engine code
accesses via `ed->v.field`. The macros are pure pointer arithmetic:

```c
#define EDICT_TO_PROG(e)  (int)((byte *)e - (byte *)qcvm->edicts)
#define PROG_TO_EDICT(e)  ((edict_t *)((byte *)qcvm->edicts + e))
#define NEXT_EDICT(e)     ((edict_t *)((byte *)e + qcvm->edict_size))
```

**There is no sync.** When QuakeC sets `self.nextthink`, the engine sees
it immediately. When the engine sets `ent->v.velocity`, QuakeC sees it
immediately. All entity fields are accessible by both C and QC through the
same memory. [#QCVM](#qcvm) This is elegant and fast — and it is the
third thing the Go port has to replace (covered in Chapter 5).

### The interpreter loop

`PR_ExecuteProgram` (`pr_exec.c:395`) is the interpreter entry point. It
walks the statement array, executing each opcode. Each statement is an
opcode plus up to three operands (typically offsets into the globals or
entity fields). When a statement calls a negative function index, the VM
dispatches to the corresponding builtin in the `builtins` array. The
interpreter has a runaway-loop limit (`0x1000000` statements) as a safety
net against QC bugs hanging the engine.

---

## The command system: everything is a command

Hickman covers scripting in §XIV, noting that scripts can set variables
(`/set`, `/unset`), create aliases, and run macros from `.cfg` files.
[#Hickman](#hickman) The deeper architectural point is that **everything
in the engine is a command**. When you press a key, it's bound to a
command (e.g., `+forward`). When you click a menu item, it queues a
command (e.g., `map start`). When the engine starts, it executes
`quake.rc`, which `exec`s `config.cfg`.

The `cmdsys` package doc explains the dispatch flow:

1. **Registration** — subsystems register command handlers via
   `AddCommand`.
2. **Buffering** — command text is added via `AddText` (deferred) or
   `InsertText` (immediate).
3. **Execution** — `Execute()` drains the buffer each frame.
4. **Tokenization** — each line is split into tokens (respecting quotes
   and semicolons).
5. **Dispatch** — the first token is looked up: if it matches a command,
   its handler is called; if an alias, it's expanded into the buffer; if
   a cvar, it's treated as a set operation. [#CmdSysDocs](#cmdsysdocs)

This design means the console, the menu, config files, and automation
all use the same control path. The single-player walkthrough notes:
*"menu actions are mostly command producers"* — choosing New Game queues
`disconnect`, `maxplayers 1`, `deathmatch 0`, `coop 0`, `map start`.
[#WalkSP](#walksp)

### Cvars

Cvars (console variables) store engine state and configuration. They have
flags: `FlagArchive` (saved to `config.cfg`), `FlagROM` (read-only),
`FlagAutoCvar` (auto-synced to an engine variable). Cvars fire callbacks
when their value changes. The formal specification covers them in §2.3.
[#Spec](#spec)

---

## Networking: datagrams, reliability, and the protocol

Quake's networking model is UDP-based, even for single-player (where it
uses a loopback driver). Hickman covers the file structure in §XIII,
listing the `net_*` files for IPX, UDP, serial, loopback, and the VCR
playback driver. [#Hickman](#hickman)

### The protocol

The Quake protocol is byte-oriented. The engine supports three protocol
versions: `PROTOCOL_NETQUAKE` (15, the original), `PROTOCOL_FITZQUAKE`
(666, Ironwail's extended protocol with larger entity counts and
additional message types), and `PROTOCOL_RMQ` (999, further extensions for
large-map coordinates). The Go port defaults to `PROTOCOL_RMQ` (999) to
support large-map coordinates. [#Spec](#spec)
The `internal/net` package doc explains the messaging model:

- **Reliable messages** use a stop-and-wait ARQ protocol. Payloads larger
  than 1400 bytes are fragmented and reassembled. Used for important
  state (map changes, precaches, signon).
- **Unreliable messages** are fire-and-forget. Used for frequently updated
  state (entity positions) where losing a packet is preferable to waiting
  for retransmission. [#NetDocs](#netdocs)

The wire format uses `SVC_*` (server-to-client) and `CLC_*`
(client-to-server) message type constants, defined in
`internal/net/protocol.go` in the Go port and scattered across the C
headers.

---

## Rendering: from `R_RenderView` to the screen

Hickman covers the rendering system in §IV, noting that *"Quake uses
OpenGL for the drawing of all graphics in the game"* and that rendering
*"mostly revolves around Alias models."* [#Hickman](#hickman) The C
Ironwail renderer is a modernized OpenGL path (core profile, shaders) in
`gl_*.c` and `r_*.c`. The key entry point is `R_RenderView` in
`gl_rmain.c`, which:

1. Sets up the view (camera, frustum).
2. Draws the world BSP (`R_DrawWorld` → `R_RecursiveWorldNode` →
   `R_DrawTextureChains`).
3. Draws entities (`R_DrawEntitiesOnList`): brush entities, alias models,
   sprites.
4. Draws water (`R_DrawWater` — opaque then translucent).
5. Draws particles.
6. Draws the viewmodel (first-person weapon).
7. Applies polyblend (underwater color tint, damage flash).
8. Draws the 2D overlay (HUD, console, menu).

The full rendering architecture — and how the Go port replaces it with
WebGPU — is the subject of Chapters 3 and 4.

---

## Game object models: alias, sprite, BSP

Hickman covers all three in §IX. [#Hickman](#hickman)

### Alias models (MDL)

Alias models represent players, monsters, and items. The format is
`IDPO` (IDPOLYGON), version 6. An MDL contains:
- Scale factors and origin.
- A bounding radius.
- Skin textures (the texture mapped onto the wireframe).
- A list of vertices and triangles.
- **Animation frames** — each frame has min/max bounding box values, a
  name, and an array of vertices (3D position + packed normal). Animation
  is achieved by interpolating between frames. [#Hickman](#hickman)
  The `internal/model` package and `internal/renderer/alias/` handle these
  in the Go port.

### Sprites

Sprites are 2D billboards that always face the camera. They are faster to
render than alias models and are used for explosions, pickups, and other
detailed static objects. The format is `IDSP`, version 1. A sprite is a
list of 2D pictures organized into frames. [#Hickman](#hickman)

### BSP models (submodels)

A BSP file contains multiple "models." Model 0 is the world itself.
Models 1+ are submodels — brush entities like doors, platforms, and
triggers that are part of the BSP geometry but can move independently.
The server loads these as `*1`, `*2`, etc. and assigns them to entities
via `setmodel()`. [#BSPDocs](#bspdocs)

---

## Audio: spatialization and mixing

The audio subsystem splits into files prefixed `snd_`. `snd_dma.c` is
the main control for streaming sound output. Sound volume and panning are
**spatialized**: volume decreases with distance (attenuation), and panning
is calculated using the dot product between the listener's right vector and
the vector to the sound source. [#Spec](#spec) The Go port uses the
[Oto][oto] library for audio output, replacing the C DMA/sound-card
drivers.

---

## Math: `mathlib` and the assembly fast paths

Hickman covers the math library in §XV. The functions in `mathlib.c` /
`mathlib.h` serve three purposes: rendering calculations, collision
detection, and physics. The key functions include:

- `VectorMA`, `DotProduct`, `VectorSubtract`, `VectorAdd`, `VectorCopy`
  — basic vector ops.
- `CrossProduct`, `VectorNormalize`, `VectorScale` — 3D vector math.
- `R_ConcatRotations`, `R_ConcatTransforms` — matrix composition.
- `BoxOnPlaneSide` — the core collision query: given a box and a plane,
  return 1 (in front), 2 (behind), or 3 (straddling). This is called
  millions of times per frame during collision and visibility traversal.
- `AngleVectors` — convert Euler angles to forward/right/up vectors.
- `FloorDivMod`, `GreatestCommonDivisor`, `Invert24To16` — integer math
  used by the renderer.

Three of these — `Invert24To16`, `TransformVector`, and `BoxOnPlaneSide`
— had hand-optimized assembly implementations in `math.s` / `matha.s` for
the original software renderer. [#Hickman](#hickman) The Go port replaces
all of this with pure-Go `float32` math in `pkg/types` and inline
operations, relying on the Go compiler's optimization.

---

## Why this architecture endures

Thirty years later, the patterns Quake established are still visible in
modern game engines:

- **Server-authoritative simulation with client prediction** is the
  standard model for networked games.
- **BSP/PVS visibility culling** (or its spiritual descendants, occlusion
  culling and portal systems) is still how engines avoid drawing what you
  can't see.
- **A scripting VM for game logic** separating engine from gameplay is
  universal (Lua in many engines, Blueprint in Unreal, C# in Unity).
- **Pre-baked lighting** (lightmaps) is still the dominant approach for
  static environments, even as real-time global illumination augments it.
- **The command system** as a universal control path is less common today
  but remains an elegant pattern for debuggability and automation.

The architecture is durable. The *implementation* is not — manual memory,
immediate-mode GL, shared-memory VM, Win32/DOS platform code. That is what
the Go port changes, and Chapter 2 begins that story.

---

## References

<a name="hickman"></a>[Hickman] Zachary Hickman, *"Quake Engine Analysis,"*
Northeastern University. Local copy: `article/analysisfinal.pdf`.

<a name="spec"></a>[Spec] `docs/QUAKE_SPECIFICATION.md`, ironwail-go
repository.

<a name="learningguide"></a>[LearningGuide] `docs/LEARNING_GUIDE.md`,
ironwail-go repository.

<a name="hostdocs"></a>[HostDocs] `docs/internal/host.md`, ironwail-go
repository.

<a name="clientdocs"></a>[ClientDocs] `docs/internal/client.md`,
ironwail-go repository.

<a name="walksp"></a>[WalkSP]
`docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md`, ironwail-go repository.

<a name="fsdocs"></a>[FSDocs] `docs/internal/fs.md`, ironwail-go
repository.

<a name="bspdocs"></a>[BSPDocs] `docs/internal/bsp.md`, ironwail-go
repository.

<a name="qcvm"></a>[QCVM] `docs/QCVM_ENTITY_SYNC.md`, ironwail-go
repository.

<a name="cmdsysdocs"></a>[CmdSysDocs] `docs/internal/cmdsys.md`,
ironwail-go repository.

<a name="netdocs"></a>[NetDocs] `docs/internal/net.md`, ironwail-go
repository.

[oto]: https://github.com/ebitengine/oto
