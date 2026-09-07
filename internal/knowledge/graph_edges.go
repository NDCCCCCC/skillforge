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

package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// Edge type constants — one per deterministic edge derivation.
const (
	EdgeTypeSectionAdjacent  = "section_adjacent"
	EdgeTypeVersionChain     = "version_chain"
	EdgeTypeKeywordCooccur   = "keyword_cooccur"
	EdgeTypeCitationCooccur  = "citation_cooccur"
)

// Edge represents a directed relation between two chunk nodes.
// From/To are decimal string chunk IDs matching int64ToChunkID output.
// EdgeType is one of the four constants above.
// Weight is in [0.0, 1.0].
type Edge struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	EdgeType string  `json:"edge_type"`
	Weight   float64 `json:"weight"`
}

// jaccardThreshold returns the configurable Jaccard similarity floor for
// keyword-cooccur edges. Default 0.3; override via
// SKILLFORGE_GRAPH_JACCARD_THRESHOLD. Re-read per call to support test
// isolation via t.Setenv (each test can set its own threshold).
func jaccardThreshold() float64 {
	if v := os.Getenv("SKILLFORGE_GRAPH_JACCARD_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
		log.Printf("[WARN] SKILLFORGE_GRAPH_JACCARD_THRESHOLD=%q invalid, using 0.3", v)
	}
	return 0.3
}

// stopWords filters out common words that carry no topical signal from
// filename token sets. Initialized once at package load.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"for": true, "with": true, "from": true, "this": true, "that": true,
	"policy": true, "procedure": true, "guideline": true,
	"md": true, "doc": true, "document": true, "file": true,
}

// tierSourceKey groups DocumentMeta rows by (tier, source) pair.
func tierSourceKey(tier, source string) string {
	return tier + "::" + source
}

