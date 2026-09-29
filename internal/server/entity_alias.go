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
// The 2026-09-29 audit of the live KG's 55 organization labels found 16
// duplicate pairs, every pair shaped <地市名> vs <地市名>+(中心支公司|支公司|
// 分部): different notes spell the same institution differently, and
// D-NZK-01's exact-name identity (deliberately alias-free so 车险 and
// 汽车险 stay distinct products) faithfully mints two individuals. This
// file adds ONE narrow, closed-grammar exception on top of that contract.
//
// The three design commitments (mirrored by the extraction-side copy in
// internal/knowledge/ecl/stub.go — keep the suffix list in sync):
//
//   - WHY ORG-ONLY: org designations form a closed administrative grammar
//     (中心支公司/支公司/分公司/分部/部门组/科室), and a bare geographic name is
//     a conventional abbreviation of the designation-bearing form. Product
//     names have no such grammar — 责任险 vs 雇主责任险 differ by a MODIFIER,
//     not a designation — so merging them would destroy distinct insurance
//     products. The rule therefore never touches insurance_product, table,
//     or note nodes.
//
//   - WHY CANON = THE LONG NAME: the suffixed form is the specific legal
//     institution; the short form is its everyday abbreviation. Canonical
//     direction short→long preserves the most precise label and matches the
//     KG's hierarchy wiring (the long form is the one carrying
//     parentOrganization edges in the live data).
//
//   - WHY DETERMINISTIC RULES, NOT FUZZY MATCHING: this is a trust layer.
//     Edit distance / embedding similarity would speculatively fuse
//     institutions the corpus spells differently on purpose (武汉江南 vs
//     江北中心支公司 share almost every character). A closed suffix grammar
//     is auditable, explainable, and idempotent: a label that already ends
//     in a designation is terminal, so canonical(canonical(x)) == x and the
//     merge can never chain or cycle.

