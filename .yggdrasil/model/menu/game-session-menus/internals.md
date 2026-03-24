# Internals

## Logic

These screens bridge menu UI into actual game/session actions. Load/save pages are simple slot pickers that queue `load`/`save` commands. Join Game combines a typed address field, a `ServerBrowser`-backed LAN search, and result selection that can copy an advertised address into the connect field before applying `connect`. Host Game mirrors cvars into editable fields, lets the player adjust max players, mode, teamplay, skill, frag limit, time limit, and map name, then queues a deterministic startup sequence ending in `map`. The listen command now toggles based on `hostMaxPlayers` (`listen 1` for multiplayer hosting, `listen 0` for single-player hosting) so menu launch behavior matches host command parity.

## Constraints

- Join Game and Host Game map/address entry are printable-ASCII only and length-limited.
- `drawJoinGame` mutates manager state by refreshing browser results and clamping the cursor during draw.
- Host-game setting ranges and wrap rules are hard-coded (e.g. maxplayers 2–16, skill 0–3, fraglimit 0–100 by tens, timelimit 0–60 by fives).
- Save/load labels fall back to plain `sN` names when no provider is installed or a slot label is blank.

## Decisions

### Treat session-launch pages as command/cvar staging UIs rather than direct subsystems

Observed decision:
- Join/host/load/save pages prepare state locally and then hand off through queued commands or providers instead of directly manipulating session/runtime objects.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The UI remains decoupled from lower-level session logic, but page behavior is tightly coupled to the exact command strings and cvars expected elsewhere in the engine.
