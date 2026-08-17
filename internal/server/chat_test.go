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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The 7 acceptance tests for the verdict pipeline + chat handler:
//
//  1. TestChatHandlerSSE — SC-6 / AGNT-03 happy path
//  2. TestNoChunksRefusal — SC-2 / GEVL-02 (refusal-by-construction)
//  3. TestPersonalTierRefusal — SC-4 / SYSK-07 (no tier bleed)
//  4. TestLowScoreRefusal — GEVL-01 (max score < threshold)
//  5. TestTemperaturePassthrough — SYSK-08 runtime enforcement
//  6. TestGroundingCheckerInvoked — D2-01 moat (D2-02 downgrade)
//  7. TestRefusalShape — ARAG-05 (all 3 refusal reasons emit full shape)

// newTestHertz spins a Hertz server on a dynamic port with the chat
// route registered against deps. Returns the bound TCP address so the
// caller can issue real HTTP requests against it.
//
// Pattern reused from scripts/spiked/sse_test.go (the proven dynamic-port
// pattern; Pitfall 7 / hertz v0.10.6 verified).
func newTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.POST("/api/chat", deps.chatHandler)

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

// waitForListener polls the TCP addr until it accepts a connection or
// the deadline expires. Pattern reused from scripts/spiked/sse.go.
func waitForListener(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("listener %s not accepting after %v", addr, timeout)
}

// portStr converts an int port to its decimal string representation
// without importing strconv at the package-test layer.
func portStr(p int) string {
	if p == 0 {
		return "0"
	}
	digits := "0123456789"
	out := ""
	for p > 0 {
		out = string(digits[p%10]) + out
		p /= 10
	}
	return out
}

// newTestDeps builds a Deps with stub embedder / retriever / runner.
// Tests overwrite the fields they care about (searcher.chunks,
// runner.cannedJSON, etc.) after construction.
func newTestDeps() *Deps {
	return &Deps{
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		Runner:    newStubRunner(""),
	}
}

// newTestManifest writes a minimal SKILL.md into a temp directory and
// runs skill.Loader.Scan() to produce a fully-resolved Manifest
// (resolved tiers populated by the Loader — the only canonical way).
//
// This is the test-side substitute for cmd/server's startup hook. We
// cannot construct a *skill.Manifest with resolved tiers set directly
// (resolvedTiers is unexported), so the only path is via the Loader.
func newTestManifest(t *testing.T, reads []string, temp float32) *skill.Manifest {
	t.Helper()
	root := t.TempDir()
	// SKILL.md must live in an immediate subdirectory (the directory
	// name IS the Skill name per the Loader contract).
	skillDir := filepath.Join(root, "test")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	yaml := fmt.Sprintf(`---
description: test skill (drives the verdict pipeline in unit tests)
agent: ChatModelAgent
reads:
  - %s
writes: []
steps: |
  1. Step 1
  2. Step 2
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: %v
---
# Test Skill
`, reads[0], temp)

	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(yaml), 0o644))

	manifests, err := skill.NewLoader(root).Scan()
	require.NoError(t, err)
	require.NotEmpty(t, manifests, "test Loader must produce ≥1 manifest")
	require.Len(t, manifests, 1)
	return manifests[0]
}

// postChat issues a real POST /api/chat via stdlib http and returns the
// response body (full SSE stream) + the Content-Type header.
func postChat(t *testing.T, addr, query, skill string) (string, string) {
	t.Helper()
	body, _ := json.Marshal(chatRequest{Query: query, Skill: skill})
	resp, err := http.Post("http://"+addr+"/api/chat", "application/json", bytes.NewReader(body))
	require.NoError(t, err, "POST /api/chat")
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read body")
	return string(b), resp.Header.Get("Content-Type")
}

