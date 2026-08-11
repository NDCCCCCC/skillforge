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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test dim matches retriever.DefaultEmbedDim (1024 for BGE-M3). The
// indexer does not assert on vector contents, only on dim via
// retriever.SeedDocuments' own check; the fake server just emits a
// 1024-element zero vector to satisfy that.
const testEmbedDim = 1024

// newFakeBGE spins a stdlib httptest.Server that mimics the OpenAI
// /v1/embeddings response shape. Every input returns a 1024-dim zero
// vector (semantically meaningless, but enough to exercise the indexer
// pipeline). The URL is returned so callers can pass it to retriever.EmbedQuery.
func newFakeBGE(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	vec := make([]float32, testEmbedDim)
	body := buildFakeEmbeddingResponse(vec)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	return srv, srv.URL
}

// buildFakeEmbeddingResponse renders a fake BGE-M3 OpenAI-shape response
// for a 1024-dim vector. The vector elements are all 0.0 — fine for the
// indexer tests which never evaluate semantic similarity.
func buildFakeEmbeddingResponse(vec []float32) []byte {
	var b strings.Builder
	b.WriteString(`{"data":[{"embedding":[`)
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		if v == 0 {
			b.WriteString("0")
		} else {
			b.WriteString("0.0001") // non-zero sentinel, not meaningful
		}
	}
	b.WriteString(`]}]}`)
	return []byte(b.String())
}

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
	indexSeedHook = func(_ context.Context, _ client.Client, _ retriever.MilvusConfig, docs []retriever.Document) error {
		// Copy so later assertions don't see downstream mutation.
		cap.docs = make([]retriever.Document, len(docs))
		copy(cap.docs, docs)
		return nil
	}
	t.Cleanup(func() { indexSeedHook = prev })
	return cap
}

// driveIndex invokes Index with a nil Milvus client (the seedDocs hook
// intercepts before the SDK is touched). knowledgeDir + bgeURL come
// from the caller.
func driveIndex(t *testing.T, knowledgeDir, bgeURL string) error {
	t.Helper()
	return Index(context.Background(), knowledgeDir, nil, retriever.MilvusConfig{CollectionName: "test_collection"}, bgeURL)
}

// TestIndexer_EmptyTierDirsOK asserts the SC-4 mechanism: an empty
// knowledge/ tree with only authority/*.md (1 file) + empty personal/
// historical/ draft/ indexes exactly 1 document with tier=权威, no
// failure from the empty dirs.
func TestIndexer_EmptyTierDirsOK(t *testing.T) {
	dir := makeKnowledgeDir(t)
	writeDoc(t, dir, "authority", "policy.md", "出差住宿费超过 500 元时须上级审批。")

	srv, bgeURL := newFakeBGE(t)
	defer srv.Close()

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, bgeURL))

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

	srv, bgeURL := newFakeBGE(t)
	defer srv.Close()

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, bgeURL))

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

	srv, bgeURL := newFakeBGE(t)
	defer srv.Close()

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, bgeURL))

	require.Len(t, cap.docs, 1)
	assert.Equal(t, "actual content here", cap.docs[0].Content,
		"YAML frontmatter must be stripped before embedding")
}

// TestIndexer_ProductionUsesEmbedQueryNotDeterministic is the static-
// analysis guarantee that production code never calls the spike-only
// EmbedQueryDeterministic fallback (Pitfall 8). If a future contributor
// adds that call by accident, this test breaks the build.
//
// We grep for the function-call form "EmbedQueryDeterministic(" (with the
// open paren) rather than the bare identifier — this allows the comments
// in indexer.go to NAME the function (so future readers know what to
// avoid) without tripping the guard.
func TestIndexer_ProductionUsesEmbedQueryNotDeterministic(t *testing.T) {
	data, err := os.ReadFile("indexer.go")
	require.NoError(t, err)
	src := string(data)
	assert.NotContains(t, src, "EmbedQueryDeterministic(",
		"internal/knowledge/indexer.go must not CALL retriever.EmbedQueryDeterministic — "+
			"that fallback is spike-only (Pitfall 8 / T-2-25); production refuses on BGE-M3 outage")
	assert.Contains(t, src, "EmbedQuery(",
		"internal/knowledge/indexer.go must CALL retriever.EmbedQuery for production embedding")
}

// TestIndexer_RejectsOversizeDocs asserts the defensive cap. A body
// over maxDocBytes triggers the skip path; only the small doc is seeded.
func TestIndexer_RejectsOversizeDocs(t *testing.T) {
	dir := makeKnowledgeDir(t)
	small := "短文档。\n"
	big := strings.Repeat("X", maxDocBytes+10) // 10 bytes over the cap
	writeDoc(t, dir, "authority", "small.md", small)
	writeDoc(t, dir, "authority", "huge.md", big)

	srv, bgeURL := newFakeBGE(t)
	defer srv.Close()

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, bgeURL))

	require.Len(t, cap.docs, 1, "expected only the small doc to be seeded; oversize doc must be skipped")
	assert.Equal(t, "small.md", filepath.Base(cap.docs[0].Source))
}

// TestIndexer_EmptyKnowledgeTreeOK asserts an empty knowledge/ root
// produces no error and no docs — the pipeline will then return
// no_authorized_source for every query, which is a valid (if sad)
// steady state.
func TestIndexer_EmptyKnowledgeTreeOK(t *testing.T) {
	dir := makeKnowledgeDir(t) // dirs exist but no .md files

	srv, bgeURL := newFakeBGE(t)
	defer srv.Close()

	cap := withSeedHook(t)
	require.NoError(t, driveIndex(t, dir, bgeURL))
	assert.Empty(t, cap.docs, "empty knowledge tree should produce 0 docs (no error)")
}

// TestIndexer_RejectsEmptyInputs guards the two hard-error entry
// conditions: empty knowledgeDir → error; empty bgeURL → error
// (Pitfall 8: we MUST fail closed, never silently fall back).
func TestIndexer_RejectsEmptyInputs(t *testing.T) {
	ctx := context.Background()
	err := Index(ctx, "", nil, retriever.MilvusConfig{}, "http://bge")
	assert.Error(t, err, "empty knowledgeDir must error")

	err = Index(ctx, "knowledge", nil, retriever.MilvusConfig{}, "")
	assert.Error(t, err, "empty bgeURL must error (Pitfall 8)")
}

// TestStripFrontmatter covers the helper directly with a few shapes.
func TestStripFrontmatter(t *testing.T) {
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
			got := string(stripFrontmatter([]byte(tc.raw)))
			assert.Equal(t, tc.want, got)
		})
	}
}