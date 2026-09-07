// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 29 / Plan 29-05 — tests for the feedback HTTP surface (FBK-01 +
// FBK-02 + FBK-03 + FBK-05).
//
// Three layers of guarantee live in this file:
//
//  1. Behavior matrix (TestFeedbackHandler) — the POST /api/feedback
//     contract hermetically, against a fake store + stub retriever that
//     panic on any method the handler must NOT call (nil-embed pattern,
//     commit 50f435f).
//  2. Admin surface (TestFeedbackReviewHandler / TestFeedbackDispositionAdminGate
//     / TestFeedbackMetricsHandler) — the CHECK ORDER gate (403 BEFORE
//     503, Pitfall 8), the disposition lifecycle against a REAL in-memory
//     sqlite store (so the reopen semantics are the store's own, not a
//     test-local copy), and the D-29-10 full-denominator metric caliber.
//  3. The FBK-05 structural red line (TestFeedbackNoTrustMutation) —
//     feedback.go (server) carries no mutation verb, touches the
//     retriever ONLY through the GetChunk read, and no feedback/ticket
//     storage file references the knowledge ingestion surface. Feedback
//     can never mutate the knowledge base except through a human
//     disposition — this test makes that a source-level fact.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
)

// ---------------------------------------------------------------------------
// fakes — the nil-embed pattern: every store/retriever method the handler
// under test must NOT call hits the nil embedded interface and panics, so
// an accidental extra dependency fails loudly instead of silently passing.
// ---------------------------------------------------------------------------

// feedbackFakeStore is a feedback-aware fake overriding ONLY the methods
// the feedback handlers call (established 50f435f pattern). Everything
// else panics through the embedded nil store.Store.
type feedbackFakeStore struct {
	store.Store

	mu sync.Mutex

	// turn space: sessionID -> turnID -> row (GetSessionTurn).
	turns map[string]map[int64]store.SessionMessage
	// getSessionTurnCalls counts every GetSessionTurn — the validation
	// matrix asserts it stays ZERO for input rejected before any SQL.
	getSessionTurnCalls int

	// audit rows + the filters the handler issued (attribution probe).
	auditRows        []store.AuditRow
	queryAuditCalls  int
	queryAuditFilter []store.AuditFilter
	failQueryAudit   bool

	// votes + refs (the INSERT-only write surface).
	feedbackRows     []store.FeedbackRow
	putFeedbackCalls int
	refs             map[int64][]store.ChunkRef
	refRowsWritten   int

	// admin read models (canned) + disposition capture.
	putAuditRows  []store.AuditRow
	dispositions  []store.DispositionRow
	pending       []store.PendingReviewRow
	pendingCount  int
	bySkillRows   []store.FeedbackCountRow
	byKBRows      []store.FeedbackCountRow
	trendRows     []store.FeedbackTrendRow
	lastSince     time.Time
	metricsCalled int
}

func newFeedbackFakeStore() *feedbackFakeStore {
	return &feedbackFakeStore{
		turns: make(map[string]map[int64]store.SessionMessage),
		refs:  make(map[int64][]store.ChunkRef),
	}
}

var _ store.Store = (*feedbackFakeStore)(nil)

func (f *feedbackFakeStore) GetSessionTurn(_ context.Context, sessionID string, id int64) (store.SessionMessage, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getSessionTurnCalls++
	m, ok := f.turns[sessionID][id]
	return m, ok, nil
}

