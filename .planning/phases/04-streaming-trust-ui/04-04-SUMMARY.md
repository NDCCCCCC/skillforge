---
phase: 04-streaming-trust-ui
plan: 04
subsystem: testing
tags: [gap-closure, tech-debt, sdk-drift, d15-alarm, tsc-clean, wave-4, ai-sdk-v7]

# Dependency graph
requires:
  - phase: 04-streaming-trust-ui
    provides: "Shipped Phase 4 wire contract (SkillForgeChunk union, D15 drift alarm, D-04 transport-swap invariant, T-1-08 sanitizer gate) — all UAT-verified 2026-08-11"
provides:
  - "ai@7.0.59 wire-contract realignment: zero TypeScript errors project-wide (21 → 0)"
  - "D15 drift alarm strengthened — finish variant now genuinely assignable to UIMessageChunk's finish arm (was union-distribution-tolerated)"
  - "Pitfall 1/3 stale-discriminator guard preserved via string-cast comparison (TS2367 resolved without semantic change)"
  - "Test fixtures migrated to current SDK v7 shape (state='done' + dynamic-tool.toolName)"
  - "ToolPart production renderer uses structural-type toolName read (covers ToolUIPart + DynamicToolUIPart)"
affects: [phase-05-skill-platform, phase-07-open-source-release]

# Tech tracking
tech-stack:
  added: []  # no new packages — T-04-04-SC disposition: accept (ai@^7.0.59 / @ai-sdk/react@^4.0.0 already installed)
  patterns:
    - "Structural-type cast for cross-version SDK field reads: (part as { toolName?: string }).toolName ?? part.type"
    - "Local narrowing interface for test-only zero-arg sendMessages contract (FixtureDrivenTransport)"
    - "String-cast comparison for stale-discriminator guards when TS narrows the union past the literal"

key-files:
  created: []
  modified:
    - web/src/types/wire-types.ts
    - web/src/__tests__/wire-contract.test.ts
    - web/src/__tests__/MessageBubble.test.tsx
    - web/src/__tests__/ReasoningPart.test.tsx
    - web/src/__tests__/transport-swap.test.tsx
    - web/src/components/chat/parts/ToolPart.tsx

key-decisions:
  - "Option B chosen (migrate fixtures + production component to ai@7 shape) over Option A (re-pin SDK) — re-pinning would fight the D15 drift-alarm philosophy"
  - "finishReason root-cause narrowing STRENGTHENS D15: the finish variant becomes genuinely assignable to UIMessageChunk's finish arm, not merely tolerated by conditional-type union distribution"
  - "Pitfall 1/3 guard semantics preserved via `const t = chunk.type as string;` cast — the guard's INTENT is to catch stale fixtures that TS would structurally reject, so the cast is the correct expression of intent"
  - "FixtureDrivenTransport local interface (zero-arg sendMessages) isolates the test-only contract from the production ChatTransport interface (which requires an options argument in ai@7)"

patterns-established:
  - "Drift-cleanup plans should narrow root causes (Option B) rather than re-pin dependencies (Option A) — the alarm's value depends on tracking the current SDK"
  - "When a per-member assignability alarm stays green via conditional-type distribution, narrowing each member to genuinely assignable STRENGTHENS the alarm"

requirements-completed: []  # gap-closure plan — no new functional requirements

# Metrics
duration: 12min
completed: 2026-08-12
---

# Phase 4 Plan 04: Wire-Contract TypeScript Realignment Summary

**ai@7.0.59 wire-contract drift cleanup: 21 tsc errors → 0 by narrowing SkillForgeChunk's finishReason to FinishReason, migrating test fixtures to state='done' + dynamic-tool.toolName, and switching ToolPart's toolName read to a structural-type cast — D15 alarm strengthened, Pitfall 1/3 guards preserved, 84/84 vitest green, 411.12 KB / 120.47 KB gzip production bundle (04-03 band).**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-08-12 (executor spawn)
- **Completed:** 2026-08-12T04:46:00Z
- **Tasks:** 3/3
- **Files modified:** 6 (exactly the 6 declared in frontmatter `files_modified`)

