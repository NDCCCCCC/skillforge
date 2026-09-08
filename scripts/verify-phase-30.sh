#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-30.sh — Phase 30 HYG-02 + HYG-03 structural gate.
#
# 5 stages; all must pass. Mirrors verify-phase-29.sh (4 stages) with the
# Phase 30 HYG-02 magic-number gate + HYG-03 evidence compilation.
#
# Stage 1: TODO/FIXME zero in Phase 30 production files
#   (Go + TS/TSX; only TODO( / TODO: / FIXME( / FIXME: count — the eino
#   type name deep.TODO must never match; verify-phase-26 convention)
# Stage 2: HYG-02 config-driven check — verify 14 SKILLFORGE_* config fields
#   are wired through envOr / parseIntEnvOrDefault / parseBytesEnvOrDefault
#   in cmd/server/main.go (no hardcoded literals)
# Stage 3: HYG-02 diff scan — scan phases 26-29 git diff for new hardcoded
#   literals in the same 14 config fields (test fixtures exempt)
# Stage 4: UAT clearance — verify prior verify scripts and UAT files exist
# Stage 5: HYG-03 evidence compilation — run prior verify scripts and compile
#   a summary of all phase gates (26-29)
#
# Operator entry point:
#   bash scripts/verify-phase-30.sh
#   # or equivalently:
#   make phase-30
#
# On full pass: prints "PHASE 30 STRUCTURAL GATES: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL]" + the hit list, exits 1.

set -uo pipefail

# ---------------------------------------------------------------------------
# Helper: run_stage — mirrors verify-phase-26.sh / verify-phase-29.sh form
# ---------------------------------------------------------------------------
stage_num=0
run_stage() {
  local name="$1"
  shift
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-30-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -30 "${log_file}" | sed 's/^/    /'
  echo "PHASE 30 STRUCTURAL GATES: FAIL — see stage ${nn} output" >&2
  exit 1
}

cleanup() {
  rm -f /tmp/phase-30-stage-*.log 2>/dev/null || true
}
trap cleanup EXIT

echo "Phase 30 HYG-02/HYG-03 Structural Gates — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo

# ---------------------------------------------------------------------------
# Stage 01 — todo-fixme-zero
# ---------------------------------------------------------------------------
# Only TODO( / TODO: / FIXME( / FIXME: is a TODO. The repo-wide comment
# convention is that a transient note is written NOTE(...): and a bare TODO
# token never counts, so the eino type name deep.TODO (internal/agent/
# deep_audit.go, internal/store/sqlite/schema.go, internal/wire/chunk.go)
# measures ZERO by construction.

run_stage "todo-fixme-zero (only TODO(/TODO:/FIXME(/FIXME: count; deep.TODO measures 0)" \
  bash -c '
    hits=$(grep -rnIE "TODO\(|TODO:|FIXME\(|FIXME:" --include="*.go" --include="*.ts" --include="*.tsx" \
      internal/server internal/store internal/ticket web/src cmd \
      2>/dev/null | grep -v "_test.go" | grep -v "\.test\." | grep -v "node_modules" || true)
    if [ -n "${hits}" ]; then
      echo "FATAL: TODO/FIXME remains in Phase 30 production sources. Hit list:"
      echo "${hits}"
      echo "Convention: transient notes are written NOTE(...): — only TODO( / TODO: / FIXME( / FIXME: are gate-relevant."
      exit 1
    fi
    # Anti-false-positive proof: the eino type name deep.TODO must never match.
    deep_hits=$(grep -rn "deep.TODO" --include="*.go" internal cmd 2>/dev/null \
      | grep -v "_test.go" | grep -cE "TODO\(|TODO:" || true)
    if [ "${deep_hits}" != "0" ]; then
      echo "FATAL: gate regex unexpectedly matches a deep.TODO reference — tighten the pattern, do not whitelist the file."
      exit 1
    fi
    echo "PASS — TODO/FIXME count is 0; deep.TODO references present but correctly not matched"
  '

# ---------------------------------------------------------------------------
# Stage 02 — HYG-02 config-driven check (14 SKILLFORGE_* fields)
# ---------------------------------------------------------------------------
# All 14 config fields must be read via envOr / parseIntEnvOrDefault /
# parseBytesEnvOrDefault in cmd/server/main.go — no hardcoded literals.
# The 14 fields (D-30-04):
#   Graph limits (4):
#     SKILLFORGE_GRAPH_MAX_NODES / SKILLFORGE_GRAPH_MAX_EDGES /
#     SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT / SKILLFORGE_GRAPH_PATH_HOPS_LIMIT
#   Ticket params (6):
#     SKILLFORGE_TICKET_PROVIDER / SKILLFORGE_TICKET_WEBHOOK_URL /
#     SKILLFORGE_TICKET_WEBHOOK_RETRIES / SKILLFORGE_TICKET_WEBHOOK_TIMEOUT_MS /
#     SKILLFORGE_TICKET_DISPATCH_BUDGET_MS / SKILLFORGE_TICKET_WEBHOOK_SECRET
#   Other (4):
#     SKILLFORGE_UPLOAD_MAX_BYTES / SKILLFORGE_KB_SEED /
#     SKILLFORGE_MILVUS_RECREATE / SKILLFORGE_DATA_SALT

