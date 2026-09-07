// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// feedback.go — Phase 29 / Plan 29-05 (FBK-01 / FBK-02 / FBK-03 / FBK-05)
// the feedback-loop HTTP surface: the public vote endpoint plus the
// admin-gated review / disposition / metrics views.
//
// TRUST MODEL — feedback is the one channel where a USER attempts to
// change what the platform trusts, so the red line is structural, not
// conventional (FBK-05):
//
//   - POST /api/feedback never touches knowledge state. It appends one
//     vote row + one refs row per cited chunk — all INSERT-only, all in
//     the feedback/tickets storage family (TestFeedbackInsertOnly). The
//     ONLY retriever call on this surface is GetChunk, a read used to
//     attribute a citation to its knowledge base slug; nothing here can
//     embed, re-index, or edit a chunk. TestFeedbackNoTrustMutation
//     source-greps this file so the red line cannot rot silently.
//   - Attribution is SERVER-side (D-29-05): the cited chunk ids + skill
//     id are copied from the turn's own audit row (the row
//     writeAuditAndTurn persisted BEFORE the assistant turn), never from
//     the request body. The body has no fields that could influence them.
//   - One vote per message is the STORE's UNIQUE(session_id, turn_id)
//     constraint (D-29-02); the FE's "first click locks" state is only a
//     cache. A replay surfaces the FIRST verdict back, never a second
//     write.
//   - The three admin endpoints run the X-Admin-Token gate as their FIRST
//     statement (CHECK ORDER — a 403 must never leak whether the store is
//     wired; same discipline as kb_admin.go / external_auth.go). A
//     disposition is the ONLY path by which feedback influences anything,
//     and even that path appends a state row + an audit row — it edits
//     nothing.
//
// Surfaces:
//
//	POST /api/feedback                  → 201 {status:"recorded",verdict}
//	                                    → 200 {status:"already_recorded",verdict}
//	                                    → 400 | 404 | 500 | 503 (store unwired)
//	GET  /api/admin/feedback/review     → 200 {pending:[…],total} | 200 {dispositions:[…]}
//	                                    → 400 | 403 | 500 | 503
//	POST /api/admin/feedback/dispositions → 201 {closed_count:n}
//	                                    → 400 | 403 | 500 | 503
//	GET  /api/admin/feedback/metrics    → 200 {dim,days,totals,by_key,trend}
//	                                    → 400 | 403 | 500 | 503

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// maxFeedbackCommentChars bounds the optional D-29-06 free-text note. The
// value is a hard constant for Phase 29 (a local deployment has no reason
// to tune it); promoting it to an env knob is a deliberate Phase 30
// decision, not an accident of omission. The body-size middleware caps the
// whole request anyway — this gate exists so an abusive comment is a
// readable 400 at the field level rather than a 413 at the transport.
const maxFeedbackCommentChars = 2000

// feedbackStoreNotWiredError is the 503 body when Deps.Store is nil: a
// vote can never be ACCEPTED without persistence (fail-closed), because
// accepting it would return "recorded" to a client whose vote evaporates.
const feedbackStoreNotWiredError = "feedback store not wired (server started without a persistence store)"

// feedbackTurnNotFoundError is the generic 404 body. A missing turn, a
// user turn, and a turn owned by another session all answer with THIS
// exact envelope — the status code must never leak WHY a turn is not
// votable (T-29-06 posture; the handler branch is deliberately merged).
const feedbackTurnNotFoundError = "session turn not found"

// feedbackVerdictNames is the closed verdict vocabulary (D-29-02) in
// display order; the DDL CHECK on feedback.verdict is the last line of
// defense behind this gate. The 400 error text lists every value so the
// FE author sees the RULE, not just the verdict (kb_admin role-text
// precedent).
var feedbackVerdictNames = []string{"helpful", "unhelpful", "outdated"}

func isFeedbackVerdict(v string) bool {
	for _, name := range feedbackVerdictNames {
		if v == name {
			return true
		}
	}
	return false
}

