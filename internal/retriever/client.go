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

// Package retriever wraps the Milvus Go SDK v2 with SkillForge production
// conveniences, graduated from internal/spike/milvus_client.go in Plan 02-01.
//
// The package is split by responsibility:
//   - client.go  — config, dial, collection bootstrap, document seeding
//   - filter.go  — BuildMilvusFilter (default-deny tier predicate)
//   - search.go  — SearchWithTierFilter (PRE-similarity tier filter) + types
//   - embed.go   — BGE-M3 HTTP client + deterministic offline fallback
//
// The collection schema is intentionally minimal: id (INT64 PK), tier
// (VARCHAR 20), content (VARCHAR 8192), source (VARCHAR 256), embedding
// (FLOAT_VECTOR 1024). The tier column is the trust boundary: the filter
// expression from BuildMilvusFilter is the only mechanism that prevents
// cross-tier retrieval, so it is applied at the SDK Search layer, never
// post-search (Pitfall 3 / P-RAG-1).
//
// SDK note: milvus-sdk-go v2.4.2's Search uses POSITIONAL arguments
// (expr is the 4th positional param), not a SearchQueryOptions struct. The
// earlier RESEARCH.md draft described a struct-based API that does not
// exist in v2.4.2; this package uses the verified positional API. See
// Phase 1 SUMMARY.md deviation #1.
package retriever

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Spike collection / index defaults. The default collection name is fixed
// so the regression binary (cmd/spiked) can reuse the seeded data without
// re-configuring. Production (02-03 cmd/server) overrides via MilvusConfig.
const (
	DefaultCollectionName = "skillforge_spike_test"
	DefaultEmbedDim       = 1024 // BGE-M3 dense output dimension
	DefaultTierMaxLen     = 20   // CN tier names: 权威/历史/个人/草稿 (≤2 chars, 20 is headroom)
	DefaultContentMaxLen  = 8192 // BGE-M3 context is 8192 tokens; content field mirrors that ceiling
	DefaultSourceMaxLen   = 256  // fixture relative path
	DefaultIVFNList       = 128  // IVF_FLAT nlist — small dataset, modest cluster count
	DefaultIVFNProbe      = 10   // search-time nprobe

	// Network resilience (per Plan 01-01 internal-api strategy).
	dialTimeout  = 5 * time.Second
	dialRetries  = 3
	embedTimeout = 10 * time.Second
	embedRetries = 2
)

// MilvusConfig configures the Milvus client. Zero-value fields are
// replaced with Defaults at NewMilvusClient / SeedDocuments entry.
type MilvusConfig struct {
	Addr           string // gRPC host:port, e.g. "10.0.0.1:19530"
	Token          string // optional auth token; empty = dev mode (no auth)
	CollectionName string // defaults to DefaultCollectionName
	EmbedDim       int    // defaults to DefaultEmbedDim
	BGEURL         string // BGE-M3 REST endpoint (EmbedQuery only)
}

// withDefaults returns a copy of cfg with zero fields populated.
func (cfg MilvusConfig) withDefaults() MilvusConfig {
	if cfg.CollectionName == "" {
		cfg.CollectionName = DefaultCollectionName
	}
	if cfg.EmbedDim == 0 {
		cfg.EmbedDim = DefaultEmbedDim
	}
	return cfg
}

// NewMilvusClient dials the configured Milvus gRPC address with a bounded
// dial timeout and exponential-backoff retry (1s, 2s, 4s). It verifies
// reachability with a GetVersion health probe before returning. Returns
// the connected client OR the last error (caller surfaces "[FAIL] milvus
// unreachable" and decides checkpoint / fallback).
//
// The internal-api strategy (Plan 01-01) means Milvus is on an 内网 server
// reachable via MILVUS_ADDRESS; LAN latency is 1-5ms so 3 retries with
// backoff comfortably absorb transient blips without masking a real outage.
func NewMilvusClient(ctx context.Context, cfg MilvusConfig) (client.Client, error) {
	cfg = cfg.withDefaults()
	if cfg.Addr == "" {
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
		// NewGrpcClient is the simplest non-deprecated-in-practice constructor
		// for v2.4.2 (NewClient with a Config struct is the SDK's preferred
		// path but NewGrpcClient remains fully supported in v2.4.2).
		mc, err := client.NewGrpcClient(dialCtx, cfg.Addr)
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
	return nil, fmt.Errorf("milvus unreachable at %s after %d attempts: %w", cfg.Addr, dialRetries, lastErr)
}

// EnsureCollection creates the collection + IVF_FLAT index if they do
// not already exist. Idempotent: safe to call on every seed run. Schema:
//
//	id      INT64  primary key (auto-id OFF — caller assigns numeric IDs)
//	tier    VARCHAR(20)
//	content VARCHAR(8192)
//	source  VARCHAR(256)
//	embedding FLOAT_VECTOR(1024)
//
// The IVF_FLAT index (nlist=128) is built with COSINE metric, the standard
// for BGE-M3 dense embeddings (BGE-M3 outputs are meaningful under cosine
// similarity). The metric choice does not affect the tier-filter bleed
// assertion — that is enforced by the filter expression, not the metric.
func EnsureCollection(ctx context.Context, mc client.Client, cfg MilvusConfig) error {
	cfg = cfg.withDefaults()

	has, err := mc.HasCollection(ctx, cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("HasCollection(%q): %w", cfg.CollectionName, err)
	}
	if has {
		// Index may still be missing if a prior run created the collection
		// but crashed before CreateIndex. Best-effort create; ignore "exists".
		if err := createIndexIfAbsent(ctx, mc, cfg); err != nil {
			return fmt.Errorf("createIndexIfAbsent: %w", err)
		}
		return nil
	}

	schema := entity.NewSchema().WithName(cfg.CollectionName).WithAutoID(false).
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(false)).
		WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultTierMaxLen))).
		WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultContentMaxLen))).
		WithField(entity.NewField().WithName("source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultSourceMaxLen))).
		WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(cfg.EmbedDim)))

	if err := mc.CreateCollection(ctx, schema, 1); err != nil { // shardsNum=1 (spike)
		return fmt.Errorf("CreateCollection(%q): %w", cfg.CollectionName, err)
	}
	if err := createIndexIfAbsent(ctx, mc, cfg); err != nil {
		return fmt.Errorf("createIndexIfAbsent: %w", err)
	}
	return nil
}

