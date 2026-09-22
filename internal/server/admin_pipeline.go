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
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/store"
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

// putPipelineAuditRow writes the INSERT-only audit row for one ACCEPTED
// ECL pipeline trigger — action_ecl_run / action_ecl_retry (WR-05 /
// T-3-AUDIT). Run and retry are the most expensive and most destructive
// admin mutations in the system (whole-generation DeleteByExpr purges,
// ~26 LLM calls per run, TBox-adjacent KG writes); without a row the
// operator cannot answer "who ran the pipeline when" from the audit table
// (SC-3 / GET /api/admin/audit filters on ActorType "admin", which this
// row carries).
//
// Emission point is the handler AT ACCEPTANCE, mirroring
// putKBAdminAuditRow: the auditable fact is the admin's decision to spend
// the budget, and it must not depend on the async outcome (the work runs
// detached — WR-01 — and may outlive this request or the process). The
// work's OUTCOME lives in the ecl_failures ledger (per-note stage/status)
// and the goroutine's WARN logs; the detail payload here records the
// trigger identity ({"trigger":"admin_api"} / {"note_id":"NNN"}).
//
// Best-effort, mirroring every other audit write in this package (chat.go,
// kb_admin.go): a failed write is WARN-logged and NEVER rolls the accepted
// trigger back or fails the request. A nil Store means no audit backend is
// configured (degraded mode, same as elsewhere).
func (s *Deps) putPipelineAuditRow(ctx context.Context, action string, detail map[string]string) {
	if s.Store == nil {
		return
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		log.Printf("[WARN] server: ecl pipeline audit row marshal (%s): %v (row skipped)", action, err)
		return
	}
	if _, err := s.Store.PutAudit(ctx, store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "admin",
		Action:     action,
		ActorType:  "admin",
		FrozenArgs: payload,
	}); err != nil {
		log.Printf("[WARN] server: ecl pipeline audit row (%s): %v (best-effort, trigger not rolled back)", action, err)
	}
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
	// WR-05: one INSERT-only admin audit row per ACCEPTED run (the work
	// itself is detached below; see putPipelineAuditRow for the emission
	// point rationale).
	s.putPipelineAuditRow(ctx, "action_ecl_run", map[string]string{"trigger": "admin_api"})
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
// another retry holds the running flag) maps to 409. All three gates run
// INLINE via TryClaimRetry so the D-Route-01 status mapping is preserved;
// only the long-running pipeline work (RetryClaimed — 3 LLM calls + up to
// 15s of retry backoff + a Milvus purge/embed/seed round-trip at worst)
// detaches onto a background-context goroutine (WR-01, mirroring the run
// handler), because holding the request goroutine that long aborts
// mid-pipeline at proxy/client timeouts and strands the note's ledger row.
// The retry is idempotent via note-level replace (D-Run-01); progress
// flows through the same status polling as run (D-Prog-01). All three
// routes are body-less — no request-body binding exists by design
// (D-Route-01).
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
	if err := mgr.TryClaimRetry(noteID); err != nil {
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
	go func() {
		if err := mgr.RetryClaimed(context.Background(), noteID); err != nil {
			log.Printf("[WARN] server: ecl pipeline retry %s failed: %v", noteID, err)
		}
	}()
	// WR-05: one INSERT-only admin audit row per ACCEPTED retry, with the
	// note id as the trigger identity.
	s.putPipelineAuditRow(ctx, "action_ecl_retry", map[string]string{"note_id": noteID})
	c.JSON(http.StatusOK, pipelineRetryResponse{Status: "queued", NoteID: noteID})
}
