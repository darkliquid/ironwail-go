# IRONWAIL-GO PARITY ANALYSIS: EXECUTIVE SUMMARY

> **Authoritative sources:** [`PORT_PARITY_REVIEW.md`](PORT_PARITY_REVIEW.md) and [`PORT_PARITY_TODO.md`](PORT_PARITY_TODO.md).
> When documentation and source code disagree, **source code wins**.
> ⚠️ Older audit docs (`parity_report.md`, older sections of this summary) are **historical snapshots only** — their percentages, severity labels, and tier tables describe an earlier state of the port and are **not** current status tracking.

## Current state (as of 2026-03)

The Go port is materially farther along than several older planning notes imply. The biggest remaining parity problems are **integration and fidelity gaps**, not total subsystem absence.

### Parity target

The **CGO/OpenGL runtime** (`renderer_opengl.go` + GLFW input + SDL3/Oto audio) is the authoritative parity target.
The **gogpu/WebGPU path** is secondary/experimental and must **not** be used as a parity gate.

### Quick status by area

| Area | Current state | Main remaining gap |
|---|---|---|
| World rendering (OpenGL) | Implemented | Broader visual polish beyond bounded pass-order/skybox slices |
| Entity rendering (OpenGL) | Mostly implemented | Particle visual fidelity polish |
| gogpu renderer | Non-parity backend | Full entity/model/sprite pipeline still missing |
| Input / bindings / config | Implemented | Key runtime UX edge cases |
| Menus / HUD / console | Mostly implemented | Expansion-pack HUD variants, mods menu, broader menu polish, alternate HUD styles |
| Audio / music | Mostly implemented | Broader codec / music format parity; underwater visual blue-shift |
| Networking / multiplayer | Partial | Broader remote networking / netgame depth |
| Save / load | Mostly implemented | Broader save-file search, transition UX long-tail |
| Demos | Mostly implemented | Demo edge-case / long-tail accuracy |
| Client prediction | Mostly implemented | Long-tail movement feel polish |

### Biggest current gaps

1. **gogpu renderer is not feature-parity** — entity/model/sprite/decal/viewmodel pipeline is still a stub.
2. **OpenGL renderer fidelity** — structurally close to C, but broader visual polish remains.
3. **Ironwail-specific UX features** — mods menu, mouse-driven UI polish, richer weapon-bind UI, alternate HUD styles.
4. **Remote multiplayer depth** — local loopback is ahead of true remote netgame parity.
5. **Audio / music fidelity** — subsystem is live, not fully accurate vs C bgmusic/codec behavior.
6. **Demo long-tail parity** — core forward path works; edge-case accuracy remains.
7. **Save/load transition UX** — broadly implemented, some long-tail behavior differs.

### What older audit docs got wrong

The following claims from older documents are **no longer accurate**:

| Old claim | Current reality |
|---|---|
| "No entity rendering pipeline" | OpenGL path has alias/sprite/brush/particle/decal/viewmodel |
| "No key binding system" | Quake-style bind/unbind/config persistence is fully wired |
| "Audio never receives events" | Sound dispatch, mixer, and static sounds are live |
| "No demo recording/playback" | Forward path including connected-state snapshots is real |
| "Lightmap index hardcoded -1" | Real lightmap allocation/upload/filtering is in place |
| "Submenus are all stubs" | Options/multiplayer/controls/setup submenus are bounded-complete |

For the full current baseline, see **[`PORT_PARITY_REVIEW.md`](PORT_PARITY_REVIEW.md)**.
For the ordered implementation backlog, see **[`PORT_PARITY_TODO.md`](PORT_PARITY_TODO.md)**.
For a cross-subsystem audit table, see **[`PARITY_AUDIT_TABLE.md`](PARITY_AUDIT_TABLE.md)**.
