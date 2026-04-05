# Internals

## Logic

The package divides cleanly into a backend-neutral contract layer and externally supplied backend implementations. `System` stores key state, text input, callback hooks, bindings, and the current key destination, while the active backend polls real platform events and translates them into engine key events, text events, mouse deltas, and gamepad state.

## Constraints

- Key numbering and bind-name conversion are persistence-critical because configs and menu rebinding depend on stable string/code mappings.
- Input routing semantics differ by destination (`game`, `menu`, `console`, `message`) and must remain coherent across backends.
- The package must still function before a concrete backend is attached, because startup initializes routing/state ahead of renderer-owned event sources.

## Decisions

### Keep engine-facing input semantics separate from platform event backends

Observed decision:
- The Go port puts stable input vocabulary/routing in one package and hides platform polling behind a backend interface supplied by the executable or renderer.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- Runtime code can reason about one consistent input model while platform- or renderer-specific backends evolve behind a narrower contract.
