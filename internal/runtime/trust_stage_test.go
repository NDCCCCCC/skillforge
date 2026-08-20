// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

func newTrustStageInput(chunks int, content string, history ...*schema.Message) *TrustStageInput {
	out := &schema.Message{Role: schema.Assistant, Content: content}
	hist := make([]schema.Message, len(history))
	for i, m := range history {
		if m != nil {
			hist[i] = *m
		}
	}
	return &TrustStageInput{
		Manifest: &skill.Manifest{Name: "compliance", Agent: "chat"},
		Output:   out,
		Chunks:   makeChunks(chunks, "compliance test chunk content "),
		History:  hist,
	}
}

func makeChunks(n int, content string) []retriever.Hit {
	ch := make([]retriever.Hit, n)
	for i := 0; i < n; i++ {
		ch[i] = retriever.Hit{
			ID:      int64(i + 1),
			Content: content,
			Score:   0.9,
		}
	}
	return ch
}

// TestApplyTrustStage_NilInput guards the nil-receiver fast path.
func TestApplyTrustStage_NilInput(t *testing.T) {
	if _, err := ApplyTrustStage(context.Background(), nil); err == nil {
		t.Fatalf("expected error on nil input")
	}
}

// TestApplyTrustStage_RequiresManifest locks the manifest guard.
func TestApplyTrustStage_RequiresManifest(t *testing.T) {
	_, err := ApplyTrustStage(context.Background(), &TrustStageInput{
		Output: &schema.Message{Role: schema.Assistant, Content: "x"},
		Chunks: makeChunks(1, "c"),
	})
	if err == nil || !strings.Contains(err.Error(), "Manifest is required") {
		t.Fatalf("expected Manifest-required error, got %v", err)
	}
}

// TestApplyTrustStage_RequiresOutput locks the output guard.
func TestApplyTrustStage_RequiresOutput(t *testing.T) {
	_, err := ApplyTrustStage(context.Background(), &TrustStageInput{
		Manifest: &skill.Manifest{Name: "compliance", Agent: "chat"},
		Chunks:   makeChunks(1, "c"),
	})
	if err == nil || !strings.Contains(err.Error(), "Output is required") {
		t.Fatalf("expected Output-required error, got %v", err)
	}
}

// TestApplyTrustStage_ShortCircuitsOnFloor locks the zero-chunk refusal
// path. PLAN-04 SC invariant: zero chunks → structured refusal, NEVER
// improvisation (D3-01 / D-09 / KNOW-04).
func TestApplyTrustStage_ShortCircuitsOnFloor(t *testing.T) {
	in := newTrustStageInput(0, "answer attempt")
	in.Manifest.Reads = []string{"authority/policies/**"} // any resolved tier for the suggestion
	out, err := ApplyTrustStage(context.Background(), in)
	if err != nil {
		t.Fatalf("ApplyTrustStage: %v", err)
	}
	if !out.TrustFloorHit {
		t.Fatalf("expected TrustFloorHit=true")
	}
	if out.Refusal == nil {
		t.Fatalf("expected Refusal != nil")
	}
	if out.Verdict != nil {
		t.Fatalf("expected Verdict == nil on floor short-circuit, got %+v", out.Verdict)
	}
	if out.Refusal.Reason != verdict.RefusalReasonNoAuthorizedSource {
		t.Fatalf("Refusal.Reason = %q, want %q", out.Refusal.Reason, verdict.RefusalReasonNoAuthorizedSource)
	}
}

// TestApplyTrustStage_AppliesGroundingAndCritique locks the
// happy path: chunks present + grounded content → Verdict != nil,
// Refusal == nil, TrustFloorHit=false. The GroundingChecker is a
// no-op when citations are empty (12-04 wires the citation flow).
func TestApplyTrustStage_AppliesGroundingAndCritique(t *testing.T) {
	in := newTrustStageInput(3, "grounded answer")
	out, err := ApplyTrustStage(context.Background(), in)
	if err != nil {
		t.Fatalf("ApplyTrustStage: %v", err)
	}
	if out.TrustFloorHit {
		t.Fatalf("did NOT expect TrustFloorHit when chunks present")
	}
	if out.Refusal != nil {
		t.Fatalf("did NOT expect Refusal when chunks present")
	}
	if out.Verdict == nil {
		t.Fatalf("expected Verdict != nil on grounded happy path")
	}
	if out.Verdict.Status != "ok" {
		t.Fatalf("Verdict.Status = %q, want \"ok\"", out.Verdict.Status)
	}
}

// TestApplyTrustStage_PreservesD9V2 is the load-bearing regression test
// for the D9-V2 escape plan: history with past refusal MUST NOT cause
// the trust floor to refuse the current turn when the current turn has
// chunks. The Plan 09-06 NO-GO decision anchored on this invariant.
func TestApplyTrustStage_PreservesD9V2(t *testing.T) {
	historyRefusal := &schema.Message{
		Role:    schema.Assistant,
		Content: `{"reason":"no_authorized_source","can_rephrase":true}`,
	}
	in := newTrustStageInput(2, "current turn with chunks", historyRefusal)
	out, err := ApplyTrustStage(context.Background(), in)
	if err != nil {
		t.Fatalf("ApplyTrustStage: %v", err)
	}
	if out.TrustFloorHit {
		t.Fatalf("D9-V2 broken: history refusal leaked into current classification (TrustFloorHit=true)")
	}
	if out.Refusal != nil {
		t.Fatalf("D9-V2 broken: history refusal produced Refusal: %+v", out.Refusal)
	}
	if out.Verdict == nil {
		t.Fatalf("expected Verdict != nil — D9-V2 demands current-turn chunks drive the verdict, not history")
	}
}

// TestApplyTrustStage_AllPass locks the all-pass path:
// floor OK + grounding OK (no citations → no-op) + critique OK.
// The Verdict should carry the input content.
func TestApplyTrustStage_AllPass(t *testing.T) {
	const content = "verified answer"
	in := newTrustStageInput(1, content)
	out, err := ApplyTrustStage(context.Background(), in)
	if err != nil {
		t.Fatalf("ApplyTrustStage: %v", err)
	}
	if out.TrustFloorHit {
		t.Fatalf("expected TrustFloorHit=false")
	}
	if out.Verdict == nil {
		t.Fatalf("expected Verdict != nil")
	}
	if out.Verdict.Answer != content {
		t.Fatalf("Verdict.Answer = %q, want %q", out.Verdict.Answer, content)
	}
}