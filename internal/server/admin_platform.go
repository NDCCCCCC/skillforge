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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

// mcpStatusResponse is the 200 body for GET /api/admin/platform/mcp/status.
type mcpStatusResponse struct {
	Status       string `json:"status"`
	ServiceSkill string `json:"service_skill"`
}

// mcpToolDesc describes one registered MCP tool for the inspector panel.
type mcpToolDesc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
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
		// mark3labs/mcp-go v1.1.0 changed Tool.InputSchema from
		// map[string]interface{} to a ToolInputSchema type alias.
		// Marshal through JSON to recover the raw map shape for the
		// FE inspector (closure-first: no new behaviour, just type
		// adaptation).
		if data, err := json.Marshal(t.InputSchema); err == nil {
			_ = json.Unmarshal(data, &schema)
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

// ---------------------------------------------------------------------------
// SPARQL-like REST endpoint types and handler
// ---------------------------------------------------------------------------

// SparqlPattern is the POST body for /api/sparql (D-SPARQL-01).
type SparqlPattern struct {
	S      string `json:"s"`      // subject IRI or "": wildcard
	P      string `json:"p"`      // predicate IRI or "": wildcard
	O      string `json:"o"`      // object IRI or "": wildcard
	Limit  int    `json:"limit"`  // default 50, max 500 (D-MCP-05)
	Offset int    `json:"offset"` // default 0
	Filter string `json:"filter"` // optional: entity name substring match (case-insensitive)
}

// ProvenanceFromTriple extracts the Provenance pointer from a store.Triple.
func ProvenanceFromTriple(t store.Triple) *store.Provenance {
	return t.Provenance
}

// SparqlResult is one result triple in the SPARQL response.
type SparqlResult struct {
	S          string            `json:"s"`
	P          string            `json:"p"`
	O          string            `json:"o"`
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
	skillName := string(c.GetHeader("X-Skill-Name"))
	if skillName == "" {
		c.JSON(http.StatusUnauthorized, graphResponseError{Error: "missing X-Skill-Name header"})
		return
	}

	// Step 3: Resolve skill to tiers via KG lookup
	tiers := resolveSkillTiersFromKG(string(skillName), s.Ontology)
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

// ---------------------------------------------------------------------------
// Hybrid Retrieval pipeline types and functions (D-HYB-01..05)
// ---------------------------------------------------------------------------

// HybridQuery is the POST body for POST /api/admin/platform/hybrid/search.
type HybridQuery struct {
	Query   string `json:"query"`    // natural language query string
	MaxHops int    `json:"max_hops"` // 1 or 2, default 1 (SKILLFORGE_HYBRID_HOPS)
	Rerank  bool   `json:"rerank"`   // default true (SKILLFORGE_RETRIEVAL_RERANK)
}

// HybridKGNode is one node in the KG subgraph returned by hybrid search.
type HybridKGNode struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Kind       string            `json:"kind"` // "note"|"table"|"skill"|"file"
	Tier       string            `json:"tier,omitempty"`
	Provenance *store.Provenance `json:"provenance,omitempty"`
}

// HybridKGEdge is one edge in the KG subgraph.
type HybridKGEdge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Predicate string `json:"predicate"`
}

// HybridMilvusHit is one Milvus search result.
type HybridMilvusHit struct {
	Score      float64 `json:"score"`
	Text       string  `json:"text"`
	SourcePath string  `json:"source_path,omitempty"`
	Section    string  `json:"section,omitempty"`
}

// HybridResponse is the response body for POST /api/admin/platform/hybrid/search.
type HybridResponse struct {
	KGNodes    []HybridKGNode    `json:"kg_nodes"`
	KGEdges    []HybridKGEdge    `json:"kg_edges"`
	MilvusHits []HybridMilvusHit `json:"milvus_hits"`
}

