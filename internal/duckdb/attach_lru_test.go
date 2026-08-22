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

package duckdb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createDatasetFile materializes a tiny persistent .duckdb dataset file and
// returns its path. Each test gets its own file so ATTACH aliases never
// collide across tests.
func createDatasetFile(t *testing.T, name string) string {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), name+".duckdb")
	c, err := New(ctx, Config{DSN: path})
	require.NoError(t, err)
	_, err = c.DB().ExecContext(ctx, "CREATE TABLE marker AS SELECT '"+name+"' AS tag")
	require.NoError(t, err)
	require.NoError(t, c.Close())
	return path
}

// newTestLRU builds an AttachLRU over a fresh :memory: connector with the
// supplied cap. The connector (and its dedicated LRU connection) are closed
// on test cleanup.
func newTestLRU(t *testing.T, max int) *AttachLRU {
	t.Helper()
	c, err := New(context.Background(), Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Close()) })

	lru, err := NewAttachLRU(c, max)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, lru.Close()) })
	return lru
}

// TestAttachLRUCap is the 10-lock series lock #9a: attaching cap+1 datasets
// DETACHes the least-recently-used one. The DETACH must be observable in the
// engine (duckdb_databases()), not just in the bookkeeping map — proving the
// eviction actually released the file handle (Windows lock boundary, T-14-17).
func TestAttachLRUCap(t *testing.T) {
	ctx := context.Background()
	lru := newTestLRU(t, 3)

	ids := []string{"ds_cap_a", "ds_cap_b", "ds_cap_c"}
	paths := make(map[string]string, 4)
	for _, id := range ids {
		paths[id] = createDatasetFile(t, id)
		_, err := lru.Get(ctx, id, paths[id])
		require.NoError(t, err)
	}

	// Fourth attach must evict ds_cap_a (least recently used).
	paths["ds_cap_d"] = createDatasetFile(t, "ds_cap_d")
	_, err := lru.Get(ctx, "ds_cap_d", paths["ds_cap_d"])
	require.NoError(t, err)

	assert.Equal(t, 3, lru.Len(), "LRU must stay at cap after eviction")

	attached, err := lru.AttachedIDs(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"ds_cap_b", "ds_cap_c", "ds_cap_d"},
		attached,
		"least-recently-used dataset must be DETACHed at cap")
}

// TestAttachLRUTouchReorder is lock #9b: a hit must re-touch the entry so a
// subsequent attach evicts a DIFFERENT (now-least-recently-used) dataset.
func TestAttachLRUTouchReorder(t *testing.T) {
	ctx := context.Background()
	lru := newTestLRU(t, 3)

	ids := []string{"ds_touch_a", "ds_touch_b", "ds_touch_c"}
	for _, id := range ids {
		p := createDatasetFile(t, id)
		_, err := lru.Get(ctx, id, p)
		require.NoError(t, err)
	}

	// Re-touch ds_touch_a (a hit — no new attach).
	hit, err := lru.Get(ctx, "ds_touch_a", "unused-on-hit")
	require.NoError(t, err)
	assert.True(t, hit, "second Get on an attached id must report a hit")

	// ds_touch_b is now the LRU entry; attaching D evicts B, not A.
	p := createDatasetFile(t, "ds_touch_d")
	_, err = lru.Get(ctx, "ds_touch_d", p)
	require.NoError(t, err)

	attached, err := lru.AttachedIDs(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"ds_touch_a", "ds_touch_c", "ds_touch_d"},
		attached,
		"re-touched entry must survive; the untouched LRU entry must be evicted")
}

// TestAttachLRUInvalidID is lock #9c: an invalid dataset identifier is
// rejected before any ATTACH executes (T-14-12).
func TestAttachLRUInvalidID(t *testing.T) {
	ctx := context.Background()
	lru := newTestLRU(t, 3)

	_, err := lru.Get(ctx, "../escape", "data/datasets/x.duckdb")
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidDatasetID)

	assert.Equal(t, 0, lru.Len(), "invalid id must not attach anything")
	attached, err := lru.AttachedIDs(ctx)
	require.NoError(t, err)
	assert.Empty(t, attached, "engine catalog must be untouched by a rejected id")
}

// TestAttachLRUDetachIdempotent is lock #9d: Detach on a not-attached id is a
// safe no-op, and double-Detach after a real detach also succeeds — the TTL
// cleanup path (Phase 15/19) relies on this (T-14-17).
func TestAttachLRUDetachIdempotent(t *testing.T) {
	ctx := context.Background()
	lru := newTestLRU(t, 3)

	// Not attached: no-op, no error.
	require.NoError(t, lru.Detach(ctx, "never_attached"))

	p := createDatasetFile(t, "ds_det_a")
	_, err := lru.Get(ctx, "ds_det_a", p)
	require.NoError(t, err)

	require.NoError(t, lru.Detach(ctx, "ds_det_a"))
	require.NoError(t, lru.Detach(ctx, "ds_det_a"), "second Detach on the same id must be a no-op")

	assert.Equal(t, 0, lru.Len())
	attached, err := lru.AttachedIDs(ctx)
	require.NoError(t, err)
	assert.Empty(t, attached, "explicit Detach must release the engine-side handle")
}
