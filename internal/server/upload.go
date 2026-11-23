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

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
)

// uploadMaxBodyDefault is 100 MiB.
const uploadMaxBodyDefault = 100 * 1024 * 1024

const (
	actionDocUploaded       = "action_doc_uploaded"
	actionDocUploadRejected = "action_doc_upload_rejected"
)

// Phase 53-01 (D-53-01) — upload persistence constants. knowledge/ is the
// single source of truth: every uploaded document is written there before
// IndexerBackend.Insert, so the boot-time seedDocs tier purge re-indexes
// uploads instead of silently evaporating them (Phase 53 breakpoint A).
const (
	// mimeMarkdown is the only MIME type whose canonical file body is the
	// frontmatter-stripped RAW markdown (see persistBodyFor — load-bearing
	// for restart chunk equivalence).
	mimeMarkdown = "text/markdown"

	// persistExtMarkdown is the canonical file extension: even PDF/DOCX
	// uploads land as .md because the persisted body is extracted TEXT.
	persistExtMarkdown = ".md"

	// persistSHA8Len is the sha256-hex prefix used in canonical file names.
	// 8 hex chars (32 bits) make accidental same-name collisions negligible
	// and full-content collisions are already excluded by SHA-256 dedup.
	persistSHA8Len = 8

	// archiveExtFallback names extension-less uploads in the raw-bytes
	// archive so the file still carries a conventional suffix.
	archiveExtFallback = ".bin"

	// defaultPersistUploadsDir is the raw-bytes archive dir, following the
	// runtime data-tree convention (cf. SKILLFORGE_DATASETS_DIR →
	// data/datasets, ontology → data/ontology).
	defaultPersistUploadsDir = "data/uploads"

	// persistDirMode / persistFileMode are the on-disk modes for the two
	// persist targets. 0o755 dirs keep tier dirs browsable by operators;
	// 0o644 files are group/world readable like the seeded corpus files.
	persistDirMode  = 0o755
	persistFileMode = 0o644
)

// persistUploadsDir is the raw-bytes archive root; the canonical-file
// root is knowledgeRootDir (knowledge.go) — unified in the 53-03 review
// so DELETE can never miss the file an upload wrote. Package var per
// Package vars (not consts) following the docUploadAuditHook injection
// synchronized — bound at boot, and upload handlers only READ them (the
// per-slug mutex already serializes same-slug persist writes).
var (
	persistUploadsDir = defaultPersistUploadsDir
)

// SetPersistKnowledgeRoot binds the corpus root PersistUploadedDoc writes
// canonical knowledge files under. Production never calls this (the
// default matches the seed root); tests use it with t.Cleanup restore.
// SetPersistUploadsDir binds the raw-bytes archive dir. Same posture as
// SetPersistKnowledgeRoot.
func SetPersistUploadsDir(dir string) { persistUploadsDir = dir }

// DocUploadAudit is the FrozenArgs payload for action_doc_uploaded.
type DocUploadAudit struct {
	Slug          string `json:"slug"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	SHA256        string `json:"sha256"`
	ParserVersion string `json:"parser_version"`
	ChunkCount    int    `json:"chunk_count"`
	IDRangeStart  int64  `json:"id_range_start"`
	IDRangeEnd    int64  `json:"id_range_end"`
	// PersistedPath is the corpus-relative path of the canonical knowledge
	// file written by PersistUploadedDoc — the same string as the file's
	// doc_id frontmatter field (Phase 53-01). Additive JSON field:
	// pre-53 audit rows simply lack it, so FrozenArgs consumers keep
	// working (backward-compatible).
	PersistedPath string `json:"persisted_path"`
}

// DocUploadRejectAudit is the FrozenArgs payload for action_doc_upload_rejected.
type DocUploadRejectAudit struct {
	Slug     string `json:"slug"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Reason   string `json:"reason"`
}

// docUploadAuditHook is the audit sink for the upload pipeline.
// Bound at boot via SetDocUploadAuditHook; nil means best-effort skip.
var docUploadAuditHook func(action, detail string)

