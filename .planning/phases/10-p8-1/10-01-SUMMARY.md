# 10-01 Plan Summary — BE Foundation (verdict renderer + neutral placeholder + budget window + classifier type isolation)

**Wave:** 1
**Plan ID:** 10-01
**Phase:** 10-p8-1 (多轮对话记忆)
**Completed:** 2026-08-18
**Status:** ✅ All tasks shipped, hermetic green

---

## What shipped

3 tasks delivered (4 commits: a146827 + fbb951c + e482527 + neutral_placeholder/budget_window additions):

### Task 1 — Verdict renderer (agent-path only)
- `internal/history/verdict_renderer.go` — `RenderConclusion(verdictJSON []byte) string` parses verdictJSON → conclusionText deterministically (handles 3 cases: well-formed / malformed JSON / empty)
- `internal/history/verdict_renderer_test.go` — 3 case coverage
- `RenderRefusal` exported but **NEVER called from `Project()`** (B3 fix; grep-gated by CI gate GG-A1-1)
- `Project()` extended to call `RenderConclusion` for assistant turns with verdictJSON content

### Task 2 — Neutral placeholder + char-budget window
- `internal/history/neutral_placeholder.go` — `RenderNeutralPlaceholder()` fixed-string template; canonical 11-entry refusal-semantic word list documented in godoc; `isRefusalRound()` matcher covers `RefusalReason*` enums
- `internal/history/neutral_placeholder_test.go` — zero word leak verified across 5 refusal reasons + 3 non-refusal cases
- `internal/history/budget_window.go` — `TruncateToCharBudget(msgs, budget)` is the **PRIMARY** window (B1 fix); 6-message D9-B3 cap survives only as `FallbackMessageCeiling` when `budget<=0`; `SafeguardFloor=2`; `DefaultCharBudget=8000`
- `internal/history/budget_window_test.go` — 4 case coverage
- `internal/history/projection.go` — 6-message unconditional pre-cap REMOVED; char-budget is the only window
- `internal/history/projection_test_fixtures.go` — fixture `neg_history_truncated_to_three_pairs` retired; **8 fixtures total** (7 existing + 1 new `budget_window_keeps_all_pairs_under_budget` documenting the D9-B4→D10-B3 evolution)
- `internal/history/projection_test.go` — extended to cover 8 fixtures

### Task 3 — RefusalClassifierInput type isolation contract
- `internal/server/classifier/types.go` — `RefusalClassifierInput{Query string, Chunks []string}` (exactly 2 fields, no History); `RefusalClassifierOutput{Decision, Reason, Confidence}`; `Valid()` checks Decision enum + Confidence ∈ [0.0, 1.0] inclusive + NaN; compile-time sentinel `_ = verdict.RefusalReason*`
- `internal/server/classifier/types_test.go` — 7 reflection/shape/import tests
- `import "math"` added for NaN detection

## Verification (all PASS)

- `go test ./internal/history/... -count=1` → ok (0.234s)
- `go test ./internal/server/classifier/... -count=1` → ok (0.790s)
- `go test ./... -count=1 -timeout 120s` → full BE suite green, zero regressions
- `grep -c RenderRefusal internal/history/projection.go` → 0 (B3 CI gate)
- `grep -E 'History\s*\[\]schema' internal/server/classifier/types.go` → empty (D10-A1 CI gate)
- Fixtures 8/8 green
- Neutral placeholder zero leakage (11-word canonical list)

## Decision log entries

- **D10-A2 agent-path enforcement:** `Project()` calls `RenderConclusion` (verdictJSON → text) + `RenderNeutralPlaceholder` (refusal → fixed template) only. `RenderRefusal` is handler-only (Plan 03 user-facing path). CI grep gate enforced.
- **D10-A1 type isolation:** `RefusalClassifierInput{Query, Chunks}` reflection-locked at 2 fields; classifier package imports zero history/store symbols.
- **D10-B3 budget primary:** Char-budget is the ONLY window (oldest-first drop with SafeguardFloor=2). 6-message D9-B3 cap survives only as `FallbackMessageCeiling` when `budget<=0`. Fixture `neg_history_truncated_to_three_pairs` retired; new `budget_window_keeps_all_pairs_under_budget` proves budget > 3 pairs under budget headroom.

## Deviations

- (None from plan; both minor bugs caught at execution boundary — `Valid()` missing NaN check + `types.go` comment had two capitalized "History" mentions — both fixed in same commit before shipping.)

## Out of scope (deferred to Plan 02+)

- `internal/server/pipeline.go` (Plan 02 will wire history → rewriter → retrieve → classifier → generate)
- `internal/server/chat.go` handler chain (Plan 02)
- `cmd/server/main.go` production assembly (Plan 02 Task 4 — B2 fix wiring)
- `cmd/spike-history/runner.go --via-pipeline` flag (Plan 04 — B5 fix)
- FE handler `GET /api/session/:id/history` (Plan 03)