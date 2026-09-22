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
	"reflect"
	"strings"
	"testing"
)

// noteWith is a test helper building a single-section note whose body is
// the given text.
func noteWith(sourceIdx, body string) Note {
	return Note{
		SourceIdx: sourceIdx,
		Title:     "t",
		Sections:  []MergedSection{{Section: "SQL / 正文", SectionIdx: 0, Body: body}},
	}
}

// TestStubExtract covers every D-Ext-01 kind-mapping bullet: line-anchored
// SQL statement detection over the merged note body, the pinned kind enum
// (delete-from / merge-into normalize into "update"), the etl_ corpus
// prefix gate, case preservation, and first-seen dedupe.
func TestStubExtract(t *testing.T) {
	tests := []struct {
		name string
		note Note
		want []ExtractedTable
	}{
		{
			name: "update at line start",
			note: noteWith("001", "UPDATE etl_payment_due_t SET a = 1;\n"),
			want: []ExtractedTable{{Name: "etl_payment_due_t", Kind: "update"}},
		},
		{
			name: "lowercase update with leading whitespace",
			note: noteWith("001", "  \tupdate etl_payment_due_t set b = 2\n"),
			want: []ExtractedTable{{Name: "etl_payment_due_t", Kind: "update"}},
		},
		{
			name: "insert into maps to insert",
			note: noteWith("001", "INSERT INTO etl_x_t VALUES (1);\n"),
			want: []ExtractedTable{{Name: "etl_x_t", Kind: "insert"}},
		},
		{
			name: "create or replace maps to create (corpus shape: schema-qualified name)",
			note: noteWith("001", "CREATE OR REPLACE PROCEDURE wuh_dw.etl_y_t(batch varchar)\nAS BEGIN\nEND;\n"),
			want: []ExtractedTable{{Name: "etl_y_t", Kind: "create"}},
		},
		{
			name: "delete from normalizes to update kind (delete NOT in enum, per D-Ext-01 pinning)",
			note: noteWith("001", "DELETE FROM etl_z_t WHERE id = 1;\n"),
			want: []ExtractedTable{{Name: "etl_z_t", Kind: "update"}},
		},
		{
			name: "merge into normalizes to update kind",
			note: noteWith("001", "  merge into etl_nonauto_policy_t a using b on (a.id = b.id)\n"),
			want: []ExtractedTable{{Name: "etl_nonauto_policy_t", Kind: "update"}},
		},
		{
			name: "select into maps to select",
			note: noteWith("001", "SELECT * INTO etl_s_t FROM src;\n"),
			want: []ExtractedTable{{Name: "etl_s_t", Kind: "select"}},
		},
		{
			name: "non-etl tables are not emitted (corpus prefix gate)",
			note: noteWith("001", "update other_table set a = 1;\ninsert into users values (1);\n"),
			want: []ExtractedTable{},
		},
		{
			name: "case preserved: etl_NONAUTO_POLICY_T stays uppercase",
			note: noteWith("001", "merge into etl_NONAUTO_POLICY_T a\n"),
			want: []ExtractedTable{{Name: "etl_NONAUTO_POLICY_T", Kind: "update"}},
		},
		{
			name: "deduped: same table hit twice yields one entry",
			note: noteWith("001", "update etl_payment_due_t set a = 1;\nupdate etl_payment_due_t set b = 2;\n"),
			want: []ExtractedTable{{Name: "etl_payment_due_t", Kind: "update"}},
		},
		{
			name: "first-seen order across kinds",
			note: noteWith("001", "insert into etl_b_t values (1);\nupdate etl_a_t set x = 1;\ninsert into etl_b_t values (2);\n"),
			want: []ExtractedTable{
				{Name: "etl_b_t", Kind: "insert"},
				{Name: "etl_a_t", Kind: "update"},
			},
		},
		{
			name: "multi-section merge: statement at start of a later section is visible (merge appends \\n after each fragment body)",
			note: Note{
				SourceIdx: "001",
				Title:     "t",
				Sections: []MergedSection{
					{Section: "说明", SectionIdx: 0, Body: "tail text without newline\n"},
					{Section: "SQL / 正文", SectionIdx: 1, Body: "update etl_payment_due_t set a = 1;\n"},
				},
			},
			want: []ExtractedTable{{Name: "etl_payment_due_t", Kind: "update"}},
		},
		{
			name: "mid-line statement is not matched (line-anchored discipline)",
			note: noteWith("001", "select 1 from dual; -- update etl_x_t in a comment\n"),
			want: []ExtractedTable{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StubExtract(tt.note)
			if !reflect.DeepEqual(got.Tables, tt.want) {
				t.Fatalf("StubExtract tables = %+v, want %+v", got.Tables, tt.want)
			}
		})
	}
}

