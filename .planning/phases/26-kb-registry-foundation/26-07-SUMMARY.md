---
phase: 26-kb-registry-foundation
plan: 07
subsystem: eval-golden-set
tags: [kbm-04, eval-lock, re-sign, fanout, golden-set, breaking-switch]
requires:
  - "26-04: kb.Registry seed slugs authority/historical/personal/draft (ResolveReads draft-role drop semantics)"
  - "26-05/26-06: registry-driven trust surfaces (contributes the vocabulary the golden set now speaks)"
provides:
  - "eval/golden/fanout.jsonl in registry-slug vocabulary (24 rows, query/expectation fields byte-identical)"
  - "eval/frozen/fanout.lock.json re-signed: eval_set_sha256 cd0e54ef…→7d74a712…, lock_sha256 68a3f786…→445d6027…, + _kb_registry_migration annotation"
  - "scripts/verify-phase-{14,15,16}.sh EXPECTED_FANOUT_SHA rippled to f00f5b31…"
affects:
  - "26-10 (docs/structural-gate plan): README four-role section + CN-literal gate whitelists"
  - "verify-phase-13.sh: prints fanout lock sha only, no pin — unaffected"
tech-stack:
  added: []
  patterns:
    - "lock re-sign follows the lock's own documented convention (strip-line sha256), annotation field added before re-hash"
key-files:
  created:
    - ".planning/phases/26-kb-registry-foundation/26-07-SUMMARY.md"
  modified:
    - "eval/golden/fanout.jsonl"
    - "eval/frozen/fanout.lock.json"
    - "scripts/verify-phase-14.sh"
    - "scripts/verify-phase-15.sh"
    - "scripts/verify-phase-16.sh"
    - "cmd/spike-fanout/main.go"
    - "cmd/spike-fanout/main_test.go"
    - "examples/skills/calculator/SKILL.md"
    - "CONTRIBUTING.md"
decisions:
  - "Re-sign is operator-visible per D14-B4: recompute eval_set_sha256 over migrated LF bytes + strip-line lock_sha256, annotation field records reason, zero metric/threshold/operator changes"
  - "Notes migration line: ASCII-parenthesised tier tokens migrated; square-bracket slice-literal quotations ([个人], [权威,历史], [草稿]) retained as frozen Phase-13 measurement prose"
  - "calculator PLACEHOLDER documents that role: draft is fail-closed-rejected (accurate behavior), not that the wildcard works"
metrics:
  duration: "~20 min"
  completed: "2026-09-07"
  tasks: 2
  files: 9
---

# Phase 26 Plan 07: Golden Set CN→Slug Migration + Lock Re-Sign Summary

**One-liner:** fanout golden set migrated 权威→authority in one continuous golden-edit → sha-recompute → lock-re-sign task (eval_set_sha256 `cd0e54ef…`→`7d74a712…`), with metrics/thresholds byte-frozen and TestFanoutLock green on the new sha.

## What Was Built

### Task 1 — golden migration + lock re-sign (single continuous task, commit `4298394`)

The red line was honored: golden edit, sha recompute, lock update, and self-verification ran as ONE uninterrupted sequence; no commit point carries a half-migrated (golden edited / lock stale) tree.

**Golden (`eval/golden/fanout.jsonl`, 24 rows):**
- `"tier_ground_truth":"权威"` → `"tier_ground_truth":"authority"` on all 19 tier-channel rows carrying the field; `fanout-12` keeps `""` (its zero-tier scenario); 4 monthly_report rows keep the key absent. CN values 历史/个人/草稿 never appeared in `tier_ground_truth`.
- Notes: ASCII-parenthesised tier tokens migrated — `(权威)`×2, `(历史)`×1, `(权威+历史)`×2, `(草稿 ∩ 权威 = ∅)`×1 → slug vocabulary.
- Script-asserted: all 24 lines JSON-parse; `query`/`ground_truth`/`reads`/`sub_skills`/`expected_*`/`parent_reads`/`template_sections`/`scenario`/`parent_skill` byte-identical before vs after; file stays LF; natural-language Chinese (「审批流中审批人变更…」, 「历史认定」, 「（含历史口径）」 full-width parens) untouched.

