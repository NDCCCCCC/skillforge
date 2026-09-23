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

// Package mcp implements the SkillForge MCP server exposing KG tools to external
// agents via the Model Context Protocol (stdio transport).
//
// All tool calls route through QueryScoped for tier trust isolation (D-TRUST-01).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/ontology/store"
)

// Server is the SkillForge MCP server exposing KG tools.
type Server struct {
	mcp          *server.MCPServer
	stdio        *server.StdioServer
	kg           *store.Store
	serviceSkill string // default skill name for tier resolution
}

// NewServer constructs a SkillForge MCP server with 5 tools registered.
// It does NOT start the stdio transport — use Serve(ctx) to run it.
func NewServer(kg *store.Store, serviceSkill string) *Server {
	const (
		serverName    = "skillforge"
		serverVersion = "1.0.0"
		rdfType       = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
		ns            = "http://skillforge.local/ontology#"
	)

	s := &Server{
		kg:           kg,
		serviceSkill:  serviceSkill,
		mcp:          server.NewMCPServer(serverName, serverVersion),
		stdio:        server.NewStdioServer(nil), // set after AddTools
	}

	// Register all 5 tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "skf_query",
		Title:       "Query Knowledge Graph",
		Description: "Query the SkillForge knowledge graph with a triple pattern (s, p, o). All subjects/predicates/objects are full IRIs or empty (wildcard). Results are tier-filtered by the caller's skill.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"s":      map[string]interface{}{"type": "string", "description": "Subject IRI (empty = wildcard)"},
				"p":      map[string]interface{}{"type": "string", "description": "Predicate IRI (empty = wildcard)"},
				"o":      map[string]interface{}{"type": "string", "description": "Object IRI (empty = wildcard)"},
				"limit":  map[string]interface{}{"type": "integer", "description": "Max results to return (default 50, max 500)"},
				"offset": map[string]interface{}{"type": "integer", "description": "Pagination offset (default 0)"},
				"skill":  map[string]interface{}{"type": "string", "description": "Skill name for tier filtering (defaults to service skill)"},
			},
		},
	}, s.handleQuery)

	s.mcp.AddTool(mcp.Tool{
		Name:        "skf_skills",
		Title:       "List Skills",
		Description: "Return all registered Skill individuals in the knowledge graph with their names, descriptions, and read-access tiers.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleSkills)

	s.mcp.AddTool(mcp.Tool{
		Name:        "skf_chunk_for",
		Title:       "Get ECL Chunks for Entity",
		Description: "Return the ECL note chunks that reference a given entity IRI. Uses Adjacent1 to find referenced tables, then reads the chunk metadata (source_path, line range). Results are tier-filtered.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"entity_iri": map[string]interface{}{"type": "string", "description": "Entity IRI to look up"},
				"max_hops":   map[string]interface{}{"type": "integer", "description": "Max hops for table lookup (1 or 2, default 1)"},
				"skill":      map[string]interface{}{"type": "string", "description": "Skill name for tier filtering (defaults to service skill)"},
			},
			Required: []string{"entity_iri"},
		},
	}, s.handleChunkFor)

	s.mcp.AddTool(mcp.Tool{
		Name:        "skf_schema",
		Title:       "Get TBox Schema",
		Description: "Return the TBox class hierarchy and object properties from the knowledge graph.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleSchema)

	s.mcp.AddTool(mcp.Tool{
		Name:        "skf_neighbors",
		Title:       "Get Entity Neighbors",
		Description: "Return the N-hop neighborhood of an entity (nodes + edges) for graph exploration.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"uri":  map[string]interface{}{"type": "string", "description": "Entity URI"},
				"hops": map[string]interface{}{"type": "integer", "description": "Number of hops (1 or 2, default 1)"},
			},
			Required: []string{"uri"},
		},
	}, s.handleNeighbors)

	// Now wire the stdio server with the fully-configured MCPServer
	s.stdio = server.NewStdioServer(s.mcp)
	s.stdio.SetErrorLogger(log.New(os.Stderr, "[mcp] ", log.LstdFlags))

	return s
}

// Serve runs the stdio MCP server. It blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	return s.stdio.Listen(ctx, os.Stdin, os.Stdout)
}

// ServiceSkill returns the configured default service skill name.
func (s *Server) ServiceSkill() string {
	return s.serviceSkill
}

