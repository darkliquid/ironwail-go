# Interface

## Main consumers

- `host`, which registers most engine commands and drives buffered config/script execution.
- runtime/input/menu/QC producers that enqueue command text or execute single lines.
- console completion flows that query registered commands and aliases.

## Main surface

- global and instance command-system registration APIs
- buffered and immediate execution entry points
- alias and completion helpers
- source tracking and forwarding hooks
- cvar helper command registration

## Contracts

- Resolution order is command → alias → cvar → forward/unknown.
- Source gating is the security/policy boundary between local, client, and server-injected command text.
- Inserted text must preempt the remaining buffered text during the current execution pass.
