---
phase: "42-v1-4-outstanding-closure"
reviewed: 2026-09-24T00:00:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - D:/code/ClaudeCode/ai-agent/cmd/server/main.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/abox.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/extract.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/extract_in01_test.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/extract_in02_test.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/failures.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/manager.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/manager_in08_test.go
  - D:/code/ClaudeCode/ai-agent/internal/knowledge/ecl/manager_test.go
  - D:/code/ClaudeCode/ai-agent/internal/ontology/store/store.go
  - D:/code/ClaudeCode/ai-agent/internal/retriever/milvus/watermark_in07_test.go
  - D:/code/ClaudeCode/ai-agent/internal/server/admin_pipeline.go
  - D:/code/ClaudeCode/ai-agent/internal/server/admin_pipeline_in05_test.go
  - D:/code/ClaudeCode/ai-agent/internal/server/admin_pipeline_test.go
  - D:/code/ClaudeCode/ai-agent/internal/store/sqlite/ecl_failures.go
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
---

# Phase 42: Code Review Report

**Reviewed:** 2026-09-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Phase 42 applies 9 IN-* fixes from 39-REVIEW.md to the ECL pipeline (`extract.go`, `manager.go`, `abox.go`, `admin_pipeline.go`, `main.go`). All code-level fixes are correctly implemented: IN-01 (dead code removal), IN-02 (section sub-split), IN-03 (real attempt count), IN-04 (store error distinction), IN-05 (panic recovery), IN-06 (ctx-aware backoff), IN-07 (nil manager on watermark failure), IN-08 (stable sort), IN-09 (bounded-impact documentation). The trust boundary posture (T-3-AUDIT INSERT-only, KNOW-02 fail-closed, Tier isolation) is preserved across all changes.

Four warnings are issued: two about missing regression tests (IN-03, IN-04, IN-06), one about an unconventional test that documents broken behavior rather than fixed behavior (IN-07), and one about the 4 missing planned test files.

## Warnings

### WR-01: IN-03 — No dedicated regression test for batch-first-try AttemptCount=1

**File:** `internal/knowledge/ecl/manager.go:403-406`

**Issue:** The IN-03 fix correctly records `attempt=1` (the real failure attempt) when `BuildDocuments` fails on the first seed attempt, instead of overstating it as `maxAttempts` (3). However, `TestManagerRecordFailureStages` (manager_test.go:386) tests `recordFailure` only at the individual stage level and does not exercise the batch-terminal path where `BuildDocuments` fails before any seed attempt. The existing `TestManagerRunFailureResetsStage` (manager_test.go:494) exercises the `seedWithRetry` exhaustion path (attempt=3) but not the first-try `BuildDocuments` failure path.

A `BuildDocuments`-first-failure regression test was specified in the plan (42-02-PLAN.md Task 6: "force BuildDocuments failure first try; assert AttemptCount == 1") but the dedicated test file (`manager_in03_test.go`) was never created. The behavior is correct in the implementation but unverified by a targeted regression test.

**Fix:** Add `TestBatchMilvusLoadRecordsRealAttempt` to a manager test file:
```go
// TestBatchMilvusLoadRecordsRealAttempt: BuildDocuments failure on first try
// records AttemptCount=1 (not maxAttempts=3).
func TestBatchMilvusLoadRecordsRealAttempt(t *testing.T) {
    fs := newFakeFailureStore()
    m := NewManager("http://skillforge.dev/ns#", threeNoteKBDir(t), store.New(),
        newFlakyExtractor(nil), fs, (&fakeSeed{}).seed)
    m.backoff = []time.Duration{0, 0, 0}
    // Force BuildDocuments failure.
    boom := errors.New("build documents failed")
    orig := m.applyNote
    m.applyNote = func(st *store.Store, ns string, note Note, exN Extraction, onChange func()) error {
        return orig(st, ns, note, exN, onChange) // real ApplyNote; seed never called
    }
    // Override seed to never be reached.
    m.seedFn = func(ctx context.Context, docs []retriever.Document) error { return boom }

    require.True(t, m.CASRunning())
    require.Error(t, m.Run(context.Background()))

    row, err := fs.GetFailure("001")
    require.NoError(t, err)
    assert.Equal(t, 1, row.AttemptCount, "first-try batch failure must record AttemptCount=1")
    assert.Equal(t, FailureDead, row.Status)
}
```

