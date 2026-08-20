# ADR-0008: Visual Fidelity — Real LMP Pics + Conchars Bitmap Text

**Status:** Accepted (2026-08-19)
**Deciders:** darkliquid (v2 Stage 1: pics-base-ttf), lifecycle driver
**Date:** 2026-08-19
**Supersedes:** ADR-0004 (v1 TTF text — abandoned)
**Related:** IRONWAIL-SPEC-002 §1.4/§4.3-4.4/AC5; gap log rows 4, 11

## Context and Problem Statement

SPEC-001 used TTF text and no real menu art, producing visuals that "look
nothing like" the original Quake UI. SPEC-002 requires visual fidelity: the
actual `gfx/*.lmp` menu images (plaques, titles, main menu artwork, cursor
dots) and the conchars bitmap text at the legacy 320x200 layout. The user
chose "pics for art, TTF for text" (G4) — but the menu text itself must match
the original conchars bitmap look, not a TTF font.

## Decision Drivers

- Visual fidelity with the original UI (the primary v2 goal).
- The menu virtual viewport is 320x200 (C lineage, `gl_draw.c:1214`; G11).
- gogpu/ui's Canvas has `DrawImage(img, at)`; WAD QPic pixels are
  palette-indexed and must be converted to RGBA.
- Conchars is a 128x128 bitmap atlas (16x16 grid of 8x8 glyphs); the original
  menu text is drawn from it, not a TTF.

## Considered Options

1. **TTF for all text (v1 ADR-0004)** — rejected: produces non-Quake visuals;
   the v1 experiment failed on this.
2. **Real LMP pics for art + conchars bitmap for text** — chosen. All menu
   art (`gfx/ttl_main.lmp`, `gfx/mainmenu.lmp`, `gfx/sp_menu.lmp`,
   `gfx/mp_menu.lmp`, `gfx/p_option.lmp`, `gfx/menudot1..6.lmp`, plaques,
   box pics) is drawn via `QPicToImage` (palette → RGBA, index 255
   transparent) + `canvas.DrawImage` at legacy 320x200 positions. All menu text
   is drawn from the conchars bitmap atlas (per-glyph `image.RGBA.SubImage`),
   preserving the 8x8 retro glyphs and the high-bit bright row. No TTF.
   Pros: pixel-faithful visuals; matches C lineage; the Canvas `DrawImage`
   path is proven. Cons: per-glyph SubImage draws (hundreds per frame) and
   atlas lifecycle management.
3. **LMP pics for art + TTF for text** — rejected: the user's "pics for art,
   TTF for text" was about art vs text; the menu text must be conchars bitmap
   to match the original. TTF only if a future full-window unscaled UI wants
   it (out of scope).

## Decision Outcome

Real LMP pics for menu art + conchars bitmap text, at the legacy 320x200
layout, scaled by the `CANVAS_MENU` transform (spec §4.4). The `quakeui` pics
bridge (`QPicToImage`) converts palette-indexed WAD pics to RGBA for
`canvas.DrawImage`; the conchars atlas provides per-glyph SubImage draws.

- **Positive:** pixel-faithful visuals; matches C lineage; Canvas `DrawImage`
  path proven.
- **Negative:** per-glyph SubImage draws and atlas lifecycle management; no
  TTF for menu text (a future full-window unscaled UI could add TTF, out of
  scope).

## Links

- IRONWAIL-SPEC-002 §1.4/§4.3-4.4/AC5; gap log 4, 11; ADR-0009 (isolation)

## Review Log

### Stage 5 — Review 2 (2026-08-19)

Verdict: APPROVED WITH ONE CLARIFICATION. Supersedes ADR-0004 honestly. The
"pics for art, TTF for text" user decision (G4) is clarified: the primary v2
goal is visual fidelity with the original UI, and the original menu text is
conchars bitmap, so menu text is conchars (not TTF) — the "TTF for text" part
 of the elicit answer does not apply to menu text. The 320x200 viewport and
CANVAS_MENU transform are pinned (G11). Consistent with SPEC-002 §4.3-4.4 and
AC5. Gap log rows 4, 12.
