#!/usr/bin/env bash
# scripts/verify-phase-37.sh — Hermetic verification gate for Phase 37.
#
# Phase 37 (STORE-01/02 + ROBUST-01..05 + CONCUR-01):
#   - 5 audit INSERT constants unified into putAuditResolutionSQL
#   - 8 LRU type assertions use , ok idiom
#   - hitlHandlers.InjectDeps wrapped with sync.Once
#   - review_report adds 3-way frozen_args check
#   - preview purge runs outside critical section
#   - Decision* constants used in transport dispatch
#   - LatestKPICallByFunc impl (v1.3.1 follow-up)
#   - putKPICallSQL ON CONFLICT rolled back (no UNIQUE constraint yet)
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

echo "=== Phase 37 verify gate ==="
echo

# --- Stage 1: STORE-01 — putAuditResolutionSQL single-source ---
info "Stage 1: putAuditResolutionSQL single-source"
hits=$(grep -rln "^const putAuditResolutionSQL = \`$" internal/store/sqlite/ 2>/dev/null | wc -l | tr -d ' ')
if [ "$hits" -ne 1 ]; then
  fail "putAuditResolutionSQL declared in $hits files (expected 1)"
fi
pass "putAuditResolutionSQL single-source"

# --- Stage 2: 5 audit INSERT constants removed ---
info "Stage 2: 5 audit INSERT constants removed (only putAuditResolutionSQL remains)"
for old in putPendingApprovalSQL putApprovedSQL putRejectedSQL putChangeRequestedSQL putRuleRevokedSQL; do
    if grep -rln "^const ${old} = \`$" internal/store/sqlite/ 2>/dev/null; then
        fail "${old} still declared (Phase 37 STORE-01 unified it into putAuditResolutionSQL)"
    fi
done
pass "5 audit INSERT constants removed"

# --- Stage 3: STORE-02 — ON CONFLICT no longer present (rolled back) ---
info "Stage 3: putKPICallSQL no ON CONFLICT (rolled back until UNIQUE added)"
if grep -E 'ON CONFLICT\(func, args_json, ts\) DO NOTHING' internal/store/sqlite/kpi_calls.go; then
  fail "ON CONFLICT still on putKPICallSQL — must roll back until UNIQUE constraint exists"
fi
pass "ON CONFLICT rolled back (UNIQUE pending CREATE TABLE migration)"

# --- Stage 4: ROBUST-01 — LRU type assertions use , ok idiom ---
info "Stage 4: LRU type assertions use , ok idiom"
bad=$(grep -rnE '\.Value\.\(\*[A-Z][a-zA-Z]+\)\s*$|entry\.\(\*[A-Z][a-zA-Z]+\)\s*$' internal/ 2>/dev/null | grep -v _test || true)
if [ -n "$bad" ]; then
  fail "LRU type assertions still use unsafe single-return form:\n$bad"
fi
pass "all LRU type assertions use , ok idiom"

# --- Stage 5: CONCUR-01 — InjectDeps wrapped with sync.Once ---
info "Stage 5: InjectDeps wrapped with sync.Once"
if ! grep -q "var injectDepsOnce sync.Once" internal/agent/hitl/registry.go; then
  fail "injectDepsOnce sync.Once not defined in hitl/registry.go"
fi
if ! grep -A1 "^func InjectDeps" internal/agent/hitl/registry.go | grep -q "injectDepsOnce.Do"; then
  fail "InjectDeps does not use injectDepsOnce.Do wrapper"
fi
pass "InjectDeps wrapped with sync.Once"

# --- Stage 6: ROBUST-04 — review_report 3-way frozen check ---
info "Stage 6: review_report has 3-way frozen_args check"
if ! grep -q "assertFrozenArgsEqual" internal/agent/hitl/hitl_review_report.go; then
  fail "review_report.go missing assertFrozenArgsEqual call"
fi
pass "review_report has 3-way frozen_args check"

# --- Stage 7: ROBUST-05 — Decision* constants used in transport ---
info "Stage 7: Decision* constants referenced in transport"
if ! grep -q "DecisionForcedStop\|DecisionPlanApproved" web/src/transport/RealChatTransport.ts; then
  fail "RealChatTransport.ts missing Decision* constant references"
fi
pass "Decision* constants used in transport"

# --- Stage 8: ROBUST-02 — preview purge unlocked wrapper exists ---
info "Stage 8: preview purge unlocked wrapper exists"
if ! grep -q "func (s \*SessionStore) purgeExpired()" internal/data/preview.go; then
  fail "preview.go missing unlocked purgeExpired wrapper"
fi
pass "preview unlocked purge wrapper exists"

# --- Stage 9: SILENTAUDIT — audit write paths emit WARN ---
info "Stage 9: silent audit write sites emit WARN log"
if grep -rnE '_ = [a-zA-Z_]+\.[Aa]udit[a-zA-Z]*\.(PutAudit|Write|Insert)\(' internal/ 2>/dev/null | grep -v _test; then
  fail "silent audit write sites still exist"
fi
pass "no silent audit write sites"

# --- Stage 10: build + vet ---
info "Stage 10: go build + vet"
go build ./internal/store/... ./internal/agent/... ./internal/data/...
go vet ./internal/store/... ./internal/agent/...
pass "go build + vet clean"

# --- Stage 11: hermetic store test ---
info "Stage 11: go test ./internal/store/sqlite/ -run TestKPICall"
go test ./internal/store/sqlite/ -count=1 -timeout 30s -run TestKPICall
pass "kpi_calls tests pass"

echo
echo -e "${GREEN}=== ALL CHECKS PASSED ===${NC}"
echo
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'no git')"
echo "Files changed: $(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l || echo '?')"
