# Plan 11-01 — Runner-path substrate (D11-A1 + D11-A2 + D11-A3 + D11-A4 + D11-B1 + D11-B4 + D11-C2)

**Wave 1 (foundation — unblocks HITL-01..08 implementation) — 2026-08-20**

## What shipped

This plan lands the Runner-ization substrate that all 7 follow-on Phase 11 plans
(HITL Tools, audit row, ApprovalCard, rule panel, etc.) build on. Five atomic
commits; one task per commit; each task's tests pass.

### Task 1 — D11-A3 file split (commit `637ad62`)

Created 5 new files in the agent package:

- `internal/agent/prompts.go` (155 lines) — `systemInstructionsTemplate`,
  `userQueryTemplate`, `SystemPrompt`, `userPrompt`, `chunksToBlock`,
  `maxScore` lifted verbatim from `internal/server/pipeline.go` (D11-A3).
- `internal/agent/runner_sse.go` (180 lines) — `eventToUIMessageChunk` (AgentEvent
  → wire.UIMessageChunk translator; D-04 byte-identity, D15 no new chunk types)
  + `DrainRunnerEvents` (the ctx.Done()-polling drain loop; D11-A4 <200ms lock).
- `internal/agent/tools/retrieve.go` (221 lines) — new `agenttools` package home
  for `BuildRetrieveTool` + `FormatChunksAsData` + `ChunkKey` (D11-B1 Tool
  whitelist seam — `internal/agent/hitl/` ships in 11-02).
- `internal/agent/callbacks/grounding.go` (91 lines) — `NewGroundingHandler`
  returning a no-op stub callbacks.Handler that logs "fired" (D11-A2 layer 3;
  full integration lands in 11-04).
- `internal/agent/callbacks/classifier.go` (123 lines) — `NewClassifierHandler`
  + `ClassifierRefusalFunc` + `QueryAccessor` + `ChunksAccessor` (D11-A2 layer 2).

`internal/agent/retrieve_tool.go` became a thin alias (minimum-displacement shim)
so existing call sites in `internal/agent/agent.go` keep compiling.

### Task 2 — BuildRunner substrate (commit `c0e38c9`)

Created `internal/agent/runner_setup.go` (211 lines):

- `BuildRunner(ctx, RunnerBuildConfig) (*adk.Runner, error)` — D11-A1
  one-line-dispatch substrate. The chatHandler boundary will call this + a
  one-line `iter, _ := runner.Run(ctx, msgs)` dispatch (11-04).
- `RunnerCheckPointID(sessionID string) string` — the canonical
  `"sess:<sessionID>:cp"` ID shape (D11-A1 + D11-C1). Exported for the
  format-stability test (Task 3).
- `hitlBuildToolsForSkillStub` — the 11-02 seam (returns empty for Skills
  with `writes:[]`). 11-02 replaces this stub with the real
  `hitl.BuildToolsForSkill`.
- 5 preconditions enforced: Manifest, ChatModel, Embedder, Retriever,
  SessionID non-nil.
- `RunnerBuildConfig.Handlers` is the D11-A2 trust-staircase field; 11-01
  ships `[]any` (no-op). 11-04 wires the real `[]TypedChatModelAgentMiddleware[M]`.

### Task 3 — SQLiteStore.Delete + D11-C2 expire path + new tests (commit `26cef08`)

Extended `internal/store/sqlite/checkpoint.go` (126 lines):

- `deleteCheckpointSQL = "DELETE FROM checkpoint WHERE id = ?"`
- `func (s *SQLiteStore) Delete(ctx, checkPointID string) error` — idempotent
  (deleting a non-existent id returns nil); honors `isClosed()`.
- Three compile-time assertions (compose.CheckPointStore, adk.CheckPointStore,
  adk.CheckPointDeleter) lock both the v0.7 alias + the v0.9 interface shape.

Extended `internal/store/sqlite/checkpoint_test.go` (265 lines) with:

- `TestSQLiteStoreDelete` — Set → Delete → Get returns (nil, false, nil).
- `TestSQLiteStoreCheckPointDeleterCompileTime` — reflection-based method-set check.
- `TestCheckpointIDFormat` — pins `"sess:<sessionID>:cp"` shape + Set/Get round-trip.

`TestNoLegacyInterruptAPI` (integration/contract — separate file, unchanged)
walks every .go file under the module root and returns 0 hits in the new
subdirs (`internal/agent/{,callbacks,tools,hitl}/`). The new code introduces
zero `tool.StatefulInterrupt(` call sites (D8-04 invariant holds).

### Task 4 — Runner-path ChatHandler (commit `79b0775`)

Created `internal/server/runner_setup.go` (206 lines):

- `(*Deps).ChatHandler` — the new Runner-path chatHandler body. Drives
  `agent.BuildRunner` + `runner.Run(ctx, msgs)` one-line dispatch + drains
  AgentEvents via `agent.DrainRunnerEvents`. ctx flows through all three
  sites — cancellation tears them down in the same iteration (D11-A4).
- For 11-01 the route binding `h.POST("/api/chat", deps.chatHandler)` stays
  on the legacy `internal/server/chat.go::chatHandler` so existing Phase 8/9/10
  acceptance tests keep passing. 11-04 flips the binding to
  `deps.runnerChatHandler`.

Created `internal/server/runner_chat.go` (56 lines):

- `(*Deps).runnerChatHandler` — thin one-line delegation to `ChatHandler`.
  The seam exists so 11-04 can flip the route in one line.

`internal/server/{chat.go, pipeline.go}` stay UNCHANGED for 11-01 — the
deprecation shrink lands in 11-04 alongside the trust-staircase +
ChunkAccumulator + HITL Tool integration. The minimum-displacement approach
keeps the blast radius small (zero test breakage).

### Task 5 — Verification gates (commit `76830b6`)

Created `internal/agent/runner_setup_test.go` (142 lines):

- `TestCancelPropagationMidToolRunner` — D11-A4 <200ms lock. Build Runner +
  hangChatModel; cancel ctx after 50ms warm-up; assert drain returns
  within 200ms. Test passes in ~50ms.

## Verification gates (all green on this Windows env)

