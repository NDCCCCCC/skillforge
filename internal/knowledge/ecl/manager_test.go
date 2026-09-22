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

// Phase 39 / Plan 39-05 Task 3 — Manager orchestrator tests. Hermetic:
// real temp-dir corpus files (WalkAndMerge needs a filesystem), in-memory
// KG store, map-backed fake FailureStore (the Plan-wide test fake), fake
// SeedFunc, stub extractor with forced-failure and concurrency counting,
// 0-duration backoff.
package ecl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/ontology/store"
	"github.com/skillforge/skillforge/internal/retriever"
)

// --- fixtures -------------------------------------------------------------

// writeFragment writes one __secNN fragment file into dir.
func writeFragment(t *testing.T, dir, fileName, sourceIdx, section, body string) {
	t.Helper()
	content := fmt.Sprintf("---\ntitle: 测试笔记 %s\nsource_idx: \"%s\"\nsection: \"%s\"\n---\n\n%s\n",
		sourceIdx, sourceIdx, section, body)
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o644))
}

// threeNoteKBDir builds a temp corpus with notes 001/002/003, one SQL
// section each touching a distinct etl_ table.
func threeNoteKBDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFragment(t, dir, "001_alpha__sec01_SQL.md", "001", "SQL", "update etl_alpha_t set x = 1;")
	writeFragment(t, dir, "002_beta__sec01_SQL.md", "002", "SQL", "update etl_beta_t set x = 1;")
	writeFragment(t, dir, "003_gamma__sec01_SQL.md", "003", "SQL", "update etl_gamma_t set x = 1;")
	return dir
}

// sixNoteKBDir builds a 6-note corpus for the concurrency-cap test.
func sixNoteKBDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for i := 1; i <= 6; i++ {
		idx := fmt.Sprintf("%03d", i)
		writeFragment(t, dir, idx+"_n__sec01_SQL.md", idx, "SQL",
			fmt.Sprintf("update etl_conc_%s_t set x = 1;\n\n正文段落，用于拉长抽取耗时以制造并发重叠。\n", idx))
	}
	return dir
}

// flakyExtractor is the controllable stub: failures[sourceIdx] counts
// forced failures before success; it also tracks the extract-stage
// concurrency ceiling for the ≤3 worker cap assertion.
type flakyExtractor struct {
	mu           sync.Mutex
	failures     map[string]int
	calls        map[string]int
	inflight     int
	maxInflight  int
	waitPerCalls time.Duration
}

func newFlakyExtractor(failures map[string]int) *flakyExtractor {
	return &flakyExtractor{
		failures: failures,
		calls:    map[string]int{},
	}
}

func (f *flakyExtractor) ExtractNote(_ context.Context, note Note) (Extraction, error) {
	f.mu.Lock()
	f.inflight++
	if f.inflight > f.maxInflight {
		f.maxInflight = f.inflight
	}
	f.calls[note.SourceIdx]++
	f.mu.Unlock()

	if f.waitPerCalls > 0 {
		time.Sleep(f.waitPerCalls)
	}
	defer func() {
		f.mu.Lock()
		f.inflight--
		f.mu.Unlock()
	}()

	f.mu.Lock()
	remaining := f.failures[note.SourceIdx]
	if remaining > 0 {
		f.failures[note.SourceIdx] = remaining - 1
	}
	f.mu.Unlock()
	if remaining > 0 {
		return Extraction{}, fmt.Errorf("forced extract failure for %s", note.SourceIdx)
	}
	return Extraction{
		Tables: []ExtractedTable{{Name: "etl_" + note.SourceIdx + "_t", Kind: "update"}},
	}, nil
}

func (f *flakyExtractor) maxObservedConcurrency() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxInflight
}

// fakeFailureStore is the Plan-wide map-backed test fake.
type fakeFailureStore struct {
	mu   sync.Mutex
	rows map[string]Failure
}

func newFakeFailureStore() *fakeFailureStore {
	return &fakeFailureStore{rows: map[string]Failure{}}
}

func (f *fakeFailureStore) UpsertFailure(fl Failure) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[fl.NoteID] = fl
	return nil
}

func (f *fakeFailureStore) ListFailures() ([]Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Failure, 0, len(f.rows))
	for _, r := range f.rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

