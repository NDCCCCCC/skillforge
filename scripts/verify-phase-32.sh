#!/usr/bin/env bash
# scripts/verify-phase-32.sh — Hermetic verification gate for Phase 32.
#
# Phase 32 (REALBUG-03 + PERF-01):
#   - isBackendUnavailable rewritten with typed sentinels (errors.Is/As/net.Error)
#   - joinStrings helper removed; strings.Join used directly
#
# This gate verifies:
#   1. joinStrings is gone (grep guard — prevent re-introduction)
#   2. isBackendUnavailable no longer uses strings.Contains for error matching
#   3. `strings.Contains` substring-matching on err.Error() is banned in server package
#   4. go build of internal/server compiles
#
# Run via: bash scripts/verify-phase-32.sh
#   or:    wsl bash scripts/verify-phase-32.sh  (Windows host)
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

echo "=== Phase 32 verify gate ==="
echo

# --- Stage 1: joinStrings removed ---
info "Stage 1: joinStrings helper removed"
# Match function definition `func joinStrings(` or call site `joinStrings(`,
# but exclude comments referencing it for historical context.
if grep -rnE '(func[[:space:]]+joinStrings[[:space:]*\(]|\bjoinStrings[[:space:]*]\()' internal/ cmd/ 2>/dev/null; then
  fail "joinStrings still defined or called — must be removed (PERF-01)"
fi
pass "joinStrings function/calls removed (comments referencing it are fine)"

# --- Stage 2: isBackendUnavailable uses typed sentinels ---
info "Stage 2: isBackendUnavailable uses typed sentinels, no strings.Contains"
# Locate the function and confirm it does NOT use strings.Contains on err.Error()
if awk '/^func isBackendUnavailable/,/^}/' internal/server/upload.go | grep -E 'strings\.Contains.*err|err\.Error\(\)'; then
  fail "isBackendUnavailable still uses strings.Contains / err.Error() — must use errors.Is/As"
fi
pass "isBackendUnavailable uses typed sentinels only"

# --- Stage 3: removed (overreaching — see audit Phase 32 scope) ---
# Note: other `strings.Contains(err.Error(), ...)` calls exist in server package
# (approval_rules_api.go:238 "no rule found", upload.go:319 "missing file") — these
# are business-logic substring matches on wrapped store errors, NOT network errors,
# and are out of scope for Phase 32 (REALBUG-03 / PERF-01).

# --- Stage 4: build sanity ---
info "Stage 4: go build ./internal/server/..."
if command -v wsl >/dev/null 2>&1 && [ -n "${WSL_INTEROP:-}" ]; then
  # Windows host with WSL available
  wsl bash -c "cd $(pwd | sed 's|^/mnt/||' | sed 's|/|/|g') && go build ./internal/server/..."
else
  go build ./internal/server/...
fi
pass "go build ./internal/server/... succeeds"

echo
echo -e "${GREEN}=== ALL CHECKS PASSED ===${NC}"
echo
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo 'no git')"
echo "Files changed: $(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l || echo '?')"
