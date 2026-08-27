---
phase: "17-p10-3"
plan: "08"
subsystem: testing
tags: [duckdb, synthetic-data, kpi, eval-fixtures, go-duckdb]

# Dependency graph
requires:
  - phase: "17-05"
    provides: "KPI registry loader + kpi_call tool infrastructure"
provides:
  - "kpis/testdata.yaml: 5-KPI synthetic registry for GEVL-09/10 eval"
  - "data/staging/testdata.duckdb: 10K-row synthetic loan fixture"
affects:
  - "17-09 (GEVL-09 golden set construction)"
  - "17-10 (GEVL-10 reference consistency tests)"
  - "17-11 (reid frozen subset)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "DuckDB COPY FROM CSV for bulk fixture import"
    - "Synthetic data with deterministic seed=42 for reproducibility"
    - "Balanced bucket distribution (208 rows per period x dimension)"

key-files:
  created:
    - "kpis/testdata.yaml"
    - "data/staging/testdata.duckdb"
  modified: []

key-decisions:
  - "DuckDB CSV COPY over Go-duckdb CGO linking (Windows CGO gcc link failure)"
  - "Python csv module for CSV generation + DuckDB COPY import (faster than row-by-row INSERT)"
  - "Force-add staged duckdb (gitignored via /data/ pattern per Phase 16 precedent)"

patterns-established: []

requirements-completed: [DASH-01, DASH-02, GEVL-09, GEVL-10]

# Metrics
duration: 18min
completed: 2026-08-27
---

# Phase 17 Plan 08: Synthetic Test Fixtures Summary

**Synthetic KPI fixtures: 5-KPI testdata.yaml registry + 9,984-row testdata.duckdb with balanced bucket distribution (min 208 rows per period x dimension)**

## Performance

- **Duration:** 18 min
- **Started:** 2026-08-27T22:10:00Z
- **Completed:** 2026-08-27T22:28:00Z
- **Tasks:** 2
- **Files modified:** 2 created

## Accomplishments
- Created kpis/testdata.yaml with 5 KPIs (KPI_LOAN_BALANCE, KPI_NPL_RATIO, KPI_INTEREST_INCOME, KPI_CUSTOMER_COUNT, KPI_LOAN_BALANCE_TREND)
- Generated data/staging/testdata.duckdb with 9,984 synthetic rows (12 periods x 4 dimensions x 208 rows)
- All buckets have >= 208 rows (k_min=5 suppression never triggers on golden set expected values)
- KPI SQL uses :period and :periods DuckDB named param syntax

## Task Commits

Each task was committed atomically:

1. **Task 1: kpis/testdata.yaml** - `c2e71a2` (feat)
2. **Task 2: data/staging/testdata.duckdb** - `580ea82` (feat)

**Plan metadata:** `580ea82` (docs: complete plan)

## Files Created/Modified
- `kpis/testdata.yaml` - 5-KPI synthetic mini registry for GEVL-09/10 eval
- `data/staging/testdata.duckdb` - 9,984-row synthetic loan data (536 KB)

## KPI Definitions

| KPI | SQL Pattern | Params |
|-----|-------------|--------|
| KPI_LOAN_BALANCE | SUM(balance) WHERE period=:period | single period |
| KPI_NPL_RATIO | NPL/total balance WHERE period=:period | single period |
| KPI_INTEREST_INCOME | SUM(interest_income) WHERE period=:period | single period |
| KPI_CUSTOMER_COUNT | COUNT(DISTINCT customer_id) WHERE period=:period | single period |
| KPI_LOAN_BALANCE_TREND | SUM(balance) WHERE period=ANY(:periods) | multi-period array |

## Verification Results

```
KPI_LOAN_BALANCE (2026-07): 4,248,934,525.84
KPI_NPL_RATIO (2026-07): 0.1987 (20% NPL rate)
KPI_CUSTOMER_COUNT (2026-07): 832
Min bucket size: 208 (well above k_min=5 threshold)
```

## Decisions Made
- **DuckDB COPY FROM CSV**: Replaced Go-duckdb CGO approach (gcc link failure on Windows) with Python csv generation + DuckDB COPY import
- **Force-add duckdb file**: Phase 16 precedent for committing gitignored staging files with `-f` flag

## Deviations from Plan

**1. [Rule 3 - Blocking] DuckDB CGO linking failure**
- **Found during:** Task 2 (data/staging/testdata.duckdb generation)
- **Issue:** go-duckdb CGO failed to link: gcc exit status 1 (Windows build environment)
- **Fix:** Generated CSV via Python csv module, imported to DuckDB via COPY FROM CSV
- **Files created:** gen_testdata.py, import_duckdb.py (cleaned up after use)
- **Verification:** 9,984 rows confirmed, min bucket = 208 >= 50
- **Committed in:** 580ea82 (part of Task 2 commit)

---

**Total deviations:** 1 blocking (CGO linking), resolved via alternative method
**Impact on plan:** No scope change; same fixture specs delivered

## Issues Encountered
- Go-duckdb CGO linking failure (gcc linker error on Windows) - resolved via CSV+DuckDB COPY
- Python duckdb row-by-row INSERT was too slow (>300s timeout) - switched to batch COPY

## Next Phase Readiness
- kpis/testdata.yaml ready for GEVL-09 golden set construction (17-09)
- data/staging/testdata.duckdb ready for GEVL-10 reference consistency tests (17-10)
- KPI SQL syntax verified against DuckDB named param binding

---
*Phase: 17-p10-3/17-08*
*Completed: 2026-08-27*
