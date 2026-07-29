# gogpu/gogpu issues — darkliquid transcript

This file is a research capture of gogpu issues authored by or commented on
by `darkliquid` (Andrew Montgomery), fetched via `gh issue view --comments`
on 2026-07-27. Used as source material for the article, especially Chapter 6.

Fetched via:
```
gh issue list --repo gogpu/gogpu --author darkliquid --state all --limit 100
gh issue list --repo gogpu/gogpu --search "darkliquid" --state all --limit 100
gh issue view <N> --repo gogpu/gogpu --comments
```

---

## Summary table

| # | Title | State | Author | Article use |
| --- | --- | --- | --- | --- |
| 157 | Full Go Port of Quake Source Port Ironwail | closed | darkliquid | Showcase issue; cgo-GLFW detour, then return to gogpu; naga swizzle + derivatives bugs surfaced |
| 129 | Input not working under linux | closed | darkliquid | X11 input stub; fixed in v0.22.8 |
| 173 | Is there a way to do mouse grab? | closed | darkliquid | Pointer lock absence |
| 175 | No pointer lock on wayland | closed | darkliquid | Wayland pointer constraints; libwldevices-go suggestion |
| 176 | Windowed renderer ignores adapter power preference | closed | darkliquid | Hybrid-GPU Linux adapter selection |
| 162 | naga generates invalid SPIR-V FMix (scalar blend with vec3) | closed | darkliquid | naga WGSL→SPIR-V scalar splat bug; fixed in v0.17.0+ |
| 163 | Ironwail-go demo | open | darkliquid | Community showcase thread |
| 227 | Support multiple keyboard layouts | closed | unxed (darkliquid commented via search) | Multi-layout input, purego X11 |
| 106 | Linux example not working | closed | (search hit) | Early Linux platform bugs |
| 114 | Wayland backend failing on linux | closed | (search hit) | Early Wayland platform bugs |
| 119 | texture example segfault on linux | closed | (search hit) | Early texture backend bugs |
| 155 | gles example not working on linux | closed | (search hit) | gles backend on linux |
| 334 | Window polish: fix live resize blank (macOS) + public API | closed | (search hit) | Window polish refactor |

---

## Issue #157 — Full Go Port of Quake Source Port Ironwail (closed)

**Author:** darkliquid (Andrew Montgomery)
**Labels:** os: linux, status: in-progress, type: feature

### Opening body

> Hi, I've been slowly working away on a complete port of the ironwail quake 1
> engine to Go. I first attempted to tackle things using GoGPU as the
> rendering backend, but eventually hit enough issues that I sadly switched
> to cgo GLFW code.
>
> However, while I'm not asking for any help on this, I figured it may be an
> interesting thing to look at as it's a mostly fully functioning engine
> with a now half-finished secondary GoGPU renderer (that currently crashes
> before getting any visuals most of the time on my linux+wayland+niri
> setup) and a working GLFW based one for comparison.
>
> Let me know what you think or if you think you could adapt it as a test bed
> for ironing out bugs/functionality in GoGPU.
>
> https://github.com/darkliquid/ironwail-go

### Key comment (darkliquid)

> I've gone through and fixed my shaders so they don't use swizzling and now I
> have actual visuals! [screenshot attached]. Input appears to not be working,
> but I need to double check my implementation on top of gogpu first before I
> can confirm it's not a bug on my end

### Key comment (kolkov — gogpu maintainer)

> @darkliquid Quake on Pure Go — this made our day. Seriously impressive.
>
> Both issues are naga bugs:
> 1. **Swizzling** (`ExprSwizzle is not a pointer expression`) — we know about
>    this gap, your workaround is correct. Will fix.
> 2. **Scene Composite crash** — `dpdx`/`dpdy`/`textureDimensions` likely
>    produce invalid SPIR-V. Your repro test is gold, we'll use it to debug
>    the generated SPIR-V directly.
>
> Both go straight into our naga priority queue. These are exactly the
> real-world 3D patterns we were missing in our test coverage.
>
> Re input — try with `GOGPU_LOG=debug` ... The
> `dev/wayland-csd-input-fixes` branch has Wayland input fixes that might
> help.

### The Wayland two-connection root cause (kolkov, edited)

> @darkliquid We found the root cause. It's not Smithay-specific — it
> affects ALL Wayland compositors.
>
> ### The problem
>
> gogpu uses two `wl_display_connect()` calls from the same process:
> - **Pure Go connection** — owns wl_seat, wl_pointer, wl_keyboard (where we
>   listen for input)
> - **C libwayland connection** (via goffi) — owns the visible wl_surface +
>   xdg_toplevel (where Vulkan renders)
>
> Wayland delivers input events to the connection that owns the focused
> surface. Your window is on the C connection. Our input listeners are on
> the Pure Go connection. **They never meet.**
>
> We verified: no toolkit does this. GLFW, Gio, winit, neurlang-wayland — all
> use a single connection. We got away with it because on X11 window IDs are
> server-side (shared across connections), but Wayland surfaces are
> client-side (connection-scoped).
>
> ### The fix
>
> We need to bind wl_seat + wl_pointer + wl_keyboard on the C connection and
> forward events to Go. Our CSD code already does exactly this for pointer
> events on decoration subsurfaces — we need to generalize it to the main
> surface.
>
> Tracked as BUG-GOGPU-002 (P0). This is next after naga.

---

## Issue #162 — naga generates invalid SPIR-V FMix (closed)

**Author:** darkliquid
**Labels:** status: needs-info, type: bug

### Opening body

