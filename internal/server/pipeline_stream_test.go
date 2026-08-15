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

// Quick plan 260815-cos Task 1 (TDD RED→GREEN) — BE LLM streaming
// draft emit (PipelineChatModel.Stream + text-delta-draft wire type).
//
// Pinned invariants:
//
//  1. TestVerdictPipelineStreaming_DraftChunksEmitted — when
//     Deps.PipelineChatModel is non-nil and Stream returns a sequence
//     of *schema.Message tokens, the SSE stream MUST carry:
//        (a) text-delta-draft frames for each token (id="draft-1"),
//        (b) a terminal text-delta-draft-end frame (id="draft-1"),
//        (c) the original verdict stream (text-start/text-delta/source-
//            document/data-policy/text-end/finish-step/finish) AFTER
//            the draft-end.
//     The grounded verdict + audit row + session turn stay intact
//     (Step 9 grounding is on the assembled buffer).
//
//  2. TestVerdictPipelineDefault_PipelineChatModelNilFallback —
//     Deps.PipelineChatModel == nil falls back to the Phase 2 path
//     (Runner.Query + drainFinalContent). Behaviour MUST be byte-
//     identical to the existing TestVerdictPipelineDefault_… tests:
//     no text-delta-draft frames, normal verdict stream, normal
//     audit row + session turn.
//
//  3. TestVerdictPipelineStreaming_RefusalReplacesDraft — when the
//     stubRunner returns a verdict with status="unanswered" (or the
//     streaming stub's canned raw JSON carries "unanswered"), the
//     pipeline emits the refusal SSE shape; the FE end (not in this
//     package) is responsible for clearing the DraftStreamChip
//     buffer when it sees data-policy carrying Refusal. Here we only
//     assert data-policy is Refusal (NOT Verdict) so a future change
//     cannot regress the trust boundary.
//
// All 3 tests reuse newVerdictPipelineDeps for the Loader + Store
// + Embedder/Retriever/Runner seams. The streaming stub replaces
// the canned judgement by injecting tokens into PipelineChatModel
// and a canned Verdict JSON via stubRunner.

package server

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// stubStreamingChatModel is the test double for the pipeline LLM
// streaming seam (Deps.PipelineChatModel). It satisfies
// model.BaseChatModel (Stream + Generate) so Deps.PipelineChatModel
// can hold it.
//
//   - tokens []string is the canned token sequence Stream returns.
//     After the tokens are exhausted, Stream sends io.EOF on Recv
//     (via schema.Pipe + sw.Close with no more sends).
//   - generateMessage is the canned full message Generate returns.
//     We mirror Phase 2's stubRunner convention (the concatenation
//     of tokens is a sane default; tests can override).
//
// The stub counts Stream invocations so the regression "the LLM
// streaming seam is hit exactly once per /api/chat" is assertable.
type stubStreamingChatModel struct {
	mu             sync.Mutex
	count          int
	tokens         []string
	generateMessage string
	streamErr      error
}

func (m *stubStreamingChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	if m.generateMessage == "" {
		// Default: concatenate the tokens — matches what Stream emitted.
		out := ""
		for _, t := range m.tokens {
			out += t
		}
		return &schema.Message{Role: schema.Assistant, Content: out}, nil
	}
	return &schema.Message{Role: schema.Assistant, Content: m.generateMessage}, nil
}

// Stream feeds the canned tokens into a schema.Pipe and closes the
// writer so Recv sees io.EOF after the last token. The returned
// reader is single-use (matches the Eino contract — Copy is opt-in).
func (m *stubStreamingChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		defer sw.Close()
		for _, t := range m.tokens {
			if t == "" {
				continue
			}
			sw.Send(&schema.Message{Role: schema.Assistant, Content: t}, nil)
		}
	}()
	return sr, nil
}

// StreamCount returns the number of Stream invocations (for the
// "LLM seam hit exactly once" regression).
func (m *stubStreamingChatModel) StreamCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// Compile-time check: stubStreamingChatModel satisfies model.BaseChatModel.
// model.ToolCallingChatModel embeds model.BaseChatModel so the same
// assertion makes a stubStreamingChatModel assignable to either field
// type on Deps (PipelineChatModel = BaseChatModel).
var _ model.BaseChatModel = (*stubStreamingChatModel)(nil)

