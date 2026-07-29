# Article Plan: Porting Quake to Go — The ironwail-go Story

This document is the research-and-writing plan for a longform technical
article about the development of `ironwail-go`, a pure-Go port of the
Ironwail Quake engine. The article is structured as a series of chapters,
each with its own research corpus, thesis, and deliverable. All drafts and
supporting material live in this `article/` folder.

---

## Audience and tone

- **Audience:** systems programmers, game engine hobbyists, Go community
  readers, and graphics programmers who want a real-world WebGPU case study.
- **Tone:** technically rigorous but narrative. Heavy use of `file:line`
  references, code excerpts, commit links, and issue citations. Not a
  beginner tutorial — assume the reader knows what a GPU is and roughly how
  Go works. The article *explains* Quake-specific and WebGPU-specific
  concepts inline but does not reteach them from zero.
- **Length target:** 12,000–18,000 words across all chapters. Each chapter
  is self-contained enough to be read on its own, but ordered to build a
  single arc.

---

## Output format and location

- All prose is Markdown.
- Drafts live in `article/draft/` as numbered `NN_topic.md` files.
- Supporting artifacts (the downloaded Hickman PDF, extracted text, issue
  transcripts, diagrams) live alongside in `article/`.
- Final assembled article: `article/ironwail_go.md`.
- A bibliography / source index: `article/sources.md`.

---

## Source corpus (canonical list)

These are the primary sources the article draws on. Each chapter cites the
subset relevant to it.

### In-repo (ironwail-go)

| Source | Path | Use for |
| --- | --- | --- |
| README | `README.md` | Project intent, scope, "is this AI slop?" framing |
| AGENTS.md | `AGENTS.md` | Build/test commands, gotchas, conventions |
| doc.go (root) | `doc.go` | Architecture diagram, subsystem prose |
| Learning Guide | `docs/LEARNING_GUIDE.md` | Package map, client/server mental model |
| Comparison doc | `docs/COMPARISON.md` | C-vs-Go divergence summary |
| Parity guide | `docs/PARITY.md` | Current parity gaps, qbj3 status, evidence |
| Renderer learning plan | `docs/RENDERER_LEARNING_PLAN.md` | Stage-by-stage renderer curriculum (Stages 0–14) |
| Vertex layout | `docs/VERTEX_LAYOUT.md` | The 48-byte WorldVertex contract |
| Quake specification | `docs/QUAKE_SPECIFICATION.md` | Formal behavior spec (FS, cvars, physics, net) |
| QCVM entity sync | `docs/QCVM_ENTITY_SYNC.md` | Dual-storage sync problem and fixes |
| QGo guide | `docs/QGO_QUAKEGO_GUIDE.md` | QuakeGo subset, compiler workflow |
| Package docs | `docs/internal/*.md` (qc, renderer, mods, server, ...) | Per-subsystem detail |
| Walkthroughs | `docs/WALKTHROUGH_*.md` | Cross-subsystem flows |
| Diagnoses | `docs/diagnoses/qbj2_water.md`, `qbj2_materials.md` | Bug investigation case studies |
| Git history | `git log` of this repo | Commit-level narrative (water fixes, atlas, sync unification) |
| Renderer source | `internal/renderer/*_gogpu.go`, `world_*` | WebGPU implementation reality |

### C reference (ironwail)

| Source | Path | Use for |
| --- | --- | --- |
| Quake sources | `ironwail/Quake/*.c` (gl_rmain.c, r_world.c, sv_phys.c, pr_exec.c, ...) | Canonical C behavior, parity oracle |
| CLAUDE.md / GEMINI.md | `ironwail/CLAUDE.md`, `ironwail/GEMINI.md` | How the C project documents itself |

### External

