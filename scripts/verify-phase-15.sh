#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-15.sh — Phase 15 hermetic acceptance gate (Plan 15-07).
#
# 10 stages (01-10): gcc precondition → go build + vet → full-repo test with
# -race → sqlite store suite → duckdb staging/supersede suite → internal/data
# lock-name suite → internal/server import HTTP suite → contract structural
# bans → source-grep locks → frozen lock byte-identity.
#
# Operator entry point (Linux / WSL Ubuntu-22.04 / any gcc-equipped host):
#   bash scripts/verify-phase-15.sh
#   # or equivalently:
#   make verify-phase-15
#
# NOTE ON HOSTS: go-duckdb CANNOT link on Windows-native MSYS2 gcc (emutls
# ABI mismatch, duckdb-go-bindings#105). Run this FULL gate on Linux, WSL2,
# or Docker. On WSL:
#   MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c \
#     'cd /mnt/d/code/ClaudeCode/ai-agent && bash scripts/verify-phase-15.sh'
#
# The internal/data pure-function suite (Stage 06) can be run on Windows for
# fast feedback, but the full gate (Stages 04/05/08 with go-duckdb / mattn
# sqlite CGO) requires a gcc chain that can link the DuckDB static libraries.
#
# On full pass:  prints "PHASE 15 HERMETIC GATE: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL] ..." + tail of failing stage log
#                 + "PHASE 15 HERMETIC GATE: FAIL" and exits 1.
#
# The gate is HERMETIC — no live GLM_API_KEY / MILVUS_ADDRESS / BGE_M3_URL
# required.

set -euo pipefail

# Normalize PATH for common Go installation locations (WSL Ubuntu-22.04,
# Linux tarballs, and systems where go is already on PATH).
if ! command -v go >/dev/null 2>&1 && [ -x /usr/local/go/bin/go ]; then
  export PATH="${PATH}:/usr/local/go/bin"
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

stage_num=0

run_stage() {
  local name="$1"
  shift
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-15-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -20 "${log_file}" | sed 's/^/    /'
  echo "PHASE 15 HERMETIC GATE: FAIL — see stage ${nn} output" >&2
  exit 1
}

