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

// Regression tests for the VerdictPipeline-as-default /api/chat path.
//
// These tests pin the two load-bearing invariants of the production
// default after the agent-llm-query-rewrite debug session (see
// .planning/debug/agent-llm-query-rewrite.md):
//
//  1. A relevant query (original phrasing hits the correct doc in the
//     stubbed retriever) MUST produce a grounded, cited verdict AND
//     write exactly one audit row + one (user, assistant) session turn.
//     This is the regression lock for "年假有几天 must answer, not refuse"
//     AND for the audit-persistence fix (commit 1cef0f3) that made
//     VerdictPipeline call writeAuditAndTurn.
//
//  2. An out-of-scope / low-retrieval-score query MUST still refuse
//     (refusal moat intact on VerdictPipeline) AND write an audit row
//     carrying the refusal_reason. This is the lock for "the refusal
//     floor is not silently bypassed when we switched the default back
//     from the agent path".
//
// The Deps constructed here mirror the production default: Agent == nil
// (so chatHandler takes the VerdictPipeline branch), Runner set to a
// stubRunner with a canned verdict, Retriever set to a stubRetriever
// seeded with the chunk the original query should hit, Store set to a
// real in-memory SQLite. The real on-disk skills/compliance/SKILL.md is
// loaded so findSkill("compliance") returns a manifest (needed for the
// audit row's prompt_hash via renderedPromptBody → agent.RenderSystemPrompt).

package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// newVerdictPipelineDeps builds a Deps wired for the VerdictPipeline
// default path: Agent nil, stub Runner/Retriever/Embedder, real
// in-memory SQLite Store, and the real on-disk compliance Skill loaded.
// The caller customizes the stubRetriever's chunks + the stubRunner's
// canned verdict after construction.
func newVerdictPipelineDeps(t *testing.T) *Deps {
	t.Helper()
	loader := skill.NewLoader(skillsDirPath(), testKBReg())
	_, err := loader.Scan()
	require.NoError(t, err, "load real skills/compliance/SKILL.md")
	return &Deps{
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		Runner:    newStubRunner(""),
		Store:     newTestStore(t),
		Loader:    loader,
	}
}

// TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict — the
// regression lock for "relevant query → grounded cited verdict + audit
// row" on the production default path. The stub retriever is seeded
// with the annual_leave chunk at score 0.65 (the live probe's measured
// relevance for the original query phrasing), and the stub runner
// returns a verdict citing that chunk with a verbatim span (so the
// grounding checker leaves it alone). After POST /api/chat:
//
//   - The SSE stream carries a data-policy Verdict with status != unanswered
//     and a non-empty citation.
//   - Exactly one audit row is written for the session, with
//     refusal_reason="", verdict_json non-empty, and chunk_ids carrying
//     the cited chunk id.
//   - A (user, assistant) session turn is appended (SC-9 / AGNT-05).
//   - The runner is called exactly once (the LLM emits the verdict; no
//     agent ReAct loop, no query rewrite).
func TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天；已满10年不满20年的，年假10天。"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	stubRun := newStubRunner(cannedVerdictJSON(verdict.DowngradeStatus, 0.9,
		[]string{"1"}, chunkText, "权威"))
	deps.Runner = stubRun

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-verdict"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "年假有几天")
	require.NoError(t, err, "POST /api/chat (verdict path)")
	body := drainBody(t, resp)

	// (a) SSE stream carries a cited verdict (NOT unanswered).
	frames := parseFrames(t, body)
	var v *verdict.Verdict
	for _, f := range frames {
		if f.Type != "data-policy" {
			continue
		}
		var pv verdict.Verdict
		// A data-policy frame may carry either a Verdict or a Refusal;
		// skip frames that do not unmarshal into a Verdict with a Status.
		if json.Unmarshal(f.Data, &pv) == nil && pv.Status != "" {
			v = &pv
		}
	}
	require.NotNil(t, v, "expected a data-policy Verdict frame on the verdict path")
	assert.NotEqual(t, "unanswered", v.Status,
		"regression: 年假 query must answer (previously refused because the agent path rewrote the query)")
	require.Len(t, v.Citations, 1, "verdict must cite the retrieved annual_leave chunk")
	assert.Equal(t, "1", v.Citations[0].ChunkID)
	assert.Equal(t, "权威", v.Citations[0].Tier)

	// (b) Exactly one audit row for the session, refusal_reason="", verdict recorded.
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err, "QueryAudit by session")
	require.Len(t, rows, 1, "GEVL-03: exactly one audit row per query")
	assert.Equal(t, "", rows[0].RefusalReason, "verdict path → refusal_reason empty")
	assert.NotEmpty(t, rows[0].VerdictJSON, "verdict path → verdict_json recorded")
	assert.Contains(t, rows[0].ChunkIDs, "1", "audit row carries the cited chunk id")
	assert.Equal(t, sessID, rows[0].SessionID)
	assert.Equal(t, "compliance", rows[0].SkillID)

	// (c) Session turn appended (SC-9 / AGNT-05).
	msgs, err := deps.Store.GetSession(ctx, sessID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "one (user, assistant) turn")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "年假有几天", msgs[0].Content)
	assert.Equal(t, "assistant", msgs[1].Role)

	// (d) VerdictPipeline invoked the runner exactly once (LLM emits the
	// verdict JSON; no ReAct loop, no query rewrite). This is the
	// structural contrast with runAgentPath, which rewrites the query.
	assert.Equal(t, 1, stubRun.CallCount(),
		"VerdictPipeline must call the runner exactly once (code-forced retrieval → single LLM verdict emit)")
}

