# Phase 18: 业务月报 Skill（P10.4） - Pattern Map

**Mapped:** 2026-08-28
**Files analyzed:** 25
**Analogs found:** 22 / 25

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/agent/hitl/hitl_report_publish.go` | handler | request-response | `internal/agent/hitl/hitl_review_report.go` | exact |
| `internal/agent/hitl/registry.go` | config | request-response | `internal/agent/hitl/registry.go` | exact |
| `internal/store/sqlite/reports.go` | store | CRUD | `internal/store/sqlite/kpi_calls.go` | exact |
| `internal/store/sqlite/schema.go` | config | CRUD | `internal/store/sqlite/schema.go` | exact |
| `internal/skill/manifest.go` | model | config | `internal/skill/manifest.go` | exact |
| `internal/skill/validate.go` | service | request-response | `internal/skill/validate.go` | exact |
| `internal/skill/loader.go` | service | file-I/O | `internal/skill/loader.go` | exact |
| `internal/agent/fanout_tools.go` | tool | request-response | `internal/agent/fanout_tools.go` | exact |
| `internal/runtime/deep_agent.go` | service | request-response | `internal/runtime/deep_agent.go` | exact |
| `internal/agent/runner_setup.go` | service | request-response | `internal/agent/runner_setup.go` | exact |
| `internal/wire/chunk.go` | model | request-response | `internal/wire/chunk.go` | exact |
| `web/src/components/ApprovalCard.tsx` | component | event-driven | `web/src/components/ApprovalCard.tsx` | exact |
| `web/src/components/chat/MessageBubble.tsx` | component | event-driven | `web/src/components/chat/MessageBubble.tsx` | exact |
| `eval/frozen/fanout.lock.json` | config | batch | `eval/frozen/fanout.lock.json` | exact |
| `eval/golden/fanout.jsonl` | config | batch | `eval/golden/fanout.jsonl` | exact |
| `examples/skills/monthly-report/SKILL.md` | config | file-I/O | `examples/skills/compliance/SKILL.md` | role-match |
| `internal/report/template.go` | model | config | `internal/skill/manifest.go` | role-match |
| `internal/report/format.go` | utility | transform | (no direct analog — new logic) | no-analog |
| `internal/report/render.go` | service | transform | (new — goldmark+bluemonday pipeline) | no-analog |
| `internal/report/publish.go` | service | file-I/O | `internal/store/sqlite/kpi_calls.go` | role-match |

---

## Pattern Assignments

### `internal/agent/hitl/hitl_report_publish.go` (handler, request-response)

**Analog:** `internal/agent/hitl/hitl_review_report.go` (lines 1-237)

**Imports pattern** (lines 30-40):
```go
import (
    "context"
    "encoding/gob"
    "encoding/json"
    "errors"
    "fmt"

    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)
```

**Two-pass HITL contract** (lines 157-183 = Pass 1; lines 185-232 = Pass 2):
```go
// Pass 1 — InsertPendingApproval BEFORE interrupt (fail-closed D11-C3)
wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)
if !wasInterrupted {
    approvalID := compose.GetToolCallID(ctx)
    if approvalID == "" {
        approvalID = newApprovalID()
    }
    info := &ApprovalInfo{
        ToolName:        ReportPublishToolName, // "report_publish_write"
        ArgumentsInJSON: args,
        ToolCallID:      approvalID,
        Target:          ReportPublishTarget, // "report:publish"
    }
    if err := h.auditOrDefault().InsertPendingApproval(ctx, approvalID, info); err != nil {
        return "", fmt.Errorf("report_publish: audit INSERT failed (fail-closed): %w", err)
    }
    return "", tool.StatefulInterrupt(ctx, info, args)
}

