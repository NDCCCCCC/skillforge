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
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/llm"
	"github.com/skillforge/skillforge/internal/verdict"
	"github.com/skillforge/skillforge/internal/wire"
)

// redlineSystemPrompt is the system prompt for the SC-7 redline LLM
// call. The prompt frames the task as "propose a clause-level rewrite
// that fixes the policy gap the verdict flagged" — advisory only, the
// operator reviews the rewrite before any action (T-5-29 accept
// disposition). The LLM is NOT given tools (pure structured output).
//
// Output contract: a JSON object matching wire.RedlinePayload (the V8
// required-field mitigation enforces presence at unmarshal time):
//
//	{
//	  "originalClause":   "<verbatim text of the cited clause>",
//	  "proposedRewrite":  "<replacement text>",
//	  "rationale":        "<one-sentence reason>",
//	  "clauseId":         "<optional stable id>"
//	}
const redlineSystemPrompt = `You are a policy-drafting assistant. The compliance auditor has flagged a policy gap. Propose a clause-level rewrite that fixes it.

Output ONLY a JSON object with this exact schema:
{
  "originalClause":   "<verbatim text of the cited clause as it currently exists>",
  "proposedRewrite":  "<your proposed replacement text, concretely fixing the gap>",
  "rationale":        "<one-sentence explanation of why the rewrite fixes the gap>",
  "clauseId":         "<optional stable id of the clause if one is identifiable from the source>"
}

Rules:
- The originalClause MUST be the verbatim text from the cited chunk (NOT a paraphrase).
- The proposedRewrite MUST be a complete, standalone replacement clause (NOT a delta).
- The rationale MUST reference the specific gap the verdict flagged.
- NEVER include HTML, markdown formatting, or extra commentary. Output ONLY the JSON object.`

