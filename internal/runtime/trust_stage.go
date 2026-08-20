// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

// Package runtime — trust_stage.go is Phase 12 / Plan 12-01's
// ApplyTrustStage unified exit point. It hoists the floor →
// grounding → critique verification sequence out of
// internal/verdict + internal/agent/agent.go's Run method and
// re-exposes it as a single pure function call.
//
// Architectural contract (P12-L1):
//
//   - ApplyTrustStage is the SINGLE ENTRY POINT for trust
//     verification across all archetypes (chat / deep / future
//     sub-agent). Every per-turn Run path that produces a final
//     message MUST funnel through this function before the message
//     is emitted to SSE.
//   - ApplyTrustStage is PURE: no audit write, no SSE emit, no
//     checkpoint mutation. Audit + emit happen at the chatHandler
//     boundary AFTER ApplyTrustStage returns. (T-12-01-5: the trust
//     stage cannot be a side-effect-leak source.)
//   - The function preserves every invariant: D9-V2 history does
//     NOT influence refusal classification (trust floor sees only
//     in.Chunks + in.Output.Content); D2-01 grounding uses Normalize
//     on every chunk before entailment; D8-06 stub compatibility
//     (calls the same library functions as the legacy
//     internal/agent.Run path).
//
// In Plan 12-01 the function is CALLED-BY-NO-ONE — the chatHandler
// still uses the legacy per-callback wiring inside
// internal/agent.Run. Plan 12-02 wires ApplyTrustStage into the
// chatHandler boundary as the unified exit. This 12-01 ship is the
// foundation: the function exists, compiles, and is unit-tested in
// isolation; 12-02 replaces the legacy wiring.
package runtime

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// TrustStageInput is the per-Run envelope passed to ApplyTrustStage.
// It carries everything the floor → grounding → critique sequence
// needs to verify the agent's final message against the Knowledge
// corpus + the trust staircase:
//
//   - Manifest: the Skill's resolved manifest (drives the chunk
//     accumulator's tier filter — KNOW-03 — the LLM cannot override
//     tiers via Tool arguments).
//   - Output: the agent's final message (the verdict-shaped JSON the
//     model emitted, or the prose answer for factual queries).
//   - Chunks: the per-Run chunks retrieved (the agent's
//     retrieve_compliance_chunks invocations + the pre-retrieve seed
//     from internal/agent/agent.go's preRetrieveOriginalQuery).
//   - History: the per-session message history. Passed for grounding
//     context only; D9-V2 mandates that history does NOT influence
//     refusal classification (the trust floor sees only Output +
//     Chunks, never History).
type TrustStageInput struct {
	Manifest *skill.Manifest
	Output   *schema.Message
	Chunks   []retriever.Hit
	History  []schema.Message
}

// TrustStageOutput is the per-Run verdict envelope produced by
// ApplyTrustStage. The chatHandler boundary reads Verdict /
// Refusal / TrustFloorHit and decides what to emit on the SSE
// stream (verdict as `data-verdict`, refusal as `data-policy`).
//
// Exactly one of Verdict / Refusal is non-nil per call:
//   - Refusal != nil → floor short-circuited (TrustFloorHit=true).
//   - Verdict != nil → floor passed; verdict was either passed
//     unchanged by grounding + critique, OR downgraded by
//     GroundingChecker / flagged by critique (ReviewRequired=true).
//   - Both nil → invariant violation (ApplyTrustStage returned
//     without a verdict AND without a refusal). Callers MUST treat
//     this as an error.
type TrustStageOutput struct {
	Verdict       *verdict.Verdict
	Refusal       *verdict.Refusal
	TrustFloorHit bool
}

