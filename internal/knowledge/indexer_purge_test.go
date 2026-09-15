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

// Tests for the tier-scoped reseed purge (sql-guides-duplicate-content,
// 2026-09-15). The bug: knowledge.Index re-inserted the corpus on every
// boot with no cleanup; Milvus does not deduplicate Insert by primary key,
// so stale generations accumulated as searchable entities (2931 physical
// rows for a 256-row corpus). Query collapsed duplicate PKs (graph looked
// clean) while vector Search ranked every generation (duplicate content +
// stale root-path citations).
//
// The fix lives in seedDocs: purge every tier present in the batch
// (milvus.DeleteTierRows) BEFORE the insert. These tests lock the two
// halves the seedDocs hook cannot intercept: the pure tier-set derivation,
// and the delete-before-insert ordering against a mock Milvus client.

import (
	"context"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedTiers_DeduplicatesAndSorts locks the purge-scope derivation: one
// entry per tier regardless of how many docs carry it, ascending. The
// ascending order keeps the Milvus delete expression byte-stable for the
// same doc set.
func TestSeedTiers_DeduplicatesAndSorts(t *testing.T) {
	docs := []retriever.Document{
		{ID: 1, Tier: "sql-guides", Source: "sql-guides/取数口径/a.md"},
		{ID: 2, Tier: "authority", Source: "authority/b.md"},
		{ID: 3, Tier: "sql-guides", Source: "sql-guides/汽车险/c.md"},
		{ID: 4, Tier: "authority", Source: "authority/d.md"},
	}
	got := seedTiers(docs)
	assert.Equal(t, []string{"authority", "sql-guides"}, got,
		"purge scope must dedupe tiers and sort ascending")
}

// TestSeedTiers_EmptyDocs: an empty batch derives an empty scope, which
// DeleteTierRows no-ops on — the purge can never fire without a matching
// reseed behind it.
func TestSeedTiers_EmptyDocs(t *testing.T) {
	assert.Empty(t, seedTiers(nil))
	assert.Empty(t, seedTiers([]retriever.Document{}))
}

// purgeMockClient is the hermetic mock for the seedDocs production path
// (mirrors recreateMockClient's discipline: every unmocked method of the
// embedded nil interface panics instead of silently passing). It records
// the DML sequence so the test can assert delete-before-insert and inspect
// the tier expression.
type purgeMockClient struct {
	client.Client
	calls []string
	expr  string
}

func (m *purgeMockClient) HasCollection(_ context.Context, collName string) (bool, error) {
	m.calls = append(m.calls, "has:"+collName)
	return true, nil
}

func (m *purgeMockClient) LoadCollection(_ context.Context, collName string, _ bool, _ ...client.LoadCollectionOption) error {
	m.calls = append(m.calls, "load:"+collName)
	return nil
}

func (m *purgeMockClient) CreateIndex(_ context.Context, collName string, _ string, _ entity.Index, _ bool, _ ...client.IndexOption) error {
	m.calls = append(m.calls, "index:"+collName)
	return nil
}

func (m *purgeMockClient) Delete(_ context.Context, collName string, _ string, expr string) error {
	m.expr = expr
	m.calls = append(m.calls, "delete:"+collName)
	return nil
}

func (m *purgeMockClient) Flush(_ context.Context, collName string, _ bool, _ ...client.FlushOption) error {
	m.calls = append(m.calls, "flush:"+collName)
	return nil
}

func (m *purgeMockClient) Insert(_ context.Context, collName string, _ string, _ ...entity.Column) (entity.Column, error) {
	m.calls = append(m.calls, "insert:"+collName)
	return nil, nil
}

func (m *purgeMockClient) indexOf(needle string) int {
	for i, c := range m.calls {
		if c == needle {
			return i
		}
	}
	return -1
}

// purgeTestDocs is a batch spanning two tiers (mirrors the live corpus
// shape: authority + sql-guides both active).
func purgeTestDocs() []retriever.Document {
	return []retriever.Document{
		{ID: 1, Tier: "authority", Content: "a", Source: "authority/a.md", Embedding: make([]float32, testEmbedDim)},
		{ID: 2, Tier: "sql-guides", Content: "s", Source: "sql-guides/取数口径/s.md", Embedding: make([]float32, testEmbedDim)},
		{ID: 3, Tier: "sql-guides", Content: "s2", Source: "sql-guides/汽车险/s2.md", Embedding: make([]float32, testEmbedDim)},
	}
}

// TestSeedDocs_PurgesTiersBeforeInsert drives the PRODUCTION seedDocs path
// (no hook bound) against a mock client and locks the replace semantics:
// the tier-scoped delete runs BEFORE the insert, targets exactly the tiers
// in the batch, and the insert lands in the same collection.
//
// This is the regression lock for the duplicate-citations bug: with the
// purge absent, a reshaped corpus stacked generations of rows into Milvus
// and Search surfaced all of them.
func TestSeedDocs_PurgesTiersBeforeInsert(t *testing.T) {
	prev := indexSeedHook
	indexSeedHook = nil // force the production path
	t.Cleanup(func() { indexSeedHook = prev })

	mc := &purgeMockClient{}
	cfg := milvus.Config{CollectionName: "test_collection"}
	require.NoError(t, seedDocs(context.Background(), mc, cfg, purgeTestDocs()))

	deleteAt := mc.indexOf("delete:test_collection")
	insertAt := mc.indexOf("insert:test_collection")
	require.GreaterOrEqual(t, deleteAt, 0, "seedDocs must purge stale tier rows, calls = %v", mc.calls)
	require.GreaterOrEqual(t, insertAt, 0, "seedDocs must insert the batch, calls = %v", mc.calls)
	if deleteAt > insertAt {
		t.Fatalf("purge ran after the insert (delete@%d > insert@%d) — stale generations would survive the reseed; calls = %v",
			deleteAt, insertAt, mc.calls)
	}
	assert.Equal(t, `tier in ["authority","sql-guides"]`, mc.expr,
		"the delete expression must scope exactly to the tiers being reseeded (quoted, ascending)")
	// The flush after the delete makes the purge durable before the insert
	// batch's own flush; both land inside the same production call.
	require.Greater(t, mc.indexOf("flush:test_collection"), deleteAt,
		"the purge must flush, calls = %v", mc.calls)
}

// TestSeedDocs_PurgeFailureAbortsInsert: a purge error must abort the seed
// (never delete-then-half-insert silently) and name the purge phase in the
// error — the boot WARN then reports "knowledge index failed" and retrieval
// refuses (D-09) instead of serving a partially replaced tier.
func TestSeedDocs_PurgeFailureAbortsInsert(t *testing.T) {
	prev := indexSeedHook
	indexSeedHook = nil
	t.Cleanup(func() { indexSeedHook = prev })

	mc := &failingDeleteClient{}
	err := seedDocs(context.Background(), mc, milvus.Config{CollectionName: "c"}, purgeTestDocs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "purge stale tier rows",
		"the error must name the purge phase so the boot log is actionable")
	assert.NotContains(t, mc.calls, "insert:c",
		"no insert may follow a failed purge (a half-replaced tier is worse than an empty one)")
}

// failingDeleteClient fails the Delete call.
type failingDeleteClient struct {
	purgeMockClient
}

func (m *failingDeleteClient) Delete(_ context.Context, _ string, _, _ string) error {
	m.calls = append(m.calls, "delete:c")
	return assert.AnError
}

// TestSeedDocs_HookStillInterceptsWholeOperation: the existing test seam
// replaces purge+insert together — hook-bound callers (every hermetic Index
// test) never touch a backend. This pins backward compatibility of the seam.
func TestSeedDocs_HookStillInterceptsWholeOperation(t *testing.T) {
	mc := &purgeMockClient{}
	cap := withSeedHook(t)
	require.NoError(t, seedDocs(context.Background(), mc, milvus.Config{CollectionName: "c"}, purgeTestDocs()))
	require.Len(t, cap.docs, 3, "the hook must receive the full batch")
	assert.Empty(t, mc.calls, "a hook-bound seedDocs must not touch any backend")
}
