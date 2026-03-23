# Interface

## Main consumers

- `client`, which uses `SizeBuf` for parsing and writing protocol-adjacent message data.
- `server/savegame`, which uses the text tokenization/newline parsing helpers.
- file/model/BSP loaders and other low-level code that need endian helpers or path utilities.

## Main surface

- `SizeBuf` plus its read/write helpers
- `COM_Parse*`, `COM_InitArgv`, and command-line lookup helpers
- path and hash helpers such as `COM_FileBase`, `COM_StripExtension`, and `COM_Hash*`
- `Link`, `BitArray`, and `binary` endian helpers

## Contracts

- `SizeBuf` read/write semantics and overflow behavior are foundational low-level contracts.
- `COM_Parse*` helpers intentionally expose stateful, C-style parsing behavior through global token/argv state.
- This package provides primitives; higher-level policy and protocol interpretation belong to consuming subsystems.
