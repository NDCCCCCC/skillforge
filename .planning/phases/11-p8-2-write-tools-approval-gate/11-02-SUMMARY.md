---
phase: 11
plan: 02
subsystem: hitl
tags: [hitl, tool, audit, approval, runner, cycle-resolution, eino-adk]
requires: [11-01]
provides:
  - HITL-05 (writes: code-level enforcement via BuildToolsForSkill pure function)
  - HITL-06 (memory:facts write Tool with StatefulInterrupt + audit writer seam)
  - D11-B1 (per-turn whitelist, pure function, re-pure-computed every Runner construction)
  - D11-B2 (ApprovalDecision dual registration: schema.RegisterName + gob.RegisterName)
  - D11-B3 (AllowedWriteTargets + hitlHandlers single source of truth, three call sites)
affects:
  - internal/agent/runner_setup.go (BuildRunner now wired to real BuildToolsForSkill)
  - internal/skill/validate.go (step-8 unknown-target check)
  - 11-03 (audit fail-closed wires noopAuditWriter → real AuditWriter)
  - 11-04 (review_report Tool + approval endpoint consume ApprovedWriteTargets)
  - 11-05 (approve-and-remember rule matcher uses AllowedWriteTargets + LookupHandler)
tech-stack:
  added: []
  patterns:
    - "Pure-function whitelist (intersection(AllowedWriteTargets, manifest.Writes))"
    - "tool.StatefulInterrupt + tool.GetResumeContext two-pass HITL"
    - "schema.RegisterName[T] wraps gob.RegisterName — single source for typed resume"
    - "Cycle resolution via relocating boundary functions to package `agent`"
key-files:
  created:
    - internal/agent/hitl/approval_decision.go
    - internal/agent/hitl/registry.go
    - internal/agent/hitl/hitl_memory_facts.go
    - internal/agent/hitl_tools.go (in package agent — Task 6 cycle resolution)
    - internal/agent/hitl/registry_test.go
    - internal/agent/hitl/hitl_memory_facts_test.go
    - internal/agent/hitl_tools_test.go
  modified:
    - internal/agent/runner_setup.go (stub → BuildToolsForSkill wiring)
    - internal/skill/validate.go (step-8 unknown-target check)
    - internal/skill/loader_test.go (TestValidateUnknownWriteTarget + TestValidateAcceptsKnownTargets)
    - integration/contract/no_legacy_interrupt_test.go (godoc note for hitl scope)
key-decisions:
  - "ApprovalDecision is registered as 'hitl.ApprovalDecision' via schema.RegisterName (which internally calls gob.RegisterName). An explicit gob.RegisterName(&ApprovalDecision{}) is added as the D11-B2 defensive redundancy — survives any future schema.RegisterName implementation change."
  - "BuildToolsForSkill relocated from `internal/agent/hitl/build_tools.go` to `internal/agent/hitl_tools.go` (package `agent`) to break the skill<->hitl import cycle. Registry (AllowedWriteTargets, hitlHandlers, KnownTargets, LookupHandler, Contains) stays in package `hitl`."
  - "auditWriterHook is unexported (not `AuditWriter`) because only the local memoryFactsTool consumes it. 11-03 will export the interface when the real audit_writer.go lands."
  - "FrozenArgs uses json.RawMessage — NOT string — so the HITL-07 byte-equality invariant `sha256(audit_row.frozen_args) == sha256(data.FrozenArgs)` holds through HTTP + gob + JSON edges."
  - "DEVIATION: AllowedWriteTargets includes 'skill_state' as a 3rd entry (deprecated D7-10 placeholder for examples/skills/calculator). Without it the existing TestLoader_ParseComplianceCanonicalSkill regression fails. See Deviations section."
requirements-completed:
  - HITL-05
  - HITL-06
duration: ~50 min
completed: 2026-08-20
---

# Phase 11 Plan 11-02: HITL Tool Substrate Summary

## One-liner
HITL Tool substrate landed: ApprovalDecision typed resume schema (D11-B2) + AllowedWriteTargets source of truth (D11-B3) + per-turn pure-function whitelist (D11-B1) + first write Tool memory:facts with audit seam (HITL-06).

## Tasks — All 8 Complete

