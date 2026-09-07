---
phase: 29-feedback-loop-ticket-interface
plan: 07
subsystem: ui
tags: [react19, feedback-row, sf-toast, server-driven-lock, idempotent-replay, fetch, ui-spec-copywriting, fbk-04, tkt-03]

# Dependency graph
requires:
  - phase: 29-feedback-loop-ticket-interface (29-05)
    provides: POST /api/feedback wire contract — 201 {status:"recorded", verdict} | 200 {status:"already_recorded", verdict} | 4xx {error} — the lock state machine's server truth
  - phase: 29-feedback-loop-ticket-interface (29-06)
    provides: POST /api/tickets wire contract — 201 {ticket_number:"TKT-n", dispatch_status:"none"|"queued"} | 200 {status:"already_exists", ticket_number} | 4xx {error} — the 4-state ticket result copy's inputs (frozen in 29-07-PLAN <interfaces>, consumed without waiting on 29-06 per the parallel-wave design)
  - phase: 29-feedback-loop-ticket-interface (29-04)
    provides: the data-turn part (TurnPayload {turnId, votedVerdict?}) — live SSE emit and chat-restore synthesis share this one consumption point; votedVerdict is the history echo that locks restored rows
  - phase: 20 (admin console) / Open Design B6
    provides: the .feedback-row CSS strip + VOTE_META/testid skeleton this plan preserves, the wizard-success-toast markup SfToast mirrors, and the App.tsx three-branch route structure SfToast mounts into
