// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 58-02 (PROV-01/PROV-02) -- cross-channel fusion.
//
// TWO RULES drive this file, both from the milestone's research round:
//
//  1. Aggregate by CHUNK PRIMARY KEY, never by text. The same paragraph
//     legitimately appears in two documents, and fusing on text would
//     silently merge two different evidence rows into one "hybrid" hit
//     whose provenance is a lie (Azure agentic-retrieval / RAGFlow lesson).
//
//  2. Weight with RRF (Reciprocal Rank Fusion, k=60 -- the published
//     baseline). A hand-tuned weight would be a hardcoded opinion; RRF is
//     the industry default and needs no per-corpus tuning.
//
// A chunk found by both channels is `hybrid` AND keeps the raw channel
// set in Channels[] -- the "why" must survive fusion (PROV-02).
package retriever

import (
	"sort"
)

// Channel names (the wire vocabulary). Anything else is an unknown channel
// and is carried verbatim -- the FE renders unknown as raw text.
const (
	ChannelVector = "vector"
	ChannelGraph  = "graph"
	ChannelHybrid = "hybrid"
)

// RRFRankConstant is the published RRF baseline (k=60). Named so the
// number is explained once instead of inlined at the call site.
const RRFRankConstant = 60

// HybridResult is one fused hit: the chunk identity, its fused score, the
// primary Origin (vector / graph / hybrid) and the RAW channel set.
type HybridResult struct {
	// ID is the chunk primary key -- the fusion key.
	ID int64
	// Hit is the winning Hit row (vector row when both channels saw it,
	// because the vector row is the one that carries the content the
	// citation is grounded on).
	Hit Hit
	// Origin is vector | graph | hybrid.
	Origin string
	// Channels is the raw channel set, in first-seen order.
	Channels []string
	// ViaEntity / ViaRels describe the graph path when a graph channel
	// contributed (empty for pure vector hits).
	ViaEntity string
	ViaRels   []string
}

// FuseChannels merges a vector result set with a graph result set.
//
// vectorHits: rows returned by the vector backend (authoritative content).
// graphHits:  rows the graph expansion contributed (content may be empty).
//
// Behaviour:
//   - A chunk present in BOTH sets is emitted once, Origin=hybrid,
//     Channels=[vector, graph], score = RRF(vector) + RRF(graph).
//   - A chunk only in vector is Origin=vector.
//   - A chunk only in graph is Origin=graph -- it is a CONTEXT candidate,
//     never a citation candidate (the caller must not put it in Citations
//     without a vector relevance check; see server-side verifyGraphOnly).
//
// Ordering: fused score descending, then ID ascending so equal scores are
// deterministic across runs.
func FuseChannels(vectorHits, graphHits []Hit) []HybridResult {
	byID := make(map[int64]*HybridResult, len(vectorHits)+len(graphHits))
	order := make([]int64, 0, len(vectorHits)+len(graphHits))

	ensure := func(h Hit) *HybridResult {
		r, ok := byID[h.ID]
		if !ok {
			r = &HybridResult{ID: h.ID, Hit: h}
			byID[h.ID] = r
			order = append(order, h.ID)
		}
		return r
	}

	score := map[int64]float64{}

	for rank, h := range vectorHits {
		r := ensure(h)
		score[h.ID] += 1.0 / float64(RRFRankConstant+rank+1)
		r.Channels = appendChannel(r.Channels, ChannelVector)
	}

	for rank, h := range graphHits {
		r := ensure(h)
		score[h.ID] += 1.0 / float64(RRFRankConstant+rank+1)
		r.Channels = appendChannel(r.Channels, ChannelGraph)
		// The graph row carries the path; keep the FIRST one seen so the
		// attribution is stable (a second path to the same chunk is a
		// detail the audit row can carry later, not a UI fact).
		if r.Hit.Content == "" {
			r.Hit = h
		}
		if r.ViaEntity == "" {
			r.ViaEntity = h.ViaEntity
			r.ViaRels = h.ViaRels
		}
	}

	for id, r := range byID {
		r.Hit.Origin = originFor(r.Channels)
		r.Hit.ViaEntity = r.ViaEntity
		r.Hit.ViaRels = r.ViaRels
		r.Hit.Channels = append([]string(nil), r.Channels...)
		r.Origin = r.Hit.Origin
		// The fused score replaces the single-channel cosine score: the
		// displayed number must be the one that produced the ordering.
		r.Hit.Score = float32(score[id])
	}

	out := make([]HybridResult, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hit.Score != out[j].Hit.Score {
			return out[i].Hit.Score > out[j].Hit.Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// appendChannel appends a channel once (first-seen order preserved).
func appendChannel(channels []string, ch string) []string {
	for _, c := range channels {
		if c == ch {
			return channels
		}
	}
	return append(channels, ch)
}

// originFor maps a raw channel set to the wire origin value.
func originFor(channels []string) string {
	if len(channels) > 1 {
		return ChannelHybrid
	}
	if len(channels) == 1 {
		return channels[0]
	}
	return ChannelVector
}
