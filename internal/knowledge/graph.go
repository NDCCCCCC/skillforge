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

// Plan 05-05 / SC-5 (KNOW-06) — knowledge graph builder.
//
// GraphBuilder turns the retriever's metadata projection
// (Retriever.ListMetadata) into the tier -> doc -> chunk tree the FE
// graph browser renders. The graph endpoint deliberately bypasses the
// vector search path: the FE only needs metadata (id, tier, source,
// effective_*) — chunk verbatim text opens separately via ChunkSourceView
// when the operator clicks a chunk (threat T-5-22 mitigation).
//
// Always returns exactly 4 tier entries (权威 / 历史 / 个人 / 草稿)
// even when the underlying collection is empty — the FE empty-state
// hero is mounted in a separate code branch when no docs exist.

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// ErrNoRetriever is returned when GraphBuilder.Build is invoked with a
// nil retriever (defense-in-depth — production always wires a real
// retriever; tests may inject).
var ErrNoRetriever = errors.New("knowledge.GraphBuilder: retriever is nil")

// ErrInvalidTier wraps a doc row with an unknown tier value. KNOW-02
// already rejects this at the indexer layer, but the graph builder
// re-checks at the metadata-read seam (defense-in-depth).
type ErrInvalidTier struct {
	ID     int64
	Source string
	Tier   string
}

func (e ErrInvalidTier) Error() string {
	return fmt.Sprintf(
		"knowledge.GraphBuilder: doc id=%d source=%s has unknown tier=%q (valid: %v)",
		e.ID, e.Source, e.Tier, skill.TierEnumValues(),
	)
}

// ErrEmptySource wraps a doc row with an empty source path. Operators
// always populate source via the indexer; a hand-edited corpus could
// in principle produce an empty source, which would collapse two docs
// into the same FE group key — fail fast with a wrapped error.
type ErrEmptySource struct {
	ID int64
}

func (e ErrEmptySource) Error() string {
	return fmt.Sprintf("knowledge.GraphBuilder: doc id=%d has empty source path", e.ID)
}

// Graph is the wire shape returned by GET /api/knowledge/graph. JSON
// tags mirror the TS KnowledgeGraph type (web/src/types/knowledge.ts).
type Graph struct {
	Tiers []TierNode `json:"tiers"`
}

// TierNode is one tier in the graph. Always emitted for each of the 4
// canonical CN-char tiers per UI-SPEC §SC-5 (even if its Docs list is
// empty).
type TierNode struct {
	Tier       string   `json:"tier"`
	DocCount   int      `json:"docCount"`
	ChunkCount int      `json:"chunkCount"`
	Docs       []DocNode `json:"docs"`
}

// DocNode groups all chunks belonging to the same source path (one .md
// file in the corpus). Filename is the basename (no dir prefix) per
// UI-SPEC §SC-5.
type DocNode struct {
	ID         string     `json:"id"`
	Filename   string     `json:"filename"`
	ChunkCount int        `json:"chunkCount"`
	Chunks     []ChunkNode `json:"chunks"`
}

// ChunkNode carries one chunk's metadata. Content is intentionally
// absent (graph endpoint contract; threat T-5-22 mitigation).
type ChunkNode struct {
	ID            string `json:"id"`
	EffectiveFrom int64  `json:"effectiveFrom"` // 0 = perpetual start
	EffectiveTo   int64  `json:"effectiveTo"`   // 0 = perpetual / open-ended
	Version       string `json:"version"`        // empty when source has no frontmatter
}

// GraphBuilder reads retriever metadata and assembles a Graph.
type GraphBuilder struct {
	retriever retriever.Retriever
}

// New constructs a GraphBuilder bound to the given retriever. The
// retriever is the metadata source — NEVER call Search here.
func New(r retriever.Retriever) *GraphBuilder {
	return &GraphBuilder{retriever: r}
}

