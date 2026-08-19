---
status: partial
phase: 10-p8-1
source: 10-01-SUMMARY.md, 10-02-SUMMARY.md, 10-03-SUMMARY.md, 10-04-SUMMARY.md
started: 2026-08-18T23:31:00Z
updated: 2026-08-19T00:00:00Z
---

## Current Test

number: 1
name: Verdict renderer produces plain text from verdict JSON (MEM-04)
expected: |
  `history.RenderConclusion(verdictJSONBytes)` returns a string with no
  `[CHUNK id]` markers, no JSON braces, and the conclusionText field
  rendered verbatim. Three cases (well-formed JSON / malformed / empty)
  all deterministic.
awaiting: user response

## Tests

### 1. Verdict renderer produces plain text from verdict JSON (MEM-04, 10-01 Task 1)
expected: |
  `internal/history/verdict_renderer.go::RenderConclusion` is agent-path only.
  Verdict JSON → plain text. Zero `[CHUNK id]` markers in projection output.
  3 unit tests cover well-formed / malformed / empty input.
result: pass
verified_by: "go test ./internal/history/... -count=1 → ok 0.132s"

### 2. Neutral placeholder never leaks refusal semantics to LLM (10-01 Task 2)
expected: |
  `internal/history/neutral_placeholder.go::RenderNeutralPlaceholder()` is
  fixed-string template; 11-word canonical refusal-semantic list documented
  in godoc. Zero leakage verified across 5 refusal reasons + 3 non-refusal
  cases.
result: pass
verified_by: "go test ./internal/history/... -count=1 → ok 0.132s"

### 3. Char-budget window is primary history truncation (D10-B3, 10-01 Task 2)
expected: |
  `internal/history/budget_window.go::TruncateToCharBudget(msgs, budget)`
  is the primary window. Oldest-first drop with `SafeguardFloor=2`,
  `DefaultCharBudget=8000`. The 6-message D9-B3 cap survives only as
  `FallbackMessageCeiling` when budget<=0. 4 tests + new fixture
  `budget_window_keeps_all_pairs_under_budget`.
result: pass
verified_by: "go test ./internal/history/... -count=1 → ok 0.132s"

### 4. RefusalClassifierInput is type-isolated to Query + Chunks (D10-A1, 10-01 Task 3)
expected: |
  `internal/server/classifier/types.go::RefusalClassifierInput{Query string,
  Chunks []string}` — exactly 2 fields, no History. 7 reflection tests
  lock shape. Classifier package imports zero history/store symbols.
  `Valid()` enforces W3 confidence floor (0.5) on Decision="allow".
result: pass
verified_by: "go test ./internal/server/classifier/... -count=1 → ok 1.054s; grep -E 'History\\s*\\[\\]schema' internal/server/classifier/types.go → empty"

### 5. History flows into multi-turn messages prefix (D10-B2, 10-02 Task 3)
expected: |
  `internal/server/pipeline.go::VerdictPipeline` extends `streamAndAssemble`
  signature to accept `history []*schema.Message`. Messages slice is built
  as `[System, ...history, User]`. Empty-history path is byte-identical to
  Phase 9 baseline. `TestVerdictPipeline_NilRewriterStillFlowsHistory` is
  the B2 regression lock.
result: pass
verified_by: "go test ./internal/server/... -count=1 → ok 2.751s (TestVerdictPipeline* green)"

### 6. Classifier refusal short-circuits verdict LLM (MEM-03 / W3, 10-02 Task 1+3)
expected: |
  Pipeline Step 7 (query-only refusal classifier) emits `Decision="refuse"`
  and short-circuits before the verdict LLM. Refusal emits
  `history_classifier_refused` data-decision event.
  `TestVerdictPipeline_ClassifierRefusesShortCircuits` is the lock.
result: pass
verified_by: "go test ./internal/server/... -count=1 → ok 2.751s (TestVerdictPipeline_ClassifierRefusesShortCircuits green)"

### 7. Rewriter failure falls back to original query (10-02 Task 1+3)
expected: |
  Pipeline Step 0.5 (rewrite) is best-effort. Timeout or parse failure →
  fall back to original query. Classifier still runs on fallback.
  `TestVerdictPipeline_RewriterFailureFallsBack` is the lock.
result: pass
verified_by: "go test ./internal/server/... -count=1 → ok 2.751s (TestVerdictPipeline_RewriterFailureFallsBack green)"