### WR-02: IN-04 — No dedicated regression test for store-I/O-error 500 vs not-found 404

**File:** `internal/server/admin_pipeline.go:225-240`

**Issue:** The IN-04 fix correctly distinguishes `ErrStoreUnavailable` (store I/O error, HTTP 500 with generic body) from `ErrUnknownNote` (no DEAD row, HTTP 404). The implementation is correct: `TryClaimRetry` returns `ErrStoreUnavailable` when `GetFailure` returns a non-404 error (ecl_failures.go:148-150, `IsNotFound` checks only `ErrNoFailureRecord`), and the handler maps it to 500. However, there is no dedicated regression test asserting this distinction. `TestAdminPipelineRetryValidation` (admin_pipeline_test.go:387) tests 400 (format), 404 (unknown/dead), and 409 (busy), but not the store-unavailable 500 path. A test was specified in the plan (42-02-PLAN.md Task 3: "mock FailureStore returns generic error; assert HTTP 500 + body not contain internal error text") but `manager_in04_test.go` was never created.

**Fix:** Add `TestRetryNoteHandlerDistinguishesStoreErrors` to `admin_pipeline_test.go` or create `manager_in04_test.go`:
```go
// fakeStoreErrors returns a non-404 error on GetFailure.
type fakeStoreErrors struct{ fakeFailureStore }
func (f fakeStoreErrors) GetFailure(noteID string) (ecl.Failure, error) {
    return ecl.Failure{}, errors.New("sqlite I/O error: database locked")
}
func (f fakeStoreErrors) IsNotFound(err error) bool { return false }

func TestRetryNoteHandlerDistinguishesStoreErrors(t *testing.T) {
    t.Setenv("SKILLFORGE_ADMIN_TOKEN", "tok")
    // Prime a dead row so the store error path is reached (not the unknown-note path).
    fs := newFakePipelineFailures()
    require.NoError(t, fs.UpsertFailure(ecl.Failure{
        NoteID: "001", Stage: ecl.StageExtract, AttemptCount: 3,
        Status: ecl.FailureDead, UpdatedAt: "2026-09-22T03:04:05Z",
    }))
    badFS := &fakeStoreErrors{fs}
    mgr := ecl.NewManager("http://skillforge.dev/ns#", pipelineKBDir(t), store.New(),
        stubPipelineExtractor{}, badFS, func(_ context.Context, _ []retriever.Document) error {
            return errors.New("seed never called")
        })
    deps := &Deps{ECL: mgr}

    resp := invokePipeline(t, deps, http.MethodPost, "/api/admin/pipeline/retry/001", "tok")
    assert.Equal(t, http.StatusInternalServerError, resp.Code)
    assert.NotContains(t, resp.Body.String(), "sqlite") // generic msg
    assert.NotContains(t, resp.Body.String(), "I/O error")
}
```

### WR-03: IN-06 — No dedicated regression test for ctx-aware backoff cancellation

**File:** `internal/knowledge/ecl/manager.go:578-584` (and lines 607-614, 646-656)

**Issue:** The IN-06 fix correctly replaces bare `time.Sleep` with `select { case <-ctx.Done(): ... case <-time.After(...): }` in all three retry loops (`extractWithRetry`, `loadWithRetry`, `seedWithRetry`). The implementation is correct. However, no regression test asserts that cancelling the context during backoff returns within reasonable time and propagates `ctx.Err()`. The plan specified `TestRetryBackoffRespectsCancellation` (42-02-PLAN.md Task 8: "cancel ctx during backoff; assert (a) backoff returns within <100ms; (b) loop exits not exhausted; (c) ctx.Err() propagated") but `manager_in06_test.go` was never created.

**Fix:** Add `TestRetryBackoffRespectsCancellation` to `manager_test.go` or create `manager_in06_test.go`:
```go
func TestRetryBackoffRespectsCancellation(t *testing.T) {
    fs := newFakeFailureStore()
    ex := newFlakyExtractor(nil) // always succeeds; we cancel during backoff
    m, _ := newTestManager(t, threeNoteKBDir(t), ex, fs, (&fakeSeed{}).seed)
    m.backoff = []time.Duration{50 * time.Millisecond, 0, 0}

    ctx, cancel := context.WithCancel(context.Background())

    require.True(t, m.CASRunning())
    // Inject a note that will be retried.
    go func() {
        time.Sleep(10 * time.Millisecond)
        cancel() // cancel during backoff
    }()

    start := time.Now()
    err := m.Run(ctx) // Run uses background context; use a wrapper
    elapsed := time.Since(start)

    assert.True(t, elapsed < 100*time.Millisecond,
        "cancellation should interrupt backoff quickly, took %v", elapsed)
    assert.ErrorIs(t, err, context.Canceled)
}
```