func (f *fakeFailureStore) GetFailure(noteID string) (Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[noteID]
	if !ok {
		return Failure{}, fmt.Errorf("ecl: no failure record for note %s", noteID)
	}
	return r, nil
}

// fakeSeed records the seeded docs; failRemaining forces N batch failures.
type fakeSeed struct {
	mu            sync.Mutex
	calls         int
	docs          []retriever.Document
	failRemaining int
}

func (f *fakeSeed) seed(_ context.Context, docs []retriever.Document) error {
	f.mu.Lock()
	f.calls++
	remaining := f.failRemaining
	if remaining > 0 {
		f.failRemaining--
	}
	f.mu.Unlock()
	if remaining > 0 {
		return fmt.Errorf("forced seed failure")
	}
	f.mu.Lock()
	f.docs = docs
	f.mu.Unlock()
	return nil
}

// newTestManager wires a manager with 0-duration backoff and a sequential
// ID allocator.
func newTestManager(t *testing.T, kbDir string, ex Extractor, fs FailureStore, seed SeedFunc) (*Manager, *store.Store) {
	t.Helper()
	kg := store.New()
	m := NewManager("http://skillforge.dev/ns#", kbDir, kg, ex, fs, seed)
	m.backoff = []time.Duration{0, 0, 0}
	m.SetIDAllocator(seqAllocator(1))
	return m, kg
}

// deadRow asserts row(noteID) is dead at the given stage with maxAttempts.
func deadRow(t *testing.T, fs FailureStore, noteID, stage string) Failure {
	t.Helper()
	row, err := fs.GetFailure(noteID)
	require.NoError(t, err)
	assert.Equal(t, FailureDead, row.Status, "note %s should be dead", noteID)
	assert.Equal(t, stage, row.Stage, "note %s dead at wrong stage", noteID)
	assert.Equal(t, maxAttempts, row.AttemptCount, "note %s should record 3 failed attempts", noteID)
	return row
}

// --- tests ---------------------------------------------------------------

// TestManagerHappyPath: 3 healthy notes run to done; the seed function saw
// Documents with the section metadata; the KG grew by the ABox triples.
func TestManagerHappyPath(t *testing.T) {
	ex := newFlakyExtractor(nil)
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, kg := newTestManager(t, threeNoteKBDir(t), ex, fs, seed.seed)

	require.True(t, m.CASRunning(), "handler protocol: claim before Run")
	require.NoError(t, m.Run(context.Background()))

	st := m.Status()
	assert.Equal(t, StageDone, st.Stage)
	assert.Equal(t, 3, st.Total)
	assert.Empty(t, st.Failures, "first-try successes write no ledger rows")

	require.Len(t, seed.docs, 3)
	for _, d := range seed.docs {
		assert.Equal(t, "sql-guides", d.Tier)
		assert.Equal(t, int64(1), d.SectionCount)
		assert.Equal(t, int64(1), d.SectionIdx)
		assert.NotEmpty(t, d.Section)
		assert.NotEmpty(t, d.SourceIdx)
		assert.NotEmpty(t, d.Content)
	}
	// Per note: rdf:type + referencesTable + TableSchema type = 3 triples.
	assert.Equal(t, 9, kg.Len(), "KG grew by 3 triples per note")
}

// TestManagerExtractDeadLetterContinues: extractor fails note 002 three
// times → 002 parks dead at extract with 3 recorded attempts while 001/003
// complete the batch (D-RT-01 single-note isolation).
func TestManagerExtractDeadLetterContinues(t *testing.T) {
	ex := newFlakyExtractor(map[string]int{"002": maxAttempts})
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, kg := newTestManager(t, threeNoteKBDir(t), ex, fs, seed.seed)

	require.True(t, m.CASRunning())
	require.NoError(t, m.Run(context.Background()), "single-note failure must not fail the run")

	st := m.Status()
	assert.Equal(t, StageDone, st.Stage)

	deadRow(t, fs, "002", StageExtract)
	_, err := fs.GetFailure("001")
	assert.Error(t, err, "healthy notes leave no ledger rows")
	_, err = fs.GetFailure("003")
	assert.Error(t, err)

	dead := false
	for _, f := range st.Failures {
		if f.NoteID == "002" && f.Status == FailureDead {
			dead = true
		}
	}
	assert.True(t, dead, "status failures[] must show 002 dead")

	// Batch continued: 2 notes loaded and seeded; KG holds only theirs.
	assert.Len(t, seed.docs, 2)
	assert.Equal(t, 6, kg.Len())

	// Each forced failure was really attempted 3 times.
	assert.Equal(t, maxAttempts, ex.calls["002"])
}

