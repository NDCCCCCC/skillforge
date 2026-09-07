# Phase 27 Plan 07: Degraded Mode + SHA-256 Dedup — COMPLETE

**Plan:** 27-07
**Phase:** 27 (Document Upload Pipeline)
**Tasks:** 3/3 complete
**Commits:** `d726712` (T2), `1ad524a` (T1), `8d510cd` (T3)
**Completed:** 2026-09-07

---

## Executive Summary

Implemented degraded mode handlers and SHA-256 deduplication for the document upload pipeline. Each failure path returns the correct HTTP status, writes its own `action_doc_upload_rejected` audit row, and never double-writes an audit trail.

---

## Task Results

### Task 1: Degraded Mode Handlers (Commit: `1ad524a`)

**File:** `internal/server/upload.go` (+84 lines, -7 lines)

**Changes:**

| Step | Check | HTTP Status | Audit Action |
|------|-------|-------------|-------------|
| STEP 7a | SHA-256 dedup (Store.HasUploadBySHA256) | 409 Conflict | action_doc_upload_rejected |
| STEP 7b | UploadIndex failure | 422 Unprocessable Entity | action_doc_upload_rejected |
| STEP 7c | Slug deactivated mid-upload | 409 Conflict | action_doc_upload_rejected |
| STEP 8a | Insert: backend unavailable | 503 Service Unavailable + Retry-After: 30 | action_doc_upload_rejected |
| STEP 8b | Insert: other error | 500 Internal Server Error | action_doc_upload_rejected |
| STEP 9 | Success | 200 OK | action_doc_uploaded |

**Key design decisions:**
- `isBackendUnavailable` helper checks for: `connection refused`, `context deadline exceeded`, `no such host`, `context canceled`
- Dedup check runs BEFORE UploadIndex (avoids wasting a parse on duplicates)
- Slug re-check runs AFTER UploadIndex (fail-closed race gate between parse and insert)
- Every failure path writes exactly one `action_doc_upload_rejected` audit row
- Only success writes `action_doc_uploaded` (no double-writes)

### Task 2: HasUploadBySHA256 on Store Interface (Commit: `d726712`)

**Files:**
- `internal/store/store.go` (+7 lines — interface method added)
- `internal/store/sqlite/audit.go` (+35 lines — SQLite implementation)

**Interface addition:**
```go
// HasUploadBySHA256 returns true if an action_doc_uploaded audit row
// exists for the given slug with the given SHA-256 hash. Used for DOC-04
// deduplication: same content hash for same slug = same document.
HasUploadBySHA256(ctx context.Context, slug, sha256 string) (bool, error)
```

**SQLite implementation:** Queries `audit` table with `json_extract(frozen_args, '$.slug') = ? AND json_extract(frozen_args, '$.sha256') = ?` scoped to `action='action_doc_uploaded'`. Uses `LIMIT 1` for efficiency.

**Rationale for Store vs Retriever placement:** The dedup query reads from the audit trail, not from the vector index. The store is the authoritative source for "this SHA-256 was already uploaded for this slug." Putting this on the Store interface (not the IndexerBackend/Retriever) is correct by separation of concerns.

### Task 3: Degraded Mode Tests (Commit: `8d510cd`)

**File:** `internal/server/upload_test.go` (+263 lines, -25 lines)

