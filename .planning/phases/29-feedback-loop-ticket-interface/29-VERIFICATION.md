---
phase: 29-feedback-loop-ticket-interface
verified: 2026-09-08T06:50:00Z
status: human_needed
score: 12/12 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification — no prior VERIFICATION.md existed
human_verification:
  - test: "UAT Test 1 — live vote flow: send one chat turn in the browser, observe the data-turn SSE frame (curl -N or devtools), click 有帮助 on FeedbackRow"
    expected: "POST /api/feedback → 201 {status:recorded, verdict}; pill locks, other two disable, toast 「已记录反馈：有帮助」"
    why_human: "Live SSE framing, browser network panel and toast rendering cannot be verified programmatically"
  - test: "UAT Test 2 — idempotency + restore echo: re-click a voted pill, then refresh and reopen the session"
    expected: "200 already_recorded with server verdict + info toast; after restore the row is locked with zero extra POSTs (voted_verdict via history data-turn part)"
    why_human: "Requires a real browser session restore and network-panel request counting"
  - test: "UAT Test 3 — outdated → /admin 复核 tab: report a cited reply as outdated, then disposition it in the review tab"
    expected: "Aggregated chunk row (report_count), verbatim trust banner, disposition 201 {closed_count:N} + toast with N, row leaves pending, history filter shows the record"
    why_human: "Admin UI interaction, toast copy and badge colors are visual/interactive"
  - test: "UAT Test 4 — reopen semantics: report the same chunk outdated again after disposition"
    expected: "Chunk reappears in pending (latest-disposition-wins); a second disposition keeps both history rows"
    why_human: "Cross-page flow timing and visual queue state"
  - test: "UAT Test 5 — metrics tab visual: caliber note, 0-vote 「—」, 7/30-day and Skill/KB switches, day/week trend toggle, per-KB footnote, 待复核 cross-link"
    expected: "Verbatim caliber note; 0-vote rates render 「—」 never 0%/NaN; day↔week re-buckets with zero extra requests; cross-link jumps to 复核"
    why_human: "Rendering, layout and client-side re-bucketing behavior"
  - test: "UAT Test 6 — ticket with provider=none (default boot): expand 创建工单, submit"
    expected: "201 {ticket_number:TKT-n, dispatch_status:none}, honest 留档 copy, trigger becomes disabled 已建单; replay → 200 already_exists with the same number"
    why_human: "Form interaction, prefill truncation display and honest result copy"
  - test: "UAT Test 7 — ticket with SKILLFORGE_TICKET_PROVIDER=webhook against a local fake endpoint (with secret header)"
    expected: "201 queued returned immediately; fake endpoint receives JSON body (ticketNumber/sessionId/turnId/description/chunkIds/skillId) + X-SkillForge-Ticket-Secret, empty URL query; action_ticket_dispatch_succeeded visible in /api/admin/audit; 5xx endpoint still yields 201 + failed audit row, server survives"
    why_human: "Requires a live process, an external webhook receiver and timing observation"
  - test: "UAT Test 8 — Phase 27→29 joint acceptance: disposition a chunk as 已更新, snapshot GET /api/admin/kb, then upload a new document via Knowledge Bases"
    expected: "Disposition produces ZERO knowledge-base change; the upload is the only source of KB change; new chunks citable in chat"
    why_human: "FBK-05 red line is only fully provable against a live stack with real document upload"
  - test: "Visual pass of the new surfaces: FeedbackRow inside the message column (no overflow), SfToast position/z-order in chat + eval + /admin, 7-tab order (反馈 6th, 复核 7th)"
    expected: "No layout breakage; toasts land in all three route branches; tab order matches D-29-12"
    why_human: "Pure visual/layout judgment"
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: human_needed
---

# Phase 29: 反馈闭环 + 工单预留 (Feedback Loop + Ticket Interface) Verification Report

**Phase Goal:** FeedbackRow 的四个按钮全部产生真实结果——三按钮投票落库幂等、过时报告进人工复核队列、指标进 admin 仪表盘、创建工单接真实端点；工单外发尽力而为不阻塞主链路；反馈绝不自动改信任。
**Verified:** 2026-09-08T06:50:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Truths merged from ROADMAP Phase 29 Success Criteria (contract) + all 9 PLAN frontmatter must_haves, deduplicated into 12 groups.

