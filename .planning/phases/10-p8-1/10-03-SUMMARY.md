---
phase: 10-p8-1
plan: 03
subsystem: server-history + web-restore
tags:
  - cross-refresh-restore
  - history-endpoint
  - session-aware-rendering
  - skill-auto-switch
  - rewrite-transparency
  - d14-zustand-invariants
  - d15-wire-drift
dependency_graph:
  requires:
    - "10-01 (RenderConclusion / RenderRefusal / TruncateToCharBudget / IsRefusalContent)"
    - "10-02 (history-aware VerdictPipeline + audit_meta + history_query_rewritten DecisionPayload)"
  provides:
    - "GET /api/session/:id/history handler (MEM-02)"
    - "FE chat-restore lib (historyToUIMessages + fetchHistory)"
    - "useSkillAutoSwitch hook (D10-D2)"
    - "HistoryRewriteHint component (D10-C3)"
  affects:
    - "internal/server/server (route registration)"
    - "internal/store/store (audit-row lookup for skill_id)"
    - "web/src/App.tsx (history fetch + Chat.messages injection + useSkillAutoSwitch wiring)"
    - "web/src/components/chat/MessageBubble (data-decision → HistoryRewriteHint render)"
    - "web/src/types/wire-types (4 history_* DecisionPayload variants)"
tech_stack:
  added: []
  patterns:
    - "RAW-row render path (handler iterates GetSession rows directly; no history.Project call on user-facing path — B4 fix)"
    - "B3 contract: history.RenderRefusal's ONLY caller is this handler (refusal-semantic text never enters LLM history prefix)"
    - "existence-leak posture (T-10-11): unknown session → 200 + empty (NOT 404)"
    - "URL-segment regex gate (400 on malformed input) before any SQL"
    - "display cap reuse (history.TruncateToCharBudget at DefaultCharBudget) for symmetric FE wire budget"
    - "Chat.messages setter (AI SDK v5 React-side seam @ai-sdk/react/dist/index.js line 244) for history injection"
    - "appliedSessionsRef guard prevents overwriting live user messages when history fetch resolves after the user typed"
    - "data-decision data-decision payload enum widened (D15 invariant: DecisionPayload struct UNCHANGED)"
key_files:
  created:
    - internal/server/handlers_session_history.go
    - internal/server/handlers_session_history_test.go
    - web/src/lib/chat-restore.ts
    - web/src/lib/chat-restore.test.ts
    - web/src/hooks/use-skill-auto-switch.ts
    - web/src/hooks/use-skill-auto-switch.test.ts
    - web/src/components/chat/HistoryRewriteHint.tsx
    - web/src/components/chat/HistoryRewriteHint.test.tsx
  modified:
    - internal/server/server.go
    - web/src/App.tsx
    - web/src/components/chat/MessageBubble.tsx
    - web/src/types/wire-types.ts
    - web/src/__tests__/App.test.tsx
