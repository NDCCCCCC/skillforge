# Phase 18 Plan 03: Fan-Out Data-Channel + Monthly-Report Cases + D18-D2 Inheritance Summary

## One-liner

fan_out_dispatch data-channel variant (FanOutDispatchArgsV2 + fallback_sequential); 4 monthly-report frozen cases; FE report_published card + ApprovalCard third state + ChatInput pre-fill; D18-D2 prev_run_context injection into DeepAgent.Run + plan_approve.

## Commits

| Hash | Message |
|------|---------|
| `85d67bc` | feat(18-03): fan_out_dispatch data-channel variant — FanOutDispatchArgsV2 + validation + fallback_sequential |
| `66914c1` | feat(18-03): freeze 4 monthly-report fan-out cases — rpt-fanout-01..04 + fanout.lock.json metadata update |
| `35d7b81` | feat(18-03): FE — report_published card + ApprovalCard third state + ChatInput pre-fill |
| `700c8e8` | feat(18-03): ChatInput pre-fill event listener — chat-input-prefill → pendingDraft |
| `63ed8cb` | feat(18-03): D18-D2 prev_run_context injection — PrevRunContext + plan_approve inheritance |

## Tasks Completed

### Task 1: fan_out_dispatch Data-Channel Variant — ArgsV2 + Validation + fallback

**internal/agent/fanout_tools.go** — FanOutDispatchArgsV2 data-channel variant:
- `FanOutDispatchArgsV2` struct: `ShouldFanOut`, `NumSubtasks`, `TaskBoundaries[]`, `Reason`
- `fallbackReason` constants: `invalid_json`, `missing_field`, `boundary_not_in_template`, `num_subtasks_mismatch`
- `validateTaskBoundaries()`: exact-match against template section titles
- `dataChannelFanOutTool` struct + `BuildFanOutDispatchToolDataChannel()` factory
- `InvokableRun`: JSON parse → field presence → boundary validation → num_subtasks → parallel fan-out via `errgroup.WithContext` (limit=4) → structured JSON result
- fallback returns `{"status":"fallback_sequential","reason":"<enum>"}` — non-error, LLM continues sequentially

**Verification**: `go build ./internal/agent/...` OK; `go test ./internal/agent/... -count=1` OK; `grep -c "fallback_sequential\|FanOutDispatchArgsV2" internal/agent/fanout_tools.go` = 8

### Task 2: Golden Set Monthly-Report Cases — fanout.jsonl + fanout.lock.json

**eval/golden/fanout.jsonl** — 4 cases appended:
- `rpt-fanout-01`: 4 sections, 12 KPIs — full fan-out
- `rpt-fanout-02`: 2 sections — comparison query
- `rpt-fanout-03`: 1 section — fallback sequential expected
- `rpt-fanout-04`: 3 sections — partial scope

**eval/frozen/fanout.lock.json** — metadata updated:
- `case_count`: 24
- `category_counts`: added `monthly_report: 4`
- `thresholds`: added `monthly_report_fanout_rate_min: 0.95`
- `metrics`: added `monthly_report_fanout_rate_actual`, `monthly_report_fanout_rate_min`, `glm_drift_rate`
- `lock_id`: `fanout-2026-08-28`

**Verification**: `grep -c "rpt-fanout" eval/golden/fanout.jsonl` = 4; `grep -c "monthly_report" eval/frozen/fanout.lock.json` = 4; `go test ./internal/eval/... -count=1` OK

### Task 3: FE — MessageBubble report_published Card + ApprovalCard Third State

**web/src/types/wire-types.ts**:
- `ReportCard` interface: `period`, `version`, `url`, `sections[]`
- `report_published` chunk type added to `SkillForgeChunk` union

**web/src/components/chat/MessageBubble.tsx**:
- `report_published` case in `renderPart` switch: renders period badge + structure summary + view report link + 带修改重生成 chip
- View report link: `target="_blank" rel="noopener noreferrer"`
- Regenerate chip: dispatches `chat-input-prefill` custom event with pre-fill text

**web/src/store/app-store.ts**:
- `ApprovalStatus` + `TerminalApproval.status`: added `change_requested`

**web/src/components/ApprovalCard.tsx**:
- `requestChangeOpen`, `requestChangeReason` state + `handleRequestChange` callback
- `change_requested` button in amber (`--color-status-needs-info`) for `report:publish` targets only
- `report:publish` structure summary: section titles + KPI names + template version sha256 prefix
- `change_requested` terminal status config: amber border/badge, `已要求修改` text

**web/src/components/PlanApprovalCard.tsx**:
- `change_requested` terminal status config added

**Verification**: `cd web && npx tsc --noEmit` OK; `grep -c "report_published\|ReportCard" MessageBubble.tsx` = 3; `grep -c "request_change\|change_requested" ApprovalCard.tsx` = 7