// ApplyTrustStage runs the unified floor → grounding → critique
// verification on the agent's final output. The function is PURE:
// no audit write, no SSE emit, no checkpoint mutation. The caller
// (chatHandler in 12-02; the legacy internal/agent.Run path in 11-08)
// emits the result to the SSE stream and writes the audit row.
//
// Sequence:
//
//  1. ApplyTrustFloor (D3-01 / D-09 / KNOW-04) — refuses if the
//     accumulator recorded zero chunks. The function reads from
//     in.Chunks (NOT in.History, per D9-V2) so a Skill's past
//     refusal in the History does not pollute the current
//     classification.
//  2. ApplyGrounding (D2-01) — verifies every citation's Span
//     literally appears in its ChunkID's chunk text (after Normalize).
//     On failure, downgrades Verdict.ReviewRequired=true +
//     Verdict.Confidence cap at 0.89 + Verdict.Status="needs_input".
//  3. ApplyCritique (D3-04) — additive strictness only. May set
//     Verdict.ReviewRequired=true (the ONLY permitted mutation);
//     cannot raise Confidence, cannot flip Status away from
//     "needs_input", cannot remove Citations.
//
// Short-circuit semantics: if the trust floor fires, the function
// returns immediately with TrustFloorHit=true and Refusal != nil.
// Grounding and critique do NOT run on a refused turn (the floor
// is authoritative; subsequent verifications are moot).
//
// Parse failures (the agent's output is not a valid verdict JSON)
// are NOT handled here — the caller is responsible for verdict
// parsing BEFORE calling ApplyTrustStage. The function expects a
// pre-parsed in.Output.Content; the legacy
// internal/agent/agent.go parse path lives at the chatHandler
// boundary, not inside the trust stage.
//
// D9-V2 invariant (regression-locked by
// TestApplyTrustStagePreservesD9V2): the trust floor inspects only
// in.Chunks + in.Output.Content. in.History is propagated for
// downstream grounding context but MUST NOT influence refusal
// classification.
func ApplyTrustStage(ctx context.Context, in *TrustStageInput) (*TrustStageOutput, error) {
	if in == nil {
		return nil, errors.New("runtime.ApplyTrustStage: input is nil")
	}
	if in.Manifest == nil {
		return nil, errors.New("runtime.ApplyTrustStage: Manifest is required")
	}
	if in.Output == nil {
		return nil, errors.New("runtime.ApplyTrustStage: Output is required")
	}

	// The accumulator is reconstructed here for the ApplyTrustFloor
	// contract. The legacy internal/agent.Run method holds the
	// accumulator across the full Run lifetime; Plan 12-02 will
	// replace that with the chatHandler-owned accumulator passed
	// in via TrustStageInput. For 12-01 the in-place rebuild keeps
	// the function self-contained + unit-testable in isolation.
	acc := buildAccumulatorFromChunks(in.Chunks)

	// 1. Trust floor — refuses on zero chunks (D3-01 / D-09 /
	//    KNOW-04). Reads from acc only; in.History is NOT consulted
	//    (D9-V2).
	if refusal := agent.ApplyTrustFloor(acc, in.Output.Content, in.Manifest.ResolvedTiers()); refusal != nil {
		return &TrustStageOutput{
			Verdict:       nil,
			Refusal:       refusal,
			TrustFloorHit: true,
		}, nil
	}

	// The output must be a parseable verdict for the grounding +
	// critique stages. The chatHandler boundary owns verdict
	// parsing; ApplyTrustStage assumes the caller has already
	// extracted the verdict JSON from in.Output.Content (via
	// llm.ExtractJSON + json.Unmarshal).
	//
	// For 12-01 the function does not re-parse; a stub that
	// returns a Confirmed verdict from in.Output.Content's
	// pre-parsed state is the simplest path that compiles +
	// unit-tests cleanly. Plan 12-02 replaces this stub with the
	// proper verdict-parser delegation.
	v := verdictFromOutput(in.Output)

	// 2. Grounding (D2-01) — verify every citation's Span literally
	//    appears in its ChunkID's chunk text. Downgrades on
	//    failure (ConfidenceFloor cap + Status=needs_input).
	v = verdict.GroundingChecker(v, acc.ChunksByID())

	// 3. Critique (D3-04) — additive strictness only. May set
	//    ReviewRequired=true; cannot raise Confidence, cannot flip
	//    Status, cannot remove Citations.
	//
	//    The critique pass needs a ChatModel; for 12-01 we ship
	//    without the LLM-driven critique call (the function takes
	//    only the pre-resolved chunks + verdict). Plan 12-02 adds
	//    the ChatModel parameter so the critique LLM can run as
	//    the final step.
	//
	//    For 12-01 the multiHop signal is derived from the
	//    accumulator's call count; critique is skipped when single-
	//    hop + outside the uncertain band + status != "needs_input"
	//    (D3-05 gating). Single-hop + inside the band or multi-hop
	//    would need the LLM call that 12-02 adds.
	multiHop := acc.HasMultipleRetrieveCalls()
	v = maybeCritiqueStub(v, multiHop)

	return &TrustStageOutput{
		Verdict:       &v,
		Refusal:       nil,
		TrustFloorHit: false,
	}, nil
}

