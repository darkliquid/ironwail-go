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