| Source | URL | Use for |
| --- | --- | --- |
| Hickman, *Quake Engine Analysis* (Northeastern) | `zhickman.com/analysisfinal.pdf` (local copy `article/analysisfinal.pdf` + `article/analysisfinal.txt`) | Academic overview of the original Quake engine: game loop, memory, rendering, physics, networking, scripting, math |
| gogpu issue tracker — darkliquid issues | `github.com/gogpu/gogpu/issues` (fetched via `gh`) | Real-world gogpu bugs: input, pointer lock, adapter power pref, naga SPIR-V bugs, Wayland two-connection bug |
| Scratchapixel | scratchapixel.com | CG theory citations (rasterization, lightmaps, BSP) |
| webgpufundamentals | webgpufundamentals.org | WebGPU API practice citations |
| id Software Quake source | github.com/id-Software/Quake | Original engine lineage |
| ironwail (C) repo | github.com/andrei-drexler/ironwail | Reference fork |
| gogpu / naga / wgpu repos | github.com/gogpu/* | Pure-Go WebGPU stack internals |

### Issue transcript (fetched)

Captured into `article/gogpu_issues.md` during research. Key issues by
darkliquid (Andrew Montgomery) in `gogpu/gogpu`:

| # | Title | Status | Article use |
| --- | --- | --- | --- |
| 157 | Full Go Port of Quake Source Port Ironwail | closed | The "showcase" issue; documents the early cgo-GLFW detour, then return to gogpu |
| 129 | Input not working under linux | closed | Linux X11/Wayland input stub gaps |
| 173 | Is there a way to do mouse grab? | closed | Pointer lock absence |
| 175 | No pointer lock on wayland | closed | Wayland pointer constraints gap |
| 176 | Windowed renderer ignores adapter power preference | closed | Hybrid-GPU Linux adapter selection |
| 162 | naga generates invalid SPIR-V FMix | closed | naga WGSL→SPIR-V scalar splat bug |
| 163 | Ironwail-go demo (showcase) | open | Community showcase thread |
| 227 | Support multiple keyboard layouts | closed (unxed) | Multi-layout input, purego X11 |

---

## Cross-cutting themes (must appear throughout)

These are not single-chapter topics; they recur across the article and each
chapter should touch the relevant ones. They are called out here so no
chapter draft forgets them.

### A. AI-assisted development, multi-agent

The project is explicitly an agentic-coding experiment, and **multiple
different AI agents were used over the course of the port** — not a single
model. The README names Claude Opus 4.6 and GPT-5.4 as the bulk of the work,
but GLM, Qwen, and Gemini were also tried. The article should be honest and
specific about this: which agents handled what kinds of tasks, where they
succeeded, where they failed, and how the human operated as architect/reviewer
per the AGENTS.md "Senior-Junior" model. This is not a footnote; it is a
load-bearing framing for the whole piece (Chapter 0 sets it up, Chapter 7
reflects on it).

### B. The Quake Brutalist Jam maps as the stress test

The **Quake Brutalist Jam (qbj)** community map packs — specifically `qbj2`
and `qbj3` (e.g. `qbj3_stickflip`) — are the project's de facto integration
test suite. They are large BSP2 maps that stress every subsystem: huge face
counts (85,936 raw faces in qbj3), >256 textures (which broke the atlas),
complex trigger stacks (a dozen trigger types firing on spawn), lit water,
external skyboxes, pusher/entity chains. The article should use qbj2/qbj3 as
the recurring narrative device: each chapter's "bugs and lessons" should
reference the specific qbj map that surfaced them. Cite `docs/PARITY.md`
qbj3 status, `docs/diagnoses/qbj2_*.md`, and the git log sweep of
qbj-related commits. The brutalist jams are *why* the renderer's open bugs
are known and documented.

### C. QuakeGo is a side project, NOT used in the engine

**Critical clarification for Chapter 5:** `pkg/qgo` (the `qgo` compiler and
`QuakeGo` gameplay source) is an **independent side project** to explore
porting the QuakeC *language* to a Go-dialect variant. It is **not wired
into the engine**. The engine runs the **original** `progs.dat` bytecode
compiled from the original QuakeC sources — there are **no tests or e2e
runs of the game using a QuakeGo-compiled `progs.dat`**. The article must
state this plainly to avoid the common misreading that ironwail-go "runs
on QuakeGo." QuakeGo is a parallel experiment in language design; the
canonical engine path uses the original QC bytecode loaded by `internal/qc`.

### D. The educational mandate

A primary goal of the codebase is to be **educational and self-explanatory**
— readable by someone *without* deep graphics-programming or game-dev
experience. The article should foreground this design intent repeatedly:
the package `doc.go` lineage sections, the `docs/internal/*.md` per-package
guides, the `RENDERER_LEARNING_PLAN.md` stage curriculum (built for a reader
who knows Go but not graphics), the `// Where in C:` citation convention,
the parity-test-naming-as-documentation pattern, and the `bspdiag` offline
inspection tool. The article itself should model this accessibility —
explaining Quake/WebGPU concepts inline rather than assuming them.

### E. Future plans

The closing synthesis must cover concrete forward-looking plans, not just
"more parity." Specifically:
- **Browser port:** investigating running the engine in the browser via
  WebGPU's native web target and Go's WASM compilation, leveraging the
  WebGPU renderer's portability.
- **More modularisation:** further package decomposition and boundary
  tightening to improve testability and reduce import-cycle pressure.
- **Embracing more idiomatic Go:** moving away from mechanical C-mirror
  patterns (within the `pkg/qgo/quakego` exception) toward more Go-idiomatic
  control flow, error handling, and naming where parity allows.
- **Arena allocator investigation:** the GC pressure in hot paths (QCVM
  edict sync, renderer per-frame allocations) has motivated investigation of
  Go-based arena/region allocators (e.g. `arena` proposals, pooling) as a
  middle ground between the C Hunk and full GC — a direct callback to the
  Chapter 2 memory-divergence discussion.
- Ongoing parity closure (atlas overflow fix, CSQC runtime wiring, qbj3
  sign-off).

---

## Chapter outline

Each chapter lists: thesis, key sources, and the research/writing tasks.

### Chapter 0 — Prologue: why port Quake to Go in 2026

**Thesis:** A personal and technical framing of the project — nostalgia,
learning, agentic-coding experiment across *multiple* AI agents, a pure-Go
graphics stress test, and an explicitly educational codebase mandate.

**Sources:** `README.md` (the "is this AI slop?" framing); gogpu issue #157;
AGENTS.md "Project at a glance" and the agentic-engineering guidelines;
Hickman PDF introduction; the multi-agent reality (Claude/Copilot/GLM/Qwen/Gemini).

**Tasks:**
- Summarize the project's stated goals (learning, agentic coding, nostalgia).
- Address the "AI slop" question honestly: the codebase was largely
  AI-generated, but across **multiple different agents** (Claude Opus 4.6 and
  GPT-5.4 as the bulk, with GLM, Qwen, and Gemini also tried), under a
  human-as-architect-and-reviewer model. Name the agents explicitly. Be
  honest about variance in agent capability and where each was tried.
- Introduce the **educational mandate** as a first-class goal: the
  codebase is built to be self-explanatory and learnable without deep
  graphics or game-dev background. Preview how this shows up (package docs,
  learning plan, `// Where in C:` citations, `bspdiag`).
- Introduce the **Quake Brutalist Jam (qbj) maps** as the recurring stress
  test that surfaces bugs throughout the engine.
- Set up the central tension: Quake is a 1996 C engine built around manual
  memory and immediate-mode OpenGL; Go + WebGPU is a very different world.
- Preview the chapter arc.

### Chapter 1 — How the Quake engine actually works

**Thesis:** A coherent architectural tour of the original Quake engine as a
client-server simulation with a BSP world, a QuakeC scripting VM, and a
software/OpenGL renderer — grounded in both the Hickman analysis and the
ironwail-go spec docs.

**Sources:** Hickman PDF (all sections); `docs/QUAKE_SPECIFICATION.md`;
`docs/LEARNING_GUIDE.md`; `doc.go` architecture diagram; `ironwail/Quake/*.c`
for spot citations.

**Tasks:**
- Time and the game loop: `Host_Frame`, 250 FPS host / 72 Hz server tick,
  prediction vs. authoritative simulation. Cross-reference Hickman §I.
- Resource management: Hunk vs. Zone vs. Cache (Hickman §III). Explain why
  Quake used these and what they enabled.
- The client-server split even in single-player: signon sequence, delta
  snapshots, prediction. Cite `QUAKE_SPECIFICATION.md` §3.
- Physics: hulls, `SV_FlyMove`, `SV_WalkMove`, pushers. Cite spec §4 and
  Hickman §VI.
- Collision via BSP: `BoxOnPlaneSide`, leaf traversal. Hickman §VII.
- Game object models: alias (MDL), sprite, BSP submodel. Hickman §IX.
- Networking: `net_*` files, datagram model. Hickman §XIII.
- Scripting: console commands, aliases, macros. Hickman §XIV.
- Math: `mathlib` functions, assembly fast paths. Hickman §XV.
- End by setting up *why* this architecture is both durable and tricky to
  port to a GC'd, package-structured language.

### Chapter 2 — The Go divergence: from C hunk to GC, from OpenGL to WebGPU

**Thesis:** ironwail-go is not a line-for-line transliteration; it is a
deliberate re-architecture that preserves behavioral parity while embracing
Go idioms (packages, GC, goroutines, stdlib). Document the major structural
divergences and the reasoning behind each.

**Sources:** `docs/COMPARISON.md`; `AGENTS.md` conventions; `doc.go`;
`docs/LEARNING_GUIDE.md`; git history (early commits); package `doc.go`
lineage sections.

**Tasks:**
- Memory: Hunk/Zone/Cache → GC + slices. Trade-offs (no manual arena, but
  GC pressure in hot paths). Cite commits touching allocation hot paths
  (e.g. `5a04a01 Optimize renderer allocations`).
- Concurrency: single-threaded C + SDL threads → goroutines for async
  loading/audio, dedicated render thread. Cite `internal/async` and the
  render-thread model.
- Packaging: flat `Quake/*.c` → `internal/*` packages with responsibilities.
  Show the package table from LEARNING_GUIDE.
- stdlib adoption: replacing custom Quake string/byte utilities with Go
  stdlib where parity allows.
- Cgo policy: `CGO_ENABLED=0` always; Oto for audio; purego for native
  bindings. Cite AGENTS.md gotcha #4.
- Parity-first discipline: `doc.go` lineage sections, `// Where in C:`
  comment convention, parity test naming.
- The cgo-GLFW detour and return: use gogpu issue #157 to narrate the
  early "hit enough gogpu issues, switched to cgo GLFW, then came back"
  arc.
- Introduce the **Quake Brutalist Jam maps** as the integration-test
  benchmark that surfaces divergences: the qbj2 large-map face/texture
  counts that broke the atlas, the qbj2 lift trigger stack that broke QCVM
  sync, the qbj3 spawn-view parity deltas. Foreshadow that each later
  chapter's "bugs and lessons" ties back to a specific qbj map.

### Chapter 3 — The renderer: OpenGL then, WebGPU now

**Thesis:** The renderer is the most divergent subsystem. Compare the C
Ironwail OpenGL (legacy + core, fixed-function-ish) renderer against the
GoGPU/WebGPU renderer, and explain the conceptual leaps required.

**Sources:** `ironwail/Quake/gl_*.c`, `r_*.c`; `docs/RENDERER_LEARNING_PLAN.md`
(Stages 0–14); `docs/VERTEX_LAYOUT.md`; `docs/internal/renderer.md`;
renderer source `internal/renderer/*_gogpu.go`; `docs/diagnoses/*`.

**Tasks:**
- The OpenGL model: immediate-mode `glBegin/glEnd` heritage in Quake,
  Ironwail's modernized core-profile path, single framebuffer, `R_RenderView`
  ordering. Cite `gl_rmain.c`.
- The WebGPU model: command-buffer "recipe" submission, explicit pipeline
  objects, bind groups, render passes. Cite the CPU/GPU split in
  `DrawContext` (`renderer_gogpu.go:16`).
- The 48-byte `WorldVertex` contract: why one vertex layout serves world,
  brush, alias, sprite, decal. Cite VERTEX_LAYOUT.md and the three-place
  agreement rule.
- Texture atlas + per-vertex material ID: contrast with C's per-texture
  bind. Cite `world_atlas_gogpu.go` and commit `e99fad0`.
- Lightmap array with 1px padding and the Vulkan vertical-stack workaround.
- Cluster-forward dynamic lights via compute shader — a feature the C
  renderer does not have. Cite Stage 9.
- OIT (weighted-blended transparency) as an optional modern path. Stage 14.
- Render order parity: opaque world → opaque entities → translucent water →
  translucent entities, matching C's `R_DrawWater(true/false)`. Cite
  `docs/diagnoses/qbj2_water.md` C Reference Architecture section.

### Chapter 4 — Render stages, broken down

**Thesis:** A stage-by-stage breakdown of one rendered frame, from clear
to overlay, explaining *what* each stage is for and *how* it is implemented
in GoGPU. This is the deepest technical chapter.

**Sources:** `docs/RENDERER_LEARNING_PLAN.md` (the canonical stage map);
`renderer_gogpu_frame.go:82` (`RenderFrame`); each stage's source files.

**Tasks:** For each of the 14 stages (0–14), write a compact subsection:
- Purpose (one paragraph).
- C reference (where it lived in `gl_*.c`/`r_*.c`).
- GoGPU reality: file:line, pipeline, shader, bind group.
- Notable bugs/lessons (cite diagnoses and commits).

Stage map to cover:
0. GPU mental model & `Core.InitHeadless`.
1. Triangle/pipeline basics (polyblend as the hello-triangle).
2. Matrices & camera (`camera.go`, world uniforms).
3. BSP world geometry upload (`UploadWorld`).
4. Textures & atlas (`world_atlas_gogpu.go`, materials buffer).
5. Lightmaps (`world_lightmap_gogpu.go`, lightstyles).
6. Visibility: BSP/PVS/FatPVS (`selectVisibleWorldFaces`).
7. Depth & opaque/translucent ordering (the water bug).
8. Sky, turbulent liquids, fog, underwater warp.
9. Cluster compute dynamic lights.
10. Entities: brush, alias (MDL), sprite, decal, viewmodel.
11. Particles.
12. Post-processing: scene composite, polyblend, 2D overlay.
13. The full frame: `RenderFrame` top-to-bottom walkthrough.
14. (Optional) OIT.

### Chapter 5 — The modding system and the QuakeC VM

**Thesis:** Quake's moddability comes from the QuakeC VM — a bytecode
interpreter with engine-provided builtins. Porting it to Go introduced a
fundamental dual-storage sync problem that does not exist in C. This chapter
explains the VM, the port, and the bugs, and then carefully separates the
canonical engine path from the independent QuakeGo side project.

**Sources:** `docs/internal/qc.md`; `docs/QCVM_ENTITY_SYNC.md`;
`internal/qc/exec.go`, `builtins.go`, `types.go`; Hickman PDF (scripting
§XIV); `docs/QGO_QUAKEGO_GUIDE.md`; commits `fe9e43c` (sync unification),
`e68aa0c` etc.

**Tasks:**
- What QuakeC is: a compiled DSL (`progs.dat`) for game logic; why it
  enabled modding without C source. Cite Hickman and qc.md.
- The VM: statement loop, opcodes, globals, edicts, `OP_LOAD/STORE`,
  builtins dispatched by negative function index. Cite `internal/qc/exec.go`.
- Bit-perfect parity concerns: IEEE divide-by-zero, `mod` semantics,
  runaway loop limit constant `0x1000000`. Cite qc.md tests.
- The C memory model: `edict_t` with embedded `entvars_t`; `EDICT_TO_PROG`
  pointer arithmetic; engine and QC share the same memory. No sync needed.
- The Go problem: GC'd `Edict` structs vs. flat `QCVM.Edicts []byte`; 78 of
  ~105+ fields synced; extension fields live only in QCVM bytes.
- The pusher/non-pusher selective sync bug chain (qbj2 lift). Cite the
  QCVM_ENTITY_SYNC bug chain — and note that the **qbj2 brutalist jam map**
  is what exposed this (the lift trigger stack).
- The fix: unified `syncAllToQCVM`/`syncAllFromQCVM` at a single dispatch
  point (`executeQCFunction`). Commit `fe9e43c`. Also: 157 typed accessor
  methods added to `Edict` (`entity_accessors.go`) that read/write the QCVM
  byte array directly, and ~27 call sites in `server.go` migrated to
  direct-VM access. But `EntVars` and `server_qc_sync.go` still exist —
  physics/movement hot paths still use `ent.Vars.*`. The long-term goal
  (steps 3–5) is to migrate all hot paths to accessors, delete `EntVars`
  and the sync layer, matching C's zero-sync model.
- Hook isolation: the package-level `serverBuiltinHooks` global
  cross-contamination bug and fix.
- Reflection cost: note the parity doc's CPU-profile finding that QC/server
  edict sync is a top hot path (`syncEntVarsFromQC`, `SetEFloat`) on qbj3.
- **QuakeGo / `qgo` — the side project, clearly fenced:** `pkg/qgo` is an
  **independent side project** exploring porting the QuakeC *language* to a
  Go-dialect variant compiled by `cmd/qgo`. It is **not wired into the
  engine**. The engine runs the **original** `progs.dat` bytecode from the
  original QuakeC sources. State plainly: there are **no tests or e2e runs
  of the game using a QuakeGo-compiled `progs.dat`**. QuakeGo is parallel
  language-design research, not a runtime dependency. The mechanical-port
  convention in `pkg/qgo/quakego` (avoid idiomatic rewrites) is a *resync*
  concern specific to that side project, not the engine.

### Chapter 6 — GoGPU: pure-Go WebGPU in practice

**Thesis:** GoGPU (`github.com/gogpu/gogpu` + `naga` + `wgpu` + `gpucontext`)
is the closest thing to a pure-Go WebGPU stack. Using it to build a real
game engine surfaced bugs and gaps in input, windowing, shader
compilation, and driver conformance. This chapter is a state-of-the-art
report on pure-Go graphics in 2026, using ironwail-go as the field test.

**Sources:** gogpu issues #157, #129, #173, #175, #176, #162, #163;
`go.mod` (gogpu/naga/wgpu/gpucontext deps); renderer gogpu adapter files;
git history of gogpu dep bumps; `docs/RENDERER_LEARNING_PLAN.md` external
citations.

**Tasks:**
- What GoGPU is: the module family (`gogpu`, `gpucontext`, `gputypes`,
  `naga`, `wgpu`) and its relationship to `go-webgpu/webgpu` + `goffi`.
  Cite `go.mod`.
- The WGSL→SPIR-V pipeline via naga, and why that matters on Vulkan/Adreno.
- Bug: naga invalid SPIR-V for `mix(vec3, vec3, f32)` (#162) — scalar splat
  fixed in naga v0.17.0+; the `vec3<f32>(fog)` workaround commit.
- Bug: naga swizzle gap and derivatives/`textureDimensions` producing
  invalid SPIR-V (referenced in #157 comments by kolkov).
- Bug: Linux X11 input stub (#129); the dev-branch fix in v0.22.8.
- Bug: no pointer lock / mouse grab (#173) and Wayland pointer constraints
  (#175); the `libwldevices-go` suggestion.
- Bug: adapter power preference ignored on hybrid-GPU Linux (#176).
- The Wayland two-connection bug (from #157): pure-Go `wl_display_connect`
  vs. C libwayland connection owning the visible surface; input never
  delivered. "BUG-GOGPU-002 (P0)". This is a defining architectural lesson.
- General state of pure-Go graphics: where it's strong (no cgo, deployable,
  WebGPU portability) and where it's weak (tooling maturity, driver
  conformance variance, naga coverage gaps, Wayland/windowing surface area).
- Lessons for engine authors: shader-authoring constraints (avoid swizzles,
  prefer explicit splats), defensive SPIR-V validation, fallback paths.
- The "showcase" framing: issue #163 and the community value of a real
  engine running on the pure-Go GPU stack.

### Chapter 7 — Synthesis: what was learned, and where it goes

**Thesis:** Tie the threads together — what porting a 1996 engine to 2026 Go
teaches about engine architecture, about Go as a systems language, about
WebGPU, about multi-agent agentic coding at scale, and where the project
goes next. Honest about what remains unfinished.

**Sources:** `docs/PARITY.md` (open gaps); `docs/RENDERER_LEARNING_PLAN.md`
"State of the Renderer"; diagnoses; git log; README framing.

**Tasks:**
- What the port validated: Quake's architecture is remarkably durable;
  behavioral parity as a discipline works; pure-Go graphics is viable for a
  real engine today, with caveats; the educational mandate is achievable —
  the codebase is genuinely navigable without graphics expertise.
- What it exposed: GC pressure in hot paths; the QCVM dual-storage sync
  tax; naga/Wayland maturity; the cost of "no cgo" on Linux desktop; the
  brutalist jam maps (qbj2/qbj3) as unforgiving integration tests that keep
  the parity claims honest.
- **Multi-agent agentic coding reflection:** the project is a case study in
  using *several different* AI agents (Claude, Copilot, GLM, Qwen, Gemini)
  on a single large port. Reflect honestly on agent variance, the
  Senior-Junior architect-reviewer loop from AGENTS.md, where agents
  produced "slop" vs. high-value work, and the parity-test discipline that
  kept them honest.
- **Future plans (concrete):**
  - **Browser port:** investigating running the engine in the browser via
    WebGPU's native web target and Go WASM, leveraging the WebGPU
    renderer's cross-platform portability for a no-install playable demo.
  - **More modularisation:** further package decomposition and boundary
    tightening to improve testability and reduce import-cycle pressure.
  - **Embracing more idiomatic Go:** moving away from mechanical C-mirror
    patterns (within the `pkg/qgo/quakego` exception) toward Go-idiomatic
    control flow, error handling, and naming where parity permits.
  - **Arena allocator investigation:** the GC pressure in hot paths (QCVM
    edict sync, renderer per-frame allocations) has motivated investigation
    of Go-based arena/region allocators (the stdlib `arena` experiment,
    pooling) as a middle ground between the C Hunk and full GC — a direct
    callback to the Chapter 2 memory-divergence discussion.
  - Ongoing parity closure: texture atlas overflow fix for BSP2 large maps,
    CSQC runtime wiring (currently deferred), qbj3 parity sign-off.
- Closing: Quake as a forever-benchmark for new language stacks; the
  educational artifact as the lasting contribution even if parity is never
  100%.

---

## Research workflow (how the writing gets done)

1. **Per-chapter source pass.** Before drafting a chapter, re-open every
   cited source and pull exact quotes/line numbers into a scratch notes
   file `article/draft/NN_notes.md`. This prevents stale citation.
2. **Commit-grounded claims.** Any claim about "why" a divergence happened
   must cite a commit hash or an issue number. Use `git show` / `gh issue
   view` to verify before writing.
3. **Parity cross-check.** For any behavioral claim, check the C source in
   `ironwail/Quake/` and the Go spec doc before asserting parity.
4. **Diagrams.** Produce a small number of Mermaid/ASCII diagrams:
   - Engine architecture (reuse `doc.go` diagram).
   - Render frame phase order (from RENDERER_LEARNING_PLAN Stage 13 table).
   - QCVM dual-storage sync flow (from QCVM_ENTITY_SYNC).
   - GoGPU module dependency graph.
5. **Draft → review → polish.** Each chapter drafted, then re-read against
   its sources for accuracy, then trimmed for concision.
6. **Assembly.** Concatenate chapters into `article/ironwail_go.md`, add a
   shared bibliography `article/sources.md`, and a TOC.

---

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Line numbers drift as code changes | Cite function/symbol names first, line numbers second; note the snapshot date (2026-07-27) as RENDERER_LEARNING_PLAN does. |
| Over-claiming parity | Anchor every parity statement to `docs/PARITY.md` status and to a commit; avoid "it works" without evidence. |
| Misrepresenting gogpu upstream | Quote issue text verbatim (already captured in `article/gogpu_issues.md`); distinguish darkliquid's reports from kolkov's responses. |
| Nostalgia bias | Use Hickman's academic analysis as an external anchor for the C engine description. |
| Scope creep | The chapter list is fixed. New findings go into the relevant chapter's notes, not into a new chapter. |
| Misrepresenting QuakeGo role | Cross-cutting theme C and Chapter 5 fence this explicitly; a reviewer check pass must verify the article never implies QuakeGo runs the engine. |
| Agent misattribution | When naming which AI agent did what, only assert from the README's named agents (Claude Opus 4.6, GPT-5.4) plus GLM/Qwen/Gemini as "tried"; do not invent task-to-agent mappings not in source. |

---

## File layout

```
article/
  PLAN.md                      (this file)
  ironwail_go.md               (final assembled article)
  sources.md                   (bibliography / source index)
  analysisfinal.pdf            (Hickman PDF, local copy)
  analysisfinal.txt            (pdftotext extraction)
  gogpu_issues.md              (transcript of fetched gogpu issues)
  draft/
    00_notes.md
    00_prologue.md
    01_notes.md
    01_quake_architecture.md
    02_notes.md
    02_go_divergence.md
    03_notes.md
    03_renderer_opengl_webgpu.md
    04_notes.md
    04_render_stages.md
    05_notes.md
    05_qcvm_modding.md
    06_notes.md
    06_gogpu_state_of_the_art.md
    07_notes.md
    07_synthesis.md
```
