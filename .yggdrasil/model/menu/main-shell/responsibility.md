# Responsibility

## Purpose

`menu/main-shell` owns the top-level shell flows that hang directly off the main menu: main-page navigation, single-player and multiplayer entry pages, help pages, player setup, quit/confirmation prompts, and the mods browser.

## Owns

- Main menu cursor movement and item selection.
- Single-player and multiplayer entry-page navigation.
- Help-page cycling.
- Setup menu field editing, color wrapping, and setup-command application.
- Quit/confirmation behavior.
- Main-menu split art handling for the optional Mods entry.
- Mods list selection and `game` command queueing.

## Does not own

- Load/save/join/host-game page behavior.
- Options/video/audio/controls pages.
- Shared drawing primitives.
