---
phase: 12
plan: 04
subsystem: planning-layer
tags: [plan-api, rf-5-mitigation, deep-agent, spike-binary, p12-l10-lock, d11-c2-expire, d-04-wire-stable, d-04-d-15-no-new-chunk-types, t-12-04-2-target-gate, t-3-sqli]
dependency_graph:
  requires:
    - "12-01 (runtime substrate — BuildDeepAgent + MaxIteration=10 hard cap)"
    - "12-02 (plan_todos store + QueryPlanTodos seam)"
    - "12-03 (hitl/hitl_plan_approve.go — PlanApproveTarget constant + ApprovalDecision.Validate)"
    - "Phase 11 11-04 (approval_api.go — approval API pattern, approvalAlreadyResolved, runnerEmitDecision)"
  provides:
    - "internal/server/plan_api.go — GET /api/run/:id/plan + POST /api/run/:id/plan/:approvalId/approve"
    - "internal/llm/extract.go — ExtractVerdictJSONWithStructuredOutput (RF-5 mitigation)"
    - "cmd/spike-deep + eval/golden/deep.jsonl — RF-5 verdict parse rate spike"
    - "P12-L10 escalation lock: direct_rate<90% -> structured-output mandatory; <70% -> ESCALATE"
  affects:
    - "12-07 (FE PlanReplayView consumes GET /api/run/:id/plan)"
    - "12-07 (FE PlanApprovalCard POSTs to /api/run/:id/plan/:approvalId/approve)"
    - "12-08 acceptance gate reports actual RF-5 parse rate from spike-deep live run"
tech-stack:
  added: []
  patterns:
    - "approval_api.go verbatim mirror for plan endpoints (D11-C2 expire-check + 404 moat + idempotency + interrupt_id passthrough)"
    - "model.WrapImplSpecificOptFn for OpenAI-compatible response_format=json_object call-time option"
    - "spike-only binary pattern (mirrors cmd/spike-history): --dry-run + --cost-limit + JSONL output + operator-gate"
key-files:
  created:
    - internal/server/plan_api.go
    - internal/server/plan_api_test.go
    - cmd/spike-deep/main.go
    - cmd/spike-deep/main_test.go
    - eval/golden/deep.jsonl
  modified:
    - internal/server/server.go
    - internal/llm/extract.go
    - internal/llm/extract_test.go
decisions:
  - "planApproveHandler restored pending-row 404 moat + idempotency + interrupt_id passthrough + FrozenArgs carry-over (the plan's design rationale: 'mirrors approvalPostHandler verbatim') — the interrupted partial draft had dropped these (Rule 2 deviation: missing critical functionality / T-12-04-2 elevation-of-privilege gate)"
  - "ExtractVerdictJSONWithStructuredOutput uses model.WrapImplSpecificOptFn (NOT model.WithGenerateConfig) — the latter does not exist in eino v0.9.14; the impl-specific channel is the supported escape hatch for provider-specific call-time config (Rule 3)"
  - "phase2SystemInstructions is a VERBATIM COPY in llm (not an import from agent/prompts.go) — internal/agent imports internal/llm, so the reverse import would be a cycle; same lift discipline as 11-01 used"
  - "spike-deep builds DeepAgent DIRECTLY via runtime.BuildDeepAgent (NOT runtime.DispatchAgent) — pins the deep archetype and its P12-L2 MaxIteration=10 hard cap for the measurement"
  - "--max-iterations flag records intent and CLAMPS to P12-L2 hard cap (10) — the runtime call-site LITERALS 10 (source-grep locked); the flag is NOT a parameterization knob"
metrics:
  duration: "~55min (resumed from interrupted Task 1 partial; rebuilt + verified Tasks 2-4 from scratch)"
  completed_date: 2026-08-20
  tasks_completed: 4
  tasks_failed: 0
  commits_created: 5
---

# Phase 12 Plan 04: Plan API + RF-5 Mitigation Summary

## One-liner

BE planning-layer HTTP surface (`GET /api/run/:id/plan` replay + `POST /api/run/:id/plan/:approvalId/approve` confirm-gate) + structured-output verdict extraction (`ExtractVerdictJSONWithStructuredOutput`) + RF-5 verdict parse rate spike binary (`cmd/spike-deep`).