// HybridSearchScoped implements the Hybrid Retrieval pipeline:
// query → KG 1-2 hop subgraph expansion → Milvus hybrid top-50 →
// BGE-reranker-base rerank → top-5 (D-HYB-01).
//
// Tier filtering is applied to all KG operations via QueryScoped with
// allowedTiers. Milvus search is called with the tier-filtered entity set
// as a pre-filter.
//
// The embedder is used to embed the natural language query for Milvus
// vector search. If rerank=true a reranker is invoked (SKILLFORGE_RETRIEVAL_RERANK).
// The reranker parameter accepts a function that re-ranks Milvus hits given
// the query and hit texts; pass nil to skip reranking.
func HybridSearchScoped(
	query HybridQuery,
	allowedTiers []string,
	kg *store.Store,
	retrieverClient retriever.Retriever,
	emb embedder.Embedder,
	reranker func(ctx context.Context, query string, hits []HybridMilvusHit) ([]HybridMilvusHit, error),
) (HybridResponse, error) {
	const (
		ns              = "http://skillforge.local/ontology#"
		rdfType         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
		skfNote         = ns + "NoteChunk"
		skfTable        = ns + "TableSchema"
		skfLabel        = ns + "label"
		referencesTable = ns + "referencesTable"
		belongsToTier   = ns + "belongsToTier"
		milvusTopK      = 50
		rerankTopK      = 5
	)

	// Normalize MaxHops: default 1, cap at 2
	maxHops := query.MaxHops
	if maxHops <= 0 {
		maxHops = 1
	}
	if maxHops > 2 {
		maxHops = 2
	}

	// Normalize Rerank: default true
	rerank := query.Rerank
	if !rerank { // already false means explicitly disabled
		rerank = false
	} else {
		rerank = true // default to true
	}

	// -------------------------------------------------------------------------
	// Step 1 — KG Expansion
	// -------------------------------------------------------------------------
	// Collect all ABox note/tbl entities visible to caller skill via QueryScoped.
	noteTriples := kg.QueryScoped("", rdfType, skfNote, allowedTiers)
	tableTriples := kg.QueryScoped("", rdfType, skfTable, allowedTiers)

	// Deduplicate entity IRI set; also collect labels.
	entitySet := make(map[string]bool)
	entityLabels := make(map[string]string)
	entityKinds := make(map[string]string)

	for _, t := range noteTriples {
		entitySet[t.S] = true
		entityKinds[t.S] = "note"
	}
	for _, t := range tableTriples {
		entitySet[t.S] = true
		entityKinds[t.S] = "table"
	}

	// KG expansion: traverse referencesTable edges up to maxHops.
	// For 1-hop: all objects of referencesTable from any starting entity.
	// For 2-hop: also the objects reachable from those 1-hop neighbors.
	expanded := make(map[string]bool)
	for entity := range entitySet {
		expanded[entity] = true
	}

	// First hop
	oneHop := make(map[string]bool)
	for entity := range entitySet {
		neighbors := kg.Adjacent1(entity, referencesTable)
		for _, n := range neighbors {
			oneHop[n] = true
			expanded[n] = true
		}
	}

	// Second hop (if maxHops == 2)
	if maxHops >= 2 {
		for entity := range oneHop {
			neighbors := kg.Adjacent1(entity, referencesTable)
			for _, n := range neighbors {
				expanded[n] = true
			}
		}
	}

	// Build KGNodes: for each expanded entity, fetch label and kind.
	kgnodes := make([]HybridKGNode, 0, len(expanded))
	kgedges := make([]HybridKGEdge, 0)

	// Collect label triples for expanded entities
	for entity := range expanded {
		labelTriples := kg.QueryScoped(entity, skfLabel, "", allowedTiers)
		label := entity // default label = entity IRI
		if len(labelTriples) > 0 {
			// Strip quotes from literal
			l := labelTriples[0].O
			if len(l) >= 2 && l[0] == '"' {
				l = l[1 : len(l)-1]
			}
			label = l
			entityLabels[entity] = l
		}
		kind := entityKinds[entity]
		if kind == "" {
			kind = "file"
		}
		// Fetch tier
		tierTriples := kg.QueryScoped(entity, belongsToTier, "", allowedTiers)
		tier := ""
		if len(tierTriples) > 0 {
			tier = strings.TrimPrefix(tierTriples[0].O, ns)
		}
		kgnodes = append(kgnodes, HybridKGNode{
			ID:         entity,
			Label:      label,
			Kind:       kind,
			Tier:       tier,
			Provenance: nil, // KG nodes are metadata-only for now
		})
	}

	// Build KGEdges: all referencesTable edges between expanded entities.
	edgeSet := make(map[string]bool)
	for entity := range expanded {
		edges := kg.QueryScoped(entity, referencesTable, "", allowedTiers)
		for _, t := range edges {
			if expanded[t.O] {
				key := t.S + "|" + t.P + "|" + t.O
				if !edgeSet[key] {
					edgeSet[key] = true
					kgedges = append(kgedges, HybridKGEdge{
						Source:    t.S,
						Target:    t.O,
						Predicate: t.P,
					})
				}
			}
		}
	}

	// -------------------------------------------------------------------------
	// Step 2 — Milvus Hybrid Search
	// -------------------------------------------------------------------------
	// Embed the query string.
	queryVec, err := emb.Embed(context.Background(), query.Query)
	if err != nil {
		return HybridResponse{}, fmt.Errorf("hybrid: embed query: %w", err)
	}

	// Build tier list for Milvus pre-filter.
	// We use the allowedTiers directly (tier isolation enforced upstream).
	now := time.Now().Unix()
	milvusHits, err := retrieverClient.Search(context.Background(), queryVec, allowedTiers, milvusTopK, now)
	if err != nil {
		return HybridResponse{}, fmt.Errorf("hybrid: milvus search: %w", err)
	}

	// Convert to HybridMilvusHit.
	hits := make([]HybridMilvusHit, 0, len(milvusHits))
	for _, h := range milvusHits {
		hits = append(hits, HybridMilvusHit{
			Score:      float64(h.Score),
			Text:       h.Content,
			SourcePath: h.Source,
			Section:    "", // Milvus search doesn't return section info by default
		})
	}

	// -------------------------------------------------------------------------
	// Step 3 — BGE-reranker (optional)
	// -------------------------------------------------------------------------
	if rerank && reranker != nil && len(hits) > 0 {
		reranked, err := reranker(context.Background(), query.Query, hits)
		if err != nil {
			// Reranker error is non-fatal; fall back to Milvus results.
			// Log but don't fail the request.
			log.Printf("hybrid: reranker error (falling back to Milvus results): %v", err)
		} else {
			hits = reranked
			if len(hits) > rerankTopK {
				hits = hits[:rerankTopK]
			}
		}
	} else if len(hits) > milvusTopK {
		// No reranker or rerank disabled: use Milvus top-k directly (already top-50)
		hits = hits[:milvusTopK]
	}

	// Cap at rerankTopK when reranker is used.
	if rerank && reranker == nil && len(hits) > milvusTopK {
		hits = hits[:milvusTopK]
	}

	return HybridResponse{
		KGNodes:    kgnodes,
		KGEdges:    kgedges,
		MilvusHits: hits,
	}, nil
}