// TestManagerRetryNoteResolves: after the extractor recovers, RetryNote
// re-runs the FULL pipeline for the dead note and resolves its row.
func TestManagerRetryNoteResolves(t *testing.T) {
	ex := newFlakyExtractor(map[string]int{"002": maxAttempts})
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, kg := newTestManager(t, threeNoteKBDir(t), ex, fs, seed.seed)

	require.True(t, m.CASRunning())
	require.NoError(t, m.Run(context.Background()))
	require.Equal(t, 6, kg.Len())

	require.NoError(t, m.RetryNote(context.Background(), "002"))

	row, err := fs.GetFailure("002")
	require.NoError(t, err)
	assert.Equal(t, FailureResolved, row.Status)

	// Full-note re-run converged: KG now holds 002's triples too (idempotent
	// note-level replace, D-Run-01); the retry seeded only 002's docs — a
	// single-note re-run never re-seeds the whole batch.
	assert.Equal(t, 9, kg.Len())
	require.Len(t, seed.docs, 1)
	assert.Equal(t, "002", seed.docs[0].SourceIdx)
	assert.Equal(t, StageDone, m.Status().Stage)
}

// TestManagerRetryNoteGuards: busy → ErrPipelineBusy; unknown or non-dead
// note → ErrUnknownNote.
func TestManagerRetryNoteGuards(t *testing.T) {
	ex := newFlakyExtractor(nil)
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, _ := newTestManager(t, threeNoteKBDir(t), ex, fs, seed.seed)

	// Claim the flag (simulates an in-flight run).
	require.True(t, m.CASRunning())
	assert.ErrorIs(t, m.RetryNote(context.Background(), "002"), ErrPipelineBusy)
	m.releaseRunning()

	// No ledger row at all.
	assert.ErrorIs(t, m.RetryNote(context.Background(), "999"), ErrUnknownNote)

	// A PENDING row is not retryable — only DEAD notes re-enqueue.
	require.NoError(t, fs.UpsertFailure(Failure{
		NoteID: "001", Stage: StageExtract, AttemptCount: 1,
		Status: FailurePending, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}))
	assert.ErrorIs(t, m.RetryNote(context.Background(), "001"), ErrUnknownNote)
}

// TestManagerCASRunning: second concurrent claim is refused.
func TestManagerCASRunning(t *testing.T) {
	ex := newFlakyExtractor(nil)
	m, _ := newTestManager(t, threeNoteKBDir(t), ex, newFakeFailureStore(), (&fakeSeed{}).seed)

	assert.True(t, m.CASRunning())
	assert.False(t, m.CASRunning(), "second claim while running must be refused")
	m.releaseRunning()
	assert.True(t, m.CASRunning(), "flag is claimable again after release")
	m.releaseRunning()
}

// TestManagerKGLoadDeadLetter: the kg-load leg is force-failed through the
// applyNote seam for note 002 — it retries ×3 then parks dead with the row
// tagged "kg-load" while the batch continues and the KG only holds the
// other notes' triples (D-RT-01's second retry stage).
func TestManagerKGLoadDeadLetter(t *testing.T) {
	ex := newFlakyExtractor(nil)
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, kg := newTestManager(t, threeNoteKBDir(t), ex, fs, seed.seed)

	boom := errors.New("forced kg-load failure")
	m.applyNote = func(st *store.Store, ns string, note Note, exN Extraction, onChange func()) error {
		if note.SourceIdx == "002" {
			return boom
		}
		return ApplyNote(st, ns, note, exN, onChange)
	}

	require.True(t, m.CASRunning())
	require.NoError(t, m.Run(context.Background()))

	deadRow(t, fs, "002", StageKGLoad)
	assert.Equal(t, 6, kg.Len(), "KG reflects only the non-failing notes")
	assert.Len(t, seed.docs, 2, "dead note never reaches milvus-load")
	assert.Equal(t, StageDone, m.Status().Stage)
}

