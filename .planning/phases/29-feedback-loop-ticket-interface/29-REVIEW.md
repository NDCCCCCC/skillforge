---
phase: 29-feedback-loop-ticket-interface
reviewed: 2026-09-07T22:27:36Z
depth: standard
files_reviewed: 45
files_reviewed_list:
  - cmd/server/main.go
  - internal/server/admin_audit_test.go
  - internal/server/admin_schedules_test.go
  - internal/server/chat.go
  - internal/server/chat_session_test.go
  - internal/server/feedback.go
  - internal/server/feedback_test.go
  - internal/server/feedback_wire.go
  - internal/server/feedback_wire_test.go
  - internal/server/handlers_session_history.go
  - internal/server/handlers_session_history_test.go
  - internal/server/pipeline.go
  - internal/server/pipeline_history_test.go
  - internal/server/runner_setup.go
  - internal/server/server.go
  - internal/server/tickets.go
  - internal/server/tickets_test.go
  - internal/server/upload_test.go
  - internal/store/sqlite/feedback.go
  - internal/store/sqlite/feedback_test.go
  - internal/store/sqlite/mattn_zero_regression_test.go
  - internal/store/sqlite/schema.go
  - internal/store/sqlite/session.go
  - internal/store/sqlite/session_meta_test.go
  - internal/store/sqlite/session_turnid_test.go
  - internal/store/sqlite/tickets.go
  - internal/store/sqlite/tickets_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/ticket/registry.go
  - internal/ticket/registry_test.go
  - internal/ticket/ticket.go
  - internal/ticket/webhook.go
  - internal/ticket/webhook_test.go
  - internal/wire/chunk.go
  - internal/wire/chunk_test.go
  - scripts/verify-phase-29.sh
  - web/src/App.tsx
  - web/src/__tests__/AdminFeedbackTabs.test.tsx
  - web/src/__tests__/FeedbackMetricsPanel.test.tsx
  - web/src/__tests__/FeedbackRow.test.tsx
  - web/src/__tests__/MessageBubble.test.tsx
  - web/src/__tests__/ReviewQueuePanel.test.tsx
  - web/src/__tests__/SfToast.test.tsx
  - web/src/components/admin/AdminDashboard.tsx
  - web/src/components/admin/FeedbackMetricsPanel.tsx
  - web/src/components/admin/ReviewQueuePanel.tsx
  - web/src/components/chat/MessageBubble.tsx
  - web/src/components/chat/parts/FeedbackRow.tsx
  - web/src/components/layout/SfToast.tsx
  - web/src/lib/chat-restore.test.ts
  - web/src/lib/chat-restore.ts
  - web/src/store/app-store.ts
  - web/src/types/wire-types.ts
findings:
  critical: 1
  warning: 6
  info: 8
  total: 15
status: issues_found
---

# Phase 29: Code Review Report

**Reviewed:** 2026-09-07T22:27:36Z
**Depth:** standard
**Files Reviewed:** 45 (all production files in scope read in full; test files reviewed for reliability only)
**Status:** issues_found

## Summary

Phase 29 (feedback loop + ticket interface) is a large, unusually well-documented surface: the INSERT-only storage discipline is real (I traced every write path in `internal/store/sqlite/{feedback,tickets}.go` — only three INSERT verbs plus `appendTurnHalf`, zero UPDATE/DELETE), the mattn `LastInsertId` stale-rowid trap is correctly avoided in `PutFeedback`/`PutTicket` (RowsAffected guard + read-back instead of LastInsertId), and the async ticket dispatch is structurally non-blocking with `context.Background()` + budget + `defer recover` as specified. The wire contract between Go emitters and `web/src/types/wire-types.ts` / `chat-restore.ts` is consistent (I cross-checked `feedbackResponse`, `ticketResponse`, `historyEntry.turn_id/voted_verdict`, `TurnPayload`, and the metrics/review envelopes field-by-field). Admin gate ordering (`isAdminRequest` as first statement, 403 before the nil-Store 503) holds on all three feedback admin handlers, and the FBK-05 red line holds: the only retriever call on the feedback surface is `GetChunk`.

