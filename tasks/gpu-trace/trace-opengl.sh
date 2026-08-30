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

# Resolve C Ironwail binary
IRONWAIL_C="${IRONWAIL_BIN:-${ROOT_DIR}/ironwail/Linux/ironwail}"
if [ ! -x "${IRONWAIL_C}" ] && [ -x "${ROOT_DIR}/ironwail" ]; then
    IRONWAIL_C="${ROOT_DIR}/ironwail"
fi

if [ ! -x "${IRONWAIL_C}" ]; then
    echo "ERROR: C Ironwail binary not found at '${IRONWAIL_C}'." >&2
    echo "Please build C Ironwail or set IRONWAIL_BIN." >&2
    exit 1
fi

WIDTH="${TRACE_WIDTH:-1280}"
HEIGHT="${TRACE_HEIGHT:-720}"
TIMEOUT="${TRACE_TIMEOUT:-5}"
OUTPUT_DIR="${ROOT_DIR}/traces/opengl"
mkdir -p "${OUTPUT_DIR}"

# Determine map name
MAP_NAME="${1:-test_transparency}"

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

echo "=== OpenGL GPU Trace Capture Harness ==="
echo "Quake Data Dir: ${QUAKE_DIR}"
echo "Ironwail C:     ${IRONWAIL_C}"
echo "Map:            ${MAP_NAME}"
echo "Timeout:        ${TIMEOUT}s"
echo "Output Dir:     ${OUTPUT_DIR}"
echo

# Locate OpenGL capture tool
APITRACE_BIN=""
if command -v apitrace >/dev/null 2>&1; then
    APITRACE_BIN="$(command -v apitrace)"
fi

RENDERDOC_BIN=""
if command -v renderdoccmd >/dev/null 2>&1; then
    RENDERDOC_BIN="$(command -v renderdoccmd)"
fi

if [ -n "${APITRACE_BIN}" ]; then
    OUTPUT_FILE="${2:-${OUTPUT_DIR}/${MAP_NAME}.trace}"
    echo "Using apitrace: ${APITRACE_BIN}"
    echo "Capture target: ${OUTPUT_FILE}"

    # Execute trace capture under timeout
    timeout "${TIMEOUT}" "${APITRACE_BIN}" trace \
        -o "${OUTPUT_FILE}" \
        "${IRONWAIL_C}" \
        -basedir "${QUAKE_DIR}" \
        -window \
        -width "${WIDTH}" \
        -height "${HEIGHT}" \
        +map "${MAP_NAME}" || true

    if [ -f "${OUTPUT_FILE}" ]; then
        echo "Trace successfully saved to: ${OUTPUT_FILE}"
        echo "Trace info:"
        "${APITRACE_BIN}" info "${OUTPUT_FILE}" || true
    else
        echo "Warning: Trace file was not created at ${OUTPUT_FILE}"
    fi

elif [ -n "${RENDERDOC_BIN}" ]; then
    OUTPUT_FILE="${2:-${OUTPUT_DIR}/${MAP_NAME}}"
    echo "Using RenderDoc: ${RENDERDOC_BIN}"
    echo "Capture target: ${OUTPUT_FILE}.rdc"

    timeout "${TIMEOUT}" "${RENDERDOC_BIN}" capture \
        -c "${OUTPUT_FILE}" \
        -w \
        "${IRONWAIL_C}" \
        -basedir "${QUAKE_DIR}" \
        -window \
        -width "${WIDTH}" \
        -height "${HEIGHT}" \
        +map "${MAP_NAME}" || true

    if [ -f "${OUTPUT_FILE}.rdc" ]; then
        echo "RenderDoc capture saved to: ${OUTPUT_FILE}.rdc"
    fi
else
    echo "ERROR: Neither apitrace nor RenderDoc (renderdoccmd) was found in PATH." >&2
    echo "Please install apitrace or renderdoc to capture OpenGL traces." >&2
    exit 1
fi

echo
echo "=== OpenGL Trace Capture Completed ==="
