#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-39.sh — Phase 39 hermetic acceptance gate (Plan 39-09).
#
# Stages mapping to ROADMAP SC-1..6 (ECL pipeline / ONT-10..13):
#   1. SC-1/SC-2 — chunker 100% fragment-contribution boundary + note 001
#      3/3 corpus SQL targets (TestMergeBoundary, TestNote001SQL)
#   2. SC-2 — stub-vs-reviewed-golden macro-F1 >= 0.8 harness
#      (TestStubVsGoldenF1 over the D-GT-01 human-reviewed golden set)
#   3. SC-2/ONT-12 — extraction seam + scoring math
#      (TestLLMExtract*, TestF1*, TestExtract*)
#   4. SC-3 — ABox dual-write + replace + manager lifecycle + dead-letter
#      flow (TestABox*/TestManager*) and the whole internal/ontology tree
#   5. SC-3/ONT-13 — retriever schema/fields/DeleteByExpr suites
#   6. SC-4/SC-5 — /admin/pipeline gates (TestAdminPipeline*)
#   7. SC-4 UI — web/ contract suite (vitest); runner resolution below
#   8. SC-5/ONT-10 — knowledge indexer routing branch (TestIndex*)
#
# Operator entry point (from repo root):
#   bash scripts/verify-phase-39.sh
#
# On full pass: prints "PHASE 39 HERMETIC GATE: PASS" and exits 0.
# On any failure: prints "[SC-nn-FAIL]-style lines + the FAIL summary and
# exits 1. Regex-gated stages also require a minimum number of --- PASS
# lines so a renamed test cannot vacuously "pass" a -run filter.
#
# HERMETIC: no GLM_API_KEY / MILVUS_ADDRESS / BGE_M3_URL required — every
# stage is stub-driven against the in-repo corpus (knowledge/sql-guides)
# and the committed golden set (eval/ecl-golden/sql-guides.jsonl). The
# live-LLM extraction F1 gate remains a manual operator gate.
#
# Windows/split-toolchain note: run under WSL bash (the Go toolchain is
# WSL-only here; see .planning STATE.md env runbook). This box's WSL has
# no node and WSL interop is disabled, so stage 7 resolves its FE runner
# in order: node → pnpm → explicit PHASE39_FE_GATE=skip-external
# acknowledgment of an out-of-band FE run on the FE-toolchain side. A
# missing FE runtime WITHOUT that acknowledgment is a clean FAIL.

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

WORK="$(mktemp -d /tmp/verify39-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

pass_count=0
fail_count=0
fe_external=0

ok() { echo "[SC-PASS] $1"; pass_count=$((pass_count + 1)); }
fail() { echo "[SC-FAIL] $1"; fail_count=$((fail_count + 1)); }

# pass_lines <log> — number of --- PASS lines in a -v go test log.
pass_lines() { grep -c -- '--- PASS' "$1" || true; }

echo "=== Phase 39 hermetic gate (root: $ROOT) ==="

# ---------------------------------------------------------------------------
# SC-1/SC-2 (stage 1): chunker boundary + note 001 SQL completeness
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/knowledge/ecl/ -run 'TestMergeBoundary|TestNote001SQL' -count=1 -v >"$WORK/stage1.log" 2>&1 \
  && [ "$(pass_lines "$WORK/stage1.log")" -eq 2 ]; then
  ok "SC-1/SC-2 chunker 100% boundary + note 001 3/3 SQL targets"
else
  fail "SC-1/SC-2 chunker boundary — tail:"
  tail -20 "$WORK/stage1.log"
fi

# ---------------------------------------------------------------------------
# SC-2 (stage 2): stub-vs-reviewed-golden F1 harness (D-GT-01 golden)
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/knowledge/ecl/ -run 'TestStubVsGoldenF1' -count=1 -v >"$WORK/stage2.log" 2>&1 \
  && grep -q -- '--- PASS: TestStubVsGoldenF1' "$WORK/stage2.log"; then
  ok "SC-2 stub-vs-golden macro-F1 >= 0.8 harness (26-row reviewed golden)"
  grep -E 'macro-F1|note .*tables P=' "$WORK/stage2.log" | sed 's/^/    /' || true
else
  fail "SC-2 F1 harness — tail:"
  tail -20 "$WORK/stage2.log"
fi

# ---------------------------------------------------------------------------
# SC-2/ONT-12 (stage 3): extraction seam (LLM JSON contract) + F1 math
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/knowledge/ecl/ -run 'TestLLMExtract|TestF1|TestExtract' -count=1 -v >"$WORK/stage3.log" 2>&1 \
  && [ "$(pass_lines "$WORK/stage3.log")" -ge 10 ]; then
  ok "SC-2/ONT-12 extraction seam + scoring math ($(pass_lines "$WORK/stage3.log") tests)"
else
  fail "SC-2/ONT-12 extraction seam + math — tail:"
  tail -20 "$WORK/stage3.log"
fi

# ---------------------------------------------------------------------------
# SC-3 (stage 4): ABox dual-write + manager lifecycle + ontology tree
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/knowledge/ecl/ -run 'TestABox|TestApplyNote|TestManager' -count=1 -v >"$WORK/stage4a.log" 2>&1 \
  && [ "$(pass_lines "$WORK/stage4a.log")" -ge 10 ]; then
  ok "SC-3 ABox dual-write/replace + manager lifecycle ($(pass_lines "$WORK/stage4a.log") tests)"
