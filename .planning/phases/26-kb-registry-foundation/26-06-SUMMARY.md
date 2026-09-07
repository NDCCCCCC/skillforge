---
phase: 26-kb-registry-foundation
plan: 06
subsystem: agent-trust
tags: [kb-registry, citability-gate, hitl-enum, fail-closed, tdd, kb-m-03]
requires:
  - "internal/kb Registry + CitableRole (26-01)"
  - "skill.ResolveTiers delegating to kb.Registry.ResolveReads + slug resolvedTiers (26-04)"
  - "server Deps.KBRegistry (26-04/26-05)"
provides:
  - "ChunkAccumulator citability gate at the Add insertion point — non-authority hits structurally excluded from TierMatched/ChunksByID (KBM-03, T-26-06-1)"
  - "registry-driven refusal suggestion keyword (first authority-role DisplayName; seed copy byte-identical, T-26-06-4)"
  - "hitl memory_facts tier Enum = kb.Registry.ActiveSlugs() construction snapshot + authority-role default, fail-closed on nil registry / no authority KB (T-26-06-2)"
  - "BuildToolsForSkill(..., reg kb.Registry) signature — both production call sites pass a registry, zero nil-escape"
  - "RunnerBuildConfig.KBRegistry + AgentConfig.KBRegistry seams (26-09 consumes at boot)"
affects:
  - cmd/server (26-09 threads the real kb.Open registry into Deps + agent configs)
  - internal/server VerdictPipeline chunksByID — insertion point (b) of Pattern R3 is NOT gated yet (see Threat Flags)
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "insertion-point gate: filter at the physical entrance to the grounding closure, not in the retriever or the prompt (Pattern R3)"
    - "nil = never-bound legacy posture; SetKBRegistry ignores nil so legacy is only expressible as unbound"
    - "snapshot-per-construction: tool schema Enum and retrieve closure both snapshot at build time; runtime KB creation takes effect on Runner rebuild (restart)"
    - "package-level BindKBRegistry mirroring InjectDeps: shared handler singletons bound from the single-threaded Agent-construction path"
key-files:
  created:
    - internal/agent/trust_floor_test.go
  modified:
    - internal/agent/trust_floor.go
    - internal/agent/agent.go
    - internal/agent/runner_setup.go
    - internal/agent/hitl_tools.go
    - internal/agent/hitl_tools_test.go
    - internal/agent/runner_setup_test.go
    - internal/agent/tools_for_channel_test.go
    - internal/agent/fanout_tools_test.go
    - internal/agent/tools/retrieve.go
    - internal/agent/hitl/hitl_memory_facts.go
    - internal/agent/hitl/hitl_memory_facts_test.go
    - internal/agent/hitl/registry.go
    - internal/agent/hitl/registry_test.go
    - internal/agent/hitl/hitl_review_report_test.go
    - internal/agent/hitl/threat_mitigations_test.go
    - internal/server/approval_api.go
    - internal/server/plan_api.go
    - internal/server/approval_api_test.go
    - cmd/spike-deep/main.go
    - cmd/spike-hitl/main.go
decisions:
  - "D26-06-A: the accumulator injection lives at the accumulator's real construction site — (*Agent).Run via AgentConfig.KBRegistry/newRunAccumulator — because BuildRunner constructs NO accumulator (the plan's premise was wrong); the adk chat path is the 11-01 stub and the live trust floor for HTTP chat is the server VerdictPipeline"
  - "D26-06-B: hitl tier vocabulary binds onto the shared handler singleton (hitl.BindKBRegistry, last-write-wins like InjectDeps/WithContext) instead of replacing hitlHandlers values — a per-call fresh MemoryFactsTool would lose the InjectDeps audit hook"
  - "D26-06-C: Info() fails closed when unbound, and executeMemoryFactsWrite refuses to invent a default tier — an unbound tool must never publish an empty Enum nor write into an empty slug"
  - "D26-06-D: server approval/plan resume endpoints thread Deps.KBRegistry into their Runner builds — the new fail-closed seam would otherwise 500 the resume path for any memory:facts Skill"
  - "D26-06-E: T-HITL-07 sha256 baseline re-ranged (263-365 → 320-422) with an unchanged hash — the InvokableRun body did not rotate, only shifted"
metrics:
  duration: ~75 min
  completed: 2026-09-07
  tasks: 2
  commits: 4
  test_functions_added: 17
  files: 21
  insertions: 800
  deletions: 77
status: complete
---

# Phase 26 Plan 06: Agent Citability Gating + Hitl Registry Snapshot Summary

