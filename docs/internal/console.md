# Package `console`

## Purpose
The `console` package implements the engine's text-based interface. It maintains a scrollable buffer of messages printed by the engine and gameplay code, handles user input for commands, and manages "notify" lines displayed briefly during gameplay. It serves as the primary diagnostic and configuration tool for both players and developers.

## Key Types & Interfaces

- **`Console`**: The central state struct. It manages a large byte slice as a ring buffer for scrollback, an input line for current typing, and notification timestamps.
- **`globalConsole`**: A singleton instance of `Console` used by most of the engine.
- **`DefaultTextSize`**: A constant (1MB) defining the default scrollback buffer capacity.

## Core Workflow

1.  **Printing**: Code calls `Printf`, `DPrintf` (developer-only), or `Warning`. These funnel into `printRaw`, which handles:
    *   **Word Wrapping**: Breaking lines to fit the current `lineWidth`.
    *   **Special Characters**: Handling newlines, carriage returns, and Quake's "bronze" text prefix (0x01/0x02).
    *   **Notifications**: Recording the timestamp of the new line so it can be shown on-screen.
2.  **Scrolling**: The `backScroll` field tracks how far up the user has scrolled. Functions like `Scroll` and `SetBackScroll` manipulate this state.
3.  **Resizing**: When the window dimensions change, `Resize` re-calculates the buffer layout. This involves re-mapping the flat ring buffer contents to a new line width while preserving the most recent history.
4.  **Logging**: If enabled via `condebug`, all console output is mirrored to a file on disk.

## Integration

- **`internal/cvar`**: The console uses the cvar system to look up flags like `developer` and `con_logcenterprint`.
- **`internal/draw`**: Higher-level rendering packages read from the `Console` buffer and use characters from `draw.GetConcharsData()` to render the text to the screen.
- **`internal/cmdsys`**: When the user presses Enter, the console's `inputLine` is typically passed to the command system for execution (this interaction usually happens in the `host` or `input` packages).
- **Filesystem**: `Dump` and `EnableDebugLog` interact with the OS filesystem to save logs and buffer captures.

## Learning Tips

- **Ring Buffer Implementation**: Look at how `c.current` and `c.totalLines` are used with modular arithmetic (`%`) to manage the flat `text` slice.
- **Thread Safety**: Notice the use of `sync.RWMutex`. Rendering often happens on a different goroutine than engine updates, so thread-safe access to the text buffer is critical.
- **High-Bit Encoding**: Quake text often sets the 7th bit (0x80) to indicate the "bronze" color variant. Look at `printRaw` and `Dump` to see how this is applied and stripped.
- **Resize Logic**: The `Resize` method is a great example of handling data layout transitions. It's more complex than a simple slice resize because it must preserve line-based readability.

## Tests

### `completion_test.go`

**`TestExtractPartialSingleToken`** — Asserts `extractPartial("tog") == "tog"`. The token extraction underpins tab-completion; if it returns the wrong partial, no completion occurs.

**`TestTabCompleterCompletesCurrentToken`** — With a `CommandProvider` that returns `["toggleconsole"]` for partial `"tog"`, verifies `Complete("tog",true)` returns `"toggleconsole"` with match label `"toggleconsole (command)"`. Validates basic command tab-completion, the primary UX feature of the console. Injects a provider closure and calls `Complete`.

**`TestTabCompleterIncludesAliases`** — Same as above but with an `AliasProvider` for `"qa"` → `["qalias"]`. Expects match label `"qalias (alias)"`. User-defined aliases must be as discoverable as built-in commands. Uses `SetAliasProvider` injection.

**`TestTabCompleterCompletesMapArgument`** — `Complete("map e1", true)` should call the file provider with pattern `"maps/*.bsp"` and return `"map e1m1"` with match `"e1m1 (map)"`. The `map` command takes a filename; argument-aware completion is required for discoverability. Injects `FileProvider`, captures the pattern argument.

**`TestTabCompleterCompletesExecArgument`** — `Complete("exec auto", true)` with provider returning `["autoexec.cfg"]` should give `"exec autoexec.cfg"` and match `"autoexec.cfg (config)"`. The `exec` command loads a config file; completing `.cfg` filenames reduces user errors. Injects `FileProvider` with pattern `"*.cfg"`.

**`TestTabCompleterFirstTabUsesCommonPrefix`** — When two matches exist (`"toggleconsole"`, `"togglemenu"`), the first `Complete` call returns the longest common prefix `"toggle"`, prints the match list, and reports both matches. Standard shell-style completion: first Tab narrows to the common prefix so the user can see options without overwriting their input. Injects a `PrintFunc` and verifies the builder is non-empty after the call.

### `draw_test.go`