// feedbackBody is the wire shape of POST /api/feedback (the 29-07 FE
// FeedbackRow contract — field names frozen, do not rename). There is
// deliberately NO skill_id and NO chunk_ids field here: attribution is
// copied server-side from the audit row (D-29-05), and a client-supplied
// field would either be ignored (confusing) or trusted (dangerous).
type feedbackBody struct {
	// SessionID is the conversation partition the vote belongs to.
	SessionID string `json:"session_id"`
	// TurnID is session_turn.id, echoed back from the data-turn frame
	// (29-04). A client-invented id simply fails the existence check.
	TurnID int64 `json:"turn_id"`
	// Verdict is one of "helpful" | "unhelpful" | "outdated".
	Verdict string `json:"verdict"`
	// Comment is the optional free-text note (bounded by
	// maxFeedbackCommentChars).
	Comment string `json:"comment"`
}

// feedbackResponse is the success envelope — and nothing else. No row id,
// no timestamps, no attribution echo: the FE needs the status and the
// durable verdict, and every extra field on a public write endpoint is
// surface without a consumer.
type feedbackResponse struct {
	// Status is "recorded" (201) or "already_recorded" (200).
	Status string `json:"status"`
	// Verdict is the DURABLE verdict: the one just written, or the FIRST
	// one on an idempotent replay (D-29-02).
	Verdict string `json:"verdict"`
}

