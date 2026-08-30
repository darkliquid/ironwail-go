#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Resolve Quake game directory
QUAKE_DIR="${QUAKE_DIR:-/home/darkliquid/Games/Heroic/Quake Enhanced/}"
if [ ! -d "${QUAKE_DIR}" ] && [ -d "${ROOT_DIR}/quake-data" ]; then
    QUAKE_DIR="${ROOT_DIR}/quake-data"
fi

if [ ! -d "${QUAKE_DIR}" ]; then
    echo "ERROR: Quake data directory not found at '${QUAKE_DIR}'." >&2
    echo "Please set QUAKE_DIR or run 'mise run setup-symlinks'." >&2
    exit 1
fi

IRONWAIL_GO="${IRONWAIL_GO:-${ROOT_DIR}/ironwailgo}"
WIDTH="${TRACE_WIDTH:-1280}"
HEIGHT="${TRACE_HEIGHT:-720}"
OUTPUT_DIR="${ROOT_DIR}/traces/vulkan"
mkdir -p "${OUTPUT_DIR}"

# Build Go binary if missing
if [ ! -x "${IRONWAIL_GO}" ]; then
    echo "Building ironwailgo binary..."
    (cd "${ROOT_DIR}" && CGO_ENABLED=0 go build -o "${IRONWAIL_GO}" ./cmd/ironwailgo)
fi

# Determine map name
MAP_NAME="${1:-test_transparency}"
FRAMES="${2:-1-60}"

# Check map existence; fallback to start if test map is missing
MAP_FOUND=0
for search_path in \
    "${QUAKE_DIR}/id1/maps/${MAP_NAME}.bsp" \
    "${QUAKE_DIR}/maps/${MAP_NAME}.bsp" \
    "${ROOT_DIR}/testdata/maps/${MAP_NAME}.bsp" \
    "${ROOT_DIR}/testdata/parity/${MAP_NAME}.bsp"; do
    if [ -f "${search_path}" ]; then
        MAP_FOUND=1
        break
    fi
done

if [ "${MAP_FOUND}" -eq 0 ] && [ "${MAP_NAME}" = "test_transparency" ]; then
    echo "Notice: '${MAP_NAME}.bsp' not found in game dir. Falling back to map 'start'."
    MAP_NAME="start"
fi

echo "=== Vulkan GPU Trace Capture Harness ==="
echo "Quake Data Dir: ${QUAKE_DIR}"
echo "Ironwail Go:    ${IRONWAIL_GO}"
echo "Map:            ${MAP_NAME}"
echo "Frames:         ${FRAMES}"
echo "Output Dir:     ${OUTPUT_DIR}"
echo

# Locate Vulkan capture tool
GFXRECON_BIN=""
if command -v gfxrecon-capture-vulkan >/dev/null 2>&1; then
    GFXRECON_BIN="$(command -v gfxrecon-capture-vulkan)"
elif command -v gfxrecon-capture.py >/dev/null 2>&1; then
    GFXRECON_BIN="$(command -v gfxrecon-capture.py)"
fi

RENDERDOC_BIN=""
if command -v renderdoccmd >/dev/null 2>&1; then
    RENDERDOC_BIN="$(command -v renderdoccmd)"
fi

if [ -n "${GFXRECON_BIN}" ]; then
    OUTPUT_FILE="${3:-${OUTPUT_DIR}/${MAP_NAME}.gfxr}"
    echo "Using gfxrecon: ${GFXRECON_BIN}"
    echo "Capture target: ${OUTPUT_FILE}"

    if [ -d "/usr/share/vulkan/explicit_layer.d" ] && [ -z "${VK_LAYER_PATH:-}" ]; then
        export VK_LAYER_PATH="/usr/share/vulkan/explicit_layer.d"
    fi

    # Execute trace capture
    "${GFXRECON_BIN}" \
        -o "${OUTPUT_FILE}" \
        -f "${FRAMES}" \
        --no-file-timestamp \
        "${IRONWAIL_GO}" \
        -basedir "${QUAKE_DIR}" \
        -width "${WIDTH}" \
        -height "${HEIGHT}" \
        -screenshot "${OUTPUT_DIR}/${MAP_NAME}_screenshot.png" \
        -headless \
        +map "${MAP_NAME}" || true

    if [ -f "${OUTPUT_FILE}" ]; then
        echo "Trace successfully saved to: ${OUTPUT_FILE}"
        if command -v gfxrecon-convert >/dev/null 2>&1; then
            JSON_OUT="${OUTPUT_FILE%.*}.jsonl"
            echo "Converting trace to JSON lines: ${JSON_OUT}"
            gfxrecon-convert "${OUTPUT_FILE}" --format jsonl --output "${JSON_OUT}" || true
        fi
    else
        echo "Warning: Trace file was not created at ${OUTPUT_FILE} (layer may require non-headless or active Vulkan instance)"
    fi

elif [ -n "${RENDERDOC_BIN}" ]; then
    OUTPUT_FILE="${3:-${OUTPUT_DIR}/${MAP_NAME}}"
    echo "Using RenderDoc: ${RENDERDOC_BIN}"
    echo "Capture target: ${OUTPUT_FILE}.rdc"

    "${RENDERDOC_BIN}" capture \
        -c "${OUTPUT_FILE}" \
        -w \
        "${IRONWAIL_GO}" \
        -basedir "${QUAKE_DIR}" \
        -width "${WIDTH}" \
        -height "${HEIGHT}" \
        -screenshot "${OUTPUT_DIR}/${MAP_NAME}_screenshot.png" \
        -headless \
        +map "${MAP_NAME}" || true

    if [ -f "${OUTPUT_FILE}.rdc" ]; then
        echo "RenderDoc capture saved to: ${OUTPUT_FILE}.rdc"
    fi
else
    echo "ERROR: Neither gfxrecon (gfxrecon-capture-vulkan / gfxrecon-capture.py) nor RenderDoc (renderdoccmd) was found in PATH." >&2
    echo "Please install gfxreconstruct or renderdoc to capture Vulkan traces." >&2
    exit 1
fi

echo
echo "=== Vulkan Trace Capture Completed ==="