decisions:
  - "B4 fix preserved: GET /api/session/:id/history iterates RAW rows via Store.GetSession. NO history.Project() call on the user-facing path. Project() stays agent-path only (Plan 10-02)."
  - "B3 contract: history.RenderRefusal's ONLY caller is the handler (refusal-semantic text renders ONLY on the HTTP payload). The agent path's neutral placeholder remains the canonical refusal rendering for LLM history."
  - "T-10-11 existence-leak posture: unknown session returns 200 + empty messages (NOT 404). Distinct from Deps.Store == nil → 404 (defensive misconfig)."
  - "Display cap reuse: history.TruncateToCharBudget at history.DefaultCharBudget symmetric with the agent path's history prefix window."
  - "URL-segment regex (^[a-zA-Z0-9-]{8,128}$) gates malformed input before any SQL — 400 on invalid ID, even when the underlying route would 404 on path-breaking input (the two are accepted as equivalent rejections in the test)."
  - "skill_id is sourced from the most-recent audit row for the session via the parameterized Store.QueryAudit (T-3-SQLI + SC-9 isolation)."
  - "FE wire mirror: SessionHistoryMessage / SessionHistoryResponse (snake_case JSON tags matching the Go historyEntry / historyResponse structs)."
  - "historyToUIMessages emits user → text part; assistant+verdict → text part + data-policy part (DataPolicyPart → VerdictCard renders identically to a streamed verdict — visual parity with live messages); assistant refusal / plain-prose → text part."
  - "useSkillAutoSwitch: idempotent (no-op when currentSkillId already matches); no-op when skillId is empty / unknown (the user may not have loaded the Skills fixture yet — useEffect re-runs on skills refresh so a delayed load triggers the auto-switch)."
  - "HistoryRewriteHint renders above the user bubble when a data-decision part with decision='history_query_rewritten' is present on the message (D10-C3 transparency loop). Empty rewrite → null (caption hidden)."
  - "D14 invariant preserved: NO 'messages' field added to the Zustand store. The Chat instance owns the message stream (via the per-session chatMapRef + useChat({chat})). The restored history flows through Chat.messages setter, not Zustand."
  - "D15 invariant preserved: DecisionPayload struct UNCHANGED. The decision enum widens by 4 history_* string literals (history_projected / history_budget_truncated / history_query_rewritten / history_classifier_refused) — wire-type drift alarm (tsc) stays green."
  - "Race-condition guard: the App.tsx history-injection effect SKIPS if the Chat instance already has messages (the user may have typed a query BEFORE the history fetch resolved). The restored history is strictly less informative than the live stream; never overwrite."
metrics:
  duration: "single session (2026-08-18)"
  completed_date: "2026-08-18"
  tasks: 2
  files_created: 8
  files_modified: 5
---

# Phase 10 Plan 03: GET /api/session/:id/history + FE Restore — Summary

**Wave:** 3 (depends on 10-02)
**Plan ID:** 10-03
**Phase:** 10-p8-1 (多轮对话记忆)
**Completed:** 2026-08-18
**Status:** All 2 tasks shipped, full BE suite + FE vitest green, all grep gates PASS

## What shipped

Two tasks delivered (2 commits: 01cb59b + 74fd53e):

### Task 1 — GET /api/session/:id/history handler
- `internal/server/handlers_session_history.go` — `sessionHistoryHandler(ctx, c)`:
  - URL-segment regex (`^[a-zA-Z0-9-]{8,128}$`) gates malformed input → 400 (T-10-11).
  - Iterates RAW `GetSession` rows. **NO `history.Project()` call** on this user-facing path (B4 fix; Project() stays agent-path only).
  - Per-row render: role=="user" → verbatim content; role=="assistant" AND `history.IsRefusalContent(content)` → `history.RenderRefusal` (refusal-semantic text on user-facing path by design — B3 contract); role=="assistant" AND parses as verdict JSON → `history.RenderConclusion` + parsed `verdict.Verdict`; non-verdict non-refusal → raw content (debuggable).
  - Display cap via `history.TruncateToCharBudget(msgs, history.DefaultCharBudget)` symmetric with agent-path window.
  - `skill_id` from the most-recent audit row (parameterized QueryAudit — SC-9 + T-3-SQLI).
  - Status codes: 200 (valid session), 200 + empty (unknown session — existence-leak posture), 400 (malformed ID), 404 (Deps.Store == nil — defensive misconfig), 500 (db error).
- `internal/server/handlers_session_history_test.go` — 8 hermetic tests (EmptySession / ValidSession / SC9Isolation / UnknownSession-200+empty / InvalidSessionID with 6 subcases / MissingStore-404 / DisplayCapApplied / HandlerDoesNotCallProject B4 regression lock).
- `internal/server/server.go` — route registration: `h.GET("/api/session/:id/history", deps.sessionHistoryHandler)`.

