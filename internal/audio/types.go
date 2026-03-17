// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package audio provides the sound system for the Ironwail Go port.
// It handles sound effect loading, caching, mixing, and playback.
//
// The audio system is structured around several key components:
//   - DMA (Direct Memory Access): The audio backend that interfaces with the OS audio system
//   - Channels: Individual sound sources with position, volume, and attenuation
//   - SFX (Sound Effects): Cached audio data loaded from WAV files
//   - Mixer: Combines multiple channels into a single output stream
//
// The system supports:
//   - Mono and stereo sound effects
//   - 8-bit and 16-bit audio formats
//   - Sample rate conversion (resampling)
//   - 3D spatialization (stereo positioning based on listener position)
//   - Underwater audio effects
//   - Streaming audio for music playback
package audio

import (
	"sync"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// MaxChannels is the maximum number of simultaneous sound channels.
	// This includes dynamic entity sounds, ambient sounds, and static sounds.
	// Increased from original 128 to support complex scenes.
	MaxChannels = 1024

	// MaxDynamicChannels is the maximum number of dynamic (entity) sound channels.
	// These are sounds that move with entities like monsters, weapons, etc.
	MaxDynamicChannels = 128

	// NumAmbients is the number of ambient sound channels (water, wind, etc.).
	NumAmbients = 2

	// MaxSFX is the maximum number of unique sound effects that can be loaded.
	MaxSFX = 1024

	// MaxRawSamples is the buffer size for streaming audio (music).
	// This is a ring buffer that holds raw PCM samples.
	MaxRawSamples = 8192

	// PaintBufferSize is the size of the intermediate mixing buffer.
	// The mixer paints samples into this buffer before transferring to DMA.
	PaintBufferSize = 2048

	// SoundNominalClipDist is the default distance at which sounds are fully attenuated.
	// Sounds beyond this distance become inaudible.
	SoundNominalClipDist = 1000.0

	// WAVFormatPCM indicates standard PCM audio format in WAV files.
	WAVFormatPCM = 1
)

// ============================================================================
// PORTABLE SAMPLE PAIR
// ============================================================================

// SamplePair represents a single stereo sample with left and right channels.
// Each channel is a 32-bit integer to allow for mixing headroom before
// final clamping to 16-bit output range.
//
// The internal representation uses fixed-point arithmetic:
//   - Sample values are stored as 24.8 fixed point (24 bits integer, 8 bits fraction)
//   - This provides sufficient precision for mixing without floating-point overhead
//   - Final output is clamped to 16-bit signed integer range
type SamplePair struct {
	// Left channel sample value (32-bit for mixing headroom)
	Left int32
	// Right channel sample value (32-bit for mixing headroom)
	Right int32
}

// ============================================================================
// SOUND EFFECT (SFX)
// ============================================================================

// SFX represents a sound effect resource.
// Each SFX has a unique name and cached audio data. The cache system
// allows sounds to be loaded once and reused multiple times.
//
// SFX objects are typically created via PrecacheSound and retrieved
// by name during gameplay. The actual audio data is stored separately
// in a SoundCache object managed by the caching system.
type SFX struct {
	// Name is the filesystem path to the sound file (relative to "sound/" directory)
	// Maximum length is determined by the engine's MAX_QPATH constant.
	Name string

	// Cache holds the actual audio data once loaded.
	// Nil if the sound hasn't been loaded yet or failed to load.
	Cache *SoundCache

	// Cached indicates whether the sound data has been loaded into memory.
	// Used by the precache system to ensure sounds are ready before map start.
	Cached bool
}

// ============================================================================
// SOUND CACHE
// ============================================================================

// SoundCache contains the actual audio data for a loaded sound effect.
// The data is stored in a format ready for mixing (resampled to output rate).
//
// The cache system handles:
//   - Loading WAV files from disk
//   - Resampling to match the output device sample rate
//   - Converting between 8-bit and 16-bit formats
//   - Memory management for sound data
type SoundCache struct {
	// Length is the number of samples in the sound (not bytes).
	// For stereo sounds, this is the number of sample frames.
	Length int

	// LoopStart is the sample index where looping begins, or -1 if not looped.
	// When a sound reaches its end and LoopStart >= 0, playback jumps to
	// this position instead of stopping.
	LoopStart int

	// Speed is the sample rate in Hz (e.g., 11025, 22050, 44100).
	// This should match the output device rate after resampling.
	Speed int

	// Width is the bytes per sample (1 for 8-bit, 2 for 16-bit).
	// All sounds are stored as signed values internally.
	Width int

	// Stereo indicates whether this is a stereo (1) or mono (0) sound.
	// Quake primarily uses mono sounds for 3D spatialization.
	Stereo int

	// Data contains the raw audio samples.
	// For 8-bit sounds: signed 8-bit values (-128 to 127)
	// For 16-bit sounds: signed 16-bit values (-32768 to 32767)
	// For stereo: interleaved left/right samples
	Data []byte
}

