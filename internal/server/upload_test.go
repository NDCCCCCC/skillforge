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

// Phase 27 Plan 03 — Unit tests for POST /api/admin/kb/:slug/upload.
// All tests are hermetic: no real Milvus, no real BGE, no real parser file
// fixtures (bytes.Buffer for md content). The mock IndexerBackend records
// inserted docs; the mock Retriever returns configurable GetMaxID values.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"sync"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
)

// testAdminToken is declared in import_api_test.go — reuse it here.

// mockIndexerBackend is the test double for server.IndexerBackend.
// It records every Insert call and returns a configurable error.
type mockIndexerBackend struct {
	mu      sync.Mutex
	docs    [][]retriever.Document
	err     error
	onInsert func(ctx context.Context, docs []retriever.Document) error
}

func (m *mockIndexerBackend) Insert(ctx context.Context, docs []retriever.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.onInsert != nil {
		return m.onInsert(ctx, docs)
	}
	// Copy so later assertions don't see downstream mutation.
	cp := make([]retriever.Document, len(docs))
	copy(cp, docs)
	m.docs = append(m.docs, cp)
	return m.err
}

func (m *mockIndexerBackend) Docs() [][]retriever.Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.docs
}

// mockRetrieverForUpload is the test double for retriever.Retriever used in upload tests.
// It embeds mockIndexerBackend so all retriever.Retriever methods are
// inherited; the GetMaxID override reads the SAME backend's recorded
// Insert batches (production contract: GetMaxID queries the live
// collection), so the per-slug serialization test observes non-overlapping
// ID ranges across concurrent uploads.
//
// IMPORTANT: the embedded mockIndexerBackend is a *POINTER* so the
// Retriever and IndexerBackend share the same mu/docs — copying the
// struct by value would split the state and break the GetMaxID read.
type mockRetrieverForUpload struct {
	*mockIndexerBackend
}

func (m *mockRetrieverForUpload) GetMaxID(_ context.Context, _ string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var maxID int64
	for _, batch := range m.docs {
		for _, d := range batch {
			if d.ID > maxID {
				maxID = d.ID
			}
		}
	}
	return maxID, nil
}

func (m *mockRetrieverForUpload) Health(_ context.Context) error { return nil }
func (m *mockRetrieverForUpload) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) { return nil, nil }
func (m *mockRetrieverForUpload) Ensure(_ context.Context) error { return nil }
func (m *mockRetrieverForUpload) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) { return nil, nil }
func (m *mockRetrieverForUpload) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, retriever.ErrChunkNotFound
}
func (m *mockRetrieverForUpload) Disconnect() error { return nil }

// stubEmbedderForUpload is the test double for embedder.Embedder.
// Reused from testhelpers — kept here for clarity.
type stubEmbedderForUpload struct {
	vec   []float32
	calls []string
	mu    sync.Mutex
}

func newStubEmbedderForUpload() *stubEmbedderForUpload {
	v := make([]float32, 1024)
	v[0] = 1.0
	return &stubEmbedderForUpload{vec: v}
}

var _ embedder.Embedder = (*stubEmbedderForUpload)(nil)

func (e *stubEmbedderForUpload) Embed(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, text)
	return e.vec, nil
}

func (e *stubEmbedderForUpload) Dimensions() int { return 1024 }

// uploadTestDeps builds a fully-wired Deps for upload tests.
// KBRegistry has one active base: "authority".
// The IndexerBackend and Retriever mocks share the SAME underlying state
// (idxBackend embeds mockIndexerBackend, and we wrap it as the Retriever)
// so GetMaxID reflects Insert calls — this is the production contract.
func uploadTestDeps(t *testing.T, idxBackend *mockIndexerBackend) *Deps {
	t.Helper()
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		} else {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		}
	})

	kbReg := kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	return &Deps{
		KBRegistry:       kbReg,
		Embedder:         newStubEmbedderForUpload(),
		Retriever:        &mockRetrieverForUpload{mockIndexerBackend: idxBackend},
		IndexerBackend:   idxBackend,
		ParserRegistry:  document.NewParserRegistry(),
		UploadMaxBodyBytes: 50 << 20, // 50 MiB for tests
	}
}

// buildUploadMultipart builds a multipart/form-data body with a "file" part
// carrying the given Content-Type header (mimeType) and content. The route
// reads the declared MIME from the file part's Content-Type header, NOT from
// a "metadata" form field (DOC-03 contract; the part header is source-of-truth).
// Returns body bytes, Content-Type string, and total length.
func buildUploadMultipart(t *testing.T, filename, mimeType string, content []byte) ([]byte, string, int) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// Use CreatePart (NOT CreateFormFile) so we can set the file part's
	// Content-Type header explicitly — CreateFormFile would default it to
	// application/octet-stream and break the content-type allowlist test.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	if mimeType != "" {
		h.Set("Content-Type", mimeType)
	}
	fw, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType(), buf.Len()
}