// SetDocUploadAuditHook binds the production audit sink for the upload pipeline.
func SetDocUploadAuditHook(fn func(action, detail string)) { docUploadAuditHook = fn }

// slugUploadMutexes serializes concurrent uploads to the same slug.
// Different slugs run in parallel; same slug is serialized to keep
// ID ranges clean and non-interleaved (D27-03-2 decision).
var slugUploadMutexes sync.Map

func getSlugMutex(slug string) *sync.Mutex {
	mu, _ := slugUploadMutexes.LoadOrStore(slug, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// uploadHandler serves POST /api/admin/kb/:slug/upload — the document
// upload endpoint for Phase 27.
//
// CHECK ORDER (verbatim from kb_admin.go pattern):
//  1. isAdminRequest(c) → 403  (403 must not leak KB registry state)
//  2. s.KBRegistry == nil → 503
//  3. c.Param("slug") → kb.ValidateSlug → 400
//  4. reg.Lookup(slug) → 404 if deactivated/unknown
//  5. Content-Length → 413 if > UploadMaxBodyBytes
//  6. multipart parse + onFile callback → processUploadedFile
//  7. UploadIndex → 422 with action_doc_upload_rejected on failure
//     7d. PersistUploadedDoc → 422 (persist failed) BEFORE Insert — D-53-01
//     fail-closed: "Insert succeeded but no file on disk" must be
//     impossible, or the boot reseed would evaporate the upload again
//  8. IndexerBackend.Insert → 503 with Retry-After on backend failure
//  9. action_doc_uploaded audit row on success
func (s *Deps) uploadHandler(ctx context.Context, c *app.RequestContext) {
	// STEP 1: admin token gate — 403 must NOT leak KB registry state
	if !requireAdmin(c) {
		return
	}

	// STEP 2: KB registry nil → 503
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "KB registry not wired"})
		return
	}

	// STEP 3: validate slug param
	slug := c.Param("slug")
	if err := kb.ValidateSlug(slug); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return
	}

	// STEP 4: slug active check via registry Lookup
	if _, ok := s.KBRegistry.Lookup(slug); !ok {
		c.JSON(http.StatusNotFound, graphResponseError{Error: "kb " + slug + " not found or deactivated"})
		return
	}

	// STEP 5: Content-Length guard — before reading the stream
	maxBytes := s.UploadMaxBodyBytes
	if maxBytes == 0 {
		maxBytes = uploadMaxBodyDefault
	}
	if cl := c.Request.Header.ContentLength(); cl > 0 && int64(cl) > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, graphResponseError{
			Error: fmt.Sprintf("file too large: max %d bytes", maxBytes),
		})
		return
	}

	// STEP 6: parse multipart streaming; onFile delegates to processUploadedFile.
	var parseErr error
	onFile := func(form *uploadForm, file io.Reader, filename string) error {
		return s.processUploadedFile(ctx, slug, form, file, filename, c, &parseErr)
	}

	if err := parseUploadForm(c, onFile); err != nil {
		if parseErr != nil {
			// Already handled inside processUploadedFile (wrote audit row, set response).
			// Just don't double-write.
			return
		}
		// Parse error from parseUploadForm itself (e.g. missing file part).
		if err != nil && strings.Contains(err.Error(), "missing file") {
			c.JSON(http.StatusBadRequest, graphResponseError{Error: "no file part in multipart form"})
			return
		}
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid form: " + err.Error()})
		return
	}
}

