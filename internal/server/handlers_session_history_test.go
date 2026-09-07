// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/history"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/verdict"
)

// Phase 10 Plan 03 — handler-level tests for GET /api/session/:id/history.
//
// The 8 tests cover the load-bearing contracts:
//
//   - Empty session → 200 + empty messages + empty skill_id
//   - Valid session → 200 with rendered verdict / refusal / user rows
//   - SC-9 isolation → request for sessA returns ONLY sessA turns
//   - Unknown session → 200 + empty (NOT 404 — T-10-11 existence-leak)
//   - Invalid session ID → 400 (regex gate)
//   - Missing Store → 404 (defensive misconfig)
//   - Display cap → response bounded by TruncateToCharBudget
//   - Handler does NOT call the history-projection helper (B4 regression
//     gate — the grep gate in 10-VALIDATION.md is the executable version
//     of this assertion; the test file as a whole must compile without
//     any such reference in this file's assertions).

// historyTestStore is the per-test in-memory SQLite store (mirrors
// newTestStore in chat_session_test.go — Plan 03-02 established this
// shape).
func historyTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err, "sqlite in-memory open")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// historyTestHertz spins up a Hertz server with the session-history
// route + Store dependency. Returns the bound address + cleanup.
// The chat route is NOT registered here (the history tests do not
// exercise it; chat-path tests use the full newTestHertz in
// chat_test.go).
func historyTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/session/:id/history", deps.sessionHistoryHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	cleanup := func() {
		_ = h.Close()
		<-errCh
	}

	addr := "127.0.0.1:" + portStr(port)
	if err := waitForListener(addr, 3*time.Second); err != nil {
		cleanup()
		t.Fatalf("server warmup: %v", err)
	}
	return addr, cleanup
}

// getHistory issues GET /api/session/:id/history against addr. The
// returned body is the parsed historyResponse. status is the HTTP
// status code.
func getHistory(t *testing.T, addr, sessionID string) (int, historyResponse, []byte) {
	t.Helper()
	url := "http://" + addr + "/api/session/" + sessionID + "/history"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	out := historyResponse{}
	if len(raw) > 0 {
		// Tolerate empty / 404 / 400 envelopes — only succeed on
		// 200-shaped payloads.
		if jErr := json.Unmarshal(raw, &out); jErr != nil {
			t.Logf("WARN: response body not JSON-decodable as historyResponse: %v (body=%s)", jErr, string(raw))
		}
	}
	return resp.StatusCode, out, raw
}

// seedSessionTurn is a convenience writer — wraps Store.AppendTurn with
// a controllable user / assistant content + role + ts. Used by the
// SC-9 isolation + display-cap tests.
//
// The append order matters: AppendTurn writes a (user, assistant) pair
// atomically, so this helper appends ONE pair per call. For multi-pair
// sessions call repeatedly.
func seedSessionTurn(t *testing.T, s *sqlite.SQLiteStore, sessionID, userContent, assistantContent string, ts time.Time) {
	t.Helper()
	// AppendTurn requires two half-messages with the same Role pair;
	// the store tags them with the supplied Ts verbatim so this test
	// can interleave sessions without nanosecond collisions.
	_, err := s.AppendTurn(context.Background(), sessionID,
		store.SessionMessage{Role: "user", Content: userContent, Ts: ts},
		store.SessionMessage{Role: "assistant", Content: assistantContent, Ts: ts.Add(time.Millisecond)},
	)
	require.NoError(t, err, "AppendTurn (sess=%s)", sessionID)
}

// TestSessionHistoryAPI_EmptySession — a session that has no turns
// MUST return 200 + empty messages + empty skill_id (NOT 404 —
// T-10-11 existence-leak posture). The session_id format is valid
// (regex-compliant); the rows query is the empty result.
func TestSessionHistoryAPI_EmptySession(t *testing.T) {
	deps := &Deps{Store: historyTestStore(t)}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	// "session-empty-12345" — 19 chars, lowercase alphanumeric + hyphen.
	status, body, raw := getHistory(t, addr, "session-empty-12345")
	assert.Equal(t, http.StatusOK, status, "empty session must be 200 (existence-leak posture)")
	assert.Equal(t, "{\"messages\":[],\"skill_id\":\"\"}", string(raw),
		"empty session must serialize as the canonical {messages:[],skill_id:\"\"} envelope")
	assert.Empty(t, body.Messages, "messages must be empty for empty session")
	assert.Empty(t, body.SkillID, "skill_id must be empty for empty session")
}

