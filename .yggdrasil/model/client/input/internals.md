# Internals

## Logic

### Input state

The input path tracks Quake-style button transitions and computes fractional key-state contributions for the current frame.

One-shot button/impulse sampling is split by responsibility:
- `AccumulateCmd` computes movement axes and view-derived command fields.
- `BuildPendingMove` latches attack/jump bits (`state&3`) and impulse at send time, then clears impulse-down flags (`&^=2`) and consumed impulse.
- `BaseMove` / `AccumulateCmd` now hard-gate movement/button/impulse accumulation on `Client.Signon == Signons`, matching C behavior where pre-signon packets carry no movement intent.

This mirrors classic Ironwail/Quake timing where send-time packet construction owns one-shot button consumption.

### View and drift

Pitch drift and angle adjustment logic preserve classic Quake-style view behavior, including special handling for cutscenes and look-spring behavior. Intermission/finale states are treated as cutscenes when `FixAngle` is active so keyboard look/turn input does not continue rotating the camera after the server has forced an intermission angle.

### Prediction

Prediction replays buffered commands from the last known server base state, tracks prediction error, and smooths that error over time. The current implementation is explicitly simplified and collisionless.

## Constraints

- Prediction is only meaningful in active client state.
- Prediction should not override server-driven render truth; it serves as fallback/support behavior.
- Input and prediction are tightly coupled to the shared `Client` runtime state.

## Decisions

### Simplified local prediction

Observed decision:
- The current prediction path is intentionally simplified and collisionless.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- prediction can reduce perceived lag without yet reproducing full authoritative movement behavior
