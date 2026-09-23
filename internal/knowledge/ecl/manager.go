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

package ecl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

// Stage sentinels — LOCKED FE contract (39-UI-SPEC §Stage Mapping:
// isRunning = stage !== 'idle' && stage !== 'done'). Do not rename.
const (
	StageIdle       = "idle"
	StageMerge      = "merge"
	StageExtract    = "extract"
	StageKGLoad     = "kg-load"
	StageMilvusLoad = "milvus-load"
	StageDone       = "done"
)

// maxAttempts is the hard retry cap per note per stage (D-RT-01 /
// T-39-11: bounded retry storms — dead notes leave the active set).
const maxAttempts = 3

// extractConcurrency is the LLM worker-pool width for the extract stage
// (planner pin: ≤ 3 concurrent LLM calls).
const extractConcurrency = 3

// loadedNote pairs a merged note with its successful Extraction — the unit
// handed from the extract stage to the kg-load stage.
type loadedNote struct {
	note Note
	ex   Extraction
}

// Sentinel errors Plan 07 maps to 409 / 404 / 500 (RESEARCH Pattern 2).
var (
	// ErrPipelineBusy is returned by RetryNote while a run holds the
	// running flag.
	ErrPipelineBusy = errors.New("pipeline already running")
	// ErrUnknownNote is returned by RetryNote when the ledger has no DEAD
	// row for the requested note (T-39-10: retry requires an existing
	// dead failure row — arbitrary note_ids cannot trigger work).
	ErrUnknownNote = errors.New("unknown or non-dead note")
	// ErrStoreUnavailable is returned by TryClaimRetry when the ledger store
	// has an I/O error (SQLite errors other than a missing row). The handler
	// maps this to HTTP 500 so the admin knows a real infrastructure problem
	// occurred rather than confusing it with an unknown note (IN-04).
	ErrStoreUnavailable = errors.New("ledger store unavailable")
)

// SeedFunc persists built Documents into the vector store. main.go injects
// the production implementation (DeleteByExpr marker purge +
// GetMaxIDCollection watermark → idAlloc closure + embed + SeedDocuments);
// tests fake it. The milvus package is deliberately NOT imported here —
// this seam is the ONLY coupling point between the ecl orchestrator and
// the vector store (RESEARCH "Milvus details stay behind the seam").
type SeedFunc func(ctx context.Context, docs []retriever.Document) error

// defaultBackoff is the D-RT-01 exponential pin: 1s / 4s / 16s between
// attempts. Overridable in tests (0-durations).
var defaultBackoff = []time.Duration{time.Second, 4 * time.Second, 16 * time.Second}

// Manager is the ECL stage orchestrator (D-Run-01 / D-RT-01 / D-DL-01 /
// D-Prog-01): admin-triggered runs over merge → extract → kg-load →
// milvus-load, per-note retry ×3 with backoff → dead-letter while the
// batch continues, idempotent full-note re-enqueue, and a polled status
// snapshot. A single Manager instance serves one ECL scope; the admin
// handler claims it with CASRunning before launching Run in a goroutine.
type Manager struct {
	mu sync.Mutex
	// running is the CAS flag: claimed by CASRunning (Run's caller) and
	// by RetryNote, released when the run finishes.
	running     bool
	stage       string
	currentFile string
	done        int
	total       int
	queueLen    int

	ns        string
	kbDir     string
	kg        *store.Store
	extractor Extractor
	failures  FailureStore
	seedFn    SeedFunc
	onChange  func()

	// Injection points (defaults set by NewManager; tests override).
	// backoff holds maxAttempts-1 sleep durations, indexed by the failed
	// attempt number minus one.
	backoff []time.Duration
	// applyNote is the kg-load seam — default abox ApplyNote. D-RT-01
	// names kg-load as one of the three retry stages, so tests must be
	// able to force that leg to fail.
	applyNote func(*store.Store, string, Note, Extraction, func()) error
	// idAlloc mints Document IDs (Plan 06 wires the GetMaxIDCollection
	// watermark counter; tests inject a sequence).
	idAlloc func() int64
}

