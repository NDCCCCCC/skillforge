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

// Phase 54 / Plan 54-02 — embed_cache store tests. The BLOB round-trip test
// pins the LITTLE-ENDIAN byte order at the literal-bytes level: a
// backup/restored skillforge.db on another machine must decode identically
// (plan Task 2 test ④).
package sqlite

import (
	"context"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func embedCacheTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

var embedCacheNow = time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)

// TestEmbedCacheBlobRoundTrip — encode→decode restores every float32
// bit-exactly (negative, zero, subnormal-precision, +Inf-adjacent magnitudes
// included), and the packed bytes are verifiably LITTLE-ENDIAN:
// float32 1.0 = 0x3F800000 packs as 00 00 80 3F (a big-endian packer would
// emit 3F 80 00 00).
func TestEmbedCacheBlobRoundTrip(t *testing.T) {
	in := []float32{1.0, -2.5, 0.0, 3.14159265, -1e-8, 1e18, math.SmallestNonzeroFloat32, math.MaxFloat32}
	out := decodeVector(encodeVector(in))
	require.Len(t, out, len(in))
	for i := range in {
		// Bit-exact comparison: math.Float32bits equality (== would also
		// work here since NaN is absent, but bits make the contract explicit).
		assert.Equal(t, math.Float32bits(in[i]), math.Float32bits(out[i]), "index %d", i)
	}

	// Little-endian pin (the plan's cross-machine byte-order requirement):
	// [1.0, -2.5, 0.0] → 00 00 80 3F | 00 00 20 C0 | 00 00 00 00.
	const wantHex = "0000803f000020c000000000"
	assert.Equal(t, wantHex, hex.EncodeToString(encodeVector([]float32{1.0, -2.5, 0.0})))

	// Empty vector round-trips as an empty BLOB (no panic).
	assert.Empty(t, encodeVector(nil))
	assert.Empty(t, decodeVector(nil))
}

// TestEmbedCachePutGetRoundTrip — two rows under one model round-trip; a
// hash that was never put is a MISS (absent from the returned map), and the
// batch lookup is a single call across both hashes.
func TestEmbedCachePutGetRoundTrip(t *testing.T) {
	s := embedCacheTestStore(t)
	ctx := context.Background()

	vecA := []float32{0.1, 0.2, 0.3}
	vecB := []float32{-4, -5}
	require.NoError(t, s.PutEmbedding(ctx, "bge-m3", "hash-a", vecA, embedCacheNow))
	require.NoError(t, s.PutEmbedding(ctx, "bge-m3", "hash-b", vecB, embedCacheNow))

	got, err := s.GetEmbeddings(ctx, "bge-m3", []string{"hash-a", "hash-b", "hash-miss"})
	require.NoError(t, err)
	require.Len(t, got, 2) // hash-miss is a miss, not a nil entry
	assertFloat32Equal(t, vecA, got["hash-a"])
	assertFloat32Equal(t, vecB, got["hash-b"])

	// Empty batch: no rows, no error (the IN-list placeholder guard).
	empty, err := s.GetEmbeddings(ctx, "bge-m3", nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestEmbedCacheModelKeyIsolation — the plan's Task 2 test ②: the cache key
// includes the model, so the same content hash under two models is TWO
// independent rows; switching models must MISS (not return the other
// model's vector — different models are dimensionally incomparable).
func TestEmbedCacheModelKeyIsolation(t *testing.T) {
	s := embedCacheTestStore(t)
	ctx := context.Background()

	vecA := []float32{1, 2, 3, 4}
	vecB := []float32{9, 9}
	require.NoError(t, s.PutEmbedding(ctx, "bge-m3", "same-hash", vecA, embedCacheNow))
	require.NoError(t, s.PutEmbedding(ctx, "other-model", "same-hash", vecB, embedCacheNow))

	gotA, err := s.GetEmbeddings(ctx, "bge-m3", []string{"same-hash"})
	require.NoError(t, err)
	require.Len(t, gotA, 1)
	assertFloat32Equal(t, vecA, gotA["same-hash"])

	gotB, err := s.GetEmbeddings(ctx, "other-model", []string{"same-hash"})
	require.NoError(t, err)
	require.Len(t, gotB, 1)
	assertFloat32Equal(t, vecB, gotB["same-hash"])
}

// TestEmbedCachePutIsIdempotent — immutable cache rows: a second put under
// the same (model, hash) is a no-op (INSERT OR IGNORE), so the FIRST
// vector and its created_at survive (plan Task 2 test ①'s cache-hit path
// must return the original row).
func TestEmbedCachePutIsIdempotent(t *testing.T) {
	s := embedCacheTestStore(t)
	ctx := context.Background()

	first := []float32{1, 1, 1}
	require.NoError(t, s.PutEmbedding(ctx, "bge-m3", "h", first, embedCacheNow))
	require.NoError(t, s.PutEmbedding(ctx, "bge-m3", "h", []float32{7, 7}, embedCacheNow.Add(time.Minute)))

	got, err := s.GetEmbeddings(ctx, "bge-m3", []string{"h"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assertFloat32Equal(t, first, got["h"])

	// created_at keeps the FIRST put (provenance, not last-write).
	var createdAt string
	require.NoError(t, s.DB().QueryRow(
		`SELECT created_at FROM embed_cache WHERE model='bge-m3' AND content_hash='h'`,
	).Scan(&createdAt))
	assert.Equal(t, embedCacheNow.Format(time.RFC3339Nano), createdAt)
}

// TestEmbedCacheGuards — empty model / hash are rejected loudly (the
// errEmptySessionID posture: an "" key would silently partition the cache),
// and a dim-zero row is treated as a miss, never returned as a zero vector.
func TestEmbedCacheGuards(t *testing.T) {
	s := embedCacheTestStore(t)
	ctx := context.Background()

	require.Error(t, s.PutEmbedding(ctx, "", "h", []float32{1}, embedCacheNow))
	require.Error(t, s.PutEmbedding(ctx, "bge-m3", "", []float32{1}, embedCacheNow))
	_, err := s.GetEmbeddings(ctx, "", []string{"h"})
	require.Error(t, err)

	// A dim-0 row (only writable via raw SQL; PutEmbedding always stores
	// len(vec)) is skipped by the read path.
	_, execErr := s.DB().Exec(
		`INSERT INTO embed_cache (model, content_hash, dim, vector, created_at) VALUES ('bge-m3', 'bad', 0, X'', '2026')`)
	require.NoError(t, execErr)
	got, getErr := s.GetEmbeddings(ctx, "bge-m3", []string{"bad"})
	require.NoError(t, getErr)
	assert.Empty(t, got) // miss → the caller re-embeds
}

// assertFloat32Equal compares bit-exactly (NaN is absent from cache vectors,
// but bits keep the contract explicit about float32, not float64).
func assertFloat32Equal(t *testing.T, want, got []float32) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		assert.Equal(t, math.Float32bits(want[i]), math.Float32bits(got[i]), "index %d", i)
	}
}
