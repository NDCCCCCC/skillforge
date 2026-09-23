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

// Plan 41-01 — Admin platform API for MCP Inspector panel.
//
// Wire contract (all routes X-Admin-Token-gated; 403 before 503):
//
//	GET  /api/admin/platform/mcp/status
//	     → 200 {status: "online"|"offline", service_skill: string}
//	       Always "online" when MCPServer is non-nil (subprocess spawned at boot).
//	GET  /api/admin/platform/mcp/tools
//	     → 200 [{name, description, inputSchema}]
//	       Returns the registered MCP tool list.
//	POST /api/admin/platform/mcp/invoke
//	     Body: {tool: string, params: object}
//	     → 200 tool result
//	       400 if tool name unknown; 500 for internal errors.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/server/platform/mcp"
)

// graphResponseError is the standard admin JSON error envelope.
type graphResponseError struct {
	Error string `json:"error"`
}

// mcpStatusResponse is the 200 body for GET /api/admin/platform/mcp/status.
type mcpStatusResponse struct {
	Status       string `json:"status"`
	ServiceSkill string `json:"service_skill"`
}

// mcpToolDesc describes one registered MCP tool for the inspector panel.
type mcpToolDesc struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	InputSchema   map[string]interface{} `json:"inputSchema"`
}

// adminPlatformMcpStatusHandler implements GET /api/admin/platform/mcp/status.
func (s *Deps) adminPlatformMcpStatusHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	c.JSON(http.StatusOK, mcpStatusResponse{
		Status:       "online",
		ServiceSkill: s.MCPServer.ServiceSkill(),
	})
}

