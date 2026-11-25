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

// Package anchor turns the knowledge graph's rdfs:label set into a
// deterministic ENTITY DICTIONARY for chat queries (Phase 57 / ONT-02).
//
// Why a dictionary and not an embedder: GraphRAG's local-search lesson is
// that entity linking must be cheap, inspectable and tier-scoped -- a
// cosine pass over 162 labels tells an operator nothing when it misses,
// while a longest-match over the label set can say exactly which labels
// were considered. The matcher is therefore PURE string work:
//
//   - build the index ONCE per (approved tier set) from QueryScoped
//     label triples (tier filtering is enforced by the store, so a
//     session can never anchor on a tier its Skill may not read);
//   - longest-label-first match, so "差旅住宿费标准" wins over "差旅";
//   - stable output order (longest match first, then IRI) so the
//     downstream prompt and audit rows are reproducible.
//
// DEGRADATION IS EXPLICIT (ONT-02 SC 1): a nil/unreadable KG yields
// Available()==false, and the caller must fall back to pure vector
// retrieval with kg_available:false + a WARN. The matcher NEVER returns
// a fabricated empty "success" -- an empty KG and a matching KG with
// zero hits are different facts and this package keeps them distinct.
package anchor

import (
	"sort"
	"strings"

	"github.com/skillforge/skillforge/internal/ontology/store"
)

// LabelPredicate is the KG predicate carrying human-readable entity names.
//
// It is RDFS-LABEL, not a project-local predicate. The emitter writes
// `rdfs:label "..."` for every class, property and ABox individual
// (emitter.go), and the store's label queries read the same term; a
// pre-Phase-57 constant pointing at skf:label matched ZERO triples in the
// whole store, so BuildIndex silently built a dictionary out of whatever
// else happened to come back (Skill names, temperatures, agent types).
// Entity anchoring then reported entities_matched=0 on every real domain
// question -- the live symptom that surfaced this (2026-11-25).
const LabelPredicate = "http://www.w3.org/2000/01/rdf-schema#label"

// belongsToTierPredicate marks an ABox data entity (every Organization /
// InsuranceProduct / NoteChunk / TableSchema individual carries it). The
// anchor dictionary uses its presence to keep schema-layer labels out
// (see BuildIndex).
const belongsToTierPredicate = "http://skillforge.local/ontology#belongsToTier"

// Entity is one dictionary entry: an IRI with a display label.
type Entity struct {
	IRI   string
	Label string
}

// Match is one anchored entity plus the matched surface form. Surface
// and Label differ when the query mentioned a case/width variant of the
// label; keeping both makes the audit row honest about what matched.
type Match struct {
	IRI     string
	Label   string
	Surface string
}

// Index is the immutable entity dictionary for one tier scope.
type Index struct {
	available bool
	// entries sorted longest-label-first, then IRI, so Match is stable.
	entries []Entity
}

// BuildIndex reads the label triples visible to allowedTiers and builds
// the dictionary. A nil store is NOT an error -- it is the degraded case
// (Available()==false) the caller must surface honestly.
//
// Empty/literal-only labels are dropped: a bare IRI or a quoted-empty
// string cannot anchor a natural-language query, and keeping them would
// inflate the match surface with unusable entries.
func BuildIndex(kg *store.Store, allowedTiers []string) *Index {
	if kg == nil {
		return &Index{available: false}
	}
	triples := kg.QueryScopedExact("", LabelPredicate, "", allowedTiers)

	// SCHEMA-LABEL FILTER (2026-11-25).
	//
	// The KG labels BOTH ABox domain entities ("武汉中心支公司", "车险") and
	// the SCHEMA layer itself: classes ("TableSchema" -> "Table Schema"),
	// properties ("referencesTable" -> "note references table"), and every
	// SKILL.md individual (whose FULL prose description is its label). A
	// dictionary that mixes the two anchors prose: schema labels are
	// thousands of runes long, so they shadow real entities and match
	// inside ordinary sentences.
	//
	// The discriminator is belongsToTier: every ABox data entity carries it,
	// no schema-layer individual does. A subject with a label but no
	// belongsToTier triple is schema, not data, and must not enter the
	// dictionary.
	dataSubjects := make(map[string]bool)
	for _, t := range kg.QueryScopedExact("", belongsToTierPredicate, "", nil) {
		dataSubjects[t.S] = true
	}
	f := triples[:0]
	for _, t := range triples {
		if dataSubjects[t.S] {
			f = append(f, t)
		}
	}
	triples = f
	byIRI := make(map[string]string, len(triples))
	for _, t := range triples {
		label := unquote(t.O)
		label = strings.TrimSpace(label)
		if label == "" || label == t.S {
			continue
		}
		// First writer wins: the store is append-only, so the earliest
		// label an entity ever had is the one operators curated.
		if _, ok := byIRI[t.S]; !ok {
			byIRI[t.S] = label
		}
	}

	entries := make([]Entity, 0, len(byIRI))
	for iri, label := range byIRI {
		entries = append(entries, Entity{IRI: iri, Label: label})
	}
	// Longest label first (specific beats generic), IRI as the tiebreak so
	// the same KG always produces the same order.
	sort.Slice(entries, func(i, j int) bool {
		if len([]rune(entries[i].Label)) != len([]rune(entries[j].Label)) {
			return len([]rune(entries[i].Label)) > len([]rune(entries[j].Label))
		}
		return entries[i].IRI < entries[j].IRI
	})

	return &Index{available: true, entries: entries}
}

// Available reports whether the dictionary was built from a real KG.
// false means "no ontology in this deployment / the KG failed to load" --
// the caller must degrade to pure vector retrieval, not silently continue.
func (ix *Index) Available() bool { return ix != nil && ix.available }

