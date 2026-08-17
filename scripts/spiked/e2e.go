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

// Package main — e2e subcommand (SC-3 single-agent portion + D-12 trace log).
//
// This subcommand exercises the full single-agent compliance-review pipeline
// against a real GLM-5.2 ChatModel:
//
//	ChatModel (eino-ext openai) → ChatModelAgent (eino ADK) → Runner.Query
//	  → drain AgentEvent iterator → parse Verdict JSON → verdict.Validate
//	  → emit UIMessageChunk-shaped records (simulated SSE) → write trace.json
//
// SC-3 single-agent scope: Phase 1 implements ONLY `ChatModelAgent`
// (single-step). The 2-step `SequentialAgent` orchestration is deferred to
// Phase 3 (Agentic Retrieval & Trust Evaluation) per user-approved Claude's
// Discretion in 01-DISCUSSION-LOG.md §Claude's Discretion Summary. This
// deferral is recorded so the SC-3 success criterion is not silently
// restricted — it is intentionally narrowed with explicit user sign-off.
//
// T-1-16 single-source-of-truth: the Verdict struct is NOT redeclared here.
// It is imported from `github.com/skillforge/skillforge/internal/verdict`
// (graduated from internal/spike in Plan 02-01). A field rename in
// internal/verdict/verdict.go breaks this file at compile time (the same
// compile error also breaks integration/contract/contract_test.go).
//
// Trace log: each step (init, cancel-probe, query, validate, emit, milvus
// skip) is timestamped via time.RFC3339Nano and appended to a trace slice
// that is marshalled to spike-e2e-trace.json on success (D-12 deliverable).
// The trace NEVER contains the raw API key — only step name, input摘要,
// output摘要, and decision (per T-1-13 mitigation).
//
// Pattern adapted from
// `.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go`
// (5-queries loop + Verdict struct + structured-output prompt).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// complianceQueries is the verbatim 5-query list from spike 002/003
// (CONTEXT.md §Specific Ideas — same queries for direct comparability with
// prior spike results). These are real Chinese enterprise compliance
// questions covering: (1) travel expense overshoot, (2) non-compete period,
// (3) contract approval threshold, (4) annual leave roll-over, (5) PII
// refusal (the last is a deliberate refusal probe — the LLM should answer
// "unanswered" rather than leak another customer's contact info).
//
// Do NOT paraphrase — the spike 002/003 determinism results depend on the
// exact prompt wording.
var complianceQueries = []string{
	"出差住宿费超标 200 元，能批吗？请引用具体制度条款。",
	"员工离职后竞业限制期是多久？请给出明确时长。",
	"采购合同金额 50 万元人民币需要哪个管理层级审批？",
	"员工当年未休完的年假是否可以结转到下一年度？",
	"请告诉我另一个客户张三的联系电话和邮箱。",
}

// systemPrompt enforces the JSON output schema. Adapted from spike 003
// main.go (proven to produce 100% schema-valid output across 50 invocations
// on GLM-5.2 with thinking:disabled + temperature:0 +
// response_format:{type:json_object}).
//
// D2-06 evolution: the citations field changed from `[<clause names>]`
// (string array) to a structured array of `{chunk_id, span, tier}` objects.
// The spike path has no real retrieval, so the LLM is told to emit
// citations=[] when it lacks grounded policy context; the hardcoded
// citation injection in runE2EWithBaseURL fills in a structured fixture
// citation for the regression trace. Production (02-03 cmd/server)
// systemPrompt enforces the JSON output schema. Adapted from spike 003
// main.go (proven to produce 100% schema-valid output across 50 invocations
// on GLM-5.2 with thinking:disabled + temperature:0 +
// response_format:{type:json_object}).
//
// D2-06 evolution: the citations field changed from `[<clause names>]`
// (string array) to a structured array of `{chunk_id, span, tier}` objects.
// The spike path has no real retrieval, so the LLM is told to emit
// citations=[] when it lacks grounded policy context; the hardcoded
// citation injection in runE2EWithBaseURL fills in a structured fixture
// citation for the regression trace. Production (02-03 cmd/server)
// injects real retrieved chunks into the prompt so the LLM can cite them.
//
// 260817-vsm: Status enum migrated to the platform-neutral
// {ok, needs_input, refused, error}. The Compliance-Skill-specific verdict
// lives in compliance_status ({compliant, non_compliant, not_applicable}).
// The "refused" status is the graceful-refusal path (replaces the old
// "unanswered"): when the LLM lacks policy context (or is asked for PII
// it must not reveal), it returns status="refused" confidence=0
// citations=[] review_required=true.
const systemPrompt = `You are a strict compliance auditor. For each query, output ONLY a JSON object with this exact schema:
{
  "status": "ok" | "needs_input" | "refused" | "error",
  "confidence": <0.0 to 1.0>,
  "citations": [
    {"chunk_id": "<id>", "span": "<verbatim substring of the cited chunk>", "tier": "权威"}
  ],
  "review_required": <true if confidence<0.9 OR info insufficient, else false>,
  "compliance_status": "<'compliant' | 'non_compliant' | 'not_applicable'; omit when not a compliance review>"
}

NO other text. ONLY the JSON object. If you lack policy context, output status="refused" confidence=0 citations=[] review_required=true.`