// TestChatHandlerSSE — SC-6 / AGNT-03 happy path. With a mock retriever
// returning 2 retriever.Hit chunks (tier=权威) + a mock runner
// returning a valid Verdict JSON with 2 citations, the POST /api/chat
// SSE stream must contain:
//
//	(a) ≥2 source-document UIMessageChunks with SourceID matching the
//	    citation chunk IDs AND a tier hint somewhere in the stream
//	    (tier rides in the data-policy Verdict payload's Citations[].Tier
//	    per Pattern 2).
//	(b) 1 data-policy UIMessageChunk whose Data unmarshals to a
//	    Verdict with Citations matching the mock.
//	(c) 1 finish-step UIMessageChunk.
//
// Plus: no snake_case types in the stream.
func TestChatHandlerSSE(t *testing.T) {
	deps := newTestDeps()
	chunk1 := "出差住宿费超标 200 元以内由部门负责人审批"
	chunk2 := "采购合同 50 万元以下由部门负责人审批"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunk1, Source: "authority/travel_expense.md", Score: 0.92},
		{ID: 2, Tier: "权威", Content: chunk2, Source: "authority/contract_approval.md", Score: 0.88},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	// Two chunks returned but the verdict only cites chunk 1 (the
	// relevant one for the query). The pipeline emits source-document
	// for every citation in the Verdict, not every chunk in the
	// retrieval set — so chunk 2 does NOT appear as a source-document
	// frame. The assertion below reflects this.
	deps.Runner = newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95,
		[]string{"1"}, chunk1, "权威"))

	// Loader with the real compliance SKILL.md so findSkill returns the manifest.
	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, ct := postChat(t, addr, "出差住宿费超标 200 元合规吗", "compliance")
	assert.Contains(t, ct, "text/event-stream",
		"Content-Type must be text/event-stream (auto-set by sse.NewWriter)")

	frames := parseFrames(t, body)
	require.GreaterOrEqual(t, len(frames), 3, "expected ≥3 frames (1 source-document + 1 data-policy + 1 finish-step)")

	// (a) source-document with SourceID matching citation chunk IDs.
	var sourceDocIDs []string
	for _, f := range frames {
		if f.Type == "source-document" {
			sourceDocIDs = append(sourceDocIDs, f.SourceID)
		}
	}
	assert.Contains(t, sourceDocIDs, "1",
		"source-document for chunk_id=1 (the cited chunk) must be on the wire")

	// (b) data-policy with a Verdict carrying the citation + tier="权威".
	var dataPolicy *verdict.Verdict
	for _, f := range frames {
		if f.Type == "data-policy" {
			var v verdict.Verdict
			require.NoError(t, json.Unmarshal(f.Data, &v),
				"data-policy Data field must unmarshal to verdict.Verdict (D2-06 schema)")
			dataPolicy = &v
		}
	}
	require.NotNil(t, dataPolicy, "expected at least one data-policy frame")
	assert.Equal(t, "ok", dataPolicy.Status,
		"platform-neutral Status is ok when ComplianceStatus=compliant (260817-vsm)")
	assert.Equal(t, verdict.CompliantStatus, dataPolicy.ComplianceStatus,
		"Compliance Skill verdict rides in ComplianceStatus, not Status (260817-vsm)")
	assert.Equal(t, 0.95, dataPolicy.Confidence)
	require.Len(t, dataPolicy.Citations, 1)
	assert.Equal(t, "权威", dataPolicy.Citations[0].Tier,
		"tier must ride in the data-policy Verdict payload (Pattern 2 line 311)")

	// (c) finish-step.
	var hasFinish bool
	for _, f := range frames {
		if f.Type == "finish-step" {
			hasFinish = true
		}
	}
	assert.True(t, hasFinish, "finish-step must terminate the stream (AGNT-03)")

	// (d) no snake_case types (T-1-11 mitigation).
	for _, f := range frames {
		assert.NotContains(t, f.Type, "_",
			"frame type %q must be kebab-case (T-1-11), not snake_case", f.Type)
	}
}