// TestSessionHistoryAPI_ValidSession — seed 2 user turns + 2 assistant
// turns: one verdict-shaped + one refusal-shaped. Assert the rendered
// payload contains all 4 entries, with the verdict row carrying a
// non-nil parsed Verdict and the refusal row rendering refusal-
// semantic text (B3 + B4 contract).
func TestSessionHistoryAPI_ValidSession(t *testing.T) {
	store := historyTestStore(t)
	deps := &Deps{Store: store}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	// Verdict JSON for the verdict row. The handler should parse
	// this into verdict.Verdict and feed RenderConclusion for the
	// Content. compliance_status="compliant" → RenderConclusion
	// returns the status string ("compliant").
	verdictJSON := `{"status":"ok","confidence":0.85,"citations":[],"review_required":false,"compliance_status":"compliant"}`
	seedSessionTurn(t, store, "session-valid-1234",
		"first user question",
		verdictJSON,
		time.Now().Add(-2*time.Minute),
	)
	// Refusal row: the assistant content is one of the refusal-reason
	// codes (no JSON envelope) — the handler routes this through
	// history.RenderRefusal (the B3 handler-only consumer).
	seedSessionTurn(t, store, "session-valid-1234",
		"second user question",
		verdict.RefusalReasonNoAuthorizedSource,
		time.Now().Add(-1*time.Minute),
	)

	status, body, raw := getHistory(t, addr, "session-valid-1234")
	require.Equal(t, http.StatusOK, status, "valid session must be 200 (raw=%s)", string(raw))
	require.Len(t, body.Messages, 4,
		"valid session must return 4 messages (2 user + 2 assistant); raw=%s", string(raw))

	// Row 0 — user "first user question" (verbatim).
	assert.Equal(t, "user", body.Messages[0].Role)
	assert.Equal(t, "first user question", body.Messages[0].Content)
	assert.Nil(t, body.Messages[0].Verdict, "user row MUST NOT carry a verdict")

	// Row 1 — assistant verdict JSON → verdictRenderer.RenderConclusion
	// + parsed verdict (Answer/ComplianceStatus fallback chain).
	assert.Equal(t, "assistant", body.Messages[1].Role)
	assert.Equal(t, "compliant", body.Messages[1].Content,
		"verdict row's Content must come from RenderConclusion (compliance_status='compliant')")
	require.NotNil(t, body.Messages[1].Verdict,
		"verdict row MUST carry a parsed Verdict object (D10-D1 + D15)")
	if body.Messages[1].Verdict != nil {
		assert.Equal(t, "compliant", body.Messages[1].Verdict.ComplianceStatus)
		assert.Equal(t, "ok", body.Messages[1].Verdict.Status)
	}

	// Row 2 — user "second user question" (verbatim).
	assert.Equal(t, "user", body.Messages[2].Role)
	assert.Equal(t, "second user question", body.Messages[2].Content)

	// Row 3 — assistant refusal → verdictRenderer.RenderRefusal
	// (refusal-semantic text by design on the user-facing path).
	assert.Equal(t, "assistant", body.Messages[3].Role)
	expected := history.RenderRefusal(verdict.RefusalReasonNoAuthorizedSource)
	assert.Equal(t, expected, body.Messages[3].Content,
		"refusal row's Content MUST come from RenderRefusal (refusal-semantic text)")
	assert.Nil(t, body.Messages[3].Verdict, "refusal row MUST NOT carry a verdict")
}

