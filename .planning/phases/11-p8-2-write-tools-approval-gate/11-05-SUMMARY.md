---
phase: 11-p8-2-write-tools-approval-gate
plan: 05
subsystem: agent/hitl + store/sqlite + server/api
tags: [hitl, approval-rules, frozen-args, sha256, sqlite, eino-adk, hertz]

# Dependency graph
requires:
  - phase: 11-02
    provides: ApprovalDecision (FrozenArgs json.RawMessage), HITL Tool registry, memoryFactsTool Pass1/Pass2, BuildToolsForSkill
  - phase: 11-03
    provides: AuditWriter (MarkApproved/MarkRejected/MarkRuleRevoked), audit lifecycle DDL, putRuleRevokedSQL, checkpoint snapshot
provides:
  - approval_rules table (PK target+args_hash+skill_id+scope_session_id, D11-D2) + ApprovalRulesStore narrow interface
  - RuleMatcher seam (SqlRuleMatcher + NoopRuleMatcher) wired into MemoryFactsTool Pass 1 (D11-D1 auto-pass)
  - HITL-07 3-way byte-identity invariant (ComputeFrozenArgsHash + assertThreeWayByteEqual + ErrFrozenArgsDrift)
  - AuditWriter.MarkAutoPassedRule (action_approved + refusal_reason='auto_passed_rule')
  - GET/DELETE /api/session/:id/approval-rules backend handlers (D11-D4 viewable + revocable)
