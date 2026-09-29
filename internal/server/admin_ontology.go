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

// Plan 38-04 — Phase 38 Ontology Layer admin surface (D-Route-01).
//
// Wire contract (all routes X-Admin-Token-gated; 403 before any 503):
//
//	GET  /api/admin/ontology/tbox
//	     → 200 {version, triple_count, ttl}
//	GET  /api/admin/ontology/kg/stats
//	     → 200 {triple_count, byS_size, byP_size, byO_size, memory_kb_estimate}
//	GET  /api/admin/ontology/kg/query?s=&p=&o=
//	     → 200 {matches: [{s,p,o}, ...]}   (empty param = wildcard)
//	GET  /api/admin/ontology/graph?notes=note_032,note_033   (260929-g7x)
//	     → 200 {nodes: [{id,kind,label,tier?}], edges: [{from,to,type}]}
//	       (absent notes param = project every note)
//	POST /api/admin/ontology/proposals/generate   body {text}
//	     → 200 {proposal}
//	GET  /api/admin/ontology/proposals
//	     → 200 {proposals: [...]}
//	POST /api/admin/ontology/proposals/:id/approve   body {custom_local_name?}
//	     → 200 {proposal, tbox_version}
//	POST /api/admin/ontology/proposals/:id/reject
//	     → 200 {proposal}
//	POST /api/admin/ontology/snapshot
//	     → 200 {path, triple_count}
//	GET  /api/admin/ontology/changelog
//	     → 200 {current_version, entries: [...]}
//	POST /api/admin/ontology/rollback
//	     → 200 {version, new_violations}   body {target_version, reason?}
//
// Trust model (ONT-04): generate writes ONLY a pending proposal record;
// nothing enters the TBox or the KG store without the explicit approve
// verb. Approve is the single mutation path and it goes through
// induction.Manager's version-archive pipeline (v{N+1}.ttl + pointer +
// version bump + KG insert). Reject is a state mark, not a delete.
package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/ontology/induction"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// OntologyDeps bundles the Phase 38 ontology components. Deps.Ontology ==
// nil means the server started without the ontology layer: every route in
// this file answers 503 (mirrors the Knowledge / Wizard / EvalReporter
// nil-posture).
type OntologyDeps struct {
	Induction *induction.Manager
	KG        *store.Store
}

// requireOntology gates a handler on the ontology layer being wired.
func (s *Deps) requireOntology(c *app.RequestContext) (*OntologyDeps, bool) {
	if !requireAdmin(c) {
		return nil, false
	}
	if s.Ontology == nil || s.Ontology.Induction == nil || s.Ontology.KG == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ontology layer not wired"})
		return nil, false
	}
	return s.Ontology, true
}

// tboxResponse is the 200 envelope for GET /api/admin/ontology/tbox.
type tboxResponse struct {
	Version     int    `json:"version"`
	TripleCount int    `json:"triple_count"`
	TTL         string `json:"ttl"`
}

// adminOntologyTBoxHandler implements GET /api/admin/ontology/tbox.
//
// Reads the pointer TTL file (skillforge_tbox.ttl — always the newest
// version post-approve) and reports the KG store's live triple count so
// the FE class browser and the store cannot diverge.
func (s *Deps) adminOntologyTBoxHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	ttlBytes, err := os.ReadFile(filepath.Join(dep.Induction.DataDir(), "skillforge_tbox.ttl"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "read tbox: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, tboxResponse{
		Version:     dep.Induction.Version(),
		TripleCount: dep.KG.Len(),
		TTL:         string(ttlBytes),
	})
}

// adminOntologyKGStatsHandler implements GET /api/admin/ontology/kg/stats.
func (s *Deps) adminOntologyKGStatsHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, dep.KG.Stats())
}

// kgQueryResponse is the 200 envelope for GET /api/admin/ontology/kg/query.
type kgQueryResponse struct {
	Matches []store.Triple `json:"matches"`
}