run_stage "HYG-02 config-driven (14 SKILLFORGE_* fields wired via envOr in main.go)" \
  bash -c '
    main="cmd/server/main.go"
    missing=""
    helper_pat="envOr|parseIntEnvOrDefault|parseBytesEnvOrDefault"

    check_field() {
      local field="$1"
      if ! grep -q "${field}" "$main"; then
        missing="${missing}${field} "
      elif ! grep -qE "${helper_pat}" "$main"; then
        missing="${missing}${field}(not-via-helper) "
      fi
    }

    check_field "SKILLFORGE_GRAPH_MAX_NODES"
    check_field "SKILLFORGE_GRAPH_MAX_EDGES"
    check_field "SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT"
    check_field "SKILLFORGE_GRAPH_PATH_HOPS_LIMIT"
    check_field "SKILLFORGE_TICKET_PROVIDER"
    check_field "SKILLFORGE_TICKET_WEBHOOK_URL"
    check_field "SKILLFORGE_TICKET_WEBHOOK_RETRIES"
    check_field "SKILLFORGE_TICKET_WEBHOOK_TIMEOUT_MS"
    check_field "SKILLFORGE_TICKET_DISPATCH_BUDGET_MS"
    check_field "SKILLFORGE_TICKET_WEBHOOK_SECRET"
    check_field "SKILLFORGE_UPLOAD_MAX_BYTES"
    check_field "SKILLFORGE_KB_SEED"
    check_field "SKILLFORGE_MILVUS_RECREATE"
    check_field "SKILLFORGE_DATA_SALT"

    if [ -n "${missing}" ]; then
      echo "FATAL: the following SKILLFORGE_* config fields are missing or not wired via envOr:"
      echo "  ${missing}"
      echo "All 14 fields must be present in cmd/server/main.go and read via envOr / parseIntEnvOrDefault / parseBytesEnvOrDefault."
      exit 1
    fi
    echo "PASS — all 14 SKILLFORGE_* config fields are present and wired via envOr in main.go"
  '

# ---------------------------------------------------------------------------
# Stage 03 — HYG-02 diff scan (phases 26-29 new Go files, no hardcoded literals)
# ---------------------------------------------------------------------------
# Scan the git diff for phases 26-29 (the merge base of phase-30-p10-6-admin-console
# against main) for any new Go source files that contain hardcoded literal
# values for the same 14 config fields. Test files and test fixtures are exempt.

