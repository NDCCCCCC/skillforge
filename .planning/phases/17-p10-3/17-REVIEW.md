---
phase: 17-p10-3
reviewed: 2026-08-28T10:52:00Z
depth: standard
files_reviewed: 44
files_reviewed_list:
  - internal/kpi/define.go
  - internal/kpi/executor.go
  - internal/kpi/executor_test.go
  - internal/kpi/loader.go
  - internal/kpi/multi_period.go
  - internal/kpi/multi_period_test.go
  - internal/kpi/registry.go
  - internal/kpi/validate_text.go
  - internal/kpi/validate_text_test.go
  - internal/agent/tools/kpi_tool.go
  - internal/agent/tools/kpi_tool_test.go
  - internal/agent/sql_surface_test.go
  - internal/agent/tools_for_channel_test.go
  - internal/wire/chunk.go
  - internal/wire/chunk_test.go
  - internal/skill/loader.go
  - internal/skill/loader_kpi_test.go
  - internal/skill/examples_validate_test.go
  - internal/eval/kpi_metrics.go
  - internal/eval/kpi_metrics_test.go
  - cmd/eval/main.go
  - cmd/server/main.go
  - internal/server/chat.go
  - internal/store/sqlite/kpi_calls_test.go
  - web/src/types/wire-types.ts
  - web/src/components/chat/MessageBubble.tsx
  - web/src/components/chat/ChatInput.tsx
  - web/src/components/chat/parts/Sparkline.tsx
  - web/src/components/chat/parts/Sparkline.test.tsx
  - web/src/components/chat/parts/DataReferenceChip.tsx
  - web/src/components/chat/parts/DataReferenceChip.test.tsx
  - web/src/components/chat/parts/DataReferencePopup.tsx
  - web/src/components/chat/parts/DrillChip.tsx
  - web/src/components/chat/parts/DrillChip.test.tsx
  - web/src/store/app-store.ts
  - web/src/transport/RealChatTransport.ts
  - web/src/App.tsx
  - web/vitest.config.ts
  - eval/golden/kpi.jsonl
  - eval/frozen/kpi.lock.json
  - eval/frozen/reid.jsonl
  - examples/skills/kpi-dashboard/SKILL.md
  - skills/kpi-dashboard/SKILL.md
  - kpis/testdata.yaml
  - Makefile
  - scripts/verify-phase-17.sh
  - scripts/uat_split_testdata.py
findings:
  critical: 1
  warning: 5
  info: 4
  total: 10
status: issues_found
---

## Summary

Phase 17 implements the KPI Dashboard Skill (P10.3) — a substantial feature adding multi-period KPI calls, sparkline rendering, and drill-down UX. The Go KPI engine (executor/loader/registry) is well-structured with proper fail-closed validation, but one correctness bug in `CallMultiPeriod` ignores the dataset-level k_min override, and several quality issues were found in the eval infrastructure and data-reference popup logic.

## Critical Issues

### CR-01: CallMultiPeriod ignores dataset-level k_min override

**File:** `internal/kpi/executor.go:430`

**Issue:** In `CallMultiPeriod`, k_min is resolved as `ResolveKMin(k.FileKMin, 0)` — the dataset-level override is hardcoded to `0`, ignoring the per-period dataset override entirely. By contrast, the single-period `Call` at line 308 correctly uses `ResolveKMin(k.FileKMin, version.KMinOverride)`.

**Failure scenario:** A dataset with `k_min_override=10` (stricter than the global default of 5) will have k_min=5 applied to multi-period sparkline calls instead of k_min=10, potentially exposing small buckets in trend data that should have been suppressed.

**Fix:**
```go
// BEFORE (line 430):
kmin := ResolveKMin(k.FileKMin, 0) // dataset-level override not available here; use file default

// AFTER — use per-period version's KMinOverride:
periodVersion, _ := e.meta.VersionForPeriod(ctx, k.Dataset, periods[0])
kmin := ResolveKMin(k.FileKMin, periodVersion.KMinOverride)
```

---

## Warnings

### WR-01: Duplicate condition in containsASOf

