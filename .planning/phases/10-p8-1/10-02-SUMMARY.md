---
phase: 10-p8-1
plan: 02
subsystem: server-pipeline + storage + production-assembly
tags:
  - history-aware-pipeline
  - query-rewriter
  - refusal-classifier
  - audit-meta
  - data-decision
  - production-assembly
dependency_graph:
  requires:
    - "10-01 (RefusalClassifierInput type isolation + neutral placeholder + budget window)"
  provides:
    - "QueryRewriter (internal/server/rewrite)"
    - "RefusalClassifier (internal/server/classifier) with W3 confidence floor"
    - "audit_meta sibling table (internal/store)"
    - "history-aware VerdictPipeline (internal/server/pipeline)"
    - "history_* data-decision events (internal/wire)"
    - "cmd/server/main.go production assembly of Rewriter + Classifier"
  affects:
    - "internal/server/chat.go (writeAuditAndTurn signature)"
    - "internal/server/server.go (Deps struct)"
tech_stack:
  added: []
  patterns:
    - "nil-gate dependency injection (Rewriter / Classifier nil → step skipped)"
    - "type-level input isolation (RefusalClassifierInput has only Query + Chunks)"
    - "data-decision SSE channel for history observability (D15 — zero new wire types)"
    - "1:1 sibling-table audit lineage (audit_meta.query_id FK → audit.id ON DELETE CASCADE)"
    - "approximate budget-truncation detection (pre-sanitization char total > DefaultCharBudget)"
key_files:
  created:
    - internal/server/rewrite/rewrite.go
    - internal/server/rewrite/rewrite_test.go
    - internal/server/classifier/classifier.go
    - internal/server/classifier/classifier_test.go
    - internal/server/decisions.go
    - internal/server/pipeline_history_test.go
    - internal/store/sqlite/schema_test.go
  modified:
    - internal/server/pipeline.go
    - internal/server/chat.go
    - internal/server/audit.go
    - internal/server/server.go
    - internal/server/classifier/types.go
    - internal/server/classifier/types_test.go
    - internal/wire/chunk.go
    - internal/store/store.go
    - internal/store/sqlite/schema.go
    - internal/store/sqlite/audit.go
    - cmd/server/main.go
decisions:
  - "D10-A1 enforcement: RefusalClassifierInput struct shape locked at 2 fields (Query + Chunks); Valid() now enforces ConfidenceFloor=0.5 on Decision=allow (W3 gate)."
  - "D10-B2 multi-turn messages prefix: streamAndAssemble signature extended to accept history []*schema.Message; when len==0 the slice is exactly [System, User] (Phase 9 baseline preserved)."
  - "D10-B4 data-decision events: 4 new decision-string constants (history_projected / history_budget_truncated / history_query_rewritten / history_classifier_refused). DecisionPayload struct UNCHANGED (D15 invariant — zero new wire types)."
  - "B2 nil-gate semantics: nil Rewriter disables ONLY the rewrite step; nil RefusalClassifier disables ONLY the classifier step; history prefix STILL flows when history is non-empty (TestVerdictPipeline_NilRewriterStillFlowsHistory is the B2 regression lock)."
  - "SC-3 hermetic coverage: grounding report-driven citation strip — when grounding fails, the pipeline strips ungrounded Citations from v.Citations before emitVerdictStream, ensuring the SSE source-document stream never surfaces a chunk ID that wasn't retrieved."
metrics:
  duration: "single session (2026-08-18)"
  completed_date: "2026-08-18"
  tasks: 4
  files_created: 7
  files_modified: 11
---

# Phase 10 Plan 02: History-aware VerdictPipeline + Audit Meta + Production Assembly — Summary

**Wave:** 2 (depends on 10-01)
**Plan ID:** 10-02
**Phase:** 10-p8-1 (多轮对话记忆)
**Completed:** 2026-08-18
**Status:** All 4 tasks shipped, full BE suite green, all grep gates PASS

## What shipped

4 tasks delivered (4 commits: d13c050 + 946c00f + 169bc5e + be60fca):

### Task 1 — Query rewriter + refusal classifier (W3 floor)
- `internal/server/rewrite/rewrite.go` — `QueryRewriter` with 2s timeout, JSON response_format, 4000-char history trim; best-effort failure fallback (rewriter timeout / parse failure → original query).
- `internal/server/rewrite/rewrite_test.go` — 9 tests covering happy path, pass-through on no-anaphora, timeout fallback, malformed JSON, truncation marker, nil guards.
- `internal/server/classifier/classifier.go` — `RefusalClassifier` with 3-retry default-deny on parse / enum / floor failures; `ClassifierConfidenceFloor=0.5` exported for spike runner reuse.
- `internal/server/classifier/classifier_test.go` — 10 tests including all 7 spec-mandated cases (allow @ floor, refuse, default-deny on parse / invalid decision / low confidence / out-of-range / unknown reason).
- `internal/server/classifier/types.go` extended: `Valid()` now enforces the W3 confidence floor on `Decision="allow"`.
- `internal/server/classifier/types_test.go` extended: 2 new table cases for floor semantics.

