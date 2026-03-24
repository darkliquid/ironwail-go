# Interface

## Main consumers

- backend-specific render paths
- renderer runtime code preparing entity/effect state

## Contracts

- this node provides backend-neutral rendering helpers that concrete backends consume when drawing models, particles, and dynamic effects
- `ParticleSystem.RocketTrail` preserves tracer phase (`tracerCount`) across trail emissions so alternating tracer lateral velocity and color cadence match Quake behavior frame-to-frame, while `Clear` resets that phase with other particle state
