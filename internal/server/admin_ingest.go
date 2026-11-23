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

// Phase 54 / Plan 54-01 (ING-04/ING-05) — the ingest job observation
// surface.
//
// Wire contract (X-Admin-Token-gated; 403 before any 503, LOCKED order —
// the admin_pipeline.go requireECL pattern: an unauthenticated caller
// must never learn whether the pipeline is wired):
//
//	GET /api/admin/ingest/jobs/:id
//	  → 200 {job_id, slug, filename, sha256, doc_id, status, error,
//	         created_at, updated_at,
//	         steps: [{step, status, attempt, started_at, completed_at,
//	                  duration_ms, error, artifact_ref}, ...],
//	         failed_items: [...]}
//	  → 404 {"error":"unknown ingest job"}
//	  → 403 {"error":"admin token required"}
//	  → 503 {"error":"ingest pipeline not wired"}
//	  → 500 {"error":"ingest job lookup failed"}   (store I/O — detail
//	                                                logged, never leaked)
//
// The steps array is the FULL eight-step sequence in ingest.StepSequence
// order regardless of execution state (pending rows included) — the FE
// renders the progress bar from the timestamp deltas alone, zero push
// (SC 1 / SC 5: started_at/completed_at ARE the progress data).
// duration_ms is derived HERE from the stored RFC3339Nano strings and is
// never persisted — the store keeps raw instants, the handler derives
// presentation. failed_items is the ingest_items projection (SC 7);
// 54-01 ships the schema but no writer yet, so it is always an empty
// array until 54-02 wires the embed per-chunk ledger.
//
// Read-only by construction: this surface has no verb that mutates ingest
// state (the 54-03 retry endpoint will live beside it, separately
// gated). Trust model mirrors admin_pipeline.go: the job ids are
// server-minted ("ing-<hex>") and :id reaches the store only as a
// parameterized `?` value — never a path component on disk.
package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/ingest"
)

// requireIngest gates a handler on the ingest runner being wired. ORDER
// IS LOCKED (D-Route-01): the 403 admin check fires BEFORE the
// nil-runner 503 (requireECL / requireOntology pattern).
func (s *Deps) requireIngest(c *app.RequestContext) (*ingest.Runner, bool) {
	if !requireAdmin(c) {
		return nil, false
	}
	if s.Ingest == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "ingest pipeline not wired"})
		return nil, false
	}
	return s.Ingest, true
}

// ingestStepDTO is one row of the steps[] array. The json tags are the
// LOCKED wire shape (SC 5 tuple + duration_ms); they live here so there
// is exactly one place owning the field names (the ecl.StatusDTO
// verbatim-serialization precedent — but unlike StatusDTO this DTO is
// derived, not serialized straight from the store row, because
// duration_ms is presentation-only).
type ingestStepDTO struct {
	Step        string `json:"step"`
	Status      string `json:"status"`
	Attempt     int64  `json:"attempt"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	DurationMS  int64  `json:"duration_ms"`
	Error       string `json:"error"`
	ArtifactRef string `json:"artifact_ref"`
}

// ingestJobDTO is the 200 envelope. Fields mirror the ingest_jobs row
// projection plus the steps array and the (currently always-empty)
// failed_items array.
type ingestJobDTO struct {
	JobID       string          `json:"job_id"`
	Slug        string          `json:"slug"`
	Filename    string          `json:"filename"`
	SHA256      string          `json:"sha256"`
	DocID       string          `json:"doc_id"`
	Status      string          `json:"status"`
	Error       string          `json:"error"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Steps       []ingestStepDTO `json:"steps"`
	FailedItems []ingestItemDTO `json:"failed_items"`
}

// ingestItemDTO is one row of the failed_items[] array (the SC 7 item
// projection). No rows exist until 54-02 writes the embed item ledger —
// the field stays in the envelope from day one so the FE shape is stable.
type ingestItemDTO struct {
	Step    string `json:"step"`
	ItemKey string `json:"item_key"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

// adminIngestJobHandler implements GET /api/admin/ingest/jobs/:id.
func (s *Deps) adminIngestJobHandler(ctx context.Context, c *app.RequestContext) {
	runner, ok := s.requireIngest(c)
	if !ok {
		return
	}
	jobID := c.Param("id")

	job, err := runner.GetJob(ctx, jobID)
	if err != nil {
		if runner.IsNotFound(err) {
			c.JSON(http.StatusNotFound, graphResponseError{Error: "unknown ingest job"})
			return
		}
		// IN-04 posture: store I/O detail is logged, never leaked.
		log.Printf("[WARN] server: ingest job %s lookup failed: %v", jobID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "ingest job lookup failed"})
		return
	}

	steps, err := runner.ListSteps(ctx, jobID)
	if err != nil {
		log.Printf("[WARN] server: ingest job %s step listing failed: %v", jobID, err)
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "ingest job lookup failed"})
		return
	}

	dtos := make([]ingestStepDTO, 0, len(steps))
	for _, r := range steps {
		dtos = append(dtos, ingestStepDTO{
			Step:        r.Step,
			Status:      r.Status,
			Attempt:     r.Attempt,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			DurationMS:  durationMS(r.StartedAt, r.CompletedAt),
			Error:       r.Error,
			ArtifactRef: r.ArtifactRef,
		})
	}

	c.JSON(http.StatusOK, ingestJobDTO{
		JobID:     job.JobID,
		Slug:      job.Slug,
		Filename:  job.Filename,
		SHA256:    job.SHA256,
		DocID:     job.DocID,
		Status:    job.Status,
		Error:     job.Error,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
		Steps:     dtos,
		// 54-01 ships no item writer (the embed per-chunk ledger is
		// 54-02) — always empty, present for shape stability (SC 7).
		FailedItems: []ingestItemDTO{},
	})
}

// durationMS derives the wall-clock duration of one step from its stored
// RFC3339Nano stamps. Either stamp missing (pending/running step) → 0.
// Parse failures → 0 (a corrupt stamp must never 500 a read-only
// surface).
func durationMS(startedAt, completedAt string) int64 {
	if startedAt == "" || completedAt == "" {
		return 0
	}
	start, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339Nano, completedAt)
	if err != nil {
		return 0
	}
	d := end.Sub(start).Milliseconds()
	if d < 0 {
		return 0
	}
	return d
}