**One-liner:** the citation whitelist is now code-enforced at the `ChunkAccumulator.Add` insertion point (a hit passes only if `kb.Lookup(tier)` succeeds AND its role equals `kb.CitableRole`), and the agent side's last two tier-literal surfaces — the hitl `memory_facts` schema Enum and the refusal-suggestion keyword — are registry snapshots, with `BuildToolsForSkill` failing closed on every path that lacks a registry.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `ChunkAccumulator` gains an unexported `kb kb.Registry` + `SetKBRegistry` (nil ignored — legacy = never-bound) + a `citable` filter applied in `Add` BEFORE dedupe; godoc records the three-layer separation (retrieval per reads / presentation per annotate channel / closure per this gate); `ruleBasedSuggestion(reg, …)` resolves the keyword from the first slug-ascending authority-role DisplayName with nil-safe fallback to `resolvedTiers[0]`; `newRunAccumulator(AgentConfig)` binds `AgentConfig.KBRegistry` at `(*Agent).Run`; `RunnerBuildConfig.KBRegistry` field added | `13a5205` (RED), `157d698` (GREEN) |
| 2 | `hitl.NewMemoryFactsTool(reg)`/`bindKBRegistry`: tier `Enum` = `reg.ActiveSlugs()` snapshot, default = `SlugsByRole(RoleAuthority)[0]`, Desc rewritten to slug vocabulary; `hitl.BindKBRegistry(reg)` (InjectDeps-shaped) invoked by `BuildToolsForSkill` before any `Info()`; `BuildToolsForSkill` signature extended with trailing `reg kb.Registry` + fail-closed when `writes` contains `memory:facts`; both production call sites pass a registry; retrieve-tool Desc reworded (closure logic zero diff); test literals CN → slug | `6672071` (RED), `7f2ac17` (GREEN) |

Trust semantics locked by the 17 new tests: authority-only citability (historical/personal/unknown/deactivated hits never reach `TierMatched`/`ChunksByID`), nil-registry legacy byte-parity, historical-only grounding **refuses** (`TestTrustFloorRefusesWithoutCitable` — the KBM-03 intent, T-26-06-3 accepted), the suggestion keyword is registry data (seed DisplayName keeps the copy byte-identical; an ASCII display name proves it is not a compiled-in literal), and the hitl construction seam fails closed in all three directions (nil registry, no authority-role KB, `BuildToolsForSkill(..., nil)` with a `memory:facts` Skill) while a read-only Skill's nil call stays legal.

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| WSL `go test ./internal/agent/ -run 'TestCitable\|TestTrustFloor\|TestRuleBased\|TestBuildRunner' -count=1 -v` | PASS — 8/8 gate tests + existing trust-floor tests green |
| WSL `go test ./internal/agent/... -count=1` | PASS (agent, callbacks, hitl, tools) |
| WSL `go test ./internal/agent/ ./internal/agent/hitl/ ./internal/agent/tools/ -count=1` | PASS (plan Task 2 verify) |
| WSL `go vet ./internal/agent/...` | CLEAN |
| WSL `go build ./...` | GREEN (exit 0) |
| WSL `go test ./... -count=1` | zero FAIL lines repo-wide (incl. cmd/spike-hitl, cmd/spike-deep, internal/server, internal/eval) |
| `grep -rn "权威\|历史\|个人\|草稿" internal/agent/ --include='*.go' \| grep -v _test` | **0** (CN survives only in test fixtures, per the gate's `_test` exclusion) |
| `grep -rn "BuildToolsForSkill(" internal/ cmd/ --include='*.go'` | 2 production sites (runner_setup.go:382 with `cfg.KBRegistry`; cmd/spike-deep/main.go:458 with `kbReg`) + all test sites 6-arg — zero stale-arity residue |
| `grep -n "BuildToolsForSkill" cmd/spike-deep/main.go` | `nil` was the ruleMatcher arg; the registry slot is `kbReg` (PROVISIONAL (26-06), never nil) |
| `git diff --stat internal/agent/fanout_tools.go` | empty — source zero-diff, test literals migrated only |
| `git status --porcelain eval/` | only the two untracked artifacts that pre-date the plan (`calibration.json`, `report.json`); zero tracked frozen-asset modifications |
| `grep -rn "TODO\|FIXME"` on the touched sources | 0 |

## Deviations from Plan

**1. [Rule 3 - Blocking] The accumulator is not constructed in `BuildRunner` — injection landed at the accumulator's real construction site**
- **Found during:** Task 1
- **Issue:** the plan's action ("BuildRunner 中 accumulator 构造后调用 SetKBRegistry") and acceptance grep (`SetKBRegistry` in runner_setup.go) assume `BuildRunner` builds a `ChunkAccumulator`. It builds none: the accumulator exists in legacy `(*Agent).Run` (internal/agent/agent.go — the path cmd/eval and cmd/spike-history drive), and the adk chat handler is the 11-01 stub with no trust floor.
- **Fix:** `AgentConfig.KBRegistry kb.Registry` + `newRunAccumulator(cfg)` called from `(*Agent).Run` (the gate is live on the eval/spike path); `RunnerBuildConfig.KBRegistry` added for the hitl seam Task 2 threads at :382. `TestBuildRunnerInjectsRegistry` keeps the plan's name and observes the seam helper (the Runner's Agent field is unexported — same rationale as `agentChannelTools`). The runner_setup.go `SetKBRegistry` grep hit is the godoc anchor, not a call; the call lives in agent.go.
- **Commit:** `157d698`

