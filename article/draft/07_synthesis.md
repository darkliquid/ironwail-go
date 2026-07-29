# Chapter 7: Synthesis — What Was Learned, and Where It Goes

`ironwail-go` began as an experiment driven by three motives: nostalgia for
school days spent hacking Quake mods, a desire to test modern AI agentic coding
capabilities on a non-trivial codebase, and a technical curiosity to see if a
1996 3D engine could be re-architected into pure, safe Go with a WebGPU renderer
and zero C dependencies. [README](#ref-readme)

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
  reduces the first rendered frame to just 1,002 visible faces. [Parity](#ref-parity)
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

While the unified sync fixed fragile selective-sync bugs (like the `qbj2` lift trigger failure), the long-term resolution requires completing the migration to direct-VM accessor methods (`Edict.Velocity()`, `Edict.SetVelocity()`), deleting `EntVars` and `server_qc_sync.go` entirely to achieve C's zero-sync model. [QCVM](#ref-qcvm)

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

`ironwail-go` was developed as an agentic coding experiment under the "Senior-Junior" partnership model codified in `AGENTS.md` — the human engineer acts as architect and reviewer, while AI agents perform code translation, refactoring, and test writing. [AGENTS](#ref-agents)

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

This will achieve C Quake's zero-sync architecture, eliminating the reflection overhead and matching native VM performance. [QCVM](#ref-qcvm)

### 4. Continued parity closure & CSQC integration

- **Texture atlas storage upgrade:** Replace the uniform buffer materials array with
  a storage buffer (`var<storage, read> materials`) to remove the hardcoded
  256-texture limit, fully resolving the `qbj2` atlas overflow bug. [MaterialsDiag](#ref-materialsdiag)
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

<a id="ref-agents"></a>[AGENTS] [`AGENTS.md`](../../AGENTS.md), ironwail-go repository.

<a id="ref-materialsdiag"></a>[MaterialsDiag] [`docs/diagnoses/qbj2_materials.md`](../../docs/diagnoses/qbj2_materials.md), ironwail-go repository.

<a id="ref-parity"></a>[Parity] [`docs/PARITY.md`](../../docs/PARITY.md), ironwail-go repository.

<a id="ref-qcvm"></a>[QCVM] [`docs/QCVM_ENTITY_SYNC.md`](../../docs/QCVM_ENTITY_SYNC.md), ironwail-go repository.

<a id="ref-readme"></a>[README] [`README.md`](../../README.md), ironwail-go repository.


[ironwail]: https://github.com/andrei-drexler/ironwail
[gogpu]: https://github.com/gogpu/gogpu
[scratchapixel]: https://www.scratchapixel.com/
[webgpufundamentals]: https://webgpufundamentals.org/
[oto]: https://github.com/ebitengine/oto
