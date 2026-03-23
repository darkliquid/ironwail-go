# Interface

## Main consumers

- `audio/runtime`, which delegates music control and refill behavior to this logic.

## Main API

Observed music control surface:
- `PlayCDTrack(...)`
- `PlayMusic(...)`
- `StopMusic()`
- `PauseMusic()`
- `ResumeMusic()`
- `SetMusicLoop(...)`
- `ToggleMusicLoop()`
- `CurrentMusicTrack()`
- `CurrentMusic()`
- `JumpMusic(order int) bool`

## Contracts

- Music playback requires either a loader or resolver path capable of returning music bytes.
- Playback state is advanced by the runtime update path through `updateMusic(endTime)`.
- Track looping behavior differs between explicit file playback and CD-style track playback with loop-track policy.