// adminPlatformHybridSearchHandler implements POST /api/admin/platform/hybrid/search.
//
// D-HYB-01..05: X-Skill-Name header carries caller identity for tier isolation.
// Env vars SKILLFORGE_RETRIEVAL_RERANK (default true) and SKILLFORGE_HYBRID_HOPS
// (default 1) control the pipeline.
func (s *Deps) adminPlatformHybridSearchHandler(ctx context.Context, c *app.RequestContext) {
	// Step 1: Parse body
	var q HybridQuery
	if err := c.Bind(&q); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid request body: " + err.Error()})
		return
	}

	// Step 2: Extract X-Skill-Name header (D-HYB-05)
	skillName := string(c.GetHeader("X-Skill-Name"))
	if skillName == "" {
		c.JSON(http.StatusUnauthorized, graphResponseError{Error: "missing X-Skill-Name header"})
		return
	}

	// Step 3: Resolve skill to tiers via KG lookup
	tiers := resolveSkillTiersFromKG(string(skillName), s.Ontology)
	if tiers == nil {
		c.JSON(http.StatusUnauthorized, graphResponseError{Error: "skill not found in KG"})
		return
	}

	// Step 4: Read env vars for pipeline config (D-HYB-03, D-HYB-04)
	if os.Getenv("SKILLFORGE_RETRIEVAL_RERANK") == "false" {
		q.Rerank = false
	}
	if hops := os.Getenv("SKILLFORGE_HYBRID_HOPS"); hops == "2" {
		q.MaxHops = 2
	}

	// Step 5: Build reranker function (nil = skip reranking)
	var rerankerFn func(ctx context.Context, query string, hits []HybridMilvusHit) ([]HybridMilvusHit, error)
	// Reranker is plumbed through Deps; if a Reranker field is added later,
	// wire it here. For now, nil means reranking is disabled/unavailable.
	// The plan specifies SKILLFORGE_RETRIEVAL_RERANK controls it; when the
	// flag is true but no reranker is configured, we proceed without reranking.

	// Step 6: Call HybridSearchScoped
	resp, err := HybridSearchScoped(q, tiers, s.Ontology.KG, s.Retriever, s.Embedder, rerankerFn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: err.Error()})
		return
	}

	// Step 7: Return 200
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
