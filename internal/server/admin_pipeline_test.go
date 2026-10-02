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

// Phase 39 / Plan 39-07 Task 2 — ECL pipeline admin handler tests
// (SC-4/SC-5 server half, D-Route-01/D-Prog-01). Hermetic: real temp-dir
// corpus files (WalkAndMerge needs a filesystem), in-memory KG store,
// map-backed fake FailureStore (the Plan-wide test fake), fake SeedFunc
// that signals a channel, all-succeeding stub extractor — no backoff
// sleeps occur because nothing fails, so the unexported backoff field
// never needs touching from this external test package.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
	sqlstore "github.com/skillforge/skillforge/internal/store"
)

// --- fixtures -------------------------------------------------------------

// writePipelineFragment writes one __secNN fragment file into dir (the
// manager_test.go fixture shape: secRegex base name AND non-empty section
// frontmatter, both required by WalkFragments).
func writePipelineFragment(t *testing.T, dir, fileName, sourceIdx, section, body string) {
	t.Helper()
	content := fmt.Sprintf("---\ntitle: 测试笔记 %s\nsource_idx: \"%s\"\nsection: \"%s\"\n---\n\n%s\n",
		sourceIdx, sourceIdx, section, body)
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write fragment %s: %v", fileName, err)
	}
}

// pipelineKBDir builds a temp corpus with notes 001/002/007 (007 exists so
// the dead-letter retry happy path can walk it back out of the corpus).
func pipelineKBDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writePipelineFragment(t, dir, "001_alpha__sec01_SQL.md", "001", "SQL", "update etl_alpha_t set x = 1;")
	writePipelineFragment(t, dir, "002_beta__sec01_SQL.md", "002", "SQL", "update etl_beta_t set x = 1;")
	writePipelineFragment(t, dir, "007_zeta__sec01_SQL.md", "007", "SQL", "update etl_zeta_t set x = 1;")
	return dir
}

// stubPipelineExtractor always succeeds with one update table per note.
type stubPipelineExtractor struct{}

func (stubPipelineExtractor) ExtractNote(_ context.Context, note ecl.Note) (ecl.Extraction, error) {
	return ecl.Extraction{
		Tables: []ecl.ExtractedTable{{Name: "etl_" + note.SourceIdx + "_t", Kind: "update"}},
	}, nil
}

// fakePipelineFailures is the map-backed ecl.FailureStore fake (manager_test.go
// fakePipelineFailureStore pattern, replicated for the external test package).
type fakePipelineFailures struct {
	mu   sync.Mutex
	rows map[string]ecl.Failure
}

func newFakePipelineFailures() *fakePipelineFailures {
	return &fakePipelineFailures{rows: map[string]ecl.Failure{}}
}

func (f *fakePipelineFailures) UpsertFailure(fl ecl.Failure) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[fl.NoteID] = fl
	return nil
}

func (f *fakePipelineFailures) ListFailures() ([]ecl.Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ecl.Failure, 0, len(f.rows))
	for _, r := range f.rows {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakePipelineFailures) GetFailure(noteID string) (ecl.Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[noteID]
	if !ok {
		return ecl.Failure{}, fmt.Errorf("ecl: no failure record for note %s", noteID)
	}
	return r, nil
}

func (f *fakePipelineFailures) IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no failure record")
}

// fakePipelineSeed records seeded docs and signals its channel once per
// call (deterministic "the run actually executed" witness).
type fakePipelineSeed struct {
	mu     sync.Mutex
	docs   []retriever.Document
	signal chan struct{}
}

