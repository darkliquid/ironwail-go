# Interface

## Consumers

- Console input
- config scripts (`quake.rc`, config files, `stuffcmds`)
- runtime code that registers host commands during startup

## Main surface

The public surface is command-oriented rather than method-oriented. Observed command groups:
- map/session lifecycle
- remote networking and reconnect
- save/load
- demo playback and capture
- system/config execution
- audio-facing host helpers

## Contracts

- Host commands execute under command-source gating from `cmdsys`.
- Some commands intentionally forward to the remote server only when the host is connected remotely and not acting as a local server.
- Save/load commands enforce strict save-name and path safety rules.
- Local map/changelevel/load commands may synchronously mutate both host and loopback client state.

## Failure modes

- Invalid save names, unsafe paths, wrong active game directories, or missing save files are rejected.
- Network commands surface transport/setup failures.
- Demo commands reject incompatible or unsupported states.
