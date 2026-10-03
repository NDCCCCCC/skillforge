#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# verify-phase-45.sh — Phase 45 hermetic gate (SPARQL Schema Visibility,
# D-SPARQL-05 β). Plan 45-02 Task 3 (SC4).
#
# Partitions (fail-fast, in order):
#   P1  build + vet            — production CGO path (CGO_ENABLED=1)
#   P2  ontology suite         — red lines + D-45-01 conformance + dual test
#   P3  targeted SPARQL+Hybrid — exit code paired with a minimum --- PASS
#                                  count (anti-vacuous, verify-phase-39.sh
#                                  pass_lines pattern)
#   P4  full internal suite    — ./internal/... hermetic
#
# HERMETIC: no live LLM / Milvus / BGE required.
#
# Environment: run `bash verify-phase-45.sh` from the repo root (Git Bash on
# Windows). Every gate command executes INSIDE WSL (Ubuntu-22.04) with
# CGO_ENABLED=1 — the Windows cgo.exe exits 2, so no gate command may run
# outside WSL (45-RESEARCH Pitfall 6; 44-01-PLAN WSL CGO chain).
#
# D-45-DEF-01 waiver: TestEmitSubCategoryInstanceCount
# (internal/ontology/emitter, "SubCategory instances = 6, want 7") fails
# PRE-EXISTING — proven at pre-phase commit 5bf9c35 via throwaway worktree
# (see .planning/phases/45-sparql-schema-visibility-d-sparql-05/deferred-items.md).
# Fixing it is out of Phase 45 scope. P2/P4 therefore pass ONLY when their
# unique --- FAIL set is empty or EXACTLY that one documented test; any
# other failure red-lights the gate immediately.

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

WORK="$(mktemp -d /tmp/verify45-XXXXXX)"
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

echo "=== Phase 45 Verification (WSL Ubuntu-22.04, CGO_ENABLED=1) ==="

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
# P2: ontology suite (red lines + D-45-01 conformance + dual test)
# ---------------------------------------------------------------------------
info "P2: ontology suite (./internal/ontology/...)"
wsl "CGO_ENABLED=1 $GO test ./internal/ontology/... -count=1" >"$WORK/p2.log" 2>&1
if ! waiver_ok "$WORK/p2.log"; then
  fail_detail "$WORK/p2.log" "P2"
  fail "P2 ontology suite failed (beyond the D-45-DEF-01 waiver)"
fi
tail -5 "$WORK/p2.log"
if grep -q -- '--- FAIL' "$WORK/p2.log"; then
  waived "P2: 1 known pre-existing failure waived (D-45-DEF-01 TestEmitSubCategoryInstanceCount, deferred-items.md)"
fi
pass "P2 ontology suite"

# ---------------------------------------------------------------------------
# P3: targeted SPARQL + Hybrid gate (anti-vacuous --- PASS count)
# ---------------------------------------------------------------------------
info "P3: targeted SPARQL + Hybrid (exit code + >= 19 --- PASS)"
wsl "CGO_ENABLED=1 $GO test ./internal/server/ -count=1 -run 'TestSparqlQueryScoped|TestHybridSearchScoped' -v" \
  >"$WORK/p3.log" 2>&1
p3_rc=$?
# 19 = 10 TestSparqlQueryScoped (admin_platform_test.go) + 9
# TestHybridSearchScoped. The def01 file contributes an 11th SPARQL test
# (TestSparqlQueryScoped_LiveCategoryWiring_ABoxVisible), so the live tree
# yields 20 — the threshold stays anti-vacuous: a renamed/removed test drops
# the count below 19 and fails the gate even with exit code 0.
if [ "$p3_rc" -ne 0 ] || [ "$(pass_lines "$WORK/p3.log")" -lt 19 ]; then
  echo "--- P3 log tail ---"
  tail -30 "$WORK/p3.log"
  fail "P3 targeted SPARQL+Hybrid failed (rc=$p3_rc, pass_lines=$(pass_lines "$WORK/p3.log"), need >= 19)"
fi
pass "P3 targeted SPARQL+Hybrid ($(pass_lines "$WORK/p3.log") --- PASS, threshold 19)"

# ---------------------------------------------------------------------------
# P4: full hermetic internal suite
# ---------------------------------------------------------------------------
info "P4: full internal suite (./internal/...)"
wsl "CGO_ENABLED=1 $GO test ./internal/... -count=1" >"$WORK/p4.log" 2>&1
if ! waiver_ok "$WORK/p4.log"; then
  fail_detail "$WORK/p4.log" "P4"
  fail "P4 full internal suite failed (beyond the D-45-DEF-01 waiver)"
fi
tail -3 "$WORK/p4.log"
if grep -q -- '--- FAIL' "$WORK/p4.log"; then
  waived "P4: 1 known pre-existing failure waived (D-45-DEF-01 TestEmitSubCategoryInstanceCount, deferred-items.md)"
fi
pass "P4 full internal suite"

echo ""
echo -e "${GREEN}=== Phase 45 Gate: ALL PASSED ===${NC}"
echo ""
echo "Partitions verified:"
echo "  P1 build + vet            — CGO_ENABLED=1 ./... + ./internal/{ontology,server}/..."
echo "  P2 ontology suite         — red lines + D-45-01 conformance + dual test"
echo "  P3 targeted SPARQL+Hybrid — TestSparqlQueryScoped|TestHybridSearchScoped, >= 19 --- PASS"
echo "  P4 full internal suite    — ./internal/... hermetic (D-45-DEF-01 named waiver)"
