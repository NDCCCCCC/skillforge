---
phase: 47-debt-cleanup-reproducible-baseline
plan: 03
subsystem: testing
tags: [go, test-fixture, hermetic, turtle, ontology, emitter, sql-guides]

requires:
  - phase: 47-01
    provides: REL-01 determinism fix (D-46-DEF-02) landed, leaving the emitter count tests as the only ontology failure
  - phase: 47-02
    provides: RollbackTBox SHACL cache write (REL-02) + both cache-branch tests; measured baseline "exactly ONE failure: TestEmitSubCategoryInstanceCount"
provides:
  - Hermetic `TestEmitSkillInstanceCount` (3-skill t.TempDir() fixture, asserts 3)
  - Hermetic `TestEmitSubCategoryInstanceCount` (synthetic KB tree with sql-guides scoping control, asserts SubCategory==2 and KBCategory==2)
  - Documented rationale for keeping `TestEmitTierInstanceCount` corpus-pinned (5 committed knowledge/ dirs)
  - Evidence row for D-45-DEF-01 / BL-04 disposition and for plan 47-08's clean-clone re-run
affects: [47-06, 47-08, phase-48-ci, D-04a-zero-waiver]

tech-stack:
  added: []
  patterns:
    - "Hermetic count test: t.TempDir() fixture, assert the fixture's own count (precedent: TestEmitMultiLineDescriptionParses, store_emitter_conformance_test.go)"
    - "Falsifiable scoping control: the non-target category carries its own nested sub-directory so a dropped guard turns the assertion red"

key-files:
  created: []
  modified:
    - internal/ontology/emitter/emitter_test.go

key-decisions:
  - "Fix on the assertion side only — emitter.go is untouched; 'fixing' by making the implementation emit a 7th SubCategory would invent a category (D-04b)."
  - "Do not downgrade want 7 to want 6: that would keep the test pinned to the repository corpus; the expected count is now derived from the fixture (D-04b/D-04c)."
  - "Keep TestEmitTierInstanceCount on projectRoot — its expected 5 equals the five committed top-level knowledge/ directories, so it is already green on a clean clone."

patterns-established:
  - "Fixture-derived counts: a count test that must pass on `git archive HEAD` may not walk the repo working tree (untracked dirs are inputs to the assertion)."
  - "Mutation-verified scoping: temporarily replacing `if ent.Name() == \"sql-guides\"` with `if true` must turn the SubCategory assertion red (3 vs 2) — otherwise the fixture proves nothing."

requirements-completed: [REL-04]

# Metrics
duration: 12min
completed: 2026-10-19
---

# Phase 47 Plan 03: Hermetic Emitter Instance-Count Tests

**Both emitter instance-count tests now build their own `t.TempDir()` fixtures and assert the fixture's own counts (Skill=3, SubCategory=2, KBCategory=2), with `emitter.go` untouched — the emitter suite is green both in the working tree and in a clean `git archive HEAD` checkout.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-10-19
- **Completed:** 2026-10-19
- **Tasks:** 2
- **Files modified:** 1 (`internal/ontology/emitter/emitter_test.go`)

## Accomplishments

- **D-45-DEF-01 closed on the assertion side.** `TestEmitSubCategoryInstanceCount` no longer asserts 7; it drives a synthetic KB tree and asserts 2. Commit `1aaf853` (six files moved `其他/` → `操作手册/`) is the reason the corpus count became 6 while the contract "one SubCategory per sql-guides sub-directory" stayed correct — the assertion was wrong-sized, not the implementation.
- **`TestEmitSkillInstanceCount` hermeticized in the same batch** (D-04c). It previously asserted 8 against the developer's working tree (only 3 skill dirs are git-tracked), so it was guaranteed red on a clean clone.
- **The `sql-guides` scoping guard is now falsifiable.** The fixture's `authority/` control category carries its own nested sub-directory (`authority/nested-control`); a mutation run that replaced `if ent.Name() == "sql-guides"` with `if true` produced **3 SubCategories vs the asserted 2**, so the test genuinely exercises the guard. The previous childless control yielded 2 either way and proved nothing.
- **Verified in a clean checkout.** `git archive HEAD` extracted to a scratch dir (3 tracked skill dirs, 6 sql-guides sub-dirs) runs `./internal/ontology/...` fully green — the D-04a "local gate == CI gate" precondition for this package.

## Task Commits

Each task committed atomically:

1. **Task 1: Hermeticize TestEmitSkillInstanceCount against a temp skills fixture** - `22f8480` (test)
2. **Task 2: Hermeticize TestEmitSubCategoryInstanceCount against a synthetic KB tree** - `e88d6ca` (test)

## Fixtures Used (evidence for D-45-DEF-01 / BL-04)

### `TestEmitSkillInstanceCount`