// resolveSkillTiers returns the list of tier names that a skill has read access
// to, by querying the KG for hasReadAccess edges on the skill individual.
//
// D-NZK-DEF-01 (sister defect of the admin_platform.go resolver, fixed in
// 76b5290): the live KG wires skills as
//
//	skill_x hasReadAccess cat_x (KBCategory individual)
//	cat_x   belongsToTier  tier_x   (Tier individual)
//
// so each hasReadAccess object is normalized before use:
//
//  1. (O, rdf:type, skf:Tier)         → O's stripped local name (legacy
//     direct-Tier wiring stays backward compatible)
//  2. (O, skf:belongsToTier, tierIRI) → tierIRI's stripped local name
//     (KBCategory expanded through its tier edge)
//  3. neither                         → skipped (fail-closed: unknown
//     objects contribute no tier)
//
// The result is deduplicated preserving first-seen order. Unknown skills
// resolve to nil, matching resolveSkillTiersFromKG.
func (s *Deps) resolveSkillTiers(skillName string) []string {
	if s.Ontology == nil || s.Ontology.KG == nil {
		return nil
	}
	const (
		rdfType          = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
		skfSkill         = "http://skillforge.local/ontology#Skill"
		skfHasReadAccess = "http://skillforge.local/ontology#hasReadAccess"
		skfTier          = "http://skillforge.local/ontology#Tier"
		skfBelongsToTier = "http://skillforge.local/ontology#belongsToTier"
		ns               = "http://skillforge.local/ontology#"
	)

	// Find the skill individual by name.
	skillIRI := ns + "skill_" + skillName
	triples := s.Ontology.KG.Query(skillIRI, rdfType, skfSkill)
	if len(triples) == 0 {
		return nil
	}

	// Get hasReadAccess edges for this skill, normalizing each object.
	accessTriples := s.Ontology.KG.Query(skillIRI, skfHasReadAccess, "")
	var (
		tiers []string
		seen  = make(map[string]bool)
	)
	addTier := func(tierIRI string) {
		tierName := strings.TrimPrefix(tierIRI, ns)
		if tierName == "" || seen[tierName] {
			return
		}
		seen[tierName] = true
		tiers = append(tiers, tierName)
	}
	for _, t := range accessTriples {
		// Case 1: object is itself a Tier individual (legacy direct wiring).
		if len(s.Ontology.KG.Query(t.O, rdfType, skfTier)) > 0 {
			addTier(t.O)
			continue
		}
		// Case 2: KBCategory-shaped object — expand through belongsToTier.
		for _, edge := range s.Ontology.KG.Query(t.O, skfBelongsToTier, "") {
			addTier(edge.O)
		}
		// Case 3: unknown object — contributes no tier (fail-closed).
	}
	return tiers
}

// adminOntologyKGQueryHandler implements GET /api/admin/ontology/kg/query.
//
// Query params s / p / o are full IRI strings (or empty = wildcard, the
// `?x` of SC-5). Optional `skill` param triggers tier-filtered query via
// QueryScoped (D-TIER-01). Literals are matched with the store's `"value"` form.
func (s *Deps) adminOntologyKGQueryHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	subj, pred, obj := c.Query("s"), c.Query("p"), c.Query("o")
	skill := c.Query("skill")

	var matches []store.Triple
	if skill != "" {
		allowedTiers := s.resolveSkillTiers(skill)
		matches = dep.KG.QueryScoped(subj, pred, obj, allowedTiers)
	} else {
		matches = dep.KG.Query(subj, pred, obj)
	}
	if matches == nil {
		matches = []store.Triple{}
	}
	c.JSON(http.StatusOK, kgQueryResponse{Matches: matches})
}

// generateRequest is the POST body for proposals/generate.
type generateRequest struct {
	Text string `json:"text" vd:"required"`
}

// proposalGenerateResponse is the 200 envelope for proposals/generate.
type proposalGenerateResponse struct {
	Proposal induction.Proposal `json:"proposal"`
}

// adminOntologyProposalsGenerateHandler implements
// POST /api/admin/ontology/proposals/generate.
//
// ONT-04: the LLM proposes; the record lands as pending. No TBox mutation.
func (s *Deps) adminOntologyProposalsGenerateHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	var req generateRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}
	p, err := dep.Induction.Generate(ctx, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "generate: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, proposalGenerateResponse{Proposal: p})
}

// proposalsListResponse is the 200 envelope for GET proposals.
type proposalsListResponse struct {
	Proposals []induction.Proposal `json:"proposals"`
}

// adminOntologyProposalsListHandler implements
// GET /api/admin/ontology/proposals.
func (s *Deps) adminOntologyProposalsListHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	list := dep.Induction.List()
	if list == nil {
		list = []induction.Proposal{}
	}
	c.JSON(http.StatusOK, proposalsListResponse{Proposals: list})
}