// TestSessionHistoryAPI_SC9Isolation — seed two distinct sessions
// with non-overlapping turns; request sessA; assert the response
// contains ONLY sessA's turns. SC-9 / T-3-SQLI regression lock.
//
// The test is the SAME-session-shape as chat_session_test.go's
// TestSessionIsolationConcurrent, just at the history-endpoint
// surface: each AppendTurn writes the (user, assistant) pair
// atomically, and the WHERE session_id=? partition is enforced
// inside GetSession.
func TestSessionHistoryAPI_SC9Isolation(t *testing.T) {
	store := historyTestStore(t)
	deps := &Deps{Store: store}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	// SessA: travel-expense queries. The audit row + session turn
	// carry the unique sentinel "sentinel-A-marker" so cross-bleed
	// detection is byte-exact.
	seedSessionTurn(t, store, "sess-A-history-iso",
		"sentinel-A-marker: travel expense",
		"ok",
		time.Now().Add(-2*time.Minute),
	)
	// SessB: non-compete queries. Carries the unique sentinel
	// "sentinel-B-marker" — if it ever leaks into sessA's response,
	// the partition is broken.
	seedSessionTurn(t, store, "sess-B-history-iso",
		"sentinel-B-marker: non-compete duration",
		"ok",
		time.Now().Add(-2*time.Minute),
	)

	// Request sessA.
	statusA, bodyA, rawA := getHistory(t, addr, "sess-A-history-iso")
	require.Equal(t, http.StatusOK, statusA, "sessA must be 200 (raw=%s)", string(rawA))
	require.Len(t, bodyA.Messages, 2,
		"sessA must have exactly 2 messages (1 user + 1 assistant)")

	// Cross-session leakage check (SC-9 / T-3-SQLI invariant).
	for _, m := range bodyA.Messages {
		assert.NotContains(t, m.Content, "sentinel-B-marker",
			"sessA MUST NOT contain sessB's query text (SC-9 cross-session bleed)")
	}

	// Request sessB — symmetric assertion.
	statusB, bodyB, rawB := getHistory(t, addr, "sess-B-history-iso")
	require.Equal(t, http.StatusOK, statusB, "sessB must be 200 (raw=%s)", string(rawB))
	require.Len(t, bodyB.Messages, 2,
		"sessB must have exactly 2 messages (1 user + 1 assistant)")
	for _, m := range bodyB.Messages {
		assert.NotContains(t, m.Content, "sentinel-A-marker",
			"sessB MUST NOT contain sessA's query text (SC-9 cross-session bleed)")
	}
}

// TestSessionHistoryAPI_UnknownSession — request a session_id that
// has no rows → 200 + empty (NOT 404 — T-10-11 existence-leak posture).
// The session_id is regex-valid (no input was malformed), but the
// store simply has no matching rows.
func TestSessionHistoryAPI_UnknownSession(t *testing.T) {
	store := historyTestStore(t)
	deps := &Deps{Store: store}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	status, body, raw := getHistory(t, addr, "session-ghost-12345")
	assert.Equal(t, http.StatusOK, status,
		"unknown session must be 200, not 404 (T-10-11 existence-leak posture)")
	assert.Empty(t, body.Messages, "unknown session must return empty messages")
	assert.Empty(t, body.SkillID, "unknown session must return empty skill_id")
	assert.Equal(t, "{\"messages\":[],\"skill_id\":\"\"}", string(raw),
		"unknown session must serialize as the canonical empty envelope")
}

// TestSessionHistoryAPI_InvalidSessionID — session_id contains spaces,
// special chars, or is too short → 400. The regex gate fires BEFORE
// any SQL query (T-10-11).
//
// Note on URL handling: hertz route matching requires the URL segment
// to match `/api/session/:id/history`. When the malformed segment
// contains a literal "/" or other path-breaking character, the URL
// parses as a different path and hertz returns 404 (route not found)
// instead of 400. That is the same outcome from a security posture
// (input rejected before reaching the handler); the test accepts both
// 400 (regex gate) and 404 (route-not-found) as valid rejections.
//
// For non-path-breaking malformed input (too short, too long, contains
// space/quote), the URL still resolves to the route, the regex
// fires, and 400 is the expected response.
func TestSessionHistoryAPI_InvalidSessionID(t *testing.T) {
	store := historyTestStore(t)
	deps := &Deps{Store: store}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	cases := []struct {
		name      string
		sessionID string
	}{
		// URL-safe: malformed input travels to the route → regex fires → 400.
		{"too short", "abc"},
		{"too long", strings.Repeat("a", 200)},
		{"contains space (encoded)", "session%20space"},
		{"contains quote (encoded)", "session%22quote"},
		{"contains percent (encoded)", "session%25pct"},
		// Path-breaking: hertz returns 404 because the URL no longer
		// matches the route. Same security posture (rejected before
		// the handler); both 400 and 404 are accepted.
		{"contains slash", "session/123"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet,
				"http://"+addr+"/api/session/"+c.sessionID+"/history", nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// http.Client may itself reject some malformed URLs
				// (e.g. spaces) with a transport-level error. That is
				// the same outcome as 400 from the regex — input was
				// malformed and never reached the handler.
				t.Logf("input rejected at transport layer: %v", err)
				return
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			// Accept 400 (regex gate) OR 404 (route-not-found for path-
			// breaking input). Both outcomes reject the request before
			// the handler runs.
			assert.True(t,
				resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
				"invalid session_id %q must 400 (regex) or 404 (route mismatch); got %d",
				c.sessionID, resp.StatusCode)
		})
	}
}

