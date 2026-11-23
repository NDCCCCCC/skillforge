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

// Phase 53-01 (D-53-01) — upload persistence tests. Three groups:
//  1. a successful upload leaves a canonical file under knowledge/<slug>/
//     whose frontmatter carries source_sha256 (and whose body is the
//     frontmatter-stripped source — the exact chunk source);
//  2. the ORIGINAL bytes are archived under the uploads dir;
//  3. a persist failure is fail-closed: 422 + reject audit and
//     IndexerBackend.Insert is NEVER called (the forbidden half-state
//     "Insert succeeded but no file on disk" cannot be reached).
//
// Hermetic like upload_test.go: mock backend/embedder, t.TempDir() roots,
// no real Milvus / BGE. The restart-evaporation regression itself (upload
// → knowledge.Index reseed → chunk still retrievable) is the 53-04 wave's
// cross-path equivalence test; these tests pin the on-disk contract it
// builds on.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ingest"
	"github.com/skillforge/skillforge/internal/knowledge"
)

// setPersistDirs redirects both persist roots into fresh temp dirs and
// returns them. uploadTestDeps already redirects both as a hermeticity
// safety net; this helper re-points them so a test can assert on the
// exact on-disk paths. Cleanup order is LIFO (this restore runs before
// uploadTestDeps's), so the vars end up back at the defaults after the
// test.
func setPersistDirs(t *testing.T) (kbRoot, uploadsDir string) {
	t.Helper()
	kbRoot, uploadsDir = t.TempDir(), t.TempDir()
	prevKB, prevUploads := knowledge.PersistKnowledgeRoot(), knowledge.PersistUploadsDir()
	knowledge.SetPersistKnowledgeRoot(kbRoot)
	knowledge.SetPersistUploadsDir(uploadsDir)
	t.Cleanup(func() {
		knowledge.SetPersistKnowledgeRoot(prevKB)
		knowledge.SetPersistUploadsDir(prevUploads)
	})
	return kbRoot, uploadsDir
}

// persistAuditCapture binds an in-memory docUploadAuditHook collector and
// returns the recorded (action, detail) pairs plus a restore func —
// mirrors the hook-install pattern in upload_test.go.
func persistAuditCapture(t *testing.T) *[]struct{ Action, Detail string } {
	t.Helper()
	calls := &[]struct{ Action, Detail string }{}
	prev := docUploadAuditHook
	docUploadAuditHook = func(action, detail string) {
		*calls = append(*calls, struct{ Action, Detail string }{action, detail})
	}
	t.Cleanup(func() { docUploadAuditHook = prev })
	return calls
}

// --- Group 1: canonical knowledge file + provenance frontmatter ---

func TestPersistUploadWritesKnowledgeFile(t *testing.T) {
	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)
	kbRoot, _ := setPersistDirs(t)
	hookCalls := persistAuditCapture(t)

	// The source carries its own frontmatter with effective_from — the
	// window is PASSED THROUGH into the canonical file (53-01 review fix:
	// restart chunk equivalence covers the window too; dropping it would
	// silently degrade a bounded window to perpetual at the next reseed).
	// The rest of the source frontmatter still does not leak.
	content := "---\neffective_from: 1700000000\n---\n# Authority Doc\n\nReal body text."
	body, ct, n := buildUploadMultipart(t, "Policy Doc.md", "text/markdown", []byte(content))
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code)
	deps.Ingest.WaitIdle() // 54-01: persist runs on the runner goroutine

	sum := sha256.Sum256([]byte(content))
	sha256Hex := hex.EncodeToString(sum[:])
	sha8 := sha256Hex[:8]

	// Exactly one canonical file appeared under knowledge/<slug>/, named
	// <sha8>-<sanitized-stem>.md ("Policy Doc" → "policy-doc": lowercase,
	// space mapped into the whitelist charset).
	dir := filepath.Join(kbRoot, "authority")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "exactly one canonical file expected under %s", dir)
	name := entries[0].Name()
	assert.True(t, strings.HasPrefix(name, sha8+"-"),
		"canonical file %s must be prefixed with sha8 %s", name, sha8)
	assert.True(t, strings.HasSuffix(name, ".md"), "canonical file must end in .md: %s", name)
	assert.Contains(t, name, "policy-doc", "stem must be sanitized: %s", name)

	raw, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	text := string(raw)

	// Frontmatter provenance: sha matches the uploaded bytes, MIME is the
	// file part's Content-Type, doc_id is the corpus-relative path (same
	// string the success audit reports as persisted_path).
	assert.Contains(t, text, "source_sha256: "+sha256Hex)
	assert.Contains(t, text, "source_mime: text/markdown")
	assert.Contains(t, text, "uploaded_at: ")
	assert.Contains(t, text, "doc_id: authority/"+name)
	// The body is the frontmatter-stripped source — the exact bytes
	// UploadIndex chunked — and the source's own frontmatter did NOT leak
	// into the canonical file.
	assert.Contains(t, text, "# Authority Doc")
	assert.Contains(t, text, "Real body text.")
	assert.Contains(t, text, "effective_from: 1700000000", "source window must be passed through")

	// The wiring runs persist BEFORE Insert: a 200 means both happened.
	require.Len(t, idx.Docs(), 1, "Insert must still run after persist succeeds")

	// Two audit rows under the 54-01 contract: the enqueue acceptance
	// (action_doc_ingest_queued, HTTP layer) and the runner's completion
	// row (action_doc_uploaded) — the persisted_path contract lives on
	// the completion row.
	require.Len(t, *hookCalls, 2)
	require.Equal(t, "action_doc_ingest_queued", (*hookCalls)[0].Action)
	require.Equal(t, "action_doc_uploaded", (*hookCalls)[1].Action)
	var audit struct {
		PersistedPath string `json:"persisted_path"`
	}
	require.NoError(t, json.Unmarshal([]byte((*hookCalls)[1].Detail), &audit))
	assert.Equal(t, "authority/"+name, audit.PersistedPath)
}

