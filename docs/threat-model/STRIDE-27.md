# Phase 27 — Threat Model (STRIDE-27)

**Phase:** 27 — 文档上传入库 (Document Upload Pipeline)
**Authored:** 2026-09-07 (Plan 27-06)
**Status:** active — locks the trust surface for Phase 27

---

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| client → upload endpoint | Untrusted file bytes cross here (multipart) |
| upload handler → parser | Parsed content crosses here (text body) |
| parser → embedder | Trusted content (in-memory) |
| embedder → IndexerBackend | Trusted vectors (no zero-vector fallback) |
| IndexerBackend → Milvus/Mongo | Trusted vectors + metadata |
| upload → Store (audit) | INSERT-only row written |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-27-01 | Tampering | upload endpoint | mitigate | Admin token gate (isAdminRequest); Content-Length guard (413); magic byte verification (400) |
| T-27-02 | Information Disclosure | upload bytes | mitigate | No disk write (bytes.Buffer in memory); SHA-256 hash in audit row; verify-phase-27 Stage 2b locks this |
| T-27-03 | Denial of Service | upload endpoint | mitigate | SKILLFORGE_UPLOAD_MAX_BYTES (100 MB default); per-slug mutex (D27-03-2) serializes concurrent uploads to the same slug |
| T-27-04 | Tampering | audit row | mitigate | INSERT-only pattern (PutAudit); frozen args carry immutable upload metadata (sha256, file_size, id_range, parser_version) |
| T-27-05 | Repudiation | upload action | mitigate | Admin token + audit row with action=action_doc_uploaded or action_doc_upload_rejected |
| T-27-06 | Elevation of Privilege | deactivated KB upload | mitigate | validateSlug checks ActiveSlugs() (D27-02-1); re-check after UploadIndex returns 409 (deactivated mid-upload) |
| T-27-07 | Tampering | upload replay | mitigate | SHA-256 deduplication via Store.HasUploadBySHA256 → 409 on duplicate |
| T-27-08 | DoS | backend outage | mitigate | IndexerBackend unavailable → 503 + Retry-After: 30 (not 500 — caller retries) |
| T-27-09 | Tampering | nextID collision | mitigate | UploadIndex calls Retriever.GetMaxID to find current ceiling; chunks assigned sequential IDs starting from max+1 |
| T-27-SC | Supply Chain | eino-ext parser packages | mitigate | CGO status per parser recorded in internal/document/parser.go godoc; pure-Go fallbacks for missing modules |

## Mitigation Summary

- **T-27-01**: Admin token required (403 before 503); Content-Length 413 before parse; magic byte verify 400 before UploadIndex.
- **T-27-02**: bytes.Buffer only; SHA-256 in audit row; verify-phase-27 Stage 2 grep + VerifyMagicBytes no-disk verification.
- **T-27-03**: SKILLFORGE_UPLOAD_MAX_BYTES configurable; per-slug sync.Map mutex.
- **T-27-04/05/07**: INSERT-only audit rows; one row per request (success → action_doc_uploaded; rejection → action_doc_upload_rejected); dedup via SHA-256.
- **T-27-06**: validateSlug fail-closed before any I/O; re-check post-UploadIndex.
- **T-27-08**: 503 + Retry-After: 30 on backend unavailable (not 500).
- **T-27-09**: GetMaxID coordination across uploads.
- **T-27-SC**: CGO status recorded per parser.

---

_See also:_ `docs/threat-model/STRIDE-26.md` (Phase 26 registry threats), `verify-phase-27.sh` (structural gate), `27-RESEARCH.md` §Trust-contract surface.
