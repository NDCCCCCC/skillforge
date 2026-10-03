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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

const (
	testNS        = "http://skillforge.local/ontology#"
	rdfType       = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	belongsToTier = testNS + "belongsToTier"
	skfSkill      = testNS + "Skill"
	skfTier       = testNS + "Tier"
	skfNote       = testNS + "NoteChunk"
	skfTable      = testNS + "TableSchema"
)

// fixtureKG builds an in-memory store with known triples for testing
// SparqlQueryScoped. Schema triples (skf:Class, skf:ObjectProperty, etc.)
// are never filtered; ABox triples (note/tbl) are filtered by tier.
//
// ===========================================================================
// SPARQL fixture triple inventory (post D-SPARQL-02/03/04 fixes, Phase 44;
// Path B counts updated per D-SPARQL-05 β — v1.6 Phase 45, D-45-04)
// ===========================================================================
// fixtureKG() inserts 22 unique S|P|O triples (the original 23 raw inserts
// minus 1 line-58 duplicate removed by D-SPARQL-04).
//
// Breakdown by category:
//   1. Schema class definitions (4 rdf:type triples): testNS+Class,
//      testNS+ObjectProperty, testNS+Skill, testNS+Tier -> rdf:type with
//      object = Class/ObjectProperty. Caught by isSchemaTriple's rdf:type
//      arm (store.go:605-617) + the owl-IRI fix (D-SPARQL-04).
//
//   2. Schema individual types (4 rdf:type triples): skill_sql,
//      skill_compliance -> rdf:type skf:Skill; sql-guides, authority ->
//      rdf:type skf:Tier. Caught by isSchemaTriple's schemaClasses arm
//      (store.go:614-616).
//
//   3. Skill hasReadAccess triples (2 triples): skill_sql -> sql-guides,
//      skill_compliance -> authority. Schema under D-SPARQL-05 (v1.6
//      Phase 45, β arm / D-45-04): both subjects carry
//      rdf:type -> skf:Skill (skf:Skill is in schemaClasses) and
//      hasReadAccess is in the metaPredicates allowlist, so
//      isSchemaTriple's subject∩predicate arm classifies both triples as
//      schema. -> INCLUDED in EVERY Path B tier-filtered view (schema
//      passthrough is tier-agnostic) and on Path A (nil-tier) admin
//      baseline. skill_compliance->authority is included alongside
//      skill_sql->sql-guides regardless of allowedTiers.
//
//   4. ABox note_001 (4 triples, sql-guides tier): rdf:type NoteChunk +
//      belongsToTier sql-guides + label "SQL指南第一章" +
//      referencesTable tbl_payment_t. All 4 visible on Path A; on Path B
//      with sql-guides, subject is visible so ALL 4 included (D-TIER-06:
//      "if subject is visible, ALL its edges are included").
//
//   5. ABox note_002 (3 triples, authority tier): rdf:type + belongsToTier
//      authority + label "合规政策V2". Visible on Path A. On Path B with
//      sql-guides, subject NOT visible -> ALL 3 EXCLUDED (D-TIER-03
//      fail-closed).
//
//   6. ABox note_003 (2 triples, NO tier): rdf:type + label "无归属笔记".
//      Visible on Path A (both triples). On Path B with sql-guides,
//      subject NOT visible (no belongsToTier) -> ALL 2 EXCLUDED
//      (D-TIER-03 fail-closed + D-TIER-07).
//
//   7. ABox tbl_payment_t (3 triples, sql-guides tier): rdf:type
//      TableSchema + belongsToTier sql-guides + label "支付表". Visible
//      on Path A; on Path B with sql-guides, subject is visible so ALL 3
//      included.
//
//   TOTALS:
//     Path A (nil allowedTiers) - admin baseline, no filtering:
//       = 4 (schema defs) + 4 (schema individuals) + 2 (hasReadAccess) +
//         4 (note_001) + 3 (note_002) + 2 (note_003) + 3 (tbl_payment_t)
//       = 22 unique triples
//
//     Path B (allowedTiers=["sql-guides"]):
//       Schema (always visible via isSchemaTriple short-circuit at
//               store.go:550-554, D-TIER-03 + D-SPARQL-05 β):
//         - 4 schema class defs (rdf:type+Class/ObjectProperty arm)
//         - 4 schema individual types (rdf:type+schemaClasses arm)
//         - 2 hasReadAccess (β subject∩predicate arm, D-SPARQL-05:
//           skill_sql/skill_compliance carry rdf:type skf:Skill ∈
//           schemaClasses and hasReadAccess ∈ metaPredicates; schema
//           passthrough is tier-agnostic, so both appear in every tier view)
//         = 10
//       ABox (subject in visibleSubjects={note_001, tbl_payment_t}
//             computed at store.go:528-536 from belongsToTier triples):
//         - note_001: 4 triples (subject visible, D-TIER-06 includes all edges)
//         - tbl_payment_t: 3 triples (subject visible)
//       = 10 schema + 4 + 3 = 17
//
//       Subjects NOT in visibleSubjects (excluded by ABox path):
//         - note_002 (authority tier not in allowedTiers) — fail-closed
//         - note_003 (no belongsToTier) — fail-closed (D-TIER-07)
//
//     Path B (S=note_003, allowedTiers=["sql-guides"]):
//       Schema: 10 (8 schema + 2 hasReadAccess — schema passthrough
//               precedes the S/P/O filter at store.go:543-554, so all 10
//               appear even though S=note_003 matches none of them)
//       ABox: 0 (subject not in visibleSubjects -> fail-closed)
//       = 10
//
//     Path A (S=note_003, nil):
//       ABox: 2 (rdf:type + label; subject not constrained on Path A)
//       = 2
//
//     Path A (S=note_001, nil):
//       ABox: 4 (rdf:type + belongsToTier + label + referencesTable)
//       = 4
// ===========================================================================
func fixtureKG() *store.Store {
	s := store.New()

	// Schema triples (never filtered, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "Class", P: rdfType, O: testNS + "Class"})
	s.Insert(store.Triple{S: testNS + "ObjectProperty", P: rdfType, O: testNS + "ObjectProperty"})
	s.Insert(store.Triple{S: testNS + "Skill", P: rdfType, O: testNS + "Class"})
	s.Insert(store.Triple{S: testNS + "Tier", P: rdfType, O: testNS + "Class"})

	// Skill individuals (schema, D-TIER-03)
	s.Insert(store.Triple{S: testNS + "skill_sql", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_sql", P: testNS + "hasReadAccess", O: testNS + "sql-guides"})
	s.Insert(store.Triple{S: testNS + "skill_compliance", P: rdfType, O: skfSkill})
	s.Insert(store.Triple{S: testNS + "skill_compliance", P: testNS + "hasReadAccess", O: testNS + "authority"})

	// Tier individuals (schema, D-TIER-03)
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
	// All 22 unique fixture triples on Path A (admin baseline, no filter):
	// 4 schema defs + 4 schema individuals + 2 hasReadAccess + 4 note_001 +
	// 3 note_002 + 2 note_003 + 3 tbl_payment_t = 22. See fixtureKG() godoc.
	// Updated from 16 -> 22 per D-SPARQL-04b (Phase 44 closure).
	assert.Equal(t, 22, resp.Count)
	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Results, 22)
}

// TestSparqlQueryScoped_SubjectFilter tests querying with S specified
// returns only matching triples.
func TestSparqlQueryScoped_SubjectFilter(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{S: testNS + "note_001"}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	// note_001 has 4 triples: rdf:type + belongsToTier + label + referencesTable.
	// Path A (nil allowedTiers) early-returns to s.Query which does NOT
	// consult isSchemaTriple. Test author's inline comment at original
	// line 106 already said 4; the 3 assertion was a typo/miscount.
	// Updated from 3 -> 4 per D-SPARQL-04b (Phase 44 closure).
	assert.Equal(t, 4, resp.Count)
	assert.Len(t, resp.Results, 4)
	for _, r := range resp.Results {
		assert.Equal(t, testNS+"note_001", r.S)
	}
}

// TestSparqlQueryScoped_Pagination verifies offset+limit pagination works;
// Count reflects total before limit; Truncated is true when results exceed limit.
func TestSparqlQueryScoped_Pagination(t *testing.T) {
	kg := fixtureKG()
	// 22 total triples; limit=5, offset=0
	pattern := SparqlPattern{Limit: 5, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	// Path A total before pagination. Updated from 16 -> 22 per D-SPARQL-04b.
	assert.Equal(t, 22, resp.Count) // total before pagination
	assert.True(t, resp.Truncated)  // 22 > 5
	assert.Len(t, resp.Results, 5)  // page size = 5
}

// TestSparqlQueryScoped_Truncated results > limit sets Truncated=true.
func TestSparqlQueryScoped_Truncated(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{Limit: 3, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	// Path A count. Updated from 16 -> 22 per D-SPARQL-04b.
	assert.Equal(t, 22, resp.Count)
	assert.True(t, resp.Truncated)
	assert.Len(t, resp.Results, 3)

	// offset beyond start-of-last-page but within range → 2 items
	// Implementation: page = triples[offset:min(offset+limit, count)].
	// For total=22, limit=5, offset=20: page = triples[20:min(25,22)] = triples[20:22] = 2 items.
	// The plan/research originally asserted 0 here ("offset beyond count"), but
	// count > offset so the page is non-empty (the last count-offset items).
	pattern2 := SparqlPattern{Limit: 5, Offset: 20}
	resp2, err := SparqlQueryScoped(pattern2, nil, kg)
	assert.NoError(t, err)
	// Path A count (offset before end of page). Updated 16 -> 22 per D-SPARQL-04b.
	assert.Equal(t, 22, resp2.Count)
	assert.False(t, resp2.Truncated)
	assert.Len(t, resp2.Results, 2)
}

// TestSparqlQueryScoped_TierFilter verifies tier filtering: allowedTiers=["sql-guides"]
// returns only triples whose subjects have belongsToTier=sql-guides; schema triples
// (skf:Class, etc.) are never filtered (D-TIER-03).
func TestSparqlQueryScoped_TierFilter(t *testing.T) {
	kg := fixtureKG()
	pattern := SparqlPattern{}
	resp, err := SparqlQueryScoped(pattern, []string{"sql-guides"}, kg)
	assert.NoError(t, err)
	// D-SPARQL-05 (v1.6 Phase 45) / D-45-04 (45-CONTEXT): intended behavior
	// change — the β subject∩predicate arm makes the 2 hasReadAccess triples
	// schema-visible (skill_sql->sql-guides AND skill_compliance->authority;
	// schema passthrough is tier-agnostic). Count derivation:
	// 8 schema (always visible, D-TIER-03) + 2 hasReadAccess (β arm) +
	//   note_001 visible (sql-guides tier) -> rdf:type + belongsToTier + label +
	//     referencesTable = 4
	//   tbl_payment_t visible (sql-guides tier) -> rdf:type + belongsToTier +
	//     label = 3
	// = 17
	// note_002 (authority tier) and note_003 (no tier) MUST NOT appear
	// (fail-closed; D-SPARQL-03 red line preserved under β).
	assert.Equal(t, 17, resp.Count)
	// D-SPARQL-05 positive: skill_sql's hasReadAccess triple is tier-visible.
	found := false
	for _, r := range resp.Results {
		if r.S == testNS+"skill_sql" && r.P == testNS+"hasReadAccess" && r.O == testNS+"sql-guides" {
			found = true
		}
	}
	assert.True(t, found,
		"skill_sql hasReadAccess must be tier-visible under D-SPARQL-05 β (D-45-04 positive assertion)")
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
	// No tier filtering → note_003 visible (both triples)
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	assert.Equal(t, 2, resp.Count)

	// With tier filtering → note_003 ABox triples invisible (fail-closed).
	// D-SPARQL-05 (v1.6 Phase 45) / D-45-04 (45-CONTEXT): intended behavior
	// change — schema passthrough short-circuits on isSchemaTriple BEFORE
	// applying the S/P/O pattern filter (store.go:543-554). Under β the
	// 2 hasReadAccess triples are schema too (subject∩predicate arm), so
	// ALL 8 schema triples PLUS 2 hasReadAccess = 10 appear even though
	// S=note_003 matches none of them. Schema passthrough is tier-agnostic:
	// skill_compliance->authority rides along with skill_sql->sql-guides.
	// note_003 has no belongsToTier → visibleSubjects does not include
	// note_003 → all 2 of its ABox triples are EXCLUDED (correct fail-closed).
	resp2, err := SparqlQueryScoped(pattern, []string{"sql-guides"}, kg)
	assert.NoError(t, err)
	assert.Equal(t, 10, resp2.Count) // 8 schema + 2 hasReadAccess (β); 0 ABox (note_003 fail-closed)
	// D-SPARQL-05 positive: the hasReadAccess row rides schema passthrough
	// (which precedes the S/P/O filter) into this S=note_003 query.
	found := false
	for _, r := range resp2.Results {
		if r.S == testNS+"skill_sql" && r.P == testNS+"hasReadAccess" && r.O == testNS+"sql-guides" {
			found = true
		}
	}
	assert.True(t, found,
		"skill_sql hasReadAccess must appear in Path B even when S=note_003 (schema passthrough precedes pattern filter; D-SPARQL-05, D-45-04)")
	for _, r := range resp2.Results {
		// note_003 must not appear at all (fail-closed ABox)
		assert.NotEqual(t, testNS+"note_003", r.S)
	}
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
	// Path A returns all 22. Updated from 16 -> 22 per D-SPARQL-04b.
	assert.Equal(t, 22, resp.Count)
	assert.False(t, resp.Truncated) // 22 < 500, so not truncated
	assert.Len(t, resp.Results, 22)
}

// TestSparqlQueryScoped_DefaultLimit verifies default limit of 50 when 0.
func TestSparqlQueryScoped_DefaultLimit(t *testing.T) {
	kg := fixtureKG()
	// With nil (admin baseline), all 22 triples returned even with default 50
	pattern := SparqlPattern{Limit: 0, Offset: 0}
	resp, err := SparqlQueryScoped(pattern, nil, kg)
	assert.NoError(t, err)
	// Path A returns all 22. Updated from 16 -> 22 per D-SPARQL-04b.
	assert.Equal(t, 22, resp.Count)
	assert.False(t, resp.Truncated)
	assert.Len(t, resp.Results, 22)
}

// ---------------------------------------------------------------------------
// Hybrid Retrieval tests
// ---------------------------------------------------------------------------

// mockEmbedder returns a fixed vector for testing.
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a 1024-dim zero vector (matching BGE-M3 dimension).
	return make([]float32, 1024), nil
}

func (m *mockEmbedder) Dimensions() int { return 1024 }

// mockRetriever returns canned hits for testing.
type mockRetriever struct {
	hits []retriever.Hit
	err  error
}

func (m *mockRetriever) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hits, nil
}
func (m *mockRetriever) Health(ctx context.Context) error                            { return nil }
func (m *mockRetriever) Ensure(ctx context.Context) error                            { return nil }
func (m *mockRetriever) Insert(ctx context.Context, docs []retriever.Document) error { return nil }
func (m *mockRetriever) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (m *mockRetriever) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, retriever.ErrChunkNotFound
}
func (m *mockRetriever) GetMaxID(ctx context.Context, tier string) (int64, error) { return 0, nil }
func (m *mockRetriever) Disconnect() error                                        { return nil }