// approveRequest is the POST body for proposals/:id/approve.
type approveRequest struct {
	CustomLocalName string `json:"custom_local_name,omitempty"`
}

// proposalApproveResponse is the 200 envelope for proposals/:id/approve.
type proposalApproveResponse struct {
	Proposal    induction.Proposal `json:"proposal"`
	TBoxVersion int                `json:"tbox_version"`
}

// adminOntologyProposalsApproveHandler implements
// POST /api/admin/ontology/proposals/:id/approve.
//
// The ONLY TBox mutation path. IRI conflicts (custom_local_name collision)
// map to 409 so the Drawer can surface the rename error inline.
func (s *Deps) adminOntologyProposalsApproveHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	var req approveRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}
	id := c.Param("id")
	p, version, err := dep.Induction.Approve(id, req.CustomLocalName)
	if err != nil {
		if errorsIsIRIConflict(err) {
			c.JSON(http.StatusConflict, graphResponseError{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "approve: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, proposalApproveResponse{Proposal: p, TBoxVersion: version})
}

// adminOntologyProposalsRejectHandler implements
// POST /api/admin/ontology/proposals/:id/reject.
func (s *Deps) adminOntologyProposalsRejectHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	p, err := dep.Induction.Reject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "reject: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, proposalGenerateResponse{Proposal: p})
}

// snapshotResponse is the 200 envelope for POST /api/admin/ontology/snapshot.
type snapshotResponse struct {
	Path        string `json:"path"`
	TripleCount int    `json:"triple_count"`
}

// changelogResponse is the 200 envelope for GET /api/admin/ontology/changelog.
type changelogResponse struct {
	CurrentVersion int                        `json:"current_version"`
	Entries        []induction.ChangelogEntry `json:"entries"`
}

// rollbackRequest is the POST body for /api/admin/ontology/rollback.
type rollbackRequest struct {
	TargetVersion int    `json:"target_version"`
	Reason        string `json:"reason,omitempty"`
}

// rollbackResponse is the 200 envelope for POST /api/admin/ontology/rollback.
type rollbackResponse struct {
	Version       int `json:"version"`
	NewViolations int `json:"new_violations"`
}

// adminOntologySnapshotHandler implements POST /api/admin/ontology/snapshot.
//
// Manual trigger for the debounced persistence path (D-KG-04: debug /
// operational convenience; the debounced writer remains the main path).
func (s *Deps) adminOntologySnapshotHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	path := filepath.Join(dep.Induction.DataDir(), "skillforge_kg.json")
	if err := dep.KG.WriteSnapshot(path); err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "snapshot: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshotResponse{Path: path, TripleCount: dep.KG.Len()})
}

// adminOntologyChangelogHandler implements GET /api/admin/ontology/changelog.
func (s *Deps) adminOntologyChangelogHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	entries, err := dep.Induction.ListChangelog()
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "list changelog: " + err.Error()})
		return
	}
	if entries == nil {
		entries = []induction.ChangelogEntry{}
	}
	c.JSON(http.StatusOK, changelogResponse{
		CurrentVersion: dep.Induction.Version(),
		Entries:        entries,
	})
}

