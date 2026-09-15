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

package milvus

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
)

// EnsureCollection creates the collection + IVF_FLAT index if they do
// not already exist. Idempotent: safe to call on every seed run. Schema:
//
//	id            INT64  primary key (auto-id OFF — caller assigns numeric IDs)
//	tier          VARCHAR(20)
//	content       VARCHAR(8192)
//	source        VARCHAR(256)
//	embedding     FLOAT_VECTOR(1024)
//	effective_from INT64   Plan 05-02 D5-15 — policy-version window start
//	effective_to   INT64   Plan 05-02 D5-15 — policy-version window end (0 = perpetual)
//
// Plan 05-02 D5-15: the two effective_* INT64 columns use sentinel-0
// (NOT nullable). The default value 0 at the schema layer is the
// explicit "no expiry / perpetual" state. Milvus INT64 columns are
// non-nullable by default — the SDK's entity.Field type has no
// WithNullable method (verified against milvus-sdk-go v2.4.2), so the
// schema is structurally non-nullable. NULL-evaluates-FALSE is
// therefore not a concern here.
//
// The IVF_FLAT index (nlist=128) is built with COSINE metric, the standard
// for BGE-M3 dense embeddings (BGE-M3 outputs are meaningful under cosine
// similarity). The metric choice does not affect the tier-filter bleed
// assertion — that is enforced by the filter expression, not the metric.
func EnsureCollection(ctx context.Context, mc client.Client, cfg Config) error {
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
		WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(cfg.EmbedDim))).
		// Plan 05-02 D5-15 — sentinel-0 design (Milvus INT64 is
		// non-nullable by default). Default value 0 = "perpetually valid"
		// (the legacy / open-ended state).
		WithField(entity.NewField().WithName("effective_from").WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName("effective_to").WithDataType(entity.FieldTypeInt64))

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
func createIndexIfAbsent(ctx context.Context, mc client.Client, cfg Config) error {
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
// unless upsert is used; each Insert's rows become additional entities, and
// Search ranks ALL of them while Query collapses duplicate PKs to the newest
// row — the exact asymmetry behind the sql-guides duplicate-citations bug).
//
// Because of that, full-corpus reseeds must NOT call this function bare:
// knowledge.Index routes through the tier-scoped DeleteTierRows purge first
// (see knowledge.seedDocs). The incremental upload path (server upload.go →
// IndexerBackend.Insert) intentionally keeps calling this function bare —
// it writes a SUBSET of a tier and must never delete the tier's other rows.
// The spike drops+recreates between runs by deleting the collection manually
// if a clean seed is needed.
func SeedDocuments(ctx context.Context, mc client.Client, cfg Config, docs []retriever.Document) error {
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
	effectiveFrom := make([]int64, len(docs))
	effectiveTo := make([]int64, len(docs))
	for i, d := range docs {
		if len(d.Embedding) != cfg.EmbedDim {
			return fmt.Errorf("doc (id=%d) embedding dim=%d, want=%d", d.ID, len(d.Embedding), cfg.EmbedDim)
		}
		ids[i] = d.ID
		tiers[i] = d.Tier
		contents[i] = truncateRunes(d.Content, DefaultContentMaxLen)
		sources[i] = truncateRunes(d.Source, DefaultSourceMaxLen)
		vecs[i] = d.Embedding
		effectiveFrom[i] = d.EffectiveFrom
		effectiveTo[i] = d.EffectiveTo
	}

	idCol := entity.NewColumnInt64("id", ids)
	tierCol := entity.NewColumnVarChar("tier", tiers)
	contentCol := entity.NewColumnVarChar("content", contents)
	sourceCol := entity.NewColumnVarChar("source", sources)
	vecCol := entity.NewColumnFloatVector("embedding", cfg.EmbedDim, vecs)
	effectiveFromCol := entity.NewColumnInt64("effective_from", effectiveFrom)
	effectiveToCol := entity.NewColumnInt64("effective_to", effectiveTo)

	if _, err := mc.Insert(ctx, cfg.CollectionName, "", idCol, tierCol, contentCol, sourceCol, vecCol, effectiveFromCol, effectiveToCol); err != nil {
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

// DeleteTierRows deletes EVERY row whose tier is one of tiers — the
// tier-scoped purge that makes a boot-time reseed a replace instead of an
// append. Milvus does not deduplicate Insert by primary key: every reseed
// of an evolving corpus (file added, path reshaped, file removed) leaves
// the previous generation's rows behind as searchable entities. The
// live failure this fixes (sql-guides-duplicate-content, 2026-09-15): the
// skillforge_knowledge collection held 2931 physical rows for a 256-row
// corpus — Query collapsed duplicate PKs (so /api/knowledge/graph looked
// clean) while vector Search ranked all generations, returning the same
// content twice with stale root-path source citations.
//
// Scope rule — the caller must pass exactly the tiers it is about to
// (re)seed: the delete is intentionally narrow so a purge can never wipe
// a tier that is not immediately rewritten by the caller's next Insert.
// Deactivated bases' rows are NOT touched here (KBM-02
// deactivate-never-destroy; the graph builder skips their stale rows by
// design). A tier whose corpus has been deleted from disk entirely
// (zero files left) produces no docs and therefore no purge — its stale
// rows persist until a RecreateCollection re-seed; that edge is out of
// scope for this fix.
//
// Callers:
//   - knowledge.Index (the full-corpus boot reseed) — via the seedDocs
//     seam, ALWAYS paired with a following SeedDocuments of the same tiers.
//   - NOBODY else. The incremental upload path (server upload.go →
//     IndexerBackend.Insert → SeedDocuments) writes a subset of a tier and
//     must NOT purge the tier's remaining rows.
//
// Mechanics: a missing collection is a no-op (fresh deploy has nothing to
// purge); DML requires a loaded collection, so LoadCollection runs before
// the Delete; the flush afterwards makes the tombstones durable and visible
// immediately instead of at the next flush boundary (the paired
// SeedDocuments flushes again after its Insert — the delete is ordered
// before the insert by Milvus MVCC timestamps either way).
func DeleteTierRows(ctx context.Context, mc client.Client, cfg Config, tiers []string) error {
	cfg = cfg.withDefaults()
	if len(tiers) == 0 {
		return nil
	}
	if mc == nil {
		// Unreachable in production (knowledge.Index rejects a nil client
		// unless the test seed hook is bound) — mirrors how SeedDocuments
		// would nil-deref loudly. Kept as an explicit guard so the error
		// names the purge, not a panic frame.
		return errors.New("milvus.DeleteTierRows: milvus client is nil")
	}
	has, err := mc.HasCollection(ctx, cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("milvus.DeleteTierRows: HasCollection(%q): %w", cfg.CollectionName, err)
	}
	if !has {
		// Fresh deploy: the collection does not exist yet, so there is
		// nothing stale to purge (the paired SeedDocuments will create it).
		return nil
	}
	if err := mc.LoadCollection(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("milvus.DeleteTierRows: LoadCollection(%q): %w", cfg.CollectionName, err)
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		quoted[i] = strconv.Quote(t)
	}
	expr := "tier in [" + strings.Join(quoted, ",") + "]"
	if err := mc.Delete(ctx, cfg.CollectionName, "", expr); err != nil {
		return fmt.Errorf("milvus.DeleteTierRows: Delete(%q, %s): %w", cfg.CollectionName, expr, err)
	}
	if err := mc.Flush(ctx, cfg.CollectionName, false); err != nil {
		return fmt.Errorf("milvus.DeleteTierRows: Flush(%q): %w", cfg.CollectionName, err)
	}
	return nil
}