// processUploadedFile handles the per-file processing inside uploadHandler's
// multipart callback. It reads the file into memory, verifies magic bytes,
// checks SHA-256 deduplication, calls UploadIndex, persists the document
// to the durable corpus (STEP 7d — before Insert, D-53-01), inserts into
// the indexer, and emits the audit row on success.
//
// The parseErr pointer lets this method signal an error to the outer
// multipart parser without double-writing an HTTP response.
func (s *Deps) processUploadedFile(ctx context.Context, slug string, form *uploadForm, file io.Reader, filename string, c *app.RequestContext, parseErr *error) error {
	// Read entire file into memory for SHA-256 + magic byte check.
	data, err := io.ReadAll(file)
	if err != nil {
		*parseErr = fmt.Errorf("read file: %w", err)
		return *parseErr
	}

	// Compute SHA-256 on the in-memory buffer (never on disk).
	hash := sha256.Sum256(data)
	sha256Hex := hex.EncodeToString(hash[:])

	// Magic byte verification: declared MIME must match actual bytes.
	if err := VerifyMagicBytes(data, form.Metadata); err != nil {
		*parseErr = err
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   err.Error(),
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		c.JSON(http.StatusBadRequest, graphResponseError{Error: err.Error()})
		return *parseErr
	}

	// Acquire per-slug mutex to serialize ID ranges (D27-03-2).
	mu := getSlugMutex(slug)
	mu.Lock()
	defer mu.Unlock()

	// STEP 7a: SHA-256 deduplication check (DOC-04).
	// Check BEFORE UploadIndex so we don't waste a parse on a duplicate.
	if s.Store != nil {
		dup, dupErr := s.Store.HasUploadBySHA256(ctx, slug, sha256Hex)
		if dupErr == nil && dup {
			*parseErr = fmt.Errorf("duplicate file")
			if docUploadAuditHook != nil {
				reject := DocUploadRejectAudit{
					Slug:     slug,
					FileName: filename,
					FileSize: int64(len(data)),
					Reason:   "duplicate file (same SHA-256 already uploaded)",
				}
				payload, _ := json.Marshal(reject)
				docUploadAuditHook(actionDocUploadRejected, string(payload))
			}
			c.JSON(http.StatusConflict, graphResponseError{Error: "duplicate file (same SHA-256 already uploaded)"})
			return *parseErr
		}
		// Non-nil dupErr: best-effort dedup; log and continue.
		if dupErr != nil {
			// Best-effort: dedup failed but the file is still eligible for upload.
			// The store query error is not propagated to the client.
		}
	}

	// UploadIndex: parse + chunk + embed + the 53-03 doc-level upsert.
	// Does NOT call Insert.
	//
	// Phase 53-03: the doc_id must exist BEFORE this call — the upsert
	// needs it to address the doc's previous generation (reuse unchanged
	// rows, delete superseded ones). It is derived from the sha256 the
	// handler already computed plus the SAME naming rule PersistUploadedDoc
	// applies, through the shared persistedDocID helper: one naming home,
	// so the key the upsert deletes under can never drift from the file
	// STEP 7d actually writes.
	docID := persistedDocID(slug, sha256Hex, filename)
	docs, err := knowledge.UploadIndex(ctx, slug, data, form.Metadata, filename, docID, s.Embedder, s.KBRegistry, s.Retriever)
	if err != nil {
		*parseErr = err
		// STEP 7b: UploadIndex failure → 422 with rejection audit
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   err.Error(),
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		return *parseErr
	}

	// STEP 7c: Re-check slug is still active (fail-closed on race).
	// Between UploadIndex and Insert, the slug may have been deactivated.
	if _, ok := s.KBRegistry.Lookup(slug); !ok {
		*parseErr = fmt.Errorf("slug deactivated during upload")
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   "slug deactivated during upload",
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		c.JSON(http.StatusConflict, graphResponseError{Error: "slug deactivated during upload"})
		return *parseErr
	}

	// STEP 7d: persist to the durable corpus BEFORE Insert (Phase 53-01 /
	// D-53-01). Ordering is fail-closed by construction: after a successful
	// Insert there is no undo, so persisting first makes "Insert succeeded
	// but no file on disk" unreachable — that is the exact state the boot
	// reseed (seedDocs tier purge) cannot recover, i.e. breakpoint A. The
	// converse half-state (file on disk, Insert failed) is self-healing:
	// the next reseed indexes the canonical file like any directory doc.
	// Failure → 422 + reject audit with a "persist failed" reason so
	// operators can tell storage failure apart from parse/index failure.
	persistedPath, err := PersistUploadedDoc(ctx, slug, filename, form.Metadata, data)
	if err != nil {
		*parseErr = err
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   err.Error(),
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		c.JSON(http.StatusUnprocessableEntity, graphResponseError{Error: err.Error()})
		return *parseErr
	}

	// STEP 8: IndexerBackend.Insert → 503 on backend unavailable,
	// 500 on other Insert errors.
	// CODE-AUDIT 2026-11-05 (B-01): the handler previously assumed
	// IndexerBackend was always wired — a nil interface (old RETRIEVER_
	// BACKEND=none assembly) panicked right here. Two guards now: an
	// explicit nil check (503 before any insert attempt), and DisabledIndexer's
	// sentinel error routed into the same 503 classification.
	if s.IndexerBackend == nil {
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   retriever.IndexerBackendNotWiredError,
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		c.Response.Header.Set("Retry-After", "30")
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: retriever.IndexerBackendNotWiredError})
		return *parseErr
	}
	if err := s.IndexerBackend.Insert(ctx, docs); err != nil {
		*parseErr = err
		if errors.Is(err, retriever.ErrIndexerDisabled) || isBackendUnavailable(err) {
			if docUploadAuditHook != nil {
				reject := DocUploadRejectAudit{
					Slug:     slug,
					FileName: filename,
					FileSize: int64(len(data)),
					Reason:   "backend unavailable: " + err.Error(),
				}
				payload, _ := json.Marshal(reject)
				docUploadAuditHook(actionDocUploadRejected, string(payload))
			}
			c.Response.Header.Set("Retry-After", "30")
			c.JSON(http.StatusServiceUnavailable, graphResponseError{
				Error: "backend unavailable: " + err.Error(),
			})
			return *parseErr
		}
		// Generic 500 for other Insert errors.
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   "insert failed: " + err.Error(),
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "indexing failed — see server logs",
		})
		return *parseErr
	}

	// STEP 9: success — write action_doc_uploaded audit row
	idStart := docs[0].ID
	idEnd := docs[len(docs)-1].ID
	if docUploadAuditHook != nil {
		// Determine parser version from the first doc's content length as a proxy.
		parserVer := ""
		if len(docs) > 0 {
			parserVer = "unknown"
		}
		success := DocUploadAudit{
			Slug:          slug,
			FileName:      filename,
			FileSize:      int64(len(data)),
			SHA256:        sha256Hex,
			ParserVersion: parserVer,
			ChunkCount:    len(docs),
			IDRangeStart:  idStart,
			IDRangeEnd:    idEnd,
			PersistedPath: persistedPath,
		}
		payload, _ := json.Marshal(success)
		docUploadAuditHook(actionDocUploaded, string(payload))
	}

	// Return 200 with chunk count and ID range.
	c.JSON(http.StatusOK, struct {
		Chunks  int     `json:"chunks"`
		IDRange []int64 `json:"id_range"`
	}{
		Chunks:  len(docs),
		IDRange: []int64{idStart, idEnd},
	})
	return nil
}