// Pass 2 — validate decision, then deny / approve / request_change
isTarget, hasData, data := tool.GetResumeContext[*ApprovalDecision](ctx)
// ... validation ...
if data.Decision == "deny" {
    return fmt.Sprintf("user denied: %s", data.Reason), nil
}
if data.Decision == "request_change" {
    return fmt.Sprintf("change requested: %s", data.Reason), nil // D18-D4 new branch
}
// approve path: Render() + Publish() + return URL+version
```

**Differences from hitl_review_report.go:**
- Args struct: `ReportPublishArgs {Title, Period, Sections[], KPICallIDs[]}` — structural summary for D18-B4
- Pass 2 approve: calls `internal/report.Render + Publish` (real副作用 vs review_report stub)
- Pass 2 adds `request_change` third-state branch (D18-D4)
- `ReportPublishTarget = "report:publish"` (adds to AllowedWriteTargets)

**Error handling pattern** (lines 200-204):
```go
if err := data.Validate(); err != nil {
    return "", fmt.Errorf("report_publish: invalid ApprovalDecision: %w", err)
}
```

---

### `internal/agent/hitl/registry.go` (config, request-response)

**Analog:** `internal/agent/hitl/registry.go` (lines 1-157)

**Append point for `report:publish`** (lines 69-74):
```go
var AllowedWriteTargets = []string{
    "memory:facts",  // 11-02: first write target
    "review_report", // 11-04: second write target
    "plan_approve",  // 12-03: third write target
    "report:publish", // 18-XX: fourth write target — 月报发布 HITL 门 (RPT-04)
    "skill_state",   // deprecated placeholder
}
```

**Handler map entry** (lines 96-101):
```go
var hitlHandlers = map[string]tool.InvokableTool{
    "memory:facts":  &MemoryFactsTool{},
    "review_report": &reviewReportTool{},
    "plan_approve":  &planApproveTool{},
    "report:publish": &reportPublishTool{}, // 18-XX — hitl_report_publish.go
    "skill_state":   nil,
}
```

**Comment for append point** (line 44-47):
```go
// 3. PHASE 12 plan_approve add-point — appending "plan_approve" to
//    this slice AND registering &planApproveTool{} in hitlHandlers
//    (Plan 12-03 / PLAN-03 / D11-B3) is the only change required to
//    add a new write target. NO other file in the codebase needs touching.
```

---

### `internal/store/sqlite/reports.go` (store, CRUD)

**Analog:** `internal/store/sqlite/kpi_calls.go` (lines 1-207)

**INSERT-only SQL** (lines 38-43):
```go
const putReportSQL = `
INSERT INTO reports (
    ts, period, skill_id, version, title,
    file_path, approval_audit_id, kpi_call_ids, template_version, result_digest
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
```

**PutReport signature** (lines 49-65):
```go
func (s *SQLiteStore) PutReport(ctx context.Context, row store.ReportRow) error {
    _, err := s.db.ExecContext(ctx, putReportSQL,
        row.TS.UTC().Format(time.RFC3339Nano),
        row.Period, row.SkillID, row.Version, row.Title,
        row.FilePath, row.ApprovalAuditID, row.KPICallIDs,
        row.TemplateVersion, row.ResultDigest,
    )
    if err != nil {
        return fmtError("PutReport", err)
    }
    return nil
}
```

**SELECT pattern** (mirrors listKPICallsBaseSQL lines 75-91):
```go
const listReportsBaseSQL = `
SELECT id, ts, period, skill_id, version, title,
       file_path, approval_audit_id, kpi_call_ids, template_version, result_digest
FROM reports
`
// ... WHERE clause from fixed constants, ORDER BY id DESC LIMIT ?
```

---

### `internal/skill/manifest.go` (model, config)

**Analog:** `internal/skill/manifest.go` (lines 1-238)

