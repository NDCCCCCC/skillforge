#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-26.sh — Phase 26 structural hygiene gates (HYG-01).
#
# Three source-grep structural locks over the non-test Go tree, in the
# verify-phase-15/16 run_stage form. The gate is HERMETIC: grep only, no
# Go toolchain, no network.
#
#   Stage 01  todo-fixme-zero            — only TODO( / TODO: / FIXME( / FIXME:
#                                          count as a TODO. The repo-wide comment
#                                          convention is that a transient note is
#                                          written NOTE(...): and a bare TODO token
#                                          never counts, so the eino type name
#                                          deep.TODO (internal/agent/deep_audit.go,
#                                          internal/store/sqlite/schema.go,
#                                          internal/wire/chunk.go) measures ZERO by
#                                          construction — verified during 26-10.
#   Stage 02  tierenum-zero              — the Phase 26 enum deletion evidence:
#                                          the TierEnum identifier must not appear in
#                                          any non-test Go file (test files may keep
#                                          the zero-assertion itself).
#   Stage 03  cn-tier-literal-scoped-ban — CN tier vocabulary is retired from the
#                                          DATA layer: it may survive as prose
#                                          (comments, natural-language strings, UI
#                                          display names) but never as a quoted data
#                                          value in non-comment Go code.
#
# Stage 03 matching rule (Pitfall 5 anti-false-positive design):
#   a line is a violation only when it is NOT a comment line AND it contains
#   the CN tier word as an EXACT whole string literal ("权威" / "历史" /
#   "个人" / "草稿"). Prose such as "[历史结论不可解析]" or the platform
#   floor's "个人/PII 信息" is natural language, not a tier value, and never
#   matches. A code line whose TRAILING comment names a literal is still
#   flagged on purpose: the fix is to reword, the value vocabulary belongs to
#   registry data.
#
# Stage 03 whitelist (grep -v path filters — each entry needs a reason and a
# review date; an unjustified entry is how a hygiene gate dies):
#   - internal/platformfloor/platform_floor.go
#       prompt prose: "个人/PII 信息" (:111) and "历史回复" (:141-142) are
#       natural language inside the frozen prompt floor, not tier values.
#       The file is byte-frozen by D14-B4 — do NOT reword it to satisfy this
#       gate. 复核 2027-03.
#   - internal/history/verdict_renderer.go
#       display sentinel "[历史结论不可解析]" (:31) is user-facing prose,
#       not a tier value. 复核 2027-03.
#
# D8-04 negative-proof discipline: the gates were proven RED on the
# pre-cleanup tree (Stage 01 on cmd/spike-history/long.go:54; Stages 01/02/03
# on injected probes) BEFORE the cleanup commit turned them GREEN. A gate that
# has never been red proves nothing.
#
# Operator entry point:
#   bash scripts/verify-phase-26.sh
#   # or equivalently:
#   make verify-phase-26
#
# On full pass:  prints "PHASE 26 STRUCTURAL GATES: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL]" + the hit list, exits 1.

set -euo pipefail

stage_num=0

# run_stage executes one stage, captures stdout+stderr, and exits 1 with the
# [STAGE-NN-FAIL] prefix on failure (verify-phase-16.sh form).
run_stage() {
  local name="$1"
  shift
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-26-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -30 "${log_file}" | sed 's/^/    /'
  echo "PHASE 26 STRUCTURAL GATES: FAIL — see stage ${nn} output" >&2
  exit 1
}

cleanup() {
  rm -f /tmp/phase-26-stage-*.log 2>/dev/null || true
}
trap cleanup EXIT

echo "Phase 26 Structural Hygiene Gates (HYG-01) — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Scope: non-test Go sources under internal/ and cmd/"
echo

# ---------------------------------------------------------------------------
# Stage 01 — todo-fixme-zero (gate 1)
# ---------------------------------------------------------------------------
# Only TODO( / TODO: / FIXME( / FIXME: is a TODO. Whole-word matches would
# false-positive on the eino type name deep.TODO (measured: 4 reference sites,
# 0 gate hits) and would push the team toward noqa-style bypasses.
# The hit list is printed on failure so the offending file:line is immediate.

