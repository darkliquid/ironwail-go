# Responsibility

## Purpose

`ironwailgo/runtime-visual-audio` owns the command-package-side presentation helpers that apply current runtime state to rendering overlays/submission and audio output.

## Owns

- Visual update helpers that coordinate renderer submission, overlays, HUD/menu state, and related presentation wiring.
- Audio update helpers that apply current listener/view/game state to the audio system.
- The final command-package bridge from runtime state into render/audio subsystem calls.

## Does not own

- Core camera/view or entity collection logic, which provide inputs to this node.
- The renderer or audio subsystem implementations themselves.
