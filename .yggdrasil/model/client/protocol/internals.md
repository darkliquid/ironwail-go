# Internals

## Logic

### Parsing

The parser consumes a `SizeBuf`-style message stream and dispatches by service command. Entity updates use the high-bit command path, while service commands update core client state, event queues, signon, stats, sounds, and text.

It also contains a compatibility guard for an optional trailing `0xFF` sentinel used by some Go-side packet builders: `0xFF` is treated as end-of-message only when it is the last unread byte. This preserves C-compatible behavior where `0xFF` can still be a valid fast entity-update command byte (`U_SIGNAL | 0x7f`).

### Relink and interpolation

Relink interpolates entity origins and angles between the double-buffered message states, applies teleport/forcelink reset rules, emits trail events, and removes entities that were not refreshed in the latest message.

### Temp effects and blends

Temp entity decoding and view-blend state produce transient render/audio-facing information that is later consumed by other systems.

## Constraints

- Parse ordering and signon transitions are parity-sensitive.
- Interpolation depends on the double-buffered `MTime`, message origins/angles, and force-link state being updated consistently.
- Unsupported or malformed commands must fail in a controlled way to avoid silently corrupting client state.

## Decisions

### Explicit parser object over ambient protocol globals

Observed decision:
- The Go port uses a `Parser` object tied to `Client` instead of dispersing parse logic across ambient global state.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- protocol decoding is easier to test and trace in isolation