### Task 4: ChatInput Pre-Fill Event Listener

**web/src/components/chat/ChatInput.tsx**:
- `useEffect` listens for `chat-input-prefill` document event
- Handler writes to `pendingDraft` store; existing `useEffect` consumes it and pre-fills input
- Bridges MessageBubble's 带修改重生成 chip to ChatInput pre-fill

**Verification**: `cd web && npx tsc --noEmit` OK; `grep -c "chat-input-prefill" ChatInput.tsx` = 3

### Task 5: D18-D2 prev_run_context Injection — PrevRunContext + plan_approve Inheritance

**internal/runtime/factory.go**:
- `PrevRunContext` struct: `Period`, `TemplateVersion`, `PrevVersion`, `ModNotes`
- `DispatchConfig.PrevRunContext` field (optional)

**internal/runtime/deep_agent.go**:
- `BuildDeepAgent`: when `cfg.PrevRunContext != nil`, appends inherited context paragraph to system prompt
- Paragraph includes: previous version, period, template version, modification notes, plus instruction to modify report and re-fetch KPI data

**internal/agent/hitl/hitl_plan_approve.go**:
- `PrevRunContext` struct added to `hitl` package
- `PlanApproveArgs.PrevRunContext` field (optional)
- `buildPlanApproveReason()`: includes "Based on previous run: v{N} (period=X, template=Y)" in audit Reason when `PrevRunContext` non-nil

**internal/agent/hitl/threat_mitigations_test.go**:
- Updated `P123DeepAgentMaxIterationSHA256` constant: `d0ab6d81...` (MaxIteration block shifted from lines 93-98 to 106-111 due to D18-D2 injection)
- Updated line range comment: 93-98 → 106-111

**Verification**: `go build ./internal/runtime/... ./internal/agent/...` OK; `go test ./internal/runtime/... ./internal/agent/hitl/... -count=1` OK; `grep -c "PrevRunContext\|prev_run_context" internal/runtime/deep_agent.go` = 3; `grep -c "PrevRunContext\|prevRunContext" internal/agent/hitl/hitl_plan_approve.go` = 6

## Verification Summary

| Check | Result |
|-------|--------|
| `go build ./internal/agent/... ./internal/eval/... ./internal/runtime/...` | PASS |
| `go test ./internal/agent/... ./internal/eval/... ./internal/runtime/... -count=1` | PASS (all packages OK) |
| `cd web && npx tsc --noEmit` | PASS |
| `grep -c "fallback_sequential\|FanOutDispatchArgsV2" internal/agent/fanout_tools.go` | 8 |
| `grep -c "rpt-fanout" eval/golden/fanout.jsonl` | 4 |
| `grep -c "monthly_report" eval/frozen/fanout.lock.json` | 4 |
| `grep -c "report_published\|ReportCard" web/src/components/chat/MessageBubble.tsx` | 3 |
| `grep -c "request_change\|change_requested" web/src/components/ApprovalCard.tsx` | 7 |
| `grep -c "chat-input-prefill" web/src/components/chat/ChatInput.tsx` | 3 |
| `grep -c "PrevRunContext\|prev_run_context" internal/runtime/deep_agent.go` | 3 |
| `grep -c "PrevRunContext\|prevRunContext" internal/agent/hitl/hitl_plan_approve.go` | 6 |

## Files Created/Modified

| File | Change |
|------|--------|
| `internal/agent/fanout_tools.go` | 175 lines added: FanOutDispatchArgsV2 + dataChannelFanOutTool |
| `eval/golden/fanout.jsonl` | 4 JSONL lines appended |
| `eval/frozen/fanout.lock.json` | metadata updated (case_count, category_counts, thresholds, metrics, lock_id) |
| `web/src/types/wire-types.ts` | ReportCard interface + report_published chunk type |
| `web/src/components/chat/MessageBubble.tsx` | report_published case in renderPart |
| `web/src/components/ApprovalCard.tsx` | request_change state + button + report:publish structure summary |
| `web/src/components/PlanApprovalCard.tsx` | change_requested terminal status |
| `web/src/store/app-store.ts` | change_requested added to ApprovalStatus + TerminalApproval |
| `web/src/components/chat/ChatInput.tsx` | chat-input-prefill event listener useEffect |
| `internal/runtime/factory.go` | PrevRunContext struct + DispatchConfig.PrevRunContext field |
| `internal/runtime/deep_agent.go` | prev_run_context injection into system prompt |
| `internal/agent/hitl/hitl_plan_approve.go` | PrevRunContext + PlanApproveArgs.PrevRunContext + buildPlanApproveReason() |
| `internal/agent/hitl/threat_mitigations_test.go` | P123 SHA256 constant + line range updated |

## Self-Check: PASSED

All 5 tasks committed with proper commit messages. Build, tests, TypeScript compilation, and grep criteria all verified.
