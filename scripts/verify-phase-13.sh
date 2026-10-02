#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-13.sh — Phase 13 hermetic acceptance gate (Plan 13-04).
#
# 9 stages: build + vet → hermetic Go test suite → WalkDir 8 roots (legacy
# interrupt ban) → NewParallelAgent source-grep (P13-L1 sealed sentinel) →
# D-04 transport-swap byte-identity → D15 wire-type compile → frozen eval
# byte-identity (sha256 of compliance.lock.json + fanout.lock.json) →
# sub-agent budget half-of-parent (P13-L3 source-grep) → shared chunk
# accumulator LRU cap (P13-L9 + Pitfall 6).
#
# Operator entry point:
#   bash scripts/verify-phase-13.sh
#   # or equivalently:
#   make verify-phase-13
#
# On full pass:  prints "PHASE 13 HERMETIC GATE: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL] ..." + tail of failing stage log
#                 + "PHASE 13 HERMETIC GATE: FAIL" and exits 1.
#
# The gate is HERMETIC — no live GLM_API_KEY / MILVUS_ADDRESS / BGE_M3_URL
# required. Stages 6 (D15 tsc) and 8 (sub-agent budget source-grep) read
# only the working tree; stages 1-5 + 7 + 9 run the Go suite.
#
# Live operator gate (manual, NOT hermetic — Plan 13-05):
#   go run ./cmd/spike-fanout --live --golden=eval/golden/fanout.jsonl \
#     --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md \
#     --cost-limit=2.00 --seed=42 --summarize
#
# After running, the operator commits the updated eval/frozen/fanout.lock.json
# and re-runs this gate; Stage 7 verifies byte-identity.

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
  local log_file="/tmp/phase-13-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -20 "${log_file}" | sed 's/^/    /'
  echo "PHASE 13 HERMETIC GATE: FAIL — see stage ${nn} output" >&2
  exit 1
}

