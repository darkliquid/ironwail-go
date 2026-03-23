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
- QC profiling (`profile`) for top QuakeC function counters on an active local server
- demo playback and capture
- system/config execution
- audio-facing host helpers

## Contracts

- Host commands execute under command-source gating from `cmdsys`.
- Some commands intentionally forward to the remote server only when command source is local console input and no active local server session owns the command path; merely having an initialized server subsystem must not block remote forwarding.
- Save/load commands enforce strict save-name and path safety rules.
- Local map/changelevel/load commands may synchronously mutate both host and loopback client state.
- Successful load operations synchronize autosave timing with restored server time so autosave scoring does not fire immediately after a save restore.
- `profile` is local-server/QC-only: it no-ops when no active local server/QC VM is available, prints at most 10 lines in `%7d %s` format, and consumes/reset VM counters.

## Failure modes

- Invalid save names, unsafe paths, wrong active game directories, or missing save files are rejected.
- Network commands surface transport/setup failures.
- Demo commands reject incompatible or unsupported states.
