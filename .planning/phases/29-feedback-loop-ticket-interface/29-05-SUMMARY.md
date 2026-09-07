---
phase: 29-feedback-loop-ticket-interface
plan: 05
subsystem: http-api
tags: [hertz, feedback, idempotency, admin-gate, audit-trail, metrics, source-grep-lock, fbk-05]

# Dependency graph
requires:
  - phase: 29-feedback-loop-ticket-interface (29-02)
    provides: the 12-method feedback storage surface (PutFeedback/FeedbackByTurn/PutFeedbackChunkRefs/PutDisposition/PendingReviewChunks/ResolvedDispositions/PendingReviewCount/metrics×2/trend/GetSessionTurn) — every read and write the four handlers make
  - phase: 29-feedback-loop-ticket-interface (29-04)
    provides: data-turn frame + history turn_id echo — the server-assigned turn id the vote endpoint keys on (a client-invented id fails GetSessionTurn)
  - phase: 26 (KBM)
    provides: knowledge_bases slug registry — retriever chunk Tier IS the kb_slug written into refs
provides:
  - POST /api/feedback: idempotent public vote endpoint with SERVER-side attribution (audit-nearby copy, never the request body)
  - GET /api/admin/feedback/review + POST /api/admin/feedback/dispositions + GET /api/admin/feedback/metrics (CHECK ORDER 403-before-503, closed_count before-write caliber, full-denominator Go-side rates)
  - action_feedback_disposition audit row (ActorType admin, JSON snapshot in FrozenArgs) — the repudiation trail of the only path feedback influences anything
  - TestFeedbackNoTrustMutation — the FBK-05 red line as a source-grep test fact (no UPDATE/DELETE/Insert(, retriever only via GetChunk, zero Indexer refs across the feedback/ticket storage family)
  - 4 routes bound exactly once in server.go
affects: [29-06 tickets endpoint (same write-endpoint shape), 29-07 FE FeedbackRow (wire contract frozen here), 29-08 admin feedback tab (review/metrics wire shapes frozen here), 29-09 verify gate (greps the locks this plan ships)]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "kb_admin.go mirror: admin gate as FIRST statement + CHECK ORDER comment + fail-closed nil-dependency 503 + audit-row best-effort posture"
    - "handlers_session_history.go mirror: sessionIDRegex before any SQL + graphResponseError envelope + [WARN]-log-degrade-never-500 posture"
    - "nil-embed test fakes (50f435f): fake store overrides ONLY the methods the handler calls; stub retriever exposes only GetChunk (hostile by design)"
    - "real in-memory sqlite for lifecycle tests: the pending derivation/reopen semantics under test are the store's own, never a test-local copy"
key-files:
  created:
    - internal/server/feedback.go
    - internal/server/feedback_test.go
  modified:
    - internal/server/server.go

key-decisions:
  - "metrics totals are ALWAYS the strict once-per-vote per-Skill aggregation regardless of dim — the per-KB dimension double-counts by design (29-02 Open Q2), and a total that double-counts is simply a wrong number; by_key serves the selected dimension"
  - "rates are computed Go-side as *float64 with the full-vote denominator (D-29-10); nil serializes as JSON null so 'no votes yet' never renders as a 0% that reads like 'everyone voted unhelpful'"
  - "closed_count is counted BEFORE PutDisposition (先查后写) because the pending view is derived from 'disposition ts >= vote ts' — counting after the write would always answer 0"
  - "refs write failure after the vote row persisted answers 500 (not a silent 201): per-KB attribution would vanish without a trace; a client retry lands safely on already_recorded"
  - "disposition filter on the pending branch is a 400, not a silent ignore — 'honored but ignored' would lie about what was filtered"
  - "404 on a non-votable turn is ONE merged branch (missing / user-role / foreign session) with a byte-identical envelope — the distinction is exactly the information a probing caller wants"
  - "REQUIREMENTS rows stay Pending per the phase-close convention (29-02/29-04 precedent) — but FBK-01 and FBK-05's substance is fully delivered here; FBK-02/03 additionally await the 29-08 admin views that their row text names"
  - "maxFeedbackCommentChars is a hard 2000-rune constant (Phase 30 may promote to env); rune count not bytes because the product's comments are mostly Chinese"

patterns-established:
  - "CHECK ORDER made executable: a source grep asserting every admin handler's FIRST statement is the isAdminRequest gate (comment lines tolerated, statements not)"
  - "clock-relative test fixtures for store-stamped timestamps: a fixed wall-date can drift into the store's future and silently flip d.ts >= f.ts comparisons"

requirements-completed: [FBK-01, FBK-05] # substance delivered (backend complete); REQUIREMENTS.md rows flip at phase close per the 29-02/29-04 convention — FBK-02/03 additionally await the 29-08 render halves their row text names

# Metrics
duration: 15min
completed: 2026-09-08
---

# Phase 29 Plan 05: 反馈/复核/指标 HTTP 面 Summary

**FBK-01/02/03/05 的 HTTP 面一次成型：幂等投票端点（服务端 audit 邻近归因，绝不信任 FE）+ 三个 admin 端点（403 先于 503、处置只追加状态行与审计行、全票分母 null 语义）——FBK-05 红线由 source-grep 结构锁变成测试事实。**

## Performance

- **Duration:** 15 min
- **Started:** 2026-09-07T20:30:00Z (approx, Task 1 RED)
- **Completed:** 2026-09-07T20:47:34Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- `POST /api/feedback`: fixed gate order (body → sessionIDRegex → verdict closed set → turn_id>0 → 2000-rune comment cap → Store-nil 503 → GetSessionTurn existence + role binding), D-29-05 attribution copied from the turn's own audit row via parameterized `QueryAudit{SessionID, Until: turn.Ts, Limit: 1}`, UNIQUE-key replay → `200 already_recorded` with the FIRST verdict and zero extra writes, honest 2xx degradation on audit-gap AND retriever-gap
- Admin surface: review queue (pending rows + badge total / disposition history, disjoint wire halves), disposition verb (closed_count counted before the write, INSERT-only state row + `action_feedback_disposition` audit row with JSON snapshot), metrics view (full-vote denominator rates Go-side, 0-vote → null, per-KB by_key double-count by design while totals stay strictly once-per-vote, pending_count cross-link for 29-08)
- `TestFeedbackNoTrustMutation`: the FBK-05 red line executable — no mutation verb in feedback.go, retriever reachable only through the GetChunk read, zero knowledge-ingestion references across server/feedback.go + store/sqlite/{feedback,tickets}.go, four routes bound exactly once, admin gate as every admin handler's first statement
- Disposition lifecycle (close + reopen + audit row) proven against a REAL in-memory sqlite store, so the pending derivation tested is the store's own 29-02 semantics

## Task Commits

Each task was committed atomically (TDD RED → GREEN):

1. **Task 1 RED: POST /api/feedback behavior matrix + hermetic fakes** - `14b0e17` (test)
2. **Task 1 GREEN: 幂等投票 + 服务端归因 + 路由注册** - `e7cfb17` (feat)
3. **Task 2 RED: admin 门序 + 处置生命周期 + 指标口径 + FBK-05 结构锁** - `046a37c` (test)
4. **Task 2 GREEN: admin 三端点 + 结构锁落地** - `5898fc7` (feat)

_Note: both RED commits are documented build failures on the undefined handler surface (deps.feedbackHandler / feedbackReviewHandler undefined) — the same advisory posture as 29-02 and 29-03, where interface and implementation must share a commit or the package does not compile._

**Plan metadata:** this commit (docs: complete plan)

## Files Created/Modified
- `internal/server/feedback.go` — feedbackHandler + attribution/refs helpers, feedbackReviewHandler, feedbackDispositionHandler (+audit helper), feedbackMetricsHandler (+rate helpers), closed vocabularies, wire types
- `internal/server/feedback_test.go` — feedbackFakeStore (nil-embed, 13 overridden methods) + fbStubRetriever + 4 test functions (25 sub-tests total) incl. the structural red line
- `internal/server/server.go` — Phase 29 comment block + 4 route registrations (each exactly once)

## Wire Contract (frozen, consumed by 29-07/29-08)

- `POST /api/feedback` `{session_id, turn_id, verdict, comment?}` → `201 {status:"recorded",verdict}` | `200 {status:"already_recorded",verdict}` | `400 {error}` | `404 {error}` | `500` | `503`
- `GET /api/admin/feedback/review?status=pending|resolved[&disposition=…]` → `200 {pending:[{chunk_id,kb_slug,report_count,last_reported_at,comments[]}] ,total}` | `200 {dispositions:[{id,ts,chunk_id,disposition,note,actor}]}`
- `POST /api/admin/feedback/dispositions` `{chunk_id, disposition, note?}` → `201 {closed_count}` | `400` | `403` | `503`
- `GET /api/admin/feedback/metrics?dim=skill|base&days=N` → `200 {dim, days, totals:{helpful,unhelpful,outdated,help_rate,outdated_rate,total_votes,pending_count}, by_key:[{key,helpful,unhelpful,outdated,help_rate,outdated_rate}], trend:[{day,verdict,count}]}`

## Decisions Made
- **totals ignore the dim knob:** the per-KB dimension counts a multi-KB vote once per KB (Open Q2), so summing those rows for totals would report more votes than were cast. totals always come from FeedbackMetricsBySkill (strictly once-per-vote), matching the 29-02 store godoc ("any total counts every vote strictly once"); by_key serves the selected dimension. Locked by a sub-test whose KB rows sum to 4 for 3 actual votes.
- **refs-write failure = 500, not silent 201:** the vote row already consumed its UNIQUE key, so the refs are unrecoverable for that vote; surfacing 500 makes the loss loud and the safe retry path (already_recorded) explicit.
- **uniform 404:** missing turn / user turn / foreign-session turn share one branch and one byte-identical envelope (T-29-06).
- **disposition filter on pending → 400:** it is a resolved-branch dimension; silently ignoring it would let an operator believe a filter was applied.
- **REQUIREMENTS rows held Pending (phase-close convention), substance recorded:** FBK-01's substance (write path + idempotency + server-side attribution) and FBK-05's structural red line are fully delivered here; FBK-02/03's row text additionally names the /admin 复核视图/仪表盘 that 29-08 renders. Rows flip at 29-09 closeout per the 29-02/29-04 precedent.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test fixture timestamps drifted into the store's future**
- **Found during:** Task 2 (GREEN run)
- **Issue:** the disposition lifecycle fixtures used fixed wall-dates (2026-09-08 08:00Z) while the wall clock was 04:42Z — the server-stamped disposition ts lost the `d.ts >= f.ts` comparison, so a dispositioned chunk stayed pending and the reopen case counted a phantom report
- **Fix:** fixtures made clock-relative (votes at now-1h; the reopen vote at now+1s so it is strictly later than the disposition stamp by construction)
- **Files modified:** internal/server/feedback_test.go
- **Verification:** both lifecycle sub-tests green; pattern recorded in key-decisions as a future-test hazard
- **Committed in:** 5898fc7

**2. [Rule 1 - Bug] Structural-lock relative path pointed one directory too high**
- **Found during:** Task 2 (GREEN run)
- **Issue:** the test read `../../store/sqlite/feedback.go` from package dir `internal/server` (file-not-found); the correct path is `../store/sqlite/…`
- **Fix:** path corrected; the three-file zero-Indexer assertion now actually runs
- **Files modified:** internal/server/feedback_test.go
- **Committed in:** 5898fc7

**3. [Rule 1 - Bug] First-statement gate grep rejected the multi-line CHECK ORDER comment**
- **Found during:** Task 2 (GREEN run)
- **Issue:** the regex allowed only `if !isAdminRequest(c) {` on the line directly after `{`, but the handlers carry a two-line CHECK ORDER comment first — a false positive that would have forced the comment to be deleted
- **Fix:** regex tolerates comment lines between the brace and the gate (`(?:\s*//[^\n]*\n)*`), still rejecting any executable statement before the gate
- **Files modified:** internal/server/feedback_test.go
- **Committed in:** 5898fc7

---

**Total deviations:** 3 auto-fixed bugs, all test-side. **Impact on plan:** zero scope change; all acceptance criteria hold.

## Issues Encountered
- **Harness coupling across tasks:** `newFeedbackHertz` originally registered all four routes, which would have made the Task 1 GREEN commit reference the not-yet-written admin handlers. The harness now grows with the surface (public route in Task 1, admin routes in Task 2), keeping each commit self-consistent.
- **server.go carries pre-existing gofmt deltas** (Detacher interface block, GraphLimits comment alignment) unrelated to this plan — left untouched per scope boundary; this plan's files are gofmt-clean via WSL gofmt.

## User Setup Required

None — no new env vars or services. The admin endpoints reuse the existing `SKILLFORGE_ADMIN_TOKEN` + `X-Admin-Token` gate.

## Known Stubs

None — all four handlers are wired end-to-end against the real store surface; the vote path is live once 29-07 renders the buttons.

## Next Phase Readiness
- 29-06 (tickets endpoint) can mirror `feedbackHandler`'s shape verbatim: same gate order, GetSessionTurn binding, UNIQUE-key replay → `TicketByTurn` read-back, `TKT-{id}` derivation from the read-back row
- 29-07 (FE FeedbackRow) consumes the frozen `{status, verdict}` envelope; the replay 200 + echoed verdict IS the server-driven lock state (no client-side vote memory needed)
- 29-08 (admin feedback tab) consumes review + metrics shapes; `pending_count` rides the metrics totals for the cross-link badge; chunk content preview intentionally routes through the existing `GET /api/knowledge/chunk/:id` (zero new read surface)
- 29-09 (verify gate) can grep the locks this plan ships: TestFeedbackNoTrustMutation already asserts zero mutation verbs / GetChunk-only retriever access / once-each route binding

## Self-Check: PASSED

- Files verified on disk: internal/server/feedback.go, internal/server/feedback_test.go, internal/server/server.go (modified), 29-05-SUMMARY.md — all FOUND
- Commits verified in git log: 14b0e17 (Task 1 RED), e7cfb17 (Task 1 GREEN), 046a37c (Task 2 RED), 5898fc7 (Task 2 GREEN) — all FOUND
- Plan-level verification re-run at HEAD: WSL `go build ./...` exit 0 + `go test ./internal/server/... -count=1` green (14.5s) + `go vet ./internal/server/` clean + gofmt clean on both plan files + 4 routes grepped exactly once each

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