// TestNoChunksRefusal — SC-2 / GEVL-02 refusal-by-construction. With a
// mock retriever returning an empty chunk slice, the pipeline must:
//
//   - Emit a data-policy chunk whose Data unmarshals to a Refusal with
//     Reason="no_authorized_source", CanRephrase=true,
//     SuggestedAlternativeQuery non-empty.
//   - Emit a finish-step terminator.
//   - NOT call the runner (refusal-by-construction D-09).
func TestNoChunksRefusal(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{chunks: nil}

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "查询员工联系方式", "compliance")
	frames := parseFrames(t, body)

	refusal := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, refusal.Reason,
		"SC-2: 0 chunks must produce no_authorized_source refusal")
	assert.True(t, refusal.CanRephrase,
		"SC-2: refusal must be rephrasable (the user could include tier keyword)")
	assert.NotEmpty(t, refusal.SuggestedAlternativeQuery,
		"SC-2: rule-based suggestion (D2-14) must be non-empty")

	assert.Equal(t, 0, deps.Runner.CallCount(),
		"SC-2 / D-09: refusal path MUST NOT invoke the LLM (refusal-by-construction)")
}

// TestPersonalTierRefusal — SC-4 / SYSK-07 tier bleed guard. Querying
// for personal contact info against the compliance Skill (whose reads
// resolve to ["权威"]) must produce a refusal with the retriever's
// filter containing ONLY 权威 — no 个人 bleed.
func TestPersonalTierRefusal(t *testing.T) {
	deps := newTestDeps()
	stubSrch := &stubRetriever{chunks: nil}
	deps.Retriever = stubSrch

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "查询员工联系方式", "compliance")
	frames := parseFrames(t, body)

	tiers := stubSrch.LastTiers()
	require.NotEmpty(t, tiers, "retriever should have been called once")
	assert.Contains(t, tiers, "权威",
		"tier list must include 权威 (compliance skill's only read tier)")
	assert.NotContains(t, tiers, "个人",
		"tier list MUST NOT include 个人 — SC-4 / SYSK-07 tier bleed guard")

	refusal := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, refusal.Reason)
	assert.Equal(t, 0, deps.Runner.CallCount(),
		"SC-4 / D-09: no tier bleed + 0 chunks → refusal, NO LLM call")
}

// TestLowScoreRefusal — GEVL-01. Mock retriever returns 2 chunks with
// maxScore below the retrievalScoreThreshold placeholder (0.3).
func TestLowScoreRefusal(t *testing.T) {
	deps := newTestDeps()
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: "weakly related text", Source: "authority/random.md", Score: 0.15},
		{ID: 2, Tier: "权威", Content: "more noise", Source: "authority/other.md", Score: 0.20},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "weak query", "compliance")
	frames := parseFrames(t, body)

	refusal := findRefusal(t, frames)
	assert.Equal(t, verdict.RefusalReasonLowRetrievalScore, refusal.Reason,
		"GEVL-01: maxScore < threshold → low_retrieval_score refusal (D2-15 placeholder)")
	assert.True(t, refusal.CanRephrase)
	assert.Equal(t, 0, deps.Runner.CallCount(),
		"GEVL-01 / D-09: low-score refusal path MUST NOT invoke the LLM")
}

// TestTemperaturePassthrough — SYSK-08 runtime enforcement. The
// pipeline must invoke adk.WithChatModelOptions carrying
// model.WithTemperature(skill.Temperature) (compliance Skill declares
// temperature: 0.0). Verified by:
//
//  1. The pipeline reaches the LLM call step (Runner.CallCount == 1).
//  2. The captured opts slice is non-empty (proving the
//     WithChatModelOptions wrapper reached the runner).
//
// The exact model.WithTemperature value is pinned by the static
// criterion in pipeline.go (the literal call `model.WithTemperature(temp)`
// where temp=*manifest.Temperature) — verified by the plan's grep check.
//
// We additionally inspect the runner's recorded query string to confirm
// the prompt was constructed with the query the client sent (a sanity
// check that the temperature wrapper did not eat the user message).
func TestTemperaturePassthrough(t *testing.T) {
	deps := newTestDeps()
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: "出差住宿费超标 200 元以内由部门负责人审批", Source: "authority/travel_expense.md", Score: 0.95},
	}
	stubSrch := &stubRetriever{chunks: chunks}
	deps.Retriever = stubSrch
	stubRun := newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95,
		[]string{"1"}, "出差住宿费超标 200 元以内由部门负责人审批", "权威"))
	deps.Runner = stubRun

	// Loader points at the real on-disk skills/ directory which holds
	// skills/compliance/SKILL.md (temperature: 0.0 declared there).
	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "query", "compliance")
	require.NotEmpty(t, body, "HTTP round-trip should yield SSE frames")
	assert.Equal(t, 1, stubRun.CallCount(),
		"pipeline must have invoked the runner once (proving it reached step 8 — the LLM call)")

	// The captured opts slice must be non-empty (proving the
	// WithChatModelOptions wrapper reached the runner).
	opts := stubRun.LastOpts()
	require.NotEmpty(t, opts, "runner must have received ≥1 AgentRunOption (SYSK-08 wrapper)")

	// Sanity: the user query made it through the prompt builder.
	queries := stubRun.CapturedQueries()
	require.NotEmpty(t, queries)
	assert.Contains(t, queries[0], "query",
		"the user query must appear in the constructed prompt")
}

