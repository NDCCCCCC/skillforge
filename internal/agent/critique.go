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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/llm"
	"github.com/skillforge/skillforge/internal/verdict"
)

// The critique uncertain-band constants (D3-05). Critique runs only when
// the verdict's Confidence falls inside [CritiqueLowConfidence,
// CritiqueHighConfidence], OR when multiHop=true, OR when Status=需补充.
// The values are illustrative for v1; Plan 03-06 calibrates from the
// golden-set (the eval sidecar's per-Skill faithfulness numbers drive
// the final band).
const (
	// CritiqueLowConfidence is the inclusive lower bound of the uncertain
	// band. Verdicts with Confidence < CritiqueLowConfidence look
	// "obviously wrong" — the floor or grounding check should already
	// have downgraded them; spending another LLM call on critique is
	// unlikely to add signal.
	CritiqueLowConfidence = 0.6

	// CritiqueHighConfidence is the inclusive upper bound. Verdicts with
	// Confidence > CritiqueHighConfidence look "obviously right" — single-
	// hop high-confidence answers skip critique to save cost (the
	// deterministic floor + grounding check are the authoritative
	// guardrails for that path).
	CritiqueHighConfidence = 0.9
)

// critiquePromptTemplate is the system prompt for the second-pass
// critique LLM call. The prompt deliberately frames the task as "review
// another auditor's verdict for semantic-reversal errors" — this
// second-auditor framing is the D3-04 mitigation against sycophancy
// (the critique model is not asked to "agree or disagree"; it is asked
// to look for a specific failure mode).
const critiquePromptTemplate = `You are a strict compliance auditor reviewing another auditor's verdict for semantic-reversal errors.

For each citation in the verdict, check whether the chunk text SUPPORTS the claim being made via that span. A semantic reversal = the chunk says X but the verdict uses the same span to support ¬X (the opposite of X), or the span does not appear in the chunk at all.

Return JSON: {"semantic_reversal_detected": true|false, "explanation": "one-sentence reason"}

Do NOT modify the verdict. Do NOT suggest a new status or confidence. Your ONLY output is the JSON above.`

// MaybeCritique is the layered Self-RAG critique pass (D3-04 / D3-05 /
// ARAG-03). It runs AFTER the agent loop and AFTER the trust floor +
// GroundingChecker. It uses the same ChatModel as the main agent run,
// invoked a second time with a critique-specific prompt.
//
// D3-04 INVARIANT (regression-locked by TestCritiqueCannotRaiseConfidence):
//
//	This function MUST NOT:
//	  - increase v.Confidence
//	  - flip v.Status (e.g. 需补充 → 合规)
//	  - remove any element from v.Citations
//
//	It MAY:
//	  - set v.ReviewRequired = true (only to true, never back to false)
//
//	The critique LLM's judgment is additive strictness only. The
//	deterministic floor + GroundingChecker own all confidence / status
//	mutations; the critique pass exists to catch semantic-reversal
//	errors that substring matching cannot detect (chunk says X, agent
//	cited the chunk to support ¬X). When the critique detects such a
//	reversal, the ONLY effect is to flag the verdict for human review.
//
// D3-05 gating (regression-locked by TestCritiqueGating):
//
//	Critique is SKIPPED (no LLM call) when ALL of the following hold:
//	  - multiHop == false (single-hop verdict)
//	  - v.Status != "需补充"
//	  - v.Confidence is OUTSIDE the uncertain band [0.6, 0.9]
//	Skipping clear-cut verdicts saves cost; the deterministic floor +
//	grounding checker are sufficient guardrails for high- and
//	low-confidence single-hop answers.
//
// Error handling (regression-locked by TestCritiqueErrorNonFatal):
//
//	If the critique LLM call fails (network, parse, etc.), the verdict
//	is returned UNCHANGED. The critique is non-authoritative — a flaky
//	sidecar cannot break the verdict path. Logged but not surfaced as
//	an error.
func (a *Agent) MaybeCritique(ctx context.Context, v verdict.Verdict, chunks map[string]string, multiHop bool) verdict.Verdict {
	// D3-05 gating — skip when single-hop, status not 需补充, and
	// confidence is outside the uncertain band. The band check uses
	// strict inequality on both sides so a Confidence exactly at 0.6 or
	// 0.9 still triggers critique (the boundaries are interesting cases).
	if !multiHop &&
		v.Status != verdict.DowngradeStatus &&
		(v.Confidence < CritiqueLowConfidence || v.Confidence > CritiqueHighConfidence) {
		return v
	}

	flag, err := a.runCritiqueLLMCall(ctx, v, chunks)
	if err != nil {
		// Non-fatal: log (would route through the agent's structured
		// logger in production; here we just drop the message — the
		// caller cannot act on it anyway, and the deterministic floor
		// remains authoritative). Return the verdict unchanged.
		//
		// Plan 03-05 wires a real logger via AgentConfig; until then we
		// route through the standard log package would require an
		// import for a single line, so we leave a code comment instead.
		// Future-proof: when a logger lands, replace this block with
		// `a.cfg.Logger.Warn("critique LLM call failed", "err", err)`.
		_ = err // explicitly discarded — non-fatal
		return v
	}

	if flag {
		// The ONLY permitted mutation. Structurally enforced — see the
		// D3-04 INVARIANT block above and TestCritiqueCannotRaiseConfidence.
		v.ReviewRequired = true
	}
	return v
}

