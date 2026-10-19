#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# verify-phase-47.sh — Phase 47 zeroing gate (debt cleanup + reproducible baseline).
# Plan 47-08 Task 1 (ROADMAP Phase 47 SC-4).
#
# WHAT THIS GATE IS: the Phase 47 "zeroing boundary". Phase 45/46 gates could
# pass P2/P4 by naming one documented known failure; that mechanism is retired
# here and in those two scripts. The local gate and the CI gate must now say
# the same thing — a bare `go test ./internal/...` on a fresh clone is exactly
# what P4 runs. If this script ever needs to let a failure through, the failure
# becomes a row in 47-DEBT-DISPOSITION.md with a 去向 and a 证据 cell, not a
# branch in this file.
#
# Partitions (fail-fast, in order):
#   P1  build + vet         — production CGO path (CGO_ENABLED=1)
#   P2  ontology suite      — shacl/store/induction/emitter
#   P3  targeted repairs    — the four tests Phase 47 repaired, exit code paired
#                             with a minimum --- PASS count (anti-vacuous,
#                             verify-phase-45.sh pass_lines pattern)
#   P4  full internal suite — ./internal/... hermetic
#   P5  determinism strength— TestDeriveEdgesDeterministic at -count=100 plus a
#                             -shuffle=on pass over knowledge + ontology.
#                             Phase 47-01 measured the pre-fix flake rate at
#                             17–25%, so "two consecutive green runs" carries
#                             only ~53–69% confidence; the SC-4 wording of two
#                             runs is retained as a smoke threshold, not as the
#                             evidence. This segment is the evidence.
#   P6  frontend line       — tsc --noEmit + vitest run (Windows side)
#   P7  disposition record  — every v1.6 leftover ID has exactly one row whose
#                             去向 is non-empty and inside the closed vocabulary,
#                             and whose 证据 is non-empty
#
# HERMETIC: no live LLM / Milvus / BGE required.
#
# Environment: run `bash verify-phase-47.sh` from the repo root (Git Bash on
# Windows). Every Go gate command executes INSIDE WSL (Ubuntu-22.04) with
# CGO_ENABLED=1 — the Windows cgo.exe exits 2, so no Go command may run outside
# WSL (verify-phase-45.sh precedent; env-wsl-backend-runbook).
#
# FE segment: P6 runs on the Windows side, NOT through wsl(), because this box's
# WSL has no node installed. It runs inside a subshell against "$ROOT/web" so the
# script's working directory is unchanged for every later partition — P7 resolves
# its record from "$ROOT", and a shell left inside web/ would break that path.
#
# P6's ONLY non-fatal branch: if `command -v npx` fails, print an explicit info
# line naming npx as absent and continue. That guard keys on a TOOL CHECK alone,
# never on a test name, a waivable failure, or a test outcome. Every other
# command in this script is exit-code-authoritative: a non-zero rc fails the gate
# immediately, and an empty log fails it too (WR-06 — a grep-only judgment lets a
# toolchain crash, a bad flag, an OOM kill, or a failed WSL spawn pass as "zero
# failures found").

set -uo pipefail

export MSYS_NO_PATHCONV=1

REPO=/mnt/d/CODE/ClaudeCode/ai-agent
GO=/usr/local/go/bin/go

# ROOT — the repository this script lives in, resolved from the script's own
# location. P6 and P7 address repository paths through this, so neither depends
# on the caller's cwd nor on any `cd` performed by an earlier partition.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# wsl <command> — run a gate command inside WSL at the repo root.
wsl() { wsl.exe -d Ubuntu-22.04 -- bash -c "cd $REPO && $1"; }

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}PASS${NC} $1"; }
fail() { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }

WORK="$(mktemp -d /tmp/verify47-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

# pass_lines <log> — number of --- PASS lines in a -v go test log
# (verify-phase-39.sh anti-vacuous pattern).
pass_lines() { grep -c -- '--- PASS' "$1" || true; }

# fail_detail <log> <label> — print the log tail AND the unique failure set so a
# red light names the exact offending test(s).
fail_detail() {
  echo "--- $2 log tail ---"
  tail -30 "$1"
  echo "--- $2 unique failure set ---"
  grep -o -- '--- FAIL: [^[:space:]]*' "$1" | sort -u || true
}

# run_go_test <label> <logname> <args...> — exit-code-authoritative + empty-log
# guard around every `go test` invocation in this script.
run_go_test() {
  local label="$1" logname="$2"
  shift 2
  wsl "CGO_ENABLED=1 $GO test $* -count=1" >"$WORK/$logname" 2>&1
  local rc=$?
  if [ ! -s "$WORK/$logname" ]; then
    fail "$label produced no output (rc=$rc) — infrastructure failure, no evidence of a test run"
  fi
  if [ "$rc" -ne 0 ]; then
    fail_detail "$WORK/$logname" "$label"
    fail "$label failed (rc=$rc)"
  fi
}

