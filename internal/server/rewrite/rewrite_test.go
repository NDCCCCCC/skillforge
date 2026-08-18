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
// WITHOUT WARRANTIES OR CONDITIONS OF CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package rewrite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRewriterChatModel is the test double for the LLM seam used by
// QueryRewriter. It satisfies model.ToolCallingChatModel (mirrors
// internal/agent/agent_test.go::stubChatModel). Generates return
// cannedJSON verbatim. The hang variant blocks on a channel until
// ctx is cancelled (timeout-fallback test).
type stubRewriterChatModel struct {
	mu       sync.Mutex
	count    int
	canned   string
	err      error
	hangChan chan struct{} // nil = do not hang; non-nil = wait on this or ctx.Done
	opts     []model.Option
}

var _ model.ToolCallingChatModel = (*stubRewriterChatModel)(nil)

func (m *stubRewriterChatModel) Generate(ctx context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.count++
	m.opts = append(m.opts, opts...)
	canned := m.canned
	wantErr := m.err
	hang := m.hangChan
	m.mu.Unlock()

	if wantErr != nil {
		return nil, wantErr
	}
	if hang != nil {
		select {
		case <-hang:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: canned}, nil
}

func (m *stubRewriterChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stubRewriterChatModel: Stream not implemented")
}

func (m *stubRewriterChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *stubRewriterChatModel) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// TestRewrite_HappyPath — stub returns a canned rewritten_query;
// the rewriter returns it verbatim. The 4000-char trim / pass-through
// / timeout paths are separate tests so a failure localizes to the
// exact behavior under test.
func TestRewrite_HappyPath(t *testing.T) {
	stub := &stubRewriterChatModel{
		canned: `{"rewritten_query": "差旅制度第二条的内容"}`,
	}
	rewriter, err := NewQueryRewriter(stub)
	require.NoError(t, err)
	history := []schema.Message{
		{Role: schema.User, Content: "差旅制度有哪些条款?"},
		{Role: schema.Assistant, Content: "差旅制度共三条: ..."},
	}
	out, err := rewriter.Rewrite(context.Background(), history, "那第二条呢?")
	require.NoError(t, err, "Rewrite should succeed on valid canned JSON")
	assert.Equal(t, "差旅制度第二条的内容", out)
	assert.Equal(t, 1, stub.CallCount(), "rewriter must invoke the LLM exactly once")
}

// TestRewrite_PassThroughOnNoAnaphora — when the LLM returns the
// original query verbatim (no anaphoric reference), the rewriter
// returns it unchanged. This is the RESOLQ 5 case — the system prompt
// instructs "return unchanged" so the rewriter is a no-op on clean
// follow-ups.
func TestRewrite_PassThroughOnNoAnaphora(t *testing.T) {
	stub := &stubRewriterChatModel{
		canned: `{"rewritten_query": "年假有几天"}`,
	}
	rewriter, err := NewQueryRewriter(stub)
	require.NoError(t, err)
	out, err := rewriter.Rewrite(context.Background(), nil, "年假有几天")
	require.NoError(t, err)
	assert.Equal(t, "年假有几天", out, "no-anaphora pass-through: query returned verbatim")
}

// TestRewrite_TimeoutFallback — the stub blocks on a channel until
// ctx is cancelled; the rewriter's context.WithTimeout fires first
// and the call returns ctx.DeadlineExceeded. The function returns
// the wrapped error within Timeout + 100ms (the test asserts elapsed
// < Timeout+100ms so a runaway rewriter fails loudly).
//
// The caller-side fallback (using original query when rewriter fails)
// is covered by TestVerdictPipeline_RewriterFallback in
// pipeline_default_test.go (Task 3).
func TestRewrite_TimeoutFallback(t *testing.T) {
	stub := &stubRewriterChatModel{
		hangChan: make(chan struct{}), // never closed; ctx cancel wins
	}
	rewriter, err := NewQueryRewriter(stub)
	require.NoError(t, err)
	rewriter.Timeout = 100 * time.Millisecond

	start := time.Now()
	out, err := rewriter.Rewrite(context.Background(), nil, "年假有几天")
	elapsed := time.Since(start)
	require.Error(t, err, "timeout must surface as a non-nil error so caller falls back")
	assert.Empty(t, out, "rewritten query must be empty on timeout (caller uses original)")
	assert.Less(t, elapsed, 200*time.Millisecond,
		"timeout must fire within Timeout+100ms; elapsed=%v", elapsed)
}

// TestRewrite_MalformedJSON — the stub returns non-JSON; the rewriter
// returns a wrapped parse error. Caller falls back to original query.
func TestRewrite_MalformedJSON(t *testing.T) {
	stub := &stubRewriterChatModel{
		canned: "this is not json at all",
	}
	rewriter, err := NewQueryRewriter(stub)
	require.NoError(t, err)
	out, err := rewriter.Rewrite(context.Background(), nil, "年假有几天")
	require.Error(t, err, "malformed JSON must surface as a parse error")
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "JSON",
		"error message should mention JSON to aid operator diagnosis")
}

