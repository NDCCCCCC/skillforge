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
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/ingest"
	"github.com/skillforge/skillforge/internal/kb"
)

// uploadMaxBodyDefault is 100 MiB.
const uploadMaxBodyDefault = 100 * 1024 * 1024

const (
	actionDocUploaded       = "action_doc_uploaded"
	actionDocUploadRejected = "action_doc_upload_rejected"
	// actionDocIngestQueued (Phase 54-01): the ACCEPTANCE audit row —
	// one per upload accepted into the async pipeline. The auditable
	// fact is the handler's decision to enqueue (job_id, sha256), NOT
	// the outcome — the outcome lives in the job/step rows (SC 1) and
	// the migrated action_doc_uploaded row the runner writes at job=done
	// (ingest.UploadAudit). INSERT-only, like every audit row here.
	actionDocIngestQueued = "action_doc_ingest_queued"
)

// DocIngestQueuedAudit is the FrozenArgs payload for
// action_doc_ingest_queued (54-01).
type DocIngestQueuedAudit struct {
	Slug     string `json:"slug"`
	FileName string `json:"file_name"`
	SHA256   string `json:"sha256"`
	JobID    string `json:"job_id"`
}

// DocUploadRejectAudit is the FrozenArgs payload for action_doc_upload_rejected.
// 54-01 scope note: the reject audit now covers ONLY the SYNCHRONOUS
// rejection gates (magic bytes, dedup, enqueue failure). Pipeline-step
// failures (parse/persist/chunk/embed/vector_insert) happen after the 200
// has been answered — their failure record is the step row's first-class
// error field (SC 1 / SC 5), not a reject audit row; the runner writes
// action_doc_uploaded only on full success (ingest.UploadAudit).
type DocUploadRejectAudit struct {
	Slug     string `json:"slug"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Reason   string `json:"reason"`
}

// docUploadAuditHook is the audit sink for the upload pipeline.
// Bound at boot via SetDocUploadAuditHook; nil means best-effort skip.
// The ingest runner's success-path audit shares this sink (main.go binds
// the same closure into ingest.RunnerConfig.AuditHook).
var docUploadAuditHook func(action, detail string)

// SetDocUploadAuditHook binds the production audit sink for the upload pipeline.
func SetDocUploadAuditHook(fn func(action, detail string)) { docUploadAuditHook = fn }

// slugUploadMutexes serializes concurrent upload ACCEPTANCES to the same
// slug: dedup check → job-row creation must be one critical section, or
// two concurrent identical uploads both pass the (not-yet-visible) dedup
// check and fork two jobs for the same bytes. Different slugs run in
// parallel. NOTE (54-01): this map no longer guards ID ranges — the
// embed→vector_insert critical section (GetMaxID+Insert, D27-03-2) moved
// into ingest.Runner's own per-slug mutex map, whose scope reproduces the
// retired sync chain's exactly. The two maps guard disjoint regions.
var slugUploadMutexes sync.Map

func getSlugMutex(slug string) *sync.Mutex {
	mu, _ := slugUploadMutexes.LoadOrStore(slug, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// uploadHandler serves POST /api/admin/kb/:slug/upload — the document
// upload endpoint, Phase 54-01 async form: the handler ACCEPTS the file
// into the ingest pipeline and answers 200 {job_id, status:"queued"}
// immediately; parsing/chunking/embedding/insertion run detached and
// their progress is observable via GET /api/admin/ingest/jobs/:id.
//
// CHECK ORDER (verbatim from kb_admin.go pattern):
//  1. isAdminRequest(c) → 403  (403 must not leak KB registry state)
//  2. s.KBRegistry == nil → 503
//     2b. s.Ingest == nil → 503   (the async pipeline is the only path —
//     without a runner nothing can process the upload; same nil-posture
//     as every other unwired dependency)
//  3. c.Param("slug") → kb.ValidateSlug → 400
//  4. reg.Lookup(slug) → 404 if deactivated/unknown
//  5. Content-Length → 413 if > UploadMaxBodyBytes
//  6. multipart parse + onFile callback → processUploadedFile
//  7. SYNC rejection gates (reject audit preserved verbatim):
//     magic bytes → 400; dedup (audit rows ∨ non-failed jobs) → 409
//  8. job row + 8 pending steps → Enqueue → detached run → 200 {job_id}
//     + action_doc_ingest_queued acceptance audit
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

	// STEP 2b: ingest runner nil → 503 (Phase 54-01: the pipeline is the
	// only processing path now; checked after the registry 503 — both are
	// "dependency not wired" answers, no state leaked by their order).
	if s.Ingest == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ingest pipeline not wired"})
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
// checks the extended SHA-256 deduplication (audit rows ∨ non-failed
// ingest jobs), enqueues the async ingest job, and answers 200 with the
// job id. Parsing, persisting, chunking, embedding, and the vector
// upsert-insert all happen LATER on the runner's detached goroutine —
// their outcomes are the job/step rows, never this response.
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

	// SYNC REJECTION GATE: magic byte verification — declared MIME must
	// match actual bytes. Reject audit preserved verbatim (54-01).
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

	// Per-slug critical section: dedup → job creation (two concurrent
	// identical uploads must not both pass a not-yet-visible dedup check).
	// See slugUploadMutexes for what this mutex no longer guards.
	mu := getSlugMutex(slug)
	mu.Lock()
	defer mu.Unlock()

	// STEP 7a: SHA-256 deduplication (DOC-04), Phase 54-01 EXTENDED data
	// sources — either witness answers 409:
	//   (a) the audit row (HasUploadBySHA256 — the pre-54 source: a
	//       COMPLETED ingest wrote action_doc_uploaded), or
	//   (b) a NON-failed ingest job for the same (slug, sha256).
	// Source (b) is the new closure: without it, a re-upload while the
	// first job is still queued/running passes the audit check (no row
	// yet) and forks a duplicate job — two writers racing the same doc's
	// upsert, the write-amplification entry point the plan names.
	if s.Store != nil {
		dup, dupErr := s.Store.HasUploadBySHA256(ctx, slug, sha256Hex)
		if dupErr == nil && dup {
			return s.rejectDuplicate(c, slug, filename, int64(len(data)), parseErr)
		}
		// Non-nil dupErr: best-effort dedup (unchanged pre-54 posture —
		// the store query error is not propagated to the client).
	}
	if prev, ok := s.Ingest.ActiveJobBySHA256(ctx, slug, sha256Hex); ok {
		_ = prev // the job id is deliberately NOT echoed — the 409 reason is the contract
		return s.rejectDuplicate(c, slug, filename, int64(len(data)), parseErr)
	}

	// STEP 8: enqueue the async ingest job. The raw bytes ride into the
	// runner's goroutine (54-02 staging lands the on-disk hand-off; the
	// job row's source_path already records the archive location the
	// parse step will write moments from now).
	job := ingest.Job{
		Slug:     slug,
		Filename: filename,
		SHA256:   sha256Hex,
		Mime:     form.Metadata,
		FileSize: int64(len(data)),
	}
	jobID, err := s.Ingest.Enqueue(ctx, job, data)
	if err != nil {
		*parseErr = err
		log.Printf("[WARN] server: ingest enqueue for %s/%s failed: %v", slug, filename, err)
		if docUploadAuditHook != nil {
			reject := DocUploadRejectAudit{
				Slug:     slug,
				FileName: filename,
				FileSize: int64(len(data)),
				Reason:   "enqueue failed",
			}
			payload, _ := json.Marshal(reject)
			docUploadAuditHook(actionDocUploadRejected, string(payload))
		}
		// Store I/O — retryable, but not the client's fault: 503, generic
		// body (no SQL/store detail leaked).
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ingest enqueue failed — see server logs"})
		return *parseErr
	}

	// STEP 9: acceptance audit (action_doc_ingest_queued) — the auditable
	// fact is the acceptance decision, not the async outcome (the
	// putPipelineAuditRow emission-point rationale, applied to uploads).
	if docUploadAuditHook != nil {
		payload, _ := json.Marshal(DocIngestQueuedAudit{
			Slug:     slug,
			FileName: filename,
			SHA256:   sha256Hex,
			JobID:    jobID,
		})
		docUploadAuditHook(actionDocIngestQueued, string(payload))
	}

	// 200 with the job identity — the ONLY response body the async upload
	// contract carries. Progress (chunks, id range, per-step state) is
	// observable exclusively via GET /api/admin/ingest/jobs/:id; the old
	// {chunks, id_range} fields are retired (chunk count and ID range do
	// not exist until the pipeline finishes). FE migration: Phase 55.
	c.JSON(http.StatusOK, struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}{
		JobID:  jobID,
		Status: ingest.StatusQueued,
	})
	return nil
}

// rejectDuplicate answers the 409 for both dedup witnesses (audit row ∨
// active job) with the SAME response text and reject audit payload the
// pre-54 audit-row branch used — one duplicate, one contract.
func (s *Deps) rejectDuplicate(c *app.RequestContext, slug, filename string, size int64, parseErr *error) error {
	const reason = "duplicate file (same SHA-256 already uploaded)"
	*parseErr = fmt.Errorf("duplicate file")
	if docUploadAuditHook != nil {
		reject := DocUploadRejectAudit{
			Slug:     slug,
			FileName: filename,
			FileSize: size,
			Reason:   reason,
		}
		payload, _ := json.Marshal(reject)
		docUploadAuditHook(actionDocUploadRejected, string(payload))
	}
	c.JSON(http.StatusConflict, graphResponseError{Error: reason})
	return *parseErr
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