echo "=== Phase 47 Verification (WSL Ubuntu-22.04, CGO_ENABLED=1) ==="

# ---------------------------------------------------------------------------
# P1: build + vet (production CGO path)
# ---------------------------------------------------------------------------
info "P1: build + vet (CGO_ENABLED=1)"
wsl "CGO_ENABLED=1 $GO build ./... && CGO_ENABLED=1 $GO vet ./internal/ontology/... ./internal/server/... ./internal/knowledge/..." \
  || fail "P1 build+vet failed"
pass "P1 build + vet"

# ---------------------------------------------------------------------------
# P2: ontology suite (shacl + store + induction + emitter)
# ---------------------------------------------------------------------------
info "P2: ontology suite (./internal/ontology/...)"
run_go_test "P2" "p2.log" "./internal/ontology/..."
tail -5 "$WORK/p2.log"
pass "P2 ontology suite"

# ---------------------------------------------------------------------------
# P3: targeted repaired tests (anti-vacuous --- PASS count)
# ---------------------------------------------------------------------------
info "P3: targeted repairs (exit code + >= 6 --- PASS)"
wsl "CGO_ENABLED=1 $GO test ./internal/knowledge/ ./internal/ontology/... -count=1 -run 'TestDeriveEdgesDeterministic|TestRollbackShaclParity|TestEmitSkillInstanceCount|TestEmitSubCategoryInstanceCount' -v" \
  >"$WORK/p3.log" 2>&1
p3_rc=$?
# Floor 6 = the count measured on 2026-10-19 with one -v run at plan 47-08
# execution time: 1 TestDeriveEdgesDeterministic (knowledge) + 2 emitter
# (TestEmitSkillInstanceCount, TestEmitSubCategoryInstanceCount) + 1
# TestRollbackShaclParity top-level + its 2 sub-tests = 6 --- PASS lines.
# The floor stays anti-vacuous: a renamed or removed target test drops the
# count below 6 and fails the gate even with exit code 0. The other ontology
# packages legitimately report "no tests to run" for this -run filter and
# contribute no --- PASS line.
if [ ! -s "$WORK/p3.log" ]; then
  fail "P3 produced no output (rc=$p3_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p3_rc" -ne 0 ] || [ "$(pass_lines "$WORK/p3.log")" -lt 6 ]; then
  fail_detail "$WORK/p3.log" "P3"
  fail "P3 targeted repairs failed (rc=$p3_rc, pass_lines=$(pass_lines "$WORK/p3.log"), need >= 6)"
fi
pass "P3 targeted repairs ($(pass_lines "$WORK/p3.log") --- PASS, floor 6)"

# ---------------------------------------------------------------------------
# P4: full hermetic internal suite
# ---------------------------------------------------------------------------
info "P4: full internal suite (./internal/...)"
run_go_test "P4" "p4.log" "./internal/..."
tail -3 "$WORK/p4.log"
pass "P4 full internal suite"

# ---------------------------------------------------------------------------
# P5: determinism strength (-count=100 + one -shuffle=on run)
# ---------------------------------------------------------------------------
info "P5a: determinism (TestDeriveEdgesDeterministic -count=100)"
wsl "CGO_ENABLED=1 $GO test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100" \
  >"$WORK/p5a.log" 2>&1
p5a_rc=$?
if [ ! -s "$WORK/p5a.log" ]; then
  fail "P5a produced no output (rc=$p5a_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p5a_rc" -ne 0 ]; then
  fail_detail "$WORK/p5a.log" "P5a"
  fail "P5a determinism -count=100 failed (rc=$p5a_rc)"
fi
pass "P5a determinism -count=100"

info "P5b: determinism (-shuffle=on over knowledge + ontology)"
wsl "CGO_ENABLED=1 $GO test ./internal/knowledge/ ./internal/ontology/... -count=1 -shuffle=on" \
  >"$WORK/p5b.log" 2>&1
p5b_rc=$?
if [ ! -s "$WORK/p5b.log" ]; then
  fail "P5b produced no output (rc=$p5b_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p5b_rc" -ne 0 ]; then
  fail_detail "$WORK/p5b.log" "P5b"
  fail "P5b -shuffle=on failed (rc=$p5b_rc)"
fi
pass "P5b determinism -shuffle=on"
pass "P5 determinism strength (100x + shuffle)"

# ---------------------------------------------------------------------------
# P6: frontend line (Windows side — this box's WSL has no node)
# ---------------------------------------------------------------------------
info "P6: frontend (tsc --noEmit + vitest run)"
if ! command -v npx >/dev/null 2>&1; then
  info "P6: npx is absent from PATH on this host — FE segment not run (environment guard on a tool check, not on a test outcome)"