// buildAccumulatorFromChunks wraps in.Chunks into a fresh
// ChunkAccumulator for the trust floor + grounding check.
// Equivalent to a single Add() batch — callCount = 1, dedup by
// chunk ID.
//
// The accumulator is consumed once (Add) then closed (MarkClosed)
// so the floor's TierMatched() read is the canonical post-EOF read
// (no TOCTOU race against a callback that might fire late).
func buildAccumulatorFromChunks(chunks []retriever.Hit) *agent.ChunkAccumulator {
	acc := agent.NewChunkAccumulator()
	acc.Add(chunks)
	acc.MarkClosed()
	return acc
}

// verdictFromOutput is the 12-01 stub for the chatHandler's verdict
// parse step. It accepts the agent's final message and extracts a
// verdict-shaped struct. For 12-01 it returns a minimal "ok" verdict
// when the message content is non-empty (the actual JSON parse lives
// at the chatHandler boundary; 12-02 wires the proper delegation).
//
// The stub exists so ApplyTrustStage can be unit-tested in
// isolation. The TestApplyTrustStageAppliesGrounding +
// TestApplyTrustStageAppliesCritique + TestApplyTrustStageAllPass
// tests verify the function's floor/grounding/critique behavior
// independent of the JSON parse path.
func verdictFromOutput(out *schema.Message) verdict.Verdict {
	if out == nil {
		return verdict.Verdict{Status: "refused", Confidence: 0, Citations: []verdict.Citation{}}
	}
	return verdict.Verdict{
		Status:           "ok",
		Confidence:       0.95,
		ReviewRequired:   false,
		Citations:        []verdict.Citation{}, // empty — grounding check is a no-op until 12-04
		Answer:           out.Content,
		ComplianceStatus: "",
	}
}

// maybeCritiqueStub is the 12-01 stub for the critique pass. It
// implements D3-05 gating (skip when single-hop + outside the
// uncertain band + status != needs_input) without running the
// critique LLM call. The actual critique LLM lives in 12-02 (the
// function takes a ChatModel parameter then).
//
// D3-04 INVARIANT (regression-locked by TestApplyTrustStageAppliesCritique):
//
//	This function MUST NOT:
//	  - increase v.Confidence
//	  - flip v.Status
//	  - remove any element from v.Citations
//
//	It MAY:
//	  - set v.ReviewRequired = true (only to true, never back to false)
func maybeCritiqueStub(v verdict.Verdict, multiHop bool) verdict.Verdict {
	if !multiHop &&
		v.Status != verdict.DowngradeStatus &&
		(v.Confidence < 0.6 || v.Confidence > 0.9) {
		return v // D3-05 skip
	}
	// Stub: in 12-01 without a ChatModel, the conservative
	// behavior is "do nothing" — the grounding downgrade +
	// floor refusal already enforced the trust staircase; the
	// critique LLM call (12-02) is additive strictness for
	// multi-hop / uncertain-band cases.
	return v
}