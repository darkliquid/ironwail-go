# Prologue: Why Port Quake to Go in 2026

In 1996, id Software published Quake. It was the first fully-3D first-person
shooter from the studio that had defined the genre with Doom, and it shipped
with an engine whose architecture — a server-authoritative client-server
simulation, a BSP world with precomputed visibility, a bytecode scripting VM
for game logic, and a software (later OpenGL) renderer — set patterns that
game engines still follow thirty years later. Zachary Hickman, in an academic
analysis of the engine written for Northeastern University, framed it
succinctly: *"The impressive feature improvements required a much more
comprehensive engine."* [Hickman](#ref-hickman)

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
> [README](#ref-readme)

Three threads run through the whole project, and through this article:

---

## An experiment in agentic coding — with multiple agents

The README addresses the elephant directly in a section titled *"Did you say
agentic coding? Is this just AI slop?"* The answer is a candid *"Yes and
no."* A large portion of the codebase was written by AI agents converting C
to Go, but under a human-as-architect-and-reviewer model that the project's
`AGENTS.md` codifies as a "Senior-Junior partnership" — the human acts as
architect and reviewer, the agent as a fast, literal-minded junior
engineer. [AGENTS](#ref-agents)

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
  build-it-yourself milestones at each stage. [LearningPlan](#ref-learningplan)
- **The `// Where in C:` citation convention** in tests, e.g.
  `// Where in C: SV_WalkMove in sv_phys.c`, anchoring every behavioral
  assertion to the canonical reference.
- **`bspdiag`** — an offline BSP inspection CLI (`cmd/bspdiag`) built so
  that anyone can inspect map lumps, entities, leaf contents, lightmaps, and
  liquid alpha settings without writing scratch scripts. [AGENTS](#ref-agents)
- **Parity test names that document the invariant being protected** — e.g.
  `TestExecuteProgramRunawayLoopLimitConstantMatchesC` asserts the
  runaway-loop limit is exactly `0x1000000`, and the name itself explains
  *why* that constant matters for mod compatibility. [QCDocs](#ref-qcdocs)

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
  `MOVETYPE_PUSH` entities). [QCVM](#ref-qcvm) [MaterialsDiag](#ref-materialsdiag)
- **`qbj3`** (e.g. the `qbj3_stickflip` map) is the current priority
  stress case. The parity guide records its scale: 85,936 raw faces,
  77,001 built faces, 168,142 built triangles, 322,144 vertices, 22,195
  leafs, 750 models, 106 textures, four lightmap pages, 1,295
  lit-water/turbulent faces, and 228 sky faces. Its first rendered frame
  at the captured spawn view reports 1,002 visible faces, eight opaque
  world batches, seven opaque brush entities, and eleven opaque alias
  entities. [Parity](#ref-parity)

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
   bump-allocation arenas. [Hickman](#ref-hickman) Go has a garbage collector
   and forbids pointer arithmetic.
2. **Immediate-mode, single-threaded rendering.** The C renderer
   (Ironwail's modernized OpenGL path) draws directly to a framebuffer in
   `R_RenderView`, binding textures one at a time in immediate GL calls.
   WebGPU is explicitly a *retained* API: you write a "recipe" of commands
   into a command buffer, the GPU executes it later, and the CPU and GPU
   do not share memory directly. [LearningPlan](#ref-learningplan)
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
   [QCVM](#ref-qcvm)

Every architectural decision in `ironwail-go` — and every bug — flows from
the collision between Quake's 1996 assumptions and Go's 2026 reality. The
chapters that follow walk that collision, subsystem by subsystem: the
engine architecture, the Go divergence, the renderer, the render stages,
the QuakeC VM and modding system, and the GoGPU pure-Go WebGPU stack that
makes it all render without a line of C.

---

## References

<a id="ref-agents"></a>[AGENTS] [`AGENTS.md`](../../AGENTS.md), ironwail-go repository.

<a id="ref-hickman"></a>[Hickman] Zachary Hickman, *"Quake Engine Analysis,"* Northeastern University. [https://zhickman.com/analysisfinal.pdf](https://zhickman.com/analysisfinal.pdf).

<a id="ref-learningplan"></a>[LearningPlan] [`docs/RENDERER_LEARNING_PLAN.md`](../../docs/RENDERER_LEARNING_PLAN.md), ironwail-go repository.

<a id="ref-materialsdiag"></a>[MaterialsDiag] [`docs/diagnoses/qbj2_materials.md`](../../docs/diagnoses/qbj2_materials.md), ironwail-go repository.

<a id="ref-parity"></a>[Parity] [`docs/PARITY.md`](../../docs/PARITY.md), ironwail-go repository.

<a id="ref-qcdocs"></a>[QCDocs] [`docs/internal/qc.md`](../../docs/internal/qc.md), ironwail-go repository.

<a id="ref-qcvm"></a>[QCVM] [`docs/QCVM_ENTITY_SYNC.md`](../../docs/QCVM_ENTITY_SYNC.md), ironwail-go repository.

<a id="ref-readme"></a>[README] [`README.md`](../../README.md), ironwail-go repository.


[ironwail]: https://github.com/andrei-drexler/ironwail
[gogpu]: https://github.com/gogpu/gogpu
[scratchapixel]: https://www.scratchapixel.com/
[webgpufundamentals]: https://webgpufundamentals.org/
[oto]: https://github.com/ebitengine/oto
