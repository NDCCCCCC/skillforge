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

// Plan 15-06 — the HTTP surface for the Phase 15 data-import pipeline:
// POST /api/import/preview, POST /api/import/commit, POST /api/import/push,
// and DELETE /api/import/dataset/:name.
//
// The handlers in this file are intentionally stream-first:
//   - Hertz is configured with WithStreamBody(true) and
//     WithDisablePreParseMultipartForm(true) in server.go. The two options
//     must be enabled together; WithStreamBody alone still pre-parses
//     multipart forms into memory or temp files (Hertz docs).
//   - Handlers read multipart parts with multipart.NewReader over
//     c.Request.BodyStream() and c.Request.MultipartFormBoundary(). They
//     NEVER use c.FormFile or c.Request.MultipartForm — those paths
//     materialize parts on disk for large uploads (T-15-02 / D15-B1).
//
// Security boundaries:
//   - All import mutation endpoints are gated by isAdminRequest
//     (crypto/subtle.ConstantTimeCompare, T-15-21).
//   - If Deps.Importer is nil (SKILLFORGE_DATA_SALT unset), every import
//     endpoint returns 503 fail-closed.
//   - Handler logs contain ONLY batch-level metadata (row count, content
//     sha256, column names, preview/batch ids, elapsed time). SamplePairs
//     and raw row values are NEVER logged (D15-B4 / Pitfall 8).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/data"
)

// importDisabledError is the fail-closed envelope returned when the import
// orchestrator is not wired (SKILLFORGE_DATA_SALT unset).
const importDisabledError = "import disabled — SKILLFORGE_DATA_SALT unset"

// previewResponse is the JSON envelope for POST /api/import/preview.
type previewResponse struct {
	PreviewID     string                `json:"preview_id"`
	Dataset       string                `json:"dataset"`
	Period        string                `json:"period"`
	ProposedTier  string                `json:"proposed_tier"`
	RulesSHA256   string                `json:"rules_sha256"`
	ContentSHA256 string                `json:"content_sha256"`
	RowCount      int64                 `json:"row_count"`
	Columns       []data.PreviewColumn  `json:"columns"`
	SamplePairs   []data.RowPair        `json:"sample_pairs"`
}

// commitResponse is the JSON envelope for POST /api/import/commit and
// POST /api/import/push (non-dry-run).
type commitResponse struct {
	BatchID           string `json:"batch_id"`
	Rows              int64  `json:"rows"`
	Superseded        bool   `json:"superseded"`
	SupersededBatchID string `json:"superseded_batch_id,omitempty"`
}

// dryRunResponse is the JSON envelope for POST /api/import/push?dry_run=true.
type dryRunResponse struct {
	Columns    []data.PreviewColumn `json:"columns"`
	Violations []string             `json:"violations"`
}

// deleteResponse is the JSON envelope for DELETE /api/import/dataset/:name.
type deleteResponse struct {
	Deleted []string `json:"deleted"`
}

// importForm holds the parsed multipart text fields for an import request.
// The file part is never stored here; it is consumed synchronously by the
// onFile callback passed to parseImportForm.
type importForm struct {
	PreviewID       string
	Dataset         string
	Period          string
	Channel         string
	DryRun          bool
	BucketOverrides map[string]float64
}

