# Implementation Plan 21: Viewpoint JSON Console Command (`viewpos_json` / `camjson`)

**Priority**: Medium  
**Status**: Completed (2026-08-05)  
**Target Milestone**: Phase 21  

---

## 1. Executive Summary & Architectural Context

Behavioral and visual parity testing in Ironwail Go relies on camera viewpoints stored in `testdata/parity/viewpoints.json`. Currently, captured camera positions (`viewpos`) output plain-text console lines, requiring manual extraction, coordinate rounding, and JSON formatting to add new reference viewpoints.

`camdebug` (`internal/game/game_commands_camdebug.go`) already inspects the local player's view entity, render origin, and camera angles per frame. This plan adds a dedicated console command (`viewpos_json`, aliased to `camjson`) that formats the current camera state into a valid JSON object matching the `viewpoint` schema in `testdata/parity/viewpoints.json`.

---

## 2. Technical Strategy & Architecture

1. **Console Command Interface (`viewpos_json` / `camjson`)**:
   - Usage: `viewpos_json [id] [description]` or `camjson [id]`
   - Formats `camOrigin` and `camAngles` from `g.runtimeViewState()` into 2-decimal floating point arrays `[pos_x, pos_y, pos_z]` and `[pitch, yaw, roll]`.
   - Derives map name (`g.Host.Server.MapName` or `g.Client.LevelName`) and active game directory (`g.Host.GameDir`).

2. **Target JSON Output Format**:
   ```json
   {
     "id": "id1-e1m1-corridor",
     "game": "id1",
     "map": "e1m1",
     "pos": [-384.00, 512.00, -256.00],
     "angles": [0.00, 90.00, 0.00],
     "tag": "id1",
     "description": "e1m1 corridor view"
   }
   ```

3. **Subsystem Integration**:
   - `internal/game`: Implement `cmdViewposJSON` in `internal/game/game_commands_viewpoint.go`.
   - Register command centrally during `Game.Init` in `internal/game/game_commands.go`.
   - Support optional `-append` flag to write directly to `testdata/parity/viewpoints.json` when running in dev mode.

---

## 3. Step-by-Step Implementation Sequence

### Step 21.1: Implement Command Handler
- **Files**: `internal/game/game_commands_viewpoint.go`
- **Logic**: Construct `viewpoint` struct from `runtimeViewState()` and serialize using `json.MarshalIndent`.

### Step 21.2: Command Registration & Aliases
- **Files**: `internal/game/game_commands.go`
- **Logic**: Add `viewpos_json` and `camjson` to `g.Host.Cmd.AddCommand`.

### Step 21.3: Unit Test Suite
- **Files**: `internal/game/game_commands_viewpoint_test.go`
- **Logic**: Test command execution, argument parsing, JSON validity, and precision rounding.

---

## 4. Verification & Testing Strategy

1. **Unit Tests**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/game -run TestViewposJSON -count=1
   ```
2. **In-Engine Test**:
   Execute `viewpos_json e1m1-test "Test description"` in console and verify valid JSON format output.