affects: [11-06 (FE ApprovalCard consumes frozen-args path), 11-07 (FE Rules panel consumes GET/DELETE), 11-08 (E2E acceptance), Phase 12 (real auth + session manifest lookup)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D11-D1 rule key = (target, sha256(verbatim frozenArgs bytes), skill_id) — NEVER re-marshaled through encoding/json"
    - "D11-D3 binary scope: scope_session_id='' means skill-scope; non-empty means session-scope; no global scope exists structurally"
    - "Revoke tx pattern: DELETE approval_rules + INSERT audit action_rule_revoked in ONE *sql.Tx (T-3-AUDIT — audit row is the evidence)"
    - "Fail-closed direction: rule-matcher store error => no-match => interrupt (toward human review, never toward silent execution)"

key-files:
  created:
    - internal/store/sqlite/approval_rules.go
    - internal/store/sqlite/approval_rules_test.go
    - internal/agent/hitl/rule_matcher.go
    - internal/agent/hitl/frozen_args.go
    - internal/agent/hitl/frozen_args_test.go
    - internal/server/approval_rules_api.go
  modified:
    - internal/store/sqlite/schema.go
    - internal/agent/hitl/approval_decision.go
    - internal/agent/hitl/audit_writer.go
    - internal/agent/hitl/hitl_memory_facts.go
    - internal/agent/hitl/hitl_memory_facts_test.go
    - internal/agent/hitl/registry.go
    - internal/agent/hitl_tools.go
    - internal/agent/hitl_tools_test.go
    - internal/agent/runner_setup.go

key-decisions:
  - "DEVIATION: store Revoke takes (skillVersion, promptHash string) instead of *skill.Manifest — sqlite->skill->hitl->sqlite import cycle (skill/validate.go imports hitl; hitl/rule_matcher.go imports sqlite)"
  - "DEVIATION: RunnerBuildConfig.RuleMatcher interface injection instead of building NewSqlRuleMatcher(deps.Store) inside BuildRunner — BuildRunner has no Deps and documents a store-free purity invariant"
  - "DEVIATION: auditWriterHook MarkApproved/MarkRejected mirror AuditWriter's real (SkillSnapshot-carrying) signatures — Go has no overloading and 11-03's methods must not break"
  - "Auto-pass audit = single action_approved row with refusal_reason='auto_passed_rule'; no pending row (no interrupt happened)"
  - "Rule-matcher store errors map to no-match (fail-closed toward human review)"

patterns-established:
  - "3-way byte equality: preview == audit frozen_args == executed args; ANY drift -> ErrFrozenArgsDrift, write never executes"
  - "WithContext per-turn injection: shared hitlHandlers instances get sessionID/skillID/ruleMatcher bound at BuildToolsForSkill time"
  - "base64url-JSON opaque rule_id for PK transport between GET and DELETE endpoints"

requirements-completed: [HITL-07, HITL-08]

# Metrics
duration: 55min
completed: 2026-08-20
---

# Phase 11 Plan 05: 审批记忆规则 + frozen-args 字节同一性 Summary

**HITL-07 的"批的 A 不会执行成 B"不变量与 HITL-08 的 approve-and-remember 规则系统落地：独立 approval_rules 表（D11-D2）、session/skill 二元作用域（D11-D3）、单事务撤销+审计（D11-D4）、以及 Rule 面板后端 GET/DELETE。**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-08-20
- **Tasks:** 8/8 complete
- **Files modified:** 15 (6 created + 9 modified)

## Accomplishments

- **HITL-07 closed**: `ComputeFrozenArgsHash` (verbatim-bytes sha256, never re-marshaled) + `assertThreeWayByteEqual` enforce preview == audit == executed; drift returns `ErrFrozenArgsDrift` and the write NEVER executes. `ApplyFrozenArgs` implements edit-before-approve selection (FrozenArgs wins when non-empty).
- **HITL-08 closed**: independent `approval_rules` table with PK (target, args_hash, skill_id, scope_session_id); 5 store methods + `ApprovalRulesStore` narrow interface; `RuleMatcher.Match` runs BEFORE `StatefulInterrupt` in Pass 1 — matched rule skips the interrupt, writes the `auto_passed_rule` audit row, and executes directly.
- **D11-D3 binary scope locked by test**: session-scope rules never match cross-session; skill-scope rules match across sessions for the same Skill; different skill_id never matches; unknown scope rejected.
- **D11-D4 locked by test**: `Revoke` runs DELETE + `action_rule_revoked` audit INSERT in one transaction; double-revoke errors; the audit row carries the revoked rule PK in frozen_args (T-3-AUDIT INSERT-only preserved — no new UPDATE/DELETE path on the audit table).
- **Rule panel backend**: GET (scope-validated list with opaque `rule_id`) + DELETE (base64url-JSON PK decode, 400/404/500/503 mapping) handlers in `internal/server/approval_rules_api.go`. Router registration intentionally out of scope (no router file in the touched list).

## Task Commits

Each task was committed atomically:

1. **Task 1: approval_rules DDL + ApprovalRulesStore** — `3a80d44` (feat)
2. **Task 2: RuleMatcher seam** — `d20a49b` (feat)
3. **Task 3: frozen_args helpers + ApplyFrozenArgs** — `462cf57` (feat)
4. **Task 4: AuditWriter.MarkAutoPassedRule** — `8a6a765` (feat)
5. **Task 5: MemoryFactsTool rule auto-pass + HITL-07 Pass 2** — `eef4132` (feat)
6. **Task 6: BuildToolsForSkill + runner wiring** — `e1d68e3` (feat)
7. **Task 7: approval-rules GET/DELETE handlers** — `579cd11` (feat)
8. **Task 8: 4 hermetic tests** — `5105fa1` (test)

## Files Created/Modified

- `internal/store/sqlite/approval_rules.go` (372 lines) — table DDL owner; ApprovalRule + Scope; CreateRule/QueryRules/MatchRule/Revoke/UpdateLastUsedAt; Revoke tx with insertRuleRevokedAuditRow reusing putRuleRevokedSQL
- `internal/store/sqlite/schema.go` — approval_rules DDL + 2 indexes appended to schemaSQL (idempotent)
- `internal/agent/hitl/rule_matcher.go` (101 lines) — RuleMatcher iface + SqlRuleMatcher (fire-and-forget last_used_at) + NoopRuleMatcher
- `internal/agent/hitl/frozen_args.go` (97 lines) — ComputeFrozenArgsHash + FrozenArgsBytesEqual + assertThreeWayByteEqual + ErrFrozenArgsDrift + VerifyFrozenArgsRoundTrip
- `internal/agent/hitl/approval_decision.go` — ApplyFrozenArgs + assertFrozenArgsEqual wrappers
- `internal/agent/hitl/audit_writer.go` — MarkAutoPassedRule (refusal_reason='auto_passed_rule' literal)
- `internal/agent/hitl/hitl_memory_facts.go` — MemoryFactsTool exported + WithContext; Pass 1 rule-match before interrupt; Pass 2 ApplyFrozenArgs + assertFrozenArgsEqual; executeMemoryFactsWrite + userIDFromCtx
- `internal/agent/hitl/registry.go` — hitlHandlers uses exported MemoryFactsTool
- `internal/agent/hitl_tools.go` — BuildToolsForSkill(ctx, manifest, sessionID, skillID, ruleMatcher); WithContext injection per whitelist entry
- `internal/agent/runner_setup.go` — RunnerBuildConfig.RuleMatcher field (nil → NoopRuleMatcher); wired into BuildToolsForSkill
- `internal/server/approval_rules_api.go` (338 lines) — ListApprovalRulesHandler + RevokeApprovalRuleHandler + encodeRuleID/decodeRuleID + userIDFromCtx/manifestFromSession stubs
- Tests: `approval_rules_test.go` (140), `frozen_args_test.go` (87), `hitl_memory_facts_test.go` (TestRuleAutoPass + nil-matcher fallback), `hitl_tools_test.go` (mechanical signature updates)

## Deviations (all documented in-code at point of change)

1. **Revoke signature**: `*skill.Manifest` → `(skillVersion, promptHash string)`. Import cycle: sqlite→skill→hitl→sqlite.
2. **Runner wiring**: matcher injected via `RunnerBuildConfig.RuleMatcher` instead of constructed from `deps.Store` inside BuildRunner (no Deps; purity invariant documented in runner_setup.go).
3. **auditWriterHook signatures**: mirror AuditWriter's real SkillSnapshot-carrying signatures (no Go overloading; 11-03 contract preserved). Compile-time assertion `var _ auditWriterHook = (*AuditWriter)(nil)` still holds.
4. **-race unavailable**: no gcc/cgo on this machine (`cgo: C compiler "gcc" not found`). All tests run without -race; CI with a C toolchain should re-run with -race.
5. **hitl_tools_test.go**: mechanical 5-arg call-site updates (necessary consequence of the plan-mandated signature change).
6. **executeMemoryFactsWrite** keeps the 权威 tier default (plan pseudo-code dropped it; preserving 11-02 documented behavior).
7. **rule_id in GET response**: additive field carrying the base64url-JSON PK so the FE never reconstructs the key client-side.
8. **Pre-existing gofmt violations** in untouched files (internal/store/sqlite/schema_test.go, session_meta.go, internal/history/*, internal/verdict/*, etc.) predate 11-05 and are outside the touched list; every file 11-05 created or modified is gofmt-clean.

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l internal/` — clean for all 11-05 touched files (pre-existing violations elsewhere noted above)
- `go test ./internal/agent/hitl/... ./internal/store/sqlite/... ./internal/server/... -count=1 -timeout=120s` — PASS (all packages ok)
- 4 hermetic tests PASS (+1 bonus nil-matcher fallback test): TestApprovalRulesScope, TestApprovalRulesRevoke, TestFrozenArgsThreeWayByteEquality, TestRuleAutoPass
- `go test ./integration/contract -tags=contract -run TestNoLegacyInterruptAPI` — PASS (0 hits)
- T-3-AUDIT: no new UPDATE/DELETE path on the audit table (only hit: sanctioned updateAuditMetricsSQL exception + a pre-existing schema_test probe; UpdateLastUsedAt targets approval_rules, not audit)
- `grep -c ComputeFrozenArgsHash internal/agent/hitl/` — 5 files ≥ 1

## Concurrent-execution note

Plan 11-04 executed concurrently in the same working tree (interleaved commits `dbdabcb`/`66c5cf5`/`3fa04e1`/`404cb84`; untracked WIP files appeared and disappeared mid-run). All 11-05 commits were strictly file-scoped (`git add <explicit files>`); no 11-04 file was committed by 11-05. Their uncommitted working-tree changes (registry.go review_report wiring, hitl_tools_test.go review_report expectations) were left untouched and compile green against the 11-05 surface.

## Next

- **11-06 + 11-07 (Wave 4, parallel) ready**: FE ApprovalCard consumes the frozen-args edit path; FE Rules panel consumes GET/DELETE /api/session/:id/approval-rules (rule_id handle embedded).
- Phase 12: real auth for userIDFromCtx; session manifest lookup for manifestFromSession; production wiring of `hitl.NewSqlRuleMatcher(store)` into `RunnerBuildConfig.RuleMatcher` at the server boundary.