// tokenizeFilename extracts sortable, comparable tokens from a corpus
// source path for Jaccard comparison. Tokens are lowercase alphanumeric
// runs of length >= 3, stop-word filtered, and sorted ascending.
func tokenizeFilename(source string) []string {
	// Strip extension if present.
	name := strings.TrimSuffix(source, ".md")
	name = strings.TrimSuffix(name, ".txt")
	name = strings.TrimSuffix(name, ".html")
	// Split on non-alphanumeric runes.
	raw := strings.FieldsFunc(name, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	tokens := make([]string, 0, len(raw))
	for _, t := range raw {
		lower := strings.ToLower(t)
		if len(lower) >= 3 && !stopWords[lower] {
			tokens = append(tokens, lower)
		}
	}
	sort.Strings(tokens)
	return tokens
}

// jaccard computes the Jaccard similarity coefficient between two token
// sets. Both inputs MUST be sorted ascending. Returns 0 when both sets
// are empty.
func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, x := range a {
		setA[x] = true
	}
	intersection := 0
	for _, x := range b {
		if setA[x] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ComputeEdges derives all deterministic relation edges from a corpus of
// DocumentMeta rows. It is a pure function: identical docs input always
// produces byte-for-byte identical []Edge output (GRF-02 determinism gate).
//
// The four edge types are:
//   - SectionAdjacent: consecutive chunk IDs within the same (tier, source)
//   - VersionChain: chunks whose EffectiveFrom/To windows indicate supersession
//   - KeywordCooccur: chunks from different sources whose filename token
//     Jaccard similarity is >= jaccardThreshold (default 0.3)
//   - CitationCooccur: chunks co-cited in the same verdict audit row
//     (skipped with WARN log when auditQuerier is nil or query fails)
//
// This function does NOT write edges to disk; Phase 28 derives on demand.
func ComputeEdges(docs []retriever.DocumentMeta, auditQuerier any) ([]Edge, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Group by (tier, source).
	groups := make(map[string][]retriever.DocumentMeta)
	for _, d := range docs {
		key := tierSourceKey(d.Tier, d.Source)
		groups[key] = append(groups[key], d)
	}

	var edges []Edge

	// 1. SectionAdjacent + VersionChain: iterate over each (tier, source) group.
	for _, chunks := range groups {
		// Stable sort by ID ascending.
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].ID < chunks[j].ID })

		// SectionAdjacent: consecutive IDs within same group.
		for i := 0; i < len(chunks)-1; i++ {
			if chunks[i+1].ID == chunks[i].ID+1 {
				edges = append(edges, Edge{
					From:     int64ToChunkID(chunks[i].ID),
					To:       int64ToChunkID(chunks[i+1].ID),
					EdgeType: EdgeTypeSectionAdjacent,
					Weight:   1.0,
				})
			}
		}

		// VersionChain: b supersedes a when their effective windows overlap
		// or touch at the boundary. The condition is:
		//   b.EffectiveFrom > a.EffectiveFrom   (b starts after a)
		//   a.EffectiveTo > 0                   (a has a known end)
		//   b.EffectiveFrom <= a.EffectiveTo    (b starts before or when a ends)
		//
		// This correctly produces NO edge for disjoint windows (a.To=100, b.From=200:
		// 200 <= 100 = false), YES edge for touching boundary (a.To=100, b.From=101:
		// 101 <= 100 = false — but b.From > a.From is true so the boundary case
		// needs a.To == b.From-1, which is the "touching" case we want to EXCLUDE
		// per research §2: "window touches" means a.To <= b.From, but strict
		// supersession means b is NEWER than a, so b.From >= a.EffectiveTo+1).
		//
		// Final correct condition: b.From > a.From AND a.To > 0 AND b.From <= a.To
		// This means: b starts strictly after a began, a has ended, b starts before
		// or at the moment a ended (overlapping or immediately adjacent).
		for i := 0; i < len(chunks); i++ {
			for j := i + 1; j < len(chunks); j++ {
				a, b := chunks[i], chunks[j]
				if b.EffectiveFrom > a.EffectiveFrom &&
					a.EffectiveTo > 0 &&
					b.EffectiveFrom <= a.EffectiveTo {
					edges = append(edges, Edge{
						From:     int64ToChunkID(a.ID),
						To:       int64ToChunkID(b.ID),
						EdgeType: EdgeTypeVersionChain,
						Weight:   1.0,
					})
				}
			}
		}
	}

	// 2. KeywordCooccur: Jaccard on filename tokens across sources.
	sourceTokens := make(map[string][]string)    // source -> tokens
	seenSource := make(map[string]bool)
	for _, d := range docs {
		if seenSource[d.Source] {
			continue
		}
		seenSource[d.Source] = true
		sourceTokens[d.Source] = tokenizeFilename(d.Source)
	}

	// Collect unique sources and their representative chunk IDs.
	type srcInfo struct {
		source   string
		chunkID  int64
		tokens   []string
	}
	sources := make([]srcInfo, 0, len(sourceTokens))
	for src, tokens := range sourceTokens {
		var representativeID int64
		for _, d := range docs {
			if d.Source == src {
				representativeID = d.ID
				break
			}
		}
		sources = append(sources, srcInfo{source: src, chunkID: representativeID, tokens: tokens})
	}

	threshold := jaccardThreshold()
	for i := 0; i < len(sources); i++ {
		for j := i + 1; j < len(sources); j++ {
			score := jaccard(sources[i].tokens, sources[j].tokens)
			if score >= threshold {
				edges = append(edges, Edge{
					From:     int64ToChunkID(sources[i].chunkID),
					To:       int64ToChunkID(sources[j].chunkID),
					EdgeType: EdgeTypeKeywordCooccur,
					Weight:   score,
				})
			}
		}
	}

	// 3. CitationCooccur: scan audit rows with non-empty VerdictJSON.
	if auditQuerier != nil {
		if citeEdges, err := computeCitationEdges(auditQuerier); err != nil {
			log.Printf("[WARN] ComputeEdges: citation_cooccur skipped — %v", err)
		} else {
			edges = append(edges, citeEdges...)
		}
	} else {
		log.Printf("[WARN] ComputeEdges: auditQuerier is nil — citation_cooccur edges skipped (graceful degrade)")
	}

	// Sort edges for deterministic output (GRF-02: same input → byte-identical output).
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].EdgeType < edges[j].EdgeType
	})

	return edges, nil
}

// Node represents a chunk node in neighbor/path query results.
type Node struct {
	ChunkID       string `json:"chunk_id"`
	Tier          string `json:"tier"`
	Source        string `json:"source"`
	EffectiveFrom int64  `json:"effectiveFrom"`
	EffectiveTo   int64  `json:"effectiveTo"`
}

// NeighborResult is the return type of EdgeGraph.Neighbors.
type NeighborResult struct {
	ChunkID        string  `json:"chunk_id"`
	Depth          int     `json:"depth"`
	Nodes          []Node  `json:"nodes"`
	Edges          []Edge  `json:"edges"`
	Degraded       bool    `json:"degraded"`
	DegradedReason string  `json:"degraded_reason,omitempty"`
}

