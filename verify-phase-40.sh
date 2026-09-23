#!/bin/bash
# verify-phase-40.sh — Hermetic gate for Phase 40 Data Governance
# Tests: SHACL violation detection, provenance coverage, tier filtering, rollback latency
set -euo pipefail

PHASE=40
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

pass() { echo -e "${GREEN}PASS${NC} $1"; }
fail() { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }

echo "=== Phase 40 Verification ==="

# (a) Go build + vet
info "(a) Go build + vet"
cd "$PROJECT_ROOT"
go build ./... 2>&1 | head -5 || fail "build failed"
go vet ./internal/ontology/shacl/... ./internal/ontology/store/... ./internal/ontology/induction/... 2>&1 | head -5 || fail "vet failed"
pass "(a) build + vet"

# (b) SHACL violation detection (hermetic)
info "(b) SHACL violation detection"
cd "$PROJECT_ROOT"
go test ./internal/ontology/shacl/... -v -run "TestShaclViolation|TestShaclQuarantine" 2>&1 | tail -5 || fail "SHACL violation/quarantine tests failed"
pass "(b) SHACL violation detection"

# (c) Provenance coverage 100%
info "(c) Provenance coverage"
go test ./internal/knowledge/ecl/... -v -run TestProvenanceCoverage 2>&1 | tail -5 || fail "provenance coverage test failed"
pass "(c) provenance coverage 100%"

# (d) Tier filtering unit tests
info "(d) Tier filtering"
go test ./internal/ontology/store/... -v -run "TestQueryScoped|TestProvenance" 2>&1 | tail -5 || fail "QueryScoped/provenance tests failed"
pass "(d) tier filtering"

# (e) Rollback latency < 5s
info "(e) Rollback latency < 5s"
SECONDS=0
go test ./internal/ontology/induction/... -v -run TestRollbackLatency 2>&1 | tail -5 || fail "rollback latency test failed"
ELAPSED=$SECONDS
if [ $ELAPSED -lt 5 ]; then
    pass "(e) rollback ${ELAPSED}s < 5s"
else
    fail "(e) rollback ${ELAPSED}s >= 5s"
fi

# (f) Full test suite
info "(f) Full ontology + knowledge test suite"
go test ./internal/ontology/... ./internal/knowledge/ecl/... -count=1 2>&1 | tail -10 || fail "test suite failed"
pass "(f) test suite"

echo ""
echo -e "${GREEN}=== Phase 40 Gate: ALL PASSED ===${NC}"
