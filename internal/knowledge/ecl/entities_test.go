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
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

// --- sanitizeEntities gates (D-NZK-04 / D-NZK-01, T-NZK-01) ---

// TestSanitizeEntitiesKindGate — an entity whose kind is not exactly
// "InsuranceProduct" or "Organization" is DROPPED entry-wise (D-NZK-04
// deliberately differs from the tables hard-error gate): illegal entries
// disappear, legal ones survive, and no error can escape this gate at all.
func TestSanitizeEntitiesKindGate(t *testing.T) {
	raw := []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct"},
		{Name: "总公司", Kind: "Organization"},
		{Name: "幻觉险", Kind: "Product"},     // near-miss class name → drop
		{Name: "幻觉机构", Kind: "objectProperty"}, // wrong case/space → drop
		{Name: "无名", Kind: ""},                // missing kind → drop
	}
	got := sanitizeEntities(raw)
	if len(got) != 2 {
		t.Fatalf("sanitizeEntities = %#v, want 2 survivors (illegal kinds dropped)", got)
	}
	if got[0].Name != "车险" || got[1].Name != "总公司" {
		t.Errorf("survivors = %#v, want [车险 总公司]", got)
	}
}

// TestSanitizeEntitiesNameGate — empty (after TrimSpace) and >12-rune names
// are dropped; the 12-rune boundary itself survives (T-NZK-01 name cap).
func TestSanitizeEntitiesNameGate(t *testing.T) {
	raw := []ExtractedEntity{
		{Name: "   ", Kind: "InsuranceProduct"},                  // whitespace-only → empty after trim
		{Name: "", Kind: "Organization"},                         // empty
		{Name: strings.Repeat("车", 13), Kind: "InsuranceProduct"}, // 13 runes → drop
		{Name: strings.Repeat("车", 12), Kind: "InsuranceProduct"}, // 12 runes → keep
	}
	got := sanitizeEntities(raw)
	if len(got) != 1 {
		t.Fatalf("sanitizeEntities = %#v, want exactly the 12-rune boundary survivor", got)
	}
	if n := utf8.RuneCountInString(got[0].Name); n != 12 {
		t.Errorf("survivor name = %d runes, want 12 (boundary stays)", n)
	}
}

// TestSanitizeEntitiesTrimAndDedupe — surviving names are trimmed; exact
// trimmed-name duplicates keep the FIRST entry (D-NZK-01); 车险 and 汽车险
// remain two entities (no aliasing, per D-NZK-01's no-alias-table policy).
func TestSanitizeEntitiesTrimAndDedupe(t *testing.T) {
	raw := []ExtractedEntity{
		{Name: " 车险 ", Kind: "InsuranceProduct", Code: "01"},
		{Name: "车险", Kind: "InsuranceProduct", Code: "99"}, // duplicate after trim — first wins
		{Name: "汽车险", Kind: "InsuranceProduct"},            // distinct name — kept
	}
	got := sanitizeEntities(raw)
	if len(got) != 2 {
		t.Fatalf("sanitizeEntities = %#v, want 2 (dedup keep-first + no aliasing)", got)
	}
	if got[0].Name != "车险" || got[0].Code != "01" {
		t.Errorf("first survivor = %#v, want trimmed 车险 with code 01 (keep-first)", got[0])
	}
	if got[1].Name != "汽车险" {
		t.Errorf("second survivor = %#v, want 汽车险 (distinct entity, no aliasing)", got[1])
	}
}

// TestSanitizeEntitiesParentNotValidatedHere — Parent passes through
// untouched: parent resolution needs store/batch context and is abox.go's
// job (Task 3, D-NZK-05), not this gate's.
func TestSanitizeEntitiesParentNotValidatedHere(t *testing.T) {
	raw := []ExtractedEntity{
		{Name: "武汉中支", Kind: "Organization", Parent: "湖北分公司"},
		{Name: "孤儿机构", Kind: "Organization", Parent: "不存在的上级"},
	}
	got := sanitizeEntities(raw)
	if len(got) != 2 {
		t.Fatalf("sanitizeEntities = %#v, want both survivors (parent NOT gated here)", got)
	}
	if got[0].Parent != "湖北分公司" || got[1].Parent != "不存在的上级" {
		t.Errorf("parents = %q / %q, want passthrough", got[0].Parent, got[1].Parent)
	}
}

// --- Prompt stability (D-NZK-04 prompt-stability requirement) ---