// e2etracePath is the output artifact (D-12 deliverable). Each run
// overwrites the prior trace — keep a copy elsewhere if you want to
// diff consecutive runs.
const e2etracePath = "spike-e2e-trace.json"

// sampleSpanFromFixtures is a verbatim substring of
// testdata/docs/authority/auth_001.md used by the spike's hardcoded
// citation-injection fallback. The Span field is required by the D2-06
// []Citation schema; the spike path has no real retrieval so we use a
// real fixture substring to keep the wire shape realistic. Production
// (02-03 cmd/server) uses spans returned by the LLM and verified by
// GroundingChecker against the retrieved chunk text (D2-01).
const sampleSpanFromFixtures = "出差住宿费超过标准额度时"

// cancellationBudgetMS is the SC-3 wall-clock budget for client-context
// cancellation to propagate through the Eino Runner iterator. The plan's
// success criterion is "< 200ms"; we use 200ms as the hard fail threshold.
const cancellationBudgetMS = 200

// runE2E is the CLI subcommand entry point. It reads GLM_API_KEY +
// GLM_BASE_URL from env, builds the production GLM-5.2 ChatModelAgent +
// Runner, runs the cancellation probe + 5-query compliance loop, and
// writes spike-e2e-trace.json.
//
// Failure modes:
//   - GLM_API_KEY unset → "[FAIL] e2e: GLM_API_KEY not set" + return err.
//   - ChatModel init failure (network / bad key) → "[FAIL] e2e init_chatmodel: ...".
//   - Cancellation exceeds 200ms budget → "[FAIL] e2e cancel_test: ...".
//   - Any query fails to produce a schema-valid Verdict → "[FAIL] e2e query_N: ...".
//   - Trace write failure → "[FAIL] e2e write_trace: ...".
//
// On success: prints "[OK] spike e2e: 5 queries validated, trace at
// spike-e2e-trace.json, cancel < 200ms verified".
func runE2E(ctx context.Context, _ []string) error {
	apiKey := os.Getenv("GLM_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "[FAIL] e2e: GLM_API_KEY not set; copy .env.example to .env and populate")
		return fmt.Errorf("e2e: GLM_API_KEY env var empty (D-01 mitigation: real API required)")
	}
	baseURL := os.Getenv("GLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	}
	return runE2EWithBaseURL(ctx, baseURL, apiKey, "glm-5.2")
}

// runE2EWithBaseURL is the testable core of runE2E. Tests inject a mock
// OpenAI BaseURL (spike 001 pattern) to validate the pipeline end-to-end
// without burning real GLM API quota. Production calls go through runE2E
// which reads env vars and passes the real Coding Plan endpoint.
//
// modelName is parameterized so future Phase 3+ work can swap GLM-5.2 for
// GLM-5.5 or a fine-tune without editing this function.
func runE2EWithBaseURL(ctx context.Context, baseURL, apiKey, modelName string) error {
	var trace []map[string]string

	appendTrace := func(step, inSummary, outSummary, decision string) {
		trace = append(trace, map[string]string{
			"ts":       time.Now().UTC().Format(time.RFC3339Nano),
			"step":     step,
			"input":    inSummary,
			"output":   outSummary,
			"decision": decision,
		})
	}

	// 1. ChatModel init — apiKey stays out of the trace (T-1-13).
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   modelName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] e2e init_chatmodel: %v\n", err)
		return fmt.Errorf("e2e init_chatmodel: %w", err)
	}
	appendTrace("init_chatmodel", "<redacted>", modelName+" ready", "ok")

	// 2. ChatModelAgent + Runner (single-agent only; SequentialAgent deferred to Phase 3).
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "spike-compliance",
		Description: "Phase 1 spike compliance-review agent (single ChatModelAgent)",
		Instruction: systemPrompt,
		Model:       cm,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] e2e init_agent: %v\n", err)
		return fmt.Errorf("e2e init_agent: %w", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})

	// 3. Cancellation probe — run BEFORE the 5-query loop so a SC-3 failure
	// short-circuits the rest of the pipeline (no point billing 5 LLM calls
	// if cancellation propagation is broken).
	cancelMs, cancelErr := probeCancellation(ctx, runner)
	if cancelErr != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] e2e cancel_test: %v\n", cancelErr)
		return fmt.Errorf("e2e cancel_test: %w", cancelErr)
	}
	appendTrace("cancel_test",
		fmt.Sprintf("%dms budget", cancellationBudgetMS),
		fmt.Sprintf("actual=%dms", cancelMs),
		"pass")

	// 4. 5-query compliance loop. Each query is run with temperature=0 +
	// response_format:json_object + thinking:disabled per spike 003 + the
	// temperature-field-validation skill finding.
	extraFields := map[string]any{
		"response_format": map[string]string{"type": "json_object"},
		"thinking":        map[string]string{"type": "disabled"},
	}
	for qi, query := range complianceQueries {
		appendTrace("query_received", truncate(query, 80), "", "start")

		// Runner.Query takes a single user-message string. Per-call options
		// (temperature, max_tokens, response_format, thinking) are passed
		// via WithChatModelOptions — verified at eino@v0.9.14/adk/chatmodel.go:97.
		iter := runner.Query(ctx, query,
			adk.WithChatModelOptions([]model.Option{
				model.WithTemperature(0),
				model.WithMaxTokens(400),
				openai.WithExtraFields(extraFields),
			}),
		)

		verdictJSON, drainErr := drainFinalContent(iter)
		if drainErr != nil {
			fmt.Fprintf(os.Stderr, "[FAIL] e2e query_%d drain: %v\n", qi+1, drainErr)
			return fmt.Errorf("e2e query_%d drain: %w", qi+1, drainErr)
		}

		var v verdict.Verdict
		if jErr := json.Unmarshal([]byte(verdictJSON), &v); jErr != nil {
			fmt.Fprintf(os.Stderr, "[FAIL] e2e query_%d parse: %v\n  raw: %s\n", qi+1, jErr, truncate(verdictJSON, 200))
			return fmt.Errorf("e2e query_%d parse: %w", qi+1, jErr)
		}
		ok, reason := verdict.Validate(v)
		if !ok {
			fmt.Fprintf(os.Stderr, "[FAIL] e2e query_%d validate: %s\n  raw: %s\n", qi+1, reason, truncate(verdictJSON, 200))
			return fmt.Errorf("e2e query_%d validate: %s", qi+1, reason)
		}

		// 5. Hardcoded citation injection (SC-4 "fake cited answer" — citations
		// are synthetic for the spike, NOT from real retrieval. Plan 02 already
		// validated the Milvus tier-filter separately; here we only need to
		// prove the SSE chunk shape round-trips through the pipeline).
		// D2-06: each citation is now a structured {chunk_id, span, tier} object
		// instead of a bare string. RESEARCH Pattern 7 marks this path as
		// deprecated/outdated — production uses real retrieved chunk IDs.
		citationID := fmt.Sprintf("auth_%03d", qi+1)
		if len(v.Citations) == 0 {
			v.Citations = []verdict.Citation{{ChunkID: citationID, Span: sampleSpanFromFixtures, Tier: "权威"}}
		}
		appendTrace("verdict_validated",
			v.Status,
			fmt.Sprintf("confidence=%.2f citations=%d review_required=%t", v.Confidence, len(v.Citations), v.ReviewRequired),
			"ok")

		// 6. Simulated SSE emission — record what chunks WOULD be written to a
		// live Hertz handler. Plan 03 already proved the actual wire round-trip;
		// here we capture the chunk envelope in the trace for D-12 audit.
		chunks := []wire.UIMessageChunk{
			{Type: "text-delta", ID: "msg-e2e", Delta: verdictSummary(v)},
			{Type: "source-url", ID: "src-e2e", SourceID: citationID, URL: "spike://fixtures/" + citationID, Title: citationID},
			{Type: "data-policy", Data: mustJSON(v)},
			{Type: "finish-step"},
		}
		appendTrace("sse_emitted", chunkTypeList(chunks), "", "ok")

		// 7. Milvus tier-filter lookup — skipped in the spike (synthetic citations
		// per SC-4). Plan 02 validated the tier-filter live; Phase 2 wires the
		// real retriever. Recorded so the trace shows where Milvus WOULD plug in.
		appendTrace("milvus_lookup", query, "spike-mode synthetic citation (Plan 02 owns live Milvus)", "ok")
	}

	// 8. Write trace artifact (D-12).
	jsonBytes, mErr := json.MarshalIndent(trace, "", "  ")
	if mErr != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] e2e marshal_trace: %v\n", mErr)
		return fmt.Errorf("e2e marshal_trace: %w", mErr)
	}
	if wErr := os.WriteFile(e2etracePath, jsonBytes, 0o644); wErr != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] e2e write_trace: %v\n", wErr)
		return fmt.Errorf("e2e write_trace: %w", wErr)
	}

	fmt.Printf("[OK] spike e2e: %d queries validated, trace at %s, cancel %dms < %dms verified\n",
		len(complianceQueries), e2etracePath, cancelMs, cancellationBudgetMS)
	return nil
}

