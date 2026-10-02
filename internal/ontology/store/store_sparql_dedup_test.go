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

// TestQueryScoped_NilTierDedupsDuplicates verifies D-SPARQL-02: when
// allowedTiers is nil/empty and all wildcards are empty, QueryScoped
// MUST dedup by S|P|O before returning. Insert deliberately does NOT
// dedup (audit-trail-preserving contract per store.go Insert docstring),
// so dedup belongs at the query layer so the admin baseline (Path A) is
// consistent with the tier-filtered path (Path B).
//
// Regression for Phase 44 — Bucket A fix.
func TestQueryScoped_NilTierDedupsDuplicates(t *testing.T) {
	s := New()

	// Insert the same triple twice — Insert is no-dedup by design.
	dup := Triple{S: ns + "note_x", P: rdfType, O: ns + "NoteChunk"}
	s.Insert(dup)
	s.Insert(dup)

	// nil allowedTiers + all-wildcard: QueryScoped must dedup.
	out := s.QueryScoped("", "", "", nil)
	assert.Len(t, out, 1, "nil-tier path with all wildcards must dedup duplicates (D-SPARQL-02)")
	if len(out) > 0 {
		assert.Equal(t, dup.S, out[0].S)
		assert.Equal(t, dup.P, out[0].P)
		assert.Equal(t, dup.O, out[0].O)
	}
}

// TestQueryScoped_NilTierEmptyAllowedTiersDedups verifies D-SPARQL-02
// also applies when allowedTiers is an empty slice (not just nil). Both
// forms should produce identical results and both should dedup.
func TestQueryScoped_NilTierEmptyAllowedTiersDedups(t *testing.T) {
	s := New()
	dup := Triple{S: ns + "note_y", P: rdfType, O: ns + "NoteChunk"}
	s.Insert(dup)
	s.Insert(dup)

	outNil := s.QueryScoped("", "", "", nil)
	outEmpty := s.QueryScoped("", "", "", []string{})
	assert.Equal(t, 1, len(outNil), "nil allowedTiers must dedup (D-SPARQL-02)")
	assert.Equal(t, 1, len(outEmpty), "empty []string allowedTiers must dedup (D-SPARQL-02)")
	assert.Equal(t, len(outNil), len(outEmpty), "nil and empty allowedTiers must agree")
}

// TestQueryScoped_NilTierPatternSpecificDoesNotChangeNarrowResults
// verifies that D-SPARQL-02 does NOT regress pattern-specific queries
// (one of subj/pred/obj non-empty). These continue to delegate to
// Query, which returns index-driven narrow results — callers needing
// dedup on pattern queries can apply their own layer.
func TestQueryScoped_NilTierPatternSpecificDoesNotChangeNarrowResults(t *testing.T) {
	s := New()
	s.Insert(Triple{S: ns + "note_a", P: rdfType, O: ns + "NoteChunk"})
	s.Insert(Triple{S: ns + "note_a", P: ns + "belongsToTier", O: ns + "sql-guides"})
	s.Insert(Triple{S: ns + "note_b", P: rdfType, O: ns + "NoteChunk"})

	// Pattern with S specified: should return 2 triples (both for note_a).
	out := s.QueryScoped(ns+"note_a", "", "", nil)
	assert.Len(t, out, 2, "pattern-specific QueryScoped must remain narrow (no regression from D-SPARQL-02)")

	// Pattern with P specified: should return 2 rdf:type triples.
	out2 := s.QueryScoped("", rdfType, "", nil)
	assert.Len(t, out2, 2, "P-pattern QueryScoped must remain narrow")

	// Pattern with O specified: should return 2 NoteChunk triples.
	out3 := s.QueryScoped("", "", ns+"NoteChunk", nil)
	assert.Len(t, out3, 2, "O-pattern QueryScoped must remain narrow")
}

// TestQueryScoped_NilTierNoDuplicatesIsUnchanged verifies that for a
// fixture with NO duplicates, the nil-tier path returns the same total
// count it always did (no accidental dropping of legitimate triples).
func TestQueryScoped_NilTierNoDuplicatesIsUnchanged(t *testing.T) {
	s := New()
	// 3 unique triples — no duplicates.
	s.Insert(Triple{S: ns + "a", P: rdfType, O: ns + "NoteChunk"})
	s.Insert(Triple{S: ns + "a", P: ns + "belongsToTier", O: ns + "t1"})
	s.Insert(Triple{S: ns + "b", P: rdfType, O: ns + "NoteChunk"})

	out := s.QueryScoped("", "", "", nil)
	assert.Len(t, out, 3, "nil-tier path with no duplicates returns all unique triples (no regression)")
}