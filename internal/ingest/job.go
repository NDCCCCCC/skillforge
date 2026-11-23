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

// Package ingest is the async ingestion pipeline (Phase 54 / ING-04/05):
// an upload becomes a JOB whose eight steps (parse → chunk → embed →
// vector_insert → ecl_merge → ecl_extract → ecl_kg_load → ecl_milvus_load)
// each land as a queryable row (status / attempt / timestamps / error /
// artifact_ref) in ingest_steps, exposed via
// GET /api/admin/ingest/jobs/:id.
//
// Failure semantics are deliberately asymmetric (D-54-01): embedding is a
// NECESSITY — an embed failure fails the job fail-closed, never degraded;
// KG/ECL are ENHANCEMENTS — their failure (this phase: their being skipped
// entirely, since the auto-trigger is 54-04 scope) must not block the
// vector rows from being searchable.
//
// The pipeline is fed through seams (JobStore, VectorSink, ECLTrigger)
// mirroring the ecl.Manager SeedFunc posture: Milvus/LLM details stay
// behind the seam, so tests run the full state machine on fakes.
package ingest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// StepSequence is the LOCKED ordering of the eight pipeline steps (SC 1 /
// SC 5). The ingest_steps table stores no sequence column — consumers
// order rows by this slice's index, so inserting a step later cannot
// rewrite history. The four ecl_* names reuse the 39-UI-SPEC stage
// sentinels' vocabulary family and are LOCKED too (D-55-01: extension,
// never renaming).
var StepSequence = []string{
	StepParse,
	StepChunk,
	StepEmbed,
	StepVectorInsert,
	StepECLMerge,
	StepECLExtract,
	StepECLKGLoad,
	StepECLMilvusLoad,
}

// Step names (the `step` column vocabulary).
const (
	StepParse         = "parse"
	StepChunk         = "chunk"
	StepEmbed         = "embed"
	StepVectorInsert  = "vector_insert"
	StepECLMerge      = "ecl_merge"
	StepECLExtract    = "ecl_extract"
	StepECLKGLoad     = "ecl_kg_load"
	StepECLMilvusLoad = "ecl_milvus_load"
)

// StepIndex returns the StepSequence position of step, or -1 for an
// unknown step name. The sqlite store uses it to return ListSteps rows in
// pipeline order.
func StepIndex(step string) int {
	for i, s := range StepSequence {
		if s == step {
			return i
		}
	}
	return -1
}

// Step statuses (the ingest_steps.status vocabulary).
const (
	// StepPending — not yet reached. The default written by CreateSteps;
	// a step left pending after the job reached a terminal state means
	// "never ran" (the CR-01 contract: no step is ever left running).
	StepPending = "pending"
	// StepRunning — in flight. A step observed running WITHOUT a live
	// runner is a crash artifact; 54-03's recovery sweep owns reclaiming
	// it (Phase 54 SC 8 — this phase never writes running without a live
	// goroutine).
	StepRunning = "running"
	// StepDone — completed successfully; artifact_ref points at the
	// product (provisional inline refs this phase, on-disk staging in
	// 54-02 / D-54-02).
	StepDone = "done"
	// StepFailed — errored; the reason is in error (a first-class query
	// field — Dify lesson). A failed REQUIRED step (parse/chunk/embed/
	// vector_insert) fails the whole job fail-closed (D-54-01).
	StepFailed = "failed"
	// StepSkipped — intentionally not executed. The four ecl_* steps are
	// skipped while the ECL auto-trigger is unwired (54-04); skipped is
	// an ENHANCEMENT posture — it never blocks job=done (D-54-01).
	StepSkipped = "skipped"
	// StepStale — RESERVED for 54-03 (constant defined now so the state
	// vocabulary is stable across the phase): a downstream result
	// invalidated by an upstream retry (the Dify re_segment analog).
	// No writer exists in this phase.
	StepStale = "stale"
)

// Job statuses (the ingest_jobs.status vocabulary).
const (
	// StatusQueued — rows exist, no goroutine claimed it yet.
	StatusQueued = "queued"
	// StatusRunning — a runner holds the ClaimJob CAS.
	StatusRunning = "running"
	// StatusPartial — RESERVED for 54-03 (SC 7: job completed with some
	// items failed, surfaced separately). ClaimJob treats it as
	// re-claimable so a partial job can be resumed by a retry.
	StatusPartial = "partial"
	// StatusDone — every step done or skipped; the success audit row
	// (action_doc_uploaded, migrated from upload.go STEP 9) is written
	// at this transition.
	StatusDone = "done"
	// StatusFailed — a required step failed (D-54-01 fail-closed) or the
	// runner panicked. Terminal this phase; 54-03's retry endpoint
	// re-enters via the failed step.
	StatusFailed = "failed"
)

// Job is one ingest run (the ingest_jobs row projection). Timestamps are
// RFC3339Nano UTC strings SET BY THE CALLER — the runner formats them, the
// store writes them verbatim (ecl_failures.updated_at discipline: caller-
// set timestamps keep retry timing testable).
type Job struct {
	JobID      string // "ing-" + 24 hex chars (NewJobID)
	Slug       string
	Filename   string
	SHA256     string
	Mime       string
	FileSize   int64
	DocID      string // parse step assigns (the 53-01 persist relative path)
	SourcePath string // data/uploads archive path, recorded by the parse step
	ParserName string // parse step captures ParseResult.ParserName
	Status     string
	Error      string
	CreatedAt  string
	UpdatedAt  string
}

// StepRow is one (job, step) pipeline row (the ingest_steps projection).
type StepRow struct {
	JobID       string
	Step        string
	Status      string
	Attempt     int64 // incremented at every start, cumulative (54-03 retry reads it)
	StartedAt   string
	CompletedAt string
	Error       string // failure reason OR the skip reason (e.g. "ecl auto-trigger not wired (54-04)")
	ArtifactRef string // parse → doc_id; vector_insert → written PK set; 54-02 fixes on-disk staging semantics
}

// ItemRow is one item-level ledger row under a step (the ingest_items
// projection; SC 7). Defined this phase so the wire/schema vocabulary is
// stable; the first writer is 54-02 (embed per-chunk ledger, item_key =
// chunk content_hash). vector_insert is batch-atomic (no item rows) and
// the ECL item ledger is the existing ecl_failures table.
type ItemRow struct {
	JobID     string
	Step      string
	ItemKey   string
	Status    string
	Error     string
	UpdatedAt string
}

// jobIDPrefix / jobIDRandBytes size the NewJobID identifier: 12 random
// bytes → 24 hex chars after the "ing-" prefix (54-01 plan DDL comment).
const (
	jobIDPrefix    = "ing-"
	jobIDRandBytes = 12
)

// NewJobID mints a fresh job identifier: "ing-" + crypto/rand 12-byte hex.
// Random (not derived from the sha256) because the same file may
// legitimately produce multiple jobs over its lifecycle (a failed job must
// not consume the identity of its eventual successful retry).
func NewJobID() (string, error) {
	buf := make([]byte, jobIDRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("ingest: job id rand: %w", err)
	}
	return jobIDPrefix + hex.EncodeToString(buf), nil
}
