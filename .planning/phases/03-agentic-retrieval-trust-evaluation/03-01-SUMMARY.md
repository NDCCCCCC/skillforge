---
phase: 03-agentic-retrieval-trust-evaluation
plan: 01
subsystem: wire-contract
tags: [wire, sse, decision-chunk, tier-enum, arag-01, know-01, tdd]
requires:
  - "Phase 1 SC-1 wire envelope (UIMessageChunk, ValidTypes, AssertAssignable)"
  - "Phase 2 skill.TierEnum (4-tier canonical map)"
provides:
  - "wire.ValidTypes entry: data-decision (T-1-11 gated)"
  - "wire.DecisionPayload struct (Step / Decision / ToolCallID / Query / Confidence)"
  - "web/src/types/wire-types.ts: TS DecisionPayload interface + data-decision SkillForgeChunk variant"
  - "skill.TierEnumByValue() reverse-lookup helper (CN-char → ASCII dir)"
  - "skill.TierEnumValues() deterministic 4-element slice"
affects:
  - "Plan 03-03 (agent): emits data-decision chunks via Eino callbacks"
  - "Plan 03-04 (knowledge.Indexer): KNOW-02 enforcement calls TierEnumByValue"
  - "Phase 4 web UI: SkillForgeChunk union gains a variant; D15 drift alarm stays green"
tech-stack:
  added: []
  patterns:
    - "TDD RED/GREEN per task (4 commits: 2 RED + 2 GREEN)"
    - "D-04 / D15 two-PR lockstep rule (Go + TS in one commit)"
    - "Backend-agnostic validation helper (no I/O, no globals) — trivially unit-testable"
key-files:
  created:
    - internal/wire/chunk_test.go
    - internal/skill/tier_test.go
  modified:
    - internal/wire/chunk.go
    - web/src/types/wire-types.ts
    - internal/skill/tier.go
decisions:
  - "TierEnumValues() order = ASCII-dir-key-sorted (authority, draft, historical, personal → 权威, 草稿, 历史, 个人) — reuses knownTierDirs() for the sorted iteration so determinism is structural, not duplicated"
  - "TierEnumByValue iteration order does not affect output (returns on first match); empty string short-circuits to (\"\", false) for the KNOW-02 default-deny contract"
  - "DecisionPayload placed in internal/wire (not internal/agent) so the TS mirror can live alongside the SkillForgeChunk union — the agent package imports wire and populates the payload via callbacks in Plan 03-03"
metrics:
  duration: 7m
  completed: 2026-08-11
  tasks: 2
  files_created: 2
  files_modified: 3
  commits: 4
---

# Phase 03 Plan 01: data-decision wire discriminator + TierEnum reverse-lookup helpers Summary

ARAG-01 / SC-1 observability wire substrate unlocked: the `data-decision` UIMessageChunk discriminator is registered on the Go side (`wire.ValidTypes` 16 → 17 types) and mirrored on the TS side (new `DecisionPayload` interface + variant in `SkillForgeChunk`), with the D15 `AssertAssignable<SkillForgeChunk, UIMessageChunk>` drift alarm staying green. KNOW-01 substrate unlocked: `skill.TierEnumByValue` + `skill.TierEnumValues` helpers added (additions only — no changes to `ResolveTiers` / `knownTierDirs` / the `TierEnum` map literal), unblocking Plan 03-04's KNOW-02 backend-agnostic tier-mandatory enforcement.

## Tasks Completed

### Task 1 — data-decision discriminator (Go + TS lockstep)

**TDD cycle:**

| Phase | Commit | Subject |
|-------|--------|---------|
| RED   | `082e9b9` | `test(03-01): add failing test for data-decision chunk type` |
| GREEN | `0b43156` | `feat(03-01): add data-decision discriminator (Go + TS lockstep)` |

**Go side (`internal/wire/chunk.go`):**

