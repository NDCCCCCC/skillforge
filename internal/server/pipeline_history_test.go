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

// Phase 10 (Plan 10-02) Task 3 — VerdictPipeline history-aware
// regression tests. 8 tests pin the load-history → rewrite → classify
// → generate chain + the SC-3 hermetic scrub guarantee +
// nil-gate semantics.
//
// Tests:
//   - TestVerdictPipeline_HistoryAwareness       : multi-turn messages prefix
//   - TestVerdictPipeline_ClassifierRefuses      : semantic refusal short-circuits
//   - TestVerdictPipeline_RewriterFallback       : rewriter error → original query
//   - TestVerdictPipeline_ClassificationTypeIsolation : 2-field classifier input
//   - TestVerdictPipeline_NilRewriterStillFlowsHistory : B2 regression
//   - TestVerdictPipeline_HistoryChunkRefsSanitized    : W8 / SC-3 hermetic
//   - TestVerdictPipeline_BudgetTruncatedEvent        : budget event emission
//   - TestVerdictPipeline_RewriteEmitsEvent           : rewrite event emission
//
// All tests use the streaming LLM seam (Deps.PipelineChatModel set to
// stubStreamingChatModel so the messages slice reaches the test
// recorder). The Runner fallback path is exercised by the pre-Phase-10
// tests in pipeline_default_test.go (TestVerdictPipelineDefault_…).

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/server/classifier"
	"github.com/skillforge/skillforge/internal/server/rewrite"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// historyTestChunkText is the verbatim chunk text the seeded session_turn
// was paired with — kept short so the chunk text + history text stay
// under the 8000-char DefaultCharBudget without truncating.
const historyTestChunkText = "员工累计工作已满1年不满10年的，年假5天。"

// seedPriorSession appends a (user, assistant) turn pair to the
// store's session_turn table so VerdictPipeline.loadHistory returns
// a non-empty sanitized slice.
func seedPriorSession(t *testing.T, s store.Store, sessionID string, userContent string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := s.AppendTurn(ctx, sessionID,
		store.SessionMessage{Role: "user", Content: userContent, Ts: now},
		store.SessionMessage{Role: "assistant", Content: historyTestChunkText, Ts: now.Add(time.Second)},
	)
	require.NoError(t, err, "seed prior session turn")
}

// emitChunks returns the canned chunk list for the retrieval stub.
func emitChunks() []retriever.Hit {
	return []retriever.Hit{
		{ID: 1, Tier: "权威", Content: historyTestChunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}
}

// historyTestStubLM is the test double for the LLM seam used by the
// rewriter + classifier wrappers. It satisfies model.ToolCallingChatModel
// and records (a) the messages passed on each Generate call and (b)
// the canned response. Tests assert on the captured shape via
// StreamMessages() / Calls.
type historyTestStubLM struct {
	mu      sync.Mutex
	calls   int
	canned  string
	err     error
	messages [][]*schema.Message
}

var _ model.ToolCallingChatModel = (*historyTestStubLM)(nil)

func (m *historyTestStubLM) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	snapshot := make([]*schema.Message, len(msgs))
	for i, mm := range msgs {
		if mm == nil {
			continue
		}
		copyMsg := *mm
		snapshot[i] = &copyMsg
	}
	m.messages = append(m.messages, snapshot)
	if m.err != nil {
		return nil, m.err
	}
	if m.canned == "" {
		return &schema.Message{Role: schema.Assistant, Content: "{}"}, nil
	}
	return &schema.Message{Role: schema.Assistant, Content: m.canned}, nil
}

func (m *historyTestStubLM) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("historyTestStubLM: Stream not implemented")
}

func (m *historyTestStubLM) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *historyTestStubLM) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *historyTestStubLM) CapturedMessages() [][]*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*schema.Message, len(m.messages))
	for i, s := range m.messages {
		copied := make([]*schema.Message, len(s))
		copy(copied, s)
		out[i] = copied
	}
	return out
}