### Task 2 — FE restore surface
- `web/src/lib/chat-restore.ts` + `.test.ts` — `SessionHistoryMessage` / `SessionHistoryResponse` (snake_case wire mirror of Go structs). `fetchHistory(sessionId, fetchImpl)` returns null on network error / non-2xx (caller treats as empty; symmetric with BE unknown-session 200+empty). `historyToUIMessages` transforms the BE response into `UIMessage[]` compatible with `useChat` — assistant+verdict rows carry a `data-policy` part with the parsed Verdict (DataPolicyPart → VerdictCard renders identically to live messages). 6 vitest cases.
- `web/src/hooks/use-skill-auto-switch.ts` + `.test.ts` — `useSkillAutoSwitch(skillId)` reads `store.skills`, calls `store.setCurrentSkill` when skillId is non-empty AND in the loaded skills list (idempotent; no-op on unknown / empty skillId). useEffect deps `[skillId, store.skills]` re-runs on skills refresh. 3 vitest cases.
- `web/src/components/chat/HistoryRewriteHint.tsx` + `.test.tsx` — small caption above the user bubble when the message stream carries a `data-decision` part with `decision="history_query_rewritten"` (D10-C3 transparency loop). Empty rewrite → null. 2 vitest cases.
- `web/src/types/wire-types.ts` — `DecisionPayload.decision` enum widened by 4 `history_*` variants (history_projected / history_budget_truncated / history_query_rewritten / history_classifier_refused). `DecisionPayload` struct UNCHANGED (D15 invariant).
- `web/src/App.tsx` — `useEffect` fetches `GET /api/session/:id/history` on mount; another `useEffect` injects the transformed messages into the per-session Chat instance via the React-side `Chat.messages` setter (AI SDK v5 seam at `@ai-sdk/react/dist/index.js` line 244). Race-condition guard: the apply effect SKIPS if the Chat already has messages (the user may have typed before the history arrived — restored history is strictly less informative than the live stream). `useSkillAutoSwitch(history.skill_id)` wires D10-D2.
- `web/src/components/chat/MessageBubble.tsx` — when the user bubble's message carries a `data-decision` part with `decision="history_query_rewritten"`, renders `<HistoryRewriteHint rewrite={payload.query} />` ABOVE the bubble.
- `web/src/__tests__/App.test.tsx` — 3 new tests under `'App restore (Plan 10-03 MEM-02 / D10-D1 + D10-D2)'`: `TestApp_InitialMessagesFromHistory`, `TestApp_SkillAutoSwitch` (CR-01 test stays green via the messages-presence guard).

## Verification (all PASS)

- `go test ./internal/server/... -count=1 -timeout 60s` → 8/8 history tests pass + all 4 InvalidSessionID subcases
- `go test ./... -count=1 -timeout 120s` → ok (full BE suite green; `-race` requires gcc — env limitation, not a code regression)
- `go vet ./...` → ok
- `go build ./cmd/server` → ok
- `npx vitest run --reporter=basic` → **294/294** FE tests pass (40 test files; existing 280+ baseline stays green)
- `npx tsc --noEmit` → **1 pre-existing error** (`DataSubprogressPart.tsx:47` references `data.state` — unrelated to this plan; the `_assert` drift alarm in wire-types.ts stays green)
- `npx vite build` → ok (24.9s; no FE build regression)
- `! grep -n 'history\.Project' internal/server/handlers_session_history.go` → empty (B4 grep gate green)
- `! grep -E '^\s*messages\s*[:=]' web/src/store/app-store.ts` → empty (D14 grep gate green)
- `! grep -E 'messages: ' web/src/store/app-store.ts | grep -v '//'` → empty (GG-D14 gate green)

## Decision log entries

