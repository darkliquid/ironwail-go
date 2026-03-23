# Interface

## Main consumers

- the operating system / launcher invoking the binary.
- tests that drive the command package as a whole.

## Main surface

- `main()`
- `Game`
- version constants and top-level runtime helpers referenced across the package

## Contracts

- `Game` is the process-wide mutable shell tying together host, client, server, renderer, UI, caches, and runtime state.
- `main_test.go` asserts cross-cutting orchestration and policy behavior across many child areas.
