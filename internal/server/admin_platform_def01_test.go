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

package server

// D-NZK-DEF-01 live-wiring regression tests.
//
// Phase 41/44 hermetic suites hand-feed allowedTiers like ["sql-guides"]
// (see fixtureKG in admin_platform_test.go, line 136) and never exercise
// resolveSkillTiersFromKG against the LIVE KG wiring shape:
//
//	skill_sql-guides hasReadAccess cat_sql-guides   (KBCategory individual)
//	cat_sql-guides  rdf:type       skf:KBCategory
//	cat_sql-guides  belongsToTier  sql-guides        (Tier individual)
//
// The pre-fix resolver passed the hasReadAccess object through
// strings.TrimPrefix verbatim, yielding tier name "cat_sql-guides".
// Store.QueryScoped compares allowedTiers against belongsToTier objects
// (Tier IRIs), so "cat_sql-guides" matched nothing → visibleSubjects empty
// → every ABox row fail-closed filtered for EVERY skill on the live
// endpoint (schema passthrough only).
//
// These tests pin the contract introduced by the D-NZK-DEF-01 fix:
//   - hasReadAccess objects that ARE Tier individuals resolve directly
//     (backward compatible with hand-authored wiring)
//   - hasReadAccess objects that are KBCategory individuals expand through
//     their belongsToTier edge
//   - unknown objects contribute no tier (fail-closed; an all-unknown skill
//     resolves to nil, which callers reject with 401 — never an empty
//     non-nil slice, which Store.QueryScoped would treat as the unfiltered
//     admin baseline)
//   - the resolver output drives QueryScoped such that ABox triples of the
//     expanded tier are visible again (positive visibility restored)

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

const (
	skfKBCategory     = testNS + "KBCategory"
	skfHasReadAccessP = testNS + "hasReadAccess"
)

