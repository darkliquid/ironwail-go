# Ironwail Go - Rough Edges & Parity Gaps

This document tracks behavioral differences, bugs, and missing features compared to the original C Quake/Ironwail engine, as identified during manual testing and code review.

## 1. Physics & Movement

- [x] **Liquid Entry Lockup**: Game freezes or stops responding to input when entering water, lava, or slime. (Fixed: Added recursion guard to `touchLinks` to prevent infinite loops during trigger execution).
- [x] **Wall Collision Sticking**: Player gets stuck on walls/corners instead of sliding off. (Fixed: Implemented standard Quake recursive plane clipping in `FlyMove` and `ClipVelocity`).
- [x] **Teleporter Failure**: `trigger_teleport` entities do not transport the player. (Fixed: The same recursion guard in `touchLinks` was preventing some teleport sequences from completing or was hanging the server).
- [ ] **Auto-Step Traversal**: (Partially addressed with `SV_StepMove`, but needs verification against full collision hulls).

## 2. Audio

- [ ] **No Sound Output**: Sound is not audible on some systems even when backends are initialized.
- [x] **Backend Visibility**: Startup logs now clearly state which audio backends (SDL3, Oto, etc.) were compiled in and which one was successfully selected.

## 3. Input & Interface

- [ ] **Remote Connect Rejection UX**: (Addressed, needs manual verification).
- [ ] **Console Polish**: (Echo, Clear, Condump addressed, needs manual verification).
