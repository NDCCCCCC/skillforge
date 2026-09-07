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

// jaccardThreshold is the configurable Jaccard similarity floor for
// keyword-cooccur edges. Default 0.3; override via
// SKILLFORGE_GRAPH_JACCARD_THRESHOLD.
var jaccardThreshold = func() float64 {
	if v := os.Getenv("SKILLFORGE_GRAPH_JACCARD_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
		log.Printf("[WARN] SKILLFORGE_GRAPH_JACCARD_THRESHOLD=%q invalid, using 0.3", v)
	}
	return 0.3
}()

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
//     (skipped with WARN log when store is nil or query fails)
//
// This function does NOT write edges to disk; Phase 28 derives on demand.
func ComputeEdges(docs []retriever.DocumentMeta, store store.Store) ([]Edge, error) {
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

		// VersionChain: b supersedes a when b.EffectiveFrom > a.EffectiveFrom
		// AND a.EffectiveTo > 0 AND a.EffectiveTo <= b.EffectiveFrom.
		for i := 0; i < len(chunks); i++ {
			for j := i + 1; j < len(chunks); j++ {
				a, b := chunks[i], chunks[j]
				if b.EffectiveFrom > a.EffectiveFrom &&
					a.EffectiveTo > 0 &&
					a.EffectiveTo <= b.EffectiveFrom {
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

	threshold := jaccardThreshold
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
	if store != nil {
		if citeEdges, err := computeCitationEdges(store); err != nil {
			log.Printf("[WARN] ComputeEdges: citation_cooccur skipped — %v", err)
		} else {
			edges = append(edges, citeEdges...)
		}
	} else {
		log.Printf("[WARN] ComputeEdges: store is nil — citation_cooccur edges skipped (graceful degrade)")
	}

	return edges, nil
}

// computeCitationEdges scans audit rows with non-empty VerdictJSON and
// emits co-citation edges between all pairs of chunks cited in the same
// verdict. Weight = 1.0 / distinct_chunks_in_verdict.
func computeCitationEdges(s store.Store) ([]Edge, error) {
	rows, err := s.QueryAudit(context.Background(), store.AuditFilter{Limit: 10000})
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
