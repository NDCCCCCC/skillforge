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
	"errors"
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/ontology/emitter"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// rdfTypeIRI is the rdf:type predicate (same IRI store.LoadFromTurtle
// produces for `a` lines in the TBox TTL).
const rdfTypeIRI = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

// NoteIRI renders the ABox subject for a merged note (D-Ext-01):
// {ns}note_{source_idx}.
func NoteIRI(ns, sourceIdx string) string {
	return ns + "note_" + sourceIdx
}

// TableIRI renders the ABox individual for an extracted table (D-Ext-01):
// {ns}tbl_{name}. Extracted names cross the LLM trust boundary (T-39-06),
// so the local name always passes through the emitter sanitizer — the same
// single character policy the TBox individuals use. IRI breaking cannot
// inject alternate triples: S/P/O are struct fields, never concatenated
// TTL text.
func TableIRI(ns, name string) string {
	return ns + "tbl_" + emitter.SanitizeLocal(name)
}

// referencesTablePred returns the skf:referencesTable predicate IRI for a
// namespace (the D-TBox2-01 ObjectProperty: NoteChunk → TableSchema).
func referencesTablePred(ns string) string {
	return ns + "referencesTable"
}

// belongsToTierPred returns the skf:belongsToTier predicate IRI for a
// namespace (D-TIER-02: ABox tier attachment).
func belongsToTierPred(ns string) string {
	return ns + "belongsToTier"
}

