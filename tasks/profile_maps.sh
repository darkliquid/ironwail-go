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

# Number of steady-state frames captured per map (matches the console samples).
PERF_FRAMES="${PERF_FRAMES:-240}"
# Warmup frames before measurement.
WARMUP_FRAMES="${WARMUP_FRAMES:-240}"

RESULTS_CSV="${REPO_ROOT}/.tmp/profiles/steady_state_summary.csv"

declare -a MAPS_PLAYERS=( "qbj2 qbj2_zetabyt" "qbj3 qbj3_stickflip" "qbj3 qbj3_softi" )

run_map_profile() {
  local game="$1"
  local map_name="$2"
  local out_prefix="${REPO_ROOT}/.tmp/profiles/${game}_${map_name}"
  local log="${out_prefix}_steadystate.log"

  echo "==> Profiling map ${game}/${map_name}..."

  # Generate one +wait per frame so the warmup and capture windows have time
  # to elapse: each wait consumes one engine frame (250 Hz headless tick).
  # The capture must be issued while the warmup session is still active, so
  # the warmup wait window is slightly shorter than the warmup itself.
  local warmup_waits capture_waits
  warmup_waits=$((WARMUP_FRAMES - 4))
  [ "${warmup_waits}" -lt 1 ] && warmup_waits=1
  capture_waits=$((PERF_FRAMES + 8))
  local -a run_args=(
    -basedir "${QUAKE_DIR}"
    -game "${game}"
    -headless
    +profile_cpu_start "${out_prefix}_cpu.pprof"
    +map "${map_name}"
    +wait +wait +wait +wait
    +perf_warmup "${WARMUP_FRAMES}"
  )
  for _ in $(seq 1 "${warmup_waits}"); do run_args+=( +wait ); done
  run_args+=(
    +perf_capture "${PERF_FRAMES}"
  )
  for _ in $(seq 1 "${capture_waits}"); do run_args+=( +wait ); done
  run_args+=(
    +profile_dump_heap "${out_prefix}_heap.pprof"
    +profile_cpu_stop
    +quit
  )

  # Launch engine in headless mode: capture CPU and heap pprofs, then run a
  # warmup + steady-state per-frame allocation measurement session and print a
  # machine-readable PERF_RESULT line to the console.
  "${REPO_ROOT}/ironwailgo" "${run_args[@]}" >"${log}" 2>&1 || true

  echo "    Saved CPU profile:   ${out_prefix}_cpu.pprof"
  echo "    Saved Heap profile:  ${out_prefix}_heap.pprof"
  echo "    Saved steady-state:  ${log}"

  # Extract the PERF_RESULT line emitted when the capture window completes.
  local perf_line
  perf_line="$(grep 'PERF_RESULT' "${log}" | tail -1 || true)"
  if [ -z "${perf_line}" ]; then
    echo "    WARNING: no PERF_RESULT line found in ${log}" >&2
    return 1
  fi
  echo "    ${perf_line}"
  echo "${game}/${map_name},${perf_line#PERF_RESULT }" >>"${RESULTS_CSV}"
}

# Append CSV header only when the file is new.
if [ ! -f "${RESULTS_CSV}" ]; then
  echo "map,frame_budget avg_alloc avg_objects max_alloc_frame max_objects_frame samples wall_seconds" >"${RESULTS_CSV}"
fi

for entry in "${MAPS_PLAYERS[@]}"; do
  # shellcheck disable=SC2086
  run_map_profile $entry || true
done

echo ""
echo "==> Steady-state per-frame allocation summary (${RESULTS_CSV}):"
column -s, -t <"${RESULTS_CSV}" 2>/dev/null || cat "${RESULTS_CSV}"

echo ""
echo "==> Whole-run heap allocation totals (via pprof):"
for heap in "${REPO_ROOT}"/.tmp/profiles/*_heap.pprof; do
  [ -e "${heap}" ] || continue
  echo "  ${heap}: $(go tool pprof -top -nodecount=5 -alloc_objects "${heap}" 2>/dev/null | awk '/flat/{getline; print $4, $5}' | head -1)"
done

echo ""
echo "==> Profiling complete. Profiles available in .tmp/profiles/"