**Add Template field to Manifest** (after line 66):
```go
type Manifest struct {
    Description    string     `yaml:"description"             json:"description"`
    Agent          string     `yaml:"agent"                   json:"agent"`
    Reads          []string   `yaml:"reads"                   json:"reads"`
    Writes         []string   `yaml:"writes"                 json:"writes"`
    Steps          string     `yaml:"steps"                   json:"steps"`
    Guardrails     Guardrails `yaml:"guardrails"              json:"guardrails"`
    Temperature    *float32   `yaml:"temperature"             json:"temperature"`
    DescriptionVec []float32  `yaml:"-"                      json:"-"`
    Name           string     `yaml:"-"                      json:"-"`
    resolvedTiers  []string
    // NEW: Phase 18 / D18-A1 — template: frontmatter block
    Template       *ReportTemplate `yaml:"template" json:"template"`
}
```

**ReportTemplate Go type** (new, suggested placement in `internal/report/template.go`):
```go
type ReportTemplate struct {
    Title    string            `yaml:"title"    json:"title"`
    Sections []SectionTemplate `yaml:"sections" json:"sections"`
}

type SectionTemplate struct {
    Title string   `yaml:"title" json:"title"`
    KPIs  []string `yaml:"kpis"  json:"kpis"`
    Slots []Slot   `yaml:"slots" json:"slots"`
}

type Slot struct {
    Type string `yaml:"type" json:"type"` // "narrative" | "insight" | "risk_note"
}
```

---

### `internal/skill/validate.go` (service, request-response)

**Analog:** `internal/skill/validate.go` (lines 1-209)

**D18-A4 fail-closed unlock: narrow the deep+data ban to conditional** (around lines 121-126):
```go
// EXISTING ban (lines 121-126) — REPLACE with narrow conditional:
if m.HasDataChannel() {
    // (a) Data channel sole reads entry (existing check)
    if len(m.Reads) > 1 { ... }
    // (b) MODIFIED: deep+data allowed ONLY when writes contains "report:publish" AND Template != nil
    if m.Agent == "deep" {
        if !slices.Contains(m.Writes, "report:publish") || m.Template == nil {
            return fmt.Errorf(
                "agent type %q is incompatible with data channel: deep archetype requires writes:[report:publish] + template: (D18-A4); use chat archetype instead",
                m.Agent,
            )
        }
    }
    // (c) Skip ResolveTiers (existing)
} else { ... }
```

**Template fail-closed validation** (new block after existing checks):
```go
// 9. template — fail-closed static校验 (D18-A4)
if m.Template != nil {
    if len(m.Template.Sections) == 0 {
        return fmt.Errorf("template.sections must be non-empty (D18-A4)")
    }
    for i, sec := range m.Template.Sections {
        if strings.TrimSpace(sec.Title) == "" {
            return fmt.Errorf("template.sections[%d].title is empty (D18-A4)", i)
        }
        for _, kpi := range sec.KPIs {
            // KPI名必须在registry内 — cross-check at boot after LoadKPIs (D18-A4)
            // ValidationAgainstKPI in loader.go handles the registry lookup
        }
        for j, slot := range sec.Slots {
            if !slices.Contains([]string{"narrative", "insight", "risk_note"}, slot.Type) {
                return fmt.Errorf("template.sections[%d].slots[%d].type %q not in {narrative, insight, risk_note} (D18-A2)", i, j, slot.Type)
            }
        }
    }
}
```

---

### `internal/skill/loader.go` (service, file-I/O)

**Analog:** `internal/skill/loader.go` (lines 1-335)

**ValidateAgainstKPI extension for template KPI names** (around lines 164-184):
```go
// NEW: cross-check template KPI names against registry
func (l *Loader) ValidateAgainstKPI(reg *kpi.Registry) error {
    manifests := l.Skills()
    for _, m := range manifests {
        // existing data-channel routing table check
        if !m.HasDataChannel() {
            continue
        }
        // ... parseRoutingTable existing ...

        // NEW: check template KPI names
        if m.Template != nil {
            for si, sec := range m.Template.Sections {
                for _, kpi := range sec.KPIs {
                    if _, ok := reg.Lookup(kpi); !ok {
                        return fmt.Errorf("skill %q template.sections[%d].kpis: unknown KPI %q", m.Name, si, kpi)
                    }
                }
            }
        }
    }
    return nil
}
```

