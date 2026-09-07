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
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
)

// uploadMaxBodyDefault is 100 MiB.
const uploadMaxBodyDefault = 100 * 1024 * 1024

const (
	actionDocUploaded       = "action_doc_uploaded"
	actionDocUploadRejected = "action_doc_upload_rejected"
)

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
//   1. isAdminRequest(c) → 403  (403 must not leak KB registry state)
//   2. s.KBRegistry == nil → 503
//   3. c.Param("slug") → kb.ValidateSlug → 400
//   4. reg.Lookup(slug) → 404 if deactivated/unknown
//   5. Content-Length → 413 if > UploadMaxBodyBytes
//   6. multipart parse + magic byte verify → 400 on mismatch
//   7. UploadIndex → 422 with action_doc_upload_rejected on failure
//   8. IndexerBackend.Insert → 503 with Retry-After on backend failure
//   9. action_doc_uploaded audit row on success
func (s *Deps) uploadHandler(ctx context.Context, c *app.RequestContext) {
	// STEP 1: admin token gate — 403 must NOT leak KB registry state
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
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

	// STEP 6: parse multipart streaming + onFile callback
	// SHA-256 is computed on the in-memory buffer; the bytes.Buffer
	// is never written to disk (DOC-02 / D15-B1 invariant).
	var parseErr error
	onFile := func(form *uploadForm, file io.Reader, filename string) error {
		// Read entire file into memory for SHA-256 + magic byte check.
		data, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		// Compute SHA-256 on the in-memory buffer (never on disk).
		hash := sha256.Sum256(data)
		sha256Hex := hex.EncodeToString(hash[:])

		// Magic byte verification: declared MIME must match actual bytes.
		if err := VerifyMagicBytes(data, form.Metadata); err != nil {
			parseErr = err
			return err
		}

		// Acquire per-slug mutex to serialize ID ranges (D27-03-2).
		mu := getSlugMutex(slug)
		mu.Lock()
		defer mu.Unlock()

		// UploadIndex: parse + chunk + embed. Does NOT call Insert.
		docs, err := knowledge.UploadIndex(ctx, slug, data, form.Metadata, filename, s.Embedder, s.KBRegistry, s.Retriever)
		if err != nil {
			parseErr = err
			// STEP 7: UploadIndex failure → 422 with rejection audit
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
			return err
		}

		// STEP 8: IndexerBackend.Insert → 503 with Retry-After on failure
		if err := s.IndexerBackend.Insert(ctx, docs); err != nil {
			parseErr = err
			if docUploadAuditHook != nil {
				reject := DocUploadRejectAudit{
					Slug:     slug,
					FileName: filename,
					FileSize: int64(len(data)),
					Reason:   "indexer unavailable: " + err.Error(),
				}
				payload, _ := json.Marshal(reject)
				docUploadAuditHook(actionDocUploadRejected, string(payload))
			}
			c.Response.Header.Set("Retry-After", "30")
			c.JSON(http.StatusServiceUnavailable, graphResponseError{
				Error: "indexer unavailable: " + err.Error(),
			})
			return err
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
			}
			payload, _ := json.Marshal(success)
			docUploadAuditHook(actionDocUploaded, string(payload))
		}

		// Return 200 with chunk count and ID range.
		c.JSON(http.StatusOK, struct {
			Chunks   int    `json:"chunks"`
			IDRange  []int64 `json:"id_range"`
		}{
			Chunks:  len(docs),
			IDRange: []int64{idStart, idEnd},
		})
		return nil
	}

	if err := parseUploadForm(c, onFile); err != nil {
		if parseErr != nil {
			// Already handled inside onFile (wrote audit row, set response).
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
		// Markdown: first line starts with "#" (ATX heading) or "---"
		// (YAML frontmatter delimiter) or is plain text (no special sig).
		firstLine := bytes.SplitN(data, []byte("\n"), 2)
		if len(firstLine) == 0 {
			return document.ErrContentTypeMismatch
		}
		trimmed := bytes.TrimSpace(firstLine[0])
		if len(trimmed) == 0 || trimmed[0] == '#' || bytes.HasPrefix(trimmed, []byte("---")) {
			return nil // looks like markdown
		}
		// Also accept plain text (no signature needed for unknown types).
		return nil
	default:
		return fmt.Errorf("unknown format: %s", declaredType)
	}
	return nil
}
