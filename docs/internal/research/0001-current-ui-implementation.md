# RESEARCH-0001: Current UI Implementation in ironwail-go

- **Status:** Integrated
- **Owner:** research (collated from source deep-dive, 2026-08-18)
- **Date:** 2026-08-18
- **Blocks:** gap log #1 (ai-dlc/ui-gogpu-rewrite)

---

## Research Question

What is the current UI implementation in ironwail-go (menus, console, HUD,
CSQC-driven UI, demo playback controls), how is it wired into the game loop and
renderer, and what drawing primitives/theming does it use? This is the "before"
baseline for planning a rewrite on gogpu/ui. The spec's "current state" section
will consume the answer.

## Background & Constraints

- ironwail-go is a pure-Go (`CGO_ENABLED=0`) port of the C Ironwail Quake
  engine. Behavioral parity with C Ironwail/Quake is the primary test oracle.
- The canonical renderer is gogpu/WebGPU (`internal/renderer`); there is a
  software fallback (`software.go`) used for screenshots/parity.
- The current menu/console/HUD are hand-rolled Go code that mirrors the C
  Quake implementations (`menu.c/menu.h`, `con.c`, `sbar.c`, `hud.c`) drawing
  through a `RenderContext` 2D interface with Quake's 320x200 canvas model.
- No gogpu/ui dependency exists in `go.mod` today. Prior article/planning docs
  (`article/`) contain no gogpu/ui planning.

## Investigation Findings

### 1. The shared 2D draw interface: `renderer.RenderContext`

All UI layers (menu, console, HUD, demo bar, CSQC HUD) draw through
`internal/renderer/types.go:23-65`:

```go
type RenderContext interface {
    DrawPic(x, y int, pic *image.QPic)          // full image
    DrawMenuPic(x, y int, pic *image.QPic)      // 320x200 menu-space
    DrawFill(x, y, w, h int, color byte)        // palette-index fill
    DrawFillAlpha(x, y, w, h int, color byte, alpha float32)
    DrawCharacter(x, y int, num int)            // 8x8 conchars glyph
    DrawMenuCharacter(x, y int, num int)        // 320x200 menu-space glyph
    SetCanvas(ct CanvasType)                    // switch coordinate space
    Canvas() CanvasState
}
```

Concrete impl: `*DrawContext` (`internal/renderer/renderer_gogpu.go:17-48`)
wraps a gogpu `*gogpu.Context` + an `overlay2D` CPU compositor buffer
(`renderer_gogpu_overlay.go:11-20`). All 2D draws go through texture-scaled
`ctx.DrawTextureEx` calls (per-draw GPU submission) or the CPU buffer
(`blitRGBA`/`blitConcharsString`) flushed once per frame as one texture
(`flush2DOverlay`, `renderer_gogpu_overlay.go:32-118`). The live draw path
currently uses `DrawTextureEx` directly (`DrawPicAlpha` :432-453,
`DrawCharacterAlpha` :498-523); the CPU overlay buffer flush only runs if
`ov.dirty` is set, which the live path never sets (noted gap).

Palette colors: `DrawFill(color byte)` uses palette index → a 1x1 color texture
(`getOrCreateColorTexture`, `renderer_gogpu_texture.go:171-201`; index 0 =
black opaque). The Quake palette is 768 bytes from `palette.lmp` with a
hardcoded fallback `StandardQuakePaletteHex` (`internal/draw/palette.go:8`).

Font: Quake's `conchars` — a 128x128 WAD lump, 16x16 grid of 8x8 glyphs.
`getCharPic(num)` extracts an 8x8 region; `getOrCreateCharTexture` converts to
RGBA where palette index 0 is transparent (`renderer_gogpu_texture.go:43-142`).
"White" (bright) text = char code + 128 in the alternate glyph row.

Canvas model (`internal/renderer/screen.go`):
- `CanvasType` enum (:28-58): `CanvasDefault` (native px), `CanvasMenu`
  (320x200 centered, `scr_menuscale`), `CanvasSbar` (320x48 bottom),
  `CanvasSbarQWInv` (48x48 bottom-right), `CanvasSbar2` (400x225 central
  modern), `CanvasCrosshair` (centered on viewport midline), `CanvasConsole`,
  `CanvasBottomLeft/Right`, `CanvasTopRight`, `CanvasCSQC`.