---

### `internal/agent/fanout_tools.go` (tool, request-response)

**Analog:** `internal/agent/fanout_tools.go` (lines 1-195)

**FanOutDispatchArgsV2 for data channel** (new struct, appended to file):
```go
// FanOutDispatchArgsV2 is the data-channel fan-out classifier args.
// Replaces FanOutSubQuery/TierFilter with section-boundary model.
// D18-C1: per-章节 fan-out boundary 由模板定义.
type FanOutDispatchArgsV2 struct {
    ShouldFanOut   bool     `json:"should_fan_out"`
    NumSubtasks    int      `json:"num_subtasks"`
    TaskBoundaries []string `json:"task_boundaries"` // 模板 section title 逐字匹配
    Reason         string   `json:"reason"`           // 分类理由 — 进审计行 (FAN-05)
}
```

**Validation: task_boundaries must be in template sections** (mirrors lines 187-192 tier validation):
```go
for _, boundary := range args.TaskBoundaries {
    if !slices.Contains(templateSectionTitles, boundary) {
        return "", fmt.Errorf("fan_out_dispatch: task_boundary %q not in template sections %v (D18-C1)",
            boundary, templateSectionTitles)
    }
}
```

**Fallback: return structured result NOT error** (D18-C4 — non-error fallback):
```go
// If validation fails (invalid JSON / missing fields / boundary not in template):
return fmt.Sprintf(`{"status":"fallback_sequential","reason":"%s"}`, fallbackReason), nil
// LLM sees fallback signal and executes kpi_call sequentially — no noise in tool error
```

---

### `internal/runtime/deep_agent.go` (service, request-response)

**Analog:** `internal/runtime/deep_agent.go` (lines 1-201)

**deepArchetypePrefix data-channel variant** (around lines 167-177):
```go
// MODIFIED: add data-channel tool declaration (replaces hardcoded retrieve_compliance_chunks)
func deepArchetypePrefix() string {
    // Keep existing for tier channel; data channel variant:
    return `You are a SkillForge DeepAgent. You have access to:
- write_todos (Plan 12-02 — audited custom Tool)
- plan_approve (Plan 12-03 — HITL-gated approval Tool)
- kpi_call (read-only aggregated KPI data)
- report_publish (HITL-gated report publication — D18-B1)
MaxIteration = 10. Plan tightly: batched kpi_call calls, compact todos, no silent expansion.
`
}
```

**Dispatch by channel kind** — if `cfg.Manifest.HasDataChannel()` && `cfg.Manifest.Template != nil`:
use data-channel prefix; otherwise use tier-channel prefix.

---

### `internal/agent/runner_setup.go` (service, request-response)

**Analog:** `internal/agent/runner_setup.go` (lines 1-302)

**Data-channel toolset expansion** (around lines 237-249):
```go
// EXISTING data-channel branch (lines 238-248):
if cfg.Manifest.HasDataChannel() {
    // MODIFY: extend beyond just {kpi_call}
    kpiCallTool := agenttools.BuildKPICallTool(...)
    tools := []tool.BaseTool{kpiCallTool}

    // NEW: add report_publish + plan_approve if writes declares them
    if slices.Contains(cfg.Manifest.Writes, "report:publish") {
        // BuildToolsForSkill handles this via hitl.BuildToolsForSkill
        // But data channel skips BuildToolsForSkill — inject here
        reportPublishTool := agenttools.BuildReportPublishTool(...)
        tools = append(tools, reportPublishTool)
    }
    if slices.Contains(cfg.Manifest.Writes, "plan_approve") {
        planApproveTool := agenttools.BuildPlanApproveTool(...)
        tools = append(tools, planApproveTool)
    }
    // fan_out_dispatch if skill has template (D18-C1/C2)
    if cfg.Manifest.Template != nil {
        fanOutTool := agenttools.BuildFanOutDispatchTool(cfg.Manifest)
        tools = append(tools, fanOutTool)
    }
} else { ... }
```

