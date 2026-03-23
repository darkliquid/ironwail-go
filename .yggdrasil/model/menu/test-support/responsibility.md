# Responsibility

## Purpose

`menu/test-support` owns the menu package's test doubles and behavioral coverage for state transitions, command queueing, provider refresh, and menu-space rendering.

## Owns

- Mock input backend and mock draw/render contexts used by menu tests.
- Cross-screen tests that verify menu activation, navigation, command emission, provider refresh, and draw output.
- Test-only helpers for cvar setup and rendered-line extraction.

## Does not own

- Production menu behavior itself.
- Reusable test utilities shared outside the menu package.
