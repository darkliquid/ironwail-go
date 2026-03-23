# Responsibility

## Purpose

The `qc` node is the umbrella for the QuakeC subsystem. It covers VM state representation, `progs.dat` loading, bytecode execution, builtin bridging into engine services, and CSQC support.

## Owns

- The package-level decomposition of QuakeC concerns into VM model, execution core, builtin bridge, and CSQC support.
- The boundary between compiled QuakeC programs and engine-hosted services.

## Does not own

- Server or client policies that decide when QuakeC entry points are invoked.
- Concrete server/client/render/audio implementations behind builtin hooks.
