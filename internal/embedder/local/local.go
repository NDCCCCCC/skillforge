// Package local implements a fully offline, dependency-free Embedder:
// tokenizer (CJK char-bigram + latin word) + feature hashing + L2
// normalization. No model files, no network, no cgo.
//
// Semantics: this is a sparse-embedding (bag-of-features) model — two
// texts score high cosine similarity when they SHARE FEATURES (words /
// character n-grams), not when they are paraphrases. It is the honest
// middle ground between the SPIKE-ONLY bge.Deterministic (zero semantics,
// production-forbidden) and the external BGE-M3 HTTP endpoint: real
// lexical retrieval, hermetic and deterministic. Switch back to
// EMBEDDER_BACKEND=bge when a semantic model endpoint is available.
//
// Cache-safety contract: Model() MUST stay in lockstep with any change to
// the feature pipeline (tokenizer, hash seeds, weights) — the embedded
// vector cache keys on model+dim, so a pipeline change with the same
// model name would silently serve stale vectors. Version-suffix the name
// instead (local-hash-v1 → local-hash-v2).
package local

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Model is the cache/telemetry identity of THIS EXACT pipeline. Bump the
// version suffix whenever the tokenizer, hash seeds, or weighting change.
const Model = "local-hash-v1"

// DefaultDimensions is the vector width. 512 keeps a full corpus cache
// small (512 × 4 B per doc) while collisions stay acceptable at demo
// scale (≤ a few thousand docs).
const DefaultDimensions = 512

// Config configures the local embedder. Zero value is valid.
type Config struct {
	// Dimensions overrides the vector width (must be > 0 when set).
	Dimensions int
}

// Embedder is the offline feature-hashing embedder.
type Embedder struct {
	dim int
}

// New validates the config and returns an Embedder.
func New(cfg Config) (*Embedder, error) {
	dim := cfg.Dimensions
	if dim == 0 {
		dim = DefaultDimensions
	}
	if dim <= 0 || dim > 65536 {
		return nil, fmt.Errorf("local embedder: Dimensions must be in (0, 65536], got %d", dim)
	}
	return &Embedder{dim: dim}, nil
}

// NewDefault returns an Embedder with DefaultDimensions. Panics only on a
// programming error (impossible for the zero config).
func NewDefault() *Embedder {
	e, err := New(Config{})
	if err != nil {
		panic(fmt.Sprintf("local embedder: NewDefault: %v", err))
	}
	return e
}

// Dimensions implements embedder.Embedder.
func (e *Embedder) Dimensions() int { return e.dim }

// ModelName is the identity stamped into the vector cache key.
func (e *Embedder) ModelName() string { return Model }

// Embed implements embedder.Embedder: tokenize → hashed feature bag →
// sublinear tf weighting → L2 normalization. Deterministic across
// processes and platforms; never fails on input (empty input → zero
// vector, which cosine treats as zero similarity).
func (e *Embedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, e.dim)
	for feat, tf := range tokenize(text) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(feat))
		sum := h.Sum64()
		idx := int(sum%uint64(e.dim))         //nolint:gosec // dim ≤ 65536
		sign := float32(1.0)                  // second hash decides the sign → E[dot]≈0 across unrelated features
		sh := fnv.New64a()
		_, _ = sh.Write([]byte{0x5f})
		_, _ = sh.Write([]byte(feat))
		if sh.Sum64()&1 == 1 {
			sign = -1.0
		}
		vec[idx] += sign * float32(1.0+math.Log(float64(tf)))
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm == 0 {
		return vec, nil
	}
	inv := float32(1.0 / math.Sqrt(norm))
	for i := range vec {
		vec[i] *= inv
	}
	return vec, nil
}

// tokenize splits text into features:
//   - runs of CJK characters → every char (weight via count) + every
//     adjacent char bigram (the main recall driver for Chinese);
//   - runs of latin letters/digits → lowercased word (ASCII fold for the
//     rest to keep it dependency-free);
//   - everything else (punct/whitespace) is a separator.
func tokenize(text string) map[string]int {
	feats := make(map[string]int)
	var latin strings.Builder
	var cjk []rune
	flushLatin := func() {
		if latin.Len() > 0 {
			feats[latin.String()]++
			latin.Reset()
		}
	}
	flushCJK := func() {
		for i, r := range cjk {
			feats[string(r)]++
			if i > 0 {
				feats[string(cjk[i-1])+string(r)]++ // bigram
			}
		}
		cjk = cjk[:0]
	}
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF, r >= 0x3400 && r <= 0x4DBF: // CJK unified
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin.WriteRune(r)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()
	return feats
}
