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
		Description: "Return the N-hop neighborhood of an entity (nodes + edges) for graph exploration. Results are tier-filtered by the caller's skill (D-TRUST-01): neighbors outside the skill's read-access tiers are never returned.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"uri":   map[string]interface{}{"type": "string", "description": "Entity URI"},
				"hops":  map[string]interface{}{"type": "integer", "description": "Number of hops (1 or 2, default 1)"},
				"skill": map[string]interface{}{"type": "string", "description": "Skill name for tier filtering (defaults to service skill)"},
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
	rdfType          = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	skfSkill         = "http://skillforge.local/ontology#Skill"
	skfHasReadAccess = "http://skillforge.local/ontology#hasReadAccess"
	skfTier          = "http://skillforge.local/ontology#Tier"
	skfBelongsToTier = "http://skillforge.local/ontology#belongsToTier"
	ns               = "http://skillforge.local/ontology#"
	skfSkillPrefix   = ns + "skill_"
	skfCatPrefix     = ns + "cat_"
)

// resolveSkillTiers maps a skill name to its read-access tier names.
// Returns nil if the skill is not found.
//
// D-NZK-DEF-01（与兄弟 resolver 对齐）：真实 KG 的连线形状是
//
//	skf:skill_X skf:hasReadAccess skf:cat_<tier>   (KBCategory 个体)
//	skf:cat_<tier> skf:belongsToTier skf:<tier>     (Tier 个体)
//
// 所以 hasReadAccess 的对象绝不能直接 TrimPrefix 当 tier 名 —— 那样得到
// 的是 "cat_sql-guides"，而 store.QueryScoped 拿 tier 名去比对
// belongsToTier 的对象（Tier IRI = ns+"<tier>"），永远匹配不上：
// skf_query 在真实 KG 上恒返回空（信任模型形同虚设）。
//
// 归一逻辑与 internal/server/admin_platform.go 的 resolveSkillTiersFromKG
// 及 internal/server/admin_ontology.go 的 (*Deps).resolveSkillTiers 完全一致
// （三态）：
//
//  1. (O, rdf:type, skf:Tier)         → O 的本地名（旧的直连 Tier 写法）
//  2. (O, skf:belongsToTier, tierIRI) → tierIRI 的本地名（KBCategory 展开）
//  3. 都不是                            → 跳过（fail-closed：不贡献 tier）
//
// 结果按首次出现顺序去重。永远不返回"非 nil 的空切片"：
// store.QueryScoped 把空 allowedTiers 当作"不过滤"的 admin 基线，
// 因此"解析不出任何 tier"必须保持 nil 才能 fail-closed。
func (s *Server) resolveSkillTiers(skillName string) []string {
	if s.kg == nil {
		return nil
	}
	if skillName == "" {
		skillName = s.serviceSkill
	}
	skillIRI := skfSkillPrefix + skillName
	triples := s.kg.Query(skillIRI, rdfType, skfSkill)
	if len(triples) == 0 {
		return nil
	}
	accessTriples := s.kg.Query(skillIRI, skfHasReadAccess, "")
	return s.normalizeAccessTiers(accessTriples)
}

