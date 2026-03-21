# Guided Walkthrough: Starting a New Game and Pressing Forward

This walkthrough follows a very small player action with a surprisingly large blast radius:

1. choose **New Game**
2. load the `start` map
3. press the forward key
4. watch that intent become authoritative player movement

The main lesson is that even single-player is structured as a client talking to a server. The server owns truth; the client mostly expresses intent and then renders the results.

## Big picture

When you choose **New Game**, the menu does not call deep engine APIs directly. It queues a set of classic Quake console commands:

- `disconnect`
- `maxplayers 1`
- `deathmatch 0`
- `coop 0`
- `map start`

That command-driven design matters because it keeps menu actions, console actions, config scripts, and automation all using the same control path.

Once the map is running, pressing `W` also does not directly move the player. The key binding resolves to `+forward`, which updates a `KButton`. Later in the frame, the client converts its accumulated button state into a `UserCmd`, and the authoritative server consumes that `UserCmd` to run movement code.

## Control flow, step by step

### 1. Menu input is routed to the menu manager

Input enters through the input backend and is normalized by `internal/input`. `cmd/ironwailgo/game_input.go` then dispatches keys according to the current `KeyDest`.

While the menu is active, the key goes to `g.Menu.M_Key(...)`.

This layered model is worth understanding:

1. platform/backend polling
2. translation to Quake key codes
3. routing by destination (`console`, `menu`, `game`, `message`)

That separation keeps the higher-level game code blissfully unaware of SDL details.

### 2. Choosing **New Game** queues console commands

In `internal/menu/menu_main.go`, the single-player menu logic handles the **New Game** selection by hiding the menu and queueing textual commands.

This is the first unintuitive but elegant pattern in the flow: menu actions are mostly command producers.

So the data flow is:

```text
Enter key
  -> menu selection
  -> command text
  -> command buffer
```

### 3. `map start` goes through host map startup

The important host path is:

```text
CmdMap("start")
  -> startLocalServerSession(...)
  -> Server.Init(...)
  -> Server.SpawnServer(...)
```

`Server.SpawnServer(...)` loads the world, spawns entities, initializes signon buffers, and prepares the authoritative simulation state.

Even for single-player, the engine now behaves like a real server booting a level.

### 4. The local client performs a loopback signon handshake

After the map is spawned, the host starts a local session. This includes a mini signon flow very similar to multiplayer:

- local server info
- `prespawn`
- `spawn`
- `begin`

On the server side, `internal/server/user.go` handles those string commands in `handleClientStringCommand(...)`. During `spawn`, it calls the player-spawn logic, including `PutClientInServer` when the QuakeC VM provides it.

This is another big architectural point: single-player is not a shortcut around the network model. It is the same conceptual client/server lifecycle, just connected in-process.

### 5. The player entity is initialized

During spawn, the server:

- allocates or resets the player edict
- fills in baseline player physics state
- chooses a spawn point
- invokes QuakeC `PutClientInServer` when available
- sends spawn snapshot data back to the client

The player’s authoritative state lives in `server.Edict` and `server.EntVars`, not in the menu code or input layer.

### 6. Startup input mode flips from menu to gameplay

Once the client is fully active and signon is complete, the startup code uses `applyStartupGameplayInputMode()` to stop trapping input in the menu. From then on, gameplay keys are routed to the game handler instead of `menu.Manager`.

That transition is easy to miss when reading the code, but it explains why the exact same key means “navigate menu” at one moment and “move forward” a few frames later.

### 7. Pressing `W` resolves to `+forward`

Default bindings are installed during initialization. When the player presses `W`, the game input path:

1. looks up the binding text
2. executes the bound command
3. lands in `cmd/ironwailgo/game_commands.go`
4. updates the client’s `InputForward` `KButton`

That means the immediate result of `W` is not movement. It is a change in button state.

This is pure Quake DNA: input bindings are command strings, and movement keys are edge-tracked button states.

### 8. The client turns button state into a `UserCmd`

Later in the frame, the local loopback client calls `Client.AccumulateCmd()` in `internal/client/input.go`.

That function samples:

- `KButton` states like `InputForward`
- view angles
- sidemove/upmove
- impulses/buttons
- mouse-look deltas

and builds a `UserCmd`.

At this point, the client has finally transformed “the forward key is being held” into “I want to move this far forward this frame.”

### 9. Loopback sends the command directly to the authoritative server

For local play, the host uses an in-process loopback transport. `SubmitLoopbackCmd(...)` in `internal/server/user.go` stores the command as the client’s latest authoritative input:

- view angles
- forward/side/up move
- button bits
- impulse

The server also copies button bits into relevant `EntVars` fields like `Button0`.

