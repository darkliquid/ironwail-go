# Responsibility

## Purpose

`console/input-history` owns the editable console prompt line and the bounded command-history behavior layered on top of it.

## Owns

- current input-line getters/setters/editing helpers
- commit behavior that records history and clears the prompt
- previous/next history traversal
- package-level wrappers for input/history operations

## Does not own

- Command execution of committed text.
- Tab-completion session logic.
- Scrollback rendering or notify behavior.
