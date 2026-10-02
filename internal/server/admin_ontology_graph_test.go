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

// Quick 260929-g7x — GET /api/admin/ontology/graph projection pins.
//
// The endpoint projects the ABox (quick-260928-nzk wiring) into the
// note-centric entity graph the FE 本体视图 renders:
//
//	note_XXX --referencesTable-->        tbl_Y
//	note_XXX --involvesInsuranceProduct→ ent_Z
//	note_XXX --involvesOrganization→     ent_O
//	ent_O    --parentOrganization→       ent_P   (1-hop, parent not
//	                                              required to be note-linked)
//
// Pins: projection shape, entity rdfs:label resolution, note label
// fallback to source_idx, table node dedupe across notes, parent-hop
// completeness, tier as a node FIELD (never a tier NODE), unknown-note
// → 200 + empty arrays, and the X-Admin-Token gate.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/induction"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// graphFixtureKG mirrors the live ABox wiring for two notes:
//   - note_032 → tbl_pay_way, product 车险, org 湖北分公司 (→ parent 总公司)
//   - note_033 → tbl_pay_way (SHARED table — dedupe pin), org 总公司
//
// Entity IRIs use the e-hash shape (CJK names hash-encode per EntityIRI);
// rdfs:label carries the readable business name.
func graphFixtureKG() *store.Store {
	s := store.New()
	var (
		refsTbl  = skf("referencesTable")
		invProd  = skf("involvesInsuranceProduct")
		invOrg   = skf("involvesOrganization")
		parent   = skf("parentOrganization")
		tierP    = skf("belongsToTier")
		label    = ontRdfsLabel
		note032  = skf("note_032")
		note033  = skf("note_033")
		tblPay   = skf("tbl_pay_way")
		tblClaim = skf("tbl_claim")
		entProd  = skf("ent_e000000car001")
		entHubei = skf("ent_e000000hubei1")
		entHQ    = skf("ent_e000000hq0001")
	)

	// note_032 — no rdfs:label → label falls back to source_idx "032".
	s.Insert(store.Triple{S: note032, P: ontRdfType, O: skf("NoteChunk")})
	s.Insert(store.Triple{S: note032, P: tierP, O: skf("sql-guides")})
	s.Insert(store.Triple{S: note032, P: refsTbl, O: tblPay})
	s.Insert(store.Triple{S: note032, P: invProd, O: entProd})
	s.Insert(store.Triple{S: note032, P: invOrg, O: entHubei})

	// note_033 — labeled note.
	s.Insert(store.Triple{S: note033, P: ontRdfType, O: skf("NoteChunk")})
	s.Insert(store.Triple{S: note033, P: tierP, O: skf("sql-guides")})
	s.Insert(store.Triple{S: note033, P: label, O: `"保单支付方式笔记"`})
	s.Insert(store.Triple{S: note033, P: refsTbl, O: tblPay})
	s.Insert(store.Triple{S: note033, P: refsTbl, O: tblClaim})
	s.Insert(store.Triple{S: note033, P: invOrg, O: entHQ})

	// Shared table node (dedupe target) + tier attachment.
	s.Insert(store.Triple{S: tblPay, P: ontRdfType, O: skf("TableSchema")})
	s.Insert(store.Triple{S: tblPay, P: tierP, O: skf("sql-guides")})
	s.Insert(store.Triple{S: tblClaim, P: ontRdfType, O: skf("TableSchema")})

	// Entities: label + tier; 湖北分公司 → 总公司 parent hop.
	s.Insert(store.Triple{S: entProd, P: ontRdfType, O: skf("InsuranceProduct")})
	s.Insert(store.Triple{S: entProd, P: label, O: `"车险"`})
	s.Insert(store.Triple{S: entProd, P: tierP, O: skf("sql-guides")})
	s.Insert(store.Triple{S: entHubei, P: ontRdfType, O: skf("Organization")})
	s.Insert(store.Triple{S: entHubei, P: label, O: `"湖北分公司"`})
	s.Insert(store.Triple{S: entHubei, P: tierP, O: skf("sql-guides")})
	s.Insert(store.Triple{S: entHubei, P: parent, O: entHQ})
	s.Insert(store.Triple{S: entHQ, P: ontRdfType, O: skf("Organization")})
	s.Insert(store.Triple{S: entHQ, P: label, O: `"总公司"`})
	s.Insert(store.Triple{S: entHQ, P: tierP, O: skf("sql-guides")})
	return s
}

func graphTestDeps(t *testing.T) *Deps {
	t.Helper()
	kg := graphFixtureKG()
	mgr, err := induction.NewManager(skfNS, t.TempDir(), stubLLM{`{"kind":"Class","description_zh":"d","description_en":"d"}`}, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return &Deps{Ontology: &OntologyDeps{Induction: mgr, KG: kg}}
}

// invokeGraph is the token-carrying GET helper for the graph endpoint.
func invokeGraph(t *testing.T, deps *Deps, rawQuery string) (int, ontologyGraphResponse) {
	t.Helper()
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	path := "/api/admin/ontology/graph"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	resp := invokeOntology(t, deps, http.MethodGet, path, "", "tok")
	if resp.Code != http.StatusOK {
		return resp.Code, ontologyGraphResponse{}
	}
	var body ontologyGraphResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal graph body: %v", err)
	}
	return resp.Code, body
}

func nodeByID(body ontologyGraphResponse, id string) (ontologyGraphNode, bool) {
	for _, n := range body.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return ontologyGraphNode{}, false
}

func hasEdge(body ontologyGraphResponse, from, to, typ string) bool {
	for _, e := range body.Edges {
		if e.From == from && e.To == to && e.Type == typ {
			return true
		}
	}
	return false
}