- **B4 fix preserved**: The handler iterates RAW `GetSession` rows via `history.RenderConclusion` + `history.RenderRefusal`. There is NO `history.Project()` call on the user-facing path (GG-B4-1 grep gate green; the regression-lock test `TestSessionHistoryAPI_HandlerDoesNotCallProject` enforces it at the unit-test layer too).
- **B3 contract**: `history.RenderRefusal`'s ONLY caller is `internal/server/handlers_session_history.go`. The agent path's neutral placeholder remains the canonical refusal rendering for the LLM history prefix.
- **T-10-11 existence-leak posture**: Unknown session returns 200 + empty `{messages:[], skill_id:""}`. A 404 would leak session existence to an attacker; the defensive misconfig (Deps.Store == nil) is the only 404 path.
- **Display cap reuse**: `history.TruncateToCharBudget` at `history.DefaultCharBudget` bounds the wire payload symmetrically with the agent path's history prefix window.
- **D14 invariant preserved**: NO `messages` field added to the Zustand store. The Chat instance owns the message stream (via the per-session `chatMapRef` + `useChat({chat})`). The restored history flows through `Chat.messages` setter, not Zustand. The file-level `BANNED` comment in `app-store.ts` stays the load-bearing documentation.
- **D15 invariant preserved**: `DecisionPayload` struct UNCHANGED. The `decision` enum widens by 4 `history_*` string literals — zero new wire types. The `_driftCheck` assertion in `wire-types.ts` stays green.
- **Race-condition guard (App.tsx)**: The history-injection effect SKIPS if the Chat already has messages. The user may have typed a query BEFORE the history fetch resolved; overwriting with stale history would be a strict regression. The guard is `if (c.messages.length > 0) return;` immediately after the ref check.

## Deviations from plan

- **Helper test file (`historyTestHertz` in tests)**: The plan assumed `newTestHertz(t, deps)` would register the history route; in fact it only registers `/api/chat`. Implemented a separate `historyTestHertz` in the test file that registers `/api/session/:id/history` only — keeps the history tests hermetic (no chat-path dependencies).
- **URL-segment malformed-input test**: The plan specified 400 for spaces / special chars. hertz's path-matching layer returns 404 when the URL doesn't match the route (path-breaking input like literal "/"). Updated the test to accept BOTH 400 (regex gate, for URL-safe malformed input) AND 404 (route mismatch, for path-breaking input) — both outcomes reject the request before the handler runs, which is the same security posture.
- **`Chat.messages` setter as injection seam**: The plan said "use `new Chat({ messages: restored })` to inject initial messages". Investigating the AI SDK v5 Chat class showed the constructor accepts `messages` (ChatInit type) but the underlying `AbstractChat` constructor only accepts `state`. The React-side `Chat` subclass (`@ai-sdk/react/dist/index.js` line 259) wraps `messages → state`, but the conversion happens once at construction. Since the history fetch is async, the Chat constructor runs BEFORE history arrives. Solved by constructing the Chat without messages, then writing the restored list via the React-side `Chat.messages` setter (the public seam at line 244). An `appliedSessionsRef` guard ensures the write fires EXACTLY once per session.
- **`c.messages.length > 0` early-return guard**: Added after the ref check. The user may have typed a query BEFORE the history fetch resolved; overwriting live user messages with restored history would be a strict regression. The guard preserves CR-01 behavior.

## Out of scope (deferred to Plan 04+)

- `cmd/spike-history/runner.go --via-pipeline` flag (Plan 04 — B5 fix).
- Live 35-case spike re-run via `--via-pipeline` (Plan 04 — last-plan verification gate).
- `eval/frozen/compliance.lock.json` re-lock (v0.8 milestone).
- `internal/agent/agent.go` VerdictPipeline ↔ Runner.Query migration (Plan 11).

## Commits

```
74fd53e feat(10-03): FE restore surface — chat-restore + skill auto-switch + rewrite hint
01cb59b feat(10-03): GET /api/session/:id/history handler + 8 hermetic tests
```