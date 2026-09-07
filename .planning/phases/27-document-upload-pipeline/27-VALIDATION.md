# Phase 27 Validation (post-fix)

**Phase:** 27 — 文档上传入库 (Document Upload Pipeline)
**Re-checked:** 2026-09-07 (after iteration-1 fix round)
**Status:** PASS — ready for execute-phase

Iteration-1 changes applied (all 4 blockers + 4 warnings closed):

| Issue | Resolution |
|-------|------------|
| BLOCKER-1 (27-01 T1 CGO probe result unowned) | T1 done-criteria now mandates CGO status per parser documented in `internal/document/parser.go` package godoc AND in STRIDE-27.md T-27-SC row. |
| BLOCKER-2 (GetMaxID on retriever.Retriever unclaimed) | 27-02 T2 declares `GetMaxID` on the interface (`internal/retriever/retriever.go`); 27-04 owns the concrete impls. Interface split is explicit and unambiguous. |
| BLOCKER-3 (MilvusClient.GetMaxID + MongoClient.GetMaxID unowned) | 27-04 now lists `internal/retriever/milvus/client.go` and `internal/retriever/mongo/client.go` in files_modified; added 27-04-T5/T6 with full implementations + 3 hermetic tests each (empty / populated / unknown-tier). |
| BLOCKER-4 (HasUploadBySHA256 on Store interface + SQLite impl unowned) | 27-07 now lists `internal/store/store.go` and `internal/store/sqlite/audit.go` in files_modified; T2 explicitly implements SQLiteStore.HasUploadBySHA256 using json_extract. |
| WARNING-1 (27-03 omitted server.go) | 27-03 files_modified now lists `internal/server/server.go` (Deps struct lives there, not deps.go). T2 file list corrected. |
| WARNING-2 (27-06 Stage 2 grep missed VerifyMagicBytes) | Stage 2 split into 2a (streaming) + 2b (no disk writes) + 2c (VerifyMagicBytes declared) + 2d (VerifyMagicBytes body is pure in-memory). |
| WARNING-3 (27-05 App.tsx insertion point unspecified) | T2 corrected: insertion point is `AdminDashboard.tsx` line 160 (not App.tsx); concrete edit shown (import + JSX fragment). |
| WARNING-4 (D27-01-1 = D27-03-1 duplicate) | D27-03-1 now references D27-01-1 ("See D27-01-1 (decision deduplicated)"). |

## Plan Inventory (post-fix)

| Plan | Goals claimed | Files in plan | All files exist (verified by Bash `ls`) | Status |
|------|---------------|---------------|----------------------------------------|--------|
| 27-01 | DOC-01/02/03 | parser.go, parser_test.go, go.mod, go.sum | TO BE CREATED (executor) | OK |
| 27-02 | DOC-02/03/05 | upload_index.go, upload_index_test.go, retriever.go | retriever.go exists; others TO BE CREATED | OK |
| 27-03 | DOC-01/03/04/05 | upload.go, upload_test.go, server.go, main.go | server.go + main.go exist; others TO BE CREATED | OK |
| 27-04 | DOC-02/05 | backend.go + milvus/{backend,client}.go + mongo/{backend,client}.go + main.go | All client files exist; new backend.go files TO BE CREATED | OK |
| 27-05 | DOC-01/06 | DocUpload.tsx + test + AdminDashboard.tsx | AdminDashboard.tsx exists; DocUpload TO BE CREATED | OK |
| 27-06 | DOC-03/04/05 | verify-phase-27.sh + Makefile + STRIDE-27.md + README.md | Makefile + README.md exist; others TO BE CREATED | OK |
| 27-07 | DOC-03/04 | upload.go + upload_test.go + store/store.go + sqlite/audit.go | store.go + sqlite/audit.go exist; upload files TO BE CREATED | OK |

## Coverage Matrix (post-fix)

| Requirement | Plans | Test path | Status |
|-------------|-------|-----------|--------|
| DOC-01: Upload UI + endpoint | 27-01, 27-03, 27-05 | vitest TestUploadHappyPath + go TestUploadEndpointStreaming | ✓ |
| DOC-02: Parse → chunk → embed → index | 27-01, 27-02, 27-04 | TestParserRegistry + TestUploadIndexProducesValidDocuments + TestMilvusIndexerBackend | ✓ |
| DOC-03: No disk, type/size guards | 27-01, 27-03, 27-06, 27-07 | verify-phase-27.sh Stage 2a/2b/2c/2d + TestUploadEndpointBodySizeGuard + TestUploadMagicByteMismatch | ✓ |
| DOC-04: Configurable limits + dedup | 27-03, 27-06, 27-07 | SKILLFORGE_UPLOAD_MAX_BYTES env + TestUploadDeduplication + SQLiteStore.HasUploadBySHA256 | ✓ |
| DOC-05: INSERT-only audit row | 27-02, 27-03, 27-06, 27-07 | TestUploadEndpointAuditRowOnSuccess + TestUploadEndpointAuditRowOnRejected + PutAudit INSERT-only | ✓ |
| DOC-06: Immediately visible | 27-02, 27-04, 27-05 | live UAT ⑦ (chat query → citation) | ✓ |

## Breaking-Change Call Site Registry (post-fix)

