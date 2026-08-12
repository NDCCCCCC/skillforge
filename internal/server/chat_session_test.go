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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// Phase 3 Plan 03-05 integration tests. These three tests are the
// end-to-end counterparts to the unit-level SC-8 / SC-9 tests in
// internal/agent (Plan 03-03) and internal/store/sqlite (Plan 03-02).
// They exercise the chatHandler + Agent.Run + Store writes through a
// real Hertz HTTP listener on a dynamic port.
//
//  1. TestCancelPropagationMidTool — SC-8 / AGNT-04: an in-flight
//     /api/chat request cancelled mid-call MUST return within 200ms
//     (deterministic in-process channel latency; the 2s t.Fatal upper
//     bound is a generous CI-runner ceiling, NOT the assertion).
//
//  2. TestSessionIsolationConcurrent — SC-9 / AGNT-05 end-to-end: two
//     concurrent /api/chat requests with different X-Session-Id headers
//     MUST partition cleanly at the Store.GetSession layer (each session
//     sees ONLY its own messages). Run under -race.
//
//  3. TestSessionIDGeneration — POST without an X-Session-Id header
//     MUST produce a server-generated ID (32 hex chars); a follow-up
//     POST with the same ID MUST accumulate a 2nd session turn.

// hangChatModel is a model.ToolCallingChatModel whose Generate blocks
// until ctx is cancelled. Used by TestCancelPropagationMidTool — the
// request enters the agent loop, calls chatModel.Generate, and hangs
// there until the test cancels the HTTP request's context. SC-8
// asserts the cancellation propagates back through Hertz → ctx →
// agent.Run → reactAgent.Generate → chatModel.Generate within 200ms.
type hangChatModel struct{}

var _ model.ToolCallingChatModel = (*hangChatModel)(nil)

