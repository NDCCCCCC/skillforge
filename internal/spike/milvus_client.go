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

// Package spike wraps the Milvus Go SDK v2 with SkillForge-spike conveniences:
//
//   - BuildMilvusFilter converts a Skill.reads slice into a CN-char-safe,
//     default-deny Milvus filter expression (T-1-04 / T-1-05 mitigation).
//   - NewMilvusClient dials an internal Milvus gRPC endpoint with bounded
//     timeout + exponential backoff retry (internal-api strategy: no Docker).
//   - SeedDocuments creates the spike collection (IVF_FLAT nlist=128, dim=1024)
//     and inserts Document rows in column form.
//   - EmbedQuery calls an internal BGE-M3 endpoint; EmbedQueryDeterministic
//     is the offline SHA-256-seeded fallback used when BGE_M3_URL is
//     unreachable.
//   - SearchWithTierFilter runs a vector search with the filter applied
//     PRE-similarity (Pitfall 3) and returns simplified TierSearchResult rows.
//
// The collection schema is intentionally spike-minimal: id (INT64 PK),
// tier (VARCHAR 20), content (VARCHAR 8192), source (VARCHAR 256),
// embedding (FLOAT_VECTOR 1024). The tier column is the trust boundary:
// the filter expression is the only mechanism that prevents cross-tier
// retrieval, so it is applied at the SDK Search layer, never post-search.
//
// SDK note: milvus-sdk-go v2.4.2's Search uses POSITIONAL arguments
// (expr is the 4th positional param), not a SearchQueryOptions struct. The
// earlier RESEARCH.md draft described a struct-based API that does not
// exist in v2.4.2; this package uses the verified positional API. See
// SUMMARY.md deviation #1.
package spike

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Spike collection / index defaults. The spike collection name is fixed so
// Plan 04's e2e subcommand can reuse the seeded data without re-configuring.
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

// MilvusConfig configures the spike Milvus client. Zero-value fields are
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

// Document represents one seeded markdown fixture row. DocID is the
// human-readable fixture identifier (e.g. "auth_001"); it is NOT persisted
// to Milvus — only the numeric ID is — but is kept on the struct for
// trace logging during seed/search. Source holds the fixture relative path
// and IS persisted so search results can be traced back to a fixture file.
type Document struct {
	DocID     string   // fixture frontmatter id (logging only, not persisted)
	ID        int64    // numeric Milvus primary key (1..N)
	Tier      string   // one of 权威/历史/个人/草稿
	Content   string   // markdown body (post-frontmatter)
	Source    string   // fixture relative path, e.g. "testdata/docs/authority/auth_001.md"
	Embedding []float32
}

// TierSearchResult is a simplified search hit. The raw Milvus SearchResult
// is deliberately not leaked past this package so Plan 04's e2e cmd does
// not import the Milvus SDK directly.
type TierSearchResult struct {
	ID      int64
	Tier    string
	Content string
	Source  string
	Score   float32
}

