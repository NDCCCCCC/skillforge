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

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test dim matches milvus.DefaultEmbedDim (1024 for BGE-M3). The indexer
// does not assert on vector contents, only on dim via milvus.SeedDocuments'
// own check; the fake embedder just emits a 1024-element zero vector.
const testEmbedDim = 1024

// fakeEmbedder is a minimal embedder.Embedder that returns a 1024-dim
// zero vector for every input. Used by the indexer tests as the
// production-shaped Embedder.
type fakeEmbedder struct {
	dim   int
	calls []string
	mu    sync.Mutex
}

func newFakeEmbedder() *fakeEmbedder {
	return &fakeEmbedder{dim: testEmbedDim}
}

func (e *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, text)
	v := make([]float32, e.dim)
	return v, nil
}

func (e *fakeEmbedder) Dimensions() int { return e.dim }

// makeKnowledgeDir creates a temp dir shaped like the project knowledge/
// root with the four known tier subdirectories. Returns the dir path;
// t.TempDir handles cleanup.
func makeKnowledgeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, tier := range knownTierDirs {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, tier), 0o755))
	}
	return dir
}

// writeDoc writes body to <dir>/<tier>/<name>.md, creating the tier dir
// if necessary. Returns the absolute path of the written file.
func writeDoc(t *testing.T, dir, tier, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, tier, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

// indexSeedCapture holds the docs the test wants to assert on. The
// indexer is driven by setting indexSeedHook to a closure that captures
// the docs slice into this variable; tests then read captured after
// Index returns.
type indexSeedCapture struct {
	docs []retriever.Document
}

// withSeedHook sets indexSeedHook for the duration of one test. It is
// safe to call from multiple tests because the cleanup is per-test.
func withSeedHook(t *testing.T) *indexSeedCapture {
	t.Helper()
	cap := &indexSeedCapture{}
	prev := indexSeedHook
	indexSeedHook = func(_ context.Context, _ client.Client, _ milvus.Config, docs []retriever.Document) error {
		// Copy so later assertions don't see downstream mutation.
		cap.docs = make([]retriever.Document, len(docs))
		copy(cap.docs, docs)
		return nil
	}
	t.Cleanup(func() { indexSeedHook = prev })
	return cap
}

// driveIndex invokes Index with a nil Milvus client (the seedDocs hook
// intercepts before the SDK is touched). knowledgeDir + emb come from
// the caller.
func driveIndex(t *testing.T, knowledgeDir string, emb embedder.Embedder) error {
	t.Helper()
	return Index(context.Background(), knowledgeDir, emb, nil, milvus.Config{CollectionName: "test_collection"})
}

// TestIndexer_EmptyTierDirsOK asserts the SC-4 mechanism: an empty
// knowledge/ tree with only authority/*.md (1 file) + empty personal/
// historical/ draft/ indexes exactly 1 document with tier=权威, no
// failure from the empty dirs.
func TestIndexer_EmptyTierDirsOK(t *testing.T) {
	dir := makeKnowledgeDir(t)
	writeDoc(t, dir, "authority", "policy.md", "出差住宿费超过 500 元时须上级审批。")

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder()))

	require.Len(t, cap.docs, 1, "expected exactly 1 doc (1 authority file)")
	assert.Equal(t, "权威", cap.docs[0].Tier)
	assert.Equal(t, "authority/policy.md", cap.docs[0].Source)
	assert.Equal(t, int64(1), cap.docs[0].ID)
}

// TestIndexer_TierMetadataCorrect asserts dir-name → CN-char tier value:
// 2 files in authority/ + 1 in historical/ → seeded docs have tier=权威
// for the first 2, tier=历史 for the third (alphabetical order: authority
// first, then historical).
func TestIndexer_TierMetadataCorrect(t *testing.T) {
	dir := makeKnowledgeDir(t)
	writeDoc(t, dir, "authority", "alpha.md", "A 类城市差旅住宿费 800 元。")
	writeDoc(t, dir, "authority", "bravo.md", "B 类城市差旅住宿费 600 元。")
	writeDoc(t, dir, "historical", "legacy.md", "旧版制度（仅供参考）：差旅实报实销。")

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder()))

	require.Len(t, cap.docs, 3, "expected 3 docs (2 authority + 1 historical)")
	assert.Equal(t, "权威", cap.docs[0].Tier, "doc 1 should be tier=权威")
	assert.Equal(t, "权威", cap.docs[1].Tier, "doc 2 should be tier=权威")
	assert.Equal(t, "历史", cap.docs[2].Tier, "doc 3 should be tier=历史")
	assert.Equal(t, "authority/alpha.md", cap.docs[0].Source)
	assert.Equal(t, "authority/bravo.md", cap.docs[1].Source)
	assert.Equal(t, "historical/legacy.md", cap.docs[2].Source)
}

