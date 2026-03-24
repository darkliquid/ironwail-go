# Internals

## Logic

This node implements three distinct settings surfaces on top of shared menu machinery. The controls page mixes simple settings (mouse speed, invert mouse, always run, free look) with bindable actions, including a rebinding sub-mode that captures the next key or clears bindings through left/backspace. The video page cycles through a curated set of resolutions and common renderer/HUD cvars, while the audio page currently only exposes `s_volume`. Bind labels are derived by scanning every input key for matching command strings, and many helper functions exist only to keep display labels and nearest-index logic consistent.

## Constraints

- Controls-menu rows before `controlsBindingStart` are setting toggles/sliders; rows at or after that index are binding rows.
- Video rows are tightly coupled to specific cvars and curated value lists (`videoResolutions`, `maxFPSValues`).
- Back rows are treated specially across all these menus.
- Absolute mouse hit testing for the expanded video menu is more brittle than keyboard navigation because row tables must stay in sync with draw layout.

## Controls key-matrix parity

- The in-game Controls page now exposes **21 bindable actions**.
- In addition to the original rows (`+forward`, `+back`, `+left`, `+right`, `+moveleft`, `+moveright`, `+jump`, `+attack`, `+use`, `impulse 10`, `impulse 12`, `toggleconsole`), it now includes:
  - `+speed`
  - `+strafe`
  - `+lookup`
  - `+lookdown`
  - `centerview`
  - `+mlook`
  - `+klook`
  - `+moveup`
  - `+movedown`
- Rebind and clear semantics are unchanged:
  - Enter/Right starts key capture on binding rows,
  - Left/Backspace clears all keys bound to the selected command.

## Decisions

### Make settings pages live-edit engine state instead of staging everything behind Apply

Observed decision:
- Most control/video/audio menu interactions immediately update cvars or bindings as the cursor moves or selections are made.

Rationale:
- **unknown — inferred from code and Quake menu lineage, not confirmed by a developer**

Observed effect:
- The menus feel like classic Quake settings surfaces, but UI navigation has immediate engine-side effects and depends on coherent cvar/binding defaults outside the package.
