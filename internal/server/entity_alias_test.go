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

// Quick 260929-h8k — deterministic organization-alias resolution.
//
// The live KG splits one institution into two entity nodes whenever
// different notes spell it differently: 武汉 vs 武汉中心支公司. D-NZK-01's
// exact-name identity is deliberately alias-free (车险 ≠ 汽车险), so this
// layer adds exactly ONE closed-grammar exception — org designation
// suffixes — instead of any fuzzy matching.
//
// Pins: the canonicalOrgLabel suffix grammar (longest hit wins,
// suffix-ending labels are terminal, product-shaped names never merge)
// and the graph-projection merge (alias org nodes collapse into the
// long-name representative; both involves_org edges survive re-pointed;
// the short form's parent hop re-attaches to the representative).

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/skillforge/skillforge/internal/ontology/induction"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// TestCanonicalOrgLabel_Table pins the deterministic suffix grammar over a
// corpus-shaped label set.
func TestCanonicalOrgLabel_Table(t *testing.T) {
	all := map[string]bool{
		"武汉中心支公司": true,
		"武汉支公司":   true, // shorter sibling — longest-hit rule must win
		"个信保分部":   true,
		"湖北分公司":   true,
		"雇主责任险":   true, // a PRODUCT name that merely extends another product name
	}
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"longest hit wins", "武汉", "武汉中心支公司"},
		{"suffix-ending label is terminal", "武汉中心支公司", "武汉中心支公司"},
		{"suffix-ending label needs no bare partner", "公安支公司", "公安支公司"},
		{"fenbu suffix path", "个信保", "个信保分部"},
		{"fengongsi suffix path", "湖北", "湖北分公司"},
		{"products never participate", "责任险", "责任险"},
		{"bare 公司 is not a suffix", "总公司", "总公司"},
		{"no hit stays as-is", "不存在", "不存在"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, canonicalOrgLabel(tc.label, all))
		})
	}
}

// orgAliasFixtureKG mirrors the live split: note_040 involves BOTH spellings
// of the same institution (short alias 武汉 + canonical 武汉中心支公司), and the
// SHORT form carries the parentOrganization hop (the exact shape the live
// audit found — so the merge must re-attach parent_org to the
// representative, not drop it).
func orgAliasFixtureKG() *store.Store {
	s := store.New()
	var (
		invOrg  = skf("involvesOrganization")
		parent  = skf("parentOrganization")
		note040 = skf("note_040")
		entWH   = skf("ent_e000000whshort1")  // 武汉 — short alias
		entWHZ  = skf("ent_e000000whzb00001") // 武汉中心支公司 — canonical long name
		entHB   = skf("ent_e000000hubei0002") // 湖北分公司 — the parent
	)
	s.Insert(store.Triple{S: note040, P: ontRdfType, O: skf("NoteChunk")})
	s.Insert(store.Triple{S: note040, P: invOrg, O: entWH})
	s.Insert(store.Triple{S: note040, P: invOrg, O: entWHZ})
	s.Insert(store.Triple{S: entWH, P: ontRdfType, O: skf("Organization")})
	s.Insert(store.Triple{S: entWH, P: ontRdfsLabel, O: `"武汉"`})
	s.Insert(store.Triple{S: entWH, P: parent, O: entHB})
	s.Insert(store.Triple{S: entWHZ, P: ontRdfType, O: skf("Organization")})
	s.Insert(store.Triple{S: entWHZ, P: ontRdfsLabel, O: `"武汉中心支公司"`})
	s.Insert(store.Triple{S: entHB, P: ontRdfType, O: skf("Organization")})
	s.Insert(store.Triple{S: entHB, P: ontRdfsLabel, O: `"湖北分公司"`})
	return s
}

// TestOntologyGraph_OrgAliasMerged pins the projection merge: the two alias
// org nodes collapse into ONE node (the label==canonical individual) and
// BOTH involves_org edges survive re-pointed at it, with the short form's
// parent hop re-attached to the representative.
func TestOntologyGraph_OrgAliasMerged(t *testing.T) {
	kg := orgAliasFixtureKG()
	mgr, err := induction.NewManager(skfNS, t.TempDir(), stubLLM{`{"kind":"Class","description_zh":"d","description_en":"d"}`}, kg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	_, body := invokeGraph(t, &Deps{Ontology: &OntologyDeps{Induction: mgr, KG: kg}}, "")

	var orgNodes []ontologyGraphNode
	for _, n := range body.Nodes {
		if n.Kind == "organization" {
			orgNodes = append(orgNodes, n)
		}
	}
	assert.Len(t, orgNodes, 2,
		"武汉 + 武汉中心支公司 must collapse into ONE node; 湖北分公司 stays as the parent")
	if assert.Len(t, orgNodes, 2) {
		ids := []string{orgNodes[0].ID, orgNodes[1].ID}
		assert.Contains(t, ids, skf("ent_e000000whzb00001"),
			"representative = the label==canonical individual")
		assert.NotContains(t, ids, skf("ent_e000000whshort1"),
			"the short alias node must be gone")
		assert.Contains(t, ids, skf("ent_e000000hubei0002"),
			"the parent org must survive the merge")
		for _, n := range orgNodes {
			if n.ID == skf("ent_e000000whzb00001") {
				assert.Equal(t, "武汉中心支公司", n.Label)
			}
		}
	}

	// Both mentions survive as involves_org edges aimed at the representative.
	invokes := 0
	for _, e := range body.Edges {
		if e.Type != "involves_org" {
			continue
		}
		invokes++
		assert.Equal(t, skf("ent_e000000whzb00001"), e.To,
			"every involves_org edge must target the representative")
	}
	assert.Equal(t, 2, invokes, "both involves_org edges survive, re-pointed")

	// The short form's parent hop re-attaches to the representative (FROM
	// side rewritten), exactly once.
	parents := 0
	for _, e := range body.Edges {
		if e.Type != "parent_org" {
			continue
		}
		parents++
		assert.Equal(t, skf("ent_e000000whzb00001"), e.From,
			"parent_org must originate from the representative after the merge")
		assert.Equal(t, skf("ent_e000000hubei0002"), e.To)
	}
	assert.Equal(t, 1, parents, "the re-pointed parent hop survives, deduped")

	// Notes and products are untouched by the org rule (shape guard).
	assert.Len(t, body.Nodes, 3, "1 note + 2 org nodes (nothing else projected)")
}
