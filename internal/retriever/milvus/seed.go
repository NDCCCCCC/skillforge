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
// unless upsert is used). The spike drops+recreates between runs by deleting
// the collection manually if a clean seed is needed.
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