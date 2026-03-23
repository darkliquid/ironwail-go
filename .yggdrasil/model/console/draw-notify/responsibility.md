# Responsibility

## Purpose

`console/draw-notify` owns the backend-neutral visual projection of console state: the full drop-down console, notify overlay, prompt clipping, cursor blink, and fade behavior.

## Owns

- `DrawContext` abstraction
- full-console and notify draw entry points
- prompt clipping and cursor blink helpers
- notify fade/alpha logic and centered-notify behavior
- scaled background-pic caching for the full console

## Does not own

- Console text storage or history mutation.
- Actual renderer backend implementations.
- Key/input routing decisions.
