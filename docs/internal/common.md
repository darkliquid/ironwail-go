# Package `common`

## Purpose
The `common` package provides foundational, low-level utilities shared across the entire Ironwail engine. It is responsible for primitive data structures and I/O helpers that underpin Quake's networking, file handling, and entity management. By centralizing these primitives, the engine maintains consistency in how it handles binary data and shared system state.

## Key Types & Interfaces

- **`SizeBuf`**: A core abstraction for message serialization. It manages a byte slice with overflow detection and tracking for read/write positions. It is the primary structure used for Quake's network protocol.
- **`Link`**: An intrusive doubly-linked list node. Instead of using external list containers, Quake entities embed a `Link` to be part of various spatial or management lists.
- **`BitArray`**: A memory-efficient bit-level storage mechanism, primarily used for visibility data like PVS (Potentially Visible Set) and PAS (Potentially Audible Set).
- **`CPE_Mode`**: An enumeration controlling how token overflow is handled during parsing (`CPE_NOTRUNC` vs. `CPE_ALLOWTRUNC`).

## Core Workflow

1.  **Binary I/O**: Systems use `SizeBuf` to compose or decompose binary messages. Little-endian helpers (`WriteShort`, `ReadLong`, etc.) ensure compatibility with Quake's data formats (BSP, MDL, network protocol).
2.  **Tokenization**: The `COM_Parse` and `COM_ParseEx` functions act as the engine's lexer. They process text-based data (configs, entity definitions, console commands) by skipping whitespace and comments and extracting tokens or quoted strings.
3.  **Command-Line Handling**: During startup, `COM_InitArgv` captures process arguments. Other systems then use `COM_CheckParm` to detect flags (like `-dedicated` or `-game`) that influence engine behavior.
4.  **Path Manipulation**: A suite of `COM_` prefixed functions (e.g., `COM_StripExtension`, `COM_SkipPath`) provide consistent handling of Quake's virtual filesystem paths.

## Integration

- **Networking**: Relies heavily on `SizeBuf` for serializing and parsing client-server messages.
- **Filesystem & Loaders**: Use `Read*`/`Write*` helpers and path utilities to decode map (BSP) and model (MDL/SPR) files.
- **Host & Console**: Use the tokenizer and command-line helpers for processing user input and configuration files.
- **Server/Game Logic**: Use `Link` for managing active entity lists and `BitArray` for visibility culling.

## Learning Tips

- **Intrusive Lists**: Examine the `Link` type. Understanding why Quake uses intrusive lists (O(1) insertion/removal without allocations) is key to understanding its entity management performance.
- **Legacy Tokenizer**: Look at `COM_ParseEx`. It retains a C-style `goto skipwhite` pattern to exactly mirror the original Quake lexer's behavior, which is critical for compatibility with complex configuration files and mods.
- **Endianness**: Note how `SizeBuf` explicitly uses `binary.LittleEndian`. Quake is strictly little-endian, regardless of the host architecture.
- **High-Bit Colors**: Some string handling in Quake (and mirrored in the engine) uses the 8th bit of characters to indicate special rendering (like "bronze" text). This is why you'll see masks like `& 0x7F` or `| 128` in various parts of the codebase.

## Tests

**`TestCOM_CheckParm`** — Verifies `COM_CheckParm` returns the correct 0-based index for flags present in `argv` and returns 0 for missing flags. This is critical for the engine discovering command-line flags like `-game rogue`; an off-by-one in the index would cause the engine to misread the argument that follows the flag. Calls `COM_InitArgv` with a known arg list then asserts specific positions.

**`TestCOM_Parse`** — Verifies the token parser handles whitespace, quoted strings, `//` line comments, `/* */` block comments, and single-character punctuation tokens. `COM_Parse` is used for reading `.cfg` files and entity data in BSP lumps; wrong tokenization corrupts config loading. Feeds a single compound string through repeated `COM_Parse` calls.

**`TestPathUtils`** — Exercises `COM_FileBase`, `COM_FileGetExtension`, `COM_StripExtension`, and `COM_AddExtension`. Consistent path handling is essential when loading assets (e.g., stripping `.bsp` to find the `.lit` file, or adding `.wav` to a sound name). Four direct comparisons including the idempotent case for `COM_AddExtension`.

**`TestHash`** — Confirms `COM_HashString` and `COM_HashBlock` produce identical, non-zero results for the same data. Both functions are used for caching (textures, models); a mismatch between the two overloads would cause cache misses. Calls both with `"hello world"`, checks equality and non-zero.

**`TestParseNewline`** — Verifies `COM_ParseIntNewline`, `COM_ParseFloatNewline`, and `COM_ParseStringNewline` each consume exactly one line. Demo and save files use line-delimited numeric and string fields; wrong parsing corrupts the read cursor. Pipes a multi-line string through all three in sequence, checking both the returned value and the remaining string.

**`TestSizeBufWriteReadAngle`** — Round-trips 8-bit angle encoding through `WriteAngle`/`ReadAngle` for 0°, 90°, 180°, 270°, −45°, and the 400° wraparound case. Network messages compress angles to 1 byte; if the decode doesn't exactly invert the encode (including wraparound), player orientation will drift on the network. Checks within 2° tolerance (the 8-bit resolution).

**`TestSizeBufWriteReadAngle16`** — Same round-trip for 16-bit angle encoding (`WriteAngle16`/`ReadAngle16`). Higher-precision angles are needed for precise aiming; validates the 16-bit path doesn't introduce quantization bugs. Same pattern with 0.01° tolerance.

**`TestSizeBufAnglePrecision`** — Asserts that 16-bit encoding always produces less absolute error than 8-bit for the same input angle. This is the core quality guarantee of the 16-bit encoding; if it were no better than 8-bit, the extra byte would be wasted. Computes absolute errors for both encodings and asserts `err16 < err8`, plus hard bounds.