| # | Task | Commit |
|---|------|--------|
| 1 | approval_decision.go (struct + Validate + dual registration) | 60ab386 |
| 2 | registry.go (AllowedWriteTargets + hitlHandlers, D11-B3) | 800d9ad |
| 3 | hitl_tools.go (BuildToolsForSkill, package agent — final location per Task 6) | 7fa96d9 |
| 4 | hitl_memory_facts.go (memoryFactsTool + auditWriterHook seam) | b0c1a1c |
| 5 | validate.go (step-8 unknown-target check via hitl.KnownTargets) | bc226f5 |
| 6 | runner_setup.go (replace 11-01 stub with BuildToolsForSkill) | 640cbd9 |
| 7 | 3 test files (7 hermetic tests) | 7f61e41 |
| 8a | Extend no_legacy_interrupt_test.go + loader_test.go | 1f2933d |
| 8b | skill_state deviation commit (registry + tests) | 0bdb6dd |

## Verification Gate — All 11 Phase-Level Checks Green

```
[1]  go build ./...                                            PASS (exit 0)
[2]  go vet ./...                                              PASS (exit 0)
[3]  go test ./internal/agent/hitl/... ./internal/agent/
         ./internal/skill/... -count=1                          PASS (3 packages ok)
[4]  go test ./integration/contract -tags=contract
         -run TestNoLegacyInterruptAPI -count=1 -v              PASS (0 hits across hitl/)
[5]  grep -c 'tool.StatefulInterrupt(' hitl_memory_facts.go    2  ✓ (Pass-1 invoke + Pass-2 re-interrupt)
[6]  grep -c 'schema.RegisterName\[\*ApprovalDecision\]'       1  ✓ (D11-B2)
     approval_decision.go
[7]  grep -c 'gob.RegisterName' approval_decision.go (literal) 1  ✓ (defensive redundant call)
[8]  grep -rn 'skillforge/internal/skill' internal/agent/hitl/ 0  ✓ (one-way edge holds)
[9]  grep -rn 'skillforge/internal/agent/hitl' internal/skill/  1  ✓ (skill→hitl allowed)
[10] grep -c 'AllowedWriteTargets' registry.go | validate.go    registry ≥1, validate via KnownTargets() ✓
[11] wc -l internal/agent/hitl/*.go internal/agent/hitl_tools.go logged in File Counts below
```

## File Counts

```
  219 internal/agent/hitl/approval_decision.go
  148 internal/agent/hitl/registry.go
  307 internal/agent/hitl/hitl_memory_facts.go
  211 internal/agent/hitl/registry_test.go
  221 internal/agent/hitl/hitl_memory_facts_test.go
  151 internal/agent/hitl_tools.go
  125  internal/agent/hitl_tools_test.go
 1382  total (10 files: 7 created in this plan, 3 modified, plus 3 modified pre-existing files)
```

## Tests Added

| File | Tests |
|------|-------|
| internal/agent/hitl/registry_test.go | TestAllowedTargetsSubset, TestApprovalDecisionRoundTrip, TestApprovalDecisionValidate (5 sub-cases), TestApprovalDecisionTargetMatches (3 sub-cases) |
| internal/agent/hitl/hitl_memory_facts_test.go | TestMemoryFactsToolFirstPassInterrupts, TestMemoryFactsToolRejectsDenyWithoutReason, TestMemoryFactsToolInfoSchema, TestNewApprovalID, TestNoopAuditWriter, TestMemFactsArgsParse + interface compile-time assert |
| internal/agent/hitl_tools_test.go | TestBuildToolsForSkill_NilManifest, TestBuildToolsForSkill_TableDriven (6 sub-cases), TestPromptInjectionResistance, TestBuildToolsForSkill_DeterministicOrder |
| internal/skill/loader_test.go | TestValidateUnknownWriteTarget, TestValidateAcceptsKnownTargets |

## Deviations from Plan

### [Rule 3 — Misalignment] AllowedWriteTargets extended with `skill_state`

