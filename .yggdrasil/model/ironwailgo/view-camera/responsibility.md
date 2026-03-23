# Responsibility

## Purpose

`ironwailgo/view-camera` owns camera and view computation for the executable: first-person base origin/angles, chase camera placement, bob/smoothing/damage effects, and related canonical cvar/view policy.

## Owns

- Shared view calculation state and helpers in `viewcalc.go`.
- Runtime first-person camera composition in `game_camera.go`.
- Chase camera behavior in `chase.go`.
- View/camera tests covering canonical cvar usage and camera math.

## Does not own

- Input backend implementation or entity collection.
- Rendering of the resulting camera state after it is computed.