The one critical defect is an ordering consequence of the new write-first/data-turn contract: when the verdict or refusal SSE emit fails after `writeAuditAndTurn` has already persisted (the common client-disconnect case), `chatHandler`'s error branch persists a **second** audit row and a **second** (user, assistant) turn pair, violating GEVL-03 and producing duplicated, independently-votable restored messages. The remaining warnings are concentrated in the idempotency/abuse edges the phase context asked to be checked: unrecoverable loss of `feedback_chunk_refs` on a partial write, queued tickets that are silently lost on process restart, webhook secret leakage on cross-origin redirects, a misattribution race in the audit near-lookup, and the unauthenticated/unthrottled shape of the two public write endpoints once a live webhook provider is wired.

No source files were modified during review.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: SSE emit failure after write-first persists a duplicate audit row and a duplicate votable turn pair

**File:** `internal/server/pipeline.go:543-559` (write + `return emitErr`) interacting with `internal/server/chat.go:288-328` (error branch writes again); same shape on every refusal site (`pipeline.go:227-235, 286-294, 315-323, 354-367, 531-539`).

**Issue:** The verdict path persists the audit row + (user, assistant) turn pair at `pipeline.go:556`, and *then* returns `emitErr` if `emitVerdictStream` (line 543) failed — which is exactly what happens when a client disconnects mid-stream (a routine event, not an edge case). The returned error lands in `chatHandler`'s error branch, which unconditionally writes **another** audit row with `refusal_reason="internal_error"` plus another (user, assistant) turn pair (`chat.go:317`) and emits *that* turn's id as the vote key. Because `session_turn`'s idempotency key is `UNIQUE(session_id, role, ts)` and the two writes use separate `time.Now()` calls, both pairs insert.

Consequences, all newsworthy under Phase 29 specifically:

1. GEVL-03 ("exactly one audit row per query") is violated — two audit rows for one query.
2. `GET /api/session/:id/history` restores the answer **twice** plus an `internal_error` message, because `GetSession` returns all four turn halves.
3. Both assistant rows carry their own `turn_id` (29-04 fills `TurnID` for every assistant branch), so the same answer is **votable twice** — two `UNIQUE(session_id, turn_id)` votes for one logical reply, double-counting in FBK-03 metrics and the FBK-02 review queue.

The duplicate-write behavior predates this phase, but 29-04's write-first reordering deliberately kept both write sites and added vote-key emission to the error turn, which upgrades the duplicate from an audit nuisance to a feedback-integrity bug.

**Fix:**

```go
// pipeline.go — report whether this pipeline invocation already persisted,
// and let the handler branch on it instead of writing unconditionally.
turnID, persisted := s.writeAuditAndTurn(...)   // Step 11 site
if emitErr != nil {
    return &pipelineEmitError{err: emitErr, persisted: persisted}
}

// chat.go error branch — persist only if the pipeline did not.
if !persisted {
    turnID, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query,
        verdict.Verdict{}, "", "internal_error", nil)
}
```

(A simpler variant that preserves the error-branch contract: give `VerdictPipeline` a boolean/`turnID` out-param, or have `writeAuditAndTurn` reuse the same `ts` value for both halves so a same-request second write hits the `ON CONFLICT DO NOTHING` replay path and returns `turnID=0` — though the explicit flag is the clearer fix.)

## Warnings

### WR-01: A failed `feedback_chunk_refs` write is unrecoverable — the vote is permanently lost to per-KB metrics and the review queue

**File:** `internal/server/feedback.go:207-249`; `internal/store/sqlite/feedback.go:129-140`

**Issue:** `PutFeedback` (the vote row) and `PutFeedbackChunkRefs` (the per-chunk attribution rows) are two non-transactional writes. If the refs write fails, the handler returns 500 — but the vote row is already durable. The documented client retry then hits the `inserted=false` branch (`feedback.go:231-235`), which **never calls `PutFeedbackChunkRefs`**, so the refs are never backfilled. Result: a "recorded" vote that silently never appears in `PendingReviewChunks` (it JOINs `feedback_chunk_refs`) or in `FeedbackMetricsByKB`. A crash between the two writes has the same effect. The code comment acknowledges the trade-off, but "a client retry safely lands on the already_recorded path" is precisely the problem — the retry cannot heal the gap.

**Fix:** Either make the store call transactional (`PutFeedbackWithRefs(ctx, row, refs) (bool, error)` in one `BeginTx`), or backfill on the replay path — `ON CONFLICT(feedback_id, chunk_id) DO NOTHING` makes this idempotent and INSERT-only:

