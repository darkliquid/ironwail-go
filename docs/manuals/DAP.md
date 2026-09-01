# QuakeC / QuakeGo Remote Debugging (DAP) Manual

This manual explains how to use the **Debug Adapter Protocol (DAP)** remote debugger in Ironwail-Go and `qcmod` to debug QuakeC bytecode and QuakeGo gameplay logic interactively from modern IDEs (VS Code, Cursor, Neovim, Emacs) and external tools.

---

## Table of Contents

1. [Overview & Architecture](#overview--architecture)
2. [Quick Start](#quick-start)
3. [Starting the Debug Server](#starting-the-debug-server)
   - [Command-Line Flags (`ironwailgo`)](#command-line-flags-ironwailgo)
   - [In-Game Console & Cvars](#in-game-console--cvars)
   - [Standalone Dev Kit (`qcmod dap`)](#standalone-dev-kit-qcmod-dap)
4. [IDE Setup & Configuration](#ide-setup--configuration)
   - [Visual Studio Code & Cursor](#visual-studio-code--cursor)
   - [Neovim (`nvim-dap`)](#neovim-nvim-dap)
   - [Emacs (`dap-mode`)](#emacs-dap-mode)
5. [Debugger Features & Workflow](#debugger-features--workflow)
   - [Breakpoints](#breakpoints)
   - [Stepping & Execution Control](#stepping--execution-control)
   - [Call Stack Inspection](#call-stack-inspection)
   - [Variable & Edict Inspection (Locals, Globals, Edicts)](#variable--edict-inspection)
   - [Expression Evaluation & Watches](#expression-evaluation--watches)
6. [Command & Cvar Reference](#command--cvar-reference)
7. [Troubleshooting & FAQs](#troubleshooting--faqs)

---

## Overview & Architecture

Ironwail-Go includes a pure-Go implementation of Microsoft's [Debug Adapter Protocol (DAP)](https://microsoft.github.io/debug-adapter-protocol/) over standard TCP.

```text
+-------------------------------------------------------------+
| IDE / DAP Client (VS Code, Cursor, Neovim, Emacs)          |
+-------------------------------------------------------------+
                              |
                     TCP (DAP Wire Protocol)
                              |
+-------------------------------------------------------------+
| internal/qc/dap                                             |
|  - TCP Server & Codec (Content-Length Framing)             |
|  - Session State Machine (Breakpoints, Sequence Numbers)   |
|  - Execution Barrier (Thread-Safe VM Halting & Resume)     |
|  - Variable Manager (Locals, Globals, Edict Trees)         |
+-------------------------------------------------------------+
                              | Target Interface
                              v
+-------------------------------------------------------------+
| Targets:                                                    |
|  - internal/server.Server (Live Game Engine)                |
|  - cmd/qcmod Simulator World (Headless Dev Kit)             |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
| internal/qc.VM                                              |
|  - BreakHook & ExecuteFrom                                  |
|  - VM Stack, Globals, Functions, Edict Tables               |
+-------------------------------------------------------------+
```

### Key Highlights

- **Pure Go & Zero CGO**: Runs seamlessly on any platform without native binary extensions or CGO overhead.
- **Thread-Safe Stationary Inspection**: When a breakpoint is hit, the simulation execution thread blocks on an internal synchronization barrier inside `vm.BreakHook`. While paused, the DAP listener safely inspects VM memory, call stacks, globals, and entity states without data races or mutations.
- **Unified Target Support**: Both full gameplay in `ironwailgo` and headless simulator runs in `cmd/qcmod` use the exact same DAP protocol implementation.

---

## Quick Start

### 1. Launch Ironwail-Go with the debugger enabled

```bash
./ironwailgo -basedir /path/to/quake -qcdbg 2345 -qcdbg-wait
```

- `-qcdbg 2345`: Starts the DAP TCP server on `127.0.0.1:2345`.
- `-qcdbg-wait`: Pauses engine startup before initial map spawn until you connect your IDE debugger and complete configuration.

### 2. Connect from VS Code / Cursor

Create `.vscode/launch.json` in your project workspace:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Attach to Ironwail-Go QCVM",
      "type": "node",
      "request": "attach",
      "port": 2345,
      "address": "127.0.0.1"
    }
  ]
}
```

Press **F5** (Start Debugging). The debugger attaches, arms your function breakpoints, and resumes the engine!

---

## Starting the Debug Server

### Command-Line Flags (`ironwailgo`)

| Flag | Description |
|---|---|
| `-qcdbg` | Starts the DAP server on the default TCP port (`2345`). |
| `-qcdbg <port>` | Starts the DAP server on a specific TCP port (e.g. `-qcdbg 5000`). |
| `-qcdbg-wait` | Blocks engine execution right before initial `worldspawn` / progs initialization until a DAP client connects and sends `configurationDone` or `continue`. Ideal for debugging map spawn routines and entity initializers. |

**Example:**
```bash
./ironwailgo -basedir ./quake-data -qcdbg 2345 -qcdbg-wait +map e1m1
```

---

### In-Game Console & Cvars

You can dynamically start, stop, or inspect the debugger at runtime from the Quake in-game console (`~` key) without restarting the engine.

#### Console Commands

- `qc_debug_start [port|addr]` — Launches the DAP listener.
  - `qc_debug_start` (uses `qc_debug_port` or default `2345`)
  - `qc_debug_start 5000` (listens on `127.0.0.1:5000`)
  - `qc_debug_start 127.0.0.1:2345`
- `qc_debug_stop` — Stops any active DAP server and disconnects active clients.
- `qc_debug_status` — Prints current listener state (e.g. `DAP debug server listening on 127.0.0.1:2345`).

#### Cvars

- `qc_debug_port` — Sets the default port for the debug listener.
  - Setting `qc_debug_port 2345` in your `autoexec.cfg` or console ensures the engine remembers your preferred port.

---

### Standalone Dev Kit (`qcmod dap`)

When developing standalone QuakeGo gameplay mods, you can debug QCVM bytecode headlessly without launching the graphics engine or loading map files:

```bash
# Start standalone simulator on default port 2345
./qcmod dap

# Start on custom port
./qcmod dap 5000
```

The standalone simulator boots `progs.dat`, binds the DAP listener, and waits for your IDE to attach. Press `Ctrl+C` in the terminal to stop the server cleanly.

---

## IDE Setup & Configuration

### Visual Studio Code & Cursor

Because Ironwail-Go exposes a standard DAP TCP socket, you can use any DAP TCP attach configuration or a lightweight extension.

#### Method 1: Standard TCP Attach via Launch Configuration

Create or update `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Attach to Ironwail QCVM",
      "type": "node",
      "request": "attach",
      "port": 2345,
      "address": "127.0.0.1",
      "localRoot": "${workspaceFolder}",
      "remoteRoot": "${workspaceFolder}"
    }
  ]
}
```

> **Tip:** You can also configure Function Breakpoints directly in the **Breakpoints** pane in the Run & Debug view by clicking the **+** button and choosing **Add Function Breakpoint**.

---

### Neovim (`nvim-dap`)

Add the following to your `init.lua` or `dap` configuration:

```lua
local dap = require('dap')

dap.adapters.qcvm = {
  type = 'server',
  host = '127.0.0.1',
  port = 2345,
}

dap.configurations.quake = {
  {
    type = 'qcvm',
    request = 'attach',
    name = 'Attach to Ironwail-Go QCVM',
  },
}
```

Start debugging in Neovim with `:lua require('dap').continue()`.

---

### Emacs (`dap-mode`)

Add the adapter definition to your Emacs configuration:

```elisp
(dap-register-debug-template
  "Ironwail-Go QCVM Attach"
  (list :type "qcvm"
        :request "attach"
        :host "127.0.0.1"
        :port 2345
        :name "Ironwail-Go QCVM"))
```

---

## Debugger Features & Workflow

### Breakpoints

#### Function Breakpoints
You can set breakpoints on any QuakeC or QuakeGo function name (e.g. `monster_ogre`, `W_FireRocket`, `door_touch`, `player_run`, `SUB_UseTargets`).

- In VS Code: Click **+** in the **Breakpoints** panel ➔ **Add Function Breakpoint** ➔ Enter function name (e.g. `monster_ogre`).
- When the function's entry statement executes, the VM halts execution immediately and sends a `stopped` event to the IDE.

> **Note:** Breakpoints are supported on all bytecode functions. Engine built-in functions (such as `spawn` or `makestatic`) are native Go routines and cannot be set as bytecode function entry points.

---

### Stepping & Execution Control

When paused at a breakpoint, use standard IDE debug controls:

| Action | VS Code Shortcut | Behavior |
|---|---|---|
| **Continue** | `F5` | Resumes normal game simulation until the next breakpoint is hit. |
| **Step Over** (`next`) | `F10` | Executes the current QuakeC statement. If the statement is a function call, executes the entire called function and pauses on the next statement at the current call depth. |
| **Step Into** (`stepIn`) | `F11` | Steps into the very next QuakeC statement, descending into callees. |
| **Step Out** (`stepOut`) | `Shift+F11` | Continues execution until the current function returns to its caller. |
| **Pause** | `F6` | Halts simulation on the very next QuakeC statement executed anywhere in the engine. |
| **Disconnect** | `Shift+F5` | Disarms breakpoints and unblocks the engine simulation. The game continues running smoothly. |

---

### Call Stack Inspection

The **Call Stack** window displays the active call chain at the moment of the pause:

- **Top Frame**: The currently executing QuakeC function and statement index.
- **Parent Frames**: The stack of calling functions reconstructed from `vm.Stack` and `vm.Depth`.

Selecting any stack frame updates the Variables pane to reflect that frame's context.

---

### Variable & Edict Inspection

The debugger exposes three distinct variable scopes in your IDE's **Variables** pane:

```
▼ Variables
  ▼ Locals
    • self: edict 1 (player)
    • other: edict 12 (monster_ogre)
    • parm0: 100
  ▼ Globals
    • time: 42.15
    • self: 1
    • other: 12
    • world: 0
  ▼ Edicts
    ▶ [0]: worldspawn
    ▼ [1]: player
        • classname: "player"
        • health: 100
        • origin: [100.0, 250.0, -16.0]
        • velocity: [0.0, 0.0, 0.0]
        • angles: [0.0, 90.0, 0.0]
        • movetype: 3
        • solid: 3
    ▶ [2]: trigger_multiple
    ▶ [12]: monster_ogre
```

#### 1. Locals Scope
- Displays the active entity context (`self`, `other`).
- Displays function arguments (`parm0` through `parm7`).

#### 2. Globals Scope
- Displays key Quake engine globals: `time`, `self`, `other`, `world`, and global registers.

#### 3. Edicts Scope & Entity Drill-Down
- Lists all active entities in the server world by entity index (`[0]`, `[1]`, `[2]`, ...).
- Free/unused entities are filtered out for readability.
- Expanding any entity reveals its individual fields:
  - **Vectors**: `origin`, `velocity`, `angles` formatted as `[x, y, z]`.
  - **Strings**: `classname`, `model`, `target`, `targetname` formatted as quoted strings.
  - **Floats**: `health`, `nextthink`, `solid`, `movetype`, etc.

---

### Expression Evaluation & Watches

You can evaluate expressions in the **Debug Console** (REPL) or add them to the **Watch** pane:

| Expression | Example Result | Description |
|---|---|---|
| `time` | `42.15` | Current simulation time |
| `self` | `edict 1 (player)` | Current `self` entity and classname |
| `other` | `edict 12 (monster_ogre)` | Current `other` entity |
| `self.health` | `100` | Health field of `self` |
| `self.origin` | `[100, 250, -16]` | Origin vector of `self` |
| `self.velocity` | `[0, 0, 0]` | Velocity vector of `self` |
| `self.target` | `"door1"` | Target string of `self` |

---

## Command & Cvar Reference

### CLI Flags

| Flag | Argument | Description | Default |
|---|---|---|---|
| `-qcdbg` | `[port]` | Starts DAP TCP listener at startup | `2345` |
| `-qcdbg-wait` | None | Blocks startup before map spawn until debugger connects | Disabled |

### Cvars

| CVar | Type | Description | Default |
|---|---|---|---|
| `qc_debug_port` | integer | Default port for DAP debug listener | `0` (disabled) |

### Console Commands

| Command | Syntax | Description |
|---|---|---|
| `qc_debug_start` | `qc_debug_start [port\|addr]` | Starts DAP listener on given port/address |
| `qc_debug_stop` | `qc_debug_stop` | Stops active DAP listener and disconnects clients |
| `qc_debug_status` | `qc_debug_status` | Displays status and address of DAP listener |

### Developer Tools

| Command | Syntax | Description |
|---|---|---|
| `qcmod dap` | `qcmod dap [port]` | Runs standalone headless QCVM dev kit with DAP server |

---

## Troubleshooting & FAQs

### Q: Why does the game window freeze when a breakpoint hits?
**A:** This is intentional. When a breakpoint is hit, the simulation execution thread pauses at that exact statement to preserve memory integrity. Resuming (`F5` or `F10`) immediately unblocks the simulation.

### Q: Can I disconnect the debugger without closing the game?
**A:** Yes. Issuing a disconnect from your IDE (or closing the debug session) disarms all active breakpoints and signals the barrier to continue running normally. The game resumes without needing a restart.

### Q: Can I debug map spawn entities like `worldspawn` or initial item placement?
**A:** Yes! Launch Ironwail-Go with `-qcdbg-wait`:
```bash
./ironwailgo -basedir ./quake-data -qcdbg 2345 -qcdbg-wait +map e1m1
```
The engine will halt before executing `worldspawn` or entity spawn functions, allowing you to set breakpoints on functions like `worldspawn`, `light`, or `item_health` before they run.

### Q: Why is my function breakpoint not verifying?
**A:** Ensure the function name matches the QC function identifier (e.g. `monster_ogre` rather than `MonsterOgre`). Note that engine builtins (such as `makevectors` or `setorigin`) are native Go stubs with negative statement indices and cannot accept bytecode breakpoints.
