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

// Phase 39 / Plan 39-09 Task 3 — SC-2 harness gate: the deterministic stub
// extractor scored against the HUMAN-REVIEWED golden set (D-GT-01). The
// number this test prints is a HARNESS check (does the stub + golden +
// scoring pipeline wire up correctly), not an extractor-quality claim —
// human corrections legitimately pull the score below 1.0, so 0.8 is the
// gate (SC-2/SC-6), not 1.0.
package ecl

import "testing"

// goldenPath is the committed D-GT-01 golden set relative to this package
// dir (go test runs with cwd = package source dir). The file is committed
// by plan 39-09 Task 3 — the harness HARD-REQUIRES it: skip-without-file
// would silently void SC-2's yardstick, so a missing/unreadable file is a
// t.Fatal via LoadGolden's error path, never a skip.
const goldenPath = "../../../eval/ecl-golden/sql-guides.jsonl"

// pinnedGoldenRows is the reviewed golden-set size. The reviewed set has
// exactly one row per merged corpus note (D-GT-01: 26 rows over 26 notes).
const pinnedGoldenRows = 26

// harnessF1Threshold is the SC-2/SC-6 harness gate for both metrics.
const harnessF1Threshold = 0.8

// agreeOrScore implements the harness scoring convention: when BOTH sides
// are empty, extractor and golden AGREE on absence — a true negative,
// scored as perfect agreement (1, 1, 1).
//
// This deliberately differs from ScoreTables/ScoreRelated's pinned
// zero-denominator convention (TestF1ScoreTablesEmptySides: any empty side
// scores 0.0). That convention exists for the LLM gate, where "predicted
// nothing" must never be rewardable. The harness asks a different question
// — does the stub match the reviewed golden? — and for the read-only notes
// the joint emptiness IS the correct answer: the reviewed golden is
// writes-only (policy A, 2026-09-22 sign-off) and the stub's write-shaped
// rules deterministically emit nothing for those notes. Scoring that
// agreement 0.0 would measure nothing but the corpus's read/write ratio.
func agreeOrScore(predN, goldN int, score func() NoteScore) NoteScore {
	if predN == 0 && goldN == 0 {
		return NoteScore{P: 1, R: 1, F1: 1}
	}
	return score()
}

// TestStubVsGoldenF1 loads the committed golden set, runs StubExtract over
// the real merged corpus, and asserts macro-F1 >= 0.8 for BOTH the tables
// and related metrics (SC-2 harness gate; SC-6 wires this into
// scripts/verify-phase-39.sh). Exact values are always logged.
func TestStubVsGoldenF1(t *testing.T) {
	golden, err := LoadGolden(goldenPath)
	if err != nil {
		t.Fatalf("golden set is REQUIRED by the SC-2 harness (committed by 39-09; it is never optional): %v", err)
	}
	if len(golden) != pinnedGoldenRows {
		t.Fatalf("golden row count = %d, want %d — corpus/golden drift: re-run `go run ./cmd/ecl-draft-golden -kb knowledge/sql-guides` and redo the D-GT-01 human review if knowledge/sql-guides changed (golden-review-checklist.md)",
			len(golden), pinnedGoldenRows)
	}

	notes, _, err := WalkAndMerge(testCorpusDir)
	if err != nil {
		t.Fatalf("WalkAndMerge: %v", err)
	}
	byIdx := make(map[string]Note, len(notes))
	for _, n := range notes {
		byIdx[n.SourceIdx] = n
	}

	var tableScores, relatedScores []NoteScore
	for _, row := range golden {
		note, ok := byIdx[row.SourceIdx]
		if !ok {
			t.Fatalf("golden row %q has no matching corpus note (%d corpus notes vs %d golden rows — corpus drift; re-draft and re-review)", row.SourceIdx, len(notes), len(golden))
		}
		pred := StubExtract(note)

		ts := agreeOrScore(len(pred.Tables), len(row.Tables), func() NoteScore {
			p, r, f1 := ScoreTables(pred.Tables, row.Tables)
			return NoteScore{P: p, R: r, F1: f1}
		})
		tableScores = append(tableScores, ts)

		relatedScores = append(relatedScores, agreeOrScore(len(pred.RelatedNotes), len(row.Related), func() NoteScore {
			p, r, f1 := ScoreRelated(pred.RelatedNotes, row.Related)
			return NoteScore{P: p, R: r, F1: f1}
		}))

		// Per-note transparency: any note below perfect tables agreement
		// is logged with its numbers (reviewed corrections legitimately
		// produce these — e.g. note 039's CREATE FUNCTION false positives
		// the human review deleted from the golden).
		if ts.F1 < 1.0 {
			t.Logf("note %s: tables P=%.4f R=%.4f F1=%.4f (stub %d tables vs golden %d)",
				row.SourceIdx, ts.P, ts.R, ts.F1, len(pred.Tables), len(row.Tables))
		}
	}

	// Corpus-side completeness: with every golden row matched to a corpus
	// note above, equal sizes make the mapping a bijection — a corpus note
	// WITHOUT a golden row would silently escape scoring.
	if len(byIdx) != len(golden) {
		t.Fatalf("corpus note count = %d but golden rows = %d — corpus drift: every note must have a golden row (re-draft + re-review)", len(byIdx), len(golden))
	}

	tp, tr, tf1 := MacroF1(tableScores)
	rp, rr, rf1 := MacroF1(relatedScores)
	t.Logf("SC-2 harness macro-F1 vs reviewed golden (%d notes): tables P=%.4f R=%.4f F1=%.4f | related P=%.4f R=%.4f F1=%.4f",
		len(golden), tp, tr, tf1, rp, rr, rf1)

	if tf1 < harnessF1Threshold {
		t.Fatalf("tables macro-F1 = %.4f < %.2f harness gate (SC-2/SC-6) — golden, stub, or scoring drifted; inspect the per-note log above", tf1, harnessF1Threshold)
	}
	if rf1 < harnessF1Threshold {
		t.Fatalf("related macro-F1 = %.4f < %.2f harness gate (SC-2/SC-6) — golden, stub, or scoring drifted", rf1, harnessF1Threshold)
	}
}
