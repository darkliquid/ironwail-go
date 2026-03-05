# Dependency Injection Refactor Plan for Ironwail-Go

## 1. Goal
Eliminate all global and package-level mutable state (`globalCVar`, `globalConsole`, `globalCmd`, `GlobalTabCompleter`) and replace it with a formalized Dependency Injection architecture using the "Explicit Core Context" pattern. This ensures isolated unit testability and completely severs any import cycles caused by cross-package command registration.

## 2. Core Constraints & Guardrails
- **No Global Singletons:** `var globalX` must be removed from `cvar.go`, `console.go`, `cmd.go`, and `completion.go`.
- **No Package Wrappers:** Functions like `console.Printf(...)` must become `core.Console.Printf(...)`.
- **No Import Cycles:** The `engine.Core` container must live in `internal/engine/core.go` (or `internal/core`) and import `cvar`, `cmd`, and `console`. Subsystems like `host`, `server`, and `renderer` will import `engine`, but `engine` MUST NOT import the subsystems.
- **Closure-Based Registration:** Subsystems will register commands via closures (e.g. `core.Cmds.AddCommand("cmd", func() { s.DoSomething() })`) to keep `cmdsys` agnostic of the subsystems.
- **No init() Magic:** Cvars and Commands currently registered in `init()` must be moved to an explicit `Init(core *engine.Core)` method on their respective subsystems.

## 3. Phase-by-Phase Execution Plan

### Phase 1: Clean Up Internal Systems (The "Big Three")
*This phase focuses on removing the globals from the leaf packages. The codebase will temporarily not compile during this phase.*

**1. `internal/console/completion.go`**
- Remove `var GlobalTabCompleter`.
- Remove global wrapper functions (`SetGlobalCommandProvider`, `CompleteInput`, `GetCompletionHint`, `ResetCompletion`).

**2. `internal/console/console.go`**
- Add `completer *TabCompleter` to the `Console` struct.
- Update `NewConsole` to initialize `c.completer = NewTabCompleter()`.
- Remove `var globalConsole`.
- Remove all package-level wrapper functions (`InitGlobal`, `Printf`, `DPrintf`, `Warning`, `Clear`, etc.).

**3. `internal/cvar/cvar.go`**
- Remove `var globalCVar`.
- Remove all package-level wrapper functions (`Register`, `Get`, `Set`, `SetFloat`, etc.).

**4. `internal/cmdsys/cmd.go`**
- Remove `var globalCmd`.
- Remove all package-level wrapper functions (`AddCommand`, `Execute`, `Exists`, etc.).

### Phase 2: Establish the Explicit Core Context
*Create the DI container that will be passed around.*

- Create a new package `internal/engine`.
- Create `internal/engine/core.go`:
```go
package engine

import (
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

type Core struct {
	Cvars   *cvar.CVarSystem
	Cmds    *cmdsys.CmdSystem
	Console *console.Console
}

func NewCore() *Core {
	c := &Core{
		Cvars:   cvar.NewCVarSystem(),
		Cmds:    cmdsys.NewCmdSystem(),
		Console: console.NewConsole(console.DefaultTextSize),
	}
	
	// Cross-wiring base systems
	c.Console.SetCommandProvider(c.Cmds)
	c.Console.SetCVarProvider(c.Cvars)
	
	return c
}
```

### Phase 3: Subsystem Constructor Refactor
*Update the major subsystems to accept the Core Context.*

- **Host (`internal/host/types.go` & `init.go`)**: 
  - Update `NewHost()` to `NewHost(core *engine.Core)`.
  - Store `core *engine.Core` on the `Host` struct.
- **Server (`internal/server/server.go`)**:
  - Update `NewServer()` to `NewServer(core *engine.Core)`.
  - Store `core *engine.Core` on the `Server` struct.
- **QC VM (`internal/qc/vm.go`)**:
  - Update `NewVM()` to `NewVM(core *engine.Core)`.
  - Store `core *engine.Core` on the `VM` struct (needed for `print` builtin).
  - Move `serverBuiltinHooks` into the `VM` struct to avoid globals there too.
- **Renderer (`internal/renderer/renderer.go`)**:
  - Update `NewWithConfig()` to `NewWithConfig(core *engine.Core, cfg Config)`.

### Phase 4: Fix All Call Sites
*This is the heavy lifting. Find every `cvar.Get`, `console.Printf`, and `cmdsys.AddCommand` and replace it with the `core` equivalent.*

- Any package that needs to log or read cvars must either:
  1. Have `core *engine.Core` passed into its constructor.
  2. Have `core` passed into the specific function if it's a utility.
- Ensure that `init()` functions that previously registered cvars (e.g. `cvar.Register(CvarVidWidth...)`) are moved into the `Init()` methods of the respective subsystems.

### Phase 5: Wire the Application (`cmd/ironwailgo/main.go`)
*The Composition Root.*

Update `main.go` to construct the DI graph in the correct order:
1. `core := engine.NewCore()`
2. `core.Console.Printf("Ironwail-Go initializing...")`
3. `gameHost := host.NewHost(core)`
4. `gameServer := server.NewServer(core)`
5. `gameQC := qc.NewVM(core)`
6. `cfg := renderer.ConfigFromCvars(core)` // Update to take core
7. `gameRenderer := renderer.NewWithConfig(core, cfg)`
8. Wire them all together using `host.Init` as before.

### Phase 6: Final Cleanup & Testing
- Remove `svAllowedUserCommands` and `entVarsFieldIndex` globals from `internal/server/` by moving them onto the `Server` struct or initializing them within `Server.Init`.
- Run `go test ./... -race` to ensure no data races remain and all tests compile (tests will need to be updated to use `engine.NewCore()`).

## 4. Acceptance Criteria
- [x] No `var global*` declarations exist for Cvar, Console, CmdSys, or TabCompleter.
114#QX|- [x] No package-level wrapper functions exist in those four files.
115#PS|- [x] The engine builds and runs successfully in both rendering and headless modes.
116#WS|- [x] Tests pass (tests must instantiate their own `engine.Core` to avoid parallel test poisoning).

- [ ] No package-level wrapper functions exist in those four files.
- [ ] The engine builds and runs successfully in both rendering and headless modes.
- [ ] Tests pass (tests must instantiate their own `engine.Core` to avoid parallel test poisoning).

## Final Verification Wave
*(Sisyphus to append task verification links here during execution)*