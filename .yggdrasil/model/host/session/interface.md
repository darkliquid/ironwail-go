# Interface

## Main surfaces

### `remoteDatagramClient`

Observed responsibilities:
- initialize and reset remote client state
- parse inbound server messages
- accumulate frame time for client command generation
- send unreliable commands and signon replies
- expose underlying client state for host/session code

Contracts:
- Signon stages `1`, `2`, and `3` map to `prespawn`, `name/color/spawn`, and `begin`.
- Remote shutdown closes the socket and clears client state.

### Autosave support

Observed surface:
- host-internal autosave bookkeeping and trigger logic invoked after server frames.

Contracts:
- Autosave only runs in active single-player conditions judged safe by host policy.
- Rich autosave behavior depends on concrete server state beyond abstract interfaces.