// newHistoryAwareDeps constructs a Deps with the streaming LLM seam +
// stub rewriter + stub classifier (both nil-gated by default; tests
// opt in as needed). The streaming ChatModel returns a verdict JSON
// so the existing grounding path stays green.
func newHistoryAwareDeps(t *testing.T, sessionID string, seedHistory bool) *Deps {
	t.Helper()
	deps := newVerdictPipelineDeps(t)
	deps.PipelineChatModel = &stubStreamingChatModel{
		// 3-token stream so the assembled buffer equals a complete
		// Verdict JSON after concatenation.
		tokens: []string{
			`{"status":"ok","confidence":0.9,"citations":[{"chunk_id":"1","span":"`,
			historyTestChunkText,
			`","tier":"权威"}],"review_required":false,"compliance_status":"not_applicable"}`,
		},
	}
	if seedHistory {
		seedPriorSession(t, deps.Store, sessionID, "差旅制度第一条的内容")
	}
	return deps
}

// TestVerdictPipeline_HistoryAwareness — when Deps.PipelineChatModel
// is non-nil and history exists, the messages slice passed to
// PipelineChatModel.Stream MUST contain the sanitized history as a
// middle prefix between System and User messages. Empty history →
// exactly [System, User] (baseline preserved).
func TestVerdictPipeline_HistoryAwareness(t *testing.T) {
	const sessID = "sess-p10-history-aware"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	rewriterLM := &historyTestStubLM{canned: `{"rewritten_query": "差旅制度第一条的内容"}`}
	deps.Rewriter, _ = rewrite.NewQueryRewriter(rewriterLM)
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	_ = drainBody(t, resp)

	// Assert the streaming model saw [System, ...history, User].
	streamCM := deps.PipelineChatModel.(*stubStreamingChatModel)
	msgsList := streamCM.StreamMessages()
	require.NotEmpty(t, msgsList, "Stream was never called")
	msgs := msgsList[len(msgsList)-1]
	require.GreaterOrEqual(t, len(msgs), 3,
		"history path: messages slice must be ≥3 (System + ≥1 history + User); got %d", len(msgs))

	assert.Equal(t, schema.System, msgs[0].Role, "messages[0] is system")
	last := msgs[len(msgs)-1]
	assert.Equal(t, schema.User, last.Role, "messages[len-1] is user (trust-bearing evidence)")

	// At least one history message between System and User.
	foundHistory := false
	for i := 1; i < len(msgs)-1; i++ {
		if msgs[i].Role != schema.System && msgs[i].Role != schema.User {
			foundHistory = true
			break
		}
	}
	assert.True(t, foundHistory,
		"at least one non-System / non-User history message must be present in the slice")
}