// TestStubRelatedNotes pins the explicit numeric cross-reference pattern:
// a note relates to note N when its body contains 参考/见-style text
// followed by N's 3-digit source_idx. Self-references are excluded and
// duplicates collapse.
func TestStubRelatedNotes(t *testing.T) {
	note := noteWith("001", "背景见 006，字段口径详见 006，其余参见 041。\n")
	got := StubExtract(note)
	want := []RelatedNote{
		{SourceIdx: "006", Relation: "mentions-note"},
		{SourceIdx: "041", Relation: "mentions-note"},
	}
	if !reflect.DeepEqual(got.RelatedNotes, want) {
		t.Fatalf("RelatedNotes = %+v, want %+v", got.RelatedNotes, want)
	}

	self := noteWith("006", "本文档即 006 号说明，自指不算关联。\n")
	if got := StubExtract(self); len(got.RelatedNotes) != 0 {
		t.Fatalf("self-reference must be excluded, got %+v", got.RelatedNotes)
	}

	// End-to-end through the real merge: fragment boundaries must not glue
	// the reference word into the previous fragment's tail (merge appends
	// "\n" after each non-blank fragment body).
	frag1 := Fragment{Path: "a.md", SecOrder: 1, SourceIdx: "001", Section: "说明", Body: "背景说明，参见"}
	frag2 := Fragment{Path: "b.md", SecOrder: 2, SourceIdx: "001", Section: "说明", Body: "006 的口径。"}
	merged := MergeFragments([]Fragment{frag1, frag2})
	if got := StubExtract(merged[0]); len(got.RelatedNotes) != 1 || got.RelatedNotes[0].SourceIdx != "006" {
		t.Fatalf("merged-note cross-fragment reference not detected: %+v", got.RelatedNotes)
	}
}

