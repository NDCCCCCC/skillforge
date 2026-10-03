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

// External test package (store_test, not store): the D-45-01 conformance
// test must import internal/ontology/emitter to render its live fixture,
// and emitter transitively imports store (emitter/upgrade.go → induction →
// store) — an in-package test file hits "import cycle not allowed in test".
// External test packages may legally cycle through the package under test;
// the unexported members this test needs (metaPredicates, schemaSubjectSet,
// all, ns, rdfType) are reached through the in-package test seam in
// store_export_test.go (compiled only under `go test` — zero production
// API surface). Deviation from the 45-02 plan's literal `package store`
// clause recorded in 45-02-SUMMARY.md (Rule 3).
package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/emitter"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// TestEmitterPredicatesWithinAllowlist pins D-45-01's anti-drift gate:
// every predicate the emitter emits on a schema-individual subject must be
// rdf:type or one of the 13 metaPredicates. A render() change that adds a
// predicate on Skill/KBCategory/SubCategory/Tier/SkillAgent/RefusalPhrase
// individuals fails HERE (explicitly, with the offending triples listed)
// instead of silently becoming tier-visible or silently half-invisible
// under the β arm of isSchemaTriple.
//
// The emitter output is parsed through the PRODUCTION path —
// (store.New()).LoadFromTurtle — not a text/regex scan of the TTL, so
// prefix expansion (skf: → full IRI) matches exactly what production sees.
// Schema subjects are scoped via the SAME derivation Path B uses
// (ConformanceSchemaSubjectSet → Store.schemaSubjectSet), not a
// re-implementation.
//
// Scoping is deliberately limited to schema-INDIVIDUAL subjects (45-RESEARCH
// Pitfall 5): class/property DECLARATION subjects (skf:Tier a owl:Class,
// skf:hasReadAccess a owl:ObjectProperty) carry rdf:type objects that are
// owl: vocabulary IRIs, so they are NOT in schemaSubjects — their
// rdfs:label/rdfs:comment/rdfs:domain/rdfs:range triples must not trip
// this test.
//
// PRE-EXISTING QUIRK (comment only, deliberately NO assertion — 45-RESEARCH
// Corrections #2 / Open Question 1): production class declarations emit
// `a owl:Class` (OWL IRI, emitter.go render :219/:237/:252) while
// isSchemaTriple's rdf:type arm checks ns+"Class" (skf:Class). Production
// class-def triples are therefore currently tier-invisible. Out of Phase 45
// scope; recorded so Phase 46 (Entity SHACL Shapes) does not re-discover it
// as new.
func TestEmitterPredicatesWithinAllowlist(t *testing.T) {
	metaPredicates := store.ConformanceMetaPredicates()

	// --- 1. Fixture: one SKILL.md exercising every emitter predicate family
	// (skillName/skillDescription/skillTemperature/poweredBy/hasReadAccess/
	// hasWriteAccess/requiresRefusal) + a KB tree producing tierName/
	// categoryName/subCategoryName/agentTypeName/refusalPhraseText/
	// belongsToTier (modeled on emitter_test.go
	// TestEmitMultiLineDescriptionParses).
	root := t.TempDir()
	skills := filepath.Join(root, "skills")
	kb := filepath.Join(root, "knowledge")
	require.NoError(t, os.MkdirAll(filepath.Join(skills, "multi"), 0o755))
	for _, dir := range []string{
		// SubCategory individuals are only emitted under sql-guides
		// (emitter.go loadKBCategories: `if ent.Name() == "sql-guides"`).
		filepath.Join(kb, "sql-guides", "取数口径"),
		filepath.Join(kb, "authority"),
		filepath.Join(kb, "draft"),
	} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	skill := "---\n" +
		"name: multi\n" +
		"description: |\n" +
		"  第一行描述\n" +
		"  second line\n" +
		"agent: ChatModelAgent\n" +
		"temperature: 0.3\n" +
		"reads:\n" +
		"  - sql-guides/**\n" +
		"  - authority/**\n" +
		"writes:\n" +
		"  - draft/**\n" +
		"required_refusal_phrases:\n" +
		"  - 无法回答该问题\n" +
		"---\n\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(skills, "multi", "SKILL.md"), []byte(skill), 0o644))

	// DefaultNS is mandatory: the parsed IRIs must match the store's ns
	// prefix (the allowlist keys and the schemaSubjectSet derivation both
	// key on it).
	ttl, err := emitter.New(skills, kb, emitter.DefaultNS).Emit()
	require.NoError(t, err)

	// --- 2. Parse through the production store path (NOT a TTL text scan).
	st := store.New()
	_, err = st.LoadFromTurtle(ttl)
	require.NoError(t, err)
	all := store.ConformanceAll(st)
	require.NotEmpty(t, all, "emitter output parsed to 0 triples")

	// --- 3. Same schema-subject derivation QueryScoped Path B uses.
	subjects := store.ConformanceSchemaSubjectSet(st)
	require.NotEmpty(t, subjects,
		"fixture produced no schema-individual subjects — the scoping below would be vacuous")

	// --- 4. Core assertion: schema-individual predicates ⊆ {rdf:type} ∪ metaPredicates.
	var violations []string
	for _, tri := range all {
		if !subjects[tri.S] {
			continue // declaration/ABox subjects are outside the leak-surface invariant
		}
		if tri.P == store.ConformanceRdfType || metaPredicates[tri.P] {
			continue
		}
		violations = append(violations, tri.S+" "+tri.P+" "+tri.O)
	}
	assert.Empty(t, violations,
		"emitter emitted non-allowlisted predicates on schema-individual subjects (D-45-01 anti-drift): extend metaPredicates consciously or fix render(): %v", violations)

	// --- 5. Anti-vacuous coverage: every allowlist family must actually appear
	// in the parsed output — otherwise the ⊆ check above passes on an
	// impoverished fixture and drift on a missing family goes unnoticed.
	var missing []string
	for pred := range metaPredicates {
		found := false
		for _, tri := range all {
			if tri.P == pred {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, pred)
		}
	}
	assert.Empty(t, missing,
		"fixture does not exercise these metaPredicates families (the allowlist check above would pass vacuously on them): %v", missing)

	// --- 6. Negative pin (D-SPARQL-03, authoritative over D-45-01):
	// label/comment are ABox annotations and must NEVER enter the allowlist —
	// adding them would silently break the Phase 44 red-line tests.
	assert.False(t, metaPredicates[store.ConformanceNS+"label"],
		"skf:label must stay OUT of metaPredicates (D-SPARQL-03, authoritative over D-45-01)")
	assert.False(t, metaPredicates[store.ConformanceNS+"comment"],
		"skf:comment must stay OUT of metaPredicates (D-SPARQL-03, authoritative over D-45-01)")
	// rdfs: forms too — the emitter emits rdfs:label/rdfs:comment on
	// declaration subjects; neither IRI form may ever be allowlisted.
	assert.False(t, metaPredicates["http://www.w3.org/2000/01/rdf-schema#label"],
		"rdfs:label must stay OUT of metaPredicates (D-SPARQL-03)")
	assert.False(t, metaPredicates["http://www.w3.org/2000/01/rdf-schema#comment"],
		"rdfs:comment must stay OUT of metaPredicates (D-SPARQL-03)")
}