// importPreviewHandler implements POST /api/import/preview.
//
// Form parts:
//   - dataset (required)
//   - period  (required, YYYY-MM)
//   - channel (required, "csv" or "dump")
//   - file    (required)
//   - bucket_overrides (optional, JSON object mapping column -> width)
func (s *Deps) importPreviewHandler(ctx context.Context, c *app.RequestContext) {
	start := time.Now()
	if !requireAdmin(c) {
		return
	}
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}

	if err := parseImportForm(c, func(form *importForm, file io.Reader) error {
		resp, err := s.Importer.Preview(ctx, data.PreviewRequest{
			Dataset:         form.Dataset,
			Period:          form.Period,
			Channel:         form.Channel,
			Reader:          file,
			BucketOverrides: form.BucketOverrides,
		})
		if err != nil {
			mapImportError(c, err)
			return nil
		}

		// D15-B4 / Pitfall 8: log only batch-level metadata. SamplePairs and raw
		// values are deliberately omitted from the log line.
		cols := columnNames(resp.Columns)
		log.Printf("[OK] import preview: preview_id=%s dataset=%s period=%s channel=%s rows=%d content_sha256=%s columns=%v duration=%s",
			resp.PreviewID, resp.Dataset, resp.Period, form.Channel, resp.RowCount, resp.ContentSHA256, cols, time.Since(start))

		c.JSON(http.StatusOK, previewResponse{
			PreviewID:     resp.PreviewID,
			Dataset:       resp.Dataset,
			Period:        resp.Period,
			ProposedTier:  resp.ProposedTier,
			RulesSHA256:   resp.RulesSHA256,
			ContentSHA256: resp.ContentSHA256,
			RowCount:      resp.RowCount,
			Columns:       resp.Columns,
			SamplePairs:   resp.SamplePairs,
		})
		return nil
	}); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid form: " + err.Error()})
	}
}

// importCommitHandler implements POST /api/import/commit.
//
// Form parts:
//   - preview_id (required)
//   - dataset    (required)
//   - period     (required)
//   - channel    (required)
//   - file       (required, must match previewed content hash)
func (s *Deps) importCommitHandler(ctx context.Context, c *app.RequestContext) {
	start := time.Now()
	if !requireAdmin(c) {
		return
	}
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}

	if err := parseImportForm(c, func(form *importForm, file io.Reader) error {
		result, err := s.Importer.Commit(ctx, data.CommitRequest{
			PreviewID: form.PreviewID,
			Dataset:   form.Dataset,
			Period:    form.Period,
			Channel:   form.Channel,
			Reader:    file,
		})
		if err != nil {
			mapImportError(c, err)
			return nil
		}

		log.Printf("[OK] import commit: batch_id=%s dataset=%s period=%s channel=%s rows=%d superseded=%v superseded_batch_id=%s duration=%s",
			result.BatchID, form.Dataset, form.Period, form.Channel, result.Rows, result.Superseded, result.SupersededBatchID, time.Since(start))

		c.JSON(http.StatusOK, commitResponse{
			BatchID:           result.BatchID,
			Rows:              result.Rows,
			Superseded:        result.Superseded,
			SupersededBatchID: result.SupersededBatchID,
		})
		return nil
	}); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid form: " + err.Error()})
	}
}

// importPushHandler implements POST /api/import/push.
//
// Form parts:
//   - dataset  (required)
//   - period   (required)
//   - channel  (required)
//   - dry_run  (optional, "true" for zero-write validation)
//   - file     (required)
func (s *Deps) importPushHandler(ctx context.Context, c *app.RequestContext) {
	start := time.Now()
	if !requireAdmin(c) {
		return
	}
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}

	if err := parseImportForm(c, func(form *importForm, file io.Reader) error {
		if form.DryRun {
			result, err := s.Importer.DryRun(ctx, data.PushRequest{
				Dataset: form.Dataset,
				Period:  form.Period,
				Channel: form.Channel,
				Reader:  file,
				DryRun:  true,
			})
			if err != nil {
				mapImportError(c, err)
				return nil
			}
			cols := columnNames(result.Columns)
			log.Printf("[OK] import push dry_run: dataset=%s period=%s channel=%s columns=%v violations=%d duration=%s",
				form.Dataset, form.Period, form.Channel, cols, len(result.Violations), time.Since(start))
			c.JSON(http.StatusOK, dryRunResponse{
				Columns:    result.Columns,
				Violations: result.Violations,
			})
			return nil
		}

		result, err := s.Importer.Push(ctx, data.PushRequest{
			Dataset: form.Dataset,
			Period:  form.Period,
			Channel: form.Channel,
			Reader:  file,
		})
		if err != nil {
			mapImportError(c, err)
			return nil
		}

		log.Printf("[OK] import push: batch_id=%s dataset=%s period=%s channel=%s rows=%d superseded=%v superseded_batch_id=%s duration=%s",
			result.BatchID, form.Dataset, form.Period, form.Channel, result.Rows, result.Superseded, result.SupersededBatchID, time.Since(start))

		c.JSON(http.StatusOK, commitResponse{
			BatchID:           result.BatchID,
			Rows:              result.Rows,
			Superseded:        result.Superseded,
			SupersededBatchID: result.SupersededBatchID,
		})
		return nil
	}); err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid form: " + err.Error()})
	}
}

