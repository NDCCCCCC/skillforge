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
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// importDisabledError is the fail-closed envelope returned when the import
// orchestrator is not wired (SKILLFORGE_DATA_SALT unset).
const importDisabledError = "import disabled — SKILLFORGE_DATA_SALT unset"

// importPreviewHandler implements POST /api/import/preview.
// Placeholder for Task 1 route registration; full implementation in Task 2.
func (s *Deps) importPreviewHandler(ctx context.Context, c *app.RequestContext) {
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}
	c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "not implemented"})
}

// importCommitHandler implements POST /api/import/commit.
// Placeholder for Task 1 route registration; full implementation in Task 2.
func (s *Deps) importCommitHandler(ctx context.Context, c *app.RequestContext) {
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}
	c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "not implemented"})
}

// importPushHandler implements POST /api/import/push.
// Placeholder for Task 1 route registration; full implementation in Task 2.
func (s *Deps) importPushHandler(ctx context.Context, c *app.RequestContext) {
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}
	c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "not implemented"})
}

// importDeleteHandler implements DELETE /api/import/dataset/:name.
// Placeholder for Task 1 route registration; full implementation in Task 3.
func (s *Deps) importDeleteHandler(ctx context.Context, c *app.RequestContext) {
	if s.Importer == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: importDisabledError})
		return
	}
	c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "not implemented"})
}
