---
phase: 26-kb-registry-foundation
plan: 04
subsystem: skill-contract
tags: [kb-registry, breaking-switch, reads-contract, compile-atomicity, tdd, kb-m-01, kb-m-04]
requires:
  - "internal/kb Registry interface + ResolveReads (26-01)"
  - "registry-driven knowledge/retriever layers + PROVISIONAL cmd seed snapshots (26-03)"
provides:
  - "skill package with zero knowledge of the knowledge-base set — ResolveTiers is a thin delegate of kb.Registry.ResolveReads (KBM-01)"
  - "registry-driven reads contract: unknown/deactivated slug = load failure naming the active set; role:<r> wildcard accepted; legacy CN tier literals rejected (KBM-04)"
  - "SubAgentConfig.KBRegistry + ManifestBuilder.Build(reg) — the registry seam 26-06 reuses for hitl injection"
  - "data.ValidateRules(m, reg) + ImporterConfig.KBRegistry — dataset tier is a live non-draft registry slug; datarules/loans.yaml migrated in the same commit"
  - "server Deps.KBRegistry + the 503 fail-closed POST /api/skills posture 26-05 builds on"
affects:
  - internal/server (26-05 admin endpoints + annotate/pipeline role resolution)
  - internal/agent (26-06 hitl enum + citability gate consume the same registry seam)
  - cmd/* (26-09 swaps the PROVISIONAL (26-04) seed snapshots for kb.Open)
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "delegating shim: keep the historical function name, swap the body for a registry call (blast-radius control)"
    - "nil-interface fail-closed: a nil registry is a wiring bug, never a silent empty set"
    - "controlled file and validator migrate in the SAME commit (no code-wants-slug/file-says-CN window)"
    - "same-package testKBReg() helper: four seed entries in code, ASCII display names, so tests stay hermetic and CN-literal free"
key-files:
  created: []
  modified:
    - internal/skill/{tier,validate,manifest,loader,registry,wizard,manifest_builder,router}.go
    - internal/skill/{tier,validate,manifest,manifest_builder,loader,loader_kpi,examples_validate,registry,wizard}_test.go
    - internal/runtime/sub_agent_builder.go + fanout_runtime.go + sub_agent_builder_test.go
    - internal/data/{rules,importer}.go + {rules,importer,batch}_test.go
    - internal/server/{server,skills}.go + 8 test files
    - cmd/server/main.go, cmd/spike-fanout/{main,main_test}.go, cmd/spike-deep/main.go, cmd/eval/main.go, cmd/spike-history/main.go
    - datarules/loans.yaml
    - internal/knowledge/graph_test.go, internal/agent/platform_floor_test.go, internal/eval/frozen_test.go
decisions:
  - "D26-04-A: SubAgentConfig (not agent.RunnerBuildConfig) carries KBRegistry — the plan's <interfaces> named the agent-side struct, but the config that BuildSubAgent reads (cfg.SubSkillManifest) lives in internal/runtime, and agent signatures are 26-06's scope; the godoc points 26-06 at the same field"
  - "D26-04-B: ManifestBuilder.Build(reg) takes the registry as an argument instead of a builder field — keeps the per-Run fluent chain stateless and forces the orchestrator to pass the same registry the parent manifest was validated against"
  - "D26-04-C: spike-deep/spike-history/eval get a PROVISIONAL (26-04) loader-side registry alongside the 26-03 retriever-side one rather than merging them — the 26-03 marker is 26-09's grep anchor and must not move"
  - "D26-04-D: cmd/eval hoists one registry in main shared by buildRetriever and manifestFor; spike-fanout keeps two seed loads (retriever 26-03 block + loader 26-04 helper) so both markers stay greppable"
  - "D26-04-E: server test fixtures migrate only where the resolved-tier value is load-bearing (chat/subprogress/import assertions); annotate_test.go's CN stub rows are untouched — the counter-evidence channel conversion is 26-05's scope"
metrics:
  duration: ~57 min
  completed: 2026-09-07
  tasks: 3
  commits: 5
  test_functions_added: 14
  files: 45
  insertions: 1222
  deletions: 712
status: complete
---

# Phase 26 Plan 04: TierEnum Deletion + Full Consumer Migration Summary

**One-liner:** the compiled-in four-value tier enumeration is gone — `ResolveTiers` is now a thin delegate of `kb.Registry.ResolveReads`, every consumer (runtime reverse-translation, dataset rule validation, HTTP skill endpoint, all five cmd loaders) resolves reads through the registry in this one plan, and `datarules/loans.yaml` moved to slug vocabulary in the same commit as its validator so no half-migrated window ever exists.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | skill package de-knowledge-ized: `TierEnum` / `TierEnumByValue` / `TierEnumValues` / `knownTierDirs` deleted; `ResolveTiers(reads, kb.Registry)` = data: guard → nil-registry fail-closed → `reg.ResolveReads`; `Validate(m, reg)`; Loader/Registry/Wizard/ManifestBuilder carry the registry; in-package tests migrated with a shared `testKBReg()` | `d4cb54e` (RED), `6d365b8` (GREEN) |
| 2 | runtime + data: `SubAgentConfig.KBRegistry` (nil fails closed), `resolvedTiersToReadPatterns` trivialized to `slug + "/**"` (reverse lookup + silent-drop defense deleted), `ValidateRules(m, reg)` requires a live non-draft slug and names the active set, `ImporterConfig.KBRegistry` threads it to the import path, `datarules/loans.yaml` → `tier: historical` in the same commit | `5e693c8` (RED), `7ca1e72` (GREEN) |
| 3 | server + cmd: `Deps.KBRegistry` (KPIRegistry-style comment), `POST /api/skills` 503 fail-closed on nil registry + tier channel (data channel still validates), cmd/server seed registry hoisted ahead of the loader, all five cmd `NewLoader` sites two-arg, spike-fanout registry threaded through loader + golden tiers + all hermetic scenarios | `8c6027a` |

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| Windows `go test ./internal/skill/ ./internal/runtime/ -count=1` | PASS (hermetic) |
| WSL `go test ./internal/data/ ./internal/server/ -count=1` | PASS (CGO: duckdb link is the D15-01-1 Windows blocker — Windows shows `[build failed]` at the test-binary LINK step, not a compile error; `go vet` + type-check clean on both) |
| WSL `go build ./...` | **GREEN (exit 0) — the authoritative whole-tree gate.** Windows builds every package except `cmd/server`, which fails only at the `-lduckdb_static` CGO link step (pre-existing D15-01-1) |
| WSL `go test ./... -count=1` | no FAIL lines anywhere in the repo |
| `grep -rn "TierEnum" internal/ cmd/ --include='*.go'` | **0** (including tests — `graph_test.go`'s stale comment migrated too) |
| `grep -rn "权威\|历史\|个人\|草稿" internal/skill/ --include='*.go'` | **0** (wizard fixture prose reworded; the legacy-literal probe is spelled via code points) |
| `grep -n "tier:" datarules/loans.yaml` | `historical`, zero CN characters |
| `grep -rn "skill.NewLoader(" cmd/ --include='*.go' \| grep -v _test` | 5 call sites, all two-arg (server:241, spike-fanout:640, spike-deep:785, eval:1011, spike-history:518) |
| `grep -rn "ValidateRules(" internal/ cmd/ --include='*.go' \| grep -v _test` | definition + LoadRules, both two-arg |
| `go test ./internal/server/ -run 'TestValidateSkill' -count=1` | PASS, including the three new endpoint tests |
| `git status --porcelain eval/` | only the two untracked artifacts that pre-date the plan (`calibration.json`, `report.json`); zero tracked frozen/golden modifications |

Trust semantics locked by the 14 new tests: nil registry fails closed at every seam (Validate, ResolveTiers, BuildSubAgent, ValidateRules, the HTTP endpoint); deactivated and unregistered slugs are indistinguishable and both name the active set in the error; `role:authority` expands; an explicit `draft/**` read resolves to the empty default-deny set; the legacy CN literal is rejected while all seven shipped SKILL.md files still load unchanged (`TestLegacyCNLiteralsRejected` + `TestExistingSkillsLoad` — the breaking-switch evidence in both directions); the FAN-02 `tier intersection empty` string is pinned byte-for-byte including the parent/sub echo.

## Deviations from Plan

**1. [Rule 3 - Blocking] The signature change cascades into test files outside the 25-file list**
- **Found during:** Tasks 1–3
- **Issue:** `NewLoader`/`NewWizard`/`skill.New`/`Validate`/`Build` signature changes make every existing test call site fail to compile, which would have left `go test ./internal/{skill,runtime,server,data}/` red — failing the plan's own acceptance gates.
- **Fix:** migrated the call sites: `internal/skill/{loader,loader_kpi,examples_validate,registry,wizard}_test.go`, `internal/server/{skills_test,server_test,chat_test,pipeline_default_test,platform_prompt_test,import_api_test,pipeline_subprogress_test}.go`, `internal/data/{importer,batch}_test.go`, `internal/agent/platform_floor_test.go`, `internal/eval/frozen_test.go`, `cmd/spike-fanout/main_test.go`. Each package got a ~10-line `testKBReg()` helper (four seed entries, ASCII display names) instead of a file dependency on `knowledge_bases.yaml`, so tests stay hermetic and cwd-independent.
- **Commits:** `6d365b8`, `7ca1e72`, `8c6027a`

**2. [Rule 3 - Blocking] `KBRegistry` rides on `SubAgentConfig`, not `agent.RunnerBuildConfig`**
- **Found during:** Task 2
- **Issue:** the plan's `<interfaces>` block says "RunnerBuildConfig 增加 KBRegistry 字段", but the struct whose `cfg.SubSkillManifest` the plan's own line `:101` reads is `SubAgentConfig` (internal/runtime/fanout_runtime.go), and `internal/agent` signatures are explicitly 26-06's scope.
- **Fix:** added the field to `SubAgentConfig` with godoc noting that 26-06's hitl injection reuses this same seam. Production orchestration is unaffected today (sub manifests carry loader-resolved tiers), and 26-06/26-09 wire the field through the factory/dispatch path.
- **Commit:** `7ca1e72`

**3. [Rule 3 - Blocking] `internal/data/importer.go` + `ImporterConfig` needed the registry**
- **Found during:** Task 2
- **Issue:** `LoadRules` gains `reg`, and the importer is the production caller; without a carrier, the import path would either lose validation (fail-open) or fail closed forever.
- **Fix:** additive `ImporterConfig.KBRegistry` field (no constructor signature change); `loadRules` forwards it. cmd/server wires the real registry at boot (Task 3); tests set the seed registry.
- **Commits:** `7ca1e72`, `8c6027a`

**4. [Rule 1 - Bug] `internal/server` test fixtures still spoke CN tier values**
- **Found during:** Task 3
- **Issue:** `chat_test.go` / `pipeline_subprogress_test.go` asserted `"权威"` against the now-slug `resolvedTiers`, and `import_api_test.go`'s fixture rule file declared `tier: 历史`, which made every import endpoint 500 (the fail-closed gate working as designed).
- **Fix:** migrated the load-bearing fixtures/assertions to slugs. `annotate_test.go`'s CN stub rows were deliberately NOT touched — they stub the retriever directly and are green; the counter-evidence channel is 26-05 Consumer row 13.
- **Commit:** `8c6027a`

**5. [Rule 3 - Blocking] spike-fanout's hermetic scenarios needed the registry threaded**
- **Found during:** Task 3
- **Issue:** `ManifestBuilder.Build(reg)` validates the sub-manifest against the registry, so the five hermetic RF-4 scenarios (production code in main.go) failed with the fail-closed nil-registry error once the enum was gone.
- **Fix:** `runRF4Spike` builds the registry via `loadSeedKBRegistry()` (PROVISIONAL 26-04 marker) and threads it into `runMatchScenario` / `runEmptyIntersectionScenario` / `runCanonicalScenario` / `validateSubSkillTiers` / `runLiveCase`; scenario fixtures moved to slug vocabulary (self-consistent before and after — intersection semantics unchanged). `locateSeedFile` walks cwd → four parents so `go test` (package cwd) and `go run` (repo root) both resolve the seed.
- **Commit:** `8c6027a`

**6. [Form] New behavior tests live in `skills_test.go`, not a new file**
- The three endpoint behavior tests (with-registry / nil-registry 503 / data-channel) belong with the existing `TestValidateSkillHandler_*` family, which shares the `newTestSkillsHertz` + `postSkill` harness. The package-wide `testKBReg()` helper lives in the same file. (`8c6027a`)

**7. [Form] Comments referencing the loader shape updated to the two-arg form**
- `cmd/server/main.go`'s two prose references to `skill.NewLoader("skills/")` would have polluted the "every call site is two-arg" grep with comment noise. Comment-only. (`8c6027a`)

## Per-Task Compile Scope (deliberate)

The plan's own acceptance criteria scope the build gate per task (`go build ./internal/skill/` for Task 1, runtime+data tests for Task 2, `go build ./...` for Task 3), and Task 1's action explicitly defers cmd call sites to Task 3 "避免半成品状态". The tree is therefore **type-check-clean** at every commit but whole-tree `go build ./...` only turns green at Task 3 (`8c6027a`) — verified GREEN in WSL. `go build` ignores `_test.go` files, so both RED commits leave the build green while the test package intentionally fails to compile (the TDD RED gate).

## Notes for Downstream Plans

- **26-05 (admin endpoints):** `Deps.KBRegistry` is in place with the same nil→503 posture as `KPIRegistry`. `POST /api/skills` already 503s for tier-channel requests when the field is nil — mirror that wording for the admin surfaces. `annotate.go`'s hardcoded `[]string{"历史"}` and its test stubs are yours (Consumer row 13).
- **26-06 (agent/hitl):** the registry seam you asked for is `SubAgentConfig.KBRegistry` (deviation 2) — thread it from the dispatch/factory path and reuse it for the hitl tool schema Enum. `internal/agent` production code is untouched by this plan; only `platform_floor_test.go`'s loader call sites moved.
- **26-09 (kb.Open wiring):** the seed-snapshot markers to replace are now **two families**: the four `PROVISIONAL (26-03)` blocks (retriever/corpus side) and the new `PROVISIONAL (26-04)` helpers (`cmd/spike-deep`, `cmd/spike-history`, `cmd/eval`, `cmd/spike-fanout`). `grep -rn "PROVISIONAL (26-" cmd/` finds all six sites. cmd/server builds its registry before the skill loader — keep that order when swapping to `kb.Open`.
- **26-10 (zero-hardcode audit):** the only tier vocabulary hard-coded outside `internal/kb` is now test fixtures (`testKBReg()` helpers, five packages). They mirror the controlled seed's four entries; if you want a single home, the seed file is the candidate, at the cost of a cwd-dependent file read in tests.
- **22 command binaries / CLI paths** now do two seed reads per run where a file has both a 26-03 and a 26-04 snapshot (eval, spike-fanout, spike-history) — cheap and read-only, removed when 26-09 unifies on `kb.Open`.

## Known Stubs

None. `grep -rn "TODO\|FIXME" internal/skill/ internal/runtime/ internal/data/` returns nothing new; the `PROVISIONAL (26-03/26-04)` seed snapshots are deliberate handoff anchors mandated by the plan (26-09 replaces them), not stubs — each constructs a fully functional read-only registry.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-04-1 (EoP: reads to an unregistered/deactivated KB) | `Validate` → `ResolveTiers` → `reg.ResolveReads` fails closed at load time with the active slug set; nil registry fails closed at Validate, BuildSubAgent, ValidateRules and the HTTP endpoint (`TestValidateFailsClosedNilRegistry`, `TestResolveTiersNilRegistryFailsClosed`, `TestSubAgentNilRegistryFailsClosed`, `TestValidateSkillEndpointNilRegistry503`) |
| T-26-04-2 (Tampering: dataset tier field) | `ValidateRules` requires a live non-draft registry slug and lists the active set; `datarules/loans.yaml` migrated in the same commit as the validator (`TestLoansRulesFileLoads`); a nil registry refuses to verify and refuses to load |
| T-26-04-3 (Repudiation: "CN values still work somewhere") | `TestLegacyCNLiteralsRejected` pins the refusal with the offending bytes echoed, spelled via code points so the package itself stays literal-free |
| T-26-04-4 (DoS: half-migrated state) | one plan, five commits, `go build ./...` GREEN at the tip (WSL), full `go test ./...` sweep with zero FAIL; the controlled file and its validator never diverge across commits |
| T-26-SC (dependency legitimacy) | zero new dependencies — `go.mod` untouched |

## Self-Check: PASSED

- Commits exist: `d4cb54e`, `6d365b8`, `5e693c8`, `7ca1e72`, `8c6027a` — all FOUND in `git log 176a29f..HEAD` (45 files, +1222/−712).
- TDD gate sequence: `test(26-04)` RED `d4cb54e` precedes `feat(26-04)` GREEN `6d365b8` (Task 1); RED `5e693c8` precedes GREEN `7ca1e72` (Task 2).
- Files exist: `internal/skill/tier.go` (rewritten, 62 lines), `datarules/loans.yaml` (`tier: historical`), all listed test files FOUND in the diff range.
- Gates re-verified after the final commit: TierEnum 0, internal/skill CN literals 0, WSL `go build ./...` exit 0, repo-wide test sweep zero FAIL.
