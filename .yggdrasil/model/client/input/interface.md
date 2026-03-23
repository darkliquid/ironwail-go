# Interface

## Main consumers

- `internal/host`, through the client send/update path.
- `client/runtime`, which stores the state mutated by input and prediction helpers.

## Main API

Observed surfaces:
- key/button transitions
- angle adjustment helpers
- pitch drift helpers
- prediction helpers such as `PredictPlayers(...)`

## Contracts

- Input helpers mutate shared `Client` state and therefore depend on correct host-side ordering with parse/relink/render phases.
- Prediction is explicitly a client-side support mechanism rather than an authoritative source of render truth.
