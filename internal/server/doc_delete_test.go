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

package server

// Phase 53-03 (ING-03 / D-53-03) — DELETE /api/admin/kb/:slug/docs/:doc_id
// handler tests. Contracts under test:
//
//  1. Happy path: 200 {deleted_chunks: N}, the doc's rows leave the
//     retriever, the canonical knowledge/<slug>/ file leaves the disk
//     (D-53-01: the file IS the corpus — deleting rows without it would
//     resurrect the doc at the next reseed), and one action_doc_deleted
//     audit row lands.
//  2. 404 = doc_id with no lifecycle rows.
//  3. Cross-slug guard: a request scoped to one base can never address
//     another base's doc_id (400).
//  4. Traversal floor: ".." segments never become a filesystem path (400,
//     validRelDocPath gate).
//  5. 403 before anything that could leak KB registry state.
//
// Hermetic: real Hertz server on a dynamic listener (sessionsTestHertz
// pattern — the wildcard *doc_id route must go through actual routing),
// fake DocChunkStore retriever, temp knowledge root + temp store.

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/store"
)

// fakeDeleteRetriever is the lifecycle-capable Retriever double: rows in
// a PK-keyed map, DocChunkRows/DeleteDocRows operate on it exactly like
// the real backends (tier AND doc_id must both match).
type fakeDeleteRetriever struct {
	mu   sync.Mutex
	rows map[int64]retriever.Document
	next int64 // PK allocator — seeds must never collide (Milvus PKs are unique)
}

func newFakeDeleteRetriever() *fakeDeleteRetriever {
	return &fakeDeleteRetriever{rows: map[int64]retriever.Document{}, next: 100}
}

func (f *fakeDeleteRetriever) seed(t *testing.T, tier, docID string, n int) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < n; i++ {
		id := f.next
		f.next++
		f.rows[id] = retriever.Document{
			ID:         id,
			Tier:       tier,
			Content:    "chunk",
			Source:     docID,
			DocID:      docID,
			ChunkIndex: int32(i),
			Embedding:  []float32{1, 0, 0, 0},
		}
	}
}

func (f *fakeDeleteRetriever) rowCount(t *testing.T, tier, docID string) int {
	t.Helper()
	rows, err := f.DocChunkRows(t.Context(), tier, docID)
	require.NoError(t, err)
	return len(rows)
}

func (f *fakeDeleteRetriever) Health(_ context.Context) error { return nil }
func (f *fakeDeleteRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (f *fakeDeleteRetriever) Ensure(_ context.Context) error { return nil }
func (f *fakeDeleteRetriever) Insert(_ context.Context, _ []retriever.Document) error {
	return nil
}
func (f *fakeDeleteRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (f *fakeDeleteRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, retriever.ErrChunkNotFound
}
func (f *fakeDeleteRetriever) GetMaxID(_ context.Context, _ string) (int64, error) { return 0, nil }
func (f *fakeDeleteRetriever) Disconnect() error                                   { return nil }

func (f *fakeDeleteRetriever) DocChunkRows(_ context.Context, tier, docID string) ([]retriever.DocChunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []retriever.DocChunk
	for _, d := range f.rows {
		if d.Tier == tier && d.DocID == docID {
			out = append(out, retriever.DocChunk{ID: d.ID, ChunkIndex: d.ChunkIndex, Embedding: d.Embedding})
		}
	}
	return out, nil
}

func (f *fakeDeleteRetriever) DeleteDocRows(_ context.Context, tier, docID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, d := range f.rows {
		if d.Tier == tier && d.DocID == docID {
			delete(f.rows, id)
		}
	}
	return nil
}

// docDeleteAuditStore records PutAudit rows; every other store.Store
// method panics on the nil embed (mockStore discipline).
type docDeleteAuditStore struct {
	store.Store
	mu   sync.Mutex
	rows []store.AuditRow
}

func (s *docDeleteAuditStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, row)
	return int64(len(s.rows)), nil
}

func (s *docDeleteAuditStore) auditRows() []store.AuditRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.AuditRow(nil), s.rows...)
}

