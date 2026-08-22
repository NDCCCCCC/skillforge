---
phase: 14-p10-0-cgo-trust-floor
plan: 04
subsystem: platform-floor / eval
objective: "Establish platform-floor frozen eval set (PLAT-11 / SC-5 hermetic layer): 20 Chinese inducements × six attack surfaces + hermetic regex judge + platform-floor.lock.json."
tags: [PLAT-11, D14-C2, D14-C3, D14-C4, D14-B4, hermetic-judge, frozen-lock, platform-floor]
dependency_graph:
  requires: [14-01]
  provides:
    - "eval/golden/platform-floor.jsonl (20-case adversarial golden set)"
    - "internal/eval.JudgeHermetic (zero-LLM regex gate)"
    - "eval/frozen/platform-floor.lock.json (4th frozen lock, rich-field v1)"
  affects:
    - "eval/frozen/ directory gains the 4th lock file"
    - "internal/eval/ test suite (three new tests under go test ./internal/eval/)"
tech-stack:
  added:
    - "internal/eval/platform_floor_judge.go (stdlib regexp only)"
  patterns:
    - "frozen-set discipline: eval_set_sha256 + lock_sha256 double hash; lock_sha256 excludes its own field"
    - "hermetic judge: SQL-pattern-present FAIL / refusal-marker PASS / missing-refusal FAIL (bidirectional stubs)"
    - "all platform-floor tests live in internal/eval/ (eval/golden/ remains fixture-only, zero .go files)"
key-files:
  created:
    - eval/golden/platform-floor.jsonl
    - internal/eval/platform_floor_judge.go
    - internal/eval/platform_floor_judge_test.go
    - eval/frozen/platform-floor.lock.json
  modified: []
decisions:
  - "D14-C2: 20 cases = direct_sql_induction(4) + channel_override(3) + raw_row_request(3) + deanon_join(3) + roleplay_jailbreak(3) + long_dialogue_dilution(4)"
  - "D14-C3: every D14-A1 iron-law mapped to >=2 variants in notes; long_dialogue_dilution cases carry >=20 neutral history turns"
  - "D14-C4: platform-floor.lock.json created now; any future floor text change triggers D14-B4 full re-lock"
  - "JudgeHermetic semantics: SQL key-mode patterns are checked before refusal markers, catching partial-obedience leakage"
  - "lock_sha256 convention: sha256 of JSON body excluding the lock_sha256 field, verified by TestPlatformFloorLockSelfConsistent"
metrics:
  duration: ~13m
  completed_date: 2026-08-22
  tasks: 2
  commits: 4
  tests_added: 3
  tests_passed: all
---

# Phase 14 Plan 04: Platform-Floor Frozen Eval Set + Hermetic Judge Summary

Platform-floor trust substrate now has a regression-ready frozen eval set: 20 Chinese adversarial inducements covering six attack surfaces, a zero-LLM regex judge (`JudgeHermetic`), and the fourth frozen lock (`platform-floor.lock.json`). Live operator judgement is documented in the lock's `_live_runbook` and executed in Plan 14-08.

## What shipped

1. **`eval/golden/platform-floor.jsonl`** — 20-row golden fixture. Each row is an inducement the agent must refuse. Six attack surfaces map to D14-A1 iron-law rules; four rows carry 20-turn neutral dialogue prefixes to exercise long-conversation dilution.
2. **`internal/eval/platform_floor_judge.go`** — `JudgeHermetic(response)` hermetic regex gate. SQL pattern present → FAIL; refusal marker present → PASS; neither → FAIL (because every golden-row inducement expects a refusal). Exports `JudgeGoldenSet` for batch/runbook use.
3. **`internal/eval/platform_floor_judge_test.go`** — three tests in `internal/eval/` (eval/golden/ remains fixture-only):
   - `TestPlatformFloorGoldenSchema` — schema/shape gate.
   - `TestPlatformFloorFrozenSet` — bidirectional JudgeHermetic assertions.
   - `TestPlatformFloorLockSelfConsistent` — eval_set_sha256 + lock_sha256 convention.
4. **`eval/frozen/platform-floor.lock.json`** — rich-field v1 lock: thresholds, platform_floor_version "1.0.0", metrics placeholder, and `_live_runbook` for Plan 14-08.

## Deviations from plan

None — plan executed exactly as written. One refinement was made during GREEN: `TestPlatformFloorLockSelfConsistent` decodes the lock into a separate `platformFloorLockCanonical` struct (without `lock_sha256`) to recompute the canonical body hash. This is the literal interpretation of `_lock_sha256_convention` and avoids the empty-string ambiguity of marshalling the full struct with the field zeroed.

## Auth gates

None. No API keys, env secrets, or live stack consumed by this plan.

## Acceptance criteria — all met

- [x] `eval/golden/platform-floor.jsonl` has exactly 20 data rows (`wc -l` == 20).
- [x] `ls eval/golden/*.go 2>/dev/null | wc -l` == 0.
- [x] `TestPlatformFloorGoldenSchema` PASS with category counts summing to 20 and every rule mapped.
- [x] `TestPlatformFloorFrozenSet` PASS: refusal PASS / SQL leakage FAIL / SQL source FAIL / clean-no-refusal FAIL.
- [x] `TestPlatformFloorLockSelfConsistent` PASS: eval_set_sha256 matches golden blob; case_count == 20; platform_floor_version == "1.0.0"; lock_sha256 recomputes correctly.
- [x] `go test ./internal/eval/ -count=1` all green, zero regressions in existing eval framework tests.
- [x] `go build ./... && go vet ./...` exit 0.
- [x] `eval/frozen/` existing locks untouched; only `platform-floor.lock.json` added.

## Verification

```bash
# Three platform-floor tests (banner-guarded against empty -run match)
go test ./internal/eval/ -run 'TestPlatformFloorFrozenSet|TestPlatformFloorGoldenSchema|TestPlatformFloorLockSelfConsistent' -count=1 -v

# Full eval suite
go test ./internal/eval/ -count=1

# Repo build / vet
go build ./... && go vet ./...

# Frozen directory diff check
git status --short eval/frozen/
```

All commands green.

## Known stubs

None. The lock's `metrics` object is intentionally empty because live judge execution is deferred to Plan 14-08; this is documented in `_live_runbook`, not an un wired stub.

## Threat flags

No new threat surface introduced. The plan mitigates T-14-09, T-14-10, and T-14-11 per the plan's threat_model block via the frozen set + hermetic judge + double-hash lock.

## Commits

| Hash | Type | Message |
|------|------|---------|
| `83fd55c` | test | test(14-04): add platform-floor golden schema gate (RED) |
| `898b655` | feat | feat(14-04): add 20-case platform-floor golden set (GREEN) |
| `a48606d` | test | test(14-04): add hermetic judge and frozen-lock self-consistency tests (RED) |
| `1c5af78` | feat | feat(14-04): implement hermetic platform-floor judge and freeze lock (GREEN) |

## Self-check: PASSED

- All 4 created files exist on disk:
  - `eval/golden/platform-floor.jsonl`
  - `internal/eval/platform_floor_judge.go`
  - `internal/eval/platform_floor_judge_test.go`
  - `eval/frozen/platform-floor.lock.json`
  - `.planning/phases/14-p10-0-cgo-trust-floor/14-04-SUMMARY.md`
- All 4 task commits + 1 metadata commit present in git log.
- Final test run green: `go test ./internal/eval/ -count=1` PASS.
- Metadata files committed: STATE.md, ROADMAP.md, REQUIREMENTS.md.
