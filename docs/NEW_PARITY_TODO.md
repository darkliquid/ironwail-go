# Ironwail Go - Comprehensive Parity Backlog

This document represents a comprehensive audit of the original C Ironwail codebase against the Go port, categorizing missing or heavily refactored functions into actionable parity tasks.

## 1. Console Commands (`_f` functions)
The following console commands from the C engine do not have a direct 1:1 equivalent in the Go port's `cmdsys` registry (or use a different naming convention that needs verification):

### Client & Rendering Commands
- [ ] `disconnect` (`CL_Disconnect_f`)
- [ ] `pr_ents` (`CL_PrintEntities_f`)
- [ ] `setstat` (`CL_SetStat_f`) / `setstatstring` (`CL_SetStatString_f`)
- [ ] `tracepos` (`CL_Tracepos_f`)
- [ ] `viewpos` (`CL_Viewpos_f`) / `CL_Viewpos_Completion_f`
- [ ] `water` (`V_Water_f` - view tinting)
- [ ] `particle_texture` (`R_SetParticleTexture_f`)
- [ ] `soundinfo` (`S_SoundInfo_f`)

### Host & Game Flow Commands
- [ ] `begin` (`Host_Begin_f`)
- [ ] `changelevel` (`Host_Changelevel_f`)
- [ ] `demos` (`Host_Demos_f`)
- [ ] `startdemos` (`Host_Startdemos_f`) / `stopdemo` (`Host_Stopdemo_f`)
- [ ] `kick` (`Host_Kick_f`)
- [ ] `kill` (`Host_Kill_f`)
- [ ] `maps` (`Host_Maps_f`) / `randmap` (`Host_Randmap_f`)
- [ ] `pause` (`Host_Pause_f`)
- [ ] `ping` (`Host_Ping_f`)
- [ ] `restart` (`Host_Restart_f`)
- [ ] `say` (`Host_Say_f`) / `say_team` (`Host_Say_Team_f`) / `tell` (`Host_Tell_f`)
- [ ] `status` (`Host_Status_f`)
- [ ] `viewframe` (`Host_Viewframe_f`)
- [ ] `viewmodel` (`Host_Viewmodel_f`)
- [ ] `viewnext` (`Host_Viewnext_f`) / `viewprev` (`Host_Viewprev_f`)

### Cheats & Debug
- [ ] `fly` (`Host_Fly_f`)
- [ ] `give` (`Host_Give_f`)
- [ ] `god` (`Host_God_f`)
- [ ] `noclip` (`Host_Noclip_f`)
- [ ] `notarget` (`Host_Notarget_f`)
- [ ] `setpos` (`Host_SetPos_f`)
- [ ] `skies` (`Host_Skies_f`)
- [ ] `map_checks` (`Map_Checks_f`)

### Network Commands
- [ ] `listen` (`NET_Listen_f`)
- [ ] `port` (`NET_Port_f`)
- [ ] `slist` (`NET_Slist_f`)
- [ ] `ban` (`NET_Ban_f`)
- [ ] `net_stats` (`NET_Stats_f`)

## 2. Multiplayer & Networking (`net_main.c`, `net_dgrm.c`, `sv_main.c`)
- [ ] **Server Visibility & PVS**: Implement `SV_AddToFatPVS`, `SV_EdictInPVS`, `SV_VisibleToClient` for proper network culling.
- [ ] **Signon Buffer Management**: Ensure `SV_AddSignonBuffer` and `SV_ReserveSignonSpace` logic is fully replicated for complex map loading.
- [ ] **Server Browser (Slist)**: Implement local network server broadcasting and discovery (`Slist_Poll`, `Slist_Send`, `PrintSlist`).
- [ ] **Datagram Realiability**: Replicate `Datagram_CanSendUnreliableMessage` and `Datagram_SearchForHosts`.

## 3. Client & Prediction (`cl_parse.c`, `cl_tent.c`, `cl_main.c`)
- [ ] **Temporary Entities**: Ensure all `CL_ParseTEnt` and `CL_ParseBeam` logic is mapped. (Go currently maps most to dynamic lights/marks, but exact parsing parity should be verified).
- [ ] **Entity Relinking**: Implement `CL_RelinkEntities` logic for client-side interpolation/prediction.
- [ ] **Sound Parsing**: Verify `CL_ParseLocalSound`, `CL_ParseStartSoundPacket`, and `CL_ParseStaticSound` exactly match C protocol extraction.
- [ ] **Packet Diagnostics**: Implement `CL_DumpPacket` for debugging.

## 4. Input & OS (`cl_input.c`)
- [ ] **Mouse Look & Wheel**: Ensure `IN_MLookUp` and `IN_AccumMWheelPitch` are handled by the SDL3/GLFW backends.

## 5. Renderer (`r_world.c`, `r_alias.c`, `r_part.c`)
- [ ] **Brush Model Batching**: C Ironwail has extensive batching (`R_AddBModelCall`, `R_FlushBModelCalls`, `R_CanMergeBModelAlphaPasses`). Ensure the Go OpenGL port handles these transparent passes correctly.
- [ ] **Alias Batching**: `R_Alias_CanAddToBatch`, `R_FlushAliasInstances`.
- [ ] **Sky & Water Overlays**: Verify `R_DrawBrushModels_SkyCubemap`, `R_DrawBrushModels_Water`, and `GL_WaterAlphaForEntityTextureType` fidelity.
- [ ] **Particle Systems**: Verify `R_ClearParticles`, `R_FlushParticleBatch`, and `R_ParseParticleEffect`.

## 6. Audio (`snd_dma.c`, `snd_mix.c`)
- [ ] **Audio Spatialization**: Ensure `SND_Spatialize` and `S_PlayVol` logic exactly matches Quake's distance/panning curve.
- [ ] **Filter Quality**: Verify `SND_Callback_snd_filterquality` and `S_ApplyFilter` (low-pass filtering) are implemented in the Go mixer.
- [ ] **Underwater Muffling**: Ensure `S_UnderwaterIntensityForContents` correctly muffles sound when submerged.
- [ ] **Buffer Management**: `S_ClearBuffer`, `S_BlockSound`, `S_UnblockSound` (Go implements `Block()`/`Unblock()`, verify usage context).

## 7. Physics (`sv_phys.c`)
- [ ] **All Entities Check**: `SV_CheckAllEnts` (often runs physics for all entities in a frame).
- [ ] **Stuck Resolution**: `SV_TryUnstick` (Go has `SV_CheckStuck`, verify it covers the same edge cases).
- [ ] **Friction**: `SV_WallFriction` (applying friction when sliding against a wall, currently missing from Go's `FlyMove`).

## 8. Mod & Addon System (`host_cmd.c`)
- [ ] **Modlist Downloading**: C Ironwail has a built-in addon downloader (`Modlist_DownloadJSON`, `Modlist_StartInstalling`, etc.). This is entirely absent in the Go port.
- [ ] **Extra Maps**: `ExtraMaps_Add`, `ExtraMaps_ParseDescriptions` for custom map menus.
- [ ] **Save/Demo/Sky Menus**: `DemoList_Rebuild`, `SaveList_Rebuild`, `SkyList_Rebuild`.