// Build reads the retriever's documents via ListMetadata and groups them
// into the tier -> doc -> chunk tree. Returns a Graph with exactly 4
// tier entries (in TierEnum order: 权威, 草稿, 历史, 个人 — the same
// deterministic order as skill.TierEnumValues). Documents with an
// unknown / empty Tier are rejected with a wrapped error — the
// backend-agnostic KNOW-02 invariant already enforced at index time
// means this should never happen in production, but a defense-in-depth
// reject keeps a hand-edited corpus from poisoning the FE.
//
// Pure metadata — no embedding / content is loaded (T-5-22).
func (b *GraphBuilder) Build(ctx context.Context) (*Graph, error) {
	if b.retriever == nil {
		return nil, ErrNoRetriever
	}
	metas, err := b.retriever.ListMetadata(ctx)
	if err != nil {
		return nil, err
	}

	// tier -> source -> []chunk
	type chunkAccum struct {
		chunks []ChunkNode
	}
	groups := make(map[string]map[string]*chunkAccum)

	for _, m := range metas {
		if _, ok := skill.TierEnumByValue(m.Tier); !ok {
			return nil, ErrInvalidTier{ID: m.ID, Source: m.Source, Tier: m.Tier}
		}
		if m.Source == "" {
			return nil, ErrEmptySource{ID: m.ID}
		}
		byDoc, ok := groups[m.Tier]
		if !ok {
			byDoc = make(map[string]*chunkAccum)
			groups[m.Tier] = byDoc
		}
		acc, ok := byDoc[m.Source]
		if !ok {
			acc = &chunkAccum{}
			byDoc[m.Source] = acc
		}
		acc.chunks = append(acc.chunks, ChunkNode{
			ID:            int64ToChunkID(m.ID),
			EffectiveFrom: m.EffectiveFrom,
			EffectiveTo:   m.EffectiveTo,
			Version:       versionFromEffective(m.EffectiveFrom, m.EffectiveTo),
		})
	}

	// Always emit all 4 tiers in deterministic order (TierEnumValues).
	tiers := skill.TierEnumValues()
	out := &Graph{Tiers: make([]TierNode, 0, len(tiers))}
	for _, tier := range tiers {
		byDoc := groups[tier]
		node := TierNode{Tier: tier}
		if len(byDoc) == 0 {
			node.Docs = []DocNode{}
			out.Tiers = append(out.Tiers, node)
			continue
		}
		// Stable doc order: alphabetical by source path.
		docKeys := make([]string, 0, len(byDoc))
		for k := range byDoc {
			docKeys = append(docKeys, k)
		}
		sort.Strings(docKeys)
		for _, src := range docKeys {
			acc := byDoc[src]
			// Sort chunks within a doc by ID ascending (deterministic).
			sort.SliceStable(acc.chunks, func(i, j int) bool {
				return acc.chunks[i].ID < acc.chunks[j].ID
			})
			node.Docs = append(node.Docs, DocNode{
				ID:         docNodeID(tier, src),
				Filename:   filenameOf(src),
				ChunkCount: len(acc.chunks),
				Chunks:     acc.chunks,
			})
			node.ChunkCount += len(acc.chunks)
		}
		node.DocCount = len(node.Docs)
		out.Tiers = append(out.Tiers, node)
	}
	return out, nil
}

// int64ToChunkID renders the numeric primary key as a decimal string
// to match the wire shape (chunk_id is a string in TS / Go).
func int64ToChunkID(id int64) string {
	// strconv.FormatInt is heavy; for IDs < 2^31 this is sufficient
	// and avoids an extra import for a 1-line conversion.
	if id == 0 {
		return "0"
	}
	neg := id < 0
	if neg {
		id = -id
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// docNodeID derives a stable, unique synthetic ID for the doc-group
// node (the FE uses it as a React key). Tier + source are sufficient —
// no chunk row in Milvus / Mongo is associated with multiple tiers or
// multiple source paths.
func docNodeID(tier, source string) string {
	return tier + "::" + source
}

// filenameOf returns the basename of the source path (e.g.
// "authority/travel_expense.md" → "travel_expense.md"). Falls back to
// the full source if no slash is present.
func filenameOf(source string) string {
	if i := strings.LastIndex(source, "/"); i >= 0 {
		return source[i+1:]
	}
	return source
}

// versionFromEffective produces a human-readable version label when
// the operator has populated the effective_* window. Empty string when
// the chunk is perpetual (0,0) — the FE renders the 当前有效 badge
// without a version suffix in that case.
//
// Format:
//   - {0, 0}      → ""           (perpetual)
//   - {from, 0}   → "自 <YYYY-MM-DD>"   (open-ended start)
//   - {0, to}     → "<YYYY-MM-DD> 起失效"   (open-ended end)
//   - {from, to}  → "<YYYY-MM-DD> ~ <YYYY-MM-DD>"   (bounded)
//
// Dates are computed from the current epoch-seconds value. The FE uses
// the structured EffectiveFrom / EffectiveTo fields directly; this
// label is a fallback for chunks that carry no other version metadata
// in the operator's corpus.
func versionFromEffective(from, to int64) string {
	switch {
	case from == 0 && to == 0:
		return ""
	case from > 0 && to == 0:
		return "自 " + formatDate(from)
	case from == 0 && to > 0:
		return formatDate(to) + " 起失效"
	default:
		return formatDate(from) + " ~ " + formatDate(to)
	}
}

// formatDate renders epoch-seconds as "YYYY-MM-DD" in UTC. Returns
// "未知" on parse failure so the FE never renders a broken cell.
func formatDate(epochSec int64) string {
	if epochSec <= 0 {
		return "未知"
	}
	t := time.Unix(epochSec, 0).UTC()
	return t.Format("2006-01-02")
}