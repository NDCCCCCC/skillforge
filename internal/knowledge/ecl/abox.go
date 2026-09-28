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
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/ontology/emitter"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// rdfTypeIRI is the rdf:type predicate (same IRI store.LoadFromTurtle
// produces for `a` lines in the TBox TTL).
const rdfTypeIRI = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

// rdfsLabelIRI is the rdfs:label predicate for entity individuals (D-NZK-05):
// the object is the exact trimmed entity name, stored as a plain literal
// (same literal discipline as the TBox tierName literals).
const rdfsLabelIRI = "http://www.w3.org/2000/01/rdf-schema#label"

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

// involvingInsuranceProductPred returns the skf:involvesInsuranceProduct
// predicate IRI (TBox v7, prop-0003: NoteChunk → InsuranceProduct).
func involvingInsuranceProductPred(ns string) string {
	return ns + "involvesInsuranceProduct"
}

// involvingOrganizationPred returns the skf:involvesOrganization predicate
// IRI (TBox v7, prop-0004: NoteChunk → Organization).
func involvingOrganizationPred(ns string) string {
	return ns + "involvesOrganization"
}

// parentOrganizationPred returns the skf:parentOrganization predicate IRI
// (TBox v7, prop-0005: Organization → Organization).
func parentOrganizationPred(ns string) string {
	return ns + "parentOrganization"
}

// entityCodePred returns the skf:entityCode predicate IRI (TBox v7,
// prop-0006: entity business code, xsd:string).
func entityCodePred(ns string) string {
	return ns + "entityCode"
}

// entityBatch carries the per-call entity IRI assignment shared by
// BuildTriples and ApplyNote: one IRI per entity (collision-ordinal
// suffixed when two DISTINCT names fuse into one local name) plus the
// same-batch Organization name→IRI index for exact-trimmed-name parent
// resolution (D-NZK-04).
type entityBatch struct {
	iris      []string
	orgByName map[string]string
}

// assignEntityIRIs sanitizes raw entities through the same D-NZK-04 gates
// as extractBody (defense in depth — a hand-built Extraction cannot bypass
// the kind/name/dedupe gates) and assigns batch IRIs. The returned slice
// keeps input order; collisions get "-2", "-3", … suffixes in first-seen
// order.
func assignEntityIRIs(ns string, raw []ExtractedEntity) entityBatch {
	entities := sanitizeEntities(raw)
	batch := entityBatch{iris: make([]string, len(entities)), orgByName: map[string]string{}}
	seen := map[string]int{}
	for i, e := range entities {
		base := EntityIRI(ns, e.Name)
		n := seen[base]
		seen[base] = n + 1
		if n > 0 {
			batch.iris[i] = base + "-" + fmt.Sprint(n+1)
		} else {
			batch.iris[i] = base
		}
		if e.Kind == "Organization" {
			if _, exists := batch.orgByName[e.Name]; !exists {
				batch.orgByName[e.Name] = batch.iris[i]
			}
		}
	}
	return batch
}

