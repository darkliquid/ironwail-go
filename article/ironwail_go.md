# Porting Quake to Go — The ironwail-go Story

## Table of Contents

- [Prologue: Why Port Quake to Go in 2026](#prologue-why-port-quake-to-go-in-2026)
- [Chapter 1: How the Quake Engine Actually Works](#chapter-1-how-the-quake-engine-actually-works)
- [Chapter 2: The Go Divergence — From C Hunk to GC, From OpenGL to WebGPU](#chapter-2-the-go-divergence--from-c-hunk-to-gc-from-opengl-to-webgpu)
- [Chapter 3: The Renderer — OpenGL Then, WebGPU Now](#chapter-3-the-renderer--opengl-then-webgpu-now)
- [Chapter 4: Render Stages, Broken Down](#chapter-4-render-stages-broken-down)
- [Chapter 5: The Modding System and the QuakeC VM](#chapter-5-the-modding-system-and-the-quakec-vm)
- [Chapter 6: GoGPU — Pure-Go WebGPU in Practice](#chapter-6-gogpu--pure-go-webgpu-in-practice)
- [Chapter 7: Synthesis — What Was Learned, and Where It Goes](#chapter-7-synthesis--what-was-learned-and-where-it-goes)

---

# Prologue: Why Port Quake to Go in 2026

In 1996, id Software published Quake. It was the first fully-3D first-person
shooter from the studio that had defined the genre with Doom, and it shipped
with an engine whose architecture — a server-authoritative client-server
simulation, a BSP world with precomputed visibility, a bytecode scripting VM
for game logic, and a software (later OpenGL) renderer — set patterns that
game engines still follow thirty years later. Zachary Hickman, in an academic
analysis of the engine written for Northeastern University, framed it
succinctly: *"The impressive feature improvements required a much more
comprehensive engine."* [#Hickman](#hickman)

This is the story of re-implementing that engine in Go.

`ironwail-go` is a pure-Go port of [Ironwail][ironwail], a high-performance
modern fork of QuakeSpasm, which is itself a maintained fork of the original
GPL-Quake source. The port is not a line-for-line transliteration. It is a
deliberate re-architecture: garbage collection instead of a manual memory
hunk, Go packages instead of a flat `Quake/*.c` tree, goroutines instead of
SDL threads, and — most ambitiously — [WebGPU][gogpu] via the pure-Go
`gogpu` library instead of OpenGL. The project compiles with
`CGO_ENABLED=0`. There is no C in the runtime path.

The README states the intent plainly:

> Ironwail Go is an exercise in porting the entire Ironwail Quake codebase
> from C to Go, for the purposes of learning and education. It is an
> experiment to get more experience with agentic coding and furthermore to
> learn more about the Quake engine, game programming and indulge in a bit
> of nostalgia from my school days of hacking together Quake mods and maps.
> [#README](#readme)

Three threads run through the whole project, and through this article:

---

## An experiment in agentic coding — with multiple agents

The README addresses the elephant directly in a section titled *"Did you say
agentic coding? Is this just AI slop?"* The answer is a candid *"Yes and
no."* A large portion of the codebase was written by AI agents converting C
to Go, but under a human-as-architect-and-reviewer model that the project's
`AGENTS.md` codifies as a "Senior-Junior partnership" — the human acts as
architect and reviewer, the agent as a fast, literal-minded junior
engineer. [#AGENTS](#agents)

And critically: it was not one agent. The git history attributes work across
several different models:

- **GitHub Copilot** — the original workhorse, co-authored the early C-to-Go
  conversion commits and the cgo/OpenGL era (visible in commit `b2fb6e9`
  *"Retire gl+sdl"*, co-authored with Copilot). Copilot appears as a
  co-author on over 700 commits.
- **Claude Opus 4.6** — the bulk of the later, deeper work (the
  GoGPU renderer buildout, the QCVM sync unification, the renderer
  diagnosis and fix passes). Named in the README as a primary agent.
- **GPT-5.4** — the other agent named in the README as carrying the
  majority of agentic work alongside Claude.
- **GLM-5.2** (via the Crush CLI) — attributed as "Assisted-by" on
  20 commits, concentrated in the renderer fix passes of mid-2026.
- **Claude Sonnet 4.6** — appears on a smaller set of commits.
- **Gemini** — used for at least one commit (`4f754e0`, the QuakeGo module
  path cleanup).
- **Qwen** — tried but does not appear in surviving attribution.

The honest framing is that different agents have different strengths and
failure modes, and the project is as much a field test of *which models can
handle a large port at what stage* as it is a port of Quake. This article
will name agents where the git record supports it and will not invent
task-to-agent mappings the record does not contain. The later chapters —
especially the synthesis — reflect on what multi-agent porting at this
scale actually teaches.

---

## An educational artifact, not just a working engine

The second thread is that the codebase is built to be **read and learned
from**, not merely run. The stated goal is readability and self-explanation,
extensive documentation, and making it possible to understand how the
engine works *without* prior deep graphics-programming or game-development
experience. This shows up concretely in:

- **Per-package `doc.go` files** with an `# Original C lineage` section
  naming the C source files each package mirrors, so a reader can always
  locate the C counterpart before refactoring.
- **`docs/internal/*.md`** — a guide for every `internal/` package
  (`qc.md`, `renderer.md`, `server.md`, ...) explaining purpose, key types,
  core workflow, integration points, and learning tips.
- **`docs/RENDERER_LEARNING_PLAN.md`** — a 14-stage curriculum that walks a
  reader who knows Go but *not* graphics programming from "what is a GPU?"
  all the way to "read `RenderFrame()` top to bottom and explain every
  line," citing [Scratchapixel][scratchapixel] for theory and
  [webgpufundamentals][webgpufundamentals] for API practice, with
  build-it-yourself milestones at each stage. [#LearningPlan](#learningplan)
- **The `// Where in C:` citation convention** in tests, e.g.
  `// Where in C: SV_WalkMove in sv_phys.c`, anchoring every behavioral
  assertion to the canonical reference.
- **`bspdiag`** — an offline BSP inspection CLI (`cmd/bspdiag`) built so
  that anyone can inspect map lumps, entities, leaf contents, lightmaps, and
  liquid alpha settings without writing scratch scripts. [#AGENTS](#agents)
- **Parity test names that document the invariant being protected** — e.g.
  `TestExecuteProgramRunawayLoopLimitConstantMatchesC` asserts the
  runaway-loop limit is exactly `0x1000000`, and the name itself explains
  *why* that constant matters for mod compatibility. [#QCDocs](#qcdocs)

This article is written in the same spirit. It explains Quake-specific and
WebGPU-specific concepts inline rather than assuming them, and it cites
source by `file:line` so a reader can open the code and follow along.

---

## The Quake Brutalist Jams as the unforgiving integration test

The third thread is that the project's bugs, and thus its bug-fix
narrative, are driven by a specific class of stress test: the **Quake
Brutalist Jam** community map packs.

The Brutalist Jams (qbj) are community mapping events that produce large,
ambitious, idiosyncratic Quake maps. They are exactly the kind of content
that breaks a port that only ever tested against `id1/start`. `ironwail-go`
uses `qbj2` and `qbj3` as its de facto integration suite, and the parity
documentation is shot through with their specifics:

- **The `qbj2` mod** — specifically its `start` map — is a BSP2-format
  large map. It surfaced the texture atlas overflow bug (the materials buffer
  is hardcoded to 256 entries; the qbj2 start map has more), the lit-water
  fallback mismatch, and the QCVM entity-sync pusher/non-pusher bug chain
  (the qbj2 start map's lift trigger stack — a dozen trigger types firing on
  spawn — exposed that `executeQCFunction` was not syncing
  `MOVETYPE_PUSH` entities). [#QCVM](#qcvm) [#MaterialsDiag](#materialsdiag)
- **`qbj3`** (e.g. the `qbj3_stickflip` map) is the current priority
  stress case. The parity guide records its scale: 85,936 raw faces,
  77,001 built faces, 168,142 built triangles, 322,144 vertices, 22,195
  leafs, 750 models, 106 textures, four lightmap pages, 1,295
  lit-water/turbulent faces, and 228 sky faces. Its first rendered frame
  at the captured spawn view reports 1,002 visible faces, eight opaque
  world batches, seven opaque brush entities, and eleven opaque alias
  entities. [#Parity](#parity)

Each chapter of this article that discusses a bug will tie it back to the
specific qbj map that surfaced it. The brutalist jams are not a footnote;
they are the reason the open bugs are known and documented, and they keep
the parity claims honest.

---

## The central tension

Here is the tension the rest of this article explores. Quake is a 1996 C
engine built around three assumptions that Go actively pushes against:

1. **Manual memory management.** Quake's `Hunk` and `Zone` allocators are a
   single pre-allocated heap with manual pointer arithmetic and
   bump-allocation arenas. [#Hickman](#hickman) Go has a garbage collector
   and forbids pointer arithmetic.
2. **Immediate-mode, single-threaded rendering.** The C renderer
   (Ironwail's modernized OpenGL path) draws directly to a framebuffer in
   `R_RenderView`, binding textures one at a time in immediate GL calls.
   WebGPU is explicitly a *retained* API: you write a "recipe" of commands
   into a command buffer, the GPU executes it later, and the CPU and GPU
   do not share memory directly. [#LearningPlan](#learningplan)
3. **Shared memory between engine and scripting.** In C, the QuakeC VM
   and the engine code read and write the *same* `edict_t` structs in the
   *same* memory. There is no sync. In Go, the GC'd `Edict` structs and the
   flat `QCVM.Edicts []byte` array are different storage, requiring a sync
   layer between them. A round of fixes (commit `fe9e43c`) collapsed the
   old fragile multi-path selective sync (separate pusher/non-pusher
   snapshot/diff/restore at five dispatch points) into a single
   `syncAllToQCVM`/`syncAllFromQCVM` at one dispatch point
   (`executeQCFunction`), eliminating the "forgot to sync this path" class
   of bug. 157 typed accessor methods were also added to `Edict`
   (`internal/server/entity_accessors.go`) that read/write the QCVM byte
   array directly, and ~27 call sites in `server.go` have migrated to
   direct-VM access. But the sync layer still exists — the physics and
   movement hot paths still use the `EntVars` Go struct, and
   `server_qc_sync.go` still performs reflection-based field-selective
   copies at every QC callback. The long-term goal (steps 3–5 of the
   migration plan) is to migrate all hot paths to the accessors, delete
   `EntVars` and the sync layer entirely, and match C's zero-sync model.
   [#QCVM](#qcvm)

Every architectural decision in `ironwail-go` — and every bug — flows from
the collision between Quake's 1996 assumptions and Go's 2026 reality. The
chapters that follow walk that collision, subsystem by subsystem: the
engine architecture, the Go divergence, the renderer, the render stages,
the QuakeC VM and modding system, and the GoGPU pure-Go WebGPU stack that
makes it all render without a line of C.

---

## References

<a name="hickman"></a>[Hickman] Zachary Hickman, *"Quake Engine Analysis,"*
Northeastern University. Local copy: `article/analysisfinal.pdf`; text
extraction: `article/analysisfinal.txt`.

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="learningplan"></a>[LearningPlan] `docs/RENDERER_LEARNING_PLAN.md`,
ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="qcvm"></a>[QCVM] `docs/QCVM_ENTITY_SYNC.md`, ironwail-go repository.

<a name="qcdocs"></a>[QCDocs] `docs/internal/qc.md`, ironwail-go repository.

<a name="materialsdiag"></a>[MaterialsDiag] `docs/diagnoses/qbj2_materials.md`,
ironwail-go repository.

[ironwail]: https://github.com/andrei-drexler/ironwail
[gogpu]: https://github.com/gogpu/gogpu
[scratchapixel]: https://www.scratchapixel.com/
[webgpufundamentals]: https://webgpufundamentals.org/

---

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

---

# Chapter 2: The Go Divergence — From C Hunk to GC, From OpenGL to WebGPU

Chapter 1 described the Quake engine as it was built in 1996: a manual-memory,
single-threaded, immediate-mode-GL, shared-memory-VM architecture. This chapter
is about what happens when you port that to Go in 2026. The `ironwail-go`
README states the intent:

> Apart from the obvious that this is Go, rather than C, I'm building this
> with the following changes: gogpu/WebGPU as the canonical gameplay
> renderer/runtime; dividing the codebase up into packages; use Go stdlib
> for as much as possible, rather than custom implementations of things
> from the original C codebase. [#README](#readme)

This is not a transliteration. It is a deliberate re-architecture that preserves
behavioral parity while changing the substrate. Each divergence has a reason,
and each reason has a cost.

---

## Memory: from Hunk/Zone/Cache to the garbage collector

The most fundamental change. Chapter 1 covered Quake's three-tier memory model:
the Hunk (a single pre-allocated arena, bump-allocated and nuked on map change),
the Zone (a free-list heap for individual-lifetime objects), and the Cache
(LRU-evictable resource storage). All three assume the engine owns a contiguous
block and manages it with pointer arithmetic.

Go replaces all of this with the runtime garbage collector. `Hunk_Alloc` becomes
`make()` or `new()`. Raw pointer arrays become slices. Manual `Z_Free` becomes
implicit GC. The comparison doc states it plainly: *"Replaces `Hunk_Alloc` with
standard `make()` or `new()` and utilizes slices instead of raw pointers for
collections."* [#Comparison](#comparison) The boot sequence doc adds: *"The C
version's `parms.membase = malloc(parms.memsize)` is entirely absent in Go."*
[#BootSeq](#bootseq)

### The cost: GC pressure in hot paths

The garbage collector is a trade. You get safety and simplicity; you lose
deterministic deallocation and control over memory layout. In a game engine
running at 250 FPS, the GC pressure from per-frame allocations is real. The
project's git history shows a direct response: commit `5a04a01` ("Optimize
renderer allocations in hot paths") introduced:

- **Scratch buffers** on the `Renderer` struct for brush entity rendering,
  eliminating per-frame `make()` in `renderOpaqueBrushEntitiesHAL`.
- **`sync.Pool`** for the dynamic lights slice, reusing allocations across
  frames.
- **`unsafe.Slice`** for `float32ToBytes` conversions, avoiding per-call heap
  allocation.
- Elimination of per-frame map copying (holding an `RLock` instead).

These are not premature optimizations — they came from profiling the renderer
under the qbj3 brutalist jam map's 1,002 visible faces and 750 models. The
GC pressure is the Go tax on Quake's arena model, and the mitigation is pooling
and reuse rather than reverting to manual memory.

### The long-term question: arena allocators

The prologue mentioned that one of the project's future plans is investigating
Go-based arena/region allocators. The Hunk's "allocate linearly, nuke
everything on map change" pattern maps cleanly onto Go's experimental `arena`
proposal or custom region allocators: allocate a large `[]byte`, sub-allocate
into it, and discard the whole backing array when the map changes. This would
give deterministic cleanup for the bulk of per-map allocations (BSP data,
models, textures) without the GC tax, while still being memory-safe. It is an
open question, not a settled decision. [#AGENTS](#agents)

---

## Concurrency: from single-threaded + SDL threads to goroutines

The C engine is primarily single-threaded. `Host_Frame` runs on one thread.
SDL mutexes and threads are used only for specific tasks: async loading,
background music, and (in Ironwail) the renderer thread. The comparison doc
notes: *"Primarily single-threaded, with some use of SDL mutexes and threads
for specific tasks like async loading or background music."*
[#Comparison](#comparison)

Go replaces this with goroutines and channels, but the project does not naively
parallelize the engine. The core simulation remains single-threaded — the
server physics loop, the QCVM execution, and the client update are sequential,
matching C's `SV_Physics` iteration. Where Go's concurrency model shines is in
the *periphery*:

### The async queue

The `internal/async` package provides a bounded FIFO work queue that marshals
work from background goroutines back to the main frame pump. Its doc explains
the parity rationale:

> This matches the semantics of the original C Ironwail's `host.c`
> AsyncQueue. In the context of a game engine like Quake, many systems (like
> save workers or mod downloaders) run in the background but need to update
> the game state safely without racing against the client or server state.
> [#AsyncDocs](#asyncdocs)

The queue uses `sync.Mutex` and `sync.Cond` for blocking behavior, and is drained
once per frame in `Host.Frame`. The async doc is candid about the trade-off:
*"While idiomatic Go might use an unbounded channel for this purpose,
`async.Queue` mirrors the C implementation's bounded, blocking behavior and
atomic drain semantics."* [#AsyncDocs](#asyncdocs)

### Dedicated render thread

The GoGPU renderer runs on its own thread, coordinated through the `gogpu.App`
event loop. The `OnDraw` callback (`renderer_gogpu_runtime.go:149`) registers
the frame draw callback; `OnUpdate` (`:199`) registers the game logic update.
The `MainThreadQueue` in `internal/host/mainthread.go` ensures that OS-sensitive
operations (window management, renderer calls) execute on the correct thread.
[#HostDocs](#hostdocs)

### Audio streaming

The `internal/audio` package uses the Oto library as its backend, replacing C's
DMA/sound-card drivers. Audio mixing still uses the same DMA-style buffer model
(mirroring classic sound card behavior), but the output device is abstracted
behind a `Backend` interface. The audio doc notes the mixer uses 24.8 fixed-point
arithmetic in `SamplePair` for precision without floating-point overhead.
[#AudioDocs](#audiodocs)

### Parallel asset loading

The `internal/engine` package provides `ParallelLoad[T]` and `LoadPipeline[T]`
using a worker-pool pattern with a buffered-channel semaphore for concurrency
limiting. This is used during level loading to fetch multiple sounds, models,
and textures concurrently. [#EngineDocs](#enginedocs)

---

## Packaging: from flat `Quake/*.c` to `internal/*` packages

The C Ironwail source is a flat directory of `*.c` and `*.h` files under
`Quake/`. There are no packages, no visibility control, no import boundaries.
Everything is global. `extern` declarations and header files are the only
interface contracts. The `COM_*` functions in `common.c` are called from
everywhere. The `SV_*` functions in `sv_phys.c` call `CL_*` functions in
`cl_main.c` directly.

Go cannot work this way. The `internal/` package convention enforces visibility
boundaries. The project divides the engine into packages with specific
responsibilities:

| C area | Go package | Responsibility |
| --- | --- | --- |
| `host.c`, `main_sdl.c` | `internal/host` | Main loop, timing, session lifecycle |
| `common.c` (VFS) | `internal/fs` | Virtual filesystem, PAK files |
| `gl_*.c`, `r_*.c` | `internal/renderer` | WebGPU rendering pipeline |
| `in_sdl.c`, `keys.c` | `internal/input` | Input abstraction |
| `pr_exec.c`, `pr_edict.c` | `internal/qc` | QuakeC VM |
| `sv_main.c`, `sv_phys.c` | `internal/server` | Authoritative simulation |
| `cl_main.c`, `cl_parse.c` | `internal/client` | Client state, prediction |
| `cmd.c` | `internal/cmdsys` | Command system |
| `cvar.c` | `internal/cvar` | Console variables |
| `console.c` | `internal/console` | Console buffer |
| `common.c` (math) | `pkg/types` | Vec3, Mat4, angle math |
| — | `internal/engine` | Generic data structures (Cache, Registry, Queue) |
| — | `internal/async` | Thread-safe work queue |
| — | `internal/game` | Top-level coordinator wiring everything together |

The `internal/game` package is the Go equivalent of the C `main()` wiring —
it owns the `Game` struct (`internal/game/game.go`) that holds Host, Server, QC,
CSQC, Renderer, Client, Particles, Menu, Input, Draw, HUD, Audio, caches, and
overlays. Cvars are registered centrally in `internal/game/game_init.go`.

### The `doc.go` lineage convention

Every package has a `doc.go` file with an `# Original C lineage` section naming
the C source files it mirrors. For example, `internal/server/doc.go` names
`sv_main.c`, `sv_phys.c`, `world.c`, and `pr_cmds.c`. This is not decoration —
it is a navigation tool. Before refactoring a Go package, you read its lineage
section to find the C counterpart, then study the C to understand the canonical
behavior. [#AGENTS](#agents)

### The `pkg/qgo` exception

`pkg/qgo/quake` and `pkg/qgo/quakego` are **separate Go modules** with their own
`go.mod` files, intentionally outside the root module. They are not importable by
the engine. This is by design: `pkg/qgo/quakego` is QuakeGo source (a Go dialect
compiled to QCVM `progs.dat` bytecode), not regular Go library code. The root
module does not require or replace `pkg/qgo/*`. [#AGENTS](#agents) Chapter 5
covers QuakeGo in detail.

---

## stdlib adoption: replacing custom Quake utilities

The README states: *"Use Go stdlib for as much as possible, rather than custom
implementations of things from the original C codebase."* [#README](#readme)
In practice this means:

- **String handling**: Quake's custom `COM_Parse` tokenizer and string utilities
  are replaced with Go `strings` / `strconv` where the semantics are compatible.
  The command tokenizer still has custom logic (it must respect Quake's
  quote/semicolon rules), but generic string manipulation uses stdlib.
- **I/O**: `io.Reader` / `io.Writer` / `io.NewSectionReader` replace C's raw
  `FILE *` and byte-pointer I/O. The filesystem package uses `io.NewSectionReader`
  to provide a standard `io.Reader` over a portion of a `.pak` file. [#FSDocs](#fsdocs)
- **Containers**: `sync.Map`, `sync.Pool`, generic slices replace C's manual
  linked lists and arrays.
- **Math**: `pkg/types` provides `Vec3` and `Mat4` as Go structs with both
  procedural (`Vec3Add`, `Vec3Dot`) and method (`v.Add`, `v.Dot`) APIs. The
  procedural functions follow C Quake's style for parity; the methods provide
  idiomatic Go. The doc notes: *"Both produce identical results."*
  [#TypesPkg](#typespkg)

### Where custom code remains

Some C utilities cannot be replaced by stdlib because they encode Quake-specific
semantics:

- **The command tokenizer** (`internal/cmdsys/cmd_buffer.go`) replicates
  Quake's specific rules for whitespace, quotes, and semicolons.
- **The QCVM** (`internal/qc`) must bit-match C's bytecode interpretation,
  including IEEE divide-by-zero behavior and the `0x1000000` runaway-loop limit.
- **The filesystem** must preserve Quake's case-insensitive PAK lookup and
  mod override order.
- **The network protocol** must produce byte-identical wire format.

---

## The CGO policy: pure Go, always

`mise.toml` sets `CGO_ENABLED = "0"`. The project is pure Go. AGENTS.md states
this as a hard rule: *"CGO is always off. The project is pure Go. Never
introduce CGO dependencies."* [#AGENTS](#agents)

This policy was not always in place. The git history tells a story:

### The cgo-GLFW detour and return

The project started with GoGPU as the intended renderer (commit `064c027`,
2026-02-24: *"renderer: port WebGPU core initialization"*). But early gogpu
issues — naga shader compilation bugs, Wayland input failures, crashes —
forced a detour. Commit `15b888e` (2026-02-25, one day later) added *"alternate
cgo gl renderer"*. For over a month, the engine ran on a cgo-based OpenGL
renderer using GLFW for windowing and SDL for input.

The gogpu issue #157 opening body records the frustration:

> I first attempted to tackle things using GoGPU as the rendering backend,
> but eventually hit enough issues that I sadly switched to cgo GLFW code.
> [#GogpuIssues](#gogpuissues)

The return came with commit `b2fb6e9` (2026-04-05: *"Retire gl+sdl (#11)"*),
which removed the OpenGL renderer, the SDL input backend, and made Oto the
canonical audio backend, with GoGPU as the sole renderer. The commit was
co-authored with Copilot. After that, commit `889f797` (2026-04-24) dropped
the renderer shims and cleaned up the game loop.

### The current stack

The canonical gameplay stack is now:
- **Renderer**: GoGPU/WebGPU (`github.com/gogpu/gogpu`)
- **Audio**: Oto (`github.com/ebitengine/oto/v3`)
- **Input**: renderer-provided backend (gogpu's input adapter in
  `internal/renderer/gogpu/input_backend.go`)
- **Native bindings**: `github.com/ebitengine/purego` (indirect, for
  cgo-free native function calls)

`purego` appears as an indirect dependency — it is used by the gogpu stack
for cgo-free FFI to platform libraries where needed, but the engine itself
compiles with `CGO_ENABLED=0`. [#Comparison](#comparison)

---

## Input: from SDL to backend injection

The C engine uses `in_sdl.c` to interface with SDL2 for keyboard, mouse, and
gamepad events. The input handling comparison doc explains the divergence:
*"Go uses `internal/input/` as a backend-neutral abstraction layer. The active
runtime backend is supplied by the executable/renderer integration rather than
by a package-local SDL implementation."* [#InputHandling](#inputhandling)

In practice, this means:
- `internal/input` defines a `Backend` interface and `System` type that
  normalize keyboard, mouse, and gamepad events into Quake key codes and
  movement commands.
- The gogpu renderer provides its own input adapter
  (`internal/renderer/gogpu/input_backend.go`) that bridges gogpu window events
  to the `input.Backend` interface.
- The input system decides which callback to trigger based on `KeyDest`
  (console, menu, game), matching C's `key_dest` dispatch.

The Go implementation maintains identical Quake keycodes (`KMWheelUp`,
`KMouse1`, etc.) to ensure compatibility with `config.cfg` and
`autoexec.cfg`. Gamepad support is currently initial (deadzones only), compared
to C Ironwail's extensive gyro/rumble support. [#InputHandling](#inputhandling)

The gogpu input bugs that forced the cgo detour (issues #129, #173, #175) are
covered in Chapter 6.

---

## Parity-first discipline

The comparison doc states the goal: *"high-fidelity parity,"* meaning
identical `progs.dat` execution, identical physics and movement, visual parity
with the GoGPU renderer, and support for standard Quake data files.
[#Comparison](#comparison)

This is enforced through several mechanisms:

### The `// Where in C:` convention

Tests cite the C function they mirror. For example, in
`internal/cmdsys/cmd_test.go`:

```go
// Where in C: Cmd_TokenizeString in cmd.c
```

This appears throughout the test suite. Every parity test is anchored to a
specific C function, so a reader can open both side by side.

### Parity test naming

Test names document the invariant being protected:

- `TestPhysicsSendIntervalMatchesFitzQuakeParity` — the send-interval lerp
  timing matches FitzQuake's protocol extension.
- `TestWriteEntityUpdate_FieldOrderMatchesCProtocol` — the wire format field
  order matches C exactly.
- `TestRandomBuiltinMatchesCompatSequence` — the `random()` QC builtin produces
  the same sequence as C's compatrand.
- `TestLoadExternalSkyboxWindMatchesCIronwailConfig` — the external skybox wind
  config parsing matches C.

The names are documentation. A reader scanning test names learns the parity
contract.

### The parity screenshot harness

`mise run parity-ref` captures deterministic reference screenshots from C
Ironwail. `mise run parity-go` captures matching GoGPU screenshots.
`mise run parity-compare` writes visual diffs and exits nonzero if any scene
exceeds the configured mismatch threshold. This is a real CI gate, not a
manual eyeball check. [#README](#readme)

### The brutalist jam maps as integration tests

As the prologue established, the Quake Brutalist Jam (qbj) map packs are the
project's de facto integration test suite. The qbj2 mod's `start` map — a
BSP2-format large map — surfaced the texture atlas overflow (the materials
buffer is hardcoded to 256 entries but the map has more), the lit-water fallback
mismatch, and the QCVM entity-sync pusher/non-pusher bug chain (the lift trigger
stack). The qbj3 mod's `qbj3_stickflip` map is the current priority stress case:
85,936 raw faces, 750 models, 106 textures, 1,295 lit-water faces, 228 sky
faces. [#Parity](#parity)

These maps are the unforgiving test. If a parity claim survives a qbj sweep,
it is real.

---

## The technology stack

| Layer | C Ironwail | ironwail-go |
| --- | --- | --- |
| Language | C99 | Go 1.26 |
| Renderer | OpenGL 1.x–3.x (legacy/core mix) | WebGPU via `gogpu` |
| Audio | SDL2 / DMA drivers | Oto (`ebitengine/oto/v3`) |
| Input | SDL2 (`in_sdl.c`) | gogpu input adapter |
| Windowing | SDL2 | gogpu `App` (Wayland/X11/native) |
| Math | `mathlib.c` + assembly (`math.s`) | `pkg/types` (pure Go `float32`) |
| Memory | Hunk / Zone / Cache | GC + slices + `sync.Pool` |
| Concurrency | SDL threads + mutexes | Goroutines + `sync` + `internal/async` |
| Data structures | Manual linked lists, arrays | `internal/engine` (generics: `Cache[T]`, `Registry[T]`, `Queue[T]`) |
| Platform | Win32 / DOS / Linux | Linux (Wayland/X11) via gogpu |

The Go runtime no longer carries parallel legacy renderer/input/audio variants.
The canonical gameplay stack is GoGPU rendering, renderer-provided input, and
Oto audio. There are no build tags selecting between renderers — the gogpu
renderer is always compiled. [#Comparison](#comparison) [#AGENTS](#agents)

---

## What this sets up

The divergences in this chapter — GC instead of Hunk, packages instead of flat
files, goroutines instead of SDL threads, WebGPU instead of OpenGL, a sync
layer instead of shared VM memory — are the root causes of every bug the rest
of this article covers. The renderer chapters (3 and 4) cover the
OpenGL-to-WebGPU leap. Chapter 5 covers the QCVM dual-storage sync problem.
Chapter 6 covers the gogpu-specific bugs that the pure-Go stack surfaced.

But first, Chapter 3 begins the renderer story: what the C renderer does, and
what replacing it with WebGPU means.

---

## References

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="comparison"></a>[Comparison] `docs/COMPARISON.md`, ironwail-go
repository.

<a name="bootseq"></a>[BootSeq] `docs/BOOT_SEQUENCE.md`, ironwail-go repository.

<a name="inputhandling"></a>[InputHandling] `docs/INPUT_HANDLING.md`,
ironwail-go repository.

<a name="hostdocs"></a>[HostDocs] `docs/internal/host.md`, ironwail-go
repository.

<a name="asyncdocs"></a>[AsyncDocs] `docs/internal/async.md`, ironwail-go
repository.

<a name="audiodocs"></a>[AudioDocs] `docs/internal/audio.md`, ironwail-go
repository.

<a name="enginedocs"></a>[EngineDocs] `docs/internal/engine.md`, ironwail-go
repository.

<a name="fsdocs"></a>[FSDocs] `docs/internal/fs.md`, ironwail-go repository.

<a name="typespkg"></a>[TypesPkg] `pkg/types/types.go`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript
of gogpu/gogpu issues, fetched 2026-07-27).

---

# Chapter 3: The Renderer — OpenGL Then, WebGPU Now

The renderer is the most divergent subsystem in `ironwail-go`. The C Ironwail
renderer is a modernized OpenGL path — core-profile shaders, UBOs, SSBOs,
indirect draws — but it is still fundamentally an immediate-mode, single-pass,
single-framebuffer OpenGL renderer. The Go port replaces it with a WebGPU
renderer built on the `gogpu` library: explicit pipelines, bind groups,
command-buffer submission, render passes, and an offscreen scene target.

This chapter compares the two architectures and explains the conceptual leaps
required to move from one to the other. Chapter 4 will walk through the render
stages one by one.

---

## The C renderer: `R_RenderView` and the single framebuffer

The C Ironwail renderer lives in `gl_*.c` and `r_*.c`. The entry point is
`R_RenderView`, which calls `R_SetupView` (`gl_rmain.c:964`) and then
`R_RenderScene` (`gl_rmain.c:1888`). The scene rendering order, from
`R_RenderScene`, is:

```c
void R_RenderScene (void)
{
    R_SetupScene ();
    R_Clear ();
    Fog_EnableGFog ();
    S_ExtraUpdate ();

    R_DrawEntitiesOnList (false);   // opaque world geometry + opaque entities
    R_DrawParticles (false);        // opaque particles
    Sky_DrawSky ();                 // sky
    R_DrawWater (false);            // opaque water (alpha == 1.0)
    R_BeginTranslucency ();         // set up translucent mode
    R_DrawWater (true);             // translucent water (alpha < 1.0)
    R_DrawEntitiesOnList (true);    // translucent entities
    R_DrawParticles (true);         // translucent particles
    R_EndTranslucency ();
    R_DrawViewModel ();             // first-person weapon
    R_ShowTris ();                  // debug wireframe overlay
}
```

Key characteristics of the C renderer:

### Single framebuffer, no intermediate submits

The water diagnosis doc captures the essential constraint: *"C Ironwail (OpenGL)
renders the entire frame to a single framebuffer within one `R_RenderView` call.
There are no intermediate command buffer submits."* [#WaterDiag](#waterdiag)
Everything — opaque, translucent, viewmodel, particles — draws into the same
framebuffer in sequence. Blending just works because the destination buffer
accumulates results naturally.

### Per-texture binding

The C renderer uses `GL_Bind` to bind individual textures to texture units
before each draw. `gl_texmgr.c` manages `gltexture_t` objects in a linked list,
with samplers created and deleted as a group. The world renderer in `r_world.c`
builds texture chains — linked lists of surfaces that share a texture — and
draws them in batches, but each batch still requires a `GL_BindTextures` call.
This is the classic OpenGL texture-binding pattern.

### OIT as an option

C Ironwail supports Order-Independent Transparency via weighted-blended
transparency (McGuire & Bavoil 2013). `R_BeginTranslucency` (`gl_rmain.c:1833`)
checks `R_GetEffectiveAlphaMode() == ALPHAMODE_OIT` and, if so, binds a
separate OIT framebuffer with accumulation and revealage textures, sets up
stencil state, and renders translucent objects into it. A final OIT resolve
pass composites the result back into the scene framebuffer. [#WaterDiag](#waterdiag)

### OpenGL state machine

The C renderer manipulates GL state directly:
`glEnable(GL_POLYGON_OFFSET_FILL)`, `glDisable(GL_STENCIL_TEST)`,
`glStencilFunc`, `glBlendFunc`, `glDepthMask`. State leaks between draws are
a constant hazard — forgetting to reset depth-write or blend mode after a
special pass produces visual corruption. Ironwail wraps this in `GL_SetState`
with `GLS_*` flags (`GLS_BLEND_ALPHA`, `GLS_NO_ZTEST`, `GLS_NO_ZWRITE`,
`GLS_CULL_NONE`), but the underlying model is a global mutable state machine.

---

## The Go renderer: WebGPU command buffers and explicit pipelines

The Go port's renderer lives in `internal/renderer/*_gogpu.go`. The entry point
is `RenderFrame()` at `renderer_gogpu_frame.go:82`. The renderer package doc
states its core design: *"abstracts the complexities of modern GPU APIs
(specifically WebGPU via the `gogpu` library) and provides a unified interface
for rendering 3D world geometry, 2D overlays, and special effects."*
[#RendererDocs](#rendererdocs)

### The CPU/GPU split

The learning plan explains the mental model: *"the CPU writes a 'recipe'
(commands) into a command buffer, the GPU executes it later. The CPU and GPU do
not share memory directly."* [#LearningPlan](#learningplan) This is visible in
the `DrawContext` struct (`renderer_gogpu.go:16`):

```go
type DrawContext struct {
    ctx               *gogpu.Context     // the underlying gogpu context
    gamma             float32
    renderer          *Renderer
    canvas            CanvasState
    sceneRenderActive bool
    sceneRenderTarget *wgpu.TextureView
    overlay           *overlay2D         // CPU-side 2D compositor buffer
}
```

The `Renderer` struct (same file, `:101`) holds all GPU-side resources:
pipelines, buffers, textures, bind groups. The CPU-side game logic never touches
pixels directly — it fills buffers and submits command encoders.

### The `Core`: headless-capable GPU initialization

The `Core` struct (`core_gogpu.go:46`) holds the wgpu Instance, Adapter,
Device, and Queue. `CoreConfig` specifies backend type, graphics API, validation,
and GPU preference. `DefaultCoreConfig()` returns `BackendGo`,
`GraphicsAPIAuto`, validation enabled, and `GPUPreferHighPerformance`. The `Core`
is used for both windowed and headless/screenshot rendering. This is a direct
consequence of WebGPU's design — you create an Instance, request an Adapter,
open a Device, get a Queue. There is no implicit context like OpenGL's
`wglMakeCurrent`. [#LearningPlan](#learningplan)

### Explicit pipeline objects

In OpenGL, pipeline state (shaders, blend mode, depth test/write, cull mode) is
set imperatively before each draw. In WebGPU, you create a
`RenderPipelineDescriptor` once — with vertex shader, fragment shader, blend
state, depth-stencil state, primitive topology, vertex buffer layout — and the
device compiles it into an immutable `RenderPipeline` object. At draw time, you
bind the pipeline and issue draws. State cannot leak between draws because there
is no mutable global state — each draw uses exactly the pipeline it was issued
under.

The Go port has separate pipelines for each pass type:
- **Opaque world** — depth-write on, depth-test on, no blending.
- **Alpha-test** — depth-write on, depth-test on, alpha-to-coverage or
  `discard` in fragment shader.
- **Translucent** — depth-write off, depth-test on, alpha blending.
- **Turbulent (water/lava/sky)** — UV warping in fragment shader, separate
  pipeline for translucent-turbulent.
- **Sky** — depth-write off, special two-layer scrolling texture.
- **Particles** — point-sprite billboards with procedural fragment shading.
- **PolyBlend** — fullscreen triangle, alpha blend.
- **Scene composite** — fullscreen triangle sampling the offscreen scene
  target, with underwater warp.
- **2D overlay** — fullscreen blit of the CPU-composited overlay texture.

### Bind groups: the resource binding model

In C OpenGL, textures and uniforms are bound to numbered texture units and
UBO binding points. In WebGPU, resources are organized into **bind groups** —
immutable bundles of resources (buffers, textures, samplers) bound to a
pipeline at specific `@group(N) @binding(M)` slots. The Go world shader declares:

```wgsl
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var<uniform> materials: array<MaterialData, 256>;
@group(1) @binding(0) var worldSampler: sampler;
@group(1) @binding(1) var worldTexture: texture_2d<f32>;
@group(2) @binding(0) var worldLightmapSampler: sampler;
@group(2) @binding(1) var worldLightmap: texture_2d<f32>;
@group(3) @binding(0) var worldFullbrightSampler: sampler;
@group(3) @binding(1) var worldFullbrightTexture: texture_2d<f32>;
@group(4) @binding(0) var lightClusters: texture_3d<u32>;
@group(4) @binding(1) var<storage, read> dynamicLights: DynamicLights;
```

Each bind group is created once and rebound per draw. This is more verbose than
OpenGL's `GL_Bind`, but it eliminates the "forgot to bind a texture" class of
bug and allows the GPU driver to pre-validate resource compatibility.

---

## The 48-byte `WorldVertex` contract

The most important structural decision in the Go renderer is that **every world,
brush, alias, sprite, and decal vertex uses the same 48-byte layout**. The
vertex layout doc calls this the "three-place contract": the Go struct, the byte
packing functions, and the WGSL pipeline vertex layout must all agree.

```
Offset  Size  Field           Go type         WGSL type        Purpose
------  ----  -----           -------         ---------        -------
 0      12    Position        [3]float32      vec3<f32>        XYZ world position
12       8    TexCoord        [2]float32      vec2<f32>        UV into texture atlas
20       8    LightmapCoord   [2]float32      vec2<f32>        UV into lightmap array
28      12    Normal          [3]float32      vec3<f32>        Surface direction
40       4    LightmapLayer   float32         f32              Lightmap page index
44       4    MaterialID      uint32          u32              Materials buffer index
 48 bytes total (stride)
```

The Go struct lives in `internal/renderer/world/types.go`. Four packing
functions convert `WorldVertex` slices to flat byte arrays for GPU upload:
`createWorldVertexBuffer` (static world), `appendGoGPUWorldVertexBytes` (brush
entities), `VertexBytes` (sky brushes), and `aliasVertexBytesInto` (alias
models). The WGSL `VertexInput` struct in the shader must match. If any one
disagrees, the GPU reads vertex data at wrong offsets — textures scramble,
lighting artifacts appear, geometry disappears. [#VertexLayout](#vertexlayout)

In C, each vertex type (world, alias, sprite) has its own vertex format and
its own `glVertexAttribPointer` setup. The Go port unifies them into one
layout, which simplifies pipeline creation and buffer management at the cost
of some wasted bytes (a particle doesn't need a lightmap coordinate, but it
carries one anyway).

---

## Texture atlas + per-vertex material ID

This is one of the biggest conceptual departures from the C renderer.

### The problem

A Quake map has hundreds of small textures. In C OpenGL, the renderer binds
each texture individually before drawing the surfaces that use it. This works
because OpenGL's state machine tolerates frequent `GL_Bind` calls (though it is
slow). In WebGPU, binding individual textures per draw is impractical — bind
group limits and the overhead of creating/rebinding per-texture would cripple
performance. [#LearningPlan](#learningplan)

### The Go solution

The Go port packs all world textures into a single **texture atlas** — one large
2D texture (or texture array) with per-face UV offsets. Each `WorldVertex`
carries a `MaterialID` (uint32 at offset 44). The fragment shader looks up
`materials[materialID]` in a uniform buffer to find the atlas bounds and layer,
then samples the atlas at the correct sub-region. The materials buffer is updated
each frame for texture animation (water, lava, sky texture chains).

The atlas packer is a binary-tree packer in
`internal/renderer/world_atlas_gogpu.go` (`TextureAtlasNode`, `AtlasLayer`).
The materials buffer is a GPU uniform buffer with 256 entries of 32 bytes each
(32 = atlas bounds `vec4` + layer `f32` + padding). The `animateWorldMaterials`
function (`world_material_gogpu.go:24`) rewrites it each frame with the current
animation frame. A separate frame-1 buffer handles pressed button textures
(commit `aa17df6`).

### The open bug

The materials buffer is hardcoded to 256 entries, but `baseMaterials` is
allocated as `textureCount + 2` without clamping. When a map has more than 254
textures — as the qbj2 mod's `start` map does — a silent buffer overflow occurs.
This is the **texture atlas overflow** bug, currently open.
[#MaterialsDiag](#materialsdiag) It is a direct consequence of the atlas design:
the C renderer's per-texture binding has no such limit.

---

## Lightmap array with 1px padding and the Vulkan workaround

Quake's pre-baked lighting is stored in lightmaps — 16x16 texel blocks per
surface. The C renderer uploads these into a single large lightmap texture
(2D, or a 2D array in Ironwail's modernized path).

The Go port uses a **lightmap texture array** with 1px padding between pages
and a vertical-stacking workaround for Vulkan. The `uploadWorldLightmapArray()`
function (`world_lightmap_gogpu.go:11`) handles this. Lightstyles (animated
lighting like flickering lights) are evaluated per frame, and lightmap pages
whose style changed are rebuilt. The fragment shader samples
`worldLightmap` using the per-vertex `lightmapCoord` and `lightmapLayer`.
[#LearningPlan](#learningplan)

C never allocates lightmaps for `SURF_DRAWTURB` (water/lava) surfaces — they
are always fullbright. Ironwail added optional lit water via `r_litwater`. The
Go port samples the lightmap when `litWater > 0.5` in the WGSL uniform,
defaulting to `vec3<f32>(0.5)` (fullbright when multiplied by 2.0).
[#WaterDiag](#waterdiag)

---

## Cluster-forward dynamic lights via compute shader

This is a feature the C renderer **does not have**. The Go port implements a
cluster-forward dynamic lighting system:

- A **compute shader** (`world_cluster_compute_gogpu.go:13`) divides the camera
  frustum into a 3D grid of clusters (32×16×32 tiles). For each cluster, it
  computes which dynamic lights affect it and writes a bitmask.
- The **fragment shader** reads the cluster bitmask and iterates only the
  assigned lights, rather than looping all lights.
- Dynamic lights are gathered on the CPU in `internal/renderer/dynamic_light.go`
  and `dynamic_light_pool.go`, then uploaded to a storage buffer.
- The `Core.SetupFrameData()` function (`core_gogpu.go:158`) computes the
  z-scale/bias used for cluster z-slicing (log-depth).
- The compute dispatch happens before the world render pass in
  `renderWorldInternal()` at `world_render_gogpu.go:99`.

This is a modern rendering technique that goes beyond anything in C Ironwail's
OpenGL path. It exists because WebGPU's compute shader support makes it natural
to implement, and because the qbj3 stress maps push dynamic light counts that
would be prohibitively expensive with a naive "loop all lights" approach.
[#LearningPlan](#learningplan)

---

## OIT: weighted-blended transparency as an optional path

The Go port also implements Order-Independent Transparency as an optional path,
enabled by a cvar:

- **Mode selection**: `internal/renderer/oit_mode.go`.
- **Render path**: `internal/renderer/oit_render_path.go`.
- **Stub**: `internal/renderer/oit_stub.go`.
- **Shared helpers**: `internal/renderer/oit/`.

When enabled, the renderer replaces the sorted-translucent pass with a
weighted-blended one (accumulation texture + revealage texture), avoiding the
back-to-front sort. This mirrors C Ironwail's `ALPHAMODE_OIT` path, but the Go
implementation is a separate render path rather than a state switch within the
same pass. [#LearningPlan](#learningplan)

---

## Render order parity

Despite the architectural divergence, the Go renderer preserves the C render
order. The `RenderFrame()` function (`renderer_gogpu_frame.go:82`) executes
ordered phases:

| Phase | C function | Go function |
| --- | --- | --- |
| Clear | `R_Clear` | `:113-129` (clear or preserve scene target) |
| World BSP | `R_DrawWorld` → `R_DrawTextureChains` | `renderWorld` → `renderWorldInternal` (`world_render_gogpu.go:16`) |
| Opaque entities | `R_DrawEntitiesOnList(false)` | `renderEntities` (`:586`) |
| Translucent water | `R_DrawWater(true)` | within `renderWorldInternal` (translucent turbulent pipeline) |
| Translucent entities | `R_DrawEntitiesOnList(true)` | `renderGoGPUSortedTranslucentFaceRendersHAL` (`world_gogpu_translucent.go`) |
| Viewmodel | `R_DrawViewModel` | `renderViewModelHAL` (`world_gogpu_alias.go:593`) |
| Scene composite | (post-process via FBO) | `compositeSceneRenderTarget` (`warpscale_gogpu.go:472`) |
| PolyBlend | (inline in `R_SetupView`/`V_CalcBlend`) | `renderPolyBlendHAL` (`polyblend_gogpu.go:224`) |
| 2D overlay | `Draw_Console`/`SCR_UpdateScreen` | `flush2DOverlay` (`renderer_gogpu_overlay.go:32`) |

The key parity principle from the water diagnosis: *"no face is drawn both
opaquely and translucently. The split is by alpha value, not by pass."*
[#WaterDiag](#waterdiag) Both passes use the same framebuffer (in C) or the
same render pass (in Go). The Go port had to learn this the hard way — the
original architecture split the frame into multiple `queue.Submit()` calls,
and Vulkan drivers discarded the framebuffer contents between submits,
causing translucent water to blend over black instead of opaque geometry.
Commit `6802fc5` fixed this by drawing translucent liquid faces **within the
world render pass itself**, matching C's single-framebuffer model.
[#WaterDiag](#waterdiag)

---

## The offscreen scene render target

Unlike C, which renders directly to the window framebuffer (or a single FBO
for post-processing), the Go renderer uses an offscreen **scene render target**
that is later composited to the swapchain surface. This exists for the
**underwater warp** — a screen-space sinusoidal distortion applied when the
camera is in water. The scene composite pass
(`compositeSceneRenderTarget` at `warpscale_gogpu.go:472`) blits the offscreen
target to the swapchain, applying the warp if active. This adds an extra
render pass and texture allocation that C does not strictly need (C applies the
warp via OpenGL's `glScissor` and viewport tricks), but it is the clean
WebGPU way to do post-processing. [#LearningPlan](#learningplan)

---

## The 2D overlay: CPU compositing

The Go port composites the HUD, menu, and console into a single CPU-side
texture buffer (`overlay2D` in `DrawContext`) and blits it to the screen as one
GPU draw. This is different from C, which draws 2D elements via immediate-mode
GL calls. The `flush2DOverlay` function (`renderer_gogpu_overlay.go:32`) does
the blit. This approach reduces GPU draw calls for 2D (which can be hundreds of
text characters and pic draws per frame) to a single fullscreen blit.
Commit `3b9cfeb` pooled the overlay CPU buffer and cached the GPU texture to
avoid per-frame allocation. [#RendererDocs](#rendererdocs)

---

## What this means for parity

The architectural divergence is real and has cost. The bugs documented in the
`docs/diagnoses/` folder — water translucency, atlas overflow, lightmap
fallbacks, texture corruption on multi-layer atlas maps — are all consequences
of the architectural difference between OpenGL's implicit state model and
WebGPU's explicit pipeline model. Each fix is a lesson in how WebGPU's
constraints reshape the renderer:

- **Vulkan discard between submits** → draw translucent water within the world
  render pass (commit `6802fc5`).
- **Per-texture binding limit** → texture atlas with per-vertex material ID
  (commit `e99fad0`), which then introduced the 256-entry overflow bug.
- **Lightmap texture array mismatch** → `TextureViewDimension2DArray` fallback
  fix for faces without lightmap data.
- **Dynamic uniform buffer offset collision** → dynamic uniform buffer offsets
  for per-pass alpha values (commit `6802fc5`).

Chapter 4 walks through each render stage in detail, with file:line references
and the specific bugs encountered at each stage.

---

## References

<a name="rendererdocs"></a>[RendererDocs] `docs/internal/renderer.md`,
ironwail-go repository.

<a name="learningplan"></a>[LearningPlan] `docs/RENDERER_LEARNING_PLAN.md`,
ironwail-go repository.

<a name="vertexlayout"></a>[VertexLayout] `docs/VERTEX_LAYOUT.md`, ironwail-go
repository.

<a name="waterdiag"></a>[WaterDiag] `docs/diagnoses/qbj2_water.md`, ironwail-go
repository.

<a name="materialsdiag"></a>[MaterialsDiag]
`docs/diagnoses/qbj2_materials.md`, ironwail-go repository.

---

# Chapter 4: Render Stages, Broken Down

Chapter 3 compared the C OpenGL renderer and the Go WebGPU renderer
architecturally. This chapter walks through a single rendered frame,
stage by stage, from clear to overlay. For each stage: what it is for, where
it lived in C, how it works in Go, and what bugs were encountered.

The stage numbering follows `docs/RENDERER_LEARNING_PLAN.md` (Stages 0–14),
which is the project's canonical curriculum for learning the renderer.
[#LearningPlan](#learningplan) The frame orchestration lives in
`RenderFrame()` at `renderer_gogpu_frame.go:82`, and the world render pass
lives in `renderWorldInternal()` at `world_render_gogpu.go:16`.

---

## Stage 0: The GPU core — Instance, Adapter, Device, Queue

### Purpose

Before any rendering can happen, the engine must establish a connection to
the GPU. In WebGPU, this is a four-step hierarchy: create an Instance (the
entry point to the WebGPU API), request an Adapter (a physical GPU), open a
Device (a logical GPU context with its own queue), and get the Queue (the
command submission interface). [#LearningPlan](#learningplan)

### C reference

OpenGL has no equivalent — the context is created implicitly by the
platform layer (`SDL_GL_CreateContext` in `gl_vidsdl.c`). There is no
adapter selection; the OS's default GPU is used.

### GoGPU reality

The `Core` struct (`core_gogpu.go:46`) holds the Instance, Adapter, Device,
and Queue. `CoreConfig` specifies backend type (`BackendGo`), graphics API
(`GraphicsAPIAuto`), validation (enabled by default), and GPU preference
(`GPUPreferHighPerformance`). `DefaultCoreConfig()` returns these defaults.

The `Core` is used for both windowed and headless/screenshot rendering. In
windowed mode, the `gogpu.App` event loop owns the surface; in headless
mode, `Core.InitHeadless()` creates an offscreen surface for screenshot
capture. The GPU preference was the subject of gogpu issue #176 (adapter
power preference not forwarded on hybrid-GPU Linux systems).
[#GogpuIssues](#gogpuissues)

### Bugs/lessons

The screenshot path was originally a stub writing `RGB(20,20,46)` — a
plausible-looking dark color that was not a real GPU readback. This
actively misled the water translucency investigation until it was fixed.
[#WaterDiag](#waterdiag)

---

## Stage 1: The triangle — pipelines, shaders, bind groups

### Purpose

The fundamental unit of WebGPU rendering: a vertex buffer + a WGSL vertex
shader + a WGSL fragment shader + a pipeline object + a bind group, all
wired together to produce pixels. [#LearningPlan](#learningplan)

### C reference

In C, this is `glBegin/glEnd` (legacy) or VBO + `glDrawArrays` (core
profile). Shaders are GLSL, compiled at runtime. Pipeline state is mutable
and set imperatively.

### GoGPU reality

The simplest real shader is the **polyblend** fullscreen triangle
(`polyblend_gogpu.go:15`). It has no vertex buffer — positions are baked
into the shader using `@builtin(vertex_index)`:

```wgsl
@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    return output;
}
```

The fragment shader (`polyblend_gogpu.go:34`) reads a `blendColor` uniform
and outputs it. This is a fullscreen tint — Quake's "polyblend" used for
underwater color wash and damage flashes. The pipeline setup is in
`ensurePolyBlendResourcesLocked()` (`:83`); per-frame use is
`renderPolyBlendHAL()` (`:224`), called from `RenderFrame()` at
`:218`. [#LearningPlan](#learningplan)

For a real vertex-buffer example, the **particle** pipeline
(`particle_gogpu.go:20`) uses instanced vertices with per-particle position
and color attributes.

### Bugs/lessons

The naga WGSL→SPIR-V compiler had a bug with scalar `mix()` (gogpu issue
#162) — `mix(vec3, vec3, f32)` produced invalid SPIR-V that crashed on
NVIDIA. The workaround was `vec3<f32>(fog)` splat. Fixed in naga v0.17.0+.
[#GogpuIssues](#gogpuissues)

---

## Stage 2: Matrices, the camera, and 3D-to-2D

### Purpose

The **view matrix** transforms world space into camera/eye space. The
**projection matrix** transforms eye space into clip space (the GPU's
normalized cube). Together they are the VP matrix. [#LearningPlan](#learningplan)

### C reference

`R_SetupView` (`gl_rmain.c:964`) computes the view, calls
`AngleVectors` to get `vpn`/`vright`/`vup`, finds the view leaf via
`Mod_PointInLeaf`, and sets up the projection. The VP matrix is implicitly
the OpenGL projection/modelview stack.

### GoGPU reality

Camera state and VP computation live in `internal/renderer/camera.go`. The
VP matrix is packed into the world uniform buffer (`worldUniformsWGSL` at
`world_shaders_gogpu.go:10`):

```wgsl
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    litWater: f32,
    skyWindPhase: f32,
    // ...
}
```

The world vertex shader multiplies `position` by this matrix
(`worldVertexShaderWGSL` at `world_shaders_gogpu.go:27`). Uniform buffer
packing is in `renderer_gogpu_uniforms.go`. In `renderWorldInternal()`,
the VP is computed and written to the GPU buffer at `:144-153`.

---

## Stage 3: Loading real geometry — the BSP world

### Purpose

Upload the BSP world geometry (vertices, edges, faces, textures) to GPU
buffers and render it. The world is "one big mesh with extra metadata."
[#LearningPlan](#learningplan)

### C reference

`R_DrawWorld` in `r_world.c` recursively walks the BSP tree, marks visible
surfaces, builds texture chains, and draws them. The C renderer uses SSBOs
and indirect multi-draw for batching (`gl_bmodel_indirect_buffer`,
`GL_DrawTextures`).

### GoGPU reality

This is the largest stage. `UploadWorld()` at
`world_upload_gogpu.go:18` orchestrates everything:

- **BSP → GPU vertex upload**: `WorldGeometry` in
  `world_geometry_gogpu.go` constructs the vertex data.
- **Vertex construction**: the 48-byte `WorldVertex` struct (see Chapter 3
  and `docs/VERTEX_LAYOUT.md`) flows from Go struct → byte packer → WGSL
  `@vertex` input. [#VertexLayout](#vertexlayout)
- **Byte packing**: `appendGoGPUWorldVertexBytes` in `world_gogpu.go`.
- **Pipeline creation**: `createWorldPipeline()` and friends in
  `world_pipelines_gogpu.go:13`.
- **The render pass**: `renderWorldInternal()` at
  `world_render_gogpu.go:16` creates the command encoder, begins the
  render pass with `LoadOpClear`, sets the pipeline, sets vertex/index
  buffers, sets bind groups, and issues `DrawIndexed` calls.

The render pass descriptor (`world_render_gogpu.go:107-118`) attaches both
a color attachment (the surface view or scene render target) and a
depth-stencil attachment (`worldDepthTextureView`).

### Bugs/lessons

Texture corruption on multi-layer atlas maps (commit `d89b34c`) was caused
by not copying both atlas layer and bounds when animating textures. The
fix was in `animateWorldMaterials` to swap the entire material config.
[#MaterialsDiag](#materialsdiag)

---

## Stage 4: Textures and the texture atlas

### Purpose

A Quake map has hundreds of small textures. WebGPU cannot bind hundreds of
textures individually. Solution: pack them into a single atlas texture and
use per-vertex `materialID` to index into a materials uniform buffer that
holds the atlas bounds and layer for each texture. [#LearningPlan](#learningplan)

### C reference

C uses per-texture `GL_Bind` calls. Texture chains group faces by texture
to minimize bind calls, but each chain still requires a bind.

### GoGPU reality

- **Atlas packer**: binary-tree packer in `world_atlas_gogpu.go`
  (`TextureAtlasNode`, `AtlasLayer`). A 2048×2048 atlas with multiple layers.
- **Atlas upload + GPU texture**: `world_resources_gogpu.go` (search for
  atlas creation).
- **Per-face texture index**: each `WorldVertex` carries a `MaterialID`
  (uint32 at offset 44).
- **Materials buffer**: 256 entries of 32 bytes each, updated each frame by
  `animateWorldMaterials` (`world_material_gogpu.go:24`) with the current
  animation frame.
- **Fragment shader sampling**: `buildWorldFragmentShaderWGSL()` in
  `world_shaders_gogpu.go:83` samples `worldTexture` using per-vertex UV and
  `materials[materialID].atlasBounds`.

### Bugs/lessons

The **atlas overflow** bug (still open): the materials buffer is hardcoded
to 256 entries, but `baseMaterials` is allocated as `textureCount + 2`
without clamping. When the qbj2 mod's `start` map has more than 254
textures, the `WriteBuffer` call silently overflows the 8192-byte GPU
buffer. The `diagMaterialBufferCapacity` and `diagMaterialBufferWrite`
functions in `diag_atlas.go` log warnings but do not clamp. The fix would
require changing the uniform buffer to a storage buffer to remove the
256-entry limit. [#MaterialsDiag](#materialsdiag)

---

## Stage 5: Lightmaps — pre-baked lighting

### Purpose

Quake does not compute lighting at runtime. Lighting is pre-baked offline
by the map compiler (`qrad`) and stored as a lightmap: a small grayscale
texture per face. The fragment shader samples both the material texture and
the lightmap, and multiplies them. [#LearningPlan](#learningplan)

### C reference

`R_DrawTextureChains` in `r_world.c` binds the lightmap texture and draws
with multi-texturing. Lightstyles (animated lighting) are evaluated per
frame in `CL_RunLightStyles` (`cl_main.c`).

### GoGPU reality

- **Lightmap sample extraction**: `internal/renderer/lightmap_samples.go`
  and `internal/renderer/world/lightmap_samples.go`.
- **Lightmap page stacking + GPU upload**: `uploadWorldLightmapArray()` at
  `world_lightmap_gogpu.go:11`. Uses 1px padding and vertical stacking
  (a Vulkan workaround).
- **Lightstyles**: the renderer evaluates lightstyle values per frame and
  rebuilds lightmap pages whose style changed. The `setGoGPUWorldLightStyleValues`
  function is called from `RenderFrame` (`renderer_gogpu_frame.go:135`).
- **Fragment shader**: `buildWorldFragmentShaderWGSL()` samples
  `worldLightmap` and multiplies it into the final color.

C never allocates lightmaps for `SURF_DRAWTURB` (water/lava) surfaces —
they are fullbright. Ironwail added optional lit water via `r_litwater`. The
Go port samples the lightmap when `litWater > 0.5` in the WGSL uniform.
[#WaterDiag](#waterdiag)

### Bugs/lessons

The fallback lightmap was created as `TextureViewDimension2D` but the
shader declared `texture_2d_array<f32>`. WebGPU rejected it silently,
defaulting to fullbright white (×2.0 overbright). Fixed by using
`TextureViewDimension2DArray`. [#WaterDiag](#waterdiag)

---

## Stage 6: Visibility — BSP, PVS, and "don't draw what you can't see"

### Purpose

The single most important optimization in a Quake renderer. The BSP tree
organizes the world into convex leaves. Each leaf has a PVS (Potentially
Visible Set) bitmask saying which other leaves can be seen from it. Before
drawing, the engine finds the camera's leaf, looks up the PVS, and only
draws faces in visible leaves. [#LearningPlan](#learningplan)

### C reference

`R_MarkVisSurfaces` (`r_world.c:58`) and `R_MarkSurfaces` (`r_world.c:111`)
walk the BSP tree and mark visible surfaces using the PVS.

### GoGPU reality

- **BSP leaf lookup + PVS**: `WorldRenderData` in `world.go:57` is a passive
  data holder. Actual visible face selection is `selectVisibleWorldFaces` in
  `world_shared.go:172`, called from `world_render_gogpu.go:333`.
- **Face classification**: opaque, alpha-test, translucent,
  turbulent/sky — helpers in `world_shared.go`.
- **What gets drawn**: `renderWorldInternal()` only draws faces that passed
  visibility. This is why Quake can render huge maps at 60 FPS — the qbj3
  `qbj3_stickflip` map has 85,936 raw faces but only 1,002 visible at the
  spawn view. [#Parity](#parity)

### Bugs/lessons

Single-leaf PVS culled underwater geometry. Fixed by using `FatPVS` (from
C's `SV_FatPVS`) when the camera leaf contains water faces. Also, a BSP2
`HeadNode` traversal bug caused `FatPVS`/`PointInLeaf` to start at node 0
(submodel) instead of `Models[0].HeadNode[0]` — critical for BSP2 maps.
Fixed in `internal/bsp/tree.go`. [#WaterDiag](#waterdiag)

---

## Stage 7: Depth testing and the opaque/translucent ordering problem

### Purpose

Opaque objects use depth testing (draw in any order, the depth buffer
resolves which is in front). Translucent objects must be sorted
back-to-front and drawn with depth-write off. [#LearningPlan](#learningplan)

### C reference

C draws the entire frame to a single framebuffer with no intermediate
submits. Opaque water (`R_DrawWater(false)`) draws with blend=OPAQUE,
depth-write=ON. Translucent water (`R_DrawWater(true)`) draws with
blend=ALPHA, depth-write=OFF. Both use the same framebuffer. The key
principle: no face is drawn both opaquely and translucently — the split is
by alpha value, not by pass. [#WaterDiag](#waterdiag)

### GoGPU reality

- **Depth texture**: `createWorldDepthTexture()` at
  `world_depth_gogpu.go:21`.
- **Multiple pipelines**: `world_pipelines_gogpu.go` has separate opaque,
  alpha-test, translucent, turbulent, and sky pipelines — each with
  different blend state and depth-write settings.
- **Translucent face collection + sorting**: `world_gogpu_translucent.go`
  (`renderGoGPUSortedTranslucentFaceRendersHAL`).
- **Render order**: opaque world → opaque entities → translucent water →
  translucent entities (see the `RenderFrame` phase table in Chapter 3).
- **OIT** (optional): `oit_render_path.go` replaces the sort with
  weighted-blended transparency.

### Bugs/lessons

The water translucency bug (resolved in commit `6802fc5`) had three root
causes:

1. **Vulkan swapchain discard**: the original architecture split the frame
   into multiple `queue.Submit()` calls. The translucent water pass opened
   a new render pass with `LoadOpLoad` after the world pass had already
   submitted. Vulkan drivers may discard framebuffer contents between
   submits, so translucent water blended over black. Fix: draw translucent
   water **within the world render pass itself**.
2. **Uniform buffer offset collision**: the translucent water uniform
   (`alpha=0.6`) overwrote the opaque uniform (`alpha=1.0`) at offset 0.
   Fix: dynamic uniform buffer offsets.
3. **Worldspawn `wateralpha` bypass**: `ResolveLiquidAlphaSettings` only
   applied the override when `r_wateralpha` was exactly `1.0`. A stale
   config value prevented the map's `wateralpha=0.6` from taking effect.

[#WaterDiag](#waterdiag)

---

## Stage 8: Sky, liquids (turbulent), and fog

### Purpose

Quake's water/lava/sky surfaces use a "turbulent" warp: UV coordinates are
animated with a sine function to make the texture swim. Sky is a special
surface that ignores depth and uses a two-layer scrolling texture. Fog is
exponential distance fog. When underwater, the final composited scene is
distorted by a sinusoidal screen-space warp. [#LearningPlan](#learningplan)

### C reference

Turbulent warp is in `gl_warp.c` / `gl_warp_sin.h`. Sky is in `gl_sky.c`.
Fog is in `gl_fog.c`. The underwater screen-space warp is in
`gl_warp.c`'s `R_BloomScreen` / warp-scale pass.

### GoGPU reality

- **Turbulent pipeline**: the `turbulent` and `translucent-turbulent`
  pipelines in `world_pipelines_gogpu.go`. The fragment shader warps UVs
  over time using the `time` uniform.
- **Sky faces**: the `sky` pipeline. Two-layer scrolling texture with
  `skyWindPhase`/`skyWindDir` fields in `worldUniformsWGSL`.
- **External skybox**: `skybox_external.go` for loading (PNG/TGA/JPG
  cubemaps), `world_external_sky_gogpu.go` for GPU bind group/pipeline.
- **Fog**: `fog_color` / `fog_density` uniforms in `worldUniformsWGSL`;
  the fragment shader applies exponential fog based on view distance.
- **Screen-space warp**: `warpscale_gogpu.go` — the
  `sceneCompositeFragmentShaderWGSL` (`:45`) applies a sinusoidal UV
  distortion when the camera is in water.

The scene composite fragment shader (`warpscale_gogpu.go:64-79`) is the
underwater warp math:

```wgsl
let aspect = dpdy(uv.y) / dpdx(uv.x);
let warpV = vec2<f32>(warpAmp, warpAmp * aspect);
let remapped = warpV + uv * (1.0 - 2.0 * warpV);
uv = remapped + warpV * sin(vec2<f32>(remapped.y / aspect, remapped.x)
    * (3.14159265 * 8.0) + warpTime);
return textureSample(sceneTexture, sceneSampler, uv * uvScale);
```

### Bugs/lessons

The scene composite shader's use of `dpdx`/`dpdy` was one of the naga SPIR-V
bugs surfaced in gogpu issue #157 — derivatives produced invalid SPIR-V.
[#GogpuIssues](#gogpuissues)

---

## Stage 9: Dynamic lights (cluster compute)

### Purpose

Divide the camera frustum into a 3D grid of clusters (32×16×32 tiles). A
compute shader determines which lights affect each cluster. The fragment
shader iterates only the lights in its cluster, rather than looping all
lights. This is a modern technique the C renderer does not have.
[#LearningPlan](#learningplan)

### C reference

C Ironwail does not have cluster-forward lighting. It uses a simpler
dynamic light model (OpenGL point lights via `R_AddLights`).

### GoGPU reality

- **Cluster compute pipeline**: `createWorldClusterComputePipeline()` at
  `world_cluster_compute_gogpu.go:13`.
- **Compute shader**: `worldClusterComputeShaderWGSL` at
  `world_compute_shaders_gogpu.go:5`.
- **Dispatch + light upload**: `dispatchWorldClusterCompute()` at
  `world_cluster_compute_gogpu.go:75`, called from `renderWorldInternal`
  at `world_render_gogpu.go:99` — before the world render pass begins.
- **Dynamic light gathering**: `internal/renderer/dynamic_light.go` and
  `dynamic_light_pool.go`.
- **Log-depth setup**: `Core.SetupFrameData()` at `core_gogpu.go:158`
  computes the z-scale/bias for cluster z-slicing.
- **Fragment shader light loop**: `buildWorldFragmentShaderWGSL()` reads
  the cluster bitmask from `lightClusters` (a `texture_3d<u32>`) and
  iterates the assigned lights from the `dynamicLights` storage buffer.

---

## Stage 10: Entities — brush, alias, sprite, decal, viewmodel

### Purpose

Draw everything that isn't the static BSP world: doors and platforms (brush
entities), monsters and items (alias models), explosions and pickups
(sprites), bullet holes (decals), and the first-person weapon (viewmodel).
[#LearningPlan](#learningplan)

### C reference

`R_DrawEntitiesOnList` (`gl_rmain.c:1108`) dispatches by model type:
`R_DrawBrushModels` (`r_world.c:660`), `R_DrawAliasModels`
(`gl_mesh.c`), sprites, etc.

### GoGPU reality

Four sub-pipelines, each with its own shader and pipeline:

| Entity type | Pipeline setup | Render fn | Shader |
| --- | --- | --- | --- |
| Brush entity | `world_gogpu_brush_render.go` | `renderOpaqueBrushEntitiesHAL` | reuses world shaders |
| Alias (MDL) | `world_gogpu_alias.go` | `renderAliasEntitiesHAL` | `AliasVertexShaderWGSL` at `world/gogpu/shaders.go:3` |
| Sprite | `world_gogpu_sprite.go` | `renderSpriteEntitiesHAL` | `SpriteVertexShaderWGSL` at `world/gogpu/shaders.go:82` |
| Decal | `world_gogpu_decal.go` | `renderDecalMarksHAL` | `DecalVertexShaderWGSL` at `world/gogpu/shaders.go:157` |

The **viewmodel** (`renderViewModelHAL` at `world_gogpu_alias.go:593`) is
a special alias-model render with its own depth handling — it draws on top
of the world without depth-testing against it. All of these are orchestrated
in `renderEntities()` at `renderer_gogpu_frame.go:586`, ordered into opaque
→ sky → translucent passes. [#LearningPlan](#learningplan)

### Bugs/lessons

- **Back-face culling**: alias models needed `CullModeFront` (not
  `CullModeBack`) to match OpenGL's back-face culling convention
  (commits `7505c81`, `78a272d`).
- **Alias skin sampler**: needed `REPEAT` wrap mode, not
  `CLAMP_TO_EDGE` (commit `7911202`). Palette index 255 needed to be
  treated as opaque (commit `f0fb2af`).
- **Brush entity cutout faces**: needed alpha-test pipeline with nearest
  sampler (commits `e68aa0c`, `4f5e03b`, `6dfda87`).
- **Pressed button textures**: the frame-1 materials buffer was missing
  entirely — pressed buttons showed their unpressed texture (commit
  `aa17df6`). [#MaterialsDiag](#materialsdiag)

---

## Stage 11: Particles

### Purpose

Particles are camera-facing billboards with a procedural soft-circle
fragment shader. Simulated on the CPU (gravity, decay), uploaded each frame.
[#LearningPlan](#learningplan)

### C reference

`r_part.c` — `R_RunParticle` and `R_DrawParticles`.

### GoGPU reality

- **CPU-side simulation + vertex generation**:
  `internal/renderer/particle.go`.
- **GPU pipeline + shaders**: `particle_gogpu.go`
  (`particleVertexShaderWGSL` at `:20`, `particleFragmentShaderWGSL` at
  `:75`, `ensureParticleResourcesLocked` at `:148`,
  `renderParticlesHAL` at `:354`).
- **Batch capacity**: 512 particles per batch (`particleBatchCapacity`).
- The fragment shader draws a soft circle (radial alpha falloff).
- Particle instances use `@location(0) position` and `@location(1) color`
  per-instance attributes.

---

## Stage 12: Post-processing — scene composite, polyblend, overlay

### Purpose

Render the 3D scene to an offscreen texture, then draw that texture to the
screen with a fullscreen shader that can distort it (underwater warp), tint
it (polyblend), and finally draw the 2D UI on top. [#LearningPlan](#learningplan)

### C reference

C uses OpenGL FBOs for post-processing. The underwater warp is applied via
viewport/scissor tricks. The polyblend is `V_CalcBlend` → `R_SetupView`.
2D overlay is `SCR_UpdateScreen` → `Draw_Console` etc.

### GoGPU reality

Three post passes, in order:

1. **Scene composite** (`compositeSceneRenderTarget()` at
   `warpscale_gogpu.go:472`): blits the offscreen scene render target to
   the swapchain surface, applying the underwater warp if the camera is in
   water. Shaders: `sceneCompositeVertexShaderWGSL` (`:16`),
   `sceneCompositeFragmentShaderWGSL` (`:45`).
2. **PolyBlend** (`renderPolyBlendHAL()` at `polyblend_gogpu.go:224`):
   fullscreen tint. Shaders: `polyBlendVertexShaderWGSL` (`:15`),
   `polyBlendFragmentShaderWGSL` (`:34`).
3. **2D overlay** (`flush2DOverlay()` at
   `renderer_gogpu_overlay.go:32`): HUD/menu/console composited CPU-side
   into a single texture and blitted. Pipeline: `overlay_composite_gogpu.go`
   (`overlayCompositeVertexShaderWGSL` at `:11`,
   `overlayCompositeFragmentShaderWGSL` at `:37`).

All three use the same fullscreen-triangle pattern (vertex positions baked
into the shader via `@builtin(vertex_index)`).

---

## Stage 13: The full frame — `RenderFrame()` top to bottom

### Purpose

Combine all stages into one frame loop. [#LearningPlan](#learningplan)

### GoGPU reality

Reading `RenderFrame()` at `renderer_gogpu_frame.go:82` end to end:

| Frame phase | Code | Stage |
| --- | --- | --- |
| Clear | `:113-129` | Stage 0 |
| Cluster compute dispatch | `world_render_gogpu.go:99` | Stage 9 |
| World BSP render | `renderWorldInternal` `world_render_gogpu.go:16` | Stages 3-8 |
| Opaque brush/alias/sprite/particle entities | `renderEntities` `:586` | Stages 10-11 |
| Translucent water + entities (sorted) | `renderGoGPUSortedTranslucentFaceRendersHAL` | Stage 7 |
| Viewmodel | `renderViewModelHAL` `world_gogpu_alias.go:593` | Stage 10 |
| Scene composite (warp) | `compositeSceneRenderTarget` `warpscale_gogpu.go:472` | Stage 12 |
| PolyBlend | `renderPolyBlendHAL` `polyblend_gogpu.go:224` | Stage 12 |
| 2D overlay | `flush2DOverlay` `renderer_gogpu_overlay.go:32` | Stage 12 |

The `host_speeds 1` cvar enables per-phase timing (`clear_ms`,
`world_ms`, `entities_ms`, `viewmodel_ms`, `scene_composite_ms`,
`polyblend_ms`, `overlay_ms`, `total_ms`) logged each frame. [#README](#readme)

The depth-stencil is cleared before the entities phase
(`:177-188`) so entities can depth-test against the world without
re-rendering the world into the entity pass.

---

## Stage 14 (optional): Order-Independent Transparency

### Purpose

Replace the sorted-translucent pass with weighted-blended transparency
(McGuire & Bavoil 2013), avoiding the back-to-front sort. Enabled by a cvar.
[#LearningPlan](#learningplan)

### C reference

C Ironwail's `ALPHAMODE_OIT` path in `R_BeginTranslucency` — uses an OIT
framebuffer with accumulation and revealage textures, stencil state, and a
final resolve pass.

### GoGPU reality

- **Mode selection**: `internal/renderer/oit_mode.go`.
- **Render path**: `internal/renderer/oit_render_path.go`.
- **Stub**: `internal/renderer/oit_stub.go`.
- **Shared helpers**: `internal/renderer/oit/`.

When enabled, translucent objects render to an accumulation texture +
revealage texture, then composite. This avoids the sort but is optional —
the default path is sorted translucency.

---

## References

<a name="learningplan"></a>[LearningPlan] `docs/RENDERER_LEARNING_PLAN.md`,
ironwail-go repository.

<a name="vertexlayout"></a>[VertexLayout] `docs/VERTEX_LAYOUT.md`,
ironwail-go repository.

<a name="waterdiag"></a>[WaterDiag] `docs/diagnoses/qbj2_water.md`,
ironwail-go repository.

<a name="materialsdiag"></a>[MaterialsDiag]
`docs/diagnoses/qbj2_materials.md`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript
of gogpu/gogpu issues, fetched 2026-07-27).

---

# Chapter 5: The Modding System and the QuakeC VM

Quake's moddability is one of its most enduring legacies. The game logic —
how the shotgun works, how monsters think, how doors open, how triggers
fire — is not in the engine's C code. It is compiled into a `progs.dat`
bytecode file that the engine loads at map start and interprets via a
virtual machine. This chapter explains that VM, the Go port's challenges
with it, and the independent QuakeGo side project.

---

## What QuakeC is

QuakeC is a compiled domain-specific language. It compiles to bytecode that
runs on the QuakeC Virtual Machine (QCVM), a simple stack-based interpreter
embedded in the engine. The language has:

- **One numeric type**: `float` (32-bit). All numeric values — positions,
  velocities, health, flags, even booleans — are `float32`.
- **Strings**: indices into a string table, not heap-allocated Go strings.
- **Vectors**: `vec3` (three consecutive `float32` values in the globals
  array or entity fields).
- **Entities**: integer indices into the edict array.
- **Functions**: indices into a function table. Functions can be QC bytecode
  or builtins (native engine functions dispatched by negative index).

This design enabled a thriving modding community: modders could write
entirely new gameplay without access to the engine source. The engine
provides **builtins** — native functions like `traceline`, `spawn`,
`sound`, `setorigin`, `makevectors` — that QC code calls to interact with
the engine. The engine calls into QC at specific dispatch points:
`StartFrame`, `PlayerPreThink`, `PlayerPostThink`, `touch`, `think`,
`use`, `blocked`, and the client lifecycle functions (`PutClientInServer`,
`ClientConnect`, etc.). [#QCDocs](#qcdocs) [#Hickman](#hickman)

---

## The VM architecture

The Go QCVM lives in `internal/qc/`. The key types are:

### `DProgs` — the `progs.dat` header

```go
type DProgs struct {
    Version       int32 // Must be 6
    CRC           int32
    Statements    int32 // Offset to bytecode statements
    NumStatements int32
    GlobalDefs    int32 // Offset to global definitions
    NumGlobalDefs int32
    FieldDefs     int32 // Offset to field definitions (reflection)
    NumFieldDefs  int32
    Functions     int32 // Offset to function table
    NumFunctions  int32
    Strings       int32 // Offset to string table
    NumStrings    int32
    Globals       int32 // Offset to global variables
    NumGlobals    int32
    EntityFields  int32 // Number of entity fields
}
```

This is the on-disk format, parsed by `LoadProgs`. The C counterpart is
`dprograms_t` in `progs.h`.

### `DFunction` — a function definition

```go
type DFunction struct {
    FirstStatement int32  // Negative for builtins
    ParmStart      int32  // Offset of first parameter
    Locals         int32  // Number of local variables
    Profile        int32  // Profiling counter
    Name           int32  // String table index
    File           int32  // Source file string index
    NumParms       int32
    ParmSize       [MaxParms]byte
}
```

When `FirstStatement` is negative, the function is a builtin: the negative
value indexes into the `Builtins` array. When positive, it is a bytecode
function starting at that statement index.

### `DStatement` — a single bytecode instruction

Each statement is an opcode plus up to three operands (typically offsets
into the globals or entity fields). The opcode categories are:
- Arithmetic: `OPAddF`, `OPSubF`, `OPMulF`, `OPDivF`, etc.
- Comparison: `OPEqF`, `OPNeF`, `OPLE`, `OPLT`, `OPGE`, `OPGT`.
- Control flow: `OPIF`, `OPIFNot`, `OPGoto`, `OPReturn`, `OPDone`.
- Function calls: `OPCall0`–`OPCall8`.
- Memory: `OPLoad*`, `OPStore*`, `OPAddress`.
- Entity state: `OPState`.

### The interpreter loop

`ExecuteProgram` (`exec.go:62`) is the entry point. It dispatches builtins
directly for negative `FirstStatement`, or enters the bytecode loop for
positive. The loop (`:97`) reads `Statements[XStatement]`, switches on
the opcode, and increments `XStatement` at the bottom. The comment at
`:85` notes a subtle difference from C's pre-increment convention.

The runaway-loop limit is `0x1000000` (`exec.go:34`):

```go
const runawayLoopLimit = 0x1000000
```

If the statement count exceeds this, the VM aborts with `"runaway loop
error"`. This is a parity constant — changing it would break mods that
rely on the exact limit. [#QCDocs](#qcdocs)

### Profile counters

Each function has a `Profile` counter. The `profile` console command prints
the top 10 functions by statement count and resets the counters. This is
the engine's built-in QC profiler. [#README](#readme)

---

## Bit-perfect parity concerns

The QCVM must bit-match C's behavior for demo compatibility. Several tests
in `exec_test.go` guard invariants that would break demos or mods:

- **IEEE divide-by-zero**: `1/+0→+Inf`, `-1/+0→-Inf`, `0/+0→NaN`. QC
  programs sometimes divide by zero intentionally as an early-exit
  pattern. The Go `float32` division produces the same IEEE results as C.
  (`TestExecuteProgramDivByZeroBehaviorMatrixMatchesC`)
- **`mod` builtin**: 9 cases of integer/float modulo including sign
  combinations and zero divisors. The Go `%` operator uses truncated
  division and must match C exactly. (`TestModBuiltinBehaviorMatrixMatchesC`)
- **`random()` builtin**: the RNG sequence must be byte-for-byte identical
  for demo playback. Both the fixed and legacy formulas are tested with
  hardcoded expected sequences. (`TestRandomBuiltinMatchesCompatSequence`)
- **Runaway loop limit**: asserted to be exactly `0x1000000`.
  (`TestExecuteProgramRunawayLoopLimitConstantMatchesC`)

[#QCDocs](#qcdocs)

---

## The C shared-memory model

This is the critical architectural fact established in Chapter 1. In C, an
`edict_t` struct contains engine fields *and* an embedded `entvars_t v`
field. The QC bytecode's `OP_LOAD_*` / `OP_STORE_*` instructions read and
write `&ed->v + field_offset` — the **exact same memory** the C engine
code accesses via `ed->v.field`. The macros are pure pointer arithmetic:

```c
#define EDICT_TO_PROG(e)  (int)((byte *)e - (byte *)qcvm->edicts)
#define PROG_TO_EDICT(e)  ((edict_t *)((byte *)qcvm->edicts + e))
#define NEXT_EDICT(e)     ((edict_t *)((byte *)e + qcvm->edict_size))
```

**No sync.** When QC sets `self.nextthink`, the engine sees it
immediately. When the engine sets `ent->v.velocity`, QC sees it
immediately. All entity fields — standard and extension — are accessible
by both C and QC through the same memory. [#QCVM](#qcvm)

---

## The Go dual-storage problem

Go forbids pointer arithmetic and has a garbage collector. The engine
cannot share a raw `byte *` array with the QCVM and have Go structs point
into it. Instead, the Go port has **two separate storage representations**:

```
s.Edicts []*Edict          (Go structs)
  └── Edict.Vars *EntVars  (78 typed fields — Go's source of truth)

s.QCVM.Edicts []byte       (flat byte array, ~105+ fields)
  └── [entNum*EdictSize + 28 + fieldOfs*4]
```

- **Go physics, networking, and area grid** use `EntVars` (the typed
  struct). `ent.Vars.Origin`, `ent.Vars.Solid`, `ent.Vars.Velocity`, etc.
- **QC bytecode** reads/writes `QCVM.Edicts` (the flat byte array) via
  `vm.EFloat(entNum, fieldOfs)`, `vm.EVector(...)`, `vm.SetEFloat(...)`.
- These are **different storage**. The engine must copy data between them
  before and after every QC callback.

### What is synced

The per-edict sync (`syncEdictToQCVM` / `syncEdictFromQCVM` in
`server_qc_sync.go`) uses reflection to copy fields between `EntVars` and
the QCVM byte array. Only 78 of ~105+ QCVM fields are bound to `EntVars`
struct fields and thus synced. Extension fields (`state`, `speed`, `wait`,
`pos1`, `pos2`, `finaldest`, `think1`, `count`, `delay`, `killtarget`,
`trigger_field`, `th_checkattack`, `customflags`, `target2/3/4`) exist
**only in QCVM bytes** — Go physics and networking never read them through
the sync layer (though some are accessible via the direct-VM accessor
methods). [#QCVM](#qcvm)

### The sync layer

`syncAllToQCVM()` (`sync_all.go:47`) copies all `EntVars` fields → QCVM
bytes for every active entity before a QC callback.
`syncAllFromQCVM()` (`sync_all.go:14`) copies all QCVM bytes → `EntVars`
for every active entity after a QC callback. Both are called from
`executeQCFunction` (`qc_trace.go:69`), the **single sync point** for all
QC dispatch.

### The cost: reflection and GC pressure

The sync functions use `reflect.ValueOf(vars).Elem()` and iterate
`entFieldBinding` slices to copy each field. This is O(numEdicts ×
numFields) per QC callback, and it allocates nothing per call (the
bindings are cached), but the reflection overhead is real. The parity
doc's CPU profile of the qbj3 `qbj3_stickflip` map found that QC/server
edict sync paths — `syncEntVarsFromQC`, `syncEntVarsToQC`,
`captureNonPusherQCVMEdictSnapshots`, `syncMutatedNonPushersFromQCVM`,
`SetEFloat` — dominate the profile alongside the QC execution itself.
[#Parity](#parity)

---

## The bug chain: selective sync and the qbj2 lift

The original sync architecture was **selective**: it classified entities
as pushers (`MOVETYPE_PUSH`) or non-pushers and synced them differently
at each of five dispatch points (`touchLinks`, `Impact`, `PhysicsPusher`
think, `executeQCFunction`, `executeQCFunctionLeavingGlobals`). Each
dispatch point captured snapshots, executed QC, and selectively synced
back. This was fragile — "forgot to sync this path" was an entire bug
class.

The qbj2 mod's `start` map exposed this via a lift trigger stack:

1. Player touches `trigger_multiple` → `multi_touch` → `multi_trigger`
   → `SUB_UseTargets`.
2. Finds `func_button` → `button_use` → `button_fire` → `SUB_CalcMove`
   (button starts moving).
3. Button arrives → `button_wait` → `SUB_UseTargets` (with `delay=.5`).
4. Spawns `DelayedUse` entity (`MOVETYPE_NONE`) with `think=DelayThink`.
5. 0.5s later, `DelayedUse` think fires via `RunThink` →
   `executeQCFunction`.
6. `DelayThink` → `SUB_UseTargets` → finds `func_train` → `train_use`
   → `train_next` → `SUB_CalcMove` sets train velocity/nextthink in QCVM.
7. **BUG**: `executeQCFunction` synced non-pushers back but NOT pushers.
8. Train's Go-side velocity/nextthink remain 0 → `PhysicsPusher` never
   moves it. The lift doesn't work.

[#QCVM](#qcvm)

### The fix: unified sync

Commit `fe9e43c` replaced the selective sync with
`syncAllToQCVM`/`syncAllFromQCVM` — all entities, unconditionally, at a
single dispatch point. The fragile pusher/non-pusher classification,
`capturePusherSnapshots`, `syncPushersToQCVM`,
`syncMutatedPushersFromQCVM`, and related functions were deleted (~170
lines of dead code). Callers now just set `self`/`other`/`time` globals
and call `executeQCFunction`. [#QCVM](#qcvm)

### The accessor infrastructure

The same commit added 157 typed accessor methods to `Edict` in
`internal/server/entity_accessors.go`. These read/write the QCVM byte
array directly via `s.QCVM.EFloat(e.Num, qc.EntFieldModelIndex)`,
bypassing `EntVars` entirely. The doc comment is explicit:

> Entity field accessors provide typed read/write access to QCVM entity
> data via the byte array, eliminating the need for the EntVars sync layer.
> These methods read/write directly to `s.QCVM.Edicts[]` — the single
> source of truth — matching C Ironwail's shared-memory architecture.

~27 call sites in `server.go` have migrated to direct-VM access. But the
physics and movement hot paths still use `ent.Vars.*` — the full migration
to accessors is the remaining work (steps 3–5 of the migration plan).

### Hook isolation

A separate bug: a package-level `serverBuiltinHooks` global caused all VMs
to share hooks. A CSQC VM and a server VM would cross-contaminate each
other's callbacks. Fixed by moving hooks to per-VM storage. Tested by
`TestVMServerHooksIsolation`. [#QCDocs](#qcdocs)

---

## The long-term goal: eliminate sync entirely

The migration plan has five steps:

| Step | What | Status |
| --- | --- | --- |
| 1 | Add accessor methods to `Edict`, cache extension field offsets | **Done** (157 accessors) |
| 2 | Migrate hot-path code to accessors | **Partial** — `server.go` has ~27 direct-VM sites, physics/movement still use `EntVars` |
| 3 | Remove sync functions (delete `server_qc_sync.go`, simplify `qc_trace.go`) | **Partially done** — old selective sync removed, sync-all replaces it |
| 4 | Remove `EntVars` struct (rewrite `savegame.go` for QCVM bytes) | **Not done** |
| 5 | Simplify callback dispatch (match C exactly — no sync, just set globals and execute) | **Not done** — sync-all still runs at every callback |

When steps 3–5 complete, `EntVars`, `syncAllToQCVM`,
`syncAllFromQCVM`, and `server_qc_sync.go` will be deleted entirely. The
`executeQCFunction` wrapper will simplify to just save/restore
`self`/`other`/`time` globals and execute — matching C's zero-sync model
exactly. The accessor infrastructure is in place; the remaining work is
migrating the hot paths. [#QCVM](#qcvm)

---

## CSQC: client-side QuakeC

The QC package also supports CSQC (Client-Side QuakeC), a specialized
wrapper for client-side logic: custom HUD rendering, client-side effects,
and input handling. The `CSQC` type in `internal/qc/` loads a separate
`csprogs.dat` and provides hooks for `CallDrawHud`, `CallDrawOverlay`,
and client event dispatch.

CSQC runtime integration is currently **deferred** — the repo has CSQC
wrapper infrastructure, but host/client runtime wiring for a full CSQC
gameplay path is outside the current parity milestone. [#Parity](#parity)
The tests in `csqc_test.go` verify construction, loading, precache
registry behavior, and global sync, but no e2e CSQC gameplay path is
wired.

---

## QuakeGo: the side project (not used in the engine)

**Critical clarification:** `pkg/qgo` (the `qgo` compiler and `QuakeGo`
gameplay source) is an **independent side project** to explore porting the
QuakeC *language* to a Go-dialect variant. It is **not wired into the
engine**. The engine runs the **original** `progs.dat` bytecode compiled
from the original QuakeC sources — there are **no tests or e2e runs of the
game using a QuakeGo-compiled `progs.dat`**.

### What QuakeGo is

`qgo` is a compiler in `cmd/qgo/` that takes a Go package and emits
Quake `progs.dat` bytecode for the QCVM. QuakeGo is the Go subset and
runtime surface used by that compiler:

- `pkg/qgo/quake` — core QCVM-facing types (`Entity`, `Vec3`, `Func`) and
  engine builtin stubs (`pkg/qgo/quake/engine/`).
- `pkg/qgo/quakego` — translated gameplay package proving the model works
  against real Quake game logic.

The mental model from the QGo guide: *"QuakeGo is not 'full Go running on
Quake.' It is a deliberately narrow Go subset that maps cleanly onto
QuakeC VM concepts."* [#QGoGuide](#qgoguide) The supported types are
`float32`, `string`, `bool`, `quake.Vec3`, `*quake.Entity`, and function
values. Struct fields tagged for qgo map to entity fields. Methods are
lowered to QCVM-compatible functions. Engine calls are expressed as imports
from `quake/engine`.

### Why it is a separate module

`pkg/qgo/quake` and `pkg/qgo/quakego` are **separate Go modules** with
their own `go.mod` files. The root module does not require or replace them.
They cannot be imported by the engine. This is intentional: `pkg/qgo/quakego`
is QuakeGo source (a Go dialect compiled to QCVM `progs.dat` bytecode by
`cmd/qgo`), not regular Go library code. From the repo root, gopls/LSP may
report `BrokenImport` errors for these packages — these are expected.
[#AGENTS](#agents)

### The mechanical-port convention

`pkg/qgo/quakego` intentionally mirrors original QuakeC/progs source
structure. The QGo guide is explicit: *"Avoid cosmetic Go-idiom rewrites
(tagged switches, merged var decls) there — they drift the port from
`progs.src` and make resync harder."* `.golangci.yml` suppresses `unused`,
`SA4017`, `QF1003`, and `S1021` for that package for the same reason.
[#AGENTS](#agents) This is a *resync* concern specific to the QuakeGo side
project, not the engine — the engine itself uses the original QC bytecode.

### How to use it

```bash
go build -o qgo ./cmd/qgo
cd pkg/qgo/quakego
../../../qgo
```

Or via mise: `mise run build-progs`. The `qgo` CLI also has a
`source-order` utility for deterministic function/file ordering. But the
output `progs.dat` is never loaded by the engine — the engine loads the
original `progs.dat` from the Quake data directory.

---

## References

<a name="qcdocs"></a>[QCDocs] `docs/internal/qc.md`, ironwail-go repository.

<a name="qcvm"></a>[QCVM] `docs/QCVM_ENTITY_SYNC.md`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="qgoguide"></a>[QGoGuide] `docs/QGO_QUAKEGO_GUIDE.md`, ironwail-go
repository.

<a name="hickman"></a>[Hickman] Zachary Hickman, *"Quake Engine Analysis,"*
Northeastern University. Local copy: `article/analysisfinal.pdf`.

---

# Chapter 6: GoGPU — Pure-Go WebGPU in Practice

The decision to use a pure-Go WebGPU stack is the defining technical gamble of
`ironwail-go`. The README states it as a first principle: *"gogpu/WebGPU as the
canonical gameplay renderer/runtime."* [#README](#readme) The project compiles
with `CGO_ENABLED=0`. There is no C in the runtime path — not in the renderer,
not in the audio, not in the windowing. This chapter is a field report on what
that actually means, using the real bugs, issues, and lessons encountered over
the course of the port.

---

## The GoGPU module family

GoGPU is not a single library. It is a family of Go modules, each with a
specific role in the WebGPU stack:

| Module | Version | Role |
| --- | --- | --- |
| `github.com/gogpu/gogpu` | v0.44.1 | High-level renderer: `App`, event loop, window, surface, input |
| `github.com/gogpu/gpucontext` | v0.21.0 | Event source abstraction: keyboard, mouse, resize, focus, IME |
| `github.com/gogpu/gputypes` | v0.5.1 | Type definitions: vertex formats, blend states, bind group layouts |
| `github.com/gogpu/naga` | v0.17.15 | WGSL → SPIR-V shader compiler (Go port of the naga project) |
| `github.com/gogpu/wgpu` | v0.30.10 | Low-level WebGPU bindings (Instance, Device, Queue, buffers, textures) |
| `github.com/go-webgpu/goffi` | v0.5.6 (indirect) | FFI layer for native library calls without CGO |
| `github.com/go-webgpu/webgpu` | v0.5.2 (indirect) | Underlying WebGPU API definitions |

The dependency graph is: `gogpu` → `gpucontext` (events) + `wgpu` (GPU
primitives) + `gputypes` (type defs). `wgpu` → `goffi` (cgo-free native FFI)
+ `webgpu` (API types). `naga` is used at shader compilation time to translate
WGSL source (Go string constants) into SPIR-V bytecode that the Vulkan backend
can consume. None of these require CGO — the native library FFI is done via
`purego` (cgo-free dynamic loading of shared libraries).

---

## The WGSL → SPIR-V pipeline via naga

WebGPU shaders are written in WGSL (WebGPU Shading Language). But the native
Vulkan backend does not consume WGSL directly — it needs SPIR-V bytecode. The
`naga` module is the compiler that bridges this gap: it parses WGSL, builds an
intermediate representation, and emits SPIR-V. This happens at pipeline creation
time, when the Go code calls `device.CreateRenderPipeline` with a shader module
compiled from a WGSL string constant.

Every shader in `ironwail-go` is a Go string constant:

```go
const worldVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    // ...
}
@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    // ...
}
`
```

When the pipeline is created, `naga` compiles this string to SPIR-V, and the
resulting shader module is bound into the pipeline. If `naga` produces invalid
SPIR-V, the pipeline creation may succeed (naga does not always validate against
the SPIR-V spec) but the GPU driver will crash or produce garbage at draw time.

This is exactly what happened.

---

## Bug: naga invalid SPIR-V for scalar `mix()` (issue #162)

### The problem

The SPIR-V spec requires all `FMix` operands to have matching types. When
WGSL's `mix()` function uses a scalar blend factor (`f32`) with `vec3<f32>`
operands, naga v0.15.2 emitted an `FMix` instruction with mismatched operand
types — a `vec3` and a scalar `float`. AMD's RADV driver tolerated this on the
integrated GPU, but running with `DRI_PRIME=1` to enforce the discrete NVIDIA
GPU crashed with `SIGSEGV at addr=0x10`. [#GogpuIssues](#gogpuissues)

### The workaround

Commit `d5ff084` splatted the scalar to a `vec3` explicitly:

```go
// Before (crashed on NVIDIA):
result = vec4<f32>(mix(result.rgb, uniforms.fogColor, uniforms.fogDensity), 1.0);

// After (workaround):
result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), 1.0);
```

This appears in `world_shaders_gogpu.go:637` and `:723`. The explicit
`vec3<f32>(...)` splat forces naga to emit a correct
`OpCompositeConstruct` + `FMix` sequence.

### The fix

naga v0.17.0+ fixed the scalar-to-vector splat automatically. The fix
produces:

```spirv
%22 = OpLoad %float %fog_ptr
%23 = OpCompositeConstruct %v3float %22 %22 %22    ; scalar → vec3 splat
%24 = OpExtInst %v3float FMix %a %b %23            ; all operands vec3 ✅
```

After upgrading to naga v0.17.15 (the current `go.mod` version), the
workaround is no longer necessary, though the explicit splat remains in the
shader as defensive coding. [#GogpuIssues](#gogpuissues)

---

## Bug: naga swizzle gap (issue #157 comments)

### The problem

naga's WGSL parser could not handle swizzle expressions in certain
contexts. The particle vertex shader used a writable swizzle compound
assignment that triggered `ExprSwizzle is not a pointer expression` in naga.
This prevented the GoGPU renderer from compiling its shaders at all — no
visuals, just a crash. [#GogpuIssues](#gogpuissues)

### The workaround

Commits `de40302` and `ef8f1c0` replaced writable swizzle compound
assignments with explicit vector reconstruction. The particle shader was
rewritten to avoid swizzles entirely. This was the breakthrough that
produced the first actual visuals on the GoGPU renderer — the screenshot in
issue #157 shows a rendered Quake scene after the swizzle fixes.

### The response

gogpu maintainer kolkov confirmed both the swizzle gap and the
`dpdx`/`dpdy`/`textureDimensions` SPIR-V issue (which affected the scene
composite fragment shader) as naga bugs, filed them as naga #45 and #46, and
said: *"These are exactly the real-world 3D patterns we were missing in our
test coverage."* [#GogpuIssues](#gogpuissues)

---

## Bug: Linux X11 input stub (issue #129)

### The problem

The X11 key handling code in gogpu's Linux platform layer was just a stub —
no keyboard events were delivered. Mouse input was also absent. This was the
first input blocker that forced the cgo-GLFW detour (Chapter 2).

### The fix

Fixed in gogpu v0.22.8. The `InputBackend` in
`internal/renderer/gogpu/input_backend.go` bridges gogpu's `gpucontext` event
source to the engine's `internal/input.Backend` interface. It uses callback-based
input (`OnKeyPress`, `OnMouseMove`, etc.) when available, and falls back to
polling `b.app.Input().Keyboard()` / `.Mouse()` state otherwise. The polling path
has a heartbeat log to detect silent input failures. [#GogpuIssues](#gogpuissues)

### The input architecture

The Go port's input is layered:

1. **Platform polling**: gogpu's `gpucontext` polls X11/Wayland events.
2. **Backend adaptation**: `InputBackend` (`input_backend.go`) translates
   `gpucontext` events into Quake key codes via `input_map.go`
   (`MapGPUContextMouseButton`, key code mappings).
3. **Input system**: `internal/input.System` normalizes events and dispatches
   based on `KeyDest` (console, menu, game).
4. **Game handler**: `internal/game/game_input.go` routes keys to the
   appropriate subsystem.

This keeps the higher-level game code unaware of gogpu/X11/Wayland details,
matching the C engine's separation of `in_sdl.c` from `keys.c`.

---

## Bug: no pointer lock / mouse grab (issues #173, #175)

### The problem

An FPS requires pointer lock (mouse grab) to look around without the cursor
leaving the window. gogpu had no API for this. On Wayland, the pointer
constraints protocol implementation did not exist at all.

### The resolution

Issue #173 asked for the feature; issue #175 pointed to
[`libwldevices-go`](https://github.com/bnema/libwldevices-go) as a potential
Wayland implementation dependency. Both were closed after gogpu added pointer
lock support. [#GogpuIssues](#gogpuissues)

---

## Bug: adapter power preference ignored (issue #176)

### The problem

On hybrid-GPU Linux systems (integrated + discrete, e.g., Intel + NVIDIA
laptops), the windowed renderer did not forward a power preference to
`RequestAdapter`. This meant the runtime might select the discrete NVIDIA
adapter even when the application explicitly requested low-power/integrated.
`DRI_PRIME` environment variables are not a reliable substitute.

### The fix

The issue proposed adding a `PowerPreference` field to `gogpu.Config` and
forwarding it through `RequestAdapter`. The `Core` struct in
`core_gogpu.go:46` now has `GPUPreference` in `CoreConfig`, with
`DefaultCoreConfig()` returning `GPUPreferHighPerformance`. The `CoreConfig`
is the Go-side mechanism for this — the engine can expose a user-facing GPU
preference cvar and pass it through. [#GogpuIssues](#gogpuissues)

---

## The Wayland two-connection bug (BUG-GOGPU-002)

This is the defining architectural bug of the gogpu stack, and the one that
caused the most frustration during the port. It is documented in the issue
#157 comment thread by gogpu maintainer kolkov. [#GogpuIssues](#gogpuissues)

### The problem

gogpu used two separate `wl_display_connect()` calls from the same process:

- **Pure Go connection** — owned `wl_seat`, `wl_pointer`, `wl_keyboard`
  (where gogpu listened for input events).
- **C libwayland connection** (via goffi) — owned the visible `wl_surface`
  + `xdg_toplevel` (where Vulkan rendered).

Wayland delivers input events to the connection that owns the focused
surface. The window was on the C connection. The input listeners were on
the Pure Go connection. **They never met.** No mouse, no keyboard, no input
of any kind registered.

### Why it worked on X11

On X11, window IDs are server-side — they are shared across connections.
Two X11 connections to the same display server can both see the same
window. Wayland surfaces are client-side — they are scoped to the
connection that created them. The dual-connection design that worked on X11
was fundamentally broken on Wayland.

### The verification

kolkov verified that no toolkit does this: *"GLFW, Gio, winit,
neurlang-wayland — all use a single connection."* The gogpu stack had gotten
away with it because X11's server-side window IDs masked the architectural
error.

### The fix

Bind `wl_seat` + `wl_pointer` + `wl_keyboard` on the C connection and
forward events to Go. gogpu's CSD (client-side decoration) code already did
exactly this for pointer events on decoration subsurfaces — it needed to be
generalized to the main surface. Tracked as BUG-GOGPU-002 (P0).
[#GogpuIssues](#gogpuissues)

### The lesson for engine authors

This bug is not just a gogpu bug. It is a lesson in what happens when a
cross-platform abstraction assumes platform semantics that do not hold. The
X11/Wayland split in Linux desktop graphics is a minefield, and the
"pure-Go, no-CGO" constraint makes it harder, not easier, because the FFI
boundary between Go and C libraries is where these connection-scope issues
live.

---

## The cgo-GLFW detour and return

Chapter 2 covered this at a high level. Here is the gogpu-specific arc:

1. **2026-02-24**: Commit `064c027` — "renderer: port WebGPU core
   initialization." The first GoGPU commit. One day later...
2. **2026-02-25**: Commit `15b888e` — "alternate cgo gl renderer." The
   detour begins. gogpu hits naga swizzle bugs and Wayland input failures.
   A cgo-based OpenGL renderer using GLFW for windowing becomes the
   working path.
3. **2026-03-28**: Commits `de40302` and `ef8f1c0` — swizzle workarounds
   land. GoGPU shaders compile. First visuals appear.
4. **2026-04-01**: Commit `d5ff084` — scalar `mix()` splat workaround for
   NVIDIA SPIR-V crash. Issue #162 filed.
5. **2026-04-05**: Commit `b2fb6e9` — "Retire gl+sdl (#11)." The OpenGL
   renderer, SDL input, and SDL audio are removed. GoGPU becomes the sole
   renderer. Oto becomes the canonical audio backend.
6. **2026-04-21**: Issue #157 updated with screenshot showing actual
   gameplay visuals on GoGPU.
7. **2026-04-24**: Commit `889f797` — "drop renderer shims." Game loop
   cleanup, final shim removal.

The gogpu issue #157 opening body captures the state at the detour's peak:

> I first attempted to tackle things using GoGPU as the rendering backend,
> but eventually hit enough issues that I sadly switched to cgo GLFW code.
> [#GogpuIssues](#gogpuissues)

The return was driven by naga fixes (swizzle, scalar `mix()`), the X11 input
fix (v0.22.8), and the decision that pure-Go was worth the remaining pain.

---

## General state of pure-Go graphics in 2026

### Where it is strong

- **No CGO**: The entire stack compiles with `CGO_ENABLED=0`. No C toolchain
  required. Cross-compilation is trivial. Static binaries. This is the
  primary value proposition — `ironwail-go` is a real 3D game engine with
  no C in its runtime.
- **WebGPU portability**: WebGPU is designed as a cross-platform API. The
  same WGSL shaders run on Vulkan (Linux), Metal (macOS), and D3D12
  (Windows). The browser target (WASM + WebGPU) is a future possibility.
- **Real-world validation**: `ironwail-go` is the largest real-world 3D
  engine running on the pure-Go GPU stack. gogpu issue #163 ("Ironwail-go
  demo") is the showcase thread. The gogpu maintainers use it as evidence
  that the stack can handle a real engine, not just toy examples.
  [#GogpuIssues](#gogpuissues)
- **Active development**: The gogpu maintainer (kolkov) is responsive and
  uses `ironwail-go` bug reports to prioritize naga and platform fixes.

### Where it is weak

- **naga maturity**: The WGSL → SPIR-V compiler has had real gaps in
  coverage. Swizzle expressions, scalar-vector `mix()`, derivatives
  (`dpdx`/`dpdy`), and `textureDimensions` all produced invalid SPIR-V at
  various points. Each was fixed, but each required a workaround until the
  fix landed. Engine authors must be prepared to read SPIR-V disassembly and
  file compiler bugs.
- **Wayland/windowing**: The two-connection bug (BUG-GOGPU-002) is the
  most severe example, but the broader issue is that Linux desktop
  windowing (X11 vs Wayland, pointer lock, IME, multi-layout) is a large
  surface area with many compositors and edge cases. gogpu issue #227
  (multiple keyboard layouts) is another example — 75 comments of
  discussion about X11 keyboard group handling.
- **Driver conformance variance**: AMD/RADV silently tolerates invalid
  SPIR-V that crashes NVIDIA. This means bugs can be hardware-specific and
  invisible until tested on multiple GPUs. The `DRI_PRIME=1` workflow is
  essential for hybrid-GPU testing.
- **Tooling**: GoGPU debugging is harder than OpenGL debugging. There is no
  equivalent to `apitrace` or `RenderDoc` that works smoothly with the
  pure-Go stack. The project built its own diagnostic tooling: `bspdiag`
  for offline BSP inspection, `r_debug_water` for per-frame liquid face
  telemetry, `r_debug_passes` for render pass tracing, and `host_speeds`
  for per-phase timing.
- **Documentation**: WebGPU is newer than OpenGL, and the pure-Go stack is
  newer than WebGPU. The `docs/RENDERER_LEARNING_PLAN.md` was written
  precisely because there was no existing curriculum for learning WebGPU
  via a real Go codebase.

### Lessons for engine authors

1. **Avoid swizzles in WGSL shaders.** Write explicit vector
   reconstruction. It is more verbose but survives naga parser gaps.
2. **Prefer explicit splats.** `vec3<f32>(scalar)` is safer than relying on
   implicit scalar-to-vector promotion in `mix()`, `clamp()`, etc.
3. **Validate SPIR-V on multiple drivers.** What RADV tolerates, NVIDIA
   crashes on. Test on both if possible.
4. **Build your own diagnostic tooling.** `r_debug_water`, `bspdiag`,
   `host_speeds` — these exist because standard graphics debuggers do not
   integrate smoothly with the pure-Go stack.
5. **Expect platform bugs to be architectural, not superficial.** The
   Wayland two-connection bug was not a missing feature; it was a
   fundamentally broken design that happened to work on X11.
6. **Contribute upstream.** Every bug filed against gogpu/naga was fixed.
   The pure-Go graphics stack improves because real engines stress-test it.
   `ironwail-go` is that stress test.

---

## References

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript
of gogpu/gogpu issues, fetched 2026-07-27).

---

# Chapter 7: Synthesis — What Was Learned, and Where It Goes

`ironwail-go` began as an experiment driven by three motives: nostalgia for
school days spent hacking Quake mods, a desire to test modern AI agentic coding
capabilities on a non-trivial codebase, and a technical curiosity to see if a
1996 3D engine could be re-architected into pure, safe Go with a WebGPU renderer
and zero C dependencies. [#README](#readme)

Six chapters later, the engine runs. It loads BSP maps, executes QuakeC bytecode,
simulates player and monster physics, streams spatialized audio via Oto, and
renders 3D world geometry, alias models, sprites, particles, and liquids through GoGPU and WebGPU at high frame rates with `CGO_ENABLED=0`.

This final chapter ties the threads together. It reflects on what the port
validated about engine architecture, what it exposed about Go and WebGPU as a
systems graphics stack, what multi-agent agentic coding teaches about AI-assisted
engineering, and where the project goes next.

---

## What the port validated

### 1. Quake's architecture is remarkably durable

Thirty years after Carmack, Abrash, and the id Software team wrote original Quake,
its fundamental architectural patterns remain exceptionally well-suited for game
engines:

- **The client-server split** remains the gold standard for network simulation.
  Enforcing that single-player is simply a local server connected via a loopback
  channel ensures that prediction, delta compression, and state synchronization are
  never retrofitted — they are structural.
- **BSP and PVS visibility culling** continues to excel. On `qbj3_stickflip`, a
  massive map with 85,936 raw faces and 22,195 leaves, the PVS lookup instantly
  reduces the first rendered frame to just 1,002 visible faces. [#Parity](#parity)
- **The command system** (`cmdsys`) as a unified control path for keybindings,
  console execution, menus, configuration files, and script automation remains
  unmatched for engine debuggability.
- **Bytecode scripting via VM** keeps game rules isolated from engine internals,
  allowing modding without engine re-compilation.

Moving from C to Go, or OpenGL to WebGPU, does not invalidate these core patterns.
If anything, re-implementing them in a memory-safe language with strong package
boundaries highlights just how clean and decoupled Quake's original high-level
design was.

### 2. Behavioral parity as a discipline works

The project adopted a strict **parity-first discipline**: preserve behavioral
parity with canonical C Ironwail/Quake unless a divergence is explicitly intended.
This discipline was enforced not by wishful thinking, but by structural practices:

- **`// Where in C:` citations** in unit and integration tests, tying Go test
  assertions directly to function names in `ironwail/Quake/*.c`.
- **Descriptive parity test names** like
  `TestExecuteProgramRunawayLoopLimitConstantMatchesC`, documenting the exact
  invariant being protected.
- **Visual parity harness** (`mise run parity-compare`), running deterministic
  visual diffs between C Ironwail reference frames and GoGPU rendered frames.
- **The Quake Brutalist Jam (qbj) maps** as unforgiving integration benchmarks.
  Synthetic tests in small rooms pass easily; maps like `qbj2` and `qbj3` test
  every boundary condition simultaneously.

Without this discipline, a port of this scale rapidly devolves into "looks roughly
right," where subtle physics bugs, trigger sequence breaks, or rendering glitches
multiply uncontrollably.

### 3. Pure-Go graphics is viable today

Compiling a full 3D game engine with `CGO_ENABLED=0` was considered improbable
only a few years ago. `ironwail-go` proves that a pure-Go graphics stack —
`gogpu/gogpu` for windowing and event loops, `gogpu/wgpu` for low-level WebGPU
primitives, `gogpu/naga` for WGSL-to-SPIR-V translation, and `ebitengine/oto` for
audio — can drive a complex, real-time 3D rendering pipeline without a single line
of C code in the runtime path.

### 4. The educational mandate is achievable

A primary goal of the codebase was to make it **self-explanatory and educational**
— readable by someone without prior deep graphics or engine development experience.
This mandate manifested in concrete design artifacts:

- Per-package `doc.go` files with `# Original C lineage` maps.
- Detailed subsystem docs in `docs/internal/*.md`.
- A 14-stage stage-by-stage WebGPU curriculum in `docs/RENDERER_LEARNING_PLAN.md`.
- `cmd/bspdiag`, an offline inspection CLI allowing developers to inspect BSP
  lumps, lightmap pages, entity definitions, and liquid settings without writing
  scratch scripts.

The result is a codebase that serves as a working textbook for Quake engine
internals and WebGPU graphics programming in Go.

---

## What the port exposed

Re-architecting a 1996 C engine into 2026 Go also exposed significant friction
points and architectural taxes:

### 1. Garbage collection pressure in hot paths

Quake's C memory model relied on the `Hunk`: a single contiguous memory block
where geometry, lightmaps, models, and temp buffers were bump-allocated and wiped
all at once on map change.

Go's garbage collector provides memory safety, but allocating per-frame slices or
temporary objects in high-framerate loops (250 FPS) generates significant GC pressure. Profiling under `qbj3` revealed hot spots in per-frame rendering allocations and string conversions. Mitigations — such as scratch buffers on `Renderer`, `sync.Pool` for dynamic lights, `unsafe.Slice` for zero-allocation byte conversions, and RLocking shared maps — were required to maintain smooth frame rates.

### 2. The QCVM dual-storage sync tax

In C, `edict_t` structs and the QCVM memory space share the exact same memory;
pointer arithmetic connects engine code (`ed->v.velocity`) and bytecode
(`OP_STORE_F`).

Because Go forbids pointer arithmetic and requires type safety, `ironwail-go`
operates with **dual storage**: typed Go structs (`Edict.Vars`) for engine physics/networking, and a flat `QCVM.Edicts []byte` array for VM bytecode. Syncing data back and forth via reflection (`syncAllToQCVM` / `syncAllFromQCVM`) at every QuakeC callback introduces an O(numEdicts × numFields) tax. The `qbj3` CPU profiles showed that edict synchronization is one of the heaviest CPU consumers in the entire server frame.

While the unified sync fixed fragile selective-sync bugs (like the `qbj2` lift trigger failure), the long-term resolution requires completing the migration to direct-VM accessor methods (`Edict.Velocity()`, `Edict.SetVelocity()`), deleting `EntVars` and `server_qc_sync.go` entirely to achieve C's zero-sync model. [#QCVM](#qcvm)

### 3. Naga compiler and desktop windowing maturity

Building on a pure-Go WebGPU stack placed `ironwail-go` on the bleeding edge of the `gogpu` ecosystem, uncovering early platform gaps:

- **Naga SPIR-V compilation bugs:** Scalar `mix()` emitted invalid SPIR-V that
  crashed NVIDIA drivers (issue #162), and writable swizzles failed to parse
  (issue #157).
- **Linux desktop windowing & Wayland:** The Wayland two-connection bug
  (BUG-GOGPU-002) — where X11's server-side window IDs masked an architecture where
  input listeners ran on a different connection than the rendering surface — was a
  major platform lesson. Linux X11 input stubs (issue #129) and missing pointer
  lock APIs (issues #173, #175) required rapid upstream collaboration.

### 4. Stress testing via Brutalist Jam maps

The Quake Brutalist Jam map packs (`qbj2`, `qbj3`) served as unforgiving stress
tests. `qbj2_start` surfaced the 256-entry material atlas uniform buffer limit
(causing silent overflows when maps exceed 254 textures) and complex pusher/lift
sync breaks. `qbj3_stickflip` pushed face counts (85,936 raw faces) and dynamic
lighting to limits that exposed rendering and CPU bottlenecks that clean standard maps (`id1/e1m1`) never triggered.

---

## Reflection on multi-agent agentic coding

`ironwail-go` was developed as an agentic coding experiment under the "Senior-Junior" partnership model codified in `AGENTS.md` — the human engineer acts as architect and reviewer, while AI agents perform code translation, refactoring, and test writing. [#AGENTS](#agents)

Crucially, **the project was not built by a single AI model.** Work was distributed across multiple agents over the course of the port:

```
+-------------------------------------------------------------------------+
|                         HUMAN ARCHITECT & REVIEWER                      |
|            (Architecture, TDD red/green, PR review, Parity verification) |
+-------------------------------------------------------------------------+
                                     |
    +-------------------+------------+------------+-------------------+
    |                   |                         |                   |
    v                   v                         v                   v
GitHub Copilot     Claude Opus 4.6           GPT-5.4             GLM-5.2 / Gemini
(700+ commits)     (Primary Agent)           (Primary Agent)     (Renderer & Module
Early C->Go &      GoGPU Renderer,           Deep logic,          fix passes, QGo
cgo-GLFW era       QCVM Sync Unification     Refactoring          cleanup)
```

- **GitHub Copilot:** The original workhorse, co-authoring over 700 commits during
  the early C-to-Go transliteration and the initial cgo/GLFW/OpenGL phase.
- **Claude Opus 4.6 & GPT-5.4:** Carried the majority of complex agentic work,
  including building out the 14-stage GoGPU WebGPU renderer, designing the 48-byte
  `WorldVertex` contract, implementing cluster compute dynamic lighting, and
  unifying the QCVM entity synchronization layer.
- **GLM-5.2 (via Crush CLI):** Contributed to targeted renderer fix passes in mid-2026.
- **Gemini:** Assisted with specialized refactoring passes, such as the `pkg/qgo`
  module boundary cleanup.

### Lessons learned in agentic engineering

1. **Agents require an objective definition of "Done":** Without failing tests
   (Red/Green TDD) or empirical verification harnesses, agents easily produce
   "AI slop" — code that compiles but subtly breaks runtime contracts, swallows
   errors, or introduces superficial symptom patches.
2. **Specialized sub-agent tasking keeps context clean:** Assigning isolated
   sub-tasks (e.g., "port `sv_phys.c` walkmove while matching C test signatures")
   yields far higher precision than broad, multi-subsystem requests.
3. **Agent variance is real:** Different models exhibit distinct strengths.
   Copilot excelled at rapid line-by-line translation; Claude Opus 4.6 and GPT-5.4
   excelled at multi-file architectural refactoring and root-cause debugging;
   smaller or faster models worked well for localized fix passes.
4. **Code is disposable:** In complex debugging sessions (such as the initial
   Wayland input failures or selective QC sync bugs), reverting to a last stable
   commit and re-prompting with a clearer plan proved vastly superior to patching a
   degraded agent trajectory.

---

## Future directions

While `ironwail-go` is a fully functional engine today, several concrete
architectural goals remain on the horizon:

### 1. Browser port (WASM + WebGPU)

Because the canonical renderer is built on WebGPU and the engine is pure Go (`CGO_ENABLED=0`), porting `ironwail-go` to the web is a natural next step. Compiling to WebAssembly (`GOOS=js GOARCH=wasm`) and binding the GoGPU renderer directly to the browser's native `navigator.gpu` surface will enable a zero-install, full-performance Quake engine playing directly inside modern web browsers.

### 2. Arena/Region allocators for map lifetimes

To eliminate GC pressure during gameplay, future work will investigate Go-based
arena/region allocators (e.g., Go's `arena` proposals or custom byte-slice region pools). Allocating map geometry, BSP nodes, textures, and models into a region pool that is discarded in a single operation upon level change will bring the memory model back to the zero-GC-overhead efficiency of Quake's original `Hunk`.

### 3. Direct-VM accessors & zero-sync QCVM

Completing steps 3–5 of the QCVM migration plan:
- Migrate remaining physics and movement loops in `internal/server/` from
  `ent.Vars.*` to direct-VM accessor methods (`ent.Origin()`, `ent.SetOrigin()`).
- Delete `EntVars` and `internal/server/server_qc_sync.go`.
- Remove `syncAllToQCVM` and `syncAllFromQCVM` calls from `executeQCFunction`.

This will achieve C Quake's zero-sync architecture, eliminating the reflection overhead and matching native VM performance. [#QCVM](#qcvm)

### 4. Continued parity closure & CSQC integration

- **Texture atlas storage upgrade:** Replace the uniform buffer materials array with
  a storage buffer (`var<storage, read> materials`) to remove the hardcoded
  256-texture limit, fully resolving the `qbj2` atlas overflow bug. [#MaterialsDiag](#materialsdiag)
- **CSQC wiring:** Complete host and client runtime integration for Client-Side
  QuakeC (`csprogs.dat`), bringing full support for custom mod HUDs and client-side
  predicted entities.
- **`qbj3_stickflip` sign-off:** Resolve remaining lighting contrast deltas and
  z-fighting edge cases to achieve official parity sign-off on the `qbj3` stress pack.

---

## Conclusion: Quake as a forever-benchmark

Quake occupies a unique position in software engineering. Like Ray Casting or
`Hello World`, it has become a timeless benchmark for testing new programming
languages, paradigms, and graphics APIs.

`ironwail-go` demonstrates that 1996 engine architecture and 2026 Go technology can
meet harmoniously. By replacing manual memory with garbage collection, C headers
with Go packages, immediate-mode OpenGL with WebGPU pipelines, and manual coding with human-guided multi-agent engineering, the project breathes new life into classic software.

Even as features are added and parity gaps close, the codebase's lasting value remains its **educational artifact**: a clean, documented, memory-safe, pure-Go implementation of one of the most influential game engines ever written.

---

## References

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="qcvm"></a>[QCVM] `docs/QCVM_ENTITY_SYNC.md`, ironwail-go repository.

<a name="materialsdiag"></a>[MaterialsDiag] `docs/diagnoses/qbj2_materials.md`, ironwail-go repository.