// TestVerdictPipeline_ClassifierRefuses — the classifier returns
// Decision="refuse", Reason="out_of_scope", Confidence=0.92. The
// pipeline MUST emit a refusal, write audit + audit_meta, and the
// verdict LLM MUST NEVER be called (StreamCount=0 for the verdict
// model — the streaming ChatModel is never invoked).
func TestVerdictPipeline_ClassifierRefuses(t *testing.T) {
	const sessID = "sess-p10-classifier-refuses"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	rewriterLM := &historyTestStubLM{canned: `{"rewritten_query": "差旅制度第一条"}`}
	deps.Rewriter, _ = rewrite.NewQueryRewriter(rewriterLM)
	classifierLM := &historyTestStubLM{canned: `{"decision": "refuse", "reason": "out_of_scope", "confidence": 0.92}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	body := drainBody(t, resp)

	// (a) Refusal emitted.
	frames := parseFrames(t, body)
	r := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, r.Reason,
		"classifier-refused: refusal reason must match classifier output")

	// (b) Verdict LLM NEVER called (StreamCount=0).
	streamCM := deps.PipelineChatModel.(*stubStreamingChatModel)
	assert.Equal(t, 0, streamCM.StreamCount(),
		"classifier-refused: verdict LLM must NOT be invoked")

	// (c) Audit row written.
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "exactly one audit row")
	assert.Equal(t, verdict.RefusalReasonOutOfScope, rows[0].RefusalReason)
}

// TestVerdictPipeline_RewriterFallback — the rewriter returns an
// error. The pipeline MUST continue with the ORIGINAL query, the
// classifier MUST still run.
func TestVerdictPipeline_RewriterFallback(t *testing.T) {
	const sessID = "sess-p10-rewriter-fallback"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	rewriterLM := &historyTestStubLM{err: errors.New("simulated LLM outage")}
	deps.Rewriter, _ = rewrite.NewQueryRewriter(rewriterLM)
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	body := drainBody(t, resp)

	// Pipeline still produced a verdict (rewriter fallback is
	// best-effort, not a hard gate).
	frames := parseFrames(t, body)
	v := mustVerdict(t, frames)
	assert.NotNil(t, v, "rewriter fallback: verdict must still be produced")
	assert.Equal(t, "ok", v.Status, "verdict path: status=ok after rewriter fallback")

	// Classifier still ran (1 LLM call from the classifier path).
	assert.Equal(t, 1, classifierLM.CallCount(),
		"classifier must still run after rewriter fallback")
}

// TestVerdictPipeline_ClassificationTypeIsolation — the classifier's
// captured input has exactly 2 fields (Query + Chunks). Reflection
// locks the D10-A1 contract at runtime (the test fails loudly if a
// future contributor adds a third field).
func TestVerdictPipeline_ClassificationTypeIsolation(t *testing.T) {
	const sessID = "sess-p10-iso"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	_ = drainBody(t, resp)

	// Assert the classifier was called and produced output.
	assert.GreaterOrEqual(t, classifierLM.CallCount(), 1,
		"classifier LLM was never called")

	// Assert the RefusalClassifierInput type itself has exactly 2
	// fields (D10-A1 — type-level isolation contract).
	rt := reflect.TypeOf(classifier.RefusalClassifierInput{})
	assert.Equal(t, 2, rt.NumField(),
		"RefusalClassifierInput must have exactly 2 fields (D10-A1 — type-level isolation)")
	assert.Equal(t, "Query", rt.Field(0).Name)
	assert.Equal(t, "Chunks", rt.Field(1).Name)
}

// TestVerdictPipeline_NilRewriterStillFlowsHistory — B2 key fix.
// Both Rewriter AND RefusalClassifier are nil; history is non-empty.
// The captured messages slice MUST still contain the sanitized
// history as a prefix (the nil-gate MUST NOT suppress history
// projection).
func TestVerdictPipeline_NilRewriterStillFlowsHistory(t *testing.T) {
	const sessID = "sess-p10-nil-both-history"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	// BOTH nil-gated — but history exists.
	require.Nil(t, deps.Rewriter)
	require.Nil(t, deps.RefusalClassifier)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	_ = drainBody(t, resp)

	// Capture the messages slice and assert the history prefix is
	// present even though both Rewriter + Classifier are nil.
	streamCM := deps.PipelineChatModel.(*stubStreamingChatModel)
	msgsList := streamCM.StreamMessages()
	require.NotEmpty(t, msgsList)
	msgs := msgsList[len(msgsList)-1]

	// The slice must be [System, ...history, User] with ≥3 entries.
	require.GreaterOrEqual(t, len(msgs), 3,
		"B2 fix: history prefix MUST flow when both deps are nil; got %d messages", len(msgs))

	// Look for the seeded prior-turn content in the history-prefix
	// slice. history.Project sanitized it (refusal-semantic-free)
	// before reaching the LLM.
	foundHistoryContent := false
	for i := 1; i < len(msgs)-1; i++ {
		if strings.Contains(msgs[i].Content, "差旅") {
			foundHistoryContent = true
			break
		}
	}
	assert.True(t, foundHistoryContent,
		"B2 fix: history-prefix message must carry sanitized prior turn content")
}

// TestVerdictPipeline_HistoryChunkRefsSanitized — checker W8 / SC-3
// hermetic coverage. Seed a prior session_turn whose user message
// contains a `[CHUNK 9999]` literal (a fake citation ID that is NOT
// in the current retrieval set). Asserts:
//
//   - (a) The messages slice passed to the LLM contains ZERO `[CHUNK `
//     literal in the history prefix (D9-B2 scrub).
//   - (b) The emitted data-policy Verdict's citations do NOT include
//     ChunkID="9999" — the grounding checker strips ungrounded refs
//     (D2-01).
//   - (c) For any response chunk that surfaces a chunk_id, none equals
//     "9999" (the seeded fake).
func TestVerdictPipeline_HistoryChunkRefsSanitized(t *testing.T) {
	const (
		sessID    = "sess-p10-sc3"
		fakeChunk = "9999"
	)
	deps := newHistoryAwareDeps(t, sessID, false) // no auto-seed
	// Manually seed a session turn with a [CHUNK 9999] literal in
	// the user message — the adversarial input.
	seedPriorSession(t, deps.Store, sessID,
		"我刚才看到的 [CHUNK "+fakeChunk+"] 里说 ... 请验证该条款是否合规。")

	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	// Stream a verdict that attempts to cite the fake ChunkID="9999".
	// The grounding check MUST strip this citation (D2-01 — the fake
	// chunk ID is not in chunksByID).
	streamCM := deps.PipelineChatModel.(*stubStreamingChatModel)
	streamCM.tokens = []string{
		`{"status":"ok","confidence":0.9,"citations":[{"chunk_id":"9999","span":"`,
		historyTestChunkText,
		`","tier":"权威"}],"review_required":false,"compliance_status":"not_applicable"}`,
	}

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	body := drainBody(t, resp)

	// (a) ZERO `[CHUNK ` literal in the history-prefix slice.
	msgsList := streamCM.StreamMessages()
	require.NotEmpty(t, msgsList)
	msgs := msgsList[len(msgsList)-1]
	for i := 1; i < len(msgs)-1; i++ {
		assert.NotContains(t, msgs[i].Content, "[CHUNK ",
			"history-prefix message[%d] must NOT contain [CHUNK literal (D9-B2 / SC-3)", i)
		assert.NotContains(t, msgs[i].Content, fakeChunk,
			"history-prefix message[%d] must NOT carry the seeded fake chunk ID %s", i, fakeChunk)
	}

	// (b) The data-policy Verdict's citations do NOT include ChunkID="9999".
	frames := parseFrames(t, body)
	v := mustVerdict(t, frames)
	require.NotNil(t, v, "verdict must still be produced (with the ungrounded citation stripped)")
	for _, cit := range v.Citations {
		assert.NotEqual(t, fakeChunk, cit.ChunkID,
			"grounding checker must strip ungrounded ChunkID=%s (D2-01)", fakeChunk)
	}

	// (c) No source-document chunk carries the fake ID.
	for _, f := range frames {
		if f.Type != "source-document" {
			continue
		}
		assert.NotEqual(t, fakeChunk, f.SourceID,
			"source-document frame MUST NOT carry the seeded fake ChunkID=%s", fakeChunk)
	}
}

// TestVerdictPipeline_BudgetTruncatedEvent — Deps seeded with a
// history that exceeds DefaultCharBudget. Asserts the
// history_budget_truncated data-decision event is emitted.
func TestVerdictPipeline_BudgetTruncatedEvent(t *testing.T) {
	const sessID = "sess-p10-budget-trunc"
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	// Append a giant user message that pushes the rendered history
	// over the 8000-char DefaultCharBudget.
	ctx := context.Background()
	huge := strings.Repeat("A", 8500)
	_, err := deps.Store.AppendTurn(ctx, sessID,
		store.SessionMessage{Role: "user", Content: huge, Ts: time.Now().UTC().Add(2 * time.Second)},
		store.SessionMessage{Role: "assistant", Content: "B", Ts: time.Now().UTC().Add(3 * time.Second)},
	)
	require.NoError(t, err)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	body := drainBody(t, resp)

	// Search the emitted SSE stream for the history_budget_truncated
	// data-decision event.
	frames := parseFrames(t, body)
	foundBudget := false
	for _, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == wire.DecisionHistoryBudgetTruncated {
			foundBudget = true
			assert.NotEmpty(t, p.Query, "budget-truncated event must carry the dropped count in Query")
			break
		}
	}
	assert.True(t, foundBudget,
		"history_budget_truncated data-decision event MUST be emitted when the char-budget window drops messages")
}

// TestVerdictPipeline_RewriteEmitsEvent — Deps with rewriter
// returning a non-empty rewrite. Asserts the history_query_rewritten
// data-decision event is emitted.
func TestVerdictPipeline_RewriteEmitsEvent(t *testing.T) {
	const (
		sessID          = "sess-p10-rewrite-emit"
		expectedRewrite = "差旅制度第一条的内容"
	)
	deps := newHistoryAwareDeps(t, sessID, true)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	rewriterLM := &historyTestStubLM{
		canned: fmt.Sprintf(`{"rewritten_query": %q}`, expectedRewrite),
	}
	deps.Rewriter, _ = rewrite.NewQueryRewriter(rewriterLM)
	classifierLM := &historyTestStubLM{canned: `{"decision": "allow", "confidence": 0.95}`}
	deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "那第一条呢")
	require.NoError(t, err)
	body := drainBody(t, resp)

	// Search the SSE stream for the history_query_rewritten event.
	frames := parseFrames(t, body)
	found := false
	for _, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == wire.DecisionHistoryQueryRewritten {
			found = true
			assert.Equal(t, expectedRewrite, p.Query,
				"history_query_rewritten event Query must carry the rewritten text")
			break
		}
	}
	assert.True(t, found,
		"history_query_rewritten data-decision event MUST be emitted when the rewriter succeeds")
}

// mustVerdict scans parsed SSE frames for the data-policy Verdict and
// returns it (or nil if not found).
func mustVerdict(t *testing.T, frames []wire.UIMessageChunk) *verdict.Verdict {
	t.Helper()
	for _, f := range frames {
		if f.Type != "data-policy" {
			continue
		}
		var v verdict.Verdict
		if err := json.Unmarshal(f.Data, &v); err != nil {
			continue
		}
		if v.Status != "" {
			return &v
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Phase 29 / Plan 29-04 Task 2 — data-turn frame-order locks (T-29-06).
//
// The behavior matrix under test (Pitfall 3's frame-order window):
//
//   - verdict stream: data-turn sits AFTER the post-verdict annotation
//     frames (counter-evidence / redline) and BEFORE text-end /
//     finish-step / finish; turnId == the assistant row id
//     writeAuditAndTurn persisted (Store.GetSession is ground truth).
//   - every refusal shape (out_of_scope / no_authorized_source /
//     low_retrieval_score / classifier-refuse / status-refused):
//     data-turn precedes emitRefusal's text-start — and therefore the
//     finish emitRefusal writes internally — because the refusal
//     terminal frames live INSIDE emitRefusal.
//   - turnID==0 (nil Store): ZERO data-turn frames; the rest of the
//     frame sequence is byte-identical.
// ---------------------------------------------------------------------------

// frameTypeList renders the frame type sequence for assertion messages.
func frameTypeList(frames []wire.UIMessageChunk) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, f.Type)
	}
	return out
}

// indexOfChunkType returns the index of the FIRST frame with the given
// type, or -1 when absent.
func indexOfChunkType(frames []wire.UIMessageChunk, chunkType string) int {
	for i, f := range frames {
		if f.Type == chunkType {
			return i
		}
	}
	return -1
}

// dataTurnIndexes returns the indexes of every data-turn frame.
func dataTurnIndexes(frames []wire.UIMessageChunk) []int {
	out := make([]int, 0, 1)
	for i, f := range frames {
		if f.Type == "data-turn" {
			out = append(out, i)
		}
	}
	return out
}

// soleTurnID asserts exactly one data-turn frame exists and decodes its
// TurnPayload.
func soleTurnID(t *testing.T, frames []wire.UIMessageChunk) int64 {
	t.Helper()
	idx := dataTurnIndexes(frames)
	require.Len(t, idx, 1,
		"expected exactly one data-turn frame; got %d (sequence=%v)", len(idx), frameTypeList(frames))
	var p wire.TurnPayload
	require.NoError(t, json.Unmarshal(frames[idx[0]].Data, &p),
		"data-turn payload must unmarshal into wire.TurnPayload; raw=%s", string(frames[idx[0]].Data))
	return p.TurnID
}

// assistantRowID returns the id of the session's LAST assistant row
// (GetSession is the writeAuditAndTurn ground truth — 29-01 backfills
// SessionMessage.ID).
func assistantRowID(t *testing.T, s store.Store, sessionID string) int64 {
	t.Helper()
	msgs, err := s.GetSession(context.Background(), sessionID)
	require.NoError(t, err, "GetSession(%s)", sessionID)
	require.NotEmpty(t, msgs, "session must have turns")
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			require.Greater(t, msgs[i].ID, int64(0),
				"GetSession must backfill the assistant row id (29-01 contract)")
			return msgs[i].ID
		}
	}
	t.Fatal("session has no assistant row")
	return 0
}