**File:** `internal/eval/kpi_metrics.go:118-120`

**Issue:** `截至` appears twice in the `containsASOf` check:
```go
return strings.Contains(text, "截至") ||
    strings.Contains(text, "截至") ||  // duplicate
    strings.Contains(text, "截止")
```

**Fix:** Remove the duplicate.

---

### WR-02: AllowedExpressionsMap checked but never used

**File:** `internal/eval/kpi_metrics.go:93-97`

**Issue:** `getAllowedForKPI` is defined (line 125) but never called. The `AllowedExpressionsMap` field is checked for existence (line 94) but the per-KPI lookup is bypassed — the code falls through to the shared `AllowedExpressions` list instead:
```go
if len(c.AllowedExpressionsMap) > 0 {
    allowed = c.AllowedExpressions  // BUG: should be getAllowedForKPI(c, matchedKPIName)
}
```
For multi-KPI golden cases with per-KPI allowed expressions (e.g. kpi-031 through kpi-040), the wrong allowlist is used.

**Fix:** After matching a number to a KPI name, call `getAllowedForKPI(c, matchedKPIName)` to get the per-KPI expression list.

---

### WR-03: KPI eval snapRows are always empty — GEVL-10 numeric validation is structurally dead code

**File:** `cmd/eval/main.go:381-384`

**Issue:** The comment states "In production, snapRows would be populated from the KPI registry" but the implementation leaves `snapRows` as an empty slice. `JudgeKPIReferenceConsistency` Step 3 (numeric match against snapshot rows) always passes for the snapshot branch because there are no rows to check. The `ExpectedValue` and `AllowedExpressions` fields in the golden cases are used for the fallback allowlist check, but if the answer contains a number that matches neither the snapshot (empty) nor the allowlist, it fails incorrectly.

**Fix:** Populate `snapRows` from the DuckDB engine or load pre-computed snapshot values into the eval runner.

---

### WR-04: KPI type/schema mismatch — CallMultiPeriod sets "period" but KPI SQL expects ":periods"

**File:** `kpis/testdata.yaml:97-103` + `internal/kpi/executor.go:454-464`

**Issue:** `KPI_LOAN_BALANCE_TREND` declares `periods` as a `ParamTypeArray` with SQL `WHERE period = ANY(:periods)`. However, `CallMultiPeriod` at line 454 builds `periodParams["period"] = period` (singular string), and `buildNamedArgs` binds `period` (not `periods`). The SQL template references `:periods` but the bound argument is `period`.

**Note:** This KPI is not in the routing table of `kpi-dashboard/SKILL.md`, so the bug is dormant for the Phase 17 skill. However, if any future Skill routes to `KPI_LOAN_BALANCE_TREND`, it will receive a DuckDB binding error.

**Fix:** Either (a) change `CallMultiPeriod` to build `periodParams["periods"] = []string{period}` and keep the array SQL, or (b) change the KPI SQL to use `:period` (singular) and rely on the per-period loop. Option (b) is cleaner for sparkline use cases.

---

### WR-05: Evaluate whether validateParams should re-check after CallMultiPeriod builds periodParams

**File:** `internal/kpi/executor.go:397-398`

**Issue:** `validateParams` is called once before `ResolvePeriods`, but `CallMultiPeriod` mutates `periodParams` by adding a `period` key (string) for a KPI that declared `periods` (array). Since `validateParams` checks for extra undeclared keys at line 626-641, a future KPI that declared `period` as a string param and received `periods` as an array would be caught. But the reverse case (declares `periods`, receives extra `period`) is not caught because `validateParams` only checks `declared` params.

This is latent: if a routing table routes a `periods`-array KPI through `CallMultiPeriod` and the executor builds `periodParams["period"]`, the extra `period` key is invisible to validation.

**Fix:** Document the contract clearly — `CallMultiPeriod` is only valid for KPIs with `ParamTypeString period` SQL templates (not `ParamTypeArray periods`), and add a runtime guard or update the routing logic accordingly.

---

## Info

### IN-01: Unused getAllowedForKPI function