// PathResult is the return type of EdgeGraph.ShortestPath.
type PathResult struct {
	From           string  `json:"from"`
	To             string  `json:"to"`
	MaxHops        int     `json:"max_hops"`
	Hops           int     `json:"hops"`
	Path           []string `json:"path"`
	Edges          []Edge   `json:"edges"`
	Degraded       bool     `json:"degraded"`
	DegradedReason string   `json:"degraded_reason,omitempty"`
}

// EdgeGraph holds the derived edge graph for neighbor/path queries.
// It is built from DocumentMeta rows via ComputeEdges + buildAdjacency.
type EdgeGraph struct {
	Edges []Edge
	adj   map[string][]Edge // chunk_id -> outgoing edges

	// metaIndex maps chunk_id (decimal string) -> DocumentMeta for fast lookup.
	metaIndex map[string]retriever.DocumentMeta
}

// buildAdjacency builds the adjacency map and meta index from the flat edge list.
// Called lazily on first Neighbors/ShortestPath access.
func (g *EdgeGraph) buildAdjacency(metas []retriever.DocumentMeta) {
	g.adj = make(map[string][]Edge, len(g.Edges))
	for _, e := range g.Edges {
		g.adj[e.From] = append(g.adj[e.From], e)
	}
	// Build meta index for fast node lookup.
	g.metaIndex = make(map[string]retriever.DocumentMeta, len(metas))
	for _, m := range metas {
		g.metaIndex[int64ToChunkID(m.ID)] = m
	}
}

// nodeFromMeta builds a Node from a DocumentMeta.
func nodeFromMeta(m retriever.DocumentMeta) Node {
	return Node{
		ChunkID:       int64ToChunkID(m.ID),
		Tier:          m.Tier,
		Source:        m.Source,
		EffectiveFrom: m.EffectiveFrom,
		EffectiveTo:   m.EffectiveTo,
	}
}

// Neighbors returns all nodes and edges reachable from chunkID within maxDepth.
// Results are sorted by ChunkID for deterministic output. Degraded=true when
// maxNodes is exceeded or maxDepth is reached with more nodes still reachable.
func (g *EdgeGraph) Neighbors(chunkID string, maxDepth int, maxNodes int) *NeighborResult {
	if g.adj == nil {
		return &NeighborResult{ChunkID: chunkID, Depth: 0, Degraded: false}
	}

	if _, ok := g.metaIndex[chunkID]; !ok {
		return nil // chunk not found in metadata
	}

	visited := map[string]bool{chunkID: true}
	queue := []string{chunkID}
	depth := 0
	resultNodes := []Node{nodeFromMeta(g.metaIndex[chunkID])}
	resultEdges := []Edge{}

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			current := queue[0]
			queue = queue[1:]
			for _, edge := range g.adj[current] {
				if !visited[edge.To] {
					visited[edge.To] = true
					queue = append(queue, edge.To)
					if m, ok := g.metaIndex[edge.To]; ok {
						resultNodes = append(resultNodes, nodeFromMeta(m))
					}
					if len(resultEdges) < maxNodes*2 {
						resultEdges = append(resultEdges, edge)
					}
				}
			}
		}
		depth++
	}

	degraded := len(visited) >= maxNodes || depth >= maxDepth
	reason := ""
	if degraded {
		if len(visited) >= maxNodes {
			reason = fmt.Sprintf("neighbor limit exceeded (%d nodes); returning partial result. see SKILLFORGE_GRAPH_MAX_NODES config.", maxNodes)
		} else {
			reason = fmt.Sprintf("depth limit reached (depth=%d); some neighbors may exist beyond this frontier. see SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT config.", maxDepth)
		}
	}

	// Sort for deterministic output.
	sort.Slice(resultNodes, func(i, j int) bool {
		return resultNodes[i].ChunkID < resultNodes[j].ChunkID
	})
	sort.Slice(resultEdges, func(i, j int) bool {
		if resultEdges[i].From != resultEdges[j].From {
			return resultEdges[i].From < resultEdges[j].From
		}
		return resultEdges[i].To < resultEdges[j].To
	})

	return &NeighborResult{
		ChunkID:        chunkID,
		Depth:          depth,
		Nodes:          resultNodes,
		Edges:          resultEdges,
		Degraded:       degraded,
		DegradedReason: reason,
	}
}

