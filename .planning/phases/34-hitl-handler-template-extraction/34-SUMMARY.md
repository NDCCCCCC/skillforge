# Phase 34: HITL Handler Template Extraction - Summary

**Closed:** 2026-09-09
**Status:** ✅ Complete

## Goal

Eliminate copy-paste duplication across 4 HITL handlers (memory_facts, plan_approve, report_publish, review_report) by extracting shared base class and audit builder.

## Scope

| REQ-ID | Description | Status |
|--------|-------------|--------|
| **HITL-01** | Shared `auditOrDefault(audit)` helper replaces 4 per-tool methods | ✅ Done |
| **HITL-02** | Shared `writePendingAndInterrupt()` helper replaces 4 audit-then-interrupt sequences | ✅ Done |
| **HITL-03** | `noopAuditWriter` centralized in `audit_writer.go` | ✅ Done |

## Commits

| Hash | Message |
|------|---------|
| `d5dca4b` | refactor(34): HITL handler template extraction — auditOrDefault + noopAuditWriter + writePendingAndInterrupt |

## Changes

### internal/agent/hitl/audit_writer.go
- **Imports:** +`github.com/cloudwego/eino/components/tool`
- **New types:**
  - `noopAuditWriter` (moved from hitl_memory_facts.go) — 4 no-op methods
- **New functions:**
  - `auditOrDefault(audit auditWriterHook) auditWriterHook` — single source for nil-safety
  - `writePendingAndInterrupt(ctx, audit, approvalID, info, args)` — D11-C3 audit-first + StatefulInterrupt ordering

### internal/agent/hitl/hitl_memory_facts.go
- Removed `noopAuditWriter` type + 4 methods (24 lines)
- Removed `(t *MemoryFactsTool) auditOrDefault()` method (5 lines)
- 4 call sites rewired: `t.auditOrDefault()` → `auditOrDefault(t.audit)`
- Pass-1 audit+interrupt → `writePendingAndInterrupt(ctx, t.audit, approvalID, info, args)`

### internal/agent/hitl/hitl_plan_approve.go
- Removed `(h *planApproveTool) auditOrDefault()` method
- 1 call site rewired
- Pass-1 audit+interrupt → `writePendingAndInterrupt`

### internal/agent/hitl/hitl_report_publish.go
- removed `(h *reportPublishTool) auditOrDefault()` method
- 1 call site rewired
- Pass-1 audit+interrupt → `writePendingAndInterrupt`

### internal/agent/hitl/hitl_review_report.go
- removed `(h *reviewReportTool) auditOrDefault()` method
- 1 call site rewired
- Pass-1 audit+interrupt → `writePendingAndInterrupt`

### internal/agent/hitl/threat_mitigations_test.go
- Updated `HITL04AuditWriterSHA256` baseline (line range shift 324-355 → 326-356 + new hash)
- Updated `HITL07MemoryFactsInvokableRunSHA256` baseline (line range shift 339-441 → 308-410 + new hash)
- 3-way byte-equality assertion itself unchanged

## Verify Results

```
[PASS] noopAuditWriter type declaration is in audit_writer.go only
[PASS] no per-tool auditOrDefault methods (shared helper used instead)
[PASS] no bare .auditOrDefault() call sites (shared helper used)
[PASS] go build ./internal/agent/hitl/... succeeds
[PASS] all hitl tests pass
=== ALL CHECKS PASSED ===
```

## Trust Semantics Preserved

- D11-C3 audit-first invariant preserved (`writePendingAndInterrupt` ensures INSERT before StatefulInterrupt)
- noopAuditWriter behavior unchanged (4 no-op methods byte-identical to original)
- 3-way byte-equality assertion (HITL-07) byte semantics preserved — only line range and call-site format changed
- Fail-closed posture: audit INSERT error still returns error before interrupt signal

## Out of Scope

- `baseHITLTool` struct embedding (CR-01 description in audit) — current 4 handlers have different field sets and methods; helper functions achieve the same DRY goal without forcing structural inheritance. Could be revisited if 5+ HITL handlers added.
