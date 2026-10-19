---
phase: 47-debt-cleanup-reproducible-baseline
plan: 06
subsystem: planning-records
tags: [docs, debt-disposition, traceability, index, d-05, known-limitation]

requires:
  - phase: 47-01
    provides: "REL-01 determinism fix + measured flake evidence (0ecd9c3, a05929a) for the D-46-DEF-02 row"
  - phase: 47-02
    provides: "REL-02 backend cache write + both-branch test + corrected read-path comments (e7ca19a, d3f86c2, c90f91f) for the WARNING-1 row"
  - phase: 47-03
    provides: "Hermetic emitter count tests (22f8480, e88d6ca) for the D-45-DEF-01 and BL-04 rows"
  - phase: 47-04
    provides: "FE report refetch making trigger==='rollback' reachable (197421e, bf1c1e0, abbb594) for the WARNING-1 row"
  - phase: 47-05
    provides: "SC-5 assertion retarget (5f9459a) for the D-46-DEF-01 and BL-04 rows"
  - phase: 47-07
    provides: "BL-01..04 backfilled into REQUIREMENTS Traceability with the status split this plan must match (4a1ac2b)"
provides:
  - "47-DEBT-DISPOSITION.md — authoritative per-ID disposition master table, 19 rows, closed 去向 vocabulary, one citable 证据 per row"
  - "STATE.md § Deferred Items demoted to a 12-row per-ID index linking to the master table (D-03c)"
  - "D-05 rollback/quarantine asymmetry recorded as an explicit known limitation"
  - "The 19-id → 去向 list below is what plan 47-08's P7 gate segment greps"
affects: [47-08-verification-gate, phase-verification, phase-48-ci, future-maintainers]

tech-stack:
  added: []
  patterns:
    - "Closed 去向 vocabulary (v1.7 修复 / 延期+理由 / ack backlog) + mandatory citable 证据 per row makes every 'the debt was handled' claim falsifiable by grep"
    - "A planning record that must survive compaction is split into an authoritative file plus a per-ID index that only links"
    - "Honest recording: a disposition whose fix lands in a later plan says so in its evidence cell instead of claiming 已修复"

key-files:
  created:
    - .planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md
  modified:
    - .planning/STATE.md

key-decisions:
  - "去向 is a closed three-value vocabulary declared in the document itself; a fourth value is treated as record corruption, not as a new bucket"
  - "D-45-DEF-02's evidence cell states plainly that the untracked stale harness is still on disk at write time and that plan 47-08 owns the deletion — the 去向 is recorded as the phase's intent without claiming an unproven fix"
  - "Added a 12th STATE.md index row for D-05 (the plan enumerated 11); STATE.md is the index a maintainer reads first, and hiding a known limitation there would recreate the omission this plan exists to prevent"
  - "D-45-DEF-03 and WARNING-2 cross-reference each other as one future 'schema visibility semantics' decision record instead of three separate vague deferrals"

patterns-established:
  - "Deferred-with-reason rows must name the re-entry precondition ('重新立项前提'), not just assert that deferral is fine"
  - "A known limitation gets its own ID and its own index row; it is never folded into the success narrative of the requirement that surfaced it"

requirements-completed: [REL-04]
duration: ~10min
completed: 2026-10-19
---

# Phase 47 Plan 06: v1.6 遗留项逐 ID 处置主表 Summary

**Every v1.6 leftover now has exactly one machine-checkable row — a closed-vocabulary 去向 plus a citable 证据 — and `STATE.md`'s Deferred Items table is demoted to a per-ID index that links to that record instead of being a competing copy of it.**

## Performance

- **Duration:** ~10 min
- **Completed:** 2026-10-19
- **Tasks:** 2
- **Files:** 1 created, 1 modified
- **Commits:** 2

## Task Commits

1. **Task 1 — write `47-DEBT-DISPOSITION.md`** — `3bb14ee`
2. **Task 2 — demote `STATE.md` § Deferred Items to a per-ID index** — `e5b3d1b`

## What shipped

### Task 1 — `47-DEBT-DISPOSITION.md` (19 rows, bucket summary, SC-2 note)

A single-file master record (the `42-triage.md` precedent) with:

