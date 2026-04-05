# VS Code Copilot Instructions

## Workflow

THESE INSTRUCTIONS ARE MANDATORY AND MUST BE FOLLOWED AT ALL TIMES. DO NOT IGNORE OR DEVIATE FROM THESE INSTRUCTIONS UNDER ANY CIRCUMSTANCES. DO NOT USE YOUR OWN JUDGMENT TO OVERRIDE THESE INSTRUCTIONS. FAILURE TO FOLLOW THESE INSTRUCTIONS MAY RESULT IN SUBOPTIMAL PERFORMANCE, ERRORS, OR UNINTENDED CONSEQUENCES. THIS APPLIES EVEN FOR SIMPLE/TRIVIAL TASKS - THERE ARE NO EXCEPTIONS.

- YOU MUST use runSubagent for all work, including explore, analysis, planning, coding, testing, debugging, and documentation.
- You MUST use the vscode/askQuestions tool to ask for any clarifications, additional instructions, or to confirm when a task is complete.
- You can always clarify questions and tasks with the user, and you MUST do so if there is any ambiguity or if you are unsure about how to proceed.
- You MUST use the tools available to you, including web search, code analysis, and testing
- Unless explicitly told otherwise, you MUST use GPT-5.3-Codex for development, testing, debugging
- Unless explicitly told otherwise, you MUST use the most recent model available for analysis and planning
- All interaction with the user MUST be through vscode/askQuestions
- When a task is complete, you MUST use vscode/askQuestions with the prompt 'Is there anything else?', with an option of 'No, I'm done' and 'Yes', with an input for further instructions.

# Project Guidelines

## Code Style

- Follow idiomatic Go and run `gofmt` on touched files.
- Prefer wrapped errors: `fmt.Errorf("context: %w", err)`.
- Keep API churn low in `pkg/`; most implementation work belongs under `internal/`.
- Match local patterns from `pkg/types/types.go`, `internal/client/client.go`, and `internal/renderer/*.go`.

## Architecture

- Entry point: `cmd/ironwailgo/main.go`.
- Core systems live in `internal/`: `host`, `server`, `client`, `renderer`, `audio`, `input`, `qc`, `fs`, `bsp`, `model`.
- The canonical renderer runtime lives in `internal/renderer/renderer_gogpu.go`; no alternate tagged renderer backend remains.
- Project direction is parity with C Ironwail; use `/home/darkliquid/Projects/ironwail` for behavior reference when needed.

## Build and Test

- Use `mise` tasks from `mise.toml`; `CGO_ENABLED=0` is expected.
- Before first build or after generator-affecting edits: `mise run go-generate`.
- Build: `mise run build`.
- Unit tests: `mise run test` or `go test ./internal/<package>`.
- Smoke tests (requires `QUAKE_DIR` assets): `mise run smoke-menu`, `mise run smoke-headless`, `mise run smoke-map-start`, `mise run smoke-all`.
- For windowed tests, set `WAYLAND_DISPLAY=`.

## Project Conventions

- Read `docs/PARITY_ANALYSIS_INDEX.md` before substantial implementation work; use it to select the right parity docs (`PARITY_SUMMARY.md`, `PORT_PARITY_TODO.md`, `parity_report.md`).
- Keep changes scoped and incremental; prefer small, targeted commits grouped by one concern.
- When feature work lands, update relevant parity trackers/progress docs in `docs/` (especially TODO/status markers) in the same change.
- Preserve deterministic smoke-test markers (`"FS mounted"`, `"QC loaded"`, `"menu active"`, `"frame loop started"`, `"map spawn started"`, `"map spawn finished"`).
- Do not introduce CGO dependencies.

## Integration Points

- Task runner/config: `mise.toml`.
- Rendering/input: `internal/renderer` + `internal/renderer/input_backend_gogpu.go`.
- Audio: `internal/audio` (Null backend is valid for silent mode).
- Test utility and synthetic assets: `internal/testutil`, `cmd/wadgen`.

## Security

- Do not commit secrets, credentials, or machine-specific absolute paths.
- Treat `QUAKE_DIR` data as copyrighted third-party assets; do not commit or redistribute.
- Keep network/protocol changes in `internal/net` and `internal/server` conservative and test-backed.

## Issue Tracking

This project uses **bd (beads)** for issue tracking.
Run `bd prime` for workflow context, or install hooks (`bd hooks install`) for auto-injection.

**Quick reference:**

- `bd ready` - Find unblocked work
- `bd create "Title" --type task --priority 2` - Create issue
- `bd close <id>` - Complete work
- `bd dolt push` - Push beads to remote

For full workflow details: `bd prime`