else
  # No empty-log guard here, unlike the Go segments: `tsc --noEmit` prints
  # nothing on success, so silence is its normal pass signal. The exit code is
  # the evidence; fail_detail prints the diagnostics when it is non-zero.
  ( cd "$ROOT/web" && npx tsc --noEmit ) >"$WORK/p6-tsc.log" 2>&1
  p6_tsc_rc=$?
  if [ "$p6_tsc_rc" -ne 0 ]; then
    fail_detail "$WORK/p6-tsc.log" "P6 tsc"
    fail "P6 tsc --noEmit failed (rc=$p6_tsc_rc)"
  fi
  pass "P6a tsc --noEmit"

  ( cd "$ROOT/web" && npx vitest run ) >"$WORK/p6-vitest.log" 2>&1
  p6_vitest_rc=$?
  if [ ! -s "$WORK/p6-vitest.log" ]; then
    fail "P6 vitest produced no output (rc=$p6_vitest_rc) — infrastructure failure, no evidence of a test run"
  fi
  if [ "$p6_vitest_rc" -ne 0 ]; then
    fail_detail "$WORK/p6-vitest.log" "P6 vitest"
    fail "P6 vitest run failed (rc=$p6_vitest_rc)"
  fi
  tail -12 "$WORK/p6-vitest.log"
  pass "P6b frontend vitest run"
fi

# ---------------------------------------------------------------------------
# P7: disposition record — per-ID presence + non-empty closed-vocabulary 去向
# ---------------------------------------------------------------------------
# The record is read through "$ROOT", so this partition depends on neither the
# caller's cwd nor any `cd` performed by an earlier partition (P6's subshell
# leaves the script's cwd untouched by construction).
# Comment lines are stripped into a scratch copy first, so a comment that
# happens to mention an ID cannot satisfy or break the per-ID match count.
REC="$ROOT/.planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md"
[ -f "$REC" ] || fail "P7 disposition record not found: $REC"
grep -v -E '^[[:space:]]*#' "$REC" > "$WORK/record.md"

info "P7: disposition record (19 IDs, closed vocabulary, non-empty evidence)"
BT='`'
for id in D-45-DEF-01 D-45-DEF-02 D-45-DEF-03 D-46-DEF-01 D-46-DEF-02 \
          WARNING-1 WARNING-2 BL-01 BL-02 BL-03 BL-04 D-05 \
          IN-01 IN-02 IN-03 IN-04 IN-05 IN-06 IN-07; do
  rowpat="^| ${BT}${id}${BT} |"
  n=$(grep -c -- "$rowpat" "$WORK/record.md" || true)
  if [ "$n" != "1" ]; then
    fail "P7 $id appears as $n table rows in 47-DEBT-DISPOSITION.md, want exactly 1"
  fi
  row=$(grep -m1 -- "$rowpat" "$WORK/record.md")
  dest=$(printf '%s\n' "$row" | awk -F'|' '{print $4}' | sed 's/^[` ]*//; s/[` ]*$//')
  evid=$(printf '%s\n' "$row" | awk -F'|' '{print $5}' | sed 's/^[` ]*//; s/[` ]*$//')
  case "$dest" in
    'v1.7 修复'|'延期+理由'|'ack backlog') ;;
    *) fail "P7 $id 去向 cell is empty or outside the closed vocabulary: '$dest'" ;;
  esac
  if [ -z "$evid" ]; then
    fail "P7 $id 证据 cell is empty — the record may not satisfy a row with an empty evidence promise"
  fi
done
pass "P7 disposition record (19 IDs, each one row, closed 去向, non-empty 证据)"

echo ""
echo -e "${GREEN}=== Phase 47 Gate: ALL PASSED ===${NC}"
echo ""
echo "Partitions verified:"
echo "  P1 build + vet            — CGO_ENABLED=1 build ./... + vet ./internal/{ontology,server,knowledge}/..."
echo "  P2 ontology suite         — shacl/store/induction/emitter, exit-code authoritative"
echo "  P3 targeted repairs       — 4 repaired tests, >= 6 --- PASS (floor measured 2026-10-19)"
echo "  P4 full internal suite    — ./internal/... hermetic, exit-code authoritative"
echo "  P5 determinism strength   — TestDeriveEdgesDeterministic -count=100 + one -shuffle=on run"
echo "  P6 frontend line          — tsc --noEmit + vitest run (Windows side; npx-absent guard is a tool check)"
echo "  P7 disposition record     — 19 v1.6 leftover IDs, closed 去向 vocabulary, non-empty 证据"