### WR-04: Four planned regression test files were never created

**Files:** `internal/knowledge/ecl/manager_in03_test.go`, `internal/knowledge/ecl/manager_in04_test.go`, `internal/knowledge/ecl/manager_in06_test.go`, `internal/knowledge/ecl/abox_in09_test.go`

**Issue:** The plan (42-02-PLAN.md) specified 9 new regression test files. Only 5 exist:
- `extract_in01_test.go` — created
- `extract_in02_test.go` — created
- `manager_in08_test.go` — created
- `watermark_in07_test.go` — created
- `admin_pipeline_in05_test.go` — created

The following 4 planned files were never created:
- `manager_in03_test.go` (IN-03 batch attempt count regression)
- `manager_in04_test.go` (IN-04 store error vs not-found distinction)
- `manager_in06_test.go` (IN-06 ctx-aware backoff cancellation)
- `abox_in09_test.go` (IN-09 ApplyNote bounded-impact documentation — plan said "no new test needed" but the file was listed as created)

Note: IN-09 is document-only per the plan, and `abox_test.go` already covers ApplyNote idempotency/isolation. The absence of `abox_in09_test.go` is acceptable. The three manager tests (IN-03/04/06) represent genuine coverage gaps.

**Fix:** Create the three missing manager test files with the regression tests described in WR-01, WR-02, and WR-03 above.

## Info

### IN-07: Test documents broken behavior, not fixed behavior

**File:** `internal/retriever/milvus/watermark_in07_test.go`

**Issue:** The IN-07 fix is correct (main.go:1013-1020 breaks out of manager construction when watermark fails, leaving `eclManager = nil` so routes 503). However, the regression test does not assert the fixed behavior (nil manager). Instead, it constructs a manager with `idAlloc` starting at 1 (simulating the broken watermark failure path) and logs that this is "the broken behavior." The test then verifies `mgr.Enabled() == true` — confirming the manager was constructed in the broken scenario — rather than asserting that the fixed scenario (nil manager) is what actually occurs in production.

A more conventional regression test would mock `GetMaxIDCollection` to error and assert `eclManager == nil` in `server.Deps` or that admin routes return 503.

**Fix:** The existing test serves as documentation of the broken/fixed behavior boundary. A stronger regression test would be:
```go
func TestBootECLManagerNilOnWatermarkError(t *testing.T) {
    // This would require injecting a failing GetMaxIDCollection into the
    // buildRetriever path and checking the resulting deps.ECL is nil.
    // Not achievable without modifying buildStore or a store wrapper.
    // The current test documents the correct prevention at the manager level.
}
```

This is **Info** because the IN-07 fix in `main.go:1013-1020` is correct and the test does document the broken/fixed boundary, just not in the conventional "assert fixed behavior" style.

## Verified-Clean Fixes

The following IN-* fixes are correctly implemented with adequate test coverage:

- **IN-01 (dead code removal):** `buildExtractionPrompt` deleted; `joinSectionBodies` + `promptForBody` remain as the under-cap path. `extract_in01_test.go` regression tests confirm the correct behavior.
- **IN-02 (section sub-split):** Section over `MinSplitRunes` (3500 runes) is now sub-split via `subSplit` before LLM calls. `extract_in02_test.go` regression tests confirm multiple LLM calls for oversize sections.
- **IN-08 (stable sort):** `sort.SliceStable` correctly replaces `sort.Slice`; non-numeric SourceIdx values are handled with `false` return (stable relative order preserved). `manager_in08_test.go` regression tests confirm.
- **IN-05 (panic recovery):** `defer recover()` in the detached goroutine resets stage to idle via `mgr.RecoverFromPanic()`. `admin_pipeline_in05_test.go` regression test confirms server survival and stage reset.
- **IN-09 (bounded-impact documentation):** `abox.go:136-165` godoc correctly documents the non-transactional multi-step ApplyNote and the bounded impact contract. No code change required per plan.

---

_Reviewed: 2026-09-24T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
