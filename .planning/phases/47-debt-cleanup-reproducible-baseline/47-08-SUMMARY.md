---
phase: 47-debt-cleanup-reproducible-baseline
plan: "08"
type: execute
wave: 4
status: complete
requirements: [REL-01, REL-02, REL-04]
files_modified:
  - verify-phase-47.sh
  - verify-phase-45.sh
  - verify-phase-46.sh
  - .planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md
  - internal/server/platform/verify_main.go (deleted, never tracked)
commits:
  - 8194cfd test(47-08): add zero-waiver Phase 47 gate at the repo root
  - 6b46bee chore(47-08): retire the D-45-DEF-01 waiver from the Phase 45/46 gates
  - 7382d69 docs(47-08): close D-45-DEF-02 by deleting the stale untracked harness
  - bdbfae4 docs(47-08): name the recording commit in the D-45-DEF-02 evidence cell
---

# Plan 47-08 Summary — zero-waiver phase gate, clean-checkout proof, stale harness removal

**Wave 4 (final plan of Phase 47).** Plan 47-06 left D-45-DEF-02 open with an explicit hand-off: the untracked harness had to be deleted by 47-08, and the evidence cell backfilled. Plan 47-03's fix made the D-45-DEF-01 waiver in the Phase 45/46 gates obsolete, and leaving it would have made the local gate disagree with the CI gate Phase 48 is about to publish. This plan closes both, and pins the claim with a gate that has no way to pass by naming a known failure.

---

## What shipped

| # | Task | Commit | Result |
|---|---|---|---|
| 1 | Create `verify-phase-47.sh` (7 partitions, zero-waiver) | `8194cfd` | exits 0, P1..P7 all PASS |
| 2 | Retire the D-45-DEF-01 waiver from `verify-phase-45.sh` / `verify-phase-46.sh` | `6b46bee` | both exit 0, no waiver path |
| 3 | Clean-clone proof + delete `internal/server/platform/verify_main.go` | `7382d69`, `bdbfae4` | clean checkout rc=0 × 4; D-45-DEF-02 closed |

---

## Task 1 — `verify-phase-47.sh`

Created at the repo root (mode 755), matching the Phase 40/41/45/46 convention. Seven fail-fast partitions:

| Partition | What it proves |
|---|---|
| **P1** | `go build ./...` + `go vet ./internal/{ontology,server,knowledge}/...` under `CGO_ENABLED=1` |
| **P2** | `./internal/ontology/...` green, exit-code authoritative |
| **P3** | the four Phase-47-repaired tests, paired with an anti-vacuous `--- PASS` floor |
| **P4** | `./internal/...` green, exit-code authoritative |
| **P5** | determinism strength: `-count=100` **and** one `-shuffle=on` run |
| **P6** | frontend line: `tsc --noEmit` + `vitest run`, Windows side |
| **P7** | the 19-ID disposition record, checked mechanically |

### P3 `pass_lines` floor — measured, not guessed

Measured with one `-v` run at execution time (2026-10-19), **floor = 6**:

```
--- PASS: TestDeriveEdgesDeterministic (0.00s)                          knowledge
--- PASS: TestEmitSkillInstanceCount (0.00s)                            ontology/emitter
--- PASS: TestEmitSubCategoryInstanceCount (0.00s)                      ontology/emitter
--- PASS: TestRollbackShaclParity (0.01s)                               ontology/induction
    --- PASS: TestRollbackShaclParity/env_list_consumed_violations_reported
    --- PASS: TestRollbackShaclParity/missing_shapes_warn_and_return_zero
```

1 + 2 + 1 + 2 sub-tests = 6. The other ontology packages legitimately report `no tests to run` for this `-run` filter and contribute no `--- PASS` line. The floor is anti-vacuous: a renamed or removed target test drops the count below 6 and fails the gate even with exit code 0. The number is recorded in a comment beside the check, with its decomposition.

### Zero waiver — proven, not asserted

| Check | Result |
|---|---|
| `grep -v '^#' verify-phase-47.sh \| grep -cE 'waiver_ok\|WAIVED_FAIL\|waived()\|t.Skip\|allowlist'` | **0** |
| `grep -vE '^[[:space:]]*#' verify-phase-47.sh \| grep -c -- '--- FAIL: Test'` (mechanical backstop against a test-name-keyed waiver) | **0** |
| `grep -c 'WAIVED_FAIL\|waiver_ok' verify-phase-47.sh` | **0** |
| `grep -v '^#' verify-phase-47.sh \| grep -n 'command -v npx'` | line 102 — the only non-fatal branch |

