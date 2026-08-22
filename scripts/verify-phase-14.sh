#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-14.sh — Phase 14 hermetic acceptance gate (Plan 14-07).
#
# 10 stages (00-09): gcc precondition → build + vet → full-repo test with
# -race (restored — gcc is now a hard precondition) → sqlite store suite +
# mattn zero-regression → duckdb connector/lifecycle suite (10-lock series)
# → platform-floor structural locks (Pitfall 17) → PLAT-10 audit/preview
# surfaces → frozen-set hermetic judges (SC-5) → structural-constant
# source-greps → frozen lock byte-identity (5 locks).
#
# Operator entry point (Linux / WSL Ubuntu-22.04 / any gcc-equipped host):
#   bash scripts/verify-phase-14.sh
#   # or equivalently:
#   make verify-phase-14
#
# NOTE ON HOSTS: go-duckdb CANNOT link on Windows-native MSYS2 gcc (emutls
# ABI mismatch, duckdb-go-bindings#105). Run this gate on Linux, WSL2, or
# Docker. On WSL:
#   MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c \
#     'cd /mnt/d/code/ClaudeCode/ai-agent && bash scripts/verify-phase-14.sh'
#
# On full pass:  prints "PHASE 14 HERMETIC GATE: PASS" and exits 0.
# On any failure: prints "[STAGE-NN-FAIL] ..." + tail of failing stage log
#                 + "PHASE 14 HERMETIC GATE: FAIL" and exits 1.
#
# The gate is HERMETIC — no live GLM_API_KEY / MILVUS_ADDRESS / BGE_M3_URL
# required.
#
# Live operator gates (manual, NOT hermetic — Plan 14-08):
#   1. Docker build (CGO flip):
#        docker build -f Dockerfile.backend -t skillforge-backend:cgoflip .
#   2. platform-floor live judge + compliance/hitl frozen metric re-run
#      (GLM key required; same Phase 13 Wave 4 precedent):
#        go run ./cmd/eval --frozen=platform-floor ...   # per 14-08 plan
#  After running, the operator commits updated live-metric fields in the
#  locks and re-runs this gate; Stage 09 verifies byte-identity.

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
  local log_file="/tmp/phase-14-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -20 "${log_file}" | sed 's/^/    /'
  echo "PHASE 14 HERMETIC GATE: FAIL — see stage ${nn} output" >&2
  exit 1
}