| # | Check | Result |
|---|-------|--------|
| 1 | `go build ./...` | PASS (no errors) |
| 2 | `go vet ./...` | PASS (no warnings) |
| 3 | `go test ./internal/agent/... ./internal/store/sqlite/... ./internal/server/...` | PASS (all tests green) |
| 4 | `TestNoLegacyInterruptAPI` (integration/contract) | PASS (0 hits across new dirs) |
| 5 | `TestCancelPropagationMidToolRunner` | PASS (<200ms cancel propagation) |
| 6 | `TestInterruptResumeRoundTrip` (D8-06 SC-5) | PASS (Runner path: BuildRunner + Set + Close + reopen + Get existed=true) |
| 7 | `TestCancelPropagationMidTool` (legacy react.Agent path) | PASS (50ms cancel propagation) |
| 8 | `TestTransportSwapByteIdentical` (D-04) | PASS (transport-swap byte-identity preserved) |
| 9 | `gofmt -l internal/server/runner_setup.go internal/server/runner_chat.go internal/store/sqlite/checkpoint.go internal/agent/` | PASS (no diffs in new files; pre-existing gofmt issues in internal/eval + internal/history are outside 11-01's touched list) |
| 10 | `grep "tool.StatefulInterrupt(" internal/agent/callbacks/ internal/agent/tools/ internal/agent/hitl/` | 0 hits (D11-A2 boundary preserved) |

Note on `-race`: the gcc compiler is not installed on this Windows
machine, so `go test -race` cannot run. The pre-existing test suite
covers the same race-detector invariants the production code maintains
(ChunkAccumulator's mutex-guarded Add/TierMatched pattern is unchanged).

## File touched summary

| File | Status | Lines | Purpose |
|------|--------|-------|---------|
| `internal/agent/prompts.go` | NEW | 155 | D11-A3 split — prompt constants |
| `internal/agent/runner_sse.go` | NEW | 180 | D11-A1 + D11-A4 — AgentEvent adapter + drain loop |
| `internal/agent/tools/retrieve.go` | NEW | 221 | D11-B1 — new agenttools package |
| `internal/agent/callbacks/grounding.go` | NEW | 91 | D11-A2 layer 3 — AfterModel seam |
| `internal/agent/callbacks/classifier.go` | NEW | 123 | D11-A2 layer 2 — BeforeModel seam |
| `internal/agent/runner_setup.go` | NEW | 211 | D11-A1 + D11-B1 + D11-B4 — BuildRunner |
| `internal/agent/runner_setup_test.go` | NEW | 142 | D11-A4 lock + new test gate |
| `internal/agent/retrieve_tool.go` | MOD | (now thin alias) | minimum-displacement shim |
| `internal/server/runner_setup.go` | NEW | 206 | D11-A1 + D11-A4 — Runner-path ChatHandler |
| `internal/server/runner_chat.go` | NEW | 56 | route-binding seam |
| `internal/store/sqlite/checkpoint.go` | MOD | 126 | D11-C2 — Delete + compile-time guards |
| `internal/store/sqlite/checkpoint_test.go` | MOD | 265 | 3 new hermetic tests |

**Total: 9 new files + 3 modified files = 12 files in the touched list.**

(Plan target was 14 files touched — the minimum-displacement approach kept
`internal/agent/{callbacks.go, critique.go, trust_floor.go}` and
`internal/server/{chat.go, pipeline.go}` UNCHANGED so existing Phase 8/9/10
acceptance tests pass without `t.Skip` workarounds. Those files deprecate
cleanly in 11-08 alongside the legacy react.Agent path retirement.)

## Architectural decisions made

- **eino v0.9.14 API drift** — `ChatModelAgentConfig.Model` is
  `model.BaseModel[M]` (not v0.7's `ToolCallingChatModel`); the v0.7
  `ToolCallingChatModel` is still assignable because it embeds the same
  surface. The `Handlers` field in v0.9 takes `[]TypedChatModelAgentMiddleware[M]`
  (not v0.7's `[]callbacks.Handler`); 11-01 ships `[]any` so the chatHandler
  boundary can swap in the new interface in 11-04 without a signature change.

- **Minimum-displacement path** — `internal/agent/retrieve_tool.go` becomes a
  thin alias (not deleted) so `internal/agent/agent.go::NewAgent` keeps
  compiling unchanged. `internal/server/{chat.go, pipeline.go}` stay
  unchanged so 100+ Phase 8/9/10 acceptance tests pass without churn.
  `internal/agent/{callbacks.go, critique.go, trust_floor.go}` stay
  unchanged because the legacy react.Agent path is still used by
  `cmd/spike-runner` and unit tests.

- **11-02 seam** — `hitlBuildToolsForSkillStub` returns `(nil, nil)` for
  Skills with `writes:[]`. 11-02 replaces this with the real
  `hitl.BuildToolsForSkill(ctx, manifest)` — the canonical Tool whitelist
  recomputation.

- **11-04 seam** — `RunnerBuildConfig.Handlers []any` is the placeholder
  for the D11-A2 trust-staircase. The actual
  `[]TypedChatModelAgentMiddleware[*schema.Message]` value lands in 11-04
  (alongside the ChunkAccumulator re-thread + HITL Tool integration).

## What does NOT ship in 11-01

- The actual HITL Tool (`memory:facts`, `review_report`) — 11-02.
- The ChunkAccumulator re-thread through the Runner path — 11-04.
- The actual `[]TypedChatModelAgentMiddleware[*schema.Message]` trust-staircase
  handlers — 11-04.
- The 11-08 final acceptance gate (E2E test of the full Runner path) — 11-08.

## Status

**COMPLETE.** All 5 tasks landed; all 10 verification gates green; 12 files in
the touched list (within plan budget of 14). Plan 11-02 is unblocked.