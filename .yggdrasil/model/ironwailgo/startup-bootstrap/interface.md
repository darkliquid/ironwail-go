# Interface

## Main consumers

- the top-level app shell during executable startup.
- tests that verify startup flag parsing and bootstrap policies.

## Main surface

- startup option parsing helpers
- `initSubsystems` and bootstrap helpers such as `initGameQC`, `initGameServer`, `initGameRenderer`

## Contracts

- Startup builds the authoritative subsystem graph through `host.Subsystems`.
- The server-owned QC VM becomes the authoritative QC VM used by app startup.
- Renderer/input initialization follows explicit platform/build-tag policy rather than being left implicit.
