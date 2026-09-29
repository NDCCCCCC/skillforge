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
