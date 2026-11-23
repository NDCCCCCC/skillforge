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
	"errors"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder is a test embedder that returns a configurable fixed vector
// or a configurable error.
type mockEmbedder struct {
	EmbedFunc func(ctx context.Context, text string) ([]float32, error)
	Dim       int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

func (m *mockEmbedder) Dimensions() int {
	if m.Dim != 0 {
		return m.Dim
	}
	return 4
}

// mockRetriever is a hermetic test double for retriever.Retriever.
// All methods are configurable function fields; nil = no-op / zero value.
// It also implements retriever.DocChunkStore (Phase 53-03): the upsert
// type-asserts the capability and fails closed without it, so the mock
// carries it with configurable function fields (nil DocChunkRowsFunc =
// "doc has no existing rows").
type mockRetriever struct {
	HealthFunc        func(ctx context.Context) error
	SearchFunc        func(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error)
	EnsureFunc        func(ctx context.Context) error
	InsertFunc        func(ctx context.Context, docs []retriever.Document) error
	ListMetadataFunc  func(ctx context.Context) ([]retriever.DocumentMeta, error)
	GetChunkFunc      func(ctx context.Context, id int64) (retriever.ChunkContent, error)
	GetMaxIDFunc      func(ctx context.Context, tier string) (int64, error)
	DisconnectFunc    func() error
	DocChunkRowsFunc  func(ctx context.Context, tier, docID string) ([]retriever.DocChunk, error)
	DeleteDocRowsFunc func(ctx context.Context, tier, docID string) error
}

func (m *mockRetriever) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

func (m *mockRetriever) Search(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, queryVec, tiers, topK, now)
	}
	return nil, nil
}

func (m *mockRetriever) Ensure(ctx context.Context) error {
	if m.EnsureFunc != nil {
		return m.EnsureFunc(ctx)
	}
	return nil
}

func (m *mockRetriever) Insert(ctx context.Context, docs []retriever.Document) error {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, docs)
	}
	return nil
}

func (m *mockRetriever) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	if m.ListMetadataFunc != nil {
		return m.ListMetadataFunc(ctx)
	}
	return nil, nil
}

func (m *mockRetriever) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	if m.GetChunkFunc != nil {
		return m.GetChunkFunc(ctx, id)
	}
	return retriever.ChunkContent{}, retriever.ErrChunkNotFound
}

func (m *mockRetriever) GetMaxID(ctx context.Context, tier string) (int64, error) {
	if m.GetMaxIDFunc != nil {
		return m.GetMaxIDFunc(ctx, tier)
	}
	return 0, nil
}

func (m *mockRetriever) Disconnect() error {
	if m.DisconnectFunc != nil {
		return m.DisconnectFunc()
	}
	return nil
}

// retriever.DocChunkStore (Phase 53-03) — the upsert capability. Nil
// DocChunkRowsFunc = the doc has no existing rows (first upload); nil
// DeleteDocRowsFunc = the delete is a no-op.
func (m *mockRetriever) DocChunkRows(ctx context.Context, tier, docID string) ([]retriever.DocChunk, error) {
	if m.DocChunkRowsFunc != nil {
		return m.DocChunkRowsFunc(ctx, tier, docID)
	}
	return nil, nil
}

func (m *mockRetriever) DeleteDocRows(ctx context.Context, tier, docID string) error {
	if m.DeleteDocRowsFunc != nil {
		return m.DeleteDocRowsFunc(ctx, tier, docID)
	}
	return nil
}

// compile-time proof that mockRetriever satisfies retriever.Retriever.
var _ retriever.Retriever = (*mockRetriever)(nil)

