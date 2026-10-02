#!/usr/bin/env bash
# scripts/verify-phase-36.sh — Hermetic verification gate for Phase 36.
#
# Phase 36 (SILENTAUDIT-01 + REPEATFE-01..03):
#   - 3 silent audit write sites in agent/runtime emit WARN log on err
#   - shared FE apiFetch helper exists with extractReason
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

echo "=== Phase 36 verify gate ==="
echo

# --- Stage 1: no silent audit _ = .audit.PutAudit(...) ---
info "Stage 1: no silent audit write patterns"
bad=$(grep -rnE '_ = [a-zA-Z_]+\.[Aa]udit[a-zA-Z]*\.(PutAudit|Write|Insert)\(' internal/ 2>/dev/null || true)
if [ -n "$bad" ]; then
  fail "silent audit write patterns still present:\n$bad"
fi
pass "no silent audit write patterns"

# --- Stage 2: WARN log present for audit write paths ---
info "Stage 2: audit write paths emit WARN log"
hits=$(grep -rn '\[WARN\].*audit write failed' internal/agent/ internal/runtime/ 2>/dev/null | wc -l)
if [ "$hits" -lt 3 ]; then
  fail "expected ≥3 WARN audit log sites, got $hits"
fi
pass "$hits audit write failure WARN log sites"

# --- Stage 3: apiFetch helper exists ---
info "Stage 3: apiFetch helper exists"
if ! grep -q "^export async function apiFetch" web/src/lib/api.ts; then
  fail "apiFetch helper not exported from web/src/lib/api.ts"
fi
pass "apiFetch helper exported from web/src/lib/api.ts"

# --- Stage 4: extractReason helper exists ---
info "Stage 4: extractReason helper exists"
if ! grep -q "^export function extractReason" web/src/lib/api.ts; then
  fail "extractReason helper not exported from web/src/lib/api.ts"
fi
pass "extractReason helper exported from web/src/lib/api.ts"

# --- Stage 5: build sanity ---
info "Stage 5: go build ./internal/agent/... ./internal/runtime/..."
go build ./internal/agent/... ./internal/runtime/...
pass "go build clean"

# --- Stage 6: web type check ---
info "Stage 6: web tsc --noEmit"
cd web && npx tsc --noEmit 2>&1 | tail -5
pass "tsc --noEmit clean"

echo
echo -e "${GREEN}=== ALL CHECKS PASSED ===${NC}"
echo
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'no git')"
echo "Files changed: $(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l || echo '?')"
