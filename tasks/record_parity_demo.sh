#!/bin/bash
# Record a demo playback from a Quake engine (C Ironwail or ironwail-go) to an
# MP4 file (and optionally a PNG sequence) for side-by-side visual parity
# comparison on Wayland.
#
# Usage:
#   tasks/record_parity_demo.sh <engine-bin> <out-base> [demo] [duration] [fps]
#
# Arguments:
#   engine-bin  Path to the engine binary (e.g. /.../ironwail or ./ironwailgo-wgpu).
#   out-base    Output basename. Produces <out-base>.mp4 and, if EXTRACT_PNG=1,
#               <out-base>/frame_%05d.png.
#   demo        Demo name to play. Default: demo1.
#   duration    Capture length in seconds. Default: 30.
#   fps         Capture framerate. Default: 72 (matches Quake's netInterval).
#
# Environment:
#   QUAKE_BASEDIR  Required. Directory containing id1/ with pak files.
#   GSR_WINDOW     gpu-screen-recorder -w value. Default: "screen" (whole desktop).
#                  Use a monitor name (e.g. "DP-1") or "portal" for window capture.
#                  On niri, list outputs with:   niri msg outputs
#   GSR_QUALITY    gpu-screen-recorder -q value. Default: "ultra" (near-lossless).
#   GSR_CODEC      gpu-screen-recorder -k value. Default: "h264".
#   WINDOW_W       Engine window width  (default 1280).
#   WINDOW_H       Engine window height (default 720).
#   STARTUP_DELAY  Seconds to wait after launching engine before capture starts
#                  (default 3). Increase if map load is slow.
#   EXTRACT_PNG    If "1", run ffmpeg afterwards to extract PNG frames.
#   CROP           Optional ffmpeg crop filter (e.g. "1280:720:100:50") applied
#                  during PNG extraction. Useful when capturing the whole screen
#                  and you want just the engine window.
#
# Notes:
#   * gpu-screen-recorder captures via DRM/PipeWire on Wayland (Niri, GNOME,
#     KDE, wlroots). Install from AUR: paru -S gpu-screen-recorder.
#   * MP4 output uses hardware encoding when available (vaapi/nvenc). Quality
#     "ultra" is effectively lossless for our purposes.
#   * PNG extraction is off by default to save disk; enable for diffing.

set -euo pipefail

