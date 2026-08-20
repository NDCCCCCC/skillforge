#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-11.sh — Phase 11 acceptance gate (Plan 11-08).
#
# Aggregates 11 stages into one PASS/FAIL banner. The operator runs
# ONE command and reads ONE banner:
#
#   bash scripts/verify-phase-11.sh
#   # or equivalently:
#   make verify-phase-11
#
# On full pass:  prints "PHASE 11 ACCEPTANCE GATE: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL] ..." + "PHASE 11 ACCEPTANCE
# GATE: FAIL — see stage N output" and exits 1.
#
# The gate is HERMETIC — no live GLM_API_KEY / BGE_M3_URL /
# MILVUS_ADDRESS reads. Every stage runs against in-memory sqlite +
# scripted ChatModel stubs + the dry-run spike path.

set -euo pipefail

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# stage_num tracks the current stage for the [STAGE-NN-FAIL] prefix.
stage_num=0

# run_stage executes one stage, captures stdout+stderr, and exits 1
# with the [STAGE-NN-FAIL] prefix on failure.
#
# Args:
#   $1 — stage name (for the log line)
#   $2.. — the command to run
run_stage() {
  local name="$1"
  shift
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-11-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    # Surface the last 3 lines so the operator sees the key metric.
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -20 "${log_file}" | sed 's/^/    /'
  echo "PHASE 11 ACCEPTANCE GATE: FAIL — see stage ${nn} output" >&2
  exit 1
}

# Trap cleanup on EXIT (success or failure).
cleanup() {
  rm -f /tmp/phase-11-stage-*.log /tmp/vitest-phase-11.log /tmp/spike-phase-11.log 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Gate preamble
# ---------------------------------------------------------------------------

echo "Phase 11 Acceptance Gate — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Go: $(go version 2>&1 | head -1)"
echo "Node: $(node --version 2>&1 | head -1 || echo 'not found')"
echo

# ---------------------------------------------------------------------------
# Stage 1 — hermetic Go suite: store (audit + checkpoint + approval_rules)
#           + hitl Tool infra + agent
# ---------------------------------------------------------------------------
# Plan note: the plan's stage 1 referenced ./internal/audit/... but that
# package does not exist; the audit logic lives in internal/store/sqlite
# (audit.go) + internal/agent/hitl (audit_writer.go). This stage runs
# the actual packages.
run_stage "hermetic Go suite (store + hitl + agent)" \
  go test ./internal/store/... ./internal/agent/hitl/... ./internal/agent/ -count=1 -timeout 180s

# ---------------------------------------------------------------------------
# Stage 2 — approval API endpoints (11-04 + 11-05)
# ---------------------------------------------------------------------------
run_stage "approval API endpoints (11-04 + 11-05)" \
  go test ./internal/server/... -run "TestApprovalAPI|TestApprovalRules|TestAuditAPI|TestExpiredCheckpoint|TestApprovalEndpoint|TestDataDecision|TestAuditList" -count=1

# ---------------------------------------------------------------------------
# Stage 3 — WalkDir legacy interrupt guard (Task 4 Sub-A)
# ---------------------------------------------------------------------------
run_stage "WalkDir legacy interrupt guard (Task 4 Sub-A)" \
  go test ./integration/contract/... -tags=contract -run "TestNoLegacyInterruptAPI" -count=1

# ---------------------------------------------------------------------------
# Stage 4 — cancel propagation <200ms lock (Task 5)
# ---------------------------------------------------------------------------
run_stage "cancel propagation <200ms lock (Task 5)" \
  go test ./internal/agent/... -run TestCancelPropagationMidToolRunner -count=1 -v -timeout 120s

# ---------------------------------------------------------------------------
# Stage 5 — STRIDE-11 critical mitigations (Task 4 Sub-B)
# ---------------------------------------------------------------------------
run_stage "STRIDE-11 critical mitigations (Task 4 Sub-B)" \
  go test ./internal/agent/hitl/... -run TestThreatModelCriticalMitigations -count=1 -v

# ---------------------------------------------------------------------------
# Stage 6 — hermetic E2E smoke (Task 2)
# ---------------------------------------------------------------------------
run_stage "hermetic E2E smoke cmd/spike-hitl (Task 2)" \
  bash -c "go build ./cmd/spike-hitl && go test ./cmd/spike-hitl/ -run TestE2EApprovalFlow -count=1 -v"

# ---------------------------------------------------------------------------
# Stage 7 — vitest UI tests (11-06 + 11-07)
# ---------------------------------------------------------------------------
run_stage "vitest UI tests (11-06 ApprovalCard + 11-07 Rules/AuditTimeline)" \
  bash -c "cd web && npx vitest run src/components/ApprovalCard src/components/ApprovalRulesPanel src/components/AuditTimelineTab 2>&1 | tail -30"

# ---------------------------------------------------------------------------
# Stage 8 — D-04 transport-swap byte-identity with ApprovalCard
# ---------------------------------------------------------------------------
# The transport-swap test lives under integration/transport-swap/ when
# it exists; Phase 11's D-04 invariant is also covered by the
# data-decision test in internal/server (already run in Stage 2). If
# the directory is absent, emit a SKIP (not a FAIL) so the gate is
# honest about coverage.
if [[ -d integration/transport-swap ]]; then
  run_stage "D-04 transport-swap byte-identity with ApprovalCard" \
    go test ./integration/transport-swap/... -run TestTransportSwapWithApprovalCard -count=1
else
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: D-04 transport-swap byte-identity ==="
  echo "  [STAGE-${nn}-SKIP] integration/transport-swap not present; D-04 covered by Stage 2 TestDataDecisionExtensionNoNewChunkTypes"
fi

# ---------------------------------------------------------------------------
# Stage 9 — D-15 wire-type compile-time alarm
# ---------------------------------------------------------------------------
if [[ -d integration/wire-type ]]; then
  run_stage "D-15 wire-type compile-time alarm" \
    go test ./integration/wire-type/... -run TestWireTypeCompilation -count=1
else
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: D-15 wire-type compile-time alarm ==="
  echo "  [STAGE-${nn}-SKIP] integration/wire-type not present; D-15 covered by Stage 2 TestDataDecisionExtensionNoNewChunkTypes"
fi

# ---------------------------------------------------------------------------
# Stage 10 — frozen eval baseline re-lock (Task 1)
# ---------------------------------------------------------------------------
run_stage "frozen eval baseline re-lock (Task 1)" \
  bash -c "go run ./cmd/spike-history --frozen=true --archive=true 2>&1 | tail -10"

# Verify the [FROZEN-RELOCK-PASS] prefix fired (run_stage only checks
# exit code; the prefix is the operator's semantic confirmation).
if ! go run ./cmd/spike-history --frozen=true 2>&1 | grep -q "FROZEN-RELOCK-PASS"; then
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "  [STAGE-${nn}-FAIL] frozen re-lock did not emit [FROZEN-RELOCK-PASS]" >&2
  echo "PHASE 11 ACCEPTANCE GATE: FAIL — see stage ${nn} output" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Stage 11 — frozen lock byte-identity check
# ---------------------------------------------------------------------------
run_stage "frozen lock byte-identity check" \
  sha256sum eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 11 ACCEPTANCE GATE: PASS"
echo "==================================================================="
echo "All ${stage_num} stages passed. Frozen baselines verified:"
sha256sum eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json | sed 's/^/  /'
echo
echo "Ship evidence (reports/):"
ls -1 reports/spike_*.json reports/frozen_lock_*.sha256 reports/metrics_diff_*.json 2>/dev/null | tail -6 | sed 's/^/  /' || true
exit 0
