#!/usr/bin/env bash
# perf-gate.sh — steady-state per-frame allocation regression gate (plan 20.5).
#
# Runs profile_maps and compares each map's mean per-frame alloc bytes and
# object counts against a recorded reference CSV. Fails (nonzero) when:
#   - any map's mean exceeds the reference by more than the tolerance, or
#   - the capture produced no PERF_RESULT (measurement broke), or
#   - any single allocation site accounts for >10% of the window delta
#     (no dominating per-frame allocator).
#
# The reference file is `steady_state_summary.csv`; snapshot it with:
#   cp .tmp/profiles/steady_state_summary.csv tasks/perf_gate_reference.csv

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOLERANCE="${PERF_GATE_TOLERANCE:-20}"   # percent over reference allowed
SITE_LIMIT="${PERF_GATE_SITE_LIMIT:-10}" # percent of window delta a single site may own
RESULTS_CSV="${REPO_ROOT}/.tmp/profiles/steady_state_summary.csv"
REFERENCE_CSV="${REPO_ROOT}/tasks/perf_gate_reference.csv"

if [ ! -f "${REFERENCE_CSV}" ]; then
  echo "perf-gate: no reference file ${REFERENCE_CSV}" >&2
  echo "  Record one after a known-good full run:" >&2
  echo "  cp .tmp/profiles/steady_state_summary.csv tasks/perf_gate_reference.csv" >&2
  exit 1
fi

"${REPO_ROOT}/tasks/profile_maps.sh" || {
  echo "perf-gate: profile_maps.sh failed" >&2
  exit 1
}

fail=0
while IFS=, read -r map rest; do
  [ "${map}" = "map" ] && continue
  [ -z "${map}" ] && continue
  avg_alloc="$(echo "${rest}" | sed -n 's/.*avg_alloc \([0-9]*\).*/\1/p')"
  avg_objects="$(echo "${rest}" | sed -n 's/.*avg_objects \([0-9]*\).*/\1/p')"
  if [ -z "${avg_alloc}" ] || [ -z "${avg_objects}" ]; then
    echo "perf-gate: ${map}: no PERF_RESULT line (measurement broke)" >&2
    fail=1
    continue
  fi

  ref_line="$(grep "^${map}," "${REFERENCE_CSV}" | tail -1 || true)"
  if [ -z "${ref_line}" ]; then
    echo "perf-gate: ${map}: no reference row; skipping" >&2
    continue
  fi
  ref_alloc="$(echo "${ref_line}" | sed -n 's/.*avg_alloc \([0-9]*\).*/\1/p')"
  ref_objects="$(echo "${ref_line}" | sed -n 's/.*avg_objects \([0-9]*\).*/\1/p')"

  alloc_pct=0; obj_pct=0
  [ "${ref_alloc}" -gt 0 ] && alloc_pct=$(( (avg_alloc - ref_alloc) * 100 / ref_alloc ))
  [ "${ref_objects}" -gt 0 ] && obj_pct=$(( (avg_objects - ref_objects) * 100 / ref_objects ))

  echo "perf-gate: ${map}: alloc ${avg_alloc} (${alloc_pct}% vs ref ${ref_alloc}), objects ${avg_objects} (${obj_pct}% vs ref ${ref_objects})"
  if [ "${alloc_pct}" -gt "${TOLERANCE}" ] || [ "${obj_pct}" -gt "${TOLERANCE}" ]; then
    echo "perf-gate: ${map}: EXCEEDS ${TOLERANCE}% tolerance" >&2
    fail=1
  fi

  # Site-dominance check: the per-map steady-state delta report must not show
  # a single site owning >SITE_LIMIT% of the window delta.
  delta_txt="${REPO_ROOT}/.tmp/profiles/$(echo "${map}" | tr '/' '_')_steadystate_delta.txt"
  if [ -f "${delta_txt}" ]; then
    # pprof top output: an awk-filtered single dominant flat% would appear in
    # the first data row with > SITE_LIMIT%.
    dom="$(sed -n '7p' "${delta_txt}" | awk -v lim="${SITE_LIMIT}" '$1+0 > lim {print $1; exit}' 2>/dev/null || true)"
    if [ -n "${dom}" ]; then
      echo "perf-gate: ${map}: dominant site ${dom}% of window delta exceeds ${SITE_LIMIT}% limit" >&2
      fail=1
    fi
  fi
done <"${RESULTS_CSV}"

if [ "${fail}" -ne 0 ]; then
  echo "perf-gate: FAILED" >&2
  exit 1
fi
echo "perf-gate: PASS"
