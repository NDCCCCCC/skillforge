# Phase 27 Plan 06: Engineering Hygiene + Verification Gate — COMPLETE

**Plan:** 27-06
**Phase:** 27 (Document Upload Pipeline)
**Tasks:** 3/3 complete
**Status:** COMPLETE
**Commits:** `4cbd3d1` (T1), `8d69381` (T2), `cd56abc` (T3)
**Completed:** 2026-09-07

---

## Executive Summary

Engineering hygiene: verify script, Makefile target, STRIDE threat model, and the Phase 27 LIVE acceptance runbook. This is the structural gate that ensures Phase 27 ships with the same hygiene standards as every prior phase.

---

## Task Results

### Task 1: scripts/verify-phase-27.sh — COMPLETE

**Commit:** `4cbd3d1`
**File:** `scripts/verify-phase-27.sh` (102 lines)

Three-stage hermetic gate:

| Stage | Check | Result |
|-------|-------|--------|
| 1 | TODO/FIXME zero in Phase 27 production files | PASS |
| 2a | `BodyStream`/`multipart.NewReader` in upload.go (streaming multipart) | PASS |
| 2b | No `os.WriteFile`/`os.Create`/`ioutil.WriteFile` in upload.go | PASS |
| 2c | `VerifyMagicBytes` function declared in upload.go | PASS |
| 2d | `VerifyMagicBytes` body contains no disk write patterns | PASS |
| 3a | `validateTiers` called in upload_index.go (KNOW-02 fail-closed gate) | PASS |
| 3b | `kb.Citable` NOT called in upload_index.go (trust invariant: query-time gate only) | PASS |

**No Go toolchain required** — all grep-only, hermetic.

---

### Task 2: Makefile target + STRIDE-27.md — COMPLETE

**Commit:** `8d69381`
**Files:** `Makefile` (+15 lines), `docs/threat-model/STRIDE-27.md` (150 lines)

**Makefile changes:**
- Added `phase-27` to `.PHONY`
- New `phase-27` target runs `scripts/verify-phase-27.sh`

**STRIDE-27.md** (adapted from STRIDE-26.md structure):

| Boundary | Description |
|----------|-------------|
| client → upload endpoint | Untrusted file bytes (multipart) |
| upload handler → parser | Parsed text content |
| parser → embedder | Trusted in-memory content |
| embedder → IndexerBackend | Trusted vectors |
| IndexerBackend → Milvus/Mongo | Trusted vectors + metadata |
| upload → Store (audit) | INSERT-only row |

**10 threats registered:** T-27-01 through T-27-09 + T-27-SC

---

### Task 3: README.md + Operator Runbook — COMPLETE

**Commit:** `cd56abc`
**Files:** `README.md` (+15 lines), `.planning/phases/27-document-upload-pipeline/27-UAT.md` (70 lines)

**README.md:** Added "Document Upload (Phase 27)" section describing formats, size limit, flow, trust properties, runbook link, and gate command.

**27-UAT.md:** 7-step LIVE acceptance runbook (operator checkpoint, human-verified, not executor-executed):
1. Upload .md via curl — expect HTTP 200 + `{"chunks":1,"id_range":[N,N]}`
2. Verify audit row in sqlite3
3. Upload without token — expect HTTP 403
4. Upload to deactivated slug — expect HTTP 409
6. Upload via FE — expect green toast "1 个片段已入库"
7. Verify chunks searchable via `/api/chat`

---

## Files Added/Modified

| Path | Lines | Delta | Description |
|------|-------|-------|-------------|
| `scripts/verify-phase-27.sh` | 102 | +102 | 3-stage hermetic structural gate |
| `Makefile` | ~107 | +15 | `phase-27` target + PHONY entry |
| `docs/threat-model/STRIDE-27.md` | 150 | +150 | Phase 27 threat model (10 threats) |
| `README.md` | ~295 | +15 | "Document Upload (Phase 27)" section |
| `.planning/phases/27-document-upload-pipeline/27-UAT.md` | 70 | +70 | Operator LIVE acceptance runbook |

---

## Verification

**Hermetic gate output:**
```
=== Stage 1: TODO/FIXME clear ===
PASS: no TODO/FIXME in Phase 27 production files

=== Stage 2: No-on-disk invariant ===
--- Stage 2a: streaming multipart (parseUploadForm uses BodyStream/multipart.NewReader) ---
PASS 2a: streaming multipart
--- Stage 2b: no disk writes in upload.go (excluding _test.go) ---
PASS 2b: no disk writes
--- Stage 2c: VerifyMagicBytes function declared in upload.go ---
PASS 2c: VerifyMagicBytes declared
--- Stage 2d: VerifyMagicBytes body is pure in-memory (no disk writes in the function body) ---
PASS 2d: VerifyMagicBytes body is pure

=== Stage 3: Trust invariant ===
--- Stage 3a: validateTiers called in upload_index.go ---
PASS 3a: validateTiers gate present
--- Stage 3b: kb.Citable NOT called in upload_index.go (it's a query-time gate, not insert-time) ---
PASS 3b: kb.Citable not called at insert time

=== Phase 27 verify PASS ===
```

**Last 10 lines of `bash scripts/verify-phase-27.sh`:**
```
PASS 3a: validateTiers gate present
--- Stage 3b: kb.Citable NOT called in upload_index.go (it's a query-time gate, not insert-time) ---
PASS 3b: kb.Citable not called at insert time

=== Phase 27 verify PASS ===
```

---

## Commits

```
cd56abc feat(27-06-T3): add Phase 27 section to README + operator runbook 27-UAT.md
8d69381 feat(27-06-T2): add Makefile phase-27 target + STRIDE-27.md threat model
4cbd3d1 feat(27-06-T1): add scripts/verify-phase-27.sh structural gate
```

All commits properly attributed with `Co-Authored-By: Claude <noreply@anthropic.com>`.

---

## Deviations from Plan

None — plan executed exactly as written.

---

## Self-Check

- [x] `scripts/verify-phase-27.sh` — 3 stages, all 4 Stage 2 sub-stages present, executable
- [x] Stage 2d uses `awk '/^func VerifyMagicBytes/,/^}/'` to isolate function body (iteration-1 closure)
- [x] `Makefile` — `phase-27` target added, PHONY updated, no existing targets modified
- [x] `STRIDE-27.md` — mirrors STRIDE-26.md structure, 6 trust boundaries, 10 threats, mitigation summary
- [x] `README.md` — Phase 27 section added after Knowledge Bases, before Eval Results
- [x] `27-UAT.md` — 7-step operator runbook written (human-verify checkpoint, NOT executed)
- [x] No TODO/FIXME in any new file
- [x] All 3 tasks committed individually with correct prefix `27-06-TN`
- [x] All commits properly attributed

**Self-Check: PASSED**
