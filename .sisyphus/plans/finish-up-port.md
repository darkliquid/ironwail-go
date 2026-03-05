# Ironwail-go Port Completion Plan

## Context
The user wants to complete the Quake port to a fully playable single-player baseline, strictly following the M0-M7 milestones. The execution strategy is strict sequential (M0 -> M7) with zero user intervention for verification.

## Task Dependency Graph

| Task | Depends On | Reason |
|------|------------|--------|
| Task 1 | None | Baseline consolidation, no prerequisites |
| Task 2 | Task 1 | Requires stable baseline to mount FS |
| Task 3 | Task 2 | Requires FS to load maps and progs |
| Task 4 | Task 3 | Requires map loaded to render BSP/Alias |
| Task 5 | Task 4 | Requires renderer to see movement/input |
| Task 6 | Task 5 | Requires gameplay loop for save/load |
| Task 7 | Task 6 | Final polish and stability |

## Parallel Execution Graph

Wave 1 (Start immediately):
└── Task 1: M0 - Consolidate Baseline & Repro Harness (no dependencies)

Wave 2 (After Wave 1 completes):
└── Task 2: M1 - Real Quake Filesystem/PAK Mounting & Hardening (depends: Task 1)

Wave 3 (After Wave 2 completes):
└── Task 3: M2/M3 - Boot-to-Map SP Pipeline & Client Signon (depends: Task 2)

Wave 4 (After Wave 3 completes):
└── Task 4: M4 - Renderer Gameplay MVP (BSP + Alias + HUD) (depends: Task 3)

Wave 5 (After Wave 4 completes):
└── Task 5: M5 - Input/Gameplay Feel & Temp Entities (depends: Task 4)

Wave 6 (After Wave 5 completes):
└── Task 6: M6 - Menu/UX & Save/Load Parity (depends: Task 5)

Wave 7 (After Wave 6 completes):
└── Task 7: M7 - Audio & Stability (depends: Task 6)

Critical Path: Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7
Estimated Parallel Speedup: 0% (Strict sequential execution required by constraints)

## Tasks

### Task 1: M0 - Consolidate Baseline & Repro Harness
**Description**: Update `cmd/ironwailgo/main.go` for deterministic startup logs and ensure `internal/testutil/assets.go` locates `id1/` cleanly.
**Delegation Recommendation**:
- Category: `quick` - Simple file changes and test additions.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: None
**Acceptance Criteria**: `make smoke-menu` and `make smoke-headless` pass. `go test ./internal/testutil` passes.

### Task 2: M1 - Real Quake Filesystem/PAK Mounting & Hardening
**Description**: Refactor `internal/fs/fs.go` to use `io/fs` or `afero.FS`, implement strict path sanitization, and ensure search path precedence.
**Delegation Recommendation**:
- Category: `deep` - Requires careful refactoring of filesystem abstractions and security hardening.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 1
**Acceptance Criteria**: `go test ./internal/fs -run TestPathTraversal` passes. Runtime headless test loads `progs.dat` and `maps/start.bsp`.

### Task 3: M2/M3 - Boot-to-Map SP Pipeline & Client Signon
**Description**: Wire client/server command handshakes in `internal/host/init.go`, `internal/host/commands.go`, `internal/client/client.go`, and `internal/client/parse.go`.
**Delegation Recommendation**:
- Category: `deep` - Complex state machine transitions and client/server handshake logic.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 2
**Acceptance Criteria**: Automated headless test `go run ./cmd/ironwailgo -headless -game id1 +map start | grep "Client state changed to Active"` passes.

### Task 4: M4 - Renderer Gameplay MVP (BSP + Alias + HUD)
**Description**: Build BSP and Alias renderers in `internal/renderer/`, extract HUD to `internal/hud/`, and implement headless `-screenshot` capture.
**Delegation Recommendation**:
- Category: `artistry` - Requires graphics programming, 3D math, and rendering pipeline setup.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 3
**Acceptance Criteria**: `go run ./cmd/ironwailgo -headless -screenshot test.png +map start` produces a valid PNG file.

### Task 5: M5 - Input/Gameplay Feel & Temp Entities
**Description**: Route inputs in `internal/input/types.go` and `internal/client/input.go`, send `UserCmd`, and stub Temp Entities in `internal/client/tent.go`.
**Delegation Recommendation**:
- Category: `deep` - Requires understanding of Quake movement prediction and input routing.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 4
**Acceptance Criteria**: `go test ./internal/input ./internal/client` passes cleanly.

### Task 6: M6 - Menu/UX & Save/Load Parity
**Description**: Implement Save/Load flows in `internal/savegame/save.go`, complete commands in `internal/host/commands.go`, and build Menu state machine in `internal/menu/`.
**Delegation Recommendation**:
- Category: `deep` - Serialization of complex game state and menu state machine.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 5
**Acceptance Criteria**: Unit test verifies Entity state equality after save and reload.

