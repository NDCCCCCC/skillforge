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

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Local test namespace — mirrors the value in
// internal/server/admin_platform_test.go (cannot import: package mismatch).
// MUST stay in sync with admin_platform_test.go:30.
const (
	testNSLocal   = "http://skillforge.local/ontology#"
	rdfTypeLocal  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	belongsLocal  = testNSLocal + "belongsToTier"
	skfSkillLocal = testNSLocal + "Skill"
	skfNoteLocal  = testNSLocal + "NoteChunk"
)

// TestIsSchemaTriple_LabelAndCommentAreABox verifies D-SPARQL-03:
// isSchemaTriple returns false for label/comment predicates regardless
// of subject. The only schema-via-predicate predicates are domain,
// range, subClassOf, subPropertyOf (OWL/RDFS ontology metadata).
//
// Schema subjects (e.g., (skill_a, rdf:type, skf:Skill)) are still
// classified as schema via the rdf:type+schemaClasses[O] arm.
//
// OWL predicates (domain/range) still classify as schema.
func TestIsSchemaTriple_LabelAndCommentAreABox(t *testing.T) {
	// (note_x, label, "y") with note_x rdf:type skf:Note — ABox, not schema.
	noteWithLabel := Triple{
		S: testNSLocal + "note_x",
		P: testNSLocal + "label",
		O: `"y"`,
	}
	assert.False(t, isSchemaTriple(noteWithLabel),
		"label predicate on ABox subject must NOT be schema (D-SPARQL-03)")

	// (skill_a, label, "y") with skill_a rdf:type skf:Skill — ABox too.
	// The rdf:type+skf:Skill arm is schema for the rdf:type triple; the
	// label triple itself is NOT schema and should fall through to ABox
	// visibility check.
	skillLabel := Triple{
		S: testNSLocal + "skill_a",
		P: testNSLocal + "label",
		O: `"y"`,
	}
	assert.False(t, isSchemaTriple(skillLabel),
		"label predicate on schema individual subject must NOT be schema (D-SPARQL-03)")

	// (skill_a, rdf:type, skf:Skill) — schema via the schemaClasses arm.
	skillType := Triple{
		S: testNSLocal + "skill_a",
		P: rdfTypeLocal,
		O: skfSkillLocal,
	}
	assert.True(t, isSchemaTriple(skillType),
		"rdf:type with skf:Skill object must be schema (schemaClasses arm)")

	// (prop_x, domain, Class) — OWL predicate still schema.
	domainTriple := Triple{
		S: testNSLocal + "prop_x",
		P: testNSLocal + "domain",
		O: testNSLocal + "Class",
	}
	assert.True(t, isSchemaTriple(domainTriple),
		"domain predicate must remain schema (OWL/RDFS ontology metadata)")

	// (prop_x, range, Class) — OWL predicate still schema.
	rangeTriple := Triple{
		S: testNSLocal + "prop_x",
		P: testNSLocal + "range",
		O: testNSLocal + "Class",
	}
	assert.True(t, isSchemaTriple(rangeTriple),
		"range predicate must remain schema (OWL/RDFS ontology metadata)")

	// (prop_x, subClassOf, Skill) — OWL predicate still schema.
	subClassTriple := Triple{
		S: testNSLocal + "skill_child",
		P: testNSLocal + "subClassOf",
		O: skfSkillLocal,
	}
	assert.True(t, isSchemaTriple(subClassTriple),
		"subClassOf predicate must remain schema (OWL/RDFS ontology metadata)")

	// (prop_x, subPropertyOf, ...) — OWL predicate still schema.
	subPropTriple := Triple{
		S: testNSLocal + "prop_child",
		P: testNSLocal + "subPropertyOf",
		O: testNSLocal + "prop_parent",
	}
	assert.True(t, isSchemaTriple(subPropTriple),
		"subPropertyOf predicate must remain schema (OWL/RDFS ontology metadata)")

	// (note_x, comment, "y") — ABox, not schema (same as label).
	commentTriple := Triple{
		S: testNSLocal + "note_x",
		P: testNSLocal + "comment",
		O: `"y"`,
	}
	assert.False(t, isSchemaTriple(commentTriple),
		"comment predicate on ABox subject must NOT be schema (D-SPARQL-03)")
}

// TestStore_LabelAndCommentNotSchemaPassthrough verifies D-SPARQL-06
// end-to-end: tier-filtered queries MUST NOT leak label/comment triples
// for non-visible subjects. Locks the D-SPARQL-03 fix against future
// regressions.
//
// Scenario:
//   - note_authority belongs to authority tier with a label and comment
//   - note_oracle has NO belongsToTier (fail-closed) with a label
//   - QueryScoped with allowedTiers=["sql-guides"] MUST NOT return
//     either subject's label/comment (fail-closed ABox semantics)
//   - Admin baseline (allowedTiers=nil) MUST return all labels
func TestStore_LabelAndCommentNotSchemaPassthrough(t *testing.T) {
	s := New()

	// note_authority in authority tier with a label and comment
	s.Insert(Triple{S: testNSLocal + "note_authority", P: rdfTypeLocal, O: skfNoteLocal})
	s.Insert(Triple{S: testNSLocal + "note_authority", P: belongsLocal, O: testNSLocal + "authority"})
	s.Insert(Triple{S: testNSLocal + "note_authority", P: testNSLocal + "label", O: `"SECRET"`})
	s.Insert(Triple{S: testNSLocal + "note_authority", P: testNSLocal + "comment", O: `"TOP SECRET"`})

	// note_oracle has NO belongsToTier (fail-closed) with a label
	s.Insert(Triple{S: testNSLocal + "note_oracle", P: rdfTypeLocal, O: skfNoteLocal})
	s.Insert(Triple{S: testNSLocal + "note_oracle", P: testNSLocal + "label", O: `"ORACLE"`})

	// Tier-filtered query for sql-guides MUST NOT leak authority/oracle labels.
	out := s.QueryScoped("", testNSLocal+"label", "", []string{"sql-guides"})
	assert.Empty(t, out,
		"label triple for non-visible subject must not leak (D-SPARQL-06 fail-closed)")

	// Tier-filtered query for comment predicate also MUST NOT leak.
	out2 := s.QueryScoped("", testNSLocal+"comment", "", []string{"sql-guides"})
	assert.Empty(t, out2,
		"comment triple for non-visible subject must not leak (D-SPARQL-06 fail-closed)")

	// Sanity: without tier filter (admin baseline), labels are visible.
	out3 := s.QueryScoped("", testNSLocal+"label", "", nil)
	assert.Len(t, out3, 2,
		"admin baseline sees all labels (Path A has no filter; D-TIER-01)")
}