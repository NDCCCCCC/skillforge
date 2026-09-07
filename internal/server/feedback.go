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