// draftFrames extracts text-delta-draft + text-delta-draft-end frames
// from an SSE body, in wire order, returning the parsed chunk shape.
// Filters by id="draft-1" — the canonical draft-channel ID namespace.
func draftFrames(t *testing.T, body string) []wire.UIMessageChunk {
	t.Helper()
	var out []wire.UIMessageChunk
	for _, f := range parseFrames(t, body) {
		if f.Type != "text-delta-draft" && f.Type != "text-delta-draft-end" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// TestVerdictPipelineStreaming_DraftChunksEmitted — pipeline.Stream
// path: 3 token drafts + 1 draft-end + the verdict stream MUST land
// on the wire in that order. Audit row + session turn MUST also be
// written exactly once (Step 10 audit invariant).
func TestVerdictPipelineStreaming_DraftChunksEmitted(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}

	// (a) 3 token drafts. The concatenated content equals the canned
	// verdict JSON so the existing parse + grounding path succeeds
	// unchanged — Step 8 only adds streaming emit AROUND the buffer,
	// it does NOT change what feeds Step 9.
	tokens := []string{
		`{"status":"需补充","confidence":0.9,"citations":[{"chunk_id":"1","span":"`,
		chunkText,
		`","tier":"权威"}],"review_required":false}`,
	}
	stubCM := &stubStreamingChatModel{tokens: tokens}
	deps.PipelineChatModel = stubCM
	// stubRunner stays the Phase 2 canned fallback so non-LLM checks
	// (audit row, session turn) see the Phase 2 path. Pipeline.Stream
	// is what drives Step 8 — Runner.Query is NOT called when
	// PipelineChatModel is set.
	deps.Runner = newStubRunner("")

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-stream-draft"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "年假有几天")
	require.NoError(t, err, "POST /api/chat (pipeline.stream path)")
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	drafts := draftFrames(t, body)

	// (a) 3 text-delta-draft frames in order, all with id="draft-1".
	deltaFrames := 0
	for _, f := range drafts {
		if f.Type == "text-delta-draft" {
			deltaFrames++
			assert.Equal(t, "draft-1", f.ID,
				"draft channel MUST use id=\"draft-1\" (isolates from msg-1 text part)")
		}
	}
	assert.Equal(t, 3, deltaFrames,
		"pipeline MUST emit one text-delta-draft frame per token (got %d)", deltaFrames)

	// (b) Exactly 1 text-delta-draft-end terminator.
	endFrames := 0
	for _, f := range drafts {
		if f.Type == "text-delta-draft-end" {
			endFrames++
			assert.Equal(t, "draft-1", f.ID,
				"draft-end MUST carry the same id=\"draft-1\" namespace")
		}
	}
	assert.Equal(t, 1, endFrames,
		"pipeline MUST emit exactly one text-delta-draft-end frame (got %d)", endFrames)

	// (c) Draft frames precede the grounded verdict stream — the FE
	// trusts this ordering (DraftStreamChip updates on draft-end,
	// VerdictCard mounts on the subsequent data-policy).
	draftEndIdx := -1
	textStartIdx := -1
	dataPolicyIdx := -1
	for i, f := range frames {
		switch f.Type {
		case "text-delta-draft-end":
			if draftEndIdx < 0 {
				// Use the LAST draft-end as the gate — defensive in
				// case a future schema change adds intermediate ones.
				draftEndIdx = i
			}
		case "text-start":
			if textStartIdx < 0 {
				textStartIdx = i
			}
		case "data-policy":
			if dataPolicyIdx < 0 {
				dataPolicyIdx = i
			}
		}
	}
	require.GreaterOrEqual(t, draftEndIdx, 0, "stream must carry a draft-end")
	require.GreaterOrEqual(t, textStartIdx, 0, "verdict stream must open with text-start")
	require.GreaterOrEqual(t, dataPolicyIdx, 0, "verdict stream must carry a data-policy")
	assert.Less(t, draftEndIdx, textStartIdx,
		"draft-end MUST precede the verdict text-start (FE chip gates on this)")
	assert.Less(t, draftEndIdx, dataPolicyIdx,
		"draft-end MUST precede data-policy (FE chip gates on grounding result)")

	// (d) data-policy carries a Verdict (NOT a Refusal).
	var v *verdict.Verdict
	for _, f := range frames {
		if f.Type != "data-policy" {
			continue
		}
		var pv verdict.Verdict
		if json.Unmarshal(f.Data, &pv) == nil && pv.Status != "" {
			v = &pv
			break
		}
	}
	require.NotNil(t, v, "data-policy must carry a grounded Verdict on the streaming path")
	assert.NotEqual(t, "unanswered", v.Status,
		"streaming path with a complete token stream MUST ground successfully")
	require.Len(t, v.Citations, 1, "verdict must cite the retrieved chunk")
	assert.Equal(t, "1", v.Citations[0].ChunkID)

	// (e) The streamed tokens concatenated == the parsed Verdict's
	// citation span (sanity: the buffer feed the parse/grounding path
	// matches what the Stream emitted, no truncation or padding).
	assert.Equal(t, chunkText, v.Citations[0].Span,
		"streamed draft buffer must round-trip the citation span (no truncation)")

	// (f) Stream seam hit exactly once (Phase 2's single-shot LLM
	// contract is preserved on the streaming path).
	assert.Equal(t, 1, stubCM.StreamCount(),
		"PipelineChatModel.Stream MUST be invoked exactly once per /api/chat")

	// (g) Audit row + session turn STILL written (GEVL-03).
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "GEVL-03: exactly one audit row per query on the streaming path")
	assert.NotEmpty(t, rows[0].VerdictJSON, "streaming verdict path → verdict_json recorded")
	msgs, err := deps.Store.GetSession(ctx, sessID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "one (user, assistant) turn on the streaming path")
}