The gate names no specific failing test anywhere. The legitimate generic extraction regex inside `fail_detail` (`'--- FAIL: [^[:space:]]*'`) does not match the backstop pattern, so the backstop stays satisfiable while the diagnostics stay useful.

**P6's guard keys on a tool check alone.** `if ! command -v npx >/dev/null 2>&1; then info "… npx is absent from PATH on this host …"` — it never inspects a test name, a result, or a waivable condition. Every other command in the script is exit-code-authoritative.

### P6 runs on the Windows side, in a subshell

This box's WSL has no node, so P6 cannot go through `wsl()`. It runs as `( cd "$ROOT/web" && npx tsc --noEmit )` and `( cd "$ROOT/web" && npx vitest run )` — subshells against an absolute path, so the script's working directory is unchanged for every later partition. This matters: P7 resolves its record through `$ROOT`, and a shell left inside `web/` would break that path. `ROOT` is computed once as `ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`, so neither P6 nor P7 depends on the caller's cwd either.

**One deviation from the interface contract, forced by observed tool behaviour.** The plan's interface contract mandates an empty-log guard on every `go test` segment. P6's `tsc` step originally carried one too and **failed the gate on its first run**:

```
FAIL P6 tsc produced no output (rc=0) — infrastructure failure, no evidence of a typecheck
```

`tsc --noEmit` prints nothing on success (verified: `tsc_rc=0`, log size 0 bytes). Silence is its normal pass signal, so the empty-log guard is a Go-test discipline being misapplied to a typechecker. The guard was removed from the `tsc` step only, with a comment recording why; its exit code remains authoritative and `fail_detail` still prints diagnostics on failure. The empty-log guard is retained on every `go test` segment (P2, P3, P4, P5a, P5b) and on the `vitest` step (which always reports a summary line).

---

## Task 2 — the waiver is gone from the legacy gates

Deleted from both `verify-phase-45.sh` and `verify-phase-46.sh`: the named-constant allowlist, its predicate helper, and the `WAIVED` reporter. P2/P4 in both scripts are now strictly exit-code-authoritative with the empty-log guard.

`verify-phase-45.sh` needed more than deletion: it had **no exit-code check at all** on P2/P4 — it judged only by `waiver_ok`, the exact WR-06 failure mode that `verify-phase-46.sh` had already been hardened against. Both scripts now use the same discipline.

| Check | 45 | 46 |
|---|---|---|
| `grep -c "waiver_ok\|WAIVED_FAIL\|waived()"` | 0 | 0 |
| `grep -v '^#' <file> \| grep -c -i waiver` | 0 | 0 |
| `bash <file>` exit code | **0** | **0** |

Partitions, commands, thresholds and `-run` filters are unchanged — only the pass conditions and the header note changed. Header notes now say the waiver is retired as of Phase 47 because plan 47-03 hermeticized the fixture, and point at row `D-45-DEF-01` of the disposition record instead of quoting the retired constant.

**Why this matters beyond tidiness:** the allowlist was a *local WSL shell mechanism*. A bare `go test ./...` in GitHub Actions never reads it. So before this task the local gate could say green where CI said red — the exact inversion of what a published green light is supposed to mean. After it, no gate in the repository can pass by naming a known failure.

---

## Task 3 — clean-checkout reproducibility + D-45-DEF-02 closed

### D-04a: the whole sequence in one WSL invocation

A single `wsl.exe -d Ubuntu-22.04` call whose body creates its **own** Linux temp dir and does everything end to end. No path crosses the MSYS/WSL boundary: the gate's `WORK` is a Windows-side `mktemp` path while WSL has its own `/tmp`, and `MSYS_NO_PATHCONV=1` is exported, so a `$WORK/clean` created on the Windows side is not reliably addressable from inside WSL. `git archive HEAD | tar -x` — not a copy, because a copy carries untracked artifacts, which is precisely the divergence under test.

Verbatim output:

```
CLEAN_DIR=/tmp/verify47-clean-zhRUeH
ARCHIVE_RC=0
TRACKED skills dirs: 3
TRACKED sql-guides subdirs: 6
verify_main.go present in clean tree: NO
RC_build=0
RC_test_internal=0
RC_det100=0
RC_emitter=0
--- internal tail ---
ok  	github.com/skillforge/skillforge/internal/store	0.075s
ok  	github.com/skillforge/skillforge/internal/store/sqlite	6.618s
ok  	github.com/skillforge/skillforge/internal/ticket	5.670s
?   	github.com/skillforge/skillforge/internal/vecmath	[no test files]
ok  	github.com/skillforge/skillforge/internal/verdict	0.004s
ok  	github.com/skillforge/skillforge/internal/wire	0.016s
--- emitter log ---
ok  	github.com/skillforge/skillforge/internal/ontology/emitter	0.027s
CLEANED
PROBE_EXIT=0
```