**File:** `internal/eval/kpi_metrics.go:125-132`

The `getAllowedForKPI` function is defined but never called from `JudgeKPIReferenceConsistency`. It should be called after a KPI name is matched to use per-KPI allowed expressions in multi-KPI cases.

---

### IN-02: kpi.lock.json has unfilled placeholder fields

**File:** `eval/frozen/kpi.lock.json:4,38`

`prompt_hash` and `lock_sha256` contain `<fill: ...>` placeholder strings. If `--frozen=kpi` runs in CI, `computeKPIRegistrySHA` passes but `prompt_hash` would never match. This means the eval runner will always fail the pin assertion for `prompt_hash` and `lock_sha256` unless these are filled in manually before the eval runs.

**Note:** This is acceptable for a development-phase lock file, but the comment in the file is misleading — it implies a `--dry-run` flag populates these, but that flag does not exist in the current `cmd/eval/main.go`.

---

### IN-03: Dual bootstrap in runKPIMode when skillName=="kpi"

**File:** `cmd/eval/main.go:490-502`

When `*frozen == "kpi"`, the code builds an inner bootstrap (emb, r, manifest, cm, ag) that is immediately discarded because `runKPIMode` performs its own bootstrap. The outer bootstrap's `manifest := firstManifest()` loads skills/ and returns the alphabetically-first skill (compliance, not kpi-dashboard), which is also unused.

This is not a correctness issue (the inner agent is discarded) but wastes bootstrap time and creates confusing dead code.

---

### IN-04: TestNoSQLGenerationSurface scans kpi-dashboard skill path via filepath.WalkDir

**File:** `internal/agent/sql_surface_test.go:411-456`

`TestDrillDownNeverReachesRaw` walks `examples/skills/kpi-dashboard/` looking for SQL keywords in `.go` files. But `examples/skills/kpi-dashboard/` contains only `SKILL.md` (no `.go` files), so the walk always finds zero hits. The test is structurally correct but exercises no code path for this specific skill. Additionally, it excludes `skills/kpi-dashboard/` (the production-deployed copy) — only scanning the examples directory.

---

## D17 Decision Compliance Check

| Decision | Status | Notes |
|----------|--------|-------|
| D17-A1: Explicit routing table in Skill steps | PASS | Fenced YAML routing table parsed correctly in `parseRoutingTable` |
| D17-A2: Single kpi-dashboard Skill | PASS | Both `examples/skills/kpi-dashboard/` and `skills/kpi-dashboard/` have identical SKILL.md |
| D17-B1: Pure SVG sparkline, zero dependencies | PASS | `Sparkline.tsx` uses only `useMemo`, no chart library |
| D17-B2: data-reference References[] array | PASS | `wire/chunk.go` + `wire-types.ts` both have `PointReference[]` |
| D17-B3: periods[] array param, CallMultiPeriod | PARTIAL | Schema correct, but k_min override ignored (CR-01) |
| D17-C1: DrillChip → setPendingDraft → ChatInput useEffect | PASS | `app-store.ts` has `pendingDraft` field and `setPendingDraft` action |
| D17-D2: 3-step numeric match | PARTIAL | Steps 1-2 correct; Step 3 has structural dead code (snapRows empty, WR-03) |
| D17-D4: Independent kpi.lock.json | PASS | 6-field lock with `kpi_registry_sha256` |

---

## Structural Findings (fallow)

The following were identified in prior structural analysis and are confirmed correct in the implementation:

- `wire/chunk.go` `DataReference.References[]` is a field extension (not a new UIMessageChunk variant) — matches D17-B2 and preserves D15 wire-lockstep invariant
- `internal/skill/loader.go` `ValidateAgainstKPI` correctly cross-checks routing table KPI names against registry at boot time
- `TestKPINarrowSurface` correctly whitelists both `Call` and `CallMultiPeriod` as the only executor entry points
- `TestNoSQLGenerationSurface` correctly excludes `_test.go` and `sql_surface_test.go` itself from the scan

---

_Reviewed: 2026-08-28T10:52:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
