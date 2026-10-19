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

// Plan 40-06 — Governance admin API (SHACL report, provenance graph, quarantine status).
//
// Wire contract (all routes X-Admin-Token-gated; 403 before 503):
//
//	GET /api/admin/governance/shacl
//	     → 200 {health, error_count, warning_count, quarantined:[...], ...}
//	       (cached report; written by cmd/server/main.go at boot or by
//	        induction.Manager.RollbackTBox after a rollback)
//	       503 "report not available" if validation has not run yet
//	GET /api/admin/governance/provenance
//	     → 200 {nodes:[...], edges:[...]}  (full graph including quarantined)
//	       503 if ontology not wired
package server

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/ontology/shacl"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// adminGovernanceShaclHandler implements GET /api/admin/governance/shacl.
//
// Returns the cached SHACL validation report. Exactly two call sites write
// that cache: the boot path in cmd/server/main.go (D-SHACL-02: validation
// runs every boot) and RollbackTBox in internal/ontology/induction/induction.go,
// which republishes the report its post-rollback revalidation produced
// (D-02a/REL-02).
//
// There is no live re-validation on GET — the report reflects the state at the
// time validation last ran. If a post-rollback validation FAILS, the previous
// (boot-triggered) report deliberately stays cached rather than being cleared
// (D-02c: rolling back is not failed by a SHACL error, and clearing would need
// a store Clear API). `trigger` and `validated_at` must therefore be read as
// properties of the last SUCCESSFUL validation, not of the last rollback.
func (s *Deps) adminGovernanceShaclHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}

	report := dep.KG.GetShaclReport()
	if report == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "report not available"})
		return
	}
	// Type-assert the interface{} back to *shacl.Report.
	r, ok := report.(*shacl.Report)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "report not available"})
		return
	}
	c.JSON(http.StatusOK, r)
}

// ProvenanceGraph is the response envelope for GET /api/admin/governance/provenance.
type ProvenanceGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents one node in the provenance graph.
type GraphNode struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"` // "file"|"note"|"table"|"skill"|"skillmd"|"quarantined"
	Label      string                 `json:"label"`
	Provenance *store.Provenance      `json:"provenance,omitempty"`
	Tier       string                 `json:"tier,omitempty"`
}

// GraphEdge represents one directed edge in the provenance graph.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// adminGovernanceProvenanceHandler implements GET /api/admin/governance/provenance.
//
// Builds a provenance graph from the KG and quarantine state:
//   - file nodes: provenance.source_path values that look like file paths
//   - note nodes: skf:note_N entities
//   - table nodes: skf:tbl_X entities
//   - skill nodes: skf:Skill individuals
//   - skillmd nodes: SKILL.md provenance entries
//   - quarantined nodes: entities in the quarantine map
//
// Edges connect: source_file → note → table (via referencesTable)
//               source_file → skillmd → skill (via skf:Skill declarations)
func (s *Deps) adminGovernanceProvenanceHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}

	nodes, edges := buildProvenanceGraph(dep.KG)
	c.JSON(http.StatusOK, ProvenanceGraph{Nodes: nodes, Edges: edges})
}

// buildProvenanceGraph constructs nodes and edges from the KG and quarantine state.
func buildProvenanceGraph(kg *store.Store) ([]GraphNode, []GraphEdge) {
	var nodes []GraphNode
	var edges []GraphEdge
	nodeIDs := make(map[string]bool) // deduplicate

	// addNode / addEdge are defined at function scope so the qEntities
	// loop below can reuse them after the triples loop.
	addNode := func(id, kind, label string, prov *store.Provenance, tier string) {
		if id == "" || nodeIDs[id] {
			return
		}
		nodeIDs[id] = true
		nodes = append(nodes, GraphNode{
			ID:         id,
			Kind:       kind,
			Label:      label,
			Provenance: prov,
			Tier:       tier,
		})
	}
	addEdge := func(src, tgt string) {
		if src == "" || tgt == "" || src == tgt {
			return
		}
		edges = append(edges, GraphEdge{Source: src, Target: tgt})
	}

	// Walk all main KG triples.
	triples := kg.Query("", "", "")
	for _, t := range triples {

		// Determine node kind and add nodes based on provenance and entity type.
		prov := t.Provenance

		// Note node: skf:note_N
		if len(t.S) > len(skfNS+"note_") && t.S[len(skfNS):len(skfNS)+5] == "note_" {
			addNode(t.S, "note", stripNS(t.S), prov, "")
			if prov != nil && prov.SourcePath != "" {
				addNode("file:"+prov.SourcePath, "file", prov.SourcePath, nil, "")
				addEdge("file:"+prov.SourcePath, t.S)
			}
			// Also add tier info for notes.
			if tierTriple := kg.Query(t.S, skf("belongsToTier"), ""); len(tierTriple) > 0 {
				tier := stripNS(tierTriple[0].O)
				nodes[len(nodes)-1].Tier = tier
			}
			continue
		}

		// Table node: skf:tbl_X
		if len(t.S) > len(skfNS+"tbl_") && t.S[len(skfNS):len(skfNS)+4] == "tbl_" {
			addNode(t.S, "table", stripNS(t.S), prov, "")
			// tables reference their source note.
			if t.P == skf("referencesTable") {
				addEdge(t.S, t.O) // table → note
			}
			continue
		}

		// Skill individual: skf:Skill (not a note or table).
		if t.P == skf("type") && t.O == skf("Skill") {
			addNode(t.S, "skill", stripNS(t.S), prov, "")
			continue
		}

		// Tier node: skf:Tier
		if t.P == skf("type") && t.O == skf("Tier") {
			addNode(t.S, "tier", stripNS(t.S), nil, "")
			continue
		}

		// KBCategory node.
		if t.P == skf("type") && t.O == skf("KBCategory") {
			addNode(t.S, "category", stripNS(t.S), nil, "")
			continue
		}
	}

	// Add quarantined entities.
	qEntities := kg.QuarantinedEntities()
	for _, qe := range qEntities {
		addNode(qe.Entity, "quarantined", stripNS(qe.Entity), nil, "")
		// Add edges from quarantined entity to its notes/tables.
		triples := kg.Query(qe.Entity, "", "")
		for _, t := range triples {
			if len(t.S) > len(skfNS)+5 && t.S[len(skfNS):len(skfNS)+5] == "note_" {
				addEdge(qe.Entity, t.S)
			}
			if len(t.S) > len(skfNS)+4 && t.S[len(skfNS):len(skfNS)+4] == "tbl_" {
				addEdge(qe.Entity, t.S)
			}
		}
	}

	return nodes, edges
}

const skfNS = "http://skillforge.local/ontology#"

// skf returns the full IRI for a short local name.
func skf(local string) string {
	return skfNS + local
}

// stripNS removes the skillforge namespace prefix from a full IRI
// and returns the short local name.
func stripNS(iri string) string {
	if len(iri) > len(skfNS) && iri[:len(skfNS)] == skfNS {
		return iri[len(skfNS):]
	}
	return iri
}