// TestGroundingCheckerInvoked — D2-01 moat. The pipeline invokes
// verdict.GroundingChecker between LLM parse and SSE emit. We verify
// the moat by feeding an obviously-ungrounded Verdict directly through
// the checker and asserting the D2-02 downgrade applies.
//
// This is a unit test of the moat itself (not an end-to-end via the
// pipeline) — the canonical pipeline integration is covered by the
// acceptance criterion in chat_test.go: the pipeline MUST call
// GroundingChecker (see pipeline.go step 9). The downgrade behavior
// is verified here as a property of the package.
func TestGroundingCheckerInvoked(t *testing.T) {
	v := verdict.Verdict{
		Status:         "合规",
		Confidence:     0.95,
		Citations:      []verdict.Citation{{ChunkID: "1", Span: "fabricated span", Tier: "权威"}},
		ReviewRequired: false,
	}
	chunksByID := map[string]string{"1": "real chunk text does not contain fabricated"}

	downgraded := verdict.GroundingChecker(v, chunksByID)
	assert.Equal(t, verdict.DowngradeStatus, downgraded.Status,
		"D2-02: ungrounded claim → Status=需补充")
	assert.LessOrEqual(t, downgraded.Confidence, verdict.ConfidenceFloor,
		"D2-02: ungrounded claim → Confidence ≤ 0.89")
	assert.True(t, downgraded.ReviewRequired,
		"D2-02: ungrounded claim → ReviewRequired=true")
}

// TestPipelineInvokesGroundingChecker — companion to
// TestGroundingCheckerInvoked. Runs the full pipeline with a chunk that
// contains the citation span verbatim. The pipeline must reach the
// grounding-check step (post-LLM, pre-emit) and produce a non-downgraded
// Verdict (the span IS in the chunk text, so the checker leaves it
// alone). Verified via HTTP round-trip + SSE frame inspection.
func TestPipelineInvokesGroundingChecker(t *testing.T) {
	deps := newTestDeps()
	chunkText := "出差住宿费超标 200 元以内由部门负责人审批"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: chunkText, Source: "authority/travel_expense.md", Score: 0.95},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	stubRun := newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95,
		[]string{"1"}, chunkText, "权威"))
	deps.Runner = stubRun

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "query", "compliance")
	require.NotEmpty(t, body)
	assert.Equal(t, 1, stubRun.CallCount())

	// The data-policy Verdict must NOT be downgraded — the span IS in
	// the chunk text (verbatim substring), so GroundingChecker leaves
	// it alone. We assert this via the Verdict payload: Status stays
	// "ok" (not "needs_input"), ComplianceStatus stays CompliantStatus
	// (not DowngradeStatus-propagated), Confidence stays 0.95 (not capped
	// at 0.89).
	frames := parseFrames(t, body)
	var v *verdict.Verdict
	for _, f := range frames {
		if f.Type == "data-policy" {
			var pv verdict.Verdict
			require.NoError(t, json.Unmarshal(f.Data, &pv))
			v = &pv
		}
	}
	require.NotNil(t, v, "data-policy Verdict must be on the wire")
	assert.Equal(t, "ok", v.Status,
		"grounded span → no D2-02 downgrade (Status must stay ok; 260817-vsm)")
	assert.Equal(t, verdict.CompliantStatus, v.ComplianceStatus,
		"grounded span → no D2-02 downgrade (ComplianceStatus must stay compliant)")
	assert.Equal(t, 0.95, v.Confidence,
		"grounded span → no D2-02 confidence cap (must stay 0.95)")
}

