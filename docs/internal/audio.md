# Audio Package

## Purpose
The `audio` package manages the sound subsystem for the engine. It is responsible for loading sound effects (SFX), caching decoded audio data, managing active sound channels, performing spatialization (3D sound), and mixing multiple sources into a single stream for output via a hardware backend.

## Key Types & Interfaces
- **`System`**: The central coordinator that owns the listener state, active channels, and the sound cache.
- **`Backend` (Interface)**: Abstracts the platform-specific audio device (e.g., SDL3, oto, or a null backend). It defines how to initialize the device, lock/unlock for DMA access, and get the current playback position.
- **`Channel`**: Represents an active sound being played. It tracks the sound effect, position in the world, volume, and playback state.
- **`SFX` & `SoundCache`**: `SFX` is the resource handle, while `SoundCache` holds the actual resampled PCM data.
- **`MixerPipeline` (Interface)**: Abstracts the mixing algorithm, allowing for different mixing strategies or mock implementations for testing.

## Core Workflow
1. **Initialization**: The `Host` initializes the `audio.System`, which in turn selects and initializes a `Backend`. The backend provides a `DMAInfo` struct describing the hardware buffer.
2. **Precaching**: Sounds are loaded from disk (WAV, OGG, MP3, etc.) and resampled to the output device's sample rate, then stored in the `SoundCache`.
3. **Triggering Sound**: Systems like the `Client` or `Server` trigger sounds (e.g., weapon fire, ambient noise). A `Channel` is allocated and configured with the sound's origin and volume.
4. **Update & Spatialization**: Every frame, the system recomputes the left/right speaker volumes for all active channels based on the current `ListenerState` (player position and orientation).
5. **Mixing (Painting)**: The system "paints" (mixes) the samples from active channels into the DMA buffer.
6. **Playback**: The backend hardware reads from the DMA buffer and plays the audio.

## Integration
- **Host**: Drives the audio system's `Update` loop every frame.
- **Client/Server**: Generate sound events that result in `StartSound` calls.
- **Filesystem (fs)**: Used to load sound files from the game directory or WAD files.
- **Cvars**: Uses the `cvar` package to control volume (`volume`, `s_volume`) and other settings.

## Learning Tips
- **Fixed-Point Arithmetic**: The mixer uses 24.8 fixed-point arithmetic in `SamplePair` to provide precision during mixing without the overhead of floating-point math.
- **DMA-Style Buffering**: The system uses a Direct Memory Access (DMA) model where it writes directly into a shared buffer that the hardware reads, mirroring classic sound card behavior.
- **Underwater Effect**: Look at `spatial.go` and `UnderwaterState` to see how a simple low-pass filter is applied when the player is submerged.
