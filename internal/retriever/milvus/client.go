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
// shim, the integration test that calls DropCollection directly).
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
func (m *MilvusClient) Ensure(ctx context.Context) error {
	return EnsureCollection(ctx, m.mc, m.cfg)
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