// ============================================================================
// DMA (Direct Memory Access) INFO
// ============================================================================

// DMAInfo describes the audio output device configuration.
// The DMA system provides direct access to the audio output buffer,
// allowing the mixer to write samples directly without additional buffering.
//
// This is a low-level interface that mirrors the hardware audio buffer.
// The engine writes samples to Buffer, and the audio device reads them
// at the position indicated by SamplePos.
type DMAInfo struct {
	// Channels is the number of audio channels (1=mono, 2=stereo).
	Channels int

	// Samples is the total number of samples in the DMA buffer.
	// For stereo, this is the number of sample frames (L+R pairs).
	Samples int

	// SubmissionChunk is the minimum number of samples that must be written
	// before submission. Writing less than this may cause audio glitches.
	SubmissionChunk int

	// SamplePos is the current playback position in the DMA buffer.
	// The mixer writes ahead of this position to ensure continuous playback.
	SamplePos int

	// SampleBits is the bits per sample (8 or 16).
	SampleBits int

	// Signed8 indicates if 8-bit mode uses signed format (for Amiga AHI).
	// Standard 8-bit audio is unsigned (0-255, with 128 as silence).
	Signed8 bool

	// Speed is the output sample rate in Hz.
	Speed int

	// Buffer is the audio output buffer.
	// Size = Samples * Channels * (SampleBits / 8)
	Buffer []byte

	// Mutex protects concurrent access to SamplePos and Buffer.
	mu sync.Mutex
}

// ============================================================================
// SOUND CHANNEL
// ============================================================================

// Channel represents an active sound source being mixed.
// Each channel plays a single sound effect with its own volume,
// position, and attenuation settings.
//
// Channels are allocated from a pool:
//   - Channels 0 to NumAmbients-1: Ambient sounds (water, wind)
//   - Channels NumAmbients to NumAmbients+MaxDynamicChannels-1: Entity sounds
//   - Remaining channels: Static sounds (placed in world)
type Channel struct {
	// SFX is the sound effect being played on this channel.
	// Nil if the channel is not currently playing.
	SFX *SFX

	// LeftVol is the left speaker volume (0-255).
	// Calculated by spatialization based on sound position relative to listener.
	LeftVol int

	// RightVol is the right speaker volume (0-255).
	// Calculated by spatialization based on sound position relative to listener.
	RightVol int

	// End is the sample count when this sound will finish playing.
	// Compared against paintedtime to determine if sound is still active.
	End int

	// Pos is the current sample position within the sound effect.
	// Incremented as the sound plays.
	Pos int

	// PosFraction is the sub-sample position for interpolation (0-1).
	PosFraction float32

	// Pitch is the playback speed multiplier (1.0 = normal).
	Pitch float32

	// Looping indicates if this is a looping sound.
	// -1 = not looping, otherwise this is the loop start position.
	Looping int

	// EntNum is the entity number this sound is associated with.
	// Used for sound overriding (e.g., new weapon sound replaces old).
	EntNum int

	// EntChannel is the entity-specific channel number.
	// Allows entities to play multiple sounds simultaneously.
	// Channel 0 never overrides, -1 overrides all channels.
	EntChannel int

	// Origin is the world position of the sound source.
	// Used for distance attenuation and stereo panning.
	Origin [3]float32

	// Velocity is the world velocity of the sound source.
	// Used for Doppler effect calculation.
	Velocity [3]float32

	// DistMult is the distance attenuation multiplier.
	// Higher values make the sound fade faster with distance.
	DistMult float32

	// MasterVol is the master volume for this channel (0-255).
	// Combined with spatialization to produce left/right volumes.
	MasterVol int
}

// ============================================================================
// WAV FILE INFO
// ============================================================================

