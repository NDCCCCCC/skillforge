#!/usr/bin/env bash
# scripts/verify-phase-35.sh — Hermetic verification gate for Phase 35.
#
# Phase 35 (REPEATHTTP-02):
#   - requireAdmin helper replaces 14 copy-paste admin gate templates
#   - All admin gates now emit graphResponseError envelope
#   - graphResponseError + sessionIDRegex single-source status documented
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

echo "=== Phase 35 verify gate ==="
echo

# --- Stage 1: no bare isAdminRequest in handlers (only requireAdmin) ---
info "Stage 1: bare isAdminRequest calls outside audit.go helper body"
# Exclude comment lines starting with `//`
bad=$(grep -rn "if !isAdminRequest(c)" internal/server/ 2>/dev/null | grep -v "^[^:]*:[0-9]*:\s*//" || true)
if [ -n "$bad" ]; then
  fail "isAdminRequest still used directly at handler entry:\n$bad"
fi
pass "all admin gates use requireAdmin (single template)"

# --- Stage 2: requireAdmin helper exists ---
info "Stage 2: requireAdmin helper defined"
if ! grep -q "^func requireAdmin(" internal/server/audit.go; then
  fail "requireAdmin helper not found in audit.go"
fi
pass "requireAdmin helper defined in audit.go"

# --- Stage 3: graphResponseError single source ---
info "Stage 3: graphResponseError declared in single file"
sources=$(grep -rln "^type graphResponseError struct" internal/server/ 2>/dev/null | wc -l)
if [ "$sources" -ne 1 ]; then
  fail "graphResponseError type declared in $sources files (expected 1)"
fi
pass "graphResponseError single-source: $(grep -rln '^type graphResponseError struct' internal/server/ 2>/dev/null)"

# --- Stage 4: sessionIDRegex single source ---
info "Stage 4: sessionIDRegex single source"
sources=$(grep -rln "^var sessionIDRegex" internal/server/ 2>/dev/null | wc -l)
if [ "$sources" -ne 1 ]; then
  fail "sessionIDRegex declared in $sources files (expected 1)"
fi
pass "sessionIDRegex single-source: $(grep -rln '^var sessionIDRegex' internal/server/ 2>/dev/null)"

# --- Stage 5: build sanity ---
info "Stage 5: go build ./internal/server/..."
go build ./internal/server/...
pass "go build ./internal/server/... succeeds"

# --- Stage 6: vet ---
info "Stage 6: go vet ./internal/server/..."
go vet ./internal/server/...
pass "go vet ./internal/server/... clean"

echo
echo -e "${GREEN}=== ALL CHECKS PASSED ===${NC}"
echo
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'no git')"
echo "Files changed: $(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l || echo '?')"