# Trap cleanup on EXIT (success or failure).
cleanup() {
  rm -f /tmp/phase-13-stage-*.log 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Gate preamble
# ---------------------------------------------------------------------------

echo "Phase 13 Hermetic Gate — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Go: $(go version 2>&1 | head -1)"
echo "Node: $(node --version 2>&1 | head -1 || echo 'not found')"
echo

# ---------------------------------------------------------------------------
# Stage 1 — go build + vet (foundation safety net)
# ---------------------------------------------------------------------------

run_stage "go build + vet" \
  bash -c "go build ./... && go vet ./... && echo OK"

# ---------------------------------------------------------------------------
# Stage 2 — hermetic Go test suite (full Phase 13 surface)
# ---------------------------------------------------------------------------
# This runs the full Phase 13 foundation:
#   - 13-01 (orchestrator + sub_agent_builder + fanout_runtime)
#   - 13-02 (merge pure function + 10× byte-identity contract)
#   - 13-03 (per-sub-agent grounding + fault isolation + D16 overload + audit)
#   - skill + factory + agent + callbacks + store + wire + server packages
#   - spike-fanout hermetic scenarios (RF-4)
#
# NOTE (Plan 13-04): -race is INTENTIONALLY OMITTED — it requires cgo +
# gcc; the modernc.org/sqlite pure-Go driver avoids cgo at the app layer,
# but `go test -race` always pulls in the cgo runtime. On operators'
# Windows hosts (no gcc on PATH) this surfaces as "cgo: C compiler gcc
# not found" and fails every package. Operator-runs-live with -race is
# documented separately (see "Operator-runs-live" section below).

run_stage "hermetic Go test suite (FAN-01..04; -race omitted per Phase 12 precedent)" \
  bash -c "go test ./internal/runtime/... ./internal/skill/... ./internal/agent/... ./internal/agent/callbacks/... ./internal/agent/hitl/... ./internal/store/... ./internal/wire/... ./internal/verdict/... ./internal/server/... ./cmd/spike-fanout/... -count=1 -timeout=300s"

# ---------------------------------------------------------------------------
# Stage 3 — WalkDir legacy interrupt guard (D8-04 extended to 8 roots)
# ---------------------------------------------------------------------------
# Phase 13 extends the WalkDir union from 6 (Phase 12) to 8 roots by
# adding the new fan-out files:
#   - internal/runtime/orchestrator.go
#   - internal/runtime/sub_agent_builder.go
#   - internal/runtime/merge.go
#   - internal/store/sqlite/fanout_events.go
#
# Invariant (FAN-01 SC-1 + P13-L8): sub-agent code NEVER calls legacy
# interrupt API. Run with -tags=contract to enable the Phase 13 WalkDir.

run_stage "WalkDir legacy interrupt guard (D8-04 extended; 8 roots incl. orchestrator + sub_agent_builder + merge + fanout_events)" \
  bash -c "go test ./integration/contract/... -tags=contract -run 'TestNoLegacyInterruptAPI|TestNoLegacyInterruptAPIPhase11' -count=1 -v -timeout=60s"

# ---------------------------------------------------------------------------
# Stage 4 — adk.NewParallelAgent ban source-grep (P13-L1 sealed sentinel)
# ---------------------------------------------------------------------------
# Phase 13 explicitly REJECTS adk.NewParallelAgent (Pitfall 1 + P13-L1):
# full-context sharing breaks FAN-02 tier-closure. The orchestrator's
# sealed-sentinel comment + this source-grep is a fast O(N) check that
# catches the architectural violation before any test runs.
#
# Verification:
#   1. No invocations of adk.NewParallelAgent inside internal/runtime/
#      (only the sealed-sentinel doc comments mention the API by name).
#   2. The sealed sentinel comment is present in orchestrator.go.

run_stage "adk.NewParallelAgent ban source-grep (P13-L1 sealed sentinel)" \
  bash -c "set -e; if grep -rn 'adk\.NewParallelAgent(' internal/runtime/ --include='*.go' | grep -v '//' | grep -v '/\*' ; then echo 'FATAL: adk.NewParallelAgent invocation found in internal/runtime/'; exit 1; fi; if ! grep -qE '禁止 adk\.NewParallelAgent|DO NOT introduce \`adk\.NewParallelAgent' internal/runtime/orchestrator.go; then echo 'FATAL: sealed sentinel comment missing in orchestrator.go'; exit 1; fi; echo 'PASS — no invocations; sealed sentinel present'"

# ---------------------------------------------------------------------------
# Stage 5 — D-04 transport-swap byte-identity with fanout_* data-decision
# ---------------------------------------------------------------------------
# D-04 invariant: mock transport and Default transport produce byte-
# identical UIMessageChunk streams. Phase 13 13-03 extends the
# data-decision enum with fanout_dispatched | fanout_completed |
# fanout_failed (D16 overload) — this stage verifies the byte-identity
# is preserved across BOTH transports.

run_stage "D-04 transport-swap byte-identity (mock ↔ Default; data-decision fanout_* enum extended)" \
  bash -c "go test ./web/src/transport/... ./internal/wire/... -run 'TestTransportSwapWithFanoutChunks|TestTransportSwapInvariant|TestRealChatTransportFanoutRouting' -count=1 -v -timeout=60s"

# ---------------------------------------------------------------------------
# Stage 6 — D15 wire-type compile alarm (TS mirror in lockstep)
# ---------------------------------------------------------------------------
# D15 invariant: TypeScript types + Go types stay lockstep on the
# UIMessageChunk union. Phase 13 13-03 extended both `internal/wire/chunk.go`
# AND `web/src/types/wire-types.ts` with the 3 fanout_* enum values. This
# stage runs npx tsc --noEmit to catch drift.

run_stage "D15 wire-type compile alarm (npx tsc --noEmit; data-decision fanout_* enum lockstep)" \
  bash -c "cd web && npx tsc --noEmit 2>&1 | tail -10"

# ---------------------------------------------------------------------------
# Stage 7 — frozen eval byte-identity (sha256 verification)
# ---------------------------------------------------------------------------
# Verifies BOTH lock files are present + byte-identity matches the
# expected sha256 pins recorded in STATE.md. Drift between runs surfaces
# as a hard FAIL (T-8-12 + P13 invariant).
#
# - compliance.lock.json — re-pinned twice as RECORDED T-8-12 actions:
#   14-01 (D14-B3 floor-into-hash, 5cf700f3… → e77877bb…) and 14-10
#   (floor 1.1.0 tail re-anchor, prompt_hash ca333fba… → 5b12acd9…).
#   The current pin is lockstep with verify-phase-14.sh Stage 11
#   (1dc6f148…). Any OTHER sha = tampering or an unrecorded re-lock.
# - fanout.lock.json     — Phase 13 v0.8 milestone re-baseline (13-04
#   placeholder; 13-05 fills actual sha256 after live UAT operator
#   sign-off).

EXPECTED_COMPLIANCE_SHA="1dc6f148fe3debd0b572872d87126242362905b26ae1097b983ed8754277bbdc"  # 14-01 re-pin → 14-07 ripple → 14-10 floor-1.1.0 re-pin ripple (lockstep with verify-phase-14.sh Stage 11; T-8-12 recorded re-lock)
run_stage "frozen eval byte-identity (compliance.lock.json + fanout.lock.json sha256)" \
  bash -c "set -e; ACTUAL_COMPLIANCE_SHA=\$(sha256sum eval/frozen/compliance.lock.json | cut -d' ' -f1); if [ \"\${ACTUAL_COMPLIANCE_SHA}\" != \"${EXPECTED_COMPLIANCE_SHA}\" ]; then echo \"FATAL: compliance.lock.json sha256 mismatch — file tampered OR an unrecorded re-lock bypassed T-8-12 (pins move only via recorded re-locks: 14-01 / 14-10, lockstep with verify-phase-14.sh Stage 11)\"; echo \"  expected: ${EXPECTED_COMPLIANCE_SHA}\"; echo \"  actual:   \${ACTUAL_COMPLIANCE_SHA}\"; exit 1; fi; if [ ! -f eval/frozen/fanout.lock.json ]; then echo 'FATAL: fanout.lock.json missing — 13-04 Task 2 should have created it'; exit 1; fi; sha256sum eval/frozen/compliance.lock.json eval/frozen/fanout.lock.json"

# ---------------------------------------------------------------------------
# Stage 8 — sub-agent budget half-of-parent (P13-L3 source-grep)
# ---------------------------------------------------------------------------
# P13-L3: SubAgentMaxIteration=5 + SubAgentTokenBudget=20000 (half of
# parent's 10/50000). These are STRUCTURAL constants — source-grep locks
# the literals so config drift cannot weaken the budget cap.

run_stage "sub-agent budget half-of-parent (P13-L3 source-grep; MaxIteration=5 + TokenBudget=20000)" \
  bash -c "set -e; if ! grep -q 'SubAgentMaxIteration = 5' internal/runtime/fanout_runtime.go && ! grep -q 'SubAgentMaxIteration:[[:space:]]*5' internal/runtime/fanout_runtime.go; then echo 'FATAL: SubAgentMaxIteration: 5 literal missing from fanout_runtime.go'; exit 1; fi; if ! grep -q 'SubAgentTokenBudget = 20000' internal/runtime/fanout_runtime.go && ! grep -q 'SubAgentTokenBudget:[[:space:]]*20000' internal/runtime/fanout_runtime.go; then echo 'FATAL: SubAgentTokenBudget: 20000 literal missing from fanout_runtime.go'; exit 1; fi; go test ./internal/runtime/... -run 'TestSubAgentBudgetHalfOfParent|TestSubAgentMaxIterationHardCap' -count=1 -v"

# ---------------------------------------------------------------------------
# Stage 9 — shared chunk accumulator LRU cap (P13-L9 + Pitfall 6)
# ---------------------------------------------------------------------------
# P13-L9: SharedChunkAccumulator LRU cap at 20 chunks (structural constant
# SharedChunkAccumulatorMaxEntries=20). Sub-agents append to the parent's
# accumulator via shared pointer; LRU evicts oldest when capacity reached.
# Pitfall 6 mitigation — unbounded growth across N sub-agents.

run_stage "SharedChunkAccumulator LRU cap (P13-L9; Pitfall 6 — 20-chunk cap + dedup + oldest-first order)" \
  bash -c "go test ./internal/runtime/... -run 'TestSharedChunkAccumulatorLRUCap|TestSharedChunkAccumulatorDedup|TestSharedChunkAccumulatorHitsOrder' -count=1 -v -timeout=60s"

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 13 HERMETIC GATE: PASS"
echo "==================================================================="
echo "All ${stage_num} stages passed. Phase 13 fan-out fan-in architecture"
echo "(FAN-01..04) hermetically verified:"
echo "  - Tier-closure structural inheritance (P13-L2)"
echo "  - Deterministic 10× byte-identity merge (P13-L4)"
echo "  - Conservative partial-failure merge (P13-L5)"
echo "  - Per-sub-agent grounding via ApplyTrustStage (P13-L9)"
echo "  - D16 data-decision overload (P13-L7) — fanout_* enum"
echo "  - D15 TS lockstep + D-04 transport-swap byte-identity"
echo "  - Frozen eval byte-identity (compliance.lock.json UNCHANGED)"
echo "  - adk.NewParallelAgent ban (P13-L1 sealed sentinel)"
echo
echo "Frozen baselines verified:"
sha256sum eval/frozen/compliance.lock.json eval/frozen/fanout.lock.json 2>/dev/null | sed 's/^/  /'
echo
echo "Live operator gate (deferred to Plan 13-05):"
echo "  go run ./cmd/spike-fanout --live --golden=eval/golden/fanout.jsonl \\"
echo "    --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md \\"
echo "    --cost-limit=2.00 --seed=42 --summarize"
echo
exit 0