- `GetCanvasTransform` (:223-307) builds scale/offset transforms; menu:
  `s = min(GUIWidth/320, GUIHeight/200)`, clamped by `scr_menuscale`, centered.
- Params: `renderer.CanvasTransformParams` (:157-193) fed from
  `internal/game/ui/ui.go` `CanvasParams` (GUI dims, console dims, console
  slide fraction, scalars).

### 2. Frame pipeline (where UI composites)

`DrawContext.RenderFrame(state, draw2DOverlay)` (`renderer_gogpu_frame.go:128-310`):
1. Clear (skipped when menu active to preserve frozen scene behind it :169-175)
2. World (BSP world to surface)
3. Entities (alias/brush/sprites/decals/particles)
4. Post: scene-target composite (water warp), polyblend, viewmodel
5. **2D overlay** (:274-289): `draw2DOverlay(dc)` then `dc.flush2DOverlay()`

The game supplies the overlay callback: `drawRuntimeOverlayFrame`
(`internal/game/game_runtime_frame.go:293-360`):
```
set canvas params → (forcedup?) drawRuntimeConsole(full) → drawRuntimeHUDLayer
→ drawRuntimeDemoControls → (console visible/animating?) drawRuntimeConsole
→ drawRuntimeMenu (last, on top)
```
mirroring C's `Sbar_Draw()` then `M_Draw()` order (`game_runtime_frame.go:351-354`).

WASM fallback: `drawRuntimeFallbackFrame` and the screenshot path
(`game_loop.go:591-604`) also call `drawRuntimeMenu`; software renderer
(`renderer/software.go`) implements the same `RenderContext` (placeholder
`DrawCharacter` box fill).

### 3. Menu (`internal/menu`)

- `MenuState` enum (`manager.go:30-49`): `MenuNone, MenuMain, MenuSinglePlayer,
  MenuLoad, MenuSave, MenuMultiPlayer, MenuJoinGame, MenuHostGame, MenuOptions,
  MenuControls, MenuVideo, MenuAudio, MenuHelp, MenuQuit, MenuSetup, MenuMods`.
- `Manager` struct (`manager.go:266-349`) holds all state: state, 14 per-page
  cursors, text-entry buffers (hostname/name/address/map name), host-game
  settings, mods list, `mouseAccumY`, injected services: `cvars`,
  `drawManager DrawManager` (interface = `Pic(name) *image.QPic`,
  `manager.go:352-354`, satisfied by `draw.Manager`), `inputSystem *input.System`,
  callbacks `commandText`, `playSound`, save slot/mods providers.
- Per-page item counts hard-coded mirroring C: `mainItems=6` (mods may skip),
  `singlePlayerItems=3`, `multiPlayerItems=3`, `joinGameBaseItems=4`,
  `hostGameItems=9`, `optionsItems=5`, `controlsItems=26`, `videoItems=12`,
  `audioItems=2`, `maxSaveGames=12`, `helpPages=6` (`manager.go:56-90`).
- Submenu navigation is a **flat state machine**: `*Key` handlers set
  `m.state = MenuX` to descend; Escape/Backspace returns to parent
  (`menu_main.go:94-97`, `menu_options.go:46-49,130-133`). Main select:
  `mainSelect()` (`menu_main.go:46-62`).
- `M_Key` (`manager.go:624-660`) dispatches to per-page handlers;
  `normalizeMenuKey` (:664-683) maps gamepad to arrows/Enter/Esc/Backspace.
  `M_Char` (:686-695) routes to text entry. `M_Mousemove` (dy accumulation,
  8px per step, `manager_navigation.go:15-32`), `M_MousemoveAbsolute`
  (hit-test via `menuCursorForPoint` hit-test table :104-186),
  `moveCursorDown/Up` modulo wrap (:189-305).