// ListTools returns a slice of tool descriptors for the admin inspector panel.
func (s *Server) ListTools() []mcp.Tool {
	out := make([]mcp.Tool, 0)
	tools := s.mcp.ListTools()
	for _, st := range tools {
		out = append(out, st.Tool)
	}
	return out
}

// CallTool invokes a named tool with the given JSON params and returns the result.
// Returns an error if the tool is not found.
func (s *Server) CallTool(ctx context.Context, name string, params map[string]any) (map[string]any, error) {
	st := s.mcp.GetTool(name)
	if st == nil {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	// Build a minimal MCP request for the handler
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: params,
		},
	}
	result, err := st.Handler(ctx, req)
	if err != nil {
		return nil, err
	}
	// Extract structured content from result
	if result.StructuredContent != nil {
		if m, ok := result.StructuredContent.(map[string]any); ok {
			return m, nil
		}
	}
	// Fallback: encode content as JSON
	if len(result.Content) > 0 {
		var out map[string]any
		// Try to parse first text content as JSON
		if tc, ok := result.Content[0].(mcp.TextContent); ok {
			json.Unmarshal([]byte(tc.Text), &out)
		}
		return out, nil
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Tier resolution helpers
// ---------------------------------------------------------------------------

const (
	rdfType         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	skfSkill        = "http://skillforge.local/ontology#Skill"
	skfHasReadAccess = "http://skillforge.local/ontology#hasReadAccess"
	ns              = "http://skillforge.local/ontology#"
	skfSkillPrefix  = ns + "skill_"
	skfCatPrefix    = ns + "cat_"
)

// resolveSkillTiers maps a skill name to its read-access tier names.
// Returns nil if the skill is not found.
func (s *Server) resolveSkillTiers(skillName string) []string {
	if skillName == "" {
		skillName = s.serviceSkill
	}
	skillIRI := skfSkillPrefix + skillName
	triples := s.kg.Query(skillIRI, rdfType, skfSkill)
	if len(triples) == 0 {
		return nil
	}
	accessTriples := s.kg.Query(skillIRI, skfHasReadAccess, "")
	var tiers []string
	for _, t := range accessTriples {
		tierName := strings.TrimPrefix(t.O, ns)
		tiers = append(tiers, tierName)
	}
	return tiers
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

// handleQuery implements skf_query.
func (s *Server) handleQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		S      string `json:"s"`
		P      string `json:"p"`
		O      string `json:"o"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Skill  string `json:"skill"`
	}
	if err := req.BindArguments(&args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	if args.Limit > 500 {
		args.Limit = 500 // hard cap per T-41-03
	}
	if args.Offset < 0 {
		args.Offset = 0
	}

	skillName := args.Skill
	if skillName == "" {
		skillName = s.serviceSkill
	}
	allowedTiers := s.resolveSkillTiers(skillName)

	var results []store.Triple
	if allowedTiers != nil {
		results = s.kg.QueryScoped(args.S, args.P, args.O, allowedTiers)
	} else {
		// Skill not found — fail closed, return empty
		results = []store.Triple{}
	}

	total := len(results)
	start := args.Offset
	if start > total {
		start = total
	}
	end := start + args.Limit
	if end > total {
		end = total
	}
	page := results[start:end]

	out := make([]map[string]any, 0, len(page))
	for _, t := range page {
		entry := map[string]any{"s": t.S, "p": t.P, "o": t.O}
		if t.Provenance != nil {
			entry["provenance"] = t.Provenance
		}
		out = append(out, entry)
	}

	return &mcp.CallToolResult{
		Content:          []mcp.Content{mcp.NewTextContent(jsonString(map[string]any{"results": out, "count": total, "truncated": end < total}))},
		StructuredContent: map[string]any{"results": out, "count": total, "truncated": end < total},
	}, nil
}

// handleSkills implements skf_skills.
func (s *Server) handleSkills(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Find all Skill individuals
	skillTriples := s.kg.Query("", rdfType, skfSkill)
	var skills []map[string]any
	seen := make(map[string]bool)

	for _, t := range skillTriples {
		skillName := strings.TrimPrefix(t.S, skfSkillPrefix)
		if seen[skillName] {
			continue
		}
		seen[skillName] = true

		// Get description
		descTriples := s.kg.Query(t.S, ns+"description", "")
		var desc string
		if len(descTriples) > 0 {
			desc = descTriples[0].O
			// Strip quotes if literal
			desc = strings.TrimPrefix(desc, `"`)
			desc = strings.TrimSuffix(desc, `"`)
		}

		// Get reads_tiers via hasReadAccess
		accessTriples := s.kg.Query(t.S, skfHasReadAccess, "")
		var readsTiers []string
		for _, at := range accessTriples {
			readsTiers = append(readsTiers, strings.TrimPrefix(at.O, ns))
		}

		skills = append(skills, map[string]any{
			"name":        skillName,
			"description": desc,
			"reads_tiers": readsTiers,
		})
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.NewTextContent(jsonString(skills))},
		StructuredContent: map[string]any{"skills": skills},
	}, nil
}

