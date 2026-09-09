#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/.."

echo "=== Phase 31 verification ==="

echo "[1/4] vitest — sanitize-url tests"
cd web && pnpm vitest run src/lib/sanitize-url.test.ts src/hooks/useEvalReports.test.ts --reporter=verbose

echo "[2/4] tsc --noEmit"
pnpm tsc --noEmit

echo "[3/4] grep guard — no hardcoded ALLOWED_HOSTNAMES re-introduction"
COUNT=$(grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" web/src | grep -v node_modules | wc -l)
if [ "$COUNT" -ne 0 ]; then
  echo "FAIL: found $COUNT hardcoded ALLOWED_HOSTNAMES = new Set[ occurrences"
  grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" web/src | grep -v node_modules
  exit 1
fi
echo "PASS: no hardcoded ALLOWED_HOSTNAMES"

echo "[4/4] grep guard — setEvalStatus('ready') is present"
COUNT=$(grep -rn "setEvalStatus('ready')" web/src | grep -v node_modules | wc -l)
if [ "$COUNT" -lt 1 ]; then
  echo "FAIL: setEvalStatus('ready') not found"
  exit 1
fi
echo "PASS: setEvalStatus('ready') found in $COUNT location(s)"

echo "=== ALL CHECKS PASSED ==="
