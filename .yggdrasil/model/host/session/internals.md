# Internals

## Logic

### Remote datagram client

The remote adapter owns a concrete client/parser pair plus socket state. It reads inbound messages until transport exhaustion, parses server packets, remembers the last signon reply sent, and emits the next required reply commands automatically.

### Autosave heuristics

Autosave is host policy, not merely elapsed-time scheduling. The score combines:
- elapsed safe time
- current health and armor
- recent damage and recent shooting
- movement speed
- noclip/godmode/notarget cheat windows
- secret discovery boost
- recent teleport boost

Only when the score reaches threshold does host enqueue an autosave command.

## Constraints

- Remote signon sequencing depends on the underlying client parser reporting signon progress correctly.
- Autosave behavior depends on host state, loopback client state, player edict state, and concrete server globals.

## Decisions

### Conservative autosave policy

Observed decision:
- Autosave uses a weighted safety score instead of a simple fixed interval trigger.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- autosaves are delayed during danger, motion, or cheat-like states
- level progression and secret discovery can bias the save timing
