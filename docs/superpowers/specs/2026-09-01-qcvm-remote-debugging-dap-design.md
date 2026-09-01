# QCVM Remote Debugging Protocol (DAP) Design Specification

## Overview

This specification defines the architecture, protocol, and integration for the remote debugging protocol in `ironwail-go` and `qcmod` as tracked by issue `ironwail-go-g1p`. The implementation provides a pure Go Debug Adapter Protocol (DAP) server over TCP, allowing standard IDEs (VS Code, Cursor, Neovim via `nvim-dap`, Emacs) and external debugging tools to attach to the QuakeC Virtual Machine (QCVM).

## Architecture

The debugging subsystem is structured into a dedicated, clean subpackage `internal/qc/dap` that interacts with the QCVM and the engine via a pluggable `Target` interface.

```
+-------------------------------------------------------------+
| DAP Client (VS Code, Cursor, Neovim, Custom Tools)          |
+-------------------------------------------------------------+
                              |
                     TCP (DAP Wire Protocol)
                              |
+-------------------------------------------------------------+
| internal/qc/dap                                             |
|  - Server (TCP Listener & Connection Lifecycle)            |
|  - Codec (Content-Length Framing & JSON Encoding)           |
|  - Session (Breakpoints, Variable Ref Tracking, Stepping)   |
|  - Barrier (Thread-safe Synchronization & State Isolation)  |
+-------------------------------------------------------------+
                              | Target Interface
                              v
+-------------------------------------------------------------+
| Debug Targets                                               |
|  - internal/server.Server (Full Engine)                     |
|  - cmd/qcmod Simulator World (Standalone Dev Kit)           |
|  - Test Harnesses (Deterministic Unit / E2E Tests)          |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
| internal/qc.VM                                              |
|  - BreakHook & ExecuteFrom                                  |
|  - Call Stack (vm.Stack, vm.Depth, vm.XFunction)           |
|  - Globals, Locals, Edict Memory, Functions, Strings        |
+-------------------------------------------------------------+
```

### Components

1. **`internal/qc/dap/protocol.go`**:
   * Standard DAP 1.5x message structures: `Request`, `Response`, `Event`, `Message`.
   * Wire protocol parser: framing `Content-Length: <n>\r\n\r\n<json>`.
   * Typed structures for DAP commands: `initialize`, `attach`, `launch`, `setBreakpoints`, `setFunctionBreakpoints`, `threads`, `stackTrace`, `scopes`, `variables`, `evaluate`, `continue`, `next`, `stepIn`, `stepOut`, `pause`, `disconnect`.

2. **`internal/qc/dap/target.go`**:
   * `Target` interface decoupling DAP from specific server or simulation implementations:
     ```go
     type Target interface {
         VM() *qc.VM
         EdictCount() int
         GetEdictFloat(entNum, offset int) float32
         GetEdictString(entNum, offset int) string
         GetEdictVector(entNum, offset int) [3]float32
         GetEdictClassName(entNum int) string
         FieldNames() map[string]int
     }
     ```

3. **`internal/qc/dap/barrier.go` & `session.go`**:
   * Manages breakpoints (by function name or statement index).
   * Holds the execution barrier between the VM execution thread and the DAP TCP listener goroutine.
   * While paused at a breakpoint or single-step, the VM execution thread blocks on a channel.
   * State inspection is guaranteed data-race-free because the VM thread is stationary inside `BreakHook`.
   * Stepping commands (`next`, `stepIn`, `stepOut`, `continue`) update stepping rules and signal the resume channel.

4. **`internal/qc/dap/server.go`**:
   * TCP listener on `127.0.0.1:<port>`.
   * Accepts connections, spawns session handlers, dispatches incoming requests, and handles clean disconnects/resumptions.

## Protocol & Command Specifications

### Supported DAP Commands

| Command | Action |
|---|---|
| `initialize` | Negotiates capabilities: `supportsFunctionBreakpoints: true`, `supportsConfigurationDoneRequest: true`, `supportsEvaluateForHovers: true`. |
| `attach` / `launch` | Binds DAP session to the active target; triggers `initialized` event. |
| `setFunctionBreakpoints` | Arms breakpoints on QC function names (e.g. `monster_ogre`, `W_FireRocket`). Resolves names to statement indices via `vm.FindFunction`. |
| `setBreakpoints` | Arms statement-level breakpoints (`stmt:<idx>`). |
| `threads` | Returns thread ID `1`: `"QuakeC VM Thread"`. |
| `stackTrace` | Reconstructs the QuakeC call stack from `vm.Depth` and `vm.Stack`, providing function names, source/function indices, and statement offsets. |
| `scopes` | For each stack frame, creates 3 scopes: **Locals**, **Globals**, and **Edicts**. |
| `variables` | Resolves variables by `variablesReference`: <br>• Locals: function arguments and local variables. <br>• Globals: Quake globals (`self`, `other`, `time`, `world`, `v_forward`, etc.). <br>• Edicts: Active non-free entities with classnames. <br>• Edict Fields: Deep expansion of entity fields (`origin`, `health`, `angles`, etc.). |
| `evaluate` | Evaluates expressions in hovers or REPL (e.g. `self.health`, `12.origin`, `time`). |
| `continue` / `next` / `stepIn` / `stepOut` / `pause` | Controls simulation execution flow. |
| `disconnect` | Disarms breakpoints, releases the barrier, and cleanly shuts down the connection. |

## Engine & CLI Integration

1. **CLI Flags** (`cmd/ironwailgo`):
   * `-qcdbg [port]`: Starts the DAP TCP listener on port (default `2345`).
   * `-qcdbg-wait`: Blocks startup execution before initial map spawn until a DAP client connects and sends `configurationDone` or `continue`.

2. **Cvars & Commands** (`internal/server` & `internal/game`):
   * `qc_debug_port`: Cvar specifying listener port (`0` disables). Changing the cvar dynamically starts/stops the server.
   * `qc_debug_start [port]`, `qc_debug_stop`, `qc_debug_status`: Console commands to manage the debug listener.

3. **Standalone Dev Kit** (`cmd/qcmod`):
   * `qcmod dap [port]`: Starts the standalone QCVM simulator in headless DAP mode.

## Testing Strategy

1. **Protocol Framing & Codec Tests** (`internal/qc/dap/protocol_test.go`):
   * Validate DAP message framing (`Content-Length` headers) and serialization roundtrips for requests, responses, and events.

2. **Session & Barrier Logic Tests** (`internal/qc/dap/session_test.go`):
   * Verify breakpoint registration, step-over depth tracking, step-out depth tracking, and disconnect unblocking.

3. **End-to-End Integration Tests** (`internal/qc/dap/integration_test.go` & `internal/server/dap_integration_test.go`):
   * Run real TCP DAP server on `127.0.0.1:0`.
   * Connect with a mock client over TCP.
   * Perform end-to-end handshake (`initialize`, `setFunctionBreakpoints`, `configurationDone`).
   * Trigger QC execution in a background goroutine.
   * Assert receipt of `stopped` event (reason `"breakpoint"`).
   * Request `stackTrace`, `scopes`, and `variables`.
   * Perform single-step (`next`), assert `stopped` event (reason `"step"`).
   * Send `continue` and verify execution completes with expected side effects.