The four commands and their raw exit codes, each printed immediately after it ran:

| Command | rc |
|---|---|
| `CGO_ENABLED=1 go build ./...` | **0** |
| `CGO_ENABLED=1 go test ./internal/... -count=1` | **0** |
| `CGO_ENABLED=1 go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100` | **0** |
| `CGO_ENABLED=1 go test ./internal/ontology/emitter/ -count=1` | **0** |

Two facts in that output carry the whole argument: the clean tree has only **3** tracked `skills/` directories and only **6** tracked `knowledge/sql-guides/` sub-directories (the developer tree has 8 and 6), and `verify_main.go` is **absent** from it. Any assertion still pinned to the external corpus fails here and nowhere else. Local gate and CI gate now say the same thing.

### D-45-DEF-02: the stale untracked harness is gone

Evidence captured before deletion:

```
git ls-files internal/server/platform/verify_main.go   → (empty)
git log --all -- internal/server/platform/verify_main.go → (empty)
wc -l    → 755
sha256   → 8c694fc8f7fb79388acab35c26e6fd8c84b1d33dbe625751a8dcf1d9b7f3e83e
mtime    → 2026-09-23 15:00:49 +0800   (untouched since Phase 44)
```

Runtime-stale for exactly the four reasons plan 47-06 recorded: pre-44 fixture, `owl:Class` IRIs, raw counts, label-as-schema assertions — all detached from the post-Phase-45 emitter / `isSchemaTriple`. It compiled locally only because it sat on disk.

**A note on the "deletion commit SHA".** The plan asked for the resulting commit SHA to be backfilled into the evidence cell. There is no such SHA, and citing one would be false: the file was never tracked, so deleting it produces no diff at all — `git log --all` on that path returns nothing across the entire repository history. The commit that *records* the deletion is `7382d69`, which carries the evidence-cell backfill; the backfill cell says exactly this rather than inventing a SHA. (Named explicitly in `bdbfae4` so an auditor does not have to infer it.)

Harness **rewrite** stays out of scope, as plan 47-06 decided and this plan confirms: `grep -rn verify_main internal/ cmd/ web/` finds no call site. A rewrite would be invention.

### Other untracked leftovers — recorded, not deleted

Per Task 3 step 3, `internal/server/skillforge-server-test-*` still exists as **9** directories: `1315038830`, `2346041323`, `2516342759`, `282873371`, `2853703676`, `2861472387`, `2967410029`, `4158293520`, `480999863`. They are historical test temp dirs, outside D-04d's scope, so they were **not** deleted — but they are now named in the disposition record rather than left implicit.

The acceptance check `git status --porcelain internal/ | grep -v '^?? internal/server/skillforge-server-test-'` is **empty**: the exemption is narrowed to that exact path prefix, so any *other* stray untracked file under `internal/` still fails. Qualified, not disabled.

### Gate still green after the deletion

```
PASS P1 build + vet
PASS P2 ontology suite
PASS P3 targeted repairs (6 --- PASS, floor 6)
PASS P4 full internal suite
PASS P5a determinism -count=100
PASS P5b determinism -shuffle=on
PASS P5 determinism strength (100x + shuffle)
PASS P6a tsc --noEmit
PASS P6b frontend vitest run
PASS P7 disposition record (19 IDs, each one row, closed 去向, non-empty 证据)
=== Phase 47 Gate: ALL PASSED ===
GATE_EXIT=0
```

---

## Final gate run (real tail + real exit code)

```
PASS P5b determinism -shuffle=on
PASS P5 determinism strength (100x + shuffle)
INFO P6: frontend (tsc --noEmit + vitest run)
PASS P6a tsc --noEmit

 Test Files  85 passed (85)
      Tests  688 passed (688)
   Duration  19.32s

PASS P6b frontend vitest run
INFO P7: disposition record (19 IDs, closed vocabulary, non-empty evidence)
PASS P7 disposition record (19 IDs, each one row, closed 去向, non-empty 证据)

=== Phase 47 Gate: ALL PASSED ===

Partitions verified:
  P1 build + vet            — CGO_ENABLED=1 build ./... + vet ./internal/{ontology,server,knowledge}/...
  P2 ontology suite         — shacl/store/induction/emitter, exit-code authoritative
  P3 targeted repairs       — 4 repaired tests, >= 6 --- PASS (floor measured 2026-10-19)
  P4 full internal suite    — ./internal/... hermetic, exit-code authoritative
  P5 determinism strength   — TestDeriveEdgesDeterministic -count=100 + one -shuffle=on run
  P6 frontend line          — tsc --noEmit + vitest run (Windows side; npx-absent guard is a tool check)
  P7 disposition record     — 19 v1.6 leftover IDs, closed 去向 vocabulary, non-empty 证据

GATE_EXIT=0
```