- Drawing: `Manager.M_Draw` (`manager.go:698-731`) dispatches per-state to
  `drawMain`/`drawOptions`/... in `menu_main.go`, `menu_options.go`,
  `menu_game.go`, `menu_draw.go`. Primitives:
  - `drawMenuTextBox` 9-patch using `gfx/box_tl/ml/bl/tm/mm/mm2/bm/tr/mr/br.lmp`
    (`menu_draw.go:14-66`)
  - `drawPlaqueAndTitle` `gfx/qplaque.lmp` + centered title pic
    (`menu_draw.go:87-100`)
  - `drawCursor` animated `gfx/menudot1..6.lmp` on 200ms frames, fallback
    `gfx/m_surfs.lmp` then char 12 (`menu_draw.go:105-119`);
    `drawArrowCursor` blinking chars 12/13 at 250ms (:123-126)
  - per-page pics: `gfx/ttl_main.lmp`, `gfx/mainmenu.lmp` (split at row 60 for
    mods, `ensureMainMenuSplit`), `gfx/sp_menu.lmp`, `gfx/mp_menu.lmp`,
    `gfx/p_load.lmp`, `gfx/p_save.lmp`, `gfx/p_multi.lmp`, `gfx/p_option.lmp`,
    `gfx/help%d.lmp`, `gfx/bigbox.lmp`, `gfx/menuplyr.lmp` (with
    `TranslatePlayerSkinPixels` shirt/pants palette remap)
  - row strides: options 20px, controls 8px (`controlRowY(i)=24+i*8`),
    video 14px (`videoRowY(i)=28+i*14`), audio 32px; load/save slots at
    `(24, 32+i*8)` with arrow at `(8, 32+cursor*8)`
- Theming: no theme system. Colors are palette indices; text alternates
  low row (brownish) vs high row (+128, bright) via `white` flag in
  `drawText` (`menu_draw.go:132-148`). Backdrop: `drawMenuBackdrop`
  (`game_runtime_ui.go:170-183`) full-screen `DrawFillAlpha(..., 0, scr_menubgalpha)`.
- Sounds: `menuSoundNavigate/Select/Cancel = misc/menu1.wav, menu2.wav,
  menu3.wav` (`manager.go:87-89`) → `playMenuSound`
  → `g.Audio.PlayLocalSound("sound/"+name)` (`game_audio.go:190-199`).
- Wiring: `game_init.go:768-773`: `g.Draw = draw.NewManager()`,
  `g.Menu = menu.NewManager(g.Draw, g.Input, g.Host.CVar)` +
  `SetCommandText(g.Host.Cmd.AddText)` + `SetSoundPlayer(g.playMenuSound)`.

### 4. Console (`internal/console`)

- `Console` struct (`console.go:64-152`): ring buffer of fixed-width lines
  (`text []byte`, `lineWidth=78` default, `totalLines`), `backScroll`,
  `notifyTimes[4]`, `inputLine []rune`, `history[32]`, `cursorPos`,
  `insertMode`, CVar ref, injectable clock, `Title` (default
  "Ironwail-Go"; never overridden to version — noted gap).
- Open/closed state lives in the game layer: `Game.ConsoleSlideFraction`
  (`game.go:74`); `updateRuntimeConsoleSlide` eases 0..1
  (`game_runtime_ui.go:64-91`, `step = scr_conspeed * dt / 300`);
  `runtimeConsoleForcedUp` when not in active state/signon. Dropdown slide is
  implemented as a canvas transform offset, not moved draw coords:
  `CanvasConsole` transform adds `t.Offset[1] += (1.0 - p.ConSlideFraction) * 2.0`
  (`screen.go:229-236`).
- Drawing (`console/draw.go`): `Draw(rc, w, h, full, background, forcedup)`
  → `drawFull` (height = screenH/2, or full when forcedup / <32px min). BG:
  `gfx/conback.lmp` nearest-neighbour scaled to rect (`scaledBackgroundPic`,
  cached), else `DrawFill(...,0)` black (:146-202). Text rows at 8px pitch
  with `^` scroll indicator and `]` prompt; blinking block cursor glyphs
  10/11 at 4Hz (:498-501); title bottom-right (`drawWhiteText`).
  Notify lines: alpha per-line from `con_notifytime` (3s default), fade tail
  from `con_notifyfade/time`, `con_notifycenter` centers, fade faked by
  char dithering (`shouldDrawNotifyChar` :537-548).
- Input (`console_input.go`): append/backspace/delete/word-jump/cursor
  moves/history navigation/insert toggle; `CommitInput` returns line and
  pushes history. Completion (`completion.go`): `TabCompleter` with 6
  providers (command/cvar/alias/file/args/value), file specs per command
  (`playdemo|timedemo|record` → `*.dem`), column match list capped by
  `con_maxcols`. Key routing in `game_input.go:251-322`
  (`handleConsoleKeyEvent`): full key map incl. Ctrl-word-jump, PgUp/Dn
  scroll ±2, Home/End (Ctrl = scroll top/bottom). Backspace auto-repeat
  `updateRuntimeTextEditRepeat` (:355-391, 0.45s delay / 0.05s repeat).