// feedbackHandler serves POST /api/feedback — the FBK-01 vote endpoint.
//
// Gate order is FIXED (mirrors kb_admin's CHECK ORDER discipline): body
// decode → session-id shape → verdict vocabulary → turn-id positivity →
// comment bound → nil-Store 503 → turn existence/role binding. The shape
// gates run BEFORE any SQL so malformed input never reaches the store
// (T-10-11 discipline shared with the history handler).
//
// Degradation posture (both are 2xx — an honest attribution gap must
// never fail a vote, Pitfall 4):
//
//   - the audit-nearby probe fails or finds no row → the vote is recorded
//     with skill_id="" and chunk_ids="" (unattributed votes are a legal
//     state the metrics views simply do not count per-KB);
//   - the retriever is nil or GetChunk errors → the refs row is still
//     written with kb_slug="" (the chunk id itself is known from the CSV).
func (s *Deps) feedbackHandler(ctx context.Context, c *app.RequestContext) {
	var body feedbackBody
	if err := c.Bind(&body); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}

	// SC-9 / T-10-11: the session-id shape gate runs BEFORE any SQL, so a
	// malformed id can never reach the store layer.
	if !sessionIDRegex.MatchString(body.SessionID) {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid session id"})
		return
	}
	// Closed verdict vocabulary: an unknown value is a typo today and a
	// metrics-poisoning value tomorrow; the 400 names all three legal
	// values (kb_admin role-text precedent).
	if !isFeedbackVerdict(body.Verdict) {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("verdict %q must be one of %v", body.Verdict, feedbackVerdictNames),
		})
		return
	}
	if body.TurnID <= 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "turn_id must be a positive integer"})
		return
	}
	// Rune count, not bytes: the cap is about CONTENT length, and the
	// product's comments are mostly Chinese (3 bytes per rune).
	if utf8.RuneCountInString(body.Comment) > maxFeedbackCommentChars {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("comment exceeds %d characters", maxFeedbackCommentChars),
		})
		return
	}

	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: feedbackStoreNotWiredError})
		return
	}

	// Turn binding (T-29-06): the vote must resolve to an ASSISTANT turn
	// of THIS session. All three failure shapes merge into the one
	// generic 404 — do not split them, the distinction is exactly the
	// information a probing caller wants and the operator does not.
	turn, ok, err := s.Store.GetSessionTurn(ctx, body.SessionID, body.TurnID)
	if err != nil {
		log.Printf("[WARN] server: feedback turn lookup (sess=%s turn=%d) failed: %v", body.SessionID, body.TurnID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback lookup failed — see server logs"})
		return
	}
	if !ok || turn.Role != "assistant" {
		c.JSON(http.StatusNotFound, graphResponseError{Error: feedbackTurnNotFoundError})
		return
	}

	// D-29-05 server-side attribution: copy the citations + skill from
	// the turn's own audit row (the row writeAuditAndTurn persisted just
	// before the assistant turn). The request body has no say in this.
	skillID, chunkIDs := s.feedbackAttribution(ctx, body.SessionID, turn.Ts)
	refs := s.feedbackChunkRefs(ctx, chunkIDs)

	inserted, err := s.Store.PutFeedback(ctx, store.FeedbackRow{
		SessionID: body.SessionID,
		TurnID:    body.TurnID,
		Verdict:   body.Verdict,
		Comment:   body.Comment,
		SkillID:   skillID,
		ChunkIDs:  chunkIDs,
	})
	if err != nil {
		log.Printf("[WARN] server: feedback write (sess=%s turn=%d) failed: %v", body.SessionID, body.TurnID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback write failed — see server logs"})
		return
	}

	// The store assigns the row id; the read-back is the only way to name
	// it (and on the replay path it IS the first row — exactly what the
	// idempotent response must surface).
	saved, ok, err := s.Store.FeedbackByTurn(ctx, body.SessionID, body.TurnID)
	if err != nil || !ok {
		log.Printf("[WARN] server: feedback read-back (sess=%s turn=%d) failed (ok=%v, err=%v)", body.SessionID, body.TurnID, ok, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback write failed — see server logs"})
		return
	}

	if !inserted {
		// D-29-02: the UNIQUE key swallowed the replay. Surface the FIRST
		// verdict and write NOTHING — no second vote, no duplicate refs.
		c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", Verdict: saved.Verdict})
		return
	}

	// Normalized per-chunk refs (the FBK-02/03 aggregation substrate).
	// A failure here leaves the vote row persisted but unattributed per
	// chunk; that is an internal persistence fault and is surfaced as 500
	// (a silent 201 would lose per-KB metrics without a trace, and a
	// client retry safely lands on the already_recorded path).
	if err := s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs); err != nil {
		log.Printf("[WARN] server: feedback refs write (feedback_id=%d, %d refs) failed: %v (vote row persisted; per-KB attribution for this vote is lost)", saved.ID, len(refs), err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback write failed — see server logs"})
		return
	}

	c.JSON(http.StatusCreated, feedbackResponse{Status: "recorded", Verdict: saved.Verdict})
}

// feedbackAttribution returns the (skill_id, chunk_ids CSV) the platform
// attributes to a turn, read from the most-recent audit row STRICTLY
// BEFORE the turn's ts — the row writeAuditAndTurn wrote in the same
// request that produced the turn. Until (not Since) + SessionID + Limit
// ride the parameterized AuditFilter: no SQL is assembled here, ever
// (T-29-04).
//
// ANY failure (probe error, no row) degrades to ("", "") and logs — the
// vote still lands, unattributed (Pitfall 4: an honest gap must not fail
// the user's gesture).
func (s *Deps) feedbackAttribution(ctx context.Context, sessionID string, turnTs time.Time) (string, string) {
	if s.Store == nil {
		return "", ""
	}
	rows, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID: sessionID,
		Until:     turnTs,
		Limit:     1,
	})
	if err != nil {
		log.Printf("[WARN] server: feedback attribution probe (sess=%s) failed: %v (recording the vote without attribution; non-fatal)", sessionID, err)
		return "", ""
	}
	if len(rows) == 0 {
		return "", ""
	}
	return rows[0].SkillID, rows[0].ChunkIDs
}

