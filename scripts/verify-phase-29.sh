#!/usr/bin/env bash
# scripts/verify-phase-29.sh — Phase 29 structural gate (HERMETIC).
#
# 4 stages; all must pass. Mirrors verify-phase-28.sh (3 stages) with the
# Phase 29 additions: FBK-05 trust red line and the D15 data-turn pairing.
#
# Stage 1: TODO clear — zero TODO/FIXME in Phase 29 production files
#   (Go + TS/TSX; only TODO( / TODO: / FIXME( / FIXME: count — the eino
#   type name deep.TODO must never match; verify-phase-26 convention)
# Stage 2: FBK-05 trust red line — feedback surface is INSERT-only and
#   read-only toward the knowledge base:
#   2a. internal/server/feedback.go + internal/store/sqlite/{feedback,tickets}.go
#       contain zero "UPDATE " / "DELETE " / "Insert(" verbs
#   2b. internal/server/feedback.go touches the retriever ONLY through
#       GetChunk (>=1) — zero "Search(" and zero "Indexer"
#   2c. the same three files import no internal/knowledge (zero Indexer
#       / knowledge-package references)
# Stage 3: D15 pairing — "data-turn" exists on BOTH wire halves
#   (Go emitter in internal/wire/chunk.go AND TS consumer in
#   web/src/types/wire-types.ts). Either half missing = the turn-id
#   contract silently died on that side.
# Stage 4: DDL discipline — the feedback/tickets write surface is
#   INSERT-only: zero "DO UPDATE" and >= 4 "DO NOTHING" across
#   internal/store/sqlite/{schema,feedback,tickets}.go (the ON CONFLICT
#   clauses live in the DML files, schema.go carries the DDL), and the 4
#   additive tables each appear with CREATE TABLE IF NOT EXISTS in
#   schema.go.
set -uo pipefail

phase_29_files=(
  "internal/store/sqlite/schema.go"
  "internal/store/sqlite/feedback.go"
  "internal/store/sqlite/tickets.go"
  "internal/store/sqlite/session.go"
  "internal/server/feedback.go"
  "internal/server/tickets.go"
  "internal/server/feedback_wire.go"
  "internal/server/chat.go"
  "internal/server/pipeline.go"
  "internal/server/runner_setup.go"
  "internal/server/handlers_session_history.go"
  "internal/server/server.go"
  "internal/ticket/ticket.go"
  "internal/ticket/registry.go"
  "internal/ticket/webhook.go"
  "internal/wire/chunk.go"
  "web/src/components/chat/parts/FeedbackRow.tsx"
  "web/src/components/chat/MessageBubble.tsx"
  "web/src/components/layout/SfToast.tsx"
  "web/src/components/admin/FeedbackMetricsPanel.tsx"
  "web/src/components/admin/ReviewQueuePanel.tsx"
  "web/src/lib/chat-restore.ts"
  "web/src/types/wire-types.ts"
  "web/src/store/app-store.ts"
  "cmd/server/main.go"
)

# The three files that carry the FBK-05 trust red line (Stage 2 / Stage 4).
fbk05_files=(
  "internal/server/feedback.go"
  "internal/store/sqlite/feedback.go"
  "internal/store/sqlite/tickets.go"
)