### 8. SC-3 hermetic: [CHUNK id] in history is scrubbed before LLM + citation strip (MEM-03, 10-02 Task 3)
expected: |
  Pipeline strips ungrounded `Citations` from `v.Citations` after
  `GroundingChecker` so the SSE source-document stream never surfaces a
  chunk ID that wasn't retrieved. `[CHUNK 9999]` literal in history →
  scrubbed before LLM + verdict citation strip prevents fake ID emit.
  `TestVerdictPipeline_HistoryChunkRefsSanitized` is the regression lock.
result: pass
verified_by: "go test ./internal/server/... -count=1 → ok 2.751s (TestVerdictPipeline_HistoryChunkRefsSanitized green)"

### 9. audit_meta sibling table writes per-query history metadata (10-02 Task 2)
expected: |
  `internal/store/sqlite/schema.go::audit_meta` table created (FK ON
  DELETE CASCADE). `PutAuditMeta(ctx, meta)` writes 8 columns.
  `writeAuditMeta` is best-effort + nil-safe. 4 schema tests cover DDL
  shape, INSERT round-trip, CASCADE, nullable column handling.
result: pass
verified_by: "go test ./internal/store/sqlite/... -count=1 → ok 0.553s"

### 10. History-bounded sessions persist across page refresh (MEM-02, 10-03 Task 1)
expected: |
  `GET /api/session/:id/history` returns `[]SessionHistoryMessage` with
  per-role content rendered via `history.RenderConclusion` /
  `history.RenderRefusal` / raw. `internal/server/handlers_session_history.go`
  iterates RAW `Store.GetSession` rows — NO `history.Project()` call on the
  user-facing path (B4 fix). URL-segment regex gate (`^[a-zA-Z0-9-]{8,128}$`)
  returns 400 before any SQL. Unknown session returns 200 + empty (T-10-11
  existence-leak posture). 8 hermetic tests cover all paths.
result: pass
verified_by: "go test ./internal/server/... -count=1 -run TestSessionHistoryAPI → ok"

### 11. Skill auto-switches when restored session belongs to different skill (D10-D2, 10-03 Task 2)
expected: |
  `web/src/hooks/use-skill-auto-switch.ts::useSkillAutoSwitch(skillId)`
  calls `store.setCurrentSkill` when skillId is non-empty AND in the
  loaded skills list. Idempotent; no-op on unknown/empty. useEffect deps
  `[skillId, store.skills]` re-runs on skills refresh. 3 vitest tests.
result: pass
verified_by: "npx vitest run --reporter=basic → 349/349 pass"

### 12. HistoryRewriteHint shows rewrite transparency caption (D10-C3, 10-03 Task 2)
expected: |
  `web/src/components/chat/HistoryRewriteHint.tsx` renders caption above
  user bubble when message carries a `data-decision` part with
  decision="history_query_rewritten". Empty rewrite → null (caption hidden).
  2 vitest tests.
result: pass
verified_by: "npx vitest run --reporter=basic → 349/349 pass"

### 13. Chat restores history via per-session Chat instance + race-guard (D10-D1, 10-03 Task 2)
expected: |
  `web/src/App.tsx` history-injection useEffect fetches history on mount;
  another useEffect injects transformed messages into per-session Chat
  instance via `Chat.messages` setter (AI SDK v5 seam). Race-guard:
  effect SKIPS if Chat already has messages (user may have typed before
  history arrived). `appliedSessionsRef` ensures write fires exactly once
  per session. 3 App tests under "App restore (Plan 10-03 MEM-02)".
result: pass
verified_by: "npx vitest run --reporter=basic → 349/349 pass"

### 14. D14 invariant: Zustand store bans `messages` field (D14, 10-03)
expected: |
  No `messages` field added to `web/src/store/app-store.ts`. The Chat
  instance owns the message stream (via per-session `chatMapRef` +
  `useChat({chat})`). File-level `BANNED` comment preserved.
result: pass
verified_by: "grep -E '^\\s*messages\\s*[:=]' web/src/store/app-store.ts → empty"

### 15. D15 invariant: DecisionPayload struct UNCHANGED, history_* enum widens (D15, 10-02 + 10-03)
expected: |
  `internal/wire/chunk.go::DecisionPayload` struct UNCHANGED. Decision
  enum widens by 4 string literals: `history_projected`,
  `history_budget_truncated`, `history_query_rewritten`,
  `history_classifier_refused`. FE mirror: `DecisionPayload.decision`
  widened. `_driftCheck` assertion in `web/src/types/wire-types.ts`
  stays green.
