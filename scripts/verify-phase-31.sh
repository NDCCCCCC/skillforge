#!/usr/bin/env bash
# Phase 31 verification gate — runs all four hermetic checks.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Phase 31 verification ==="

echo "[1/4] vitest — sanitize-url + useEvalReports tests"
cd "$PROJECT_ROOT/web" && node node_modules/vitest/vitest.mjs run src/lib/sanitize-url.test.ts src/hooks/useEvalReports.test.ts --reporter=verbose
echo "PASS: all tests green"

echo "[2/4] tsc --noEmit"
cd "$PROJECT_ROOT/web" && node node_modules/typescript/bin/tsc --noEmit
echo "PASS: tsc clean"

echo "[3/4] grep guard — no hardcoded ALLOWED_HOSTNAMES re-introduction"
COUNT=$(grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" "$PROJECT_ROOT/web/src" 2>/dev/null | grep -v node_modules | wc -l || true)
if [ "$COUNT" -ne 0 ]; then
  echo "FAIL: found $COUNT hardcoded ALLOWED_HOSTNAMES = new Set[ occurrences"
  grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" "$PROJECT_ROOT/web/src" 2>/dev/null | grep -v node_modules || true
  exit 1
fi
echo "PASS: no hardcoded ALLOWED_HOSTNAMES"

echo "[4/4] grep guard — setEvalStatus('ready') is present"
COUNT=$(grep -rn "setEvalStatus('ready')" "$PROJECT_ROOT/web/src" 2>/dev/null | grep -v node_modules | wc -l || true)
if [ "$COUNT" -lt 1 ]; then
  echo "FAIL: setEvalStatus('ready') not found"
  exit 1
fi
echo "PASS: setEvalStatus('ready') found in $COUNT location(s)"

echo ""
echo "=== ALL CHECKS PASSED ==="
