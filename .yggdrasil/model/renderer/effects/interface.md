# Interface

## Main consumers

- backend-specific render paths
- renderer runtime code preparing entity/effect state

## Contracts

- this node provides backend-neutral rendering helpers that concrete backends consume when drawing models, particles, and dynamic effects
- `ParticleSystem.RocketTrail` preserves tracer phase (`tracerCount`) across trail emissions so alternating tracer lateral velocity and color cadence match Quake behavior frame-to-frame, while `Clear` resets that phase with other particle state
- dynamic-light helper paths honor `r_dynamic` as a hard gate for both spawn-side emission (`EmitDynamicLights`, `EmitEntityEffectLights`) and contribution evaluation (`evaluateDynamicLightsAtPoint`)
- `r_dynamic` gate behavior defaults to enabled when the cvar is absent so isolated/unit helper paths preserve historical dynamic-light behavior unless the runtime explicitly registers and disables `r_dynamic`
- particle emission helpers now treat `nil` RNG inputs as "use process-global compatibility stream" and pull from `internal/compatrand` rather than creating per-call fixed-seed RNGs
- entity-particle angular velocity seeds are initialized once from the shared compatibility RNG stream so static-velocity tables match process-global rand-sequence parity expectations