// TestRefusalShape — ARAG-05. All 3 refusal reasons emit Refusal
// structs with all 3 fields {reason, can_user_rephrase,
// suggested_alternative_query}.
func TestRefusalShape(t *testing.T) {
	t.Run("no_authorized_source", func(t *testing.T) {
		deps := newTestDeps()
		deps.Retriever = &stubRetriever{chunks: nil}
		loader := skill.NewLoader(skillsDirPath())
		_, err := loader.Scan()
		require.NoError(t, err)
		deps.Loader = loader
		addr, cleanup := newTestHertz(t, deps)
		defer cleanup()
		body, _ := postChat(t, addr, "out of scope", "compliance")
		r := findRefusal(t, parseFrames(t, body))
		assert.Equal(t, verdict.RefusalReasonNoAuthorizedSource, r.Reason)
		assert.True(t, r.CanRephrase)
		assert.NotEmpty(t, r.SuggestedAlternativeQuery)
	})

	t.Run("low_retrieval_score", func(t *testing.T) {
		deps := newTestDeps()
		deps.Retriever = &stubRetriever{chunks: []retriever.Hit{
			{ID: 1, Tier: "权威", Content: "x", Source: "authority/x.md", Score: 0.10},
		}}
		loader := skill.NewLoader(skillsDirPath())
		_, err := loader.Scan()
		require.NoError(t, err)
		deps.Loader = loader
		addr, cleanup := newTestHertz(t, deps)
		defer cleanup()
		body, _ := postChat(t, addr, "weak query", "compliance")
		r := findRefusal(t, parseFrames(t, body))
		assert.Equal(t, verdict.RefusalReasonLowRetrievalScore, r.Reason)
		assert.True(t, r.CanRephrase)
	})

	t.Run("out_of_scope", func(t *testing.T) {
		// Out-of-scope path: unknown skill name → handler emits the
		// refusal BEFORE VerdictPipeline runs (the searcher / runner
		// must not be invoked at all).
		deps := newTestDeps()
		stubSrch := &stubRetriever{chunks: nil}
		deps.Retriever = stubSrch
		stubRun := newStubRunner("")
		deps.Runner = stubRun
		deps.Loader = nil
		addr, cleanup := newTestHertz(t, deps)
		defer cleanup()
		body, _ := postChat(t, addr, "anything", "unknown_skill")
		r := findRefusal(t, parseFrames(t, body))
		assert.Equal(t, verdict.RefusalReasonOutOfScope, r.Reason)
		assert.False(t, r.CanRephrase,
			"out_of_scope refusals cannot be rephrased (the skill is unknown)")
		assert.Equal(t, 0, stubSrch.CallCount(),
			"unknown skill MUST NOT reach the retriever")
		assert.Equal(t, 0, stubRun.CallCount(),
			"unknown skill MUST NOT reach the LLM")
	})
}

// findRefusal locates the data-policy Refusal in a frame slice.
func findRefusal(t *testing.T, frames []wire.UIMessageChunk) verdict.Refusal {
	t.Helper()
	for _, f := range frames {
		if f.Type == "data-policy" {
			var r verdict.Refusal
			if err := json.Unmarshal(f.Data, &r); err == nil && r.Reason != "" {
				return r
			}
		}
	}
	t.Fatalf("no data-policy Refusal frame found in %d frames", len(frames))
	return verdict.Refusal{}
}

// parseFrames decodes SSE `data: {...}` lines into UIMessageChunk
// values. Mirrors scripts/spiked/sse.go:parseSSEFrames; we re-implement it
// locally to avoid an import cycle (spiked is package main).
func parseFrames(t *testing.T, body string) []wire.UIMessageChunk {
	t.Helper()
	var out []wire.UIMessageChunk
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "" {
			continue
		}
		var ch wire.UIMessageChunk
		require.NoError(t, json.Unmarshal([]byte(payload), &ch),
			"unmarshal data line: %s", payload)
		out = append(out, ch)
	}
	return out
}