// TestHybridSearchScoped_OneHop verifies KG expansion with MaxHops=1 returns
// direct neighbors only (via referencesTable edges).
func TestHybridSearchScoped_OneHop(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "note_a", P: testNS + "referencesTable", O: testNS + "tbl_x"})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: belongsToTier, O: testNS + "sql-guides"})

	mr := &mockRetriever{hits: []retriever.Hit{
		{Score: 0.9, Content: "test content", Source: "authority/test.md"},
	}}
	emb := &mockEmbedder{}

	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	// note_a + tbl_x = 2 KG nodes
	assert.Len(t, resp.KGNodes, 2)
	// 1 edge: note_a → tbl_x
	assert.Len(t, resp.KGEdges, 1)
	// Milvus called
	assert.Len(t, resp.MilvusHits, 1)
}

// TestHybridSearchScoped_TwoHops verifies MaxHops=2 returns 2-hop neighbors.
func TestHybridSearchScoped_TwoHops(t *testing.T) {
	kg := store.New()
	// note_a → tbl_x (1-hop)
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "note_a", P: testNS + "referencesTable", O: testNS + "tbl_x"})
	// tbl_x → note_b (2-hop)
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: testNS + "referencesTable", O: testNS + "note_b"})
	kg.Insert(store.Triple{S: testNS + "note_b", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_b", P: belongsToTier, O: testNS + "sql-guides"})

	mr := &mockRetriever{hits: []retriever.Hit{}}
	emb := &mockEmbedder{}

	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 2, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	// note_a + tbl_x + note_b = 3 nodes
	assert.Len(t, resp.KGNodes, 3)
}

// TestHybridSearchScoped_TierFilter verifies tier filtering: only entities
// in allowedTiers appear in KG expansion.
func TestHybridSearchScoped_TierFilter(t *testing.T) {
	kg := store.New()
	// note_sql in sql-guides tier
	kg.Insert(store.Triple{S: testNS + "note_sql", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_sql", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "note_sql", P: testNS + "referencesTable", O: testNS + "tbl_sql"})
	kg.Insert(store.Triple{S: testNS + "tbl_sql", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_sql", P: belongsToTier, O: testNS + "sql-guides"})
	// note_auth in authority tier (should be filtered when sql-guides only)
	kg.Insert(store.Triple{S: testNS + "note_auth", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_auth", P: belongsToTier, O: testNS + "authority"})
	kg.Insert(store.Triple{S: testNS + "note_auth", P: testNS + "referencesTable", O: testNS + "tbl_auth"})
	kg.Insert(store.Triple{S: testNS + "tbl_auth", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_auth", P: belongsToTier, O: testNS + "authority"})

	mr := &mockRetriever{hits: []retriever.Hit{}}
	emb := &mockEmbedder{}

	// Restrict to sql-guides tier only
	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	// Only sql-guides entities visible
	for _, node := range resp.KGNodes {
		assert.NotEqual(t, testNS+"note_auth", node.ID)
		assert.NotEqual(t, testNS+"tbl_auth", node.ID)
	}
}

// TestHybridSearchScoped_Rerank verifies that with Rerank=true and a reranker
// function, results are re-ordered.
func TestHybridSearchScoped_Rerank(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})

	hits := []retriever.Hit{
		{Score: 0.9, Content: "content A", Source: "a.md"},
		{Score: 0.8, Content: "content B", Source: "b.md"},
		{Score: 0.7, Content: "content C", Source: "c.md"},
	}
	mr := &mockRetriever{hits: hits}
	emb := &mockEmbedder{}

	reranker := func(ctx context.Context, query string, hits []HybridMilvusHit) ([]HybridMilvusHit, error) {
		// Reverse order for test verification
		out := make([]HybridMilvusHit, len(hits))
		for i, h := range hits {
			out[len(hits)-1-i] = h
		}
		return out, nil
	}

	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: true}, []string{"sql-guides"}, kg, mr, emb, reranker)
	assert.NoError(t, err)
	// After reversing, first hit should be "content C"
	assert.Equal(t, "content C", resp.MilvusHits[0].Text)
}

// TestHybridSearchScoped_NoRerank verifies that with Rerank=false, Milvus
// top-k results are returned directly without reranking.
func TestHybridSearchScoped_NoRerank(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})

	hits := []retriever.Hit{
		{Score: 0.9, Content: "top content", Source: "top.md"},
		{Score: 0.8, Content: "second content", Source: "second.md"},
	}
	mr := &mockRetriever{hits: hits}
	emb := &mockEmbedder{}

	// nil reranker + false rerank flag
	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	assert.Len(t, resp.MilvusHits, 2)
	// Order preserved from Milvus
	assert.Equal(t, "top content", resp.MilvusHits[0].Text)
}

// TestHybridSearchScoped_EmptyKGExpansion verifies that when KG returns no
// matching entities, Milvus is still queried (KG subgraph empty but Milvus works).
func TestHybridSearchScoped_EmptyKGExpansion(t *testing.T) {
	kg := store.New()
	// No ABox triples — KG expansion yields empty node/edge set
	mr := &mockRetriever{hits: []retriever.Hit{
		{Score: 0.9, Content: "milvus hit", Source: "test.md"},
	}}
	emb := &mockEmbedder{}

	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	// KG subgraph empty
	assert.Len(t, resp.KGNodes, 0)
	assert.Len(t, resp.KGEdges, 0)
	// Milvus still returned its hits
	assert.Len(t, resp.MilvusHits, 1)
	assert.Equal(t, "milvus hit", resp.MilvusHits[0].Text)
}

// TestHybridSearchScoped_DefaultHops verifies MaxHops defaults to 1 when 0.
func TestHybridSearchScoped_DefaultHops(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "note_a", P: testNS + "referencesTable", O: testNS + "tbl_x"})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: belongsToTier, O: testNS + "sql-guides"})

	mr := &mockRetriever{hits: []retriever.Hit{}}
	emb := &mockEmbedder{}

	// MaxHops = 0 should default to 1
	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 0, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	assert.Len(t, resp.KGNodes, 2) // note_a + tbl_x (1-hop expansion)
}