```go
if !inserted {
    // Replay: refs may be missing from a previously failed write; the
    // UNIQUE(feedback_id, chunk_id) key makes the backfill a no-op when
    // they already exist.
    _ = s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs)
    c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", Verdict: saved.Verdict})
    return
}
```

### WR-02: Queued tickets are silently lost on process restart — no durable retry or sweep for the async dispatch

**File:** `internal/server/tickets.go:325-385` (`dispatchForTicket`), `internal/store/sqlite/tickets.go:47-102`

**Issue:** The 201 is returned the moment the row is durable (correct per TKT-04), but the outbound half lives only in a fire-and-forget goroutine. If the process exits (deploy, crash, operator restart) after the 201 and before the webhook settles, the row stays `dispatch_status='queued'` forever: the INSERT-only discipline forbids flipping it, and nothing ever re-reads `tickets WHERE dispatch_status='queued'`. The user was told the ticket exists ("正在排队外发"); the operator gets no audit row at all — the failure mode is *silence*, which is worse than the failed-dispatch audit row the panic/error paths produce. A goroutine whose provider ignores `ctx` can also outlive the budget (contract violation, but nothing bounds it structurally).

**Fix:** Add an INSERT-only recovery sweep (no UPDATE needed): at boot and/or on a ticker, re-read queued rows older than some threshold and re-dispatch through the provider (providers are idempotent-shaped here — the payload carries the stable `TKT-{id}` number), emitting a second `action_ticket_dispatch_*` audit row per attempt series so the trail shows the retry. If a sweep is out of scope for Phase 29, at minimum emit an `action_ticket_dispatch_started` audit row *before* the first attempt so a lost goroutine leaves a visible "started, never settled" trail.

### WR-03: Webhook client follows redirects and forwards `X-SkillForge-Ticket-Secret` to the redirect target (secret leak + SSRF pivot)

**File:** `internal/ticket/webhook.go:93-96` (client construction), `140-163` (`postOnce`)

**Issue:** `http.Client` is built from a cloned `DefaultTransport` with no `CheckRedirect`, so it follows up to 10 redirects. Go's redirect logic only strips the `Authorization` header on cross-origin hops; custom headers such as `SecretHeader` **are copied to the redirect target**. A compromised or hijacked webhook host (or any host that can answer the operator's configured URL) can 302 the POST — carrying the full ticket payload *and the shared secret* — to any other destination, including internal-only addresses (SSRF pivot: `http://169.254.169.254/`, internal admin panels). The URL is operator-configured (T-29-02 correctly keeps user text out of it), which caps the severity, but the secret-forwarding behavior is unconditional.

**Fix:**

```go
client: &http.Client{
    Transport: http.DefaultTransport.(*http.Transport).Clone(),
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 3 {
            return fmt.Errorf("ticket webhook: too many redirects")
        }
        if req.URL.Host != via[0].URL.Host {
            return fmt.Errorf("ticket webhook: refusing cross-host redirect to %s", req.URL.Host)
        }
        return nil
    },
},
```

(Drop the secret header before re-issue if cross-host redirects must be tolerated for some deployment.)

### WR-04: Feedback/ticket attribution near-lookup can bind the wrong audit row (concurrent same-session requests; action-agnostic pick)

**File:** `internal/server/feedback.go:262-279` (`feedbackAttribution`); `internal/store/sqlite/audit.go:185-209` (filter semantics)

**Issue:** Attribution takes `QueryAudit{SessionID, Until: turnTs, Limit: 1}` — `ts < ?`, `ORDER BY ts DESC`, **no `Action` filter, no tie-break**. Two problems:

1. `writeAuditAndTurn` writes the audit row and the assistant turn with two separate `time.Now()` calls, so the audit row's ts is only *slightly* before the turn's ts. If a second chat request on the **same session** runs concurrently, its audit row can be written into that window and becomes the newest row `< turnTs` — the vote/ticket is then attributed with the *other* request's `skill_id`/`chunk_ids` (D-29-05 violation).
2. The filter matches any action: a session-scoped `action_pending_approval` or `action_fanout_*` row (both carry `SessionID`; fanout rows carry `ChunkIDs=""`) newer than the verdict row wins the near-lookup and degrades or corrupts attribution. `QueryAudit` also has no deterministic tie-break on equal ts (`ORDER BY ts DESC` only), so `LIMIT 1` on a tie is plan-dependent.