// NewManager constructs the orchestrator. This exact signature is a
// cross-plan contract (Plan 06 boot wiring / Plan 07 handler construction)
// — do not rename or reorder parameters.
func NewManager(ns string, kbDir string, kg *store.Store, ex Extractor, fs FailureStore, seedFn SeedFunc) *Manager {
	return &Manager{
		ns:        ns,
		kbDir:     kbDir,
		kg:        kg,
		extractor: ex,
		failures:  fs,
		seedFn:    seedFn,
		stage:     StageIdle,
		backoff:   append([]time.Duration(nil), defaultBackoff...),
		applyNote: ApplyNote,
	}
}

// SetOnChange registers the post-mutation callback (induction.Manager hook
// shape). Fires after every status mutation so a wired observer (the
// debounced KG snapshot writer stays on ApplyNote's onChange parameter;
// this hook is for progress observers) never needs to poll sooner than the
// FE cadence.
func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// SetIDAllocator injects the Document ID source for the milvus-load stage
// (Plan 06 wires the collection-wide watermark counter).
func (m *Manager) SetIDAllocator(fn func() int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idAlloc = fn
}

// Enabled reports whether the pipeline is usable. Always true when
// constructed — Plan 06/07 gate on construction + env, so a constructed
// Manager is by definition enabled.
func (m *Manager) Enabled() bool { return true }

// CASRunning atomically claims the running flag: true = claimed (caller
// must launch Run), false = a run is already active (the admin handler
// answers 409).
func (m *Manager) CASRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return false
	}
	m.running = true
	return true
}

// releaseRunning unclaims the running flag (Run/RetryNote defer; tests use
// it to simulate a finished run).
func (m *Manager) releaseRunning() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
}

// Status is the internal status snapshot. Failures are excluded from JSON
// (`json:"-"`) — the wire shape is StatusDTO below.
type Status struct {
	Stage       string
	CurrentFile string
	Done        int
	Total       int
	QueueLen    int
	Failures    []Failure `json:"-"`
}

// StatusDTO is the LOCKED wire shape (D-Prog-01 / 39-UI-SPEC §Data
// Contract; Plan 07 serializes verbatim). Failure carries its own json
// tags — the failures[] keys are owned by the Failure struct.
type StatusDTO struct {
	Stage       string    `json:"stage"`
	CurrentFile string    `json:"current_file"`
	Done        int       `json:"done"`
	Total       int       `json:"total"`
	QueueLen    int       `json:"queue_len"`
	Failures    []Failure `json:"failures"`
}

// Status returns the current pipeline state plus the full failure ledger.
// done/total/queue_len are PER-STAGE counters: done counts notes finished
// in the current stage, queue_len the notes still awaiting processing in
// it (total stays len(notes) for the whole run).
func (m *Manager) Status() StatusDTO {
	m.mu.Lock()
	snap := Status{
		Stage:       m.stage,
		CurrentFile: m.currentFile,
		Done:        m.done,
		Total:       m.total,
		QueueLen:    m.queueLen,
	}
	m.mu.Unlock()

	rows, err := m.failures.ListFailures()
	if err != nil {
		// Ledger read failure must not corrupt the status answer; surface
		// via the log and answer with an empty failures list (no silent
		// swallow — the WARN is the audit trail).
		fmt.Fprintf(os.Stderr, "[WARN] ecl.Manager: list failures: %v\n", err)
		rows = []Failure{}
	}
	snap.Failures = rows
	return StatusDTO{
		Stage:       snap.Stage,
		CurrentFile: snap.CurrentFile,
		Done:        snap.Done,
		Total:       snap.Total,
		QueueLen:    snap.QueueLen,
		Failures:    snap.Failures,
	}
}

