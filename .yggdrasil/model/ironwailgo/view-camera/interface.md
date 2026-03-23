# Interface

## Main consumers

- runtime rendering/presentation code that needs camera and viewmodel state.
- runtime loop logic that updates view-dependent subsystems.

## Main surface

- runtime camera helpers such as `runtimeViewState`, `runtimeCameraState`, `runtimeWeaponBaseOrigin`, `runtimePlayerOrigin`
- chase update helpers
- shared view calculation helpers/state in `viewcalc.go`

## Contracts

- The first-person camera is authoritative-first: authoritative origin is preferred and predicted XY is only a policy/telemetry aid.
- Shared view smoothing state is reused across camera/viewmodel/audio consumers.
- Chase and first-person view math are constrained by canonical cvars and Quake-style bounds logic.