// --- Group 2: original bytes archived under the uploads dir ---

func TestPersistUploadArchivesOriginalBytes(t *testing.T) {
	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)
	_, uploadsDir := setPersistDirs(t)

	content := []byte("# Archive Me\n\nRaw bytes must survive byte-exact.")
	body, ct, n := buildUploadMultipart(t, "report.md", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)
	require.Equal(t, http.StatusOK, resp.Code)
	deps.Ingest.WaitIdle() // 54-01: the archive write runs on the runner

	sum := sha256.Sum256(content)
	archivePath := filepath.Join(uploadsDir, hex.EncodeToString(sum[:])+".md")
	raw, err := os.ReadFile(archivePath)
	require.NoError(t, err, "archive %s must exist", archivePath)
	assert.Equal(t, content, raw, "archive must be byte-exact original upload bytes")

	// And exactly one archive file: no second write under a different name.
	entries, err := os.ReadDir(uploadsDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// --- Group 3: persist failure is fail-closed before Insert ---

func TestUploadPersistFailureRejectedBeforeInsert(t *testing.T) {
	idx := &mockIndexerBackend{}
	deps := uploadTestDeps(t, idx)
	kbRoot, _ := setPersistDirs(t)
	hookCalls := persistAuditCapture(t)

	// Portable persist-failure injection (works on Windows and Unix):
	// occupy the would-be slug directory path with a regular FILE, so
	// MkdirAll(kbRoot/authority) fails with ENOTDIR before any canonical
	// write can happen. The uploads-dir archive (write 1 of 2) still
	// succeeds — which is exactly what makes this a clean 422 instead of
	// a "file exists but upload rejected" half-state (archive-before-
	// canonical ordering in PersistUploadedDoc).
	require.NoError(t, os.WriteFile(filepath.Join(kbRoot, "authority"), []byte("blocking file"), 0o644))

	content := []byte("# Persist Fail\n\nInsert must never see this.")
	body, ct, n := buildUploadMultipart(t, "blocked.md", "text/markdown", content)
	resp := invokeUpload(t, deps, "authority", testAdminToken, body, ct, n)

	// 54-01 contract: the enqueue itself succeeds (200 — the job id IS
	// the acceptance), and the persist failure surfaces as the job's
	// terminal state after the runner settles. The fail-closed ordering
	// (persist BEFORE Insert) is unchanged — the zero-insert assertion
	// below is the same guard, moved from the HTTP layer to the job layer.
	require.Equal(t, http.StatusOK, resp.Code)
	var acc struct {
		JobID string `json:"job_id"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &acc))
	require.NotEmpty(t, acc.JobID)
	deps.Ingest.WaitIdle()

	job, err := deps.Ingest.GetJob(context.Background(), acc.JobID)
	require.NoError(t, err)
	assert.Equal(t, ingest.StatusFailed, job.Status, "a persist failure must fail the job")

	steps, err := deps.Ingest.ListSteps(context.Background(), acc.JobID)
	require.NoError(t, err)
	parseErrText := ""
	for _, s := range steps {
		if s.Step == "parse" && s.Error != "" {
			parseErrText = s.Error
		}
	}
	assert.Contains(t, parseErrText, "persist failed",
		"the parse step row must carry the persist failure reason")

	// Acceptance audit only — the failure lives in the job/step rows.
	require.Len(t, *hookCalls, 1, "exactly the enqueue acceptance audit expected")
	assert.Equal(t, "action_doc_ingest_queued", (*hookCalls)[0].Action)

	// Fail-closed ordering: persist runs BEFORE Insert, so a persist
	// failure must mean zero Insert calls — the forbidden half-state
	// "Insert succeeded but no file on disk" is unreachable by
	// construction.
	assert.Empty(t, idx.Docs(), "IndexerBackend.Insert must not be called on persist failure")

	// No canonical file anywhere under the knowledge root — the only
	// entry is the blocking file itself.
	entries, err := os.ReadDir(kbRoot)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "authority", entries[0].Name())
	assert.False(t, entries[0].IsDir())
}

// --- Direct unit coverage of the naming/sanitization contract ---

func TestPersistUploadedDocFileNameContract(t *testing.T) {
	kbRoot, uploadsDir := setPersistDirs(t)

	raw := []byte("# Stem Sanitize\n\nbody")
	docID, err := knowledge.PersistUploadedDoc(t.Context(), "authority", "Weird Name!.md", "text/markdown", raw)
	require.NoError(t, err)

	sum := sha256.Sum256(raw)
	sha8 := hex.EncodeToString(sum[:])[:8]
	assert.Equal(t, "authority/"+sha8+"-weird-name.md", docID)

	// Both targets landed where the doc_id says.
	_, err = os.Stat(filepath.Join(kbRoot, docID))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(uploadsDir, hex.EncodeToString(sum[:])+".md"))
	require.NoError(t, err)
}