// TestUploadIndexProducesValidDocuments verifies that a plain markdown file
// with no frontmatter is parsed, chunked, and returned as a single document
// with correct Tier, Source, and Embedding fields.
func TestUploadIndexProducesValidDocuments(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{}

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", "authority/test.md", me, reg, mr)

	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, "authority", docs[0].Tier)
	assert.Equal(t, "test.md", docs[0].Source)
	assert.Equal(t, "Hello world", docs[0].Content)
	assert.NotNil(t, docs[0].Embedding)
	assert.Len(t, docs[0].Embedding, 4)
	assert.Equal(t, int64(1), docs[0].ID)
}

// TestUploadIndexMDWithFrontmatter verifies that YAML frontmatter is stripped
// from the body and that EffectiveFrom/EffectiveTo are parsed correctly.
func TestUploadIndexMDWithFrontmatter(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{}

	// Frontmatter sets effective_from to 2024-01-01 00:00:00 UTC = 1704067200
	md := `---
effective_from: 1704067200
effective_to: 1735689600
---
# Heading

This is the body content.
`
	docs, err := UploadIndex(context.Background(), "authority", []byte(md), "text/markdown", "policy.md", "authority/policy.md", me, reg, mr)

	require.NoError(t, err)
	require.Len(t, docs, 1)
	// Body must not contain frontmatter markers.
	assert.NotContains(t, docs[0].Content, "---")
	assert.NotContains(t, docs[0].Content, "effective_from")
	assert.Equal(t, int64(1704067200), docs[0].EffectiveFrom)
	assert.Equal(t, int64(1735689600), docs[0].EffectiveTo)
	// ID starts at 1 when GetMaxID returns 0 (default).
	assert.Equal(t, int64(1), docs[0].ID)
}

// TestUploadIndexNextIDCoordination verifies that the mock Retriever's
// GetMaxID value flows through to the returned document IDs correctly.
// When GetMaxID returns 47, the first document should have ID 48.
func TestUploadIndexNextIDCoordination(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{
		GetMaxIDFunc: func(ctx context.Context, tier string) (int64, error) {
			return 47, nil
		},
	}
	me := &mockEmbedder{}

	docs, err := UploadIndex(context.Background(), "authority", []byte("Chunk one"), "text/markdown", "test.md", "authority/test.md", me, reg, mr)

	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, int64(48), docs[0].ID, "first doc ID should be GetMaxID+1")
}

// TestUploadIndexDeactivatedSlugRejection verifies that a deactivated slug
// is rejected before any I/O (fail-closed).
func TestUploadIndexDeactivatedSlugRejection(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: false},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{}

	_, err := UploadIndex(context.Background(), "authority", []byte("Hello"), "text/markdown", "test.md", "authority/test.md", me, reg, mr)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deactivated")
	// Verify GetMaxID was NOT called (fail-closed before I/O).
	assert.Nil(t, mr.GetMaxIDFunc, "GetMaxID must not be called for deactivated slug")
}

// TestUploadIndexUnknownSlugRejection verifies that an unknown slug is
// rejected before any I/O (fail-closed).
func TestUploadIndexUnknownSlugRejection(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{}

	_, err := UploadIndex(context.Background(), "nonexistent", []byte("Hello"), "text/markdown", "test.md", "nonexistent/test.md", me, reg, mr)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
	// Verify GetMaxID was NOT called (fail-closed before I/O).
	assert.Nil(t, mr.GetMaxIDFunc, "GetMaxID must not be called for unknown slug")
}

// TestUploadIndexEmbedFailureFailsClosed verifies that an embedder error
// propagates as "embed failed: ..." and returns no documents.
func TestUploadIndexEmbedFailureFailsClosed(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return nil, errors.New("BGE-M3 unreachable")
		},
	}

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", "authority/test.md", me, reg, mr)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "embed failed")
	assert.Contains(t, err.Error(), "BGE-M3 unreachable")
	assert.Nil(t, docs)
}