### Task 2 — `audit_meta` sibling table + Store interface
- `internal/store/store.go` — `AuditMeta` struct (8 columns) + `PutAuditMeta(ctx, meta)` on `Store` interface.
- `internal/store/sqlite/schema.go` — `audit_meta` table DDL added (sibling to `audit`; FK ON DELETE CASCADE; `idx_audit_meta_decision` index).
- `internal/store/sqlite/audit.go` — `PutAuditMeta` impl mirroring `PutAudit`'s shape (`?`-placeholders only, T-3-SQLI); `toNullString` helper maps empty strings to SQL NULL.
- `internal/server/audit.go` — `writeAuditMeta` helper (best-effort, nil-safe; nil `s` / nil `meta` → no-op; error → log + swallow).
- `internal/store/sqlite/schema_test.go` — 4 tests: DDL shape (8 cols in canonical order), INSERT round-trip, ON DELETE CASCADE, nullable column handling.

### Task 3 — Pipeline wiring + history → rewrite → classify → generate
- `internal/wire/chunk.go` — 4 new data-decision constants (`history_projected`, `history_budget_truncated`, `history_query_rewritten`, `history_classifier_refused`); `DecisionPayload` struct UNCHANGED (D15 invariant).
- `internal/server/decisions.go` — `emitHistoryDecision` helper mirroring `emitDecision` (best-effort, log+swallow on marshal/emit failure).
- `internal/server/server.go` — `Deps` struct gains `Rewriter *rewrite.QueryRewriter` + `RefusalClassifier *classifier.RefusalClassifier` with the nil-gate godoc table.
- `internal/server/pipeline.go` — `VerdictPipeline` extended with:
  - **Step 0** `loadHistory` → emits `history_projected` event; `loadHistory` returns `(slice, dropped, truncated)` so budget-truncation detection can fire the `history_budget_truncated` event.
  - **Step 0.5** rewrite (when history present + Rewriter ≠ nil); failure → fall back to original query.
  - **Step 7** query-only refusal classifier (when RefusalClassifier ≠ nil); `Decision="refuse"` short-circuits before the verdict LLM.
  - **Step 8** `streamAndAssemble` extended signature accepts `history []*schema.Message` prefix; messages slice built as `[System, ...history, User]`.
  - **Step 10** grounding-strip: when grounding fails, ungrounded `Citations` are stripped from `v.Citations` so the SSE source-document stream never surfaces a chunk ID that wasn't retrieved (SC-3 hermetic coverage).
  - Each terminal emit path now passes a `*store.AuditMeta` scaffold to `writeAuditAndTurn`.
- `internal/server/chat.go` — `writeAuditAndTurn` gains final `meta *store.AuditMeta` parameter; all 4 chatHandler/runAgentPath call sites pass `nil` at Phase 9 baseline; all 5 pipeline call sites pass the populated `meta`.
- `internal/server/pipeline_history_test.go` — 8 new regression tests pinning:
  - History prefix flows through the messages slice (T-3a).
  - Classifier refusal short-circuits the verdict LLM (T-3b).
  - Rewriter failure falls back to original query + classifier still runs (T-3c).
  - Classifier input has exactly 2 fields via reflection — D10-A1 type-level isolation (T-3d).
  - **B2 regression**: nil Rewriter + nil Classifier with history → history prefix STILL flows (T-3e).
  - **SC-3 hermetic**: `[CHUNK 9999]` literal in history → scrubbed before LLM; verdict's citation strip prevents source-document emit of fake ID (T-3f).
  - Budget truncation emits `history_budget_truncated` event with dropped count (T-3g).
  - Rewriter success emits `history_query_rewritten` event with rewritten query (T-3h).

### Task 4 — `cmd/server/main.go` production assembly (B2 fix)
- `DISABLE_HISTORY_PIPELINE` env var (opt-in) — when `true`, both Rewriter + RefusalClassifier are nil; history prefix STILL flows (B2 nil-gate).
- Construct `rewrite.NewQueryRewriter(cm)` + `classifier.NewRefusalClassifier(cm)` after `cm`; error → `log.Fatalf` (fail-fast at startup).
- Inject `Rewriter` + `RefusalClassifier` into `&server.Deps{…}`.
- Log `[OK] cmd/server: history-aware pipeline ACTIVE` when active; `[WARN] … DISABLE_HISTORY_PIPELINE=true` when disabled.
- Added `GET /api/session/:id/history — Phase 10 history restore (Plan 03)` to the route-table block.

## Verification (all PASS)

