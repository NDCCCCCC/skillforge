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
	"os"
	"regexp"
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
	"github.com/skillforge/skillforge/internal/store/sqlite"
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
	// putFeedbackChunkRefsCalls counts PutFeedbackChunkRefs invocations —
	// used to verify the WR-01 replay backfill is called.
	putFeedbackChunkRefsCalls int

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
		// WR-04 ActionEmpty: only verdict rows (empty action) qualify.
		if filter.ActionEmpty && r.Action != "" {
			continue
		}
		out = append(out, r)
	}
	// WR-04 OrderIDDesc: tie-break by id descending.
	if filter.OrderIDDesc {
		sort.Slice(out, func(i, j int) bool {
			if out[i].Timestamp.Equal(out[j].Timestamp) {
				return out[i].ID > out[j].ID
			}
			return out[i].Timestamp.After(out[j].Timestamp)
		})
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	}
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
	f.putFeedbackChunkRefsCalls++
	f.refRowsWritten += len(refs)
	// Deduplicate: ON CONFLICT(feedback_id, chunk_id) DO NOTHING means a
	// replay of the same (feedback_id, chunk_id) pair is a no-op, not a
	// second row.  The real store enforces this; the fake must mirror it.
	existing := make(map[int64]bool, len(f.refs[feedbackID]))
	for _, r := range f.refs[feedbackID] {
		existing[r.ChunkID] = true
	}
	for _, r := range refs {
		if !existing[r.ChunkID] {
			f.refs[feedbackID] = append(f.refs[feedbackID], r)
			existing[r.ChunkID] = true
		}
	}
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
		// Assign a deterministic ID for tie-break testing.
		id := int64(len(f.auditRows) + 1)
		f.auditRows = append(f.auditRows, store.AuditRow{
			ID:        id,
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
	h.GET("/api/admin/feedback/review", deps.feedbackReviewHandler)
	h.POST("/api/admin/feedback/dispositions", deps.feedbackDispositionHandler)
	h.GET("/api/admin/feedback/metrics", deps.feedbackMetricsHandler)
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
		// WR-04: ActionEmpty binds to verdict rows only; OrderIDDesc
		// provides a deterministic tie-break.
		require.Len(t, fake.queryAuditFilter, 1)
		assert.Equal(t, fbTestSession, fake.queryAuditFilter[0].SessionID)
		assert.True(t, fake.queryAuditFilter[0].Until.Equal(fbTurnTs), "Until must bind the turn ts")
		assert.Equal(t, 1, fake.queryAuditFilter[0].Limit)
		assert.True(t, fake.queryAuditFilter[0].ActionEmpty, "WR-04: attribution must filter to verdict rows (empty action)")
		assert.True(t, fake.queryAuditFilter[0].OrderIDDesc, "WR-04: attribution must order by ts DESC, id DESC for deterministic tie-break")
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
		assert.Equal(t, 2, fake.putFeedbackChunkRefsCalls, "WR-01: replay backfill must call PutFeedbackChunkRefs")
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

// ---------------------------------------------------------------------------
// Task 2 — admin review / disposition / metrics surface + FBK-05 red line
// ---------------------------------------------------------------------------

// feedbackRealStore is the per-test in-memory sqlite store (historyTestStore
// shape). The disposition lifecycle tests run against THIS — the pending
// derivation, the reopen semantics and the closed_count race ordering are
// the store's own 29-02 semantics, never a test-local copy of them.
func feedbackRealStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err, "sqlite in-memory open")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// fbPutVote appends one vote + its refs through the REAL store (the same
// INSERT-only paths the vote endpoint drives), with an explicit ts so the
// disposition-competes-with-vote ordering is deterministic.
func fbPutVote(t *testing.T, s *sqlite.SQLiteStore, sessionID string, turnID int64, verdict string, ts time.Time, chunkIDs ...int64) {
	t.Helper()
	ctx := context.Background()
	inserted, err := s.PutFeedback(ctx, store.FeedbackRow{
		SessionID: sessionID, TurnID: turnID, Verdict: verdict, TS: ts,
	})
	require.NoError(t, err)
	require.True(t, inserted, "test setup votes must not collide on (session, turn)")
	row, ok, err := s.FeedbackByTurn(ctx, sessionID, turnID)
	require.NoError(t, err)
	require.True(t, ok)
	refs := make([]store.ChunkRef, 0, len(chunkIDs))
	for _, id := range chunkIDs {
		refs = append(refs, store.ChunkRef{ChunkID: id, KBSlug: "authority"})
	}
	require.NoError(t, s.PutFeedbackChunkRefs(ctx, row.ID, refs))
}

// TestFeedbackReviewHandler — the review queue read model: the pending
// branch (derived rows + badge total) and the resolved branch (disposition
// history), each carrying ONLY its own half of the wire shape.
func TestFeedbackReviewHandler(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)

	fake := newFeedbackFakeStore()
	fake.pending = []store.PendingReviewRow{
		{ChunkID: 7, KBSlug: "authority", ReportCount: 3,
			LastReportedAt: time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC),
			Comments:       []string{"第 3 条已废止", ""}},
		{ChunkID: 9, KBSlug: "historical", ReportCount: 1,
			LastReportedAt: time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC),
			Comments:       []string{}},
	}
	fake.pendingCount = 2
	fake.dispositions = []store.DispositionRow{
		{ID: 2, TS: time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC), ChunkID: 7, Disposition: "updated", Note: "已更新", Actor: "admin"},
		{ID: 1, TS: time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC), ChunkID: 9, Disposition: "false_positive", Actor: "admin"},
	}
	h := newFeedbackHertz(&Deps{Store: fake})

	t.Run("pending view rides the derived rows plus the badge total", func(t *testing.T) {
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/review", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Pending []struct {
				ChunkID        int64    `json:"chunk_id"`
				KBSlug         string   `json:"kb_slug"`
				ReportCount    int      `json:"report_count"`
				LastReportedAt string   `json:"last_reported_at"`
				Comments       []string `json:"comments"`
			} `json:"pending"`
			Total int `json:"total"`
		}
		fbDecode(t, resp, &body)
		require.Len(t, body.Pending, 2)
		assert.EqualValues(t, 7, body.Pending[0].ChunkID, "rows arrive in the store's report_count-desc order")
		assert.Equal(t, "authority", body.Pending[0].KBSlug)
		assert.Equal(t, 3, body.Pending[0].ReportCount)
		ts, err := time.Parse(time.RFC3339, body.Pending[0].LastReportedAt)
		require.NoError(t, err, "last_reported_at must be an RFC3339 timestamp")
		assert.True(t, ts.Equal(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
		assert.Equal(t, []string{"第 3 条已废止", ""}, body.Pending[0].Comments)
		assert.Equal(t, 2, body.Total, "total is the store badge count, not len(pending) guesswork")
		assert.NotContains(t, resp.Body.String(), "dispositions", "the pending branch carries no disposition half")
	})

	t.Run("default status is pending", func(t *testing.T) {
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/review", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), `"pending"`)
	})

	t.Run("resolved view rides the disposition history", func(t *testing.T) {
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=resolved", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Dispositions []struct {
				ID          int64  `json:"id"`
				TS          string `json:"ts"`
				ChunkID     int64  `json:"chunk_id"`
				Disposition string `json:"disposition"`
				Note        string `json:"note"`
				Actor       string `json:"actor"`
			} `json:"dispositions"`
		}
		fbDecode(t, resp, &body)
		require.Len(t, body.Dispositions, 2)
		assert.EqualValues(t, 2, body.Dispositions[0].ID, "newest disposition first")
		assert.Equal(t, "updated", body.Dispositions[0].Disposition)
		assert.Equal(t, "已更新", body.Dispositions[0].Note)
		assert.Equal(t, "admin", body.Dispositions[0].Actor)
		ts, err := time.Parse(time.RFC3339, body.Dispositions[0].TS)
		require.NoError(t, err)
		assert.True(t, ts.Equal(time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)))
		assert.NotContains(t, resp.Body.String(), `"pending"`, "the resolved branch carries no pending half")
	})

	t.Run("disposition filter narrows the resolved view", func(t *testing.T) {
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=resolved&disposition=false_positive", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code)
		var body struct {
			Dispositions []struct {
				Disposition string `json:"disposition"`
			} `json:"dispositions"`
		}
		fbDecode(t, resp, &body)
		require.Len(t, body.Dispositions, 1)
		assert.Equal(t, "false_positive", body.Dispositions[0].Disposition)
	})

	t.Run("invalid status and misplaced filters are 400s", func(t *testing.T) {
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=weird", "", testAdminToken)
		require.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, resp.Body.String(), "pending")

		resp = fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=resolved&disposition=nope", "", testAdminToken)
		require.Equal(t, http.StatusBadRequest, resp.Code, "an unknown disposition filter is refused, never read as 'no history'")
		assert.Contains(t, resp.Body.String(), "must be one of")

		resp = fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=pending&disposition=updated", "", testAdminToken)
		assert.Equal(t, http.StatusBadRequest, resp.Code, "the disposition filter is resolved-branch only (a silent ignore would lie about what was filtered)")
	})

	t.Run("nil store answers 503 with a valid token", func(t *testing.T) {
		h2 := newFeedbackHertz(&Deps{})
		resp := fbRequest(h2, http.MethodGet, "/api/admin/feedback/review", "", testAdminToken)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code)
		assert.Contains(t, resp.Body.String(), "not wired")
	})
}

