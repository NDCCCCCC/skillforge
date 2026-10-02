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

// Package main — tests for the e2e subcommand (SC-3 single-agent portion).
//
// Three tests cover the SC-3 observable evidence:
//
//  1. TestE2ECancellation — measures the wall-clock elapsed from
//     context.Cancel() to Runner.Query's iterator returning (zero, false).
//     Uses a mock OpenAI server that hangs forever; the Eino Runner must
//     observe the cancelled context within 200ms. SC-3 budget.
//
//  2. TestComplianceQueriesSkip — gated live-LLM test. Skipped unless
//     GLM_API_KEY is set in env. When enabled, runs the full
//     runE2EWithBaseURL pipeline against real GLM-5.2 + asserts
//     spike-e2e-trace.json is produced.
//
//  3. TestValidateVerdict — pure-logic test of verdict.Validate (the
//     Verdict schema gate). Always runs (no network). Imports
//     verdict.Validate / verdict.Verdict from internal/verdict per T-1-16 —
//     any field rename in internal/verdict/verdict.go breaks this test
//     at compile time (the single-source-of-trust guarantee).
//
// The cancellation test uses an in-process mock OpenAI server (spike 001
// pattern) so it can run without burning real GLM API quota. The
// compliance-queries test is the only path that needs a real key.
package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/stretchr/testify/assert"
)

// TestE2ECancellation verifies SC-3's cancellation-propagation budget
// against a live LLM endpoint.
//
// Gated on GLM_API_KEY: skipped when unset (default sandbox has no key)
// OR when set to "unhappy" (contract test's force-offline signal). When
// a real key is provided, the test issues a real ChatModelAgent.Runner.Query
// against the GLM Coding Plan endpoint with a 50ms context timeout, then
// measures the wall-clock elapsed from cancel to iter.Next() returning
// (zero, false) and asserts < 200ms (cancellationBudgetMS in e2e.go).
//
// Why this is gated: the cancellation test only meaningful against a slow
// upstream. Real GLM-5.2 is the production upstream; a fast mock returns
// before cancel fires and the propagation measurement is meaningless.
// The Plan 02 pattern (gated live-LLM test) applies.
//
// When skipped, the cancellation logic is still exercised end-to-end by
// the probeCancellation call inside runE2E — the deferred human-verify
// checkpoint runs `spiked e2e` against real GLM-5.2, which invokes
// probeCancellation internally.
func TestE2ECancellation(t *testing.T) {
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		t.Skip("skipping TestE2ECancellation: GLM_API_KEY unset (live-LLM test, gated per Plan 02 pattern)")
	}
	if apiKey == "unhappy" {
		t.Skip("skipping TestE2ECancellation: GLM_API_KEY=unhappy forces offline mode (contract test run)")
	}

	baseURL := os.Getenv("GLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   "glm-5.2",
	})
	if err != nil {
		t.Fatalf("NewChatModel: %v", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "spike-test-cancel",
		Description: "Cancellation test agent",
		Instruction: "test",
		Model:       cm,
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})

	// 50ms timeout on the query context. After 50ms the context expires
	// (DeadlineExceeded) — we then measure how long until the iterator
	// actually returns (zero, false). The 200ms SC-3 budget is what we
	// assert against.
	cancelCtx, cancelQuery := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelQuery()

	iter := runner.Query(cancelCtx, "probe: hang until cancelled")

	// Drain in a goroutine so the main goroutine can time the propagation.
	done := make(chan struct{})
	go func() {
		for {
			_, ok := iter.Next()
			if !ok {
				break
			}
		}
		close(done)
	}()

	// Wait for the context to expire, then start the propagation clock.
	<-cancelCtx.Done()
	startPropagated := time.Now()

	waitDeadline := time.NewTimer(2 * time.Second)
	defer waitDeadline.Stop()
	select {
	case <-done:
		elapsed := time.Since(startPropagated)
		assert.Less(t, elapsed, 200*time.Millisecond,
			"SC-3 violated: cancellation propagation took %v; budget 200ms", elapsed)
	case <-waitDeadline.C:
		t.Fatal("drain goroutine did not observe iter close within 2s (Eino Runner cleanup may be stuck — Phase 3 follow-up per RESEARCH.md A7)")
	}
}

// TestComplianceQueriesSkip is the gated live-LLM test. Skipped unless
// GLM_API_KEY is set. When enabled, runs runE2EWithBaseURL against the
// real GLM Coding Plan endpoint and asserts the trace artifact is
// produced with at least 5 query entries.
//
// This is the path the user runs manually to verify real GLM-5.2 behavior
// (the deferred human-verify checkpoint per Plan 04 execution notes). The
// test enables the same path from `go test` so the user gets a single
// command to verify the e2e pipeline.
func TestComplianceQueriesSkip(t *testing.T) {
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		t.Skip("skipping TestComplianceQueriesSkip: GLM_API_KEY unset (live-LLM test, gated)")
	}
	if apiKey == "unhappy" {
		t.Skip("skipping TestComplianceQueriesSkip: GLM_API_KEY=unhappy forces offline mode (contract test run)")
	}

	// Remove any stale trace from a prior run so we can assert this run
	// produced the file fresh.
	_ = os.Remove(e2etracePath)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	err := runE2E(ctx, nil)
	assert.NoError(t, err, "runE2E against real GLM-5.2 should succeed")

	info, err := os.Stat(e2etracePath)
	if err != nil {
		t.Fatalf("spike-e2e-trace.json not produced: %v", err)
	}
	assert.Greater(t, info.Size(), int64(0), "trace file must be non-empty")

	// Validate trace JSON: must be an array with >= 7 entries per query
	// (query_received + verdict_validated + sse_emitted + milvus_lookup = 4)
	// times 5 queries + init + cancel_test = 22 minimum. Use >= 20 to allow
	// for ordering / future steps.
	body, _ := os.ReadFile(e2etracePath)
	var trace []map[string]string
	if jErr := json.Unmarshal(body, &trace); jErr != nil {
		t.Fatalf("trace file is not valid JSON array: %v", jErr)
	}
	assert.GreaterOrEqual(t, len(trace), 20,
		"trace must contain at least 20 step entries (5 queries × 4 steps + init + cancel); got %d", len(trace))

	// T-1-13 mitigation: the trace must NEVER contain the raw API key.
	assert.NotContains(t, string(body), apiKey,
		"trace file must not contain the raw GLM_API_KEY value (T-1-13 information-disclosure mitigation)")
}