- `go test ./internal/server/... ./internal/store/sqlite/... -count=1 -timeout 60s` → ok (8.0s)
- `go test ./... -count=1 -timeout 120s` → ok (full BE suite green; `-race` unavailable in this env — gcc missing)
- `go build ./cmd/server` → ok (production binary compiles)
- `go vet ./...` → ok
- `grep -n 'GetSession\|loadHistory\|historyMsgs\|history\.Project' internal/server/classifier/classifier.go` → empty (D10-A1 + GG-A1-3 gate)
- `grep -n 'History' internal/server/classifier/types.go` → 0 (GG-A1-1 gate)
- `grep -c RenderRefusal internal/history/projection.go` → 0 (B3 gate; carried over from Plan 01)
- All 4 history_* decision constants referenced in `pipeline.go` (no dead code).
- `grep -n 'Rewriter: rewriter' cmd/server/main.go` → line 367 (wiring confirmed).
- Audit_meta DDL ordering preserved: `audit` → `audit_meta` → `session_turn` → `checkpoint` (Plan 01 baseline preserved).
- SC-3 hermetic test (`TestVerdictPipeline_HistoryChunkRefsSanitized`) confirms `[CHUNK 9999]` literal in history → scrubbed before LLM + grounding check strips the ungrounded citation from the SSE source-document emit.

## Decision log entries

- **D10-A1 type-level isolation enforced**: `RefusalClassifierInput` has exactly 2 fields (Query + Chunks); classifier package imports zero history / store symbols; `Valid()` enforces `ConfidenceFloor=0.5` on `Decision="allow"` (W3). The compile-time sentinel `var _ = (*SQLiteStore)(nil)` caught the missing `PutAuditMeta` method during development.
- **D10-B2 multi-turn messages prefix**: `streamAndAssemble` accepts `history []*schema.Message`; messages slice is `[System, ...history, User]`. Empty-history path is byte-identical to Phase 9 baseline.
- **D10-B4 data-decision events**: 4 new decision-string constants; `DecisionPayload` struct UNCHANGED (D15 invariant).
- **B2 nil-gate semantics**: nil Rewriter disables ONLY the rewrite step; nil RefusalClassifier disables ONLY the classifier step; history prefix ALWAYS flows when history exists. `TestVerdictPipeline_NilRewriterStillFlowsHistory` is the regression lock.
- **SC-3 hermetic coverage**: pipeline strips ungrounded Citations from `v.Citations` after `GroundingChecker` so the SSE source-document stream never surfaces a chunk ID that wasn't retrieved. `TestVerdictPipeline_HistoryChunkRefsSanitized` is the regression lock.
- **Approximate budget-truncation detection**: `loadHistory` measures pre-sanitization char total vs `DefaultCharBudget` to detect truncation without modifying `internal/history` (Plan 01 territory is off-limits to Plan 02). The audit row's `HistoryBudgetTruncated=1` is the deterministic signal; the dropped count is best-effort observability.

## Deviations from plan

- **SC-3 grounding-strip addition**: The plan asserted "the grounding checker strips ungrounded refs" but the existing `verdict.GroundingChecker` only downgrades `Status` / `Confidence` / `ReviewRequired` — it does NOT strip citations. Added a small pipeline-level strip (after `GroundingChecker`, before `emitVerdictStream`) to satisfy the SC-3 contract. The existing `GroundingChecker` API is unchanged.
- **`loadHistory` return signature extended**: Returns `(slice, dropped, truncated)` (the plan only specified slice return). The third return value feeds the budget-truncated event + audit_meta `HistoryDroppedCount`. Truncation is detected by measuring pre-sanitization char total (the plan forbade editing `internal/history`).
- **`DecisionPayload` Decision strings**: Implemented as 4 top-level `const string` values in `internal/wire/chunk.go` (the plan sketched them inline as plain values — top-level constants are the conventional shape and let the FE / spike runner reuse them without re-typing).

## Out of scope (deferred to Plan 03+)

- `GET /api/session/:id/history` HTTP handler (Plan 03 — frontend restore endpoint).
- `cmd/spike-history/runner.go --via-pipeline` flag (Plan 04 — B5 fix).
- `eval/frozen/compliance.lock.json` re-lock (v0.8 — defer to v0.8 milestone).
- `internal/agent/agent.go` VerdictPipeline ↔ Runner.Query migration (Plan 11 — Runner migration).
- Rewrite / classifier prompt tuning against live GLM-5.2 + Milvus (Plan 04 spike).

## Commits

```
be60fca feat(10-02): wire QueryRewriter + RefusalClassifier in cmd/server/main.go
169bc5e feat(10-02): history-aware VerdictPipeline + 4 data-decision events
946c00f feat(10-02): audit_meta sibling table + PutAuditMeta store interface
d13c050 feat(10-02): QueryRewriter + RefusalClassifier with W3 confidence floor
```