import (
	"regexp"
	"sort"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// orgSuffixes lists the org-designation suffixes observed in the corpus
// audit above. Order is irrelevant to the result (canonicalOrgLabel scans
// every candidate and keeps the longest hit); the slice form just makes
// candidate enumeration deterministic.
var orgSuffixes = []string{"中心支公司", "支公司", "分公司", "分部", "部门组", "科室"}

// orgSuffixRe matches a label that ALREADY ends with an org designation —
// such labels are terminal (already the specific institution name).
var orgSuffixRe = regexp.MustCompile(`(中心支公司|支公司|分公司|分部|部门组|科室)$`)

// canonicalOrgLabel resolves a short org label to the longer label in the
// label set that equals label + a single org suffix (e.g. 武汉 →
// 武汉中心支公司). Deterministic and explainable by design:
//
//   - label ends with a designation suffix → returned as-is (terminal)
//   - otherwise every label+suffix candidate present in `all` is a hit;
//     the longest hit wins (武汉 with both 武汉支公司 and 武汉中心支公司 in the
//     set resolves to the 中心支公司 form — the most specific institution)
//   - no hit → returned as-is
//
// The caller gates by node kind: only organization nodes are passed in.
// Suffix byte lengths are directly comparable here because all candidates
// share the label prefix and every suffix is 3-byte CJK runes, so longest
// candidate == most suffix runes.
func canonicalOrgLabel(label string, all map[string]bool) string {
	if label == "" || orgSuffixRe.MatchString(label) {
		return label
	}
	best := ""
	for _, suffix := range orgSuffixes {
		if cand := label + suffix; all[cand] && len(cand) > len(best) {
			best = cand
		}
	}
	if best == "" {
		return label
	}
	return best
}

// mergeOrgAliases rewrites the accumulated graph projection in place:
// organization nodes whose labels canonicalize to the same string collapse
// into ONE representative node, edges are re-pointed at the representative,
// self-edges introduced by the rewrite are dropped, and duplicate edges
// dedupe (first occurrence wins). insurance_product / table / note nodes
// are untouched.
//
// Pass 1 builds the alias label set corpus-wide — every Organization
// individual in the STORE (not merely the projected subset) plus the
// projected org nodes — so canonical resolution is deterministic regardless
// of the notes= filter scope: the same entity renders under the same
// canonical name in the 全库 view and in every filtered view. The
// label→IRI index lets the representative be the individual whose label
// IS the canonical name even when that individual sits outside the
// requested projection (a filtered view then materializes the
// representative node so re-pointed edges keep a visible target).
//
// Representative election per the quick task: the individual whose label
// equals the canonical (long) name; failing that, the lexicographically
// smallest member IRI.
func mergeOrgAliases(kg *store.Store, acc *graphAccumulator) {
	if len(acc.nodes) == 0 {
		return
	}

	// Pass 1 — corpus-wide label set + label→IRI index.
	labelSet := map[string]bool{}
	labelToIRI := map[string]string{}
	addLabel := func(label, iri string) {
		if label == "" {
			return
		}
		labelSet[label] = true
		if _, ok := labelToIRI[label]; !ok {
			labelToIRI[label] = iri
		}
	}
	for _, t := range kg.Query("", ontRdfType, skf("Organization")) {
		addLabel(literalOf(kg, t.S, ontRdfsLabel), t.S)
	}
	for _, n := range acc.nodes {
		if n.Kind == "organization" {
			addLabel(n.Label, n.ID)
		}
	}
	if len(labelSet) == 0 {
		return
	}

	// Pass 2 — group projected org nodes by canonical label; elect reps.
	rekey := map[string]string{} // merged-away node IRI → representative IRI
	for _, n := range acc.nodes {
		if n.Kind != "organization" {
			continue
		}
		canon := canonicalOrgLabel(n.Label, labelSet)
		if canon == n.Label {
			continue // this node IS a canonical form — it can only receive
		}
		rep := labelToIRI[canon]
		if rep == "" || rep == n.ID {
			continue
		}
		rekey[n.ID] = rep
	}
	if len(rekey) == 0 {
		return
	}

	// Nodes: keep original order, drop merged-away nodes.
	kept := map[string]bool{}
	nodes := make([]ontologyGraphNode, 0, len(acc.nodes))
	for _, n := range acc.nodes {
		if _, gone := rekey[n.ID]; gone {
			continue
		}
		kept[n.ID] = true
		nodes = append(nodes, n)
	}

	// Edges: re-point, drop self-edges. Edge MULTIPLICITY is preserved —
	// two distinct mentions (note→武汉, note→武汉中心支公司) that converge on
	// the representative stay two edges (the task pins "2 条边同目标");
	// exact duplicates were already deduped at accumulation time. The FE
	// tolerates the now-identical edge pair: its cytoscape element id is
	// from|to|type, so the render shows one edge for them.
	edges := make([]ontologyGraphEdge, 0, len(acc.edges))
	for _, e := range acc.edges {
		if rep, ok := rekey[e.From]; ok {
			e.From = rep
		}
		if rep, ok := rekey[e.To]; ok {
			e.To = rep
		}
		if e.From == e.To {
			continue
		}
		edges = append(edges, e)
	}

	// A representative may sit outside the original projection scope
	// (notes=-filtered view where only the short alias is involved):
	// materialize its node so the re-pointed edges keep a visible target.
	// Sorted for deterministic node order.
	var missing []string
	for _, rep := range rekey {
		if !kept[rep] {
			missing = append(missing, rep)
		}
	}
	sort.Strings(missing)
	for _, rep := range missing {
		nodes = append(nodes, ontologyGraphNode{
			ID:    rep,
			Kind:  "organization",
			Label: entityLabel(kg, rep),
			Tier:  ontologyTierOf(kg, rep),
		})
	}

	acc.nodes, acc.edges = nodes, edges
}