**2. [Rule 3 - Blocking] hitl tools are shared singletons — a constructor parameter cannot deliver the registry**
- **Found during:** Task 2
- **Issue:** the plan says "工具构造函数追加 `reg kb.Registry` 参数", but `MemoryFactsTool` has no constructor; handlers are package-init struct literals (`hitlHandlers`) shared across sessions and mutated by `InjectDeps`/`WithContext`. Replacing them with per-call fresh instances would silently drop the production audit hook.
- **Fix:** `NewMemoryFactsTool(reg)` exists for construction-time fail-closed validation, and the production path binds the shared singleton via `hitl.BindKBRegistry(reg)` (InjectDeps-shaped, last-write-wins, single-threaded Agent-construction path) before any `Info()` publishes a schema. Validation completes before assignment so a failed bind leaves no half-configured tool.
- **Commit:** `7f2ac17`

**3. [Rule 2 - Missing wiring] Server approval/plan resume endpoints needed the registry**
- **Found during:** Task 2 verification
- **Issue:** the new fail-closed seam makes `BuildRunner` reject any `memory:facts` Skill when `KBRegistry` is unset — including the two server resume endpoints, which would have turned a working approval flow into a 500.
- **Fix:** both configs thread `KBRegistry: s.KBRegistry` (the field 26-04/26-05 already put on `Deps`); fixtures set `testKBReg()`. `TestPlanAPIApproveExpiredCheckpoint`, `TestCancelPropagationMidToolRunner`, `TestDataDecisionExtensionNoNewChunkTypes` all green again.
- **Commits:** `7f2ac17`

**4. [Rule 3 - Blocking] Compile/behavior cascade into files outside the 13-file list**
- **Found during:** Tasks 1–2
- **Issue:** signature + fail-closed changes touch test/binary call sites the plan did not enumerate: `runner_setup_test.go`, `tools_for_channel_test.go` registry fixtures, `cmd/spike-hitl/main.go` (two `BuildRunner` configs), `internal/agent/hitl/hitl_review_report_test.go` (shared-singleton `Info()` needed a bind), and `internal/agent/hitl/threat_mitigations_test.go` (line-range baseline).
- **Fix:** threaded `agentTestKBReg()`/`testKBReg()` fixtures; spike-hitl got a `smokeKBRegistry()` PROVISIONAL (26-06) seed snapshot with a cwd-walking `locateSeedFile` (mirroring spike-fanout); the review-report test binds before `Info()`; T-HITL-07 re-ranged 263-365 → 320-422 with the **hash unchanged** (re-range only, 14-02 precedent — the InvokableRun body did not rotate).
- **Commits:** `7f2ac17`

**5. [Form] `internal/agent/tools/retrieve_test.go` and `internal/agent/fanout_tools_test.go` locations**
- The plan lists `internal/agent/tools/retrieve_test.go`, which does not exist — the retrieve-tool assertions live in `internal/agent/tools_for_channel_test.go` and `internal/agent/agent_test.go` (package `agent_test`). Only the tool Desc wording needed updating (no test asserted the old phrasing). `fanout_tools.go` is `internal/agent/fanout_tools.go`, as the plan's zero-diff requirement assumed.

**6. [Form] hitl test tier literals migrated to slugs, including `registry_test.go`**
- `registry_test.go` was not in the plan's file list but carries a `tier` fixture value; migrated alongside the memory_facts literals so the whole hitl test package speaks slug vocabulary.

## Notes for Downstream Plans