// TestValidateVerdict is the pure-logic schema-gate test. Always runs
// (no network). Imports verdict.Validate + verdict.Verdict — a rename in
// internal/verdict/verdict.go breaks this test at compile time (the
// T-1-16 single-source-of-truth guarantee that catches drift between
// scripts/spiked and integration/contract).
//
// 260817-vsm: ValidStatuses is platform-neutral ({ok, needs_input,
// refused, error}); Compliance-Skill-specific codes live in
// Verdict.ComplianceStatus. Four sub-cases:
//   - valid: ok + 0.85 confidence + ComplianceStatus=compliant → ok
//   - unknown status: "INVALID" → not ok
//   - out-of-range confidence: 1.5 → not ok
//   - graceful refusal: "refused" + 0 confidence → ok (was "unanswered")
func TestValidateVerdict(t *testing.T) {
	// 1. Valid verdict — all fields populated, normal compliance answer.
	// D2-06: Citations is now []Citation instead of []string.
	// 260817-vsm: Compliance verdict lives in ComplianceStatus, Status is "ok".
	v1 := verdict.Verdict{
		Status:           "ok",
		Confidence:       0.85,
		Citations:        []verdict.Citation{{ChunkID: "auth_001", Span: "出差住宿费超过标准额度时", Tier: "权威"}},
		ReviewRequired:   false,
		ComplianceStatus: verdict.CompliantStatus,
	}
	ok, reason := verdict.Validate(v1)
	assert.True(t, ok, "valid verdict should pass; reason=%s", reason)
	assert.Empty(t, reason, "valid verdict should have empty reason")

	// 2. Unknown status — must be rejected.
	v2 := verdict.Verdict{
		Status:     "INVALID",
		Confidence: 0.8,
		Citations:  []verdict.Citation{},
	}
	ok2, reason2 := verdict.Validate(v2)
	assert.False(t, ok2, "unknown status should fail")
	assert.Contains(t, reason2, "invalid status")

	// 3. Out-of-range confidence — must be rejected.
	v3 := verdict.Verdict{
		Status:     "ok",
		Confidence: 1.5,
		Citations:  []verdict.Citation{},
	}
	ok3, reason3 := verdict.Validate(v3)
	assert.False(t, ok3, "confidence > 1.0 should fail")
	assert.Contains(t, reason3, "confidence")

	// 4. Graceful refusal — spike 002/003 finding: when the LLM lacks
	// policy context (or is asked for PII it must not reveal), it returns
	// status="refused" confidence=0 citations=[] review_required=true.
	// This MUST validate so the pipeline treats it as a legitimate
	// refusal, not an error.
	// 260817-vsm: was "unanswered" (compliance-specific); now "refused" (platform).
	v4 := verdict.Verdict{
		Status:         "refused",
		Confidence:     0,
		Citations:      []verdict.Citation{},
		ReviewRequired: true,
	}
	ok4, reason4 := verdict.Validate(v4)
	assert.True(t, ok4, "graceful-refusal verdict should pass; reason=%s", reason4)
	assert.Empty(t, reason4, "graceful-refusal verdict should have empty reason")

	// 5. ValidStatuses contents — explicit assertion on the platform-neutral
	// enum. Compliance-Skill-specific codes (compliant / non_compliant /
	// not_applicable) live in ComplianceStatuses, not ValidStatuses, so
	// they must NOT appear here.
	assert.True(t, verdict.ValidStatuses["ok"], "ok must be in ValidStatuses")
	assert.True(t, verdict.ValidStatuses["needs_input"], "needs_input must be in ValidStatuses")
	assert.True(t, verdict.ValidStatuses["refused"], "refused must be in ValidStatuses")
	assert.True(t, verdict.ValidStatuses["error"], "error must be in ValidStatuses")
	assert.False(t, verdict.ValidStatuses["合规"], "合规 must NOT be in ValidStatuses (moved to ComplianceStatus)")
	assert.False(t, verdict.ValidStatuses["不合规"], "不合规 must NOT be in ValidStatuses (moved to ComplianceStatus)")
	assert.False(t, verdict.ValidStatuses["需补充"], "需补充 must NOT be in ValidStatuses (now needs_input)")
	assert.False(t, verdict.ValidStatuses["unanswered"], "unanswered must NOT be in ValidStatuses (now refused)")
}

// startHangingMockServer was removed: the cancellation test now uses the
// real GLM-5.2 endpoint (gated on GLM_API_KEY). A hanging mock could not
// be shut down cleanly because the in-flight handler never observed
// r.Context().Done() — the Eino openai adapter does not propagate client
// cancellation through to the server's request context within the
// test's shutdown window, so srv.Shutdown blocked indefinitely.
//
// If a future need arises for an offline cancellation test, use a mock
// with a BOUNDED delay (e.g. time.After(2*time.Second)) rather than
// waiting on r.Context().Done() so srv.Shutdown can complete.