**Note:** This expansion MUST NOT break existing Phase 16/17 tests asserting `data-channel Skill sees exactly {kpi_call}` — those assertions must be updated to parameterize on writes declarations.

---

### `internal/wire/chunk.go` (model, request-response)

**Analog:** `internal/wire/chunk.go` (lines 1-439)

**report_published DecisionPayload extension** (around lines 157-171):
```go
type DecisionPayload struct {
    Step           int             `json:"step"`
    Decision       string          `json:"decision"`
    // ... existing fields ...
    // NEW (D18-B3):
    ReportCard     *ReportCard     `json:"reportCard,omitempty"`
}

// NEW struct (D18-B3):
type ReportCard struct {
    Title    string   `json:"title"`
    Period   string   `json:"period"`
    Version  int      `json:"version"`
    URL      string   `json:"url"`       // /reports/2026-08/v1.html
    Versions []string `json:"versions,omitempty"` // 历史版本链接
    RegenerateHint string `json:"regenerateHint,omitempty"` // chip 预填文案
}
```

**Add to ValidTypes if needed** — `report_published` is a Decision value, not a chunk type, so no ValidTypes change needed (D17-B2 philosophy: reuse data-decision channel).

---

### `web/src/components/ApprovalCard.tsx` (component, event-driven)

**Analog:** `web/src/components/ApprovalCard.tsx` (lines 1-542)

**"要求修改" third state button** (around line 330-360 — add to button trio):
```tsx
{/* Button trio — extend with third state (D18-D4) */}
<div className="flex gap-2 pt-2">
  <button
    type="button"
    onClick={() => handleApprove(false)}
    disabled={inFlight !== null}
    className="px-3 py-1.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)] text-sm font-semibold"
    data-testid="approve-once"
  >
    {inFlight === 'approve' ? '批准一次...' : '批准一次'}
  </button>
  {/* ... approve-remember button ... */}
  <button
    type="button"
    onClick={() => setDenyPromptOpen(!denyPromptOpen)}
    disabled={inFlight !== null}
    className="px-3 py-1.5 rounded bg-[var(--color-danger)] text-white text-sm font-semibold"
    data-testid="deny-button"
  >
    拒绝
  </button>
  {/* NEW: 要求修改 third state (D18-D4) */}
  <button
    type="button"
    onClick={() => setRequestChangeOpen(!requestChangeOpen)}
    disabled={inFlight !== null}
    className="px-3 py-1.5 rounded bg-[var(--color-status-needs-info)] text-black text-sm font-semibold"
    data-testid="request-change-button"
  >
    要求修改
  </button>
</div>
```

**Request change prompt** (new block after deny prompt, lines ~409-448):
```tsx
{requestChangeOpen && (
  <div className="px-2 py-2 rounded border border-[var(--color-border-muted)] bg-[var(--color-surface-2)]"
       data-testid="request-change-prompt">
    <p className="text-xs text-[var(--color-text-muted)] mb-1">
      请填写修改意见（至少 3 个字符）
    </p>
    <textarea
      className="w-full h-20 px-2 py-1 text-sm bg-[var(--color-surface-3)] border border-[var(--color-border)] rounded"
      value={requestChangeReason}
      onChange={(e) => setRequestChangeReason(e.target.value)}
      placeholder="例如：更突出 Q2 风险..."
      data-testid="request-change-reason-input"
    />
    <div className="flex gap-2 mt-1">
      <button
        type="button"
        onClick={() => handleRequestChange()}
        disabled={requestChangeReason.trim().length < 3 || inFlight !== null}
        className="px-3 py-1.5 rounded bg-[var(--color-status-needs-info)] text-black text-sm font-semibold"
        data-testid="request-change-submit"
      >
        {inFlight === 'request_change' ? '确认修改...' : '确认修改'}
      </button>
    </div>
  </div>
)}
```