## What Shipped

### Task 1 — `internal/server/plan_api.go` + `plan_api_test.go`

GET `/api/run/:id/plan` (PLAN-05 BE replay substrate) returns the run's `plan_todos` rows in ts-ASC latest-snapshot order; JSON shape `{runId, todos, generatedAt, count}` mirroring `wire.PlanTodo` (12-02 lockstep). POST `/api/run/:id/plan/:approvalId/approve` (PLAN-03 BE confirm endpoint) is a verbatim mirror of `approvalPostHandler` with `target="plan_approve"`:

1. Deps gate → URL regex → JSON decode + body/URL cross-check
2. `ApprovalDecision.Validate` (422) + canonical-target spoof check
3. Idempotency probe (200 `already_resolved`, T-11-04-06)
4. Pending-row lookup (404 cross-session moat + **T-12-04-2 target gate: only `plan_approve` rows are resolvable through `/plan/`**)
5. D11-C2 expire-check (410 Gone + checkpoint Delete, audit chain preserved)
6. `AuditWriter.MarkApproved`/`MarkRejected` with HITL-07 frozenArgs carry-over from the pending row
7. `Runner.ResumeWithParams` with **interrupt UUID target** (FE echoes from `data-decision.interrupted`), approvalID fallback
8. SSE drain + `data-decision{decision: "plan-approved" | "plan-denied"}` (D-04 string-enum extension; NO new chunk types — D15)

7 hermetic tests pass: replay ordering + latest-status-per-todo, empty 200, session isolation (T-3-SQLI/SC-9), 422 validation, 410 expire, 404 unknown run, 404 unknown approval + no forged resolution row.

### Task 2 — Route wiring in `internal/server/server.go`

Registered `GET /api/run/:id/plan` + `POST /api/run/:id/plan/:approvalId/approve` in `New()` next to the Phase 11 approval block; godoc route list updated. `handlers_session_history.go` untouched — routing is Hertz method-chaining in `server.New` (no central route table exists), so the plan's Task 2B conditional was a documented no-op.

### Task 3 — `ExtractVerdictJSONWithStructuredOutput` (RF-5 mitigation, P12-L10)

New function on `internal/llm/extract.go`: ONE final-step LLM call with the **Phase 2 systemInstructionsTemplate rendered VERBATIM** + temperature 0 + `response_format=json_object` call-time signal + a deterministic extraction instruction; defensively parses via `ExtractJSON` (fences / trailing prose) before `json.Unmarshal`.

`renderPhase2SystemPrompt(verdictSchema string) string` — pure helper that interpolates an optional verdict JSON schema into the Phase 2 prompt.

`withJSONObjectResponseFormat() model.Option` — uses `model.WrapImplSpecificOptFn` (the supported eino v0.9.14 escape hatch for provider-specific call-time config).

5 new tests pass: happy path (parsed map + temperature-0 + response_format option propagation + verbatim prompt + schema interpolation), empty finalMessage, nil model, invalid JSON prose, non-object array.

### Task 4 — `cmd/spike-deep` + `eval/golden/deep.jsonl` (operator-gated live spike)

Spike-only binary mirroring the `cmd/spike-history` pattern (mirrored bootstrap helpers, NOT lifted — that refactor is a Phase 13 follow-up). Single subcommand `parse-rate` with flags `--golden`, `--output`, `--cost-limit` (default 5.00), `--seed` (default 42), `--max-iterations` (default 10, clamped to P12-L2 hard cap), `--dry-run`.

- **Dry-run**: loads + validates the golden set + prints `"<N> cases loaded from <path>"` + category breakdown + exits 0 (no env required).
- **Live**: requires `GLM_API_KEY`, `BGE_M3_URL`, `MILVUS_ADDRESS`. Missing → exit 1 with the named vars + the `--dry-run` remediation hint.
- **Per case**: `agent.BuildToolsForSkill` + `agenttools.BuildRetrieveTool` → `runtime.BuildDeepAgent` (P12-L2 MaxIteration hard cap) → drain `AsyncIterator` events → collect last non-empty assistant `Message.Content` → `llm.ExtractJSON` direct pass → on miss → `ExtractVerdictJSONWithStructuredOutput` (12-04 Task 3 fallback) → record parse outcome + cost (call-count estimator: $0.002/call; --cost-limit aborts cleanly preserving JSONL).
- **Output**: per-case JSONL rows + trailing summary row `{type: "summary", direct_rate, total_rate, p12l10_verdict, model_ver, prompt_hash, seed, max_iterations, cost_usd}`.