// TestUploadIndexParseFailureFailsClosed verifies that a parse error
// propagates as "parse failed: ..." and returns no documents.
func TestUploadIndexParseFailureFailsClosed(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{}
	me := &mockEmbedder{}

	// application/pdf parser rejects non-PDF bytes.
	_, err := UploadIndex(context.Background(), "authority", []byte("not a pdf"), "application/pdf", "evil.pdf", "authority/evil.pdf", me, reg, mr)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse failed")
}

// TestUploadIndexChunkingOversizedDoc verifies that a document exceeding
// maxDocBytes (4000) is split into multiple chunks, each with a valid ID
// and content length <= maxDocBytes.
func TestUploadIndexChunkingOversizedDoc(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	mr := &mockRetriever{
		GetMaxIDFunc: func(ctx context.Context, tier string) (int64, error) {
			return 0, nil
		},
	}
	me := &mockEmbedder{}

	// Build a body larger than chunkSizeRunes (3500 runes). 53-02: the
	// upload path uses the SAME recursive splitter as Index; a separator-
	// starved body converges through the splitter's per-rune fallback, so
	// every chunk must fit the rune budget.
	const oversizedRunes = 9000
	largeBody := make([]byte, oversizedRunes)
	for i := range largeBody {
		largeBody[i] = 'x'
	}

	docs, err := UploadIndex(context.Background(), "authority", largeBody, "text/markdown", "large.md", "authority/large.md", me, reg, mr)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(docs), 2, "oversized doc should produce multiple chunks")
	joined := ""
	for i, doc := range docs {
		assert.LessOrEqual(t, utf8.RuneCountInString(doc.Content), chunkSizeRunes,
			"chunk %d exceeds the shared rune budget (chunkSizeRunes=%d)", i, chunkSizeRunes)
		joined += doc.Content
	}
	assert.Equal(t, oversizedRunes, utf8.RuneCountInString(joined), "chunks must cover the body with no loss")
	// IDs must be sequential starting from 1.
	for i, doc := range docs {
		assert.Equal(t, int64(i+1), doc.ID, "chunk %d should have sequential ID", i)
	}
}

// TestValidateSlugEmpty verifies that an empty slug is rejected.
func TestValidateSlugEmpty(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})

	err := validateSlug("", reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is empty")
}

// TestValidateSlugActive verifies that an active slug passes validation.
func TestValidateSlugActive(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "history", DisplayName: "History", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})

	err := validateSlug("authority", reg)
	require.NoError(t, err)

	err = validateSlug("history", reg)
	require.NoError(t, err)
}

// TestGetUploadNextIDGetMaxIDZero verifies that GetMaxID returning 0
// (no existing chunks) produces startID=1.
func TestGetUploadNextIDGetMaxIDZero(t *testing.T) {
	mr := &mockRetriever{
		GetMaxIDFunc: func(ctx context.Context, tier string) (int64, error) {
			return 0, nil
		},
	}

	startID, err := getUploadNextID(context.Background(), "authority", mr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), startID)
}