provides:
  - SfToast (web/src/components/layout/SfToast.tsx) — the app's FIRST 'sf-toast' listener: window listener + single-slot latest-wins viewport, lenient kind mapping ('error'→error, 'success'/'compliant'→success, else→info so SessionDrawer's existing dispatch works verbatim), auto-dismiss 4000ms/6000ms(error), mounted in the chat + eval branches (admin mount belongs to 29-08)
  - FeedbackRow rewritten from toast placeholder to the real API state machine: POST /api/feedback + POST /api/tickets raw fetch (public endpoints, no X-Admin-Token), server-driven lock (201 locks / 200 already_* renders server truth as non-error / failure leaves the row retryable, no auto-retry)
  - outdated inline optional-note flow (D-29-06): selecting 报告知识过时 expands a 1-row textarea; 提交说明 posts with body.comment (empty = clean skip); locked pill carries 「已附说明」 in aria-label only when a note was stored
  - inline ticket form (D-29-13/14): 问题描述（必填） prefilled from chatText truncated to 200 chars, mono reference strip (会话 短id · 轮次 #n · chunk chips / 无引用), 4 honest result states verbatim from UI-SPEC §Copywriting Contract, failure keeps the form open/editable/resubmittable, success collapses to the result line + disabled 已建单 trigger
  - MessageBubble wiring: data-turn part → turnId/votedVerdict (null/zero/non-numeric → honest disabled row, T-29-06), verdict.citations[].chunk_id → read-only chunkIds (never reported back, D-29-05/T-29-04), store activeSessionId + aggregated chatText into the row
affects: [29-08 admin FE (mounts its own SfToast inside AdminDashboard root div + consumes the same endpoints from the admin side), 29-09 verify gate (placeholder-sweep + dangerouslySetInnerHTML greps stay green), future disposition toasts (FeedbackRow/SfToast are the chat-side toast producers SfToast consumes)]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies (D17-B1) — lucide-react icons + existing Button + existing tokens only
  patterns:
    - "server-driven lock: FE vote state is a CACHE, never truth — the DB UNIQUE constraint (29-02) is the real idempotency guard and the history echo can override the cache on restore"
    - "idempotent replay is not an error: already_recorded / already_exists render exactly like success with an info toast and the server-returned identifier"
    - "honest failure: body.error parsed as the real reason with `HTTP {status}` fallback, error toast, NO lock, NO auto-retry (D-29-04)"
    - "component-owns-its-listener: SfToast carries both halves (listener + viewport) so each route branch mounts its own instance instead of App owning a global"
key-files:
  created:
    - web/src/components/layout/SfToast.tsx
    - web/src/__tests__/SfToast.test.tsx
  modified:
    - web/src/App.tsx
    - web/src/components/chat/parts/FeedbackRow.tsx
    - web/src/components/chat/MessageBubble.tsx
    - web/src/__tests__/FeedbackRow.test.tsx
    - web/src/__tests__/MessageBubble.test.tsx

key-decisions:
  - "outdated is a two-step vote: clicking 报告知识过时 expands the note box and 提交说明 posts — UI-SPEC §Copywriting Contract ships the 提交说明 button copy, so selection-without-submission is the contract shape (empty note still submits cleanly per D-29-06)"
  - "success detection keys on resp.ok + body.status==='recorded' (not the literal 201): the semantic is the envelope, and a 200-with-recorded proxy rewrite still renders the truthful outcome; already_* branches pin the exact 200 the frozen contract specifies"
  - "body.comment is sent ONLY on outdated votes ('' when the note is empty); helpful/unhelpful bodies are exactly {session_id, turn_id, verdict} — test-locked key sets, and messageId never enters any body (Pitfall 7)"
  - "reference-strip session short id = last 8 chars of activeSessionId, and chunkIds are order-preserving deduped citations — display-only data the client already has; the POST carries no chunk ids (server self-attributes, D-29-05)"
  - "votedVerdict seeds useState at mount only: the live stream never emits votedVerdict (Go emit side never marshals it) and history restore always has it before first render, so no prop-sync effect is needed"
  - "grep-hygiene prose: the admin-branch comment avoids the literal `<SfToast />` string and the FeedbackRow header avoids naming the raw-HTML escape hatch, so the plan's own acceptance greps (mount count == 2, escape-hatch count == 0) measure markup, not comments"
  - "REQUIREMENTS rows stay Pending per the phase-close convention (29-02..29-06 precedent) — FBK-04 and TKT-03's chat-side substance is fully delivered here"

patterns-established:
  - "Pattern: sf-toast event contract — CustomEvent('sf-toast', {detail:{kind, message}}) with lenient kind mapping; producers stay dependency-free, SfToast instances are per-route-branch consumers"
  - "Pattern: server-truth state machine for public write UIs — idle → submitting → (recorded | already_recorded | failure-unlocked), with the failure branch leaving the form exactly as it was"

requirements-completed: [] # FBK-04/TKT-03 substance delivered (chat side complete); REQUIREMENTS.md rows flip at phase close per the 29-02..29-06 convention

# Metrics
duration: 13min
completed: 2026-09-08
---

# Phase 29 Plan 07: FE FeedbackRow 接真 Summary

**FeedbackRow turned from a toast placeholder into the real server-driven vote/ticket state machine (POST /api/feedback + POST /api/tickets, idempotent replays rendered as truth, failures left retryable) plus SfToast — the app's first sf-toast listener — mounted in the chat and eval branches.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-09-08T05:20:00+08:00
- **Completed:** 2026-09-08T05:33:00+08:00
- **Tasks:** 2 (both TDD — RED/GREEN each)
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- FBK-04 chat side complete: every vote button POSTs the frozen contract body {session_id, turn_id, verdict[, comment]}; the lock renders ONLY from a server response (201 recorded → active pill + losers disabled; 200 already_recorded → server verdict active + info toast; failure → pills re-enabled + error toast with the real reason; votedVerdict history echo → locked with zero requests).
- TKT-03 chat side complete: 创建工单 expands the inline form (never a modal) with the ≤200-char prefilled description, mono reference strip, and the 4 honest result states verbatim — 「✓ 工单已受理 · TKT-n — 未配置外发渠道，工单已留档」 / 「…正在排队外发」 / 「该消息已建过工单 · TKT-n」 / inline failure + toast with the form kept open, editable, resubmittable.
- The B6-era placeholder is gone (「功能即将上线」 sweep == 0) and sf-toast finally has a listener: SfToast owns the window listener + a single latest-wins slot with per-kind auto-dismiss, mounted exactly twice in App.tsx (chat + eval); the admin mount is documented for 29-08.
- turnId==null degrades honestly: all four buttons disabled with 「该消息缺少服务端轮次标识，暂时无法反馈」, never hidden, never a fabricated id (T-29-06).
- Full FE regression green: 563/563 tests across 71 files (535 pre-existing + 28 new), `tsc --noEmit` clean, zero new npm dependencies, zero new design tokens, no 10px/11px literals in new markup.

## Task Commits

Each task was committed atomically (TDD RED → GREEN per task):

1. **Task 1: SfToast 全局视口（监听 + 单槽 latest-wins）挂进 chat/eval 分支** — `90c84ff` (test RED: SfToast module absent) + `72c77dc` (feat GREEN)
2. **Task 2: FeedbackRow 重写（真实投票 + outdated 说明框 + 工单表单）+ MessageBubble 接线** — `0da0428` (test RED: 19/19 FeedbackRow cases failing against the placeholder) + `cff8267` (feat GREEN)

**Plan metadata:** (this commit) docs(29-07): complete FE feedback/ticket 接真 plan

## Files Created/Modified

- `web/src/components/layout/SfToast.tsx` — CREATED: window 'sf-toast' listener (removed on unmount) + fixed bottom-right viewport; lenient kind mapping; per-message timer reset; role="status" aria-live="polite"; variant classes per UI-SPEC §Surface 4
- `web/src/__tests__/SfToast.test.tsx` — CREATED: 8 fake-timer cases (empty idle, render+aria, 3 variant maps, lenient legacy kinds, 4000ms/6000ms auto-dismiss, latest-wins with timer reset, close button)
- `web/src/App.tsx` — import + exactly two `<SfToast />` mounts (chat branch after ReloadToast, eval branch); admin early-return annotated with the 29-08 mount split
- `web/src/components/chat/parts/FeedbackRow.tsx` — full rewrite preserving the VOTE_META/testid/.feedback-row skeleton: postJson helper (network-error vs HTTP reason extraction), server-driven vote lock, outdated note box, inline ticket form + 4-state result, data-turn-id/message-id/voted attributes, role="group" aria-label="回答反馈"
- `web/src/components/chat/MessageBubble.tsx` — store hook lifted above the user-role early return; data-turn part scan (turnId>0 guard) + votedVerdict guard + citations→chunkIds derivation (tolerant, deduped); FeedbackRow mounted with the 6 new props
- `web/src/__tests__/FeedbackRow.test.tsx` — full rewrite (Pitfall 7): 19 fetch-mocked cases covering the whole behavior matrix incl. exact body shapes, no-lock-on-failure with manual retry, non-JSON error fallback reason, 已附说明 aria semantics, ticket 4 states + truncation + 无引用 + empty-description disable + cancel-without-submit
- `web/src/__tests__/MessageBubble.test.tsx` — +3 wiring cases: data-turn-derived lock, mid-stream disabled state, store session + citations feeding the ticket strip

## Decisions Made

See `key-decisions` in the frontmatter. The load-bearing ones: outdated is a two-step vote whose 提交说明 button comes verbatim from the UI-SPEC copy contract; success detection keys on the response envelope rather than the literal 201; comment rides the body only on outdated votes; the admin branch comment and the FeedbackRow header were worded so the plan's own acceptance greps measure markup instead of comments.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test bug] Fixed a null-unsafe aria-label assertion in my own RED suite**
- **Found during:** Task 2 GREEN run
- **Issue:** the empty-comment case asserted `getAttribute('aria-label')).not.toContain('已附说明')`, but the attribute is legitimately ABSENT for a note-less lock, so `getAttribute` returns null and `toContain` throws on null.
- **Fix:** assert `not.toHaveAttribute('aria-label')` instead — same semantics, no null deref.
- **Files modified:** web/src/__tests__/FeedbackRow.test.tsx
- **Verification:** FeedbackRow 19/19 green
- **Committed in:** cff8267 (part of the GREEN commit)

### Advisory (not a deviation)

- Task 2's test-suite update deliberately REPLACES the old FeedbackRow.test.tsx (Pitfall 7 in the plan: 整文件重写而非增量补丁) — the B6 placeholder assertions (toast-on-click, no-fetch lock) describe behavior this plan deletes, so keeping them would assert the bug.
- The wire contracts were consumed exactly as frozen in the plan's `<interfaces>` block; 29-06's SUMMARY was not probed (per the parallel-wave instruction) and no drift surfaced: `ticket_number` / `dispatch_status` / `already_exists` all matched.

---

**Total deviations:** 1 auto-fixed (test bug) + 1 advisory
**Impact on plan:** No scope creep — all acceptance greps, body shapes, and copy are exactly as planned.

## Issues Encountered

- None blocking. Two self-inflicted grep trips were resolved by rewording comments (the admin-branch comment initially contained the literal `<SfToast />` mount string, and the FeedbackRow header named the raw-HTML escape hatch it never uses) — both acceptance gates now count markup only.

## User Setup Required

None - no external service configuration required. Live behavior requires the backend from 29-05/29-06 running (`POST /api/feedback`, `POST /api/tickets`); with the server down the UI degrades exactly as designed (error toast with the network reason, row stays retryable).

## Next Phase Readiness

- 29-08 (admin FE) can start immediately: it mounts its own `<SfToast />` inside AdminDashboard's root div (App.tsx documents the split at the admin early-return), and the disposition/metrics panels consume the admin endpoints frozen in 29-05's SUMMARY.
- 29-09 (verify gate) inherits green guards: `功能即将上线` == 0 and escape-hatch count == 0 across both new files, plus the full 563-test suite as the regression baseline.
- Residual: none known for this plan; REQUIREMENTS rows for FBK-04/TKT-03 flip at phase close per convention.

## Self-Check: PASSED

- Files: web/src/components/layout/SfToast.tsx, web/src/__tests__/SfToast.test.tsx, web/src/App.tsx, web/src/components/chat/parts/FeedbackRow.tsx, web/src/components/chat/MessageBubble.tsx, web/src/__tests__/FeedbackRow.test.tsx, web/src/__tests__/MessageBubble.test.tsx — all FOUND on disk.
- Commits: 90c84ff (RED-1), 72c77dc (GREEN-1), 0da0428 (RED-2), cff8267 (GREEN-2) — all FOUND in git log (TDD gate: test commit precedes feat commit per task).
- Verification re-run green: `npx vitest run FeedbackRow MessageBubble SfToast` 45/45; `npx tsc --noEmit` clean; `npm run test` 563/563; escape-hatch counts 0/0; placeholder sweep 0.
- Isolation: the 4 pre-existing dirty files (.planning/config.json, internal/server/knowledge.go, web GraphCanvas.tsx, InspectorRetrieval.tsx) appear in none of this plan's 4 commits.

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