// TestIndexer_StripFrontmatter asserts YAML frontmatter is removed from
// the embedded body — `---\nkey: val\n---\nactual content` produces a
// Document with Content = "actual content".
func TestIndexer_StripFrontmatter(t *testing.T) {
	dir := makeKnowledgeDir(t)
	body := "---\nkey: val\nnote: hello\n---\nactual content here"
	writeDoc(t, dir, "authority", "frontmatter.md", body)

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder()))

	require.Len(t, cap.docs, 1)
	assert.Equal(t, "actual content here", cap.docs[0].Content,
		"YAML frontmatter must be stripped before embedding")
}

// TestIndexer_ProductionUsesEmbedderInterface is the static-analysis
// guarantee that production code uses the embedder.Embedder interface
// (and never falls back to a deterministic offline vector). If a future
// contributor switches to a deterministic fallback, this test breaks the
// build.
//
// We grep for the function-call form "EmbedQueryDeterministic(" (with
// the open paren) rather than the bare identifier — this allows the
// comments in indexer.go to NAME the function (so future readers know
// what to avoid) without tripping the guard. The test also asserts that
// the indexer calls Embed(...) (the interface method) directly.
func TestIndexer_ProductionUsesEmbedderInterface(t *testing.T) {
	data, err := os.ReadFile("indexer.go")
	require.NoError(t, err)
	src := string(data)
	assert.NotContains(t, src, "EmbedQueryDeterministic(",
		"internal/knowledge/indexer.go must not CALL a deterministic fallback — "+
			"that fallback is spike-only (Pitfall 8 / T-2-25); production refuses on BGE-M3 outage")
	assert.Contains(t, src, "emb.Embed(",
		"internal/knowledge/indexer.go must CALL emb.Embed (the Embedder interface) for production embedding")
}

// TestIndexer_RejectsOversizeDocs asserts the defensive cap. A body
// over maxDocBytes triggers the skip path; only the small doc is seeded.
func TestIndexer_RejectsOversizeDocs(t *testing.T) {
	dir := makeKnowledgeDir(t)
	small := "短文档。\n"
	big := strings.Repeat("X", maxDocBytes+10) // 10 bytes over the cap
	writeDoc(t, dir, "authority", "small.md", small)
	writeDoc(t, dir, "authority", "huge.md", big)

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder()))

	require.Len(t, cap.docs, 1, "expected only the small doc to be seeded; oversize doc must be skipped")
	assert.Equal(t, "small.md", filepath.Base(cap.docs[0].Source))
}

// TestIndexer_EmptyKnowledgeTreeOK asserts an empty knowledge/ root
// produces no error and no docs — the pipeline will then return
// no_authorized_source for every query, which is a valid (if sad)
// steady state.
func TestIndexer_EmptyKnowledgeTreeOK(t *testing.T) {
	dir := makeKnowledgeDir(t) // dirs exist but no .md files

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, newFakeEmbedder()))
	assert.Empty(t, cap.docs, "empty knowledge tree should produce 0 docs (no error)")
}

// TestIndexer_RejectsEmptyInputs guards the two hard-error entry
// conditions: empty knowledgeDir → error; nil embedder → error
// (Pitfall 8: we MUST fail closed, never silently fall back).
func TestIndexer_RejectsEmptyInputs(t *testing.T) {
	ctx := context.Background()
	err := Index(ctx, "", newFakeEmbedder(), nil, milvus.Config{})
	assert.Error(t, err, "empty knowledgeDir must error")

	err = Index(ctx, "knowledge", nil, nil, milvus.Config{})
	assert.Error(t, err, "nil embedder must error (Pitfall 8)")
}