// TestGetUploadNextIDGetMaxIDError verifies that a GetMaxID error
// propagates correctly.
func TestGetUploadNextIDGetMaxIDError(t *testing.T) {
	mr := &mockRetriever{
		GetMaxIDFunc: func(ctx context.Context, tier string) (int64, error) {
			return 0, errors.New("Milvus disconnected")
		},
	}

	_, err := getUploadNextID(context.Background(), "authority", mr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get max id for slug")
	assert.Contains(t, err.Error(), "Milvus disconnected")
}

// TestUploadIndexValidateTiersBeforeEmbed verifies that validateTiers is
// called and fails before the embedder is invoked.
func TestUploadIndexValidateTiersBeforeEmbed(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})

	var embedCalls int
	me := &mockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			embedCalls++
			return []float32{0.1}, nil
		},
	}

	// Inject a custom validateTiersHook that fails — this lets us
	// verify the hook is called before embed.
	hookCalled := false
	oldHook := validateTiersHook
	validateTiersHook = func(docs []retriever.Document, r kb.Registry) error {
		hookCalled = true
		return errors.New("tier validation failed")
	}
	defer func() { validateTiersHook = oldHook }()

	_, err := UploadIndex(context.Background(), "authority", []byte("Hello"), "text/markdown", "test.md", "authority/test.md", me, reg, &mockRetriever{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tier validation failed")
	assert.True(t, hookCalled, "validateTiers must be called before embedder")
	assert.Equal(t, 0, embedCalls, "embedder must not be called when validateTiers fails")
}

// ---- concurrent safety ----

// TestUploadIndexConcurrent calls UploadIndex concurrently with different
// slugs to verify there is no shared mutable state in the function itself.
func TestUploadIndexConcurrent(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "history", DisplayName: "History", Role: kb.RoleHistorical, RetrievalEnabled: true},
	})

	var mu sync.Mutex
	embedCalls := 0

	me := &mockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			mu.Lock()
			embedCalls++
			mu.Unlock()
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			slug := []string{"authority", "history"}[idx%2]
			_, err := UploadIndex(context.Background(), slug, []byte("content"), "text/markdown", "test.md", slug+"/test.md", me, reg, &mockRetriever{})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, 10, embedCalls)
	mu.Unlock()
}

// ---- Phase 53-03 (D-53-03): doc-level upsert ----

// fakeDocStore simulates the backend row set for upsert tests: rows keyed
// by PK with tier+docID tags, a PK watermark GetMaxID reads, and a
// deletion log. Insert doubles as "the caller's Insert" so a test drives
// the full upload → insert → re-upload cycle hermetically.
type fakeDocStore struct {
	mu      sync.Mutex
	rows    map[int64]retriever.Document
	maxID   int64
	deleted []string
}

func newFakeDocStore() *fakeDocStore {
	return &fakeDocStore{rows: map[int64]retriever.Document{}}
}

// insert simulates the caller's IndexerBackend.Insert after UploadIndex
// returns.
func (f *fakeDocStore) insert(docs []retriever.Document) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range docs {
		f.rows[d.ID] = d
		if d.ID > f.maxID {
			f.maxID = d.ID
		}
	}
}

func (f *fakeDocStore) Health(_ context.Context) error { return nil }
func (f *fakeDocStore) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (f *fakeDocStore) Ensure(_ context.Context) error { return nil }
func (f *fakeDocStore) Insert(_ context.Context, docs []retriever.Document) error {
	f.insert(docs)
	return nil
}
func (f *fakeDocStore) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (f *fakeDocStore) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, retriever.ErrChunkNotFound
}
func (f *fakeDocStore) GetMaxID(_ context.Context, _ string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxID, nil
}
func (f *fakeDocStore) Disconnect() error { return nil }

func (f *fakeDocStore) DocChunkRows(_ context.Context, tier, docID string) ([]retriever.DocChunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []retriever.DocChunk
	for _, d := range f.rows {
		if d.Tier == tier && d.DocID == docID {
			out = append(out, retriever.DocChunk{
				ID:          d.ID,
				ChunkIndex:  d.ChunkIndex,
				ContentHash: d.ContentHash,
				Embedding:   d.Embedding,
			})
		}
	}
	return out, nil
}

func (f *fakeDocStore) DeleteDocRows(_ context.Context, tier, docID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, tier+"/"+docID)
	for id, d := range f.rows {
		if d.Tier == tier && d.DocID == docID {
			delete(f.rows, id)
		}
	}
	return nil
}

func (f *fakeDocStore) deleteLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

func (f *fakeDocStore) docRows(t *testing.T, tier, docID string) []retriever.DocChunk {
	t.Helper()
	rows, err := f.DocChunkRows(t.Context(), tier, docID)
	require.NoError(t, err)
	return rows
}

// countingEmbedder wraps mockEmbedder with a per-text call log.
type countingEmbedder struct {
	mockEmbedder
	mu    sync.Mutex
	texts []string
}