| Call site | Type | Location | Owned by | Status |
|-----------|------|----------|----------|--------|
| GetMaxID on retriever.Retriever | ADD to interface | `internal/retriever/retriever.go` | 27-02 T2 | ✓ OWNED |
| MilvusClient.GetMaxID | ADD method | `internal/retriever/milvus/client.go` | 27-04 T5 | ✓ OWNED |
| MongoClient.GetMaxID | ADD method | `internal/retriever/mongo/client.go` | 27-04 T6 | ✓ OWNED |
| VerifyMagicBytes function | ADD function | `internal/server/upload.go` | 27-03 T1 | ✓ OWNED |
| HasUploadBySHA256 on Store interface | ADD to interface | `internal/store/store.go` | 27-07 T2 | ✓ OWNED |
| HasUploadBySHA256 in SQLite implementation | ADD method | `internal/store/sqlite/audit.go` | 27-07 T2 | ✓ OWNED |
| SetDocUploadAuditHook in main.go | ADD boot wiring | `cmd/server/main.go` | 27-03 T2 | ✓ OWNED |
| SetUploadAuditHook in main.go | ADD boot wiring | `cmd/server/main.go` | 27-02 T1 | ✓ OWNED |
| IndexerBackend wiring in main.go | ADD wiring | `cmd/server/main.go` | 27-04 T4 | ✓ OWNED |
| ParserRegistry wiring in main.go | ADD wiring | `cmd/server/main.go` | 27-03 T2 | ✓ OWNED |
| UploadMaxBodyBytes / IndexerBackend / ParserRegistry on Deps | ADD struct fields | `internal/server/server.go` | 27-03 T2 | ✓ OWNED |

All 11 call sites owned — the same structural risk that Phase 26 had to fix
post-hoc (5 unowned sites, dbd7f80) is closed at planning time here.

## Decision Inventory (post-fix)

| Decision | Answer | Owner |
|----------|--------|-------|
| D27-01-1 | SKILLFORGE_UPLOAD_MAX_BYTES env var, default 100 MB | 27-01 |
| D27-01-2 | bytes.Buffer in memory (100 MB bounded) | 27-01 |
| D27-01-3 | Try eino-ext first; pure-Go fallback if CGO surfaces | 27-01 |
| D27-02-1 | Deactivated slug → 409 (fail-closed via validateTiers) | 27-02 |
| D27-03-1 | (refers to D27-01-1 — deduplicated) | 27-01 |
| D27-03-2 | Per-slug sync.Map mutex serialization | 27-03 |

All decisions consistent between research and plans; no duplicates.

## Nyquist Validation Rows

All 23 tasks (27-01:3 + 27-02:3 + 27-03:3 + 27-04:6 + 27-05:3 + 27-06:3 + 27-07:3)
have automated `<verify>` commands. Wave 3 has 2 automated + 1 human checkpoint.

| Task | Plan | Wave | Automated? | Status |
|------|------|------|-----------|--------|
| 27-01 T1 | 27-01 | 1 | go list -m + go build | OK |
| 27-01 T2 | 27-01 | 1 | go test -run TestParserRegistry | OK |
| 27-01 T3 | 27-01 | 1 | go test -v | OK |
| 27-02 T1 | 27-02 | 1 | go build | OK |
| 27-02 T2 | 27-02 | 1 | go build retriever + knowledge | OK |
| 27-02 T3 | 27-02 | 1 | go test -run TestUploadIndex | OK |
| 27-03 T1 | 27-03 | 1 | go build server | OK |
| 27-03 T2 | 27-03 | 1 | grep IndexerBackend main.go + go build cmd/server | OK |
| 27-03 T3 | 27-03 | 1 | go test -run TestUpload | OK |
| 27-04 T1 | 27-04 | 1 | go build retriever | OK |
| 27-04 T2 | 27-04 | 1 | go build retriever/milvus | OK |
| 27-04 T3 | 27-04 | 1 | go build retriever/mongo | OK |
| 27-04 T4 | 27-04 | 1 | go build cmd/server | OK |
| 27-04 T5 | 27-04 | 1 | go test milvus -run TestMilvusClientGetMaxID | OK |
| 27-04 T6 | 27-04 | 1 | go test mongo -run TestMongoClientGetMaxID | OK |
| 27-05 T1 | 27-05 | 2 | npm run build | OK |
| 27-05 T2 | 27-05 | 2 | npm run build | OK |
| 27-05 T3 | 27-05 | 2 | npm test --filter=DocUpload | OK |
| 27-06 T1 | 27-06 | 3 | bash verify-phase-27.sh | OK |
| 27-06 T2 | 27-06 | 3 | grep -c phase-27 Makefile | OK |
| 27-06 T3 | 27-06 | 3 | Human sign-off | CHECKPOINT |
| 27-07 T1 | 27-07 | 2 | go build server | OK |
| 27-07 T2 | 27-07 | 2 | go build store | OK |
| 27-07 T3 | 27-07 | 2 | go test -run TestUploadDegraded | OK |

PASS — all waves have ≥2/3 automated verify tasks.

## Verdict

**PASS — ready for execute-phase.**

Iteration-1 fixes close all 4 blockers + 4 warnings. The 11 breaking-change
call sites that Phase 26 had to repair post-execution are owned at planning
time here. All 6 requirements (DOC-01..06) have a test path through at
least one plan with executable tasks and concrete file paths.

Recommendations for the executor:
1. Wave 1 (`27-01`, `27-02`, `27-03`, `27-04`) can run in parallel — no file
   conflicts after the 27-04 files_modified update.
2. Wave 2 (`27-05`, `27-07`) runs after Wave 1.
3. Wave 3 (`27-06`) is the hygiene gate + operator checkpoint; do NOT skip
   the human sign-off — the 7-step runbook catches what hermetic tests miss.
4. If 27-01 T1 surfaces CGO for any eino-ext parser, the plan's done-criteria
   mandate a comment + STRIDE-27 row update before moving to Wave 2.
