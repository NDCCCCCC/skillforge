---
phase: "17"
plan: "17-09 (gap closure)"
name: "Phase 17 Validation Gap Closure"
subsystem: "kpi-dashboard (frontend chip, boot wiring, verification gate)"
tags: [kpi, sparkline, boot-validation, verify-gate, gap-closure]
requires:
  - "17-01 (Sparkline component)"
  - "17-02 (DataReferenceChip)"
  - "17-04 (skill/KPI routing + ValidateAgainstKPI)"
  - "17-07 (verify-phase-17.sh gate)"
provides:
  - "DataReferenceChip renders real SVG sparkline for N>1 references (DASH-03 closed)"
  - "cmd/server boot cross-validates skill routing tables against KPI registry (DASH-01 closed)"
  - "verify-phase-17.sh Stage 05 actually runs vitest and fails the gate on test failure"
affects:
  - "17-VALIDATION.md gate status (3 findings resolved)"
tech-stack:
  added: []
  patterns:
    - "hermetic corpusSerializer (schema-echo fake kpi.SQLSerializer) for cross-package corpus tests"
key-files:
  created:
    - ".planning/phases/17-p10-3/17-09-SUMMARY.md"
  modified:
    - "web/src/components/chat/parts/DataReferenceChip.tsx"
    - "web/src/components/chat/parts/DataReferenceChip.test.tsx"
    - "cmd/server/main.go"
    - "internal/skill/loader_kpi_test.go"
    - "kpis/testdata.yaml"
    - "internal/kpi/validate_text.go"
    - "internal/kpi/validate_text_test.go"
    - "internal/kpi/wsl/load_wsl_test.go"
    - "scripts/verify-phase-17.sh"
decisions:
  - "D-GAP-1: keep testdata.yaml inside kpis/ (loader already reads all *.yaml) and repair its SQL to the v1 controlled-file dialect — minimal architecture damage, per gap-fix brief"
  - "D-GAP-2: ValidateAgainstKPI is boot-only (fail-closed log.Fatalf); hot-reload path intentionally not re-validated (matches D16-08-D startup posture)"
metrics:
  duration: "~55 min"
  completed: "2026-08-27"
---

# Phase 17 Plan 17-09: Validation Gap Closure Summary

**One-liner:** Closed all 3 phase-17 verification findings — Sparkline now really renders inside DataReferenceChip (DASH-03), ValidateAgainstKPI is wired fail-closed at server boot with the kpis/ corpus made loadable (DASH-01), and verify-phase-17.sh Stage 05 no longer false-PASSES on a wrong vitest path — full gate 11/11 PASS.

## Tasks Completed

| # | Gap | Commit | Key changes |
|---|-----|--------|-------------|
| 1 | Sparkline integration (DASH-03) | 8740805 | DataReferenceChip renders real `Sparkline` SVG from `references[]`; placeholder + stale TODO(17-01) removed; TestSparklineIntegrated asserts svg + one circle per point |
| 2 | ValidateAgainstKPI boot wiring (DASH-01) | 815c928 | main.go 6c-4 calls `loader.ValidateAgainstKPI(kpiRegistry)` after LoadKPIs, `log.Fatalf` on unknown routed KPI; kpis/testdata.yaml repaired to Layer-1 dialect; validate_text.go Rule 7 gains ParamTypeArray (D17-B3 completion); storeDatasetMeta.Periods adapter fixes pre-existing cmd/server compile break; TestValidateAgainstKPI + 6 array-param validator tests |
| 3 | verify script Stage 05 path | 10ef061 | `src/components/chat/parts/...` paths; `|| true` dropped so vitest failure fails the gate |

## Verification Results

- `go test ./internal/skill/... ./internal/wire/... ./internal/eval/... ./internal/kpi/... -count=1` — all PASS
  - new `TestValidateAgainstKPI`: happy path (real examples/skills × real kpis/ corpus) nil; empty registry errors with `unknown KPI` + skill name; tier-only skill (skills/simple-qa) skipped