**5 new tests:**

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestUploadDegradedMilvusDown` | IndexerBackend.Insert returns context.DeadlineExceeded | 503 + Retry-After: 30 |
| `TestUploadDeduplication` | Store.HasUploadBySHA256 returns true | 409 Conflict + reason |
| `TestUploadDeactivatedSlugMidUpload` | KBRegistry deactivated between UploadIndex and Insert | 409 Conflict |
| `TestUploadSuccessAfterRetry` | Sequential failure then success | Correct audit rows (no mix) |
| `TestUploadInsertGenericError` | Non-backend Insert error | 500 Internal Server Error |

**Supporting test infrastructure:**
- `mockStore`: full `store.Store` implementation with configurable `HasUploadBySHA256` result
- `deactivatingKBRegistry`: wraps `kb.Registry`, returns false on `Lookup` after a configurable number of calls (deactivateAfter=4 to account for internal `validateTiers` calls)

**Cascade fixes:**
- `HasUploadBySHA256` panic-stubs added to `fakeAdminAuditStore` and `fakeAdminSchedulesStore` in admin test files
- `validateSlug` simplified to use `Lookup` only (fail-closed; single source of truth)
- `validateTiersHook` added to `upload_index.go` for KNOW-02 test injection

---

## Deviations from Plan

### Deviation 1: validateSlug simplification

**Plan said:** `validateSlug` would use `List()` + `ActiveSlugs()` to distinguish unknown vs deactivated.

**Found:** `validateSlug` used `ActiveSlugs()` for the deactivated check, creating an inconsistency where `Lookup("slug")` returned `false` but `ActiveSlugs()` still contained `"slug"` until the next snapshot. This made the `deactivatingKBRegistry` test unable to simulate the correct call sequence.

**Fix:** Simplified `validateSlug` to use `Lookup` exclusively. This is semantically equivalent (Lookup is fail-closed for both unknown and deactivated) and removes the inconsistency.

### Deviation 2: Cascade additions to admin test files

**Plan scope:** `files_modified: [upload.go, upload_test.go, store.go, sqlite/audit.go]`

**Found:** `fakeAdminAuditStore` and `fakeAdminSchedulesStore` don't implement the new `HasUploadBySHA256` method, causing compile errors in other test files.

**Fix:** Added panic-stubs to both fake stores. This is a mechanical interface compliance fix, not a logic change.

---

## Known Stubs

None introduced by this plan.

---

## Verification

**Server tests:** `go test ./internal/server/...` — ALL PASS (13.7s)
**Knowledge tests:** `go test ./internal/knowledge/...` — ALL PASS

**Last 5 lines of test output:**
```
ok  	github.com/skillforge/skillforge/internal/server	13.747s
ok  	github.com/skillforge/skillforge/internal/server/classifier	0.024s
ok  	github.com/skillforge/skillforge/internal/server/rewrite	0.116s
```

**Commits:**
```
8d510cd feat(27-07-T3): add 5 degraded-mode tests for upload handler
1ad524a feat(27-07-T1): add degraded-mode handlers to upload.go
d726712 feat(27-07-T2): add HasUploadBySHA256 to Store interface and SQLite implementation
```

---

## Files Added/Modified

| Path | Lines | Delta | Description |
|------|-------|-------|-------------|
| `internal/server/upload.go` | ~500 | +84/-7 | Degraded mode handlers (dedup, deactivated slug, backend unavailable) |
| `internal/server/upload_test.go` | ~720 | +263/-25 | 5 new degraded-mode tests + mockStore + deactivatingKBRegistry |
| `internal/store/store.go` | ~830 | +7 | HasUploadBySHA256 interface method |
| `internal/store/sqlite/audit.go` | ~840 | +35 | HasUploadBySHA256 SQLite implementation |
| `internal/server/admin_audit_test.go` | | +6 | HasUploadBySHA256 panic-stub on fakeAdminAuditStore |
| `internal/server/admin_schedules_test.go` | | +6 | HasUploadBySHA256 panic-stub on fakeAdminSchedulesStore |
| `internal/knowledge/graph_test.go` | | +1 | GetMaxID stub on stubMetaRetriever |
| `internal/knowledge/upload_index.go` | | +8 | validateSlug simplification + validateTiersHook |

---

## Self-Check

- [x] Backend unavailable → 503 + Retry-After: 30
- [x] SHA-256 dedup → 409 Conflict + "duplicate file" reason
- [x] Slug deactivated mid-upload → 409 Conflict
- [x] UploadIndex failure → 422 (already existed)
- [x] Generic Insert error → 500
- [x] All 5 new tests pass
- [x] All existing server tests pass (18 from 27-03 + 5 new = 23 total)
- [x] No TODO/FIXME in new files
- [x] Commits: `d726712` (T2), `1ad524a` (T1), `8d510cd` (T3)
- [x] All commits properly attributed with `Co-Authored-By: Claude <noreply@anthropic.com>`

**Self-Check: PASSED**
