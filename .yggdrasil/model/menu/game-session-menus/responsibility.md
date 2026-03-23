# Responsibility

## Purpose

`menu/game-session-menus` owns the menu pages that interact with save slots, network join flow, LAN server search, and local host-game setup.

## Owns

- Load and Save menu navigation/command issuance.
- Join Game address editing, search-LAN behavior, result selection, and connect command application.
- Host Game settings editing and startup command sequencing.
- Save-slot label refresh and default labeling.
- Server-browser result display and row selection.

## Does not own

- The higher-level single-player/multiplayer entry pages that route into these screens.
- Generic options/settings pages.
- The actual networking, save IO, or game launch implementation behind the queued commands/provider callbacks.
