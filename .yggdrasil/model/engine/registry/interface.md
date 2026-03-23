# Interface

## Main consumers

- Systems that build tables during startup and then depend on stable runtime lookup.
- Callers that want programming errors surfaced immediately when registration is inconsistent.

## Main surface

- `NewRegistry`
- `Register`
- `Lookup`
- `MustLookup`
- `Freeze`
- `Len`
- `Range`

## Contracts

- `Register` panics on duplicate keys or any registration after `Freeze`.
- `MustLookup` panics when a required key is missing.
- Runtime readers observe a stable map once initialization is complete.
