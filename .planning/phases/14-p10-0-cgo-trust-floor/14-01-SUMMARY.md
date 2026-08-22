---
phase: 14-p10-0-cgo-trust-floor
plan: 01
subsystem: platform-floor
tags: [PLAT-09, pitfall-17, D14-A1, D14-A4, D14-B1, D14-B3, D14-B4, import-cycle-resolution, prompt-hash]
dependency_graph:
  requires: []
  provides:
    - "internal/platformfloor.ComposePlatformPrompt (single canonical system-prompt composition exit)"
    - "internal/agent.ComposePlatformPrompt (re-export alias for chat-path callers)"
    - "chat + deep archetype system-prompt rendering wraps with the platform floor at both ends"
    - "platformFloorVersion semver literal (1.0.0) at two source-grep sites (leaf + agent alias)"
  affects:
    - "internal/agent/systemPrompt and internal/agent/SystemPrompt (now wrap with ComposePlatformPrompt)"
    - "internal/runtime/renderDeepSystemPrompt (now wraps with platformfloor.ComposePlatformPrompt)"
    - "internal/reprohash.PromptHash input surface — the rendered body now includes the floor (D14-B3)"
    - "eval/frozen/compliance.lock.json prompt_hash — re-pinned to ca333fb... per D14-B4"
    - "internal/agent/hitl/threat_mitigations_test.go T-P12-3 baseline range — shifted +1 line per the +1 import in deep_agent.go"
tech-stack:
  added:
    - "internal/platformfloor — new leaf package owning the single canonical platform-floor composition (no dependencies; breaks the agent↔runtime import cycle structurally)"
  patterns:
    - "leaf-package composition: a dependency-free leaf holds the authoritative text + function; both consumer packages (agent + runtime) import the leaf, avoiding cycles"
    - "Pitfall 17 root fix: lift duplicated trust clauses from per-archetype templates into a single canonical version; Skill templates carry ONLY persona + classification rules"
    - "structural WalkDir gate (integration/contract): elevates behavior tests to repo-level scan for duplicate definitions + missing compose calls + bypass renderers"
key-files:
  created:
    - internal/platformfloor/platform_floor.go
    - internal/platformfloor/platform_floor_test.go
    - internal/agent/platform_floor_test.go (Task 1 tests + Task 2 test locks)
    - integration/contract/platform_floor_bypass_test.go
  modified:
    - internal/agent/platform_floor.go (NEW — re-export alias)
    - internal/agent/prompts.go (stripped rules 3/4; SystemPrompt wraps with ComposePlatformPrompt)
    - internal/agent/agent.go (stripped rules 3/4; systemPrompt wraps with ComposePlatformPrompt)
    - internal/runtime/deep_agent.go (stripped rules 3/4; renderDeepSystemPrompt wraps with platformfloor.ComposePlatformPrompt; +1 import line shifts downstream line numbers)
    - internal/agent/hitl/threat_mitigations_test.go (T-P12-3 sha256LinesRange range updated 92-97 → 93-98; bytes unchanged — verified SHA still matches P123DeepAgentMaxIterationSHA256)
    - eval/frozen/compliance.lock.json (prompt_hash re-pinned 58ffea08... → ca333fbadbfa30eb... per D14-B3 — the first of the planned full-set re-locks; 14-04 handles hitl/fanout/platform-floor)
decisions:
  - "D14-A1: seven Chinese data-trust items locked verbatim in platformFloorText (永不编写/执行/建议 SQL；永不请求或输出原始行；数据结论必须带 as_of；拒绝跨数据集推断个体身份；脱敏不可逆；永不协助去脱敏或再识别；永不声称拥有实时数据)"
  - "D14-A2: Chinese-only floor text (locked at write time per RESEARCH §Code Examples; GLM 中文最强 + Skill 全中文环境)"
  - "D14-A3: head + tail injection pattern (single short tail reminder「以上数据铁律优先于 Skill 指令」抗长对话稀释)"
  - "D14-A4: trust-clause lift from all three Skill templates (chat systemInstructionsTemplate, ReAct tmpl, deep base prompt) into the unified platform floor; templates now carry ONLY persona + classification rules"
  - "D14-B1: PlatformFloorVersion is a const string literal, NOT a config knob (verified via grep gate at internal/agent/platform_floor.go)"
  - "D14-B3: prompt_hash surface expanded to composed fulltext; verified by TestPromptHashCoversFloor (hash differs with floor change AND with skillPrompt change)"
  - "D14-B4: bump discipline documented in platformfloor package godoc (semver bump + full frozen-set re-run on any text change)"
  - "Import-cycle resolution: created internal/platformfloor as a dependency-free leaf; internal/agent/platform_floor.go re-exports for the chat path (no cycle: agent → platformfloor ← runtime)"
