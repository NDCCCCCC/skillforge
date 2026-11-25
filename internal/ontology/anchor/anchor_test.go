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

// Phase 57-01 (ONT-02) -- the entity dictionary pins: tier scoping,
// longest-match precision, stable ordering, and the EXPLICIT degraded
// posture (a nil KG is never a silent empty success).
package anchor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

const (
	iriTravel = "http://skillforge.local/ontology#tbl_travel"
	iriHotel  = "http://skillforge.local/ontology#tbl_hotel"
	iriNote   = "http://skillforge.local/ontology#note_001"
	tierAuth  = "http://skillforge.local/ontology#belongsToTier"
	tierDraft = tierAuth
)

func seedKG() *store.Store {
	kg := store.New()
	// Authority-tier entities (visible to an authority-reading Skill).
	kg.Insert(store.Triple{S: iriTravel, P: LabelPredicate, O: `"差旅住宿费标准"`})
	kg.Insert(store.Triple{S: iriTravel, P: tierAuth, O: "http://skillforge.local/ontology#权威"})
	kg.Insert(store.Triple{S: iriHotel, P: LabelPredicate, O: `"住宿费"`})
	kg.Insert(store.Triple{S: iriHotel, P: tierAuth, O: "http://skillforge.local/ontology#权威"})
	kg.Insert(store.Triple{S: iriNote, P: LabelPredicate, O: `"年假制度"`})
	kg.Insert(store.Triple{S: iriNote, P: tierAuth, O: "http://skillforge.local/ontology#权威"})
	// Draft-tier entity the authority Skill must NOT see.
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#draft_entity", P: LabelPredicate, O: `"草稿里的实体"`})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#draft_entity", P: tierDraft, O: "http://skillforge.local/ontology#草稿"})
	return kg
}

func TestBuildIndexNilStoreIsUnavailableNotEmpty(t *testing.T) {
	ix := BuildIndex(nil, nil)
	require.NotNil(t, ix)
	assert.False(t, ix.Available(), "a nil KG must report unavailable so the caller degrades loudly")
	assert.Equal(t, 0, ix.Len())
	assert.Nil(t, ix.Match("差旅住宿费标准", 0), "an unavailable index never fabricates matches")
}

func TestBuildIndexReadsLabelsAndDropsUnusableOnes(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: "http://x#a", P: LabelPredicate, O: `""`})
	kg.Insert(store.Triple{S: "http://x#b", P: LabelPredicate, O: `"http://x#b"`}) // label == IRI
	kg.Insert(store.Triple{S: "http://x#c", P: LabelPredicate, O: `" 真实标签 "`})
	kg.Insert(store.Triple{S: "http://x#a", P: belongsToTierPredicate, O: "http://skillforge.local/ontology#sql-guides"})
	kg.Insert(store.Triple{S: "http://x#b", P: belongsToTierPredicate, O: "http://skillforge.local/ontology#sql-guides"})
	kg.Insert(store.Triple{S: "http://x#c", P: belongsToTierPredicate, O: "http://skillforge.local/ontology#sql-guides"})

	ix := BuildIndex(kg, nil)
	assert.True(t, ix.Available())
	assert.Equal(t, 1, ix.Len(), "empty labels and IRI-as-label entries are unusable and dropped")
	assert.Equal(t, "真实标签", ix.entries[0].Label, "surrounding quotes/space are trimmed")
}

func TestMatchLongestLabelWinsAndNestedLabelsDoNotDoubleCount(t *testing.T) {
	ix := BuildIndex(seedKG(), []string{"权威"})
	matches := ix.Match("差旅住宿费标准是多少？住宿费能报吗？", 0)

	require.GreaterOrEqual(t, len(matches), 2)
	// Longest label anchored first.
	assert.Equal(t, iriTravel, matches[0].IRI)
	assert.Equal(t, "差旅住宿费标准", matches[0].Label)
	// The nested "住宿费" inside the already-anchored span is masked, but
	// the SECOND independent mention still anchors (masking is span-local).
	second := matches[1]
	assert.Equal(t, iriHotel, second.IRI)
}