**688 tests / 85 files / 0 failed / 0 skipped.** Go: build OK, vet OK, `./internal/...` all `ok`, zero failures, zero waivers.

---

## P7's mechanical check — proven to fail, then reverted

A gate that can pass by naming a known failure is worthless if its own check is decorative. So the disposition check was made to fail on purpose: `47-DEBT-DISPOSITION.md` line 40 (row `D-45-DEF-02`) had its 证据 cell blanked to `|  |`, then the gate was run:

```
PASS P6b frontend vitest run
INFO P7: disposition record (19 IDs, closed vocabulary, non-empty evidence)
FAIL P7 D-45-DEF-02 证据 cell is empty — the record may not satisfy a row with an empty evidence promise
GATE_EXIT=1
```

The file was then restored from a backup; `git diff` on it came back clean and the 证据 cell measured 758 chars again. P7 checks each of the 19 IDs for **exactly one** table row (positive match count, never a bare `== 0` on an unfiltered file), a 去向 inside the closed vocabulary `v1.7 修复` / `延期+理由` / `ack backlog`, and a non-empty 证据 cell. Comment lines are stripped into a scratch copy first, so a comment mentioning an ID can neither satisfy nor break the per-ID count. The check was written against the file's real measured shape — 19 data rows at lines 39–57, each splitting into 6 pipe-separated fields, 去向 at field 4 and 证据 at field 5.

---

## An honest finding, outside this plan's fix scope

While retiring the waiver, `verify-phase-46.sh` P4 failed **once** with `TestFailClosedAudit` and `TestWriteTodosAuditedToolAuditFailClosed`, then passed on every subsequent run. That is worth recording rather than shrugging at, because a strict gate turns a previously-invisible flake into a red light.

Root cause, isolated rather than guessed:

- 6 serial runs of the two packages: rc=0 every time.
- 8 serial runs of the full `./internal/...` suite: rc=0 every time (8/8).
- 8 **concurrent** full suites: **every** run failed, with a recurring set — `TestFailClosedAudit`, `TestWriteTodosAuditedToolAuditFailClosed`, `TestAuditWriterPreconditions` (up to 137s wall clock), plus load-shaped failures in `TestDuckDBConcurrentReadNoBlockWrite`, `TestCancelPropagationMidToolRunner`, `TestAdminPipelineRunConflict`, `TestRewrite_TimeoutFallback`.

The shape says timeout, not nondeterminism: those tests open a file-backed SQLite store under `context.WithTimeout(ctx, 5*time.Second)` (`internal/agent/hitl/audit_writer_test.go:59`, and the same pattern across `internal/agent/deep_audit_test.go`). Under 8x oversubscription the open takes 40–137s and the 5s budget expires. Under one suite at a time it never does.

**Not fixed here, deliberately.** Two reasons: it is a pre-existing condition unrelated to any Phase 47 change (it predates this plan and is unrelated to D-46-DEF-02's edge-direction root cause), and the fix — widening the timeout or converting to a shared fixture — is a change to tests this plan has no mandate over. Phase 48's CI runs one job per line, not eight concurrent copies, so the public green is not at risk. No waiver was added for it, which is the whole point of this plan. Flagged here for the verification step.

---

## Files touched

| File | Change |
|---|---|
| `verify-phase-47.sh` | **new**, mode 755, 7 partitions, zero-waiver |
| `verify-phase-45.sh` | waiver mechanism deleted; P2/P4 made exit-code-authoritative; header note updated |
| `verify-phase-46.sh` | same |
| `.planning/.../47-DEBT-DISPOSITION.md` | D-45-DEF-02 evidence cell completed (backfilled twice: content, then the recording commit SHA) |
| `internal/server/platform/verify_main.go` | **deleted** (755 lines, never tracked — no diff, no commit of its own) |

`.planning/STATE.md` and `.planning/ROADMAP.md` were **not** touched by this plan.

---

## Hand-offs

- **`47-VERIFICATION.md` frontmatter** — plan 47-06 asked that it carry a D-03b pointer line to `47-DEBT-DISPOSITION.md`, following the Phase 44 `deferred: addressed_in / evidence` precedent. Not this plan's file to create.
- **`internal/server/platform/verify_main.go` rewrite** — remains uncommissioned (BL-05 backlog candidate), per 47-06.
- **`internal/server/skillforge-server-test-*`** (9 dirs) — recorded, not deleted; outside D-04d.
- **Load-induced SQLite timeout flake** — recorded above, not waived, not fixed.