// feedbackChunkRefs normalizes an audit.chunk_ids CSV into the per-chunk
// refs rows the FBK-02/03 aggregation needs: unparseable segments are
// skipped, duplicates collapse (one ref per distinct chunk), and each
// surviving id gets its knowledge-base slug resolved through GetChunk —
// a READ. Post-Phase 26, a chunk's Tier IS the knowledge_bases registry
// slug, so Tier is what lands in feedback_chunk_refs.kb_slug.
//
// A nil retriever or a failed lookup degrades that ref's kb_slug to ""
// (the chunk id itself is known from the server-side CSV; only its
// knowledge-base attribution is missing). This is the single retriever
// call the feedback surface is allowed to make — FBK-05's red line lives
// here, guarded by TestFeedbackNoTrustMutation.
func (s *Deps) feedbackChunkRefs(ctx context.Context, chunkIDsCSV string) []store.ChunkRef {
	if chunkIDsCSV == "" {
		return nil
	}
	fields := strings.Split(chunkIDsCSV, ",")
	refs := make([]store.ChunkRef, 0, len(fields))
	seen := make(map[int64]bool, len(fields))
	for _, field := range fields {
		id, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64)
		if err != nil || id <= 0 || seen[id] {
			continue // 不可解析的段跳过 — never guess a chunk id
		}
		seen[id] = true
		refs = append(refs, store.ChunkRef{ChunkID: id, KBSlug: s.feedbackKBSlug(ctx, id)})
	}
	return refs
}

// feedbackKBSlug resolves one chunk id to its knowledge-base slug (the
// retriever chunk Tier). Unwired retriever / lookup failure → "" (an
// honest attribution gap, logged at WARN).
func (s *Deps) feedbackKBSlug(ctx context.Context, chunkID int64) string {
	if s.Retriever == nil {
		return ""
	}
	content, err := s.Retriever.GetChunk(ctx, chunkID)
	if err != nil {
		log.Printf("[WARN] server: feedback kb attribution (chunk=%d) failed: %v (refs row keeps an empty kb_slug; non-fatal)", chunkID, err)
		return ""
	}
	return content.Tier
}

// ---------------------------------------------------------------------------
// Admin surface — review queue / disposition verb / metrics view.
// Every handler below opens with the SAME gate block, in the SAME order:
// the admin-token check is the FIRST statement (a 403 must never leak
// whether the store is wired), and only then the nil-Store 503. The order
// is locked by TestFeedbackDispositionAdminGate's source grep.
// ---------------------------------------------------------------------------

// feedbackDispositionNames is the closed disposition vocabulary (D-29-09)
// in display order; the DDL CHECK backstops it. All three are FINAL
// states for a queue entry — the pending view is derived, so "closing" a
// report is an appended state row, never an edit of the report.
var feedbackDispositionNames = []string{"updated", "false_positive", "ignored"}

func isFeedbackDisposition(v string) bool {
	for _, name := range feedbackDispositionNames {
		if v == name {
			return true
		}
	}
	return false
}

// actionFeedbackDisposition is the audit action for one review decision.
// It is the repudiation trail of the ONLY path on which feedback
// influences anything (FBK-02): who dispositioned which chunk, when, and
// how many pending entries that decision resolved.
const actionFeedbackDisposition = "action_feedback_disposition"

// feedbackReviewPendingRow is one derived queue row as rendered for the
// admin view (the 29-08 FE contract — field names frozen). Chunk content
// is deliberately NOT part of this payload: the FE fetches the verbatim
// text through the existing GET /api/knowledge/chunk/:id when it expands
// a row, so the queue view stays cheap and adds no new read surface.
type feedbackReviewPendingRow struct {
	// ChunkID is the reported chunk.
	ChunkID int64 `json:"chunk_id"`
	// KBSlug is the aggregate-display knowledge base (store-side MAX over
	// the qualifying votes' refs).
	KBSlug string `json:"kb_slug"`
	// ReportCount is the unresolved report batch — the priority signal.
	ReportCount int `json:"report_count"`
	// LastReportedAt is the newest qualifying vote (RFC3339 UTC).
	LastReportedAt string `json:"last_reported_at"`
	// Comments are the qualifying votes' notes in report order (a
	// multi-line note splits into several entries — the 29-02 store
	// trade-off; the FE renders them as a list).
	Comments []string `json:"comments"`
}