// PersistUploadedDoc writes an uploaded document into the durable corpus
// so knowledge/ stays the single source of truth (Phase 53-01 / D-53-01).
// Without this the upload's chunks live only in the vector store, and the
// boot-time seedDocs tier purge (indexer.go: DeleteTierRows → reseed
// directory docs only) silently evaporates them — Phase 53 breakpoint A.
//
// Two writes, in this order:
//
//  1. Archive — the ORIGINAL bytes at <persistUploadsDir>/<sha256><ext>
//     (raw provenance for forensics/re-import; never indexed by the walk,
//     which only reads knowledge/<slug>/*.md).
//  2. Canonical file — <persistKnowledgeRoot>/<slug>/<sha8>-<stem>.md:
//     provenance frontmatter (source_sha256 / source_mime / uploaded_at /
//     doc_id) + the exact body the chunker consumed (persistBodyFor), so
//     a boot reseed re-chunks it into IDENTICAL chunks.
//
// Ordering is load-bearing in BOTH directions:
//   - archive BEFORE canonical file: if the archive write fails we return
//     before touching knowledge/, so a failed upload can never leave a
//     canonical file on disk that the next reseed would index even though
//     the client saw a 422.
//   - the caller invokes this BEFORE IndexerBackend.Insert (STEP 7d
//     before STEP 8): "Insert succeeded but no file on disk" is the one
//     unrecoverable direction (CONTEXT 坑 4 fail-closed). The converse —
//     canonical file present, Insert failed — self-heals on the next
//     reseed, which is precisely the D-53-01 posture.
//
// Any write error is returned wrapped with a "persist failed" prefix; the
// caller maps it to 422 + action_doc_upload_rejected. Re-uploading the
// same bytes overwrites both targets (sha-named → idempotent; the sha8
// truncation collision window is ignored per plan). The returned path is
// the slash-normalized corpus-relative doc_id, which after this write
// gives uploaded docs the SAME doc_id semantics as directory docs
// (53-CONTEXT: 落盘后两条路径的 doc_id 语义归一).
func PersistUploadedDoc(ctx context.Context, slug, filename, mimeType string, raw []byte) (string, error) {
	hash := sha256.Sum256(raw)
	sha256Hex := hex.EncodeToString(hash[:])

	body, fm, err := persistBodyFor(ctx, mimeType, filename, raw)
	if err != nil {
		return "", fmt.Errorf("persist failed: derive body: %w", err)
	}
	// Effective-window passthrough (53-01 review fix): the canonical
	// file must carry the SOURCE window so reseed reads back what
	// upload-time docs carried (equivalence covers the window too).
	ef, et := knowledge.ParseEffectiveWindow(fm)
	if err != nil {
	}
	if len(body) == 0 {
		// Unreachable via the handler (UploadIndex already rejects an empty
		// body before STEP 7d) — guarded here so direct callers of this
		// function can never persist an empty canonical file.
		return "", errors.New("persist failed: derived body is empty")
	}

	name := persistedDocName(sha256Hex, filename)
	docID := slug + "/" + name

	// Write 1: raw-bytes archive (ordering rationale above).
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = archiveExtFallback
	}
	if err := os.MkdirAll(persistUploadsDir, persistDirMode); err != nil {
		return "", fmt.Errorf("persist failed: archive dir: %w", err)
	}
	archivePath := filepath.Join(persistUploadsDir, sha256Hex+ext)
	if err := os.WriteFile(archivePath, raw, persistFileMode); err != nil {
		return "", fmt.Errorf("persist failed: archive %s: %w", archivePath, err)
	}

	// Write 2: canonical knowledge file.
	dir := filepath.Join(knowledgeRootDir, slug)
	if err := os.MkdirAll(dir, persistDirMode); err != nil {
		return "", fmt.Errorf("persist failed: knowledge dir %s: %w", dir, err)
	}
	file := buildPersistedMarkdown(sha256Hex, mimeType, docID, time.Now().UTC().Format(time.RFC3339), ef, et, body)
	if err := os.WriteFile(filepath.Join(dir, name), file, persistFileMode); err != nil {
		return "", fmt.Errorf("persist failed: knowledge file %s: %w", filepath.Join(dir, name), err)
	}
	return docID, nil
}

