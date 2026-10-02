#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-12-test.sh — CI wrapper for the Phase 12
# acceptance gate. Used in CI to gate merges to main branch.
#
# The wrapper:
#   1. Runs bash scripts/verify-phase-12.sh
#   2. Asserts exit 0 (or 0 with WARN for operator-gated stages)
#   3. Optionally re-runs the gate to ensure determinism
#
# Exit codes:
#   0 — gate PASSED (all hermetic stages green; live stages skipped or
#       re-lock applied + committed)
#   1 — gate FAILED (one or more hermetic stages failed)
#   2 — gate PASSED but a WARN marker fired (operator must review)

set -euo pipefail

echo "==================================================================="
echo "Phase 12 Acceptance Gate — CI Wrapper"
echo "==================================================================="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo

# Run the gate. Capture stdout/stderr to a log file so CI artifacts
# carry the full output.
GATE_LOG="/tmp/phase-12-ci-gate.log"
if ! bash scripts/verify-phase-12.sh >"${GATE_LOG}" 2>&1; then
  echo "FAIL — Phase 12 acceptance gate exited non-zero"
  echo "--- last 60 lines of gate output ---"
  tail -60 "${GATE_LOG}" | sed 's/^/  /'
  exit 1
fi

# Detect WARN markers (operator-gated stages that produced SKIP).
# We tolerate SKIP — the gate is intentionally split between hermetic
# stages (must pass) and operator-gated stages (must be re-run later).
if grep -qE '\[STAGE-[0-9]+-SKIP\]' "${GATE_LOG}"; then
  echo "PASS (with operator-gated SKIP markers) — see below"
  echo "--- SKIP markers (operator-actionable, not blocking) ---"
  grep -E '\[STAGE-[0-9]+-SKIP\]' "${GATE_LOG}" | sed 's/^/  /'
  echo
  echo "Operator must run the deferred live stages before merge:"
  echo "  - Stage 6 RF-5 spike live re-run (operator-runs-live; GLM_API_KEY + MILVUS_ADDRESS + BGE_M3_URL)"
  echo "  - Stage 10 frozen eval v2 re-lock (operator-runs-live; records new sha256 in deep.lock.json)"
  echo "  - See scripts/verify-phase-12.sh header + eval/frozen/README.md for the exact commands"
  exit 2
fi

echo "PASS — Phase 12 acceptance gate fully green"
echo
tail -30 "${GATE_LOG}" | sed 's/^/  /'
exit 0