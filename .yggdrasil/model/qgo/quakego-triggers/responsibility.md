# Responsibility

`qgo/quakego-triggers` owns the trigger-family gameplay slice implemented in `pkg/qgo/quakego/triggers.go`, with emphasis on trigger setup, touch dispatch, and one activation path.

This node is responsible for preserving QuakeC-compatible top-level trigger entry points (`trigger_multiple`, `trigger_once`, `multi_touch`, `multi_use`, `multi_trigger`) while allowing bounded receiver-style grouping via a local trigger adapter. It is not responsible for unrelated gameplay families or broad quakego package-wide prototype organization.
