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
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestF1LoadGolden covers the jsonl loader: row parsing (the D-GT-01
// CONTEXT row shape), blank-line skipping, and the never-silent error
// posture (missing file / malformed line both carry filename + line).
func TestF1LoadGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sql-guides.jsonl")
	content := "{\"source_idx\":\"001\",\"tables\":[{\"name\":\"etl_a\",\"kind\":\"update\"}],\"related\":[\"006\"]}\n" +
		"\n" +
		"{\"source_idx\":\"002\",\"tables\":[],\"related\":[]}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadGolden(path)
	if err != nil {
		t.Fatalf("LoadGolden: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (blank line skipped)", len(rows))
	}
	if rows[0].SourceIdx != "001" || len(rows[0].Tables) != 1 ||
		rows[0].Tables[0].Name != "etl_a" || rows[0].Tables[0].Kind != "update" || len(rows[0].Related) != 1 || rows[0].Related[0] != "006" {
		t.Errorf("row 0 = %#v, want the D-GT-01 row shape", rows[0])
	}
	if rows[1].SourceIdx != "002" {
		t.Errorf("row 1 source_idx = %q, want 002", rows[1].SourceIdx)
	}

	// Malformed line → error naming the file and the 1-based line number.
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(bad, []byte("{\"source_idx\":\"001\"}\n{not json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGolden(bad); err == nil {
		t.Fatal("malformed jsonl line should error")
	} else if !strings.Contains(err.Error(), "bad.jsonl:2") {
		t.Errorf("error should carry filename:line, got: %v", err)
	}

	// Missing file → wrapped error, never a silent empty set.
	if _, err := LoadGolden(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Fatal("missing golden file should error")
	}
}

// TestF1ScoreTablesIdentity — identical sets score perfect precision,
// recall and F1; duplicate entries collapse (set semantics).
func TestF1ScoreTablesIdentity(t *testing.T) {
	golden := []ExtractedTable{
		{Name: "etl_a", Kind: "update"},
		{Name: "etl_b", Kind: "insert"},
	}
	p, r, f1 := ScoreTables(golden, golden)
	if p != 1.0 || r != 1.0 || f1 != 1.0 {
		t.Errorf("identical sets = P:%v R:%v F1:%v, want all 1.0", p, r, f1)
	}

	// Duplicate predicted entry for etl_a must not double-count the match.
	duped := []ExtractedTable{{Name: "etl_a", Kind: "update"}, {Name: "etl_a", Kind: "update"}, {Name: "etl_b", Kind: "insert"}}
	if p, r, f1 = ScoreTables(duped, golden); p != 1.0 || r != 1.0 || f1 != 1.0 {
		t.Errorf("deduped sets = P:%v R:%v F1:%v, want all 1.0", p, r, f1)
	}
}

// TestF1ScoreTablesKindMismatch — the kind is a secondary check: when both
// sides specify it, a disagreement is NOT a match; when either side leaves
// it empty, the name match alone counts.
func TestF1ScoreTablesKindMismatch(t *testing.T) {
	pred := []ExtractedTable{{Name: "etl_a", Kind: "select"}}
	golden := []ExtractedTable{{Name: "etl_a", Kind: "update"}}
	p, r, f1 := ScoreTables(pred, golden)
	if p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("kind mismatch = P:%v R:%v F1:%v, want all 0.0 (both kinds specified, disagree)", p, r, f1)
	}

	// Either side empty kind → name match counts.
	if p, r, f1 = ScoreTables([]ExtractedTable{{Name: "etl_a", Kind: ""}}, golden); p != 1.0 || r != 1.0 || f1 != 1.0 {
		t.Errorf("predicted kind empty = P:%v R:%v F1:%v, want all 1.0", p, r, f1)
	}
	if p, r, f1 = ScoreTables(pred, []ExtractedTable{{Name: "etl_a", Kind: ""}}); p != 1.0 || r != 1.0 || f1 != 1.0 {
		t.Errorf("golden kind empty = P:%v R:%v F1:%v, want all 1.0", p, r, f1)
	}

	// Mixed: partial hit — precision 0.5, recall 1.0, F1 = 2/3.
	p, r, f1 = ScoreTables(
		[]ExtractedTable{{Name: "etl_a", Kind: "update"}, {Name: "etl_zzz", Kind: "update"}},
		[]ExtractedTable{{Name: "etl_a", Kind: "update"}})
	if p != 0.5 || r != 1.0 {
		t.Errorf("partial hit = P:%v R:%v, want 0.5 / 1.0", p, r)
	}
	if want := 2 * 0.5 * 1.0 / 1.5; f1 != want {
		t.Errorf("partial hit F1 = %v, want %v", f1, want)
	}
}