// createIndexIfAbsent creates the IVF_FLAT index on the embedding field.
// Errors indicating the index already exists are swallowed.
func createIndexIfAbsent(ctx context.Context, mc client.Client, cfg MilvusConfig) error {
	cfg = cfg.withDefaults()
	idx, err := entity.NewIndexIvfFlat(entity.COSINE, DefaultIVFNList)
	if err != nil {
		return fmt.Errorf("NewIndexIvfFlat: %w", err)
	}
	// async=false so the index is ready before we return; safe for a spike.
	if err := mc.CreateIndex(ctx, cfg.CollectionName, "embedding", idx, false); err != nil {
		// Milvus returns a specific error if the index already exists; treat
		// "already exists"-style messages as success. The error text varies
		// by Milvus version, so we match on a substring.
		if strings.Contains(strings.ToLower(err.Error()), "already exists") ||
			strings.Contains(strings.ToLower(err.Error()), "exists") {
			return nil
		}
		return fmt.Errorf("CreateIndex: %w", err)
	}
	return nil
}

// SeedDocuments inserts docs into the collection in column form,
// flushes (so the rows are immediately searchable), and loads the collection
// into memory (Milvus requires LoadCollection before Search). Idempotent at
// the collection level via EnsureCollection; NOT idempotent at the row level
// — re-running inserts duplicate IDs (Milvus INT64 PK is not auto-deduped
// unless upsert is used). The spike drops+recreates between runs by deleting
// the collection manually if a clean seed is needed.
func SeedDocuments(ctx context.Context, mc client.Client, cfg MilvusConfig, docs []Document) error {
	cfg = cfg.withDefaults()
	if len(docs) == 0 {
		return errors.New("SeedDocuments: no docs to seed")
	}

	if err := EnsureCollection(ctx, mc, cfg); err != nil {
		return err
	}

	ids := make([]int64, len(docs))
	tiers := make([]string, len(docs))
	contents := make([]string, len(docs))
	sources := make([]string, len(docs))
	vecs := make([][]float32, len(docs))
	for i, d := range docs {
		if len(d.Embedding) != cfg.EmbedDim {
			return fmt.Errorf("doc %s (id=%d) embedding dim=%d, want=%d", d.DocID, d.ID, len(d.Embedding), cfg.EmbedDim)
		}
		ids[i] = d.ID
		tiers[i] = d.Tier
		contents[i] = truncateRunes(d.Content, DefaultContentMaxLen)
		sources[i] = truncateRunes(d.Source, DefaultSourceMaxLen)
		vecs[i] = d.Embedding
	}

	idCol := entity.NewColumnInt64("id", ids)
	tierCol := entity.NewColumnVarChar("tier", tiers)
	contentCol := entity.NewColumnVarChar("content", contents)
	sourceCol := entity.NewColumnVarChar("source", sources)
	vecCol := entity.NewColumnFloatVector("embedding", cfg.EmbedDim, vecs)

	if _, err := mc.Insert(ctx, cfg.CollectionName, "", idCol, tierCol, contentCol, sourceCol, vecCol); err != nil {
		return fmt.Errorf("Insert: %w", err)
	}
	// async=false + flush so the rows are durable + visible to the next Search.
	if err := mc.Flush(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("Flush: %w", err)
	}
	// LoadCollection is required before Search in Milvus v2.x. async=false
	// blocks until the collection is in memory.
	if err := mc.LoadCollection(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("LoadCollection: %w", err)
	}
	return nil
}
