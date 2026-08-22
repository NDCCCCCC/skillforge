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

	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// stubChatModel is the test double for the redline LLM seam. It counts
// Generate invocations and returns a canned JSON RedlinePayload string.
// The signature matches model.BaseChatModel.Generate (ctx, msgs,
// ...model.Option) so the stub satisfies model.ToolCallingChatModel at
// compile time (the assertion at the bottom of this file pins it).
type stubChatModel struct {
	mu          sync.Mutex
	count       int
	cannedJSON  string
	err         error
	capturedMsg []*schema.Message
}

func (m *stubChatModel) Generate(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.capturedMsg = msgs
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: m.cannedJSON,
	}, nil
}

// Stream is required by model.BaseChatModel. The redline pass never
// streams (it calls Generate only); this stub returns a nil reader +
// the same canned content so the interface assertion compiles.
func (m *stubChatModel) Stream(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

// callCount reads count under the stub's mutex. Generate runs on the
// Hertz handler goroutine while assertions run on the test goroutine —
// reading m.count directly is a data race under -race (surfaced when
// Phase 14 restored -race to the acceptance gate).
func (m *stubChatModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// WithTools is required by model.ToolCallingChatModel. The redline pass
// never binds tools (it's a pure structured-output call); this stub
// returns the receiver unchanged so the interface assertion compiles.
func (m *stubChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// Compile-time check: stubChatModel satisfies model.ToolCallingChatModel.
// Mirrors the pattern in internal/agent/critique.go line 240.
var _ model.ToolCallingChatModel = (*stubChatModel)(nil)

// runRedlinePass spins a Hertz server with a single POST route that
// invokes emitRedline directly with the supplied verdict + chunksByID.
// Returns the parsed SSE frames so the test can assert on the data-redline
// chunk shape.
func runRedlinePass(t *testing.T, deps *Deps, v verdict.Verdict, chunksByID map[string]string) []wire.UIMessageChunk {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.POST("/api/test/redline", func(ctx context.Context, c *app.RequestContext) {
		_ = deps.emitRedline(ctx, c, v, chunksByID)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	cleanup := func() { _ = h.Close(); <-errCh }

	addr := "127.0.0.1:" + portStr(port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "server warmup")
	defer cleanup()

	resp, err := http.Post("http://"+addr+"/api/test/redline", "application/json", strings.NewReader("{}"))
	require.NoError(t, err, "POST /api/test/redline")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")
	return parseSSEFramesLocal(t, string(body))
}

// findRedlineFrame scans frames for a data-redline chunk; returns
// (payload, frame, ok).
func findRedlineFrame(frames []wire.UIMessageChunk) (wire.RedlinePayload, wire.UIMessageChunk, bool) {
	for _, f := range frames {
		if f.Type != "data-redline" {
			continue
		}
		var p wire.RedlinePayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		return p, f, true
	}
	return wire.RedlinePayload{}, wire.UIMessageChunk{}, false
}

// TestRedline_Conditional — THE D5-14 COST-CONTROL REGRESSION.
//
// A verdict with ComplianceStatus="compliant" (or empty) and Status="ok"
// MUST:
//   - make ZERO ChatModel.Generate calls (the gate is the FIRST line of
//     emitRedline; a compliant verdict returns nil before any LLM work)
//   - emit NO data-redline chunk on the wire
//
// A verdict with ComplianceStatus=NonCompliantStatus ("non_compliant") MUST:
//   - make exactly 1 ChatModel.Generate call
//   - emit a data-redline chunk on the wire
//
// A verdict with Status=DowngradeStatus ("needs_input") MUST also fire the
// redline pass — D5-14 explicitly includes this platform status as a
// policy-gap signal.
//
// 260817-vsm: Compliance verdicts moved from Verdict.Status to
// Verdict.ComplianceStatus; the gate ORs Status against DowngradeStatus
// AND ComplianceStatus against NonCompliantStatus.
func TestRedline_Conditional(t *testing.T) {
	// Sub-case 1: compliant verdict → no LLM call, no chunk.
	t.Run("compliant_skips_LLM", func(t *testing.T) {
		stubCM := &stubChatModel{cannedJSON: `{"originalClause":"x","proposedRewrite":"y","rationale":"z"}`}
		deps := newTestDeps()
		deps.ChatModel = stubCM
		v := verdict.Verdict{
			Status:           "ok",
			Confidence:       0.95,
			Citations: []verdict.Citation{
				{ChunkID: "1", Span: "权威条款", Tier: "权威"},
			},
			ComplianceStatus: verdict.CompliantStatus,
		}
		frames := runRedlinePass(t, deps, v, map[string]string{"1": "权威条款"})

		assert.Equal(t, 0, stubCM.callCount(),
			"D5-14 cost control: compliant verdict MUST NOT call ChatModel.Generate")
		_, _, ok := findRedlineFrame(frames)
		assert.False(t, ok, "compliant verdict MUST NOT emit a data-redline chunk")
	})

	// Sub-case 2: non_compliant verdict → 1 LLM call, 1 chunk.
	t.Run("non_compliant_fires_LLM", func(t *testing.T) {
		stubCM := &stubChatModel{cannedJSON: `{"originalClause":"原条款","proposedRewrite":"建议改写","rationale":"修改理由"}`}
		deps := newTestDeps()
		deps.ChatModel = stubCM
		v := verdict.Verdict{
			Status:           "ok",
			Confidence:       0.85,
			Citations: []verdict.Citation{
				{ChunkID: "1", Span: "违规条款", Tier: "权威"},
			},
			ComplianceStatus: verdict.NonCompliantStatus,
		}
		frames := runRedlinePass(t, deps, v, map[string]string{"1": "违规条款"})

		assert.Equal(t, 1, stubCM.callCount(),
			"non_compliant verdict MUST call ChatModel.Generate exactly once")
		_, _, ok := findRedlineFrame(frames)
		assert.True(t, ok, "non_compliant verdict MUST emit a data-redline chunk")
	})

	// Sub-case 3: needs_input verdict → also fires (D5-14).
	t.Run("needs_info_fires_LLM", func(t *testing.T) {
		stubCM := &stubChatModel{cannedJSON: `{"originalClause":"原条款","proposedRewrite":"建议改写","rationale":"修改理由"}`}
		deps := newTestDeps()
		deps.ChatModel = stubCM
		v := verdict.Verdict{
			Status:     verdict.DowngradeStatus,
			Confidence: 0.6,
			Citations: []verdict.Citation{
				{ChunkID: "1", Span: "不完整条款", Tier: "权威"},
			},
		}
		frames := runRedlinePass(t, deps, v, map[string]string{"1": "不完整条款"})

		assert.Equal(t, 1, stubCM.callCount(),
			"needs_input verdict MUST call ChatModel.Generate exactly once (D5-14)")
		_, _, ok := findRedlineFrame(frames)
		assert.True(t, ok, "needs_input verdict MUST emit a data-redline chunk (D5-14)")
	})
}

// TestRedline_PayloadStructure — the data-redline chunk's Data field
// unmarshals into wire.RedlinePayload with all three required fields
// (OriginalClause / ProposedRewrite / Rationale) populated. The V8
// mitigation (RedlinePayload.UnmarshalJSON) already enforces required-
// field presence at the unmarshal boundary; this test additionally
// pins the round-trip via emit + parse so a future producer that drops
// a field is caught at the SSE layer.
func TestRedline_PayloadStructure(t *testing.T) {
	stubCM := &stubChatModel{cannedJSON: `{"originalClause":"原条款内容","proposedRewrite":"建议改写后的文本","rationale":"该条款与上位法不一致"}`}
	deps := newTestDeps()
	deps.ChatModel = stubCM
	v := verdict.Verdict{
		Status:           "ok",
		Confidence:       0.8,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "违规原文", Tier: "权威"},
		},
		ComplianceStatus: verdict.NonCompliantStatus,
	}

	frames := runRedlinePass(t, deps, v, map[string]string{"1": "违规原文"})
	payload, frame, ok := findRedlineFrame(frames)
	require.True(t, ok, "expected a data-redline chunk")
	assert.Equal(t, "data-redline", frame.Type,
		"wire type must be data-redline (the 18th type added in Plan 05-01)")

	// All three required fields populated.
	assert.Equal(t, "原条款内容", payload.OriginalClause)
	assert.Equal(t, "建议改写后的文本", payload.ProposedRewrite)
	assert.Equal(t, "该条款与上位法不一致", payload.Rationale)
	assert.Empty(t, payload.ClauseID, "clauseId is optional; absent on this payload")
}

// TestRedline_LLMErrorIsNonFatal — when ChatModel.Generate fails, the
// redline pass returns the error (caller logs and continues) and emits
// NO data-redline chunk. The verdict + counter-evidence stand.
func TestRedline_LLMErrorIsNonFatal(t *testing.T) {
	stubCM := &stubChatModel{err: assertErr("simulated LLM outage")}
	deps := newTestDeps()
	deps.ChatModel = stubCM
	v := verdict.Verdict{
		Status:     verdict.NonCompliantStatus,
		Confidence: 0.8,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "x", Tier: "权威"},
		},
	}

	frames := runRedlinePass(t, deps, v, map[string]string{"1": "x"})
	_, _, ok := findRedlineFrame(frames)
	assert.False(t, ok, "LLM error → no data-redline chunk (verdict stands)")
}

// TestRedline_LLMReturnsInvalidJSON — the LLM returns malformed JSON.
// emitRedline must surface this as a non-fatal error (no chunk emitted).
// V8 wire-smuggling mitigation lives at wire.RedlinePayload.UnmarshalJSON
// — this test exercises the parse path in emitRedline's own unmarshal.
func TestRedline_LLMReturnsInvalidJSON(t *testing.T) {
	stubCM := &stubChatModel{cannedJSON: `not valid json`}
	deps := newTestDeps()
	deps.ChatModel = stubCM
	v := verdict.Verdict{
		Status:     verdict.NonCompliantStatus,
		Confidence: 0.8,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "x", Tier: "权威"},
		},
	}

	frames := runRedlinePass(t, deps, v, map[string]string{"1": "x"})
	_, _, ok := findRedlineFrame(frames)
	assert.False(t, ok, "invalid JSON → no data-redline chunk (parse fails; verdict stands)")
}