if [[ $# -lt 2 ]]; then
    sed -n '2,33p' "$0"
    exit 1
fi

ENGINE_BIN=$1
OUT_BASE=$2
DEMO=${3:-demo1}
DURATION=${4:-30}
FPS=${5:-72}

: "${QUAKE_BASEDIR:?QUAKE_BASEDIR must point at a directory containing id1/}"
GSR_WINDOW=${GSR_WINDOW:-screen}
GSR_QUALITY=${GSR_QUALITY:-ultra}
GSR_CODEC=${GSR_CODEC:-h264}
WINDOW_W=${WINDOW_W:-1280}
WINDOW_H=${WINDOW_H:-720}
STARTUP_DELAY=${STARTUP_DELAY:-3}
EXTRACT_PNG=${EXTRACT_PNG:-0}

command -v gpu-screen-recorder >/dev/null || {
    echo "gpu-screen-recorder not installed. Install with: paru -S gpu-screen-recorder" >&2
    exit 1
}
[[ -x "$ENGINE_BIN" ]] || { echo "engine binary not executable: $ENGINE_BIN" >&2; exit 1; }

OUT_MP4="${OUT_BASE}.mp4"
mkdir -p "$(dirname "$OUT_MP4")"

echo ">> engine:     $ENGINE_BIN"
echo ">> output:     $OUT_MP4"
echo ">> demo:       $DEMO"
echo ">> duration:   ${DURATION}s @ ${FPS}fps"
echo ">> resolution: ${WINDOW_W}x${WINDOW_H}"
echo ">> gsr window: $GSR_WINDOW (quality=$GSR_QUALITY codec=$GSR_CODEC)"

# Engines take slightly different flag sets. C Ironwail uses "-window", the
# Go port is always windowed and doesn't accept it. Sniff by probing --help.
ENGINE_FLAGS=(-basedir "$QUAKE_BASEDIR" -width "$WINDOW_W" -height "$WINDOW_H")
if "$ENGINE_BIN" -h 2>&1 | grep -q -- '-window'; then
    ENGINE_FLAGS+=(-window)
fi

# Launch engine with deterministic settings and auto-playdemo.
# host_framerate pins the simulation timestep to (1/FPS) seconds per host
# frame, decoupling demo playback from wall-clock jitter so both engines
# produce byte-identical simulation states at equal frame indices. Without
# this, host frame jitter causes ±1-frame drift that accumulates over the
# recording duration.
# +togglemenu dismisses the main menu that both engines auto-open at startup
# (Ironwail's splash screen shows the menu until the user dismisses it; Go
# mirrors that). Without togglemenu, the demo is paused behind the menu for
# whatever time the menu is visible, and small differences in engine startup
# speed between C and Go translate directly into a frame-index desync at
# capture time. Issuing togglemenu from the command-line buffer guarantees
# both engines enter "menu closed, demo running" state deterministically
# before the first rendered frame.
FIXED_DT=$(awk "BEGIN{printf \"%.10f\", 1.0/${FPS}}")
# Force identical UI cvars so C and Go render the same menu/HUD layout
# regardless of what each engine's persisted config.cfg happens to contain.
# - scr_menuscale / scr_sbarscale: both write to different user-config paths
#   (C: ~/.ironwail/id1/ironwail.cfg, Go: ~/.ironwail/ironwail.cfg) so their
#   saved values drift. Pin to 2 for both.
# - hudstyle (C) and hud_style (Go): the two engines use different cvar names
#   AND different value enumerations. Force both to 0 (Classic) so the
#   parity harness renders a HUD Go fully supports. Modern HUD styles (1/2
#   in C) are not implemented in Go and would render as garbage.
"$ENGINE_BIN" \
    "${ENGINE_FLAGS[@]}" \
    +vid_vsync 0 \
    +host_maxfps "$FPS" +cl_maxfps "$FPS" \
    +host_framerate "$FIXED_DT" \
    +gamma 1 +contrast 1 \
    +crosshair 0 +scr_showturtle 0 \
    +scr_menuscale 2 +scr_sbarscale 2 \
    +hudstyle 0 +hud_style 0 \
    +playdemo "$DEMO" \
    +togglemenu \
    >/tmp/parity_engine_$$.log 2>&1 &
ENGINE_PID=$!

GSR_PID=""
cleanup() {
    kill "$ENGINE_PID" 2>/dev/null || true
    wait "$ENGINE_PID" 2>/dev/null || true
    [[ -n "$GSR_PID" ]] && kill "$GSR_PID" 2>/dev/null || true
}
trap cleanup EXIT

echo ">> engine PID: $ENGINE_PID; sleeping ${STARTUP_DELAY}s for window + demo start"
sleep "$STARTUP_DELAY"

if ! kill -0 "$ENGINE_PID" 2>/dev/null; then
    echo "engine exited before capture started. Log:" >&2
    cat /tmp/parity_engine_$$.log >&2
    exit 1
fi

echo ">> starting gpu-screen-recorder for ${DURATION}s"
gpu-screen-recorder \
    -w "$GSR_WINDOW" \
    -f "$FPS" \
    -q "$GSR_QUALITY" \
    -k "$GSR_CODEC" \
    -c mp4 \
    -o "$OUT_MP4" &
GSR_PID=$!

sleep "$DURATION"
kill -SIGINT "$GSR_PID" 2>/dev/null || true
wait "$GSR_PID" 2>/dev/null || true
GSR_PID=""

if [[ ! -s "$OUT_MP4" ]]; then
    echo "capture failed (empty output). Engine log:" >&2
    cat /tmp/parity_engine_$$.log >&2
    exit 1
fi
echo ">> wrote $OUT_MP4 ($(stat -c%s "$OUT_MP4") bytes)"

if [[ "$EXTRACT_PNG" == "1" ]]; then
    command -v ffmpeg >/dev/null || { echo "ffmpeg needed for EXTRACT_PNG=1" >&2; exit 1; }
    PNG_DIR="$OUT_BASE"
    mkdir -p "$PNG_DIR"
    VF_ARGS=()
    if [[ -n "${CROP:-}" ]]; then
        VF_ARGS=(-vf "crop=$CROP")
    fi
    echo ">> extracting PNG frames to $PNG_DIR${CROP:+ (crop=$CROP)}"
    ffmpeg -y -hide_banner -loglevel warning -i "$OUT_MP4" \
        "${VF_ARGS[@]}" \
        "$PNG_DIR/frame_%05d.png"
    echo ">> extracted $(ls "$PNG_DIR" | wc -l) frames"
fi

echo ">> done"
