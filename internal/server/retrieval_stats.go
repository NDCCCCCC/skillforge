// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58-04 (PROV-03) -- the data-retrieval broadcast.
//
// WHY IT EXISTS: a refusal that says "no source" without saying WHAT was
// searched is unactionable. This frame carries the channel distribution of
// the retrieval pass (vector / graph / hybrid counts), the anchored
// entities, and whether the KG took part -- emitted BEFORE answer
// generation, so it reaches the operator on every path including refusals.
//
// The counts come from the fused result set (retriever.FuseChannels): a
// dual-channel hit counts as ONE hybrid, not as one vector + one graph.
// The FE never recomputes these numbers.
package server

import (
	"encoding/json"
	"log"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/wire"
)

// buildRetrievalStats derives the channel distribution from a fused set.
//
// fused == nil is legal: it means the caller has no fusion result (pure
// vector path). In that case the vector bucket is len(chunks) and the
// graph/hybrid buckets are zero -- which is the TRUTH for a vector-only
// deployment, not a placeholder.
func buildRetrievalStats(fused []retriever.HybridResult, chunks []retriever.Hit, kgc KGContext) wire.RetrievalStats {
	stats := wire.RetrievalStats{
		KG: &wire.KGStats{
			Available:       kgc.Available,
			EntitiesMatched: kgc.EntitiesMatched,
		},
		Degraded: !kgc.Available,
	}
	for _, m := range kgc.Anchors {
		stats.AnchorEntities = append(stats.AnchorEntities, m.Label)
	}

	if fused == nil {
		// Single-channel path: the chunks carry their own channel stamp
		// (markVectorChannel), so COUNT what was actually stamped rather
		// than assuming "all vector". The counts then describe the same
		// facts the citations carry -- one source, one truth. A chunk with
		// no stamp (a pre-58 replay path) still counts as vector, which is
		// the legacy reading.
		for _, ch := range chunks {
			switch ch.Origin {
			case retriever.ChannelGraph:
				stats.Channels.Graph++
			case retriever.ChannelHybrid:
				stats.Channels.Hybrid++
			default:
				stats.Channels.Vector++
			}
		}
		return stats
	}
	for _, r := range fused {
		switch r.Origin {
		case retriever.ChannelHybrid:
			stats.Channels.Hybrid++
		case retriever.ChannelGraph:
			stats.Channels.Graph++
		default:
			stats.Channels.Vector++
		}
	}
	return stats
}

// emitRetrievalStats broadcasts the data-retrieval frame. Best-effort: a
// write failure is logged, never fatal (the answer itself must still
// stream -- observability never gates the turn).
func emitRetrievalStats(c *app.RequestContext, stats wire.RetrievalStats) {
	if c == nil {
		return
	}
	payload, err := json.Marshal(stats)
	if err != nil {
		log.Printf("[WARN] server: marshal retrieval stats: %v (non-fatal)", err)
		return
	}
	if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-retrieval", Data: payload}); err != nil {
		log.Printf("[WARN] server: emit data-retrieval: %v (non-fatal)", err)
	}
}

// retrievalStatsJSON renders the stats for the audit_meta sibling row
// (ROI-01). It returns "" when the stats carry nothing worth persisting
// (no chunks at all AND no KG participation), so a chit-chat-style turn
// does not write a meaningless blob.
//
// WHY the sibling row and not a second audit row: the verdict turn
// already writes exactly ONE audit row (a pinned invariant -- the
// persistence tests assert one row per turn). ROI data therefore rides
// audit_meta.retrieval_stats, beside the rewriter/classifier signals,
// and the ROI query joins audit x audit_meta.
func retrievalStatsJSON(stats wire.RetrievalStats) string {
	if stats.Channels.Vector == 0 && stats.Channels.Graph == 0 && stats.Channels.Hybrid == 0 &&
		stats.KG == nil {
		return ""
	}
	payload, err := json.Marshal(stats)
	if err != nil {
		log.Printf("[WARN] server: marshal retrieval stats for audit_meta: %v (non-fatal)", err)
		return ""
	}
	return string(payload)
}
