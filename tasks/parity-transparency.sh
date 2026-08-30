#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

QUAKE_DIR="${QUAKE_DIR:-/home/darkliquid/Games/Heroic/Quake Enhanced/}"
if [ ! -d "${QUAKE_DIR}" ] && [ -d "${ROOT_DIR}/quake-data" ]; then
    QUAKE_DIR="${ROOT_DIR}/quake-data"
fi

IRONWAIL_C="${IRONWAIL_BIN:-${ROOT_DIR}/ironwail/Linux/ironwail}"
if [ ! -x "${IRONWAIL_C}" ] && [ -x "${ROOT_DIR}/ironwail" ]; then
    IRONWAIL_C="${ROOT_DIR}/ironwail"
fi

IRONWAIL_GO="${IRONWAIL_GO:-${ROOT_DIR}/ironwailgo}"
WIDTH="${PARITY_WIDTH:-1280}"
HEIGHT="${PARITY_HEIGHT:-720}"
OUTPUT_DIR="${ROOT_DIR}/testdata/parity/transparency"

echo "=== Transparency Parity Comparison Harness ==="
echo "Quake Data Dir: ${QUAKE_DIR}"
echo "C Ironwail:     ${IRONWAIL_C}"
echo "Go Ironwail:    ${IRONWAIL_GO}"
echo "Resolution:     ${WIDTH}x${HEIGHT}"
echo "Output Dir:     ${OUTPUT_DIR}"
echo

# Build Go binary if missing
if [ ! -x "${IRONWAIL_GO}" ]; then
    echo "Building ironwailgo..."
    (cd "${ROOT_DIR}" && CGO_ENABLED=0 go build -o "${IRONWAIL_GO}" ./cmd/ironwailgo)
fi

mkdir -p "${OUTPUT_DIR}"

MAP_NAME="test_transparency"
MAP_FOUND=0

for search_path in \
    "${QUAKE_DIR}/id1/maps/${MAP_NAME}.bsp" \
    "${QUAKE_DIR}/maps/${MAP_NAME}.bsp" \
    "${ROOT_DIR}/testdata/maps/${MAP_NAME}.bsp" \
    "${ROOT_DIR}/testdata/parity/${MAP_NAME}.bsp"; do
    if [ -f "${search_path}" ]; then
        MAP_FOUND=1
        echo "Found transparency test map at: ${search_path}"
        break
    fi
done

if [ "${MAP_FOUND}" -eq 1 ]; then
    REF_PNG="${OUTPUT_DIR}/${MAP_NAME}_ref.png"
    GO_PNG="${OUTPUT_DIR}/${MAP_NAME}_go.png"
    DIFF_PNG="${OUTPUT_DIR}/${MAP_NAME}_diff.png"

    if [ -x "${IRONWAIL_C}" ]; then
        echo "Capturing C reference screenshot for ${MAP_NAME}..."
        HOME_DIR="${HOME:-/home/darkliquid}"
        REF_USER_DIR="${HOME_DIR}/.ironwail/id1/screenshots"
        mkdir -p "${REF_USER_DIR}"

        CFG_REF="${OUTPUT_DIR}/ref_capture.cfg"
        cat <<EOF > "${CFG_REF}"
vid_fullscreen 0
vid_width ${WIDTH}
vid_height ${HEIGHT}
scr_viewsize 130
r_drawviewmodel 0
crosshair 0
fov 90
gamma 1
cl_screenshotname screenshots/test_transparency_ref
hideconsole
host_framerate 0.0001
setpos 0 0 0 0 0 0
wait; wait; wait; wait;
screenshot png
wait
quit
EOF
        "${IRONWAIL_C}" -basedir "${QUAKE_DIR}" -window -width "${WIDTH}" -height "${HEIGHT}" +map "${MAP_NAME}" +exec "${CFG_REF}" >/dev/null 2>&1 || true
        rm -f "${CFG_REF}"

        if [ -f "${REF_USER_DIR}/test_transparency_ref.png" ]; then
            mv "${REF_USER_DIR}/test_transparency_ref.png" "${REF_PNG}"
            echo "Saved C reference screenshot to: ${REF_PNG}"
        fi
    else
        echo "C Ironwail binary not found at ${IRONWAIL_C} - skipping C capture"
    fi

    echo "Capturing Go screenshot for ${MAP_NAME}..."
    "${IRONWAIL_GO}" -basedir "${QUAKE_DIR}" -screenshot "${GO_PNG}" -width "${WIDTH}" -height "${HEIGHT}" -headless +map "${MAP_NAME}" >/dev/null 2>&1 || true

    if [ -f "${REF_PNG}" ] && [ -f "${GO_PNG}" ]; then
        echo "Comparing reference vs Go captures..."
        if command -v compare >/dev/null 2>&1; then
            compare -metric SSIM "${REF_PNG}" "${GO_PNG}" "${DIFF_PNG}" 2>&1 || true
            echo "Generated diff image: ${DIFF_PNG}"
        fi
    fi
else
    echo "Notice: ${MAP_NAME}.bsp not found in game or testdata directories."
    echo "Running synthetic multi-liquid transparency test suite..."
    (cd "${ROOT_DIR}" && TMPDIR="${ROOT_DIR}/.tmp" CGO_ENABLED=0 go test -v ./internal/renderer -run TestSyntheticMultiLiquidTransparencyParity -count=1)
fi

echo
echo "=== Transparency Parity Harness Finished ==="