// TestVerdictPipeline_DataTurnVerdictFrameOrder — the verdict path
// emits data-turn AFTER the counter-evidence annotation frame and
// BEFORE text-end / finish-step / finish, carrying the assistant row id
// writeAuditAndTurn just persisted.
func TestVerdictPipeline_DataTurnVerdictFrameOrder(t *testing.T) {
	const sessID = "sess-29-04-verdict-turn"
	deps := newHistoryAwareDeps(t, sessID, false)
	deps.Retriever = &stubRetriever{chunks: emitChunks()}
	// Wire the registry so the post-verdict counter-evidence pass emits
	// its data-decision annotation — the frame data-turn must FOLLOW.
	deps.KBRegistry = testKBReg()

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatRaw(t, context.Background(), addr, sessID, "年假有几天")
	require.NoError(t, err)
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	turnIdx := dataTurnIndexes(frames)
	require.Len(t, turnIdx, 1,
		"verdict stream must carry exactly one data-turn frame; sequence=%v", frameTypeList(frames))

	counterEvidence := indexOfChunkType(frames, "data-decision")
	textEnd := indexOfChunkType(frames, "text-end")
	finishStep := indexOfChunkType(frames, "finish-step")
	finish := indexOfChunkType(frames, "finish")
	require.Greater(t, counterEvidence, -1,
		"fixture must produce a counter-evidence annotation frame; sequence=%v", frameTypeList(frames))
	require.Greater(t, textEnd, -1, "verdict stream must reach text-end; sequence=%v", frameTypeList(frames))
	require.Greater(t, finish, -1, "verdict stream must reach finish; sequence=%v", frameTypeList(frames))

	assert.Greater(t, turnIdx[0], counterEvidence,
		"data-turn MUST follow the post-verdict counter-evidence annotation (behavior matrix)")
	assert.Less(t, turnIdx[0], textEnd,
		"data-turn MUST precede text-end (T-29-06 frame-order window)")
	assert.Less(t, turnIdx[0], finishStep,
		"data-turn MUST precede finish-step")
	assert.Less(t, turnIdx[0], finish,
		"data-turn MUST precede finish — useChat registers the message at finish")

	// turnId == the persisted assistant row id.
	assert.Equal(t, assistantRowID(t, deps.Store, sessID), soleTurnID(t, frames),
		"data-turn turnId MUST equal the assistant row id writeAuditAndTurn persisted")
}

