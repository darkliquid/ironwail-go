# Responsibility

## Purpose

`console` owns the Quake-style textual front end for diagnostics, developer output, command entry, scrollback, and on-screen notify lines.

## Owns

- Scrollback buffer state, notify timestamps, and optional debug-log output.
- Console input editing and command history.
- Tab-completion infrastructure through injected providers.
- Backend-neutral drawing of the full console and gameplay notify overlay.

## Does not own

- Command execution itself, which belongs to `cmdsys`.
- Key-destination policy and input routing decisions outside the console package.
- Renderer backend implementation details beyond the `DrawContext` abstraction.
