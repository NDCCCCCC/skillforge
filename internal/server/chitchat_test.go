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

// Regression tests for the chitchat gate (greeting-refusal-gate debug
// session, 2026-09-14 — see .planning/debug/greeting-refusal-gate.md).
//
// The pre-fix behavior: a pure greeting query ("你好") entered the
// VerdictPipeline as a knowledge query, paid embed + retrieve + (on the
// live build) a full LLM refusal-classifier round-trip, and terminated in
// one of the five refusal gates — the user saw the 拒答 card with the
// feedback row for a friendly hello. The gate added at the pipeline entry
// must:
//
//  1. Match greeting queries WHOLE-QUERY only (never a substring match —
//     "你好，年假有几天" is a knowledge query; "history" is not "hi").
//  2. Short-circuit BEFORE embed/retrieve/LLM (stubRetriever.CallCount==0,
//     stubRunner.CallCount==0) — zero retrieval cost, zero LLM latency.
//  3. Emit a plain text reply with NO data-policy frame (no refusal card,
//     no verdict card) and NO refusal text ("无法给出结论…").
//  4. Persist exactly one audit row with refusal_reason="chitchat" and
//     one (user, assistant) session turn whose assistant content is the
//     reply text — so the session-restore plain-prose branch
//     (handlers_session_history.go renderRow) renders it verbatim instead
//     of RenderRefusal's generic refusal sentence.
//  5. NOT swallow knowledge queries: a greeting-prefixed knowledge query
//     still reaches the retriever.

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
)

// TestIsChitchatQuery — the detector is WHOLE-QUERY, normalization-first
// (trim whitespace + CJK/ASCII punctuation, case-fold Latin). Every true
// case is a pure greeting; every false case is a real or greeting-prefixed
// knowledge query that MUST still reach retrieval.
func TestIsChitchatQuery(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  bool
	}{
		{"zh hello", "你好", true},
		{"zh polite hello", "您好", true},
		{"zh hello variant", "你好呀", true},
		{"full-width punctuation", "你好！", true},
		{"surrounding whitespace", "  你好  ", true},
		{"morning greeting", "早上好", true},
		{"presence check", "在吗", true},
		{"thanks", "谢谢", true},
		{"goodbye", "再见", true},
		{"latin case-folded", "Hello", true},
		{"latin short", "hi", true},
		// False cases — the overreach guards.
		{"greeting-prefixed knowledge query", "你好，年假有几天", false},
		{"latin substring trap (history)", "history", false},
		{"latin with suffix", "hi there", false},
		{"plain knowledge query", "年假有几天", false},
		{"empty", "", false},
		{"punctuation only", "！？", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isChitchatQuery(tc.query),
				"isChitchatQuery(%q)", tc.query)
		})
	}
}

// TestVerdictPipelineDefault_ChitchatShortCircuits — the pipeline-level
// regression lock. A greeting query must short-circuit at the pipeline
// entry: no retrieval, no LLM, no data-policy frame, plain-text reply,
// audit row tagged refusal_reason="chitchat", session turn storing the
// reply text as assistant content.
func TestVerdictPipelineDefault_ChitchatShortCircuits(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	// Seed a chunk that WOULD be retrieved if the gate failed — the
	// CallCount assertions below are what prove the short-circuit.
	stubSrch := &stubRetriever{chunks: []retriever.Hit{
		{ID: 1, Tier: "权威", Content: "年假5天", Source: "authority/annual_leave.md", Score: 0.65},
	}}
	deps.Retriever = stubSrch
	stubRun := newStubRunner("")
	deps.Runner = stubRun

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-chitchat"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "你好")
	require.NoError(t, err, "POST /api/chat (chitchat path)")
	body := drainBody(t, resp)

	// (a) Plain-text reply — a text-delta carrying the greeting, and
	// absolutely no refusal text.
	frames := parseFrames(t, body)
	var gotText bool
	for _, f := range frames {
		assert.NotEqual(t, "data-policy", f.Type,
			"chitchat reply must NOT carry a data-policy frame (no refusal card, no verdict card)")
		assert.NotEqual(t, "data-subprogress", f.Type,
			"chitchat short-circuits BEFORE retrieval: no retrieve subprogress frame")
		if f.Type == "text-delta" {
			gotText = true
			assert.Contains(t, f.Delta, "你好",
				"reply text must be a friendly greeting")
			assert.NotContains(t, f.Delta, "无法给出结论",
				"reply text must NOT be refusal copy")
		}
	}
	assert.True(t, gotText, "chitchat path must emit a text-delta reply frame")

	// (b) Zero retrieval, zero LLM (the gate precedes embed/retrieve).
	assert.Equal(t, 0, stubSrch.CallCount(),
		"chitchat gate must short-circuit BEFORE the retriever")
	assert.Equal(t, 0, stubRun.CallCount(),
		"chitchat gate must short-circuit BEFORE the LLM runner")

	// (c) Exactly one audit row, tagged with the chitchat reason.
	rows, err := deps.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessID})
	require.NoError(t, err, "QueryAudit by session")
	require.Len(t, rows, 1, "GEVL-03: exactly one audit row per query")
	assert.Equal(t, refusalReasonChitchat, rows[0].RefusalReason,
		"chitchat path audit row carries the chitchat reason (distinguishes greeting turns in the audit trail)")
	assert.Empty(t, rows[0].VerdictJSON, "chitchat path → verdict_json empty")

	// (d) Session turn appended, assistant content = reply text (NOT a
	// refusal code) so the restore handler's plain-prose branch renders
	// it verbatim.
	msgs, err := deps.Store.GetSession(ctx, sessID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "one (user, assistant) turn")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "你好", msgs[0].Content)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, chitchatReplyText, msgs[1].Content,
		"assistant turn stores the reply text (restore renders it verbatim, not as a refusal)")
}

// TestVerdictPipelineDefault_GreetingPrefixedKnowledgeQueryStillRetrieves —
// the overreach guard at the pipeline level. "你好，年假有几天" is a
// knowledge query with a greeting prefix: it MUST NOT be swallowed by the
// chitchat gate and MUST reach the retriever.
func TestVerdictPipelineDefault_GreetingPrefixedKnowledgeQueryStillRetrieves(t *testing.T) {
	deps := newVerdictPipelineDeps(t)
	stubSrch := &stubRetriever{chunks: []retriever.Hit{
		{ID: 1, Tier: "权威", Content: "年假5天", Source: "authority/annual_leave.md", Score: 0.65},
	}}
	deps.Retriever = stubSrch

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	const sessID = "sess-vp-chitchat-neg"
	ctx := context.Background()
	resp, err := postChatRaw(t, ctx, addr, sessID, "你好，年假有几天")
	require.NoError(t, err)
	_ = drainBody(t, resp)

	assert.Equal(t, 1, stubSrch.CallCount(),
		"a greeting-prefixed knowledge query must NOT be swallowed by the chitchat gate")
}
