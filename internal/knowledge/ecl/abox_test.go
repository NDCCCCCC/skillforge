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
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/ontology/emitter"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

const testNS = "http://skillforge.local/ontology#"

// TestABoxNoteAndTableIRI pins the D-Ext-01 IRI forms: skf:note_{idx} and
// skf:tbl_{sanitized name}, with the sanitizer coming from the emitter
// package (single source of truth — no second sanitizer).
func TestABoxNoteAndTableIRI(t *testing.T) {
	if got := NoteIRI(testNS, "001"); got != testNS+"note_001" {
		t.Errorf("NoteIRI = %q, want %q", got, testNS+"note_001")
	}
	name := "etl_NONAUTO_POLICY_T"
	if got := TableIRI(testNS, name); got != testNS+"tbl_"+emitter.SanitizeLocal(name) {
		t.Errorf("TableIRI = %q, want emitter-sanitized form", got)
	}

	// WR-03 collision guard: a name the sanitizer actually rewrites carries
	// a stable hash suffix of the original input — the sanitized base shape
	// stays recognizable, but distinct inputs collapsing to the same shape
	// can no longer merge into one IRI.
	dirty := "etl_weird name.99"
	got := TableIRI(testNS, dirty)
	base := testNS + "tbl_etl_weird_name_99_"
	if !strings.HasPrefix(got, base) {
		t.Errorf("TableIRI(%q) = %q, want the %q<hash8> collision-guarded form", dirty, got, base)
	}
	if len(got) != len(base)+8 {
		t.Errorf("TableIRI(%q) = %q, want base + 8 hex hash chars", dirty, got)
	}
	// A name that already matches the character policy is verbatim (no
	// suffix) — the gated LLM path never pays the collision tax.
	clean := TableIRI(testNS, "etl_weird_name_99")
	if clean != testNS+"tbl_etl_weird_name_99" {
		t.Errorf("TableIRI(clean) = %q, want verbatim %q", clean, testNS+"tbl_etl_weird_name_99")
	}
}

