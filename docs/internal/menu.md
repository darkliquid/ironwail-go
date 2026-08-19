# Package menu

## Purpose

The `menu` package implements the Quake in-game menu system. It manages the menu's finite state machine (navigating between screens like Options, Multiplayer, and Video), handles UI-specific input, and orchestrates the drawing of menu elements.

## Key Types & Interfaces

- **`Manager`**: The central object that tracks the current `MenuState`, cursor positions, and internal buffers (like the server address field).
- **`MenuState`**: An enum representing each distinct menu page (e.g., `MenuMain`, `MenuOptions`, `MenuVideo`).
- **`DrawManager` Interface**: An abstraction used to retrieve graphics (`QPic`) without tying the menu package directly to the filesystem or a specific renderer.

## Core Workflow

1.  **Activation**: The menu is toggled via `Manager.ToggleMenu()`, which switches the input system to `KeyMenu` mode.
2.  **Navigation**: Keyboard and gamepad events are passed to `Manager.M_Key()`. This method routes the key to a page-specific handler (e.g., `mainKey`, `optionsKey`), which updates the cursor or transitions to a new `MenuState`.
3.  **Input**: Character events for text fields (like the Player Name) are handled via `Manager.M_Char()`.
4.  **Rendering**: Each frame, the host calls `Manager.M_Draw()`. The manager uses a `renderer.RenderContext` to draw pictures and text on a virtual 320×200 grid.

## Integration

- **`internal/input`**: The menu package relies on the input system to receive events and to set the `KeyDest`.
- **`internal/renderer`**: All menu drawing is performed through the `RenderContext` provided by the renderer.
- **`internal/cvar`**: Many menu items (sliders, toggles) directly read and write console variables.
- **`internal/mods`**: The Mods menu uses the `downloader` from this package to browse and install addons.

## Learning Tips

- **Gamepad Mapping**: Look at `normalizeMenuKey` to see how gamepad buttons (like the 'A' button) are mapped to standard keyboard equivalents (like 'Enter') for the menu.
- **Virtual Resolution**: Observe how all coordinates in `menu_draw.go` assume a 320x200 screen, regardless of the actual window size.
- **Hierarchical Navigation**: Notice how `MenuQuit` or `MenuMain` often store a `prev` state to allow the Escape key to function as a "Back" button.

## Tests

This section is a curated summary of representative menu tests, not an exhaustive inventory of every `*_test.go` file or `Test...` case in `internal/menu`. Use it as a guide to notable behaviors covered by the package tests.
### Controls & Options (`manager_controls_test.go`)

**`TestHelpNavigation`** — Right-arrow advances pages; wrapping from last page returns to page 0; Escape returns to main menu. Verifies the help screen's pagination and exit.

**`TestOptionsNavigationAndAction`** — Enter on each options item (Controls, Video, Audio, Vsync toggle, Back) navigates to the correct sub-menu or performs the expected cvar toggle.

**`TestControlsMenuRebindingAndClearing`** — Pressing Enter enters rebinding mode; pressing `'i'` binds `+forward` to `i`, clears old bindings for `w` and `UPARROW`; pressing LeftArrow clears the new binding. Validates the full bind/clear lifecycle.

**`TestControlsMenuCancelRebinding`** — Pressing Escape while rebinding cancels without changing the existing binding.

