---
status: complete
phase: 18-skill-p10-4
source: [18-01-SUMMARY.md, 18-02-SUMMARY.md, 18-03-SUMMARY.md]
started: 2026-08-28T00:00:00Z
updated: 2026-08-29T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server. Clear ephemeral state. Start the server from scratch. Server boots without errors. Primary query (health check, or /api/chat) returns live data.
result: blocked
blocked_by: other
reason: "gcc linking error: undefined reference to __emutls_v._ZSt11__once_call (DuckDB Go bindings + MSYS2 ucrt64 gcc incompatibility)"

### 2. deep+data Conditional Allow — report:publish skill
expected: |
  A skill with writes: ["report:publish"] and template: defined loads without "deep+data banned" error.
  The skill appears in the skill list and its template sections are visible.
result: pass

### 3. report:publish HITL — Two-Pass Contract
expected: |
  When an agent calls report_publish tool:
  Pass 1: Request is held (interrupt), pending approval appears in /api/approvals.
  Operator can then approve, deny, or request_change.
  If approve: report HTML is published and URL returned.
  If deny: tool returns denial, no file created.
  If request_change: modification notes are returned to the agent for the next run.
result: pass

### 4. Reports Store — Persist and List
expected: |
  After a report is published, it appears in /api/reports?period=<period>.
  Each report has period, version (v1, v2...), title, file_path, and URL.
  Publishing again for the same period increments version (v1 → v2).
result: pass

### 5. /reports/*filepath — Traversal Protection
expected: |
  GET /reports/../../../etc/passwd returns 403 or 404 (not the file contents).
  GET /reports/period/v1.html returns the published HTML if it exists.
result: pass

### 6. Fan-Out Dispatch — Valid Task Boundaries
expected: |
  When a monthly-report skill runs with template sections that match the fan-out request,
  fan_out_dispatch tool fans out in parallel (4 subtasks at once) and returns structured
  JSON result with per-section outputs.
result: pass

### 7. Fan-Out Dispatch — Invalid Boundaries Fallback
expected: |
  When task boundaries do not exactly match template section titles,
  fan_out_dispatch returns {"status":"fallback_sequential","reason":"boundary_not_in_template"}
  and the LLM continues without error.
result: pass

### 8. FE — report_published Card
expected: |
  After a report is published, the chat shows a ReportCard:
  - Period badge and version number
  - Section structure summary (titles listed)
  - "查看报告" link opens /reports/<period>/v<N>.html in new tab
  - "带修改重生成" chip
result: pass

### 9. FE — ApprovalCard change_requested State
expected: |
  On the operator approval page (/approve or /approvals):
  For a report:publish pending item, there is a "请求修改" button (amber).
  Clicking it and entering a reason sets status to "已要求修改" (amber badge).
result: pass

### 10. FE — ChatInput Pre-Fill from "带修改重生成"
expected: |
  Clicking "带修改重生成" on a published report card pre-fills the chat input
  with modification context text. User can type and send as a follow-up.
result: pass

### 11. PrevRunContext — Previous Version Shown to Agent
expected: |
  When re-running a monthly-report for the same period:
  DeepAgent's system prompt includes a paragraph about the previous run
  (version, period, template version, modification notes).
  The agent references this to produce an updated report rather than a fresh one.
result: pass

## Summary

total: 11
passed: 10
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps

[none yet]