### Task 7: M7 - Audio & Stability
**Description**: Wire listener updates in `internal/audio/` and run automated memory leak/panic profiling.
**Delegation Recommendation**:
- Category: `deep` - Audio spatialization and memory profiling.
- Skills: [`git-master`] - For atomic commits.
**Skills Evaluation**:
- INCLUDED `git-master`: Required for committing changes.
- OMITTED `dev-browser`, `frontend-ui-ux`, `playwright`: No web/UI tasks.
**Depends On**: Task 6
**Acceptance Criteria**: `go run ./cmd/ironwailgo -headless +map start +wait 1800 +quit` completes without panic or excessive heap growth.

## Commit Strategy
Commit after each task completes successfully and passes its acceptance criteria. Use atomic commits with clear messages (e.g., "feat(fs): implement strict path sanitization").

## Success Criteria
All tasks complete, and the final verification wave passes:
```bash
go test ./...
make smoke-menu
make smoke-headless
make smoke-map-start
```

## TODO List (ADD THESE)

> CALLER: Add these TODOs using TodoWrite/TaskCreate and execute by wave.

### Wave 1 (Start Immediately - No Dependencies)

- [x] **1. M0 - Consolidate Baseline & Repro Harness**
  - What: Update `cmd/ironwailgo/main.go` for deterministic startup logs and ensure `internal/testutil/assets.go` locates `id1/` cleanly.
  - Depends: None
  - Blocks: 2
  - Category: `quick`
  - Skills: [`git-master`]
  - QA: `make smoke-menu` and `make smoke-headless` pass. `go test ./internal/testutil` passes.

### Wave 2 (After Wave 1 Completes)

- [x] **2. M1 - Real Quake Filesystem/PAK Mounting & Hardening**
  - What: Refactor `internal/fs/fs.go` to use `io/fs` or `afero.FS`, implement strict path sanitization, and ensure search path precedence.
  - Depends: 1
  - Blocks: 3
  - Category: `deep`
  - Skills: [`git-master`]
  - QA: `go test ./internal/fs -run TestPathTraversal` passes. Runtime headless test loads `progs.dat` and `maps/start.bsp`.

### Wave 3 (After Wave 2 Completes)

- [x] **3. M2/M3 - Boot-to-Map SP Pipeline & Client Signon**
  - What: Wire client/server command handshakes in `internal/host/init.go`, `internal/host/commands.go`, `internal/client/client.go`, and `internal/client/parse.go`.
  - Depends: 2
  - Blocks: 4
  - Category: `deep`
  - Skills: [`git-master`]
  - QA: Automated headless test `go run ./cmd/ironwailgo -headless -game id1 +map start | grep "Client state changed to Active"` passes.

### Wave 4 (After Wave 3 Completes)

- [ ] **4. M4 - Renderer Gameplay MVP (BSP + Alias + HUD)**
  - What: Build BSP and Alias renderers in `internal/renderer/`, extract HUD to `internal/hud/`, and implement headless `-screenshot` capture.
  - Depends: 3
  - Blocks: 5
  - Category: `artistry`
  - Skills: [`git-master`]
  - QA: `go run ./cmd/ironwailgo -headless -screenshot test.png +map start` produces a valid PNG file.

### Wave 5 (After Wave 4 Completes)

- [ ] **5. M5 - Input/Gameplay Feel & Temp Entities**
  - What: Route inputs in `internal/input/types.go` and `internal/client/input.go`, send `UserCmd`, and stub Temp Entities in `internal/client/tent.go`.
  - Depends: 4
  - Blocks: 6
  - Category: `deep`
  - Skills: [`git-master`]
  - QA: `go test ./internal/input ./internal/client` passes cleanly.

### Wave 6 (After Wave 5 Completes)

- [ ] **6. M6 - Menu/UX & Save/Load Parity**
  - What: Implement Save/Load flows in `internal/savegame/save.go`, complete commands in `internal/host/commands.go`, and build Menu state machine in `internal/menu/`.
  - Depends: 5
  - Blocks: 7
  - Category: `deep`
  - Skills: [`git-master`]
  - QA: Unit test verifies Entity state equality after save and reload.

### Wave 7 (After Wave 6 Completes)

- [ ] **7. M7 - Audio & Stability**
  - What: Wire listener updates in `internal/audio/` and run automated memory leak/panic profiling.
  - Depends: 6
  - Blocks: None
  - Category: `deep`
  - Skills: [`git-master`]
  - QA: `go run ./cmd/ironwailgo -headless +map start +wait 1800 +quit` completes without panic or excessive heap growth.

## Execution Instructions

1. **Wave 1**: Fire these tasks IN PARALLEL (no dependencies)
   ```
   task(category="quick", load_skills=["git-master"], run_in_background=false, prompt="Task 1: M0 - Consolidate Baseline & Repro Harness")
   ```

2. **Wave 2**: After Wave 1 completes, fire next wave IN PARALLEL
   ```
   task(category="deep", load_skills=["git-master"], run_in_background=false, prompt="Task 2: M1 - Real Quake Filesystem/PAK Mounting & Hardening")
   ```

3. Continue until all waves complete

4. Final QA: Verify all tasks pass their QA criteria