- Found during: Task 5 + 8 (validate.go step-8 broke the existing `TestLoader_ParseComplianceCanonicalSkill` because the existing `examples/skills/calculator/SKILL.md` declares `writes: [skill_state]` — a deprecated D7-10 placeholder target not in the plan's literal 2-element list `["memory:facts", "review_report"]`).
- Fix: appended `"skill_state"` as a 3rd entry of `AllowedWriteTargets` with handler `nil` (so `BuildToolsForSkill` silently skips; no LLM-visible Tool). Documented in the registry as the "Phase 12 may prune this entry" placeholder.
- Files modified: internal/agent/hitl/registry.go + internal/agent/hitl/registry_test.go (sorted 3-element assertion).
- Verification: `go test ./internal/skill/... -count=1` exits 0; existing TestLoader_ParseComplianceCanonicalSkill green.
- Commit: 0bdb6dd.

### [Rule 3 — Plan Bypass] BuildToolsForSkill relocated directly to `package agent` (skipped temporary `internal/agent/hitl/build_tools.go`)

- Found during: Task 3 (plan says create in `internal/agent/hitl/build_tools.go` package `hitl`; Task 6 says relocate to `internal/agent/hitl_tools.go` package `agent`).
- Fix: created the function directly at its final location `internal/agent/hitl_tools.go` (package agent). The temporary `internal/agent/hitl/build_tools.go` never existed in committed code. Task 6's "DELETE" became "do nothing".
- Rationale: per Task 6 explicit guidance "DO NOT duplicate the function — DO NOT create internal/agent/hitl/build_tools.go AND internal/agent/hitl_tools.go".
- Files: internal/agent/hitl_tools.go (created once at final location).
- Verification: same — full repo compiles, BuildToolsForSkill tests green.

### [Rule 2 — Tooling] `gob.RegisterName("hitl.ApprovalDecision", &ApprovalDecision{})` — explicit redundant call

- The plan's verbatim init() calls `gob.RegisterName("hitl.ApprovalDecision")` without a value, which won't compile (Go's `encoding/gob.RegisterName(name string, value any)` requires a value).
- Fix: kept both halves as the plan's intent (D11-B2 dual registration), but called `gob.RegisterName("hitl.ApprovalDecision", &ApprovalDecision{})` with an instance. The first call `schema.RegisterName[*ApprovalDecision]` already calls `gob.RegisterName(name, *ApprovalDecision)` internally with the same name and type — the explicit second call is idempotent defensive redundancy (Pitfall 4 of the Phase 11 AI-SPEC).
- Verification: approval_decision.go compiles, TestApprovalDecisionRoundTrip passes for BOTH gob and JSON half-edges.

## Self-Check: PASSED

All 8 tasks committed with their `<done>` criteria met. All 11 phase-level verification checks green. No files outside the touched list modified (except the documented deviation). No 11-03 / 11-04 / 11-05-owned files touched.

## Architecture Notes (for downstream plans)

- **11-03 audit fail-closed**: implement `auditWriter.AuditWriter` (export the interface; rename `auditWriterHook` → `AuditWriter`) with the same `InsertPendingApproval(ctx, approvalID, *ApprovalInfo) error` method. Wire it via `runner_setup.go`'s `BuildRunner` after creating the Tool. The Tool already calls `auditOrDefault()` so the noop writer remains the fallback.

- **11-04 approval endpoint + review_report**: add `"review_report"` handler (replace the existing `nil` in `hitlHandlers`). Implement `internal/server/approval_api.go` to decode `*ApprovalDecision` from JSON, `Validate()` it, then `runner.ResumeWithParams(checkPointID, &adk.ResumeParams{Targets: ...})`.

- **11-05 approve-and-remember rule matcher**: consume `hitl.AllowedWriteTargets` for rule-key target validation; consume `hitl.LookupHandler(target)` for "handler not yet registered" forward-compat check (returns nil = rule never matches).

- **Phase 12 plan_approve add-point**: append `"plan_approve"` to `AllowedWriteTargets` AND register `&planApproveTool{}` in `hitlHandlers`. No other files in the codebase need touching (the validator auto-accepts via `hitl.KnownTargets()`).

## Threat Model (Summary)

| Threat | Mitigation | Status |
|--------|------------|--------|
| T-11-02-B1 (tampered `writes` frontmatter) | Step-8 validator rejects unknown targets at load time | Locked by `TestValidateUnknownWriteTarget` |
| T-11-02-B2 (LLM tool-call elevation) | `BuildToolsForSkill` pure-function whitelist | Locked by `TestPromptInjectionResistance` |
| T-11-02-B3 (ApprovalDecision.Target spoof) | `TargetMatches` helper (empty-allowed, mismatch-rejected) | Locked by `TestApprovalDecisionTargetMatches` |
| T-11-02-B4 (gob registration mismatch) | Dual registration both pinned to literal "hitl.ApprovalDecision" | Locked by `TestApprovalDecisionRoundTrip` |
| T-11-02-B5 (audit chain incompleteness) | Accepted by plan — 11-03 ships the real audit_writer | Tracked |
| T-11-02-B6 (BuildToolsForSkill per-turn cost) | O(n×m); n≤3, m=len(manifest.Writes); no caching | Accept |
| T-11-02-B7 (skill↔hitl import cycle) | BuildToolsForSkill relocated to package `agent` | Locked by `grep -rn` checks |

## Self-Check: PASSED