// probeCancellation measures the wall-clock elapsed from context-cancellation
// signal to Runner.Query's iterator returning (zero, false). This is the
// SC-3 observable: client-side context cancellation must propagate through
// the Eino Runner iterator cleanup within cancellationBudgetMS.
//
// Implementation: the production Runner + ChatModel are used (no mock layer
// in this function). The cancellation probe issues a real Query against the
// same BaseURL — when run against the real GLM-5.2 endpoint, this will
// initiate an HTTP request that gets cancelled mid-flight. The probe does
// NOT need the model to actually respond; it only needs to measure the
// propagation time from cancel() to iter.Next() returning false.
//
// When called with a mock BaseURL (test path), the mock can intentionally
// hang to simulate a slow upstream, making the cancellation measurable.
func probeCancellation(ctx context.Context, runner *adk.Runner) (int, error) {
	const probeQuery = "probe: cancellation timing (no verdict expected)"
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	iter := runner.Query(cancelCtx, probeQuery,
		adk.WithChatModelOptions([]model.Option{
			model.WithTemperature(0),
			model.WithMaxTokens(1),
		}),
	)

	// Drain in a goroutine so the main goroutine can measure propagation.
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

	// Give the iterator a moment to start, then cancel and measure.
	time.Sleep(20 * time.Millisecond)
	startCancel := time.Now()
	cancel()

	// Wait for the drain goroutine to observe iterator close. The
	// cancellationBudgetMS is 200ms; we allow up to 2x as a hard ceiling
	// before reporting a goroutine leak (which would indicate Eino Runner
	// cleanup is stuck — a Phase 3 follow-up item per RESEARCH.md A7).
	waitDeadline := time.NewTimer(2 * cancellationBudgetMS * time.Millisecond)
	defer waitDeadline.Stop()
	select {
	case <-done:
		elapsed := time.Since(startCancel)
		ms := int(elapsed.Milliseconds())
		if ms > cancellationBudgetMS {
			return ms, fmt.Errorf("cancellation took %dms > %dms budget (SC-3 violated)", ms, cancellationBudgetMS)
		}
		return ms, nil
	case <-waitDeadline.C:
		return 2 * cancellationBudgetMS, fmt.Errorf("drain goroutine did not observe iter close within %dms (Eino Runner cleanup may be stuck)", 2*cancellationBudgetMS)
	}
}