func (f *fakePipelineSeed) seed(_ context.Context, docs []retriever.Document) error {
	f.mu.Lock()
	f.docs = append(f.docs, docs...)
	f.mu.Unlock()
	if f.signal != nil {
		select {
		case f.signal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (f *fakePipelineSeed) seeded() []retriever.Document {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]retriever.Document(nil), f.docs...)
}

// newPipelineManager wires Deps{ECL: mgr} over the 3-note temp corpus with
// an all-succeeding stub extractor, fake ledger, fake seed, and a
// sequential ID allocator (exported injection points only — this is the
// external-package construction Plan 06's boot wiring mirrors).
func newPipelineManager(t *testing.T) (*ecl.Manager, *fakePipelineSeed, *fakePipelineFailures) {
	t.Helper()
	seed := &fakePipelineSeed{signal: make(chan struct{}, 1)}
	fs := newFakePipelineFailures()
	mgr := ecl.NewManager("http://skillforge.dev/ns#", pipelineKBDir(t), store.New(),
		stubPipelineExtractor{}, fs, seed.seed)
	next := int64(1)
	mgr.SetIDAllocator(func() int64 {
		id := next
		next++
		return id
	})
	return mgr, seed, fs
}

// --- harness --------------------------------------------------------------

func pipelineRoutes(h *server.Hertz, deps *Deps) {
	h.POST("/api/admin/pipeline/run", deps.adminPipelineRunHandler)
	h.GET("/api/admin/pipeline/status", deps.adminPipelineStatusHandler)
	h.POST("/api/admin/pipeline/retry/:note_id", deps.adminPipelineRetryHandler)
}

func invokePipeline(t *testing.T, deps *Deps, method, path, token string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	pipelineRoutes(h, deps)
	var headers []ut.Header
	if token != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: token})
	}
	return ut.PerformRequest(h.Engine, method, path, nil, headers...)
}

// awaitStage polls Status until the stage sentinel is reached or the
// deadline expires (the async run goroutine is outside the request
// lifecycle, so tests must poll).
func awaitStage(t *testing.T, mgr *ecl.Manager, stage string) ecl.StatusDTO {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := mgr.Status()
		if st.Stage == stage {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipeline never reached stage %q (now %q)", stage, st.Stage)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- tests ----------------------------------------------------------------

// TestAdminPipelineGateOrder: no token → 403 on ALL 3 routes EVEN WHEN
// deps.ECL is nil (the locked 403-before-503 regression guard, D-Route-01 /
// T-39-19 — an unauthenticated caller must not learn wiring state); valid
// token + nil ECL → 503 "ECL pipeline not wired".
func TestAdminPipelineGateOrder(t *testing.T) {
	deps := &Deps{} // ECL deliberately nil
	paths := []struct{ method, path string }{
		{http.MethodPost, "/api/admin/pipeline/run"},
		{http.MethodGet, "/api/admin/pipeline/status"},
		{http.MethodPost, "/api/admin/pipeline/retry/007"},
	}

	for _, p := range paths {
		resp := invokePipeline(t, deps, p.method, p.path, "")
		if resp.Code != http.StatusForbidden {
			t.Errorf("%s %s without token (ECL nil): expected 403, got %d", p.method, p.path, resp.Code)
		}
		var body struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Errorf("%s %s: unmarshal 403 body: %v", p.method, p.path, err)
			continue
		}
		if body.Error != "admin token required" {
			t.Errorf("%s %s: 403 error = %q, want %q", p.method, p.path, body.Error, "admin token required")
		}
	}

	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	for _, p := range paths {
		resp := invokePipeline(t, deps, p.method, p.path, "tok")
		if resp.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s with token (ECL nil): expected 503, got %d", p.method, p.path, resp.Code)
		}
		if !strings.Contains(resp.Body.String(), "ECL pipeline not wired") {
			t.Errorf("%s %s: 503 body = %s", p.method, p.path, resp.Body.String())
		}
	}
}

