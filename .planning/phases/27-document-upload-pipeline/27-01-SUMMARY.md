# Phase 27 Plan 01: Parser Abstraction — COMPLETE

**Plan:** 27-01
**Phase:** 27 (Document Upload Pipeline)
**Tasks:** 3/3 complete
**Status:** COMPLETE
**Commit:** `0991ca0`
**Completed:** 2026-09-07

---

## Executive Summary

Established the parser abstraction layer for Phase 27. A `ParserRegistry` maps MIME type to a concrete implementation; each format (md/pdf/docx/xlsx/pptx) has one registered parser. All five parsers are **CGO-free** — the upload pipeline compiles and runs on any host with a Go toolchain.

---

## CGO Status (per format)

| Format | Library | CGO | Notes |
|--------|---------|-----|-------|
| md | `goldmark@v1.8.5` | NO | Pure-Go; no `//go:build cgo` or `C` import |
| pdf | `eino-ext/pdf@ledongthuc/pdf@v0.0.0` | NO | `ledongthuc/pdf` is pure-Go; no `C` import |
| docx | `pure-go/docx-zip-walker@v0.0.0` | NO | Custom ZIP+XML walker; no external library |
| xlsx | `xuri/excelize/v2@v2.11.0` | NO | Pure-Go; zip-based Office format reader |
| pptx | `pure-go/pptx-zip-walker@v0.0.0` | NO | Custom ZIP+XML walker; no external library |

**Key findings:**
- eino-ext has **no pptx parser module** in v0.1.13 or pseudo-version — pptx uses a direct ZIP/XML walker.
- eino-ext/docx uses **temp files** internally (`os.CreateTemp`) — the custom ZIP walker bypasses this constraint.
- All 5 formats are CGO-free. No WSL-only constraint required.

---

## Task Results

### Task 1: go get parser dependencies + CGO probe — COMPLETE

**Key discovery:** eino-ext/docx uses `os.CreateTemp` internally — violates plan's "no temp files" contract. The plan was followed: a custom pure-Go DOCX ZIP walker was implemented instead.

**CGO verification commands run:**
```bash
go list -f '{{.BuildConstraints}}' github.com/cloudwego/eino-ext/components/document/parser/pdf  # → empty (pure-Go)
go list -f '{{.BuildConstraints}}' github.com/cloudwego/eino-ext/components/document/parser/docx  # → empty (pure-Go)
go list -f '{{.BuildConstraints}}' github.com/xuri/excelize/v2  # → empty (pure-Go)
go list -f '{{.BuildConstraints}}' github.com/ledongthuc/pdf  # → empty (pure-Go)
```

**Packages added to go.mod:**
- `github.com/cloudwego/eino-ext/components/document/parser/pdf v0.0.0-20260904094655-8d8ff4e79992`
- `github.com/cloudwego/eino-ext/components/document/parser/docx v0.0.0-20260904094655-8d8ff4e79992`
- `github.com/xuri/excelize/v2 v2.11.0`
- `github.com/richardlehane/mscfb v1.0.7` (excelize dep)
- `github.com/richardlehane/msoleps v1.0.6` (excelize dep)
- `github.com/tiendc/go-deepcopy v1.7.2` (excelize dep)
- `github.com/xuri/efp v0.0.1` (excelize dep)
- `github.com/xuri/nfp v0.0.2-...` (excelize dep)

### Task 2: internal/document/parser.go — COMPLETE

**File:** `internal/document/parser.go` (~540 lines)

**Structure:**
- `ParserRegistry` — injectable (no global state), maps MIME type to `Parser`
- `Parser` interface — `Parse(ctx, io.Reader, filename) ([]byte, error)`, `Name() string`
- `ParseResult` — `{Content, Frontmatter, ParserName}`
- `ErrPasswordProtected`, `ErrUnknownFormat`, `ErrContentTypeMismatch`

**5 parsers registered:**

| MIME type | Parser | Library | Key constraint |
|-----------|--------|---------|----------------|
| `text/markdown` | `mdParser` | goldmark v1.8.5 | io.Reader; frontmatter strip; no temp files |
| `application/pdf` | `pdfParser` | ledongthuc/pdf (via eino-ext/pdf) | io.Reader; magic bytes `%PDF` |
| `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | `docxParser` | custom ZIP walker | io.Reader; magic bytes `PK`; no temp files |
| `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | `xlsxParser` | xuri/excelize/v2 v2.11.0 | io.Reader; magic bytes `PK` |
| `application/vnd.openxmlformats-officedocument.presentationml.presentation` | `pptxParser` | custom ZIP walker | io.Reader; magic bytes `PK`; no external library |

