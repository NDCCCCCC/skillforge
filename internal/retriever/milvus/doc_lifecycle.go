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
	"fmt"
	"strconv"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
)

// doc_lifecycle.go implements retriever.DocChunkStore on MilvusClient
// (Phase 53-03 / D-53-03) — the (tier, doc_id)-addressed primitives behind
// the upload upsert and the DELETE /api/admin/kb/:slug/docs/:doc_id
// endpoint.
//
// The delete-half is the D-53-03 contract verbatim: upsert = "delete the
// doc's old rows by expr, then insert the new generation" — it must NEVER
// degrade to bare append. Milvus does not deduplicate Insert by primary
// key, so re-inserting a row whose PK still exists would stack a second
// searchable generation (the 2931/256 write-amplification incident
// class). Delete-then-insert is therefore ordered strictly before the
// caller's Insert, and Milvus's asynchronously-visible deletes are an
// accepted transient: a brief window where old and new rows coexist is
// tolerable (53-CONTEXT) — Query collapses duplicate PKs so the graph
// never shows the doubling, and the flush inside DeleteByExpr minimizes
// the window.

// Compile-time proof that *MilvusClient satisfies the doc-lifecycle
// capability interface — drift surfaces here, next to the implementation,
// not at a distant type-assertion site.
var _ retriever.DocChunkStore = (*MilvusClient)(nil)

// docIDExpr builds the injection-safe (tier, doc_id) expression both
// primitives filter on. Both values are interpolated with strconv.Quote
// (the filter.go / DeleteTierRows precedent — T-39-01 mitigation): doc_id
// originates from corpus paths and 53-01 persist naming, tier from the
// registry slug whitelist, but the escaping duty stands regardless.
//
// The two clauses are ANDed so a doc_id collision across tiers can never
// leak rows between knowledge bases — the tier clause is the trust
// boundary (SYSK-07) and stays load-bearing on the lifecycle path too.
func docIDExpr(tier, docID string) string {
	return "tier == " + strconv.Quote(tier) + " and doc_id == " + strconv.Quote(docID)
}

// DocChunkRows returns every row of one (tier, doc_id) as the
// retriever.DocChunk projection: PK, chunk_index, content_hash, and the
// stored embedding. The embedding is the load-bearing field — it is what
// lets the upsert re-insert an UNCHANGED chunk with its original PK AND
// original vector, skipping the BGE-M3 call, while still rewriting the
// row's chunk_index for the new sequence (retriever.Document.ChunkIndex
// godoc).
//
// Postures:
//   - a missing collection is an empty result with nil error (fresh
//     deploy: the first upload has nothing to reuse — the same
//     missing-collection no-op contract as DeleteTierRows/DeleteByExpr);
//   - a doc_id with no rows returns (empty slice, nil);
//   - a window-filling result set fails loudly (the scanMaxID WR-02
//     truncation guard: an under-reported doc row set would silently drop
//     old rows from the reuse/delete decision).
func (m *MilvusClient) DocChunkRows(ctx context.Context, tier, docID string) ([]retriever.DocChunk, error) {
	if m == nil || m.cfg.CollectionName == "" {
		return nil, fmt.Errorf("milvus.DocChunkRows: client or collection name is empty")
	}
	has, err := m.mc.HasCollection(ctx, m.cfg.CollectionName)
	if err != nil {
		return nil, fmt.Errorf("milvus.DocChunkRows: HasCollection(%q): %w", m.cfg.CollectionName, err)
	}
	if !has {
		// Fresh deploy: nothing to reuse or delete; the caller's Insert
		// (via SeedDocuments' EnsureCollection) is the only creator.
		return nil, nil
	}
	expr := docIDExpr(tier, docID)
	rs, err := m.mc.Query(
		ctx,
		m.cfg.CollectionName,
		nil, // partitions (nil = default)
		expr,
		// embedding IS requested here (unlike ListMetadata, which stays
		// lean): the upsert re-inserts unchanged chunks with their
		// original vectors, so it must read them back in the same
		// round-trip that hashes the rows.
		[]string{"id", "chunk_index", "content_hash", "embedding"},
		client.WithLimit(maxIDQueryWindow),
	)
	if err != nil {
		return nil, fmt.Errorf("milvus.DocChunkRows: Query(%s): %w", expr, err)
	}

	var idCol, chunkIdxCol, hashCol, vecCol entity.Column
	for _, c := range rs {
		switch c.Name() {
		case "id":
			idCol = c
		case "chunk_index":
			chunkIdxCol = c
		case "content_hash":
			hashCol = c
		case "embedding":
			vecCol = c
		}
	}
	if idCol == nil {
		// No rows matched the (tier, doc_id) — first upload of this doc.
		return nil, nil
	}
	n := idCol.Len()
	if n >= maxIDQueryWindow {
		// WR-02 analog: a window-filling result set may hide rows beyond
		// it — the upsert would decide reuse/delete over an incomplete
		// view of the doc. Fail loudly and name the remedy instead.
		return nil, fmt.Errorf(
			"milvus.DocChunkRows: result set filled the %d-row Query window for doc_id %q — rows may exist beyond it (purge dead generations or compact the collection before re-uploading)",
			maxIDQueryWindow, docID)
	}

	out := make([]retriever.DocChunk, 0, n)
	for i := 0; i < n; i++ {
		row := retriever.DocChunk{}
		if v, e := idCol.GetAsInt64(i); e == nil {
			row.ID = v
		}
		if chunkIdxCol != nil {
			if v, e := chunkIdxCol.GetAsInt64(i); e == nil {
				row.ChunkIndex = int32(v)
			}
		}
		if hashCol != nil {
			if v, e := hashCol.GetAsString(i); e == nil {
				row.ContentHash = v
			}
		}
		if vecCol != nil {
			// entity.Column.Get returns the untyped column value; for a
			// FLOAT_VECTOR column that is a []float32 slice. A type
			// mismatch leaves the embedding nil, and SeedDocuments
			// rejects nil-embedding rows loudly (dim != cfg.EmbedDim) —
			// a corrupted vector column can never silently poison the
			// upsert.
			if raw, e := vecCol.Get(i); e == nil {
				if vec, ok := raw.([]float32); ok {
					row.Embedding = vec
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// DeleteDocRows deletes EVERY row of one (tier, doc_id) — the upsert's
// delete-half (always paired with the caller's Insert of the replacement
// generation) and the DELETE endpoint's row purge.
//
// Delegation goes through DeleteByExpr, which owns the full mechanics:
// missing-collection no-op, LoadCollection-before-Delete (DML needs a
// loaded collection), Flush-after-Delete (tombstones durable + visible
// immediately). This wrapper contributes exactly one thing: the
// injection-safe, tier-scoped expression — the caller-facing signature
// takes raw strings and must never let them reach Milvus unescaped.
func (m *MilvusClient) DeleteDocRows(ctx context.Context, tier, docID string) error {
	return DeleteByExpr(ctx, m.mc, m.cfg, docIDExpr(tier, docID))
}