// WAVInfo contains parsed information from a WAV file header.
// Used during sound loading to validate and configure the sound cache.
type WAVInfo struct {
	// Rate is the sample rate in Hz.
	Rate int

	// Width is the bytes per sample (1 for 8-bit, 2 for 16-bit).
	Width int

	// Channels is the number of audio channels.
	// Quake primarily supports mono (1 channel) sounds.
	Channels int

	// LoopStart is the sample index for looping, or -1 if not looped.
	// Extracted from the WAV "cue " chunk if present.
	LoopStart int

	// Samples is the total number of samples in the file.
	Samples int

	// DataOfs is the byte offset to the audio data in the WAV file.
	DataOfs int
}

// ============================================================================
// SOUND SYSTEM STATE
// ============================================================================

// ListenerState holds the current listener (player) position and orientation.
// Used for 3D sound spatialization to determine left/right panning
// and distance attenuation.
type ListenerState struct {
	// Origin is the listener's world position.
	Origin [3]float32

	// Velocity is the listener's world velocity.
	Velocity [3]float32

	// Forward is the forward direction vector.
	Forward [3]float32

	// Right is the right direction vector.
	Right [3]float32

	// Up is the up direction vector.
	Up [3]float32
}

// ============================================================================
// RAW SAMPLES (STREAMING AUDIO)
// ============================================================================

// RawSamplesBuffer is a ring buffer for streaming audio (music).
// Audio decoders write samples here, and the mixer reads them during
// the paint cycle.
//
// The buffer is used as a circular buffer with s_rawend as the write
// position. The read position is derived from paintedtime.
type RawSamplesBuffer struct {
	// Samples holds the raw stereo samples.
	Samples [MaxRawSamples]SamplePair

	// End is the write position (sample count).
	End int
}

// ============================================================================
// SCALE TABLE
// ============================================================================

// ScaleTable is a precomputed volume scaling table.
// Used for efficient 8-bit sample mixing without floating-point math.
//
// The table is indexed by [volume][sample] where:
//   - volume ranges from 0-31 (representing volumes 0-255 in steps of 8)
//   - sample is the 8-bit sample value (0-255)
//
// Each entry contains the scaled sample value ready for accumulation.
type ScaleTable [32][256]int32

// ============================================================================
// FILTER STATE (UNDERWATER EFFECT)
// ============================================================================

// UnderwaterState tracks the underwater audio effect state.
// When the player is submerged, a low-pass filter is applied to simulate
// the muffling effect of water.
type UnderwaterState struct {
	// Intensity is the current filter intensity (0-2).
	// Smoothly interpolated to target value.
	Intensity float32

	// Alpha is the filter coefficient for smooth transitions.
	Alpha float32

	// Accum holds the filter state for each channel.
	Accum [2]float32
}

// ============================================================================
// MIXER PIPELINE INTERFACE
// ============================================================================

// MixerPipeline abstracts the audio mixing algorithm used by System.Update.
// The default implementation (Mixer) combines active sound channels plus raw
// streaming samples into the DMA buffer consumed by the backend.
//
// This indirection improves testability (mock/no-op mixers in headless tests)
// and allows alternative mixing strategies without changing System wiring.
type MixerPipeline interface {
	// PaintChannels mixes active channels into the DMA buffer and returns the
	// updated painted time cursor.
	PaintChannels(channels []Channel, rawSamples *RawSamplesBuffer, dma *DMAInfo, paintedTime, endTime int) int

	// SetSndSpeed updates the expected source sample rate used by mixer logic.
	SetSndSpeed(speed int)

	// SndSpeed reports the currently configured source sample rate.
	SndSpeed() int
}

// ============================================================================
// AUDIO BACKEND INTERFACE
// ============================================================================

// Backend defines the platform audio device contract used by System.
// Implementations (SDL3, oto, null, etc.) hide OS/hardware details while
// exposing just enough control for safe DMA writes and playback state queries.
type Backend interface {
	// Init opens/configures the playback device and allocates its DMA buffer.
	// It returns DMAInfo describing the negotiated format and buffer layout.
	Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error)

	// Shutdown closes the playback device and releases backend resources.
	Shutdown()

	// Lock synchronizes with backend playback so callers can safely write DMA.
	// Typical implementations stop/serialize the callback while the lock is held.
	Lock()

	// Unlock releases the DMA synchronization lock acquired by Lock.
	// Playback callbacks may resume reading the DMA buffer after this returns.
	Unlock()

	// GetPosition reports the current hardware play cursor in sample units.
	// System uses this to keep soundTime monotonic and avoid over/under-mixing.
	GetPosition() int

	// Block pauses/suspends output without tearing down backend state.
	// Used when the app is backgrounded or explicitly mutes runtime audio.
	Block()

	// Unblock resumes playback after Block.
	Unblock()
}
