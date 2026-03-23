# Interface

## Main consumers

- binary data loaders and decoders that need endian-safe scalar extraction
- any caller that wants small stream-oriented little-endian read/write helpers

## Main surface

- `LittleShort`, `LittleLong`, `LittleFloat`
- `BigShort`, `BigLong`, `BigFloat`
- `WriteLittleShort`, `WriteLittleLong`, `WriteLittleFloat`
- `ReadLittleShort`, `ReadLittleLong`, `ReadLittleFloat`

## Contracts

- Helpers assume callers provide enough bytes for slice-based reads.
- Reader-based helpers consume exact fixed widths via `io.ReadFull`.
- The subpackage is intentionally tiny and policy-free: callers supply framing, validation, and structure.