50-case golden set: **25 multi-hop + 15 plan-style + 10 refusal-required**, distributed per the plan's contract.

4 hermetic tests pass: dry-run exit 0 + "50 cases loaded" message; missing-env exit 1 with named vars; 50-case schema + category distribution; `TestSpikeDeepVerdictParseRateMeasurement` SKELETON (skipped unless `GLM_API_KEY` env; when unskipped asserts P12-L10 escalation floor `direct_rate >= 0.70`).

## Deviations from Plan

### Auto-fixed Issues (Rules 1 + 2 + 3)

**1. [Rule 2 — Security] Restored pending-row 404 moat + idempotency + interrupt_id passthrough + FrozenArgs carry-over in `planApproveHandler`**

The interrupted partial draft had dropped these seams — keeping the plan endpoint as a thinly-modified `approvalPostHandler`-without-the-lookup. The dropped pending-row lookup opened a **T-12-04-2 elevation-of-privilege vulnerability**: the plan endpoint could resolve any pending approval (memory:facts / review_report) regardless of the audit row's target column. Restored to make `planApproveHandler` a verbatim mirror of `approvalPostHandler` with `target="plan_approve"`, as the plan's `<notes>` design rationale requires ("mirrors approvalPostHandler verbatim ... The only addition is the routing path"). Also restored:
- Idempotency probe (`approvalAlreadyResolved`) — 200 `already_resolved` on a second POST.
- FrozenArgs carry-over (HITHL-07 edit-before-approve) — operator's edited `FrozenArgs` win; otherwise the pending row's byte snapshot is carried.
- `interrupt_id` resume target — the FE echoes the eino interrupt UUID from `data-decision.interrupted`; the approvalID is the D11-C4 plan-literal fallback (probe-tested on eino v0.9.14: raw tool_call_id alone re-interrupts).

**2. [Rule 2 — Security] Fixed test for unknown-approval: 404 (not 200/500)**

The interrupted draft's `TestPlanAPIApproveUnknownApproval` accepted 200/500 + an audit row. The plan's Task 1 specification is "POST with valid runID but unknown approvalId → 404" — and T-12-04-2 demands "no resolution audit row written for a forged approvalId". Fixed to expect 404 + assert empty `action_approved` rows.

**3. [Rule 1 — Bug] Restored `planTestManifest` writes + fixed replay test's positional assumption**

- `planTestManifest` was declaring `Writes: ["plan_approve"]` only — `interruptSession` (which drives a scripted `memory_facts_write` tool call to produce a durable checkpoint) failed with `tool memory_facts_write not found in toolsNode indexes`. Added `memory:facts` to the test manifest's writes so the interrupt fixture runs.
- The replay test's final assertion assumed `Todos[0]` is `t1`; under ts-ASC of *latest* writes the order is `t5, t1, t2, t3, t4` (t1's re-upsert at base+5s pushes it after t5's base+4s). Fixed by locating `t1` by ID + asserting the canonical order directly.

**4. [Rule 3 — Blocking issue] `model.WithGenerateConfig` does not exist in eino v0.9.14**

Plan sketched `model.WithGenerateConfig(map[string]any{"response_format": ...})` — that option does not exist in the locked eino version (`components/model/option.go` exposes `WithTemperature` / `WithMaxTokens` / `WithTools` / `WithToolChoice` / `WrapImplSpecificOptFn`). Used the real mechanism `model.WrapImplSpecificOptFn(func(*map[string]any))` — provider adapters that extract impl-specific options (eino-ext openai reads `ResponseFormat` from `Config` at construction; call-time extraction follows the same channel) can honor the flag, and adapters that do not simply ignore it — the JSON-only system prompt remains the primary contract either way. In-code deviation documented at the function site.