**`TestConsoleDrawRendersConsoleLinesAndPrompt`** — Logs two lines via `Printf`, appends `"sv"` to the input, calls `Draw(mock, 80, 80, true, nil)`, and verifies a background fill was drawn and both lines appear in the character output, with the prompt row starting with `"]sv"`. The console's primary function is to display scrollback and the current input; if any are missing, the console is unusable. Uses `mockRenderContext` that records all `DrawCharacter` calls.

**`TestConsoleDrawRendersBlinkCursorAndClipsPrompt`** — Sets the input line to `"longcommand"` (11 chars), clocks time to the second blinking phase, and verifies the cursor glyph appears at X=64, the prompt starts with `"]"`, and only the last visible characters of the clipped text are `"om"`. The cursor must blink at the correct position; long input lines must be clipped to fit within the console width. Fixes `c.now` to a deterministic time and checks exact X coordinates.

**`TestConsoleDrawNotifyHonorsNotifyLifetime`** — Writes two lines, sets the first line's notify timestamp to 1 second past expiry and the second to now. Draws in notify mode and verifies only the second line appears. On-screen messages during gameplay must disappear after `con_notifytime` seconds. Directly sets `c.notifyTimes` entries to past/present times.

**`TestConsoleDrawNotifyUsesCVarLifetime`** — Sets `con_notifytime=1`, logs a line, sets its notify timestamp to 1.5 seconds ago, draws, and asserts it is not rendered. Ensures the TTL is read from the cvar at draw time, not hardcoded.

**`TestConsoleDrawNotifyCanCenterLines`** — Sets `con_notifycenter=1`, logs `"abc"`, and verifies the first character is drawn at X=28 (centered in an 80-pixel-wide area). Tournament HUDs often center notify messages; wrong centering looks unprofessional.

**`TestConsoleDrawNotifyFadeStipplesLateLines`** — Enables `con_notifyfade` with `con_notifyfadetime=1`, sets a line's timestamp past the TTL + 750 ms into the fade window. Verifies some (but not all) of the 6 characters of `"fading"` are rendered. The fade effect must progressively hide characters; all-or-nothing would break the visual fade.

**`TestConsoleDrawResizesWithMargins`** — Draws on an 80-pixel-wide canvas with the 8-pixel default text size and checks that `LineWidth()` returns 8 (= 10 chars wide − 2 margin chars). Margins prevent text from touching the screen edge; wrong margin calculation would clip or overflow text.

**`TestConsoleDrawUsesBackgroundPicWhenProvided`** — Passes a 320×200 QPic background; verifies exactly one background pic is drawn (not a solid fill), scaled to 640×240, and that text still appears. The `conback.lmp` background image must replace the solid color when available.

**`TestConsoleDrawRendersTitleString`** — Sets `c.Title = "Test 1.0"`, draws on a 320×200 canvas, and asserts the title string appears in the rendered character rows. Ironwail displays a version string in the console overlay; this verifies it is actually rendered.

**`TestNotifyBoxPrintsFramedMessage`** — Calls `NotifyBox("Sound init failed.\n")` and scans the console scrollback for both the message and the "Press a key." prompt. `NotifyBox` is used for critical startup errors; the user must see both the message and instructions to dismiss it.

### `console_test.go`

**`TestConsoleInputHistory`** — Types and commits two commands (`"test"`, `"next"`), then navigates with `PreviousHistory`/`NextHistory` and asserts the correct order and draft restoration. The console history ring is fundamental UX; wrong navigation order or off-by-one indexing would confuse users.

**`TestConsoleBackspaceInput`** — Types `'g'` and `'o'`, calls `BackspaceInput()`, expects `InputLine()=="g"`. Basic text editing; without working backspace, users cannot correct mistakes.

**`TestConsoleWordWrapAtWordBoundary`** — Resizes the console to 10 chars wide, prints `"12345 67890"`, and checks the two resulting lines. The console must wrap at word boundaries (space), not in the middle of tokens.

**`TestQuakeBarSuppressesNewlineAtFullWidth`** — `QuakeBar(8)` on an 8-char-wide console must NOT end with `"\n"`. A trailing newline would push the bar to the next line and create a blank line.

**`TestDPrintf2RequiresDeveloperLevel2`** — With `developer=1`, `DPrintf2("hidden\n")` must not appear. With `developer=2`, it must appear. Level-2 debug printing is for verbose diagnostics; printing at level 1 would flood the console.

**`TestLogCenterPrintDedupesAndGatesByGameType`** — With `con_logcenterprint=1` (singleplayer only), a multiplayer-mode centerprint is suppressed. With `con_logcenterprint=2` (always), it appears. A duplicate message does not advance the line counter. Prevents scrollback spam from repeated centerprints.

**`TestConsoleCursorEditingAndHistoryRestore`** — After `SetInputLine("hello world")`, `DeleteWordLeft()` must leave `"hello "`. Then verifies that navigating to history and back restores the draft `"draft"`. Ctrl+W word deletion and draft-restore are standard console editing features.