- Theming: palette-indexed text; bit 7 = bronze row; `\x01` (chat) / `\x02`
  (warnings) prefixes set the high bit (`console.go:565-569`); `conchars`
  font; `conback.lmp` background; hardcoded prompt/`^`/title.
- Cvars: `scr_conwidth`, `scr_conscale`, `scr_conspeed`, `con_notifytime`,
  `con_logcenterprint`, `con_maxcols`, `con_notifycenter`,
  `con_notifyfade`, `con_notifyfadetime` (registered `game_init.go:260-282`).
- The console implements a pluggable `DrawContext` interface
  (`console/draw.go:40-44`: `DrawFill/DrawCharacter/DrawPic` + optional
  `StringDrawer`), so it can be retargeted.

### 5. HUD (`internal/hud`)

- `HUD` struct (`hud.go:46-60`) owns `StatusBar`, `CompactHUD`, `Crosshair`,
  `Centerprint`; consumes `hud.State` (`hud.go:63-96`: health/armor/ammo x4,
  `Items` mask, mod flags, game type, scores, paused/cutscene/intermission,
  centerprint text, level name, secrets/monsters counts, time).
- `HUDStyle` enum (`hud.go:19-37`): `Classic=0`, `ModernCenterAmmo=1`,
  `ModernSideAmmo=2`, `QuakeWorld=3` (+ `Compact` alias). Selected by
  `hud_style` cvar (legacy alias `hudstyle`), re-read every frame.
- `HUD.Draw` (`hud.go:158-190`): `setHUDCanvasParams` → dispatch:
  - Modern (`viewsize<120`): `CanvasSbar2` → `status.DrawModern`
  - QuakeWorld: `CanvasSbar` → `status.DrawQuakeWorld`
  - Classic: `CanvasSbar` → `status.Draw`
  - `CanvasCrosshair` → crosshair; `CanvasDefault` → centerprint
- `StatusBar` (`status.go:18-50`) preloads all WAD palette pics (sbar/ibar/
  scorebar/ranking/disc/weapons x7/ammo/armor/items/sigils/faces/nums 0-9 +
  anum alt set); pickup flash times; drawBigNum uses `num_*`/`anum_*` pics
  (24px, right-aligned; alt set when armor ≤25/health ≤25/ammo ≤10 —
  `status.go:517-548`); scoreboard (`drawScoreboard`, :859-887);
  `colorForMap` = map value +8; sbar alpha `currentSbarAlpha()`
  (`scr_sbaralpha`, default 0.75).
- `Centerprint` (`centerprint.go:30-75`): typewriter reveal (scr_printspeed),
  background modes (`scr_centerprintbg`: 0 off, 1 box via 9-patch
  `gfx/box_*.lmp`, 2 solid panel palette-4, 3 strip),
  intermission/finale pics, big numbers/slashes.
- `Crosshair` (`crosshair.go:24-46`): conchars glyph centered in
  `CanvasCrosshair`; cvar `crosshair`: 0 off, <0 custom char, >1 dot(15), 1 `'+'`.
- `CompactHUD` (`compact.go:26`): **instantiated but never drawn — dead code
  outside tests** (noted gap).
- Cvars: `hud_style`, `scr_sbarscale/alpha`, `scr_menuscale`,
  `scr_crosshairscale`, `scr_centerprintbg`, `scr_centertime`,
  `scr_printspeed`, `scr_menubgalpha`, `crosshair`, plus telemetry:
  `scr_showfps`, `scr_clock`, `scr_showspeed`, `scr_showspeed_ofs`,
  `scr_demobar_timeout` (`game_runtime_overlay.go:15-27`).

### 6. CSQC-driven UI (`internal/qc/csqc.go` + `internal/game/game_runtime_csqc.go`)

- `CSQC` struct (`internal/qc/csqc.go:58-81`): VM wrapper w/ cached entry
  points `CSQC_Init`, `CSQC_Shutdown`, `CSQC_DrawHud` (required), 
  `CSQC_DrawScores`, `CSQC_InputEvent`, `CSQC_Parse_StuffCmd`,
  `CSQC_Ent_Update` (all resolved in `Load` :103-146).
