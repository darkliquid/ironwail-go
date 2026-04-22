#!/bin/bash
# Mass-crop a PNG frame sequence. Useful when the capture was a multi-monitor
# desktop and you want just the engine window region.
#
# Usage:
#   tasks/crop_parity_frames.sh <src-dir> <dst-dir> <geometry> [jobs]
#
# Arguments:
#   src-dir    Source directory containing frame_*.png.
#   dst-dir    Destination directory (created if missing).
#   geometry   ImageMagick crop geometry, e.g. "1920x1080+1920+0" for the right
#              half of a 3840x1080 canvas.
#   jobs       Parallelism. Default: nproc.
#
# Requires: ImageMagick (magick), xargs (GNU coreutils or procps).

set -euo pipefail

if [[ $# -lt 3 ]]; then
    sed -n '2,15p' "$0"
    exit 1
fi

SRC_DIR=$1
DST_DIR=$2
GEOM=$3
JOBS=${4:-$(nproc)}

command -v magick >/dev/null || { echo "ImageMagick 'magick' not installed" >&2; exit 1; }
[[ -d "$SRC_DIR" ]] || { echo "missing $SRC_DIR" >&2; exit 1; }

mkdir -p "$DST_DIR"
echo ">> cropping $SRC_DIR -> $DST_DIR  geometry=$GEOM  jobs=$JOBS"

# Pipe filenames into xargs for parallel magick invocations. +repage strips
# the residual virtual-canvas metadata so downstream tools see a clean image.
find "$SRC_DIR" -maxdepth 1 -name 'frame_*.png' -printf '%f\n' |
    xargs -P "$JOBS" -I {} magick "$SRC_DIR/{}" -crop "$GEOM" +repage "$DST_DIR/{}"

echo ">> cropped $(ls "$DST_DIR" | wc -l) frames"