// TestManagerRecordFailureStages: recordFailure unit test — the stage tag
// and the pending→dead transition are correct across ALL THREE retry
// stage names (extract / kg-load / milvus-load).
func TestManagerRecordFailureStages(t *testing.T) {
	fs := newFakeFailureStore()
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, (&fakeSeed{}).seed)

	stages := []string{StageExtract, StageKGLoad, StageMilvusLoad}
	for _, stage := range stages {
		note := Note{SourceIdx: "007"}
		m.recordFailure(note, stage, 1, errors.New("boom 1"), FailurePending)
		row, err := fs.GetFailure("007")
		require.NoError(t, err)
		assert.Equal(t, stage, row.Stage)
		assert.Equal(t, FailurePending, row.Status)
		assert.Equal(t, 1, row.AttemptCount)
		assert.Equal(t, "boom 1", row.LastError)
		assert.NotEmpty(t, row.UpdatedAt)
		_, perr := time.Parse(time.RFC3339, row.UpdatedAt)
		assert.NoError(t, perr, "updated_at must be an RFC3339 string")

		m.recordFailure(note, stage, maxAttempts, errors.New("boom 3"), FailureDead)
		row, err = fs.GetFailure("007")
		require.NoError(t, err)
		assert.Equal(t, stage, row.Stage)
		assert.Equal(t, FailureDead, row.Status)
		assert.Equal(t, maxAttempts, row.AttemptCount)
	}
}

// TestManagerStatusDTOWireShape: the status DTO marshals to the EXACT
// locked JSON keys (D-Prog-01 / 39-UI-SPEC §Data Contract) — top-level
// stage/current_file/done/total/queue_len/failures, failure objects
// note_id/stage/attempt_count/last_error/status/updated_at.
func TestManagerStatusDTOWireShape(t *testing.T) {
	fs := newFakeFailureStore()
	require.NoError(t, fs.UpsertFailure(Failure{
		NoteID:       "002",
		Stage:        StageExtract,
		AttemptCount: 3,
		LastError:    "forced extract failure for 002",
		Status:       FailureDead,
		UpdatedAt:    "2026-09-22T03:04:05Z",
	}))
	m, _ := newTestManager(t, threeNoteKBDir(t), newFlakyExtractor(nil), fs, (&fakeSeed{}).seed)

	raw, err := json.Marshal(m.Status())
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &top))
	wantTop := []string{"stage", "current_file", "done", "total", "queue_len", "failures"}
	require.Len(t, top, len(wantTop))
	for _, k := range wantTop {
		assert.Contains(t, top, k)
	}
	var stage string
	require.NoError(t, json.Unmarshal(top["stage"], &stage))
	assert.Equal(t, StageIdle, stage)

	var failures []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(top["failures"], &failures))
	require.Len(t, failures, 1)
	wantFailure := []string{"note_id", "stage", "attempt_count", "last_error", "status", "updated_at"}
	require.Len(t, failures[0], len(wantFailure))
	for _, k := range wantFailure {
		assert.Contains(t, failures[0], k)
	}
	assert.Equal(t, `"002"`, string(failures[0]["note_id"]))
	assert.Equal(t, `"extract"`, string(failures[0]["stage"]))
	assert.Equal(t, "3", string(failures[0]["attempt_count"]))
	assert.Equal(t, `"dead"`, string(failures[0]["status"]))
	assert.Equal(t, `"2026-09-22T03:04:05Z"`, string(failures[0]["updated_at"]))
}

// TestManagerExtractConcurrencyCap: the extract worker pool never exceeds
// 3 concurrent LLM calls (counting stub, 6-note corpus).
func TestManagerExtractConcurrencyCap(t *testing.T) {
	ex := newFlakyExtractor(nil)
	ex.waitPerCalls = 15 * time.Millisecond
	fs := newFakeFailureStore()
	seed := &fakeSeed{}
	m, _ := newTestManager(t, sixNoteKBDir(t), ex, fs, seed.seed)

	require.True(t, m.CASRunning())
	require.NoError(t, m.Run(context.Background()))
	assert.Equal(t, 6, len(ex.calls), "all notes extracted")
	assert.LessOrEqual(t, ex.maxObservedConcurrency(), extractConcurrency,
		"extract concurrency must never exceed 3")
	assert.Len(t, seed.docs, 6)
}