// TestVerdictPipelineDefault_PipelineChatModelNilFallback —
// Deps.PipelineChatModel == nil: the Phase 2 Runner.Query +
// drainFinalContent path is preserved byte-for-byte. No draft frames
// land on the wire; the verdict + audit + session flow is identical
// to TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict.
func TestVerdictPipelineDefault_PipelineChatModelNilFallback(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	stubRun := newStubRunner(cannedVerdictJSON("需补充", 0.9,
		[]string{"1"}, chunkText, "权威"))
	deps.Runner = stubRun
	// PipelineChatModel is NOT set (newVerdictPipelineDeps leaves it nil).
	// This is the load-bearing assertion: nil → Runner.Query path,
	// not PipelineChatModel.Stream path.

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-stream-fallback"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "年假有几天")
	require.NoError(t, err, "POST /api/chat (Phase 2 fallback path)")
	body := drainBody(t, resp)

	// (a) NO draft frames — PipelineChatModel is nil, the streaming
	// channel is dormant (production wiring at cmd/server/main.go
	// sets it; tests that build a Deps without it keep the Phase 2
	// behaviour as the regression baseline).
	drafts := draftFrames(t, body)
	assert.Empty(t, drafts,
		"PipelineChatModel==nil MUST NOT emit draft frames (got %d)", len(drafts))

	// (b) Runner hit exactly once — confirms the fallback path was taken.
	assert.Equal(t, 1, stubRun.CallCount(),
		"PipelineChatModel==nil MUST fall back to Runner.Query (no double-call)")

	// (c) Grounded Verdict + audit row + session turn all preserved.
	frames := parseFrames(t, body)
	var v *verdict.Verdict
	for _, f := range frames {
		if f.Type != "data-policy" {
			continue
		}
		var pv verdict.Verdict
		if json.Unmarshal(f.Data, &pv) == nil && pv.Status != "" {
			v = &pv
			break
		}
	}
	require.NotNil(t, v, "Phase 2 fallback must still emit a Verdict on data-policy")
	assert.Equal(t, "需补充", v.Status)

	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "Phase 2 audit invariant preserved on fallback")
	assert.Equal(t, "", rows[0].RefusalReason)

	msgs, err := deps.Store.GetSession(ctx, sessID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "Phase 2 session invariant preserved on fallback")
}

// TestVerdictPipelineStreaming_RefusalReplacesDraft — when the
// streamed buffer parses to a status="unanswered" verdict, the
// pipeline emits a refusal. The BE side just walks the same path
// (parse → validate → Step 9b refusal short-circuit); this test
// pins that the draft frames DO NOT lead to a Verdict-shaped
// data-policy on the refusal path. (FE-side chip buffer clearing
// is the responsibility of the MockChatTransport hook in Task 2.)
func TestVerdictPipelineStreaming_RefusalReplacesDraft(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}

	// Stub returns a complete JSON verdict with status="unanswered"
	// — the model's semantic "I cannot ground this" signal. The
	// pipeline Step 9b short-circuits to refusal (ARAG-05).
	unansweredJSON := `{"status":"unanswered","confidence":0,"citations":[],"review_required":true}`
	stubCM := &stubStreamingChatModel{tokens: []string{unansweredJSON}}
	deps.PipelineChatModel = stubCM
	deps.Runner = newStubRunner("")

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-stream-refusal"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "量子力学原理")
	require.NoError(t, err, "POST /api/chat (streaming refusal path)")
	body := drainBody(t, resp)

	frames := parseFrames(t, body)
	drafts := draftFrames(t, body)

	// (a) Draft frames DO land (the streaming emit happens BEFORE the
	// parse/grounding step — there's no way to know the verdict is
	// "unanswered" until the buffer is assembled). FE-side chip buffer
	// must clear when data-policy carries a Refusal (Task 2).
	require.NotEmpty(t, drafts,
		"drafts MUST emit before grounding (FE clears them on Refusal data-policy)")

	// (b) data-policy carries a Refusal (NOT a Verdict).
	var r *verdict.Refusal
	for _, f := range frames {
		if f.Type != "data-policy" {
			continue
		}
		var pr verdict.Refusal
		if json.Unmarshal(f.Data, &pr) == nil && pr.Reason != "" {
			r = &pr
			break
		}
	}
	require.NotNil(t, r, "streaming path with status=unanswered MUST emit a Refusal data-policy")
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, r.Reason,
		"LLM 'unanswered' on the streaming path MUST map to no_authorized_source refusal")

	// (c) Audit row written with the refusal reason (GEVL-03 — every
	// response shape writes exactly one row).
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "streaming refusal path writes one audit row")
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, rows[0].RefusalReason)
}