## Accomplishments

- **Zero TypeScript errors project-wide.** `cd web && npx tsc --noEmit` exits 0 (baseline was 21 errors concentrated in 6 files).
- **D15 drift alarm strengthened, not weakened.** The `finish` variant of `SkillForgeChunk` now uses `finishReason?: FinishReason` (imported from `ai`), making it genuinely assignable to the SDK's `UIMessageChunk` finish arm. Previously the alarm stayed green only because conditional-type distribution tolerated the variant via other assignable members.
- **Pitfall 1/3 stale-discriminator guard preserved.** The wire-contract test still rejects any fixture containing `type: "finish-message"` or `type: "tool-invocation"`, but now via a string-cast comparison (`const t = chunk.type as string;`) that resolves TS2367 without changing semantics. The test name and assertion messages retain their Pitfall 1 / Pitfall 3 references (part of the 04-RESEARCH.md contract).
- **Test fixtures migrated to current SDK v7 shape.** All reasoning parts use `state: 'done'` (or `'streaming'`); both `dynamic-tool` fixtures in MessageBubble.test.tsx carry the SDK-required top-level `toolName: string`. No test deleted, skipped, or weakened.
- **Production ToolPart renderer fixed.** `getToolName` now reads toolName via a structural-type cast `(part as { toolName?: string }).toolName ?? part.type`, covering both `ToolUIPart` and `DynamicToolUIPart` paths under ai@7. The `dynamic-tool` early-return branch is preserved verbatim.
- **D-04 transport-swap invariant preserved.** `makeDefaultTransportFromFixture` now returns a local `FixtureDrivenTransport` interface (zero-arg `sendMessages`) — the test owns this contract because the fixture is closure-captured. 5/5 D-04 invariant tests + 1 T-1-08 sanitizer sanity test all green.

## Task Commits

Each task was committed individually:

1. **Task 1: 收紧 wire-types.ts finish 变体并修正 wire-contract drift test** — `d3ac4ed` (fix)
2. **Task 2: 把 MessageBubble / ReasoningPart 测试 fixture 迁移到当前 SDK v7 形状** — `6857996` (test)
3. **Task 3: 修正 ToolPart.tsx 的 toolName 读取路径 + transport-swap.test.tsx 的 sendMessages arity 与 fixture 赋值** — `e68b942` (fix)

