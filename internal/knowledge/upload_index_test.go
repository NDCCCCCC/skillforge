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
	"sync"
	"testing"

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
type mockRetriever struct {
	HealthFunc       func(ctx context.Context) error
	SearchFunc       func(ctx context.Context, queryVec []float32, tiers []string, topK int, now int64) ([]retriever.Hit, error)
	EnsureFunc       func(ctx context.Context) error
	InsertFunc       func(ctx context.Context, docs []retriever.Document) error
	ListMetadataFunc func(ctx context.Context) ([]retriever.DocumentMeta, error)
	GetChunkFunc     func(ctx context.Context, id int64) (retriever.ChunkContent, error)
	GetMaxIDFunc     func(ctx context.Context, tier string) (int64, error)
	DisconnectFunc   func() error
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

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", me, reg, mr)

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
	docs, err := UploadIndex(context.Background(), "authority", []byte(md), "text/markdown", "policy.md", me, reg, mr)

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

	docs, err := UploadIndex(context.Background(), "authority", []byte("Chunk one"), "text/markdown", "test.md", me, reg, mr)

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

	_, err := UploadIndex(context.Background(), "authority", []byte("Hello"), "text/markdown", "test.md", me, reg, mr)

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

	_, err := UploadIndex(context.Background(), "nonexistent", []byte("Hello"), "text/markdown", "test.md", me, reg, mr)

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

	docs, err := UploadIndex(context.Background(), "authority", []byte("Hello world"), "text/markdown", "test.md", me, reg, mr)

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
	_, err := UploadIndex(context.Background(), "authority", []byte("not a pdf"), "application/pdf", "evil.pdf", me, reg, mr)

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

	// Build a body larger than maxDocBytes (4000).
	// The chunker adds 200-byte overlap, so 3 chunks are expected.
	largeBody := make([]byte, 9000)
	for i := range largeBody {
		largeBody[i] = 'x'
	}

	docs, err := UploadIndex(context.Background(), "authority", largeBody, "text/markdown", "large.md", me, reg, mr)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(docs), 2, "oversized doc should produce multiple chunks")
	for i, doc := range docs {
		assert.LessOrEqual(t, len(doc.Content), maxDocBytes, "chunk %d content length must be <= maxDocBytes", i)
	}
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

	_, err := UploadIndex(context.Background(), "authority", []byte("Hello"), "text/markdown", "test.md", me, reg, &mockRetriever{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tier validation failed")
	assert.True(t, hookCalled, "validateTiers must be called before embedder")
	assert.Equal(t, 0, embedCalls, "embedder must not be called when validateTiers fails")
}

// TestChunkBody verifies the chunker produces correct sizes and counts.
func TestChunkBody(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		wantMin   int
		wantMax   int
	}{
		{"small fits in one", 100, 1, 1},
		{"exactly max", maxDocBytes, 1, 1},
		{"just over max", maxDocBytes + 1, 2, 2},
		{"triple max", maxDocBytes*3 - 1, 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := make([]byte, tt.size)
			for i := range body {
				body[i] = 'x'
			}
			chunks := chunkBody(body, maxDocBytes)
			require.GreaterOrEqual(t, len(chunks), tt.wantMin)
			require.LessOrEqual(t, len(chunks), tt.wantMax)
			for i, c := range chunks {
				assert.LessOrEqual(t, len(c), maxDocBytes, "chunk %d exceeds maxDocBytes", i)
			}
		})
	}
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
			_, err := UploadIndex(context.Background(), slug, []byte("content"), "text/markdown", "test.md", me, reg, &mockRetriever{})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, 10, embedCalls)
	mu.Unlock()
}