// TestExtractionPromptTablesAndRelatedByteIdentical pins that the entities
// extension APPENDED to extractionPromptTmpl left the D-Ext-01 contract
// byte-identical: the tables shape line, the related_notes shape line, and
// the Rules bullets — with the entities spec strictly AFTER the contract
// block (append, never rewrite).
//
// quick-261001-cz7 (2026-10-01): the scope line + three Rules bullets were
// REWRITTEN by the sanctioned 39-09 policy-A change (writes-only semantics
// — the live L1 gate measured the old "touches" scope at F1=0.3938). The
// fragments below pin the NEW contract wording; a future edit must update
// them deliberately, not silently.
func TestExtractionPromptTablesAndRelatedByteIdentical(t *testing.T) {
	originals := []string{
		`{"tables": [{"name": "<etl table name>", "kind": "update" | "select" | "insert" | "create"}],`,
		` "related_notes": [{"source_idx": "<note id>", "relation": "<short relation tag>"}]}`,
		`- "kind" is the DML type the note performs against the table, normalized: UPDATE → "update"; INSERT INTO → "insert"; CREATE → "create"; DELETE FROM or MERGE INTO → "update"; SELECT INTO → "select".`,
		"- Include only tables whose name starts with etl_. Report the BARE name without any schema/database prefix (e.g. \"wuh_dw.etl_policy\" → \"etl_policy\").",
		"- If the note writes no etl_ tables or references no other notes, emit empty arrays.",
		"- Output ONLY the JSON object, no fences, no prose.",
	}
	lastOriginal := 0
	for _, frag := range originals {
		idx := strings.Index(extractionPromptTmpl, frag)
		if idx < 0 {
			t.Fatalf("extractionPromptTmpl lost an original fragment (must stay byte-identical): %q", frag)
		}
		if idx < lastOriginal {
			t.Errorf("fragment out of original order: %q", frag)
		}
		lastOriginal = idx
	}

	// The entities spec is APPENDED after the original block.
	entitiesIdx := strings.Index(extractionPromptTmpl, `"entities"`)
	if entitiesIdx < 0 {
		t.Fatal("extractionPromptTmpl missing the appended entities spec")
	}
	if entitiesIdx < lastOriginal {
		t.Errorf("entities spec at offset %d must be appended after the original block (last original at %d)", entitiesIdx, lastOriginal)
	}
	for _, want := range []string{"InsuranceProduct", "Organization", "12"} {
		if !strings.Contains(extractionPromptTmpl, want) {
			t.Errorf("entities spec missing %q", want)
		}
	}

	// The rendered prompt (what the model actually sees) carries the spec.
	rendered := promptForBody("001", "t", "body")
	if !strings.Contains(rendered, `"entities"`) {
		t.Error("rendered prompt missing the entities spec")
	}
}

// --- ExtractNote entities plumbing (single-call + over-cap union) ---

// TestLLMExtractNoteEntitiesParsed — a fake response carrying entities
// parses through sanitizeEntities into Extraction.Entities on the
// single-call path. Identity is the exact trimmed name (D-NZK-01): a same-
// name different-kind duplicate is collapsed keep-first.
func TestLLMExtractNoteEntitiesParsed(t *testing.T) {
	fake := &fakeGenerator{contents: []string{
		`{"tables":[],"related_notes":[],"entities":[{"name":"车险","kind":"InsuranceProduct","code":"01"},{"name":" 车险","kind":"Organization"}]}`,
	}}
	got, err := NewLLMExtractor(fake).ExtractNote(context.Background(), noteWith("001", "车险代码01"))
	if err != nil {
		t.Fatalf("ExtractNote: %v", err)
	}
	want := ExtractedEntity{Name: "车险", Kind: "InsuranceProduct", Code: "01"}
	if len(got.Entities) != 1 || got.Entities[0] != want {
		t.Errorf("entities = %#v, want exactly [%+v] (name-identity dedupe keep-first)", got.Entities, want)
	}
}

// TestLLMExtractNoteIllegalEntityKindDropped — an illegal entity kind is
// DROPPED silently while the legal tables still parse: D-NZK-04's drop
// semantics (the tables hard-error gate must NOT extend to entities).
func TestLLMExtractNoteIllegalEntityKindDropped(t *testing.T) {
	fake := &fakeGenerator{contents: []string{
		`{"tables":[{"name":"etl_a","kind":"update"}],"related_notes":[],"entities":[{"name":"幻觉","kind":"Product"},{"name":"车险","kind":"InsuranceProduct"}]}`,
	}}
	got, err := NewLLMExtractor(fake).ExtractNote(context.Background(), noteWith("001", "body"))
	if err != nil {
		t.Fatalf("illegal entity kind must DROP the entry, not error: %v", err)
	}
	if len(got.Tables) != 1 {
		t.Fatalf("tables = %#v, want the legal table to survive", got.Tables)
	}
	if len(got.Entities) != 1 || got.Entities[0].Name != "车险" {
		t.Errorf("entities = %#v, want exactly the legal 车险 (幻觉 dropped)", got.Entities)
	}
}

// TestLLMExtractNoteOversizedEntityUnion — on the per-section over-cap
// path, the same entity name returned by two section calls appears once in
// the union (keep-first mirrors the seenTables pattern, D-NZK-01).
func TestLLMExtractNoteOversizedEntityUnion(t *testing.T) {
	note := Note{
		SourceIdx: "001",
		Title:     "t",
		Sections: []MergedSection{
			{Section: "概述", SectionIdx: 0, Body: strings.Repeat("SECTION-ONE-MARKER line of note prose;\n", 4)},
			{Section: "SQL / 正文", SectionIdx: 1, Body: strings.Repeat("SECTION-TWO-MARKER line of note prose;\n", 4)},
		},
	}
	fake := &fakeGenerator{contents: []string{
		`{"tables":[],"related_notes":[],"entities":[{"name":"车险","kind":"InsuranceProduct","code":"01"}]}`,
		`{"tables":[],"related_notes":[],"entities":[{"name":"车险","kind":"InsuranceProduct","code":"XX"},{"name":"农险","kind":"InsuranceProduct"}]}`,
	}}

	got, err := NewLLMExtractor(fake, 10).ExtractNote(context.Background(), note)
	if err != nil {
		t.Fatalf("ExtractNote: %v", err)
	}
	if fake.calls != len(note.Sections) {
		t.Fatalf("calls = %d, want %d (per-section calls)", fake.calls, len(note.Sections))
	}
	want := []ExtractedEntity{
		{Name: "车险", Kind: "InsuranceProduct", Code: "01"}, // first section wins
		{Name: "农险", Kind: "InsuranceProduct"},
	}
	if len(got.Entities) != len(want) {
		t.Fatalf("union entities = %#v, want %d entries (deduped keep-first)", got.Entities, len(want))
	}
	for i, w := range want {
		if got.Entities[i] != w {
			t.Errorf("union entity[%d] = %#v, want %#v", i, got.Entities[i], w)
		}
	}
}
