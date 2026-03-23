# Interface

## Main consumers

- host/runtime and executable wiring that create and drive a renderer instance

## Main surface

- backend-neutral renderer abstractions
- adapter-facing helpers
- runtime fallback behavior when rendering is stubbed

## Contracts

- runtime callers depend on this layer to remain backend-agnostic
- stub behavior is a deliberate supported path for tests/headless operation