// setStatus mutates the live status under the mutex and fires onChange
// outside the lock (hook shape mirrors induction.Manager.fireChange).
func (m *Manager) setStatus(stage, currentFile string, done, total, queueLen int) {
	m.mu.Lock()
	m.stage = stage
	m.currentFile = currentFile
	m.done = done
	m.total = total
	m.queueLen = queueLen
	onChange := m.onChange
	m.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

// fireChange invokes the onChange hook if registered (mutex-safe read,
// invocation outside the lock).
func (m *Manager) fireChange() {
	m.mu.Lock()
	onChange := m.onChange
	m.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

// RecoverFromPanic resets the manager to a clean idle state after an
// unexpected panic in a detached goroutine (admin_pipeline.go IN-05).
// It clears the running flag and resets the stage to StageIdle so the
// admin UI recovers without a restart.
func (m *Manager) RecoverFromPanic() {
	m.mu.Lock()
	m.running = false
	m.stage = StageIdle
	m.currentFile = ""
	m.done = 0
	m.total = 0
	m.queueLen = 0
	onChange := m.onChange
	m.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

// Run executes the full pipeline: merge → extract → kg-load → milvus-load
// → done. The caller claims CASRunning() FIRST and launches Run in a
// goroutine (admin handler posture); Run releases the flag via defer.
//
// Per-note failures retry up to maxAttempts with backoff, then park the
// note dead in the ledger and CONTINUE the batch (D-RT-01: single-note
// failure never aborts the run). Only batch-level failures (merge walk,
// doc build) abort the run with a returned error.
//
// Terminal-stage discipline (CR-01): every batch-level failure path leaves
// the stage at a TERMINAL sentinel — a returned error releases the running
// flag (deferred), and an in-flight stage value paired with a released flag
// would render 运行中 forever in the admin pipeline UI with the run button
// disabled until restart (the LOCKED FE mapping treats any non-idle/non-done
// stage as running).
func (m *Manager) Run(ctx context.Context) (err error) {
	defer m.releaseRunning()
	defer func() {
		if err != nil {
			// CR-01: batch-level failure — nil ID allocator (milvus-load
			// entry), BuildDocuments, or seedWithRetry exhaustion (e.g. the
			// documented Milvus intranet flakiness). Reset to the idle
			// terminal sentinel (D-Prog-01) so the FE stays operable: the
			// operator must be able to simply run again.
			m.setStatus(StageIdle, "", 0, 0, 0)
		}
	}()
	m.setStatus(StageMerge, "", 0, 0, 0)

	notes, _, err := WalkAndMerge(m.kbDir)
	if err != nil {
		return fmt.Errorf("ecl: manager: merge: %w", err)
	}
	m.setStatus(StageMerge, "", 0, len(notes), len(notes))

	// --- extract: worker pool of extractConcurrency over notes ---
	m.setStatus(StageExtract, "", 0, len(notes), len(notes))
	var (
		loadedMu sync.Mutex
		loaded   []loadedNote
		wg       sync.WaitGroup
		sem      = make(chan struct{}, extractConcurrency)
		done     atomic.Int64
	)
	for _, note := range notes {
		wg.Add(1)
		go func(n Note) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			m.setStatus(StageExtract, firstSourcePath(n), int(done.Load()), len(notes), len(notes)-int(done.Load()))
			ex, err := m.extractWithRetry(ctx, n)
			if err == nil {
				loadedMu.Lock()
				loaded = append(loaded, loadedNote{note: n, ex: ex})
				loadedMu.Unlock()
			}
			done.Add(1)
			m.setStatus(StageExtract, firstSourcePath(n), int(done.Load()), len(notes), len(notes)-int(done.Load()))
		}(note)
	}
	wg.Wait()

	// Deterministic downstream order (stable IDs / stable kg write order).
	sort.Slice(loaded, func(i, j int) bool {
		a, _ := strconv.Atoi(loaded[i].note.SourceIdx)
		b, _ := strconv.Atoi(loaded[j].note.SourceIdx)
		return a < b
	})

	// --- kg-load: per successful note, via the applyNote seam ---
	m.setStatus(StageKGLoad, "", 0, len(loaded), len(loaded))
	kgLoaded := make([]loadedNote, 0, len(loaded))
	for i, ln := range loaded {
		m.setStatus(StageKGLoad, NoteIRI(m.ns, ln.note.SourceIdx), i, len(loaded), len(loaded)-i)
		err := m.loadWithRetry(ctx, ln.note, StageKGLoad, func(ctx context.Context) error {
			return m.applyNote(m.kg, m.ns, ln.note, ln.ex, m.fireChange)
		})
		if err == nil {
			kgLoaded = append(kgLoaded, ln)
		}
	}

	// --- milvus-load: build docs once, seed with batch retry ---
	if len(kgLoaded) > 0 {
		m.setStatus(StageMilvusLoad, NoteIRI(m.ns, kgLoaded[0].note.SourceIdx), 0, len(kgLoaded), len(kgLoaded))
		m.mu.Lock()
		alloc := m.idAlloc
		m.mu.Unlock()
		if alloc == nil {
			return errors.New("ecl: manager: no ID allocator wired (SetIDAllocator) — cannot build documents")
		}
		batchNotes := make([]Note, len(kgLoaded))
		for i, ln := range kgLoaded {
			batchNotes[i] = ln.note
		}
		docs, err := BuildDocuments(batchNotes, alloc)
		if err != nil {
			// Batch-level failure: park every note that reached this stage.
			// IN-03: record attempt 1 (BuildDocuments failed before any seed attempt).
			for _, ln := range kgLoaded {
				m.recordFailure(ln.note, StageMilvusLoad, 1, err, FailureDead)
			}
			return fmt.Errorf("ecl: manager: build documents: %w", err)
		}
		if _, err := m.seedWithRetry(ctx, kgLoaded, docs); err != nil {
			return fmt.Errorf("ecl: manager: milvus-load: %w", err)
		}
	}

	m.setStatus(StageDone, "", len(notes), len(notes), 0)
	return nil
}

// TryClaimRetry validates that noteID has a DEAD ledger row (T-39-10) and
// claims the running flag — the synchronous, cheap half of a retry. The
// admin handler runs it inline so 409/404/500 still answer before the HTTP
// response (D-Route-01 mapping); only the long-running pipeline work is
// detached (WR-01 — a retry's worst case is 3 LLM calls + up to 15s of
// backoff + a Milvus round-trip, well past proxy timeouts). On success the
// caller OWNS the running flag and MUST complete the retry with
// RetryClaimed (which releases it); on failure the claim is released here.
// Errors: ErrPipelineBusy (a run or another retry holds the flag),
// ErrUnknownNote (no DEAD row for noteID), ErrStoreUnavailable (store I/O error).
func (m *Manager) TryClaimRetry(noteID string) error {
	if !m.CASRunning() {
		return ErrPipelineBusy
	}
	row, err := m.failures.GetFailure(noteID)
	if err != nil {
		m.releaseRunning()
		if m.failures.IsNotFound(err) {
			return ErrUnknownNote
		}
		return ErrStoreUnavailable
	}
	if row.Status != FailureDead {
		m.releaseRunning()
		return ErrUnknownNote
	}
	return nil
}

// RetryClaimed runs the FULL note pipeline (extract → kg-load →
// milvus-load) for one dead note whose running flag was ALREADY claimed via
// TryClaimRetry — idempotent via note-level replace (ApplyNote deletes the
// note's old triples first; no stage checkpointing, planner pin). Releases
// the running flag on every exit.
//
// Terminal-stage discipline (CR-01): every error path AFTER stage entry
// (merge walk, corpus miss, extract, kg-load, milvus-load) resets the stage
// to a terminal sentinel — a returned error releases the running flag, and
// an in-flight stage value paired with a released flag renders 运行中
// forever in the admin pipeline UI with the run button disabled until
// restart. The guard is installed only after the pre-entry DEAD-row check,
// so a refused retry never touches the previous run's terminal state.
func (m *Manager) RetryClaimed(ctx context.Context, noteID string) (err error) {
	defer m.releaseRunning()

	// Re-check the DEAD row under the claim: the row may have been
	// resolved or flipped between the handler's TryClaimRetry and this
	// call. This runs BEFORE the stage-reset guard installs — a refused
	// retry must not touch the previous run's terminal state.
	row, rerr := m.failures.GetFailure(noteID)
	if rerr != nil {
		if m.failures.IsNotFound(rerr) {
			return ErrUnknownNote
		}
		return ErrStoreUnavailable
	}
	if row.Status != FailureDead {
		return ErrUnknownNote
	}
	defer func() {
		if err != nil {
			// CR-01: the retry entered the pipeline and failed — reset to
			// the idle terminal sentinel (D-Prog-01) so the FE stays
			// operable.
			m.setStatus(StageIdle, "", 0, 0, 0)
		}
	}()
	// Re-arm the row: pending, attempts reset (D-DL-01 lifecycle).
	m.recordFailure(Note{SourceIdx: noteID}, row.Stage, 0, nil, FailurePending)

	m.setStatus(StageMerge, "", 0, 1, 1)
	notes, _, err := WalkAndMerge(m.kbDir)
	if err != nil {
		return fmt.Errorf("ecl: manager: retry merge: %w", err)
	}
	var target *Note
	for i := range notes {
		if notes[i].SourceIdx == noteID {
			target = &notes[i]
			break
		}
	}
	if target == nil {
		// The ledger row exists but the corpus no longer carries the note:
		// nothing re-runnable — park it dead again with the explanation.
		err := fmt.Errorf("ecl: manager: note %s no longer in corpus", noteID)
		m.recordFailure(Note{SourceIdx: noteID}, StageMerge, maxAttempts, err, FailureDead)
		return fmt.Errorf("%w: note %s not found in corpus walk", ErrUnknownNote, noteID)
	}
	m.setStatus(StageMerge, firstSourcePath(*target), 1, 1, 0)

	m.setStatus(StageExtract, firstSourcePath(*target), 0, 1, 1)
	ex, err := m.extractWithRetry(ctx, *target)
	if err != nil {
		return fmt.Errorf("ecl: manager: retry note %s extract: %w", noteID, err)
	}

	m.setStatus(StageKGLoad, NoteIRI(m.ns, noteID), 0, 1, 1)
	if err := m.loadWithRetry(ctx, *target, StageKGLoad, func(ctx context.Context) error {
		return m.applyNote(m.kg, m.ns, *target, ex, m.fireChange)
	}); err != nil {
		return fmt.Errorf("ecl: manager: retry note %s kg-load: %w", noteID, err)
	}

	m.setStatus(StageMilvusLoad, NoteIRI(m.ns, noteID), 0, 1, 1)
	m.mu.Lock()
	alloc := m.idAlloc
	m.mu.Unlock()
	if alloc == nil {
		err := errors.New("ecl: manager: no ID allocator wired (SetIDAllocator)")
		m.recordFailure(*target, StageMilvusLoad, maxAttempts, err, FailureDead)
		return err
	}
	docs, err := BuildDocuments([]Note{*target}, alloc)
	if err != nil {
		m.recordFailure(*target, StageMilvusLoad, maxAttempts, err, FailureDead)
		return fmt.Errorf("ecl: manager: retry note %s build documents: %w", noteID, err)
	}
	loaded := []loadedNote{{note: *target, ex: ex}}
	if _, err := m.seedWithRetry(ctx, loaded, docs); err != nil {
		return fmt.Errorf("ecl: manager: retry note %s milvus-load: %w", noteID, err)
	}

	// Full pipeline succeeded: the row (reset to pending above) resolves.
	m.recordFailure(*target, StageMilvusLoad, 1, nil, FailureResolved)
	m.setStatus(StageDone, "", 1, 1, 0)
	return nil
}

// RetryNote is the synchronous claim+work convenience wrapper — the
// manager-level API for in-process callers and tests. The admin handler
// splits it: TryClaimRetry inline (409/404 before the response), then
// RetryClaimed on a detached goroutine (WR-01).
func (m *Manager) RetryNote(ctx context.Context, noteID string) error {
	if err := m.TryClaimRetry(noteID); err != nil {
		return err
	}
	return m.RetryClaimed(ctx, noteID)
}

// extractWithRetry runs ExtractNote with the D-RT-01 retry loop: up to
// maxAttempts, backoff sleeps between attempts, pending rows while
// retrying, dead row after the final failure.
func (m *Manager) extractWithRetry(ctx context.Context, note Note) (Extraction, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			lastErr = cerr
			break
		}
		ex, err := m.extractor.ExtractNote(ctx, note)
		if err == nil {
			m.resolveIfFailed(note, StageExtract, attempt)
			return ex, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			m.recordFailure(note, StageExtract, attempt, err, FailurePending)
			// IN-06: backoff sleep is ctx-aware — cancellation waits out the full
			// sleep before, so check ctx after the sleep too.
			select {
			case <-ctx.Done():
				m.recordFailure(note, StageExtract, attempt, ctx.Err(), FailureDead)
				return Extraction{}, ctx.Err()
			case <-time.After(m.backoffFor(attempt)):
			}
		}
	}
	m.recordFailure(note, StageExtract, maxAttempts, lastErr, FailureDead)
	return Extraction{}, lastErr
}