// invokeUpload issues POST /api/admin/kb/:slug/upload against a test server.
func invokeUpload(t *testing.T, deps *Deps, slug, token string, body []byte, contentType string, contentLength int) *ut.ResponseRecorder {
	t.Helper()
	h := New(":0", deps)
	headers := []ut.Header{{Key: "Content-Type", Value: contentType}}
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	return ut.PerformRequest(h.Engine, http.MethodPost, "/api/admin/kb/"+slug+"/upload", &ut.Body{Body: bytes.NewReader(body), Len: contentLength}, headers...)
}

// --- Test 1: Admin token required ---
func TestUploadAdminTokenRequired(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	// No token → 403
	resp := invokeUpload(t, deps, "authority", "", []byte("# Hello"), "text/markdown", 7)
	assert.Equal(t, http.StatusForbidden, resp.Code)

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error"], "admin token")
}

// --- Test 2: Wrong token → 403 ---
func TestUploadWrongToken(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	resp := invokeUpload(t, deps, "authority", "wrong-token", []byte("# Hello"), "text/markdown", 7)
	assert.Equal(t, http.StatusForbidden, resp.Code)
}

// --- Test 3: KB registry nil → 503 ---
func TestUploadKBRegistryNotWired(t *testing.T) {
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	t.Cleanup(func() {
		if prev == "" { _ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN") } else { _ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev) }
	})

	deps := &Deps{
		KBRegistry:       nil, // not wired
		Embedder:         newStubEmbedderForUpload(),
		Retriever:        &mockRetrieverForUpload{mockIndexerBackend: &mockIndexerBackend{}},
		IndexerBackend:   &mockIndexerBackend{},
		ParserRegistry:  document.NewParserRegistry(),
		UploadMaxBodyBytes: 50 << 20,
	}
	resp := invokeUpload(t, deps, "authority", testAdminToken, []byte("# Hello"), "text/markdown", 7)
	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// --- Test 4: Unknown slug → 404 ---
func TestUploadUnknownSlug(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	resp := invokeUpload(t, deps, "nonexistent", testAdminToken, []byte("# Hello"), "text/markdown", 7)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

// --- Test 5: Invalid slug → 400 ---
func TestUploadInvalidSlug(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	resp := invokeUpload(t, deps, "Invalid.Slug!", testAdminToken, []byte("# Hello"), "text/markdown", 7)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

// --- Test 6: Oversized file → 413 ---
func TestUploadBodySizeGuard(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})
	deps.UploadMaxBodyBytes = 10 // 10 bytes max

	largeContent := bytes.Repeat([]byte("#x"), 100) // 200 bytes
	resp := invokeUpload(t, deps, "authority", testAdminToken, largeContent, "text/markdown", len(largeContent))
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
}

// --- Test 7: Unknown MIME type → 400 ---
func TestUploadContentTypeAllowlist(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	content := []byte("hello world")
	body, ct, n := buildUploadMultipart(t, "test.txt", "application/octet-stream", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	var errResp map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error"], "unknown format")
}

// --- Test 8: Magic byte mismatch → 400 ---
func TestUploadMagicByteMismatch(t *testing.T) {
	deps := uploadTestDeps(t, &mockIndexerBackend{})

	// Content starts with %PDF (PDF magic) but declared as text/markdown
	content := []byte("%PDF-1.4 fake pdf content")
	body, ct, n := buildUploadMultipart(t, "fake.pdf", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	var errResp map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error"], "content type does not match file signature")
}

// --- Test 9: Successful upload → 200 + audit row ---
func TestUploadSuccess(t *testing.T) {
	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)

	content := []byte("# Authority Document\n\nThis is the content.")
	body, ct, n := buildUploadMultipart(t, "policy.md", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)

	assert.Equal(t, http.StatusOK, resp.Code)

	var uploadResp struct {
		Chunks  int    `json:"chunks"`
		IDRange []int64 `json:"id_range"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &uploadResp))
	assert.Greater(t, uploadResp.Chunks, 0)
	require.Len(t, uploadResp.IDRange, 2)
	assert.Equal(t, int64(1), uploadResp.IDRange[0])
	assert.Equal(t, int64(uploadResp.Chunks), uploadResp.IDRange[1])

	// Verify IndexerBackend.Insert was called
	docs := idx.Docs()
	assert.Len(t, docs, 1)
	assert.Len(t, docs[0], uploadResp.Chunks)
}

// --- Test 10: Audit row on success ---
func TestUploadAuditRowOnSuccess(t *testing.T) {
	var hookCalls []struct{ Action, Detail string }
	hook := func(action, detail string) {
		hookCalls = append(hookCalls, struct{ Action, Detail string }{action, detail})
	}

	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	// Suppress the global hook during this test.
	originalHook := docUploadAuditHook
	docUploadAuditHook = hook
	defer func() {
		docUploadAuditHook = originalHook
		if prev == "" { _ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN") } else { _ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev) }
	}()

	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)

	content := []byte("# Test\n\nBody text here.")
	body, ct, n := buildUploadMultipart(t, "test.md", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code)

	require.Len(t, hookCalls, 1)
	assert.Equal(t, "action_doc_uploaded", hookCalls[0].Action)

	var audit struct {
		Slug   string `json:"slug"`
		SHA256 string `json:"sha256"`
		Chunks int    `json:"chunk_count"`
	}
	require.NoError(t, json.Unmarshal([]byte(hookCalls[0].Detail), &audit))
	assert.Equal(t, "authority", audit.Slug)
	assert.NotEmpty(t, audit.SHA256)
	assert.Greater(t, audit.Chunks, 0)
}

// --- Test 11: Audit row on rejection ---
func TestUploadAuditRowOnRejected(t *testing.T) {
	var hookCalls []struct{ Action, Detail string }
	hook := func(action, detail string) {
		hookCalls = append(hookCalls, struct{ Action, Detail string }{action, detail})
	}

	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	originalHook := docUploadAuditHook
	docUploadAuditHook = hook
	defer func() {
		docUploadAuditHook = originalHook
		if prev == "" { _ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN") } else { _ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev) }
	}()

	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)

	// PDF magic bytes but declared as text/markdown → magic byte mismatch → 400
	content := []byte("%PDF-1.4 fake")
	body, ct, n := buildUploadMultipart(t, "fake.pdf", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	require.Len(t, hookCalls, 1)
	assert.Equal(t, "action_doc_upload_rejected", hookCalls[0].Action)

	var audit struct {
		Slug   string `json:"slug"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal([]byte(hookCalls[0].Detail), &audit))
	assert.Equal(t, "authority", audit.Slug)
	assert.Contains(t, audit.Reason, "content type does not match file signature")
}

// --- Test 12: Per-slug serialization ---
func TestUploadPerSlugSerialization(t *testing.T) {
	// CRITICAL: the same mockIndexerBackend instance must back BOTH the
	// IndexerBackend (for Insert) AND the Retriever (for GetMaxID) — the
	// production contract is that GetMaxID reads the live collection, and
	// the per-slug serialization test depends on that read to see the first
	// upload's chunks before the second upload computes its nextID.
	idx := &mockIndexerBackend{}
	deps := &Deps{
		KBRegistry: kb.New([]kb.Entry{
			{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
		}),
		Embedder:          newStubEmbedderForUpload(),
		Retriever:         &mockRetrieverForUpload{mockIndexerBackend: idx},
		IndexerBackend:    idx,
		ParserRegistry:    document.NewParserRegistry(),
		UploadMaxBodyBytes: 50 << 20,
	}
	// Set admin token (uploadTestDeps helper does this; replicated here).
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", testAdminToken)
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		} else {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		}
	})

	content := []byte("# Chunk\n\nLine one.\nLine two.")
	body, ct, n := buildUploadMultipart(t, "doc.md", "text/markdown", content)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var ranges [][]int64

	// Fire 2 concurrent uploads to the same slug.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
			if resp.Code != http.StatusOK {
				return
			}
			var uploadResp struct {
				Chunks  int      `json:"chunks"`
				IDRange []int64 `json:"id_range"`
			}
			_ = json.Unmarshal(resp.Body.Bytes(), &uploadResp)
			mu.Lock()
			ranges = append(ranges, uploadResp.IDRange)
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Len(t, ranges, 2)
	// ID ranges must NOT overlap (serialization guarantee).
	r1, r2 := ranges[0], ranges[1]
	assert.True(t, r1[1] < r2[0] || r2[1] < r1[0],
		"ID ranges should not overlap: %v vs %v", r1, r2)
}

// --- VerifyMagicBytes unit tests ---
func TestVerifyMagicBytesPDF(t *testing.T) {
	data := []byte("%PDF-1.4 test content")
	err := VerifyMagicBytes(data, "application/pdf")
	assert.NoError(t, err)
}

func TestVerifyMagicBytesPDFMismatch(t *testing.T) {
	data := []byte("not a pdf")
	err := VerifyMagicBytes(data, "application/pdf")
	assert.Error(t, err)
	assert.ErrorIs(t, err, document.ErrContentTypeMismatch)
}

func TestVerifyMagicBytesMarkdown(t *testing.T) {
	data := []byte("# Hello\n\nWorld")
	err := VerifyMagicBytes(data, "text/markdown")
	assert.NoError(t, err)
}

func TestVerifyMagicBytesMarkdownFrontmatter(t *testing.T) {
	data := []byte("---\ntitle: Test\n---\n# Hello\nWorld")
	err := VerifyMagicBytes(data, "text/markdown")
	assert.NoError(t, err)
}

func TestVerifyMagicBytesPlainText(t *testing.T) {
	data := []byte("plain text content without any markdown sig")
	err := VerifyMagicBytes(data, "text/markdown")
	assert.NoError(t, err)
}

func TestVerifyMagicBytesUnknownType(t *testing.T) {
	data := []byte("some content")
	err := VerifyMagicBytes(data, "application/unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}
