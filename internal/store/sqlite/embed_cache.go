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

// Phase 54 / Plan 54-02 (ING-04 / SC 5) — the SQLite implementation of the
// ingest.EmbedCache narrow seam, backed by the embed_cache DDL in schema.go.
// The composite PK (model, content_hash) is the Dify `Embedding(model_name,
// hash)` uniqueness pattern: a 54-03 step retry (or a re-upload of unchanged
// content) resolves every unchanged chunk's vector here at zero BGE-M3 cost.
//
// Rows are IMMUTABLE cache entries: PutEmbedding is INSERT OR IGNORE (first
// write wins — the same (model, hash) is deterministic, so ignoring a later
// put of the "same" vector preserves the original evidence), and there is no
// UPDATE or DELETE path in this file. Vectors ride as float32
// LITTLE-ENDIAN-packed BLOBs; the codec lives here so the byte order has
// exactly one home and the round-trip test (TestEmbedCacheBlobRoundTrip,
// including a literal-bytes assertion) pins it across machines — a
// backup/restore must decode identically.
package sqlite

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"time"
)

// putEmbedCacheSQL is INSERT OR IGNORE, deliberately NOT OR REPLACE: the
// (model, content_hash) row is deterministic, so a conflicting put would
// carry a logically identical vector — ignoring it keeps the FIRST write's
// provenance (created_at) intact instead of refreshing it on every re-run.
const putEmbedCacheSQL = `
INSERT INTO embed_cache (model, content_hash, dim, vector, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(model, content_hash) DO NOTHING
`

// getEmbedCacheSQL is the exact-key batch lookup; hashes arrive as an IN
// list built by getEmbeddingsPlaceholders. Missing hashes are cache MISSES,
// not errors — the caller embeds them.
const getEmbedCacheSQL = `
SELECT content_hash, dim, vector
FROM embed_cache
WHERE model = ? AND content_hash IN (%s)
`

// embedCacheBatchSize bounds the IN-list length per query (SQLite's
// historical 999-host-parameter floor; the bundled 3.53.4 allows far more,
// but batching keeps the statement under the conservative floor and the
// chunk-per-doc counts are orders of magnitude below it).
const embedCacheBatchSize = 500

// PutEmbedding stores one vector under (model, content_hash). created_at is
// caller-set RFC3339Nano (stored verbatim — the house timestamp discipline).
// A row that already exists under the key is left untouched (immutable
// cache rows — see the file comment).
func (s *SQLiteStore) PutEmbedding(ctx context.Context, model, contentHash string, vec []float32, now time.Time) error {
	if model == "" || contentHash == "" {
		return fmtError("PutEmbedding", errEmptyEmbedCacheKey)
	}
	_, err := s.db.ExecContext(ctx, putEmbedCacheSQL,
		model,
		contentHash,
		len(vec),
		encodeVector(vec),
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("PutEmbedding", err)
	}
	return nil
}

// GetEmbeddings batch-resolves vectors for model across hashes. The
// returned map carries ONLY the hashes found — an absent key is a cache
// miss the caller embeds fresh. The dim column is NOT second-guessed here:
// the cache read path in the ingest embed step compares it against the
// embedder's Dimensions() and fails loudly on drift (a same-key row whose
// dimension no longer matches the deployed model is a stale row, and
// silently inserting it would poison the collection with mixed-dimension
// vectors). A zero-dim row (unreachable via PutEmbedding) is skipped.
func (s *SQLiteStore) GetEmbeddings(ctx context.Context, model string, hashes []string) (map[string][]float32, error) {
	if model == "" {
		return nil, fmtError("GetEmbeddings", errEmptyEmbedCacheKey)
	}
	out := make(map[string][]float32, len(hashes))
	for start := 0; start < len(hashes); start += embedCacheBatchSize {
		end := start + embedCacheBatchSize
		if end > len(hashes) {
			end = len(hashes)
		}
		batch := hashes[start:end]
		sqlStmt := strings.Replace(getEmbedCacheSQL, "%s", getEmbeddingsPlaceholders(len(batch)), 1)
		args := make([]any, 0, len(batch)+1)
		args = append(args, model)
		for _, h := range batch {
			args = append(args, h)
		}
		rows, err := s.db.QueryContext(ctx, sqlStmt, args...)
		if err != nil {
			return nil, fmtError("GetEmbeddings", err)
		}
		for rows.Next() {
			var (
				hash   string
				dim    int
				vector []byte
			)
			if err := rows.Scan(&hash, &dim, &vector); err != nil {
				_ = rows.Close()
				return nil, fmtError("GetEmbeddings.Scan", err)
			}
			if dim <= 0 || len(vector) != dim*4 {
				// Malformed row: treat as a miss (the caller re-embeds and
				// INSERT OR IGNORE keeps the malformed row from being
				// overwritten — the operator cleans it via SQL). Never
				// surface a half-decoded vector.
				continue
			}
			out[hash] = decodeVector(vector)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmtError("GetEmbeddings.Rows", err)
		}
		_ = rows.Close()
	}
	return out, nil
}

// getEmbeddingsPlaceholders renders "?,?,?" for n parameters.
func getEmbeddingsPlaceholders(n int) string {
	if n <= 0 {
		return "NULL" // matches nothing; an empty IN () is a syntax error
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// errEmptyEmbedCacheKey guards against an empty model or hash silently
// partitioning the cache under the "" key (the errEmptySessionID posture).
var errEmptyEmbedCacheKey = errors.New("sqlite: embed cache model/content_hash must be non-empty")

// encodeVector packs a float32 slice as LITTLE-ENDIAN raw bytes
// (math.Float32bits → binary.LittleEndian.PutUint32). Little-endian is
// pinned by TestEmbedCacheBlobRoundTrip's literal-bytes assertion: a
// backup/restored skillforge.db on another architecture must decode
// identically (the file format must not inherit Go's native byte order).
func encodeVector(vec []float32) []byte {
	out := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}

// decodeVector unpacks a BLOB written by encodeVector. The length is
// whatever the BLOB carries — callers validate against the embedder's
// Dimensions() (the dim column is checked at the row level in
// GetEmbeddings).
func decodeVector(blob []byte) []float32 {
	out := make([]float32, len(blob)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out
}
