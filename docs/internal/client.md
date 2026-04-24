# Client Package

## Purpose
The `client` package implements the client-side state of the Quake engine. It tracks everything the player knows about the world: the connection to the server, entity positions, stats (health, ammo), sounds, particles, and the local player's movement. It also handles the generation of input commands and the playback of demos.

## Key Types & Interfaces
- **`Client`**: The primary structure holding the entire client state, including entities, precached resources, and movement state.
- **`UserCmd`**: A structure representing a single frame of player input (view angles, movement, buttons, impulse) sent to the server.
- **`ClientState`**: An enum (`StateDisconnected`, `StateConnected`, `StateActive`) representing the connection lifecycle.
- **`KButton`**: Represents a key state (e.g., +forward, +attack) used to build movement commands.
- **`ColorShift`**: Used to calculate the screen tint (polyblend) for effects like being underwater or taking damage.

## Core Workflow
1. **Connection & Signon**: The client starts as `StateDisconnected`. After connecting, it goes through a multi-stage "signon" process (handled by `HandleSignonReply`) to synchronize map info, precache models/sounds, and spawn the player.
2. **Message Parsing**: Network messages from the server are parsed (in `parse.go`) and used to update the `Client` state (e.g., moving entities, updating health).
3. **Input Handling**: Each frame, the client samples the state of `KButton`s and mouse movement to build a `UserCmd`.
4. **Command Transmission**: The `SendCmd` function serializes the `UserCmd` into a `CLCMove` message and sends it to the server.
5. **Prediction**: To hide network latency, the client predicts the player's movement locally (in `prediction.go`) using the same physics logic as the server.
6. **Interpolation**: The `LerpPoint` function calculates a fraction (0.0 to 1.0) used to smoothly interpolate entity positions between server updates.

## Integration
- **Host**: Drives the client each frame, calling for input sampling, command transmission, and state updates.
- **Net**: Uses the `internal/net` package for protocol constants and `EntityState` definitions.
- **Audio/HUD/Renderer**: These subsystems consume the `Client` state to play sounds, draw the interface, and render the world from the player's perspective.

## Learning Tips
- **Signon Sequence**: The 4-stage signon sequence is a classic Quake protocol detail. See `HandleSignonReply` and `SignonReplyCommands`.
- **Interpolation Logic**: Examine `LerpPoint` in `client.go` to understand how Quake achieves smooth 60+ FPS visuals even when the server is only sending 20 updates per second.
- **Movement Prediction**: The code in `prediction.go` is a great study in "latency compensation," showing how the client simulates physics ahead of server confirmation.
- **State Separation**: Notice how `Entities` and `StaticEntities` are stored in maps/slices, modernizing the original C engine's fixed-size array approach.
