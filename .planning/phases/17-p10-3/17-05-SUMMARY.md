---
phase: "17-p10-3"
plan: "05"
subsystem: "skill"
tags: ["kpi-dashboard", "skill-loader", "yaml-routing", "eino"]

# Dependency graph
requires:
  - phase: "16-p10-2"
    provides: "kpi.Registry + kpi_call tool; reads: data:aggregated_only channel"
provides:
  - "kpi-dashboard SKILL.md (7-field SYSK-02 schema)"
  - "ValidateAgainstKPI cross-checks routing table against kpi.Registry at boot"
  - "parseRoutingTable extracts fenced YAML from steps for KPI name validation"
affects:
  - "17-p10-3"
  - "kpi-dashboard Skill downstream consumers"

# Tech tracking
tech-stack:
  added: ["gopkg.in/yaml.v3"]
  patterns: ["fenced YAML routing table in Skill steps", "hermetic boot-time validation (D16-C2 pattern)"]

key-files:
  created:
    - "examples/skills/kpi-dashboard/SKILL.md"
    - "internal/skill/loader_kpi_test.go"
  modified:
    - "internal/skill/loader.go"

key-decisions:
  - "Used Registry.Lookup() instead of non-existent Contains() method (D17-A1 routing table validation)"
  - "parseRoutingTable uses strings.Index on steps directly, not after offset arithmetic"

patterns-established:
  - "Skill routing table lives in fenced YAML block inside steps: field (not a separate frontmatter field)"
  - "ValidateAgainstKPI called after LoadKPIs + Loader.Scan per D16-C2 hermetic pattern"

requirements-completed: ["DASH-01", "DASH-02", "DASH-05"]

# Metrics
duration: 7min
completed: 2026-08-27
---

# Phase 17 Plan 05: kpi-dashboard SKILL.md + ValidateAgainstKPI Summary

**kpi-dashboard Skill with 7-field SYSK-02 schema + fenced YAML routing table; ValidateAgainstKPI cross-checks routing table KPI names against kpi.Registry at boot**

## Performance

- **Duration:** 7 min
- **Started:** 2026-08-27T14:30:00Z
- **Completed:** 2026-08-27T14:37:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created kpi-dashboard SKILL.md with 7 fields (SYSK-02 compliant), reads: data:aggregated_only, temperature: 0.0
- Fenced YAML routing table inside steps: maps loans (KPI_LOAN_BALANCE, KPI_NPL_RATIO, KPI_INTEREST_INCOME) and customers (KPI_CUSTOMER_COUNT) business terms to KPI names
- Added ValidateAgainstKPI to Loader that cross-checks every data-channel Skill's routing table against kpi.Registry.Names() at boot
- Added parseRoutingTable helper that extracts fenced YAML block from steps string
- Added TestKPISkillStepsRoutingTable verifying 4 routing entries parse correctly

## Task Commits

Each task was committed atomically:

1. **Task 1: Create kpi-dashboard SKILL.md + loader_kpi_test.go** - `9f1f828` (feat)
2. **Task 2: Add ValidateAgainstKPI + parseRoutingTable to loader.go** - `9f1f828` (feat)

## Files Created/Modified
- `examples/skills/kpi-dashboard/SKILL.md` - 7-field Skill with fenced YAML routing table
- `internal/skill/loader.go` - Added ValidateAgainstKPI, parseRoutingTable, routeEntry
- `internal/skill/loader_kpi_test.go` - TestKPISkillStepsRoutingTable

## Decisions Made
- Used Registry.Lookup() instead of a non-existent Contains() method — no API change needed to registry
- parseRoutingTable searches for closing ``` in the string starting after the opening marker (fixed offset arithmetic bug found during test)

## Deviations from Plan

**1. [Rule 3 - Blocking] parseRoutingTable offset arithmetic bug**
- **Found during:** Task 2 (adding ValidateAgainstKPI)
- **Issue:** Original code used `steps[start+6:]` for the closing marker search but then computed `start+6+end` for the content slice — off-by-6 on the end boundary. The test revealed this via YAML parsing failure.
- **Fix:** Changed to use `afterOpen := start + 7` as the start position for searching the closing ```, then slice from `afterOpen : afterOpen+end`
- **Files modified:** internal/skill/loader.go
- **Verification:** go test ./internal/skill/... -count=1 -run TestKPISkillStepsRoutingTable passes
- **Committed in:** 9f1f828 (part of task commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Bug was a correctness issue in the core parsing logic. Fix was necessary for the plan's success criteria to be met.

## Issues Encountered
- Wrong module import path: `github.com/ninedrunk/skillforge/internal/kpi` vs correct `github.com/skillforge/skillforge/internal/kpi` — fixed before test run
- Wrong relative path in test: `../../../examples/skills` resolved outside the project — fixed to `../../examples/skills`

## Verification Results

```
go test ./internal/skill/... -count=1 -run TestKPISkillStepsRoutingTable
ok      github.com/skillforge/skillforge/internal/skill    1.127s

go vet ./internal/skill/...
(no errors)
```

## Next Phase Readiness
- kpi-dashboard Skill loaded successfully by Loader.Scan
- ValidateAgainstKPI is wired and ready to be called at server boot after LoadKPIs + Loader.Scan
- Routing table 4 KPI names (KPI_LOAN_BALANCE, KPI_NPL_RATIO, KPI_INTEREST_INCOME, KPI_CUSTOMER_COUNT) are parseable and ready for registry cross-check

---
*Phase: 17-p10-3*
*Plan: 05*
*Completed: 2026-08-27*
