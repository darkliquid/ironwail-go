# Internals

## Logic

### Parsing

The parser consumes a `SizeBuf`-style message stream and dispatches by service command. Entity updates use the high-bit command path, while service commands update core client state, event queues, signon, stats, sounds, and text.

Intermission-sensitive updates mirror C Ironwail details:
- `svc_killedmonster` / `svc_foundsecret` increment `Client.Stats[STAT_MONSTERS/STAT_SECRETS]` and matching float mirrors (`StatsF`) in addition to convenience counters.
- Re-release opcodes `svc_levelcompleted` and `svc_backtolobby` are treated as no-payload no-ops to avoid parser aborts when present in modern streams.

It also contains a compatibility guard for an optional trailing `0xFF` sentinel used by some Go-side packet builders: `0xFF` is treated as end-of-message only when it is the last unread byte. This preserves C-compatible behavior where `0xFF` can still be a valid fast entity-update command byte (`U_SIGNAL | 0x7f`).

### Relink and interpolation

Relink interpolates entity origins and angles between the double-buffered message states, applies teleport/forcelink reset rules, emits trail events, and removes entities that were not refreshed in the latest message.

### Temp effects and blends

Temp entity decoding and view-blend state produce transient render/audio-facing information that is later consumed by other systems. Temp entities now route coordinate decoding through the parser's protocol-aware helpers rather than the legacy fixed-point-only helpers used by older message paths, so beams/explosions stay aligned with servers that enable float/int32/24-bit coord flags.
Beam segment generation mirrors C roll jitter and RNG side effects by reseeding the package-shared compat rand stream (`ResetShared(int32(Client.Time*1000))`) at each temp-entity update pass, then consuming `rand()%360` from that shared stream per emitted segment in sequence across active beams.
`svc_fog` parsing routes through the shared runtime fog helper and decodes fade time from the C wire format (`short` divided by `100.0`) instead of float payload bytes, keeping parsed server messages and local console fog changes aligned on transition semantics.
`svc_bf` now executes the same bonus-flash side effect as the command path by calling `Client.BonusFlash()` directly during parse dispatch.
`svc_disconnect` now performs full `Client.ClearState()` teardown before transitioning state to disconnected and returning the disconnect error.

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