// BuildMilvusFilter converts a Skill.reads slice (e.g. ["权威"]) into a
// Milvus filter expression applied PRE-similarity at the SDK Search layer.
//
// Empty / nil tiers returns the literal string "false" which Milvus
// evaluates as a never-matching predicate (default-deny per D-09). This is
// the structural enforcement of "refuse when no tier matches": the database
// itself returns zero rows, independent of any agent policy.
//
// CN characters in tier names are escaped via Go's %q formatter (Pitfall 7
// mitigation): %q produces a valid double-quoted string with hostile
// characters (single quotes, double quotes, backslashes) backslash-escaped,
// so the resulting expression is injection-safe for all tier values.
func BuildMilvusFilter(tiers []string) string {
	if len(tiers) == 0 {
		// "false" is a valid Milvus boolean predicate that matches zero rows.
		// Returning "" would be a default-allow (no filter), which violates
		// the default-deny contract — so we explicitly return "false".
		return "false"
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		// %q produces a double-quoted, backslash-escaped Go string literal.
		// For CN chars it yields "权威" verbatim (UTF-8 safe); for hostile
		// chars it yields "O\"Brien" / "a\\b" — both safe inside [ ... ].
		quoted[i] = fmt.Sprintf("%q", t)
	}
	return fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
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

// EnsureCollection creates the spike collection + IVF_FLAT index if they do
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

// SeedDocuments inserts docs into the spike collection in column form,
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

// SearchWithTierFilter runs a vector search with the given filter expression
// (produced by BuildMilvusFilter) applied PRE-similarity at the SDK layer
// (Pitfall 3: never post-filter). topK bounds the number of hits. The query
// vector must be cfg.EmbedDim-dimensional.
//
// The filter is a parameter (not derived inside this function) so callers
// can assert on the exact filter string they built — making the trust
// boundary auditable end-to-end.
func SearchWithTierFilter(ctx context.Context, mc client.Client, cfg MilvusConfig, queryVec []float32, filter string, topK int) ([]TierSearchResult, error) {
	cfg = cfg.withDefaults()
	if len(queryVec) != cfg.EmbedDim {
		return nil, fmt.Errorf("queryVec dim=%d, want=%d", len(queryVec), cfg.EmbedDim)
	}

	sp, err := entity.NewIndexIvfFlatSearchParam(DefaultIVFNProbe)
	if err != nil {
		return nil, fmt.Errorf("NewIndexIvfFlatSearchParam: %w", err)
	}

	// SDK v2.4.2 positional Search API (verified against module source).
	// expr (the filter) is the 4th positional argument and is applied at
	// index-scan time, BEFORE vector similarity ranking.
	results, err := mc.Search(
		ctx,
		cfg.CollectionName,
		nil, // partitions (nil = default)
		filter,
		[]string{"id", "tier", "content", "source"},
		[]entity.Vector{entity.FloatVector(queryVec)},
		"embedding",
		entity.COSINE,
		topK,
		sp,
		client.WithSearchQueryConsistencyLevel(entity.ClStrong),
	)
	if err != nil {
		return nil, fmt.Errorf("Search: %w", err)
	}

	out := make([]TierSearchResult, 0)
	for _, sr := range results {
		if sr.Err != nil {
			return nil, fmt.Errorf("SearchResult error: %w", sr.Err)
		}
		tierCol := sr.Fields.GetColumn("tier")
		contentCol := sr.Fields.GetColumn("content")
		sourceCol := sr.Fields.GetColumn("source")
		for i := 0; i < sr.ResultCount; i++ {
			row := TierSearchResult{Score: -1}
			if i < len(sr.Scores) {
				row.Score = sr.Scores[i]
			}
			if sr.IDs != nil {
				if v, e := sr.IDs.GetAsInt64(i); e == nil {
					row.ID = v
				}
			}
			if tierCol != nil {
				if v, e := tierCol.GetAsString(i); e == nil {
					row.Tier = v
				}
			}
			if contentCol != nil {
				if v, e := contentCol.GetAsString(i); e == nil {
					row.Content = v
				}
			}
			if sourceCol != nil {
				if v, e := sourceCol.GetAsString(i); e == nil {
					row.Source = v
				}
			}
			out = append(out, row)
		}
	}
	return out, nil
}

// EmbedQuery calls the internal BGE-M3 endpoint for a 1024-dim embedding.
// Request body shape: {"input": "<text>"} (OpenAI-/volcano-compatible; the
// .env.example BGE_M3_URL points at a /v1/embeddings-style endpoint). The
// response is parsed tolerantly: OpenAI shape {"data":[{"embedding":[...]}]}
// is tried first, then a bare-array shape [{"embedding":[...]}] (raw TEI).
//
// Bounded by embedTimeout with embedRetries attempts (1s backoff between).
// Returns an error on ANY failure — the caller decides whether to fall back
// to EmbedQueryDeterministic. We do not silently mock here: hiding a
// BGE-M3 outage behind a hash vector would mask a production-incident
// signal during the spike.
func EmbedQuery(ctx context.Context, bgeURL, text string) ([]float32, error) {
	if bgeURL == "" {
		return nil, errors.New("EmbedQuery: bgeURL is empty (set BGE_M3_URL)")
	}

	var lastErr error
	for attempt := 0; attempt < embedRetries+1; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
		vec, err := embedQueryOnce(ctx, bgeURL, text)
		if err == nil {
			return vec, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("EmbedQuery failed after %d attempts: %w", embedRetries+1, lastErr)
}

// embedQueryOnce performs a single BGE-M3 POST and parses the response.
func embedQueryOnce(ctx context.Context, bgeURL, text string) ([]float32, error) {
	body, err := json.Marshal(map[string]string{"input": text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, bgeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := authTokenFromEnv(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", bgeURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("POST %s: status %d: %s", bgeURL, resp.StatusCode, string(preview))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024)) // 8 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Try OpenAI shape first: {"data":[{"embedding":[...]}]}.
	var openaiResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &openaiResp); err == nil && len(openaiResp.Data) > 0 && len(openaiResp.Data[0].Embedding) > 0 {
		return openaiResp.Data[0].Embedding, nil
	}

	// Fall back to bare-array TEI shape: [{"embedding":[...]}].
	var teiResp []struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &teiResp); err == nil && len(teiResp) > 0 && len(teiResp[0].Embedding) > 0 {
		return teiResp[0].Embedding, nil
	}

	return nil, fmt.Errorf("could not parse embedding from response (no 'embedding' array found)")
}

// EmbedQueryDeterministic is the offline BGE-M3 fallback: it derives a
// 1024-dim pseudo-random vector deterministically from the SHA-256 of the
// input text. This is NOT a semantic embedding — vectors carry no meaning —
// but they are stable (same text → same vector) and distinct (different
// text → different vector with overwhelming probability), which is enough
// to exercise the Milvus insert/search/filter pipeline when the BGE-M3
// endpoint is down or the spike is run offline.
//
// Production code MUST NOT use this: it exists solely so the spike's
// tier-filter assertion (SC-2) can run end-to-end without blocking on
// embedding-service availability. See SUMMARY.md.
func EmbedQueryDeterministic(text string) []float32 {
	sum := sha256.Sum256([]byte(text))
	// Use the first 8 bytes of the digest as a deterministic int64 seed.
	seed := int64(binary.LittleEndian.Uint64(sum[:8]))
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic by design
	vec := make([]float32, DefaultEmbedDim)
	for i := range vec {
		vec[i] = r.Float32()*2 - 1 // [-1, 1)
	}
	return vec
}

// authTokenFromEnv reads the optional BGE_M3_AUTH bearer token. Kept in this
// package so EmbedQuery is self-contained for Plan 04 reuse.
func authTokenFromEnv() string {
	return os.Getenv("BGE_M3_AUTH")
}

// truncateRunes caps s to at most max UTF-8 rune count so VARCHAR(max)
// fields cannot overflow on insert. Counts runes (not bytes) so CN text is
// measured correctly.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