// TestF1ScoreTablesEmptySides pins the documented zero-denominator
// conventions: undefined precision/recall are treated as 0.0.
func TestF1ScoreTablesEmptySides(t *testing.T) {
	golden := []ExtractedTable{{Name: "etl_a", Kind: "update"}}

	// Empty predicted vs non-empty golden: P undefined → 0.0, R = 0.0.
	if p, r, f1 := ScoreTables(nil, golden); p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("empty predicted = P:%v R:%v F1:%v, want all 0.0", p, r, f1)
	}
	// Non-empty predicted vs empty golden: P = 0.0, R undefined → 0.0.
	if p, r, f1 := ScoreTables(golden, nil); p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("empty golden = P:%v R:%v F1:%v, want all 0.0", p, r, f1)
	}
	// Both empty → 0.0 across the board (zero-score convention).
	if p, r, f1 := ScoreTables(nil, nil); p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("both empty = P:%v R:%v F1:%v, want all 0.0", p, r, f1)
	}
}

// TestF1ScoreRelated — the related-notes metric runs the identical set math
// over source_idx sets.
func TestF1ScoreRelated(t *testing.T) {
	pred := []RelatedNote{
		{SourceIdx: "006", Relation: "mentions-note"},
		{SourceIdx: "007", Relation: "mentions-note"},
	}
	golden := []string{"006", "008"}
	p, r, f1 := ScoreRelated(pred, golden)
	if p != 0.5 || r != 0.5 {
		t.Errorf("related = P:%v R:%v, want 0.5 / 0.5", p, r)
	}
	if want := 0.5; f1 != want {
		t.Errorf("related F1 = %v, want %v", f1, want)
	}

	// Duplicate predicted source_idx collapses: {006,006} → one predicted
	// entry which IS a golden match → precision 1.0, recall 0.5.
	if p, r, _ = ScoreRelated([]RelatedNote{{SourceIdx: "006"}, {SourceIdx: "006"}}, golden); p != 1.0 || r != 0.5 {
		t.Errorf("dup predicted = P:%v R:%v, want 1.0 / 0.5 (set semantics)", p, r)
	}
	// Empty sides score 0.0 by convention.
	if p, r, f1 = ScoreRelated(nil, []string{"006"}); p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("empty related predicted = P:%v R:%v F1:%v, want all 0.0", p, r, f1)
	}
}

// TestF1MacroAverage — macro-averaging is the mean over ALL notes; a note
// missing from predictions contributes a zero-score row (it is never
// skipped).
func TestF1MacroAverage(t *testing.T) {
	// Three notes: perfect, half, and a fully-missed note (0 rows — the
	// harness feeds NoteScore{0,0,0} for notes absent from predictions).
	got, gotR, gotF1 := MacroF1([]NoteScore{
		{P: 1.0, R: 1.0, F1: 1.0},
		{P: 0.5, R: 1.0, F1: 2.0 / 3.0},
		{P: 0.0, R: 0.0, F1: 0.0}, // missing note scores 0, not skipped
	})
	if want := (1.0 + 0.5 + 0.0) / 3.0; got != want {
		t.Errorf("macro P = %v, want %v", got, want)
	}
	if want := (1.0 + 1.0 + 0.0) / 3.0; gotR != want {
		t.Errorf("macro R = %v, want %v", gotR, want)
	}
	if want := (1.0 + 2.0/3.0 + 0.0) / 3.0; math.Abs(gotF1-want) > 1e-12 {
		t.Errorf("macro F1 = %v, want %v", gotF1, want)
	}

	// Empty input → zeros (never NaN).
	if p, r, f1 := MacroF1(nil); p != 0.0 || r != 0.0 || f1 != 0.0 {
		t.Errorf("empty macro = %v/%v/%v, want all 0.0", p, r, f1)
	}
}
