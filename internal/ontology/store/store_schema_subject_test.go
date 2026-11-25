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

// TestStoreSchemaSubject_HasReadAccessVisibleLabelHidden pins the β
// semantics boundary at a SINGLE subject (D-45-04 store layer, 45-CONTEXT):
// one test, two assertions, same subject.
//
//   - POSITIVE (D-SPARQL-05, v1.6 Phase 45): a schema individual's
//     allowlisted metadata predicate (hasReadAccess) IS tier-visible in the
//     restrictive tier view — the isSchemaTriple β arm
//     (schemaSubjects[t.S] && metaPredicates[t.P]) classifies it as schema.
//
//   - NEGATIVE (D-SPARQL-03, red line): the SAME subject's non-allowlisted
//     annotation (label) is NOT visible in that same view — label is
//     deliberately excluded from metaPredicates, so the triple falls to the
//     ABox fail-closed path (no belongsToTier on the skill → dropped).
//
// The asymmetry is deliberate: the same subject is simultaneously
// schema-visible (allowlisted predicate) and fail-closed (non-allowlisted
// predicate) — exactly the β semantics boundary.
//
// Uses the package-level unexported ns/rdfType consts directly (in-package
// access; no third namespace alias).
func TestStoreSchemaSubject_HasReadAccessVisibleLabelHidden(t *testing.T) {
	s := New()

	// Fixture: skill_x is a schema individual with both an allowlisted
	// metadata predicate and a non-allowlisted annotation; note_sql anchors
	// a non-empty visible set for the sql-guides view.
	s.Insert(Triple{S: ns + "skill_x", P: rdfType, O: ns + "Skill"})
	s.Insert(Triple{S: ns + "skill_x", P: ns + "hasReadAccess", O: ns + "cat_sql"})
	s.Insert(Triple{S: ns + "skill_x", P: ns + "label", O: `"secret"`})
	s.Insert(Triple{S: ns + "note_sql", P: rdfType, O: ns + "NoteChunk"})
	s.Insert(Triple{S: ns + "note_sql", P: ns + "belongsToTier", O: ns + "sql-guides"})

	// Path B, subject-scoped: exactly the rdf:type triple (existing
	// schemaClasses arm) + the hasReadAccess triple (β arm). The label row
	// is fail-closed (skill_x has no belongsToTier → not in
	// visibleSubjects; label not in metaPredicates → not schema).
	pathB := s.QueryScoped(ns+"skill_x", "", "", []string{"sql-guides"})
	assert.Len(t, pathB, 2,
		"Path B returns exactly rdf:type + hasReadAccess for the schema individual")

	// POSITIVE (D-SPARQL-05): the hasReadAccess row is present.
	foundRead := false
	for _, row := range pathB {
		if row.S == ns+"skill_x" && row.P == ns+"hasReadAccess" && row.O == ns+"cat_sql" {
			foundRead = true
		}
	}
	assert.True(t, foundRead,
		"schema individual's hasReadAccess must be tier-visible (D-SPARQL-05 β arm, D-45-04)")

	// NEGATIVE (D-SPARQL-03 red line): no row in the SAME result carries
	// the label predicate.
	for _, row := range pathB {
		assert.NotEqual(t, ns+"label", row.P,
			"schema individual's label must stay fail-closed in the tier view (D-SPARQL-03 red line)")
	}

	// Path B, pred-scoped: the schema-passthrough contract is preserved.
	// QueryScoped keeps D-SPARQL-05 / D-45-04 -- schema rows bypass the
	// S/P/O filter, so this predicate-scoped query still returns both the
	// rdf:type row and the hasReadAccess row (the admin FailClosed view
	// depends on that passthrough). The exact-lookup variant
	// (QueryScopedExact) is what anchor.BuildIndex uses; that is the one
	// that must not leak non-matching schema rows.
	predB := s.QueryScoped("", ns+"hasReadAccess", "", []string{"sql-guides"})
	assert.Len(t, predB, 2,
		"pred-scoped Path B returns both schema rows (schema passthrough precedes the pattern filter; D-SPARQL-05)")
	predExact := s.QueryScopedExact("", ns+"hasReadAccess", "", []string{"sql-guides"})
	assert.Len(t, predExact, 1,
		"the exact lookup returns ONLY the pattern-matching row")
	foundReadPred := false
	for _, row := range predB {
		if row.S == ns+"skill_x" && row.P == ns+"hasReadAccess" && row.O == ns+"cat_sql" {
			foundReadPred = true
		}
		assert.NotEqual(t, ns+"label", row.P,
			"pred-scoped Path B must not leak the label row (D-SPARQL-03 red line)")
	}
	assert.True(t, foundReadPred,
		"pred-scoped Path B contains the hasReadAccess row (D-SPARQL-05 β arm)")

	// Path A sanity (nil allowedTiers): admin baseline sees all 3 skill_x
	// triples (rdf:type + hasReadAccess + label; early-return never consults
	// isSchemaTriple, D-45-03).
	pathA := s.QueryScoped(ns+"skill_x", "", "", nil)
	assert.Len(t, pathA, 3,
		"admin baseline sees rdf:type + hasReadAccess + label (Path A, no filtering)")
}