### Pre-existing repo-wide gofmt deviations

`gofmt -l` flags 16+ pre-existing files (`audit.go`, `classifier/*.go`, `pipeline.go`, `decisions.go`, `knowledge.go`, `redline.go`, `handlers_session_history.go`, etc.). Likely CRLF on Windows; out of scope per the scope-boundary rule. The two 12-04 files (`plan_api.go` initially missed a trailing newline; fixed) are now gofmt-clean.

## Verification (all 9 phase-level checks green)

1. `go build ./...` → exit 0 ✓
2. `go vet ./...` → exit 0 ✓
3. `go test ./internal/server/... ./internal/llm/... ./cmd/spike-deep/... -count=1 -race` → all PASS ✓
4. `go test -run 'TestPlanAPI|TestExtractVerdictJSONWithStructuredOutput|TestSpikeDeep'` → all PASS ✓
5. `go run ./cmd/spike-deep parse-rate --dry-run --golden=eval/golden/deep.jsonl` → exits 0 with "50 cases loaded" message (verified by TestSpikeDeepDryRun) ✓
6. `grep planGetHandler internal/server/server.go` → line 336 registered ✓
7. `grep -c '"id"' eval/golden/deep.jsonl` → 50 ✓
8. `grep -n 'response_format' internal/llm/extract.go` → 7 hits (comments + code) ✓
9. `wc -l eval/golden/deep.jsonl` → 50 lines ✓

Contract test `go test -tags=contract -run TestNoLegacyInterruptAPI ./integration/contract/...` → PASS (D8-04 verified; plan_api.go does NOT call `tool.StatefulInterrupt` — that lives in 12-03's `hitl_plan_approve.go`).

## Commits Created

| Commit  | Task | Files                                                          |
| ------- | ---- | -------------------------------------------------------------- |
| bc29621 | 1    | internal/server/plan_api.go, plan_api_test.go                  |
| 929b1d7 | 2    | internal/server/server.go                                      |
| 1ffb305 | 3    | internal/llm/extract.go, extract_test.go                       |
| f09bffd | 4    | cmd/spike-deep/main.go, main_test.go, eval/golden/deep.jsonl   |
| (this)  | docs | .planning/phases/12-p8-3-planning-layer/12-04-SUMMARY.md       |

## Risks Carried Forward

1. **RF-5 spike measures parse rate <70% on GLM-5.2.** Mitigated by shipping the structured-output final step in Task 3 + the escalation verdict surfaced by `parseRateRow.P12L10Verdict`. If live rate is <70%, 12-08 acceptance gate escalates to verdict-suffix + forced_stop (out of scope for 12-04).
2. **Plan approve happy-path resume target**: when the FE doesn't echo `interrupt_id`, the resume target is the approvalID (D11-C4 fallback). Probe-tested on eino v0.9.14: raw tool_call_id re-interrupts — 12-07 FE must echo the interrupt UUID from `data-decision.interrupted`.
3. **POST approve p99 latency**: `Runner.ResumeWithParams` may take 5+ seconds (re-hydrate + re-run from interrupt point). HTTP blocks until resume returns. 12-08 acceptance gate measures p99; if >5s, add SSE streaming response (Phase 999.2 backlog).

## Self-Check: PASSED

```
[✓] internal/server/plan_api.go exists (404 lines, 2 files in commit bc29621)
[✓] internal/server/plan_api_test.go exists (7 tests pass)
[✓] internal/llm/extract.go exists (265 lines; 5 new tests pass)
[✓] cmd/spike-deep/main.go exists (796 lines)
[✓] cmd/spike-deep/main_test.go exists (4 tests pass; 1 SKIP — operator-gated live)
[✓] eval/golden/deep.jsonl exists (50 lines, 25/15/10 distribution)
[✓] internal/server/server.go: planGetHandler registered at line 336
[✓] Commit bc29621 verified in git log
[✓] Commit 929b1d7 verified in git log
[✓] Commit 1ffb305 verified in git log
[✓] Commit f09bffd verified in git log
```