// TestVerdictPipeline_DataTurnRefusalShapes — the four reachable
// pipeline refusal short-circuits all emit data-turn BEFORE the refusal
// stream's text-start (emitRefusal writes text-start AND finish
// internally, so "before text-start" is the strictest honest lock) and
// carry the persisted assistant row id.
func TestVerdictPipeline_DataTurnRefusalShapes(t *testing.T) {
	cases := []struct {
		name string
		want string
		mut  func(t *testing.T, deps *Deps)
	}{
		{
			name: "no_authorized_source",
			want: verdict.RefusalReasonNoAuthorizedSource,
			mut: func(t *testing.T, deps *Deps) {
				t.Helper()
				// Default stub retriever returns zero chunks → SC-2.
			},
		},
		{
			name: "low_retrieval_score",
			want: verdict.RefusalReasonLowRetrievalScore,
			mut: func(t *testing.T, deps *Deps) {
				t.Helper()
				low := emitChunks()
				for i := range low {
					low[i].Score = 0.05 // < retrievalScoreThreshold (0.3)
				}
				deps.Retriever = &stubRetriever{chunks: low}
			},
		},
		{
			name: "classifier_refuse",
			want: verdict.RefusalReasonOutOfScope,
			mut: func(t *testing.T, deps *Deps) {
				t.Helper()
				deps.Retriever = &stubRetriever{chunks: emitChunks()}
				classifierLM := &historyTestStubLM{canned: `{"decision": "refuse", "reason": "out_of_scope", "confidence": 0.92}`}
				deps.RefusalClassifier, _ = classifier.NewRefusalClassifier(classifierLM)
			},
		},
		{
			name: "status_refused",
			want: verdict.RefusalReasonNoAuthorizedSource,
			mut: func(t *testing.T, deps *Deps) {
				t.Helper()
				deps.Retriever = &stubRetriever{chunks: emitChunks()}
				// The LLM itself refuses → Step 10b maps it to the
				// structured refusal shape.
				deps.PipelineChatModel = &stubStreamingChatModel{
					tokens: []string{`{"status":"refused","confidence":0.0,"citations":[],"review_required":true}`},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sessID := "sess-29-04-refuse-" + tc.name
			deps := newHistoryAwareDeps(t, sessID, false)
			tc.mut(t, deps)

			addr, cleanup := newTestHertz(t, deps)
			defer cleanup()

			resp, err := postChatRaw(t, context.Background(), addr, sessID, "那第一条呢")
			require.NoError(t, err)
			body := drainBody(t, resp)

			frames := parseFrames(t, body)
			r := findRefusal(t, frames)
			assert.Equal(t, tc.want, r.Reason, "fixture must produce the expected refusal reason")

			turnIdx := dataTurnIndexes(frames)
			require.Len(t, turnIdx, 1,
				"refusal stream (%s) must carry exactly one data-turn frame; sequence=%v", tc.name, frameTypeList(frames))
			textStart := indexOfChunkType(frames, "text-start")
			finish := indexOfChunkType(frames, "finish")
			require.Greater(t, textStart, -1, "refusal stream must have text-start")
			require.Greater(t, finish, -1, "refusal stream must reach finish")
			assert.Less(t, turnIdx[0], textStart,
				"data-turn MUST precede emitRefusal's text-start (the refusal terminal frames live inside emitRefusal)")
			assert.Less(t, turnIdx[0], finish,
				"data-turn MUST precede finish")
			assert.Equal(t, assistantRowID(t, deps.Store, sessID), soleTurnID(t, frames),
				"data-turn turnId MUST equal the assistant row id (refusal rounds are votable too — Open Q3)")
		})
	}
}

// TestVerdictPipeline_DataTurnOutOfScopeEmptyTiers — the defensive
// empty-resolved-tiers branch (pipeline Step 1). The Loader can never
// produce it (Validate rejects empty reads), so the test invokes
// VerdictPipeline directly with a zero Manifest through a real listener.
func TestVerdictPipeline_DataTurnOutOfScopeEmptyTiers(t *testing.T) {
	const sessID = "sess-29-04-oos-empty-tiers"
	deps := newVerdictPipelineDeps(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	h := server.Default(server.WithListener(ln))
	h.POST("/api/chat", func(ctx context.Context, c *app.RequestContext) {
		_ = deps.VerdictPipeline(ctx, c, "年假有几天", &skill.Manifest{}, sessID, "compliance")
	})
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	cleanup := func() { _ = h.Close(); <-errCh }
	defer cleanup()

	addr := "127.0.0.1:" + portStr(ln.Addr().(*net.TCPAddr).Port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "server warmup")

	resp, err := postChatRaw(t, context.Background(), addr, sessID, "年假有几天")
	require.NoError(t, err)
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	r := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, r.Reason,
		"empty-tiers branch must refuse out_of_scope")
	turnIdx := dataTurnIndexes(frames)
	require.Len(t, turnIdx, 1, "sequence=%v", frameTypeList(frames))
	textStart := indexOfChunkType(frames, "text-start")
	assert.Less(t, turnIdx[0], textStart,
		"data-turn MUST precede emitRefusal's text-start on the out_of_scope shape")
	assert.Equal(t, assistantRowID(t, deps.Store, sessID), soleTurnID(t, frames))
}

// TestVerdictPipeline_DataTurnZeroIDSendsNothing — turnID==0 (nil
// Store → writeAuditAndTurn returns (0, nil)) suppresses the frame
// entirely; the rest of the verdict frame sequence is unchanged. The FE
// renders its disabled feedback state instead of wiring a vote to a
// fabricated id (T-29-06).
func TestVerdictPipeline_DataTurnZeroIDSendsNothing(t *testing.T) {
	const sessID = "sess-29-04-zero-turn-id"
	deps := newVerdictPipelineDeps(t)
	deps.Store = nil // degraded: writeAuditAndTurn returns (0, nil)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	deps.Retriever = &stubRetriever{chunks: []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}}
	deps.Runner = newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95, []string{"1"}, chunkText, "权威"))

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	resp, err := postChatRaw(t, context.Background(), addr, sessID, "年假有几天")
	require.NoError(t, err)
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	assert.Empty(t, dataTurnIndexes(frames),
		"turnID==0 must emit ZERO data-turn frames; sequence=%v", frameTypeList(frames))
	require.NotNil(t, mustVerdict(t, frames), "verdict must still stream")
	textEnd := indexOfChunkType(frames, "text-end")
	finish := indexOfChunkType(frames, "finish")
	require.Greater(t, textEnd, -1, "verdict lifecycle unchanged: text-end present")
	require.Greater(t, finish, -1, "verdict lifecycle unchanged: finish present")
	assert.Less(t, indexOfChunkType(frames, "data-policy"), textEnd,
		"rest of the frame sequence must be unchanged (data-policy before text-end)")
}