# Quake Engine Specification (Ironwail Go Implementation)

This document formally defines the expected behaviors of the Quake engine as implemented in Ironwail Go, with references to the canonical C implementation where applicable.

---

## 1. Filesystem (FS) Subsystem

The filesystem provides a virtualized view of game data, prioritizing assets based on a specific search path precedence.

### 1.1 Search Path Precedence
The engine must prioritize files according to the following order (highest to lowest):
1. **Mod Loose Files**: Files in the active mod directory (e.g., `hipnotic/config.cfg`).
2. **Mod PAK Files**: Files within `pak%d.pak` in the active mod directory, in descending numeric order (e.g., `pak1.pak` overrides `pak0.pak`).
3. **Base Loose Files**: Files in the `id1/` directory.
4. **Base PAK Files**: Files within `pak%d.pak` in `id1/`, in descending numeric order.
5. **Engine PAK**: Files in `ironwail.pak` located in the application root.

- **Input**: Filename (e.g., `progs.dat`).
- **Expected Output**: Content of the first matching file found in the search path.
- **Where in C**: `common.c`, `COM_InitFilesystem`, `COM_AddGameDirectory`.

### 1.2 PAK File Format
The engine must support the standard Quake PACK format.
- **Header**: `PACK` (4 bytes), Directory Offset (int32), Directory Length (int32).
- **Directory Entry**: Filename (56 bytes, null-terminated), Position (int32), Length (int32).
- **Behavior**: Lookups must be case-insensitive.
- **Where in C**: `common.c`, `COM_LoadPackFile`.

---

## 2. Command and Cvar Systems

### 2.1 Command Tokenization
Command lines must be parsed into tokens, respecting double quotes for strings containing spaces and backslashes for escaping.
- **Example**: `bind t "say \"hello world\""` → tokens: `["bind", "t", "say \"hello world\""]`.
- **Where in C**: `cmd.c`, `Cmd_TokenizeString`.

### 2.2 Alias Expansion
Aliases provide a way to map a new command name to a string of one or more existing commands.
- **Behavior**: Built-in commands take precedence over aliases with the same name. Aliases can contain multiple semicolon-separated commands.
- **Where in C**: `cmd.c`, `Cmd_ExecuteString`.

### 2.3 Cvar Behavior
Cvars (Console Variables) store engine state and configuration.
- **Flags**: 
    - `FlagArchive`: Saved to `config.cfg`.
    - `FlagROM`: Read-only; cannot be changed by the user.
    - `FlagAutoCvar`: Automatically synchronizes with an engine-side variable.
- **Callbacks**: Triggered when the cvar's value changes.
- **Where in C**: `cvar.c`, `Cvar_Set`.

---

## 3. Client/Server Networking

### 3.1 Signon Sequence
The client must transition through several signon stages before becoming active:
1. `SignonNone`: Initial state.
2. `SignonPrespawn`: Receiving server info and precaches.
3. `SignonClientInfo`: Sending client information.
4. `SignonBegin`: Loading the map.
5. `SignonDone`: Fully connected and active.
- **Where in C**: `cl_main.c`, `CL_SignonReply`.

### 3.2 Entity Snapshots
The server sends delta-compressed snapshots of entities to the client.
- **Behavior**: The client maintains the previous frame's state to interpolate positions and angles. If a frame is missed, the client must "force link" (snap) to the new position to prevent visual glitches.
- **Where in C**: `cl_parse.c`, `CL_ParseDelta`.

---

## 4. Physics and Movement

### 4.1 Collision Hulls
Quake uses three fixed hulls for collision detection:
1. **Hull 0**: Point hull (0x0x0), used for projectiles and small objects.
2. **Hull 1**: Player hull (-16x-16x-24 to 16x16x32), used for the player and most monsters.
3. **Hull 2**: Large hull (-32x-32x-24 to 32x32x64), used for large monsters (e.g., Shambler).
- **Where in C**: `sv_phys.c`, `SV_HullForEntity`.

### 4.2 Movement Model
The engine implements a specific movement model including friction and gravity.
- **Friction**: Applied only when the entity is on the ground.
- **Gravity**: Applied constantly to non-flying, non-swimming entities.
- **Unsticking**: If an entity becomes stuck in geometry, the engine attempts to nudge it to a nearby free position using a recursive search.
- **Where in C**: `sv_phys.c`, `SV_Physics_Client`, `SV_CheckStuck`.

---

## 5. Rendering

### 5.1 Light Styles
Light styles define how lightmaps flicker or pulse.
- **Map**: A string of characters 'a' (dark) to 'z' (bright). 'm' is normal (1.0).
- **Behavior**: Interpolation between style characters is required for smooth transitions if enabled.
- **Where in C**: `cl_main.c`, `CL_RunLightStyles`.

### 5.2 Particle Systems
Particles are used for explosions, trails, and environment effects.
- **Behavior**: Particle positions are updated based on velocity, gravity, and decay.
- **Where in C**: `r_part.c`, `R_RunParticle`.

---

## 6. Audio Subsystem

### 6.1 Spatialization
Sound volume and panning are calculated based on the distance and orientation of the listener relative to the sound source.
- **Attenuation**: Volume decreases linearly or exponentially with distance.
- **Panning**: Calculated using the dot product between the listener's right vector and the vector to the sound source.
- **Where in C**: `snd_dma.c`, `SND_Spatialize`.
