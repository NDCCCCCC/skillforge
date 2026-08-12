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

package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/skillforge/skillforge/internal/retriever"
)

// Case is one row of the golden-set JSONL. The schema is the
// eval/golden/compliance.jsonl wire contract — every row has exactly
// these 5 fields (a 6th optional field, "source", tags provenance:
// "spike-anchor" marks the 5 cases reused from cmd/spiked/e2e.go:76-82).
//
// Category is one of:
//
//   - "adversarial"      — questions where the LLM is tempted to
//                          hallucinate. Faithfulness most differs here.
//   - "refusal-required" — questions that MUST be refused (out-of-scope,
//                          PII, or insufficient context). The moat's
//                          headline behavior (over-weighted per GEVL-05).
//   - "multi-hop"        — questions requiring 2+ retrievals. Stress
//                          context_recall.
//
// ExpectedChunks is the list of chunk IDs (decimal strings) the
// retriever SHOULD surface for this query. Empty for refusal-required
// cases (no expected grounding). Used by an optional stricter context_
// recall variant; the production ContextRecall uses groundTruth
// statements, not chunk IDs.
type Case struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Query          string   `json:"query"`
	GroundTruth    string   `json:"ground_truth"`
	ExpectedChunks []string `json:"expected_chunks"`
}

// Sample is a Case after the agent has produced a verdict. The agent's
// answer (verdict JSON), the contexts (retrieved chunks' contents) and
// the refusal reason are filled in by cmd/eval before the metrics run.
//
// RefusalReason is "" when the agent answered, or the verdict.Refusal
// reason code ("no_authorized_source", "low_retrieval_score", ...) when
// the agent refused. The metric functions consume the (Answer,
// Contexts, RefusalReason) tuple.
type Sample struct {
	Case
	Answer        string
	Contexts      []string
	RefusalReason string
}

// CaseResult is the per-case metric output in a Report. Carries the
// Case ID + Category (so the report is self-describing without a join
// back to the golden set) and the 4 metric scores.
type CaseResult struct {
	ID                string  `json:"id"`
	Category          string  `json:"category"`
	Faithfulness      float64 `json:"faithfulness"`
	ContextPrecision  float64 `json:"context_precision"`
	AnswerRelevance   float64 `json:"answer_relevance"`
	ContextRecall     float64 `json:"context_recall"`
	Refused           bool    `json:"refused"`
}

// Report is the top-level JSON written by WriteReport. The Overall map
// holds mean scores across all cases; the PerCategory map holds mean
// scores per category (adversarial / refusal-required / multi-hop).
// Cases carries the per-case detail so downstream tooling (the Phase 6
// CI promotion gate) can sort/filter/aggregate.
type Report struct {
	Overall     map[string]float64            `json:"overall"`
	PerCategory map[string]map[string]float64 `json:"per_category"`
	Cases       []CaseResult                  `json:"cases"`
}

// SampleToResult converts a Sample + its 4 metric scores into a
// CaseResult for the Report. Kept as a free function (not a Sample
// method) so the metric scores can be computed independently and
// assembled at the call site.
func SampleToResult(s Sample, faith, cp, ar, cr float64) CaseResult {
	return CaseResult{
		ID:               s.ID,
		Category:         s.Category,
		Faithfulness:     faith,
		ContextPrecision: cp,
		AnswerRelevance:  ar,
		ContextRecall:    cr,
		Refused:          s.RefusalReason != "",
	}
}

// HitsToContexts flattens a retriever.Hit slice into the []string the
// metrics consume (one entry per chunk's Content). Helper used by
// cmd/eval before calling Faithfulness/ContextRecall.
func HitsToContexts(hits []retriever.Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Content)
	}
	return out
}

// LoadGolden reads a golden-set JSONL file. Each non-empty line is one
// Case. Lines starting with '#' (after TrimSpace) are skipped as
// comments — the golden file is JSONL, but a leading comment block is
// a useful documentation surface. The file MUST be UTF-8 (the same
// assumption cmd/server makes for skill files).
//
// Returns an error if:
//   - the file cannot be opened.
//   - any non-comment line fails to parse as JSON.
//   - zero cases are loaded (an empty golden set is a caller bug).
func LoadGolden(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("eval.LoadGolden: open %q: %w", path, err)
	}
	defer f.Close()

	var cases []Case
	scanner := bufio.NewScanner(f)
	// Bump the per-line token budget — some ground_truth values carry
	// multi-statement Chinese strings.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("eval.LoadGolden: line %d: parse JSON: %w", lineNo, err)
		}
		if c.ID == "" {
			return nil, fmt.Errorf("eval.LoadGolden: line %d: case missing id", lineNo)
		}
		if c.Category == "" {
			return nil, fmt.Errorf("eval.LoadGolden: line %d: case %q missing category", lineNo, c.ID)
		}
		if strings.TrimSpace(c.Query) == "" {
			return nil, fmt.Errorf("eval.LoadGolden: line %d: case %q missing query", lineNo, c.ID)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("eval.LoadGolden: scan: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("eval.LoadGolden: %q loaded 0 cases (expected >=1)", path)
	}
	return cases, nil
}

// WriteReport serializes a Report to path as pretty-printed JSON.
// Creates the file (truncating any prior copy — the report is
// overwritten on every eval run; keep historical runs in git or
// a separate archive directory).
func WriteReport(path string, r Report) error {
	if path == "" {
		return fmt.Errorf("eval.WriteReport: path is empty")
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("eval.WriteReport: marshal: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("eval.WriteReport: write %q: %w", path, err)
	}
	return nil
}