**handleRequestChange** (mirrors handleDeny pattern):
```tsx
const handleRequestChange = useCallback(async () => {
  const reason = requestChangeReason.trim();
  if (reason.length < 3) return;
  setInFlight('request_change');
  const status = await postDecision(sessionId, approval, {
    decision: 'request_change',
    reason,
    target: approval.target,
    approval_id: approval.approvalId,
  });
  // settle with status='change_requested'
}, [...]);
```

---

### `web/src/components/chat/MessageBubble.tsx` (component, event-driven)

**Analog:** `web/src/components/chat/MessageBubble.tsx` (existing data-decision branch)

**Report card rendering** (in data-decision switch — add case):
```tsx
// In the data-decision rendering switch:
// case 'report_published':
//   <ReportCardComponent payload={chunk.data} />
//   — links to /reports/2026-08/v1.html (new tab)
//   — "带修改重生成" chip → ChatInput pre-fill
```

---

### `eval/frozen/fanout.lock.json` (config, batch)

**Analog:** `eval/frozen/fanout.lock.json` (lines 1-49)

**Extension fields for monthly-report** (additive, lines 14-19 category_counts):
```json
{
  "lock_id": "fanout-2026-08-28",
  "model_version": "v0.9",  // incremented from v0.8
  "phase": 18,
  "phase_slug": "18-skill-p10-4",
  "case_count": 24,          // +4 monthly-report cases
  "category_counts": {
    "multi_hop": 8,
    "tier_mismatch_rejection": 6,
    "partial_failure_conservative_merge": 4,
    "single_vertex_fallback": 2,
    "monthly_report": 4      // NEW: D18-C3 FAN-06 fill
  },
  // ... existing thresholds ...
  // NEW monthly-report specific threshold:
  "thresholds": {
    // ... existing ...
    "monthly_report_fanout_rate_min": 0.95  // D18-C3: expected_fan_out consistency
  },
  // NEW metadata:
  "metrics": {
    // ... existing 13 metrics ...
    "monthly_report_fanout_rate_actual": 1.0,  // N=5 runs, all agreed fan-out
    "glm_drift_rate": 0.0                      // 5/5一致 = 0% drift
  }
}
```

---

### `eval/golden/fanout.jsonl` (config, batch)

**Analog:** `eval/golden/fanout.jsonl` (lines 1-20)

**Monthly-report cases to append** (new lines after 20):
```jsonl
{"id":"rpt-fanout-01","scenario":"monthly_report","parent_skill":"monthly-report",
 "query":"出 2026-08 月报","template_sections":["业务概览","贷款质量","风险提示","客户情况"],
 "expected_fan_out":true,"expected_num_subtasks":4,
 "expected_fallback":false,
 "notes":"4 sections, 12 KPIs — classifier should fan-out with boundary=template sections"}
{"id":"rpt-fanout-02","scenario":"monthly_report","parent_skill":"monthly-report",
 "query":"对比 2026-07 和 2026-08 月报","template_sections":["业务概览","贷款质量"],
 "expected_fan_out":true,"expected_num_subtasks":2,
 "expected_fallback":false,
 "notes":"2 sections — comparison query, both suitable for fan-out"}
{"id":"rpt-fanout-03","scenario":"monthly_report","parent_skill":"monthly-report",
 "query":"生成季度报告","template_sections":["业务概览"],
 "expected_fan_out":false,"expected_num_subtasks":1,
 "expected_fallback":true,
 "notes":"Single section — fallback sequential expected"}
{"id":"rpt-fanout-04","scenario":"monthly_report","parent_skill":"monthly-report",
 "query":"月报数据不全，只看贷款","template_sections":["业务概览","贷款质量","风险提示"],
 "expected_fan_out":true,"expected_num_subtasks":3,
 "expected_fallback":false,
 "notes":"3 sections — partial scope, classifier should still fan-out per template"}
```