// adminOntologyRollbackHandler implements POST /api/admin/ontology/rollback.
func (s *Deps) adminOntologyRollbackHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	var req rollbackRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
		return
	}
	if req.TargetVersion <= 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "target_version must be positive"})
		return
	}
	newVersion, newViolations, err := dep.Induction.RollbackTBox(req.TargetVersion, req.Reason)
	if err != nil {
		if os.IsNotExist(errors.Unwrap(err)) || strings.Contains(err.Error(), "no such file") {
			c.JSON(http.StatusNotFound, graphResponseError{Error: "version not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "rollback: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, rollbackResponse{Version: newVersion, NewViolations: newViolations})
}

// errorsIsIRIConflict maps induction.ErrIRIConflict to the 409 path.
func errorsIsIRIConflict(err error) bool { return errors.Is(err, induction.ErrIRIConflict) }

// ─── Quick 260929-g7x — ontology graph projection ──────────────────────
//
// GET /api/admin/ontology/graph?notes=note_032,note_033   (optional param)
//
// Projects the ABox knowledge graph into a note-centric entity graph for
// the FE 本体视图: instead of the retrieval-corpus tree (tier→doc→chunk)
// this surfaces the REAL ontology relationships quick-260928-nzk wrote —
//
//	note_XXX --referencesTable-->        tbl_Y     (SQL 笔记 ↔ 数据表)
//	note_XXX --involvesInsuranceProduct→ ent_Z     (笔记 ↔ 险种)
//	note_XXX --involvesOrganization→     ent_O     (笔记 ↔ 机构)
//	ent_O    --parentOrganization→       ent_P     (机构层级, 1-hop)
//
// belongsToTier never becomes a node (a Tier node would be a mega-hub
// dwarfing the business entities) — it attaches as a `tier` field on the
// nodes that carry the triple. Unknown note names yield 200 with empty
// arrays (the FE renders its own empty state). Since quick-260929-h8k the
// projection also merges organization alias pairs (武汉 vs 武汉中心支公司) —
// see entity_alias.go for the rule and its rationale.

// ontRdfType / ontRdfsLabel — predicate IRIs. Distinct names from the
// _test.go consts (rdfType etc.) that share this package under test builds.
const (
	ontRdfType   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	ontRdfsLabel = "http://www.w3.org/2000/01/rdf-schema#label"
)

// ontologyGraphNode is one projected node. Kind ∈ note|table|
// insurance_product|organization.
type ontologyGraphNode struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Tier  string `json:"tier,omitempty"`
}

// ontologyGraphEdge is one projected edge. Type ∈ references_table|
// involves_product|involves_org|parent_org.
type ontologyGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// ontologyGraphResponse is the 200 envelope for GET /api/admin/ontology/graph.
type ontologyGraphResponse struct {
	Nodes []ontologyGraphNode `json:"nodes"`
	Edges []ontologyGraphEdge `json:"edges"`
}

// adminOntologyGraphHandler implements GET /api/admin/ontology/graph.
//
// `notes` is a comma-separated list of note local names (note_032 or full
// IRIs — the skfNS prefix is tolerated). Absent/empty = project every note
// in the store (全库 mode).
func (s *Deps) adminOntologyGraphHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}
	var noteIRIs []string
	if param := strings.TrimSpace(c.Query("notes")); param != "" {
		for _, name := range strings.Split(param, ",") {
			name = strings.TrimPrefix(strings.TrimSpace(name), skfNS)
			if name != "" {
				noteIRIs = append(noteIRIs, skf(name))
			}
		}
	} else {
		noteIRIs = ontologyAllNoteIRIs(dep.KG)
	}
	c.JSON(http.StatusOK, projectOntologyGraph(dep.KG, noteIRIs))
}

// ontologyAllNoteIRIs returns every note subject in the store, deduped and
// sorted. A subject counts as a note when it is either declared
// rdf:type skf:NoteChunk or appears as the subject of one of the three
// note-owned predicates (the type triple alone is not trusted: legacy
// fixtures and hand-wired KGs may omit it).
func ontologyAllNoteIRIs(kg *store.Store) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(iri string) {
		if iri != "" && !seen[iri] {
			seen[iri] = true
			out = append(out, iri)
		}
	}
	for _, t := range kg.Query("", ontRdfType, skf("NoteChunk")) {
		add(t.S)
	}
	for _, pred := range []string{skf("referencesTable"), skf("involvesInsuranceProduct"), skf("involvesOrganization")} {
		for _, t := range kg.Query("", pred, "") {
			add(t.S)
		}
	}
	sort.Strings(out)
	return out
}

// graphAccumulator accumulates projection output with node/edge dedupe
// (the same triple re-read through several notes must yield one node).
type graphAccumulator struct {
	nodes    []ontologyGraphNode
	edges    []ontologyGraphEdge
	nodeSeen map[string]bool
	edgeSeen map[string]bool
}

func (g *graphAccumulator) node(id, kind, label, tier string) {
	if id == "" || g.nodeSeen[id] {
		return
	}
	g.nodeSeen[id] = true
	g.nodes = append(g.nodes, ontologyGraphNode{ID: id, Kind: kind, Label: label, Tier: tier})
}

func (g *graphAccumulator) edge(from, to, typ string) {
	if from == "" || to == "" || from == to {
		return
	}
	key := from + "|" + to + "|" + typ
	if g.edgeSeen[key] {
		return
	}
	g.edgeSeen[key] = true
	g.edges = append(g.edges, ontologyGraphEdge{From: from, To: to, Type: typ})
}