// skillsDirPath resolves the on-disk skills/ directory. The tests use
// the real skills/compliance/SKILL.md (already validated by
// internal/skill/loader_test.go) so findSkill returns a valid Manifest.
// Run from the repo root (the test is in internal/server/, so we walk
// up three levels to reach the repo root).
func skillsDirPath() string {
	return "../../skills"
}

// routerEmbedder is a deterministic embedder used by the routing
// tests in this file. It points the query toward a single index in
// dim-dimensional space so the router can match against a Skill with
// DescVec at that same index.
type routerEmbedder struct {
	dim        int
	pivotIndex int
}

func (e *routerEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	v := make([]float32, e.dim)
	v[e.pivotIndex] = 1.0
	return v, nil
}
func (e *routerEmbedder) Dimensions() int { return e.dim }

// registryWithSkills builds a fresh skills/ root with two Skills
// under custom names. Both Skills use the same description so their
// embeddings are identical (cos=1 with each other AND with any
// query at the matching pivot) — gap=0 < margin → ambiguous. Used by
// TestChatHandler_AmbiguousRoutingDoesNotInvokePipeline to set up a
// tied-Skill snapshot deterministically.
//
// Caller must close via cancel() (which stops the watcher goroutine).
func registryWithSkills(t *testing.T, emb *routerEmbedder, names ...string) (*skill.Registry, context.CancelFunc) {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		skillDir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		yaml := fmt.Sprintf(`---
description: shared description
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. step
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses"
    - "Never reveal personal info"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---
# %s
`, name)
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(yaml), 0o644))
	}

	r, err := skill.New(root, emb)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, r.Watch(ctx))
	return r, cancel
}

// TestChatHandler_EmitsRoutingDecisionBeforePipeline — SC-1 / D5-05:
// chatHandler emits the routing decision (data-decision, decision="route")
// BEFORE VerdictPipeline is invoked. The router picks the Skill with
// the highest cosine against the query embedder; chatHandler emits the
// routing decision chunk with SkillScores + Selected + Ambiguous, then
// runs the verdict.
func TestChatHandler_EmitsRoutingDecisionBeforePipeline(t *testing.T) {
	emb := &routerEmbedder{dim: 8, pivotIndex: 3}
	r, cancel := registryWithSkills(t, emb, "compliance")
	defer cancel()

	deps := &Deps{
		Embedder: emb,
		Retriever: &stubRetriever{chunks: []retriever.Hit{
			{ID: 1, Tier: "权威", Content: "x", Source: "authority/x.md", Score: 0.92},
		}},
		Runner:   newStubRunner(cannedVerdictJSON(verdict.CompliantStatus, 0.95, []string{"1"}, "x", "权威")),
		Router:   skill.NewRouter(emb),
		Registry: r,
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "test query", "")
	frames := parseFrames(t, body)

	// Find the routing decision frame — it's the first data-decision
	// frame with decision="route". The DecisionPayload rides in
	// Data as raw JSON; unmarshal it for assertions.
	var routeIdx int = -1
	var routePayload wire.DecisionPayload
	for i, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == "route" {
			routeIdx = i
			routePayload = p
			break
		}
	}
	require.NotEqual(t, -1, routeIdx, "expected a data-decision with decision=route")
	require.NotEmpty(t, routePayload.SkillScores, "routing decision must carry SkillScores")
	assert.Equal(t, "compliance", routePayload.SkillScores[0].SkillID, "Selected must be 'compliance'")
	assert.True(t, routePayload.SkillScores[0].Selected, "the chosen Skill must have Selected=true")
	assert.False(t, routePayload.Ambiguous, "single-Skill install must not be ambiguous")

	// The route frame must appear BEFORE the verdict / source-document frames.
	var verdictIdx = -1
	for i, f := range frames {
		if f.Type == "data-policy" {
			verdictIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, verdictIdx, 0, "expected a data-policy frame")
	assert.Less(t, routeIdx, verdictIdx,
		"routing decision MUST be emitted BEFORE the verdict pipeline output")
}