// feedbackDispositionEntry is one disposition-history row as rendered for
// the admin view.
type feedbackDispositionEntry struct {
	ID          int64  `json:"id"`
	TS          string `json:"ts"`
	ChunkID     int64  `json:"chunk_id"`
	Disposition string `json:"disposition"`
	Note        string `json:"note"`
	Actor       string `json:"actor"`
}

// feedbackReviewResponse is the GET /api/admin/feedback/review envelope.
// The two branches carry DISJOINT halves: ?status=pending answers
// {pending,total} and ?status=resolved answers {dispositions} — the
// absent half serializes away (omitempty) instead of arriving as a
// misleading empty array.
type feedbackReviewResponse struct {
	Pending      []feedbackReviewPendingRow `json:"pending,omitempty"`
	Total        int                        `json:"total"`
	Dispositions []feedbackDispositionEntry `json:"dispositions,omitempty"`
}

// feedbackDispositionBody is the wire shape of POST
// /api/admin/feedback/dispositions.
type feedbackDispositionBody struct {
	// ChunkID is the chunk being dispositioned.
	ChunkID int64 `json:"chunk_id"`
	// Disposition is one of "updated" | "false_positive" | "ignored".
	Disposition string `json:"disposition"`
	// Note is the optional operator remark (recorded verbatim).
	Note string `json:"note"`
}

// feedbackDispositionResponse answers the disposition POST: how many
// pending entries this one decision resolved (counted BEFORE the write).
type feedbackDispositionResponse struct {
	ClosedCount int `json:"closed_count"`
}

// feedbackDispositionAudit is the FrozenArgs snapshot of one disposition —
// the durable "who decided what" record inside the admin audit trail.
type feedbackDispositionAudit struct {
	ChunkID     int64  `json:"chunk_id"`
	Disposition string `json:"disposition"`
	Note        string `json:"note"`
	ClosedCount int    `json:"closed_count"`
}

// feedbackReviewHandler serves GET /api/admin/feedback/review.
//
// ?status=pending (default) → the derived review queue, highest priority
// first, plus the PendingReviewCount badge total. ?status=resolved → the
// disposition history, newest first, optionally narrowed by
// ?disposition=<closed value>. Unknown status → 400; an unknown
// disposition filter → 400 as well (the store fails closed on the same
// value — an empty-looking history would be read as "no decisions ever",
// which would be a lie).
func (s *Deps) feedbackReviewHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// feedback store is wired.
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: feedbackStoreNotWiredError})
		return
	}

	status := c.Query("status")
	if status == "" {
		status = "pending"
	}
	disposition := c.Query("disposition")
	if disposition != "" && !isFeedbackDisposition(disposition) {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("disposition %q must be one of %v", disposition, feedbackDispositionNames),
		})
		return
	}

	switch status {
	case "pending":
		// The disposition filter is a resolved-branch dimension; honoring
		// it here would silently mean "ignored", which is worse than a 400.
		if disposition != "" {
			c.JSON(http.StatusBadRequest, graphResponseError{
				Error: "disposition filter applies to status=resolved only",
			})
			return
		}
		rows, err := s.Store.PendingReviewChunks(ctx)
		if err != nil {
			log.Printf("[WARN] server: feedback pending view read failed: %v", err)
			c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback review read failed — see server logs"})
			return
		}
		total, err := s.Store.PendingReviewCount(ctx)
		if err != nil {
			log.Printf("[WARN] server: feedback pending count read failed: %v", err)
			c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback review read failed — see server logs"})
			return
		}
		pending := make([]feedbackReviewPendingRow, 0, len(rows))
		for _, r := range rows {
			comments := r.Comments
			if comments == nil {
				comments = []string{}
			}
			pending = append(pending, feedbackReviewPendingRow{
				ChunkID:        r.ChunkID,
				KBSlug:         r.KBSlug,
				ReportCount:    r.ReportCount,
				LastReportedAt: r.LastReportedAt.UTC().Format(time.RFC3339),
				Comments:       comments,
			})
		}
		c.JSON(http.StatusOK, feedbackReviewResponse{Pending: pending, Total: total})

	case "resolved":
		rows, err := s.Store.ResolvedDispositions(ctx, disposition)
		if err != nil {
			log.Printf("[WARN] server: feedback disposition history read failed: %v", err)
			c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback review read failed — see server logs"})
			return
		}
		entries := make([]feedbackDispositionEntry, 0, len(rows))
		for _, r := range rows {
			entries = append(entries, feedbackDispositionEntry{
				ID:          r.ID,
				TS:          r.TS.UTC().Format(time.RFC3339),
				ChunkID:     r.ChunkID,
				Disposition: r.Disposition,
				Note:        r.Note,
				Actor:       r.Actor,
			})
		}
		c.JSON(http.StatusOK, feedbackReviewResponse{Dispositions: entries})

	default:
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("status %q must be one of [pending resolved]", status),
		})
	}
}

