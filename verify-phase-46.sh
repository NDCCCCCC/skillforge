#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# verify-phase-46.sh — Phase 46 hermetic gate (Entity SHACL Shapes).
# Plan 46-05 Task 1 (ROADMAP Phase 46 SC4).
#
# Partitions (fail-fast, in order):
#   P1  build + vet            — production CGO path (CGO_ENABLED=1)
#   P2  ontology suite         — shacl/store/induction/emitter coverage
#                                (quarantine plumbing + boot-survival chain)
#   P3  targeted shacl+induct. — exit code paired with a minimum --- PASS
#                                count (anti-vacuous, verify-phase-45.sh
#                                pass_lines pattern)
#   P4  full internal suite    — ./internal/... hermetic
#
# HERMETIC: no live LLM / Milvus / BGE required.
#
# Environment: run `bash verify-phase-46.sh` from the repo root (Git Bash on
# Windows). Every gate command executes INSIDE WSL (Ubuntu-22.04) with
# CGO_ENABLED=1 — the Windows cgo.exe exits 2, so no gate command may run
# outside WSL (verify-phase-45.sh precedent; env-wsl-backend-runbook).
#
# FE scope note: frontend vitest is NOT part of this gate (Go-only, per the
# Phase 45 gate mode). Phase 46 FE deliverables (46-04) are verified via
# direct node tsc/vitest invocation on the Windows side (this box's WSL has
# no node). The pre-existing FE failure D-46-DEF-01 (DetailPanel SC-5) is
# recorded in .planning/phases/46-entity-shacl-shapes/deferred-items.md and
# is outside this gate's scope.
#
# D-45-DEF-01 waiver: TestEmitSubCategoryInstanceCount
# (internal/ontology/emitter, "SubCategory instances = 6, want 7") fails
# PRE-EXISTING — proven at the Phase 45 pre-phase commit 5bf9c35 via
# throwaway worktree (see .planning/phases/45-sparql-schema-visibility-
# d-sparql-05/deferred-items.md) and RE-PROBED at Phase 46 HEAD 2026-10-08:
# still failing. Fixing it is out of Phase 46 scope. P2/P4 therefore pass
# ONLY when the command exits 0 (empty --- FAIL set) or exits non-zero with
# a unique --- FAIL set of EXACTLY that one documented test; an empty log or
# any other failure red-lights the gate immediately (WR-06: the exit code is
# authoritative, grep-only judgment let infrastructure failures pass).

set -uo pipefail

export MSYS_NO_PATHCONV=1

REPO=/mnt/d/CODE/ClaudeCode/ai-agent
GO=/usr/local/go/bin/go

# wsl <command> — run a gate command inside WSL at the repo root.
wsl() { wsl.exe -d Ubuntu-22.04 -- bash -c "cd $REPO && $1"; }

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass()    { echo -e "${GREEN}PASS${NC} $1"; }
fail()    { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info()    { echo -e "${YELLOW}INFO${NC} $1"; }
waived()  { echo -e "${YELLOW}WAIVED${NC} $1"; }

WORK="$(mktemp -d /tmp/verify46-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

# pass_lines <log> — number of --- PASS lines in a -v go test log
# (verify-phase-39.sh anti-vacuous pattern).
pass_lines() { grep -c -- '--- PASS' "$1" || true; }

# waiver_ok <log> → 0 iff the log's unique --- FAIL set is empty or EXACTLY
# the D-45-DEF-01 pre-existing failure. Package-level FAIL lines (no "---")
# are ignored by the pattern; a second --- FAIL anywhere red-lights.
WAIVED_FAIL='--- FAIL: TestEmitSubCategoryInstanceCount'
waiver_ok() {
  local fails
  fails=$(grep -o -- '--- FAIL: [^[:space:]]*' "$1" | sort -u)
  [ -z "$fails" ] || [ "$fails" = "$WAIVED_FAIL" ]
}

echo "=== Phase 46 Verification (WSL Ubuntu-22.04, CGO_ENABLED=1) ==="

# ---------------------------------------------------------------------------
# P1: build + vet (production CGO path)
# ---------------------------------------------------------------------------
info "P1: build + vet (CGO_ENABLED=1)"
wsl "CGO_ENABLED=1 $GO build ./... && CGO_ENABLED=1 $GO vet ./internal/ontology/... ./internal/server/..." \
  || fail "P1 build+vet failed"
pass "P1 build + vet"

# fail_detail <log> <label> — print the log tail AND the unique --- FAIL set
# so a waiver mismatch names the exact offending test(s).
fail_detail() {
  echo "--- $2 log tail ---"
  tail -30 "$1"
  echo "--- $2 unique --- FAIL set ---"
  grep -o -- '--- FAIL: [^[:space:]]*' "$1" | sort -u || true
}

# ---------------------------------------------------------------------------
# P2: ontology suite (shacl + store + induction + emitter)
# ---------------------------------------------------------------------------
# WR-06: the exit code is authoritative. A grep-only --- FAIL judgment let
# infrastructure failures pass as "zero failures found": a toolchain crash,
# a bad test flag, an OOM kill, or an empty log from a WSL spawn failure all
# produce no --- FAIL lines. Now: an empty log fails outright, and a
# non-zero exit code passes only when the unique --- FAIL set is exactly
# the D-45-DEF-01 waiver.
info "P2: ontology suite (./internal/ontology/...)"
wsl "CGO_ENABLED=1 $GO test ./internal/ontology/... -count=1" >"$WORK/p2.log" 2>&1
p2_rc=$?
if [ ! -s "$WORK/p2.log" ]; then
  fail "P2 produced no output (rc=$p2_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p2_rc" -ne 0 ]; then
  if ! waiver_ok "$WORK/p2.log"; then
    fail_detail "$WORK/p2.log" "P2"
    fail "P2 ontology suite failed (rc=$p2_rc, beyond the D-45-DEF-01 waiver)"
  fi
  waived "P2: rc=$p2_rc, unique --- FAIL set matches the D-45-DEF-01 waiver (TestEmitSubCategoryInstanceCount, deferred-items.md)"