cleanup() {
  rm -f /tmp/phase-15-stage-*.log 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Gate preamble
# ---------------------------------------------------------------------------

echo "Phase 15 Hermetic Gate — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Go: $(go version 2>&1 | head -1)"
echo "gcc: $(gcc --version 2>&1 | head -1 || echo 'not found')"
echo

# ---------------------------------------------------------------------------
# Stage 01 — gcc precondition (CGO hard prerequisite; Makefile check-cgo parity)
# ---------------------------------------------------------------------------

run_stage "gcc precondition (CGO hard prerequisite; Makefile check-cgo parity)" \
  bash -c "if ! command -v gcc >/dev/null 2>&1; then echo '[check-cgo] ERROR: gcc not found on PATH.' >&2; echo '  Windows-native sqlite-only work: install MSYS2 UCRT64 gcc (C:\\\\msys64\\\\ucrt64\\\\bin on PATH).' >&2; echo '  This FULL gate (duckdb included): use WSL2 Ubuntu or Linux/Docker —' >&2; echo '  go-duckdb cannot link on Windows-native gcc (emutls ABI, duckdb-go-bindings#105).' >&2; exit 1; fi; gcc --version | head -1"

# ---------------------------------------------------------------------------
# Stage 02 — go build + vet (full repo)
# ---------------------------------------------------------------------------

run_stage "go build + vet" \
  bash -c "go build ./... && go vet ./... && echo OK"

# ---------------------------------------------------------------------------
# Stage 03 — full-repo test suite WITH -race
# ---------------------------------------------------------------------------
# Phase 15 keeps the race-detector posture restored in Phase 14.  New
# concurrent surfaces (import idempotency checks, DuckDB Appender use,
# streaming HTTP handlers) are exercised under -race.

run_stage "full-repo go test -count=1 -race" \
  bash -c "go test ./... -count=1 -race -timeout 900s"

# ---------------------------------------------------------------------------
# Stage 04 — sqlite store suite (DATA-17 audit/registry locks)
# ---------------------------------------------------------------------------

run_stage "sqlite store suite (audit/registry migration locks)" \
  bash -c "go test ./internal/store/... -run 'TestBatchAuditRowShape|TestImportBatchInsertOnly|TestImportBatches|TestMigrateLegacy' -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 05 — duckdb staging + supersede + appender floor (SC-1)
# ---------------------------------------------------------------------------

run_stage "duckdb staging + supersede + appender throughput floor" \
  bash -c "go test ./internal/duckdb/ -run 'TestSupersedeAtomicSwap|TestWriteStagingBatch|TestCompositeNameAttach|TestDuckDBAppenderThroughput' -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 06 — internal/data lock-name suite (DATA-01/02/15/16/D15-D1)
# ---------------------------------------------------------------------------
# Covers every test name listed in 15-VALIDATION.md Per-Task Verification Map
# for the data package, plus the preview/session locks from 15-CONTEXT.md.

run_stage "internal/data lock-name suite" \
  bash -c "go test ./internal/data/ -run 'TestDecode|TestDump|TestHMACNormalizationJoinability|TestHashStableAcrossPeriods|TestUnknownColumnFailClosed|TestRawNeverOnDisk|TestPreviewCommitHashBridge|TestCommitSessionExpired|TestIdempotentReimport|TestBucketBoundaryStableAcrossPeriods|TestPreviewSession|TestPush|TestPreviewFirst100Pairs|TestSupersedeChainAudited' -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 07 — internal/server import HTTP suite (DATA-01/D15-C3)
# ---------------------------------------------------------------------------

run_stage "internal/server import HTTP suite" \
  bash -c "go test ./internal/server/ -run 'TestImportStreaming|TestDeleteDatasetAudited|TestNonImportBodyLimit|TestImportEndpointsAdminGated|TestImportSaltUnset' -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 08 — contract structural bans (D8-04 pattern; import surface)
# ---------------------------------------------------------------------------

run_stage "contract structural bans (multipart/INSERT/row-value logging)" \
  bash -c "go test ./integration/contract/... -tags=contract -count=1 -timeout 180s"

# ---------------------------------------------------------------------------
# Stage 09 — structural-constant source-grep locks
# ---------------------------------------------------------------------------

run_stage "structural-constant source-grep locks" \
  bash -c "set -e
    grep -q 'KMinDefault.*=.*5' internal/data/desensitize.go || { echo 'FATAL: KMinDefault = 5 literal missing'; exit 1; }
    grep -q 'PreviewSessionTTL.*=.*30 \* time.Minute' internal/data/preview.go || { echo 'FATAL: PreviewSessionTTL = 30 * time.Minute literal missing'; exit 1; }
    grep -q 'WithStreamBody(true)' internal/server/server.go || { echo 'FATAL: WithStreamBody(true) missing'; exit 1; }
    grep -q 'WithDisablePreParseMultipartForm(true)' internal/server/server.go || { echo 'FATAL: WithDisablePreParseMultipartForm(true) missing'; exit 1; }
    if grep -R 'sha256\.Sum256(append(' internal/data/ >/dev/null 2>&1; then echo 'FATAL: hand-rolled HMAC pattern sha256.Sum256(append(...)) found in internal/data/'; exit 1; fi
    if [ -z \"\$(git ls-files datarules/)\" ]; then echo 'FATAL: datarules/ directory is empty or not tracked'; exit 1; fi
    echo 'PASS — all source-grep locks present'"

# ---------------------------------------------------------------------------
# Stage 10 — frozen lock byte-identity (4 locks unchanged since Phase 14)
# ---------------------------------------------------------------------------
# Phase 15 does not touch prompt/eval surfaces, so the four byte-frozen locks
# from verify-phase-14.sh Stage 10 must remain byte-identical.  The
# platform-floor lock is field-self-consistent (TestPlatformFloorLockSelfConsistent
# runs in Stage 03 full suite) rather than byte-frozen because live metrics can
# legitimately append operator fields.

EXPECTED_COMPLIANCE_SHA="1dc6f148fe3debd0b572872d87126242362905b26ae1097b983ed8754277bbdc"
EXPECTED_HITL_SHA="6e91881f4aafeffb843e996c89ff1c27ef22eb18fac94eceff9184f938481d08"
EXPECTED_FANOUT_SHA="af3020fd7f38fce2f7396dce81eb151e4ed6952262565785554a59ff51e01252"
EXPECTED_DEEP_SHA="f3958dcd4930ac09e03c824f781df9c6eb04b691770cab4d512e11fba3c3c482"

run_stage "frozen lock byte-identity (4 byte-frozen locks)" \
  bash -c "set -e
    check() { local f=\$1 want=\$2 label=\$3; local got; got=\$(sha256sum \"eval/frozen/\${f}\" | cut -d' ' -f1); if [ \"\${got}\" != \"\${want}\" ]; then echo \"FATAL: \${f} sha256 mismatch (\${label})\"; echo \"  expected: \${want}\"; echo \"  actual:   \${got}\"; exit 1; fi }
    check compliance.lock.json ${EXPECTED_COMPLIANCE_SHA} 'Phase 14 baseline'
    check hitl.lock.json ${EXPECTED_HITL_SHA} 'Phase 14 baseline'
    check fanout.lock.json ${EXPECTED_FANOUT_SHA} 'Phase 14 baseline'
    check deep.lock.json ${EXPECTED_DEEP_SHA} 'Phase 14 baseline'
    sha256sum eval/frozen/*.lock.json"

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 15 HERMETIC GATE: PASS"
echo "==================================================================="
echo "All ${stage_num} stages passed. Phase 15 data-import pipeline"
echo "hermetically verified:"
echo "  - Raw never on disk: streaming multipart + no pre-parse path"
echo "  - Desensitization: HMAC + fixed-width bucketing at import time"
echo "  - Idempotent re-import + audited delete + supersede atomicity"
echo "  - Appender-only bulk write (no row-by-row INSERT INTO)"
echo "  - Structural bans enforced by contract WalkDir gate"
echo
echo "Frozen baselines verified:"
sha256sum eval/frozen/*.lock.json 2>/dev/null | sed 's/^/  /'
echo
exit 0