> The SPIR-V spec requires all FMix operands to have matching types. AMD's
> RADV driver tolerates this on my integrated GPU, but running with
> DRI_PRIME=1 to enforce my discrete NVIDIA GPU sees crashing with SIGSEGV
> at addr=0x10.

### Reproducer

> https://github.com/darkliquid/ironwail-go/commit/d5ff08441e0a8a6e7efec9d9b9fad71f76b1afbc
> shows the commit where I worked around the issue.

### Resolution (kolkov)

> We tested `mix(vec3, vec3, f32)` with the current naga (v0.17.1) and it
> **compiles correctly** — `OpCompositeConstruct` splats the scalar to vec3
> before `FMix`, and spirv-val passes:
>
> ```
> %22 = OpLoad %float %fog_ptr
> %23 = OpCompositeConstruct %v3float %22 %22 %22    ; scalar → vec3 splat
> %24 = OpExtInst %v3float FMix %a %b %23            ; all operands vec3 ✅
> ```
>
> This was likely fixed in a recent naga release. Which version were you on
> when you hit the SIGSEGV?

### darkliquid

> On that commit, I was on naga v0.15.2. I'll try out a revert of my
> workaround later tonight and see if the latest naga resolves the original
> issue or not.
>
> That is all working on latest naga

### kolkov close

> @darkliquid confirmed working on latest naga. Closing!
>
> The `mix(vec3, vec3, f32)` SPIR-V scalar-to-vector splat was already fixed
> in naga v0.17.0+ ... Thanks for the reproducer commit and for verifying
> the fix!
>
> For reference: the latest naga release is **v0.17.2** which also includes
> the Workgroup ArrayStride fix for Adreno Vulkan.

---

## Issue #129 — Input not working under linux (closed)

**Author:** darkliquid
**Labels:** area: input, area: platform, effort: 5, os: linux, priority: high,
status: confirmed, type: bug

### Opening body

> I'm trying to make use of the input handling under linux, but I'm hitting
> issues.
>
> Due to the other bugs where rendering with the wayland backend crashes, I'm
> forcing use of the X11 backend. However, I noticed that the X11 key
> handling code for the linux platform is just a stub. I am however not seeing
> any mouse input either.

### kolkov

> Fixed in [v0.22.8](https://github.com/gogpu/gogpu/releases/tag/v0.22.8).
> Please try the new version and let us know if it works for you.

### darkliquid

> I'm on the latest everything for gogpu and it's working for me now.

### kolkov

> @darkliquid Great to hear it's working! 🎉 Would love to see a video of
> ironwail-go running on the Pure Go GPU stack — that would be an amazing
> showcase for the community.

---

## Issue #173 — Is there a way to do mouse grab? (closed)

**Author:** darkliquid
**Labels:** area: input, priority: high, status: confirmed, type: feature

### Opening body

> Reading through the API, I didn't see a way to do stuff like locking the
> pointer to the window. Am I missing something?

---

## Issue #175 — No pointer lock on wayland (closed)

**Author:** darkliquid
**Labels:** area: platform, os: linux, priority: high, status: confirmed,
type: feature

### Opening body

> I noticed the pointer constraints implementation doesn't exist for wayland
> yet, and looking into what it would take to implement, I stumbled across
> this: https://github.com/bnema/libwldevices-go which might be worth looking
> at/adopting as a dependency.

---

## Issue #176 — Windowed renderer ignores adapter power preference (closed)

**Author:** darkliquid
**Labels:** area: renderer, priority: medium, status: confirmed, type: enhancement

### Opening body (full)

> ## Summary
> The windowed runtime renderer does not expose or forward an adapter power
> preference, so applications cannot reliably force integrated vs discrete
> GPU selection.
>
> ## Current behavior
> In `renderer.go`, the native runtime path requests an adapter with only
> `CompatibleSurface` set:
>
> ```go
> r.adapter, err = r.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
>     CompatibleSurface: r.surface,
> })
> ```
>
> The headless path in downstream projects can pass `PowerPreference`, but
> the windowed gogpu path cannot. On hybrid-GPU Linux systems this means the
> runtime may continue selecting the discrete NVIDIA adapter even when the
> application explicitly requests a low-power/integrated GPU.
>
> ## Expected behavior
> `gogpu.Config` should expose a power preference option and the windowed
> renderer should forward it to `RequestAdapter`. The Rust path should apply
> the same preference when choosing from enumerated compatible adapters.
>
> ## Why this matters
> Downstream engines may expose a user-facing GPU preference cvar/setting
> and expect it to be authoritative. Environment hints like `DRI_PRIME` are
> not a reliable substitute for an explicit runtime preference.
>
> ## Suggested fix
> 1. Add a `PowerPreference` field to `gogpu.Config`.
> 2. Pass it through `newRenderer(...)` to the native runtime
>    `RequestAdapter` call.
> 3. Apply equivalent adapter ranking on the Rust path when multiple
>    compatible adapters are available.

---

## Issue #163 — Ironwail-go demo (open)

**Author:** darkliquid
**Labels:** showcase, community

Community showcase thread; the gogpu maintainers use it as a "real-world
3D engine on the pure-Go GPU stack" reference.

---

## Issue #227 — Support multiple keyboard layouts (closed)

**Author:** unxed
**Labels:** area: input, priority: high, type: feature
**Comments:** 75

Surfaced via the darkliquid search because ironwail-go's input path was
cited. Relevant for the article's discussion of pure-Go X11 keyboard layout
handling via purego/libX11 (no cgo) vs. xkbcommon on Wayland.