// feedbackDispositionHandler serves POST /api/admin/feedback/dispositions
// — the FBK-02 review verb, and the ONLY path by which user feedback
// influences the platform.
//
// Semantics (D-29-08/D-29-09): the disposition is an INSERT-only state
// row plus one audit row — nothing is edited anywhere. closed_count is
// the number of pending entries the decision resolves, counted BEFORE the
// write (the pending view is derived from "disposition ts >= vote ts", so
// counting after the write would always answer 0). A newer outdated
// report after this decision reopens the chunk (latest-disposition-wins).
func (s *Deps) feedbackDispositionHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// feedback store is wired.
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: feedbackStoreNotWiredError})
		return
	}

	var body feedbackDispositionBody
	if err := c.Bind(&body); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}
	if body.ChunkID <= 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "chunk_id must be a positive integer"})
		return
	}
	if !isFeedbackDisposition(body.Disposition) {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("disposition %q must be one of %v", body.Disposition, feedbackDispositionNames),
		})
		return
	}

	// closed_count FIRST (先查后写): the derived view still shows this
	// chunk's unresolved reports at this instant; after the state row
	// lands they are resolved by definition.
	pending, err := s.Store.PendingReviewChunks(ctx)
	if err != nil {
		log.Printf("[WARN] server: feedback pending read before disposition failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback disposition failed — see server logs"})
		return
	}
	closedCount := 0
	for _, row := range pending {
		if row.ChunkID == body.ChunkID {
			closedCount = row.ReportCount
			break
		}
	}

	if _, err := s.Store.PutDisposition(ctx, store.DispositionRow{
		ChunkID:     body.ChunkID,
		Disposition: body.Disposition,
		Note:        body.Note,
		Actor:       "admin",
	}); err != nil {
		log.Printf("[WARN] server: feedback disposition write (chunk=%d) failed: %v", body.ChunkID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback disposition failed — see server logs"})
		return
	}

	// The repudiation trail — best-effort, mirroring every audit write in
	// this package: the state row is already persisted and a failed audit
	// row must not roll a decision back (it is logged loudly instead).
	s.putFeedbackAuditRow(ctx, actionFeedbackDisposition, feedbackDispositionAudit{
		ChunkID:     body.ChunkID,
		Disposition: body.Disposition,
		Note:        body.Note,
		ClosedCount: closedCount,
	})

	c.JSON(http.StatusCreated, feedbackDispositionResponse{ClosedCount: closedCount})
}

