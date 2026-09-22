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

// Plan 39-07 — Phase 39 ECL Pipeline admin surface (D-Route-01).
//
// Wire contract (all routes X-Admin-Token-gated; 403 before any 503):
//
//	POST /api/admin/pipeline/run
//	     → 200 {"status":"started"}        (fire-and-forget; progress via
//	                                        status polling, D-Prog-01)
//	     → 409 {"error":"pipeline already running"}
//	GET  /api/admin/pipeline/status
//	     → 200 {stage, current_file, done, total, queue_len,
//	            failures: [{note_id, stage, attempt_count, last_error,
//	                        status, updated_at}, ...]}
//	POST /api/admin/pipeline/retry/:note_id
//	     → 200 {"status":"queued","note_id":"NNN"}
//	     → 400 {"error":"invalid note id"}
//	     → 404 {"error":"unknown or non-dead note"}
//	     → 409 {"error":"pipeline already running"}
//	     → 503 only when the manager is unwired; 403 always first
//
// Every route answers 503 {"error":"ECL pipeline not wired"} when
// deps.ECL is nil, but ONLY AFTER the 403 {"error":"admin token
// required"} check (locked order, D-Route-01 / T-39-19: an
// unauthenticated caller must never learn whether the pipeline is
// wired).
//
// Trust model: run and retry trigger LLM spend + KG/Milvus writes, so
// both sit behind the admin token gate (T-39-18). Run uses a CAS claim
// on the manager so concurrent run requests answer 409 instead of
// queueing (T-39-20). :note_id is format-gated against ^[0-9]{3}$
// before it reaches the manager, and the manager itself requires an
// existing DEAD ledger row (T-39-10 layer 2); the id never touches a
// filesystem path.
package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
)

// noteIDPattern is the defense-in-depth format gate for the :note_id
// path parameter: note ids are zero-padded 3-digit source_idx values
// (T-39-10 layer 2 — kills malformed / traversal-shaped input at the
// door; the manager's DEAD-row requirement is the layer-1 semantic
// check).
var noteIDPattern = regexp.MustCompile(`^[0-9]{3}$`)

// requireECL gates a handler on the ECL pipeline manager being wired.
// ORDER IS LOCKED (D-Route-01): the 403 admin check fires BEFORE the
// nil-manager 503 (admin_ontology.go requireOntology pattern).
func (s *Deps) requireECL(c *app.RequestContext) (*ecl.Manager, bool) {
	if !requireAdmin(c) {
		return nil, false
	}
	if s.ECL == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ECL pipeline not wired"})
		return nil, false
	}
	return s.ECL, true
}

// pipelineRunResponse is the 200 envelope for POST /api/admin/pipeline/run.
type pipelineRunResponse struct {
	Status string `json:"status"`
}

// pipelineRetryResponse is the 200 envelope for
// POST /api/admin/pipeline/retry/:note_id.
type pipelineRetryResponse struct {
	Status string `json:"status"`
	NoteID string `json:"note_id"`
}

// adminPipelineRunHandler implements POST /api/admin/pipeline/run.
//
// D-Prog-01: the response is an immediate 200 {"status":"started"} —
// progress arrives exclusively via GET status polling, never in the run
// response. The run executes in a detached goroutine on a background
// context (a run outlives the HTTP request that triggered it); a failed
// run WARN-logs inside the goroutine — never silent.
func (s *Deps) adminPipelineRunHandler(ctx context.Context, c *app.RequestContext) {
	mgr, ok := s.requireECL(c)
	if !ok {
		return
	}
	if !mgr.CASRunning() {
		c.JSON(http.StatusConflict, graphResponseError{Error: "pipeline already running"})
		return
	}
	go func() {
		if err := mgr.Run(context.Background()); err != nil {
			log.Printf("[WARN] server: ecl pipeline run failed: %v", err)
		}
	}()
	c.JSON(http.StatusOK, pipelineRunResponse{Status: "started"})
}

// adminPipelineStatusHandler implements GET /api/admin/pipeline/status.
//
// The StatusDTO is serialized verbatim — Plan 05 owns the json tags (the
// LOCKED D-Prog-01 wire shape); re-mapping field by field here would
// create a second source of truth for the FE contract.
func (s *Deps) adminPipelineStatusHandler(ctx context.Context, c *app.RequestContext) {
	mgr, ok := s.requireECL(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, mgr.Status())
}

// adminPipelineRetryHandler implements
// POST /api/admin/pipeline/retry/:note_id.
//
// Validation layers (T-39-10): the ^[0-9]{3}$ format gate answers 400
// before the manager is consulted; the manager then requires an existing
// DEAD ledger row (else ErrUnknownNote → 404). ErrPipelineBusy (a run or
// another retry holds the running flag) maps to 409. RetryNote runs
// synchronously in the request goroutine and is idempotent via
// note-level replace (D-Run-01). All three routes are body-less — no
// request-body binding exists by design (D-Route-01).
func (s *Deps) adminPipelineRetryHandler(ctx context.Context, c *app.RequestContext) {
	mgr, ok := s.requireECL(c)
	if !ok {
		return
	}
	noteID := c.Param("note_id")
	if !noteIDPattern.MatchString(noteID) {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid note id"})
		return
	}
	if err := mgr.RetryNote(ctx, noteID); err != nil {
		switch {
		case errors.Is(err, ecl.ErrPipelineBusy):
			c.JSON(http.StatusConflict, graphResponseError{Error: "pipeline already running"})
		case errors.Is(err, ecl.ErrUnknownNote):
			c.JSON(http.StatusNotFound, graphResponseError{Error: "unknown or non-dead note"})
		default:
			c.JSON(http.StatusInternalServerError, graphResponseError{Error: "retry: " + err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, pipelineRetryResponse{Status: "queued", NoteID: noteID})
}
