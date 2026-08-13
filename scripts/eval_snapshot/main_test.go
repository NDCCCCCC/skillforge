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

package main

import (
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/eval"
)

// These tests cover the refusal_accuracy derivation in isolation. The
// fixtures are built IN-TEST (no disk I/O) so they exercise the pure
// derivation logic, not a live-stack eval/report.json — that file is
// produced by `cmd/eval --frozen=compliance` against a live Milvus +
// BGE-M3 + GLM stack (Plan 07-05 Task 2, autonomous:false).
//
// refusal_accuracy is NOT a field the evaluator writes; it is DERIVED
// from Report.Cases by filtering on Category=="refusal-required" and
// counting Refused==true (RESEARCH §Open Questions Q1). These tests
// pin the derivation.

func TestDeriveRefusalAccuracy_AllRefused(t *testing.T) {
	report := eval.Report{
		Cases: []eval.CaseResult{
			{ID: "r1", Category: "refusal-required", Refused: true},
			{ID: "r2", Category: "refusal-required", Refused: true},
		},
	}
	ratio, total, refused := deriveRefusalAccuracy(report)
	if ratio != 1.0 {
		t.Fatalf("ratio: want 1.0, got %v", ratio)
	}
	if total != 2 || refused != 2 {
		t.Fatalf("counts: want total=2 refused=2, got total=%d refused=%d", total, refused)
	}
}

func TestDeriveRefusalAccuracy_HalfRefused(t *testing.T) {
	report := eval.Report{
		Cases: []eval.CaseResult{
			{ID: "r1", Category: "refusal-required", Refused: true},
			{ID: "r2", Category: "refusal-required", Refused: false},
			{ID: "r3", Category: "refusal-required", Refused: true},
			{ID: "r4", Category: "refusal-required", Refused: false},
		},
	}
	ratio, total, refused := deriveRefusalAccuracy(report)
	const wantRatio = 0.5
	if ratio != wantRatio {
		t.Fatalf("ratio: want %v, got %v", wantRatio, ratio)
	}
	if total != 4 || refused != 2 {
		t.Fatalf("counts: want total=4 refused=2, got total=%d refused=%d", total, refused)
	}
}

func TestDeriveRefusalAccuracy_ZeroRefusalRequired(t *testing.T) {
	// A report with zero refusal-required cases is a degenerate
	// fixture (the golden set always has 20), but the derivation
	// must be graceful: total=0, refused=0, ratio=0 (the renderer
	// turns this into "N/A", not "0%").
	report := eval.Report{
		Cases: []eval.CaseResult{
			{ID: "a1", Category: "adversarial", Refused: true},
			{ID: "m1", Category: "multi-hop", Refused: true},
		},
	}
	ratio, total, refused := deriveRefusalAccuracy(report)
	if total != 0 || refused != 0 {
		t.Fatalf("counts: want total=0 refused=0, got total=%d refused=%d", total, refused)
	}
	if ratio != 0 {
		t.Fatalf("ratio: want 0 (graceful zero-total), got %v", ratio)
	}
}

func TestDeriveRefusalAccuracy_OnlyCountsRefusalRequiredCategory(t *testing.T) {
	// refused==true on adversarial / multi-hop cases must NOT inflate
	// the numerator — only refusal-required cases belong in the ratio.
	report := eval.Report{
		Cases: []eval.CaseResult{
			{ID: "a1", Category: "adversarial", Refused: true},
			{ID: "m1", Category: "multi-hop", Refused: true},
			{ID: "m2", Category: "multi-hop", Refused: false},
			{ID: "r1", Category: "refusal-required", Refused: true},
		},
	}
	ratio, total, refused := deriveRefusalAccuracy(report)
	if ratio != 1.0 {
		t.Fatalf("ratio: want 1.0 (only the 1 refusal-required case counts), got %v", ratio)
	}
	if total != 1 || refused != 1 {
		t.Fatalf("counts: want total=1 refused=1, got total=%d refused=%d", total, refused)
	}
}

func TestRenderSnapshot_IncludesLockMetadataAndColumns(t *testing.T) {
	report := eval.Report{
		Skill: "compliance",
		Overall: map[string]float64{
			"faithfulness":      0.92,
			"context_precision": 0.85,
		},
		Cases: []eval.CaseResult{
			{ID: "r1", Category: "refusal-required", Refused: true},
			{ID: "r2", Category: "refusal-required", Refused: false},
		},
	}
	lock := eval.LockFile{
		ModelVersion:   "glm-5.2",
		PromptHash:     "bd59cbe0941d19174c3a29271ce710be02585c7c2df5ff5c13f166eeaffbecae",
		EvalSetVersion: "compliance-2026-08-13-v1",
		SkillName:      "compliance",
	}
	out := renderSnapshot(report, lock, "2026-08-13")

	wantSubstrings := []string{
		"2026-08-13",                       // run date
		"compliance-2026-08-13-v1",        // eval_set_version
		"glm-5.2",                         // model_ver
		"bd59cbe0",                        // prompt_hash short prefix
		"compliance",                      // skill name (from report.Skill)
		"Faithfulness",                    // column header
		"Context Precision",               // column header
		"Refusal Accuracy",                // column header
		"Cases",                           // column header
		"50%",                             // derived 1/2 refusal accuracy → 50% (1/2)
		"92%",                             // faithfulness 0.92 → 92%
		"85%",                             // context_precision 0.85 → 85%
		"go run ./cmd/eval --frozen=compliance", // reproduce command
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("snapshot missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderSnapshot_ZeroRefusalRequired_ShowsNA(t *testing.T) {
	// When a report has no refusal-required cases, the renderer must
	// surface "N/A" (not "0%") so the README never reads as "0%
	// refusal accuracy" for a fixture that simply has no such cases.
	report := eval.Report{
		Skill: "compliance",
		Overall: map[string]float64{
			"faithfulness":      0.9,
			"context_precision": 0.8,
		},
		Cases: []eval.CaseResult{
			{ID: "a1", Category: "adversarial", Refused: false},
		},
	}
	lock := eval.LockFile{
		ModelVersion:   "glm-5.2",
		PromptHash:     "abcdef0011223344",
		EvalSetVersion: "compliance-test-v0",
		SkillName:      "compliance",
	}
	out := renderSnapshot(report, lock, "2026-08-13")
	if !strings.Contains(out, "N/A") {
		t.Errorf("expected N/A in Refusal Accuracy cell when 0 refusal-required cases; got:\n%s", out)
	}
	if strings.Contains(out, "0% (") {
		t.Errorf("did not expect a percentage in Refusal Accuracy cell when 0 refusal-required cases; got:\n%s", out)
	}
}