func (f *feedbackFakeStore) QueryAudit(_ context.Context, filter store.AuditFilter) ([]store.AuditRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryAuditCalls++
	f.queryAuditFilter = append(f.queryAuditFilter, filter)
	if f.failQueryAudit {
		return nil, errors.New("simulated audit outage")
	}
	var out []store.AuditRow
	for _, r := range f.auditRows {
		if filter.SessionID != "" && r.SessionID != filter.SessionID {
			continue
		}
		if !filter.Until.IsZero() && !r.Timestamp.Before(filter.Until) {
			continue // mirrors the store's `ts < ?` (Until) semantics
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *feedbackFakeStore) PutFeedback(_ context.Context, row store.FeedbackRow) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putFeedbackCalls++
	for _, existing := range f.feedbackRows {
		if existing.SessionID == row.SessionID && existing.TurnID == row.TurnID {
			return false, nil // UNIQUE(session_id, turn_id) replay
		}
	}
	saved := row
	saved.ID = int64(len(f.feedbackRows) + 1) // AUTOINCREMENT stand-in
	f.feedbackRows = append(f.feedbackRows, saved)
	return true, nil
}

func (f *feedbackFakeStore) FeedbackByTurn(_ context.Context, sessionID string, turnID int64) (store.FeedbackRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.feedbackRows {
		if r.SessionID == sessionID && r.TurnID == turnID {
			return r, true, nil
		}
	}
	return store.FeedbackRow{}, false, nil
}

func (f *feedbackFakeStore) PutFeedbackChunkRefs(_ context.Context, feedbackID int64, refs []store.ChunkRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refRowsWritten += len(refs)
	f.refs[feedbackID] = append(f.refs[feedbackID], refs...)
	return nil
}

func (f *feedbackFakeStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putAuditRows = append(f.putAuditRows, row)
	return int64(len(f.putAuditRows)), nil
}

func (f *feedbackFakeStore) PutDisposition(_ context.Context, row store.DispositionRow) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	saved := row
	saved.ID = int64(len(f.dispositions) + 1)
	f.dispositions = append(f.dispositions, saved)
	return saved.ID, nil
}

func (f *feedbackFakeStore) PendingReviewChunks(_ context.Context) ([]store.PendingReviewRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.PendingReviewRow, len(f.pending))
	copy(out, f.pending)
	return out, nil
}

func (f *feedbackFakeStore) PendingReviewCount(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pendingCount, nil
}

func (f *feedbackFakeStore) ResolvedDispositions(_ context.Context, disposition string) ([]store.DispositionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if disposition != "" {
		switch disposition {
		case "updated", "false_positive", "ignored":
		default:
			return nil, fmt.Errorf("unknown disposition %q", disposition)
		}
	}
	out := make([]store.DispositionRow, 0, len(f.dispositions))
	for _, d := range f.dispositions {
		if disposition == "" || d.Disposition == disposition {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID }) // newest-first
	return out, nil
}

func (f *feedbackFakeStore) FeedbackMetricsBySkill(_ context.Context, since time.Time) ([]store.FeedbackCountRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsCalled++
	f.lastSince = since
	return append([]store.FeedbackCountRow(nil), f.bySkillRows...), nil
}

func (f *feedbackFakeStore) FeedbackMetricsByKB(_ context.Context, since time.Time) ([]store.FeedbackCountRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsCalled++
	f.lastSince = since
	return append([]store.FeedbackCountRow(nil), f.byKBRows...), nil
}

func (f *feedbackFakeStore) FeedbackDailyTrend(_ context.Context, since time.Time) ([]store.FeedbackTrendRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.FeedbackTrendRow(nil), f.trendRows...), nil
}

// auditRowsSnapshot / refsSnapshot / putAuditSnapshot are the test-facing
// read-backs (lock-captured copies).
func (f *feedbackFakeStore) putAuditSnapshot() []store.AuditRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.AuditRow, len(f.putAuditRows))
	copy(out, f.putAuditRows)
	return out
}

func (f *feedbackFakeStore) refsSnapshot() []store.ChunkRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ChunkRef
	for _, rs := range f.refs {
		out = append(out, rs...)
	}
	return out
}

func (f *feedbackFakeStore) feedbackSnapshot() []store.FeedbackRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.FeedbackRow, len(f.feedbackRows))
	copy(out, f.feedbackRows)
	return out
}

// fbSeedTurn installs one assistant turn + its preceding audit row (the
// writeAuditAndTurn shape: the audit row lands BEFORE the assistant turn,
// carrying the citations CSV + skill id).
func (f *feedbackFakeStore) fbSeedTurn(sessionID string, turnID int64, role string, turnTs time.Time, skillID, chunkIDs string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.turns[sessionID] == nil {
		f.turns[sessionID] = make(map[int64]store.SessionMessage)
	}
	f.turns[sessionID][turnID] = store.SessionMessage{ID: turnID, Role: role, Content: "conclusion", Ts: turnTs}
	if skillID != "" || chunkIDs != "" {
		f.auditRows = append(f.auditRows, store.AuditRow{
			Timestamp: turnTs.Add(-2 * time.Second),
			SessionID: sessionID,
			SkillID:   skillID,
			ChunkIDs:  chunkIDs,
		})
	}
}