// TestFeedbackDispositionAdminGate — the disposition verb: the CHECK ORDER
// gate (403 BEFORE 503, Pitfall 8), the closed_count caliber (counted
// BEFORE the write), the audit row, and the lifecycle against a REAL
// store (closes every pending entry; a newer report reopens).
func TestFeedbackDispositionAdminGate(t *testing.T) {
	const dispositionBody = `{"chunk_id":7,"disposition":"updated","note":"已更新"}`

	t.Run("CHECK ORDER: a nil store behind a closed gate answers 403", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
		h := newFeedbackHertz(&Deps{}) // Store nil — the 503 must be unreachable
		for _, tc := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/admin/feedback/review", ""},
			{http.MethodPost, "/api/admin/feedback/dispositions", dispositionBody},
			{http.MethodGet, "/api/admin/feedback/metrics", ""},
		} {
			resp := fbRequest(h, tc.method, tc.path, tc.body, "")
			require.Equal(t, http.StatusForbidden, resp.Code,
				"%s %s: 403 must PRECEDE the nil-Store 503 (Pitfall 8)", tc.method, tc.path)
			assert.NotContains(t, resp.Body.String(), "not wired", "a 403 must not leak configuration state")
		}

		// A WRONG token is equally blind to configuration state.
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", dispositionBody, "wrong-token")
		assert.Equal(t, http.StatusForbidden, resp.Code)
		assert.NotContains(t, resp.Body.String(), "not wired")
	})

	t.Run("admin gate is the FIRST statement of every admin handler", func(t *testing.T) {
		// The executable version of the CHECK ORDER review comment: any
		// handler that grows a pre-gate statement (a nil check, a param
		// read that 503s first) breaks this grep before it can leak.
		src, err := os.ReadFile("feedback.go")
		require.NoError(t, err)
		for _, fn := range []string{"feedbackReviewHandler", "feedbackDispositionHandler", "feedbackMetricsHandler"} {
			// Only COMMENT lines may sit between the brace and the gate —
			// any executable statement before isAdminRequest breaks the lock.
			gate := regexp.MustCompile(`func \(s \*Deps\) ` + fn + `\([^)]*\) \{\s*\n(?:\s*//[^\n]*\n)*\s*if !isAdminRequest\(c\) \{`)
			assert.Regexp(t, gate, string(src),
				"%s must run the admin-token gate as its FIRST statement", fn)
		}
	})

	t.Run("no token with a live store is a 403 and a zero-write", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		s := feedbackRealStore(t)
		h := newFeedbackHertz(&Deps{Store: s})
		resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", dispositionBody, "")
		require.Equal(t, http.StatusForbidden, resp.Code)
		rows, err := s.ResolvedDispositions(context.Background(), "")
		require.NoError(t, err)
		assert.Empty(t, rows, "a gate-rejected disposition writes nothing")
	})

	t.Run("valid token with a nil store answers 503", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		h := newFeedbackHertz(&Deps{})
		resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", dispositionBody, testAdminToken)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})

	t.Run("invalid body and unknown disposition are 400s with zero writes", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		s := feedbackRealStore(t)
		h := newFeedbackHertz(&Deps{Store: s})

		for name, payload := range map[string]string{
			"unknown disposition": `{"chunk_id":7,"disposition":"maybe"}`,
			"zero chunk id":       `{"chunk_id":0,"disposition":"updated"}`,
			"not json":            `{`,
		} {
			resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", payload, testAdminToken)
			assert.Equal(t, http.StatusBadRequest, resp.Code, "%s must 400", name)
		}
		assert.Contains(t,
			fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", `{"chunk_id":7,"disposition":"maybe"}`, testAdminToken).Body.String(),
			"must be one of", "the 400 must name the closed disposition vocabulary")

		rows, err := s.ResolvedDispositions(context.Background(), "")
		require.NoError(t, err)
		assert.Empty(t, rows, "a rejected disposition writes nothing")
		audit, err := s.QueryAudit(context.Background(), store.AuditFilter{Action: "action_feedback_disposition"})
		require.NoError(t, err)
		assert.Empty(t, audit, "a rejected disposition writes no audit row")
	})

	t.Run("disposition closes every pending entry and lands an audit row", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		s := feedbackRealStore(t)
		h := newFeedbackHertz(&Deps{Store: s})
		ctx := context.Background()
		// Vote timestamps are clock-RELATIVE (one hour ago) on purpose:
		// the disposition's ts is stamped server-side at write time, and
		// the pending derivation closes a vote iff d.ts >= f.ts. A fixed
		// wall-date fixture can drift into the store's future and silently
		// flip the comparison.
		base := time.Now().UTC().Add(-time.Hour)

		// Two outdated reports on chunk 7 (the queue priority signal),
		// one helpful vote on the same chunk (must NOT count), and an
		// unrelated pending chunk 9.
		fbPutVote(t, s, "fbstore-session-01", 1, "outdated", base, 7)
		fbPutVote(t, s, "fbstore-session-01", 2, "outdated", base.Add(time.Minute), 7)
		fbPutVote(t, s, "fbstore-session-01", 3, "helpful", base.Add(2*time.Minute), 7)
		fbPutVote(t, s, "fbstore-session-01", 4, "outdated", base.Add(3*time.Minute), 9)

		resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions", dispositionBody, testAdminToken)
		require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())
		var body struct {
			ClosedCount int `json:"closed_count"`
		}
		fbDecode(t, resp, &body)
		assert.Equal(t, 2, body.ClosedCount, "closed_count = the pending entries the disposition resolved (the helpful vote is not one)")

		// The queue is a derived read model: chunk 7 fell out, chunk 9 remains.
		after := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=pending", "", testAdminToken)
		require.Equal(t, http.StatusOK, after.Code)
		assert.NotContains(t, after.Body.String(), `"chunk_id":7`, "a dispositioned chunk leaves the pending view")
		assert.Contains(t, after.Body.String(), `"chunk_id":9`)

		// Exactly one audit row, admin actor, JSON snapshot in FrozenArgs.
		audit, err := s.QueryAudit(ctx, store.AuditFilter{Action: "action_feedback_disposition"})
		require.NoError(t, err)
		require.Len(t, audit, 1)
		assert.Equal(t, "admin", audit[0].ActorType)
		var snap struct {
			ChunkID     int64  `json:"chunk_id"`
			Disposition string `json:"disposition"`
			Note        string `json:"note"`
			ClosedCount int    `json:"closed_count"`
		}
		require.NoError(t, json.Unmarshal(audit[0].FrozenArgs, &snap))
		assert.EqualValues(t, 7, snap.ChunkID)
		assert.Equal(t, "updated", snap.Disposition)
		assert.Equal(t, "已更新", snap.Note)
		assert.Equal(t, 2, snap.ClosedCount)

		// The resolved view now carries it.
		history := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=resolved", "", testAdminToken)
		require.Equal(t, http.StatusOK, history.Code)
		assert.Contains(t, history.Body.String(), `"disposition":"updated"`)
	})

	t.Run("a newer outdated report reopens the chunk (latest-disposition-wins)", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
		s := feedbackRealStore(t)
		h := newFeedbackHertz(&Deps{Store: s})

		fbPutVote(t, s, "fbstore-session-01", 1, "outdated", time.Now().UTC().Add(-time.Hour), 7)
		resp := fbRequest(h, http.MethodPost, "/api/admin/feedback/dispositions",
			`{"chunk_id":7,"disposition":"ignored"}`, testAdminToken)
		require.Equal(t, http.StatusCreated, resp.Code)

		pendingAfterDisposition := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=pending", "", testAdminToken)
		require.Equal(t, http.StatusOK, pendingAfterDisposition.Code)
		assert.NotContains(t, pendingAfterDisposition.Body.String(), `"chunk_id":7`)

		// A NEW report, strictly later than the disposition's server-side
		// stamp: the chunk is back (Open Q1 resolved semantics, driven
		// through the store's own NOT EXISTS derivation).
		fbPutVote(t, s, "fbstore-session-01", 2, "outdated", time.Now().UTC().Add(time.Second), 7)
		reopened := fbRequest(h, http.MethodGet, "/api/admin/feedback/review?status=pending", "", testAdminToken)
		require.Equal(t, http.StatusOK, reopened.Code, "body: %s", reopened.Body.String())
		assert.Contains(t, reopened.Body.String(), `"chunk_id":7`, "a post-disposition report must reopen the chunk")
		var body struct {
			Pending []struct {
				ReportCount int `json:"report_count"`
			} `json:"pending"`
			Total int `json:"total"`
		}
		fbDecode(t, reopened, &body)
		require.Len(t, body.Pending, 1)
		assert.Equal(t, 1, body.Pending[0].ReportCount, "report_count is the UNRESOLVED batch, not the all-time total")
		assert.Equal(t, 1, body.Total)
	})
}