// ShortestPath returns the BFS shortest path from 'from' to 'to' within maxHops.
// Unweighted BFS; all edge types are equal weight. Degraded=true when no path
// is found within maxHops (hops=-1, empty path).
func (g *EdgeGraph) ShortestPath(from, to string, maxHops int) *PathResult {
	if g.adj == nil {
		return &PathResult{From: from, To: to, MaxHops: maxHops, Hops: -1, Degraded: true, DegradedReason: "edge graph not initialized"}
	}

	if from == to {
		return &PathResult{From: from, To: to, MaxHops: maxHops, Hops: 0, Path: []string{from}, Edges: []Edge{}, Degraded: false}
	}

	parent := map[string]string{}
	end := ""
	queue := []string{from}
	found := false

	for len(queue) > 0 && len(parent) < maxHops*20 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range g.adj[current] {
			if _, seen := parent[edge.To]; !seen && edge.To != from {
				parent[edge.To] = current
				if edge.To == to {
					found = true
					end = to
					break
				}
				queue = append(queue, edge.To)
			}
		}
		if found {
			break
		}
	}

	if !found || end == "" {
		return &PathResult{
			From: from, To: to, MaxHops: maxHops, Hops: -1,
			Path: []string{}, Edges: []Edge{},
			Degraded: true, DegradedReason: fmt.Sprintf("no path found within %d hops (max_hops); graph may be disconnected.", maxHops),
		}
	}

	// Reconstruct path.
	path := []string{}
	cur := to
	for cur != "" && cur != from {
		path = append([]string{cur}, path...)
		cur = parent[cur]
	}
	path = append([]string{from}, path...)

	// Reconstruct edges along path.
	pathEdges := []Edge{}
	for i := 0; i < len(path)-1; i++ {
		for _, e := range g.adj[path[i]] {
			if e.To == path[i+1] {
				pathEdges = append(pathEdges, e)
				break
			}
		}
	}

	// Sort for deterministic output.
	sort.Slice(pathEdges, func(i, j int) bool {
		if pathEdges[i].From != pathEdges[j].From {
			return pathEdges[i].From < pathEdges[j].From
		}
		return pathEdges[i].To < pathEdges[j].To
	})

	return &PathResult{
		From: from, To: to, MaxHops: maxHops,
		Hops: len(path) - 1, Path: path, Edges: pathEdges,
		Degraded: false,
	}
}

// EdgeGraphFromMetadata builds a fully-populated EdgeGraph from retriever metadata.
// It calls ComputeEdges, then buildAdjacency, and is the seam used by the HTTP handlers.
func EdgeGraphFromMetadata(ctx context.Context, r retriever.Retriever) (*EdgeGraph, error) {
	metas, err := r.ListMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("EdgeGraphFromMetadata: ListMetadata: %w", err)
	}
	edges, err := ComputeEdges(metas, nil)
	if err != nil {
		return nil, fmt.Errorf("EdgeGraphFromMetadata: ComputeEdges: %w", err)
	}
	eg := &EdgeGraph{Edges: edges}
	eg.buildAdjacency(metas)
	return eg, nil
}

// auditQuerier is the minimal interface needed for citation co-occurrence.
// Defined as a named type so the interface embedding in computeCitationEdges
// matches concretely at compile time.
type auditQuerier interface {
	QueryAudit(ctx context.Context, filter store.AuditFilter) ([]store.AuditRow, error)
}

// computeCitationEdges scans audit rows with non-empty VerdictJSON and
// emits co-citation edges between all pairs of chunks cited in the same
// verdict. Weight = 1.0 / distinct_chunks_in_verdict.
func computeCitationEdges(aq any) ([]Edge, error) {
	q, ok := aq.(auditQuerier)
	if !ok {
		return nil, errors.New("auditQuerier does not satisfy auditQuerier interface")
	}
	rows, err := q.QueryAudit(context.Background(), store.AuditFilter{Limit: 10000})
	if err != nil {
		return nil, err
	}

	var edges []Edge
	for _, row := range rows {
		if row.VerdictJSON == "" {
			continue
		}
		// Parse VerdictJSON to extract Citations.
		var v verdict.Verdict
		if err := json.Unmarshal([]byte(row.VerdictJSON), &v); err != nil {
			log.Printf("[WARN] computeCitationEdges: failed to parse VerdictJSON row %d — %v", row.ID, err)
			continue
		}
		if len(v.Citations) < 2 {
			continue
		}
		// Emit edges between every pair of distinct chunks in the same verdict.
		distinct := make(map[string]bool)
		for _, c := range v.Citations {
			distinct[c.ChunkID] = true
		}
		if len(distinct) < 2 {
			continue
		}
		ids := make([]string, 0, len(distinct))
		for id := range distinct {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		weight := 1.0 / float64(len(ids))
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				edges = append(edges, Edge{
					From:     ids[i],
					To:       ids[j],
					EdgeType: EdgeTypeCitationCooccur,
					Weight:   weight,
				})
			}
		}
	}
	return edges, nil
}