func (m *hangChatModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (m *hangChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("hangChatModel: Stream not implemented")
}

func (m *hangChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// cannedChatModel returns a fixed JSON verdict on Generate. Used by
// TestSessionIsolationConcurrent + TestSessionIDGeneration — the
// agent's trust floor will refuse (the canned model never calls the
// retrieve tool → accumulator stays empty → ApplyTrustFloor fires),
// which is exactly what we want to assert at the HTTP layer: the
// audit row + session turn are written on the REFUSAL path too
// (GEVL-03 + AGNT-05 require persistence on every response shape,
// not just the verdict path).
type cannedChatModel struct {
	content string
}

var _ model.ToolCallingChatModel = (*cannedChatModel)(nil)

func (m *cannedChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	c := m.content
	if c == "" {
		c = `{"status":"unanswered","confidence":0.0,"citations":[],"review_required":true}`
	}
	return &schema.Message{Role: schema.Assistant, Content: c}, nil
}

func (m *cannedChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("cannedChatModel: Stream not implemented")
}

func (m *cannedChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// newTestAgent constructs an *agent.Agent with a stub manifest whose
// ResolvedTiers are injected via the SetResolvedTiersForTest affordance
// (Plan 03-03 exported this for exactly this cross-package test shape).
// The manifest is otherwise zero-value — agent.systemPrompt does not
// read any field the test relies on, and the retrieve tool closes over
// ResolvedTiers only.
func newTestAgent(t *testing.T, cm model.ToolCallingChatModel) *agent.Agent {
	t.Helper()
	m := &skill.Manifest{}
	m.SetResolvedTiersForTest([]string{"权威"})
	ag, err := agent.NewAgent(context.Background(), agent.AgentConfig{
		Manifest:  m,
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		ChatModel: cm,
		MaxHops:   1,
	})
	require.NoError(t, err, "agent.NewAgent must succeed with stub deps")
	return ag
}

// newTestStore returns a fresh in-memory SQLite store (Plan 03-02)
// suitable for SC-9 isolation tests. Each test gets its own private
// database — no cross-test leakage.
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err, "sqlite in-memory open")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// postChatRaw issues a POST /api/chat with custom headers + a
// cancellable context. Returns the resp + any error so the caller can
// time assertion boundaries (TestCancelPropagationMidTool) or read the
// body (TestSessionIDGeneration).
func postChatRaw(t *testing.T, ctx context.Context, addr, sessionID, query string) (*http.Response, error) {
	t.Helper()
	body, _ := json.Marshal(chatRequest{Query: query, Skill: "compliance"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/api/chat", bytes.NewReader(body))
	require.NoError(t, err, "build request")
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-Id", sessionID)
	}
	return http.DefaultClient.Do(req)
}

// drainBody reads + closes the response body, returning its string
// content. Tests use it to free the connection before assertion logic.
func drainBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil {
		return ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(b)
}

// TestCancelPropagationMidTool — SC-8 / AGNT-04 end-to-end. The agent
// uses hangChatModel whose Generate blocks on <-ctx.Done(). The HTTP
// client cancels its request context after a 50ms warm-up; the SSE
// stream MUST close within 200ms of the cancel signal (the in-process
// channel-close latency, NOT network latency — the 2s t.Fatal ceiling
// is a generous CI-runner backstop, NOT the SC-8 assertion).
//
// The audit-row state after cancel is intentionally not asserted here:
// cancel may race the audit write (chat_session_test.go documents
// "either NOT written OR written with RefusalReason=cancelled"; this
// implementation lands in the "request aborted before audit" branch
// because runAgentPath's writeAuditAndTurn call never executes when
// Agent.Run returns ctx.Err() first). Both behaviors are SC-8-
// compatible — the contract is about cancel latency, not audit
// completeness on the cancel path.
func TestCancelPropagationMidTool(t *testing.T) {
	deps := &Deps{
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		Agent:     newTestAgent(t, &hangChatModel{}),
		Store:     newTestStore(t),
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	type result struct {
		resp *http.Response
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := postChatRaw(t, ctx, addr, "sess-cancel", "any query")
		resCh <- result{resp, err}
	}()

	// Warm-up: give the request time to reach chatModel.Generate.
	time.Sleep(50 * time.Millisecond)
	cancelTime := time.Now()
	cancel()

	select {
	case res := <-resCh:
		elapsed := time.Since(cancelTime)
		t.Logf("SC-8: request returned in %v (warm-up-to-return %v); err=%v body=%q",
			elapsed, time.Since(start), res.err, truncateLogString(drainBody(t, res.resp), 200))
		assert.Less(t, elapsed, 200*time.Millisecond,
			"SC-8: cancel did not propagate within 200ms (elapsed=%v)", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("SC-8 FAIL: request did not return within 2s of cancel")
	}
}

// truncateLogString caps the body preview in test logs so a flushed
// 500-line SSE stream does not drown the test output.
func truncateLogString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// TestSessionIsolationConcurrent — SC-9 / AGNT-05 end-to-end. Two
// /api/chat requests with different X-Session-Id headers run in
// parallel against the same store; GetSession(sess-A) MUST return
// ONLY sess-A's messages, GetSession(sess-B) ONLY sess-B's. Run
// under -race.
//
// The agent uses cannedChatModel which never calls the retrieve tool,
// so the trust floor refuses on every request (no_authorized_source).
// This is the correct path to exercise: the SC-9 contract is about
// session-memory isolation, not verdict content — and the refusal
// path writes the audit row + session turn just like the verdict
// path (writeAuditAndTurn is called on every response shape). A
// future test could additionally assert the verdict path's session
// turn content, but that is not load-bearing for SC-9.
func TestSessionIsolationConcurrent(t *testing.T) {
	deps := &Deps{
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		Agent:     newTestAgent(t, &cannedChatModel{}),
		Store:     newTestStore(t),
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const (
		sessA = "sess-A-isolation"
		sessB = "sess-B-isolation"
	)
	var wg sync.WaitGroup
	wg.Add(2)
	runOne := func(sessionID, query string) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := postChatRaw(t, ctx, addr, sessionID, query)
		if err != nil {
			t.Errorf("POST /api/chat (sess=%s): %v", sessionID, err)
			return
		}
		_ = drainBody(t, resp)
	}
	go runOne(sessA, "what is the travel expense limit?")
	go runOne(sessB, "what is the non-compete duration?")
	wg.Wait()

	// Each session sees its own (user + assistant) pair ONLY.
	ctx := context.Background()
	msgsA, err := deps.Store.GetSession(ctx, sessA)
	require.NoError(t, err)
	msgsB, err := deps.Store.GetSession(ctx, sessB)
	require.NoError(t, err)

	require.Len(t, msgsA, 2, "sess-A must have exactly 2 messages (user + assistant)")
	require.Len(t, msgsB, 2, "sess-B must have exactly 2 messages (user + assistant)")
	assert.Equal(t, "user", msgsA[0].Role)
	assert.Contains(t, msgsA[0].Content, "travel expense")
	assert.Equal(t, "assistant", msgsA[1].Role)
	assert.Equal(t, "user", msgsB[0].Role)
	assert.Contains(t, msgsB[0].Content, "non-compete")
	assert.Equal(t, "assistant", msgsB[1].Role)

	// Negative: cross-session leakage check. The OTHER session's query
	// text MUST NOT appear in this session's message content — this is
	// the SC-9 regression lock. If a future refactor bleeds sessions
	// together (e.g. by reusing a session_id variable across goroutines,
	// or by missing the WHERE clause in GetSession), this fires.
	for _, m := range msgsA {
		assert.NotContains(t, m.Content, "non-compete",
			"sess-A MUST NOT contain sess-B's query text (SC-9 cross-session bleed)")
	}
	for _, m := range msgsB {
		assert.NotContains(t, m.Content, "travel expense",
			"sess-B MUST NOT contain sess-A's query text (SC-9 cross-session bleed)")
	}
}

// TestSessionIDGeneration — POST without an X-Session-Id header MUST
// produce a server-generated ID (32 hex chars per newSessionID's
// crypto/rand output). A follow-up POST with the returned ID MUST
// accumulate a 2nd session turn in the store (AGNT-05 persistence
// contract — the ID is the session partition key).
func TestSessionIDGeneration(t *testing.T) {
	deps := &Deps{
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		Agent:     newTestAgent(t, &cannedChatModel{}),
		Store:     newTestStore(t),
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	// 1. POST without session header → server generates one.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	resp1, err := postChatRaw(t, ctx1, addr, "", "first query")
	cancel1()
	require.NoError(t, err, "first POST (no session header)")
	generatedID := resp1.Header.Get("X-Session-Id")
	require.NotEmpty(t, generatedID, "server MUST echo an X-Session-Id when the client omitted one")
	assert.Len(t, generatedID, 32, "newSessionID returns a 32-hex-char ID (128 bits entropy, ASVS V6)")
	assert.Regexp(t, `^[0-9a-f]{32}$`, generatedID, "ID must be lowercase hex")
	_ = drainBody(t, resp1)

	// 2. POST with the returned ID → 2 turns for this session.
	resp2, err := postChatRaw(t, context.Background(), addr, generatedID, "second query")
	require.NoError(t, err, "second POST (echoed session ID)")
	echoedID := resp2.Header.Get("X-Session-Id")
	assert.Equal(t, generatedID, echoedID, "server MUST echo the client-supplied X-Session-Id verbatim")
	_ = drainBody(t, resp2)

	msgs, err := deps.Store.GetSession(context.Background(), generatedID)
	require.NoError(t, err)
	assert.Len(t, msgs, 4, "session must have 4 messages after 2 turns (user+assistant × 2)")
	if len(msgs) >= 2 {
		assert.Equal(t, "user", msgs[0].Role)
		assert.Equal(t, "first query", msgs[0].Content)
		assert.Equal(t, "user", msgs[2].Role)
		assert.Equal(t, "second query", msgs[2].Content)
	}
}