// TestFeedbackMetricsHandler — the D-29-10 caliber: rates are Go-side,
// the denominator is ALL THREE verdicts, zero votes answer null (never
// 0%), and the per-KB dimension double-counts by design while totals
// stay strictly once-per-vote.
func TestFeedbackMetricsHandler(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)

	t.Run("per-skill caliber with the full-vote denominator", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.bySkillRows = []store.FeedbackCountRow{{Key: "compliance", Helpful: 1, Unhelpful: 1, Outdated: 2}}
		fake.pendingCount = 4
		fake.trendRows = []store.FeedbackTrendRow{
			{Day: "2026-09-08", Verdict: "outdated", Count: 2},
			{Day: "2026-09-08", Verdict: "unhelpful", Count: 1},
		}
		h := newFeedbackHertz(&Deps{Store: fake})

		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics?dim=skill&days=7", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Dim    string `json:"dim"`
			Days   int    `json:"days"`
			Totals struct {
				Helpful      int      `json:"helpful"`
				Unhelpful    int      `json:"unhelpful"`
				Outdated     int      `json:"outdated"`
				HelpRate     *float64 `json:"help_rate"`
				OutdatedRate *float64 `json:"outdated_rate"`
				TotalVotes   int      `json:"total_votes"`
				PendingCount int      `json:"pending_count"`
			} `json:"totals"`
			ByKey []struct {
				Key      string   `json:"key"`
				Helpful  int      `json:"helpful"`
				Outdated int      `json:"outdated"`
				HelpRate *float64 `json:"help_rate"`
			} `json:"by_key"`
			Trend []struct {
				Day     string `json:"day"`
				Verdict string `json:"verdict"`
				Count   int    `json:"count"`
			} `json:"trend"`
		}
		fbDecode(t, resp, &body)
		assert.Equal(t, "skill", body.Dim)
		assert.Equal(t, 7, body.Days)
		assert.Equal(t, 1, body.Totals.Helpful)
		assert.Equal(t, 1, body.Totals.Unhelpful)
		assert.Equal(t, 2, body.Totals.Outdated)
		require.NotNil(t, body.Totals.HelpRate)
		assert.InDelta(t, 0.25, *body.Totals.HelpRate, 1e-9, "help_rate = helpful / (h+u+o) — outdated STAYS in the denominator")
		require.NotNil(t, body.Totals.OutdatedRate)
		assert.InDelta(t, 0.5, *body.Totals.OutdatedRate, 1e-9)
		assert.Equal(t, 4, body.Totals.TotalVotes)
		assert.Equal(t, 4, body.Totals.PendingCount, "pending_count rides the metrics response (29-08 cross-link)")
		require.Len(t, body.ByKey, 1)
		assert.Equal(t, "compliance", body.ByKey[0].Key)
		require.Len(t, body.Trend, 2)
		assert.Equal(t, "2026-09-08", body.Trend[0].Day)

		// The window rides the store call: default-ish 7 days back.
		assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, -7), fake.lastSince, 2*time.Minute,
			"the since window must be wired into every metrics query")
	})

	t.Run("per-KB dimension double-counts by design while totals stay once-per-vote", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		fake.bySkillRows = []store.FeedbackCountRow{{Key: "compliance", Helpful: 1, Unhelpful: 1, Outdated: 1}}
		fake.byKBRows = []store.FeedbackCountRow{
			{Key: "authority", Helpful: 1, Unhelpful: 0, Outdated: 1},
			{Key: "historical", Helpful: 1, Unhelpful: 0, Outdated: 0},
		}
		h := newFeedbackHertz(&Deps{Store: fake})

		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics?dim=base", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())

		var body struct {
			Dim   string `json:"dim"`
			ByKey []struct {
				Key     string `json:"key"`
				Helpful int    `json:"helpful"`
			} `json:"by_key"`
			Totals struct {
				TotalVotes int `json:"total_votes"`
				Helpful    int `json:"helpful"`
				Unhelpful  int `json:"unhelpful"`
				Outdated   int `json:"outdated"`
			} `json:"totals"`
		}
		fbDecode(t, resp, &body)
		assert.Equal(t, "base", body.Dim)
		require.Len(t, body.ByKey, 2, "one row per KB — a multi-KB vote counts once per KB (Open Q2)")
		assert.Equal(t, 3, body.Totals.TotalVotes,
			"totals stay the STRICT once-per-vote aggregation: the KB rows sum to 4 votes for 3 actual votes, and reporting 4 would be a lie")
		assert.Equal(t, 1, body.Totals.Unhelpful)
	})

	t.Run("zero votes answer null rates, never 0 percent", func(t *testing.T) {
		fake := newFeedbackFakeStore() // no rows at all
		h := newFeedbackHertz(&Deps{Store: fake})
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code)

		raw := resp.Body.String()
		assert.Contains(t, raw, `"help_rate":null`, "an empty denominator is NULL — a 0%% help rate would read as 'everyone voted unhelpful'")
		assert.Contains(t, raw, `"outdated_rate":null`)
		assert.NotContains(t, raw, `"help_rate":0`)
		assert.Contains(t, raw, `"total_votes":0`)
		assert.Contains(t, raw, `"by_key":[]`, "by_key is an empty JSON array, not null")

		var body struct {
			Totals struct {
				HelpRate *float64 `json:"help_rate"`
			} `json:"totals"`
		}
		fbDecode(t, resp, &body)
		assert.Nil(t, body.Totals.HelpRate)
	})

	t.Run("window and dimension validation", func(t *testing.T) {
		fake := newFeedbackFakeStore()
		h := newFeedbackHertz(&Deps{Store: fake})

		for name, query := range map[string]string{
			"zero days":     "?days=0",
			"negative days": "?days=-3",
			"junk days":     "?days=last-week",
			"unknown dim":   "?dim=kb",
		} {
			resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics"+query, "", testAdminToken)
			assert.Equal(t, http.StatusBadRequest, resp.Code, "%s must 400 (body: %s)", name, resp.Body.String())
		}

		// days=30 widens the window that reaches the store.
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics?days=30", "", testAdminToken)
		require.Equal(t, http.StatusOK, resp.Code)
		assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, -30), fake.lastSince, 2*time.Minute)

		// Unknown dim never reaches the store as a silent "all rows" query.
		callsBefore := fake.metricsCalled
		_ = fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics?dim=kb", "", testAdminToken)
		assert.Equal(t, callsBefore, fake.metricsCalled, "a 400 must not issue store queries")
	})

	t.Run("nil store answers 503 with a valid token", func(t *testing.T) {
		h := newFeedbackHertz(&Deps{})
		resp := fbRequest(h, http.MethodGet, "/api/admin/feedback/metrics", "", testAdminToken)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})
}