| #  | Truth                                                                                                                                          | Status     | Evidence |
|----|------------------------------------------------------------------------------------------------------------------------------------------------|------------|----------|
| 1  | Turn-id identity substrate: AppendTurn returns assistant row id, conflict replay returns exactly 0, GetSession carries id (D-29-01)              | ✓ VERIFIED | `internal/store/sqlite/session.go:82-113` (RowsAffected guard, LastInsertId forbidden on replay), `getSessionSQL` selects `id`; `TestAppendTurnReturnsAssistantID` 4/4 subtests PASS (WSL) |
| 2  | One vote per (session, turn): feedback INSERT-only table with UNIQUE(session_id, turn_id); replay returns 200 already_recorded with the FIRST verdict | ✓ VERIFIED | `schema.go:396-407` + `feedback.go:43-79` (ON CONFLICT DO NOTHING + RowsAffected→inserted=false); `feedback.go:230-236` replay branch writes nothing; `TestFeedbackHandler` PASS |
| 3  | Citations are server truth: voted chunk ids + skill copied from the turn's audit row, never from the request body (D-29-05); honest degradation  | ✓ VERIFIED | `feedback.go:212-216` `feedbackAttribution` via parameterized `AuditFilter{SessionID, Until: turn.Ts, Limit:1}`; body has no chunk field; refs degrade to kb_slug='' on retriever failure |
| 4  | Turn id exposed on the live stream: data-turn frame before finish on every assistant reply including all 5 refusal shapes; turnID<=0 emits zero bytes | ✓ VERIFIED | `wire/chunk.go:480` ValidTypes["data-turn"] (count comment 24); emitTurnID == 11 sites (pipeline 6 + chat 3 + runner 2, chat.go:449 deliberately absent with comment); `TestVerdictPipeline_DataTurnVerdictFrameOrder`, `_DataTurnRefusalShapes`, `_DataTurnZeroIDSendsNothing`, `TestRunnerChatHandler_DataTurnBeforeFinish`, `TestRunAgentPath_ErrorBranchHasNoDataTurn` all PASS |
| 5  | History restore echoes turn_id + voted_verdict in ONE request (D-29-03), all four assistant branches fill both                                  | ✓ VERIFIED | `handlers_session_history.go:69-70` (omitempty fields), `:155` single batch `VotedVerdictsBySession` (grep count 1 call), `:260/:287/:301/:317` four assistant branches; `TestSessionHistoryAPI_TurnIDAndVotedVerdictEcho` PASS; FE `chat-restore.ts:155` synthesizes the data-turn part |
| 6  | FeedbackRow is real: POST /api/feedback + /api/tickets, server-driven lock, placeholder toast removed, failure does not lock (D-29-04)           | ✓ VERIFIED | `FeedbackRow.tsx:197/:223` real fetches; `grep 功能即将上线 web/src` = 0 matches; lock seeded from `votedVerdict` + 201/200 branches; 19 FeedbackRow tests PASS (incl. no-messageId body shape, 5xx stays retryable, real error reason); SfToast listener + viewport in chat/eval (`App.tsx:362/:390`) and admin (`AdminDashboard.tsx:212`), 8 SfToast tests PASS |
| 7  | outdated reports land in a per-chunk aggregated review queue; three terminal dispositions with closed_count + INSERT-only audit row; reopen on newer report | ✓ VERIFIED | `feedback.go:532-614` disposition handler (closed_count computed before write, `action_feedback_disposition` audit row); derived pending view `feedback.go:252-260` (NOT EXISTS latest-disposition-wins, no second write path); `TestPendingReviewChunks`, `TestFeedbackDispositionAdminGate` PASS; `ReviewQueuePanel.tsx` verbatim trust banner `:75` + three dispositions + closed_count toast |
| 8  | Admin metrics: per-Skill / per-KB help rate + outdated trend; full-vote denominator computed Go-side; 0 votes → JSON null not 0% (D-29-10/11/12) | ✓ VERIFIED | `feedback.go:773-789` `feedbackRate` full-denominator with nil→null; by_key serves selected dim (per-KB counts once per KB), totals strict once-per-vote; `FeedbackMetricsPanel.tsx:85/:88` verbatim caliber note + per-KB footnote, `:142` null→「—」, `:442` cross-link; `TestFeedbackMetricsHandler` + panel tests PASS; 反馈 = 6th tab, 复核 = 7th (`AdminDashboard.tsx:170-206`, store union `app-store.ts:466-467`) |
| 9  | POST /api/tickets: idempotent one-ticket-per-message, TKT-{row id} derivation, dispatch_status none/queued at INSERT; FE shows 4 honest states (TKT-02/03) | ✓ VERIFIED | `tickets.go:142-253` (validation chain, replay 200 already_exists with first number, no second dispatch), `ticketNumberFor`; `FeedbackRow.test.tsx` 4 ticket-state cases PASS (none 留档 / queued 排队 / already_exists / failure keeps form editable) |
| 10 | TicketService interface + config-driven registry: Provider has exactly one Dispatch method; none = store-only; webhook = HTTP outbound; unknown → error (TKT-01) | ✓ VERIFIED | `internal/ticket/ticket.go:72-75` one-method interface; `registry.go:48-66` none/webhook/unknown tri-branch with webhook-no-URL error; 14 hermetic tests PASS (registry 3-branch + webhook 7-case matrix incl. timeout, secret header, body-only payload, ctx cancel) |
| 11 | Outbound dispatch is structurally non-blocking and audited: goroutine on context.Background()+budget, defer recover, INSERT-only success/failure audit rows; 5 env knobs + dual log.Fatalf (TKT-04) | ✓ VERIFIED | `tickets.go:325-353` (`go func` + `recover()` + `context.WithTimeout(context.Background(), budget)`); audit actions `action_ticket_dispatch_succeeded/failed` with ActorType=admin; `TestTicketDispatchNonBlocking`, `TestTicketDispatchAuditRows` PASS; `cmd/server/main.go:354-373` 5 SKILLFORGE_TICKET_* envs + 2 log.Fatalf branches |
| 12 | FBK-05 red line + hygiene gate: feedback/ticket code cannot mutate trust; 4-stage verify gate + make target + STRIDE-29 + operator runbook       | ✓ VERIFIED | `TestFeedbackNoTrustMutation` PASS (zero UPDATE/DELETE/Insert(, retriever only GetChunk, zero Indexer refs); `bash scripts/verify-phase-29.sh` exit 0 with all 4 stages PASS (re-run by verifier); `Makefile:9/:111-113` phase-29 in .PHONY; `docs/threat-model/STRIDE-29.md` 7 boundaries + 10 threats all dispositioned; `29-UAT.md` 9 operator tests |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/store/store.go` | AppendTurn (int64, error) + SessionMessage.ID + 14 new feedback/ticket methods | ✓ VERIFIED | 785, 51, 1021-1119 |
| `internal/store/sqlite/session.go` | RowsAffected guard + id column | ✓ VERIFIED | 34, 60-62, 110-129 |
| `internal/store/sqlite/session_turnid_test.go` | turn-id semantics lock | ✓ VERIFIED | 4 subtests PASS |
| `internal/store/sqlite/schema.go` | 4 additive tables + UNIQUE/CHECK | ✓ VERIFIED | 396/421/435/460 |
| `internal/store/sqlite/feedback.go` | INSERT-only votes/refs/dispositions/queue/metrics | ✓ VERIFIED | 581 lines; zero UPDATE/DELETE |
| `internal/store/sqlite/tickets.go` | idempotent ticket rows | ✓ VERIFIED | 134 lines |
| `internal/ticket/{ticket,registry,webhook}.go` | Provider + registry + webhook outbound | ✓ VERIFIED | 99/72/163 lines, stdlib only |
| `internal/wire/chunk.go` | data-turn + TurnPayload | ✓ VERIFIED | 174-190, 480 |
| `internal/server/feedback_wire.go` | emitTurnID with <=0 zero-byte guard | ✓ VERIFIED | 58-69 |
| `internal/server/handlers_session_history.go` | turn_id + voted_verdict echo | ✓ VERIFIED | 69-70, 155, 4 branches |
| `internal/server/feedback.go` | 4 handlers + admin gate order + FBK-05 lock | ✓ VERIFIED | 817 lines; isAdminRequest first statement ×3 |
| `internal/server/tickets.go` | ticketsHandler + async dispatch | ✓ VERIFIED | 419 lines |
| `cmd/server/main.go` | 5 SKILLFORGE_TICKET_* envs + fail-fast | ✓ VERIFIED | 354-373 |
| `internal/server/server.go` | 5 routes bound once | ✓ VERIFIED | 565-568, 578 |
| `web/src/types/wire-types.ts` | data-turn union member + TurnPayload | ✓ VERIFIED | 380-388, 454 (D15 paired) |
| `web/src/lib/chat-restore.ts` | data-turn part synthesis | ✓ VERIFIED | 149-155 |
| `web/src/components/chat/parts/FeedbackRow.tsx` | real state machine | ✓ VERIFIED | 444 lines, zero placeholders |
| `web/src/components/chat/MessageBubble.tsx` | data-turn scan + props wiring | ✓ VERIFIED | 295-300 |
| `web/src/components/layout/SfToast.tsx` | listener + single-slot viewport | ✓ VERIFIED | 76, 84, 94 |
| `web/src/components/admin/{FeedbackMetricsPanel,ReviewQueuePanel}.tsx` | 2 new panels | ✓ VERIFIED | 450/530 lines, verbatim copy |
| `web/src/components/admin/AdminDashboard.tsx` | 7 tabs + SfToast | ✓ VERIFIED | 170-212 |
| `web/src/store/app-store.ts` | union-only change | ✓ VERIFIED | 457-467, 533 |
| `scripts/verify-phase-29.sh` | 4-stage gate | ✓ VERIFIED | exit 0 re-run by verifier |
| `Makefile` | phase-29 target | ✓ VERIFIED | 9, 106-113 |
| `docs/threat-model/STRIDE-29.md` | threat register | ✓ VERIFIED | 7 boundaries, 10 threats |
| `.planning/.../29-UAT.md` | operator runbook | ✓ VERIFIED | 9 tests incl. joint acceptance |
| Test files (12) | behavior locks | ✓ VERIFIED | all PASS in WSL/vitest runs |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| store.Store interface | sqlite feedback/tickets impl | `var _ store.Store` compile gate | ✓ WIRED | `go build ./...` exit 0 (WSL) |
| writeAuditAndTurn | store.AppendTurn return value | (int64, error) signature | ✓ WIRED | chat.go:552-609 |
| pipeline/chat/runner emission sites | emitTurnID | turn write → frame → terminal | ✓ WIRED | 11 exact sites, frame-order tests PASS |
| feedbackHandler | store.QueryAudit (AuditFilter) | audit-nearby attribution | ✓ WIRED | feedback.go:262-279 |
| feedbackHandler | retriever.GetChunk().Tier | chunk→kb_slug refs | ✓ WIRED | feedback.go:293-324 |
| ticketsHandler | ticket.Provider.Dispatch | goroutine, request ctx independent | ✓ WIRED | tickets.go:325-353 |
| dispatch goroutine | store.PutAudit | INSERT-only audit rows | ✓ WIRED | tickets.go:401-410 |
| handlers_session_history | store.VotedVerdictsBySession | one batch query | ✓ WIRED | 1 call site |
| MessageBubble | data-turn part | parts.find | ✓ WIRED | MessageBubble.tsx:300 |
| FeedbackRow | POST /api/feedback + /api/tickets | raw fetch | ✓ WIRED | 197, 223 |
| SfToast | window 'sf-toast' CustomEvent | addEventListener | ✓ WIRED | 4 mounts total |
| ReviewQueuePanel | review + dispositions endpoints | token'd fetch | ✓ WIRED | 154, 233 |
| FeedbackMetricsPanel | metrics endpoint + cross-link | fetch + setAdminSubroute('review') | ✓ WIRED | 222, 442 |
| Makefile phase-29 | scripts/verify-phase-29.sh | bash invocation | ✓ WIRED | exit 0 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| FeedbackRow | turnId / votedVerdict | live data-turn SSE frame + history-synthesized part | Yes — store-assigned session_turn.id, GetSession/VotedVerdictsBySession queries | ✓ FLOWING |
| FeedbackRow | vote result | POST /api/feedback → PutFeedback (real sqlite) | Yes — DB row + UNIQUE replay | ✓ FLOWING |
| ReviewQueuePanel | pending rows | GET /api/admin/feedback/review → PendingReviewChunks (derived SQL over feedback JOIN refs) | Yes | ✓ FLOWING |
| FeedbackMetricsPanel | totals/by_key/trend | GET /api/admin/feedback/metrics → real GROUP BY aggregations | Yes — no static returns found in any new endpoint | ✓ FLOWING |
| FeedbackRow ticket form | ticket result | POST /api/tickets → PutTicket (real sqlite) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Backend build + phase packages | `wsl go build ./... && go test ./internal/{store,server,ticket,wire}/... -count=1` | ok ×7 packages, 0 FAIL | ✓ PASS |
| turn-id semantics incl. conflict→0 | `go test ./internal/store/sqlite/ -run TestAppendTurnReturnsAssistantID -v` | 4/4 subtests PASS | ✓ PASS |
| Store-layer INSERT-only locks + pending/metrics | `go test -run 'TestFeedbackInsertOnly\|TestTicketsInsertOnly\|TestPendingReview\|TestFeedbackMetrics' -v` | all PASS | ✓ PASS |
| Vote/ticket endpoint behavior matrices | `go test ./internal/server/ -run 'TestFeedbackHandler\|TestTicketsHandler\|TestFeedbackDisposition\|TestFeedbackReview\|TestFeedbackMetrics\|TestFeedbackNoTrustMutation\|TestTicketDispatch' -v` | 8/8 PASS (incl. non-blocking + audit rows) | ✓ PASS |
| data-turn frame ordering on all shapes | `go test ./internal/server/ -run 'DataTurn\|TurnID\|TestEmitTurn' -v` | 10/10 PASS | ✓ PASS |
| FE full regression | `npx vitest run` | 74 files, 595/595 PASS | ✓ PASS |
| FE type safety | `npx tsc --noEmit` | exit 0 | ✓ PASS |
| Structural hygiene gate | `bash scripts/verify-phase-29.sh` | exit 0, 4 stages PASS | ✓ PASS |
| Runnable live-stack flows (SSE frame in browser, real clicks, live webhook) | not run — requires live server + browser + external endpoint | — | ? SKIP → human verification |

### Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `scripts/verify-phase-29.sh` (declared + conventional) | `bash scripts/verify-phase-29.sh` | exit 0; Stage 1 TODO clear (+deep.TODO anti-false-positive), 2a/2b/2c FBK-05 locks, Stage 3 D15 pairing (Go 5 / TS 4), 4a/4b/4c DDL discipline | PASS |
| Negative self-proof of the gate (claimed in 29-UAT Test 9.3) | inject `UPDATE ` comment → gate | recorded in UAT as executed 2026-09-08; verifier confirmed the gate's grep logic reads real counts (Stage 2a FAIL path points at file+line) | PASS (evidence accepted: gate source inspected + positive run reproduced) |

### Requirements Coverage

All 9 requirement IDs mapped to Phase 29 in REQUIREMENTS.md (line 546) are claimed by plan frontmatter — no orphaned requirements.

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| FBK-01 | 01, 02, 04, 05, 09 | 反馈落库 INSERT-only + 一次一票幂等 | ✓ SATISFIED | Truths 1-4; `POST /api/feedback` + feedback table + UNIQUE key |
| FBK-02 | 02, 05, 08, 09 | 过时复核队列 + /admin 处置 + 处置审计 | ✓ SATISFIED | Truth 7; review/disposition endpoints + audit row + admin panel |
| FBK-03 | 02, 05, 08, 09 | 反馈指标进 admin 仪表盘 per-Skill/per-KB + 趋势 | ✓ SATISFIED | Truth 8; metrics endpoint + FeedbackMetricsPanel |
| FBK-04 | 01, 04, 07, 09 | FE FeedbackRow 接真，锁定态服务端驱动 | ✓ SATISFIED | Truth 6; real fetch + server-driven lock + honest toasts (live UI in human list) |
| FBK-05 | 05, 09 | 反馈绝不自动改信任 | ✓ SATISFIED | Truth 12; TestFeedbackNoTrustMutation + verify gate Stage 2 (double lock) |
| TKT-01 | 03 | TicketService 接口 + none/webhook 注册表 | ✓ SATISFIED | Truth 10; one-method Provider + registry |
| TKT-02 | 02, 06, 09 | POST /api/tickets INSERT-only + 外发结果进审计 | ✓ SATISFIED | Truth 9/11; idempotent rows + dispatch audit rows |
| TKT-03 | 07, 08, 09 | FE 创建工单接真，诚实展示三态 | ✓ SATISFIED | Truth 9; 4-state copy + tests (live UI in human list) |
| TKT-04 | 06, 09 | 工单外发不阻塞主链路 | ✓ SATISFIED | Truth 11; structural goroutine isolation + non-blocking test |

### Code Review Findings Assessment (29-REVIEW.md factored honestly)

**CR-01 (critical) — SSE-emit-failure persists a duplicate audit row + duplicate votable turn pair.** Confirmed present in the code: on `emitVerdictStream` failure the verdict path persists at `pipeline.go:556` and returns the error, and `chatHandler`'s error branch (`chat.go:317`) persists a second audit row + turn pair with its own `time.Now()` timestamps, so both insert. **However, verified against git history (`8cbe270`, pre-phase-29), the identical double-write shape already existed on every emit-failure path (verdict + all refusal sites: `emitVerdictStream/emitRefusal` → `writeAuditAndTurn` → `return err`).** Phase 29 added zero new write calls on these paths; what it added is vote-key emission on the error turn, which makes the pre-existing duplicate votable and double-countable in metrics/review queue. Verdict: **not a falsified Phase 29 must-have** (one-vote-per-turn idempotency at the DB layer holds; every data-turn still precedes finish; the duplication is a pre-existing GEVL-03-adjacent defect), but it is a real defect with phase-amplified blast radius and should be fixed as an immediate post-phase item (guard the error branch with a "pipeline already persisted" signal, per the review's suggested fix). No later phase in the milestone covers it — Phase 30 is engineering-hygiene closeout, not this fix.

Warnings confirmed in code, none falsifying a must-have:
- **WR-01** — a failed `PutFeedbackChunkRefs` is never backfilled on the replay path (`feedback.go:230-236` returns before refs); vote survives, per-KB attribution permanently lost for that vote.
- **WR-02** — no boot/ticker sweep for `dispatch_status='queued'` rows; a process restart between the 201 and webhook settle leaves the row queued forever with no trail (verified: no re-read of queued rows exists).
- **WR-03** — `webhook.go:94` clones the transport but sets no `CheckRedirect`, so Go follows up to 10 redirects and forwards `X-SkillForge-Ticket-Secret` to redirect targets. Severity capped by the URL being operator config.
- **WR-06** — `FeedbackRow.tsx:170` seeds the server lock only at mount; a late-arriving data-turn part would leave the row clickable until a submit self-corrects via the 200 already_recorded branch. Both shipped consumption paths (live pre-finish emit, restore-time synthesis) populate the part before mount, so impact is bounded.

Info-level findings (IN-01..IN-08) from 29-REVIEW.md are noted and not re-adjudicated here; none block the phase goal.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| internal/server/tickets.go | 325-385 | queued ticket never re-dispatched after restart (no sweep/started-marker) | ⚠️ Warning | silent loss of outbound half on restart; admission (201) is durable and honest |
| internal/ticket/webhook.go | 94 | missing CheckRedirect (secret forwarded on redirects) | ⚠️ Warning | hardening gap; operator-configured URL caps severity |
| internal/server/feedback.go | 230-236 | refs not backfilled on idempotent replay | ⚠️ Warning | per-KB attribution loss in a partial-failure window |
| web/src/components/chat/parts/FeedbackRow.tsx | 170 | lock seeded from prop only at mount | ⚠️ Warning | mitigated by pre-mount part population + server idempotency |
| (all phase files) | — | TODO/FIXME/TBD/XXX debt markers, placeholder text, stub returns | ✓ none | `grep 功能即将上线` = 0; debt-marker scan clean (only the gate's own self-referential grep literals) |

### Human Verification Required

See the 9 items in the frontmatter `human_verification` list — they correspond to 29-UAT.md Tests 1-8 plus a visual pass. All are live-stack/browser/external-webhook items that the automated suites deliberately cannot cover. Test 9 of the UAT (structural gate) was re-executed by the verifier and passes.

### Gaps Summary

No gaps at the must-have level: all 12 merged truth groups are verified in code with passing hermetic locks, the 4-stage structural gate passes on re-run, the full Go suite (phase packages) and the 595-test FE suite are green, and all 9 requirement IDs (FBK-01..05, TKT-01..04) are implemented and accounted for. The phase goal — four FeedbackRow buttons producing real results, best-effort non-blocking ticket dispatch, and feedback never auto-mutating trust — is achieved at the code level.

The status is `human_needed` (not `passed`) solely because the shipped UX behaviors — live SSE framing in the browser, click-to-lock flows, admin disposition interaction, toast copy, live webhook dispatch, and the Phase 27→29 joint acceptance — require an operator run of 29-UAT.md Tests 1-8.

One decision item for the developer (not a gap): whether to schedule the CR-01 fix (suppress the error-branch duplicate write when the pipeline already persisted) as an immediate post-phase task — the duplicate write predates Phase 29, but Phase 29 makes the duplicate votable.

---

_Verified: 2026-09-08T06:50:00Z_
_Verifier: Claude (gsd-verifier)_
