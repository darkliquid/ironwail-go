#!/usr/bin/env bash
# doc_check.sh — plan 26.4: status-consistency + stale-reference lint for docs/.
#
# Fail-fast rules (exit 1 on any):
#   1. Every docs/plans/*.md (non-archive) has a "**Status**:" line whose value
#      is one of {PLANNED, DONE, COMPLETED, SUPERSEDED, APPROVED}.
#   2. No docs/**/*.md references symbols removed from the codebase
#      (syncAllToQCVM, syncAllFromQCVM, capturePusherSnapshots,
#      syncPushersToQCVM, syncMutatedPushersFromQCVM,
#      captureNonPusherQCVMEdictSnapshots, syncMutatedNonPushersFromQCVM,
#      syncEntVarsFromQC, syncEntVarsToQC).
#   3. No docs/**/*.md references the forbidden build tags
#      (-tags gogpu, -tags opengl, go:build gogpu, go:build opengl).
#
# Warn-only rules (exit 0, printed):
#   4. docs/**/*.md file:line anchors that point at files that don't exist
#      (heuristic: backtick paths ending in .go).
#
# Usage: bash tasks/doc_check.sh   (or: mise run doc-check)

cd "$(dirname "$0")/.." || exit 2

root="$(pwd)"
status_fail=0
stale_fail=0
missing_anchor_warn=0

echo "== doc_check: plans status lines =="
for f in "$root"/docs/plans/*.md; do
	[ -e "$f" ] || continue
	base="$(basename "$f")"
	[ "$base" = "README.md" ] && continue
	status="$(grep -m1 '^\*\*Status\*\*' "$f" | sed -E 's/^\*\*Status\*\*:[[:space:]]*//; s/[[:space:]]*$//')"
	if [ -z "$status" ]; then
		echo "FAIL: $base has no **Status**: line"
		status_fail=1
		continue
	fi
	case "$status" in
	PLANNED|DONE|COMPLETED|SUPERSEDED|APPROVED) ;;
	*)
		echo "FAIL: $base status '$status' not in {PLANNED,DONE,COMPLETED,SUPERSEDED,APPROVED}"
		status_fail=1
		;;
	esac
done

echo "== doc_check: removed symbols (live docs only; diagnoses/ + archive/ are historical) =="
removed_symbols=(
	syncAllToQCVM syncAllFromQCVM capturePusherSnapshots
	syncPushersToQCVM syncMutatedPushersFromQCVM
	captureNonPusherQCVMEdictSnapshots syncMutatedNonPushersFromQCVM
	syncEntVarsFromQC syncEntVarsToQC
)
for sym in "${removed_symbols[@]}"; do
	# Match only backticked code references (\`sym\`) — plain-noun mentions in
	# historical prose (diagnoses, the sync-history section of
	# QCVM_ENTITY_SYNC.md) are intentional. A backticked reference means the
	# doc treats it as a live symbol.
	hits=$(grep -rln -- "\`$sym\`" "$root"/docs/ 2>/dev/null \
		| grep -v 'plans/26_docs_consolidation.md' \
		| grep -v 'plans/22_browser_engine_walkthrough.md' \
		| grep -v 'plans/23_parity_hardening.md' \
		| grep -v 'plan/refactor_plan_v2.md' \
		| grep -v 'docs/diagnoses/' \
		| grep -v 'docs/plans/archive/' \
		| grep -v 'docs/ENGINE_ROADMAP_PLAN.md' `# historic roadmap describing the sync-removal plan` \
		| grep -v 'docs/qbj2_zetabyt_investigation_log.md' `# historic perf diagnosis of the removed sync layer` \
		| grep -v 'docs/QUAKE_SPECIFICATION.md' || true)
	if [ -n "$hits" ]; then
		echo "FAIL: removed symbol '$sym' still referenced in:"
		echo "$hits" | sed 's/^/    /'
		stale_fail=1
	fi
done

echo "== doc_check: forbidden build tags (live docs only) =="
tag_hits=$(grep -rln -- '-tags gogpu\|-tags opengl\|go:build gogpu\|go:build opengl' "$root"/docs/ 2>/dev/null \
	| grep -v 'docs/diagnoses/' \
	| grep -v 'docs/plans/archive/' \
	| grep -v 'docs/plans/26_docs_consolidation.md' || true)
if [ -n "$tag_hits" ]; then
	echo "FAIL: forbidden build tags referenced in:"
	echo "$tag_hits" | sed 's/^/    /'
	stale_fail=1
fi

echo "== doc_check: file-anchor resolution (warn only) =="
# Heuristic: backticked `path/to/file.go` anchors; check the file exists.
while IFS=: read -r file line anchor; do
	[ -n "$anchor" ] || continue
	if [ ! -e "$root/$anchor" ]; then
		echo "WARN: $file:$line -> \`$anchor\` does not exist"
		missing_anchor_warn=1
	fi
done < <(grep -rnoE '`[a-zA-Z0-9_./-]+\.go`' "$root"/docs/ 2>/dev/null | sed -E 's/`//g')

if [ "$status_fail" -ne 0 ] || [ "$stale_fail" -ne 0 ]; then
	echo ""
	echo "doc_check: FAILED (status=$status_fail stale=$stale_fail)"
	exit 1
fi
echo "doc_check: OK (warnings=$missing_anchor_warn)"
exit 0