// feedbackMetricsTotals is the whole-window aggregate (every Skill, every
// KB). Rates are computed HERE (D-29-10) with the FULL-VOTE denominator —
// an outdated vote is a negative signal and stays IN the denominator, or
// every per-Skill / per-KB comparison flatters the worst Skills. A nil
// rate serializes as JSON null: "no votes yet" must never render as a
// 0% that reads like "everyone voted unhelpful".
type feedbackMetricsTotals struct {
	Helpful      int      `json:"helpful"`
	Unhelpful    int      `json:"unhelpful"`
	Outdated     int      `json:"outdated"`
	HelpRate     *float64 `json:"help_rate"`
	OutdatedRate *float64 `json:"outdated_rate"`
	TotalVotes   int      `json:"total_votes"`
	// PendingCount is the current review-queue depth — the 29-08
	// cross-link between the metrics view and the review queue.
	PendingCount int `json:"pending_count"`
}

// feedbackMetricsKeyRow is one by_key cell of the selected dimension.
// Deliberately NOT embedding feedbackMetricsTotals: a per-key row has no
// pending_count, and an accidental extra field on the wire is contract
// drift the FE never asked for.
type feedbackMetricsKeyRow struct {
	Key          string   `json:"key"`
	Helpful      int      `json:"helpful"`
	Unhelpful    int      `json:"unhelpful"`
	Outdated     int      `json:"outdated"`
	HelpRate     *float64 `json:"help_rate"`
	OutdatedRate *float64 `json:"outdated_rate"`
}

// feedbackTrendCell is one (day, verdict) bucket of the daily trend; the
// weekly view is a FE-side merge of these cells (D-29-11).
type feedbackTrendCell struct {
	Day     string `json:"day"`
	Verdict string `json:"verdict"`
	Count   int    `json:"count"`
}

// feedbackMetricsResponse is the GET /api/admin/feedback/metrics envelope.
type feedbackMetricsResponse struct {
	// Dim is the served dimension: "skill" or "base".
	Dim string `json:"dim"`
	// Days is the resolved window length (default 7).
	Days   int                     `json:"days"`
	Totals feedbackMetricsTotals   `json:"totals"`
	ByKey  []feedbackMetricsKeyRow `json:"by_key"`
	Trend  []feedbackTrendCell     `json:"trend"`
}

