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

// D-NZK-DEF-01 sister-defect regression tests for (*Deps).resolveSkillTiers
// (the /admin/ontology KG query panel resolver).
//
// 260928-wzl fixed resolveSkillTiersFromKG (admin_platform.go — the
// /api/sparql + hybrid resolvers) for the live KG wiring shape:
//
//	skill_x hasReadAccess cat_x   (KBCategory individual)
//	cat_x          belongsToTier  tier_x            (Tier individual)
//
// (*Deps).resolveSkillTiers — the sibling resolver feeding
// adminOntologyKGQueryHandler's ?skill= param — carried the same
// pre-fix bug: it passed every hasReadAccess object through
// strings.TrimPrefix verbatim, yielding tier names like
// "cat_sql-guides". Store.QueryScoped compares allowedTiers against
// belongsToTier objects (Tier IRIs), so the raw "cat_" local name
// matched nothing → every ABox row fail-closed filtered on the
// /admin/ontology KG query panel for EVERY skill, while the same
// query through /api/sparql returned rows. One KG, two resolvers,
// two answers.
//
// These tests pin the same contract as admin_platform_def01_test.go:
//   - hasReadAccess objects that ARE Tier individuals resolve directly
//   - KBCategory-shaped objects expand through belongsToTier
//   - unknown objects contribute no tier (fail-closed → nil)
//   - dedupe preserving first-seen order
//   - resolver output restores ABox visibility through QueryScoped

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// ontologyResolverDeps wraps a KG store in the Deps shape
// (*Deps).resolveSkillTiers reads.
func ontologyResolverDeps(s *store.Store) *Deps {
	return &Deps{Ontology: &OntologyDeps{KG: s}}
}

// TestResolveSkillTiers_LiveCategoryWiring pins the D-NZK-DEF-01
// normalization on the ontology-panel resolver: a KBCategory-shaped
// hasReadAccess object expands through belongsToTier to the Tier's
// short name — NOT the raw "cat_" local name.
func TestResolveSkillTiers_LiveCategoryWiring(t *testing.T) {
	tiers := ontologyResolverDeps(fixtureKGLiveCat()).resolveSkillTiers("sql-guides")
	assert.Equal(t, []string{"sql-guides"}, tiers,
		"live KBCategory wiring must expand cat_sql-guides to its belongsToTier target")
	assert.NotContains(t, tiers, "cat_sql-guides",
		"raw KBCategory local name must never appear as a tier name")
}

// TestResolveSkillTiers_DirectTierObjectBackwardCompat pins case 1:
// hand-authored wiring pointing hasReadAccess directly at a Tier
// individual keeps resolving to the Tier's short name.
func TestResolveSkillTiers_DirectTierObjectBackwardCompat(t *testing.T) {
	tiers := ontologyResolverDeps(fixtureKGLiveCat()).resolveSkillTiers("direct")
	assert.Equal(t, []string{"sql-guides"}, tiers)
}

// TestResolveSkillTiers_UnknownSkillNil pins the nil semantics: a
// skill missing from the KG resolves to nil.
func TestResolveSkillTiers_UnknownSkillNil(t *testing.T) {
	assert.Nil(t, ontologyResolverDeps(fixtureKGLiveCat()).resolveSkillTiers("nope"))
}

// TestResolveSkillTiers_FailClosedOnUnknownAccessObject pins case 3:
// an access object that is neither a Tier individual nor carries
// belongsToTier contributes no tier. A skill whose every object is
// unknown resolves to nil.
func TestResolveSkillTiers_FailClosedOnUnknownAccessObject(t *testing.T) {
	tiers := ontologyResolverDeps(fixtureKGLiveCat()).resolveSkillTiers("orphan")
	assert.Nil(t, tiers,
		"all-unknown access objects must resolve to nil (fail-closed), never [\"cat_nothing\"]")
}

// TestResolveSkillTiers_DedupesPreservesOrder pins the dedupe
// contract: multiple access paths reaching the same tier yield the
// tier exactly once, in first-seen order.
func TestResolveSkillTiers_DedupesPreservesOrder(t *testing.T) {
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

	tiers := ontologyResolverDeps(s).resolveSkillTiers("multi")
	assert.Equal(t, []string{"tierA", "tierB"}, tiers,
		"duplicate tier reachable via multiple paths must appear exactly once, first-seen order")
}

// TestOntologyKGQuery_ResolverDrivesABoxVisibility is the end-to-end
// regression: through the SAME store, the /api/sparql resolver output
// and the /admin/ontology panel resolver output must agree — the
// ABox note of the expanded tier is visible via both.
func TestOntologyKGQuery_ResolverDrivesABoxVisibility(t *testing.T) {
	s := fixtureKGLiveCat()
	dep := ontologyResolverDeps(s)

	tiers := dep.resolveSkillTiers("sql-guides")
	assert.Equal(t, []string{"sql-guides"}, tiers)

	matches := s.QueryScoped(testNS+"note_live", "", "", tiers)
	assert.NotEmpty(t, matches,
		"ABox note of the expanded tier must be visible through the ontology-panel resolver output")

	var sawLabel bool
	for _, m := range matches {
		if m.S == testNS+"note_live" && m.P == testNS+"label" && m.O == `"live wiring note"` {
			sawLabel = true
		}
	}
	assert.True(t, sawLabel, "the note's label triple must be among the visible rows")
}