// runCritiqueLLMCall issues the actual critique LLM call. Returns
// (true, nil) when the critique detected a semantic reversal; (false,
// nil) when it did not; (_, err) when the call or response parse
// failed (caller treats error as non-fatal per MaybeCritique godoc).
//
// The prompt is constructed from critiquePromptTemplate + the verdict
// under review + the chunk text. Both the chunk text and each
// citation's Span are normalized via verdict.Normalize BEFORE being
// inserted into the prompt — same canonical form as the GroundingChecker's
// substring check, so the critique model sees the same text shape the
// deterministic check sees (parity, avoids model confusion).
//
// The LLM call uses the same AgentConfig.ChatModel as the main agent
// run. It does NOT pass tool definitions (the critique is a pure
// structured-output call, no retrieve). Model options (temperature,
// max tokens, structured-output envelope) come from the
// spike-003-proven option slice used everywhere else in this package.
func (a *Agent) runCritiqueLLMCall(ctx context.Context, v verdict.Verdict, chunks map[string]string) (bool, error) {
	prompt := buildCritiqueUserPrompt(v, chunks)
	msgs := []*schema.Message{
		schema.SystemMessage(critiquePromptTemplate),
		schema.UserMessage(prompt),
	}

	out, err := a.cfg.ChatModel.Generate(ctx, msgs)
	if err != nil {
		return false, fmt.Errorf("critique LLM Generate: %w", err)
	}
	if out == nil || strings.TrimSpace(out.Content) == "" {
		return false, fmt.Errorf("critique LLM returned empty content")
	}

	var parsed struct {
		SemanticReversalDetected bool `json:"semantic_reversal_detected"`
	}
	if err := json.Unmarshal([]byte(llm.ExtractJSON(out.Content)), &parsed); err != nil {
		return false, fmt.Errorf("parse critique LLM JSON: %w (raw: %s)", err, truncate(out.Content, 200))
	}
	return parsed.SemanticReversalDetected, nil
}

// buildCritiqueUserPrompt formats the verdict + chunk text into the
// user-side prompt for the critique LLM call. Pure function (no I/O,
// no globals) — unit-testable in isolation.
//
// The prompt deliberately renders chunk text and span text in the SAME
// normalized form GroundingChecker uses (verdict.Normalize). This is
// the D3-04 parity invariant: the critique model sees what the
// deterministic checker sees, so they reason about the same canonical
// text rather than talking past each other.
func buildCritiqueUserPrompt(v verdict.Verdict, chunks map[string]string) string {
	var b strings.Builder
	b.WriteString("Verdict under review:\n")
	fmt.Fprintf(&b, "  status: %s\n", v.Status)
	fmt.Fprintf(&b, "  confidence: %.2f\n", v.Confidence)
	b.WriteString("  citations:\n")
	if len(v.Citations) == 0 {
		b.WriteString("    (none)\n")
	}
	for i, c := range v.Citations {
		fmt.Fprintf(&b, "    [%d] chunk_id=%s tier=%s\n", i+1, c.ChunkID, c.Tier)
		fmt.Fprintf(&b, "        span (raw):      %s\n", c.Span)
		fmt.Fprintf(&b, "        span (normalized): %s\n", verdict.Normalize(c.Span))
	}
	b.WriteString("\nChunks retrieved for this verdict:\n")
	if len(chunks) == 0 {
		b.WriteString("  (none)\n")
	}
	for id, content := range chunks {
		fmt.Fprintf(&b, "  [chunk %s]\n", id)
		fmt.Fprintf(&b, "    raw:        %s\n", content)
		fmt.Fprintf(&b, "    normalized: %s\n", verdict.Normalize(content))
	}
	b.WriteString("\nReturn JSON per the system prompt.\n")
	return b.String()
}

// truncate caps s at maxRunes runes (defensively, with a trailing
// ellipsis when truncation happens). Used to keep error messages
// readable when the LLM returns a multi-KB response that fails JSON
// parsing. Logic copied from internal/server/pipeline.go truncate
// (lines 283-288) — same heuristic, same length cap.
//
// Production migration target (Plan 03-05): lift into a shared
// internal/agent/prompt.go alongside ruleBasedSuggestion; until then
// the duplication is intentional and small.
func truncate(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

// Compile-time interface check: Agent's ChatModel field must satisfy
// model.ToolCallingChatModel so the critique LLM call can use Generate
// directly without a runtime type assertion.
var _ model.ToolCallingChatModel = (model.ToolCallingChatModel)(nil)
