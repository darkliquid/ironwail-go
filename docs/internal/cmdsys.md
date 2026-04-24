# CmdSys Package

## Purpose
The `cmdsys` package implements the Quake engine's console command system. It acts as the primary "glue" for user interaction, allowing actions to be triggered via text. It handles the registration of command handlers, management of command aliases, and the buffering and execution of command strings from various sources (console, config files, network).

## Key Types & Interfaces
- **`CmdSystem`**: The central state manager that holds the registry of commands, defined aliases, and the buffer of pending text.
- **`Command`**: Represents a registered action, including its name, description, and the `CommandFunc` callback.
- **`CommandFunc`**: A function type `func(args []string)` that serves as the handler for a command.
- **`CommandSource`**: An enum (`SrcCommand`, `SrcClient`, `SrcServer`) used to track the origin of a command string for security or routing purposes.

## Core Workflow
1. **Registration**: Subsystems (like the renderer or server) register their "verbs" using `AddCommand`.
2. **Buffering**: Command text is added to the system via `AddText` (queue for later) or `InsertText` (run immediately on the next execution).
3. **Execution**: The `Execute()` method is called (usually once per frame). It drains the buffer and processes each line.
4. **Tokenization**: Each line is split into tokens (respecting quotes and semicolons).
5. **Dispatch**: The system looks up the first token:
   - If it matches a **Command**, its `CommandFunc` is called with the arguments.
   - If it matches an **Alias**, the alias is expanded into the buffer.
   - If it matches a **Cvar**, it is treated as a set operation (e.g., `volume 0.5`).
   - Unrecognized commands can be forwarded via `ForwardFunc` (e.g., sending a command to the server).

## Integration
- **Host**: Owns the `CmdSystem` and calls `Execute()` every frame.
- **Console/Menu**: Submit user-typed text to the buffer.
- **Cvar System**: Closely coupled; the command system often falls back to the cvar system for unknown tokens.
- **Filesystem**: Used to execute configuration files (`exec config.cfg`) by reading the file and passing its contents to `AddText`.

## Learning Tips
- **The "Wait" Command**: Look at how the `wait` command is implemented in `NewCmdSystem`. It uses a `waitCount` to pause the processing of the command buffer for a frame, which is critical for complex script macros.
- **Source Tracking**: Examine `CommandSource` to see how the engine distinguishes between a local console command and a command "stuffed" into the buffer by a server.
- **Alias Expansion**: See how aliases are recursively expanded into the buffer, enabling players to create shorthand commands.
- **Quake-Style Tokenization**: The logic in `cmd_buffer.go` (and related search functions) replicates Quake's specific rules for handling whitespace, quotes, and semicolons.
