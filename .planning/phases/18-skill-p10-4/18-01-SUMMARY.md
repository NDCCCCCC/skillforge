# Phase 18 Plan 01: Skill System - Three Guards + Report Package Summary

## One-liner

Three-guard deep+data ban unlocked; internal/report rendering sandbox skeleton created; monthly-report Skill with KPI-validated template loaded.

## Commits

| Hash | Message |
|------|---------|
| `ed643be` | feat(18): unlock three guards — deep+data conditional allow (D18-A4) |
| `8ea3c2e` | feat(18): monthly-report SKILL.md + loader KPI registry cross-validation |

## Tasks Completed

### Task 1: Three Guards — validate.go / runner_setup.go / deep_agent.go

**validate.go** — deep+data ban changed to conditional allow:
- Lines 121-135: deep+data allowed ONLY when `writes` contains `"report:publish"` AND `Template != nil`
- Error message updated to reference D18-A4 and `"report:publish"`

**deep_agent.go** — `deepArchetypePrefix()` now branches:
- Data channel (`HasDataChannel() && Template != nil`): `kpi_call + fan_out_dispatch + report_publish + plan_approve + write_todos`
- Tier channel: original `retrieve_compliance_chunks` promise unchanged

**runner_setup.go** — data-channel tool set expanded from `{kpi_call}`:
- Added `report_publish` when writes has `"report:publish"`
- Added `plan_approve` when writes has `"plan_approve"`
- Added `fan_out_dispatch` when `Template != nil`

**hitl/registry.go** — `report:publish` added to `AllowedWriteTargets` (4th entry) and `hitlHandlers` registered

**hitl_report_publish.go** — new HITL tool:
- Two-pass contract: validate+interrupt → approval → publish
- Delegates to `report.Publish()` after operator approval
- Tool name: `report_publish`, target: `report:publish`

**agenttools/report_publish_tool.go** — `BuildReportPublishTool()` exported

**manifest.go** — `Template any` field added (YAML tag, validated at load time)

**Test updates**: validate_test.go, registry_test.go, threat_mitigations_test.go

### Task 2: internal/report Package Skeleton

| File | Lines | Purpose |
|------|-------|---------|
| `template.go` | 52 | `ReportTemplate`/`SectionTemplate`/`Slot` types + `Validate()` |
| `format.go` | 97 | `ValueFormatter` (千分位/percent/markdown escaping) + `CheckNarrativeConsistency` |
| `render.go` | 97 | `Render()` pipeline: bind→goldmark(GFM+CJK)→bluemonday(UGCPolicy) |
| `publish.go` | 112 | `Publish()` with vN increment, `os.WriteFile`, `PublishResult` |

**Key properties locked:**
- goldmark uses `WithExtensions(GFM, CJK)` — NO `html.WithUnsafe()`
- bluemonday uses `UGCPolicy().AllowTables()` — built via `sync.OnceValue`
- Pipeline order structurally locked: bind→goldmark→bluemonday (never reordered)
- `CheckNarrativeConsistency` returns `*ConsistencyReport` with mismatches; Render adds HTML comment markers (non-blocking per D18-A3 "标记" semantics)

### Task 3: monthly-report SKILL.md + loader.go KPI Validation

**SKILL.md** (`examples/skills/monthly-report/SKILL.md`):
- `agent: deep`, `reads: [data:aggregated_only]`, `writes: [report:publish, plan_approve]`
- `template:` with 4 sections, slot types restricted to `{narrative, insight, risk_note}`
- Uses only KPIs present in `kpis/testdata.yaml`: `KPI_LOAN_BALANCE`, `KPI_NPL_RATIO`, `KPI_INTEREST_INCOME`, `KPI_CUSTOMER_COUNT`

**loader.go** — `ValidateAgainstKPI` extended:
- `validateTemplateKPIs()` helper walks `template.sections[].kpis[]` via type-assertion on `any` (no skill→report import cycle)
- Error format: `skill %q template.sections[%d].kpis: unknown KPI %q`

**examples_validate_test.go** — skill count 5→6

## Deviations from Plan

### Rule 2 (Auto-add missing critical functionality)

**Import cycle resolved**: `Manifest.Template` stored as `any` (not `*report.ReportTemplate`) to break the `hitl→skill→hitl` cycle. Type-assertion happens at the `validateTemplateKPIs` call site in loader.go and at the HITL tool's `publishReport` method.

### Test Assertion Updates

- `validate_test.go`: `"deep_agent.go"` substring replaced with `"report:publish"` and `"template:"` (error message changed from "hardcoded retrieve promise" to conditional-allow description)
- `registry_test.go`: `KnownTargets()` expected slice extended to include `"report:publish"` (sorted lexicographically between `plan_approve` and `review_report`)
- `threat_mitigations_test.go`: `AllowedWriteTargets` expected slice extended
- `examples_validate_test.go`: skill count 5→6 (added monthly-report)

## Build Verification

```
go build ./internal/skill/... ./internal/agent/... ./internal/runtime/... ./internal/report/...  # PASS
go test ./internal/skill/... ./internal/agent/... ./internal/runtime/... ./internal/report/... -count=1  # PASS
grep -c "report:publish" internal/skill/validate.go internal/runtime/deep_agent.go  # 4 + 1
grep -c "fan_out_dispatch" internal/agent/runner_setup.go  # 2
grep -c "template.sections\[.*\].kpis.*unknown KPI" internal/skill/loader.go  # 1
```

## Threat Flags

None — all new surface (report_publish HITL tool, internal/report package) is within the plan's threat model scope (D18-A4 PATTERNS.md).

## Self-Check

- [x] All files created exist
- [x] All commits found via `git log --oneline`
- [x] validate.go conditional-allow logic: PASS (4 occurrences of `report:publish`)
- [x] deep_agent.go data-channel variant: PASS (1 occurrence of `kpi_call` in data variant)
- [x] runner_setup.go fan_out_dispatch: PASS (2 occurrences)
- [x] loader.go KPI validation grep matches: PASS
- [x] internal/report goldmark+bluemonday pipeline: PASS (14 matches)
- [x] internal/report publish vN logic: PASS (os.WriteFile + os.MkdirAll found)

## Self-Check: PASSED