// TestAdminPipelineRunConflict: busy manager (running flag claimed) →
// 409 "pipeline already running"; free manager → 200 {"status":"started"}
// and the run actually executes (seed channel witness + stage done).
func TestAdminPipelineRunConflict(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")

	// Busy: claim the running flag directly (simulates an in-flight run).
	busyMgr, _, _ := newPipelineManager(t)
	if !busyMgr.CASRunning() {
		t.Fatal("CASRunning claim failed")
	}
	resp := invokePipeline(t, &Deps{ECL: busyMgr}, http.MethodPost, "/api/admin/pipeline/run", "tok")
	if resp.Code != http.StatusConflict {
		t.Fatalf("run while busy: expected 409, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "pipeline already running") {
		t.Errorf("409 body = %s", resp.Body.String())
	}

	// Free: a fresh manager — the run POST must start it for real.
	freeMgr, seed, _ := newPipelineManager(t)
	resp = invokePipeline(t, &Deps{ECL: freeMgr}, http.MethodPost, "/api/admin/pipeline/run", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("run on free manager: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var started struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal run response: %v", err)
	}
	if started.Status != "started" {
		t.Errorf("run status = %q, want %q", started.Status, "started")
	}

	// The detached goroutine really ran the pipeline: the seed signals
	// from the milvus-load stage, then the run settles at done.
	select {
	case <-seed.signal:
	case <-time.After(5 * time.Second):
		t.Fatal("seed never saw documents — the run did not execute")
	}
	st := awaitStage(t, freeMgr, ecl.StageDone)
	if st.Total != 3 {
		t.Errorf("total = %d, want 3", st.Total)
	}
	if len(seed.seeded()) != 3 {
		t.Errorf("seeded docs = %d, want 3", len(seed.seeded()))
	}
}

// TestAdminPipelineStatusShape: the status endpoint serializes the EXACT
// locked key sets (D-Prog-01 / 39-UI-SPEC §Data Contract) — top-level
// {stage, current_file, done, total, queue_len, failures} and failure
// objects {note_id, stage, attempt_count, last_error, status, updated_at},
// no extra keys.
func TestAdminPipelineStatusShape(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	mgr, _, fs := newPipelineManager(t)
	if err := fs.UpsertFailure(ecl.Failure{
		NoteID:       "007",
		Stage:        ecl.StageExtract,
		AttemptCount: 3,
		LastError:    "forced extract failure for 007",
		Status:       ecl.FailureDead,
		UpdatedAt:    "2026-09-22T03:04:05Z",
	}); err != nil {
		t.Fatalf("prime dead row: %v", err)
	}

	resp := invokePipeline(t, &Deps{ECL: mgr}, http.MethodGet, "/api/admin/pipeline/status", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("status: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(resp.Body.Bytes(), &top); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	wantTop := []string{"stage", "current_file", "done", "total", "queue_len", "failures"}
	if len(top) != len(wantTop) {
		t.Errorf("top-level keys = %d (%v), want exactly %d", len(top), top, len(wantTop))
	}
	for _, k := range wantTop {
		if _, ok := top[k]; !ok {
			t.Errorf("top-level key %q missing", k)
		}
	}

	var failures []map[string]json.RawMessage
	if err := json.Unmarshal(top["failures"], &failures); err != nil {
		t.Fatalf("unmarshal failures: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %d rows, want 1", len(failures))
	}
	wantFailure := []string{"note_id", "stage", "attempt_count", "last_error", "status", "updated_at"}
	if len(failures[0]) != len(wantFailure) {
		t.Errorf("failure keys = %d (%v), want exactly %d", len(failures[0]), failures[0], len(wantFailure))
	}
	for _, k := range wantFailure {
		if _, ok := failures[0][k]; !ok {
			t.Errorf("failure key %q missing", k)
		}
	}
	if string(failures[0]["note_id"]) != `"007"` {
		t.Errorf("failure note_id = %s, want \"007\"", failures[0]["note_id"])
	}
	if string(failures[0]["status"]) != `"dead"` {
		t.Errorf("failure status = %s, want \"dead\"", failures[0]["status"])
	}
}

// awaitRowStatus polls the ledger until noteID's row reaches status (the
// retry work is detached from the request goroutine — WR-01 — so tests
// poll, same posture as awaitStage).
func awaitRowStatus(t *testing.T, fs *fakePipelineFailures, noteID, status string) ecl.Failure {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		row, err := fs.GetFailure(noteID)
		if err == nil && row.Status == status {
			return row
		}
		if time.Now().After(deadline) {
			t.Fatalf("note %s never reached status %q (row %+v, err %v)", noteID, status, row, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAdminPipelineRetryValidation: "abc" → 400 "invalid note id"
// (format gate fires before the manager is consulted); "999" with no
// failure row → 404 "unknown or non-dead note"; dead "007" → 200 queued
// and the DETACHED retry (WR-01) actually resolves the row (full-note
// pipeline — polled, not synchronous); retry while busy → 409 (asserted
// last — the test-held CAS claim is never released, the flag API is
// manager-internal).
func TestAdminPipelineRetryValidation(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	mgr, seed, fs := newPipelineManager(t)
	deps := &Deps{ECL: mgr}

	// Malformed id → 400 before the manager sees it.
	resp := invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/abc", "tok")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("retry/abc: expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "invalid note id") {
		t.Errorf("400 body = %s", resp.Body.String())
	}

	// Well-formed but no ledger row → 404.
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/999", "tok")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("retry/999: expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "unknown or non-dead note") {
		t.Errorf("404 body = %s", resp.Body.String())
	}

	// A PENDING row is not retryable — only DEAD re-enqueues (manager guard).
	if err := fs.UpsertFailure(ecl.Failure{
		NoteID: "002", Stage: ecl.StageExtract, AttemptCount: 1,
		Status: ecl.FailurePending, UpdatedAt: "2026-09-22T03:04:05Z",
	}); err != nil {
		t.Fatalf("prime pending row: %v", err)
	}
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/002", "tok")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("retry/002 (pending): expected 404, got %d: %s", resp.Code, resp.Body.String())
	}

	// Dead 007 → 200 queued; the retry work runs detached (WR-01) and the
	// full-note pipeline is observed via polling.
	if err := fs.UpsertFailure(ecl.Failure{
		NoteID: "007", Stage: ecl.StageExtract, AttemptCount: 3,
		LastError: "forced extract failure for 007", Status: ecl.FailureDead,
		UpdatedAt: "2026-09-22T03:04:05Z",
	}); err != nil {
		t.Fatalf("prime dead row: %v", err)
	}
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/007", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("retry/007 (dead): expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var queued struct {
		Status string `json:"status"`
		NoteID string `json:"note_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &queued); err != nil {
		t.Fatalf("unmarshal retry response: %v", err)
	}
	if queued.Status != "queued" || queued.NoteID != "007" {
		t.Errorf("retry response = %+v, want {queued 007}", queued)
	}
	row := awaitRowStatus(t, fs, "007", ecl.FailureResolved)
	if row.Stage != ecl.StageMilvusLoad {
		t.Errorf("resolved row stage = %q, want milvus-load (the final stage)", row.Stage)
	}
	// The detached work really seeded 007's documents and the run settled
	// at the done terminal sentinel.
	seeded := seed.seeded()
	if len(seeded) != 1 || seeded[0].SourceIdx != "007" {
		t.Errorf("retry seeded %+v, want exactly note 007", seeded)
	}
	st := awaitStage(t, mgr, ecl.StageDone)
	if st.Total != 1 {
		t.Errorf("status total after retry = %d, want 1 (single-note run)", st.Total)
	}

	// Busy: claim the flag, retry answers 409 regardless of row state.
	if !mgr.CASRunning() {
		t.Fatal("CASRunning claim failed")
	}
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/007", "tok")
	if resp.Code != http.StatusConflict {
		t.Fatalf("retry while busy: expected 409, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "pipeline already running") {
		t.Errorf("409 body = %s", resp.Body.String())
	}
}

// TestAdminPipelineRunStarts: happy run over the 3-note fixture (stub
// extractor, fake seed) reaches stage done with an empty failure ledger.
func TestAdminPipelineRunStarts(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	mgr, _, fs := newPipelineManager(t)

	resp := invokePipeline(t, &Deps{ECL: mgr}, http.MethodPost, "/api/admin/pipeline/run", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	st := awaitStage(t, mgr, ecl.StageDone)
	if st.Total != 3 {
		t.Errorf("total = %d, want 3", st.Total)
	}
	if st.Done != 3 {
		t.Errorf("done = %d, want 3", st.Done)
	}
	if st.QueueLen != 0 {
		t.Errorf("queue_len = %d, want 0", st.QueueLen)
	}
	rows, err := fs.ListFailures()
	if err != nil {
		t.Fatalf("list failures: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("failures = %+v, want empty (first-try successes write no rows)", rows)
	}
}

// pipelineAuditCapture is a minimal store.Store stub capturing PutAudit
// rows (embedded nil interface = INSERT-only lock: any other store method
// the audit path might reach panics, same discipline as the package's
// other fake stores).
type pipelineAuditCapture struct {
	sqlstore.Store
	mu   sync.Mutex
	rows []sqlstore.AuditRow
}

func (f *pipelineAuditCapture) PutAudit(_ context.Context, row sqlstore.AuditRow) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, row)
	return int64(len(f.rows)), nil
}

func (f *pipelineAuditCapture) captured() []sqlstore.AuditRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sqlstore.AuditRow(nil), f.rows...)
}

// TestAdminPipelineAuditRows (WR-05 / T-3-AUDIT): an ACCEPTED run/retry
// writes exactly one INSERT-only admin audit row (action_ecl_run /
// action_ecl_retry, ActorType "admin" so ADM-06 surfaces it); refused
// triggers (404 unknown note) write nothing; the acceptance row is
// written synchronously before the response, independent of the async
// outcome.
func TestAdminPipelineAuditRows(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
	mgr, seed, fs := newPipelineManager(t)
	audit := &pipelineAuditCapture{}
	deps := &Deps{ECL: mgr, Store: audit}

	// Accepted run → exactly one action_ecl_run row, written at acceptance.
	resp := invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/run", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("run: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	rows := audit.captured()
	if len(rows) != 1 {
		t.Fatalf("audit rows after accepted run = %d, want 1", len(rows))
	}
	if rows[0].Action != "action_ecl_run" || rows[0].ActorType != "admin" || rows[0].UserID != "admin" {
		t.Errorf("run audit row = action %q actor %q user %q, want action_ecl_run/admin/admin",
			rows[0].Action, rows[0].ActorType, rows[0].UserID)
	}
	if rows[0].Timestamp.IsZero() {
		t.Error("run audit row timestamp is zero")
	}

	// Let the detached run finish (so the manager is claimable again).
	awaitStage(t, mgr, ecl.StageDone)

	// Refused retry (unknown note, 404) writes NO audit row.
	if err := fs.UpsertFailure(ecl.Failure{
		NoteID: "007", Stage: ecl.StageExtract, AttemptCount: 3,
		LastError: "forced extract failure for 007", Status: ecl.FailureDead,
		UpdatedAt: "2026-09-22T03:04:05Z",
	}); err != nil {
		t.Fatalf("prime dead row: %v", err)
	}
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/999", "tok")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("retry/999: expected 404, got %d", resp.Code)
	}
	if got := len(audit.captured()); got != 1 {
		t.Fatalf("audit rows after refused retry = %d, want still 1 (refusals are not audit-worthy)", got)
	}

	// Accepted retry → one action_ecl_retry row carrying the note id.
	resp = invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/007", "tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("retry/007: expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	rows = audit.captured()
	if len(rows) != 2 {
		t.Fatalf("audit rows after accepted retry = %d, want 2", len(rows))
	}
	if rows[1].Action != "action_ecl_retry" || rows[1].ActorType != "admin" {
		t.Errorf("retry audit row = action %q actor %q, want action_ecl_retry/admin",
			rows[1].Action, rows[1].ActorType)
	}
	if !strings.Contains(string(rows[1].FrozenArgs), "007") {
		t.Errorf("retry audit detail = %s, want the trigger note id", rows[1].FrozenArgs)
	}

	// The detached retry work still really runs.
	select {
	case <-seed.signal:
	case <-time.After(5 * time.Second):
		t.Fatal("retry never seeded — the detached work did not run")
	}
}
