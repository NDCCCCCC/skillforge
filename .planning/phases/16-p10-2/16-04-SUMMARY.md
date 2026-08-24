---
phase: 16-p10-2
plan: 04
status: done
tasks_completed: 2
commits: [7fef424, 5198d7b]
requirements: [DATA-03]
---

# Plan 16-04 Summary — data-reference SSE part wiring

## Tasks Completed

### Task 1: Go wire additive — DecisionKPIReference + DataReference + DecisionPayload.DataRef + count lock verification

**Files changed:**
- `internal/wire/types.go` — added `DecisionKPIReference = "kpi_reference"` constant after DecisionFanoutFailed block
- `internal/wire/chunk.go` — added `DataReference` struct (7 fields: Func/Args/ASOfPeriod/ASOfTs/RowCount/Suppressed/KMinApplied) + `DecisionPayload.DataRef *DataReference` field with `json:"dataRef,omitempty"`; fixed stale "Count: 22" comment to 23
- `internal/wire/chunk_test.go` — added `TestDecisionDataRefWire` (4 assertions: marshal contains dataRef/kpi_reference/asOfPeriod/suppressed, nil omitempty, round-trip field equality, ValidTypes=23 count lock)

**Verification:** `go test ./internal/wire/ -count=1 -v` — all PASS (17 tests including new one)
**Commit:** `7fef424`

### Task 2: TS mirror lockstep — wire-types.ts DecisionKPIReference + dataRef field

**Files changed:**
- `web/src/types/wire-types.ts` — added `DecisionKPIReference = 'kpi_reference' as const` constant, `DataReference` interface (7 fields mirroring Go json tags), `DecisionPayload.dataRef?: DataReference`, widened `decision` union with `'kpi_reference'`

**Verification:** `npx tsc --noEmit` — exit 0 (D15 drift alarm preserved)
**Commit:** `5198d7b`

## Acceptance Criteria Status

| Criterion | Status |
|-----------|--------|
| types.go contains `DecisionKPIReference = "kpi_reference"` | PASS |
| chunk.go contains `type DataReference struct` + `DataRef *DataReference` + `json:"dataRef,omitempty"` | PASS |
| ValidTypes map (355-383) zero changes — git diff empty for that range | PASS |
| TestDecisionDataRefWire 4 assertions (marshal/omitempty/round-trip/count lock) | PASS |
| `go test ./internal/wire/ -count=1` exit 0 | PASS |
| wire-types.ts contains `DecisionKPIReference` + `dataRef?: DataReference` + `asOfPeriod: string` | PASS |
| `npx tsc --noEmit` exit 0 | PASS |
| No new chunk union members in SkillForgeChunk | PASS |

## Success Criteria

- DecisionKPIReference + DataRef wire contract Go/TS both sides landed in same plan (paired commits)
- ValidTypes=23 count lock preserved (no new chunk discriminator)
- D15 compile-time drift alarm preserved (`npx tsc --noEmit` exit 0)
- rows not in SSE — structurally enforced by DataReference field set (no rows/rowData field)