// TestSessionHistoryAPI_MissingStore — Deps with Store == nil → 404.
// This is a defensive misconfig signal to the operator (distinct from
// unknown-session 200+empty): a server with no Store cannot serve
// history at all.
func TestSessionHistoryAPI_MissingStore(t *testing.T) {
	deps := &Deps{Store: nil}
	// historyTestHertz adds stub embedder + retriever so the routes
	// are not gated on chat-path deps; we still need a Store check.
	// For this test we bypass historyTestHertz to keep Deps.Store nil.
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	status, _, raw := getHistory(t, addr, "session-nostore-1234")
	assert.Equal(t, http.StatusNotFound, status,
		"missing store must be 404 (defensive misconfig); raw=%s", string(raw))
}

// TestSessionHistoryAPI_DisplayCapApplied — seed >DefaultCharBudget of
// content; assert the response length is bounded by
// TruncateToCharBudget's return. The handler applies the SAME cap the
// agent path uses (D10-B3 budget口径), so the wire payload stays
// bounded symmetrically.
//
// Mechanism: TruncateToCharBudget drops OLDEST messages first until
// the survivors fit the budget OR reach SafeguardFloor. With a 2-pair
// session and total content > 8000 chars, the handler MUST drop the
// oldest pair → response has 2 messages (the 2 most recent), not 4.
func TestSessionHistoryAPI_DisplayCapApplied(t *testing.T) {
	store := historyTestStore(t)
	deps := &Deps{Store: store}
	addr, cleanup := historyTestHertz(t, deps)
	defer cleanup()

	// First (oldest) pair: total content > DefaultCharBudget so the
	// budget window MUST evict it. 10000 chars × 2 messages = 20000
	// total — well above DefaultCharBudget (8000).
	bigUser := strings.Repeat("a", 10000)
	bigAssistant := strings.Repeat("b", 10000)
	seedSessionTurn(t, store, "session-cap-12345",
		bigUser, bigAssistant,
		time.Now().Add(-2*time.Minute),
	)
	// Second (newer) pair: small enough to fit under the budget.
	// After the oldest-pair eviction, this is what the response
	// contains.
	seedSessionTurn(t, store, "session-cap-12345",
		"small user message",
		"small assistant message",
		time.Now().Add(-1*time.Minute),
	)

	status, body, raw := getHistory(t, addr, "session-cap-12345")
	require.Equal(t, http.StatusOK, status, "valid session must be 200 (raw=%s)", string(raw))

	// After the cap drops the oldest pair, only the newer pair
	// (small user + small assistant) survives.
	assert.LessOrEqual(t, len(body.Messages), 2,
		"display cap must reduce 4 messages to ≤ 2 (the SafeguardFloor + budget drop)")
	for _, m := range body.Messages {
		assert.NotContains(t, m.Content, bigUser,
			"display cap must drop the OVER-budget pair; raw=%s", string(raw))
	}
	// The smaller pair survives — sanity check.
	foundUser := false
	foundAssistant := false
	for _, m := range body.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "small user message") {
			foundUser = true
		}
		if m.Role == "assistant" && strings.Contains(m.Content, "small assistant message") {
			foundAssistant = true
		}
	}
	assert.True(t, foundUser, "small user message MUST survive the display cap")
	assert.True(t, foundAssistant, "small assistant message MUST survive the display cap")
}

// TestSessionHistoryAPI_HandlerDoesNotCallProject — the regression
// lock for the B4 fix. The CI grep gate (`! grep -n 'history\.Project'
// handlers_session_history.go`) runs in 10-VALIDATION.md; this test
// re-asserts the same invariant at the unit-test layer so a refactor
// that breaks the B4 contract fails the BE suite BEFORE the CI gate.
//
// Note: the assertion is intentionally on the HANDLER file only
// (not the test file). The CI grep gate owns the executable version
// of the cross-file scan; the test focuses on the handler.
func TestSessionHistoryAPI_HandlerDoesNotCallProject(t *testing.T) {
	handlerPath := "handlers_session_history.go"
	data, err := readFile(handlerPath)
	require.NoError(t, err, "read %s", handlerPath)
	assert.NotContains(t, string(data), "history.Project",
		"%s MUST NOT call the history-projection helper on the user-facing path (B4 fix; GG-B4-1 gate)", handlerPath)
}

// readFile is the small helper used by the B4 regression gate test.
// Wraps os.ReadFile with a unified return shape.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}