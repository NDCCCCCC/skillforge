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
