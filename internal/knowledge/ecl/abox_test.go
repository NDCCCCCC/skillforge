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

// TestABoxBelongsToTierApplyNote — ApplyNote emits belongsToTier triples
// for note and all its tables (D-TIER-02). A note without a Tier field
// emits no belongsToTier triples.
func TestABoxBelongsToTierApplyNote(t *testing.T) {
	st := store.New()

	// Tier = "sql-guides".
	noteWithTier := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides",
		Sections: []MergedSection{{Body: "body", SourcePaths: []string{"knowledge/sql-guides/001__sec01.md"}}}}
	if err := ApplyNote(st, testNS, noteWithTier, Extraction{Tables: []ExtractedTable{
		{Name: "etl_a", Kind: "update"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	noteIRI := NoteIRI(testNS, "001")
	tblIRI := TableIRI(testNS, "etl_a")

	// Note has belongsToTier triple.
	noteTierTriples := st.Query(noteIRI, testNS+"belongsToTier", "")
	if len(noteTierTriples) != 1 {
		t.Errorf("note belongsToTier triples = %d, want 1", len(noteTierTriples))
	}
	if len(noteTierTriples) > 0 && noteTierTriples[0].O != testNS+"sql-guides" {
		t.Errorf("note belongsToTier O = %q, want %q", noteTierTriples[0].O, testNS+"sql-guides")
	}

	// Table has belongsToTier triple.
	tblTierTriples := st.Query(tblIRI, testNS+"belongsToTier", "")
	if len(tblTierTriples) != 1 {
		t.Errorf("table belongsToTier triples = %d, want 1", len(tblTierTriples))
	}
	if len(tblTierTriples) > 0 && tblTierTriples[0].O != testNS+"sql-guides" {
		t.Errorf("table belongsToTier O = %q, want %q", tblTierTriples[0].O, testNS+"sql-guides")
	}

	// Note without Tier emits no belongsToTier triples.
	noteNoTier := Note{SourceIdx: "002", Title: "t"}
	if err := ApplyNote(st, testNS, noteNoTier, Extraction{Tables: []ExtractedTable{
		{Name: "etl_b", Kind: "insert"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	note2IRI := NoteIRI(testNS, "002")
	if len(st.Query(note2IRI, testNS+"belongsToTier", "")) != 0 {
		t.Error("note without Tier should have no belongsToTier triple")
	}
}

// TestABoxTierOrphanCleanup — when a note is replaced and drops a table
// reference, the orphaned table's belongsToTier triple is also removed
// (D-TIER-07). Tables still referenced by other notes survive with their
// belongsToTier intact.
func TestABoxTierOrphanCleanup(t *testing.T) {
	st := store.New()

	note1 := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	note2 := Note{SourceIdx: "002", Title: "t", Tier: "sql-guides"}

	// Note 1 references etl_orphan (will be dropped) and etl_shared.
	// Note 2 references etl_shared only.
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

	orphanIRI := TableIRI(testNS, "etl_orphan")

	// Orphan should have exactly 1 belongsToTier before replacement
	// (inherited from note1 via etl_orphan reference).
	beforeOrphanTier := st.Query(orphanIRI, testNS+"belongsToTier", "")
	if len(beforeOrphanTier) != 1 {
		t.Fatalf("before replacement: orphan belongsToTier = %d, want 1", len(beforeOrphanTier))
	}

	// Replace note 1: drops etl_orphan reference, orphans the table.
	if err := ApplyNote(st, testNS, note1, Extraction{Tables: []ExtractedTable{
		{Name: "etl_new", Kind: "create"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	// Orphan table itself should be fully gone (referencesTable + type + belongsToTier).
	if len(st.Query(orphanIRI, "", "")) != 0 {
		t.Errorf("orphan table still has triples after deletion: %v", st.Query(orphanIRI, "", ""))
	}

	// etl_shared should still have its belongsToTier (note2 still references it).
	// note1 had Tier=sql-guides (now deleted) and note2 also has Tier=sql-guides,
	// so tbl_shared accumulates 2 belongsToTier triples (D-TIER-06: any match = visible).
	sharedIRI := TableIRI(testNS, "etl_shared")
	sharedTier := st.Query(sharedIRI, testNS+"belongsToTier", "")
	if len(sharedTier) != 2 {
		t.Errorf("shared table belongsToTier after orphan cleanup = %d, want 2 (note1+note2 both sql-guides)", len(sharedTier))
	}
}

// TestABoxMultiTierTableInheritance — a table referenced by notes from
// different tiers gets multiple belongsToTier triples, one per tier
// (D-TIER-06: any match makes the table visible).
func TestABoxMultiTierTableInheritance(t *testing.T) {
	st := store.New()

	noteSqlguides := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	noteAnother := Note{SourceIdx: "002", Title: "t", Tier: "another-tier"}

	// Note 1 references etl_cross.
	if err := ApplyNote(st, testNS, noteSqlguides, Extraction{Tables: []ExtractedTable{
		{Name: "etl_cross", Kind: "update"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	// Note 2 also references etl_cross.
	if err := ApplyNote(st, testNS, noteAnother, Extraction{Tables: []ExtractedTable{
		{Name: "etl_cross", Kind: "insert"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	tblIRI := TableIRI(testNS, "etl_cross")
	tierTriples := st.Query(tblIRI, testNS+"belongsToTier", "")
	if len(tierTriples) != 2 {
		t.Errorf("cross-tier table belongsToTier count = %d, want 2", len(tierTriples))
	}
}

// --- D-NZK-05: entity ABox shapes ---

// findTriples is a set-style pattern filter over in-memory triples ("" =
// wildcard), keeping entity assertions readable without pinning triple
// indices.
func findTriples(ts []store.Triple, s, p, o string) []store.Triple {
	var out []store.Triple
	for _, t := range ts {
		if (s == "" || t.S == s) && (p == "" || t.P == p) && (o == "" || t.O == o) {
			out = append(out, t)
		}
	}
	return out
}

// TestBuildTriplesEntityShapes pins the per-entity triple set (D-NZK-05):
// rdf:type {ns}{Kind}, rdfs:label, optional entityCode, belongsToTier
// inherited from the note, the note→entity involves edge per kind, and the
// parentOrganization edge ONLY for a same-batch exact-name Organization
// match. The existing table triples stay byte-for-byte in front (index
// stability for TestABoxBuildTriples).
func TestBuildTriplesEntityShapes(t *testing.T) {
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides",
		Sections: []MergedSection{{Body: "body", SourcePaths: []string{"knowledge/sql-guides/001__sec01.md"}}}}
	cheXian := EntityIRI(testNS, "车险")
	wuhan := EntityIRI(testNS, "武汉中支")
	hubei := EntityIRI(testNS, "湖北分公司")
	ex := Extraction{
		Tables: []ExtractedTable{{Name: "etl_a", Kind: "update"}},
		Entities: []ExtractedEntity{
			{Name: "车险", Kind: "InsuranceProduct", Code: "01"},
			{Name: "武汉中支", Kind: "Organization", Parent: "湖北分公司"},
			{Name: "湖北分公司", Kind: "Organization"},
			// parent on a non-Organization must be dropped (仅 Organization).
			{Name: "车险二号", Kind: "InsuranceProduct", Parent: "湖北分公司"},
		},
	}
	got := BuildTriples(testNS, note, ex)

	// Existing table triple count (8, per TestABoxBuildTriples) comes first.
	if len(findTriples(got, testNS+"note_001", testNS+"referencesTable", testNS+"tbl_etl_a")) != 1 {
		t.Fatal("table reference triple missing — entities extension broke the table path")
	}

	// rdf:type per entity kind.
	if n := len(findTriples(got, cheXian, rdfTypeIRI, testNS+"InsuranceProduct")); n != 1 {
		t.Errorf("车险 type triples = %d, want 1", n)
	}
	if n := len(findTriples(got, wuhan, rdfTypeIRI, testNS+"Organization")); n != 1 {
		t.Errorf("武汉中支 type triples = %d, want 1", n)
	}
	if n := len(findTriples(got, hubei, rdfTypeIRI, testNS+"Organization")); n != 1 {
		t.Errorf("湖北分公司 type triples = %d, want 1", n)
	}

	// rdfs:label = exact trimmed name.
	if n := len(findTriples(got, cheXian, rdfsLabelIRI, "车险")); n != 1 {
		t.Errorf("车险 label triples = %d, want 1", n)
	}

	// entityCode only when non-empty.
	if n := len(findTriples(got, cheXian, testNS+"entityCode", "01")); n != 1 {
		t.Errorf("车险 entityCode triples = %d, want 1", n)
	}
	if n := len(findTriples(got, wuhan, testNS+"entityCode", "")); n != 0 {
		t.Errorf("武汉中支 entityCode triples = %d, want 0 (no code)", n)
	}

	// belongsToTier inherited from the note's tier — never LLM-supplied.
	for name, iri := range map[string]string{"车险": cheXian, "武汉中支": wuhan, "湖北分公司": hubei} {
		if n := len(findTriples(got, iri, testNS+"belongsToTier", testNS+"sql-guides")); n != 1 {
			t.Errorf("%s belongsToTier triples = %d, want 1", name, n)
		}
	}

	// note→entity edges carry the kind-specific predicate.
	if n := len(findTriples(got, testNS+"note_001", testNS+"involvesInsuranceProduct", cheXian)); n != 1 {
		t.Errorf("involvesInsuranceProduct edges = %d, want 1", n)
	}
	if n := len(findTriples(got, testNS+"note_001", testNS+"involvesOrganization", wuhan)); n != 1 {
		t.Errorf("involvesOrganization(武汉中支) edges = %d, want 1", n)
	}
	if n := len(findTriples(got, testNS+"note_001", testNS+"involvesOrganization", hubei)); n != 1 {
		t.Errorf("involvesOrganization(湖北分公司) edges = %d, want 1", n)
	}

	// parentOrganization only for the same-batch Organization match.
	if n := len(findTriples(got, wuhan, testNS+"parentOrganization", hubei)); n != 1 {
		t.Errorf("武汉中支 parentOrganization triples = %d, want 1", n)
	}
	if n := len(findTriples(got, cheXian, testNS+"parentOrganization", "")); n != 0 {
		t.Errorf("车险 parentOrganization triples = %d, want 0 (InsuranceProduct never carries parent)", n)
	}
	if n := len(findTriples(got, EntityIRI(testNS, "车险二号"), testNS+"parentOrganization", "")); n != 0 {
		t.Error("InsuranceProduct parent reference not dropped")
	}

	// Every entity triple carries provenance (D-PROV-02/05/11).
	for _, tr := range findTriples(got, cheXian, "", "") {
		if tr.Provenance == nil {
			t.Errorf("entity triple missing provenance: %+v", tr)
		}
	}
}

// TestBuildTriplesEntityParentNotInBatchDropped — a parent referencing a
// non-existent (not-in-batch) name drops the parent triple but KEEPS the
// entity (D-NZK-04).
func TestBuildTriplesEntityParentNotInBatchDropped(t *testing.T) {
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	ex := Extraction{Entities: []ExtractedEntity{
		{Name: "武汉中支", Kind: "Organization", Parent: "不存在的上级"},
	}}
	got := BuildTriples(testNS, note, ex)
	wuhan := EntityIRI(testNS, "武汉中支")
	if n := len(findTriples(got, wuhan, testNS+"parentOrganization", "")); n != 0 {
		t.Errorf("unresolvable parent emitted %d triples, want 0", n)
	}
	if n := len(findTriples(got, wuhan, rdfTypeIRI, testNS+"Organization")); n != 1 {
		t.Error("entity itself must survive an unresolvable parent")
	}
}

// TestBuildTriplesEntitySameBatchCollisionOrdinals — two DISTINCT names
// whose local names coincide (ASCII PascalCase fusion) get ordinal suffixes
// (-2, -3, …) within one BuildTriples call so a batch never collapses two
// entities into one IRI.
func TestBuildTriplesEntitySameBatchCollisionOrdinals(t *testing.T) {
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	ex := Extraction{Entities: []ExtractedEntity{
		{Name: "ab", Kind: "Organization"},
		{Name: "AB", Kind: "Organization"}, // same PascalCase local "Ab"
	}}
	got := BuildTriples(testNS, note, ex)
	base := EntityIRI(testNS, "ab")
	first := findTriples(got, base, rdfsLabelIRI, "ab")
	second := findTriples(got, base+"-2", rdfsLabelIRI, "AB")
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("collision ordinals wrong: base=%v suffixed=%v", first, second)
	}
	// Both still get note→entity edges under their assigned IRIs.
	if n := len(findTriples(got, testNS+"note_001", testNS+"involvesOrganization", base+"-2")); n != 1 {
		t.Error("suffixed entity missing its note→entity edge")
	}
}

// TestApplyNoteEntitiesIdempotent — re-running ApplyNote for the same note
// with the same extraction adds ZERO new entity triples: entity-owned
// triples are InsertUnique, note→entity edges are rebuilt via Insert after
// DeleteSubject. onChange still fires exactly once per apply.
func TestApplyNoteEntitiesIdempotent(t *testing.T) {
	st := store.New()
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	ex := Extraction{Entities: []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct", Code: "01"},
		{Name: "武汉中支", Kind: "Organization", Parent: "湖北分公司"},
		{Name: "湖北分公司", Kind: "Organization"},
	}}

	fires := 0
	if err := ApplyNote(st, testNS, note, ex, func() { fires++ }); err != nil {
		t.Fatalf("ApplyNote 1: %v", err)
	}
	first := st.Len()
	if first == 0 {
		t.Fatal("no triples after first apply")
	}
	if err := ApplyNote(st, testNS, note, ex, func() { fires++ }); err != nil {
		t.Fatalf("ApplyNote 2: %v", err)
	}
	if st.Len() != first {
		t.Errorf("store grew on re-apply: %d → %d (entity apply not idempotent)", first, st.Len())
	}
	if fires != 2 {
		t.Errorf("onChange fired %d times, want exactly once per apply (2)", fires)
	}
	cheXian := EntityIRI(testNS, "车险")
	if n := len(st.Query(cheXian, rdfTypeIRI, testNS+"InsuranceProduct")); n != 1 {
		t.Errorf("车险 type triples after re-apply = %d, want exactly 1", n)
	}
}

// TestApplyNoteEntityOrphanSweep — extended sweep semantics: an entity the
// replaced note no longer mentions is fully removed UNLESS another note
// still involves it or references it as a parentOrganization target.
func TestApplyNoteEntityOrphanSweep(t *testing.T) {
	st := store.New()
	note1 := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	note2 := Note{SourceIdx: "002", Title: "t", Tier: "sql-guides"}

	// note1: orphan candidate + parent target; note2: shared entity, and a
	// child whose parent lives only in note1's batch.
	if err := ApplyNote(st, testNS, note1, Extraction{Entities: []ExtractedEntity{
		{Name: "孤儿险", Kind: "InsuranceProduct"},
		{Name: "湖北分公司", Kind: "Organization"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(st, testNS, note2, Extraction{Entities: []ExtractedEntity{
		{Name: "共享险", Kind: "InsuranceProduct"},
		{Name: "武汉中支", Kind: "Organization", Parent: "湖北分公司"}, // parent NOT in note2's batch
	}}, nil); err != nil {
		t.Fatal(err)
	}

	wuhan := EntityIRI(testNS, "武汉中支")
	hubei := EntityIRI(testNS, "湖北分公司")
	orphan := EntityIRI(testNS, "孤儿险")
	shared := EntityIRI(testNS, "共享险")

	// note1 applied first, so 湖北分公司 was NOT in note2's batch — the
	// second-chance store check must have emitted the parent edge.
	if n := len(st.Query(wuhan, testNS+"parentOrganization", hubei)); n != 1 {
		t.Fatalf("second-chance parent edge = %d, want 1", n)
	}

	// Replace note1 dropping BOTH its entities.
	if err := ApplyNote(st, testNS, note1, Extraction{Entities: []ExtractedEntity{
		{Name: "新车险", Kind: "InsuranceProduct"},
	}}, nil); err != nil {
		t.Fatal(err)
	}

	// 孤儿险: no remaining involves edge → swept entirely.
	if got := st.Query(orphan, "", ""); len(got) != 0 {
		t.Errorf("orphan entity survived: %v", got)
	}
	// 湖北分公司: no involves edge remains, but 武汉中支 (note2) still points
	// at it via parentOrganization → survives (parent-target guard).
	if n := len(st.Query(hubei, rdfTypeIRI, testNS+"Organization")); n != 1 {
		t.Errorf("parent-target entity swept: %d type triples, want 1", n)
	}
	// 共享险: note2 still involves it → survives.
	if n := len(st.Query(shared, rdfTypeIRI, testNS+"InsuranceProduct")); n != 1 {
		t.Errorf("shared entity swept: %d type triples, want 1", n)
	}
	// 武汉中支's parent edge survived untouched.
	if n := len(st.Query(wuhan, testNS+"parentOrganization", hubei)); n != 1 {
		t.Errorf("parent edge lost during sweep: %d, want 1", n)
	}
}

// TestApplyNoteSecondChanceParentGates — the second-chance store check
// requires the parent to exist as an Organization individual: a parent name
// absent from batch AND store emits nothing (entity kept).
func TestApplyNoteSecondChanceParentGates(t *testing.T) {
	st := store.New()
	note := Note{SourceIdx: "001", Title: "t", Tier: "sql-guides"}
	ex := Extraction{Entities: []ExtractedEntity{
		{Name: "武汉中支", Kind: "Organization", Parent: "湖北分公司"},
	}}
	if err := ApplyNote(st, testNS, note, ex, nil); err != nil {
		t.Fatal(err)
	}
	wuhan := EntityIRI(testNS, "武汉中支")
	hubei := EntityIRI(testNS, "湖北分公司")
	if n := len(st.Query(wuhan, testNS+"parentOrganization", "")); n != 0 {
		t.Errorf("parent edge emitted for a store-absent parent: %d, want 0", n)
	}
	if n := len(st.Query(wuhan, rdfTypeIRI, testNS+"Organization")); n != 1 {
		t.Error("entity must survive an unresolvable parent")
	}

	// Apply 湖北分公司 via another note; re-apply the child note — the
	// parent now resolves through the store.
	if err := ApplyNote(st, testNS, Note{SourceIdx: "002", Title: "t", Tier: "sql-guides"}, Extraction{Entities: []ExtractedEntity{
		{Name: "湖北分公司", Kind: "Organization"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(st, testNS, note, ex, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Query(wuhan, testNS+"parentOrganization", hubei)); n != 1 {
		t.Errorf("second-chance parent edge = %d, want 1 once the parent exists", n)
	}
}

// TestApplyNoteEntityTierFromNoteOnly — entity belongsToTier is ALWAYS the
// note's tier (server-derived, T-NZK-03): no Tier → no belongsToTier on the
// entity, and two notes in different tiers accumulate two tier triples on
// the shared entity individual (D-TIER-06 any-match visibility).
func TestApplyNoteEntityTierFromNoteOnly(t *testing.T) {
	st := store.New()
	if err := ApplyNote(st, testNS, Note{SourceIdx: "001", Title: "t"}, Extraction{Entities: []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	cheXian := EntityIRI(testNS, "车险")
	if n := len(st.Query(cheXian, testNS+"belongsToTier", "")); n != 0 {
		t.Errorf("tier-less note gave the entity %d belongsToTier triples, want 0", n)
	}

	if err := ApplyNote(st, testNS, Note{SourceIdx: "002", Title: "t", Tier: "sql-guides"}, Extraction{Entities: []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(st, testNS, Note{SourceIdx: "003", Title: "t", Tier: "other-tier"}, Extraction{Entities: []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct"},
	}}, nil); err != nil {
		t.Fatal(err)
	}
	tiers := st.Query(cheXian, testNS+"belongsToTier", "")
	if len(tiers) != 2 {
		t.Errorf("cross-tier entity belongsToTier = %d, want 2 (sql-guides + other-tier)", len(tiers))
	}
	if n := len(st.Query(cheXian, rdfTypeIRI, testNS+"InsuranceProduct")); n != 1 {
		t.Errorf("cross-tier entity type triples = %d, want exactly 1 (InsertUnique)", n)
	}
}
