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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

const (
	testNS       = "http://skillforge.local/ontology#"
	rdfType      = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	belongsToTier = testNS + "belongsToTier"
	skfSkill     = testNS + "Skill"
	skfTier      = testNS + "Tier"
	skfNote      = testNS + "NoteChunk"
	skfTable     = testNS + "TableSchema"
)

// fixtureKG builds an in-memory store with known triples for testing
// SparqlQueryScoped. Schema triples (owl:Class, skf:Skill, etc.) are never
// filtered; ABox triples (note/tbl) are filtered by tier.
func fixtureKG() *store.Store {
	s := store.New()

	// Schema triples (never filtered, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "Class", P: rdfType, O: testNS + "owl:Class"})
	s.Insert(store.Triple{S: testNS + "ObjectProperty", P: rdfType, O: testNS + "owl:ObjectProperty"})
	s.Insert(store.Triple{S: testNS + "Skill", P: rdfType, O: testNS + "owl:Class"})
	s.Insert(store.Triple{S: testNS + "Tier", P: rdfType, O: testNS + "owl:Class"})

	// Skill individuals (schema, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "skill_sql", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_sql", P: testNS + "hasReadAccess", O: testNS + "sql-guides"})
	s.Insert(store.Triple{S: testNS + "skill_compliance", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_compliance", P: testNS + "hasReadAccess", O: testNS + "authority"})

	// Tier individuals (schema, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "Tier", P: rdfType, O: testNS + "owl:Class"})
	s.Insert(store.Triple{S: testNS + "sql-guides", P: rdfType, O: skfTier})
	s.Insert(store.Triple{S: testNS + "authority", P: rdfType, O: skfTier})

	// ABox triples with tier membership
	// sql-guides tier
	s.Insert(store.Triple{S: testNS + "note_001", P: rdfType, O: skfNote})
	s.Insert(store.Triple{S: testNS + "note_001", P: belongsToTier, O: testNS + "sql-guides"})
	s.Insert(store.Triple{S: testNS + "note_001", P: testNS + "label", O: `"SQL指南第一章"`})
	s.Insert(store.Triple{S: testNS + "note_001", P: testNS + "referencesTable", O: testNS + "tbl_payment_t"})

	// authority tier
	s.Insert(store.Triple{S: testNS + "note_002", P: rdfType, O: skfNote})
	s.Insert(store.Triple{S: testNS + "note_002", P: belongsToTier, O: testNS + "authority"})
	s.Insert(store.Triple{S: testNS + "note_002", P: testNS + "label", O: `"合规政策V2"`})

	// note_003 has NO belongsToTier — fail-closed, invisible when tier filtering active
	s.Insert(store.Triple{S: testNS + "note_003", P: rdfType, O: skfNote})
	s.Insert(store.Triple{S: testNS + "note_003", P: testNS + "label", O: `"无归属笔记"`})

	// Table in sql-guides tier
	s.Insert(store.Triple{S: testNS + "tbl_payment_t", P: rdfType, O: skfTable})
	s.Insert(store.Triple{S: testNS + "tbl_payment_t", P: belongsToTier, O: testNS + "sql-guides"})
	s.Insert(store.Triple{S: testNS + "tbl_payment_t", P: testNS + "label", O: `"支付表"`})

	return s
}

// TestSparqlQueryScoped_AllWildcard tests querying with all empty strings
// returns all KG triples (no filter, no pagination).
func TestSparqlQueryScoped_AllWildcard(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	// All triples including schema + ABox
	assert.Equal(t, 16, resp.Count)
	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Results, 16)
}

// TestSparqlQueryScoped_SubjectFilter tests querying with S specified
// returns only matching triples.
func TestSparqlQueryScoped_SubjectFilter(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{S: testNS + "note_001"}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 3, resp.Count) // rdf:type + belongsToTier + label + referencesTable
	assert.Len(t, resp.Results, 3)
	for _, r := range resp.Results {
		assert.Equal(t, testNS+"note_001", r.S)
	}
}

// TestSparqlQueryScoped_Pagination verifies offset+limit pagination works;
// Count reflects total before limit; Truncated is true when results exceed limit.
func TestSparqlQueryScoped_Pagination(t *testing.T) {
	kg := fixtureKG()
	// 16 total triples; limit=5, offset=0
	pattern := SparqlPattern{Limit: 5, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 16, resp.Count)   // total before pagination
	assert.True(t, resp.Truncated)    // 16 > 5
	assert.Len(t, resp.Results, 5)    // page size = 5
}

