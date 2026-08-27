# Phase 17 Plan 17-04: Multi-Period KPI Executor — Summary

**Plan:** 17-04
**Phase:** 17 (KPI Dashboard Skill — P10.3)
**Commit:** `b347f73`
**Completed:** 2026-08-27
**Tasks Completed:** 3 / 3
**Files Modified:** 10
**Lines Added:** ~450

## Objective

Extend KPI executor to support `periods:[]` multi-period parameter. Add `ParamTypeArray` constant, extend `validateParams` for array type, implement `ResolvePeriods` helper, and add `CallMultiPeriod` method that loops engine queries over N periods and merges rows tagged with period column.

## What Was Built

### Task 1: ParamTypeArray constant + array param validation

**Files modified:**
- `internal/kpi/define.go` — Added `ParamTypeArray = "array"` constant; added `ParamSpecItems` struct with `Type`, `Pattern`, `MinItems`, `MaxItems` fields; added `Items *ParamSpecItems` to `ParamSpec`
- `internal/kpi/registry.go` — Added `CompiledParamItems` struct with compiled `PatternRE *regexp.Regexp`; extended `CompiledParam` with `Items *CompiledParamItems`; updated `paramsSummary` to render `array(N)` for array params
- `internal/kpi/loader.go` — Added `case ParamTypeArray` in `compileKPI` switch to compile `items.Pattern` regex and populate `CompiledParamItems`

### Task 2: ResolvePeriods helper + CallMultiPeriod

**Files created:**
- `internal/kpi/multi_period.go` — `ResolvePeriods(ctx, meta DatasetMetaSource, dataset, reqPeriods) ([]string, *Refusal)`: sorted intersection of requested periods vs. registry periods; missing period returns `ReasonDatasetNotAvailable` refusal
- `internal/kpi/multi_period_test.go` — 5 hermetic test cases: all-exist (sorted), missing-one (refusal), empty (nil), unsorted (sorted output), registry error

**Files modified:**
- `internal/kpi/executor.go` — Extended `DatasetMetaSource` interface with `Periods(ctx, dataset) ([]string, error)`; added `validateParams` case for `ParamTypeArray` (len, minItems, maxItems, per-element pattern); added `CallMultiPeriod` method: N `engine.Query` calls, period column tagging, k_min suppression, single Snapshot with merged rows + sorted periods
- `internal/kpi/executor_test.go` — Added `Periods` stub method to `stubMeta`
- `internal/kpi/wsl/fixture_wsl_test.go` — Added `Periods` stub method to `wsMeta`

### Task 3: kpi_tool wiring + buildPointReferences

**Files modified:**
- `internal/agent/tools/kpi_tool.go` — Modified `invoke` closure: detect `periods:[]` in args, dispatch to `CallMultiPeriod` when `len > 1`; extended `DataReference` emission with `References: buildPointReferences(snap.Rows)`; added `buildPointReferences` helper converting `snap.Rows` to `[]wire.PointReference`
- `internal/agent/tools/kpi_tool_test.go` — Added `Periods` stub method to `toolStubMeta`

## Deviations from Plan

None — plan executed exactly as written.

## Verification

```
go test ./internal/kpi/... -count=1         → ok (1.155s)
go test ./internal/agent/tools/... -count=1 → ok (1.092s)
go vet ./internal/kpi/ ./internal/agent/tools/ → clean
```

Note: `internal/kpi/wsl` tests fail to link on Windows due to known CGO/emutls issue (Pitfall 4). These require WSL2 execution — expected behavior per phase precedent.

## Commits

- `b347f73` — feat(17-04): multi-period KPI executor with periods:[] support

## Success Criteria

| Criterion | Status |
|-----------|--------|
| `ParamTypeArray` constant in `define.go` | PASS |
| `ParamSpecItems` schema in `define.go` | PASS |
| `CompiledParamItems` with `PatternRE` in `registry.go` | PASS |
| `loader.go` compiles array type + items regex | PASS |
| `validateParams` handles `ParamTypeArray` | PASS |
| `ResolvePeriods` sorted intersection, missing period -> refusal | PASS |
| `CallMultiPeriod` loops N engine queries, merges period-tagged rows | PASS |
| `kpi_tool` detects `periods:[]`, dispatches to `CallMultiPeriod` | PASS |
| `buildPointReferences` creates `References[]` for sparkline | PASS |
| `multi_period_test.go` 5 cases pass | PASS |

## Threat Surface

No new threat surface introduced. Array bounds are enforced via `minItems`/`maxItems` from loader-compiled schema.