---

### `examples/skills/monthly-report/SKILL.md` (config, file-I/O)

**Analog:** `examples/skills/compliance/SKILL.md` (lines 1-52)

**SKILL.md structure** (D18-A1 + D18-A2):
```yaml
---
description: 业务月报生成。当用户说「出月报」「2026-08 月报」「生成月度报告」时触发。触发短语：「出月报」「月报」「2026-08 月报」...
agent: deep
reads:
  - data:aggregated_only
writes:
  - report:publish
  - plan_approve
steps: |
  1. 解析用户请求的 period（如「2026-08」）。
  2. 调用 plan_approve 确认报告结构（章节 + KPI 集）。
  3. 使用 fan_out_dispatch 决定是否并行取数：
     - should_fan_out=true：每个章节并行调用 kpi_call 取数。
     - should_fan_out=false 或分类失败：顺序逐章节取数。
  4. 基于取数结果填写叙事槽位：
     - narrative：业务描述文字（LLM 自由叙事）。
     - insight：基于 KPI 值的分析点评（数字来自快照，非 LLM 产生）。
     - risk_note：风险提示，必须引用具体 KPI 值。
  5. 调用 report:publish 提交审批。
  6. 审批通过后发布报告，提供「带修改重生成」chip。
guardrails:
  prohibitions:
    - "Never write a number that is not server-injected from a KPI snapshot"
    - "Never SELECT raw rows or attempt SQL"
    - "Never overwrite an existing report version file"
    - "Never cite external knowledge outside the KPI data"
  required_refusal_phrases:
    - "period_not_found"
    - "insufficient_kpi_data"
temperature: 0.0
---

# 业务月报 Skill

业务月报生成工具。读取聚合数据（`data:aggregated_only`），生成固定结构的月度报告。

## 模板（template:）

月报模板定义报告结构和叙事槽位：

```yaml
template:
  title: "{period} 业务月报"
  sections:
    - title: 业务概览
      kpis: [KPI_LOAN_BALANCE, KPI_CUSTOMER_COUNT]
      slots:
        - type: narrative
        - type: insight
    - title: 贷款质量
      kpis: [KPI_NPL_RATIO, KPI_LOAN_LOSS_RATE]
      slots:
        - type: narrative
        - type: risk_note
    - title: 风险提示
      kpis: [KPI_NPL_RATIO]
      slots:
        - type: risk_note
    - title: 客户情况
      kpis: [KPI_NEW_CUSTOMER_COUNT, KPI_ACTIVE_CUSTOMER_RATE]
      slots:
        - type: narrative
        - type: insight
```

## 槽位填写规则

- **narrative**: LLM 自由撰写业务描述，不引用具体数字。
- **insight**: LLM 基于 KPI 快照值撰写分析点评；数字必须来自快照。
- **risk_note**: LLM 必须引用具体 KPI 数字（如「NPL 率 1.23%」），不得自由发挥。

## 发布

发布成功后聊天内显示报告卡片，含「查看报告」链接（新 tab 打开）和「带修改重生成」chip。
```

---

## Shared Patterns

### Authentication / Authorization
**Source:** `internal/agent/hitl/registry.go` (lines 25-47)
**Apply to:** All write targets

The AllowedWriteTargets slice is the single source of truth. Adding a new target requires:
1. Append to `AllowedWriteTargets` slice
2. Register handler in `hitlHandlers` map
3. No other files need modification (D11-B3 contract)

```go
var AllowedWriteTargets = []string{
    "memory:facts",
    "review_report",
    "plan_approve",
    "report:publish", // 18-XX: append here
    "skill_state",
}
```

### Error Handling
**Source:** `internal/agent/hitl/hitl_review_report.go` (lines 200-204)
**Apply to:** All HITL handlers

```go
if err := data.Validate(); err != nil {
    return "", fmt.Errorf("report_publish: invalid ApprovalDecision: %w", err)
}
```