**Commit ordering note:** Task 1 was re-committed at `d3ac4ed` AFTER Tasks 2 and 3 due to concurrent-process interference (see Deviations §1). The logical task dependency (Task 3 depends on Task 1's root-cause narrowing) was respected during execution — all three were verified together at each step. The git-log ordering reflects re-commit timing, not execution order.

## Files Created/Modified

- `web/src/types/wire-types.ts` — Root-cause fix: `finishReason?: string` → `finishReason?: FinishReason` (line 69); import block gains `FinishReason` (line 35); D15 alarm lines 107-112 unchanged; added a 9-line comment block explaining the 2026-08-12 narrowing.
- `web/src/__tests__/wire-contract.test.ts` — Pitfall 1/3 guard (lines 71-84) migrated to `const t = chunk.type as string;` cast comparison; line 31 `const _sdk: UIMessageChunk = chunk;` now compiles naturally (no fallback cast needed); added a 9-line comment block explaining the cast rationale.
- `web/src/__tests__/MessageBubble.test.tsx` — Two `state: 'complete'` → `state: 'done'` (lines 39, 89); two `dynamic-tool` fixtures gained top-level `toolName: 'dynamic-tool'` (lines 53, 182); `tool-search_authority` cast unchanged (intentional fall-through arm test).
- `web/src/__tests__/ReasoningPart.test.tsx` — Three `state: 'complete'` → `state: 'done'` (makePart default line 26, + two explicit overrides at lines 69, 74); `state: 'streaming'` line 62 unchanged.
- `web/src/__tests__/transport-swap.test.tsx` — Dropped `ChatTransport` named import (kept `UIMessage`, `UIMessageChunk`); added local `FixtureDrivenTransport` interface (zero-arg `sendMessages`); changed `makeDefaultTransportFromFixture` return type to `FixtureDrivenTransport`; added `as UIMessageChunk[]` casts at the 5 call sites with inline comments documenting the D15-alarm-tolerated providerMetadata boundary.
- `web/src/components/chat/parts/ToolPart.tsx` — `getToolName` body changed from `(part as ToolUIPart).toolName ?? part.type` to `((part as { toolName?: string }).toolName) ?? part.type`; `if (part.type === 'dynamic-tool') return 'dynamic-tool';` branch preserved verbatim; added a 7-line comment block explaining the structural-type cast.

## Decisions Made

- **Option B over Option A.** The plan's `<objective>` pre-decided this; executor confirmed. Re-pinning `ai` to an older version would fight the D15 drift-alarm philosophy — the alarm's value depends on tracking the current SDK.
- **No fallback `as UIMessageChunk` cast needed at wire-contract.test.ts line 31.** Task 1's root-cause narrowing made the assignment compile naturally, which is the preferred outcome per the plan's "优先依赖 Task 1(a) 的根因修复使其无需 cast 即通过" instruction.
- **Applied the cast-at-call-site branch for TS2345.** After Task 1's narrowing, residual TS2345 on `text-start` (and similar) variants remained because of `providerMetadata?: unknown` vs `SharedV4ProviderMetadata` — a D15-alarm-tolerated boundary that the plan explicitly excluded from Task 1's scope ("`messageMetadata?: unknown` 保持原样"). The plan's Task 3(b) anticipated this exact case and prescribed the `as UIMessageChunk[]` call-site cast with inline comment.
- **No new packages installed.** T-04-04-SC threat disposition: accept. `ai@^7.0.59` and `@ai-sdk/react@^4.0.0` were already installed and verified in Phase 1/4.

## Deviations from Plan

### Issues Auto-resolved

**1. [Concurrent-process interference — non-code] Task 1 commit was briefly bundled into an unrelated 03-06 commit, then dropped during a concurrent rebase**
- **Found during:** Task 1 commit attempt
- **Issue:** A concurrent agent process (Phase 03-06 work) running in the same working tree committed my two staged Task 1 files (`wire-types.ts` + `wire-contract.test.ts`) inside their `feat(03-06): agent.LastRetrieved + store.UpdateAuditMetrics (additive)` commit (originally `77d493f`). That process then rebased its own work, dropping my files from the rewritten commit (`f7979af`). My edits remained in the working tree the whole time; no content was lost.
- **Fix:** Re-staged and re-committed the two Task 1 files atomically at `d3ac4ed` after Tasks 2 (`6857996`) and 3 (`e68b942`) were already committed. Logical execution order (1 → 2 → 3) was preserved during execution — only the git-log ordering reflects re-commit timing.
- **Files modified:** None beyond the declared 6.
- **Verification:** All three phase-level proofs (tsc / vitest / build) re-run after the re-commit; identical results (tsc exit 0, vitest 84/84, build exit 0 with 411.12 KB JS / 120.47 KB gzip).
- **Committed in:** `d3ac4ed`

---

**Total deviations:** 1 issue auto-resolved (0 code-level deviation rules triggered — no Rule 1/2/3 auto-fixes were needed because the plan was extremely precise).
**Impact on plan:** All declared work completed as specified. The only deviation is an environmental artifact (concurrent-process commit timing) that affected commit-hash ordering, not code content or correctness. The 6 declared files were the only files modified across the 3 commits.

## Issues Encountered

- **Bash backtick escape in Task 3 commit body.** The commit message's verbatim code snippet `\`type: \\\`tool-\${string}\\\`\`` triggered a transient bash command-substitution error (`tool-\: No such file or directory`). Git still created the commit (`e68b942`) correctly; the message body is intact in `git log`. No remediation needed.

## Verification Evidence

Phase-level proofs (run after all 3 task commits in place):

| Proof | Command | Result |
|-------|---------|--------|
| 1. tsc clean | `cd web && npx tsc --noEmit; echo "exit=$?"` | `exit=0` (baseline 21 → 0 errors) |
| 2. vitest suite | `cd web && npx vitest run` | **84/84 passing** across 14 test files |
| 3. production build | `cd web && npm run build` | `exit=0`; bundle **411.12 KB JS / 120.47 KB gzip** (04-03 baseline band) |
| 4. D15 alarm present | `grep '_driftCheckTrue: _driftCheck = true' web/src/types/wire-types.ts` | 1 hit (line 117 unchanged) |
| 5. D15 alarm green | `npx tsc --noEmit 2>&1 \| grep '_driftCheckTrue'` | 0 hits (alarm not in error output) |
| 6. FinishReason narrowing | `grep 'finishReason?: FinishReason' web/src/types/wire-types.ts` | 1 hit (line 76) |
| 7. Old signature gone | `grep -v '^//' web/src/types/wire-types.ts \| grep 'finishReason?: string'` | 0 hits |
| 8. No tests weakened | manual review of all 4 modified test files | No `.skip`, no deleted tests, no weakened assertions |

**Per-task proofs:** each task's `<verify><automated>` block was run individually before its commit and produced the expected zero-error / all-pass output (see Task 1/2/3 verification transcripts in execution history).

## D15 Alarm + Pitfall 1/3 Preservation Notes

- **D15 alarm** (`web/src/types/wire-types.ts` lines 107-112): The `AssertAssignable<SkillForgeChunk, UIMessageChunk>` conditional type distributes over the `SkillForgeChunk` union; it stays green when ANY member is assignable to `UIMessageChunk`. Pre-04-04, the `finish` variant was tolerated via this distribution (other members carried the assignability). Post-04-04, the `finish` variant is **itself** assignable because `finishReason?: FinishReason` is exactly the SDK's `finishReason?: FinishReason`. The alarm is strictly stronger.
- **Pitfall 1** (terminal chunk discriminator = `'finish'`, NOT `'finish-message'`): wire-contract.test.ts lines 71-84 still assert no fixture emits `'finish-message'`. The check now uses `const t = chunk.type as string; expect(t === 'finish-message', ...).toBe(false);` — the cast is necessary because TS knows the discriminated union has no literal overlap with the stale discriminator (TS2367), but the runtime guard's intent is precisely to catch a stale fixture that bypasses the type system.
- **Pitfall 3** (tool parts use `'tool-${name}'` or `'dynamic-tool'`, NOT `'tool-invocation'`): same cast pattern, same intent preservation.
- **D-04 transport-swap invariant** (`web/src/__tests__/transport-swap.test.tsx`): All 6 assertions (chunk-count equality, JSON byte-identity, canonical-type presence, data-policy payload match, citation sourceId match, T-1-08 sanitizer sanity) are unchanged in text and expected values. The `as UIMessageChunk[]` casts at call sites do not affect runtime behavior — they only satisfy TS that the D15-tolerated `providerMetadata` boundary is intentional.

## User Setup Required

None — no external service configuration required. No new packages installed; no env vars added; no UI behavior changed.

## Next Phase Readiness

- **Phase 4 is now fully debt-free.** `cd web && npx tsc --noEmit` exits 0 — the D15 philosophy ("the compiler IS the drift alarm") is now actually true.
- **Phase 5 (Skill Platform) front-end work** can proceed against a clean type-check baseline. Future fixture authors should use `state: 'done'` (not `'complete'`) and include top-level `toolName` on `dynamic-tool` parts — the migrated fixtures in MessageBubble.test.tsx and ReasoningPart.test.tsx are now the canonical reference.
- **Phase 7 (OSS Release) credibility check passed.** A stranger forking SkillForge now gets a green `tsc --noEmit` on first run, which is the D15 quality bar.

---

*Phase: 04-streaming-trust-ui (gap-closure)*
*Plan: 04-04*
*Completed: 2026-08-12*

## Self-Check: PASSED

- All 6 declared modified files exist on disk.
- All 3 task commits (`d3ac4ed`, `6857996`, `e68b942`) exist in git log.
- Phase-level proofs (tsc exit 0 / vitest 84/84 / build exit 0 with 411.12 KB JS / 120.47 KB gzip) re-verified after the final commit.
