#!/usr/bin/env bash
# scripts/verify-phase-34.sh — Hermetic verification gate for Phase 34.
#
# Phase 34 (HITL-01..03):
#   - HITL-03: noopAuditWriter moved to audit_writer.go
#   - HITL-01: shared auditOrDefault() helper replaces 4 per-tool methods
#   - HITL-02: shared writePendingAndInterrupt() helper replaces 4 sequences
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

echo "=== Phase 34 verify gate ==="
echo

# --- Stage 1: noopAuditWriter defined in audit_writer.go (single source) ---
info "Stage 1: noopAuditWriter in audit_writer.go only"
# Search for the type declaration. Allow comment references.
if grep -rnE '^type noopAuditWriter struct\{\}' internal/agent/hitl/ 2>/dev/null | grep -v audit_writer.go; then
  fail "noopAuditWriter type declaration exists outside audit_writer.go"
fi
pass "noopAuditWriter type declaration is in audit_writer.go only"

# --- Stage 2: per-tool auditOrDefault methods removed ---
info "Stage 2: no per-tool auditOrDefault methods remain"
bad=$(grep -rnE 'func \([a-zA-Z]+ \*[A-Za-z]+\) auditOrDefault\(\) auditWriterHook' internal/agent/hitl/ 2>/dev/null || true)
if [ -n "$bad" ]; then
  fail "per-tool auditOrDefault methods still exist:\n$bad"
fi
pass "no per-tool auditOrDefault methods (shared helper used instead)"

# --- Stage 3: all call sites use shared helper ---
info "Stage 3: call sites use auditOrDefault or writePendingAndInterrupt"
# Count bare .auditOrDefault() call sites (should be zero — method removed)
bare=$(grep -rnE '\.auditOrDefault\(\)' internal/agent/hitl/ 2>/dev/null | grep -v _test || true)
if [ -n "$bare" ]; then
  fail ".auditOrDefault() bare call sites (method should be removed):\n$bare"
fi
pass "no bare .auditOrDefault() call sites (shared helper used)"

# --- Stage 4: build sanity ---
info "Stage 4: go build ./internal/agent/hitl/..."
go build ./internal/agent/hitl/...
pass "go build ./internal/agent/hitl/... succeeds"

# --- Stage 5: hermetic test run ---
info "Stage 5: go test ./internal/agent/hitl/ (hermetic subset)"
go test ./internal/agent/hitl/ -count=1 -timeout 60s
pass "all hitl tests pass"

echo
echo -e "${GREEN}=== ALL CHECKS PASSED ===${NC}"
echo
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'no git')"
echo "Files changed: $(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l || echo '?')"