// projectOntologyGraph builds the note-centric projection for the given
// note IRIs. Each note expands to: the note node, its table nodes
// (references_table edges), its entity nodes (involves_product /
// involves_org edges), and — for every involved organization — its
// parentOrganization target as a first-class node + parent_org edge, so
// the 1-hop hierarchy stays complete even when the parent is not directly
// involved by any projected note.
func projectOntologyGraph(kg *store.Store, noteIRIs []string) ontologyGraphResponse {
	acc := &graphAccumulator{nodeSeen: map[string]bool{}, edgeSeen: map[string]bool{}}
	for _, noteIRI := range noteIRIs {
		// Unknown note (no triples at all) → skipped entirely: the
		// response carries empty arrays, not a ghost note node.
		if len(kg.Query(noteIRI, "", "")) == 0 {
			continue
		}
		acc.node(noteIRI, "note", ontologyNoteLabel(kg, noteIRI), ontologyTierOf(kg, noteIRI))
		ontologyExpandNote(kg, acc, noteIRI)
	}
	// Quick 260929-h8k: collapse org alias pairs (武汉 vs 武汉中心支公司) into
	// their canonical long-name node before the response is built.
	mergeOrgAliases(kg, acc)
	if acc.nodes == nil {
		acc.nodes = []ontologyGraphNode{}
	}
	if acc.edges == nil {
		acc.edges = []ontologyGraphEdge{}
	}
	return ontologyGraphResponse{Nodes: acc.nodes, Edges: acc.edges}
}

// ontologyExpandNote emits one note's tables + entities (+ org parents).
func ontologyExpandNote(kg *store.Store, acc *graphAccumulator, noteIRI string) {
	for _, t := range kg.Query(noteIRI, skf("referencesTable"), "") {
		acc.node(t.O, "table", tableLabel(t.O), ontologyTierOf(kg, t.O))
		acc.edge(noteIRI, t.O, "references_table")
	}
	for _, spec := range []struct{ pred, kind, edgeType string }{
		{skf("involvesInsuranceProduct"), "insurance_product", "involves_product"},
		{skf("involvesOrganization"), "organization", "involves_org"},
	} {
		for _, t := range kg.Query(noteIRI, spec.pred, "") {
			acc.node(t.O, spec.kind, entityLabel(kg, t.O), ontologyTierOf(kg, t.O))
			acc.edge(noteIRI, t.O, spec.edgeType)
			for _, p := range kg.Query(t.O, skf("parentOrganization"), "") {
				acc.node(p.O, "organization", entityLabel(kg, p.O), ontologyTierOf(kg, p.O))
				acc.edge(t.O, p.O, "parent_org")
			}
		}
	}
}

// literalOf returns the first object of (iri, pred, ?) with the store's
// `"value"` literal quoting stripped, or "" when absent.
func literalOf(kg *store.Store, iri, pred string) string {
	if ts := kg.Query(iri, pred, ""); len(ts) > 0 {
		return strings.Trim(ts[0].O, `"`)
	}
	return ""
}

// ontologyNoteLabel renders a note node label: rdfs:label when present,
// else the bare source_idx (note_032 → "032").
func ontologyNoteLabel(kg *store.Store, noteIRI string) string {
	if label := literalOf(kg, noteIRI, ontRdfsLabel); label != "" {
		return label
	}
	return strings.TrimPrefix(stripNS(noteIRI), "note_")
}

// entityLabel renders an entity node label: rdfs:label (the exact trimmed
// business name per D-NZK-05), else the IRI local name (CJK entities are
// hash-encoded — e.g. ent_e3f2a1b4c5d6).
func entityLabel(kg *store.Store, entIRI string) string {
	if label := literalOf(kg, entIRI, ontRdfsLabel); label != "" {
		return label
	}
	return stripNS(entIRI)
}

// tableLabel renders a table node label: the local name with the tbl_
// prefix stripped (tbl_pay_way → "pay_way").
func tableLabel(tblIRI string) string {
	return strings.TrimPrefix(stripNS(tblIRI), "tbl_")
}

// ontologyTierOf returns the subject's belongsToTier short name, or "".
func ontologyTierOf(kg *store.Store, iri string) string {
	if ts := kg.Query(iri, skf("belongsToTier"), ""); len(ts) > 0 {
		return stripNS(ts[0].O)
	}
	return ""
}