# Trap cleanup on EXIT (success or failure).
cleanup() {
  rm -f /tmp/phase-14-stage-*.log 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Gate preamble
# ---------------------------------------------------------------------------

echo "Phase 14 Hermetic Gate — $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Repo root: $(pwd)"
echo "Go: $(go version 2>&1 | head -1)"
echo "gcc: $(gcc --version 2>&1 | head -1 || echo 'not found')"
echo

# ---------------------------------------------------------------------------
# Stage 00 — gcc precondition (same guidance as Makefile check-cgo)
# ---------------------------------------------------------------------------
# CGO is a hard precondition for Phase 14: mattn/go-sqlite3 (14-03) and
# go-duckdb (14-05) both require it. Fail fast with actionable guidance
# instead of a confusing mid-build "cgo: C compiler gcc not found".

run_stage "gcc precondition (CGO hard prerequisite; Makefile check-cgo parity)" \
  bash -c "if ! command -v gcc >/dev/null 2>&1; then echo '[check-cgo] ERROR: gcc not found on PATH.' >&2; echo '  Windows-native sqlite-only work: install MSYS2 UCRT64 gcc (C:\\\\msys64\\\\ucrt64\\\\bin on PATH).' >&2; echo '  This FULL gate (duckdb included): use WSL2 Ubuntu or Linux/Docker —' >&2; echo '  go-duckdb cannot link on Windows-native gcc (emutls ABI, duckdb-go-bindings#105).' >&2; exit 1; fi; gcc --version | head -1"

# ---------------------------------------------------------------------------
# Stage 01 — go build + vet (full repo)
# ---------------------------------------------------------------------------

run_stage "go build + vet" \
  bash -c "go build ./... && go vet ./... && echo OK"

# ---------------------------------------------------------------------------
# Stage 02 — full-repo test suite WITH -race (restored in Phase 14)
# ---------------------------------------------------------------------------
# Phase 13's gate INTENTIONALLY OMITTED -race (no gcc on operator hosts;
# modernc sqlite avoided cgo at the app layer but `go test -race` always
# pulls the cgo runtime). Phase 14 made gcc a hard precondition (14-03
# Makefile check-cgo gate), so the omission reason is GONE — the race
# detector runs again on the whole repo, including the new concurrent
# surfaces (duckdb boot-queries pool, 50-reader concurrency gate, LRU
# mutex, WAL double-close).
#
# NOTE: the SC-3 appender THROUGHPUT floor is NOT in this stage —
# TestDuckDBAppenderThroughput carries a !race build tag (T-14-20). The
# race detector's cgo-boundary instrumentation makes rows/sec
# load-sensitive noise (observed 106k solo / sub-100k under full-repo
# parallel load vs the 100k floor); the floor is enforced by Stage 04
# (non-race) where the measured headroom is ~6x.

run_stage "full-repo go test -count=1 -race (restored — gcc hard precondition since 14-03)" \
  bash -c "go test ./... -count=1 -race -timeout 900s"

# ---------------------------------------------------------------------------
# Stage 03 — sqlite store suite + mattn zero-regression lock
# ---------------------------------------------------------------------------
# The six-face store suite (audit/checkpoint/session/approval idempotency
# + SQLi survival) plus TestSqliteMattnZeroRegression (10th store lock,
# 14-03): aggregate + 8-goroutine concurrent write storm with zero
# SQLITE_BUSY under the mattn DSN-pragma regime.

run_stage "sqlite store suite + TestSqliteMattnZeroRegression (14-03 10th store lock)" \
  bash -c "go test ./internal/store/... -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 04 — duckdb connector + dataset lifecycle (SC-3 + D14-D3/D4)
# ---------------------------------------------------------------------------
# The 10-lock series: 4 SC-3 connector locks from 14-05 (boot-queries,
# WAL flush, appender throughput, concurrent-read-no-block) + 4 LRU locks
# + staging atomic rename four faces from 14-06.

run_stage "duckdb connector + lifecycle suite (10-lock series: SC-3 + LRU + staging)" \
  bash -c "go test ./internal/duckdb/ -count=1 -timeout 300s"

# ---------------------------------------------------------------------------
# Stage 05 — platform-floor structural locks (Pitfall 17 / 14-01)
# ---------------------------------------------------------------------------
# The floor is the single composition point (ComposePlatformPrompt): both
# render paths prepend it, Skills cannot shadow it, the prompt hash covers
# it, hot-reload survives it, and no code path bypasses the composition
# (WalkDir no-bypass gate, D8-04 pattern).

run_stage "platform-floor structural locks (prepend-both-paths + cannot-shadow + hash-covers + hot-reload)" \
  bash -c "go test ./internal/agent/... ./internal/platformfloor/... -run 'TestPlatformFloorPrependBothPaths|TestPlatformFloorCannotBeShadowed|TestPromptHashCoversFloor|TestFloorHotReloadSurvives|TestPlatformFloorVersionLocked|TestComposePlatformPromptShape' -count=1 -v -timeout 120s"

run_stage "WalkDir no-bypass gate (D8-04 pattern; no render path skips ComposePlatformPrompt)" \
  bash -c "go test ./integration/contract/... -tags=contract -run 'TestNoBypassOfPlatformFloorCompose' -count=1 -v -timeout 120s 2>&1 | tail -3"

# ---------------------------------------------------------------------------
# Stage 06 — PLAT-10 surfaces (audit floor-version column + admin preview)
# ---------------------------------------------------------------------------
# Every audit INSERT path carries platform_floor_version (ALTER ADD COLUMN,
# 14-02) and the admin preview endpoint returns the full composed prompt +
# version + hash behind X-Admin-Token.

run_stage "PLAT-10 audit floor-version column + admin preview endpoint" \
  bash -c "go test ./internal/store/sqlite/ ./internal/server/ ./internal/agent/hitl/ -run 'TestAuditPlatformFloorVersionColumn|TestPlatformPromptPreview|TestHITLPlatformFloorVersionRows|TestHITLPlatformFloorVersionFailClosed' -count=1 -v -timeout 180s"

# ---------------------------------------------------------------------------
# Stage 07 — frozen-set hermetic judges (SC-5 / PLAT-11 / 14-04)
# ---------------------------------------------------------------------------
# The 20-case platform-floor golden set + regex judge + self-consistent
# lock, plus the shipped frozen lock/set sync checks (T-8-12 posture).

run_stage "frozen-set hermetic judges (platform-floor golden + schema + lock self-consistency)" \
  bash -c "go test ./internal/eval/ -run 'TestPlatformFloorFrozenSet|TestPlatformFloorGoldenSchema|TestPlatformFloorLockSelfConsistent|TestShippedFrozenLockIsInSync|TestShippedFrozenSetLoads' -count=1 -v -timeout 120s"

# ---------------------------------------------------------------------------
# Stage 08 — structural-constant source-greps
# ---------------------------------------------------------------------------
# Config drift cannot weaken the floor or the capacity gates: the version
# literal, both hermetic SET constants, the LRU cap, and the appender
# throughput floor are locked as source literals.

run_stage "structural-constant source-greps (floor version + hermetic SETs + LRU cap + appender floor)" \
  bash -c "set -e
    grep -q 'PlatformFloorVersion.*=.*\"1\.0\.0\"' internal/platformfloor/platform_floor.go || { echo 'FATAL: PlatformFloorVersion semver literal missing'; exit 1; }
    grep -q 'autoinstall_known_extensions=false' internal/duckdb/connector.go || { echo 'FATAL: bootHermetic autoinstall SET missing'; exit 1; }
    grep -q 'autoload_known_extensions=false' internal/duckdb/connector.go || { echo 'FATAL: bootHermetic autoload SET missing'; exit 1; }
    grep -q 'MaxAttachedDatasets = 8' internal/duckdb/attach_lru.go || { echo 'FATAL: MaxAttachedDatasets literal missing'; exit 1; }
    grep -q 'AppenderMinRowsPerSec = 100_000' internal/duckdb/appender_throughput_test.go || { echo 'FATAL: AppenderMinRowsPerSec literal missing'; exit 1; }
    echo 'PASS — all five structural constants present'"

# ---------------------------------------------------------------------------
# Stage 09 — frozen lock byte-identity (5 locks; sha256 verification)
# ---------------------------------------------------------------------------
# Baselines pinned at Phase 14 close (14-07, 2026-08-22):
#   - compliance.lock.json — re-pinned by 14-01 (58ffea08… → ca333fba…,
#     D14-B3 floor-into-hash planned re-lock; T-8-12 recorded action).
#   - hitl.lock.json — UNCHANGED from Phase 11 (prompt_hash is the
#     SKILL.md file digest, NOT the rendered prompt — the floor does not
#     touch it; see 14-07-SUMMARY deviation note).
#   - platform-floor.lock.json — created 14-04 (SC-5 hermetic layer).
#   - fanout.lock.json / deep.lock.json — no prompt surface, byte-frozen.
# Drift between runs surfaces as a hard FAIL (T-8-12 + T-14-18/19).

EXPECTED_COMPLIANCE_SHA="e77877bb3b3076bc67d33a1f7a1cf4bea3b9af20ebafdd457ef6e9c366ff6ce5"      # 14-01 re-pin
EXPECTED_HITL_SHA="6e91881f4aafeffb843e996c89ff1c27ef22eb18fac94eceff9184f938481d08"            # Phase 11 baseline UNCHANGED
EXPECTED_PLATFORM_FLOOR_SHA="b28f30ae37568de261aaa9a797cec05b42c75bd5f19c9d317a7cd270290e954a"  # 14-04 creation
EXPECTED_FANOUT_SHA="af3020fd7f38fce2f7396dce81eb151e4ed6952262565785554a59ff51e01252"          # Phase 13 baseline UNCHANGED
EXPECTED_DEEP_SHA="f3958dcd4930ac09e03c824f781df9c6eb04b691770cab4d512e11fba3c3c482"            # Phase 10 placeholder UNCHANGED

run_stage "frozen lock byte-identity (5 locks sha256 — compliance/hitl/platform-floor/fanout/deep)" \
  bash -c "set -e
    check() { local f=\$1 want=\$2 label=\$3; local got; got=\$(sha256sum \"eval/frozen/\${f}\" | cut -d' ' -f1); if [ \"\${got}\" != \"\${want}\" ]; then echo \"FATAL: \${f} sha256 mismatch (\${label})\"; echo \"  expected: \${want}\"; echo \"  actual:   \${got}\"; exit 1; fi }
    check compliance.lock.json ${EXPECTED_COMPLIANCE_SHA} '14-01 floor re-pin'
    check hitl.lock.json ${EXPECTED_HITL_SHA} 'Phase 11 baseline'
    check platform-floor.lock.json ${EXPECTED_PLATFORM_FLOOR_SHA} '14-04 creation'
    check fanout.lock.json ${EXPECTED_FANOUT_SHA} 'Phase 13 baseline'
    check deep.lock.json ${EXPECTED_DEEP_SHA} 'Phase 10 placeholder'
    sha256sum eval/frozen/*.lock.json"

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

echo
echo "==================================================================="
echo "PHASE 14 HERMETIC GATE: PASS"
echo "==================================================================="
echo "All ${stage_num} stages passed. Phase 14 CGO trust floor hermetically"
echo "verified:"
echo "  - Platform floor: single composition point, unshadowable, no bypass"
echo "  - PLAT-10: floor version on every audit row + admin preview endpoint"
echo "  - mattn/go-sqlite3 zero-regression + DSN pragma regime (14-03)"
echo "  - go-duckdb connector: hermetic boot-queries + WAL flush + capacity gates (14-05)"
echo "  - Dataset lifecycle: lazy ATTACH LRU + atomic staging commit (14-06)"
echo "  - Frozen baselines: 5 locks byte-identical (T-8-12 recorded re-pins)"
echo
echo "Frozen baselines verified:"
sha256sum eval/frozen/*.lock.json 2>/dev/null | sed 's/^/  /'
echo
echo "Live operator gates (deferred to Plan 14-08):"
echo "  1. Docker CGO flip build:  docker build -f Dockerfile.backend -t skillforge-backend:cgoflip ."
echo "  2. platform-floor live judge 20-case + compliance/hitl metric re-run (GLM key)"
echo
exit 0
