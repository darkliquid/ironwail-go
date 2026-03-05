# Ironwail-Go Final Cleanup & Wiring Plan

## Goal
Finish wiring up the Ironwail-Go engine so it can launch a window, run the game loop, and fully integrate the QuakeC VM builtins with the newly ported Server and BSP logic. Clear out remaining TODOs and remove dead code.

## Key Directives
- **Dependency Flow**: The QC VM depends on the Server for entity management. The main loop depends on the Renderer and the Host.
- **Scope Control**: Do not add new features not present in Quake. Focus on implementing the specific `TODO`s identified.

## Task Dependency Graph

```text
[Wave 1]
  (A) Main Game Loop Hookup (main.go) -> Required to see anything happen
       |
       v
[Wave 2]
  (B) Entity Linking & Basic Builtins (edict.go, builtins.go)
       |
       v
[Wave 3]
  (C) Advanced QC Builtins (Movement & Search)
       |
       v
[Wave 4]
  (D) Host Console Commands (commands.go)
       |
       v
[Wave 5]
  (E) Final Codebase Verification & Cleanup (unused files)
```

## Task Definitions & Skills

### Wave 1: The Loop

**Task A: Main Game Loop Hookup**
- [x] **Description**: Modify `cmd/ironwailgo/main.go` to use `gameRenderer.OnUpdate` to call `gameHost.Frame(dt)` and then call `gameRenderer.Run()` to start the blocking event loop.

### Wave 2: Entity Foundation

**Task B: Entity Linking & Basic Builtins**
- [x] **Description**: Implement `SV_UnlinkEdict` and field value parsing in `internal/server/edict.go`. Wire up `spawn`, `remove`, `setorigin`, `setsize`, `setmodel` in `internal/qc/builtins.go` to call the appropriate server methods.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/qc` passes new tests validating builtin behavior.

### Wave 3: Advanced Builtins

**Task C: QC Builtins - Movement & Search**
- [x] **Description**: Wire up `find`, `findfloat`, `findradius`, `nextent` in `internal/qc/builtins.go`. Wire up `walkmove`, `droptofloor`, `movetogoal`, `changeyaw` using the ported movement logic in `internal/server`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/qc` passes movement builtin tests.

### Wave 4: Console Commands

**Task D: Host Console Commands**
- [x] **Description**: Implement the remaining console commands in `internal/host/commands.go`: `changelevel`, `restart`, `kill`, `god`, `noclip`, `notarget`, `give`, `name`, `color`, `ping`.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/host` passes command execution tests.

### Wave 5: Cleanup

**Task E: Final Codebase Cleanup**
- [x] **Description**: Scan for and remove any remaining original C files or unused Go packages/files that have been superseded by the port.
- **Category**: `quick`
- **Skills**: None needed.
- **QA**: `go build ./...` succeeds and `C` directory is removed.

## Actionable TODO List for Caller

```typescript
// WAVE 1
task(category="quick", load_skills=[], description="Task A: Main Game Loop Hookup in main.go", prompt="Read .sisyphus/plans/cleanup-plan.md for Task A details", run_in_background=false)

// WAVE 2
task(category="deep", load_skills=[], description="Task B: Implement Entity Linking and basic QC builtins", prompt="Read .sisyphus/plans/cleanup-plan.md for Task B details", run_in_background=false)

// WAVE 3
task(category="deep", load_skills=[], description="Task C: Implement Movement and Search QC builtins", prompt="Read .sisyphus/plans/cleanup-plan.md for Task C details", run_in_background=false)

// WAVE 4
task(category="unspecified-high", load_skills=[], description="Task D: Implement Host Console Commands", prompt="Read .sisyphus/plans/cleanup-plan.md for Task D details", run_in_background=false)

// WAVE 5
task(category="quick", load_skills=[], description="Task E: Final Codebase Cleanup", prompt="Read .sisyphus/plans/cleanup-plan.md for Task E details", run_in_background=false)
```