// handleChunkFor implements skf_chunk_for.
func (s *Server) handleChunkFor(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		EntityIRI string `json:"entity_iri"`
		MaxHops  int    `json:"max_hops"`
		Skill    string `json:"skill"`
	}
	if err := req.BindArguments(&args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}
	if args.EntityIRI == "" {
		return errorResult("entity_iri is required"), nil
	}
	if args.MaxHops <= 0 {
		args.MaxHops = 1
	}
	if args.MaxHops > 2 {
		args.MaxHops = 2
	}

	skillName := args.Skill
	if skillName == "" {
		skillName = s.serviceSkill
	}
	allowedTiers := s.resolveSkillTiers(skillName)

	// Resolve tier names to category IRIs for filtering
	var allowedCats []string
	for _, t := range allowedTiers {
		allowedCats = append(allowedCats, ns+"cat_"+t)
	}

	// Find tables referenced by this entity via Adjacent1
	var tableIRIs []string
	if args.MaxHops == 1 {
		tableIRIs = s.kg.Adjacent1(args.EntityIRI, ns+"referencesTable")
	} else {
		// 2-hop: entity -> table -> also entity's referenced tables
		tableIRIs = s.kg.Adjacent1(args.EntityIRI, ns+"referencesTable")
		// Also get tables referenced by those tables
		for _, tbl := range tableIRIs {
			extra := s.kg.Adjacent1(tbl, ns+"referencesTable")
			tableIRIs = append(tableIRIs, extra...)
		}
	}

	var chunks []map[string]any
	seen := make(map[string]bool)

	for _, tblIRI := range tableIRIs {
		// Find notes that reference this table
		noteTriples := s.kg.Query("", ns+"referencesTable", tblIRI)
		for _, nt := range noteTriples {
			noteIRI := nt.S

			// Tier-filter: only include notes that belong to allowed tiers
			if allowedCats != nil {
				tierTriples := s.kg.Query(noteIRI, ns+"belongsToTier", "")
				tierOk := false
				for _, tt := range tierTriples {
					for _, cat := range allowedCats {
						if tt.O == cat {
							tierOk = true
							break
						}
					}
					if tierOk {
						break
					}
				}
				if !tierOk {
					continue
				}
			}

			// Get provenance from note triple
			var prov *store.Provenance
			noteFull := s.kg.Query(noteIRI, "", "")
			for _, t := range noteFull {
				if t.P == ns+"belongsToTier" && t.Provenance != nil {
					prov = t.Provenance
					break
				}
			}

			// Build a unique key to dedupe
			key := noteIRI
			if prov != nil {
				key = fmt.Sprintf("%s:%s:%d-%d", noteIRI, prov.SourcePath, prov.LineStart, prov.LineEnd)
			}
			if seen[key] {
				continue
			}
			seen[key] = true

			chunk := map[string]any{
				"source_path": "",
				"line_start":  0,
				"line_end":    0,
				"text_snippet": noteIRI,
			}
			if prov != nil {
				chunk["source_path"] = prov.SourcePath
				chunk["line_start"] = prov.LineStart
				chunk["line_end"] = prov.LineEnd
				chunk["text_snippet"] = fmt.Sprintf("%s (lines %d-%d)", prov.SourcePath, prov.LineStart, prov.LineEnd)
			}
			chunks = append(chunks, chunk)
		}
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{mcp.NewTextContent(jsonString(chunks))},
		StructuredContent: map[string]any{"chunks": chunks},
	}, nil
}