- **26-07 (eval golden re-sign):** the agent-side refusal copy is now registry-DisplayName-driven; with the controlled seed the strings are byte-identical to pre-26-04 behavior. `cmd/eval` and `cmd/spike-history` do NOT yet pass `AgentConfig.KBRegistry` (nil = legacy `resolvedTiers[0]` fallback = today's slug string) — if 26-07 wants the eval harness to exercise the gate, wire the registry there and re-baseline any suggestion assertion in the same commit.
- **26-09 (kb.Open wiring):** three new PROVISIONAL anchors join the 26-03/26-04 families: `cmd/spike-deep/main.go:244`, `cmd/spike-hitl/main.go:298` (both `PROVISIONAL (26-06)`), plus the existing runner/agent config fields to feed (`RunnerBuildConfig.KBRegistry`, `AgentConfig.KBRegistry`, `Deps.KBRegistry` already threaded through approval/plan endpoints). `grep -rn "PROVISIONAL (26-" cmd/` now finds eight sites.
- **26-10 (zero-hardcode audit):** `internal/agent` source is CN-literal-free and Enum-free; remaining tier vocabulary in the agent tree is test fixtures (`testKBReg`/`agentTestKBReg` in packages `agent`, `agent_test`, `hitl`) plus spike-hitl's seed-file read (no new hardcode).
- **Server VerdictPipeline (insertion point (b)):** `internal/server/pipeline.go:465` still builds `chunksByID` from every retriever hit without the `CitableRole` filter, and no phase-26 plan file list owns it. The agent path is gated; the HTTP chat pipeline is not. Flagged in Threat Flags for the phase verifier to route.

## Known Stubs

None. The two `PROVISIONAL (26-06)` seed snapshots are deliberate handoff anchors (26-09 replaces them with `kb.Open`), each constructing a fully functional read-only registry; `grep -rn "TODO\|FIXME"` on the touched files returns nothing.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-06-1 (EoP: non-authority chunk into evidence) | `citable` gate at `Add`: `Lookup` failure or `Role != kb.CitableRole` structurally excludes the hit from `TierMatched`/`ChunksByID` (`TestCitableGateDropsNonAuthority`, `TestCitableGateUnknownTierDropped`, `TestCitableGateDeactivatedDropped`); non-citable material keeps its own annotate channel |
| T-26-06-2 (EoP: hitl tier Enum over-broad) | Enum = `ActiveSlugs()` snapshot + authority default; nil registry / no authority KB / unbound `Info()` / unbound default all fail closed (`TestMemFactsNilRegistryFailsClosed`, `TestMemFactsNoAuthorityRoleFailsClosed`, `TestBuildToolsForSkillNilRegistryFailsClosed`, `TestMemFactsWriteUsesSlug`) |
| T-26-06-3 (DoS: citable gate over-refuses) | accepted and locked: `TestTrustFloorRefusesWithoutCitable` pins historical-only grounding → refusal as the intended KBM-03 semantics; `TestTrustFloorPassesWithCitable` proves one authority hit still satisfies the floor; frozen compliance reads are authority-only so the baseline is untouched (verified: zero frozen-asset diffs) |
| T-26-06-4 (Repudiation: suggestion-copy drift) | keyword = registry DisplayName; `TestRuleBasedSuggestionUsesDisplayName` pins the seed byte-identity AND proves registry-drivenness with an ASCII-named registry; nil-registry fallback keeps today's `resolvedTiers[0]` string |
| T-26-SC (dependency legitimacy) | zero new dependencies — `go.mod` untouched |

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: ungated_pipeline_chunksByID | internal/server/pipeline.go | The VerdictPipeline insertion point (b) of Pattern R3 still admits non-authority hits into `chunksByID` (:465). Out of 26-06's file list and unowned by any phase-26 plan — the agent path is gated, the HTTP chat pipeline is not. Needs an owner (phase verifier routing or a follow-up plan). |

## Self-Check: PASSED

- Commits exist: `13a5205`, `157d698`, `6672071`, `7f2ac17` — all FOUND in `git log 6a375b9..HEAD` (21 files, +800/−77).
- TDD gate sequence: `test(26-06)` RED `13a5205` precedes `feat(26-06)` GREEN `157d698` (Task 1); RED `6672071` precedes GREEN `7f2ac17` (Task 2).
- Files exist: `internal/agent/trust_floor_test.go` (created), `internal/agent/hitl/hitl_memory_facts.go`, `internal/agent/hitl/registry.go`, `cmd/spike-hitl/main.go` — all FOUND in the diff range.
- Gates re-verified after the final commit: WSL `go test ./... -count=1` zero FAIL, `go build ./...` exit 0, agent-tree CN literal grep 0, `fanout_tools.go` zero diff, eval/ untouched.