// normalizeAccessTiers 把一组 hasReadAccess 三元组归一成 tier 名列表。
// 三态归一与去重规则见 resolveSkillTiers 的注释；handleSkills 复用同一套
// 逻辑，保证"列出来的 reads_tiers"和"实际用于过滤的 tiers"永远是同一个值。
func (s *Server) normalizeAccessTiers(accessTriples []store.Triple) []string {
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
		// Case 1: 对象本身就是 Tier 个体（旧的直连写法，向后兼容）。
		if len(s.kg.Query(t.O, rdfType, skfTier)) > 0 {
			addTier(t.O)
			continue
		}
		// Case 2: KBCategory 形状的对象 —— 沿 belongsToTier 展开。
		for _, edge := range s.kg.Query(t.O, skfBelongsToTier, "") {
			addTier(edge.O)
		}
		// Case 3: 未知对象 —— 不贡献任何 tier（fail-closed）。
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
	if len(allowedTiers) > 0 {
		results = s.kg.QueryScoped(args.S, args.P, args.O, allowedTiers)
	} else {
		// Skill not found / no resolvable tier — fail closed, return empty
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

		// Get reads_tiers via hasReadAccess — 走与 resolveSkillTiers
		// 相同的归一逻辑，否则这里会漏出 "cat_<tier>" 形态的本地名，
		// 外部 agent 拿到的 tier 名与实际过滤用的 tier 名对不上。
		accessTriples := s.kg.Query(t.S, skfHasReadAccess, "")
		readsTiers := s.normalizeAccessTiers(accessTriples)
		if readsTiers == nil {
			readsTiers = []string{}
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
	if len(allowedTiers) == 0 {
		// Fail-closed: 解析不出任何 tier（未知 skill / 未知访问对象）
		// 就不返回任何 chunk —— 绝不返回未按 tier 过滤的结果。
		return &mcp.CallToolResult{
			Content:           []mcp.Content{mcp.NewTextContent(jsonString([]map[string]any{}))},
			StructuredContent: map[string]any{"chunks": []map[string]any{}},
		}, nil
	}

	// tier IRI = ns + tier 名。belongsToTier 的对象是 Tier 个体
	// （skf:sql-guides），不是 KBCategory 个体（skf:cat_sql-guides）——
	// 之前这里又拼了一次 "cat_" 前缀，得到 ns+"cat_sql-guides"，与真实
	// 对象永远不相等，于是 tier 过滤恒不通过、chunk_for 恒空。
	allowedTierIRIs := make([]string, 0, len(allowedTiers))
	for _, t := range allowedTiers {
		allowedTierIRIs = append(allowedTierIRIs, ns+t)
	}

	// Find tables referenced by this entity via Adjacent1
	var tableIRIs []string
	if args.MaxHops == 1 {
		tableIRIs = s.referencedTables(args.EntityIRI, allowedTiers)
	} else {
		// 2-hop: entity -> table -> also entity's referenced tables
		tableIRIs = s.referencedTables(args.EntityIRI, allowedTiers)
		// Also get tables referenced by those tables
		for _, tbl := range tableIRIs {
			extra := s.referencedTables(tbl, allowedTiers)
			tableIRIs = append(tableIRIs, extra...)
		}
	}

	var chunks []map[string]any
	seen := make(map[string]bool)

	for _, tblIRI := range tableIRIs {
		// Find notes that reference this table — QueryScoped keeps the
		// note lookup inside the tier fence (D-TRUST-01).
		//
		// 谓词/对象必须在这里再判一次：store.QueryScoped 的 schema 层
		// 直通分支（D-TIER-03）不施加 (s,p,o) 模式过滤，所以返回值里
		// 混着全部 schema 三元组。少了这道过滤，chunk 列表会被
		// Skill/Tier/KBCategory 个体污染。
		noteTriples := s.kg.QueryScoped("", ns+"referencesTable", tblIRI, allowedTiers)
		for _, nt := range noteTriples {
			if nt.P != ns+"referencesTable" || nt.O != tblIRI {
				continue
			}
			noteIRI := nt.S

			// Tier-filter: only include notes that belong to allowed tiers
			tierTriples := s.kg.QueryScoped(noteIRI, skfBelongsToTier, "", allowedTiers)
			tierOk := false
			for _, tt := range tierTriples {
				if tt.S != noteIRI || tt.P != skfBelongsToTier {
					continue
				}
				for _, tierIRI := range allowedTierIRIs {
					if tt.O == tierIRI {
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

			// Get provenance from note triple
			var prov *store.Provenance
			noteFull := s.kg.QueryScoped(noteIRI, "", "", allowedTiers)
			for _, t := range noteFull {
				if t.S != noteIRI || t.P != ns+"belongsToTier" || t.Provenance == nil {
					continue
				}
				prov = t.Provenance
				break
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

// referencedTables 返回 entity 通过 referencesTable 引用到的 table IRI。
// 走 QueryScoped 而不是裸 Adjacent1：entity 本身不可见（不在调用方
// 可见的 tier 里）时返回空，避免把别的 tier 的 table 拉进候选集。
func (s *Server) referencedTables(entityIRI string, allowedTiers []string) []string {
	triples := s.kg.QueryScoped(entityIRI, ns+"referencesTable", "", allowedTiers)
	out := make([]string, 0, len(triples))
	seen := make(map[string]bool, len(triples))
	for _, t := range triples {
		if t.P != ns+"referencesTable" || seen[t.O] {
			continue
		}
		seen[t.O] = true
		out = append(out, t.O)
	}
	return out
}

// handleNeighbors implements skf_neighbors.
func (s *Server) handleNeighbors(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		URI   string `json:"uri"`
		Hops  int    `json:"hops"`
		Skill string `json:"skill"`
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

	// D-TRUST-01: 本 tool 与其它 tool 走同一套 tier 解析逻辑。
	// 解析不出任何 tier 时必须 fail-closed（返回空），绝不能退化成
	// 未过滤的邻居查询 —— 那等于给外部 agent 开了一个绕过 tier
	// 过滤的后门（cmd/server/main.go 的公开声明即为此）。
	skillName := args.Skill
	if skillName == "" {
		skillName = s.serviceSkill
	}
	allowedTiers := s.resolveSkillTiers(skillName)
	if len(allowedTiers) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(jsonString(map[string]any{
				"nodes": []map[string]any{},
				"edges": []map[string]string{},
			}))},
			StructuredContent: map[string]any{
				"nodes": []map[string]any{},
				"edges": []map[string]string{},
			},
		}, nil
	}

	var nodes []map[string]any
	var edges []map[string]string
	nodeIDs := make(map[string]bool)

	// Start node
	nodes = append(nodes, map[string]any{"id": args.URI, "label": stripPrefix(args.URI)})
	nodeIDs[args.URI] = true

	if args.Hops == 1 {
		// Forward edges
		//
		// 模式在本地再判一次：store.QueryScoped 的 schema 层直通分支
		// （D-TIER-03）不施加 (s,p,o) 过滤，直接用它的返回值会把与
		// 起点无关的 schema 三元组一起当成"邻居"。
		fwd := s.kg.QueryScoped(args.URI, "", "", allowedTiers)
		for _, t := range fwd {
			if t.S != args.URI {
				continue
			}
			if !nodeIDs[t.O] {
				nodes = append(nodes, map[string]any{"id": t.O, "label": stripPrefix(t.O)})
				nodeIDs[t.O] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
		// Reverse edges
		rev := s.kg.QueryScoped("", "", args.URI, allowedTiers)
		for _, t := range rev {
			if t.O != args.URI {
				continue
			}
			if !nodeIDs[t.S] {
				nodes = append(nodes, map[string]any{"id": t.S, "label": stripPrefix(t.S)})
				nodeIDs[t.S] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
	} else {
		// 2-hop
		hop1 := s.kg.QueryScoped(args.URI, "", "", allowedTiers)
		hop1Set := make(map[string]bool)
		for _, t := range hop1 {
			if t.S != args.URI {
				continue
			}
			if !nodeIDs[t.O] {
				nodes = append(nodes, map[string]any{"id": t.O, "label": stripPrefix(t.O)})
				nodeIDs[t.O] = true
				hop1Set[t.O] = true
			}
			edges = append(edges, map[string]string{"source": t.S, "target": t.O, "predicate": t.P})
		}
		// Hop 2 from hop-1 nodes
		for h1 := range hop1Set {
			h2edges := s.kg.QueryScoped(h1, "", "", allowedTiers)
			for _, t := range h2edges {
				if t.S != h1 {
					continue
				}
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
