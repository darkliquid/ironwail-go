# Responsibility

## Purpose

`renderer/effects` owns backend-neutral helper logic for alias skin/state handling, alpha modes, client-side effect derivation, dynamic lights, particles, entity typing, and player color handling.

## Owns

- alias skin/state helpers
- alpha mode interpretation
- renderer-side client effect derivation
- dynamic light and particle helper logic
- player color handling and common entity-type categorization

## Does not own

- concrete OpenGL/GoGPU draw submission
- shared world geometry preparation
