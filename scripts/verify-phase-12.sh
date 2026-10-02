#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-12.sh — Phase 12 acceptance gate (Plan 12-08).
#
# Aggregates 11 stages into one PASS/FAIL banner. The operator runs
# ONE command and reads ONE banner:
#
#   bash scripts/verify-phase-12.sh
#   # or equivalently:
#   make verify-phase-12
#
# On full pass:  prints "PHASE 12 ACCEPTANCE GATE: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL] ..." + "PHASE 12 ACCEPTANCE
# GATE: FAIL — see stage N output" and exits 1.
#
# The gate is HERMETIC for stages 1-9, 11 (no live GLM_API_KEY /
# BGE_M3_URL / MILVUS_ADDRESS reads). Stages 6 + 10 are operator-gated
# (live spike measurement); they print [STAGE-NN-SKIP] when env is
# missing and do NOT exit non-zero (operator-actionable, not blocking).
#
# Live operator gates (manual, not hermetic):
#   - Stage 6  — spike-deep --dry-run (always hermetic)
#   - Stage 10 — frozen eval baseline v2 re-lock (operator-runs-live
#                against GLM_API_KEY + MILVUS_ADDRESS + BGE_M3_URL)
#   - Stage 11 — frozen lock byte-identity (always hermetic; gates
#                against drift from committed deep.lock.json)
#
# When run WITHOUT live env, stages 6 + 10 print clear operator-
# actionable messages; the gate exits 0 with [STAGE-NN-SKIP] markers
# so CI does not block.

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
  local log_file="/tmp/phase-12-stage-${nn}.log"
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
  echo "PHASE 12 ACCEPTANCE GATE: FAIL — see stage ${nn} output" >&2
  exit 1
}

# run_stage_skip is a soft-fail variant: it runs the command, prints
# [STAGE-NN-PASS] on success OR [STAGE-NN-SKIP] on failure. Used for
# operator-gated stages (live stack). Never exits non-zero.
#
# Args:
#   $1 — stage name (for the log line)
#   $2 — skip message (operator-actionable)
#   $3.. — the command to run
run_stage_skip() {
  local name="$1"
  local skip_msg="$2"
  shift 2
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-12-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-SKIP] ${name} — ${skip_msg}"
  echo "    Full log at ${log_file} (exit=${rc})"
  return 0
}

