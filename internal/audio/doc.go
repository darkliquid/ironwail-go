// Package audio manages Quake sound playback, mixing, caching, and backend integration.
//
// # Purpose
//
// The package turns gameplay sound events into mixed sample streams. It loads
// sound effects, keeps a cache of decoded data, spatializes active channels,
// mixes them into a DMA-style buffer, and feeds that buffer to a concrete audio
// backend.
//
// # High-level design
//
// A System owns listener state, active channels, precached sounds, and the
// selected Backend. Runtime code precaches effects, starts or replaces dynamic
// channels, recomputes stereo volumes from listener orientation, and paints
// samples ahead of playback time.
//
// # Role in the engine
//
// This is the engine's sound subsystem. Host startup wires it in, client and
// server activity generate sound events, and the renderer-independent listener
// update path keeps audio aligned with player view state.
//
// # Original C lineage
//
// The closest Ironwail/Quake sources are snd_dma.c, snd_mix.c, snd_mem.c,
// snd_wave.c, snd_sdl.c, plus related CD/music concepts from bgmusic.c.
//
// # Deviations and improvements
//
// The Go port uses interface-driven backend selection and can fall back among
// SDL3, oto, or a null backend instead of assuming one C audio path. Typed
// structs, slice-backed buffers, and explicit errors replace pointer-heavy
// shared globals. The result keeps Quake's channel-mixing model while making it
// testable and pure Go.
package audio