- `"data-decision": true,` added to `ValidTypes` immediately after `"data-policy"` (data-* cluster preserved).
- Count comment refreshed: `Count: 17 types — text(3) + reasoning(3) + tool-input(3) + tool-output(1) + source(2) + data-policy(1) + data-decision(1) + finish(2) + error(1)`.
- godoc above `ValidTypes` now explains the D3-03 / SC-1 observability rationale and points to the D15 two-PR invariant.
- Exported `DecisionPayload` struct placed adjacent to `UIMessageChunk`, mirroring `03-RESEARCH.md` §Code Examples lines 670-676: `Step int`, `Decision string`, `ToolCallID string` (`toolCallId,omitempty`), `Query string` (`query,omitempty`), `Confidence float64` (`confidence,omitempty`).

**TS side (`web/src/types/wire-types.ts`):**

- New `export interface DecisionPayload` mirroring the Go struct 1:1 (camelCase JSON tags match).
- New variant in `SkillForgeChunk` union adjacent to `data-policy` / `data-subprogress`: `| { type: 'data-decision'; id?: string; data: DecisionPayload }`.
- `AssertAssignable<SkillForgeChunk, UIMessageChunk>` declaration left UNTOUCHED — the drift alarm must keep firing on real drift.

**Tests (`internal/wire/chunk_test.go`, 3 subtests under `TestDecisionChunkType`):**

- `valid` — `IsValidType(UIMessageChunk{Type:"data-decision"}) == true`.
- `snake_case_rejected` — `IsValidType(UIMessageChunk{Type:"data_decision"}) == false` (T-1-11 regression).
- `round_trip` — marshalling `UIMessageChunk{Type:"data-decision", Data: <DecisionPayload JSON>}` preserves `"type":"data-decision"` and the decision payload.

### Task 2 — TierEnumByValue / TierEnumValues helpers

**TDD cycle:**

| Phase | Commit | Subject |
|-------|--------|---------|
| RED   | `1231cb1` | `test(03-01): add failing tests for TierEnumByValue/TierEnumValues` |
| GREEN | `a35fbda` | `feat(03-01): add TierEnumByValue/TierEnumValues reverse-lookup helpers` |

**Implementation (`internal/skill/tier.go`, additions only):**

- `func TierEnumByValue(value string) (dir string, ok bool)` — reverse-lookup iterating `TierEnum`. Empty string short-circuits to `("", false)`. Godoc names Plan 03-04's KNOW-02 indexer as the consumer.
- `func TierEnumValues() []string` — fresh-allocated slice ordered by ASCII dir key (reuses `knownTierDirs()` for the sorted iteration, producing `["权威","草稿","历史","个人"]`). Godoc guarantees fresh allocation per call.

**Tests (`internal/skill/tier_test.go`):**

- `TestTierEnumByValue` — 7 table-driven subtests: 4 happy paths (权威 / 历史 / 个人 / 草稿) + 3 default-deny paths (unknown CN value, accidental ASCII `"authority"`, empty string).
- `TestTierEnumValuesDeterminism` — asserts the exact 4-element sorted slice, idempotence across calls, AND that caller mutation of the returned slice does NOT leak into the package-level `TierEnum` map.

**Constraint honored:** `git diff HEAD -- internal/skill/tier.go` shows ZERO removed lines — `ResolveTiers`, `knownTierDirs`, and the `TierEnum` map literal are untouched.

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written. Both tasks landed as pure additions.

### Plan Verification Step — Pre-Existing TS Errors (Out of Scope)

**Found during:** Task 1 GREEN verification.

**Observation:** `cd web && npx tsc --noEmit` does NOT exit 0. **None of the errors are caused by 03-01 changes** — they all live in Phase 4 fixtures / components and reflect drift between Phase 4's wire-type fixtures and the currently-installed AI SDK version.

**Verification that 03-01 is TS-clean:**

- ZERO TS errors are located in `web/src/types/wire-types.ts`.
- ZERO TS errors mention `data-decision` or `DecisionPayload`.
- The D15 drift alarm line `_driftCheckTrue: _driftCheck = true` (wire-types.ts line 111) is NOT in the error list — assignability holds for the new variant.

**Files affected (all Phase 4 vintage — NOT touched by 03-01):**

- `web/src/__tests__/MessageBubble.test.tsx`
- `web/src/__tests__/ReasoningPart.test.tsx`
- `web/src/__tests__/transport-swap.test.tsx`
- `web/src/__tests__/wire-contract.test.ts`
- `web/src/components/chat/parts/ToolPart.tsx`