**Fix:** Bind attribution to verdict rows only, with a deterministic order — either a dedicated store helper (`NearestVerdictAuditBySession(ctx, sessionID, before time.Time)`) with `AND action = '' AND chunk_ids != '' ORDER BY ts DESC, id DESC LIMIT 1`, or extend `AuditFilter` with an `ActionEmpty bool` + `OrderIDDesc` tie-break and use it here.

### WR-05: Public unauthenticated write endpoints can drive outbound webhooks and cross-session vote/ticket spam

**File:** `internal/server/tickets.go:142-250`; `internal/server/feedback.go:146-250`; `web/src/store/app-store.ts:566-567` (guessable local ids)

**Issue:** Both endpoints are public by documented design (same trust shape as `/api/chat`, local deployment), and I do not contest that posture for the store-only case. Three things compound badly in production, though:

1. With `SKILLFORGE_TICKET_PROVIDER=webhook`, every accepted ticket triggers up to `retries+1` outbound POSTs from an **unauthenticated** request, with no per-session/per-IP rate limit anywhere on the route — an anonymous spam relay against the operator's webhook endpoint (cost, log noise, upstream blocking).
2. The store and audit tables grow unboundedly from anonymous writes (`feedback`, `tickets`, one audit row per settled dispatch), with no retention path by design (INSERT-only).
3. `sessionIDRegex` (`^[a-zA-Z0-9-]{8,128}$`) accepts the FE's boot-local ids `local-session-1`, `local-session-2`… (`app-store.ts`), which are trivially guessable, and `session_turn.id` is a sequential integer. Anyone who guesses a live `local-session-N` session can vote and open tickets on another user's turns — feedback is the channel that feeds the review queue, so this is cross-session influence, not just noise. (Server-minted 32-hex ids are safe; the FE only uses local ids when the BE roster is empty.)

**Fix (minimum, posture-preserving):** add a per-`session_id` token-bucket rate limit on `POST /api/feedback` and `POST /api/tickets` (in-process is fine for a single-binary deployment) sized so the webhook budget is bounded (e.g. ≤ N queued tickets per session per minute); exclude `local-session-` ids from ever being accepted as an `X-Session-Id` claim server-side (reject the prefix at the header read in `chatHandler`) so FE-local sessions can never become a server-side partition.

### WR-06: `FeedbackRow` seeds its server-truth lock from the prop only at mount

**File:** `web/src/components/chat/parts/FeedbackRow.tsx:170`

**Issue:** `const [locked, setLocked] = useState<Vote | null>(votedVerdict);` reads `votedVerdict` once. The prop is derived from the `data-turn` part (`MessageBubble.tsx:300-310`), which normally exists at mount (live: the part precedes `finish`; restore: synthesized in `historyToUIMessages`). But any path where the part (or its `votedVerdict`) is attached/updated after the row mounted — a late-arriving part, a re-hydrated history snapshot merged into an existing message — leaves `locked === null`: the row renders clickable, and the submit only corrects itself via the 200 `already_recorded` branch. The "server-driven lock" invariant (D-29-04) then depends on mount timing rather than server truth.

**Fix:**

```tsx
const [localLock, setLocalLock] = useState<Vote | null>(null);
useEffect(() => { setLocked(votedVerdict); }, [votedVerdict]); // server truth wins
const locked = localLock ?? votedVerdict;                      // + drop the seed
```

(Keep the post-submit `setLocked(verdict)` writing to the local override so an optimistic lock is not clobbered by a re-render before the history echo catches up.)

## Info

### IN-01: Unconditional routing INFO logs claim a branch was taken before its condition is evaluated

**File:** `internal/server/chat.go:239` and `chat.go:254-255`

**Issue:** `chatHandler routing to runAgentPath (USE_AGENT_PATH=true)` and `chatHandler routing to Registry+Router` log unconditionally *above* the `if s.Agent != nil` / `if s.Registry != nil && s.Router != nil` checks they describe. In the production default (Agent nil, Registry nil) every request logs both lies before `routing to VerdictPipeline`. The comment block at the top of the handler defines these log lines as the hang-diagnosis contract ("log 1 only = decode/lookup"), and the lines violate it. (Pre-existing debug-log artifact, in a file this phase modified.)

