# Package `cvar`

## Purpose
The `cvar` package manages Quake's console variables. Cvars are the primary mechanism for persistent, named configuration state (e.g., `volume`, `sensitivity`, `sv_gravity`). Unlike the command system, which handles actions, the cvar system handles engine settings that must be accessible across multiple subsystems.

## Key Types & Interfaces

- **`CVar`**: Represents a single variable. It stores its value simultaneously as a `String`, `Float64`, and `Int`. This "triple storage" pattern avoids repeated parsing when numeric values are needed in performance-critical loops.
- **`CVarSystem`**: A thread-safe registry that manages a collection of `CVar` objects. It handles registration, lookups, and value updates.
- **`CVarFlags`**: A bitmask controlling cvar behavior (e.g., `FlagArchive` for saving to config, `FlagROM` for read-only, `FlagServerInfo` for network replication).

## Core Workflow

1.  **Registration**: Subsystems call `Register` during initialization to define their settings. If a cvar was already set (e.g., from a config file), the registered cvar adopts that value but applies the newly defined flags and description.
2.  **Update**: Calling `Set`, `SetFloat`, etc., updates the cvar's canonical string value and re-parses it into numeric fields.
3.  **Callbacks**: When a cvar's value changes, its `Callback` function is invoked. This allows systems to react instantly (e.g., the audio system can update its mixer gain as soon as `volume` is changed).
4.  **Serialization**: The `ArchiveVars` method collects all cvars marked with `FlagArchive` and formats them as commands (e.g., `sensitivity "5"`) to be written to the `config.cfg` file.

## Integration

- **Console**: Cvars are exposed to the player via the console. If a command is entered that doesn't match a known action, the engine checks if it's a cvar and performs a `Set`.
- **Command System**: Powers tab-completion for cvar names and values.
- **Host**: Manages the persistence of cvars by calling `ArchiveVars` during shutdown.
- **Network**: Uses `FlagServerInfo` and `FlagUserInfo` to synchronize settings between clients and servers.

## Learning Tips

- **Triple Storage**: Understand why `CVar` stores `String`, `Float`, and `Int` fields. It's a classic example of trading a small amount of memory for significant CPU savings in tight loops.
- **Flag Mechanics**: Study the different flags. `FlagLatched`, for instance, is a subtle but important behavior where a change is accepted but shouldn't take effect until a map restart or video reset.
- **Case Insensitivity**: Note that all cvar lookups and registrations are case-insensitive, ensuring that `Sensitivity` and `sensitivity` refer to the same setting.
- **Thread Safety**: Observe how `CVarSystem` uses `sync.RWMutex`. This allows the renderer to read cvars (like `fov` or `gamma`) simultaneously without blocking, while still allowing the main thread to update them.

## Tests

**`TestSetCallbackCanReadUpdatedValue`** — Registers a cvar with a `Callback`, calls `Set` from a goroutine, and asserts the callback receives the new value immediately and can read it back via the system. Engine subsystems (e.g., the audio mixer) use callbacks to react to volume changes; the callback must see the committed value, not the old one. Uses a buffered channel to capture callback errors across goroutines.

**`TestFlagROM`** — Registers a cvar with `FlagROM` = 42, calls `Set("100")`, and asserts the value remains 42. Read-only cvars (version strings, hardware caps) must be immune to user `set` commands.

**`TestLockedCvarRejectsSet`** — Locks a cvar, confirms `Set` is rejected, unlocks it, confirms `Set` then succeeds. The server locks networking-related cvars while connected so clients cannot change them mid-session.

**`TestAutoCvarCallback`** — Sets `FlagAutoCvar` on `sv_gravity`, registers `AutoCvarChanged`, calls `Set("400")`, and asserts the hook fires with `"400"`. Also verifies a non-autocvar doesn't trigger the hook. The QC VM monitors autocvars to sync server globals; without the callback, QC physics globals would never update.

**`TestMarkAutoCvarEnablesAutoCallback`** — Registers a cvar without `FlagAutoCvar`, then calls `MarkAutoCvar`, sets it, and asserts the hook fires. Some cvars are declared normal but later marked autocvar at runtime; `MarkAutoCvar` must retroactively enable the callback.

**`TestLockedAutoCvarRejectsSetWithoutAutoCallback`** — Locks an autocvar, calls `Set`, and asserts both the value and the callback are unaffected. A locked autocvar must also suppress the autocvar hook, not just the value write.

**`TestLatchedAutoCvarSetSuppressesAutoCallback`** — A `FlagLatched|FlagAutoCvar` cvar stores the new value but does NOT fire `AutoCvarChanged` (the change takes effect next map). Latched cvars like `sv_maxplayers` only apply on map load; triggering the autocvar hook immediately would apply a half-initialized change.