// TestABoxBuildTriples pins the exact D-Ext-01 triple forms and order for a
// two-table extraction: note type, per-table referencesTable, per-table
// TableSchema type triple. Provenance is attached to every triple (D-PROV-11).
// NO belongsToNote triples, NO related_notes triples (D-TBox2-01 property
// set + open-question resolution #1).
func TestABoxBuildTriples(t *testing.T) {
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides",
		Sections: []MergedSection{{Body: "body", SourcePaths: []string{"knowledge/sql-guides/001__sec01.md"}}},
		StartLine: 10, EndLine: 50}
	ex := Extraction{
		Tables: []ExtractedTable{
			{Name: "etl_a", Kind: "update"},
			{Name: "etl_b", Kind: "insert"},
		},
		RelatedNotes: []RelatedNote{{SourceIdx: "006", Relation: "mentions-note"}},
	}
	got := BuildTriples(testNS, note, ex)

	// Verify we get 8 triples: note-type + belongsToTier + 2x(ref + schema + tbl-belongsToTier)
	if len(got) != 8 {
		t.Fatalf("BuildTriples returned %d triples, want 8", len(got))
	}

	// Triple 0: note type + provenance
	tr := got[0]
	if tr.S != testNS+"note_001" || tr.P != rdfTypeIRI || tr.O != testNS+"NoteChunk" {
		t.Errorf("triple[0] = %+v, want note type triple", tr)
	}
	if tr.Provenance == nil {
		t.Error("triple[0] provenance is nil, want non-nil")
	} else {
		if tr.Provenance.SourceKind != "file" {
			t.Errorf("provenance source_kind = %q, want file", tr.Provenance.SourceKind)
		}
		if tr.Provenance.SourcePath != "knowledge/sql-guides/001__sec01.md" {
			t.Errorf("provenance source_path = %q, want first source path", tr.Provenance.SourcePath)
		}
		if tr.Provenance.LineStart != 10 || tr.Provenance.LineEnd != 50 {
			t.Errorf("provenance line range = %d-%d, want 10-50", tr.Provenance.LineStart, tr.Provenance.LineEnd)
		}
	}

	// Triple 1: note belongsToTier
	tr1 := got[1]
	if tr1.S != testNS+"note_001" || tr1.P != testNS+"belongsToTier" || tr1.O != testNS+"sql-guides" {
		t.Errorf("triple[1] = %+v, want belongsToTier triple", tr1)
	}
	if tr1.Provenance == nil {
		t.Error("triple[1] provenance is nil, want non-nil (shared with note)")
	}

	// Triples 2-3: etl_a (ref + schema)
	if got[2].S != testNS+"note_001" || got[2].P != testNS+"referencesTable" || got[2].O != testNS+"tbl_etl_a" {
		t.Errorf("triple[2] = %+v, want etl_a referencesTable", got[2])
	}
	if got[3].S != testNS+"tbl_etl_a" || got[3].P != rdfTypeIRI || got[3].O != testNS+"TableSchema" {
		t.Errorf("triple[3] = %+v, want etl_a TableSchema type", got[3])
	}

	// Triple 4: etl_a belongsToTier
	tr4 := got[4]
	if tr4.S != testNS+"tbl_etl_a" || tr4.P != testNS+"belongsToTier" || tr4.O != testNS+"sql-guides" {
		t.Errorf("triple[4] = %+v, want etl_a belongsToTier", tr4)
	}

	// Triples 5-6: etl_b
	if got[5].S != testNS+"note_001" || got[5].P != testNS+"referencesTable" || got[5].O != testNS+"tbl_etl_b" {
		t.Errorf("triple[5] = %+v, want etl_b referencesTable", got[5])
	}
	if got[6].S != testNS+"tbl_etl_b" || got[6].P != rdfTypeIRI || got[6].O != testNS+"TableSchema" {
		t.Errorf("triple[6] = %+v, want etl_b TableSchema type", got[6])
	}

	// Duplicate table entries dedupe within the note (TableSchema type
	// triple and referencesTable emitted once per distinct name).
	dup := BuildTriples(testNS, note, Extraction{Tables: []ExtractedTable{
		{Name: "etl_a", Kind: "update"},
		{Name: "etl_a", Kind: "update"},
	}})
	// 1 note-type + 1 belongsToTier + 1 ref + 1 schema + 1 tbl-belongsToTier = 5
	if len(dup) != 5 {
		t.Errorf("dup-name triples = %d, want 5 (type + tier + ref + schema + tbl-tier)", len(dup))
	}
}

// TestABoxApplyNoteIdempotent — re-running ApplyNote for the same note
// replaces in place: Len() after the second run equals Len() after the
// first (D-Run-01 idempotency), and onChange fires once per call.
func TestABoxApplyNoteIdempotent(t *testing.T) {
	st := store.New()
	note := Note{SourceIdx: "001", Title: "t"}
	ex := Extraction{Tables: []ExtractedTable{
		{Name: "etl_a", Kind: "update"},
		{Name: "etl_b", Kind: "insert"},
	}}

	fires := 0
	if err := ApplyNote(st, testNS, note, ex, func() { fires++ }); err != nil {
		t.Fatalf("ApplyNote 1: %v", err)
	}
	first := st.Len()
	if first != 5 {
		t.Fatalf("after first ApplyNote Len = %d, want 5", first)
	}
	if err := ApplyNote(st, testNS, note, ex, func() { fires++ }); err != nil {
		t.Fatalf("ApplyNote 2: %v", err)
	}
	if st.Len() != first {
		t.Errorf("after second ApplyNote Len = %d, want %d (no duplicates)", st.Len(), first)
	}
	if fires != 2 {
		t.Errorf("onChange fired %d times, want exactly once per ApplyNote (2)", fires)
	}
}