// TestSparqlQueryScoped_Truncated results > limit sets Truncated=true.
func TestSparqlQueryScoped_Truncated(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{Limit: 3, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 16, resp.Count)
	assert.True(t, resp.Truncated)
	assert.Len(t, resp.Results, 3)

	// offset beyond count → no truncated
	pattern2 := SparqlPattern{Limit: 5, Offset: 20}
	resp2, err := SparqlQueryScoped(pattern2, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 16, resp2.Count)
	assert.False(t, resp2.Truncated)
	assert.Len(t, resp2.Results, 0)
}

// TestSparqlQueryScoped_TierFilter verifies tier filtering: allowedTiers=["sql-guides"]
// returns only triples whose subjects have belongsToTier=sql-guides; schema triples
// (owl:Class, etc.) are never filtered (D-TIER-03).
func TestSparqlQueryScoped_TierFilter(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{}
	resp, err := SparqlQueryScoped(pattern, []string{"sql-guides"}, kg)
	assert.NoError(t, err)
	// Schema triples: Class/ObjectProperty/Skill/Tier individuals + skill_sql + sql-guides tier
	// + authority tier (schema individual) = 8 schema
	// ABox: note_001 (3 triples) + tbl_payment_t (2 triples) = 5
	// note_002 (authority tier) is NOT sql-guides → excluded
	// note_003 (no tier) → fail-closed → excluded
	assert.Equal(t, 13, resp.Count)
	for _, r := range resp.Results {
		// note_002 and note_003 must not appear
		assert.NotEqual(t, testNS+"note_002", r.S)
		assert.NotEqual(t, testNS+"note_003", r.S)
	}
}

// TestSparqlQueryScoped_FailClosedNoBelongsToTier verifies that an entity with
// no belongsToTier triple is invisible when tier filtering is active (D-TIER-07).
func TestSparqlQueryScoped_FailClosedNoBelongsToTier(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{S: testNS + "note_003"}
	// No tier filtering → note_003 visible
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 2, resp.Count)

	// With tier filtering → note_003 invisible (fail-closed)
	resp2, err := SparqlQueryScoped(pattern, []string{"sql-guides"}, kg)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp2.Count)
}

// TestSparqlQueryScoped_Filter verifies pattern.Filter applies substring match
// on S/P/O values after KG query.
func TestSparqlQueryScoped_Filter(t *testing.T) {
	kg := fixtureKG()
	// Filter on label value (SQL指南)
	pattern := SparqlPattern{P: testNS + "label", Filter: "SQL"}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, `"SQL指南第一章"`, resp.Results[0].O)

	// Filter case-insensitive
	pattern2 := SparqlPattern{P: testNS + "label", Filter: "sql"}
	resp2, err := SparqlQueryScoped(pattern2, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp2.Count)

	// Filter that matches nothing
	pattern3 := SparqlPattern{P: testNS + "label", Filter: "不存在"}
	resp3, err := SparqlQueryScoped(pattern3, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp3.Count)
}

// TestSparqlQueryScoped_InvalidLimit negative limit/offset returns error.
func TestSparqlQueryScoped_InvalidLimit(t *testing.T) {
	kg := fixtureKG()
	_, err := SparqlQueryScoped(SparqlPattern{Limit: -1, Offset: 0}, nil, kg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "negative")

	_, err2 := SparqlQueryScoped(SparqlPattern{Limit: 10, Offset: -5}, nil, kg)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "negative")
}

// TestSparqlQueryScoped_LimitCap verifies limit > 500 is capped to 500.
func TestSparqlQueryScoped_LimitCap(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{Limit: 9999}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 16, resp.Count)
	assert.False(t, resp.Truncated) // 16 < 500, so not truncated
	assert.Len(t, resp.Results, 16)
}

// TestSparqlQueryScoped_DefaultLimit verifies default limit of 50 when 0.
func TestSparqlQueryScoped_DefaultLimit(t *testing.T) {
	kg := fixtureKG()
	// With nil (admin baseline), all 16 triples returned even with default 50
	pattern := SparqlPattern{Limit: 0, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 16, resp.Count)
	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Results, 16)
}