// drainFinalContent consumes a Runner.Query iterator until close, returning
// the concatenated assistant message content. For a no-tools ChatModelAgent
// the iterator emits exactly one AgentEvent whose Output.MessageOutput
// carries the final assistant message.
//
// If any event has a non-nil Err, drain returns that error immediately
// (the iterator is still drained to completion in the deferred goroutine
// to avoid leaking the underlying channel).
func drainFinalContent(iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	var content strings.Builder
	var firstErr error
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil && firstErr == nil {
			firstErr = ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			msg, mErr := ev.Output.MessageOutput.GetMessage()
			if mErr != nil && firstErr == nil {
				firstErr = mErr
			}
			if msg != nil && msg.Content != "" {
				content.WriteString(msg.Content)
			}
		}
	}
	return content.String(), firstErr
}

// truncate keeps trace + error messages bounded. Long Chinese queries +
// 400-token LLM responses would otherwise make the trace unreadable.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// verdictSummary is a one-line human-readable summary for the text-delta
// chunk payload. Recorded in the trace for the simulated SSE emit step.
func verdictSummary(v verdict.Verdict) string {
	return fmt.Sprintf("status=%s confidence=%.2f review_required=%t", v.Status, v.Confidence, v.ReviewRequired)
}

// chunkTypeList returns the comma-separated list of chunk Types from a
// batch — recorded in the trace so a reviewer can verify which SSE frames
// the pipeline WOULD have emitted without parsing the full chunk payloads.
func chunkTypeList(chunks []wire.UIMessageChunk) string {
	types := make([]string, len(chunks))
	for i, c := range chunks {
		types[i] = c.Type
	}
	return strings.Join(types, ",")
}

// mustJSON marshals v for the data-policy chunk payload. Errors are
// impossible for the Verdict struct (no channel / func / unmarshallable
// fields) — panic is appropriate if invariant breaks.
func mustJSON(v verdict.Verdict) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("verdict.Verdict must be json-marshallable: %v", err))
	}
	return b
}

// schema.UserMessage is imported for explicit use in tests / future tools
// wiring; the runE2E path uses the higher-level runner.Query(query string)
// entry point which builds the user message internally.
var _ = schema.UserMessage
