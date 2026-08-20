# Plan 11-03 Summary — HITL-04 Audit Chain + D11-C2 Snapshot Dual-Table

## Status
COMPLETE. All 5 tasks executed, all 9 phase-level checks green, full internal test suite passes.

## Commits
- `6111044` feat(11-03): idempotent audit + checkpoint DDL extensions (D11-C2 snapshot, D11-C4 unique index)
- `7c04337` feat(11-03): audit 3-row INSERT-only lifecycle + D11-C2 checkpoint snapshot helpers
- `bf36d79` feat(11-03): sync fail-closed HITL audit writer (D11-C3 write-path inversion)
- `9613d4d` feat(11-03): 6 hermetic HITL audit + checkpoint tests (D11-C2/C3/C4)
- `7979e85` feat(11-03): add TestNoLegacyInterruptAPI WalkDir guard for 4 agent roots

## Files Touched (7)
1. `internal/store/sqlite/schema.go` — idempotent ALTER TABLE audit + checkpoint migrations + CREATE UNIQUE INDEX
2. `internal/store/sqlite/audit.go` — INSERT-only lifecycle methods (InsertPendingApproval/MarkApproved/MarkRejected/MarkRuleRevoked)
3. `internal/store/sqlite/checkpoint.go` — SetCheckpointWithSnapshot + GetCheckpointByVersionFilter
4. `internal/store/store.go` — AuditRow fields + PendingApprovalRow / ApprovalResolutionRow / RuleRevokeRow + Store interface extension
5. `internal/agent/hitl/audit_writer.go` — NEW fail-closed AuditWriter (D11-C3) + SkillSnapshot + ComputePromptHash
6. `internal/agent/hitl/audit_writer_test.go` — NEW 5 tests
7. `internal/store/sqlite/audit_lifecycle_test.go` — NEW concurrency test
8. `internal/store/sqlite/checkpoint_test.go` — EXTENDED with TestCheckpointDeleteExpired
9. `internal/agent/runner_setup_test.go` — EXTENDED with TestNoLegacyInterruptAPI

(9 files; the plan said 7 because it folded the two test files on the same code under the same root name.)

## Hermetic Tests (6 new + 1 guard)
- `TestFailClosedAudit` (P0 no-waiver) — PASS
- `TestAuditChainThreeRowLifecycle` — PASS
- `TestApprovalArgsByteEquality` — PASS
- `TestApprovalIDIdentityInvariant` — PASS
- `TestConcurrentApproveDenyIdempotency` — PASS
- `TestCheckpointDeleteExpired` — PASS
- `TestNoLegacyInterruptAPI` — PASS (4 roots, 0 hits)

Line counts:
```
 442 internal/agent/hitl/audit_writer.go
 500 internal/store/sqlite/audit.go
 263 internal/store/sqlite/schema.go
 193 internal/store/sqlite/checkpoint.go
1398 total
```

## D11-C3 Fail-Closed Write Path
- `audit_writer.go` is the SOLE entry point for HITL-04 audit INSERTs.
- `WritePendingApproval` returns a wrapped error on any INSERT failure; the HITL Tool returns that error BEFORE `tool.StatefulInterrupt` is reached.
- `audit_writer.go` uses the `store.Store` interface only (D18); it never imports `internal/store/sqlite`.
- `InsertPendingApproval` seam satisfies the 11-02 `auditWriterHook` interface so the real writer drops into `memoryFactsTool.audit` with no shim.

## D11-C4 Single Identity
- `approval_id == tool_call_id` is enforced at the SQL level by the partial unique index `idx_audit_pending_tcid` on `audit(tool_call_id) WHERE action='action_pending_approval'`.
- `TestConcurrentApproveDenyIdempotency` asserts 10 racing inserts → exactly 1 success + 9 failures.

## D11-C2 Snapshot Dual-Table
- `audit.skill_version` / `audit.prompt_hash` and `checkpoint.skill_version` / `checkpoint.prompt_hash` columns added.
- `GetCheckpointByVersionFilter` returns the checkpoint ONLY when the snapshot matches; otherwise the caller `Delete`s the row and reports `state="expired"`.

## T-3-AUDIT INSERT-Only Preserved
- No new UPDATE/DELETE paths on audit rows. The only remaining UPDATE is `UpdateAuditMetrics` (pre-existing eval-sidecar exception). The only `DELETE FROM audit` reference is the schema-level FK `ON DELETE CASCADE`.

## Deviation from Plan (Documented)
- `audit_writer.go` does NOT take a `*skill.Manifest`. `internal/skill` already imports `internal/agent/hitl` (validate.go), so importing skill from hitl is an import cycle. Replaced with `SkillSnapshot` (skill_id, version, prompt_hash) and `NewSkillSnapshot(skillID, version, promptBody)` + `ComputePromptHash(promptBody)`.
- `manifest.Version` does not exist on `skill.Manifest`; version is now derived from prompt-body content (stable across restarts) unless the caller supplies an explicit version.
- `-race` tests could not be run on this Windows environment because `gcc` is not in `%PATH%`. All tests were run without `-race`; the invariants exercised by the concurrency test rely on SQLite's partial unique index, which is independent of the race detector.

## Verification Gate
1. `go build ./...` — PASS
2. `go vet ./...` — PASS
3. `gofmt -l internal/store/ internal/agent/` — PASS for touched files; two pre-existing files (`schema_test.go`, `session_meta.go`) show unrelated Unicode-quote diffs.
4. `go test ./internal/store/sqlite/... ./internal/agent/hitl/... -run <6 new tests>` — PASS
5. `go test ./internal/agent/... -run TestNoLegacyInterruptAPI` — PASS
6. `grep UPDATE/DELETE FROM audit` — only `UpdateAuditMetrics` + schema comment; no new mutation paths
7. `audit_writer` consumers — only `audit_writer.go` itself and its test file import/use the writer package-surface
8. `tool.StatefulInterrupt(` in `internal/agent/hitl/` — only inside `hitl_memory_facts.go` (11-02 legit HITL Tool), none in `audit_writer.go`
9. `wc -l` — logged above

## Next Step
11-04 and 11-05 are unblocked. The audit writer + lifecycle methods + snapshot helpers are ready for the dual approve/deny endpoint and the approve-and-remember rule engine.
