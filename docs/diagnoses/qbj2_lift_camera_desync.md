# qbj2 Lift Descent — Camera/Player Desync Diagnosis

## Root Cause: 16-bit Coordinate Wraparound

**RESOLVED** — the Go server defaulted to FitzQuake protocol (666) with 16-bit
fixed-point coordinates (max ±4096), while C Ironwail defaults to RMQ protocol
(999) with 32-bit integer coordinates. qbj2's lift shaft extends to Z=-6608,
exceeding the 16-bit range. Coordinates wrapped around, causing the client to
render the player at a completely different Z than the server had them.

## The Bug

The Go server (`internal/server/server.go:271`) hardcoded
`Protocol: ProtocolFitzQuake` (666). C Ironwail defaults to
`sv_protocol = PROTOCOL_RMQ` (999) with `PRFL_INT32COORD | PRFL_SHORTANGLE`
(`ironwail/Quake/sv_main.c:32,1962`).

With 16-bit fixed-point encoding (`value * 8` as `int16`), the maximum
representable coordinate is ±4096. qbj2's lift path goes from Z=-2000 to
Z=-6608. When the player crossed below Z=-4096, the coordinate wrapped:

| Server Z | int16(value*8) | uint16 | Decoded Z | Error |
|----------|---------------|--------|-----------|-------|
| -4387.32 | -35099 → 30437 | 30437 | +3804.62 | +8192 |
| -6237.23 | -49898 → 15638 | 15638 | +1954.75 | +8192 |
| -6567.97 | -52544 → 12992 | 12992 | +1624.00 | +8192 |

The server correctly moved the player (physics fine, triggers activated), but
the client received wrapped coordinates and rendered the camera 8192 units
above the actual position.

## The Fix

Changed the default protocol from `ProtocolFitzQuake` to `ProtocolRMQ`,
matching C Ironwail. This enables `PRFL_INT32COORD` (32-bit integer coordinates,
max ±2,097,152) and `PRFL_SHORTANGLE` (16-bit angles), which is what C
Ironwail uses by default.

### Files Changed

- `internal/server/server.go` — default `Protocol: ProtocolRMQ`
- `internal/server/server_test.go` — updated `TestStartSound` expected sizes
  for 32-bit coords
- `internal/client/client_parse_misc_test.go` — set client protocol flags in
  `TestParseLiveServerEntityDatagrams` to match RMQ server

### Supporting Code Already Present

The Go codebase already had full support for RMQ protocol — both the
server-side `WriteCoord`/`ReadCoord` (`message.go:151-164,287-299`) and
client-side `p.readCoord`/`p.readAngle` (`parse.go:488-516,464-484`) handled
all protocol flag formats. Only the default was wrong.

## Diagnostic Tool

A `camdebug` console command was added (`game_commands.go`) that dumps the
camera origin, entity origin, network origin, server origin, and interpolation
state. Bind it to a key with `bind x camdebug` to inspect camera/entity sync
during gameplay.