// buildProvenance builds a Provenance record for a note's triples (D-PROV-02/05/11).
// source_kind = "file" for ECL notes; source_path from the note's first section's
// first SourcePath; line range from note.StartLine/EndLine; extracted_at = now.
func buildProvenance(note Note) *store.Provenance {
	var srcPath string
	if len(note.Sections) > 0 && len(note.Sections[0].SourcePaths) > 0 {
		srcPath = note.Sections[0].SourcePaths[0]
	}
	return &store.Provenance{
		SourceKind:  "file",
		SourcePath:  srcPath,
		LineStart:   note.StartLine,
		LineEnd:     note.EndLine,
		ExtractedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// BuildTriples derives the D-Ext-01 ABox triples for one extracted note:
//
//	{ns}note_{idx}  rdf:type                {ns}NoteChunk
//	{ns}note_{idx}  {ns}referencesTable     {ns}tbl_{name}     (per table)
//	{ns}tbl_{name}  rdf:type                {ns}TableSchema    (per table, deduped within the note)
//	{ns}note_{idx}  {ns}belongsToTier       {ns}{tier}         (per note, if Tier is set)
//	{ns}tbl_{name}  {ns}belongsToTier       {ns}{tier}         (per table, shares note's tier)
//
// Every triple carries Provenance (D-PROV-02/05/11): source_kind="file",
// source_path from note's first section source, line range from note's
// StartLine/EndLine, extracted_at = call time. belongsToNote triples and
// related_notes triples are NOT emitted (D-TBox2-01). Table names are
// sanitized via TableIRI. Global dedupe of TableSchema type triples
// happens at apply time via InsertUnique.
func BuildTriples(ns string, note Note, ex Extraction) []store.Triple {
	noteIRI := NoteIRI(ns, note.SourceIdx)
	pred := referencesTablePred(ns)
	tierPred := belongsToTierPred(ns)
	prov := buildProvenance(note)

	triples := []store.Triple{
		{S: noteIRI, P: rdfTypeIRI, O: ns + "NoteChunk", Provenance: prov},
	}

	// belongsToTier for the note itself (D-TIER-02).
	if note.Tier != "" {
		triples = append(triples, store.Triple{
			S: noteIRI, P: tierPred, O: ns + note.Tier, Provenance: prov,
		})
	}

	seen := map[string]bool{}
	for _, t := range ex.Tables {
		tblIRI := TableIRI(ns, t.Name)
		if seen[tblIRI] {
			continue
		}
		seen[tblIRI] = true
		triples = append(triples,
			store.Triple{S: noteIRI, P: pred, O: tblIRI, Provenance: prov},
			store.Triple{S: tblIRI, P: rdfTypeIRI, O: ns + "TableSchema", Provenance: prov},
		)
		// Table inherits the note's tier (D-TIER-02/07).
		if note.Tier != "" {
			triples = append(triples, store.Triple{
				S: tblIRI, P: tierPred, O: ns + note.Tier, Provenance: prov,
			})
		}
	}
	return triples
}

// ApplyNote performs the note-level KG replace (D-Run-01 idempotency,
// D-Boot-01 delete primitive): the target note's existing triples are
// removed, orphaned TableSchema individuals are swept, and the fresh
// extraction is inserted. Re-running for the same note converges to the
// same store state (no duplicates); other notes' triples are never
// touched.
//
// Ordering (T-39-08 — KG state corruption on partial replace):
//  1. validate the extraction BEFORE any mutation (a bad extraction can
//     never leave a half-replaced note behind)
//  2. capture the table IRIs the note currently references
//  3. DeleteSubject(noteIRI) — removes every triple with the note subject
//  4. orphan sweep — a captured table IRI with no remaining
//     referencesTable triples gets DeleteSubject'd (its TableSchema type
//     triple included); tables still referenced by other notes survive
//  5. insert fresh triples — note type + referencesTable via Insert (the
//     subject was just deleted, so these cannot duplicate); TableSchema
//     type triples via InsertUnique (tables are globally shared
//     individuals — re-asserting an existing type triple must not grow
//     the store)
//  6. invoke onChange exactly once after success — the caller (Plan 05/06
//     manager) wires the debounced snapshot fn here; ABox is not durable
//     without this trigger (RESEARCH anti-pattern: KG writes without
//     snapshot trigger)
//
// On any error the store is untouched and onChange does not fire.
//
// Bounded Impact (IN-09): steps 3-5 are separate per-op-locked store calls,
// not one atomic transaction. A concurrent reader — notably the manual
// POST /api/admin/ontology/snapshot handler — can snapshot the intermediate
// state (note deleted, orphans swept, fresh triples not yet inserted) and
// persist it; a crash before step 5 completes leaves that half-replaced
// state as the restored KG. The impact is bounded: the intermediate state
// is valid RDF (no dangling references) and the next successful run/retry
// of the same note converges to the correct state. If hardening is needed
// in the future, route manual snapshots through the store-level RLock held
// across Snapshot plus an ApplyNote "in-flight" flag.
func ApplyNote(st *store.Store, ns string, note Note, ex Extraction, onChange func()) error {
	if st == nil {
		return errors.New("ecl: abox: KG store is required")
	}
	if err := ValidateExtraction(ex); err != nil {
		return fmt.Errorf("ecl: abox: %w", err)
	}

	noteIRI := NoteIRI(ns, note.SourceIdx)
	pred := referencesTablePred(ns)

	// (2) capture currently-referenced table IRIs before deleting.
	var captured []string
	for _, t := range st.Query(noteIRI, pred, "") {
		captured = append(captured, t.O)
	}

	// (3) note-level delete.
	st.DeleteSubject(noteIRI)

	// (4) orphan sweep: solely-referenced tables disappear entirely.
	for _, tblIRI := range captured {
		if len(st.Query("", pred, tblIRI)) == 0 {
			st.DeleteSubject(tblIRI)
		}
	}

	// (5) insert fresh triples.
	for _, tr := range BuildTriples(ns, note, ex) {
		if tr.P == rdfTypeIRI && tr.O == ns+"TableSchema" {
			st.InsertUnique(tr) // tables are globally shared individuals
			continue
		}
		st.Insert(tr)
	}

	// (6) exactly one snapshot trigger per successful apply.
	if onChange != nil {
		onChange()
	}
	return nil
}