// loadWithRetry runs one kg-load attemptFn with the same retry loop, tag
// stage.
func (m *Manager) loadWithRetry(ctx context.Context, note Note, stage string, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			lastErr = cerr
			break
		}
		err := fn(ctx)
		if err == nil {
			m.resolveIfFailed(note, stage, attempt)
			return nil
		}
		lastErr = err
		if attempt < maxAttempts {
			m.recordFailure(note, stage, attempt, err, FailurePending)
			// IN-06: backoff sleep is ctx-aware.
			select {
			case <-ctx.Done():
				m.recordFailure(note, stage, attempt, ctx.Err(), FailureDead)
				return ctx.Err()
			case <-time.After(m.backoffFor(attempt)):
			}
		}
	}
	m.recordFailure(note, stage, maxAttempts, lastErr, FailureDead)
	return lastErr
}

// seedWithRetry runs the batch SeedFunc with the same retry loop. The
// ledger stays per-note (planner pin): each failed attempt writes a
// pending row per note in the batch; the final failure parks ALL of them
// dead at milvus-load; success resolves any rows the earlier attempts
// wrote. It returns the error and the real attempt number at which the
// seed failed (or 0 if it succeeded).
func (m *Manager) seedWithRetry(ctx context.Context, batch []loadedNote, docs []retriever.Document) (int, error) {
	var lastErr error
	var attempt int
	for attempt = 1; attempt <= maxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			lastErr = cerr
			break
		}
		err := m.seedFn(ctx, docs) // NOTE: intentionally new err var; lastErr = err captures it
		if err == nil {
			for _, ln := range batch {
				m.resolveIfFailed(ln.note, StageMilvusLoad, attempt)
			}
			return 0, nil
		}
		lastErr = err // assign the loop-level err to outer lastErr
		if attempt < maxAttempts {
			for _, ln := range batch {
				m.recordFailure(ln.note, StageMilvusLoad, attempt, err, FailurePending)
			}
			// IN-06: backoff sleep is ctx-aware.
			select {
			case <-ctx.Done():
				lastErr = ctx.Err()
				// Record dead for remaining batch notes.
				for _, ln := range batch {
					m.recordFailure(ln.note, StageMilvusLoad, attempt, ctx.Err(), FailureDead)
				}
				return attempt, lastErr
			case <-time.After(m.backoffFor(attempt)):
			}
		}
	}
	for _, ln := range batch {
		// IN-03: record the REAL attempt number at which seed failed.
		// After the loop, attempt == maxAttempts+1, so attempt-1 is the real count.
		m.recordFailure(ln.note, StageMilvusLoad, attempt-1, lastErr, FailureDead)
	}
	return attempt - 1, lastErr
}

