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

// Phase 54 / Plan 54-01 (ING-04/05) — the ingest job runner: the
// synchronous 53-series upload chain re-expressed as a job state machine.
//
// Execution model (per the plan): Enqueue persists the job + eight
// pending step rows, then runs the pipeline on a DETACHED goroutine with
// a background context (an ingest outlives the HTTP request that
// triggered it — the admin_pipeline.go run-handler posture); the upload
// handler answers 200 {job_id, status:"queued"} immediately and progress
// arrives exclusively via GET /api/admin/ingest/jobs/:id polling
// (D-Prog-01 analog). Inside the goroutine the steps run SEQUENTIALLY —
// full concurrency (per-step worker pools) is 54-02/54-04 scope.
//
// Terminal-state discipline (CR-01, ecl manager.Run verbatim): EVERY
// error path must land in a terminal sentinel. A returned error fails the
// job; a panic is recovered at the goroutine top, marks the in-flight
// step failed, and fails the job (remaining steps stay pending — a step
// left "running" would render the FE timeline as a forever-spinner the
// same way a stuck ECL stage would).
//
// D27-03-2 invariant: the runner holds the per-slug mutex from the START
// of the embed step to the END of vector_insert — GetMaxID (inside
// ResolveReuseAndEmbed) and the Insert (inside the sink) must be ONE
// critical section per slug or two same-slug jobs allocate colliding
// IDs. This reproduces the retired sync chain's mutex scope exactly
// (it held getSlugMutex across UploadIndex(embed)+Insert). The upload
// handler keeps its own short critical section (dedup → CreateJob) under
// the server package's mutex map; the two maps guard disjoint regions
// and never interleave.
package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/retriever"
)

// actionDocUploaded is the success audit action the runner writes at the
// job=done transition — migrated from upload.go STEP 9, which could only
// run after Insert returned; in the async world that moment is "all steps
// terminal" (the chunk count and ID range exist only then).
const actionDocUploaded = "action_doc_uploaded"