func TestMatchIsStableAndBounded(t *testing.T) {
	ix := BuildIndex(seedKG(), []string{"权威"})
	a := ix.Match("差旅住宿费标准 住宿费 年假制度", 2)
	b := ix.Match("差旅住宿费标准 住宿费 年假制度", 2)
	require.Equal(t, a, b, "the same query must anchor identically every time")
	assert.Len(t, a, 2, "maxResults bounds prompt growth")
}

func TestMatchNormalizesWhitespaceAndWidth(t *testing.T) {
	ix := BuildIndex(seedKG(), []string{"权威"})
	assert.Len(t, ix.Match("差旅 住宿费标准", 0), 1, "spaces inside the label are collapsed before matching")
	assert.Empty(t, ix.Match("与知识库无关的问题", 0))
}

func TestMatchHonoursTierScoping(t *testing.T) {
	ix := BuildIndex(seedKG(), []string{"权威"})
	assert.Empty(t, ix.Match("草稿里的实体", 0),
		"an authority Skill must not anchor on a draft-tier entity (tier PRE-filter, D-TIER-07)")

	ixDraft := BuildIndex(seedKG(), []string{"草稿"})
	assert.NotEmpty(t, ixDraft.Match("草稿里的实体", 0),
		"the same dictionary with a draft scope anchors the draft entity")
}

func TestNeighborsWalksLabelledOutgoingEdges(t *testing.T) {
	kg := seedKG()
	// The travel table references a table entity; the tier edge must be
	// skipped so the context does not flood with tier nodes.
	kg.Insert(store.Triple{
		S: iriTravel,
		P: "http://skillforge.local/ontology#referencesTable",
		O: "http://skillforge.local/ontology#tbl_expense",
	})
	kg.Insert(store.Triple{
		S: "http://skillforge.local/ontology#tbl_expense",
		P: LabelPredicate,
		O: `"费用报销表"`,
	})
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#tbl_expense", P: belongsToTierPredicate, O: "http://skillforge.local/ontology#authority"})

	ix := BuildIndex(kg, nil)
	matches := ix.Match("差旅住宿费标准", 0)
	require.NotEmpty(t, matches)

	neighbors := ix.Neighbors(kg, matches, 1, 8)
	require.NotEmpty(t, neighbors)
	found := false
	for _, n := range neighbors {
		if n.Entity.Label == "费用报销表" {
			found = true
			assert.Equal(t, "referencesTable", n.Predicate,
				"the predicate is rendered as a local name so the model can read it")
		}
		assert.NotEqual(t, "belongsToTier", n.Predicate,
			"structural tier edges must never enter prompt context")
	}
	assert.True(t, found)
}

func TestNeighborsUnavailableIndexReturnsNil(t *testing.T) {
	ix := BuildIndex(nil, nil)
	assert.Nil(t, ix.Neighbors(seedKG(), []Match{{IRI: iriTravel}}, 1, 8),
		"an unavailable index must not fabricate a neighbourhood")
}

func TestNeighborsHonoursLimit(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: iriTravel, P: LabelPredicate, O: `"差旅住宿费标准"`})
	kg.Insert(store.Triple{S: iriTravel, P: belongsToTierPredicate, O: "http://skillforge.local/ontology#authority"})
	for i := 0; i < 20; i++ {
		obj := fmt.Sprintf("http://skillforge.local/ontology#tbl_%02d", i)
		kg.Insert(store.Triple{S: iriTravel, P: "http://skillforge.local/ontology#referencesTable", O: obj})
		kg.Insert(store.Triple{S: obj, P: LabelPredicate, O: fmt.Sprintf("\"表%d\"", i)})
		kg.Insert(store.Triple{S: obj, P: belongsToTierPredicate, O: "http://skillforge.local/ontology#authority"})
	}
	ix := BuildIndex(kg, nil)
	got := ix.Neighbors(kg, ix.Match("差旅住宿费标准", 0), 1, 5)
	assert.Len(t, got, 5, "the cap bounds prompt growth")
}