// TestOntologyGraph_ProjectionShape pins the full projection: note nodes
// with label / fallback, entity labels, reference + involves edges, the
// parent hop, and tier carried as a FIELD.
func TestOntologyGraph_ProjectionShape(t *testing.T) {
	code, body := invokeGraph(t, graphTestDeps(t), "")
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, body.Nodes, 7, "2 notes + 2 tables + 3 entities (HQ shared)")
	assert.Len(t, body.Edges, 7,
		"3 references_table + 1 involves_product + 2 involves_org + 1 parent_org")

	// Note node: rdfs:label wins; missing label falls back to source_idx.
	n32, ok := nodeByID(body, skf("note_032"))
	assert.True(t, ok)
	assert.Equal(t, "note", n32.Kind)
	assert.Equal(t, "032", n32.Label, "no rdfs:label → bare source_idx")
	assert.Equal(t, "sql-guides", n32.Tier)

	n33, ok := nodeByID(body, skf("note_033"))
	assert.True(t, ok)
	assert.Equal(t, "保单支付方式笔记", n33.Label, "rdfs:label (unquoted) wins")

	// Table node: label = local name with tbl_ stripped, tier attached.
	tbl, ok := nodeByID(body, skf("tbl_pay_way"))
	assert.True(t, ok)
	assert.Equal(t, "table", tbl.Kind)
	assert.Equal(t, "pay_way", tbl.Label)
	assert.Equal(t, "sql-guides", tbl.Tier)

	// Entity node: rdfs:label = business name; kind by predicate.
	prod, ok := nodeByID(body, skf("ent_e000000car001"))
	assert.True(t, ok)
	assert.Equal(t, "insurance_product", prod.Kind)
	assert.Equal(t, "车险", prod.Label)

	hq, ok := nodeByID(body, skf("ent_e000000hq0001"))
	assert.True(t, ok)
	assert.Equal(t, "organization", hq.Kind)
	assert.Equal(t, "总公司", hq.Label)

	// parent hop present even though 总公司 is involved only by note_033 —
	// and note_032's 湖北分公司 chain reaches it.
	assert.True(t, hasEdge(body, skf("ent_e000000hubei1"), skf("ent_e000000hq0001"), "parent_org"))
	assert.True(t, hasEdge(body, skf("note_032"), skf("ent_e000000hubei1"), "involves_org"))
	assert.True(t, hasEdge(body, skf("note_032"), skf("ent_e000000car001"), "involves_product"))
	assert.True(t, hasEdge(body, skf("note_032"), skf("tbl_pay_way"), "references_table"))

	// tier is a FIELD, never a node (a Tier node would be a mega-hub).
	for _, n := range body.Nodes {
		assert.NotEqual(t, "tier", n.Kind, "belongsToTier must not become a node")
	}
}

// TestOntologyGraph_TableDedupedAcrossNotes pins node/edge dedupe: both
// notes reference tbl_pay_way → ONE table node, TWO reference edges.
func TestOntologyGraph_TableDedupedAcrossNotes(t *testing.T) {
	_, body := invokeGraph(t, graphTestDeps(t), "")
	count := 0
	for _, n := range body.Nodes {
		if n.ID == skf("tbl_pay_way") {
			count++
		}
	}
	assert.Equal(t, 1, count, "shared table node must appear exactly once")
	assert.True(t, hasEdge(body, skf("note_032"), skf("tbl_pay_way"), "references_table"))
	assert.True(t, hasEdge(body, skf("note_033"), skf("tbl_pay_way"), "references_table"))
}

// TestOntologyGraph_NotesFilter pins the notes param: only the requested
// notes and their neighborhoods are projected — including the parent hop
// of their own organizations (1-hop completeness is per-note, not
// corpus-wide).
func TestOntologyGraph_NotesFilter(t *testing.T) {
	_, body := invokeGraph(t, graphTestDeps(t), "notes=note_032")
	assert.Len(t, body.Nodes, 5, "note_032 + tbl_pay_way + 车险 + 湖北分公司 + its parent 总公司")
	_, ok := nodeByID(body, skf("note_033"))
	assert.False(t, ok, "unrequested note must be absent")
	_, ok = nodeByID(body, skf("tbl_claim"))
	assert.False(t, ok, "note_033's exclusive table must be absent")
	// 总公司 IS present — via 湖北分公司's parent hop (note_032's own 1-hop).
	hq, ok := nodeByID(body, skf("ent_e000000hq0001"))
	assert.True(t, ok, "parent hop of a projected org must stay in the neighborhood")
	assert.Equal(t, "organization", hq.Kind)

	// Whitespace-tolerant multi-note list.
	_, body = invokeGraph(t, graphTestDeps(t), "notes=note_032, note_033")
	assert.Len(t, body.Nodes, 7, "both notes' projections merge with dedupe")
}

// TestOntologyGraph_UnknownNoteEmpty pins the honest empty contract: a
// note name missing from the KG yields 200 with empty (non-null) arrays.
func TestOntologyGraph_UnknownNoteEmpty(t *testing.T) {
	code, body := invokeGraph(t, graphTestDeps(t), "notes=note_999")
	assert.Equal(t, http.StatusOK, code)
	assert.NotNil(t, body.Nodes)
	assert.NotNil(t, body.Edges)
	assert.Empty(t, body.Nodes)
	assert.Empty(t, body.Edges)
}

// TestOntologyGraph_NilLayer503 mirrors the nil-posture contract.
func TestOntologyGraph_NilLayer503(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	deps := &Deps{}
	if code := invokeOntology(t, deps, http.MethodGet, "/api/admin/ontology/graph", "", "tok").Code; code != http.StatusServiceUnavailable {
		t.Errorf("nil ontology layer: expected 503, got %d", code)
	}
}