**`TestControlsMenuCanBindBackquote`** — Backquote (`` ` ``) can be bound to `toggleconsole` via the controls menu. Historically problematic because the backquote opens the console in some engines.

**`TestControlsMenuCursorWrapWithExpandedBindings`** — Down from the last item wraps to the first; up from the first wraps to the last. Standard list navigation invariant.

**`TestControlsMenuLabelForNewCommand`** — `controlBindingLabel` returns `"UNBOUND"` with no binding, and the key name with one.

**`TestControlsMenuRebindAndClearNewCommand`** — Full rebind + clear cycle for a dynamically-added control item.

**`TestControlsMenuAdjustsLiveControlCvars`** — Right-arrow on Mouse Speed increments `sensitivity` by 0.5; Enter on Invert Mouse negates `m_pitch`; Enter on Always Run toggles `cl_alwaysrun`; LeftArrow on Freelook toggles `freelook`; Backspace on a settings row returns to Options. Validates that every UI control writes the expected cvar.

**`TestVideoMenuAdjustmentsWriteCvars`** — Each video menu item (resolution, fullscreen, maxfps, gamma, viewmodel, showfps, showspeed, showtime, back) writes the expected cvar change.

**`TestAudioMenuVolumeAdjustment`** — Right/left arrows adjust `s_volume` by 0.1; Back returns to Options.

### Multiplayer & Setup (`manager_multi_setup_test.go`)

**`TestMultiPlayerNavigation`** — Multiplayer menu items 0/1/2 navigate to Join/Host/Setup sub-menus.

**`TestJoinGameMenuEditingAndConnectCommand`** — Backspace edits the address, typing appends chars, navigating to Connect and pressing Enter hides the menu and queues `connect "local:2600"\n`.

**`TestHostGameMenuEditingAndCommands`** — Full host-game form: adjusts all fields, enters map name, presses Start, verifies the exact sequence of 10 console commands. Tests that every host-game setting translates to the right command.

**`TestHostGameStartQueuesListenZeroForSinglePlayer`** — With `maxplayers=1`, start queues `listen 0\n` (no listening socket needed for solo play).

**`TestJoinGameMenuConnectsSelectedServerResult`** — Selecting a discovered server from the list queues `connect "10.0.0.3:26000"\n` and updates `joinAddress`.

**`TestHostGameMenuSyncsFromLiveNetgameCVars`** — Entering the host menu reads live `coop`, `deathmatch`, `teamplay`, `skill` cvars and populates the UI fields. Ensures the menu reflects actual server state.

**`TestHostGameMenuMaxPlayersClampsAtBounds`** — Decrement at min stays at min; increment at max stays at max.

**`TestSetupMenuLoadsCurrentHostnameNameAndColor`** — Entering setup reads `sv_hostname`, player name, and top/bottom colours from cvars.

**`TestSetupMenuHostnameNameColorAndAccept`** — Full setup edit cycle: clears fields, types new values, accepts, verifies `name "Ranger"\n` and `color 1 1\n` commands and cvar update.

**`TestSetupMenuBackspaceOnColorRowDoesNotExit`** — Backspace on a non-text row stays in the setup menu.

**`TestSetupMenuEscapesBackslashesAndQuotesInName`** — Player name containing `\` and `"` must be properly shell-escaped in the `name` command.

**`TestDrawSetupUsesTextBoxesAndTranslatedPlayerArt`** — Drawing the setup menu uses the bigbox pic and a colour-translated copy of `menuplyr.lmp`, not DrawFill colour swatches.

**`TestTranslateSetupPlayerPicMapsTopAndBottomRanges`** — `translateSetupPlayerPic` maps palette range 16–31 (top colour band) and 96–111 (bottom colour band) to the selected colours without mutating the source.

**`TestLoadAndSaveCursorWrap`** — Up from slot 0 wraps to `maxSaveGames-1`; down from last slot wraps to 0.

**`TestMultiPlayerAndOptionsEscBack`** — Escape from Multiplayer returns to Main; Backspace from Options returns to Main.

### Mouse & Controller Input (`manager_input_test.go`)

**`TestMouseBindingsForActivationAndBack`** — Mouse1 activates the selected item; Mouse2 goes back; Mouse1 on the quit dialog queues `quit\n`. Validates the three most fundamental mouse interactions in any menu.

**`TestControllerButtonsMapToMenuAcceptAndBack`** — A-button acts as Enter; B-button acts as Escape. Core gamepad UX for menu navigation.

**`TestControllerDpadMapsToArrowNavigation`** — D-pad down/up moves the cursor; the alt-layer variant keys (KDpadUpAlt) behave identically to their primary counterparts. Verifies both primary and alt D-pad layers work.

**`TestControllerStartAndBackMapInOptionsMenu`** — Start/Back buttons navigate the options menu as expected (additional coverage of the gamepad button mapping layer).

## gogpu/ui path (`ui_backend 1`)

When `ui_backend=1`, the menu presentation moves to the gogpu/ui widget tree
in `internal/quakeui/menu` while this package stays the source of truth:

- The legacy `Manager` state machine (state, cursors, text buffers, host
  settings, mods, save slots) is unchanged and read via the exported
  accessors in `accessors.go` (`State`, `CursorFor`, `TextBuffer`,
  `HostSettings`, `Mods`, `CurrentMod`, `SaveSlots`).
- `internal/quakeui/menu.MenuRoot` reads those accessors each frame, exposes
  the active page's rows (label + value), and routes key/char events back
  through `M_Key`/`M_Char` so navigation and actions are shared verbatim.
- The `draw*` methods here (`M_Draw`) remain the parity oracle for path 0 and
  the layout-constant reference for the widget row model.
