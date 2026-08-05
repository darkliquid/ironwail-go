#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

mkdir -p "${REPO_ROOT}/.tmp/profiles"

QUAKE_DIR="${QUAKE_DIR:-${REPO_ROOT}/quake-data}"

if [ ! -d "${QUAKE_DIR}" ]; then
  echo "Error: QUAKE_DIR (${QUAKE_DIR}) does not exist" >&2
  exit 1
fi

echo "==> Building ironwailgo binary..."
(cd "${REPO_ROOT}" && mise run build)

run_map_profile() {
  local game="$1"
  local map_name="$2"
  local out_prefix="${REPO_ROOT}/.tmp/profiles/${game}_${map_name}"

  echo "==> Profiling map ${game}/${map_name}..."
  
  # Launch engine in headless mode with CPU profiling console commands queued
  "${REPO_ROOT}/ironwailgo" \
    -basedir "${QUAKE_DIR}" \
    -game "${game}" \
    -headless \
    +profile_cpu_start "${out_prefix}_cpu.pprof" \
    +map "${map_name}" \
    +wait \
    +wait \
    +wait \
    +wait \
    +profile_dump_heap "${out_prefix}_heap.pprof" \
    +profile_cpu_stop \
    +quit || true

  echo "    Saved CPU profile: ${out_prefix}_cpu.pprof"
  echo "    Saved Heap profile: ${out_prefix}_heap.pprof"
}

run_map_profile "qbj2" "qbj2_zetabyt"
run_map_profile "qbj3" "qbj3_stickflip"

echo "==> Profiling complete. Profiles available in .tmp/profiles/"