func (e *countingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	e.texts = append(e.texts, text)
	e.mu.Unlock()
	return e.mockEmbedder.Embed(ctx, text)
}

func (e *countingEmbedder) calls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.texts...)
}

// para builds one splitter-stable paragraph: the sentence repeated n
// times so the recursive splitter always cuts at 。 boundaries and the
// paragraph's chunk boundaries are identical across versions that keep
// the sentence shape.
func para(tag string, n int) string {
	return strings.Repeat(tag+"政策条款内容测试。", n)
}

// TestUploadIndexAssignsDocLifecycleMetadata pins the producer contract:
// every doc carries doc_id (verbatim from the parameter), chunk_index
// (0-based position in the new sequence), and content_hash (sha256 hex of
// the chunk text — the exact digest contentHashHex computes).
func TestUploadIndexAssignsDocLifecycleMetadata(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	me := &mockEmbedder{}

	const sentence = "条款内容若干字。"
	// 3 sentences/paragraph keeps each paragraph under chunkSizeRunes; two
	// paragraphs over 3500 runes total force a multi-chunk split.
	body := strings.Repeat(sentence, 300) + "\n\n" + strings.Repeat(sentence, 300)

	docs, err := UploadIndex(context.Background(), "authority", []byte(body), "text/markdown", "doc.md", "authority/doc.md", me, reg, &mockRetriever{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(docs), 2, "the body must split into multiple chunks to make chunk_index meaningful")
	for i, d := range docs {
		assert.Equal(t, "authority/doc.md", d.DocID, "chunk %d doc_id", i)
		assert.Equal(t, int32(i), d.ChunkIndex, "chunk %d position", i)
		assert.Equal(t, contentHashHex(d.Content), d.ContentHash, "chunk %d content_hash must be sha256-hex of the chunk text", i)
		assert.Len(t, d.ContentHash, 64, "chunk %d content_hash must be a full sha256 hex digest", i)
	}
}

// TestUploadIndexUpsertReuploadKeepsUnchangedPKs is the D-53-03
// acceptance scenario: re-upload the SAME document with one character
// changed in the middle paragraph and assert that
//  1. unchanged chunks KEEP their PKs (reused, no re-embed),
//  2. changed chunks get fresh PKs above the tier watermark,
//  3. only the changed chunks cost an embedding call (省 BGE-M3),
//  4. the doc's previous generation is deleted exactly once,
//  5. after the simulated re-insert the doc's rows are exactly the new
//     generation with contiguous chunk_index 0..N-1.
func TestUploadIndexUpsertReuploadKeepsUnchangedPKs(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	emb := &countingEmbedder{mockEmbedder: mockEmbedder{}}
	store := newFakeDocStore()
	ctx := context.Background()
	docID := "authority/doc.md"

	v1Body := para("甲", 500) + "\n\n" + para("乙", 500) + "\n\n" + para("丙", 500)
	v1, err := UploadIndex(ctx, "authority", []byte(v1Body), "text/markdown", "doc.md", docID, emb, reg, store)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(v1), 3, "the three paragraphs must split into multiple chunks")
	v1Embeds := len(emb.calls())
	require.Equal(t, len(v1), v1Embeds, "first upload embeds every chunk")
	require.Empty(t, store.deleteLog(), "first upload must not delete anything")
	store.insert(v1)

	// v2: one character changed in the MIDDLE paragraph — the paragraph
	// shape (sentence count + boundaries) is preserved so only that
	// paragraph's chunk hashes change.
	v2Body := para("甲", 500) + "\n\n" + para("乙改", 500) + "\n\n" + para("丙", 500)
	v2, err := UploadIndex(ctx, "authority", []byte(v2Body), "text/markdown", "doc.md", docID, emb, reg, store)
	require.NoError(t, err)
	require.NotEmpty(t, v2)

	// Partition v2 by hash against v1.
	v1ByHash := make(map[string]retriever.Document, len(v1))
	for _, d := range v1 {
		v1ByHash[d.ContentHash] = d
	}
	var reused, fresh []retriever.Document
	for _, d2 := range v2 {
		if d1, ok := v1ByHash[d2.ContentHash]; ok {
			reused = append(reused, d2)
			assert.Equal(t, d1.ID, d2.ID, "unchanged chunk must KEEP its PK (content_hash %s)", d2.ContentHash[:12])
		} else {
			fresh = append(fresh, d2)
		}
	}
	require.NotEmpty(t, reused, "the 甲/丙 paragraphs are unchanged — at least their chunks must be reused")
	require.NotEmpty(t, fresh, "the edited 乙 paragraph must produce fresh chunks")

	// (3) 省 BGE-M3: exactly the fresh chunks cost an embedding call.
	v2Embeds := len(emb.calls()) - v1Embeds
	assert.Equal(t, len(fresh), v2Embeds,
		"only changed chunks may be re-embedded (fresh=%d, actual embeds=%d)", len(fresh), v2Embeds)

	// (2) fresh PKs sit above the tier watermark v1 established.
	var v1Max int64
	for _, d := range v1 {
		if d.ID > v1Max {
			v1Max = d.ID
		}
	}
	for _, d := range fresh {
		assert.Greater(t, d.ID, v1Max, "fresh chunk id must be above the previous watermark")
	}

	// (4) exactly one delete of the doc's previous generation.
	assert.Equal(t, []string{"authority/" + docID}, store.deleteLog(),
		"the upsert must delete the doc's previous generation exactly once")

	// (5) simulated re-insert converges the doc's rows onto the new
	// generation: contiguous chunk_index, every superseded PK gone.
	store.insert(v2)
	rows := store.docRows(t, "authority", docID)
	require.Len(t, rows, len(v2), "the doc's rows must be exactly the new generation")
	seenIdx := make(map[int32]bool, len(rows))
	seenID := make(map[int64]bool, len(rows))
	for _, r := range rows {
		seenIdx[r.ChunkIndex] = true
		seenID[r.ID] = true
	}
	for i := 0; i < len(rows); i++ {
		assert.True(t, seenIdx[int32(i)], "chunk_index must be contiguous 0..N-1 (missing %d)", i)
	}
	v2Hashes := make(map[string]bool, len(v2))
	for _, d2 := range v2 {
		v2Hashes[d2.ContentHash] = true
	}
	for _, d1 := range v1 {
		if !v2Hashes[d1.ContentHash] {
			assert.False(t, seenID[d1.ID], "superseded chunk id %d must be gone after delete+insert", d1.ID)
		}
	}
}

