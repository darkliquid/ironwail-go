# Internals

## Logic

### Input state

The input path tracks Quake-style button transitions and computes fractional key-state contributions for the current frame.

### View and drift

Pitch drift and angle adjustment logic preserve classic Quake-style view behavior, including special handling for cutscenes and look-spring behavior.

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
