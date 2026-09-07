# STRIDE-27 — Phase 27 (文档上传入库 — Document Upload Pipeline) Threat Model

**Phase:** 27 (Parser abstraction + upload handler + dual-backend indexing)
**Version:** 1.0
**Last updated:** 2026-09-07
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Draft — Wave 1 (parser registry) executed. T-27-SC resolved.

---

## 1. Scope & Trust Boundaries

Phase 27 introduces a document upload pipeline: `POST /api/admin/kb/:slug/upload`
behind `X-Admin-Token`. The upload handler receives a multipart stream, routes to a
format-specific parser (md/pdf/docx/xlsx/pptx), and passes the resulting plain text
to the Phase 26 indexer for chunking and dual-backend storage (Milvus + MongoDB).

Trust boundary crossings introduced by Wave 1 (parser registry):

| Boundary | Description |
|----------|-------------|
| `ParserRegistry.Parse` (io.Reader, MIME type) | Handler trusts the declared Content-Type header; parser verifies magic bytes at the format boundary before parsing. |
| `io.Reader` over in-memory buffer | Upload bytes are held in a `bytes.Buffer` (max 100 MB); no temp files. SHA-256 is computed on the buffer. |
| `ParserRegistry` allowlist | Only 5 MIME types are accepted; unknown types return `ErrUnknownFormat` before any content is consumed. |

---

## 2. STRIDE Register — Phase 27

### T-27-SC: Supply Chain / CGO Constraint

| Field | Value |
|-------|-------|
| **Category** | Supply Chain / Information Disclosure |
| **Component** | `internal/document/parser.go` |
| **Disposition** | Mitigated |
| **Mitigation** | All five parser libraries are pure-Go (CGO-free). CGO status resolved at plan execution: |

**CGO status table** (as of `eino-ext` pseudo-version `v0.0.0-20260904094655-8d8ff4e79992` and `xuri/excelize/v2@v2.11.0`):

| Format | Library | CGO | Notes |
|--------|---------|-----|-------|
| md | `goldmark@v1.8.5` | NO | Pure-Go; no `//go:build cgo` or `C` import |
| pdf | `eino-ext/pdf@ledongthuc/pdf@v0.0.0` | NO | `ledongthuc/pdf` is pure-Go; no `C` import |
| docx | `pure-go/docx-zip-walker@v0.0.0` | NO | Custom ZIP+XML walker; no external library |
| xlsx | `xuri/excelize/v2@v2.11.0` | NO | Pure-Go; zip-based Office format reader |
| pptx | `pure-go/pptx-zip-walker@v0.0.0` | NO | Custom ZIP+XML walker; no external library |

**Verification:** `go list -f '{{.BuildConstraints}}'` on each module returns empty (no CGO constraint).
eino-ext has **no pptx parser module** in v0.1.13 or pseudo-version; pptx uses a direct ZIP/XML walker.

**Impact:** Upload pipeline is portable to any host with a Go toolchain (Linux/Windows/macOS, with or without CGO). No WSL-only constraint required.

### T-27-CTM: Content Type Mismatch (Magic Byte Spoofing)

| Field | Value |
|-------|-------|
| **Category** | Tampering / Information Disclosure |
| **Component** | `internal/document/parser.go:checkMagicBytes` |
| **Disposition** | Mitigated |
| **Mitigation** | Every binary format parser (pdf/docx/xlsx/pptx) checks magic bytes before parsing. `checkMagicBytes(buf, expected, mimeType)` returns `ErrContentTypeMismatch` if the buffer's first bytes do not match the format's magic signature. The upload handler never sees content on mismatch. |
| **Verification Gate** | `TestParserMagicByteMismatch` — ZIP-header buffer parsed as PDF → `ErrContentTypeMismatch` |

### T-27-OOB: Out-of-Bounds Memory (Upload Size)

| Field | Value |
|-------|-------|
| **Category** | Denial of Service |
| **Component** | Upload handler (future Wave 2) |
| **Disposition** | Mitigated in design |
| **Mitigation** | `bytes.Buffer` is bounded to 100 MB (configurable via `SKILLFORGE_UPLOAD_MAX_BYTES`). The parser receives `io.Reader` over the buffer, not a temp file. Parsing a 100 MB file in memory is acceptable for the document domain. |
| **Verification Gate** | Wave 2 tests will verify 100 MB buffer limit; `SKILLFORGE_UPLOAD_MAX_BYTES=104857600` is the default. |

### T-27-URI: URI-Based Resource Access (Slug Traversal)

| Field | Value |
|-------|-------|
| **Category** | Information Disclosure / Tampering |
| **Component** | Upload handler route `POST /api/admin/kb/:slug/upload` |
| **Disposition** | Mitigated by Phase 26 slug validation |
| **Mitigation** | The slug is validated by `kb.ValidateSlug` (Phase 26 T-26-02) before the upload handler processes it. Upload targets an active registry entry only. |
| **Verification Gate** | Phase 26 slug validation tests (`TestValidateSlug` in `internal/kb/`). |

---

## 3. Deferred Threats (Future Waves)

| Threat ID | Description | Phase |
|-----------|-------------|-------|
| T-27-Audit | Upload audit row missing (DOC-05: every upload → INSERT-only audit row) | Wave 2 |
| T-27-Dedup | Content hash deduplication (DOC-04: SHA-256 based) | Wave 2 |
| T-27-Graph | Uploaded document not visible in knowledge graph (GRF-01 node + GRF-02 edge) | Wave 3 |
| T-27-Milvus | Upload pipeline couples to Milvus-only indexer (RETRIEVER_BACKEND=mongo incompatibility) | Wave 4 |

---

## 4. CGO Verification Commands

```bash
# Verify all parser modules are CGO-free (empty = pure-Go)
go list -f '{{.BuildConstraints}}' github.com/cloudwego/eino-ext/components/document/parser/pdf
go list -f '{{.BuildConstraints}}' github.com/cloudwego/eino-ext/components/document/parser/docx
go list -f '{{.BuildConstraints}}' github.com/xuri/excelize/v2
go list -f '{{.BuildConstraints}}' github.com/ledongthuc/pdf

# Expected: empty output (no CGO constraint)
```