// TestABoxApplyNoteIsolation — replacing one note must not touch another
// note's triples.
func TestABoxApplyNoteIsolation(t *testing.T) {
	st := store.New()
	note1 := Note{SourceIdx: "001", Title: "t"}
	note2 := Note{SourceIdx: "002", Title: "t"}

	if err := ApplyNote(st, testNS, note1, Extraction{Tables: []ExtractedTable{{Name: "etl_a", Kind: "update"}}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(st, testNS, note2, Extraction{Tables: []ExtractedTable{{Name: "etl_b", Kind: "insert"}}}, nil); err != nil {
		t.Fatal(err)
	}
	note2Before := len(st.Query(NoteIRI(testNS, "002"), "", ""))

	// Replace note 001 with a different table set.
	if err := ApplyNote(st, testNS, note1, Extraction{Tables: []ExtractedTable{{Name: "etl_c", Kind: "create"}}}, nil); err != nil {
		t.Fatal(err)
	}

	if got := len(st.Query(NoteIRI(testNS, "002"), "", "")); got != note2Before {
		t.Errorf("note_002 triples = %d, want %d (isolation violated)", got, note2Before)
	}
	refs := st.Adjacent1(NoteIRI(testNS, "001"), testNS+"referencesTable")
	if len(refs) != 1 || refs[0] != testNS+"tbl_etl_c" {
		t.Errorf("note_001 refs = %v, want [tbl_etl_c]", refs)
	}
}

// TestABoxOrphanCleanup — a table referenced ONLY by the replaced note is
// fully removed (referencesTable AND its TableSchema type triple); a table
// shared with another note survives with exactly one shared type triple.
func TestABoxOrphanCleanup(t *testing.T) {
	st := store.New()
	note1 := Note{SourceIdx: "001", Title: "t"}
	note2 := Note{SourceIdx: "002", Title: "t"}

	if err := ApplyNote(st, testNS, note1, Extraction{Tables: []ExtractedTable{
		{Name: "etl_shared", Kind: "update"},
		{Name: "etl_orphan", Kind: "select"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(st, testNS, note2, Extraction{Tables: []ExtractedTable{
		{Name: "etl_shared", Kind: "update"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	// Replace note 001: drops its etl_shared ref, orphans etl_orphan.
	if err := ApplyNote(st, testNS, note1, Extraction{Tables: []ExtractedTable{
		{Name: "etl_new", Kind: "create"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	orphan := TableIRI(testNS, "etl_orphan")
	if got := st.Query("", testNS+"referencesTable", orphan); len(got) != 0 {
		t.Errorf("orphan still referenced: %v", got)
	}
	if got := st.Query(orphan, "", ""); len(got) != 0 {
		t.Errorf("orphan type triple survived: %v", got)
	}

	shared := TableIRI(testNS, "etl_shared")
	if got := st.Query(shared, rdfTypeIRI, testNS+"TableSchema"); len(got) != 1 {
		t.Errorf("shared table type triples = %v, want exactly 1 (globally shared)", got)
	}
	if got := st.Adjacent1(NoteIRI(testNS, "002"), testNS+"referencesTable"); len(got) != 1 || got[0] != shared {
		t.Errorf("note_002 shared ref = %v, want [%s]", got, shared)
	}
}

// TestABoxApplyNoteErrors — nil store and invalid extractions error out
// without mutating anything or firing onChange.
func TestABoxApplyNoteErrors(t *testing.T) {
	if err := ApplyNote(nil, testNS, Note{SourceIdx: "001"}, Extraction{}, nil); err == nil {
		t.Error("nil store should error")
	}

	st := store.New()
	fires := 0
	bad := Extraction{Tables: []ExtractedTable{{Name: "etl_x", Kind: "drop"}}}
	if err := ApplyNote(st, testNS, Note{SourceIdx: "001"}, bad, func() { fires++ }); err == nil {
		t.Error("invalid extraction kind should error")
	}
	if st.Len() != 0 {
		t.Errorf("store mutated on error path: Len = %d", st.Len())
	}
	if fires != 0 {
		t.Errorf("onChange fired on error path (%d times)", fires)
	}
}