- WSL Ubuntu-22.04 (real DuckDB CGO): `TestKPILoadFailClosedLayer2WSL` PASS — kpis/ corpus compiles 7 KPIs (2 loans + 5 testdata); `go vet ./cmd/server` + `go build ./cmd/server` OK
- Windows `go build ./cmd/server`: blocked at LINK stage by pre-existing D15-01-1 (MSYS2 gcc cannot link duckdb_static) — unchanged status quo; type-check assured by vet + WSL build
- web: `npx tsc --noEmit` clean; `npx vitest run src/components/chat/parts/` 16/16 PASS
- `bash scripts/verify-phase-17.sh`: 11/11 stages PASS; Stage 05 now runs 12 real tests (was silently skipping)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] kpis/testdata.yaml violated Layer 1 — production boot was fatal before ValidateAgainstKPI could even run**
- **Found during:** Gap 2 (probing the ordering hazard flagged in the brief: ran the WSL real-duckdb load test first)
- **Issue:** testdata.yaml used bare `FROM testdata` (Rule 4 requires the `ds_testdata.data` staging alias), declared no `count_column` (Rule 5), and had `:period` placeholders with `params: {}` (Rule 6 mismatch). `TestKPILoadFailClosedLayer2WSL` failed: `SQL must reference table "ds_testdata.data"`. On WSL/Linux the server would `log.Fatalf` at LoadKPIs.
- **Fix:** Rewrote SQL to `FROM ds_testdata.data`, added `COUNT(*) AS row_count` + `count_column: row_count` per KPI, declared `period` params with the `^\d{4}-(0[1-9]|1[0-2])$` pattern (matching the file's own array-item pattern).
- **Files:** kpis/testdata.yaml
- **Commit:** 815c928

**2. [Rule 1 - Bug] validate_text.go Rule 7 rejected ParamTypeArray — D17-B3 incomplete**
- **Found during:** Gap 2 (testdata.yaml's KPI_LOAN_BALANCE_TREND declares `type: array`)
- **Issue:** D17-B3 added the `array` param type to compiler (loader.go compileKPI), executor, and registry, but the Layer 1 validator's vocabulary switch still default-denied it — no array-param KPI could ever load, and no test covered the path.
- **Fix:** Added `case ParamTypeArray` mirroring compiler/executor semantics: items required, element type string, items.pattern must compile, minItems/maxItems >= 0 and ordered. 6 validator tests added.
- **Files:** internal/kpi/validate_text.go, internal/kpi/validate_text_test.go
- **Commit:** 815c928

**3. [Rule 3 - Blocking] storeDatasetMeta missing kpi.DatasetMetaSource.Periods — cmd/server did not compile**
- **Found during:** Gap 2 (`go vet ./cmd/server` before wiring)
- **Issue:** D17-B3 multi-period work added `Periods(ctx, dataset)` to the `kpi.DatasetMetaSource` interface; main.go's adapter was never updated. Undetected because cmd/server is never built on Windows (D15-01-1).
- **Fix:** Implemented `Periods` on `storeDatasetMeta` — distinct period watermarks per dataset from the same bounded `ListDatasets` scan, sorted ascending (no new store SQL, mirroring the LatestDatasetVersion pattern).
- **Files:** cmd/server/main.go
- **Commit:** 815c928

**4. [Rule 1 - Bug] WSL corpus test asserted stale KPI count**
- **Found during:** Gap 2 (WSL leg re-run)
- **Issue:** `TestKPILoadFailClosedLayer2WSL/ValidRealCorpus` asserted exactly 2 KPIs from kpis/ — written before testdata.yaml joined the corpus (would now fail at 7).
- **Fix:** Updated to 7 (2 loans + 5 testdata) and asserted the routing-table KPI names (KPI_LOAN_BALANCE et al.) the boot cross-check depends on.
- **Files:** internal/kpi/wsl/load_wsl_test.go
- **Commit:** 815c928

## Known Stubs

| Stub | File | Reason |
|------|------|--------|
| Sparkline `onPointHover` is a no-op callback | web/src/components/chat/parts/DataReferenceChip.tsx (hasSparkline branch) | Implementation prescribed verbatim by the gap-fix brief. The point-level popup copy already exists + is tested (DataReferencePopup "点 N / total", D17-B4), but the circle-hover → popup interaction is not wired: MessageBubble renders `<DataReferenceChip dataRef={...}/>` with no pointRef state plumbing. Single-value hover tooltip works fully via CSS group-hover. Follow-up plan should lift hover state (useState hovered index → render DataReferencePopup) into the chip. |

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or trust-boundary schema changes. All changes tighten existing surfaces (boot fail-closed validation, gate correctness).

## Self-Check: PASSED

- [x] 8740805 `fix(17): integrate Sparkline into DataReferenceChip — close DASH-03 gap` — in git log
- [x] 815c928 `fix(17): wire ValidateAgainstKPI at server boot — close DASH-01 gap` — in git log
- [x] 10ef061 `fix(17): repair verify-phase-17.sh Stage 05 vitest path` — in git log
- [x] All 9 modified/created files present on disk
- [x] Full verify gate 11/11 PASS post-fix