- **No `CSQC_UpdateView` entry point anywhere** (grep confirms). The engine
  does not call CSQC to mutate the camera.
- CSQC draw builtins (`internal/qc/builtins_csqc.go`): `drawcharacter` (320),
  `drawrawstring` (321), `drawpic` (322), `drawfill` (323),
  `drawsetcliparea` (324), `drawresetcliparea` (325), `drawstring` (326),
  `stringwidth` (327), `drawsubpic` (328); client builtins `getstati/f/s`,
  `getplayerkeyvalue`, `registercommand`. Registered `builtins.go:184-202`.
- Game-side hooks (`game_runtime_csqc.go`): `buildCSQCDrawHooksWithActivity`
  (:94-253) renders through `RenderContext` — `DrawCharacter` 8x8 glyph,
  `DrawString` per-rune advance, `DrawPic` via `cacheCSQCPic`, `DrawFill`
  RGB→nearest palette index (`internal/game/csqc/csqc.go:39`),
  `DrawSubPic` sub-rect, clip area. `drawRuntimeCSQCHUD(rc, showScores)`
  (:280-307) sets `CanvasCSQC`, calls `CallDrawHud` then `CallDrawScores`.
  `drawRuntimeHUDLayer` (:309-323): **if CSQC draws, native HUD is skipped**;
  else fall back to native.
- Input: `handleGameKeyEvent` (`game_input.go:51-66`) calls
  `CSQC_InputEvent` first and swallows the key if nonzero. `StuffCmd` filter:
  `ConsumeStuffCommands` (`client_events.go:63-77`). Lifecycle: init on
  `prespawn` (`client.go:727`), shutdown on `ClearSignons()`.
  `HandleCSQCEntUpdate` exists but is **never called** (dead wiring, noted).
- No CSQC source ships in `pkg/qgo` — CSQC UI only exists when a mod provides
  `csprogs.dat`; the engine draws HUD natively otherwise.
- Cvar `cl_nocsqc` disables (`game_init.go:592`).

### 7. Demo playback controls and UI

- `DemoState` (`internal/client/demo.go:25-52`): file/reader/writer, playback/
  recording/paused/speed/baseSpeed/timedemo flags, `Frames []DemoFrame`
  (pre-indexed file offsets :904-936), `DemoFrame{FileOffset, SerializedEvents}`.
  Format: header CD-track line, then per frame: int32 size + 3 float32 view
  angles + message bytes.
- Commands (`internal/host/commands_demo.go`): `record`, `stop`, `playdemo`
  (PAK-aware), `timedemo`, `demoseek <frame>`, `rewind [frames]`,
  `demogoto <seconds>` (72Hz assumption `demo.go:871`), `demopause`,
  `demospeed <mult>`, `stopdemo`, `startdemos`, `demos`.
- **Seeking is full reparse from frame 0**: `seekDemoFrame`
  (`commands_demo.go:400-432`) → `SeekFrame(0)` + ClearState + replay frames
  0..n-1 through `ParseServerMessage`. No snapshot restore.
- Keyboard: `handleDemoPlaybackKeyEvent` (`game_input.go:139-176`):
  Space=play/pause, Up/Down=base speed ±, Left/Right (+Shift/Ctrl) = held
  speed adjustments via `UpdatePlaybackSpeed` (`demo.go:807-845`; ±5x base;
  Shift/Ctrl = 0.25x slow; negative = rewind, `SetRewindBackstop`).
- Frame loop (`game_loop.go:120-254`): `ShouldReadFrame` gate → AdvanceTime by
  `Speed*frametime` → rewind path re-reads/`SeekDemoFrame` → forward
  `ReadDemoFrame` → `bootstrapDemoPlaybackWorld` (loads BSP world so it
  renders) → `ParseServerMessage`. EOF → `stopRuntimeDemoPlayback` + queue
  `"demos\n"` for attract loop.