// TestHybridSearchScoped_RerankError verifies reranker errors are non-fatal
// (falls back to Milvus results).
func TestHybridSearchScoped_RerankError(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})

	hits := []retriever.Hit{{Score: 0.9, Content: "original", Source: "orig.md"}}
	mr := &mockRetriever{hits: hits}
	emb := &mockEmbedder{}

	// Reranker that always returns error
	badReranker := func(ctx context.Context, query string, hits []HybridMilvusHit) ([]HybridMilvusHit, error) {
		return nil, assert.AnError
	}

	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 1, Rerank: true}, []string{"sql-guides"}, kg, mr, emb, badReranker)
	assert.NoError(t, err) // error is logged, not returned
	// Falls back to Milvus results
	assert.Equal(t, "original", resp.MilvusHits[0].Text)
}

// TestHybridSearchScoped_MaxHopsCap verifies MaxHops > 2 is capped to 2.
func TestHybridSearchScoped_MaxHopsCap(t *testing.T) {
	kg := store.New()
	kg.Insert(store.Triple{S: testNS + "note_a", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_a", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "note_a", P: testNS + "referencesTable", O: testNS + "tbl_x"})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: rdfType, O: skfTable})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: belongsToTier, O: testNS + "sql-guides"})
	kg.Insert(store.Triple{S: testNS + "tbl_x", P: testNS + "referencesTable", O: testNS + "note_b"})
	kg.Insert(store.Triple{S: testNS + "note_b", P: rdfType, O: skfNote})
	kg.Insert(store.Triple{S: testNS + "note_b", P: belongsToTier, O: testNS + "sql-guides"})

	mr := &mockRetriever{hits: []retriever.Hit{}}
	emb := &mockEmbedder{}

	// MaxHops = 99 should be capped to 2
	resp, err := HybridSearchScoped(HybridQuery{Query: "test", MaxHops: 99, Rerank: false}, []string{"sql-guides"}, kg, mr, emb, nil)
	assert.NoError(t, err)
	// With 2-hop cap: note_a + tbl_x + note_b = 3
	assert.Len(t, resp.KGNodes, 3)
}
