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

package ecl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// GoldenRow is one line of the D-GT-01 ground-truth jsonl
// (eval/ecl-golden/sql-guides.jsonl):
//
//	{"source_idx": "001", "tables": [{"name": "etl_payment_due_t", "kind": "update"}], "related": ["006"]}
//
// Tables is the per-note table-reference list (human-reviewed); Related is
// the list of source_idx values this note cross-references. The LLM never
// builds these rows (D-GT-01: regex draft + human review only).
type GoldenRow struct {
	SourceIdx string           `json:"source_idx"`
	Tables    []ExtractedTable `json:"tables"`
	Related   []string         `json:"related"`
}

// LoadGolden reads a line-delimited jsonl golden file. Blank lines are
// skipped; a malformed row aborts with the filename and 1-based line
// number. A missing or unreadable file is a wrapped error — never a silent
// empty set (SC-2's harness must not "pass" on a vanished golden file).
func LoadGolden(path string) ([]GoldenRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ecl: golden: load %s: %w", path, err)
	}
	var rows []GoldenRow
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var row GoldenRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("ecl: golden: %s:%d: unmarshal: %w", path, lineNo+1, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// tableSet collapses an ExtractedTable list into a name→kind map (first
// entry wins per name) — set semantics per RESEARCH Pattern 5.
func tableSet(tables []ExtractedTable) map[string]string {
	set := make(map[string]string, len(tables))
	for _, t := range tables {
		if _, ok := set[t.Name]; !ok {
			set[t.Name] = t.Kind
		}
	}
	return set
}

// ScoreTables compares a predicted table list against the golden table list
// for one note (SC-2's evaluation math, pure).
//
// Match rule: same table name AND (kinds equal OR either kind empty) — the
// kind is a secondary check that only bites when both sides specify it.
// Both sides are deduped by name before comparison.
//
// Conventions (documented): any zero denominator yields 0.0 for that
// metric — an undefined precision (empty prediction) or recall (empty
// golden) counts as 0.0, and F1 is 0 whenever P+R is 0. This makes
// missing/absent predictions score zero rather than being skipped.
func ScoreTables(predicted, golden []ExtractedTable) (precision, recall, f1 float64) {
	pred := tableSet(predicted)
	gold := tableSet(golden)
	matches := 0
	for name, kind := range pred {
		goldenKind, ok := gold[name]
		if !ok {
			continue
		}
		if kind != "" && goldenKind != "" && kind != goldenKind {
			continue
		}
		matches++
	}
	return setScores(matches, len(pred), len(gold))
}

// ScoreRelated compares predicted note-relations against the golden
// source_idx set for one note (same math as ScoreTables over sets of
// source_idx values; the relation tag is not scored — golden rows carry
// only ids).
func ScoreRelated(predicted []RelatedNote, golden []string) (precision, recall, f1 float64) {
	pred := make(map[string]bool, len(predicted))
	for _, r := range predicted {
		pred[r.SourceIdx] = true
	}
	gold := make(map[string]bool, len(golden))
	for _, g := range golden {
		gold[g] = true
	}
	matches := 0
	for idx := range pred {
		if gold[idx] {
			matches++
		}
	}
	return setScores(matches, len(pred), len(gold))
}

// setScores computes precision = |P∩G|/|P|, recall = |P∩G|/|G|,
// F1 = 2PR/(P+R) with the zero-denominator→0.0 convention.
func setScores(matches, predN, goldN int) (precision, recall, f1 float64) {
	if predN > 0 {
		precision = float64(matches) / float64(predN)
	}
	if goldN > 0 {
		recall = float64(matches) / float64(goldN)
	}
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}
	return precision, recall, f1
}

// NoteScore is one note's per-note (precision, recall, F1) triple feeding
// the macro-average.
type NoteScore struct {
	P  float64
	R  float64
	F1 float64
}

// MacroF1 macro-averages per-note scores: the plain arithmetic mean over
// every note. Notes absent from predictions must be fed in as zero-score
// rows by the caller (they count toward the mean — they are never skipped),
// which is what keeps recall honest when the extractor misses whole notes.
// Empty input returns zeros (never NaN).
func MacroF1(perNote []NoteScore) (p, r, f1 float64) {
	if len(perNote) == 0 {
		return 0, 0, 0
	}
	var sumP, sumR, sumF1 float64
	for _, n := range perNote {
		sumP += n.P
		sumR += n.R
		sumF1 += n.F1
	}
	n := float64(len(perNote))
	return sumP / n, sumR / n, sumF1 / n
}