// docDeleteTestEnv builds the Deps + a live Hertz server bound to the
// wildcard DELETE route, with the knowledge root redirected into a temp
// dir and the admin token set. Cleanup is via t.Cleanup.
func docDeleteTestEnv(t *testing.T, retr retriever.Retriever, st store.Store) string {
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

	prevRoot := knowledge.PersistKnowledgeRoot()
	knowledge.SetPersistKnowledgeRoot(t.TempDir())
	t.Cleanup(func() { knowledge.SetPersistKnowledgeRoot(prevRoot) })

	deps := &Deps{
		KBRegistry: kb.New([]kb.Entry{
			{Slug: "authority", DisplayName: "权威", Role: kb.RoleAuthority, RetrievalEnabled: true},
			{Slug: "personal", DisplayName: "个人", Role: kb.RolePersonal, RetrievalEnabled: true},
		}),
		Retriever: retr,
		Store:     st,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port
	h := server.Default(server.WithListener(ln))
	h.DELETE("/api/admin/kb/:slug/docs/*doc_id", deps.docDeleteHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	t.Cleanup(func() {
		_ = h.Close()
		<-errCh
	})

	addr := "127.0.0.1:" + portStr(port)
	if err := waitForListener(addr, 3*time.Second); err != nil {
		t.Fatalf("server warmup: %v", err)
	}
	return addr
}

// deleteDoc issues the DELETE with the admin token and returns the status
// plus the raw body bytes.
func deleteDoc(t *testing.T, addr, urlPath, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, "http://"+addr+urlPath, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("X-Admin-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// seedDocFile writes the canonical knowledge file the handler must
// remove, under the (temp) knowledge root.
func seedDocFile(t *testing.T, slug, rel string, body []byte) string {
	t.Helper()
	dir := filepath.Join(knowledge.PersistKnowledgeRoot(), slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, rel)
	require.NoError(t, os.WriteFile(path, body, 0o644))
	return path
}

// TestDocDeleteHappyPath: rows out, file out, audit row in, 200 with the
// chunk count.
func TestDocDeleteHappyPath(t *testing.T) {
	retr := newFakeDeleteRetriever()
	retr.seed(t, "authority", "authority/delme.md", 3)
	audit := &docDeleteAuditStore{}
	addr := docDeleteTestEnv(t, retr, audit)

	filePath := seedDocFile(t, "authority", "delme.md", []byte("# Delme\n\nbody"))
	require.FileExists(t, filePath, "precondition: the canonical file exists")

	status, raw := deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/delme.md", testAdminToken)
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))
	var body struct {
		DeletedChunks int `json:"deleted_chunks"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, 3, body.DeletedChunks)

	// The doc's rows left the retriever — a search metadata lookup finds
	// nothing (retrieval can never hit the deleted doc again).
	assert.Equal(t, 0, retr.rowCount(t, "authority", "authority/delme.md"))

	// The canonical file left the disk (D-53-01: the file IS the corpus).
	assert.NoFileExists(t, filePath, "the canonical file must be removed with the rows")

	// One INSERT-only audit row with the lifecycle facts.
	rows := audit.auditRows()
	require.Len(t, rows, 1)
	assert.Equal(t, actionDocDeleted, rows[0].Action)
	assert.Equal(t, "admin", rows[0].ActorType)
	var payload DocDeleteAudit
	require.NoError(t, json.Unmarshal(rows[0].FrozenArgs, &payload))
	assert.Equal(t, "authority", payload.Slug)
	assert.Equal(t, "authority/delme.md", payload.DocID)
	assert.Equal(t, 3, payload.ChunkCount)
}

// TestDocDeleteOtherDocsSurvive: deleting one doc must not touch sibling
// docs in the same tier.
func TestDocDeleteOtherDocsSurvive(t *testing.T) {
	retr := newFakeDeleteRetriever()
	retr.seed(t, "authority", "authority/delme.md", 2)
	retr.seed(t, "authority", "authority/keeper.md", 2)
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})

	keeperPath := seedDocFile(t, "authority", "keeper.md", []byte("# Keeper\n\nbody"))
	seedDocFile(t, "authority", "delme.md", []byte("# Delme\n\nbody"))

	status, raw := deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/delme.md", testAdminToken)
	require.Equal(t, http.StatusOK, status, "body=%s", string(raw))
	assert.Equal(t, 2, retr.rowCount(t, "authority", "authority/keeper.md"))
	assert.FileExists(t, keeperPath)
}

// TestDocDeleteUnknownDocID404: a doc_id with no lifecycle rows is a 404
// (the message names the pre-53 collection remedy).
func TestDocDeleteUnknownDocID404(t *testing.T) {
	retr := newFakeDeleteRetriever()
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})

	// A file exists on disk but no rows carry the doc_id — the lifecycle
	// key is the ROWS, so this is 404 and the file stays untouched.
	seedDocFile(t, "authority", "orphan.md", []byte("# Orphan\n\nbody"))

	status, raw := deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/orphan.md", testAdminToken)
	require.Equal(t, http.StatusNotFound, status)
	var errResp graphResponseError
	require.NoError(t, json.Unmarshal(raw, &errResp))
	assert.Contains(t, errResp.Error, "no lifecycle rows")
	assert.FileExists(t, filepath.Join(knowledge.PersistKnowledgeRoot(), "authority", "orphan.md"))
}

// TestDocDeleteCrossSlugGuard: the URL slug and the doc_id's slug segment
// must agree — a request scoped to one base can never address another
// base's rows or file.
func TestDocDeleteCrossSlugGuard(t *testing.T) {
	retr := newFakeDeleteRetriever()
	retr.seed(t, "personal", "personal/secret.md", 2)
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})

	secretPath := seedDocFile(t, "personal", "secret.md", []byte("# Secret\n\nbody"))

	status, _ := deleteDoc(t, addr, "/api/admin/kb/authority/docs/personal/secret.md", testAdminToken)
	assert.Equal(t, http.StatusBadRequest, status)
	// Nothing was deleted — the rows and the file both survive.
	assert.Equal(t, 2, retr.rowCount(t, "personal", "personal/secret.md"))
	assert.FileExists(t, secretPath)
}

// TestDocDeleteTraversalGuard: ".." segments must never become a
// filesystem path (validRelDocPath floor, same rule as the doc reader).
func TestDocDeleteTraversalGuard(t *testing.T) {
	retr := newFakeDeleteRetriever()
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})

	// Plant the would-be traversal target OUTSIDE the knowledge root, in
	// the temp root's parent — if the gate failed, a successful delete
	// would have removed this file.
	outside := filepath.Join(filepath.Dir(knowledge.PersistKnowledgeRoot()), "secret.md")
	require.NoError(t, os.WriteFile(outside, []byte("victim"), 0o644))
	t.Cleanup(func() { _ = os.Remove(outside) })

	status, _ := deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/../secret.md", testAdminToken)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.FileExists(t, outside, "the traversal target must survive")
}

// TestDocDeleteAdminGate: 403 before anything that could leak KB registry
// state (wrong token), and the mutation must not happen.
func TestDocDeleteAdminGate(t *testing.T) {
	retr := newFakeDeleteRetriever()
	retr.seed(t, "authority", "authority/delme.md", 1)
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})
	seedDocFile(t, "authority", "delme.md", []byte("# Delme\n\nbody"))

	status, _ := deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/delme.md", "wrong-token")
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, 1, retr.rowCount(t, "authority", "authority/delme.md"))

	// Empty token → same 403.
	status, _ = deleteDoc(t, addr, "/api/admin/kb/authority/docs/authority/delme.md", "")
	assert.Equal(t, http.StatusForbidden, status)
}

// TestDocDeleteEmptyDocID400: a bare /docs/ wildcard with nothing behind
// it must NEVER delete — the handler rejects an empty doc_id with 400,
// and a router that does not match the empty wildcard at all answers
// 404; both outcomes leave every row intact. The assertion pins the
// contract (never 200, nothing deleted), not the router's empty-wildcard
// spelling, which is version-dependent.
func TestDocDeleteEmptyDocID400(t *testing.T) {
	retr := newFakeDeleteRetriever()
	retr.seed(t, "authority", "authority/delme.md", 1)
	addr := docDeleteTestEnv(t, retr, &docDeleteAuditStore{})

	status, _ := deleteDoc(t, addr, "/api/admin/kb/authority/docs/", testAdminToken)
	if status != http.StatusBadRequest && status != http.StatusNotFound {
		t.Fatalf("empty doc_id must answer 400/404, got %d", status)
	}
	assert.Equal(t, 1, retr.rowCount(t, "authority", "authority/delme.md"), "nothing may be deleted")
}
