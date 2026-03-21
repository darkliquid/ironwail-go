# Quake Engine Specification (QSPEC) - Ironwail Go

This document defines the formal behavioral specifications for the Ironwail Go engine, ensuring 100% parity with the canonical Quake C implementation.

---

## 1. Filesystem (VFS) Subsystem

The Virtual Filesystem (VFS) manages asset loading across multiple data sources with strict precedence rules.

### 1.1 Search Path Logic
The engine must resolve file requests by scanning prioritized "search paths."

```mermaid
graph TD
    A[File Request: 'progs.dat'] --> B{Check Mod Dir?}
    B -- Yes --> C{Loose File?}
    C -- Found --> D[Return Content]
    C -- Not Found --> E{PAK Files 9-0?}
    E -- Found in PAK --> D
    E -- Not Found --> F{Check Base 'id1'?}
    F -- Yes --> G{Loose File?}
    G -- Found --> D
    G -- Not Found --> H{PAK Files 9-0?}
    H -- Found in PAK --> D
    H -- Not Found --> I{Check Engine PAK?}
    I -- Found in 'ironwail.pak' --> D
    I -- Not Found --> J[Return Error: Not Found]
```

- **Precedence**: Mod Loose > Mod PAKs > Base Loose > Base PAKs > Engine PAK.
- **Where in C**: `common.c:COM_InitFilesystem`.

---

## 2. Command and Cvar Systems

### 2.1 Execution Flow
Commands originate from the console, scripts (`.cfg`), or the network.

```mermaid
flowchart LR
    In[Input String] --> Token[Tokenization]
    Token --> Alias{Is Alias?}
    Alias -- Yes --> Expand[Expand Alias]
    Expand --> Token
    Alias -- No --> Cmd{Is Command?}
    Cmd -- Yes --> Exec[Execute Handler]
    Cmd -- No --> Cvar{Is Cvar?}
    Cvar -- Yes --> Set[Set Cvar Value]
    Cvar -- No --> Forward[Forward to Server]
```

- **Tokenization Rules**: Respect double quotes and backslash escapes.
- **Precedence**: Commands > Aliases > Cvars.
- **Where in C**: `cmd.c:Cmd_ExecuteString`.

---

## 3. Client/Server Networking

### 3.1 Signon Sequence
The protocol ensures both sides are synchronized before gameplay begins.

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Server
    C->>S: connect
    S->>C: svc_serverinfo (Map, Precaches)
    Note over C: Load Map & Assets
    C->>S: prespawn
    S->>C: svc_setview, svc_signonnum 1
    C->>S: spawn (name, colors)
    S->>C: svc_signonnum 2
    C->>S: begin
    S->>C: svc_signonnum 3
    Note over S: Send initial entities
    S->>C: svc_signonnum 4 (Done)
    Note over C,S: Gameplay Active
```

- **Protocol**: NetQuake / FitzQuake.
- **Where in C**: `cl_main.c:CL_SignonReply`.

---

## 4. Physics and Movement

### 4.1 Player Physics Loop
The movement model is calculated per-frame based on user input and environment state.

```mermaid
flowchart TD
    Start[Physics Frame] --> OnGround{On Ground?}
    OnGround -- Yes --> Friction[Apply Friction]
    OnGround -- No --> Gravity[Apply Gravity]
    Friction --> Velocity[Calculate Velocity]
    Gravity --> Velocity
    Velocity --> Move[SV_PushEntity / SV_Move]
    Move --> Collision{Collision?}
    Collision -- Yes --> Clip[SV_ClipVelocity]
    Clip --> Move
    Collision -- No --> Unstick[SV_CheckStuck]
    Unstick --> End[Update Origin/Angles]
```

- **Hulls**: Point (0), Player (1), Large (2).
- **Step Height**: Maximum 18 units.
- **Where in C**: `sv_phys.c:SV_Physics_Client`.

---

## 5. Rendering & Effects

### 5.1 Light Style Evaluation
Dynamic lighting is controlled by "lightstyle" strings.

- **String Format**: `a-z` (0.0 to 2.0 brightness).
- **Interpolation**: Linear lerp between characters based on `cl.time * 10`.
- **Where in C**: `cl_main.c:CL_RunLightStyles`.

### 5.2 Particle Physics
Particles are purely client-side and do not affect gameplay state.
- **Types**: Tracers, Explosions, Blood, Bubbles, etc.
- **Where in C**: `r_part.c:R_RunParticle`.

---

## 6. Audio Spatialization

Calculates 3D volume and panning for mono samples.

- **Volume**: `(1.0 - distance * attenuation) * master_volume`.
- **Panning**: `0.5 + 0.5 * (DotProduct(ListenerRight, VectorToSource))`.
- **Where in C**: `snd_dma.c:SND_Spatialize`.
