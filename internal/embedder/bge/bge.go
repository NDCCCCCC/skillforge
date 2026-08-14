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

// Package bge is the BGE-M3 HTTP embedder implementation of
// embedder.Embedder. It is the Phase 2 reference impl (the demo Skill
// uses BAAI BGE-M3 for multilingual Chinese/English embedding with
// 8K-token context).
//
// Deterministic is a package-level fallback used ONLY by the regression
// binary (cmd/spiked) when BGE_M3_URL is unreachable. The BGE.Embed
// method does NOT call Deterministic — production refuses on BGE-M3
// outage (Pitfall 8: a hash-based fallback would silently mask a real
// outage with semantically meaningless vectors).
package bge

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
	"time"

	"github.com/skillforge/skillforge/internal/embedder"
)

const (
	defaultEmbedTimeout = 60 * time.Second // generous: Ollama cold-loads the 1.2GB bge-m3 model on first call (internal Volcano/TEI returns <1s, so this is a harmless ceiling)
	embedRetries        = 2
	dimensions          = 1024 // BGE-M3 dense output dimension
)

// Config configures the BGE-M3 HTTP embedder.
type Config struct {
	URL       string        // BGE-M3 REST endpoint (e.g. http://bge-m3.internal/v1/embeddings)
	AuthToken string        // optional bearer token; empty = no auth
	Model     string        // model name for OpenAI-compatible /v1/embeddings (Ollama needs "bge-m3"; vLLM/TEI accept it). Empty OMITS the field (back-compat with endpoints that reject unknown fields).
	Timeout   time.Duration // per-request timeout; zero → defaultEmbedTimeout (60s). Ollama cold-start tolerant.
}

// BGE implements embedder.Embedder against a BGE-M3 HTTP service. Safe
// for concurrent use; the http.DefaultClient handles connection pooling.
type BGE struct {
	cfg Config
}

// Compile-time check that BGE implements embedder.Embedder.
var _ embedder.Embedder = (*BGE)(nil)

// New constructs a BGE-M3 HTTP embedder. Returns an error if URL is
// empty (the caller decides the failure mode; production fails fast
// at startup, the indexer tests skip if BGE is nil).
func New(cfg Config) *BGE {
	return &BGE{cfg: cfg}
}

// Embed calls the BGE-M3 HTTP endpoint for a 1024-dim embedding.
// Request body shape: {"model": "<Model>", "input": "<text>"} when Model is
// set (OpenAI-compatible; required by Ollama), else {"input": "<text>"}
// (the .env.example BGE_M3_URL points at a /v1/embeddings-style endpoint).
// Response is parsed tolerantly: OpenAI shape {"data":[{"embedding":[...]}]}
// is tried first, then a bare-array shape [{"embedding":[...]}]}
// (raw TEI).
//
// Bounded by embedTimeout with embedRetries attempts (1s backoff
// between). Returns an error on ANY failure — production refuses
// on this error (Pitfall 8) and never silently degrades to Deterministic.
func (b *BGE) Embed(ctx context.Context, text string) ([]float32, error) {
	if b.cfg.URL == "" {
		return nil, errors.New("bge.Embed: URL is empty (set BGE_M3_URL)")
	}
	timeout := b.cfg.Timeout
	if timeout <= 0 {
		timeout = defaultEmbedTimeout
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
		vec, err := embedOnce(ctx, b.cfg.URL, b.cfg.AuthToken, b.cfg.Model, timeout, text)
		if err == nil {
			return vec, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("BGE.Embed failed after %d attempts: %w", embedRetries+1, lastErr)
}

// Dimensions returns the output dimensionality (BGE-M3 = 1024).
func (b *BGE) Dimensions() int { return dimensions }

// embedOnce performs a single BGE-M3 POST and parses the response.
func embedOnce(ctx context.Context, url, token, model string, timeout time.Duration, text string) ([]float32, error) {
	type embedReq struct {
		Model string `json:"model,omitempty"`
		Input string `json:"input"`
	}
	body, err := json.Marshal(embedReq{Model: model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if envToken := authTokenFromEnv(); envToken != "" {
		// Fallback to BGE_M3_AUTH env var if not configured inline.
		// Kept for backwards-compatibility with the spike-era caller.
		req.Header.Set("Authorization", "Bearer "+envToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, string(preview))
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

// authTokenFromEnv reads the optional BGE_M3_AUTH bearer token. Kept
// here for backwards-compatibility with the spike-era caller.
func authTokenFromEnv() string {
	return os.Getenv("BGE_M3_AUTH")
}

// Deterministic is the offline BGE-M3 fallback: it derives a 1024-dim
// pseudo-random vector deterministically from the SHA-256 of the input
// text. This is NOT a semantic embedding — vectors carry no meaning —
// but they are stable (same text → same vector) and distinct (different
// text → different vector with overwhelming probability), which is enough
// to exercise the Milvus insert/search/filter pipeline when the BGE-M3
// endpoint is down or the spike is run offline.
//
// SPIKE-ONLY FALLBACK — production refuses on BGE-M3 outage (Pitfall 8).
// This exists solely so cmd/spiked's tier-filter assertion (SC-2) can run
// end-to-end without blocking on embedding-service availability. The
// production verdict pipeline (cmd/server) MUST call BGE.Embed only
// and surface any error as a refusal, never silently degrade to this
// fallback (the moat would fail open: semantically-meaningless vectors
// would still return hits, hiding the outage from the grounding path).
func Deterministic(text string) []float32 {
	sum := sha256.Sum256([]byte(text))
	// Use the first 8 bytes of the digest as a deterministic int64 seed.
	seed := int64(binary.LittleEndian.Uint64(sum[:8]))
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic by design
	vec := make([]float32, dimensions)
	for i := range vec {
		vec[i] = r.Float32()*2 - 1 // [-1, 1)
	}
	return vec
}