// TestSplitFrontmatter covers the helper directly with a few shapes.
// Plan 05-02 D5-15: splitFrontmatter returns (body, frontmatter) so
// callers can extract effective_from / effective_to. Tests assert on
// the body only; the frontmatter return is covered indirectly by
// TestParseEffectiveWindow.
func TestSplitFrontmatter(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"no_frontmatter_passthrough", "plain markdown body", "plain markdown body"},
		{"empty_input", "", ""},
		{"with_frontmatter_stripped", "---\nkey: val\n---\nbody", "body"},
		{"unterminated_frontmatter_passthrough", "---\nkey: val\nbody", "---\nkey: val\nbody"},
		{"crlf_normalized", "---\r\nkey: val\r\n---\r\nbody", "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := splitFrontmatter([]byte(tc.raw))
			got := string(body)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestParseEffectiveWindow — Plan 05-02 D5-15. The helper reads
// effective_from / effective_to (epoch seconds) from a YAML frontmatter
// block. Cases:
//
//   - nil / empty frontmatter → (0, 0) (sentinel).
//   - frontmatter with effective_* → parsed values.
//   - malformed YAML → (0, 0) + warning (don't brick the indexer).
func TestParseEffectiveWindow(t *testing.T) {
	t.Run("nil_returns_perpetual_sentinel", func(t *testing.T) {
		from, to := parseEffectiveWindow(nil)
		assert.Equal(t, int64(0), from)
		assert.Equal(t, int64(0), to)
	})
	t.Run("empty_returns_perpetual_sentinel", func(t *testing.T) {
		from, to := parseEffectiveWindow([]byte{})
		assert.Equal(t, int64(0), from)
		assert.Equal(t, int64(0), to)
	})
	t.Run("parses_bounded_window", func(t *testing.T) {
		fm := []byte("effective_from: 1699000000\neffective_to: 1700000000\n")
		from, to := parseEffectiveWindow(fm)
		assert.Equal(t, int64(1699000000), from)
		assert.Equal(t, int64(1700000000), to)
	})
	t.Run("malformed_yaml_falls_back_to_perpetual", func(t *testing.T) {
		fm := []byte("not: valid: yaml: at all:") // unbalanced
		from, to := parseEffectiveWindow(fm)
		assert.Equal(t, int64(0), from)
		assert.Equal(t, int64(0), to,
		"malformed YAML must not brick the indexer (D5-15 fail-soft)")
	})
}

// TestIndexer_ValidateEffective — Plan 05-02 D5-15. KNOW-02 extension:
// validateEffective rejects documents with invalid effective_to windows
// at index time (fail-fast, before any backend write). Cases:
//
//	(a) effective_to == 0          → ok (perpetual sentinel).
//	(b) effective_to < 0           → error (negative epoch nonsense).
//	(c) both nonzero AND effective_to < effective_from → error (inverted window).
//	(d) one side unset (from=0, to!=0) → ok (open-ended-from is valid).
func TestIndexer_ValidateEffective(t *testing.T) {
	const now int64 = 1700000000
	cases := []struct {
		name    string
		doc     retriever.Document
		wantErr bool
	}{
		{
			name:    "perpetual_sentinel_ok",
			doc:     retriever.Document{Tier: "权威", Source: "authority/a.md", EffectiveFrom: 0, EffectiveTo: 0},
			wantErr: false,
		},
		{
			name:    "negative_effective_to_errors",
			doc:     retriever.Document{Tier: "权威", Source: "authority/b.md", EffectiveFrom: 0, EffectiveTo: -1},
			wantErr: true,
		},
		{
			name:    "inverted_window_errors",
			doc:     retriever.Document{Tier: "权威", Source: "authority/c.md", EffectiveFrom: now, EffectiveTo: now - 1},
			wantErr: true,
		},
		{
			name:    "one_side_unset_ok",
			doc:     retriever.Document{Tier: "权威", Source: "authority/d.md", EffectiveFrom: 0, EffectiveTo: now},
			wantErr: false,
		},
		{
			name:    "valid_bounded_window_ok",
			doc:     retriever.Document{Tier: "权威", Source: "authority/e.md", EffectiveFrom: now - 100, EffectiveTo: now + 100},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEffective([]retriever.Document{tc.doc})
			if tc.wantErr {
				assert.Error(t, err, "expected error for %s", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for %s", tc.name)
			}
		})
	}
}

// TestIndexer_ProductionWiresValidateEffective — static guard. The
// production Index path MUST call validateEffective after building the
// docs slice (before seedDocs / any backend write). A future refactor
// that drops the call would silently let invalid effective windows
// through to the corpus.
func TestIndexer_ProductionWiresValidateEffective(t *testing.T) {
	data, err := os.ReadFile("indexer.go")
	require.NoError(t, err)
	src := string(data)
	assert.Contains(t, src, "validateEffective(",
		"internal/knowledge/indexer.go MUST call validateEffective on the built docs slice (Plan 05-02 D5-15)")
}