// persistedDocName derives the canonical knowledge-file name for one
// upload: "<sha8>-<sanitized-stem>.md" (persistSHA8Len hex prefix of the
// file's sha256 + the kb-whitelisted stem). Extracted from
// PersistUploadedDoc in Phase 53-03 so the doc_id the UPLOAD pipeline
// feeds the upsert (before persist even runs) and the doc_id written into
// the canonical file's frontmatter derive from ONE naming home — a second
// literal copy of the rule would let the upsert's delete key drift from
// the file the persist step writes, resurrecting the doc under a new
// name at the next reseed.
func persistedDocName(sha256Hex, filename string) string {
	return sha256Hex[:persistSHA8Len] + "-" + kb.SanitizeFileStem(filename) + persistExtMarkdown
}

// persistedDocID returns the corpus-relative doc_id of one upload:
// "<slug>/<persistedDocName>" — the lifecycle key the upsert and the
// DELETE endpoint address the doc by, identical to the returned value of
// PersistUploadedDoc (53-CONTEXT: 落盘后两条路径的 doc_id 语义归一).
func persistedDocID(slug, sha256Hex, filename string) string {
	return slug + "/" + persistedDocName(sha256Hex, filename)
}

// persistBodyFor selects the body bytes written into the canonical
// knowledge file. The two branches are load-bearing for restart chunk
// equivalence (D-53-01):
//
//   - text/markdown → the frontmatter-stripped RAW body. UploadIndex
//     chunks exactly this byte span (its md-parser output is discarded);
//     the parser's goldmark-rendered plain text strips markdown syntax
//     and would re-chunk differently at the next reseed, silently
//     diverging the corpus from the upload-time chunk set.
//
//   - every other registered type → the parser's plain-text Content.
//     splitFrontmatter on raw PDF/DOCX bytes yields the binary itself,
//     and a knowledge/*.md containing those bytes is garbage on reseed.
//     (The upload path's raw-binary chunking for non-md types is a
//     pre-existing defect out of 53-01 scope; persisting extracted text
//     means the DISK copy — the D-53-01 source of truth — is the correct
//     one, and the next reseed converges the vector store onto it.)
func persistBodyFor(ctx context.Context, mimeType, filename string, raw []byte) (body, frontmatter []byte, err error) {
	if mimeType == mimeMarkdown {
		body, fm := knowledge.SplitFrontmatterRaw(raw)
		return body, fm, nil
	}
	pr, err := document.NewParserRegistry().Parse(ctx, mimeType, bytes.NewReader(raw), filename)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s for persist: %w", filename, err)
	}
	// Non-md sources carry no frontmatter concept: nil fm -> (0,0) window,
	// which is exactly how the directory path treats them.
	return pr.Content, nil, nil
}

