# Article Bibliography and Sources Index

This document is the canonical source index and bibliography for the longform article *"Porting Quake to Go — The ironwail-go Story"*.

---

## Academic and Reference Literature

- **Hickman, Zachary.** *"Quake Engine Analysis."* Northeastern University.
  - URL: [https://zhickman.com/analysisfinal.pdf](https://zhickman.com/analysisfinal.pdf)
  - Cited in Chapters 0, 1, 2, 5.
- **McGuire, Morgan, and Louis Bavoil.** *"Weighted Blended Order-Independent Transparency."* Journal of Computer Graphics Techniques (JCGT), vol. 2, no. 2, 122–141, 2013.
  - Cited in Chapters 3, 4 (OIT path).
- **id Software.** *Quake Source Code (GPL).*
  - URL: `https://github.com/id-Software/Quake`
  - Cited in Chapters 0, 1.

---

## Repository Documentation (`ironwail-go`)

- **README:** [`README.md`](../README.md) — Stated intent, "AI slop" framing, feature flags, parity goals.
- **AGENTS:** [`AGENTS.md`](../AGENTS.md) — Agentic engineering guidelines, Senior-Junior partnership model, gotchas, build/test commands.
- **Doc Go (Root):** [`doc.go`](../doc.go) — Root architecture diagram and subsystem overview.
- **Learning Guide:** [`docs/LEARNING_GUIDE.md`](../docs/LEARNING_GUIDE.md) — Package map, client/server mental model.
- **Comparison Doc:** [`docs/COMPARISON.md`](../docs/COMPARISON.md) — Structural C-vs-Go divergence summary.
- **Parity Guide:** [`docs/PARITY.md`](../docs/PARITY.md) — Canonical parity status, `qbj3_stickflip` evidence workflow, sign-off rules.
- **Renderer Learning Plan:** [`docs/RENDERER_LEARNING_PLAN.md`](../docs/RENDERER_LEARNING_PLAN.md) — 14-stage renderer curriculum.
- **Vertex Layout Contract:** [`docs/VERTEX_LAYOUT.md`](../docs/VERTEX_LAYOUT.md) — The 48-byte `WorldVertex` specification.
- **Quake Specification:** [`docs/QUAKE_SPECIFICATION.md`](../docs/QUAKE_SPECIFICATION.md) — Formal engine behavior specification (FS, cvars, physics, net).
- **QCVM Entity Sync Guide:** [`docs/QCVM_ENTITY_SYNC.md`](../docs/QCVM_ENTITY_SYNC.md) — Dual-storage VM sync problem, bug chain, unified sync architecture.
- **QuakeGo Guide:** [`docs/QGO_QUAKEGO_GUIDE.md`](../docs/QGO_QUAKEGO_GUIDE.md) — QuakeGo subset, compiler workflow, side-project boundaries.
- **Per-Package Docs:** [`docs/internal/*.md`](../docs/internal/) (`qc.md`, `renderer.md`, `server.md`, `host.md`, `client.md`, `fs.md`, `bsp.md`, `net.md`, `cmdsys.md`, `audio.md`, `engine.md`, `async.md`).
- **Diagnoses:**
  - [`docs/diagnoses/qbj2_water.md`](../docs/diagnoses/qbj2_water.md) — Water translucency, Vulkan discard, uniform offset diagnosis.
  - [`docs/diagnoses/qbj2_materials.md`](../docs/diagnoses/qbj2_materials.md) — Texture atlas materials buffer capacity diagnosis.

---

## Issue Transcripts and Upstream Bug Reports

Captured in [`article/gogpu_issues.md`](gogpu_issues.md) (fetched from `gogpu/gogpu` repository on 2026-07-27):

- **Issue #157:** *"Full Go Port of Quake Source Port Ironwail"* — Showcase issue; cgo-GLFW detour, return to gogpu, naga swizzle and derivatives bugs, Wayland two-connection root cause (BUG-GOGPU-002).
- **Issue #162:** *"naga generates invalid SPIR-V FMix (scalar blend with vec3)"* — Scalar `mix()` splat bug, fixed in naga v0.17.0+.
- **Issue #129:** *"Input not working under linux"* — X11 input stub gap, fixed in gogpu v0.22.8.
- **Issue #173:** *"Is there a way to do mouse grab?"* — Pointer lock feature request.
- **Issue #175:** *"No pointer lock on wayland"* — Wayland pointer constraints, `libwldevices-go` suggestion.
- **Issue #176:** *"Windowed renderer ignores adapter power preference"* — Hybrid-GPU adapter selection.
- **Issue #163:** *"Ironwail-go demo"* — Community showcase thread.
- **Issue #227:** *"Support multiple keyboard layouts"* — X11/purego keyboard layout discussion.

---

## External Resources and Learning Materials

- **Scratchapixel:** `https://www.scratchapixel.com/` — Computer graphics theory (rasterization, lightmaps, BSP).
- **WebGPU Fundamentals:** `https://webgpufundamentals.org/` — WebGPU API patterns and practices.
- **Ironwail C Repository:** `https://github.com/andrei-drexler/ironwail` — Canonical C reference fork.
- **GoGPU Repositories:** `https://github.com/gogpu/*` (`gogpu`, `wgpu`, `naga`, `gpucontext`, `gputypes`).