else
  fail "SC-3 ABox + manager — tail:"
  tail -20 "$WORK/stage4a.log"
fi
if /usr/local/go/bin/go test ./internal/ontology/... -count=1 >"$WORK/stage4b.log" 2>&1; then
  ok "SC-3 internal/ontology suites (KG store / emitter / induction)"
else
  fail "SC-3 internal/ontology suites — tail:"
  tail -20 "$WORK/stage4b.log"
fi

# ---------------------------------------------------------------------------
# SC-3/ONT-13 (stage 5): retriever schema/fields/DeleteByExpr
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/retriever/... -count=1 >"$WORK/stage5.log" 2>&1; then
  ok "SC-3/ONT-13 retriever suites (schema fields + DeleteByExpr)"
else
  fail "SC-3/ONT-13 retriever suites — tail:"
  tail -20 "$WORK/stage5.log"
fi

# ---------------------------------------------------------------------------
# SC-4/SC-5 (stage 6): /admin/pipeline gate order + DTO + conflicts
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/server/ -run 'TestAdminPipeline' -count=1 -v >"$WORK/stage6.log" 2>&1 \
  && [ "$(pass_lines "$WORK/stage6.log")" -ge 1 ]; then
  ok "SC-4/SC-5 admin pipeline gates ($(pass_lines "$WORK/stage6.log") tests)"
else
  fail "SC-4/SC-5 admin pipeline gates — tail:"
  tail -20 "$WORK/stage6.log"
fi

# ---------------------------------------------------------------------------
# SC-4 UI (stage 7): web/ contract suite — runner resolution:
#   1. PHASE39_FE_GATE=skip-external → EXPLICIT operator override:
#      acknowledgment of an out-of-band FE run on the FE-toolchain side
#      (counted as neither pass nor fail; surfaced in the summary line).
#      Checked FIRST because an explicit override must outrank auto-detect —
#      on the split-toolchain box this repo is developed on, WSL sees a
#      broken Windows pnpm shim via PATH that would otherwise shadow the
#      acknowledgment. Never silent: without the var a missing/unusable FE
#      runtime is a clean FAIL.
#   2. node  → direct `node node_modules/vitest/vitest.mjs run` (bypasses a
#      broken pnpm pre-run hook observed in split worktrees; 39-08 precedent)
#   3. pnpm  → plan-literal `pnpm -C web test -- --run`
# ---------------------------------------------------------------------------
if [ "${PHASE39_FE_GATE:-}" = "skip-external" ]; then
  fe_external=1
  echo "[SC-EXT] SC-4 UI (FE) — out-of-band delegation acknowledged (PHASE39_FE_GATE=skip-external)."
  echo "[SC-EXT] The FE suite must have been run on the FE-toolchain side:"
  echo "[SC-EXT]   cd web && node node_modules/vitest/vitest.mjs run"
elif command -v node >/dev/null 2>&1; then
  if (cd web && node node_modules/vitest/vitest.mjs run >"$WORK/stage7.log" 2>&1); then
    ok "SC-4 UI contract suite (vitest run via node)"
  else
    fail "SC-4 UI contract suite — tail:"
    tail -30 "$WORK/stage7.log"
  fi
elif command -v pnpm >/dev/null 2>&1; then
  if pnpm -C web test -- --run >"$WORK/stage7.log" 2>&1; then
    ok "SC-4 UI contract suite (pnpm test)"
  else
    fail "SC-4 UI contract suite — tail:"
    tail -30 "$WORK/stage7.log"
  fi
else
  fail "SC-4 UI contract suite — no FE runtime in this environment (no node/pnpm under WSL; interop disabled); run the suite where the FE toolchain lives, or set PHASE39_FE_GATE=skip-external to acknowledge an out-of-band run"
fi

# ---------------------------------------------------------------------------
# SC-5/ONT-10 (stage 8): knowledge indexer ECL routing branch
# ---------------------------------------------------------------------------
if /usr/local/go/bin/go test ./internal/knowledge/ -run 'TestIndex' -count=1 -v >"$WORK/stage8.log" 2>&1 \
  && [ "$(pass_lines "$WORK/stage8.log")" -ge 10 ]; then
  ok "SC-5/ONT-10 knowledge indexer routing branch ($(pass_lines "$WORK/stage8.log") tests)"
else
  fail "SC-5/ONT-10 knowledge indexer routing — tail:"
  tail -20 "$WORK/stage8.log"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "=== PHASE 39 HERMETIC GATE: $pass_count passed, $fail_count failed ==="
if [ "$fail_count" -eq 0 ]; then
  if [ "$fe_external" -eq 1 ]; then
    echo "PHASE 39 HERMETIC GATE: PASS (SC-4 UI delegated out-of-band — see [SC-EXT] above)"
  else
    echo "PHASE 39 HERMETIC GATE: PASS"
  fi
  exit 0
else
  echo "PHASE 39 HERMETIC GATE: FAIL"
  exit 1
fi
