---
phase: 11
plan: 04
status: complete
date: 2026-08-20
commits:
  - dbdabcb: feat(11-04): review_report write Tool — Pass 1 interrupt + Pass 2 resume (HITL-06)
  - 66c5cf5: feat(11-04): registry wires reviewReportTool into hitlHandlers[review_report]
  - 3fa04e1: feat(11-04): AuditFilter Action + ToolCallID extension (?-parameterized)
  - 404cb84: feat(11-04): approval dual endpoint + audit timeline + server wiring
  - 0cc1aa8: feat(11-04): 8 hermetic tests — review_report tool + approval endpoints + resume cancel lock
---

# 11-04 SUMMARY — Approval dual endpoint + review_report Tool + audit timeline

## What shipped

**BE half of the Phase 11 approval gate (HITL-01/02/03/06 + D11-C1/C2 + D-04/D15).**

1. **`internal/agent/hitl/hitl_review_report.go`** (new, 236 lines) — the second write
   Tool (HITL-06 complete). `reviewReportTool` mirrors `hitl_memory_facts.go`:
   Pass 1 = `InsertPendingApproval` (fail-closed) THEN `tool.StatefulInterrupt`;
   Pass 2 = `GetResumeContext[*ApprovalDecision]` → `Validate()` → deny message or
   structured success string (`report=%d_chars citations=%d`). Payload schema
   `{report, citations}` per 11-CONTEXT §Claude's Discretion E.
   `ReviewReportToolName = "review_report_write"` distinct from target
   `"review_report"` (D11-B2).

2. **`internal/agent/hitl/registry.go`** (one-line additive) —
   `hitlHandlers["review_report"] = &reviewReportTool{}` replaces 11-02's nil
   placeholder. `AllowedWriteTargets` unchanged (D11-B3).

3. **`internal/server/approval_api.go`** (new, 558 lines) — D11-C1 dual endpoint:
   - `GET /api/session/:id/approval` — pending rows with derived state
     (`active` | `expired` | `auto_passed`) from the D11-C2
     (skill_version, prompt_hash) snapshot compare against the active Skill.
   - `POST /api/session/:id/approval/:approvalId` — ordered pipeline:
     regex gates (400) → deps gate (404) → JSON decode (400) →
     `ApprovalDecision.Validate()` (422 with `{field, msg}`, HITL-02) →
     already-resolved idempotency (200) → pending lookup (404, cross-session
     moat) → target spoof check (422) → **D11-C2 expire-check** (410 Gone +
     checkpoint `Delete`, audit row preserved per T-3-AUDIT) →
     `AuditWriter.MarkApproved/MarkRejected` → `Runner.ResumeWithParams` →
     drain → `data-decision{decision: tool-resumed|tool-denied}` SSE chunk.

4. **`internal/server/audit_api.go`** (new, 167 lines) —
   `GET /api/session/:id/audit?type=...&limit=N` for the FE AuditTimelineTab
   (11-07). Allowlisted 4-action enum + empty; limit 1–200 (default 50);
   200 + empty for unknown sessions (existence-leak posture). All values
   `?`-parameterized (T-3-SQLI).

5. **`internal/server/server.go`** — `Deps.AuditWriter` + `Deps.ActiveManifest`
   fields; 3 route registrations; `New()` initializes the writer from Store.

6. **Tests** (11 new + 1 updated, all hermetic, all green):
   - `hitl_review_report_test.go` (5): FirstPassInterrupts, SecondWriteTool
     (registry has 2 distinct tools — HITL-06 lock), deny-without-reason
     Validate gate, Info schema, args parse.
   - `hitl_tools_test.go`: `review_report` case 0→1 tool;
     TestReviewReportToolAppearsInWhitelist (1/2/0 tool cases).
   - `approval_api_test.go` (5): TestExpiredCheckpointUIState (GET expired +
     POST 410 + audit preserved + zero resolution rows),
     TestApprovalEndpointBoundToSession (wrong session → 404),
     TestDataDecisionExtensionNoNewChunkTypes (**full interrupt → checkpoint →
     HTTP POST → resume → Tool Pass 2 → final answer round trip**;
     `data-decision{decision:"tool-resumed"}` observed; zero new chunk types;
     idempotent `already_resolved`),
     TestCancelPropagationMidToolRunner (**resume path <200ms cancel lock**),
     TestAuditListHandler.

