# Responsibility

## Purpose

`ironwailgo/startup-bootstrap` owns startup argument parsing and the one-time initialization path that composes engine subsystems into a running `Game` instance.

## Owns

- Startup option parsing and environment/cvar-driven boot policy.
- Filesystem, QC, networking, server, renderer, input, menu, draw, HUD, audio, and host initialization.
- Construction of `host.Subsystems` and loopback client/server setup.
- Startup-time renderer/input backend selection policy and related tests.

## Does not own

- Per-frame runtime loop behavior after startup succeeds.
- Ongoing input, camera, entity, or audio update policy during gameplay.