// recordFailure writes one ledger row. Upsert errors are WARN-logged, not
// fatal: losing a dead-letter row must never abort the batch (the run's
// job is the corpus, the ledger is its bookkeeping).
func (m *Manager) recordFailure(note Note, stage string, attempt int, err error, status string) {
	if m.failures == nil {
		return
	}
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	f := Failure{
		NoteID:       note.SourceIdx,
		Stage:        stage,
		AttemptCount: attempt,
		LastError:    lastError,
		Status:       status,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if uerr := m.failures.UpsertFailure(f); uerr != nil {
		fmt.Fprintf(os.Stderr, "[WARN] ecl.Manager: record failure row for note %s: %v\n", note.SourceIdx, uerr)
	}
}

// resolveIfFailed flips an existing failure row to resolved after a late
// success. Notes with no ledger row stay unrecorded — the ledger is
// failure-only until a row exists (a first-try success writes nothing).
func (m *Manager) resolveIfFailed(note Note, stage string, attempt int) {
	row, err := m.failures.GetFailure(note.SourceIdx)
	if err != nil || row.Status == FailureResolved {
		return
	}
	m.recordFailure(note, stage, attempt, nil, FailureResolved)
}

// backoffFor returns the sleep before the retry following failed attempt n.
func (m *Manager) backoffFor(attempt int) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if attempt < 1 || attempt > len(m.backoff) {
		return 0
	}
	return m.backoff[attempt-1]
}

// firstSourcePath is the extract-stage current_file value: the note's
// first fragment path (slash-normalized), or "" when unavailable.
func firstSourcePath(n Note) string {
	if len(n.Sections) > 0 && len(n.Sections[0].SourcePaths) > 0 {
		return filepath.ToSlash(n.Sections[0].SourcePaths[0])
	}
	return ""
}