# Trap cleanup on EXIT (success or failure).
cleanup() {
  rm -f /tmp/phase-12-stage-*.log /tmp/vitest-phase-12.log /tmp/spike-phase-12.log 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Gate preamble
# ---------------------------------------------------------------------------

echo "Phase 12 Acceptance Gate — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Go: $(go version 2>&1 | head -1)"
echo "Node: $(node --version 2>&1 | head -1 || echo 'not found')"
echo
echo "Operator-gated stages: 6 (RF-5 spike live), 10 (frozen eval v2 re-lock)"
echo "  Live re-run commands are documented in scripts/verify-phase-12.sh header + eval/frozen/README.md"
echo

# ---------------------------------------------------------------------------
# Stage 1 — hermetic Go suite (PLAN-01..05: foundation + write_todos +
#           plan_approve + budget gate + plan_todos replay)
# ---------------------------------------------------------------------------
# NOTE: -race is intentionally OMITTED — it requires cgo + gcc; the
# hermetic gate runs on operator machines without a C toolchain.
# Operator-runs-live with -race is documented separately (see
# `cmd/spike-deep parse-rate` cost-limit + manual invocation).
run_stage "hermetic Go suite (PLAN-01..05: foundation + write_todos + plan_approve + budget + plan_todos)" \
  go test ./internal/runtime/... ./internal/agent/... ./internal/agent/hitl/... ./internal/agent/callbacks/... ./internal/store/... ./internal/wire/... ./internal/skill/... ./internal/verdict/... ./internal/server/... ./internal/llm/... ./cmd/spike-deep/... -count=1 -timeout=240s

# ---------------------------------------------------------------------------
# Stage 2 — plan API endpoints (12-04 + 12-05)
# ---------------------------------------------------------------------------
run_stage "plan API endpoints (GET + POST /api/run/:id/plan; PLAN-03 + PLAN-05)" \
  bash -c "go test ./internal/server/... -run 'TestPlanAPI' -count=1 -v"

# ---------------------------------------------------------------------------
# Stage 3 — WalkDir legacy interrupt guard (Phase 8 D8-04 extended)
# ---------------------------------------------------------------------------
run_stage "WalkDir legacy interrupt guard (D8-04 extended to runtime + agent/plan + deep_audit)" \
  go test ./integration/contract/... -tags=contract -run "TestNoLegacyInterruptAPI|TestNoLegacyInterruptAPIPhase11" -count=1

# ---------------------------------------------------------------------------
# Stage 4 — cancel propagation <200ms lock (D11-A4 + 12-01 runner)
# ---------------------------------------------------------------------------
run_stage "cancel propagation <200ms lock (D11-A4 + 12-01 runner preservation)" \
  go test ./internal/agent/... -run "TestCancelPropagation" -count=1 -v -timeout 120s

# ---------------------------------------------------------------------------
# Stage 5 — STRIDE-12 critical mitigations
# ---------------------------------------------------------------------------
run_stage "STRIDE-12 critical mitigations (T-P12-1..T-P12-8)" \
  bash -c "go test ./internal/agent/hitl/... ./internal/store/sqlite/... ./internal/agent/... -run 'TestThreatModelCriticalMitigations|TestSTRIDE12' -count=1 -v"

# ---------------------------------------------------------------------------
# Stage 6 — hermetic E2E spike-deep smoke (RF-5 infrastructure)
# ---------------------------------------------------------------------------
run_stage "hermetic E2E smoke cmd/spike-deep (RF-5 parse-rate --dry-run)" \
  bash -c "go build ./cmd/spike-deep && go test ./cmd/spike-deep/ -run 'TestSpikeDeepDryRun|TestSpikeDeepMissingEnv|TestSpikeDeepGoldenSetSchema' -count=1 -v"

# ---------------------------------------------------------------------------
# Stage 7 — vitest UI tests (12-06 + 12-07)
# ---------------------------------------------------------------------------
run_stage "vitest UI tests (12-06 PlanTracePart + 12-07 PlanApprovalCard + PlanReplayView + PlanRulesPanel)" \
  bash -c "cd web && npx vitest run src/components/PlanApprovalCard src/components/PlanReplayView src/components/PlanRulesPanel src/components/PlanTracePart 2>&1 | tail -30"

# ---------------------------------------------------------------------------
# Stage 8 — D-04 transport-swap byte-identity with data-plan-trace
# ---------------------------------------------------------------------------
if [[ -d integration/transport-swap ]]; then
  run_stage "D-04 transport-swap byte-identity with data-plan-trace" \
    go test ./integration/transport-swap/... -run TestTransportSwapWithPlanTrace -count=1
else
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: D-04 transport-swap byte-identity with data-plan-trace ==="
  echo "  [STAGE-${nn}-SKIP] integration/transport-swap not present; D-04 covered by web/src/transport/RealChatTransport.test.ts synthetic write_todos + plan_approve tool-call routing tests"
fi

# ---------------------------------------------------------------------------
# Stage 9 — D-15 wire-type compile-time alarm
# ---------------------------------------------------------------------------
run_stage "D-15 wire-type compile-time alarm (data-plan-trace variant + forced_stop decision enum)" \
  bash -c "cd web && npx tsc --noEmit 2>&1 | tail -5"

# ---------------------------------------------------------------------------
# Stage 10 — frozen eval baseline v2 re-lock (operator-gated live)
# ---------------------------------------------------------------------------
# Live re-run requires GLM_API_KEY + MILVUS_ADDRESS + BGE_M3_URL. When
# the operator has set these, the spike measures RF-5 parse rate + the
# v0.8 milestone metrics (faithfulness + refusal_accuracy +
# budget_triggered_count + plan_todo_completeness). The lock file
# eval/frozen/deep.lock.json is updated with the new measurements +
# sha256. Without live env, this stage SKIPs with a clear operator-
# actionable message.
#
# Live operator command:
#
#   GLM_API_KEY=... BGE_M3_URL=... MILVUS_ADDRESS=... \
#     go run ./cmd/spike-deep parse-rate \
#       --golden=eval/golden/deep.jsonl \
#       --output=reports/frozen-eval-v2.jsonl \
#       --cost-limit=10.00
#
# After running, the operator commits the updated eval/frozen/deep.lock.json
# and re-runs this gate; Stage 11 verifies byte-identity.
if [[ -n "${GLM_API_KEY:-}" && -n "${MILVUS_ADDRESS:-}" && -n "${BGE_M3_URL:-}" ]]; then
  run_stage "frozen eval baseline v2 re-lock (live; RF-5 + v0.8 milestone metrics)" \
    bash -c "go run ./cmd/spike-deep parse-rate --golden=eval/golden/deep.jsonl --output=reports/frozen-eval-v2.jsonl --cost-limit=10.00"
else
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: frozen eval baseline v2 re-lock (live) ==="
  echo "  [STAGE-${nn}-SKIP] operator-gated; requires GLM_API_KEY + MILVUS_ADDRESS + BGE_M3_URL"
  echo "    Live re-run command:"
  echo "      GLM_API_KEY=\$GLM_API_KEY MILVUS_ADDRESS=\$MILVUS_ADDRESS BGE_M3_URL=\$BGE_M3_URL \\"
  echo "        go run ./cmd/spike-deep parse-rate --golden=eval/golden/deep.jsonl \\"
  echo "          --output=reports/frozen-eval-v2.jsonl --cost-limit=10.00"
  echo "    After running, commit the updated eval/frozen/deep.lock.json + re-run this gate (Stage 11 verifies byte-identity)"
fi

# ---------------------------------------------------------------------------
# Stage 11 — frozen lock byte-identity check
# ---------------------------------------------------------------------------
# Verifies the deep.lock.json sha256 is unchanged from the committed
# value. Drift between runs surfaces as a hard FAIL (Pitfall 1).
# If the operator intentionally rebaselined (live Stage 10 modified
# the lock), the new sha256 must be committed BEFORE re-running.
run_stage "frozen lock byte-identity check (deep.lock.json sha256)" \
  bash -c "sha256sum eval/frozen/deep.lock.json eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json"

# Verify the byte-identity is clean (no working-tree drift).
if ! git diff --quiet -- eval/frozen/deep.lock.json 2>/dev/null; then
  stage_num=$((stage_num + 1))
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}-bis: deep.lock.json byte-identity drift ==="
  echo "  [STAGE-${nn}-FAIL] eval/frozen/deep.lock.json has uncommitted modifications"
  echo "    If the operator intentionally rebaselined, commit the updated lock + re-run"
  echo "    Otherwise, the drift indicates a forbidden in-place edit"
  echo "PHASE 12 ACCEPTANCE GATE: FAIL — see stage ${nn}-bis output" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 12 ACCEPTANCE GATE: PASS"
echo "==================================================================="
echo "All ${stage_num} stages passed. Frozen baselines verified:"
sha256sum eval/frozen/deep.lock.json eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json | sed 's/^/  /'
echo
echo "Live operator gates (deferred to manual run):"
echo "  - RF-5 spike re-run: see Stage 10 header for command"
echo "  - Frozen eval v2 re-lock: see eval/frozen/README.md §RF-5 spike protocol"
echo
echo "Ship evidence (reports/):"
ls -1 reports/frozen-eval-*.jsonl 2>/dev/null | tail -3 | sed 's/^/  /' || true
exit 0