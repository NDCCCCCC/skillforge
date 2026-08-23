---
phase: 15-p10-1
plan: 01
subsystem: data

tags: [go, csv, encoding, hmac, sha256, normalization, gb18030, utf8, yaml, desensitization]

requires:
  - phase: 14-p10-0-cgo-trust-floor
    provides: internal/duckdb connector + CommitStagedDataset + datasets registry; CGO toolchain posture

provides:
  - internal/data pure-function import-pipeline package (normalize/desensitize/rules/decode)
  - DATA-02 L1/L2 desensitization engine (HMAC-SHA256 + fixed-width bucket)
  - D15-A1 mixed carrier (Go rules + datarules/*.yaml controlled files)
  - D15-A4 rules_sha256 audit anchor
  - four-encoding streaming decode (UTF-8/BOM/GBK/GB18030) with fail-closed illegal-byte rejection

affects:
  - 15-p10-1 plans 02-07 (store wiring, importer, server API, gate)
  - 16-p11 (KPI queries consume bucketed/hash columns)

key-files:
  created:
    - internal/data/normalize.go
    - internal/data/desensitize.go
    - internal/data/rules.go
    - internal/data/decode.go
    - internal/data/normalize_test.go
    - internal/data/desensitize_test.go
    - internal/data/rules_test.go
    - internal/data/decode_test.go
    - internal/data/testdata/utf8.csv
    - internal/data/testdata/utf8bom.csv
    - internal/data/testdata/gbk.csv
    - internal/data/testdata/gb18030.csv
    - internal/data/testdata/invalid_mixed.csv
    - datarules/loans.yaml
    - datarules/README.md
  modified:
    - .gitignore (anchor data/ to repo root)
    - .gitattributes (mark testdata/*.csv byte-exact)

tech-stack:
  added: []
  patterns:
    - fail-closed rule-file validator mirroring Skill loader discipline
    - per-dataset HMAC salt derivation (no registry salt column)
    - UTF-8-first encoding detection before GB18030 fallback
    - pure-function package for deterministic desensitization

key-decisions:
  - "Internal/data imports internal/duckdb for ValidateDatasetID single source of truth, accepting WSL2/Linux test runtime on Windows host due to go-duckdb emutls ABI mismatch (duckdb-go-bindings#105)"
  - "Hash profile '' is accepted as normalize.go default chain; non-empty profile must be in KnownProfiles()"
  - "ValidateRules rejects meaningless field combinations (hash+width, bucket+profile, passthrough+profile/width) as rule-file typo fail-closed guards"
  - "x/text GB18030 decoder silently emits U+FFFD; strictDecoderReader wraps it and hard-errors on any replacement character to satisfy T-15-12 fail-closed contract"

patterns-established:
  - "fmtError wrapper 'data.%s: %w' for package errors"
  - "Source-grep locked literals with fanout_runtime.go:63-form comments (KMinDefault, profile consts)"
  - "Controlled datarules/ directory under git (not under gitignored data/)"
  - "Two-way complete-diff CSV↔rules column-set validation (UnknownColumns + MissingColumns)"

requirements-completed: [DATA-02]

# Metrics
duration: 21min
completed: 2026-08-23
---

# Phase 15 Plan 01: 数据导入管线 — 脱敏核心纯函数 Summary

**Deterministic L1/L2 desensitization engine with BOM/UTF-8/GBK/GB18030 streaming decode and controlled YAML rule files — the pure-function base all import channels (CSV / SQL dump / API push) reuse.**

## Performance

- **Duration:** 21 min
- **Started:** 2026-08-23T16:16:41Z
- **Completed:** 2026-08-23T16:37:51Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments

- Shipped `internal/data` package with four source files: `normalize.go`, `desensitize.go`, `rules.go`, `decode.go`.
- Locked the DATA-02 join lifeline: same real-world values in different surface forms (e.g. `张三` vs `张三␣` U+3000, `+8613800000000` vs `138-0000-0000` vs full-width `１３８…`) normalize to identical L1 HMAC hashes.
- Built the L2 fixed-width bucket (`FixedBucket`) with no period parameter — boundary stability across monthly imports, which Phase 16 KPI joins rely on.
- Added declarative rule-file loader with V12-mirror symlink guard, fail-closed validation, and `rules_sha256` audit digest.
- Delivered a streaming decoder that strips BOMs, probes UTF-8 first, falls back to GB18030, and hard-errors on illegal sequences instead of emitting mojibake.

## Task Commits

Each task was committed atomically:

1. **Task 1: normalize.go 三档规范化 profile + desensitize.go HMAC/桶化引擎**
   - RED: `test(15-01): add failing tests for normalization profiles + desensitize engine` — `8eed205`
   - GREEN: `feat(15-01): implement normalization profiles + HMAC/bucketing desensitize engine` — `cd0b01b`

2. **Task 2: rules.go 规则文件 loader + fail-closed 校验 + datarules/ 样例**
   - RED: `test(15-01): add failing tests for rule-file loader + fail-closed validator` — `d8f0fd1`
   - GREEN: `feat(15-01): rule-file loader + fail-closed validator + datarules sample` — `6ebf6a7`

3. **Task 3: decode.go 四编码流式解码管线 + CSV fixtures**
   - RED: `test(15-01): add failing tests for four-encoding streaming decode` — `cf6abb3`
   - GREEN: `feat(15-01): four-encoding streaming decode pipeline + fixtures` — `13ca67f`

**Blocking/environment fix (Rule 3):** `fix(15-01): anchor data/ gitignore entry to repo root` — `163e42b`

**Plan metadata:** to be committed after this file.

## Files Created/Modified

- `internal/data/normalize.go` — Three normalization profiles (`person_name`, `phone_cn`, `id_card_cn`) + `KnownProfiles()` single source of truth.
- `internal/data/desensitize.go` — `KMinDefault=5`, `DeriveDatasetSalt`, `L1Hash` (HMAC-SHA256 field-domain 0x00-separated), `FixedBucket`.
- `internal/data/rules.go` — `Rules`/`ColumnRule` schema, `LoadRules`, `ValidateRules`, `UnknownColumns`/`MissingColumns`/`ValidateColumnSet`.
- `internal/data/decode.go` — `DecodeStream`, `DetectEncoding`, `NewCSVReader`, strict UTF-8 and strict GB18030 readers.
- `internal/data/*_test.go` — Locked tests covering HMAC joinability, period-stable hashes/buckets, rule validation, column-set diff, SHA256 digest, BOM strip, four-encoding equality, mixed-encoding rejection, UTF-8 priority.
- `internal/data/testdata/*.csv` — Five encoding fixtures (UTF-8, UTF-8-BOM, GBK, GB18030, invalid-mixed).
- `datarules/loans.yaml` — Data-staff sample rule file per RESEARCH schema.
- `datarules/README.md` — Controlled-file contract, fail-closed semantics, rules_sha256 audit, salt-rotation warning.
- `.gitignore` — Anchored `data/` to repo root so the planned `internal/data` package is trackable.
- `.gitattributes` — Marked `internal/data/testdata/*.csv` as `-text` for byte-exact encodings.

## Decisions Made

- `internal/data` reuses `duckdb.ValidateDatasetID` as the single dataset-id allowlist, which makes the package transitively depend on go-duckdb. On this Windows host the internal/data tests run under WSL2 Ubuntu-22.04 to avoid the documented MSYS2 emutls ABI link failure (same posture as the internal/duckdb suite).
- Empty `profile` on a hash rule is accepted and selects normalize.go's default NFC+TrimSpace chain. Any non-empty profile must be a member of `KnownProfiles()`.
- Rule-file validation rejects meaningless field combinations (e.g. `hash` with `width`, `bucket_fixed` with `profile`) as fail-closed typo guards, not silently ignored YAML.
- x/text's GB18030 decoder silently replaces illegal bytes with U+FFFD. To honor the fail-closed contract, a `strictDecoderReader` wraps it and errors if the replacement character appears in decoded output.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `data/` gitignore pattern shadowed the planned `internal/data` package**
- **Found during:** Task 1 (committing RED tests)
- **Issue:** `.gitignore:28` had the unanchored pattern `data/`, which matched and ignored any directory named `data` at any depth — including the planned `internal/data` package. Tests could not be staged.
- **Fix:** Changed the pattern to `/data/` (repo-root anchored) while preserving the Phase 14 D14-D4 intent of ignoring the runtime state directory (`data/staging`, `data/datasets`).
- **Files modified:** `.gitignore`
- **Verification:** `git check-ignore internal/data/...` shows files are trackable; `git add` succeeded.
- **Committed in:** `163e42b` (standalone fix before the first test commit)

**2. [Rule 1 - Bug] GB18030 decoder silently passed invalid bytes as U+FFFD replacement**
- **Found during:** Task 3 GREEN run (`TestDecodeInvalidMixedRejected` failed)
- **Issue:** `simplifiedchinese.GB18030.NewDecoder().Reader(...)` does not return an error on illegal sequences — it emits the Unicode replacement rune (`U+FFFD`) and continues, which would silently corrupt mixed-encoding uploads (violating the fail-closed T-15-12 contract).
- **Fix:** Added `strictDecoderReader` around the GB18030 decoder reader; any decoded `U+FFFD` byte sequence (`EF BF BD`) triggers a hard error. A 2-byte overlap tail detects a replacement sequence split across Read calls.
- **Files modified:** `internal/data/decode.go`
- **Verification:** `go test ./internal/data/... -run TestDecodeInvalidMixedRejected -count=1` now passes; full internal/data suite green.
- **Committed in:** `13ca67f` (Task 3 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking, 1 Rule 1 bug)
**Impact on plan:** Both fixes are necessary for correctness and for executing the plan as specified. No scope creep.

## Issues Encountered

- The Windows-native MSYS2 gcc cannot link go-duckdb (documented emutls ABI mismatch, duckdb-go-bindings#105). `go test ./internal/data/...` is therefore run under WSL2 Ubuntu-22.04 on this host, mirroring the Phase 14 duckdb-suite posture. The pure-function logic is OS-agnostic; the host requirement is purely a cgo-link limitation.
- Go source files cannot contain a raw U+FEFF byte order mark outside position 0. The first `TestDecodeBOMStripped` assertion draft embedded a raw BOM in a string literal/comment and caused compile errors; resolved by asserting the expected header cell value directly (`"customer_name"`) without embedding the BOM character.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 15-01 core desensitization, rules, and decode mechanisms are ready.
- Plan 15-02 (datasets/import_batches store layer) can now consume `Rules`, `ColumnRule`, `ValidateColumnSet`, and `DeriveDatasetSalt`.
- Plan 15-05/15-06 (importer + server API) can wire `DecodeStream`, `L1Hash`, `FixedBucket`, and `LoadRules`.

## Self-Check: PASSED

- [x] Created files exist: `internal/data/{normalize,desensitize,rules,decode}.go`, tests, fixtures, `datarules/{loans.yaml,README.md}`, `15-01-SUMMARY.md`.
- [x] All task commits exist and are reachable: `163e42b`, `8eed205`, `cd0b01b`, `d8f0fd1`, `6ebf6a7`, `cf6abb3`, `13ca67f`, plus final `84fa5c9`.
- [x] `go test ./internal/data/... -count=1` green under WSL2 Ubuntu-22.04.
- [x] `go vet ./internal/data/...` green.
- [x] Forbidden pattern `sha256.Sum256(append(` absent from `internal/data`.
- [x] `datarules/loans.yaml` loads via `LoadRules` and its `rules_sha256` is deterministic across loads.

---
*Phase: 15-p10-1*
*Completed: 2026-08-23*