// fbStubRetriever is the retriever double: only GetChunk is reachable
// (nil-embed panics on anything else — the FBK-05 read-only posture, made
// hostile on purpose).
type fbStubRetriever struct {
	retriever.Retriever
	chunks map[int64]retriever.ChunkContent
	err    error
	calls  []int64
}

func (s *fbStubRetriever) GetChunk(_ context.Context, id int64) (retriever.ChunkContent, error) {
	s.calls = append(s.calls, id)
	if s.err != nil {
		return retriever.ChunkContent{}, s.err
	}
	ch, ok := s.chunks[id]
	if !ok {
		return retriever.ChunkContent{}, fmt.Errorf("chunk %d not found", id)
	}
	return ch, nil
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

// fbTestSession matches sessionIDRegex ([a-zA-Z0-9-]{8,128}).
const fbTestSession = "fbtest-session-01"

// fbTurnTs is the assistant turn instant; the audit row precedes it.
var fbTurnTs = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

func newFeedbackHertz(deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(":0"))
	h.POST("/api/feedback", deps.feedbackHandler)
	return h
}

// fbRequest performs one request. jsonBody "" sends no body; token ""
// sends no admin header.
func fbRequest(h *server.Hertz, method, path, jsonBody, token string) *ut.ResponseRecorder {
	headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	var body *ut.Body
	if jsonBody != "" {
		body = &ut.Body{Body: strings.NewReader(jsonBody), Len: len(jsonBody)}
	}
	return ut.PerformRequest(h.Engine, method, path, body, headers...)
}

// fbDecode decodes a response body into the anonymous wire shape the FE
// consumes (asserting the JSON contract, not internal Go type names).
func fbDecode(t *testing.T, resp *ut.ResponseRecorder, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), out), "body: %s", resp.Body.String())
}

// ---------------------------------------------------------------------------
// Task 1 — POST /api/feedback behavior matrix
// ---------------------------------------------------------------------------