// TestUploadIndexUpsertRewritesChunkIndexForReusedRows pins the plan note:
// a retained row's chunk_index is REWRITTEN to the new sequence, so a
// change at the HEAD of the document still leaves 0..N-1 in order (the
// retained rows carry their old PKs but fresh positions).
func TestUploadIndexUpsertRewritesChunkIndexForReusedRows(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	emb := &countingEmbedder{mockEmbedder: mockEmbedder{}}
	store := newFakeDocStore()
	ctx := context.Background()
	docID := "authority/doc.md"

	// v1 head chunk differs from v2 head chunk; the tail is identical.
	v1Body := para("旧", 500) + "\n\n" + para("同", 500)
	v2Body := para("新", 500) + "\n\n" + para("同", 500)

	v1, err := UploadIndex(ctx, "authority", []byte(v1Body), "text/markdown", "doc.md", docID, emb, reg, store)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(v1), 2)
	store.insert(v1)

	v2, err := UploadIndex(ctx, "authority", []byte(v2Body), "text/markdown", "doc.md", docID, emb, reg, store)
	require.NoError(t, err)
	require.Len(t, v2, len(v1), "the tail is unchanged, so the chunk count must hold")

	// The tail chunk (identical hash) keeps its old PK but its chunk_index
	// must be the NEW sequence position.
	v1Tail := v1[len(v1)-1]
	v2Tail := v2[len(v2)-1]
	assert.Equal(t, v1Tail.ContentHash, v2Tail.ContentHash, "tail chunk must be byte-identical")
	assert.Equal(t, v1Tail.ID, v2Tail.ID, "tail chunk keeps its PK")
	assert.Equal(t, int32(len(v2)-1), v2Tail.ChunkIndex, "chunk_index must follow the NEW sequence")
	assert.Equal(t, int32(0), v2[0].ChunkIndex, "the head chunk starts the new sequence at 0")
}