// TestChatHandler_EmptyRegistryDoesNotInvokePipeline — SC-1 empty-
// state: registry has zero Skills installed. chatHandler emits the
// routing decision with Selected="" + Ambiguous=true + Scores=nil and
// returns WITHOUT calling VerdictPipeline. The FE renders the picker
// empty state ("no Skills installed, open the wizard").
func TestChatHandler_EmptyRegistryDoesNotInvokePipeline(t *testing.T) {
	emb := &routerEmbedder{dim: 8, pivotIndex: 0}
	// Build a registry rooted at an empty temp dir — New() will succeed
	// (no SKILL.md to seed; the snapshot stays empty).
	root := t.TempDir()
	r, err := skill.New(root, emb)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, r.Watch(ctx))

	sr := &stubRetriever{chunks: []retriever.Hit{}}
	stubRun := newStubRunner("")
	deps := &Deps{
		Embedder:  emb,
		Retriever: sr,
		Runner:    stubRun,
		Router:    skill.NewRouter(emb),
		Registry:  r,
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "anything", "")
	frames := parseFrames(t, body)

	// The route decision must carry Selected="" + Ambiguous=true.
	var foundRoute bool
	for _, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == "route" {
			foundRoute = true
			assert.Empty(t, p.Selected, "empty registry -> Selected=''")
			assert.True(t, p.Ambiguous, "empty registry -> Ambiguous=true")
			assert.Empty(t, p.SkillScores, "empty registry -> SkillScores=[]")
		}
	}
	assert.True(t, foundRoute, "expected a route data-decision frame")

	// VerdictPipeline must NOT be invoked (no source-document, no
	// data-policy verdict; the runner must NOT be called).
	assert.Equal(t, 0, stubRun.CallCount(),
		"empty registry MUST NOT invoke VerdictPipeline / Runner")
	// The retriever's Search is also not called (route short-circuits
	// before any tier lookup).
	assert.Equal(t, 0, sr.CallCount(),
		"empty registry MUST NOT invoke the retriever either")
}

// TestChatHandler_AmbiguousRoutingDoesNotInvokePipeline — D5-03 /
// SC-1 picker UX: when the router returns Ambiguous=true, chatHandler
// emits the routing decision and returns WITHOUT running VerdictPipeline.
// The FE renders the SkillPicker so the user picks.
func TestChatHandler_AmbiguousRoutingDoesNotInvokePipeline(t *testing.T) {
	emb := &routerEmbedder{dim: 8, pivotIndex: 3}
	// Two Skills with identical descriptions (cos=1 with each other AND
	// cos=1 with the query at the matching pivot) — gap=0 < margin → ambiguous.
	r, cancel := registryWithSkills(t, emb, "alpha", "bravo")
	defer cancel()

	sr := &stubRetriever{chunks: []retriever.Hit{}}
	stubRun := newStubRunner("")
	deps := &Deps{
		Embedder:  emb,
		Retriever: sr,
		Runner:    stubRun,
		Router:    skill.NewRouter(emb),
		Registry:  r,
	}
	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "anything", "")
	frames := parseFrames(t, body)

	// Find the routing decision; assert ambiguous.
	var foundRoute bool
	for _, f := range frames {
		if f.Type != "data-decision" {
			continue
		}
		var p wire.DecisionPayload
		if err := json.Unmarshal(f.Data, &p); err != nil {
			continue
		}
		if p.Decision == "route" {
			foundRoute = true
			assert.True(t, p.Ambiguous, "tied Skills must produce Ambiguous=true")
			assert.Empty(t, p.Selected, "ambiguous must clear Selected")
			assert.GreaterOrEqual(t, len(p.SkillScores), 2, "ambiguous carries ≥2 candidates")
		}
	}
	assert.True(t, foundRoute, "expected a route data-decision frame")
	assert.Equal(t, 0, stubRun.CallCount(),
		"ambiguous routing MUST NOT invoke VerdictPipeline / Runner")
	assert.Equal(t, 0, sr.CallCount(),
		"ambiguous routing MUST NOT invoke the retriever")
}