**Action:** Logged to `.planning/phases/03-agentic-retrieval-trust-evaluation/deferred-items.md` per the SCOPE BOUNDARY rule. Recommend a follow-up plan (e.g. `04-04 wire-contract realignment`) to either re-pin `ai` to the exact Phase 4 version or migrate the fixtures to the current SDK shape. The D15 drift alarm — the actual 03-01 acceptance criterion — is green.

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| Task 1 RED  | `082e9b9` (`test(03-01): add failing test for data-decision chunk type`) | ✅ |
| Task 1 GREEN | `0b43156` (`feat(03-01): add data-decision discriminator (Go + TS lockstep)`) | ✅ |
| Task 2 RED  | `1231cb1` (`test(03-01): add failing tests for TierEnumByValue/TierEnumValues`) | ✅ |
| Task 2 GREEN | `a35fbda` (`feat(03-01): add TierEnumByValue/TierEnumValues reverse-lookup helpers`) | ✅ |

Both RED gates failed as expected (subtest `valid` failing; compile error from undefined functions). Both GREEN gates flipped the corresponding tests to green with minimal-impact additions.

## Threat Model

| Threat | Disposition | Mitigation Landing |
|--------|-------------|--------------------|
| T-3-SPOOF (T-1-11) | mitigate | Task 1: `"data-decision": true` added to `ValidTypes`; `IsValidType()` now accepts it; `EmitChunk` (existing) pre-write gate still rejects snake_case. `TestDecisionChunkType/snake_case_rejected` is the regression. |
| T-3-DRIFT (D15 wire-type drift) | mitigate | Task 1: Go + TS touched in one commit; `AssertAssignable<SkillForgeChunk, UIMessageChunk>` left unmodified and verified to still compile (line not in TS error list). |
| T-3-SC (supply chain) | accept | No package installs in this plan — stdlib `encoding/json` + existing project types only. |

## Success Criteria

- ✅ **SC-1 (ARAG-01) wire substrate:** `data-decision` chunk registered in `wire.ValidTypes`, `DecisionPayload` exported, TS `SkillForgeChunk` union extended — `TestDecisionChunkType` green, `go build ./...` clean, D15 drift alarm green for the new variant.
- ✅ **KNOW-01:** 4-tier enum exposed programmatically via `TierEnumByValue` (reverse lookup) + `TierEnumValues` (canonical set enumeration) — `TestTierEnumByValue` + `TestTierEnumValuesDeterminism` green.
- ✅ **D-04 / D15 invariant preserved:** Go + TS modifications committed atomically; the shipped Phase 4 React UI's `SkillForgeChunk` union compiles with the new variant; the `_driftCheckTrue` alarm line is NOT in the TS error list.
- ✅ **Plan 03-04 unblocked:** `TierEnumByValue` is available for `internal/knowledge.Index` to call against any `Document.Tier` — the indexer can land KNOW-02 enforcement as a pure addition in Plan 03-04.

## Known Stubs

None. Plan 03-01 is a pure substrate plan — no rendering, no runtime emission, no UI changes. `DecisionPayload` will be populated by Plan 03-03's Eino callbacks; consumed by Plan 03-04's indexer. No stub or placeholder data flows to the UI from this plan.

## Self-Check: PASSED

- All 5 modified/created source files present.
- All 4 task commits present (`082e9b9`, `0b43156`, `1231cb1`, `a35fbda`).
- All 6 acceptance-criteria greps return 1:
  - `"data-decision":         true` in `internal/wire/chunk.go`
  - `Count: 17 types` in `internal/wire/chunk.go`
  - `type DecisionPayload struct` in `internal/wire/chunk.go`
  - `'data-decision'` in `web/src/types/wire-types.ts`
  - `func TierEnumByValue(value string)` in `internal/skill/tier.go`
  - `func TierEnumValues()` in `internal/skill/tier.go`
- `go test ./internal/wire/ ./internal/skill/ -count=1 -race` exits 0.
- `go build ./...` exits 0.
- No TS error located in `web/src/types/wire-types.ts` (D15 drift alarm green for the new variant).