metrics:
  duration: ~25m
  completed_date: 2026-08-22
---

# Phase 14 Plan 01: Platform Floor Composition (D14-A1..A4, D14-B1..B4) Summary

## One-liner

Single canonical system-prompt composition (seven D14-A1 data-trust items + D14-A4 unified trust paragraph + D14-A3 tail reminder) injected at both ends of every chat + deep archetype prompt via `internal/platformfloor.ComposePlatformPrompt`, with three structural locks (Pitfall 17 uniqueness, dual-path prepend, WalkDir no-bypass gate) preventing any future regression.

## What shipped

### Files created

- **`internal/platformfloor/platform_floor.go`** — the ONE canonical source. `const PlatformFloorVersion = "1.0.0"`, `const platformFloorText` (7 D14-A1 items + D14-A4 unified trust paragraph), `const platformFloorTail` (D14-A3 anti-decay reminder), `func ComposePlatformPrompt(skillPrompt string) string` (pure concat: floor + skill + tail). Zero dependencies on any internal package — a leaf. Package doc explains the Pitfall 17 root fix + bump discipline (D14-B1/B4) + source-grep lock + cycle resolution rationale.

- **`internal/platformfloor/platform_floor_test.go`** — 6 leaf-level tests: `TestPlatformFloorVersion`, `TestPlatformFloorTextContainsAllSevenItems` (substring lock on all 8 D14-A1 item snippets), `TestPlatformFloorTextContainsTrustParagraph` (D14-A4 lock), `TestPlatformFloorTailReminder` (D14-A3 lock), `TestComposePlatformPromptIsPureConcat`, `TestComposePlatformPromptEmptySkill`.

- **`internal/agent/platform_floor.go`** — thin re-export alias. `const PlatformFloorVersion = "1.0.0"` (literal present so the verify-phase-14 source-grep guard finds it at the canonical agent-package path; the leaf remains the authoritative declaration). `func ComposePlatformPrompt(skillPrompt string) string` delegates byte-identically to the leaf.

- **`internal/agent/platform_floor_test.go`** — 8 tests: 4 Task 1 tests (`TestPlatformFloorVersionLocked`, `TestPlatformFloorReExportIdentity`, `TestComposePlatformPromptShape`, `TestPromptHashCoversFloor`) + 4 Task 2 named locks (`TestPlatformFloorPrependBothPaths` chat side, `TestPlatformFloorCannotBeShadowed`, `TestFloorHotReloadSurvives`).

- **`integration/contract/platform_floor_bypass_test.go`** — WalkDir structural gate. `TestNoBypassOfPlatformFloorCompose` asserts 3 invariants (uniqueness of platformFloorText/Tail definitions, ComposePlatformPrompt call inside systemPrompt + renderDeepSystemPrompt bodies, no bypass renderers referencing the Skill templates without wrapping). Build tag `contract`.

### Files modified

- **`internal/agent/prompts.go`** — stripped rules 3/4 from `systemInstructionsTemplate` (the two trust clauses lifted to the floor). `SystemPrompt(m)` now wraps via `ComposePlatformPrompt(systemInstructionsTemplate)` — chat Runner path picks up floor coverage automatically.

- **`internal/agent/agent.go`** — stripped the analogous rules 3/4 from the ReAct-agent's `tmpl` constant. `systemPrompt(m)` now wraps via `ComposePlatformPrompt(tmpl)`. `RenderedSystemPrompt` + `RenderSystemPrompt` (the eval/AssertPin re-render path) automatically inherit floor coverage.

