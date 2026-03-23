# Responsibility

## Purpose

`client/demo` owns demo recording and playback state, frame indexing, and timedemo-related bookkeeping.

## Owns

- `DemoState`
- demo file creation and write flow
- playback reader/source management
- frame indexing and timedemo counters
- disconnect trailers and initial-state snapshot recording

## Does not own

- Host command policy for when demos start or stop.
- Protocol parsing of live server messages outside the demo IO path itself.