// TestRedline_LLMReturnsFencedJSON — the LLM wraps its JSON in a
// ```json ... ``` markdown fence (common GLM behavior despite the
// "Output ONLY the JSON" instruction). emitRedline MUST strip the fence
// via llm.ExtractJSON before parsing, else json.Decode chokes on the
// leading backtick and the redline suggestion is silently dropped.
//
// Live failure: every 需补充 verdict dropped its redline (non-fatal WARN
// "invalid character '`'") until the strip was added. Mirrors the verdict
// path's use of ExtractJSON (pipeline.go). Without this test a regression
// that drops the strip reads as "redline occasionally missing" — this pins
// it as a hard parse failure.
func TestRedline_LLMReturnsFencedJSON(t *testing.T) {
	stubCM := &stubChatModel{cannedJSON: "```json\n{\n  \"originalClause\": \"原条款\",\n  \"proposedRewrite\": \"建议改写\",\n  \"rationale\": \"修改理由\"\n}\n```"}
	deps := newTestDeps()
	deps.ChatModel = stubCM
	v := verdict.Verdict{
		Status:     verdict.DowngradeStatus,
		Confidence: 0.6,
		Citations: []verdict.Citation{
			{ChunkID: "1", Span: "x", Tier: "权威"},
		},
	}

	frames := runRedlinePass(t, deps, v, map[string]string{"1": "x"})
	payload, _, ok := findRedlineFrame(frames)
	require.True(t, ok, "fenced JSON MUST be unwrapped and emitted (strip the ```json fence via llm.ExtractJSON)")
	assert.Equal(t, "建议改写", payload.ProposedRewrite, "payload fields parse after fence strip")
}
