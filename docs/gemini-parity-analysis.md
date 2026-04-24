# Ironwail vs Ironwail-Go: Parity Analysis

This document outlines the weaknesses, incorrect implementations, and missing subsystems in the `ironwail-go` port compared to the reference `ironwail` C implementation, based on a comprehensive function-level parity audit and codebase analysis.

## 1. Network Protocol & Client Parsing Weaknesses

The client network parser currently lacks the resilience and advanced features of modern Ironwail:
*   **Coordinate and Angle Precision:** `readCoord` and `readAngle` in `client/parse_entities.go` are hardcoded to the default 16-bit fixed-point (FitzQuake) encoding and single-byte angles. 
    *   *Weakness:* Protocol flags for extended precision formats (`float`, `int32`, `24-bit` coords, and `float`/`short` angles) are ignored (`TODO`), meaning servers using modern Ironwail network protocol extensions will render entities incorrectly or desync.
*   **Temporary Entities (TEnts) Missing:** The entire `cl_tent.c` subsystem is unported.
    *   *Weakness:* Functions like `CL_ParseBeam`, `CL_ParseTEnt`, and `CL_UpdateTEnts` have no Go equivalents. Transient visual effects driven by network packets—such as explosions, rocket trails, lightning beams, and spawn particles—are missing or risk causing network parse errors if encountered in the datagram stream.
*   **Baseline and Sound Parsing:** Key packet parsers from `cl_parse.c` are missing, including `CL_ParseBaseline`, `CL_ParseLocalSound`, `CL_ParseStartSoundPacket`, and `CL_ParseStaticSound`. Sound playback tied to network events is likely broken or severely degraded.

## 2. Engine and Host Lifecycle Gaps

Several core gameplay and quality-of-life engine features from the C implementation are completely missing in the Go port (`host_cmd.c`):
*   **Save/Load System:** The entire save and load infrastructure (`Host_Loadgame_f`, `Host_Savegame_f`, `SaveList_Init`, `Host_InitSaveThread`) is unimplemented. Single-player progression cannot be saved.
*   **Demo Lifecycle:** While the Go port has some demo playback testing, the robust UI list and management commands (`DemoList_*`, `Host_Demos_f`, `Host_Startdemos_f`) are missing.
*   **Mod Management:** Ironwail's built-in mod/addon downloading system (`Modlist_DownloadJSON`, `Modlist_InstallerThread`, `Modlist_StartInstalling`) has not been ported.
*   **Skybox Manifests:** External skybox tracking (`SkyList_AddDirRec`, `SkyList_Rebuild`) is missing, limiting custom skybox support in community map packs.

## 3. Server and Multiplayer Limitations

The networking layer in `net_main.c` and `net_dgrm.c` reveals missing functionality for robust multiplayer:
*   **Server Browser:** The in-game server list (`slist`) and datagram polling mechanics (`NET_Slist_f`, `NET_SlistSort`, `Slist_Poll`) are unported.
*   **Hosting:** `NET_Listen_f` and `Datagram_Listen` are missing, suggesting the Go engine cannot cleanly bind ports to host internet/LAN listen servers.
*   **Server Physics:** `SV_Physics_Client` from `sv_phys.c` is flagged as missing, pointing to a potential weakness or incorrect implementation path in how client movement/physics is validated on the server frame.

## 4. Input and Audio Deficiencies

*   **Input Handling:** Functions handling specific mouse wheel behaviors (`IN_AccumMWheelPitch`) and mouse look states (`IN_MLookUp`) are missing from the `cl_input.c` translation, which could result in incorrect "mlook" behavior or scroll-wheel weapon switching parity issues.
*   **Audio Spatialization and Filtering:** Several C sound mixer functions are missing (`S_Spatialize`, `S_UnderwaterIntensityForContents`, `S_ApplyFilter`). Since Ironwail-Go uses its own `Oto`-based mixer (`internal/audio/mixer_async.go`), these missing C-functions suggest that environmental audio filtering (like muffled sound underwater) and accurate 3D spatialization may not perfectly match the C implementation's legacy DMA mixer.

## 5. Renderer Architectural Divergence

The legacy C renderer files (`r_world.c`, `r_alias.c`, `r_part.c`) show massive numbers of missing functions (e.g., `R_DrawBrushModels_Water`, `R_DrawParticles_Real`).
*   *Note:* The Go port uses a unified `GoGPU` backend (`internal/renderer/particle_gogpu.go`, `renderer_gogpu_frame.go`).
*   *Weakness:* While particles and models *are* rendered in Go, the divergence means strict 1:1 bug compatibility with C Ironwail's BSP/BModel sorting (transparency overlaps, water alpha, sky stencil) is difficult to verify and likely contains visual artifacts that will fail the `PARITY_SCENE_MATRIX.md` RMSE threshold tests under edge cases.