// TestExtractionJSONShape asserts the stub output unmarshals into the
// exact D-Ext-01 wire shape:
// {"tables":[{"name","kind"}], "related_notes":[{"source_idx","relation"}]}
func TestExtractionJSONShape(t *testing.T) {
	ex := StubExtract(noteWith("001", "update etl_payment_due_t set a = 1;\n详见 006。\n"))
	b, err := json.Marshal(ex)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if len(raw) != 2 {
		t.Fatalf("top-level keys = %v, want exactly [tables related_notes]", raw)
	}
	if _, ok := raw["tables"]; !ok {
		t.Fatalf("missing tables key: %s", b)
	}
	if _, ok := raw["related_notes"]; !ok {
		t.Fatalf("missing related_notes key: %s", b)
	}
	var tables []map[string]json.RawMessage
	if err := json.Unmarshal(raw["tables"], &tables); err != nil {
		t.Fatalf("unmarshal tables: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("tables = %s", raw["tables"])
	}
	for _, key := range []string{"name", "kind"} {
		if _, ok := tables[0][key]; !ok {
			t.Fatalf("table object missing %q key: %s", key, raw["tables"])
		}
	}
	if len(tables[0]) != 2 {
		t.Fatalf("table object has extra keys: %s", raw["tables"])
	}
}

// TestValidateExtraction pins the kind-enum gate Plan 04 reuses on LLM
// output: only update|select|insert|create are legal.
func TestValidateExtraction(t *testing.T) {
	for _, kind := range []string{"update", "select", "insert", "create"} {
		ex := Extraction{Tables: []ExtractedTable{{Name: "etl_x_t", Kind: kind}}}
		if err := ValidateExtraction(ex); err != nil {
			t.Errorf("ValidateExtraction kind %q: unexpected error %v", kind, err)
		}
	}
	for _, kind := range []string{"delete", "merge", "drop", "", "UPDATE"} {
		ex := Extraction{Tables: []ExtractedTable{{Name: "etl_x_t", Kind: kind}}}
		if err := ValidateExtraction(ex); err == nil {
			t.Errorf("ValidateExtraction kind %q: want error, got nil", kind)
		}
	}
	if err := ValidateExtraction(Extraction{}); err != nil {
		t.Errorf("empty extraction should validate, got %v", err)
	}
}

// ---- Corpus-backed boundary tests (SC-1 proof) ----
//
// These run against the REAL in-repo corpus at knowledge/sql-guides,
// hermetically: file reads only, no network / LLM / Milvus (same posture
// as scripts/verify-phase-38.sh). The pinned SC numbers (185 fragments /
// 26 notes / 69 section records) were validated by spike 005c on
// 2026-09-21; the dynamic contribution-completeness assertion guards
// against corpus drift turning a real regression into a confusing number
// mismatch (RESEARCH anti-pattern: never hardcode the SC numbers without
// the dynamic cross-check).

// testCorpusDir is the sql-guides corpus relative to this package dir
// (go test runs with cwd = package source dir).
const testCorpusDir = "../../../knowledge/sql-guides"

// TestMergeBoundary proves the chunker's boundary guarantee:
//   - (a) DYNAMIC: fragment-contribution completeness — every walked
//     __secNN fragment contributes to exactly one merged section
//     (sum of len(SourcePaths) over all notes/sections == fragment count).
//   - (b) PINNED: the SC-1 corpus shape (185 fragments → 26 notes).
//   - (c) PINNED: 69 section records (the ChunkMeta count).
//   - (d) every ChunkMeta is internally consistent.
func TestMergeBoundary(t *testing.T) {
	frags, err := WalkFragments(testCorpusDir)
	if err != nil {
		t.Fatalf("WalkFragments: %v", err)
	}
	notes := MergeFragments(frags)
	metas := ChunkMetas(notes)

	// (a) DYNAMIC — contribution completeness. The assertion is 100%,
	// not the constant: every fragment walked must contribute exactly one
	// SourcePaths entry across the merged output.
	totalContrib := 0
	for _, n := range notes {
		for _, s := range n.Sections {
			totalContrib += len(s.SourcePaths)
		}
	}
	t.Logf("contribution completeness: %d/%d fragment-contributions across %d notes, %d section records",
		totalContrib, len(frags), len(notes), len(metas))
	if totalContrib != len(frags) {
		t.Fatalf("contribution completeness broken: %d fragments walked but %d fragment-contributions across merged sections (every fragment must map to exactly one merged section)", len(frags), totalContrib)
	}

	// (b) PINNED — SC-1 corpus shape. If these fail while (a) passes, the
	// corpus drifted: re-verify counts if knowledge/ changed.
	if len(frags) != 185 {
		t.Fatalf("pinned fragment count drifted: got %d, want 185 (corpus drift — re-verify counts if knowledge/ changed)", len(frags))
	}
	if len(notes) != 26 {
		t.Fatalf("pinned note count drifted: got %d, want 26 (corpus drift — re-verify counts if knowledge/ changed)", len(notes))
	}

	// (c) PINNED — section records.
	if len(metas) != 69 {
		t.Fatalf("pinned section-record count drifted: got %d, want 69 (corpus drift — re-verify counts if knowledge/ changed)", len(metas))
	}

	// (d) ChunkMeta internal consistency.
	for _, n := range notes {
		if n.SourceIdx == "" {
			t.Fatalf("note with empty source_idx (title %q)", n.Title)
		}
		for _, s := range n.Sections {
			if s.Section == "" {
				t.Fatalf("note %s has a section with empty name", n.SourceIdx)
			}
			if s.SectionIdx < 0 || s.SectionIdx >= len(n.Sections) {
				t.Fatalf("note %s section %q SectionIdx %d outside [0,%d)", n.SourceIdx, s.Section, s.SectionIdx, len(n.Sections))
			}
			if len(s.SourcePaths) == 0 {
				t.Fatalf("note %s section %q has no source paths", n.SourceIdx, s.Section)
			}
			for _, p := range s.SourcePaths {
				if p == "" {
					t.Fatalf("note %s section %q has an empty source path", n.SourceIdx, s.Section)
				}
			}
			if len(s.Body) == 0 {
				t.Fatalf("note %s section %q has an empty merged body", n.SourceIdx, s.Section)
			}
		}
	}
	for _, m := range metas {
		if m.SourceIdx == "" || m.Section == "" {
			t.Fatalf("ChunkMeta with empty source_idx/section: %+v", m)
		}
		if m.SectionIdx < 0 || m.SectionIdx >= m.Sections {
			t.Fatalf("ChunkMeta %s/%s SectionIdx %d outside [0,%d)", m.SourceIdx, m.Section, m.SectionIdx, m.Sections)
		}
		if len(m.SourcePaths) == 0 {
			t.Fatalf("ChunkMeta %s/%s has empty source_paths", m.SourceIdx, m.Section)
		}
		if m.BodyBytes != bodyBytesFor(t, notes, m) {
			t.Fatalf("ChunkMeta %s/%s BodyBytes %d != merged body length", m.SourceIdx, m.Section, m.BodyBytes)
		}
	}
}

// bodyBytesFor finds the merged section a ChunkMeta was built from and
// returns its body byte length, proving BodyBytes == len(Body).
func bodyBytesFor(t *testing.T, notes []Note, m ChunkMeta) int {
	t.Helper()
	for _, n := range notes {
		if n.SourceIdx != m.SourceIdx {
			continue
		}
		for _, s := range n.Sections {
			if s.SectionIdx == m.SectionIdx && s.Section == m.Section {
				return len(s.Body)
			}
		}
	}
	t.Fatalf("no merged section backs ChunkMeta %s/%s idx %d", m.SourceIdx, m.Section, m.SectionIdx)
	return 0
}

// TestNote001SQL proves SC-1's SQL-completeness claim: note 001's merged
// body (all sections concatenated) contains all 3 corpus SQL target tables.
func TestNote001SQL(t *testing.T) {
	notes, _, err := WalkAndMerge(testCorpusDir)
	if err != nil {
		t.Fatalf("WalkAndMerge: %v", err)
	}
	var note001 *Note
	for i := range notes {
		if notes[i].SourceIdx == "001" {
			note001 = &notes[i]
			break
		}
	}
	if note001 == nil {
		t.Fatal("note 001 not found in merged output")
	}
	var body strings.Builder
	for _, s := range note001.Sections {
		body.WriteString(s.Body)
	}
	full := strings.ToLower(body.String())
	targets := []string{"etl_payment_due_t", "etl_policy_request_issue_t", "etl_NONAUTO_POLICY_T"}
	found := 0
	var missing []string
	for _, tgt := range targets {
		if strings.Contains(full, strings.ToLower(tgt)) {
			found++
		} else {
			missing = append(missing, tgt)
		}
	}
	if found != len(targets) {
		t.Fatalf("note 001 contains %d/%d corpus SQL targets, missing: %v", found, len(targets), missing)
	}
	t.Logf("note 001 (title %q, %d sections): %d/%d corpus SQL targets present",
		note001.Title, len(note001.Sections), found, len(targets))
}

// TestSec01PathologyTolerated documents the known upstream data-quality
// pathology (README 005c, note 001's sec01 has section == title): the
// chunker merges the note without error and tolerates the redundant
// title-section (CONTEXT §specifics: 照常合并, 多一个冗余 section 无害).
func TestSec01PathologyTolerated(t *testing.T) {
	notes, _, err := WalkAndMerge(testCorpusDir)
	if err != nil {
		t.Fatalf("WalkAndMerge must not error on the sec01 pathology: %v", err)
	}
	var note001 *Note
	for i := range notes {
		if notes[i].SourceIdx == "001" {
			note001 = &notes[i]
			break
		}
	}
	if note001 == nil {
		t.Fatal("note 001 not found in merged output")
	}
	if len(note001.Sections) == 0 {
		t.Fatal("note 001 merged with zero sections")
	}
	// The pathology signature: sec01's section value equals the note title,
	// producing a redundant first section. Documented non-fix — merging
	// proceeds anyway.
	if note001.Sections[0].Section != note001.Title {
		t.Fatalf("expected sec01 section==title pathology signature on note 001: section %q != title %q", note001.Sections[0].Section, note001.Title)
	}
	// README 005c: note 001 merges to 3 sections (title-section / 说明 /
	// SQL / 正文) with the pathology tolerated.
	if len(note001.Sections) != 3 {
		t.Fatalf("note 001 sections = %d, want 3 (title-section pathology tolerated as a redundant section)", len(note001.Sections))
	}
}

// TestChunkMetaContractShape asserts the ChunkMeta JSON key set equals the
// spike 005c contract exactly (D-ECL-02: byte-for-key fidelity with
// chunk_metadata.json).
func TestChunkMetaContractShape(t *testing.T) {
	notes := []Note{
		{
			SourceIdx: "001",
			Title:     "t",
			Sections: []MergedSection{
				{Section: "SQL / 正文", SectionIdx: 0, Body: "body", SourcePaths: []string{"a.md"}},
			},
		},
	}
	meta := ChunkMetas(notes)[0]
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal ChunkMeta: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal ChunkMeta: %v", err)
	}
	want := []string{
		"source_idx", "title", "section", "section_idx",
		"section_count", "source_paths", "body_bytes",
	}
	if len(raw) != len(want) {
		t.Fatalf("ChunkMeta JSON has %d keys (%v), want exactly %v", len(raw), raw, want)
	}
	for _, key := range want {
		if _, ok := raw[key]; !ok {
			t.Fatalf("ChunkMeta JSON missing contract key %q: %s", key, b)
		}
	}
}
