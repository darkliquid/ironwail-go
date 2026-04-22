#!/bin/bash
# Per-frame diff between two PNG sequences produced by record_parity_demo.sh.
#
# Usage:
#   tasks/compare_parity_frames.sh <ref-dir> <go-dir> <diff-dir> [fuzz-percent]
#
# Outputs:
#   <diff-dir>/frame_NNNNN.png   highlight of per-pixel differences
#   <diff-dir>/report.tsv        "<mismatched-pixels>\t<frame>" one per line
#   <diff-dir>/top.txt           top 20 worst frames (sorted)
#
# Requires: ImageMagick (compare).

set -euo pipefail

if [[ $# -lt 3 ]]; then
    sed -n '2,14p' "$0"
    exit 1
fi

REF_DIR=$1
GO_DIR=$2
DIFF_DIR=$3
FUZZ=${4:-5}

command -v compare >/dev/null || { echo "ImageMagick 'compare' not installed" >&2; exit 1; }
[[ -d "$REF_DIR" ]] || { echo "missing $REF_DIR" >&2; exit 1; }
[[ -d "$GO_DIR"  ]] || { echo "missing $GO_DIR"  >&2; exit 1; }

# Quick sanity check: warn if frames are desktop-sized rather than a single
# engine window (usually 1280x720). Huge AE numbers in top.txt almost always
# mean most of the diff signal is desktop chrome, not the game.
first_ref=$(ls "$REF_DIR"/frame_*.png 2>/dev/null | head -1 || true)
if [[ -n "$first_ref" ]] && command -v identify >/dev/null; then
    geom=$(identify -format '%wx%h' "$first_ref")
    if [[ "$geom" != "1280x720" && "$geom" != "1920x1080" ]]; then
        echo "WARNING: frames are $geom — likely a whole-desktop capture." >&2
        echo "         Capture just the engine monitor (GSR_WINDOW=<output>)" >&2
        echo "         or crop via CROP=WIDTH:HEIGHT:X:Y for meaningful diffs." >&2
    fi
fi

mkdir -p "$DIFF_DIR"
REPORT="$DIFF_DIR/report.tsv"
: > "$REPORT"

shopt -s nullglob
count=0
for ref in "$REF_DIR"/frame_*.png; do
    name=$(basename "$ref")
    go="$GO_DIR/$name"
    [[ -f "$go" ]] || { echo "missing go frame: $name" >&2; continue; }
    diff="$DIFF_DIR/$name"
    # AE = absolute count of mismatched pixels (after fuzz). compare prints
    # something like "1.07211e+06 (0.258515)" to stderr. Take the first token
    # and normalise scientific notation to an integer via awk.
    raw=$(compare -metric AE -fuzz "${FUZZ}%" "$ref" "$go" "$diff" 2>&1 || true)
    ae=$(printf '%s\n' "$raw" | awk '{printf "%d", $1+0; exit}')
    [[ -z "$ae" ]] && ae=0
    printf '%012d\t%s\n' "$ae" "$name" >> "$REPORT"
    count=$((count + 1))
done

echo ">> compared $count frames (fuzz=${FUZZ}%)"
sort -rn "$REPORT" | head -20 > "$DIFF_DIR/top.txt"
echo ">> top offenders:"
cat "$DIFF_DIR/top.txt"
