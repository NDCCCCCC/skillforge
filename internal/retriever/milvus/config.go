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

// Package milvus is the Milvus-backed implementation of
// retriever.Retriever. It is the Phase 2 reference impl; future backends
// (MongoDB, pgvector) will sit alongside this package under
// internal/retriever/.
//
// Package-level helpers (BuildMilvusFilter, EnsureCollection,
// SeedDocuments) are exported because scripts/spiked (regression binary)
// and the BGE-M3 fallback tests exercise them directly. The
// MilvusClient methods consume them internally so the production
// pipeline stays backend-agnostic via the Retriever interface.
package milvus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"

	"github.com/skillforge/skillforge/internal/kb"
)

// Spike collection / index defaults. The default collection name is fixed
// so the regression binary (scripts/spiked) can reuse the seeded data without
// re-configuring. Production (cmd/server) overrides via Config.
const (
	DefaultCollectionName = "skillforge_spike_test"
	DefaultEmbedDim       = 1024 // BGE-M3 dense output dimension
	// DefaultTierMaxLen is the Milvus tier (VARCHAR) column width. The slug
	// ceiling's single home is internal/kb.MaxSlugLen — this constant is
	// only that ceiling's projection onto the collection schema, so the two
	// can never drift (a second literal copy of the number would be a new
	// hardcode under the v1.2 zero-hardcode standard).
	DefaultTierMaxLen    = kb.MaxSlugLen
	DefaultContentMaxLen = 8192 // BGE-M3 context is 8192 tokens; content field mirrors that ceiling
	DefaultSourceMaxLen  = 256  // fixture relative path
	// DefaultSourceIdxMaxLen is the Milvus source_idx (VARCHAR) column width
	// (Plan 39-02 / D-Milvus-01). source_idx carries the zero-padded logical
	// note number the ECL chunker assigns (e.g. "001"); 64 leaves room for
	// non-ECL identifier schemes. Schema widths derive from this single home
	// (zero-hardcode standard) — no literal widths at use sites.
	DefaultSourceIdxMaxLen = 64
	// DefaultSectionMaxLen is the Milvus section (VARCHAR) column width
	// (Plan 39-02 / D-Milvus-01). section carries the merged logical-section
	// title (CJK-heavy), the same width class as source paths.
	DefaultSectionMaxLen = 256
	DefaultIVFNList      = 128 // IVF_FLAT nlist — small dataset, modest cluster count
	DefaultIVFNProbe     = 10  // search-time nprobe

	// Network resilience (per Plan 01-01 internal-api strategy).
	dialTimeout  = 5 * time.Second
	dialRetries  = 3
	embedTimeout = 10 * time.Second
	embedRetries = 2
)

// Config configures the Milvus client. Zero-value fields are replaced
// with Defaults at New entry.
type Config struct {
	Addr           string // gRPC host:port, e.g. "10.0.0.1:19530"
	Token          string // optional auth token; empty = dev mode (no auth)
	CollectionName string // defaults to DefaultCollectionName
	EmbedDim       int    // defaults to DefaultEmbedDim
	BGEURL         string // BGE-M3 REST endpoint (used by the test/regression helpers)
}

// withDefaults returns a copy of cfg with zero fields populated.
func (cfg Config) withDefaults() Config {
	if cfg.CollectionName == "" {
		cfg.CollectionName = DefaultCollectionName
	}
	if cfg.EmbedDim == 0 {
		cfg.EmbedDim = DefaultEmbedDim
	}
	return cfg
}

// dialMilvus is the shared dial helper used by New + the regression
// helpers (TestSeedAndFilterIntegration etc). Bounded retries with
// exponential backoff (1s, 2s, 4s) — Milvus is on the internal LAN so
// transient blips should be rare. After 3 attempts we surface the last
// error so the caller can decide whether to fail fast (production) or
// run with a mock (regression).
func dialMilvus(ctx context.Context, addr string) (client.Client, error) {
	if addr == "" {
		return nil, errors.New("milvus config: Addr is empty (set MILVUS_ADDRESS)")
	}

	var lastErr error
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; attempt < dialRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffs[attempt-1]):
			}
		}

		dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		mc, err := client.NewGrpcClient(dialCtx, addr)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("dial attempt %d/%d: %w", attempt+1, dialRetries, err)
			continue
		}

		// Health probe: confirms the connection is live, not just dialed.
		probeCtx, probeCancel := context.WithTimeout(ctx, dialTimeout)
		_, verr := mc.GetVersion(probeCtx)
		probeCancel()
		if verr != nil {
			_ = mc.Close()
			lastErr = fmt.Errorf("health probe attempt %d/%d (GetVersion): %w", attempt+1, dialRetries, verr)
			continue
		}
		return mc, nil
	}
	return nil, fmt.Errorf("milvus unreachable at %s after %d attempts: %w", addr, dialRetries, lastErr)
}

// truncateRunes caps s to at most max UTF-8 rune count so VARCHAR(max)
// fields cannot overflow on insert. Counts runes (not bytes) so CN text
// is measured correctly.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// RecreateEnvVar is the ONE-SHOT re-seed gate (plan 26-09 / Pitfall 1a):
// set to exactly "1" and the next boot drops the collection and re-creates
// it empty before the boot-time knowledge.Index re-seeds the corpus.
//
// Fail-closed by construction: ShouldRecreate accepts the literal "1" and
// nothing else, so the default (unset — and any of the loose spellings an
// ops runbook might reach for) leaves the corpus untouched. The cost
// asymmetry is the reason for the strictness: a missed re-seed leaves a
// known, documented migration step undone, while a surprise re-seed wipes
// a corpus and burns a full re-embed.
const RecreateEnvVar = "SKILLFORGE_MILVUS_RECREATE"

// ShouldRecreate reports whether the operator explicitly opted into the
// one-shot drop + recreate re-seed. Deliberately NOT a bool.Parse: the
// gate is conservative by design (T-26-09-2) — "true", "yes", "on", any
// casing, and any padding stay closed, because the destructive path this
// gate opens is a full corpus re-embed, not a flag flip.
//
// Mongo needs no twin gate: mongo.SeedFromKnowledgeDir resets the
// collection itself before every seed (internal/retriever/mongo/mongo.go
// Reset), so the mongo backend has no stale-row failure mode to escape.
func ShouldRecreate(envValue string) bool {
	return envValue == "1"
}