| Item | Value |
|------|-------|
| Root | `t.TempDir()` |
| Skill dirs (each with `SKILL.md`) | `fixture-a`, `fixture-b`, `fixture-c` (3) — names chosen to avoid collisions with repo skills |
| Front matter | `---\nname: <name>\ndescription: hermetic fixture skill\nagent: ChatModelAgent\n---\n\nbody\n` |
| `knowledge/` | created empty (Skill count is independent of KB dirs) |
| Assertion | `countByType(t, g, DefaultNS+"Skill") == 3` |

### `TestEmitSubCategoryInstanceCount`

| Item | Value |
|------|-------|
| Root | `t.TempDir()` |
| Skills | `skills/fixture-skill/SKILL.md` (1, so the emit path has a skill to process) |
| `sql-guides` sub-dirs | `取数口径`, `操作手册` (2) — same names as the `store_emitter_conformance_test.go` precedent |
| Control (non-`sql-guides`) | `authority/nested-control` — a top-level category **with** its own nested sub-directory |
| Assertions | `SubCategory == 2`; `KBCategory == 2` |

`TestEmitTierInstanceCount` was left corpus-pinned (expected `5` = the five committed top-level `knowledge/` directories) with a comment recording why it is safe on a clean clone. `projectRoot(t)` remains in use by that test plus `TestEmitRoundtrip`, `TestEmitNamespaceOverride`, `TestEmitSchemaCounts`, `TestEmitECLSchemaDeclarations`.

## Files Created/Modified

- `internal/ontology/emitter/emitter_test.go` — `TestEmitSkillInstanceCount` and `TestEmitSubCategoryInstanceCount` rewritten against `t.TempDir()` fixtures; explanatory comments added above both and above `TestEmitTierInstanceCount`. **No production file changed** (`git diff --stat internal/ontology/emitter/emitter.go` empty at both task commits).

## Decisions Made

- Fix lives entirely on the assertion side (D-04b). Making the implementation emit an extra SubCategory to reach 7 would fabricate a category; changing 7 → 6 would keep the corpus pin alive.
- Chose `t.TempDir()` synthetic trees over committed fixture directories, per `Claude's Discretion` in 47-CONTEXT and the in-file precedent at `emitter_test.go:234-270`.
- Added the `KBCategory == 2` assertion so the fixture is shown to feed both branches of `loadKBCategories`, and to pin that the nested control does not add a third top-level category.

## Deviations from Plan

**1. [Rule 1 — Bug] Added an explicit mutation verification of the `sql-guides` scoping guard**
- **Found during:** Task 2
- **Issue:** The plan asserts the fixture makes the guard falsifiable but did not specify proving it. A childless control silently passes with or without the guard.
- **Fix:** With the nested control in place, temporarily patched `emitter.go` (`"sql-guides"` → `if true`), ran the single test → FAIL `SubCategory instances = 3, want 2`, then restored `emitter.go` from a backup copy. `git diff --stat internal/ontology/emitter/emitter.go` confirmed the production file is byte-identical to HEAD afterwards.
- **Files modified:** none (temporary, reverted)
- **Verification:** recorded in this summary; the mutation result is quoted above.
- **Committed in:** n/a (verification only)

No other deviations. The plan's file scope was respected exactly.

## Issues Encountered

- None blocking. Go tooling only ran under WSL with `CGO_ENABLED=1`, per project rules.

## User Setup Required

None - no external service configuration required.

## Verification Evidence

```
# working tree (WSL, CGO_ENABLED=1)
go test ./internal/ontology/... -count=1
  ok  internal/ontology/emitter      (baseline before this plan: FAIL TestEmitSubCategoryInstanceCount 6 vs 7)
  ok  internal/ontology/induction
  ok  internal/ontology/shacl
  ok  internal/ontology/store

# clean checkout (git archive HEAD -> scratch dir, 3 tracked skills / 6 sql-guides sub-dirs)
go test ./internal/ontology/emitter/ -count=1 -v   -> 16/16 PASS
go test ./internal/ontology/... -count=1           -> all ok

# gofmt
gofmt -l internal/ontology/emitter/               -> clean
```

## Next Phase Readiness

- **Plan 47-06** can cite this summary as the D-45-DEF-01 / BL-04 evidence row (fix = assertion-side hermeticization; emitter contract unchanged).
- **Plan 47-08** can re-run `go test ./internal/ontology/emitter/ -count=1` inside a clean `git archive HEAD` checkout — it is expected green, as demonstrated above.
- **Phase 48** (CI) gains one fewer deterministic failure to waive; the `verify-phase-46.sh` P4 waiver covering D-45-DEF-01 is now obsolete and should be dropped when `verify-phase-47.sh` lands.
- No blockers. Not touched: `.planning/STATE.md`, `.planning/ROADMAP.md` (orchestrator-owned).

---
*Phase: 47-debt-cleanup-reproducible-baseline*
*Completed: 2026-10-19*
