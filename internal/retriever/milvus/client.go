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

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
)

// MilvusClient implements retriever.Retriever against a Milvus v2.4.2
// gRPC server. It owns the underlying milvus-sdk-go client and exposes
// the tier-filter-isolated surface the rest of SkillForge consumes.
type MilvusClient struct {
	mc  client.Client
	cfg Config
}

// Compile-time check that *MilvusClient satisfies retriever.Retriever.
// Defense-in-depth so future Retriever drift surfaces HERE (next to the
// type decl) rather than at a distant call site. The mongo backend has
// the same assertion; this matches it.
var _ retriever.Retriever = (*MilvusClient)(nil)

// New dials Milvus, runs a GetVersion health probe, and returns a
// connected MilvusClient. cfg.Addr is required; other zero-value fields
// are replaced with package defaults.
func New(ctx context.Context, cfg Config) (*MilvusClient, error) {
	cfg = cfg.withDefaults()
	mc, err := dialMilvus(ctx, cfg.Addr)
	if err != nil {
		return nil, err
	}
	return &MilvusClient{mc: mc, cfg: cfg}, nil
}

// Wrap wraps an already-connected milvus-sdk-go client into a
// MilvusClient so callers (tests, the legacy NewMilvusClient shim) can
// reuse an existing connection without re-dialing.
func Wrap(mc client.Client, cfg Config) *MilvusClient {
	return &MilvusClient{mc: mc, cfg: cfg.withDefaults()}
}

// ClientOf returns the underlying milvus-sdk-go client. Used by
// callers that need the raw SDK handle (the legacy NewMilvusClient
// shim, and the integration test that clears the collection directly).
// Returns nil after Disconnect.
func (m *MilvusClient) ClientOf() client.Client {
	return m.mc
}

// Health probes the live connection (GetVersion).
func (m *MilvusClient) Health(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	if _, err := m.mc.GetVersion(probeCtx); err != nil {
		return err
	}
	return nil
}

// Disconnect closes the underlying client. The instance is unusable
// after this call.
func (m *MilvusClient) Disconnect() error {
	if m.mc == nil {
		return nil
	}
	return m.mc.Close()
}

// Ensure creates the collection + IVF_FLAT index if absent. Idempotent.
// This is the routine boot path — it NEVER drops anything (the only
// destructive path in this package is RecreateCollection behind the
// SKILLFORGE_MILVUS_RECREATE gate).
func (m *MilvusClient) Ensure(ctx context.Context) error {
	return EnsureCollection(ctx, m.mc, m.cfg)
}

// RecreateCollection drops the named collection and re-creates it with
// the package schema + IVF_FLAT index — the idempotent drop → ensure
// sequence behind the ONE-SHOT re-seed gate (SKILLFORGE_MILVUS_RECREATE,
// plan 26-09 / Pitfall 1a).
//
// Why this exists: a deployment that switches its knowledge-base identity
// (26-01's slug vocabulary) inherits a vector library whose `tier` rows
// still carry the legacy values. Those rows match no registry slug, so
// every tier-filtered search returns zero hits and every query is refused
// with no_authorized_source — a silent, total outage that looks like a
// safe empty result (Pitfall 1's 灯下黑). tier is row-level metadata, so
// the rows cannot be updated in place by the indexer; the only clean exit
// is to drop the collection and let the next boot re-embed the corpus
// (minutes at demo scale, per 26-RESEARCH's Runtime State Inventory).
//
// This is therefore a CONFIG-DRIVEN, operator-opt-in migration door, not
// a boot behavior: buildRetriever reads the env gate before Ensure, and
// the gate opens only on the literal "1". The empty, freshly created
// collection is deliberately the safe resting state — the boot-time
// knowledge.Index that follows re-seeds it from knowledge/.
//
// An empty collection name is rejected before any call: the drop target
// must be explicit, never a silent default.
func (m *MilvusClient) RecreateCollection(ctx context.Context, collection string) error {
	if collection == "" {
		return fmt.Errorf("milvus.RecreateCollection: collection name is empty (name the collection to drop explicitly)")
	}
	if err := m.mc.DropCollection(ctx, collection); err != nil {
		return fmt.Errorf("milvus.RecreateCollection: DropCollection(%q): %w", collection, err)
	}
	cfg := m.cfg
	cfg.CollectionName = collection
	if err := EnsureCollection(ctx, m.mc, cfg); err != nil {
		return fmt.Errorf("milvus.RecreateCollection: EnsureCollection(%q): %w", collection, err)
	}
	return nil
}

// Insert seeds docs into the collection via the package-level helper.
func (m *MilvusClient) Insert(ctx context.Context, docs []retriever.Document) error {
	return SeedDocuments(ctx, m.mc, m.cfg, docs)
}

