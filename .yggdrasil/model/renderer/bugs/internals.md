# Internals

## Logic

This node holds isolated GoGPU bug repros in their own package so they do not pollute the regular `renderer` test surface. The tests spin up a tiny `gogpu.App`, wait for `DeviceProvider`, and then exercise copied `wgpu` shader/pipeline calls directly.
The particle repro WGSL now matches the production fix by replacing writable swizzle compound assignment (`clipPosition.xy += ...`) with explicit scalar `vec4<f32>` reconstruction after computing `clipOffset`, and the particle repro test now asserts compile success to guard against regressions in this specific Naga/SPIR-V lowering edge case.

## Constraints

- crash repros must stay in subprocess helpers
- repro harnesses should avoid `QUAKE_DIR`, world upload, and the normal Ironwail renderer startup path when a smaller standalone shape works
- shader strings here intentionally duplicate the failing source so production code can move independently from the repro

## Decisions

### Standalone bugs package for repros

Observed decision:
- GoGPU dependency repros now live in `internal/renderer/bugs` instead of the main `renderer` package tests.

Rationale:
- This keeps the repro surface fully isolated from normal renderer code, makes the tests' intent clearer, and avoids test-only helpers living beside ordinary renderer behavior tests.