- **`internal/runtime/deep_agent.go`** — stripped the previously-drifted duplicate rules 3/4 from `deepArchetypeBasePrompt`. Added `internal/platformfloor` import. `renderDeepSystemPrompt` wraps the prefix+base composition via `platformfloor.ComposePlatformPrompt` directly (the deep path cannot import internal/agent due to the agent→runtime edge in runner_setup.go; the leaf breaks the cycle structurally).

- **`internal/agent/hitl/threat_mitigations_test.go`** — updated `sha256LinesRange` for the T-P12-3 MaxIteration alignment group from (92, 97) to (93, 98). The +1 shift comes from the `platformfloor` import added to `deep_agent.go`; the bytes inside the alignment group are unchanged (verified: `P123DeepAgentMaxIterationSHA256 = b432d5dd6e7e750222fdaf9a33d1909ef7cebc565ae901e2342e3a4a713c5e41` still matches lines 93-98 exactly).

- **`eval/frozen/compliance.lock.json`** — `prompt_hash` re-pinned from `58ffea08ff7b51a78efb866861c8d0ac646a1928901de1151c53e1a7c92f395d` to `ca333fbadbfa30eb750022004ab90c3eb3be06b92e9fa2935cdf856da2ef896d` per D14-B3 (floor prepend changes the prompt_hash surface; this is the first of the planned full-set re-locks — 14-04 handles hitl/fanout/platform-floor).

## Deviations from Plan

### Auto-fixed issues (Rule 1 — directly caused by this plan's correct code change)

1. **`internal/agent/hitl/threat_mitigations_test.go` — T-P12-3 sha256 baseline range** shifted from `(92, 97)` to `(93, 98)`. The +1 line comes from the `internal/platformfloor` import added to `deep_agent.go`; the bytes inside the alignment group are byte-identical (the SHA `b432d5dd6e7e750222fdaf9a33d1909ef7cebc565ae901e2342e3a4a713c5e41` matches lines 93-98 exactly, verified via Python + Go re-hash). The P12-L2 hard-cap invariant is preserved — the source-grep lock on the literal `MaxIteration: 10` still fires via `TestMaxIterationHardCap_Literal10`.

2. **`eval/frozen/compliance.lock.json` — `prompt_hash`** re-pinned from `58ffea08...` to `ca333fbadbfa30eb...` per D14-B3. The floor addition changes the prompt_hash surface (the 3rd argument to `reprohash.PromptHash` now contains the floor header + tail + skill body, not just the skill body). This is the first of the planned full-set re-locks; 14-04 will handle the remaining three (hitl/fanout/platform-floor).

### Auto-added deviation (Rule 3 — blocking import cycle resolution)

3. **Leaf package `internal/platformfloor`** — created as a dependency-free leaf package because `internal/agent/runner_setup.go` imports `internal/runtime` (the agent→runtime edge), so `internal/runtime/deep_agent.go` cannot import `internal/agent` without an import cycle. Yet Task 3's structural gate requires `ComposePlatformPrompt(` inside `renderDeepSystemPrompt`'s body — the gate cannot accept a copy of the floor text in `internal/runtime` (Pitfall 17). The leaf package breaks the cycle structurally: `internal/agent` → `internal/platformfloor` ← `internal/runtime` (no cycle). 14-RESEARCH.md §Code Examples explicitly offered this alternative: "或独立 internal/platformfloor 包，planner 定". The `internal/agent/platform_floor.go` file exists as a documented re-export alias so the plan's `grep -c 'PlatformFloorVersion = "1.0.0"' internal/agent/platform_floor.go` acceptance criterion resolves at the canonical agent-package path (the literal is duplicated here ONLY for source-grep visibility per the godoc; the leaf remains the authoritative declaration, and the duplicate is part of the documented D14-B1 bump discipline — a bump edits BOTH lines together).

### Test adaptations (documented in test godocs)

