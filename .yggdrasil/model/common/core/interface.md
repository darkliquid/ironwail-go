# Interface

## Main consumers

- `client`, which uses `SizeBuf` for low-level message parsing/writing.
- `server/savegame`, which uses `COM_Parse` and newline-aware parse helpers.
- any subsystem that needs Quake-style argv checking, path manipulation, or shared small data structures.

## Main surface

- `SizeBuf`, `NewSizeBuf`, `BeginReading`, and primitive read/write helpers
- `COM_InitArgv`, `COM_CheckParm`, `COM_CheckParmNext`
- `COM_Parse`, `COM_ParseEx`, `COM_ParseIntNewline`, `COM_ParseFloatNewline`, `COM_ParseStringNewline`
- `COM_SkipPath`, `COM_StripExtension`, `COM_FileGetExtension`, `COM_FileBase`, `COM_AddExtension`
- `COM_HashString`, `COM_HashBlock`, `Link`, and `BitArray`

## Contracts

- `SizeBuf` callers must check overflow/underflow return signals instead of assuming writes/reads always succeed.
- `COM_Parse*` helpers mutate global token/argv state and are not goroutine-safe.
- Angle serialization here is a low-level compatibility helper and may not match every higher-level protocol path exactly.