fi
tail -5 "$WORK/p2.log"
pass "P2 ontology suite"

# ---------------------------------------------------------------------------
# P3: targeted shacl + induction gate (anti-vacuous --- PASS count)
# ---------------------------------------------------------------------------
info "P3: targeted shacl + induction (exit code + >= 7 --- PASS)"
wsl "CGO_ENABLED=1 $GO test ./internal/ontology/shacl/... ./internal/ontology/induction/... -count=1 -run 'TestShapesPaths|TestSeverity|TestViolationLayer|TestDanglingEdges|TestEntitiesShapesParse|TestEntityQuarantine|TestRollbackShaclParity' -v" \
  >"$WORK/p3.log" 2>&1
p3_rc=$?
# 7 = 6 shacl tests (TestShapesPaths / TestSeverity / TestViolationLayer /
# TestDanglingEdges / TestEntitiesShapesParse / TestEntityQuarantine — names
# verified against shacl_test.go and the 46-01/46-02 SUMMARYs) + 1 induction
# test (TestRollbackShaclParity, induction_test.go, 46-03). go test -run
# matches unanchored, so TestSeverity also pulls in TestSeveritySourcePath-
# Retired and sub-tests each add a --- PASS line — the live count runs
# higher; the floor stays anti-vacuous: any renamed/removed target test
# drops the count below 7 and fails the gate even with exit code 0.
if [ "$p3_rc" -ne 0 ] || [ "$(pass_lines "$WORK/p3.log")" -lt 7 ]; then
  echo "--- P3 log tail ---"
  tail -30 "$WORK/p3.log"
  fail "P3 targeted shacl+induction failed (rc=$p3_rc, pass_lines=$(pass_lines "$WORK/p3.log"), need >= 7)"
fi
pass "P3 targeted shacl+induction ($(pass_lines "$WORK/p3.log") --- PASS, threshold 7)"

# ---------------------------------------------------------------------------
# P4: full hermetic internal suite
# ---------------------------------------------------------------------------
# WR-06: same exit-code + empty-log discipline as P2.
info "P4: full internal suite (./internal/...)"
wsl "CGO_ENABLED=1 $GO test ./internal/... -count=1" >"$WORK/p4.log" 2>&1
p4_rc=$?
if [ ! -s "$WORK/p4.log" ]; then
  fail "P4 produced no output (rc=$p4_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p4_rc" -ne 0 ]; then
  if ! waiver_ok "$WORK/p4.log"; then
    fail_detail "$WORK/p4.log" "P4"
    fail "P4 full internal suite failed (rc=$p4_rc, beyond the D-45-DEF-01 waiver)"
  fi
  waived "P4: rc=$p4_rc, unique --- FAIL set matches the D-45-DEF-01 waiver (TestEmitSubCategoryInstanceCount, deferred-items.md)"
fi
tail -3 "$WORK/p4.log"
pass "P4 full internal suite"

echo ""
echo -e "${GREEN}=== Phase 46 Gate: ALL PASSED ===${NC}"
echo ""
echo "Partitions verified:"
echo "  P1 build + vet            — CGO_ENABLED=1 ./... + ./internal/{ontology,server}/..."
echo "  P2 ontology suite         — shacl/store/induction/emitter (quarantine + boot-survival chain)"
echo "  P3 targeted shacl+induct. — 7 named tests (6 shacl + 1 induction), >= 7 --- PASS"
echo "  P4 full internal suite    — ./internal/... hermetic (D-45-DEF-01 named waiver)"