**Lock re-sign (`eval/frozen/fanout.lock.json`), per its own `_lock_sha256_convention`:**
- `eval_set_sha256`: `cd0e54efcb8695601eae72102333c97c6bee97361a82b8317daaecafe84eaf32` → `7d74a712f3fcdde35b936fe5d811ceb5a92969a35caa222917a276f54bbcbe6a` (LF-normalized golden bytes, same algorithm as `internal/eval/fanout_lock_test.go:107-112`).
- `_kb_registry_migration` annotation field inserted (date + reason + "no cases added/removed; metrics/thresholds unchanged") **before** re-hashing.
- `lock_sha256`: `68a3f7863f354a9743a21e696488e2ba1631ffde96009c95ed65859e21c4fd91` → `445d6027ba5590c5ea536f2d9a7d912d5de47fd5a49baab5da06c5d0b75723b6` (strip-the-`"lock_sha256":`-line convention; whole-file sha now `f00f5b31c691070a7e5c0d76062e870148bfe65c231328ca41f7e10cc9ebf90e`).
- 29 protected lines (metrics, thresholds, case_count, category_counts, operator signature dates, pins, prior annotations) script-asserted byte-identical. No live re-run, no re-measurement, no case/threshold edits.

**Verify-script pins:** `EXPECTED_FANOUT_SHA` in `scripts/verify-phase-{14,15,16}.sh` rippled `af3020fd…` → `f00f5b31…` with a lineage comment (18 WR-05 re-freeze → 26-07 re-sign), following the 14-11 pin-ripple precedent. `verify-phase-13.sh` only prints the fanout sha — no change needed.

### Task 2 — eval-chain vocabulary sync + draft-role docs (commit `520eabd`)

- `cmd/spike-fanout/main.go`: multi_hop report header `[权威(,历史)]` → `[authority(/historical)]`; `GoldenCase.TierGroundTruth` godoc now records the slug vocabulary and that the field is parsed for golden-schema fidelity while the tier-mismatch gate asserts on BuildSubAgent load-time rejection.
- `cmd/spike-fanout/main_test.go`: synthetic golden rows in `TestLoadFanoutGolden` use `"tier_ground_truth":"authority"`. CN literals in the package: 0.
- `examples/skills/calculator/SKILL.md`: PLACEHOLDER comment + prose rewritten to draft-role KB semantics (KBM-03: draft never enters retrieval; `kb.ResolveReads` drops draft-role bases → declaration grants zero retrievable/citable source). `reads: [draft/**]` spelling unchanged; prohibition string updated; `TestExamplesValidate` green.
- `CONTRIBUTING.md`: "Add a Knowledge Tier" (TierEnum + CN-char Milvus values) replaced by "Add a Knowledge Base" — four-role trust table (authority = only citable role; draft never retrieved), seed-PR vs admin-API registration, slug regex, dir-decides-tier indexing, KNOW-02 fail-closed. The document frontmatter example was corrected to drop the bogus `tier: 权威` key (tier comes from the directory slug; frontmatter carries optional `effective_from`/`effective_to` only). `grep -c TierEnum` = 0; placeholder wording now "draft-role KB".
- `README.md`: zero diff (four-role README section belongs to 26-10).

## Verification Results