// TestVerdictStreamCarriesConclusionText — regression for
// .planning/debug/chat-conclusion-not-in-chat.md. The verdict stream's
// text-delta used to be a single-space placeholder, so the final
// conclusion never appeared in the chat conversation stream (it existed
// only as the structured data-policy payload rendered by the 结论 tab).
// The stream must now carry a human-readable conclusion sentence derived
// from the validated + grounded Verdict fields (conclusionText).
func TestVerdictStreamCarriesConclusionText(t *testing.T) {
	deps := newTestDeps()
	span := "出差住宿费超标 200 元以内由部门负责人审批"
	chunks := []retriever.Hit{
		{ID: 1, Tier: "权威", Content: span, Source: "authority/travel_expense.md", Score: 0.92},
	}
	deps.Retriever = &stubRetriever{chunks: chunks}
	// confidence 0.85 < 0.9 → ReviewRequired=true, so the conclusion
	// sentence must also surface the human-review note.
	deps.Runner = newStubRunner(cannedVerdictJSON(verdict.DowngradeStatus, 0.85, []string{"1"}, span, "权威"))

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "出差住宿费超标 200 元如何审批", "compliance")
	frames := parseFrames(t, body)

	var textDelta string
	for _, f := range frames {
		if f.Type == "text-delta" {
			textDelta = f.Delta
		}
	}
	require.NotEmpty(t, textDelta, "verdict stream must carry a text-delta part")
	assert.NotEqual(t, " ", textDelta,
		"text-delta must not be the legacy single-space placeholder (conclusion must be readable in chat)")
	assert.Contains(t, textDelta, "结论：needs_input",
		"conclusion sentence must state the verdict status (platform-neutral per 260817-vsm)")
	assert.Contains(t, textDelta, "85%",
		"conclusion sentence must state the confidence")
	assert.Contains(t, textDelta, span,
		"conclusion sentence must quote the grounded citation span verbatim")
	assert.Contains(t, textDelta, "需人工复核",
		"review_required=true verdict must surface the human-review note")
}

// TestRefusalStreamCarriesTextPart — regression for the same debug
// session. emitRefusal used to be a data-only stream (data-policy +
// finish-step), which leaves the assistant message unregistered in
// useChat (live Phase 7 finding — see pipeline.go Step-11 comment), so a
// refusal was invisible in the chat stream. The refusal stream must now
// carry a full text part (text-start/delta/end) with the human-readable
// refusal sentence (refusalText) and a finish terminator.
func TestRefusalStreamCarriesTextPart(t *testing.T) {
	deps := newTestDeps()
	deps.Retriever = &stubRetriever{chunks: nil}

	loader := skill.NewLoader(skillsDirPath())
	_, err := loader.Scan()
	require.NoError(t, err)
	deps.Loader = loader

	addr, cleanup := newTestHertz(t, deps)
	defer cleanup()

	body, _ := postChat(t, addr, "查询员工联系方式", "compliance")
	frames := parseFrames(t, body)

	var sawStart, sawEnd, sawFinish bool
	var textDelta string
	for _, f := range frames {
		switch f.Type {
		case "text-start":
			sawStart = true
		case "text-delta":
			textDelta = f.Delta
		case "text-end":
			sawEnd = true
		case "finish":
			sawFinish = true
		}
	}
	assert.True(t, sawStart, "refusal stream must open a text part (anchor the assistant message)")
	assert.True(t, sawEnd, "refusal stream must close the text part")
	assert.True(t, sawFinish, "refusal stream must end with a finish terminator")
	require.NotEmpty(t, textDelta, "refusal stream must carry a text-delta part")
	assert.Contains(t, textDelta, "无法给出结论",
		"refusal text must lead with the no-conclusion sentence")
	assert.Contains(t, textDelta, "未检索到可回答该问题的授权知识源",
		"0-chunk refusal reason must be rendered in operator-facing Chinese")
	// SC-2: CanRephrase=true + non-empty rule-based suggestion must be
	// surfaced so the user sees the rephrase hint in the chat itself.
	assert.Contains(t, textDelta, "可尝试调整问题",
		"rephrasable refusal must surface the suggestion hint in chat")
}