// importDeleteHandler implements DELETE /api/import/dataset/:name.
//
// The :name parameter is either an exact registry name ("loans__2026-07")
// or a logical dataset name ("loans"); the latter deletes every period of
// that dataset. The handler Detaches the dataset from the lazy ATTACH cache
// BEFORE os.Remove so the delete succeeds on Windows (Pitfall 9).
func (s *Deps) importDeleteHandler(ctx context.Context, c *app.RequestContext) {
	start := time.Now()
	if !requireAdmin(c) {
		return
	}
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "missing dataset name"})
		return
	}

	deleted, err := s.Importer.DeleteDataset(ctx, name)
	if err != nil {
		if errors.Is(err, data.ErrDatasetNotFound) {
			c.JSON(http.StatusNotFound, graphResponseError{Error: "dataset not found"})
			return
		}
		log.Printf("[WARN] import delete error: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "delete failed — see server logs"})
		return
	}

	log.Printf("[OK] import delete: name=%s deleted=%d targets=%v duration=%s",
		name, len(deleted), deleted, time.Since(start))
	c.JSON(http.StatusOK, deleteResponse{Deleted: deleted})
}

// parseImportForm reads the multipart form in streaming fashion. Small text
// fields are collected into an importForm; when the "file" part is reached,
// onFile is called with the populated form and the live part reader. The
// caller consumes the part reader inside onFile, keeping the entire upload
// out of memory and off disk. Any part after "file" is rejected.
//
// This function is the reason Hertz must be configured with
// WithDisablePreParseMultipartForm(true): the standard c.Request.MultipartForm
// path would spill large parts to os.TempDir().
func parseImportForm(c *app.RequestContext, onFile func(form *importForm, file io.Reader) error) error {
	boundary := string(c.Request.Header.MultipartFormBoundary())
	if boundary == "" {
		return fmt.Errorf("missing multipart boundary")
	}

	form := &importForm{BucketOverrides: nil}
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
		// Once the file part is reached, hand it to the callback and return
		// its result. The callback must consume the part reader before
		// responding; this keeps the multipart reader active in the request
		// goroutine and avoids cross-goroutine body-stream reads.
		if name == "file" {
			return onFile(form, part)
		}

		switch name {
		case "preview_id":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.PreviewID = v
		case "dataset":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.Dataset = v
		case "period":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.Period = v
		case "channel":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.Channel = v
		case "dry_run":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			form.DryRun = strings.EqualFold(v, "true")
		case "bucket_overrides":
			v, err := readStringPart(part)
			if err != nil {
				return err
			}
			if v != "" {
				if err := json.Unmarshal([]byte(v), &form.BucketOverrides); err != nil {
					return fmt.Errorf("invalid bucket_overrides JSON: %w", err)
				}
			}
		default:
			// Drain unknown parts so the multipart reader can advance.
			_, _ = io.Copy(io.Discard, part)
		}
	}
	return fmt.Errorf("missing file part")
}

func readStringPart(p *multipart.Part) (string, error) {
	b, err := io.ReadAll(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// mapImportError translates data.Importer errors into HTTP status codes.
// Structured business-rule rejections (unknown columns, period format,
// hash mismatch, etc.) map to 400; everything else is 500.
func mapImportError(c *app.RequestContext, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "preview session not found"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "hash mismatch"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "unknown column"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "missing column"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "YYYY-MM"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "invalid dataset identifier"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	case strings.Contains(msg, "cannot parse"):
		c.JSON(http.StatusBadRequest, graphResponseError{Error: msg})
	default:
		log.Printf("[WARN] import handler error: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "import processing failed — see server logs"})
	}
}

func columnNames(cols []data.PreviewColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}