// emitRedline is the SC-7 / CMPL-07 post-verdict redline pass (Plan
// 05-06, D5-14). It calls ChatModel.Generate with a structured-output
// prompt and emits ONE data-redline chunk carrying the proposed rewrite.
//
// D5-14 COST CONTROL — THE GATE IS THE FIRST LINE OF THIS FUNCTION.
//
//  260817-vsm (verdict-subject-mismatch): gate now ORs Status against
//  DowngradeStatus AND ComplianceStatus against NonCompliantStatus,
//  because the Compliance Skill's verdict now lives in ComplianceStatus
//  (Status is the platform-neutral envelope).
//
//	if v.Status != verdict.DowngradeStatus && v.ComplianceStatus != verdict.NonCompliantStatus {
//	    return nil  // BEFORE any LLM call, BEFORE any wire emit
//	}
//
// A compliant verdict (ComplianceStatus="compliant") makes ZERO LLM
// calls and emits ZERO data-redline chunks. TestRedline_Conditional is
// the load-bearing regression: it asserts stubChatModel.count == 0 on
// the compliant branch. A future "simplification" that moves the gate
// after the LLM call is a COST-CONTROL BREACH — the test catches it
// immediately.
//
// The gate's trigger set is:
//   - operational=true — NEVER fires. An operational skill (reads
//     resolve to no authority-role KB, e.g. sql-guides at role=personal)
//     structurally cannot emit a compliance verdict, so a policy-gap
//     clause rewrite is undefined for it. Before this condition existed
//     the pass fired on Status==needs_input for ANY skill and rendered a
//     compliance rewrite card on SQL 取数 answers
//     (sql-guides-needs-input, 2026-09-14; audit row 443 session).
//   - Status=DowngradeStatus ("needs_input") — platform-level downgrade
//     (grounding failure → GroundingChecker downgraded).
//   - ComplianceStatus=NonCompliantStatus ("non_compliant") —
//     Compliance Skill verdict indicating the subject fails the policy.
// Both are policy-gap signals where a rewrite proposal is actionable.
// ComplianceStatus="compliant" or empty has no gap to rewrite.
//
// LLM advisory only (T-5-29 accept): the proposedRewrite is rendered
// to the operator with copy/download CTAs. v1 does NOT write the
// rewrite back to the source doc. The verdict is final — the redline
// pass MUST NOT mutate v.Status or v.ComplianceStatus (verdict is final
// before this pass runs; the LLM is advisory only).
//
// Wire shape: ONE data-redline chunk (the 18th wire type added in
// Plan 05-01). The chunk carries wire.RedlinePayload in its Data field.
//
// Error handling: best-effort. If the LLM call fails OR the response
// does not parse into a valid RedlinePayload (V8 mitigation), the
// error is returned to the caller, who logs and continues — the
// verdict + counter-evidence stand. NO data-redline chunk is emitted
// on this path (TestRedline_LLMErrorIsNonFatal +
// TestRedline_LLMReturnsInvalidJSON).
//
// Threat model: T-5-26 (cost blowup) mitigated by the first-line gate
// + TestRedline_Conditional. T-5-29 (rewrite echoes original verbatim)
// accepted — the LLM is advisory; the user reviews the rewrite. T-5-28
// (tampering via dangerouslySetInnerHTML) is FE-side: RedlineCard
// renders text only.
func (s *Deps) emitRedline(ctx context.Context, c *app.RequestContext, v verdict.Verdict, chunksByID map[string]string, operational bool) error {
	// OPERATIONAL GATE — FIRST LINE, ahead of the cost-control gate. An
	// operational skill returns nil BEFORE any LLM call AND BEFORE any
	// wire emit. TestRedline_OperationalSkillNeverFires pins count == 0.
	if operational {
		return nil
	}
	// D5-14 COST-CONTROL GATE — FIRST LINE. A compliant verdict returns nil
	// BEFORE any LLM call AND BEFORE any wire emit. Moving this gate
	// after the LLM call is a TRUST-BOUNDARY BREACH (cost blowup).
	// 260817-vsm: OR'd check — Status=needs_input OR ComplianceStatus=
	// non_compliant triggers the rewrite pass; everything else (incl. the
	// happy path where ComplianceStatus="compliant") skips.
	if v.Status != verdict.DowngradeStatus && v.ComplianceStatus != verdict.NonCompliantStatus {
		return nil
	}
	// No ChatModel wired (e.g. test deps that exercise only the
	// counter-evidence pass). Skip without error — the verdict stands.
	if s.ChatModel == nil {
		return nil
	}

	// Build the user-side prompt from the verdict's citations + the
	// chunk text. The prompt deliberately renders chunk text verbatim
	// (same canonical form GroundingChecker sees) so the LLM proposes a
	// rewrite against the actual cited text rather than a paraphrase.
	userPrompt := buildRedlineUserPrompt(v, chunksByID)
	msgs := []*schema.Message{
		schema.SystemMessage(redlineSystemPrompt),
		schema.UserMessage(userPrompt),
	}

	out, err := s.ChatModel.Generate(ctx, msgs)
	if err != nil {
		// Non-fatal — verdict + counter-evidence stand. Caller logs.
		return fmt.Errorf("redline: LLM Generate: %w", err)
	}
	if out == nil || strings.TrimSpace(out.Content) == "" {
		return fmt.Errorf("redline: LLM returned empty content")
	}

	// Parse into wire.RedlinePayload. The V8 wire-smuggling mitigation
	// (RedlinePayload.UnmarshalJSON) rejects payloads missing any of the
	// three required fields (OriginalClause / ProposedRewrite /
	// Rationale) — this is the wire-contract enforcement at the parse
	// boundary. Unknown fields are also rejected by UnmarshalRedline.
	//
	// Strip a markdown code fence FIRST. GLMs frequently wrap JSON in
	// ```json ... ``` despite the "Output ONLY the JSON" instruction, and
	// UnmarshalRedline's json.Decode then chokes on the leading backtick.
	// The verdict path does the same via llm.ExtractJSON (pipeline.go).
	// Live failure: every 需补充 verdict silently dropped its redline
	// suggestion (non-fatal WARN) until this strip was added.
	payload, err := wire.UnmarshalRedline([]byte(llm.ExtractJSON(out.Content)))
	if err != nil {
		// Non-fatal — invalid LLM output, no chunk emitted. Caller logs.
		return fmt.Errorf("redline: parse payload: %w (raw: %s)", err, snippetOf(out.Content, 200))
	}

	// Emit ONE data-redline chunk. The FE MessageBubble dispatcher routes
	// the data-redline variant to RedlineCard (Plan 05-06 Task 2).
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redline: marshal payload: %w", err)
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{
		Type: "data-redline",
		Data: data,
	}); err != nil {
		return fmt.Errorf("redline: emit data-redline: %w", err)
	}
	return nil
}

// buildRedlineUserPrompt formats the verdict + cited chunk text into the
// user-side prompt for the redline LLM call. Pure function (no I/O, no
// globals) — unit-testable in isolation.
//
// The prompt deliberately renders chunk text verbatim (no normalization)
// so the LLM has the exact source to rewrite against. The verdict's
// citation spans are included so the LLM knows which clauses the
// auditor flagged.
func buildRedlineUserPrompt(v verdict.Verdict, chunksByID map[string]string) string {
	var b strings.Builder
	b.WriteString("The compliance auditor flagged the following policy gap:\n")
	fmt.Fprintf(&b, "  verdict status: %s\n", v.Status)
	fmt.Fprintf(&b, "  verdict confidence: %.2f\n", v.Confidence)
	b.WriteString("\nCited clauses (verbatim from the source):\n")
	if len(v.Citations) == 0 {
		b.WriteString("  (no citations)\n")
	}
	for i, cit := range v.Citations {
		fmt.Fprintf(&b, "  [%d] chunk_id=%s tier=%s\n", i+1, cit.ChunkID, cit.Tier)
		fmt.Fprintf(&b, "      span: %s\n", cit.Span)
		if content, ok := chunksByID[cit.ChunkID]; ok {
			fmt.Fprintf(&b, "      chunk text (verbatim):\n")
			for _, line := range strings.Split(content, "\n") {
				fmt.Fprintf(&b, "        %s\n", line)
			}
		} else {
			fmt.Fprintf(&b, "      (chunk text not available — base the rewrite on the span above)\n")
		}
	}
	b.WriteString("\nPropose a clause-level rewrite that fixes the gap. Return JSON per the system prompt.\n")
	return b.String()
}
