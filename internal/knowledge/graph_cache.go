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
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/skillforge/skillforge/internal/retriever"
)

const (
	// cacheKeyMain is the sync.Map key for the main edge graph cache.
	cacheKeyMain = "graph-v1"
	// cacheKeyCitation is the sync.Map key for the citation co-occurrence sub-cache.
	cacheKeyCitation = "citation-v1"
)

// defaultGraphTTLMs is the default TTL for the main graph cache in milliseconds.
const defaultGraphTTLMs = 5000

// defaultCitationTTLMs is the default TTL for the citation sub-cache in milliseconds.
const defaultCitationTTLMs = 30000

// GraphCacheEntry holds a cached EdgeGraph and its absolute expiry time.
type GraphCacheEntry struct {
	EdgeGraph *EdgeGraph
	ExpiresAt time.Time
}

// CitationCacheEntry holds a cached citation EdgeGraph and its absolute expiry time.
type CitationCacheEntry struct {
	EdgeGraph *EdgeGraph
	ExpiresAt time.Time
}

// graphCache is the sync.Map-backed main TTL cache (key = cacheKeyMain).
var graphCache sync.Map

// citationCache is the sync.Map-backed citation co-occurrence sub-cache
// (key = cacheKeyCitation).
var citationCache sync.Map

// graphCacheInvalidateHook is called by the Phase 27 upload handler after a
// successful upload. Bound at boot via SetGraphCacheInvalidateHook. nil means
// no-op (no upload has been seen yet; cache is still valid).
var graphCacheInvalidateHook func()

// SetGraphCacheInvalidateHook wires the cache invalidation seam. The Phase 27
// upload handler calls this at boot to attach InvalidateCache as the hook body.
func SetGraphCacheInvalidateHook(fn func()) { graphCacheInvalidateHook = fn }

// TTL returns the graph cache TTL by reading SKILLFORGE_GRAPH_CACHE_TTL_MS.
// It returns defaultGraphTTLMs when the env var is absent or invalid.
func TTL() time.Duration {
	if v := os.Getenv("SKILLFORGE_GRAPH_CACHE_TTL_MS"); v != "" {
		if ms, err := time.ParseDuration(v + "ms"); err == nil && ms > 0 {
			return ms
		}
		log.Printf("[WARN] knowledge: SKILLFORGE_GRAPH_CACHE_TTL_MS=%q invalid, using %dms", v, defaultGraphTTLMs)
	}
	return time.Duration(defaultGraphTTLMs) * time.Millisecond
}

// citationTTL returns the citation sub-cache TTL by reading
// SKILLFORGE_GRAPH_CITATION_CACHE_TTL_MS. It returns defaultCitationTTLMs when
// the env var is absent or invalid.
func citationTTL() time.Duration {
	if v := os.Getenv("SKILLFORGE_GRAPH_CITATION_CACHE_TTL_MS"); v != "" {
		if ms, err := time.ParseDuration(v + "ms"); err == nil && ms > 0 {
			return ms
		}
		log.Printf("[WARN] knowledge: SKILLFORGE_GRAPH_CITATION_CACHE_TTL_MS=%q invalid, using %dms", v, defaultCitationTTLMs)
	}
	return time.Duration(defaultCitationTTLMs) * time.Millisecond
}

// InvalidateCache clears both the main and citation sub-cache.
// It is called by the invalidation hook when a Phase 27 upload succeeds.
func InvalidateCache() {
	graphCache.Delete(cacheKeyMain)
	citationCache.Delete(cacheKeyCitation)
	log.Printf("[WARN] knowledge: graph cache invalidated (upload/deactivate event)")
}

// ResetCaches clears both the main and citation cache.
// Exported for test isolation; not called in production.
func ResetCaches() {
	graphCache.Delete(cacheKeyMain)
	citationCache.Delete(cacheKeyCitation)
}

// GetOrCompute returns a cached EdgeGraph if still valid, otherwise recomputes
// via EdgeGraphFromMetadata and caches the result with TTL() expiry.
// Lazy eviction: expired entries are replaced on next call (no background
// goroutine needed for v1.2).
func GetOrCompute(ctx context.Context, r retriever.Retriever) (*EdgeGraph, error) {
	// Defensive nil check — fail fast rather than returning an empty graph.
	if r == nil {
		return nil, errors.New("GetOrCompute: retriever is nil")
	}
	// Check main cache.
	if entry, ok := graphCache.Load(cacheKeyMain); ok {
		ce, _ := entry.(*GraphCacheEntry) //nolint:errcheck // ROBUST-01: invariant — see preview.go
		if time.Now().Before(ce.ExpiresAt) {
			return ce.EdgeGraph, nil
		}
		// Expired — delete so recompute proceeds below.
		graphCache.Delete(cacheKeyMain)
	}

	// Recompute.
	eg, err := EdgeGraphFromMetadata(ctx, r)
	if err != nil {
		return nil, err
	}
	graphCache.Store(cacheKeyMain, &GraphCacheEntry{
		EdgeGraph: eg,
		ExpiresAt: time.Now().Add(TTL()),
	})
	return eg, nil
}

// GetCitationEdges returns cached citation co-occurrence edges if still valid,
// otherwise scans the audit via the provided auditQuerier, derives citation
// edges, caches the result with citationTTL() expiry, and returns it.
// Degrades gracefully: audit scan failure logs a WARN and returns an empty
// EdgeGraph (not nil, not an error) so the main graph remains usable.
func GetCitationEdges(ctx context.Context, auditQuerier any) (*EdgeGraph, error) {
	// Check citation sub-cache.
	if entry, ok := citationCache.Load(cacheKeyCitation); ok {
		ce, _ := entry.(*CitationCacheEntry) //nolint:errcheck // ROBUST-01: invariant — see preview.go
		if time.Now().Before(ce.ExpiresAt) {
			return ce.EdgeGraph, nil
		}
		citationCache.Delete(cacheKeyCitation)
	}

	// Compute citation edges (scan audit rows).
	edges, err := computeCitationEdges(auditQuerier)
	if err != nil {
		log.Printf("[WARN] knowledge: GetCitationEdges: audit scan failed — %v; returning empty EdgeGraph", err)
		// Degrade gracefully: return empty EdgeGraph, not nil and not the error.
		return &EdgeGraph{}, nil
	}
	if len(edges) == 0 {
		return &EdgeGraph{}, nil
	}
	eg := &EdgeGraph{Edges: edges}
	// buildAdjacency expects metas — but citation edges have no metadata here.
	// We avoid calling buildAdjacency to prevent a nil map dereference by
	// populating adj with what we have: a simple map from each edge endpoint
	// to the outgoing edges.
	eg.adj = make(map[string][]Edge, len(edges))
	for _, e := range edges {
		eg.adj[e.From] = append(eg.adj[e.From], e)
	}
	citationCache.Store(cacheKeyCitation, &CitationCacheEntry{
		EdgeGraph: eg,
		ExpiresAt: time.Now().Add(citationTTL()),
	})
	return eg, nil
}