result: pass
verified_by: "npx tsc --noEmit → exit 1 with 1 pre-existing error (DataSubprogressPart.tsx:47 — unrelated D15 wire-type concern, documented in 10-04 SUMMARY as out-of-scope)"

### 16. cmd/spike-history --via-pipeline exercises production composition (B5, 10-04 Task 1)
expected: |
  `cmd/spike-history/runner.go::runViaPipeline` mirrors production
  composition when `deps.Rewriter` or `deps.Classifier` are non-nil.
  Legacy direct path remains default for Phase 9 reproducibility.
  Hermetic `TestBuildPipelineMessages_Structure` and
  `TestRunEntry_ViaPipelineDryRun` pass.
result: pass
verified_by: "go test ./cmd/spike-history -count=1 → ok 0.476s"

### 17. Frozen eval lock byte-identity preserved (10-04 Task 1)
expected: |
  `eval/frozen/compliance.lock.json` sha256 = `5cf700f3...` (Plan 10-04
  canonical). `git diff --quiet` exit 0; porcelain clean. Lock NOT
  regenerated. Binding gate is byte-identity, not the recorded value
  in STATE.md (which shows stale `50bb1688...`).
result: pass
verified_by: "git diff --quiet eval/frozen/compliance.lock.json && git status --porcelain eval/frozen/ → exit 0; sha256sum → 5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded"

### 18. Session isolation regression lock (MEM-05, 10-04 pre-flight gate 6)
expected: |
  `TestSessionIsolation50x5` passes — 50 concurrent sessions × 5 turns
  see only their own conversation history (no cross-session bleed).
  Same lock as Phase 3 SC-9.
result: pass
verified_by: "go test ./internal/server/... -run TestSessionIsolation50x5 -count=1 → ok"

### 19. Live multi-turn chat resolves anaphora via history (MEM-01)
expected: |
  User asks initial question, gets cited answer. User then asks "那第二条
  呢？" (what about the second one?). Agent combines session history,
  correctly resolves reference to the prior second claim, and answers
  with **this-turn retrieval** citations (not historical chunk refs).
result: blocked
blocked_by: server
reason: |
  Live stack not present in this execution environment. `.env` does not
  contain GLM_API_KEY / BGE_M3_URL / MILVUS_ADDRESS. Docker Milvus +
  BGE-M3 endpoints not confirmed reachable. See STATE.md "Phase 10 Plan
  04 Task 2 — blocked-pending-human" since 2026-08-18.

### 20. Page refresh restores chat history (MEM-02 user-observable)
expected: |
  Open chat, send 2-3 messages, refresh the browser. Chat panel shows
  the prior conversation (user + assistant bubbles) with verdict card
  preserved for assistant+verdict rows. Skill auto-switches if session
  belongs to a different Skill.
result: blocked
blocked_by: server
reason: |
  Live stack not present. Same blocker as Test 19.

### 21. Live 35-case spike re-run gates (MEM-01..05 gate)
expected: |
  `go run ./cmd/spike-history --via-pipeline --summarize --seed=42 \
    --golden=eval/golden/compliance.jsonl --out=reports/ \
    --cost-limit=5.00` runs three segments (adversarial / followup /
  long) against real GLM-5.2 + Milvus + BGE-M3 and produces:
  reports/summary.md / short_history.md / long_history.md / summary.json.
  4-metric drift <5% per metric vs Phase 8 baseline. Refusal delta
  <10pp. SC-3 zero-penetration: 0/10 adversarial rows cite a fake chunk
  ID. Cost <$5.00/segment.
result: blocked
blocked_by: server
reason: |
  Live stack not present. This is the Phase 10 Plan 04 Task 2 gate that
  is the documented `blocked-pending-human` blocker. The user 2026-08-18
  signed acceptance that Phase 10 ships hermetic-gated; this gate is
  the operator's responsibility to clear when the stack is provisioned.

### 22. FE pre-existing `tsc --noEmit` error in DataSubprogressPart.tsx
expected: |
  `npx tsc --noEmit` exits 0 (no errors).
result: pass
verified_by: |
  Quick task 260819-dsp (commit c285c55) removed the dead `data.state`
  read in DataSubprogressPart.tsx (steps are append-only per godoc —
  no live pending state to model). `npx tsc --noEmit` now exits 0;
  `npx vitest run` 349/349 still pass. No D15 wire-type change.

## Summary

total: 22
passed: 19
issues: 0
pending: 0
skipped: 0
blocked: 3

## Gaps

[none — all in-scope tests resolved]