// TestFeedbackHandler is the FBK-01 write-endpoint behavior matrix: eight
// sub-cases covering the recorded path, idempotent replay, the validation
// gates, the uniform 404, both honest degradations, and the nil-Store 503.
func TestFeedbackHandler(t *testing.T) {
	const validBody = `{"session_id":"fbtest-session-01","turn_id":42,"verdict":"outdated","comment":"第 3 条已废止"}`

	t.Run("records vote with server-side attribution", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11,12")
		retr := &fbStubRetriever{chunks: map[int64]retriever.ChunkContent{
			11: {ID: 11, Tier: "authority"},
			12: {ID: 12, Tier: "historical"},
		}}
		deps := &Deps{Store: fake, Retriever: retr}
		h := newFeedbackHertz(deps)

		resp := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Status  string `json:"status"`
			Verdict string `json:"verdict"`
		}
		fbDecode(t, resp, &body)
		assert.Equal(t, "recorded", body.Status)
		assert.Equal(t, "outdated", body.Verdict)

		// The vote row carries the SERVER-side copy (D-29-05): skill +
		// chunk CSV replicated from the audit row, never from the body.
		rows := fake.feedbackSnapshot()
		require.Len(t, rows, 1)
		assert.Equal(t, fbTestSession, rows[0].SessionID)
		assert.EqualValues(t, 42, rows[0].TurnID)
		assert.Equal(t, "outdated", rows[0].Verdict)
		assert.Equal(t, "第 3 条已废止", rows[0].Comment)
		assert.Equal(t, "compliance", rows[0].SkillID, "skill_id is the audit-row copy, not a client claim")
		assert.Equal(t, "11,12", rows[0].ChunkIDs)

		// Normalized refs: one per DISTINCT cited chunk, kb_slug from the
		// retriever Tier (Phase 26 registry slug semantics).
		refs := fake.refsSnapshot()
		require.Len(t, refs, 2, "chunk count must match the audit CSV")
		byChunk := make(map[int64]string, len(refs))
		for _, ref := range refs {
			byChunk[ref.ChunkID] = ref.KBSlug
		}
		assert.Equal(t, "authority", byChunk[11])
		assert.Equal(t, "historical", byChunk[12])
		assert.ElementsMatch(t, []int64{11, 12}, retr.calls, "GetChunk resolves each cited chunk exactly once")

		// The attribution probe is the parameterized audit-nearby query:
		// bounded to this session, strictly before the turn ts, LIMIT 1.
		require.Len(t, fake.queryAuditFilter, 1)
		assert.Equal(t, fbTestSession, fake.queryAuditFilter[0].SessionID)
		assert.True(t, fake.queryAuditFilter[0].Until.Equal(fbTurnTs), "Until must bind the turn ts")
		assert.Equal(t, 1, fake.queryAuditFilter[0].Limit)
	})

	t.Run("replay returns already_recorded with the first verdict", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11,12")
		retr := &fbStubRetriever{chunks: map[int64]retriever.ChunkContent{
			11: {ID: 11, Tier: "authority"},
			12: {ID: 12, Tier: "historical"},
		}}
		deps := &Deps{Store: fake, Retriever: retr}
		h := newFeedbackHertz(deps)

		first := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, first.Code)
		refsAfterFirst := len(fake.refsSnapshot())

		// Same (session, turn), DIFFERENT verdict + comment: the UNIQUE key
		// swallows the insert and the FIRST verdict is what comes back.
		replay := fbRequest(h, http.MethodPost, "/api/feedback",
			`{"session_id":"fbtest-session-01","turn_id":42,"verdict":"helpful","comment":"changed my mind"}`, "")
		require.Equal(t, http.StatusOK, replay.Code, "a replay is not a creation — 200, never a second 201")

		var body struct {
			Status  string `json:"status"`
			Verdict string `json:"verdict"`
		}
		fbDecode(t, replay, &body)
		assert.Equal(t, "already_recorded", body.Status)
		assert.Equal(t, "outdated", body.Verdict, "the FIRST verdict is the durable truth (D-29-02)")

		assert.Len(t, fake.feedbackSnapshot(), 1, "exactly one vote row per (session, turn)")
		assert.Equal(t, 2, fake.putFeedbackCalls, "the replay still reached the store (DB constraint is the truth)")
		assert.Len(t, fake.refsSnapshot(), refsAfterFirst, "a replay must not append a second refs batch")
	})

	t.Run("invalid verdict and oversized comment are rejected with zero writes", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11")
		deps := &Deps{Store: fake}
		h := newFeedbackHertz(deps)

		resp := fbRequest(h, http.MethodPost, "/api/feedback",
			`{"session_id":"fbtest-session-01","turn_id":42,"verdict":"amazing"}`, "")
		require.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, resp.Body.String(), "must be one of", "the 400 must name the closed verdict vocabulary")
		for _, v := range []string{"helpful", "unhelpful", "outdated"} {
			assert.Contains(t, resp.Body.String(), v, "the verdict list must include %q", v)
		}

		tooLong := strings.Repeat("x", 2001)
		payload, err := json.Marshal(map[string]any{
			"session_id": fbTestSession, "turn_id": 42, "verdict": "helpful", "comment": tooLong,
		})
		require.NoError(t, err)
		resp = fbRequest(h, http.MethodPost, "/api/feedback", string(payload), "")
		require.Equal(t, http.StatusBadRequest, resp.Code, "comment over the 2000-char cap must 400")

		unparseable := fbRequest(h, http.MethodPost, "/api/feedback", `{not json`, "")
		assert.Equal(t, http.StatusBadRequest, unparseable.Code)

		assert.Empty(t, fake.feedbackSnapshot(), "a rejected vote writes nothing")
		assert.Equal(t, 0, fake.putFeedbackCalls, "zero PutFeedback calls on every rejection above")
		assert.Empty(t, fake.refsSnapshot())
	})

	t.Run("malformed session id and non-positive turn id never reach SQL", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		deps := &Deps{Store: fake}
		h := newFeedbackHertz(deps)

		for name, payload := range map[string]string{
			"bad session shape": `{"session_id":"bad_id!","turn_id":42,"verdict":"helpful"}`,
			"zero turn id":      `{"session_id":"fbtest-session-01","turn_id":0,"verdict":"helpful"}`,
			"negative turn id":  `{"session_id":"fbtest-session-01","turn_id":-5,"verdict":"helpful"}`,
		} {
			resp := fbRequest(h, http.MethodPost, "/api/feedback", payload, "")
			assert.Equal(t, http.StatusBadRequest, resp.Code, "%s must 400 (body: %s)", name, resp.Body.String())
		}

		assert.Equal(t, 0, fake.getSessionTurnCalls, "input rejected by shape must never issue a turn lookup")
		assert.Equal(t, 0, fake.queryAuditCalls, "input rejected by shape must never issue an audit probe")
		assert.Empty(t, fake.feedbackSnapshot())
	})

	t.Run("unknown, non-assistant and cross-session turns are a uniform 404", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "user", fbTurnTs, "", "")            // exists, wrong role
		fake.fbSeedTurn("other-session-9999", 7, "assistant", fbTurnTs, "", "") // exists, other session
		deps := &Deps{Store: fake}
		h := newFeedbackHertz(deps)

		cases := map[string]string{
			"unknown turn":    `{"session_id":"fbtest-session-01","turn_id":999,"verdict":"helpful"}`,
			"user turn":       `{"session_id":"fbtest-session-01","turn_id":42,"verdict":"helpful"}`,
			"foreign session": `{"session_id":"fbtest-session-01","turn_id":7,"verdict":"helpful"}`,
		}
		var bodies []string
		for name, payload := range cases {
			resp := fbRequest(h, http.MethodPost, "/api/feedback", payload, "")
			require.Equal(t, http.StatusNotFound, resp.Code, "%s must 404 (body: %s)", name, resp.Body.String())
			bodies = append(bodies, resp.Body.String())
			assert.Empty(t, fake.feedbackSnapshot(), "%s writes no vote", name)
		}
		// 不区分原因: all three envelopes are byte-identical, so the status
		// code never leaks WHY a turn is not votable (T-29-06 posture).
		require.Len(t, bodies, 3)
		assert.Equal(t, bodies[0], bodies[1])
		assert.Equal(t, bodies[1], bodies[2])
	})

	t.Run("audit attribution failure degrades without failing the vote", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11,12")
		fake.failQueryAudit = true // the probe errors — the vote still lands
		deps := &Deps{Store: fake}
		h := newFeedbackHertz(deps)

		resp := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, resp.Code, "a failed attribution probe must never 500 the vote (Pitfall 4)")

		rows := fake.feedbackSnapshot()
		require.Len(t, rows, 1)
		assert.Empty(t, rows[0].SkillID, "degraded attribution is an honest empty, not a fabricated skill")
		assert.Empty(t, rows[0].ChunkIDs)
		assert.Empty(t, fake.refsSnapshot(), "no citations known -> no refs rows")
	})

	t.Run("audit row simply absent also degrades", func(t *testing.T) {
		// Turn exists but no audit row precedes it (legacy rows, replayed
		// traffic): same honest-empty degradation, vote recorded 201.
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "", "")
		deps := &Deps{Store: fake}
		h := newFeedbackHertz(deps)

		resp := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, resp.Code)
		rows := fake.feedbackSnapshot()
		require.Len(t, rows, 1)
		assert.Empty(t, rows[0].SkillID)
		assert.Empty(t, rows[0].ChunkIDs)
		assert.Empty(t, fake.refsSnapshot())
	})

	t.Run("retriever gap degrades kb_slug to empty without failing the vote", func(t *testing.T) {
		// (a) the retriever is not wired at all...
		fake := newFeedbackFakeStore()
		fake.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11")
		deps := &Deps{Store: fake} // Retriever nil
		h := newFeedbackHertz(deps)
		resp := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, resp.Code, "an unwired retriever is a knowledge-attribution gap, not a vote failure")
		refs := fake.refsSnapshot()
		require.Len(t, refs, 1, "the chunk id itself is still known (from the audit CSV)")
		assert.Empty(t, refs[0].KBSlug, "kb_slug degrades to empty when the tier cannot be resolved")

		// (b) ...and the retriever errors on the lookup.
		fake2 := newFeedbackFakeStore()
		fake2.fbSeedTurn(fbTestSession, 42, "assistant", fbTurnTs, "compliance", "11")
		deps2 := &Deps{Store: fake2, Retriever: &fbStubRetriever{err: errors.New("simulated retriever outage")}}
		h2 := newFeedbackHertz(deps2)
		resp2 := fbRequest(h2, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusCreated, resp2.Code)
		refs2 := fake2.refsSnapshot()
		require.Len(t, refs2, 1)
		assert.Empty(t, refs2[0].KBSlug)
	})

	t.Run("store not wired answers 503", func(t *testing.T) {
		deps := &Deps{} // no Store, no Retriever
		h := newFeedbackHertz(deps)
		resp := fbRequest(h, http.MethodPost, "/api/feedback", validBody, "")
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "a vote can never be accepted without persistence (fail-closed)")
		assert.Contains(t, resp.Body.String(), "not wired")
	})
}