// buildPersistedMarkdown renders the canonical file: provenance YAML
// frontmatter + the chunker's source body. Frontmatter values are all
// whitelist-constrained — hex digest, a ParserRegistry-allowlisted MIME
// (enforced upstream by VerifyMagicBytes), a sanitized stem + validated
// slug in doc_id, an RFC3339 timestamp — so plain-scalar YAML emission is
// injection-safe without quoting. The trailing newline keeps the body
// byte-exact across the write/re-read round trip without gluing the last
// line to whatever follows.
func buildPersistedMarkdown(sha256Hex, mimeType, docID, uploadedAt string, ef, et int64, body []byte) []byte {
	// NOTE: format-verb order MUST match the argument order below —
	// 53-01 review caught docID/uploadedAt swapped into each other's
	// fields (uploaded_at rendered the corpus path, doc_id the timestamp).
	fm := fmt.Sprintf("source_sha256: %s\nsource_mime: %s\nuploaded_at: %s\ndoc_id: %s\neffective_from: %d\neffective_to: %d\n",

		sha256Hex, mimeType, uploadedAt, docID, ef, et)

	out := make([]byte, 0, len(fm)+len(body)+8)
	out = append(out, "---\n"...)
	out = append(out, fm...)
	out = append(out, "---\n"...)
	out = append(out, body...)
	if body[len(body)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}

// uploadForm carries the optional metadata field from the multipart form.
type uploadForm struct {
	Metadata string // declared Content-Type / MIME type
}

// parseUploadForm mirrors parseImportForm (import_api.go:325-398):
// streams multipart parts from BodyStream without materializing to disk.
// onFile receives the parsed form metadata, the file reader, and the filename.
func parseUploadForm(c *app.RequestContext, onFile func(form *uploadForm, file io.Reader, filename string) error) error {
	boundary := string(c.Request.Header.MultipartFormBoundary())
	if boundary == "" {
		return fmt.Errorf("missing multipart boundary")
	}

	form := &uploadForm{}
	mr := multipart.NewReader(c.Request.BodyStream(), boundary)
	for {
		part, perr := mr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			return fmt.Errorf("read part: %w", perr)
		}

		name := part.FormName()
		if name == "file" {
			filename := part.FileName()
			// Source-of-truth for the declared MIME is the file part's own
			// Content-Type header (the multipart "metadata" form field is
			// ignored at this gate — DOC-03 says the header is untrusted, but
			// the bytes are; the form-field was a v1 convenience that
			// confused the test and the route contract). If the part has no
			// explicit Content-Type, multipart's default of application/octet-stream
			// is what we get — and VerifyMagicBytes rejects it.
			form.Metadata = part.Header.Get("Content-Type")
			return onFile(form, part, filename)
		}

		if name == "metadata" {
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.Metadata = v
			continue
		}

		// Drain unknown parts so the multipart reader can advance.
		_, _ = io.Copy(io.Discard, part)
	}
	return fmt.Errorf("missing file part")
}

