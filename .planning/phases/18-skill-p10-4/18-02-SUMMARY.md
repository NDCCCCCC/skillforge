# Phase 18 Plan 02: report:publish HITL + Registry + Reports Store + Static Route

## One-liner

report:publish wired as 4th HITL write target; HITL two-pass contract with request_change third state; reports store layer and static HTML route with traversal protection.

## Commits

| Hash | Message |
|------|---------|
| `114168c` | feat(18-02): report:publish HITL handler — hitl_report_publish.go two-pass contract |
| `5b21f60` | feat(18-02): reports table DDL + PutReport/ListReports + static route |
| `5b424bb` | feat(18-02): ReportCard + report_published DecisionPayload + DecisionPlanChangeRequested |

## Tasks Completed

### Task 1: report:publish HITL Handler

**hitl_report_publish.go** — two-pass HITL tool (`report_publish` / `report:publish`):
- **Pass 1**: InsertPendingApproval → StatefulInterrupt with frozen_args snapshot
- **Pass 2**: GetResumeContext → deny / approve / request_change three branches
- `request_change` branch returns `"change requested: <reason>"` — new run resumes with modification notes
- `approve` branch: reconstructs frozen args → calls `report.Publish()` → emits ReportCard via SSE

**Pass 1** (interrupt):
```go
approvalID := compose.GetToolCallID(ctx)
info := &ApprovalInfo{
    ToolName:        ReportPublishToolName,
    ArgumentsInJSON: args, // frozen_args = complete snapshot
    ToolCallID:      approvalID,
    Target:          ReportPublishTarget,
}
t.auditOrDefault().InsertPendingApproval(ctx, approvalID, info)
return "", tool.StatefulInterrupt(ctx, info, args)
```

**Pass 2** (approve branch):
```go
pubResult, err := t.reportPub(ctx, tmpl, slots, snap,
    frozen.Period, t.store, approvalAuditID, frozen.KPICallIDs, frozen.Title)
// → PublishResult{Version, FilePath, URL}
card := &wire.ReportCard{Title: frozen.Title, Period: frozen.Period,
    Version: pubResult.Version, URL: pubResult.URL}
```

### Task 2: HITL Registry + Reports Store + Static Route

**registry.go** — `report:publish` registered:
- `AllowedWriteTargets`: `"report:publish"` added as 4th entry (line 78)
- `hitlHandlers`: `"report:publish": &reportPublishTool{}` registered (line 106)

**schema.go** — `reports` table:
```sql
CREATE TABLE IF NOT EXISTS reports (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    ts                 TEXT NOT NULL,
    period             TEXT NOT NULL,
    skill_id           TEXT NOT NULL,
    version            INTEGER NOT NULL,
    title              TEXT NOT NULL,
    file_path          TEXT NOT NULL,
    approval_audit_id  TEXT NOT NULL,
    kpi_call_ids       TEXT NOT NULL,
    template_version   TEXT NOT NULL,
    result_digest      TEXT NOT NULL DEFAULT ''
)
-- Unique index: (period, version)
-- Period index for listing
```

**reports.go** — store methods:
- `PutReport` — INSERT-only, no update/replace
- `ListReports(ctx, period, limit)` — ordered by version DESC
- `GetReport(ctx, period, version)` — exact lookup

**server.go** — static route (reportsPathAllowlist):
```go
var reportPathAllowlist = regexp.MustCompile(`^[A-Za-z0-9_-]+/v[0-9]+\.html$`)
// Blocks: ../, %2e%2e, etc. — pattern rejects anything with path traversal
h.GET("/reports/*filepath", reportsRoute)
```

### Task 3: wire/chunk.go ReportCard + DecisionPayload Extension

**DecisionPayload** extended:
```go
ReportCard *ReportCard `json:"reportCard,omitempty"` // P18 — report_published only
```

**ReportCard struct**:
```go
type ReportCard struct {
    Title    string   `json:"title"`
    Period   string   `json:"period"`
    Version  int      `json:"version"`
    URL      string   `json:"url"` // /reports/<period>/v<N>.html
    Versions []string `json:"versions,omitempty"` // history links (D18-D3)
    RegenerateHint string `json:"regenerateHint,omitempty"` // chip pre-fill text
}
```

**New decision constants**:
- `DecisionReportPublished = "report_published"` (carried in DecisionPayload.DataRef)
- `DecisionPlanChangeRequested = "plan_change_requested"`

## Build Verification

```
go build ./internal/agent/hitl/... ./internal/store/sqlite/... ./internal/wire/...  # PASS
go test ./internal/agent/hitl/... ./internal/store/sqlite/... ./internal/wire/... -count=1  # PASS
grep -c "report:publish" internal/agent/hitl/registry.go   # 2 (AllowedWriteTargets + hitlHandlers)
grep -c "CREATE TABLE.*reports" internal/store/sqlite/schema.go  # 1
grep -c "reportPathAllowlist" internal/server/server.go  # 1
grep -c "ReportCard" internal/wire/chunk.go  # 3
grep -c "report_published\|plan_change_requested" internal/wire/chunk.go  # 2
```

## Threat Flags

| ID | Threat | Mitigation | Status |
|----|--------|------------|--------|
| T-18-B1 | Directory traversal in `/reports/*` | `reportPathAllowlist` regex rejects `../`, `%2e%2e` | Mitigated |
| T-18-B2 | Report overwriting | `vN` increment (MAX+1) per period — each publish is new version | Mitigated |
| T-18-B3 | Audit gap if DB write fails after file persists | Log DB error; file already on disk; caller retries | Accepted |

## Self-Check

- [x] All commits found via `git log --oneline`
- [x] registry.go: report:publish in AllowedWriteTargets + hitlHandlers
- [x] schema.go: reports CREATE TABLE DDL with indexes
- [x] server.go: reportPathAllowlist + GET /reports/*
- [x] chunk.go: ReportCard struct + DecisionReportPublished constant

## Self-Check: PASSED