// entityEntities returns the sanitized entities behind an assignment —
// callers re-derive kind/parent from it rather than threading a second
// slice through every signature.
func batchEntities(raw []ExtractedEntity) []ExtractedEntity {
	return sanitizeEntities(raw)
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

	// D-NZK-05: business entities. assignEntityIRIs re-runs the D-NZK-04
	// drop-gates (defense in depth, T-NZK-01) and resolves same-batch
	// collisions; all entity IRIs are deterministic per D-NZK-01 so the
	// apply-time InsertUnique split dedupes globally shared individuals.
	batch := assignEntityIRIs(ns, ex.Entities)
	entities := batchEntities(ex.Entities)
	for i, e := range entities {
		iri := batch.iris[i]
		triples = append(triples,
			store.Triple{S: iri, P: rdfTypeIRI, O: ns + e.Kind, Provenance: prov},
			store.Triple{S: iri, P: rdfsLabelIRI, O: e.Name, Provenance: prov},
		)
		if e.Code != "" {
			triples = append(triples, store.Triple{S: iri, P: entityCodePred(ns), O: e.Code, Provenance: prov})
		}
		// belongsToTier is ALWAYS the note's tier (server-derived from the
		// kbDir basename, never LLM-supplied — T-NZK-03), satisfying the
		// SHACL global belongsToTierShape (object must be a skf:Tier
		// individual).
		if note.Tier != "" {
			triples = append(triples, store.Triple{S: iri, P: tierPred, O: ns + note.Tier, Provenance: prov})
		}
		// note→entity edge with the kind-specific predicate (TBox v7).
		edge := involvingOrganizationPred(ns)
		if e.Kind == "InsuranceProduct" {
			edge = involvingInsuranceProductPred(ns)
		}
		triples = append(triples, store.Triple{S: noteIRI, P: edge, O: iri, Provenance: prov})
		// parentOrganization ONLY for an Organization whose parent
		// exactly matches another Organization entity in the same batch
		// (D-NZK-04); a self-parent is meaningless and dropped.
		if e.Kind == "Organization" && e.Parent != "" && e.Parent != e.Name {
			if parentIRI, ok := batch.orgByName[e.Parent]; ok {
				triples = append(triples, store.Triple{S: iri, P: parentOrganizationPred(ns), O: parentIRI, Provenance: prov})
			}
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
	invIns := involvingInsuranceProductPred(ns)
	invOrg := involvingOrganizationPred(ns)
	parentPred := parentOrganizationPred(ns)
	entPrefix := ns + "ent_"

	// (2) capture currently-referenced table IRIs AND entity IRIs (both
	// involves-predicates) before deleting.
	var captured []string
	for _, t := range st.Query(noteIRI, pred, "") {
		captured = append(captured, t.O)
	}
	capturedEnts := map[string]bool{}
	for _, p := range []string{invIns, invOrg} {
		for _, t := range st.Query(noteIRI, p, "") {
			capturedEnts[t.O] = true
		}
	}

	// (3) note-level delete.
	st.DeleteSubject(noteIRI)

	// (4) orphan sweep: solely-referenced tables disappear entirely; an
	// entity disappears only when NO involves-edge AND NO
	// parentOrganization target-edge remains (the parent-target guard —
	// an entity other entities still point at survives even if no note
	// currently involves it).
	for _, tblIRI := range captured {
		if len(st.Query("", pred, tblIRI)) == 0 {
			st.DeleteSubject(tblIRI)
		}
	}
	for entIRI := range capturedEnts {
		if len(st.Query("", invIns, entIRI)) > 0 ||
			len(st.Query("", invOrg, entIRI)) > 0 ||
			len(st.Query("", parentPred, entIRI)) > 0 {
			continue
		}
		st.DeleteSubject(entIRI)
	}

	// (5) insert fresh triples. Entity-owned triples (subject is an ent_
	// IRI) are InsertUnique — entity individuals are globally shared
	// across notes and re-runs (D-NZK-01). involves-edges are note-owned
	// and rebuilt via Insert (the note subject was just deleted).
	// TableSchema type triples stay InsertUnique (pre-existing pin).
	for _, tr := range BuildTriples(ns, note, ex) {
		if tr.P == invIns || tr.P == invOrg {
			st.Insert(tr)
			continue
		}
		if strings.HasPrefix(tr.S, entPrefix) {
			st.InsertUnique(tr)
			continue
		}
		if tr.P == rdfTypeIRI && tr.O == ns+"TableSchema" {
			st.InsertUnique(tr) // tables are globally shared individuals
			continue
		}
		st.Insert(tr)
	}

	// (5b) second-chance parent resolution: a parent that is not in this
	// batch but already exists in the store as an Organization individual
	// still gets its parentOrganization triple (InsertUnique — the edge is
	// entity-owned). BuildTriples has no store access, so this is the
	// apply-time complement of the same-batch check.
	batch := assignEntityIRIs(ns, ex.Entities)
	entities := batchEntities(ex.Entities)
	prov := buildProvenance(note)
	for i, e := range entities {
		if e.Kind != "Organization" || e.Parent == "" || e.Parent == e.Name {
			continue
		}
		if _, inBatch := batch.orgByName[e.Parent]; inBatch {
			continue // already emitted by BuildTriples
		}
		parentIRI := EntityIRI(ns, e.Parent)
		if len(st.Query(parentIRI, rdfTypeIRI, ns+"Organization")) > 0 {
			st.InsertUnique(store.Triple{
				S: batch.iris[i], P: parentPred, O: parentIRI, Provenance: prov,
			})
		}
	}

	// (6) exactly one snapshot trigger per successful apply.
	if onChange != nil {
		onChange()
	}
	return nil
}