// TestFeedbackNoTrustMutation — THE FBK-05 STRUCTURAL RED LINE. Feedback
// must never be able to change what the platform trusts: this file greps
// the source so that "no automatic knowledge mutation" is a test fact
// (T-29-05), not a code-review convention. A future handler that edits a
// chunk, re-embeds a document, or reaches the ingestion backend breaks
// this test first.
func TestFeedbackNoTrustMutation(t *testing.T) {
	src, err := os.ReadFile("feedback.go")
	require.NoError(t, err)
	text := string(src)

	// ① No row-mutation or row-removal verb anywhere in the surface:
	// feedback rows are append-only, so the file cannot carry UPDATE or
	// DELETE statements (nor INSERT( calls — writes go through the store
	// methods).
	for _, banned := range []string{"UPDATE ", "DELETE ", "Insert("} {
		assert.NotContains(t, text, banned,
			"feedback.go must not contain %q — feedback never rewrites or removes anything (FBK-05)", banned)
	}

	// ② The retriever is reachable ONLY through the GetChunk read (the
	// citation → kb_slug attribution). Any second retriever call on this
	// surface is a knowledge-write path by definition.
	callSites := regexp.MustCompile(`Retriever\s*\.\s*\w+\(`).FindAllString(text, -1)
	require.NotEmpty(t, callSites, "attribution must reach the retriever through Deps.Retriever")
	for _, site := range callSites {
		assert.Equal(t, "Retriever.GetChunk(", site,
			"the ONLY retriever call feedback.go may make is GetChunk (FBK-05: no knowledge-write path)")
	}

	// ③ The feedback/ticket storage family never references the knowledge
	// ingestion surface — the trust boundary is between two file sets,
	// not two code paths in one file.
	for _, rel := range []string{"feedback.go", "../store/sqlite/feedback.go", "../store/sqlite/tickets.go"} {
		familySrc, err := os.ReadFile(rel)
		require.NoError(t, err, rel)
		assert.NotContains(t, string(familySrc), "Indexer",
			"%s must not reference the knowledge ingestion surface (FBK-05)", rel)
	}

	// ④ The route table binds each of the four routes exactly once.
	routes, err := os.ReadFile("server.go")
	require.NoError(t, err)
	for _, route := range []struct{ verb, path string }{
		{"POST", "/api/feedback"},
		{"GET", "/api/admin/feedback/review"},
		{"POST", "/api/admin/feedback/dispositions"},
		{"GET", "/api/admin/feedback/metrics"},
	} {
		pattern := regexp.MustCompile(`(?m)^\s*h\.` + route.verb + `\(` + regexp.QuoteMeta(`"`+route.path+`"`) + `.*$`)
		assert.Len(t, pattern.FindAllString(string(routes), -1), 1,
			"%s %s must be registered exactly once", route.verb, route.path)
	}
}