// adminPlatformMcpToolsHandler implements GET /api/admin/platform/mcp/tools.
func (s *Deps) adminPlatformMcpToolsHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	tools := s.MCPServer.ListTools()
	out := make([]mcpToolDesc, 0, len(tools))
	for _, t := range tools {
		schema := make(map[string]interface{})
		if t.InputSchema != nil {
			if m, ok := t.InputSchema.(map[string]interface{}); ok {
				schema = m
			}
		}
		out = append(out, mcpToolDesc{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	c.JSON(http.StatusOK, out)
}

// mcpInvokeRequest is the POST body for /api/admin/platform/mcp/invoke.
type mcpInvokeRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

// adminPlatformMcpInvokeHandler implements POST /api/admin/platform/mcp/invoke.
func (s *Deps) adminPlatformMcpInvokeHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.MCPServer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "MCP server not wired"})
		return
	}
	var req mcpInvokeRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid request body"})
		return
	}
	if req.Tool == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "tool name is required"})
		return
	}

	// Delegate to the MCP server for actual invocation.
	// The MCP server is a stdio subprocess; this call routes through the
	// admin API handler which calls into the MCPServer struct.
	result, err := s.MCPServer.CallTool(ctx, req.Tool, req.Params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// requireAdmin returns true and writes 403 if the admin token is missing.
func requireAdmin(c *app.RequestContext) bool {
	token := c.GetHeader("X-Admin-Token")
	if token == "" {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// SPARQL-like REST endpoint types and handler
// ---------------------------------------------------------------------------

// SparqlPattern is the POST body for /api/sparql (D-SPARQL-01).
type SparqlPattern struct {
	S      string `json:"s"`       // subject IRI or "": wildcard
	P      string `json:"p"`       // predicate IRI or "": wildcard
	O      string `json:"o"`       // object IRI or "": wildcard
	Limit  int    `json:"limit"`   // default 50, max 500 (D-MCP-05)
	Offset int    `json:"offset"`  // default 0
	Filter string `json:"filter"`  // optional: entity name substring match (case-insensitive)
}

// ProvenanceFromTriple extracts the Provenance pointer from a store.Triple.
func ProvenanceFromTriple(t store.Triple) *store.Provenance {
	return t.Provenance
}

// SparqlResult is one result triple in the SPARQL response.
type SparqlResult struct {
	S          string           `json:"s"`
	P          string           `json:"p"`
	O          string           `json:"o"`
	Provenance *store.Provenance `json:"provenance,omitempty"`
}

// SparqlResponse is the response body for /api/sparql (D-SPARQL-01).
type SparqlResponse struct {
	Results   []SparqlResult `json:"results"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated"`
}

// SparqlQueryScoped is the shared SPARQL handler logic used by both the REST
// endpoint and the MCP skf_query tool (D-SPARQL-01).
//
// Steps:
//  1. If pattern.Filter is non-empty, apply substring match on entity names
//     after the query returns results (case-insensitive on S/P/O string values).
//  2. Call kg.QueryScoped(pattern.S, pattern.P, pattern.O, allowedTiers).
//  3. Apply Offset and Limit pagination.
//  4. Compute Count (total matches before pagination) and Truncated.
//  5. Default Limit to 50 if 0; cap Limit at 500 (D-MCP-05).
//  6. Returns error for negative limit/offset (→ 400).
func SparqlQueryScoped(pattern SparqlPattern, allowedTiers []string, kg *store.Store) (SparqlResponse, error) {
	// Validate
	if pattern.Limit < 0 || pattern.Offset < 0 {
		return SparqlResponse{}, fmt.Errorf("negative limit or offset")
	}

	// Default limit to 50; cap at 500
	limit := pattern.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := pattern.Offset

	// Step 1: Query the KG with tier filtering
	triples := kg.QueryScoped(pattern.S, pattern.P, pattern.O, allowedTiers)

	// Step 2: Apply filter if specified (case-insensitive substring on S/P/O)
	if pattern.Filter != "" {
		lower := strings.ToLower(pattern.Filter)
		filtered := make([]store.Triple, 0, len(triples))
		for _, t := range triples {
			if strings.Contains(strings.ToLower(t.S), lower) ||
				strings.Contains(strings.ToLower(t.P), lower) ||
				strings.Contains(strings.ToLower(t.O), lower) {
				filtered = append(filtered, t)
			}
		}
		triples = filtered
	}

	// Step 3: Count before pagination
	count := len(triples)

	// Step 4: Apply pagination
	start := offset
	if start > count {
		start = count
	}
	end := start + limit
	if end > count {
		end = count
	}
	page := triples[start:end]

	// Step 5: Build response
	results := make([]SparqlResult, 0, len(page))
	for _, t := range page {
		results = append(results, SparqlResult{
			S:          t.S,
			P:          t.P,
			O:          t.O,
			Provenance: t.Provenance,
		})
	}

	return SparqlResponse{
		Results:   results,
		Count:     count,
		Truncated: count > offset+limit,
	}, nil
}

// adminPlatformSparqlHandler implements POST /api/sparql.
//
// D-SPARQL-01: POST body = {s?, p?, o?, limit?, offset?, filter?}
// D-SPARQL-02: Read-only — no mutation verbs.
// D-SPARQL-03: X-Skill-Name header carries skill name for tier isolation.
// Not specified → 401 (no caller context).
func (s *Deps) adminPlatformSparqlHandler(ctx context.Context, c *app.RequestContext) {
	// Step 1: Parse body
	var pattern SparqlPattern
	if err := c.Bind(&pattern); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid request body: " + err.Error()})
		return
	}

	// Step 2: Extract X-Skill-Name header (D-SPARQL-03)
	skillName := c.GetHeader("X-Skill-Name")
	if skillName == "" {
		c.JSON(http.StatusUnauthorized, graphResponseError{Error: "missing X-Skill-Name header"})
		return
	}

	// Step 3: Resolve skill to tiers via KG lookup
	tiers := resolveSkillTiersFromKG(skillName, s.Ontology)
	if tiers == nil {
		// Skill not found in KG → 401 (no silent fallback to admin baseline)
		c.JSON(http.StatusUnauthorized, graphResponseError{Error: "skill not found in KG"})
		return
	}

	// Step 4: Call shared handler
	resp, err := SparqlQueryScoped(pattern, tiers, s.Ontology.KG)
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}

	// Step 5: Return 200 with direct JSON response (no envelope)
	c.JSON(http.StatusOK, resp)
}

// resolveSkillTiersFromKG looks up a skill name in the KG and returns its
// hasReadAccess tier list. Returns nil if the skill is not found.
func resolveSkillTiersFromKG(skillName string, ont *OntologyDeps) []string {
	if ont == nil || ont.KG == nil {
		return nil
	}
	const (
		rdfType          = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
		skfSkill         = "http://skillforge.local/ontology#Skill"
		skfHasReadAccess = "http://skillforge.local/ontology#hasReadAccess"
		ns               = "http://skillforge.local/ontology#"
		skfSkillPrefix   = ns + "skill_"
	)

	skillIRI := skfSkillPrefix + skillName
	triples := ont.KG.Query(skillIRI, rdfType, skfSkill)
	if len(triples) == 0 {
		return nil
	}
	accessTriples := ont.KG.Query(skillIRI, skfHasReadAccess, "")
	var tiers []string
	for _, t := range accessTriples {
		tierName := strings.TrimPrefix(t.O, ns)
		tiers = append(tiers, tierName)
	}
	return tiers
}