// TestVerdictPipelineDefault_RefusalPathPersistsAudit — the regression
// lock for "out-of-scope / low-score query → refusal + audit row with
// refusal_reason" on the production default path. The stub retriever
// returns a chunk below the retrievalScoreThreshold (0.3), simulating
// the live probe's measured 0.24 relevance for an unrelated query. The
// pipeline MUST refuse by construction (no LLM call) AND write an audit
// row carrying refusal_reason="low_retrieval_score".
//
// This is the refusal-moat half of the debug session: switching the
// default back to VerdictPipeline must NOT silently answer off-scope
// queries (the failure mode the tweaked agent system prompt introduced).
func TestVerdictPipelineDefault_RefusalPathPersistsAudit(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	// Score 0.24 mirrors the live probe's measured relevance for an
	// unrelated query (量子力学 → 0.2397 against the strongest chunk).
	// Below the 0.3 threshold → low_retrieval_score refusal, NO LLM call.
	chunks := []retriever.Hit{
		{ID: 7, Tier: "权威", Content: "不相关条文", Source: "authority/unrelated.md", Score: 0.24},
	}
	stubSrch := &stubRetriever{chunks: chunks}
	deps.Retriever = stubSrch
	stubRun := newStubRunner("")
	deps.Runner = stubRun

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-refusal"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "量子力学原理")
	require.NoError(t, err, "POST /api/chat (refusal path)")
	body := drainBody(t, resp)

	// (a) Refusal emitted (moat intact).
	frames := parseFrames(t, body)
	r := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonLowRetrievalScore, r.Reason,
		"refusal moat: max score < threshold → low_retrieval_score (NOT an answered verdict)")
	assert.True(t, r.CanRephrase)

	// (b) No LLM call (refusal-by-construction).
	assert.Equal(t, 0, stubRun.CallCount(),
		"refusal-by-construction: low-score refusal MUST NOT invoke the LLM")
	assert.Equal(t, 1, stubSrch.CallCount(),
		"the retriever IS invoked once (code-forced original-query retrieval)")

	// (c) Audit row written with refusal_reason.
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "refusal path also writes exactly one audit row (GEVL-03)")
	assert.Equal(t, verdict.RefusalReasonLowRetrievalScore, rows[0].RefusalReason)
	assert.Empty(t, rows[0].VerdictJSON, "refusal path → verdict_json empty")
	assert.Empty(t, rows[0].ChunkIDs, "refusal path → no cited chunks")

	// (d) Session turn appended with the refusal reason as assistant content.
	msgs, err := deps.Store.GetSession(ctx, sessID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "量子力学原理", msgs[0].Content)
	assert.Equal(t, verdict.RefusalReasonLowRetrievalScore, msgs[1].Content,
		"assistant turn records the refusal reason")
}

// TestVerdictPipelineDefault_NoChunksRefusalPersistsAudit — companion
// to the low-score test. Zero chunks (the SC-2 / GEVL-02 refusal-by-
// construction path) must also write an audit row with refusal_reason
// ="no_authorized_source". This covers the second refusal shape on the
// default path.
func TestVerdictPipelineDefault_NoChunksRefusalPersistsAudit(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	deps.Retriever = &stubRetriever{chunks: nil}
	deps.Runner = newStubRunner("")

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-nochunks"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "查询员工联系方式")
	require.NoError(t, err)
	body := drainBody(t, resp)

	r := findRefusal(t, parseFrames(t, body))
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, r.Reason)

	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, rows[0].RefusalReason)
}

// TestVerdictPipeline_FinishStepIsLastAfterRedline — finish-step MUST be
// the final chunk in the SSE stream, emitted only AFTER every post-verdict
// annotation (counter-evidence, redline). Regression for the live Phase 7
// failure where finish-step was emitted inside emitVerdictStream (right
// after the verdict): the AI SDK v5 frontend stopped reading at finish-step,
// so the subsequent data-redline write hit a half-closed socket (wsasend
// connection-aborted) and the redline card never rendered.
func TestVerdictPipeline_FinishStepIsLastAfterRedline(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	chunkText := "员工累计工作已满1年不满10年的，年假5天。"
	deps.Retriever = &stubRetriever{chunks: []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/annual_leave.md", Score: 0.65},
	}}
	// 需补充 verdict trips the D5-14 redline gate (status ∈ {需补充, 不合规}).
	deps.Runner = newStubRunner(cannedVerdictJSON(verdict.DowngradeStatus, 0.9, []string{"1"}, chunkText, "权威"))
	deps.ChatModel = &stubChatModel{cannedJSON: `{"originalClause":"原条款","proposedRewrite":"建议改写","rationale":"修改理由"}`}

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, "sess-finish-order", "年假有几天")
	require.NoError(t, err, "POST /api/chat")
	frames := parseFrames(t, drainBody(t, resp))
	require.NotEmpty(t, frames, "stream must produce frames")

	// `finish` (the AI SDK v5 message-level terminator) MUST be the very
	// last frame, with finish-step just before it.
	assert.Equal(t, "finish", frames[len(frames)-1].Type,
		"finish must be the final chunk (message-level terminator)")
	assert.Equal(t, "finish-step", frames[len(frames)-2].Type,
		"finish-step must be the second-to-last chunk (step terminator)")

	// data-redline MUST precede both terminators (the bug had finish-step
	// inside emitVerdictStream, truncating the redline).
	redlineIdx, finishStepIdx := -1, -1
	for i, f := range frames {
		switch f.Type {
		case "data-redline":
			redlineIdx = i
		case "finish-step":
			finishStepIdx = i
		}
	}
	require.GreaterOrEqual(t, finishStepIdx, 0, "stream must contain a finish-step")
	if redlineIdx >= 0 {
		assert.Less(t, redlineIdx, finishStepIdx,
			"data-redline must precede finish-step; terminators close the stream")
	}
}
