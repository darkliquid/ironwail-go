# Responsibility

## Purpose

`menu/options-and-bindings` owns the settings surfaces under the Options menu: controls rebinding, video settings, audio volume, and a few top-level option shortcuts.

## Owns

- Options-page navigation into controls/video/audio.
- Controls-menu rebinding mode, binding clearing, and live cvar toggles/sliders.
- Video-menu resolution/toggle/cycle behavior.
- Audio-menu volume adjustment.
- Display-label helpers for bindings, resolutions, max-FPS choices, waterwarp, and HUD style.

## Does not own

- Main/help/setup/quit shell flow.
- Load/save/join/host-game behavior.
- The underlying implementation of the cvars and bindings it edits.
