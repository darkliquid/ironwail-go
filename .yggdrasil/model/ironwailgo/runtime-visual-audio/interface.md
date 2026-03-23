# Interface

## Main consumers

- runtime loop code that triggers per-frame presentation updates.

## Main surface

- visual update helpers in `game_visual.go`
- audio update helpers in `game_audio.go`

## Contracts

- Presentation consumes outputs from camera/view, entity collection, menu/HUD, and runtime state rather than recomputing them.
- Audio and visual updates are part of the per-frame orchestration path, not autonomous subsystems.