This is the same conceptual payload that a remote client would serialize onto the network. Local play just skips the socket hop.

### 10. `Server.Frame()` runs clients before physics

The server frame ordering matters:

1. accept/handle client input
2. run client thinking
3. run physics
4. check rules
5. send messages

When `RunClients()` executes, it calls `SV_ClientThink(client)` for spawned clients.

That function is where the server interprets the player’s latest `UserCmd` and decides how to move the player.

### 11. Player movement is applied authoritatively

Inside the server movement path, engine code chooses the appropriate movement behavior:

- walk
- air move
- water move
- noclip

Walking flows through routines like `PhysicsWalk()`. The exact path depends on flags, water state, ground state, and movement type.

The key point is that the authoritative server owns this decision. The client does not get to declare “I moved here”; it only submits intent.

### 12. QuakeC wraps the movement step

This is where the code gets especially interesting. Player movement is not just raw engine physics. The server still invokes QuakeC hooks like:

- `PlayerPreThink`
- `PlayerPostThink`

around the engine-side movement.

So the final movement result is a collaboration between:

- Go engine physics
- Go collision/world code
- QuakeC gameplay logic loaded from `progs.dat`

That design is powerful but can be unintuitive until you realize the server is constantly bridging between Go entity state and QC VM entity state.

### 13. The client reads the updated world back and renders it

After the server frame, the loopback client reads the server’s outgoing data, parses it through the normal client parser, relinks entities, predicts players, and then the renderer consumes the updated state.

So even in single-player, the visual result of pressing forward is:

```text
input -> command -> button state -> usercmd -> server movement -> parsed updates -> render
```

not:

```text
input -> move camera directly
```

## Data flow

### Control flow

```text
menu selection
  -> command buffer
  -> map start
  -> local server session
  -> local signon
  -> active gameplay input mode
```

### Input flow

```text
W key
  -> +forward binding
  -> client.KButton(InputForward)
  -> Client.AccumulateCmd()
  -> UserCmd.ForwardMove
  -> Server.SubmitLoopbackCmd(...)
```

### Simulation flow

```text
UserCmd
  -> RunClients()
  -> SV_ClientThink()
  -> movement / physics / QC hooks
  -> authoritative edict state
  -> server message
  -> client parser
  -> renderer
```

## Sequence diagram

```mermaid
sequenceDiagram
    participant Input as input.System
    participant Menu as menu.Manager
    participant Cmd as command system
    participant Host as Host
    participant Server as Server
    participant Loop as localLoopbackClient
    participant Client as Client
    participant QC as QC VM
    participant Renderer as Renderer

    Input->>Menu: Enter on "New Game"
    Menu->>Cmd: disconnect; maxplayers 1; deathmatch 0; coop 0; map start
    Cmd->>Host: CmdMap("start")
    Host->>Server: SpawnServer("start")
    Host->>Loop: local signon steps
    Loop->>Server: prespawn / spawn / begin
    Server->>QC: PutClientInServer

    Input->>Cmd: +forward
    Cmd->>Client: KeyDown(InputForward)
    Loop->>Client: AccumulateCmd()
    Loop->>Server: SubmitLoopbackCmd(UserCmd)
    Server->>QC: PlayerPreThink / PlayerPostThink
    Server-->>Loop: local server message
    Loop->>Client: ParseServerMessage(...)
    Renderer->>Client: relink/predict/render
```

## Subsystems involved

One press of the forward key touches all of these:

- input backend
- input router
- command system
- client input state
- host frame scheduler
- loopback transport
- authoritative server
- QC VM
- collision/physics code
- client parser
- renderer
- audio listener update path

That is why this tiny scenario is such a good orientation exercise.

## Clever or unintuitive abstractions

### Menu actions are commands, not bespoke APIs

This unifies menu interaction with console scripts and automated startup behavior.

### `KButton` is an intent accumulator

`+forward` changes button state now so the client can fold it into one coherent `UserCmd` later in the frame.

### Single-player is still client/server

This is the most important mental model. Local play is not “the server codepath with rendering attached”. It is a real client and a real authoritative server connected through loopback.

### QC hooks surround engine physics

Gameplay and engine responsibilities are intentionally interleaved. The server owns authority, but the QC VM still shapes player behavior.

## If you want to keep reading

The best companion files for this walkthrough are:

- `internal/menu/menu_main.go`
- `cmd/ironwailgo/game_input.go`
- `cmd/ironwailgo/game_commands.go`
- `internal/client/input.go`
- `internal/host/commands_map.go`
- `internal/host/init.go`
- `internal/server/user.go`
- `internal/server/frame.go`
- `internal/server/physics.go`

