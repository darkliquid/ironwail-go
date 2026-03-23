# Responsibility

## Purpose

`client/input` owns local client-side input state, view-angle manipulation, usercmd preparation, and prediction support used to reduce perceived latency.

## Owns

- Quake-style button state transitions and key-state sampling.
- Pitch drift and view-angle adjustment behavior.
- Input-derived movement intent and related client command fields.
- Local prediction replay and prediction-error smoothing helpers.

## Does not own

- Host scheduling of send/read phases.
- Authoritative world simulation or collision.
- Protocol parsing of server updates.