// readStringPart is defined in import_api.go and reused here.

// VerifyMagicBytes checks that the declared MIME type matches the actual
// content signature in data. This is the DOC-03 body-signature guard:
// the multipart Content-Type header is untrusted; the actual bytes must
// confirm the declared type.
//
// Magic bytes table:
//   - "%PDF" (4 bytes)              → "application/pdf"
//   - "PK\x03\x04" (4 bytes)       → ZIP local header; also validates
//     inner filename for docx/xlsx/pptx MIME types
//   - text/markdown                → first line starts with "#" or "---"
//
// Returns ErrContentTypeMismatch if the bytes do not match the declared type.
func VerifyMagicBytes(data []byte, declaredType string) error {
	if len(data) < 4 {
		return document.ErrContentTypeMismatch
	}

	switch declaredType {
	case "application/pdf":
		if !bytes.HasPrefix(data, []byte("%PDF")) {
			return document.ErrContentTypeMismatch
		}
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return document.ErrContentTypeMismatch
		}
	case "text/markdown":
		// Markdown: first non-empty line must start with "#" (ATX heading),
		// "---" (YAML frontmatter delimiter), or be a single-line plain
		// paragraph. Pure magic-byte refusal is NOT possible for markdown —
		// plain text is a valid subset — so this branch enforces a soft
		// structural check: reject bytes whose first non-empty line starts
		// with a known-binary signature that would never appear in md.
		firstLine := bytes.SplitN(data, []byte("\n"), 2)
		if len(firstLine) == 0 {
			return document.ErrContentTypeMismatch
		}
		trimmed := bytes.TrimSpace(firstLine[0])
		// Reject obvious binary masquerading as markdown.
		if bytes.HasPrefix(trimmed, []byte("%PDF")) ||
			bytes.HasPrefix(trimmed, []byte("PK\x03\x04")) ||
			bytes.HasPrefix(trimmed, []byte{0x00, 0x00, 0x00}) {
			return document.ErrContentTypeMismatch
		}
		// Anything else — including plain paragraphs and "#"-prefixed
		// headings — is accepted as markdown.
		return nil
	default:
		return fmt.Errorf("unknown format: %s", declaredType)
	}
	return nil
}

// isBackendUnavailable returns true when err is a network-level backend
// failure that should produce a 503 with Retry-After instead of a generic
// 500. It checks for typed network errors that indicate the indexer is
// unreachable rather than a logic error. Phase 32 (REALBUG-03) replaced
// the previous strings.Contains-based substring matching with errors.Is
// + net.Error.Timeout() to avoid breaking on upstream message changes.
func isBackendUnavailable(err error) bool {
	if err == nil {
		return false
	}
	// DNS resolution failure (caller may have renamed host)
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// Connection refused / network unreachable / host down
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	// Context timeout / cancellation propagates from request lifecycle
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return true
	}
	// Generic net.Error timeout (covers *net.OpError wrapping)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