### INSERT-only Store Pattern
**Source:** `internal/store/sqlite/kpi_calls.go` (lines 38-65)
**Apply to:** `internal/store/sqlite/reports.go`

```go
const putReportSQL = `
INSERT INTO reports (
    ts, period, skill_id, version, title,
    file_path, approval_audit_id, kpi_call_ids, template_version, result_digest
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
func (s *SQLiteStore) PutReport(ctx context.Context, row store.ReportRow) error {
    _, err := s.db.ExecContext(ctx, putReportSQL, ...)
    if err != nil { return fmtError("PutReport", err) }
    return nil
}
```

### Template Fail-Closed Validation
**Source:** `internal/skill/validate.go` (lines 50-193)
**Apply to:** `internal/skill/validate.go` (new block), `internal/skill/loader.go` (ValidateAgainstKPI extension)

Order of checks: cheap field-presence first, then domain rules. First failure short-circuits.

### deep+data Unlock Pattern (RESEARCH Pattern 3)
**Source:** `internal/skill/validate.go` (lines 121-126) + `internal/runtime/deep_agent.go` (lines 167-177) + `internal/agent/runner_setup.go` (lines 237-249)
**Apply to:** Three files must change together

```go
// validate.go: narrow ban to conditional
if m.Agent == "deep" && m.HasDataChannel() {
    if !slices.Contains(m.Writes, "report:publish") || m.Template == nil {
        return fmt.Errorf("...")
    }
}

// deep_agent.go: data-channel prefix variant
// runner_setup.go: extend data-channel tools beyond {kpi_call}
```

### Goldmark + Bluemonday Render Pipeline
**Source:** RESEARCH.md Pattern 1 (lines 475-499) — no direct codebase analog yet
**Apply to:** `internal/report/render.go`

```go
var md = goldmark.New(
    goldmark.WithExtensions(extension.GFM, extension.CJK),
    // 刻意不传 html.WithUnsafe() — raw HTML 默认转义是第一层防御
)
var policy = sync.OnceValue(func() *bluemonday.Policy {
    p := bluemonday.UGCPolicy()
    p.AllowElements("section")
    return p
})

func Render(tmpl *ReportTemplate, slots SlotTexts, snap *kpi.Snapshot, fmtFn ValueFormatter) ([]byte, error) {
    body := buildMarkdown(tmpl, slots, snap, fmtFn)           // 1. 绑定
    var buf bytes.Buffer
    if err := md.Convert([]byte(body), &buf); err != nil {    // 2. goldmark
        return nil, err
    }
    return policy().SanitizeBytes(buf.Bytes()), nil           // 3. bluemonday
}
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/report/format.go` | utility | transform | New KPI formatting logic (千分位/百分比/markdown cell escaping) — no prior art in codebase |
| `internal/report/render.go` | service | transform | goldmark+bluemonday pipeline — RESEARCH.md pattern confirmed via official docs, not yet in codebase |
| `internal/report/template.go` | model | config | Go struct for template: YAML block — mirrors manifest.go but specific to report template |

---

## Metadata

**Analog search scope:** `internal/agent/hitl/`, `internal/store/sqlite/`, `internal/skill/`, `internal/agent/`, `internal/runtime/`, `internal/wire/`, `web/src/components/`, `eval/`, `examples/skills/`

**Files scanned:** ~30 files across 10 packages

**Pattern extraction date:** 2026-08-28

**Key architectural decisions captured:**
- D11-B3 single source of truth pattern (AllowedWriteTargets)
- INSERT-only audit table pattern (kpi_calls.go)
- HITL two-pass interrupt/resume contract
- Fail-closed template validation
- deep+data three-guard unlock (validate.go + deep_agent.go + runner_setup.go)
- fan_out_dispatch tool structural validation + non-error fallback
- goldmark+bluemonday render pipeline order (bind → goldmark → bluemonday)