| Check | Result |
|-------|--------|
| `go test ./internal/eval/ -count=1` (incl. TestFanOutLockSelfConsistent + TestFanOutLockSHA256Recomputable on NEW sha) | PASS |
| `go test ./cmd/spike-fanout/ -count=1` | PASS |
| `go test ./internal/skill/ -run TestExamplesValidate -count=1` | PASS |
| `go build ./cmd/spike-fanout/` (non-live entry) | OK |
| `"tier_ground_truth":"权威\|历史\|个人\|草稿"` count in golden | 0 |
| Golden lines JSON-parse / row count | 24/24 |
| `git diff` metrics/thresholds/case_count lines in lock | only the `+ _kb_registry_migration` annotation line |
| Frozen assets touched | only `fanout.lock.json` (+ golden, intentional) |
| `grep -c TierEnum CONTRIBUTING.md` | 0 |
| `README.md` diff | none |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] calculator `role:` wildcard claim corrected to the landed behavior**
- **Found during:** Task 2
- **Issue:** The plan's action draft said the PLACEHOLDER comment could note "注册表化后亦可用 role: 通配" (a `role:` wildcard also works). The landed registry (`internal/kb/registry.go` ResolveReads) intersects role membership with `RetrievalEligible()` — `role: draft` expands to zero retrieval-eligible KBs and is **rejected fail-closed**, so documenting it as a valid alternative would ship a false claim and break contributors' Skills.
- **Fix:** The comment documents the accurate semantics: draft-role bases are dropped from the resolved set (default-deny), the wildcard is fail-closed for the draft role, therefore the explicit `draft/**` slug spelling is the required placeholder form.
- **Files modified:** `examples/skills/calculator/SKILL.md`
- **Commit:** `520eabd`

**2. [Rule 3 - Blocking] verify-script byte-identity pins rippled (and were already stale)**
- **Found during:** Task 1 (plan read_first directive: "grep verify 脚本…若有须同步")
- **Issue:** `verify-phase-{14,15,16}.sh` pin the whole-file sha of `fanout.lock.json`. The pinned value `af3020fd…` had been stale since the Phase 18 WR-05 re-freeze (file was `89a3112c…` before this plan) — a pre-existing gap; the 26-07 re-sign changed the file again.
- **Fix:** Ripple all three pins to the new whole-file sha `f00f5b31…` with a lineage comment, per the 14-11 pin-ripple precedent. No comparison pin exists in `verify-phase-13.sh` (print-only).
- **Files modified:** `scripts/verify-phase-14.sh`, `scripts/verify-phase-15.sh`, `scripts/verify-phase-16.sh`
- **Commit:** `4298394`

### Scope Notes (plan-letter decisions, not deviations)

- Notes tokens in **square-bracket slice-literal form** (`reads [个人]`, `ResolvedTiers=[权威,历史]`, `reads [草稿]`, `reads [权威]`) were **retained**: the plan's interfaces rule migrates only parenthesised tier markers, and these bracketed forms quote the frozen Phase-13 measurement (in which the runtime truly carried CN tier values) — historical-measurement prose stays as measured, per the lock's own retention philosophy. The `tier_ground_truth` acceptance criterion (CN value count == 0) is met.
- `fanout-13`'s notes mention of the deleted `TierEnum` identifier was left as frozen historical prose for the same reason.
- The full-repo TODO-convention note for CONTRIBUTING was **not** added (no such section exists today); per plan it is deferred to 26-10 together with structural gate 1.

## Known Stubs

None. No placeholder data paths, no unwired consumers — `TierGroundTruth` remains a parsed-but-unasserted golden field, and its godoc now states that explicitly (pre-existing posture, unchanged by this plan).

## Threat Model Compliance

| Threat | Mitigation applied |
|--------|--------------------|
| T-26-07-1 (lock tampering) | Re-sign executed strictly per `_lock_sha256_convention`; `_kb_registry_migration` annotation records date/reason; 29 protected lines byte-asserted; no operator signature date touched |
| T-26-07-2 (Chinese prose corruption) | Field-level replacement only, longest-form-first; protected-field snapshot diff before/after; full-width （） prose untouched; 24/24 lines parse |
| T-26-07-3 (false green via golden revert) | Direction enforced forward: golden carries slug values AND `TestFanoutLock*` green on the new sha — the inconsistent state is structurally excluded |
| T-26-SC (dependency installs) | Zero new dependencies |

## Self-Check: PASSED

- `eval/golden/fanout.jsonl` exists, 24 rows, slug vocabulary — verified on disk
- `eval/frozen/fanout.lock.json` re-signed, self-consistent (tests green on new sha) — verified
- Commit `4298394` present (`git log` verified)
- Commit `520eabd` present (`git log` verified)
- `cmd/spike-fanout/{main,main_test}.go`, `examples/skills/calculator/SKILL.md`, `CONTRIBUTING.md`, `scripts/verify-phase-{14,15,16}.sh` all committed, working tree clean for plan files