run_stage "todo-fixme-zero (only TODO(/TODO:/FIXME(/FIXME: count; deep.TODO measures 0)" \
  bash -c '
    hits=$(grep -rnIE "TODO\(|TODO:|FIXME\(|FIXME:" --include="*.go" internal cmd 2>/dev/null | grep -v "_test.go" || true)
    if [ -n "${hits}" ]; then
      echo "FATAL: TODO/FIXME remains in non-test Go sources (HYG-01). Hit list:"
      echo "${hits}"
      echo "Convention: transient notes are written NOTE(...): — only TODO( / TODO: / FIXME( / FIXME: are gate-relevant."
      exit 1
    fi
    # Anti-false-positive proof: the eino type name deep.TODO must never match.
    deep_hits=$(grep -rn "deep.TODO" --include="*.go" internal cmd 2>/dev/null | grep -v "_test.go" | grep -cE "TODO\(|TODO:" || true)
    if [ "${deep_hits}" != "0" ]; then
      echo "FATAL: gate 1 regex unexpectedly matches a deep.TODO reference — tighten the pattern, do not whitelist the file."
      exit 1
    fi
    echo "PASS — TODO/FIXME count is 0; deep.TODO references still present but correctly not matched"
  '

# ---------------------------------------------------------------------------
# Stage 02 — tierenum-zero (gate 2)
# ---------------------------------------------------------------------------
# The TierEnum identifier must be gone from non-test Go entirely: this is the
# grep-level complement to the enum deletion (the compile no longer sees it;
# this proves no straggler reference or comment-level resurrection remains).
# Test files are exempt so the zero-assertion itself can name the identifier.

run_stage "tierenum-zero (TierEnum identifier deleted from non-test Go)" \
  bash -c '
    hits=$(grep -rn "TierEnum" --include="*.go" internal cmd 2>/dev/null | grep -v "_test.go" || true)
    if [ -n "${hits}" ]; then
      echo "FATAL: TierEnum identifier remains in non-test Go sources. Hit list:"
      echo "${hits}"
      exit 1
    fi
    echo "PASS — TierEnum identifier count is 0 in non-test Go"
  '

# ---------------------------------------------------------------------------
# Stage 03 — cn-tier-literal-scoped-ban (gate 3)
# ---------------------------------------------------------------------------
# CN tier vocabulary is retired from the DATA layer. A violation is a
# non-comment Go line carrying one of the four CN tier words as an EXACT
# whole string literal — i.e. a data value. Two prose sites are whitelisted
# above with reasons and a review date; anything else must be migrated to
# registry slugs (or, if it is genuinely prose, moved into a comment where it
# belongs).

run_stage "cn-tier-literal-scoped-ban (exact quoted CN tier values in non-comment Go)" \
  bash -c '
    hits=$(grep -rnIE "(权威|历史|个人|草稿)" --include="*.go" internal cmd 2>/dev/null \
      | grep -v "_test.go" \
      | grep -vE ":[0-9]+:[[:space:]]*(//|\*|/\*|--)" \
      | grep -E "\"(权威|历史|个人|草稿)\"" \
      | grep -v "^internal/platformfloor/platform_floor.go:" \
      | grep -v "^internal/history/verdict_renderer.go:" || true)
    if [ -n "${hits}" ]; then
      echo "FATAL: CN tier literal used as a data value in non-test Go. Hit list:"
      echo "${hits}"
      echo "Migrate the value to a registry slug (authority/historical/personal/draft)."
      echo "Prose exemptions live in this script with reasons + review date (复核 2027-03)."
      exit 1
    fi
    echo "PASS — 0 CN tier data values outside the two annotated prose whitelists"
  '

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 26 STRUCTURAL GATES: PASS"
echo "==================================================================="
echo "All ${stage_num} structural locks hold on this tree:"
echo "  - Stage 01  TODO/FIXME-zero (only TODO(/TODO:/FIXME(/FIXME: count)"
echo "  - Stage 02  TierEnum identifier deleted from non-test Go"
echo "  - Stage 03  CN tier literal banned as a data value (2 annotated prose"
echo "              whitelists: platform_floor.go + verdict_renderer.go,"
echo "              复核 2027-03)"
echo
exit 0