**Fix:** Move each log inside its branch (or gate on the branch condition).

### IN-02: `GROUP_CONCAT` comment order is unspecified, but the wire contract claims "in report order"

**File:** `internal/store/sqlite/feedback.go:245-260`; contract claim at `internal/server/feedback.go:369-374`

**Issue:** `GROUP_CONCAT(f.comment, char(10))` without an ordering clause returns notes in whatever order the query plan visits rows; the `feedbackReviewPendingRow.Comments` doc says "in report order". The FE renders them as an unordered list so nothing breaks, but the contract line is not enforceable as written.

**Fix:** Soften the doc comment, or (SQLite 3.44+ in the bundled mattn) use `GROUP_CONCAT(f.comment, char(10) ORDER BY f.ts)`.

### IN-03: Unattributed refs surface as empty-label rows in the metrics/review UIs

**File:** `internal/store/sqlite/feedback.go:473-483` (per-KB metrics), `internal/server/feedback.go:736-746`; rendered at `web/src/components/admin/FeedbackMetricsPanel.tsx:406-409`

**Issue:** Refs rows written with `kb_slug=""` (retriever nil / `GetChunk` failure) appear as a `by_key` row with `key=""` (and can win `MAX(r.kb_slug)` in the pending view). The FE renders an empty `<code>` cell — a row with no name that still carries vote counts.

**Fix:** Filter `r.kb_slug != ''` in the by-KB aggregation and pending view, or render `（未归属）` in the FE for the empty key.

### IN-04: `ReviewQueuePanel.loadQueue` has no response-ordering guard

**File:** `web/src/components/admin/ReviewQueuePanel.tsx:151-187`

**Issue:** Tab/filter switches fire overlapping fetches with no cancellation or sequence token; the first responder clears `loading` while a slower stale request is still in flight, and a slow stale response can overwrite `pending`/`history` after the user has moved on. Displayed data can momentarily belong to the previous tab.

**Fix:** Capture a monotonically increasing `requestId` before `fetch` and drop the response if it is no longer current.

### IN-05: Vote path latency is coupled to the retriever (synchronous `GetChunk` per cited chunk)

**File:** `internal/server/feedback.go:293-324`

**Issue:** `feedbackChunkRefs` → `feedbackKBSlug` performs one retriever round-trip per cited chunk inside the `POST /api/feedback` request, before the write, with no per-call timeout beyond the request ctx. A hung Milvus hangs the vote endpoint (and one disconnecting client leaves N dangling lookups). Flagged as a robustness note only (perf itself is out of scope): the failure *handling* is correct (empty `kb_slug`, non-fatal).

**Fix:** Consider computing refs after the 201 (best-effort, as with the ticket dispatch) or bounding the whole refs computation with a short `context.WithTimeout`.

### IN-06: The disposition UI never sends `note`, though the API and audit trail support it

**File:** `web/src/components/admin/ReviewQueuePanel.tsx:233-237` vs `internal/server/feedback.go:400-407`

**Issue:** `feedbackDispositionBody.Note` is accepted and recorded verbatim, but the only client that can produce a disposition posts `{chunk_id, disposition}` — `feedback_dispositions.note` and the audit snapshot's `note` are permanently empty from the shipped UI. Either dead API surface or a missing input, depending on intent.

**Fix:** Add an optional remark field to the disposition action, or remove the note plumbing until a UI exists.

### IN-07: `feedbackMetricsHandler` queries `FeedbackMetricsBySkill` twice on the default dimension

**File:** `internal/server/feedback.go:698-715`

**Issue:** When `dim == "skill"`, `keyRows` and `totalsRows` come from two identical `FeedbackMetricsBySkill(ctx, since)` calls. Harmless (the totals aggregation deliberately reuses the once-per-vote source), just a redundant query.

**Fix:** Reuse `keyRows` for the totals aggregation when `dim == "skill"`.

### IN-08: Duplicated license-header line in `internal/store/sqlite/session.go`

**File:** `internal/store/sqlite/session.go:5-9`

**Issue:** The Apache-2.0 header contains a duplicated "You may obtain …" line ("You may obtain a copy of the License at" / "You may obtain the License at") — a copy artifact introduced in this phase's edit.

**Fix:** Restore the standard 5-line header block.

---

_Reviewed: 2026-09-07T22:27:36Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
