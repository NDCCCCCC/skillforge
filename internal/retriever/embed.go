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

package retriever

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
)

// EmbedQuery calls the internal BGE-M3 endpoint for a 1024-dim embedding.
// Request body shape: {"input": "<text>"} (OpenAI-/volcano-compatible; the
// .env.example BGE_M3_URL points at a /v1/embeddings-style endpoint). The
// response is parsed tolerantly: OpenAI shape {"data":[{"embedding":[...]}]}
// is tried first, then a bare-array shape [{"embedding":[...]}] (raw TEI).
//
// Bounded by embedTimeout with embedRetries attempts (1s backoff between).
// Returns an error on ANY failure — the caller decides whether to fall back
// to EmbedQueryDeterministic (spike regression only). Production (02-03
// cmd/server) MUST refuse on this error and never fall back (Pitfall 8 —
// hiding a BGE-M3 outage behind a hash vector would mask a production
// incident + return semantically meaningless results).
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
// SPIKE-ONLY FALLBACK — production refuses on BGE-M3 outage (Pitfall 8).
// This exists solely so cmd/spiked's tier-filter assertion (SC-2) can run
// end-to-end without blocking on embedding-service availability. The
// production verdict pipeline (02-03 cmd/server) MUST call EmbedQuery only
// and surface any error as a refusal, never silently degrade to this
// fallback (the moat would fail open: semantically-meaningless vectors
// would still return hits, hiding the outage from the grounding path).
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
// package so EmbedQuery is self-contained for cmd/spiked + 02-03 reuse.
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
