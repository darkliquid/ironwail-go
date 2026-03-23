# Responsibility

## Purpose

`common` owns the low-level shared primitives that remain after the Go port split the old monolithic C `common.c` responsibilities into dedicated packages.

## Owns

- Shared message-buffer primitives and basic angle/string/integer serialization.
- Shared tokenization, newline parsing, command-line argv helpers, and path/hash utilities.
- Small structural helpers such as intrusive links and bit arrays.
- Endian-aware binary read/write helpers in the `binary` subpackage.

## Does not own

- Filesystem, command system, console, or cvar orchestration that lived in C `common.c` but now live in dedicated Go packages.
- Protocol-flag-aware server/client message codecs layered above these primitives.