// UploadAudit is the FrozenArgs payload for the runner's
// action_doc_uploaded row — the JSON contract is IDENTICAL to the
// retired server.DocUploadAudit (slug/file_name/file_size/sha256/
// parser_version/chunk_count/id_range_start/id_range_end/persisted_path;
// additive fields only ever). It lives in THIS package because the runner
// is the writer now and ingest cannot import server; the json tags are
// the single wire-home (the ecl.Failure godoc pattern).
//
// ParserVersion carries the parse step's REAL ParserResult.ParserName
// (e.g. "eino-ext/pdf@ledongthuc/pdf") — the retired chain hardcoded
// "unknown"; Phase 54 SC 8 names the registry version as the honest
// value, and the parse step already captures it.
type UploadAudit struct {
	Slug          string `json:"slug"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	SHA256        string `json:"sha256"`
	ParserVersion string `json:"parser_version"`
	ChunkCount    int    `json:"chunk_count"`
	IDRangeStart  int64  `json:"id_range_start"`
	IDRangeEnd    int64  `json:"id_range_end"`
	// PersistedPath is the corpus-relative path of the canonical knowledge
	// file (the parse step's doc_id) — the same string the file's
	// frontmatter carries (Phase 53-01).
	PersistedPath string `json:"persisted_path"`
}

// ECLTrigger is the seam the four ecl_* steps will invoke once 54-04
// wires the real auto-trigger (摄取成功 → ECL 异步入队). nil this phase —
// the ecl steps land skipped. The seam exists from day one so the step
// functions' shape does not change when it is wired.
type ECLTrigger func(ctx context.Context, j Job) error

// VectorSink is the seam behind the vector_insert step: delete the doc's
// previous generation for (slug, doc_id), insert the new one, and return
// the written PK set (the SC 5 artifact 54-03 purges by before
// re-inserting — the write-amplification guard). Delete-then-insert is
// the ONLY legal ordering; a bare append stacks two searchable
// generations (the 2931/256 incident class). The production
// implementation is the cmd/server/main.go closure over the Milvus
// backend (DeleteDocRows + Insert — the 53-03 upsert semantics);
// "Milvus details stay behind the seam" (ecl SeedFunc precedent).
type VectorSink func(ctx context.Context, slug, docID string, docs []retriever.Document) ([]int64, error)

// Runner executes ingest jobs. Construct with NewRunner; wire into
// server.Deps.Ingest (nil = every ingest surface 503s, the ECL
// nil-posture).
type Runner struct {
	jobs             JobStore
	emb              embedder.Embedder
	reg              kb.Registry
	parserReg        *document.ParserRegistry
	retriever        retriever.Retriever
	sink             VectorSink
	eclTrigger       ECLTrigger
	embedCache       EmbedCache
	artifacts        *ArtifactStore
	existing         func(ctx context.Context, slug, docID string) (map[string]retriever.DocChunk, error)
	embedderModel    string
	retryAttempts    int
	retryBackoffBase time.Duration
	auditHook        func(action, detail string)

	// sem bounds concurrently RUNNING jobs (default maxConcurrentJobs=2):
	// two parallel ingests already hammer the intranet Milvus/BGE pair;
	// an unbounded fan-out would turn one bulk upload into a retry storm
	// (the env-intranet-flakiness lesson). Jobs beyond the cap still
	// return a job_id immediately — they queue on the semaphore.
	sem chan struct{}
	// wg tracks in-flight runs for WaitIdle (tests + graceful observation).
	wg sync.WaitGroup

	// slugMutexes serializes the embed→vector_insert critical section per
	// slug (D27-03-2 — see the file comment). Keyed by slug, same shape as
	// the server package's upload mutex map; the two maps guard disjoint
	// regions and must not be merged.
	slugMutexes sync.Map
}

// maxConcurrentJobs is the default running-job cap.
const maxConcurrentJobs = 2

// RunnerConfig is NewRunner's dependency bundle. Jobs, Emb, Reg,
// ParserReg, and Retriever are REQUIRED (a nil one is a boot
// misconfiguration — fail fast, the ecl nil-posture keeps routes 503 but
// a half-wired runner would fail jobs at step 1); Sink may be nil (the
// vector_insert step fails loudly with "vector sink not wired" — useful
// in tests that never reach insertion); ECLTrigger is nil until 54-04.
type RunnerConfig struct {
	Jobs       JobStore
	Emb        embedder.Embedder
	Reg        kb.Registry
	ParserReg  *document.ParserRegistry
	Retriever  retriever.Retriever
	Sink       VectorSink
	ECLTrigger ECLTrigger
	// EmbedCache is the (model, content_hash) vector lookaside (54-02).
	// nil = no lookaside: every chunk embeds fresh (correct, just
	// not free on retry).
	EmbedCache EmbedCache
	// Artifacts is the per-job staging root (54-02 D-54-02). nil =
	// no step products are written (retry then re-runs the chain).
	Artifacts *ArtifactStore
	// row (same fn the server package's SetDocUploadAuditHook binds —
	// one store write path, two entry points).
	// ExistingChunks feeds the embed step skip set (54-02): content_hash
	// -> kept chunk (PK + vector for verbatim re-insert). nil = no skip
	// set (every chunk embeds fresh).
	ExistingChunks func(ctx context.Context, slug, docID string) (map[string]retriever.DocChunk, error)
	// EmbedderModel names the embedding model for the cache key's model
	// half (54-02). Empty = cache lookaside disabled (every chunk
	// embeds fresh).
	EmbedderModel    string
	RetryAttempts    int
	RetryBackoffBase time.Duration
	AuditHook        func(action, detail string)
	// MaxConcurrent overrides the running-job cap (tests). 0 → default.
	MaxConcurrent int
}

// NewRunner validates the required seams and constructs the runner.
func NewRunner(cfg RunnerConfig) (*Runner, error) {
	switch {
	case cfg.Jobs == nil:
		return nil, fmt.Errorf("ingest.NewRunner: Jobs (JobStore) is required")
	case cfg.Emb == nil:
		return nil, fmt.Errorf("ingest.NewRunner: Emb (embedder.Embedder) is required")
	case cfg.Reg == nil:
		return nil, fmt.Errorf("ingest.NewRunner: Reg (kb.Registry) is required")
	case cfg.ParserReg == nil:
		return nil, fmt.Errorf("ingest.NewRunner: ParserReg (*document.ParserRegistry) is required")
	case cfg.Retriever == nil:
		return nil, fmt.Errorf("ingest.NewRunner: Retriever (retriever.Retriever) is required")
	}
	max := cfg.MaxConcurrent
	if max <= 0 {
		max = maxConcurrentJobs
	}
	return &Runner{
		jobs:             cfg.Jobs,
		emb:              cfg.Emb,
		reg:              cfg.Reg,
		parserReg:        cfg.ParserReg,
		retriever:        cfg.Retriever,
		sink:             cfg.Sink,
		eclTrigger:       cfg.ECLTrigger,
		auditHook:        cfg.AuditHook,
		embedderModel:    cfg.EmbedderModel,
		retryAttempts:    cfg.RetryAttempts,
		retryBackoffBase: cfg.RetryBackoffBase,
		existing:         cfg.ExistingChunks,
		embedCache:       cfg.EmbedCache,
		artifacts:        cfg.Artifacts,
		sem:              make(chan struct{}, max),
	}, nil
}

// Enqueue persists the job + its eight pending step rows and launches the
// detached run. The raw bytes ride in memory into the goroutine because
// step-product staging is 54-02 (D-54-02: staging first, retry second);
// the job row's source_path already records the archive location those
// retries will re-read from. ctx is used for the persistence calls only —
// the run itself gets a background context so it survives the request.
func (r *Runner) Enqueue(ctx context.Context, j Job, raw []byte) (string, error) {
	id, err := NewJobID()
	if err != nil {
		return "", err
	}
	j.JobID = id
	j.Status = StatusQueued
	nowT := nowTime()
	now := rfc3339(nowT)
	j.CreatedAt = now
	j.UpdatedAt = now
	if err := r.jobs.CreateJob(ctx, j); err != nil {
		return "", fmt.Errorf("create job row: %w", err)
	}
	if err := r.jobs.CreateSteps(ctx, j.JobID, nowT); err != nil {
		return "", fmt.Errorf("create step rows: %w", err)
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.sem <- struct{}{}
		defer func() { <-r.sem }()
		r.run(context.Background(), j.JobID, raw, "")
	}()
	return j.JobID, nil
}

// WaitIdle blocks until every launched run settles. Test-only affordance
// (deterministic assertions about detached goroutines); production code
// observes progress through the job/step rows instead.
func (r *Runner) WaitIdle() { r.wg.Wait() }

// ReadArtifact returns one staged product (54-03: the admin GET
// endpoint reads embed_failed.json through this to project failed_items —
// the handler has no ArtifactStore of its own; the runner is the seam).
func (r *Runner) ReadArtifact(jobID, name string) ([]byte, error) {
	if r.artifacts == nil {
		return nil, os.ErrNotExist
	}
	return r.artifacts.Read(jobID, name)
}

// RecoverOrphans is the boot-time crash-recovery scan (54-03 SC 8,
// single-process architecture: ANY running residue is crash residue).
//
// The asymmetry is deliberate:
//   - running -> failed("[transient] interrupted by restart"): the crash's
//     usual accomplice is the intranet flaking, and the boot path must not
//     auto-replay LLM/Milvus spend on that gamble — failed + transient
//     classification puts the job inside the retry endpoint's reach for a
//     human decision instead.
//   - queued -> resumed: nothing was spent yet, so continuing costs
//     nothing the operator did not already ask for.
func (r *Runner) RecoverOrphans(ctx context.Context) error {
	jobs, err := r.jobs.ListResumable(ctx)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		switch j.Status {
		case StatusRunning:
			nowT := nowTime()
			_ = r.jobs.UpsertStep(ctx, StepRow{
				JobID:       j.JobID,
				Step:        "parse",
				Status:      StepFailed,
				Error:       "[transient] interrupted by restart",
				CompletedAt: rfc3339(nowT),
			})
			if err := r.jobs.UpdateJob(ctx, j.JobID, StatusFailed,
				"[transient] interrupted by restart (crash recovery)", nowT); err != nil {
				return err
			}
			log.Printf("[INFO] ingest: crash recovery: job %s (running residue) -> failed; retry via the step endpoint", j.JobID)
		case StatusQueued:
			log.Printf("[INFO] ingest: crash recovery: resuming queued job %s (%s)", j.JobID, j.Filename)
			jobID := j.JobID
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				r.sem <- struct{}{}
				defer func() { <-r.sem }()
				r.run(context.Background(), jobID, nil, StepParse)
			}()
		}
	}
	return nil
}

// Retry errors (54-03): structured so admin_ingest.go maps them to
// precise HTTP codes instead of a generic 500 (RAGFlow lesson: return
// the FIRST real error, not a generic failure).
var (
	ErrRetryStepInvalid   = errors.New("unknown step")
	ErrRetryStepMissing   = errors.New("step has no row")
	ErrRetryStepNotFailed = errors.New("step is not failed")
	ErrRetryJobBusy       = errors.New("job already has an active execution")
	ErrRetryRawMissing    = errors.New("parse archive missing for parse retry")
)

// RetryStep re-runs ONE failed step and everything after it (54-03
// Task 2). Order matters:
//
//  1. validate step name + current status = failed (else 404/409)
//  2. ClaimJobRetry CAS — concurrent retry/enqueue loses -> 409
//  3. CascadeStale — downstream rows become stale in one transaction
//  4. detached goroutine resumes at step (attempt counters continue;
//     the archive re-read feeds a parse retry, later steps run on the
//     staged artifacts)
func (r *Runner) RetryStep(ctx context.Context, jobID, step string) (string, error) {
	valid := false
	for _, s := range StepSequence {
		if s == step {
			valid = true
			break
		}
	}
	if !valid {
		return "", ErrRetryStepInvalid
	}

	steps, err := r.jobs.ListSteps(ctx, jobID)
	if err != nil {
		return "", err
	}
	var target StepRow
	found := false
	for _, s := range steps {
		if s.Step == step {
			target = s
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrRetryStepMissing, target.Error)
	}
	if target.Status != StepFailed {
		return "", fmt.Errorf("%w: %s", ErrRetryStepNotFailed, target.Error)
	}

	ok, err := r.jobs.ClaimJobRetry(ctx, jobID, nowTime())
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrRetryJobBusy
	}
	if _, err := r.jobs.CascadeStale(ctx, jobID, step, nowTime()); err != nil {
		// Roll the claim back — a claimed job with a half-voided cascade
		// is exactly the half-state the transaction exists to prevent.
		_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "cascade stale failed: "+err.Error(), nowTime())
		return "", err
	}

	raw := []byte(nil)
	if step == StepParse {
		// Resuming AT parse needs the archive (data/uploads) — read it
		// now, synchronously, so a missing archive is a clean error and
		// the claim rolls back before the goroutine exists.
		job, err := r.jobs.GetJob(ctx, jobID)
		if err != nil {
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "retry job row unreadable: "+err.Error(), nowTime())
			return "", err
		}
		if job.SourcePath == "" {
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "retry parse: no archive path on job row", nowTime())
			return "", ErrRetryRawMissing
		}
		raw, err = os.ReadFile(job.SourcePath)
		if err != nil {
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "retry parse archive unreadable: "+err.Error(), nowTime())
			return "", err
		}
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.sem <- struct{}{}
		defer func() { <-r.sem }()
		r.run(context.Background(), jobID, raw, step)
	}()
	return "queued", nil
}

// GetJob returns the job row (the GET endpoint walks the Runner, never
// the store). A missing job wraps ErrNotFound — the handler maps it to
// 404 via IsNotFound.
func (r *Runner) GetJob(ctx context.Context, jobID string) (Job, error) {
	return r.jobs.GetJob(ctx, jobID)
}

// ListSteps returns the job's step rows in StepSequence order.
func (r *Runner) ListSteps(ctx context.Context, jobID string) ([]StepRow, error) {
	return r.jobs.ListSteps(ctx, jobID)
}

// IsNotFound passes the store's sentinel test through (404 vs 500
// classification at the handler).
func (r *Runner) IsNotFound(err error) bool { return r.jobs.IsNotFound(err) }

// ActiveJobBySHA256 is the extended-dedup lookup behind the upload
// handler: the newest job for (slug, sha256) that has NOT failed —
// queued/running/partial/done all answer "this exact bytes-set is already
// ingested or ingesting" (done jobs additionally carry their
// action_doc_uploaded audit row, so the two dedup sources agree). A
// FAILED job does not block re-upload — retrying a failed ingest by
// re-submitting the file is a legitimate operator flow.
func (r *Runner) ActiveJobBySHA256(ctx context.Context, slug, sha256 string) (Job, bool) {
	j, err := r.jobs.GetJobBySHA256(ctx, slug, sha256)
	if err != nil || j.Status == StatusFailed || j.Status == StatusPartial {
		return Job{}, false
	}
	return j, true
}

// run executes the pipeline for one claimed job. Called on the detached
// goroutine only. CR-01: every exit path lands the job (and the in-flight
// step) in a terminal state.
// run executes the pipeline for one claimed job, starting at startFrom
// ("" = StepSequence head; a retry passes the failed step's name so the
// chain resumes there — upstream done steps do NOT re-run, their products
// stay on disk and their PKs stay live). Called on the detached goroutine
// only. CR-01: every exit path lands the job (and the in-flight step) in
// a terminal state.
func (r *Runner) run(ctx context.Context, jobID string, raw []byte, startFrom string) {
	ok, err := r.jobs.ClaimJob(ctx, jobID, nowTime())
	if err != nil {
		// Store I/O on the claim: the job may still read queued forever —
		// land it failed instead (CR-01: no non-terminal limbo).
		log.Printf("[WARN] ingest: job %s claim failed: %v (marking failed)", jobID, err)
		_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "claim failed: "+err.Error(), nowTime())
		return
	}
	if !ok {
		// Another owner holds the job, or it reached a terminal state
		// between enqueue and claim — nothing to do.
		return
	}

	job, err := r.jobs.GetJob(ctx, jobID)
	if err != nil {
		log.Printf("[WARN] ingest: job %s row vanished after claim: %v", jobID, err)
		_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "job row unreadable after claim: "+err.Error(), nowTime())
		return
	}

	steps, err := r.jobs.ListSteps(ctx, jobID)
	if err != nil {
		_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "step rows unreadable: "+err.Error(), nowTime())
		return
	}
	// Cumulative attempts (54-03 retries re-enter this loop with rows
	// already carrying attempts — never reset).
	attempts := make(map[string]int64, len(steps))
	for _, s := range steps {
		attempts[s.Step] = s.Attempt
	}

	if startFrom != "" && startFrom != StepParse && len(raw) == 0 {
		// Resume past parse: raw is not consumed by any later step, so an
		// empty slice is correct (the 54-02 staging artifacts + the live
		// pipelineState fields carry the inputs). Resuming AT parse reads
		// the archive below.
	}
	st := &pipelineState{raw: raw}
	var (
		currentStep      string
		currentStepStart string
		slugLocked       bool
	)
	// Panic net (admin_pipeline.go run-goroutine pattern): a panic in a
	// step must fail the job + the in-flight step, never kill the server
	// process, and never leave a step stuck at running (CR-01).
	defer func() {
		if slugLocked {
			r.unlockSlug(job.Slug)
		}
		if rec := recover(); rec != nil {
			log.Printf("[ERROR] ingest: job %s goroutine panicked: %v\n%s", jobID, rec, debug.Stack())
			nowT := nowTime()
			if currentStep != "" {
				_ = r.jobs.UpsertStep(ctx, StepRow{
					JobID:       jobID,
					Step:        currentStep,
					Status:      StepFailed,
					Attempt:     attempts[currentStep] + 1,
					StartedAt:   currentStepStart,
					CompletedAt: rfc3339(nowT),
					Error:       fmt.Sprintf("panic: %v", rec),
				})
			}
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, fmt.Sprintf("panic: %v", rec), nowT)
		}
	}()

	started := startFrom == ""
	for _, name := range StepSequence {
		if !started {
			if name == startFrom {
				started = true
			} else {
				continue // upstream done step: do NOT re-run (54-03 resume semantics)
			}
		}
		currentStep = name

		// D27-03-2: open the per-slug critical section at the embed step
		// (GetMaxID lives inside ResolveReuseAndEmbed) and close it after
		// vector_insert (the Insert lives inside the sink).
		if name == StepEmbed {
			r.lockSlug(job.Slug)
			slugLocked = true
		}

		startedAt := nowTime()
		currentStepStart = rfc3339(startedAt)
		if err := r.jobs.UpsertStep(ctx, StepRow{
			JobID:     jobID,
			Step:      name,
			Status:    StepRunning,
			Attempt:   attempts[name] + 1,
			StartedAt: currentStepStart,
		}); err != nil {
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "step "+name+" start write failed: "+err.Error(), nowTime())
			return
		}

		// 54-03: transient failures retry INSIDE the step (backoff 1s/4s/16s
		// x retryAttempts); deterministic ones surface after exactly one
		// attempt. The classification rides the error text so the step row
		// (and the operator reading it) sees WHY the retry policy chose
		// the path it took.
		var artifact string
		var stepStatus string
		var stepErr error
		stepErr = r.retryLoop(ctx, func() error {
			var a string
			var sst string
			var e error
			a, sst, e = r.execStep(ctx, job, name, st)
			artifact, stepStatus, err = a, sst, e
			return e
		})
		completedAt := nowTime()
		completed := rfc3339(completedAt)
		if stepErr != nil {
			class := Classify(stepErr)
			tagged := class.Prefix(stepErr)
			logStepFailure(jobID, name, errors.New(tagged))
			_ = r.jobs.UpsertStep(ctx, StepRow{
				JobID:       jobID,
				Step:        name,
				Status:      StepFailed,
				Attempt:     attempts[name] + 1,
				StartedAt:   currentStepStart,
				CompletedAt: completed,
				Error:       tagged,
			})
			// D-54-01 fail-closed: a failed REQUIRED step fails the job.
			// Remaining steps stay pending — the SC 1 timeline shows
			// exactly where the chain stopped and why.
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, name+" failed: "+tagged, completedAt)
			return
		}
		if err := r.jobs.UpsertStep(ctx, StepRow{
			JobID:       jobID,
			Step:        name,
			Status:      stepStatus, // done, or skipped for the ecl placeholders
			Attempt:     attempts[name] + 1,
			StartedAt:   currentStepStart,
			CompletedAt: completed,
			Error:       skipReason(stepStatus, artifact),
			ArtifactRef: artifactRef(stepStatus, artifact),
		}); err != nil {
			_ = r.jobs.UpdateJob(ctx, jobID, StatusFailed, "step "+name+" done write failed: "+err.Error(), completedAt)
			return
		}

		if name == StepVectorInsert {
			r.unlockSlug(job.Slug)
			slugLocked = false
		}
	}

	// All eight steps terminal (done/skipped): job=done, then the success
	// audit row — migrated from upload.go STEP 9 verbatim, because only
	// NOW do the chunk count and ID range exist.
	finalStatus := StatusDone
	if len(st.failedItems) > 0 {
		// 54-03 SC 7: a step that finished with LEDGERED item failures
		// (embed lost some chunks) lands the job at partial -- the
		// Bedrock "COMPLETE but N docs failed" projection. The success
		// audit still runs: the landed chunks are a real, retrievable
		// result, and failed_items names exactly what did not make it.
		finalStatus = StatusPartial
	}
	if err := r.jobs.UpdateJob(ctx, jobID, finalStatus, "", nowTime()); err != nil {
		log.Printf("[WARN] ingest: job %s done transition failed: %v", jobID, err)
		return
	}
	r.writeUploadAudit(job, st)
}

// execStep dispatches one step by name. stepStatus is done or skipped
// (failed steps return via err).
func (r *Runner) execStep(ctx context.Context, job Job, name string, st *pipelineState) (artifact string, stepStatus string, err error) {
	switch name {
	case StepParse:
		artifact, err = r.stepParse(ctx, job, st)
	case StepChunk:
		artifact, err = r.stepChunk(ctx, job, st)
	case StepEmbed:
		artifact, err = r.stepEmbed(ctx, job, st)
	case StepVectorInsert:
		artifact, err = r.stepVectorInsert(ctx, job, st)
	case StepECLMerge, StepECLExtract, StepECLKGLoad, StepECLMilvusLoad:
		artifact, err = r.stepECLSkipped(ctx, job, st)
		if err == nil {
			return artifact, StepSkipped, nil
		}
	default:
		return "", "", fmt.Errorf("unknown step %q (StepSequence and execStep out of sync)", name)
	}
	if err != nil {
		return "", "", err
	}
	return artifact, StepDone, nil
}

// skipReason extracts the reason text recorded in the step row's error
// column for skipped steps (the plan-specified contract: skipped rows
// carry their reason in error so the timeline can show WHY a step did
// not run).
func skipReason(status, artifact string) string {
	if status == StepSkipped {
		return artifact
	}
	return ""
}

// artifactRef extracts the artifact pointer for done steps (skipped steps
// carry no artifact).
func artifactRef(status, artifact string) string {
	if status == StepDone {
		return artifact
	}
	return ""
}

// writeUploadAudit emits the action_doc_uploaded row (STEP 9 migration).
// Best-effort: a nil hook (no audit backend) or a failed write logs and
// never rolls the completed job back — the job/step rows are the source
// of truth, the audit row is the trail.
func (r *Runner) writeUploadAudit(job Job, st *pipelineState) {
	if r.auditHook == nil {
		return
	}
	idStart, idEnd := int64(0), int64(0)
	if len(st.docs) > 0 {
		idStart = st.docs[0].ID
		idEnd = st.docs[len(st.docs)-1].ID
	}
	payload, err := json.Marshal(UploadAudit{
		Slug:          job.Slug,
		FileName:      job.Filename,
		FileSize:      job.FileSize,
		SHA256:        job.SHA256,
		ParserVersion: st.parserName,
		ChunkCount:    len(st.docs),
		IDRangeStart:  idStart,
		IDRangeEnd:    idEnd,
		PersistedPath: st.docID,
	})
	if err != nil {
		log.Printf("[WARN] ingest: job %s audit payload marshal: %v (row skipped)", job.JobID, err)
		return
	}
	r.auditHook(actionDocUploaded, string(payload))
}

// lockSlug / unlockSlug manage the per-slug embed→vector_insert critical
// section (D27-03-2 — see the file comment).
// writeArtifact stages one step product (54-02 D-54-02). A nil
// artifacts store (unwired, tests) silently skips -- the step contract
// treats products as retry aids, not correctness requirements.
func (r *Runner) writeArtifact(jobID, name string, data []byte) {
	if r.artifacts == nil {
		return
	}
	if err := r.artifacts.Write(jobID, name, data); err != nil {
		log.Printf("[WARN] ingest: artifact %s/%s: %v", jobID, name, err)
	}
}

func (r *Runner) lockSlug(slug string) {
	mu, _ := r.slugMutexes.LoadOrStore(slug, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
}

func (r *Runner) unlockSlug(slug string) {
	if mu, ok := r.slugMutexes.Load(slug); ok {
		mu.(*sync.Mutex).Unlock()
	}
}

// nowTime / rfc3339 are the single timestamp pair every caller-set stamp
// goes through (schema.go timestamp discipline: the runner owns the
// instant, the store formats verbatim — RFC3339Nano UTC, no SQL defaults,
// so durations derive testably from the stored strings).
func nowTime() time.Time { return time.Now().UTC() }

func rfc3339(t time.Time) string { return t.Format(time.RFC3339Nano) }