- **Demo bar UI** (`game_runtime_overlay.go:346-430`,
  `drawRuntimeDemoControls`): display-only progress bar at `CanvasSbar`
  (y=-20; `CanvasMenu` y=25 during intermission), 38 chars wide track using
  conchars 128/129/130, cursor char 131 at
  `x + int((timebarChars-1)*8 * progress)`; status glyph ("II" paused, `>`
  speed, `<` rewind), base speed label, demo name (30-char truncation), M:SS
  time readout. Visibility gated by `scr_demobar_timeout` (0=always, >0
  seconds, <0 never) with a `ShowTime` countdown. **Not clickable/draggable;
  no mouse interaction with the demo bar.** (There is a bd issue
  `ironwail-go-cuy` for "interactive timeline scrubbing and director camera
  for demo playback" — the rewrite interacts with that.)
- Menu has no demo entries; demo playback is console-command-only
  (`internal/menu` grep returns nothing). Attract mode: `CmdStartdemos`
  keeps the main menu open while a demo plays behind it
  (`game_input.go:633-657`).

### 8. Input routing (KeyDest)

`input.System.HandleKeyEvent` (`internal/input/types_binding.go:185-231`)
routes by `KeyDest` (`KeyGame`, `KeyConsole`, `KeyMenu`, ...). Menu mode:
key → `OnMenuKey` then `OnKey` if still menu. Console mode: key →
`handleConsoleKeyEvent`; text via `HandleCharEvent` → `OnMenuChar` in menu /
`console.AppendInputRune` in console. `syncGameplayInputMode`
(`game_input.go:393-441`) releases gameplay buttons on menu/console open
(`releaseGameplayButtons` :596-623 sends KeyUp to all `cl.KButton`s; mouse
grab released). `applyGameplayMouseLook` early-returns unless `KeyGame`.

Mouse position → menu cursor: `applyMenuMouseMove` (`game_input.go:446-488`)
converts absolute screen pos via inverse of `GetCanvasTransform(CanvasMenu)`
(`screenToMenuCoords`), else uses deltas. Latching: KMouse1/KMouse2 handled as
keys per page. Regression tests in `game_movement_input_test.go:145-329`.

### 9. Asset loading (`internal/draw/manager.go`)

- `draw.Manager` (`draw/manager.go:31-56`) owns `gfx.wad`, filesys, `pics`
  cache, palette, conchars (custom detection via
  `standardConcharsHash = 0x9d6f0ea8` :58).
- `Pic(name)` (:209-237): cache → WAD full name → WAD bare name → pak
  filesystem → base dir (`loadPic` :257-279). This is the menu's
  `DrawManager` interface.
- `ConcharsData()` (:366-377) and `Palette()` (:395+) feed the renderer via
  `SetConchars`/`SetPalette` at `game_init.go:943-947` (and `assets.go:53-56`).

## Recommended Resolution (integration targets for the rewrite)

- The UI is **5 independent hand-rolled subsystems** (menu, console, HUD,
  CSQC-HUD-bridge, demo bar) sharing only the `RenderContext` 2D interface,
  a `KeyDest` input router, and the canvas-transform math. Any gogpu/ui rewrite
  must preserve (or deliberately re-map) each of these contracts and their
  cvar-driven appearance.
- Parity reference is C Ironwail: `menu.c/menu.h`, `con.c`, `sbar.c/hud.c`
  (`internal/menu/doc.go:22`, console doc.go, hud doc.go cite lineage).
- Key cvar surface to preserve: `scr_menuscale`, `scr_menubgalpha`,
  `scr_conwidth/conscale/conspeed`, `con_notify*`, `scr_sbarscale/alpha`,
  `scr_crosshairscale`, `hud_style`, `scr_demobar_timeout`, and the
  menu-key/gamepad bindings.
- Gaps to consider: `CompactHUD` dead code; `Console.Title` never set;
  `CSQC_Ent_Update` never called; no `CSQC_UpdateView`; demo bar not
  interactive; CSQC-native-HUD fallback coupling.

## Open Questions / Follow-ups

- To what extent must the gogpu/ui rewrite preserve retro rendering (8x8
  conchars scaled, palette-index colors) vs a "modern" reskin? (spec decision)
- Is the CPU overlay compositor / `flush2DOverlay` path retained as an adapter
  target, or replaced by gg canvas rendering? (design decision)

## Source Index

- Local source deep-dive (internal/menu, internal/console, internal/hud,
  internal/game, internal/renderer, internal/input, internal/qc,
  internal/client) — file:line references inline above.
- C lineage: `ironwail/Quake/menu.c,menu.h,con.c,sbar.c,hud.c` via doc.go
  citations in each package.