## Verification gate (Task 6)

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l` on 11-04-touched files | PASS (zero diffs; see deviation D6) |
| `go test ./internal/... -count=1` | PASS (17 packages, 0 failures) |
| TestNoLegacyInterruptAPI WalkDir (4 roots) | PASS (0 hits) |
| `grep -c tool.StatefulInterrupt( hitl_review_report.go` | 2 (D11-A2 boundary) |
| T-3-AUDIT INSERT-only | PASS (0 new UPDATE/DELETE; see D7) |
| hitl → skill one-way edge (`grep skillforge/internal/skill" hitl/`) | 0 |
| data-decision discriminator anchor (wire/chunk.go) | 9 refs, unchanged union |
| 11 named tests (plan success criteria #5) | ALL PASS |
| New go.mod dependencies | 0 (eino v0.9.14 unchanged) |
| New SSE chunk types | 0 (D-04 + D15; decision string enum extended only) |

## Deviations (documented, load-bearing)

- **D1 — Resume target key.** Plan locked `Targets: {approvalID: &decision}`
  (approvalID == tool_call_id). Empirical probe against eino v0.9.14
  (adk + compose sources + live round-trip test): interrupt IDs are
  `uuid.NewString()` generated at interrupt time; ONLY the UUID from
  `InterruptContexts[i].ID` resumes the Tool — the raw tool_call_id and the
  joined address string both re-interrupt (probe-tested). The POST body
  therefore accepts optional `interrupt_id` (FE echoes it from the
  interrupted payload); approvalID remains the plan-literal fallback.
  D11-C4 (approval_id == tool_call_id as audit PK) is unaffected.
- **D2 — RunnerBuildConfig.CheckPointStore.** eino v0.9.14 has NO
  `adk.Runner.WithCheckPointStore` setter (plan's primary form); the store
  binds only via `adk.RunnerConfig.CheckPointStore` at construction — the
  plan-sanctioned fallback. One additive field in 11-01's
  `internal/agent/runner_setup.go`.
- **D3 — AuditFilter.Action + ToolCallID.** Did not exist (11-03 deferred).
  Added per plan Task 4's CRITICAL note: one field + one `?`-parameterized
  clause each in `internal/store/store.go` + `internal/store/sqlite/audit.go`.
- **D4 — `gob.RegisterName("hitl.ApprovalInfo", ...)`** in
  hitl_review_report.go init. Probe: without it the Runner's checkpoint save
  fails (`gob: type not registered for interface: hitl.ApprovalInfo`) and NO
  HITL interrupt can persist. gob.RegisterName chosen over
  schema.RegisterName because the latter panics on duplicate registration;
  gob is idempotent for same name+type (11-05 runs concurrently in this
  tree).
- **D5 — `-race` unavailable** on this host (no gcc for cgo). Tests run with
  `-count=1 -timeout`; the race detector could not be exercised.
- **D6 — gofmt gate scope.** `gofmt -l internal/server/` flags 13
  PRE-EXISTING files (verified via stash: diffs pre-date 11-04 — toolchain
  comment-reflow skew). All 11-04-touched files are gofmt-clean; no
  out-of-scope files were reformatted.
- **D7 — T-3-AUDIT grep shows 2 hits**, both pre-existing (the
  `updateAuditMetricsSQL` constant + a comment; verified identical at
  11-03's commit). 11-04's audit.go diff adds zero UPDATE/DELETE lines.
- **D8 — TestTransportSwapByteIdentical does not exist** in the repo
  (must_have referenced it aspirationally). D-04 is locked by
  TestDataDecisionExtensionNoNewChunkTypes + the wire package suite.
- **D9 — handlers_session_history.go untouched.** `sessionIDRegex` is
  already package-level shared; the "EXTEND — minimal" touched-list entry
  needed no change.
- **D10 — Concurrent 11-05 execution.** Plan 11-05 ran in the same tree and
  (a) renamed `memoryFactsTool` → exported `MemoryFactsTool`, (b) extended
  `auditWriterHook` to 4 methods, (c) extended `BuildToolsForSkill` +
  `RunnerBuildConfig` (RuleMatcher). 11-04's reviewReportTool consumes only
  the `InsertPendingApproval` method of the extended seam (per plan); no
  11-05 uncommitted work was swept into 11-04 commits (explicit pathspec
  staging throughout).
- **D11 — Success-path response is the SSE stream** (status 200) carrying
  the drained resume events + the data-decision chunk. A trailing JSON body
  after SSE hijack is physically impossible; the must_have phrasing
  "returns 200 with data-decision SSE chunk" is honored.
- **D12 — TestReviewReportToolRejectsDenyWithoutReason exercises Validate
  directly** (mirrors 11-02's precedent — a synthetic interrupt ctx
  requires eino internals). The framework resume path IS covered
  end-to-end by TestDataDecisionExtensionNoNewChunkTypes.

## Line counts

```
 236 internal/agent/hitl/hitl_review_report.go
 148 internal/agent/hitl/hitl_review_report_test.go
 558 internal/server/approval_api.go
 642 internal/server/approval_api_test.go
 167 internal/server/audit_api.go
```

## Files touched

Created (5): hitl_review_report.go, hitl_review_report_test.go,
approval_api.go, approval_api_test.go, audit_api.go.
Modified (6): registry.go (1-line value), hitl_tools_test.go (case + new
test), server.go (Deps fields + routes + New init), agent/runner_setup.go
(CheckPointStore field), store/store.go (2 filter fields),
store/sqlite/audit.go (2 clauses).
NOT touched despite touched-list: handlers_session_history.go (D9).

## Next

11-06 (FE ApprovalCard) + 11-07 (FE Rules panel + AuditTimelineTab) —
Wave 4 parallel — can start: the dual endpoint, audit timeline, and
data-decision wire contract are live. 11-05 is COMPLETE (summary at
61ae64c), so the rule auto-pass path and these endpoints compose.
