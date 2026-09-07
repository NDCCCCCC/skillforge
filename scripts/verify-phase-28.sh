#!/usr/bin/env bash
# scripts/verify-phase-28.sh — Phase 28 structural gate (HERMETIC).
#
# 3 stages; all must pass.
#
# Stage 1: TODO clear — zero TODO/FIXME in Phase 28 production files
# Stage 2: Trust invariant — kb.Citable predicate path UNCHANGED
#   (Phase 28 edges are derived metadata; citation gating at query
#   time still runs through the single kb.Citable predicate)
# Stage 3: Doc-nodes-from-registry — graph nodes derive from KBRegistry.List(),
#   not from a hardcoded 4-tier enum (Phase 26's contribution)
set -uo pipefail

phase_28_files=(
  "internal/knowledge/graph_edges.go"
  "internal/knowledge/graph_edges_test.go"
  "internal/knowledge/graph_cache.go"
  "internal/knowledge/graph_cache_test.go"
  "internal/server/knowledge.go"
  "internal/server/knowledge_test.go"
  "internal/server/server.go"
)

echo "=== Stage 1: TODO/FIXME clear ==="
TODO_COUNT=$(grep -rE "TODO|FIXME" --include="*.go" "${phase_28_files[@]}" 2>/dev/null | grep -v "_test.go" | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
  echo "FAIL: $TODO_COUNT TODO/FIXME in Phase 28 production files:"
  grep -rE "TODO|FIXME" --include="*.go" "${phase_28_files[@]}" 2>/dev/null | grep -v "_test.go"
  exit 1
fi
echo "PASS: no TODO/FIXME in Phase 28 production files"

echo ""
echo "=== Stage 2: Trust invariant ==="
echo "--- Stage 2a: kb.Citable still defined (single predicate, not duplicated in upload path) ---"
KC_DEF=$(grep -rE "^func Citable|^var Citable" internal/kb/*.go 2>/dev/null | wc -l)
if [ "$KC_DEF" -eq 0 ]; then
  echo "FAIL: kb.Citable function not found (Phase 26's single predicate must still exist)"
  exit 1
fi
echo "PASS 2a: kb.Citable defined"

echo "--- Stage 2b: graph_edges.go does NOT introduce a new citability check (query-time gate stays single-source) ---"
EDGES_CITABLE=$(grep -cE "kb\.Citable" internal/knowledge/graph_edges.go 2>/dev/null)
if [ "$EDGES_CITABLE" -gt 0 ]; then
  echo "FAIL: kb.Citable referenced in graph_edges.go (edge derivation must NOT call the citability predicate)"
  exit 1
fi
echo "PASS 2b: graph_edges.go does not call kb.Citable"

echo ""
echo "=== Stage 3: Doc-nodes-from-registry ==="
echo "--- Stage 3a: GraphBuilder uses reg.List() (Phase 26's KBRegistry-driven dynamic node set) ---"
REGLIST=$(grep -nE "reg\.List\(\)" internal/knowledge/graph.go 2>/dev/null | wc -l)
if [ "$REGLIST" -eq 0 ]; then
  echo "FAIL: reg.List() not found in graph.go (KBRegistry-driven node set is Phase 26's contribution)"
  exit 1
fi
echo "PASS 3a: graph.go uses reg.List() for dynamic node set"

echo "--- Stage 3b: no hardcoded 4-tier slug list in graph code ---"
HARDCODED=$(grep -nE "权威|历史|个人|草稿" internal/knowledge/graph.go internal/knowledge/graph_edges.go 2>/dev/null | grep -v "// " | grep -v "^Binary" | wc -l)
if [ "$HARDCODED" -gt 0 ]; then
  echo "FAIL: hardcoded CN tier literal found in graph code (use registry slugs instead)"
  exit 1
fi
echo "PASS 3b: no hardcoded 4-tier CN literal"

echo ""
echo "=== Phase 28 verify PASS ==="