// capabilitylessRetriever is a Retriever WITHOUT retriever.DocChunkStore —
// the mongo-backend shape (it predates the lifecycle capability). Go has
// no "remove method" construct: embedding mockRetriever would promote the
// two lifecycle methods too, so this double delegates the eight Retriever
// methods explicitly and never defines DocChunkRows / DeleteDocRows.
type capabilitylessRetriever struct {
	inner *mockRetriever
}

var _ retriever.Retriever = (*capabilitylessRetriever)(nil)

func (c *capabilitylessRetriever) Health(ctx context.Context) error {
	return c.inner.Health(ctx)
}
func (c *capabilitylessRetriever) Search(ctx context.Context, q []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error) {
	return c.inner.Search(ctx, q, tiers, topK, now)
}
func (c *capabilitylessRetriever) Ensure(ctx context.Context) error { return c.inner.Ensure(ctx) }
func (c *capabilitylessRetriever) Insert(ctx context.Context, docs []retriever.Document) error {
	return c.inner.Insert(ctx, docs)
}
func (c *capabilitylessRetriever) ListMetadata(ctx context.Context) ([]retriever.DocumentMeta, error) {
	return c.inner.ListMetadata(ctx)
}
func (c *capabilitylessRetriever) GetChunk(ctx context.Context, id int64) (retriever.ChunkContent, error) {
	return c.inner.GetChunk(ctx, id)
}
func (c *capabilitylessRetriever) GetMaxID(ctx context.Context, tier string) (int64, error) {
	return c.inner.GetMaxID(ctx, tier)
}
func (c *capabilitylessRetriever) Disconnect() error { return c.inner.Disconnect() }

// TestUploadIndexUpsertCapabilityFailClosed pins the fail-closed gate: a
// real doc_id against a backend without the lifecycle capability must
// ERROR (never silently degrade to bare-append — that is breakpoint C),
// and it must fail before any embedding call.
func TestUploadIndexUpsertCapabilityFailClosed(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	me := &countingEmbedder{mockEmbedder: mockEmbedder{}}
	r := &capabilitylessRetriever{}

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", "authority/test.md", me, reg, r)
	require.Error(t, err)
	assert.Nil(t, docs)
	assert.Contains(t, err.Error(), "retriever.DocChunkStore")
	assert.Empty(t, me.calls(), "the capability gate must fire before any embedding call")
}

// TestUploadIndexLegacyAppendPostureEmptyDocID pins the empty-docID
// escape hatch: no lookup, no delete, every chunk fresh — IDs stay the
// GetMaxID+1.. sequence, exactly the pre-53 posture (direct/test callers
// only; the production upload path always passes a doc_id).
func TestUploadIndexLegacyAppendPostureEmptyDocID(t *testing.T) {
	reg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	emb := &countingEmbedder{mockEmbedder: mockEmbedder{}}
	store := newFakeDocStore()

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", "", emb, reg, store)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, int64(1), docs[0].ID)
	assert.Equal(t, "", docs[0].DocID, "empty doc_id keeps the legacy zero value")
	assert.Empty(t, store.deleteLog(), "the legacy posture must not delete anything")
	assert.Len(t, emb.calls(), 1, "the legacy posture embeds without a lifecycle lookup")
}