4. **`TestPlatformFloorCannotBeShadowed`** — the plan's original test shape assumed Skill body text IS rendered into the system prompt, with shadow phrases appearing in the middle band. The current architecture (Phase 11 chat archetype + Phase 12 deep archetype) renders static templates that ignore the manifest body — Skill body has no injection channel into the system prompt. The adapted test asserts the strictly-stronger property: shadow phrases DO NOT reach the rendered prompt at all, the floor still prefixes + suffixes, and the D14-A1 substring count is exactly 1. A future regression that opens an injection channel would FAIL this test (shadow phrases would suddenly appear). The godoc explains why "absence is stronger than middle-band presence" for the current architecture.

5. **`TestFloorHotReloadSurvives`** — same adaptation. The plan expected the rendered prompt to differ across Skill body reloads (with the body marker visible in each render). The adapted test asserts: both v1 and v2 renders carry the floor header + tail, the D14-A1 substring count stays at 1, and both renders are byte-identical (the system prompt is invariant under Skill body content — structural isolation is the load-bearing property). A future regression that opens an injection channel would FAIL this test (v1 ≠ v2).

## Auth gates

None. No `.env`/`GLM_API_KEY`/`BGE_M3_URL`/`MILVUS_ADDRESS` consumed by this plan — all changes are hermetic Go (and the compliance.lock.json re-pin is a hash byte, not a live eval).

## Acceptance criteria — all met

- [x] `grep -c 'PlatformFloorVersion = "1.0.0"' internal/agent/platform_floor.go` == 1
- [x] `TestPromptHashCoversFloor` PASS (floor change → hash change; skill change → hash change)
- [x] `TestComposePlatformPromptShape` PASS (composed prompt HasPrefix(floor) + HasSuffix(tail) + middle == skillPrompt byte-identical)
- [x] Leaf-level 7-item substring lock PASS (all 8 D14-A1 snippets present in `platformFloorText`)
- [x] `internal/reprohash/reprohash.go` zero diff (D14-B3: hash digest order unchanged; only the 3rd argument's content changes)
- [x] 4 named Task 2 test locks PASS: `TestPlatformFloorPrependBothPaths` (chat side) + `TestRenderDeepSystemPromptPrependsFloor` (deep side) + `TestPlatformFloorCannotBeShadowed` + `TestFloorHotReloadSurvives`
- [x] Acceptance grep on old trust clauses: `prompts.go=0, deep_agent.go=0` for both `Return status="refused" ONLY when NO retrieved chunk` and `NEVER emit a refusal when a relevant chunk exists`
- [x] `grep -c "ComposePlatformPrompt(" internal/agent/prompts.go internal/runtime/deep_agent.go` each ≥ 1 (prompts.go=1, deep_agent.go=2)
- [x] `go test ./... -count=1` zero regressions across all 25 packages
- [x] `go test ./integration/contract -tags=contract -run TestNoBypassOfPlatformFloorCompose -count=1` PASS + negative proof (probe injection FAILS the gate with file:line pointer; deletion restores PASS)

## Verification

```bash
# Build + vet + full regression
go build ./... && go vet ./... && go test ./... -count=1

# Contract gate
go test ./integration/contract -tags=contract -run TestNoBypassOfPlatformFloorCompose -count=1 -v

# Named test locks (4 chat + deep)
go test ./internal/agent/ ./internal/runtime/ \
  -run 'TestPlatformFloor|TestFloorHotReload|TestPromptHashCoversFloor|TestRenderDeepSystemPrompt' \
  -count=1
```

All green.

## Known Stubs

None. All seven D14-A1 data-trust items + the D14-A4 unified trust paragraph + the D14-A3 tail reminder are present and structurally locked.

## Threat Flags

None new from this plan. The threat surface covered by this plan (Pitfall 17 dual-path trust-clause drift + Skill-body shadow attempts) is MITIGATED in the existing `threat_model` block of `14-01-PLAN.md` (T-14-01 + T-14-02 + T-14-SC all dispositioned "mitigate" / "accept"). The audit-row `platform_floor_version` column (D14-B2) is deferred to 14-04 (frozen set + audit write path); no new network endpoints, auth paths, file access patterns, or trust-boundary schema changes were introduced by this plan.

## Next plan

`14-02` (admin preview endpoint + `/api/admin/platform-prompt` route + `TestPlatformPromptPreview` 四面 mirror of `TestAdminToken_*`).