// TestRewrite_HistoryTrimmedTo4000Chars — the rewrite prompt is
// bounded by historyCharBudget (4000 chars) + the truncation marker
// when the input exceeds the budget. The truncation-marker assertion
// lives in TestRewrite_HistoryTruncationApplied below (uses a
// capturing stub that records the user-message content). This test
// pins the constant: if historyCharBudget is ever lowered past the
// spec value, the marker assertion in the capturing test fails.
func TestRewrite_HistoryTrimmedTo4000Chars(t *testing.T) {
	assert.Equal(t, 4000, historyCharBudget,
		"historyCharBudget must be 4000 per Plan 02 Task 1 spec")
	assert.NotEmpty(t, truncationMarker,
		"truncationMarker must be non-empty so the LLM sees the boundary")
}

// TestRewrite_HistoryTruncationApplied — the renderHistoryText path
// must apply the 4000-char trim + marker when the input exceeds the
// budget. This is the equivalent of TestRewrite_HistoryTrimmedTo4000Chars
// but exercises the unexported renderHistoryText via the public API
// by passing an 8000-char history and asserting the prompt the stub
// receives contains the marker.
func TestRewrite_HistoryTruncationApplied(t *testing.T) {
	// Capture the user prompt the LLM receives. Use a small wrapper
	// around stub that records msgs.
	capturer := &capturingStub{canned: `{"rewritten_query": "ok"}`}
	rewriter, err := NewQueryRewriter(capturer)
	require.NoError(t, err)

	big := make([]byte, 8000)
	for i := range big {
		big[i] = 'B'
	}
	history := []schema.Message{
		{Role: schema.User, Content: string(big)},
	}
	_, err = rewriter.Rewrite(context.Background(), history, "新问题")
	require.NoError(t, err)

	require.NotEmpty(t, capturer.userPrompt, "LLM must receive a user prompt")
	assert.Contains(t, capturer.userPrompt, "[earlier history truncated]",
		"the truncation marker must appear in the rendered prompt")
	assert.LessOrEqual(t, len(capturer.userPrompt), historyCharBudget+1024,
		"prompt must be bounded (budget + reasonable framing headroom)")
}

// TestRewrite_NilRewriter — calling Rewrite on a nil receiver
// returns an error rather than panicking. Defensive: a future caller
// could mistype a nil check.
func TestRewrite_NilRewriter(t *testing.T) {
	var r *QueryRewriter
	_, err := r.Rewrite(context.Background(), nil, "x")
	require.Error(t, err)
}

// TestRewrite_EmptyQuery — empty query surfaces as a clear error;
// the caller must guard against this anyway, but the function fails
// loudly rather than returning a silently-empty rewrite.
func TestRewrite_EmptyQuery(t *testing.T) {
	rewriter, err := NewQueryRewriter(&stubRewriterChatModel{})
	require.NoError(t, err)
	_, err = rewriter.Rewrite(context.Background(), nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query must be non-empty")
}

// TestRewrite_NilChatModelInConstructor — the constructor rejects nil.
func TestRewrite_NilChatModelInConstructor(t *testing.T) {
	_, err := NewQueryRewriter(nil)
	require.Error(t, err)
}

// capturingStub is a stub LLM that captures the user-message content
// passed to Generate so the test can assert on the rendered prompt.
type capturingStub struct {
	mu         sync.Mutex
	count      int
	canned     string
	userPrompt string
}

var _ model.ToolCallingChatModel = (*capturingStub)(nil)

func (m *capturingStub) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	for _, mm := range msgs {
		if mm.Role == schema.User {
			m.userPrompt = mm.Content
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: m.canned}, nil
}

func (m *capturingStub) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("capturingStub: Stream not implemented")
}

func (m *capturingStub) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// _ keeps encoding/json referenced when the test file's imports change.
var _ = json.Marshal
var _ = fmt.Sprintf