- **A header declaring the document's authority** — the disposition file is the record; `STATE.md` § Deferred Items and `REQUIREMENTS.md` § Backlog Items Carried From v1.6 are indexes that point at it. The three-value 去向 vocabulary is declared inline so no new bucket can be invented silently.
- **§1 bucket summary** — 6 `v1.7 修复` / 6 `延期+理由` / 7 `ack backlog`, bolded **Total 19** matching the row count exactly (verified mechanically).
- **§2 per-ID table** — `| ID | 来源 | 去向 | 证据 |`. Every 证据 cell names a file path or a commit SHA; no cell is a promise. Every 延期 row also names its **重新立项前提** (what must happen before the item can be picked up), which is what makes "延期+理由" different from "以后再说".
- **§3 SC-2 truthfulness note** — states what REL-02 actually made true (served report and rollback violation count come from the *same* `ValidateWithTrigger` result; the banner's 「回滚后」 branch is reachable) and, in the same breath, states what it did **not** make true (rollback still does not execute `Quarantine`, so the isolation wording on that path is wrong). SC-2's acceptance statement therefore survives without scope expansion, because the two claims do not imply each other.

### Task 2 — `STATE.md` index

§ Deferred Items went from 4 rows (2 semantic + 1 `BL-01..04` range row + 1 aggregate row) to **12 per-ID rows + the untouched `future` row**, each Status cell carrying its own 去向 and a link to `47-DEBT-DISPOSITION.md`. A line directly under the heading states that the table is an index and names the authoritative file. The `BL-01..04` range row is gone — a range row cannot be linked to individually and it hides which of the four is still open (BL-04 is closed, BL-01..03 are not).

## The 19 ids and their 去向 (47-08's P7 gate greps this list)

| ID | 去向 | Owner / evidence anchor |
|---|---|---|
| `D-45-DEF-01` | `v1.7 修复` | plan 47-03 — `22f8480`, `e88d6ca`; assertion-side hermeticization, `emitter.go` untouched |
| `D-45-DEF-02` | `v1.7 修复` | working-tree cleanup owned by plan 47-08 (D-04d); harness rewrite NOT commissioned (no consumer). **Deletion not yet executed at write time — evidence cell says so** |
| `D-45-DEF-03` | `延期+理由` | `45/deferred-items.md:19-21` — WR-01 / WR-02 / IN-02, each with its own re-entry precondition |
| `D-46-DEF-01` | `v1.7 修复` | plan 47-05 — `5f9459a`; assertion retargeted to 文档 mode, mutation-proven |
| `D-46-DEF-02` | `v1.7 修复` | plan 47-01 — `0ecd9c3`, `a05929a`; **closes REL-01** |
| `WARNING-1` | `v1.7 修复` | plan 47-02 `e7ca19a` + plan 47-04 `197421e`/`bf1c1e0`/`abbb594`; **closes REL-02** |
| `WARNING-2` | `延期+理由` | `v1.6-MILESTONE-AUDIT.md:25` — tier-visibility semantics decision, admin baseline unaffected |
| `BL-01` | `延期+理由` | behavior-semantics refactor, no current defect |
| `BL-02` | `延期+理由` | only becomes well-defined once Phase 48's `golangci-lint` produces a report |
| `BL-03` | `延期+理由` | needs-live (内网 LLM/BGE) |
| `BL-04` | `v1.7 修复` | plan 47-03 (emitter) + plan 47-05 (DetailPanel); matches `REQUIREMENTS.md` = Complete |
| `D-05` | `延期+理由` | **known limitation** — rollback does not execute `Quarantine`; the banner/confirm wording overstates on that path |
| `IN-01`..`IN-07` (7 rows) | `ack backlog` | `46-REVIEW.md:177-231`, each row citing the item's one-line description |

`BL-01..04` split matches `REQUIREMENTS.md:79-82` exactly (3 Deferred + 1 Complete), so 47-07 and this plan agree.

## Verification

| Check | Result |
|---|---|
| Each of the 19 ids appears **exactly once** as a `\| \`<id>\`` row | 19/19, 0 missing, 0 duplicates |
| Rows by 去向 | 6 / 6 / 7 = 19; §1 Total = 19 |
| Every 证据 cell non-empty and containing a path or commit SHA | pass (awk scan; no cell below the length floor, no cell without a path/SHA/plan reference) |
| `grep -c "47-DEBT-DISPOSITION.md" .planning/STATE.md` | **13** (≥ 11 required; 12 index rows + 1 header pointer) |
| `grep -c "BL-01\.\.04" .planning/STATE.md` | **0** (range row removed) |
| `## Deferred Items` heading + `future` row present | yes (lines 80, 102) |
| Discrete per-ID rows in STATE.md | 12 |
| `git ls-files .../47-DEBT-DISPOSITION.md` | non-empty — **tracked**, not left untracked |
| `git diff --stat .planning/PROJECT.md` | empty (D-03d not triggered) |
| `git status --porcelain .planning/milestones/` | empty (archive untouched, D-03e) |

## Deviations from Plan

**1. [Task 2 — added a 12th index row for `D-05`]**
- **Issue:** the plan enumerated 11 per-ID index rows for STATE.md and did not list `D-05`, even though the same plan requires the quarantine asymmetry to be an explicit known limitation.
- **Fix:** added a `| known_limit |` row for `D-05` with 去向 `延期+理由` and a link to the master table's `D-05` section.
- **Why:** STATE.md is the index a maintainer reads first. Folding D-05 into the WARNING-1 row would re-create exactly the omission this plan exists to close — a limitation silently absorbed by a neighbouring success.
- **Commit:** `e5b3d1b`. Does not affect the 19-id master table or the ≥11 link threshold.

**2. [Task 1 — D-45-DEF-02 evidence cell records the fix as pending, not done]**
- **Issue:** the plan pins D-45-DEF-02's evidence as "plan 47-08 deletes the file", but 47-08 is Wave 4 and has not run. `internal/server/platform/verify_main.go` is still present and untracked at write time.
- **Fix:** the row's 去向 is `v1.7 修复` (as pinned) while its evidence states verbatim that the file is still on disk, that the deletion is owned by 47-08, and that the SHA must be backfilled once 47-08 lands.
- **Why:** the point of this phase is that the record is honest. Claiming 已修复 for an unexecuted action would be the exact failure mode 47-08's gate exists to catch.

## Issues Encountered

None blocking. No code touched, no builds needed, no package installs.

## Hand-off obligations

- **D-03b (owned by the phase verification step, NOT by this plan):** `47-VERIFICATION.md` must gain a single frontmatter pointer line to `47-DEBT-DISPOSITION.md`, following the Phase 44 `deferred: addressed_in / evidence` precedent, so an audit tool and a human reader both reach the authoritative record. **This obligation must not be dropped when 47-VERIFICATION.md is written.**
- **47-08 (`verify-phase-47.sh`, Wave 4):** the P7 gate segment should grep `47-DEBT-DISPOSITION.md` for each of the 19 ids exactly once, require a non-empty 去向 drawn from the three declared values, and require a non-empty 证据 cell. The expected mapping is the table above. 47-08 also owns the D-45-DEF-02 working-tree deletion and the `waiver_ok` / `WAIVED_FAIL` retirement in `verify-phase-45.sh` / `verify-phase-46.sh`; backfill the D-45-DEF-02 SHA into the master table afterwards.
- **Phase verification:** flip REL-01 / REL-02 / REL-04 Traceability Status to Complete; BL-04 is already Complete from 47-07 and BL-01..03 stay Deferred.
- **Suite state observed:** no build or test was re-run by this documentation-only plan. The upstream baselines it cites are 47-01 (Go, knowledge), 47-03 (Go, ontology, incl. a clean `git archive HEAD` checkout), 47-05 (FE 685/84), 47-04 (FE 688/85, 0 failed / 0 skipped).

## Next Phase Readiness

- The v1.6 debt is now **closed or explicitly dispositioned** — 6 fixed with commit evidence, 6 deferred each with a stated re-entry precondition, 7 acked as non-defect backlog. Zero items implicit.
- A later `STATE.md` rewrite can no longer silently drop the record: the index is per-ID and the authority lives in its own file.
- No blockers.

---
*Phase: 47-debt-cleanup-reproducible-baseline | Plan: 47-06 | Completed: 2026-10-19*
