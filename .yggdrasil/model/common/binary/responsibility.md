# Responsibility

## Purpose

`common/binary` owns tiny endian-aware scalar conversion and stream I/O helpers used by binary file and protocol decoding paths.

## Owns

- little-endian scalar conversion helpers for slices and readers/writers
- big-endian scalar conversion helpers for slices
- float/int read/write helpers over `io.Reader` / `io.Writer`

## Does not own

- Higher-level file-format parsing.
- Buffer sizing, overflow tracking, or token parsing.
