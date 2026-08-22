---
phase: 14-p10-0-cgo-trust-floor
plan: 02
subsystem: platform-floor / audit / admin API
objective: "Persist platform-floor semver in every audit row and expose an admin-only preview of the effective system prompt per Skill."
tags: [PLAT-10, audit, platform-floor, admin, TDD]
requires: [PLAT-09, 14-01]
provides: [PLAT-10]
affects:
  - internal/store/store.go
  - internal/store/sqlite/schema.go
  - internal/store/sqlite/audit.go
  - internal/store/sqlite/audit_test.go
  - internal/store/sqlite/approval_rules.go
  - internal/agent/hitl/audit_writer.go
  - internal/agent/hitl/audit_writer_test.go
  - internal/agent/hitl/threat_mitigations_test.go
  - internal/server/chat.go
  - internal/server/approval_api.go
  - internal/server/approval_api_test.go
  - internal/server/platform_prompt.go
  - internal/server/platform_prompt_test.go
  - internal/server/server.go
  - cmd/spike-hitl/main.go
tech-stack:
  added: []
  patterns:
    - "ALTER TABLE ADD COLUMN guarded by columnExists + auditLifecycleColumns (idempotent additive schema)"
    - "Dependency-inject floor version into hitl.AuditWriter to avoid agent->hitl import cycle"
    - "Fail-closed sentinel ErrMissingFloorVersion blocks empty platform_floor_version writes"
    - "Reuse isAdminRequest (ConstantTimeCompare) for new admin endpoint"
key-files:
  created:
    - internal/store/sqlite/audit_test.go
    - internal/server/platform_prompt.go
    - internal/server/platform_prompt_test.go
  modified:
    - internal/store/store.go
    - internal/store/sqlite/schema.go
    - internal/store/sqlite/audit.go
    - internal/store/sqlite/approval_rules.go
    - internal/agent/hitl/audit_writer.go
    - internal/agent/hitl/audit_writer_test.go
    - internal/agent/hitl/threat_mitigations_test.go
    - internal/server/chat.go
    - internal/server/approval_api.go
    - internal/server/approval_api_test.go
    - internal/server/server.go
    - cmd/spike-hitl/main.go
decisions:
  - "D14-B2: floor version injected via NewAuditWriter constructor rather than hitl importing internal/agent (avoids import cycle)"
  - "D14-B2: empty floor version treated as fail-closed error (ErrMissingFloorVersion) rather than allowing 'pre-floor' masquerading rows"
  - "Admin preview reuses existing X-Admin-Token gate (isAdminRequest) instead of creating a new auth check"
metrics:
  duration: 15m
  completed_date: "2026-08-22"
  tasks: 2
  commits: 4
  tests_added: 2
  tests_passed: all
---

# Phase 14 Plan 02: Audit Floor Version + Admin Prompt Preview Summary

Platform-floor trust substrate extended: every audit row now carries the semantic platform-floor version "1.0.0", and admins can preview the exact composed system prompt + reproducible hash for any loaded Skill.

## What Was Built

1. **Audit schema + write path** — Added `platform_floor_version TEXT NOT NULL DEFAULT ''` to the `audit` table via the existing idempotent `auditLifecycleColumns` migration gate. Updated `putAuditSQL`, `queryAuditCols`, row scanning, and the four HITL INSERT statements (`putPendingApprovalSQL`, `putApprovedSQL`, `putRejectedSQL`, `putRuleRevokedSQL`) to carry the new column.
2. **HITL version injection** — `hitl.AuditWriter` now receives the floor version through `NewAuditWriter(s store.Store, floorVersion string)`, avoiding an import cycle with `internal/agent`. A fail-closed `ErrMissingFloorVersion` sentinel prevents empty versions from masquerading as pre-floor rows.
3. **Chat + server call sites** — `internal/server/chat.go`, `internal/server/server.go`, `internal/server/approval_api.go`, and `cmd/spike-hitl/main.go` updated to pass `agent.PlatformFloorVersion`.
4. **Admin preview endpoint** — New `GET /api/admin/platform-prompt?skill={id}` returns 403 (no/wrong token), 404 (unknown Skill), or 200 JSON with `floor_version`, `skill_id`, `composed_prompt`, and `prompt_hash`. It reuses the existing `isAdminRequest` ConstantTimeCompare gate and computes the same hash used by the chat audit path.

## Verification

- `go build ./... && go vet ./...` exit 0
- `go test ./internal/store/... ./internal/agent/hitl/ ./internal/server/ -count=1` all PASS
- `TestAuditPlatformFloorVersionColumn` PASS (PutAudit round-trip + DISTINCT drift window with empty legacy rows)
- `TestPlatformPromptPreview` PASS (403 unset, 403 wrong token, 200 with composed prompt + hash, 404 unknown skill)

## Threat Mitigations Applied

| Threat ID | Mitigation |
|-----------|------------|
| T-14-04 | Admin endpoint gated by `isAdminRequest` with env-set default-deny and `subtle.ConstantTimeCompare` |
| T-14-05 | DDL is a code constant; no user input reaches DDL; `columnExists` idempotent guard |
| T-14-06 | Preview is admin-only; prompt content is open-source by design; security model relies on structural prepend, not secrecy |
| T-14-06b | `ErrMissingFloorVersion` fail-closed + five-site HITL assertion lock prevents empty-version rows |

## Deviations from Plan

None — plan executed exactly as written. One Rule-1 runtime fix occurred during Task 1 verification (`sqlite.Revoke.AuditInsert: missing argument with index 16`) and was fixed inline by importing `internal/platformfloor` in `internal/store/sqlite/approval_rules.go` and passing `platformfloor.PlatformFloorVersion` to `insertRuleRevokedAuditRow`. This was part of the same task before the GREEN commit.

## Commits

| Hash | Type | Message |
|------|------|---------|
| `0463905` | test | test(14-02): add PLAT-10 audit floor-version column locks (RED) |
| `63ca90c` | feat | feat(14-02): stamp platform_floor_version on all audit INSERT paths (GREEN) |
| `b0d1202` | test | test(14-02): add TestPlatformPromptPreview admin endpoint lock (RED) |
| `d2f57db` | feat | feat(14-02): implement GET /api/admin/platform-prompt preview (GREEN) |

## Self-Check: PASSED

- [x] `14-02-SUMMARY.md` created at `.planning/phases/14-p10-0-cgo-trust-floor/14-02-SUMMARY.md`
- [x] All commits (`0463905`, `63ca90c`, `b0d1202`, `d2f57db`) exist in `git log`
- [x] Created files `internal/store/sqlite/audit_test.go`, `internal/server/platform_prompt.go`, `internal/server/platform_prompt_test.go` exist
- [x] All automated verification commands passed