// Len is the dictionary size (an operator-visible number; 0 with
// Available()==true means the KG carries no usable labels).
func (ix *Index) Len() int {
	if ix == nil {
		return 0
	}
	return len(ix.entries)
}

// Match anchors the query against the dictionary.
//
// Rules (all deterministic):
//  1. A label matches when its normalized form appears as a substring of
//     the normalized query. Substring (not whole-token) is deliberate
//     for CJK, where "差旅住宿费标准是多少" has no word boundaries.
//  2. Once a label has anchored, its spans are masked out of the query
//     before shorter labels are tested -- otherwise "差旅住宿费标准"
//     would also anchor its nested "差旅住宿费" and "差旅", producing
//     three "entities" for one mention. Masking keeps precision.
//  3. Output order follows the dictionary order (longest first).
//
// maxResults <= 0 means unlimited; callers normally pass 3-5 to bound
// prompt growth.
func (ix *Index) Match(query string, maxResults int) []Match {
	if ix == nil || !ix.available || query == "" {
		return nil
	}
	remaining := []rune(normalizeQuery(query))
	out := make([]Match, 0, 4)
	for _, e := range ix.entries {
		labelRunes := []rune(normalizeQuery(e.Label))
		if len(labelRunes) == 0 {
			continue
		}
		span := indexRunes(remaining, labelRunes)
		if span < 0 {
			continue
		}
		out = append(out, Match{
			IRI:     e.IRI,
			Label:   e.Label,
			Surface: string(labelRunes),
		})
		// Mask the consumed span (replace with a sentinel rune that no
		// label contains) so nested/shorter labels cannot re-anchor it.
		for i := span; i < span+len(labelRunes) && i < len(remaining); i++ {
			remaining[i] = '\u0000'
		}
		if maxResults > 0 && len(out) >= maxResults {
			break
		}
	}
	return out
}

// normalizeQuery REMOVES all whitespace and lowercases ASCII so
// "差旅 住宿费" and "差旅住宿费" anchor the same dictionary entry. Dropping
// (not collapsing) is deliberate: Chinese phrasing habitually inserts a
// space after a compound ("差旅 住宿费标准"), and a collapsed single space
// would still fail the substring test. CJK is untouched (ToLower is a
// no-op on Han runes).
func normalizeQuery(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', 0x3000:
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// indexRunes reports the first rune-offset where needle occurs in
// haystack, or -1. Written locally so the masking above can operate on
// rune slices without converting back and forth.
func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// unquote strips the surrounding double quotes Turtle literals carry
// (store.Store keeps the literal verbatim, `"差旅住宿费标准"`).
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// Neighbor is one KG neighbour of an anchored entity, with the predicate
// that connects it. Phase 57-02 injects these as PROMPT CONTEXT ONLY --
// never as citation candidates (the orthogonality red line).
type Neighbor struct {
	Entity    Entity
	Predicate string
}

// Neighbors walks up to maxHops over the KG starting at the anchored
// entities and returns their neighbourhood.
//
// Rules:
//   - only OUTGOING edges from an anchored entity (1 hop) or from its
//     1-hop neighbours (2 hops);
//   - the predicate must carry a label too, otherwise the injected
//     context is an opaque IRI the model cannot use;
//   - dedup by (entity IRI, predicate) and cap the output length so an
//     adversarial fan-out cannot blow the prompt budget.
//
// Returns nil when the index is unavailable -- callers must treat that as
// "no KG context", not as "no neighbours".
func (ix *Index) Neighbors(kg *store.Store, matches []Match, maxHops, limit int) []Neighbor {
	if ix == nil || !ix.available || kg == nil || len(matches) == 0 {
		return nil
	}
	if maxHops <= 0 {
		maxHops = 1
	}
	if maxHops > 2 {
		maxHops = 2
	}
	if limit <= 0 {
		limit = 12
	}

	out := make([]Neighbor, 0, limit)
	seen := make(map[string]bool, limit)
	frontier := make([]string, 0, len(matches))
	for _, m := range matches {
		frontier = append(frontier, m.IRI)
	}

	for hop := 0; hop < maxHops; hop++ {
		next := make([]string, 0, len(frontier))
		for _, iri := range frontier {
			for _, t := range kg.QueryScoped(iri, "", "", nil) {
				if t.S != iri {
					continue
				}
				// Skip structural tier membership: it carries no domain
				// knowledge and would flood the context with tier nodes.
				if strings.HasSuffix(t.P, "#belongsToTier") {
					continue
				}
				label := ix.labelByIRI(t.O)
				if label == "" {
					// Unlabelled object: recurse on the IRI but do not emit.
					next = append(next, t.O)
					continue
				}
				key := t.O + "|" + t.P
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, Neighbor{
					Entity:    Entity{IRI: t.O, Label: label},
					Predicate: localName(t.P),
				})
				if len(out) >= limit {
					return out
				}
				next = append(next, t.O)
			}
		}
		frontier = next
	}
	return out
}

// labelByIRI looks up an entity's display label in the index (empty when
// the entity is not in the dictionary).
func (ix *Index) labelByIRI(iri string) string {
	for _, e := range ix.entries {
		if e.IRI == iri {
			return e.Label
		}
	}
	return ""
}

// localName strips the namespace from a predicate IRI so the injected
// context reads "referencesTable" rather than a full URL.
func localName(iri string) string {
	if i := strings.LastIndex(iri, "#"); i >= 0 && i+1 < len(iri) {
		return iri[i+1:]
	}
	if i := strings.LastIndex(iri, "/"); i >= 0 && i+1 < len(iri) {
		return iri[i+1:]
	}
	return iri
}