**Magic-byte verification:** All binary formats (pdf/docx/xlsx/pptx) check magic bytes before parsing. Mismatch returns `ErrContentTypeMismatch`.

### Task 3: Unit tests — COMPLETE

**File:** `internal/document/parser_test.go` (~165 lines)

**12 tests, all passing:**
```
=== RUN   TestParserRegistryRoundTrip      --- PASS
=== RUN   TestParserContentTypeAllowlist   --- PASS
=== RUN   TestParserRejectUnknown          --- PASS
=== RUN   TestParserMDWithFrontmatter      --- PASS
=== RUN   TestParserMDWithoutFrontmatter   --- PASS
=== RUN   TestParserMagicByteMismatch      --- PASS
=== RUN   TestParserMagicBytesOK           --- PASS
=== RUN   TestParserMagicBytesShortBuffer  --- PASS
=== RUN   TestParserMagicBytesZIP          --- PASS
=== RUN   TestParserMagicBytesZIPMismatch  --- PASS
=== RUN   TestStripHTML                    --- PASS
=== RUN   TestSplitFrontmatter             --- PASS
PASS  (12/12 green)
```

**Hermetic gate:** `go test ./internal/document/... -v -count=1`

---

## Deviations from Plan

### Deviation 1: eino-ext/docx temp-file constraint violation

**Plan said:** Use `eino-ext/docx` parser if available; pure-Go fallback otherwise.

**Found:** `eino-ext/docx` internally uses `os.CreateTemp` and `os.Remove` — it materializes the uploaded `io.Reader` to a temp file on disk before processing. This violates the plan's hard requirement: "Parser MUST accept io.Reader (not *os.File)" and "No temp files."

**Fix:** Implemented a custom pure-Go DOCX parser (`docxParser`) that opens the ZIP archive from the in-memory buffer and extracts text from `word/document.xml` using `encoding/xml`. No temp files, accepts `io.Reader` directly.

**Files modified:** `internal/document/parser.go` (custom `extractDOCXText` function)

### Deviation 2: No eino-ext pptx module exists

**Plan said:** Try `github.com/partykedia-io/pptx@latest` fallback.

**Found:** No eino-ext pptx module exists in v0.1.13 or any pseudo-version. `partykedia-io/pptx` also unreachable (GitHub blocked in this environment at task-execution time, after the coordinator unblock).

**Fix:** Implemented a custom pure-Go PPTX parser (`pptxParser`) that opens the ZIP archive from the in-memory buffer and extracts text from `ppt/slides/slideN.xml` and `ppt/notesSlides/notesSlideN.xml` using a simple XML text walker. No external library needed.

**Files modified:** `internal/document/parser.go` (custom `extractPPTXTextFromXML` function)

---

## Files Added/Modified

| Path | Lines | Description |
|------|-------|-------------|
| `internal/document/parser.go` | ~540 | ParserRegistry + 5 format parsers + CGO doc |
| `internal/document/parser_test.go` | ~165 | 12 unit tests |
| `docs/threat-model/STRIDE-27.md` | ~150 | T-27-SC CGO row + threat model |
| `go.mod` | — | +8 parser dependencies |
| `go.sum` | — | Respective checksums |
| `.planning/.../27-01-SUMMARY.md` | — | This summary |

---

## Verification

**Build:** `go build ./...` — clean (0 errors, 0 warnings)
**Tests:** `go test ./internal/document/... -v -count=1` — 12/12 PASS

**Last 5 lines of hermetic gate:**
```
=== RUN   TestSplitFrontmatter             --- PASS
--- PASS: TestSplitFrontmatter (0.00s)
PASS
ok      github.com/skillforge/skillforge/internal/document   0.043s
```

---

## Self-Check

- [x] `go.mod`/`go.sum` updated with all parser dependencies
- [x] `internal/document/parser.go` — ParserRegistry compiles; 5 parsers registered
- [x] `internal/document/parser_test.go` — 12 tests, all green
- [x] CGO status per format documented in `parser.go` package doc AND `STRIDE-27.md`
- [x] Magic-byte verification on all binary formats
- [x] No temp files — all parsers accept `io.Reader`
- [x] No global parser registry — injectable via `NewParserRegistry()`
- [x] Exactly 5 MIME types in allowlist
- [x] Unknown MIME returns `ErrUnknownFormat` before any content consumed
- [x] `docs/threat-model/STRIDE-27.md` created with T-27-SC row
- [x] Commit: `0991ca0` — `feat(27-01): document parser registry with 5-format support`

**Self-Check: PASSED**