// ListMetadata returns the metadata projection of every row in the
// collection (Plan 05-05 / SC-5 / KNOW-06 — graph browser data source).
// No tier filter, no effective_to PRE-filter (the graph deliberately
// surfaces superseded chunks with the 已失效 badge per UI-SPEC §SC-8).
//
// Implementation note: the milvus-sdk-go v2.4.2 Query API returns a
// ResultSet ([]entity.Column). We iterate the columns by Name() and
// pull each row's metadata. embedding + content are NOT loaded; the
// graph endpoint never carries verbatim chunk text into the browser
// (threat T-5-22 mitigation).
func (m *MilvusClient) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	rs, err := m.mc.Query(
		ctx,
		m.cfg.CollectionName,
		nil, // partitions (nil = default)
		"",  // empty expr matches all rows
		[]string{"id", "tier", "source", "effective_from", "effective_to"},
		// Milvus REQUIRES an explicit limit when the expression is empty
		// ("empty expression should be used with limit: invalid parameter"
		// — live failure against Milvus 2.4). 16384 is the server's default
		// query max window (queryNode segcore topKMixRatio window), so this
		// is the most rows a Query can return anyway — the graph endpoint
		// gets the full corpus for any dev/demo-scale knowledge tree.
		client.WithLimit(16384),
	)
	if err != nil {
		return nil, fmt.Errorf("milvus.ListMetadata: Query: %w", err)
	}

	var idCol, tierCol, sourceCol, efCol, etCol entity.Column
	for _, c := range rs {
		switch c.Name() {
		case "id":
			idCol = c
		case "tier":
			tierCol = c
		case "source":
			sourceCol = c
		case "effective_from":
			efCol = c
		case "effective_to":
			etCol = c
		}
	}
	if idCol == nil {
		// Empty collection — no rows; the graph builder produces an
		// empty tier tree (still 4 tier entries, 0 docs each per
		// UI-SPEC §SC-5).
		return nil, nil
	}

	n := idCol.Len()
	out := make([]retriever.DocumentMeta, 0, n)
	for i := 0; i < n; i++ {
		row := retriever.DocumentMeta{}
		if v, e := idCol.GetAsInt64(i); e == nil {
			row.ID = v
		}
		if tierCol != nil {
			if v, e := tierCol.GetAsString(i); e == nil {
				row.Tier = v
			}
		}
		if sourceCol != nil {
			if v, e := sourceCol.GetAsString(i); e == nil {
				row.Source = v
			}
		}
		if efCol != nil {
			if v, e := efCol.GetAsInt64(i); e == nil {
				row.EffectiveFrom = v
			}
		}
		if etCol != nil {
			if v, e := etCol.GetAsInt64(i); e == nil {
				row.EffectiveTo = v
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// GetChunk looks up a single chunk by numeric primary key and returns
// its verbatim content + metadata. Plan 260813-dno / UAT #4 — the
// on-demand chunk-content fetch backing GET /api/knowledge/chunk/:id.
// The graph payload stays metadata-only (T-5-22); this is the separate
// fetch path the FE fires when the operator clicks a graph chunk.
//
// Implementation mirrors ListMetadata: a Query-by-expression (NOT a
// vector Search) against id == <id>. The id is int64 so the %d format
// is injection-safe (no raw URL param string-concatenation). Empty
// idCol or zero rows → ErrChunkNotFound; any other SDK error is wrapped
// and surfaced to the server layer (→ 500).
//
// NOTE: milvus is runtime-broken per D11 (image configured for external
// etcd). This only needs to COMPILE and mirror the same SDK Query shape
// as ListMetadata — the by-id semantics are pinned by the mongo tests.
func (m *MilvusClient) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	rs, err := m.mc.Query(
		ctx,
		m.cfg.CollectionName,
		nil, // partitions (nil = default)
		fmt.Sprintf("id == %d", id),
		[]string{"id", "tier", "content", "source", "effective_from", "effective_to"},
	)
	if err != nil {
		return retriever.ChunkContent{}, fmt.Errorf("milvus.GetChunk: Query: %w", err)
	}

	var idCol, tierCol, contentCol, sourceCol, efCol, etCol entity.Column
	for _, c := range rs {
		switch c.Name() {
		case "id":
			idCol = c
		case "tier":
			tierCol = c
		case "content":
			contentCol = c
		case "source":
			sourceCol = c
		case "effective_from":
			efCol = c
		case "effective_to":
			etCol = c
		}
	}
	if idCol == nil || idCol.Len() == 0 {
		// No row carries this id — soft miss, not a backend error.
		return retriever.ChunkContent{}, retriever.ErrChunkNotFound
	}

	row := retriever.ChunkContent{}
	if v, e := idCol.GetAsInt64(0); e == nil {
		row.ID = v
	}
	if tierCol != nil {
		if v, e := tierCol.GetAsString(0); e == nil {
			row.Tier = v
		}
	}
	if contentCol != nil {
		if v, e := contentCol.GetAsString(0); e == nil {
			row.Content = v
		}
	}
	if sourceCol != nil {
		if v, e := sourceCol.GetAsString(0); e == nil {
			row.Source = v
		}
	}
	if efCol != nil {
		if v, e := efCol.GetAsInt64(0); e == nil {
			row.EffectiveFrom = v
		}
	}
	if etCol != nil {
		if v, e := etCol.GetAsInt64(0); e == nil {
			row.EffectiveTo = v
		}
	}
	return row, nil
}