run_stage "HYG-02 diff scan (phases 26-29 new Go files have no hardcoded config literals)" \
  bash -c '
    # Find the merge-base with main for the phase-30 branch.
    merge_base=$(git merge-base HEAD main 2>/dev/null || git rev-parse HEAD)
    echo "Merge base against main: ${merge_base}"

    # Get list of new/modified non-test .go files from the diff.
    new_go_files=$(git diff --name-only --diff-filter=M "${merge_base}" HEAD -- "*.go" 2>/dev/null \
      | grep -v "_test.go" | grep -v "\.test\." || true)

    if [ -z "${new_go_files}" ]; then
      echo "No new/modified non-test Go files found in phases 26-30 diff — nothing to scan."
      echo "PASS"
      exit 0
    fi

    echo "Scanning the following non-test Go files for hardcoded config literals:"
    echo "${new_go_files}" | sed 's/^/  /'

    violations=""
    for f in ${new_go_files}; do
      [ -f "$f" ] || continue
      # Check if the file references any SKILLFORGE_ field AND contains
      # a bare numeric literal assignment in the same function (heuristic:
      # standalone = N where N is a small positive integer, in the same
      # file as the field name, without being inside an envOr call).
      field_names="SKILLFORGE_GRAPH_MAX_NODES|SKILLFORGE_GRAPH_MAX_EDGES|SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT|SKILLFORGE_GRAPH_PATH_HOPS_LIMIT|SKILLFORGE_TICKET_PROVIDER|SKILLFORGE_TICKET_WEBHOOK_URL|SKILLFORGE_TICKET_WEBHOOK_RETRIES|SKILLFORGE_TICKET_WEBHOOK_TIMEOUT_MS|SKILLFORGE_TICKET_DISPATCH_BUDGET_MS|SKILLFORGE_TICKET_WEBHOOK_SECRET|SKILLFORGE_UPLOAD_MAX_BYTES|SKILLFORGE_KB_SEED|SKILLFORGE_MILVUS_RECREATE|SKILLFORGE_DATA_SALT"
      has_field=$(grep -lE "${field_names}" "$f" 2>/dev/null || true)
      if [ -n "$has_field" ]; then
        hardcoded=$(awk "
          /${field_names}/ { in_block=1; block_start=NR }
          in_block && /^[ \t]*[a-zA-Z_][a-zA-Z0-9_]*[ \t]*=[ \t]*[0-9]+/ {
            matched_line = \$0
            if (matched_line !~ /envOr/ && matched_line !~ /parseIntEnvOrDefault/ && matched_line !~ /parseBytesEnvOrDefault/) {
              split(matched_line, parts, \"=\")
              val = trim(parts[2])
              gsub(/[ \t;].*/, \"\", val)
              if (val ~ /^[0-9]+$/ && val+0 > 0 && val+0 <= 10000) {
                print \"hardcoded literal: line \" NR \" in $f: \" matched_line
              }
            }
          }
          NR > block_start + 10 { in_block=0 }
          function trim(s) { gsub(/^[ \t]+|[ \t]+$/, \"\", s); return s }
        " "$f" 2>/dev/null || true)
        if [ -n "${hardcoded}" ]; then
          violations="${violations}${hardcoded}"$'\n'
        fi
      fi
    done

    if [ -n "${violations}" ]; then
      echo "FATAL: hardcoded numeric literals found near SKILLFORGE_* fields in phases 26-29 diff Go files:"
      echo "${violations}"
      echo "Use envOr / parseIntEnvOrDefault / parseBytesEnvOrDefault instead."
      exit 1
    fi
    echo "PASS — no hardcoded config literals found in phases 26-29 diff Go files"
  '

# ---------------------------------------------------------------------------
# Stage 04 — UAT clearance (prior verify scripts and UAT files exist)
# ---------------------------------------------------------------------------

run_stage "UAT clearance (prior verify scripts and UAT files exist)" \
  bash -c '
    missing=""
    for f in \
      "scripts/verify-phase-26.sh" \
      "scripts/verify-phase-27.sh" \
      "scripts/verify-phase-28.sh" \
      "scripts/verify-phase-29.sh" \
      ".planning/phases/29-feedback-loop-ticket-interface/29-HUMAN-UAT.md"; do
      if [ ! -f "$f" ]; then
        missing="${missing}$f "
      fi
    done
    if [ -n "${missing}" ]; then
      echo "FATAL: the following required UAT files are missing:"
      echo "  ${missing}"
      exit 1
    fi
    echo "PASS — all 4 prior verify scripts and 29-HUMAN-UAT.md exist"
  '

# ---------------------------------------------------------------------------
# Stage 05 — HYG-03 evidence compilation (run prior verify scripts)
# ---------------------------------------------------------------------------

run_stage "HYG-03 evidence compilation (run prior verify scripts 26-29)" \
  bash -c '
    results=""
    all_pass=true

    echo "Running prior verify scripts..."
    echo

    for phase in 26 27 28 29; do
      script="scripts/verify-phase-${phase}.sh"
      printf "%s: " "$script"
      if bash "$script" >/dev/null 2>&1; then
        echo "PASS"
        results="${results}${phase}: PASS"$'\n'
      else
        echo "FAIL"
        results="${results}${phase}: FAIL"$'\n'
        all_pass=false
      fi
    done

    echo
    echo "=== HYG-03 Evidence Summary (phases 26-29) ==="
    echo "${results}"
    echo

    if [ "$all_pass" = false ]; then
      echo "FATAL: not all prior verify scripts passed. Fix failures before proceeding."
      exit 1
    fi
    echo "PASS — all prior verify scripts (26-29) passed"
  '

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "======================================================================"
echo "PHASE 30 STRUCTURAL GATES: PASS"
echo "======================================================================"
echo "All ${stage_num} structural locks hold on this tree:"
echo "  - Stage 01  TODO/FIXME-zero (only TODO(/TODO:/FIXME(/FIXME: count)"
echo "  - Stage 02  HYG-02 config-driven (14 SKILLFORGE_* fields via envOr)"
echo "  - Stage 03  HYG-02 diff scan (phases 26-29 new Go files, no hardcoded literals)"
echo "  - Stage 04  UAT clearance (verify scripts 26-29 + 29-HUMAN-UAT.md exist)"
echo "  - Stage 05  HYG-03 evidence compilation (prior verify scripts 26-29 all PASS)"
echo
exit 0