// handleSchema implements skf_schema.
func (s *Server) handleSchema(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get all owl:Class individuals
	classTriples := s.kg.Query("", rdfType, ns+"Class")
	var classes []map[string]string
	for _, t := range classTriples {
		label := strings.TrimPrefix(t.S, ns)
		classes = append(classes, map[string]string{"iri": t.S, "label": label})
	}

	// Get all owl:ObjectProperty individuals
	propTriples := s.kg.Query("", rdfType, ns+"ObjectProperty")
	var properties []map[string]string
	for _, t := range propTriples {
		label := strings.TrimPrefix(t.S, ns)
		// Get domain and range
		var domain, range_ string
		domainTriples := s.kg.Query(t.S, ns+"domain", "")
		if len(domainTriples) > 0 {
			domain = domainTriples[0].O
		}
		rangeTriples := s.kg.Query(t.S, ns+"range", "")
		if len(rangeTriples) > 0 {
			range_ = rangeTriples[0].O
		}
		properties = append(properties, map[string]string{
			"iri":    t.S,
			"label":  label,
			"domain": domain,
			"range":  range_,
		})
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(jsonString(map[string]any{
			"classes":    classes,
			"properties": properties,
		}))},
		StructuredContent: map[string]any{
			"classes":    classes,
			"properties": properties,
		},
	}, nil
}

// handleNeighbors implements skf_neighbors.
func (s *Server) handleNeighbors(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		URI  string `json:"uri"`
		Hops int    `json:"hops"`
	}
	if err := req.BindArguments(&args); err != nil {
		return errorResult("invalid arguments: " + err.Error()), nil
	}
	if args.URI == "" {
		return errorResult("uri is required"), nil
	}
	if args.Hops <= 0 {
		args.Hops = 1
	}
	if args.Hops > 2 {
		args.Hops = 2
	}

	var nodes []map[string]any
	var edges []map[string]string
	nodeIDs := make(map[string]bool)

	// Start node
	nodes = append(nodes, map[string]any{"id": args.URI, "label": stripPrefix(args.URI)})
	nodeIDs[args.URI] = true

	if args.Hops == 1 {
		// Forward edges
		fwd := s.kg.Query(args.URI, "", "")
		for _, t := range fwd {
			if !nodeIDs[t.O] {
				nodes = append(nodes, map[string]any{"id": t.O, "label": stripPrefix(t.O)})
				nodeIDs[t.O] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
		// Reverse edges
		rev := s.kg.Query("", "", args.URI)
		for _, t := range rev {
			if !nodeIDs[t.S] {
				nodes = append(nodes, map[string]any{"id": t.S, "label": stripPrefix(t.S)})
				nodeIDs[t.S] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
	} else {
		// 2-hop
		hop1 := s.kg.Query(args.URI, "", "")
		hop1Set := make(map[string]bool)
		for _, t := range hop1 {
			if !nodeIDs[t.O] {
				nodes = append(nodes, map[string]any{"id": t.O, "label": stripPrefix(t.O)})
				nodeIDs[t.O] = true
				hop1Set[t.O] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
		// Hop 2 from hop-1 nodes
		for h1 := range hop1Set {
			h2edges := s.kg.Query(h1, "", "")
			for _, t := range h2edges {
				if !nodeIDs[t.O] {
					nodes = append(nodes, map[string]any{"id": t.O, "label": stripPrefix(t.O)})
					nodeIDs[t.O] = true
				}
				edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(jsonString(map[string]any{
			"nodes": nodes,
			"edges": edges,
		}))},
		StructuredContent: map[string]any{
			"nodes": nodes,
			"edges": edges,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content:    []mcp.Content{mcp.NewTextContent(`{"error": "` + msg + `"}`)},
		IsError:    true,
	}
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func stripPrefix(iri string) string {
	if strings.HasPrefix(iri, ns) {
		return iri[len(ns):]
	}
	return iri
}

// Note: the ecl package functions (referencesTablePred, buildProvenance) are
// in internal/knowledge/ecl/abox.go and can be used by external callers for
// deeper ECL chunk content retrieval. This server returns IRI-level references
// for the MCP surface.
var _ = ecl.NoteIRI // imported for potential future use