// feedbackMetricsHandler serves GET /api/admin/feedback/metrics.
//
// Caliber (D-29-10 + 29-02 Open Q2): by_key serves the SELECTED
// dimension — ?dim=skill counts every vote once; ?dim=base attributes
// through the refs table and counts a multi-KB vote ONCE PER KB, so the
// by_key rows may sum above the vote count by design. totals is ALWAYS
// the strict once-per-vote per-Skill aggregation (the store's documented
// "any total counts every vote strictly once"), because a total that
// double-counts is simply a wrong number.
func (s *Deps) feedbackMetricsHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// feedback store is wired.
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: feedbackStoreNotWiredError})
		return
	}

	dim := c.Query("dim")
	if dim == "" {
		dim = "skill"
	}
	if dim != "skill" && dim != "base" {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: fmt.Sprintf("dim %q must be one of [skill base]", dim),
		})
		return
	}
	days := 7
	if raw := c.Query("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			c.JSON(http.StatusBadRequest, graphResponseError{Error: "days must be a positive integer"})
			return
		}
		days = n
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	// The selected dimension drives by_key only.
	var (
		keyRows []store.FeedbackCountRow
		err     error
	)
	if dim == "base" {
		keyRows, err = s.Store.FeedbackMetricsByKB(ctx, since)
	} else {
		keyRows, err = s.Store.FeedbackMetricsBySkill(ctx, since)
	}
	if err != nil {
		log.Printf("[WARN] server: feedback metrics (dim=%s) read failed: %v", dim, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback metrics read failed — see server logs"})
		return
	}

	// totals: the strict once-per-vote source, regardless of dim.
	totalsRows, err := s.Store.FeedbackMetricsBySkill(ctx, since)
	if err != nil {
		log.Printf("[WARN] server: feedback metrics totals read failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback metrics read failed — see server logs"})
		return
	}
	trend, err := s.Store.FeedbackDailyTrend(ctx, since)
	if err != nil {
		log.Printf("[WARN] server: feedback trend read failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback metrics read failed — see server logs"})
		return
	}
	pendingCount, err := s.Store.PendingReviewCount(ctx)
	if err != nil {
		log.Printf("[WARN] server: feedback pending count read failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "feedback metrics read failed — see server logs"})
		return
	}

	var t store.FeedbackCountRow
	for _, row := range totalsRows {
		t.Helpful += row.Helpful
		t.Unhelpful += row.Unhelpful
		t.Outdated += row.Outdated
	}

	byKey := make([]feedbackMetricsKeyRow, 0, len(keyRows))
	for _, row := range keyRows {
		byKey = append(byKey, feedbackMetricsKeyRow{
			Key:          row.Key,
			Helpful:      row.Helpful,
			Unhelpful:    row.Unhelpful,
			Outdated:     row.Outdated,
			HelpRate:     feedbackRate(row.Helpful, row.Unhelpful, row.Outdated),
			OutdatedRate: feedbackOutdatedRate(row.Helpful, row.Unhelpful, row.Outdated),
		})
	}

	trendCells := make([]feedbackTrendCell, 0, len(trend))
	for _, cell := range trend {
		trendCells = append(trendCells, feedbackTrendCell{Day: cell.Day, Verdict: cell.Verdict, Count: cell.Count})
	}

	c.JSON(http.StatusOK, feedbackMetricsResponse{
		Dim:  dim,
		Days: days,
		Totals: feedbackMetricsTotals{
			Helpful:      t.Helpful,
			Unhelpful:    t.Unhelpful,
			Outdated:     t.Outdated,
			HelpRate:     feedbackRate(t.Helpful, t.Unhelpful, t.Outdated),
			OutdatedRate: feedbackOutdatedRate(t.Helpful, t.Unhelpful, t.Outdated),
			TotalVotes:   t.Helpful + t.Unhelpful + t.Outdated,
			PendingCount: pendingCount,
		},
		ByKey: byKey,
		Trend: trendCells,
	})
}

// feedbackRate = helpful / (helpful + unhelpful + outdated) — the D-29-10
// full-vote denominator, computed Go-side (the store returns raw counts
// only). Zero votes → nil, which serializes as JSON null.
func feedbackRate(helpful, unhelpful, outdated int) *float64 {
	return feedbackRatio(helpful, helpful+unhelpful+outdated)
}

// feedbackOutdatedRate = outdated / (helpful + unhelpful + outdated).
func feedbackOutdatedRate(helpful, unhelpful, outdated int) *float64 {
	return feedbackRatio(outdated, helpful+unhelpful+outdated)
}

func feedbackRatio(num, denom int) *float64 {
	if denom == 0 {
		return nil
	}
	v := float64(num) / float64(denom)
	return &v
}

// putFeedbackAuditRow writes the INSERT-only audit row for one feedback
// disposition with the decision snapshot in FrozenArgs and ActorType
// "admin" so GET /api/admin/audit (ADM-06) surfaces it.
//
// Best-effort, mirroring putKBAdminAuditRow: a failed write is logged at
// WARN and NEVER rolls the disposition back — the state row is already
// persisted, and turning a recorded decision into a 500 because its
// audit row failed would leave the queue view and the operator's intent
// out of sync.
func (s *Deps) putFeedbackAuditRow(ctx context.Context, action string, payload any) {
	if s.Store == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WARN] server: feedback audit row marshal (%s): %v (row skipped)", action, err)
		return
	}
	if _, err := s.Store.PutAudit(ctx, store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "admin",
		Action:     action,
		ActorType:  "admin",
		FrozenArgs: b,
	}); err != nil {
		log.Printf("[WARN] server: feedback audit row write (%s): %v (best-effort, disposition not rolled back)", action, err)
	}
}