// fixtureKGLiveCat mirrors the LIVE KG wiring shape (D-NZK-DEF-01): skill
// access points at a KBCategory individual whose tier membership lives on
// belongsToTier. Includes a second skill with legacy direct-Tier wiring and
// a third skill whose access object is neither a Tier nor has belongsToTier
// (fail-closed branch).
func fixtureKGLiveCat() *store.Store {
	s := store.New()

	// Tier individuals (schema, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "sql-guides", P: rdfType, O: skfTier})

	// Live wiring: skill → KBCategory → belongsToTier → Tier
	s.Insert(store.Triple{S: testNS + "skill_sql-guides", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_sql-guides", P: skfHasReadAccessP, O: testNS + "cat_sql-guides"})
	s.Insert(store.Triple{S: testNS + "cat_sql-guides", P: rdfType, O: skfKBCategory})
	s.Insert(store.Triple{S: testNS + "cat_sql-guides", P: belongsToTier, O: testNS + "sql-guides"})

	// Legacy wiring: skill → Tier individual directly
	s.Insert(store.Triple{S: testNS + "skill_direct", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_direct", P: skfHasReadAccessP, O: testNS + "sql-guides"})

	// Fail-closed wiring: skill → unknown object (no rdf:type Tier, no belongsToTier)
	s.Insert(store.Triple{S: testNS + "skill_orphan", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_orphan", P: skfHasReadAccessP, O: testNS + "cat_nothing"})

	// ABox note in the sql-guides tier (the visibility victim)
	s.Insert(store.Triple{S: testNS + "note_live", P: rdfType, O: skfNote})
	s.Insert(store.Triple{S: testNS + "note_live", P: belongsToTier, O: testNS + "sql-guides"})
	s.Insert(store.Triple{S: testNS + "note_live", P: testNS + "label", O: `"live wiring note"`})

	return s
}

func liveCatOnt() *OntologyDeps {
	return &OntologyDeps{KG: fixtureKGLiveCat()}
}

// TestResolveSkillTiersFromKG_LiveCategoryWiring pins the D-NZK-DEF-01
// normalization: a KBCategory-shaped hasReadAccess object expands through
// belongsToTier to the Tier's short name — NOT the raw "cat_" local name.
func TestResolveSkillTiersFromKG_LiveCategoryWiring(t *testing.T) {
	tiers := resolveSkillTiersFromKG("sql-guides", liveCatOnt())
	assert.Equal(t, []string{"sql-guides"}, tiers,
		"live KBCategory wiring must expand cat_sql-guides to its belongsToTier target")
	assert.NotContains(t, tiers, "cat_sql-guides",
		"raw KBCategory local name must never appear as a tier name")
}

// TestResolveSkillTiersFromKG_DirectTierObjectBackwardCompat pins case 1 of
// the normalization: hand-authored wiring pointing hasReadAccess directly at
// a Tier individual keeps resolving to the Tier's short name.
func TestResolveSkillTiersFromKG_DirectTierObjectBackwardCompat(t *testing.T) {
	tiers := resolveSkillTiersFromKG("direct", liveCatOnt())
	assert.Equal(t, []string{"sql-guides"}, tiers)
}

// TestResolveSkillTiersFromKG_UnknownSkillNil pins the unchanged nil
// semantics: a skill missing from the KG resolves to nil.
func TestResolveSkillTiersFromKG_UnknownSkillNil(t *testing.T) {
	assert.Nil(t, resolveSkillTiersFromKG("nope", liveCatOnt()))
	assert.Nil(t, resolveSkillTiersFromKG("nope", nil))
}

// TestResolveSkillTiersFromKG_FailClosedOnUnknownAccessObject pins case 3:
// an access object that is neither a Tier individual nor carries
// belongsToTier contributes no tier. A skill whose every object is unknown
// resolves to nil (NOT an empty non-nil slice — Store.QueryScoped treats
// empty allowedTiers as the unfiltered admin baseline, so an empty slice
// here would fail OPEN at the handler level).
func TestResolveSkillTiersFromKG_FailClosedOnUnknownAccessObject(t *testing.T) {
	tiers := resolveSkillTiersFromKG("orphan", liveCatOnt())
	assert.Nil(t, tiers,
		"all-unknown access objects must resolve to nil (fail-closed 401), never empty non-nil (admin-baseline fail-open)")
}

// TestResolveSkillTiersFromKG_DedupesPreservesOrder pins the dedupe
// contract: multiple access paths reaching the same tier (direct + via two
// categories) yield the tier exactly once, in first-seen order.
func TestResolveSkillTiersFromKG_DedupesPreservesOrder(t *testing.T) {
	s := store.New()
	s.Insert(store.Triple{S: testNS + "tierA", P: rdfType, O: skfTier})
	s.Insert(store.Triple{S: testNS + "tierB", P: rdfType, O: skfTier})
	s.Insert(store.Triple{S: testNS + "skill_multi", P: rdfType, O: skfSkill})
	// Direct Tier first, then two categories both → tierB.
	s.Insert(store.Triple{S: testNS + "skill_multi", P: skfHasReadAccessP, O: testNS + "tierA"})
	s.Insert(store.Triple{S: testNS + "skill_multi", P: skfHasReadAccessP, O: testNS + "cat_x"})
	s.Insert(store.Triple{S: testNS + "skill_multi", P: skfHasReadAccessP, O: testNS + "cat_y"})
	s.Insert(store.Triple{S: testNS + "cat_x", P: rdfType, O: skfKBCategory})
	s.Insert(store.Triple{S: testNS + "cat_x", P: belongsToTier, O: testNS + "tierB"})
	s.Insert(store.Triple{S: testNS + "cat_y", P: rdfType, O: skfKBCategory})
	s.Insert(store.Triple{S: testNS + "cat_y", P: belongsToTier, O: testNS + "tierB"})

	tiers := resolveSkillTiersFromKG("multi", &OntologyDeps{KG: s})
	assert.Equal(t, []string{"tierA", "tierB"}, tiers,
		"duplicate tier reachable via multiple paths must appear exactly once, first-seen order")
}

// TestSparqlQueryScoped_LiveCategoryWiring_ABoxVisible is the end-to-end
// regression: the resolver output must restore positive ABox visibility
// through Store.QueryScoped. This is the exact gap Phase 44's hermetic
// suite missed — it hand-fed ["sql-guides"] instead of the resolver's
// ["cat_sql-guides"] output.
func TestSparqlQueryScoped_LiveCategoryWiring_ABoxVisible(t *testing.T) {
	ont := liveCatOnt()
	tiers := resolveSkillTiersFromKG("sql-guides", ont)
	assert.Equal(t, []string{"sql-guides"}, tiers)

	matches := ont.KG.QueryScoped(testNS+"note_live", "", "", tiers)
	assert.NotEmpty(t, matches, "ABox note of the expanded tier must be visible through the resolver output")

	var sawLabel bool
	for _, m := range matches {
		if m.S == testNS+"note_live" && m.P == testNS+"label" && m.O == `"live wiring note"` {
			sawLabel = true
		}
	}
	assert.True(t, sawLabel, "the note's label triple must be among the visible rows")
}