echo "=== Stage 1: TODO/FIXME clear ==="
# Only TODO( / TODO: / FIXME( / FIXME: is a TODO. Whole-word matches would
# false-positive on the eino type name deep.TODO (measured: 4 reference
# sites inside this file list itself) — verify-phase-26 convention.
TODO_COUNT=$(grep -nE "TODO\(|TODO:|FIXME\(|FIXME:" "${phase_29_files[@]}" 2>/dev/null \
  | grep -v "_test.go" | grep -v "\.test\." | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
  echo "FAIL: $TODO_COUNT TODO/FIXME in Phase 29 production files:"
  grep -nE "TODO\(|TODO:|FIXME\(|FIXME:" "${phase_29_files[@]}" 2>/dev/null \
    | grep -v "_test.go" | grep -v "\.test\."
  exit 1
fi
echo "PASS: no TODO/FIXME in Phase 29 production files"

# Anti-false-positive proof: the eino type name deep.TODO must never match.
DEEP_TODO_REFS=$(grep -cE "deep\.TODO" "${phase_29_files[@]}" 2>/dev/null | awk -F: '{s+=$NF} END {print s+0}')
if [ "$DEEP_TODO_REFS" -eq 0 ]; then
  echo "WARN: no deep.TODO reference found in the file list (fixture drifted; gate unaffected)"
else
  DEEP_TODO_MATCHED=$(grep -nE "TODO\(|TODO:|FIXME\(|FIXME:" "${phase_29_files[@]}" 2>/dev/null \
    | grep -E "deep\.TODO" | wc -l)
  if [ "$DEEP_TODO_MATCHED" -gt 0 ]; then
    echo "FAIL: gate regex unexpectedly matches a deep.TODO reference — tighten the pattern, do not whitelist the file"
    exit 1
  fi
  echo "PASS: deep.TODO references present ($DEEP_TODO_REFS) but correctly not matched"
fi

echo ""
echo "=== Stage 2: FBK-05 trust red line (feedback never mutates trust) ==="
echo "--- Stage 2a: feedback surface is INSERT-only (zero UPDATE/DELETE/Insert verbs) ---"
for f in "${fbk05_files[@]}"; do
  VERB_COUNT=$(grep -cE "UPDATE |DELETE |Insert\(" "$f" 2>/dev/null)
  if [ "$VERB_COUNT" -gt 0 ]; then
    echo "FAIL: $VERB_COUNT mutation verb(s) in $f (FBK-05: feedback must never rewrite trust)"
    grep -nE "UPDATE |DELETE |Insert\(" "$f"
    exit 1
  fi
done
echo "PASS 2a: feedback/tickets store+handler files contain zero mutation verbs"

echo "--- Stage 2b: retriever access is read-only (GetChunk only; no Search, no Indexer) ---"
GETCHUNK=$(grep -cE "GetChunk" internal/server/feedback.go 2>/dev/null)
if [ "$GETCHUNK" -eq 0 ]; then
  echo "FAIL: GetChunk not found in internal/server/feedback.go (chunk-tier resolution must stay a read)"
  exit 1
fi
SEARCH_HITS=$(grep -cE "Search\(" internal/server/feedback.go 2>/dev/null)
if [ "$SEARCH_HITS" -gt 0 ]; then
  echo "FAIL: retriever Search() referenced in internal/server/feedback.go (feedback must not query the index)"
  exit 1
fi
INDEXER_HITS=$(grep -cE "Indexer" internal/server/feedback.go 2>/dev/null)
if [ "$INDEXER_HITS" -gt 0 ]; then
  echo "FAIL: Indexer referenced in internal/server/feedback.go (feedback must not touch the index surface)"
  exit 1
fi
echo "PASS 2b: retriever call surface is GetChunk only ($GETCHUNK reference sites)"

echo "--- Stage 2c: no internal/knowledge import in the feedback surface ---"
for f in "${fbk05_files[@]}"; do
  KB_HITS=$(grep -cE "internal/knowledge" "$f" 2>/dev/null)
  if [ "$KB_HITS" -gt 0 ]; then
    echo "FAIL: internal/knowledge referenced in $f (feedback/review must not reach the KB package)"
    exit 1
  fi
done
echo "PASS 2c: zero internal/knowledge references in the feedback surface"

echo ""
echo "=== Stage 3: D15 data-turn pairing (Go emitter AND TS consumer) ==="
GO_DT=$(grep -cE "data-turn" internal/wire/chunk.go 2>/dev/null)
TS_DT=$(grep -cE "data-turn" web/src/types/wire-types.ts 2>/dev/null)
if [ "$GO_DT" -eq 0 ]; then
  echo "FAIL: 'data-turn' missing from internal/wire/chunk.go (Go emitter half of D15 pairing gone)"
  exit 1
fi
if [ "$TS_DT" -eq 0 ]; then
  echo "FAIL: 'data-turn' missing from web/src/types/wire-types.ts (TS consumer half of D15 pairing gone)"
  exit 1
fi
echo "PASS: data-turn present on both halves (Go: $GO_DT, TS: $TS_DT)"

echo ""
echo "=== Stage 4: DDL discipline (INSERT-only write surface, 4 additive tables) ==="
echo "--- Stage 4a: zero DO UPDATE across the write surface ---"
DO_UPDATE=$(grep -cE "DO UPDATE" internal/store/sqlite/schema.go internal/store/sqlite/feedback.go internal/store/sqlite/tickets.go 2>/dev/null \
  | awk -F: '{s+=$NF} END {print s+0}')
if [ "$DO_UPDATE" -gt 0 ]; then
  echo "FAIL: $DO_UPDATE DO UPDATE clause(s) in the feedback/tickets write surface (upsert would rewrite rows)"
  grep -nE "DO UPDATE" internal/store/sqlite/schema.go internal/store/sqlite/feedback.go internal/store/sqlite/tickets.go
  exit 1
fi
echo "PASS 4a: zero DO UPDATE in schema.go + feedback.go + tickets.go"

echo "--- Stage 4b: DO NOTHING clauses >= 4 across the write surface ---"
# The ON CONFLICT DO NOTHING clauses live in the DML files (feedback.go /
# tickets.go); schema.go carries the DDL plus the Phase-15 audit seeds.
DO_NOTHING=$(grep -cE "DO NOTHING" internal/store/sqlite/schema.go internal/store/sqlite/feedback.go internal/store/sqlite/tickets.go 2>/dev/null \
  | awk -F: '{s+=$NF} END {print s+0}')
if [ "$DO_NOTHING" -lt 4 ]; then
  echo "FAIL: only $DO_NOTHING DO NOTHING clause(s) across the write surface (expected >= 4 INSERT-only writes)"
  exit 1
fi
echo "PASS 4b: $DO_NOTHING DO NOTHING clauses (INSERT-only shape holds)"

echo "--- Stage 4c: 4 additive tables declared with CREATE TABLE IF NOT EXISTS ---"
for t in feedback feedback_chunk_refs feedback_dispositions tickets; do
  T_COUNT=$(grep -cE "CREATE TABLE IF NOT EXISTS $t" internal/store/sqlite/schema.go 2>/dev/null)
  if [ "$T_COUNT" -eq 0 ]; then
    echo "FAIL: table '$t' missing from internal/store/sqlite/schema.go (additive DDL erased)"
    exit 1
  fi
done
echo "PASS 4c: feedback / feedback_chunk_refs / feedback_dispositions / tickets all declared"

echo ""
echo "=== Phase 29 verify PASS ==="
