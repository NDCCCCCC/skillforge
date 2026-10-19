# Phase 48 — Revision Iteration 3 Handoff

**Written:** 2026-10-19 · **Reason:** context exhaustion (8% remaining) mid-revision-loop
**Resume:** `/gsd-plan-phase 48` — RESEARCH.md / VALIDATION.md / 4×PLAN.md all exist, so the run goes straight to the plan-checker loop. Nothing upstream re-runs.

## Loop state

| Iteration | Outcome | BLOCKER | WARNING | INFO |
|---|---|---|---|---|
| 1 (initial plan) | ISSUES FOUND | 1 | 2 | 3 |
| 2 (after revision 1) | ISSUES FOUND | 1 | 3 | 3 |

`prev_issue_count` = 4. `stall_reentry_count` = 0. **Iteration 3 is required and is the last one** (cap is 3); if it does not reach 0 BLOCKER/WARNING, the loop escalates to the iteration-cap gate rather than running a 4th time.

Iteration 2's blocker is **new** — it lives in 48-04 Task 1, which iteration 1 introduced/left broken. It was not a regression in an already-verified area.

## Iteration 3 issue list (verbatim from gsd-plan-checker)

```yaml
issues:
  - plan: "48-04"
    dimension: verify_command_runnability
    severity: blocker
    required_property: "48-04 Task 1's negative self-check must remove the P9 subject file before re-running the gate, so a correct implementation exits 0 and a dishonest verifier exits non-zero."
    description: >-
      The <verify> does `bak=$(mktemp -d)/eg.bak; cp .github/workflows/eval-gate.yml "$bak";
      trap ... EXIT; out=$(bash verify-phase-48.sh 2>&1); rc=$?; ...` — it copies the subject
      to a backup but never removes or renames it (no `rm`, no `mv`). On a correct implementation
      the gate therefore still sees an intact eval-gate.yml, exits 0, `rc=0`, and
      `test "$rc" -ne 0 || exit 1` fires -> SELFCHECK-OK is never printed -> the trailing
      `&& test -s ... && echo P9-OK` never runs and the verify fails unconditionally. The revision
      claim "move/restore wrapped in trap … EXIT" is not what the text says (it is a `cp`).
    task: 1
    fix_hint: "Change the copy to a move (`mv .github/workflows/eval-gate.yml \"$bak\"`), keeping the existing `cp -f` restore and EXIT trap; the later `test -s`/`grep -q eval-gate` tail then also has something real to assert."

  - plan: "48-04"
    dimension: task_internal_consistency
    severity: warning
    required_property: "48-04 Task 2 must state one implementable P8/P11 relationship; the vacuity probe must exercise the function P8 actually uses, or say plainly that it does not."
    description: >-
      Task 2 contradicts itself three ways. Action line 98: "P8 calls the same function with the
      real directory and **P8 is not modified**". Acceptance line 115: "P11 half 2 probes a
      **parameterized scan function of its own**; P8's body is **byte-identical** to plan 48-02's
      version". Those cannot all hold: P8 either calls the new function (body changes) or it does
      not (then the probe is P11-private and proves nothing about the real scan — T-48-11 half 2
      becomes decorative, and P8's `every value ∈ {read, none}` stays vacuously satisfiable, which
      is the exact defect the partition exists to catch). Additionally, line 100 forbids touching P8
      by citing "48-02's `P1–P9 unchanged` invariant", which does not exist: 48-02 line 175 says
      "P1–P4 are unchanged" (the P1–P8 wording lives in 48-04 Task 1, the P1–P9 wording in Task 2's
      own AC). So the cross-plan compatibility the revision claimed to establish is not established.
    task: 2
    fix_hint: "Pick one: (a) shared function — P8's body is replaced by a call to scan_trust_surface, its *predicates* unchanged, and drop the 'byte-identical' / 'of its own' wording and the phantom 48-02 citation; or (b) P11-private copy — then state explicitly that the probe demonstrates the vacuity rules are implementable, not that P8 enforces them, and record that P8's empty-`jobs`/empty-`permissions` gap remains open."

  - plan: "48-04"
    dimension: task_completeness
    severity: warning
    required_property: "When a workflow does not trigger, the plan must name the cause correctly and prescribe a remedy that does not contradict a bound decision (D-24 / Pitfall 10 / T-48-13's zero-weakening rule)."
    description: >-
      Task 3 step 3 says: if `gh run list --workflow ci.yml --branch <branch>` returns nothing,
      "the workflow did not trigger — most likely because the base branch is not main and not
      matching `phase-*`; fix the trigger rather than waiting." Both halves are wrong for the
      execution this plan will run: 48-01 Task 3 binds `push.branches: [main]` (Pitfall 10), and the
      working branch is a `phase-*` branch, so a plain `git push` produces **no** run — there is no
      PR involved at all, so "base branch" is not the variable. Following the written remedy means
      widening `push.branches` (or adding `workflow_dispatch:`), which the same phase forbids in
      48-01 Task 3 and T-48-13 classifies as weakening a gate to reach green. Separately, the
      action resolves the run id with `--branch <current-branch>` but the <verify> drops `--branch`
      and takes `--limit 1`, so it can assert green from an unrelated branch's run.
    task: 3
    fix_hint: "Replace the remedy with the trigger-preserving path (open a PR whose base is `main`, or push to `main`), state that `push.branches: [main]` is intentional per Pitfall 10 and must not be widened, and add `--branch` to the verify's `gh run list` so it inspects the run it just pushed."

  - plan: "48-01"
    dimension: verify_command_runnability
    severity: warning
    required_property: "The probe must assert that the toolchain P1–P3 depend on (python) is still reachable **under $CLEANPATH**, not merely on the unmodified PATH."
    description: >-
      Task 4's probe self-check is `command -v python >/dev/null 2>&1 || { echo "PROBE-BROKEN:
      python absent before probe"; exit 2; }` — run before the surgery, under the full PATH. The
      revision claimed the probe "self-checks that Python survived"; the text only establishes a
      baseline. If `golangci-lint`'s directory also contains python (plausible for a user-local Go
      bin dir), the surgery removes it, P1/P2/P3 cannot run under `$CLEANPATH`, and the probe still
      reports success — because P4's preflight is shell-only by mandate, it prints `LINT NOT
      VERIFIED` and exits 1 regardless. The primary clause (the harness is not dishonest) still
      holds, so this is a gap in what the probe proves, not a vacuous pass.
    task: 4
    fix_hint: "Move the reachability assertion inside the neutralized environment, e.g. `PATH=\"$CLEANPATH\" command -v python >/dev/null 2>&1 || { echo \"PROBE-BROKEN: python removed by the surgery\"; exit 2; }`, keeping the existing post-check for the linter."

  - plan: "48-03"
    dimension: threat_model_consistency
    severity: info
    required_property: "The renumbered threat ID is reflected in every phase-level mapping that names it, not only in the two plan tables."
    description: >-
      48-02 (line 208) and 48-03 (line 185) both carry the canonical note that the skipped-job
      false-green threat is `T-48-15` and that `T-48-03` is canonically the `paths:`-filter threat —
      the two plan tables agree. But `48-VALIDATION.md` still assigns `T-48-03` to P4 (skip-notice
      job, line 49) while also assigning it to P17 (paths filter, line 62).
    task: null
    fix_hint: "Update the P4 row's Threat Ref from `T-48-03` to `T-48-15` in the 48-VALIDATION.md per-task verification map."

  - plan: "48-01"
    dimension: verify_command_runnability
    severity: info
    required_property: "Literal strings that later plans count with `grep -c == 1` must not be introduced into the script as prose."
    description: >-
      Task 4's P4 action instructs "Never print `ALL PASSED` on that path", while 48-02 Task 3
      (`test "$(grep -c 'ALL PASSED' …)" = "1"`) and 48-04 Task 2 (same) assert the string occurs
      exactly once. The plan already applies this discipline to `continue-on-error`
      ("appears nowhere, not even in a 'must not' comment") but not to `ALL PASSED`.
    task: 4
    fix_hint: "Add the same 'phrase appears nowhere, not even in a comment' clause to Task 4's acceptance criteria for `ALL PASSED`."

  - plan: "48-03"
    dimension: verify_command_runnability
    severity: info
    required_property: "The one-line-diff bound should remain decidable regardless of whether the executor has already committed the edit."
    description: >-
      Task 3's guard is `{ git diff --quiet -- .planning/ROADMAP.md || test "$(git diff -U0 …
      | grep -cE '^[+-][^+-]')" -eq 2; }`. `git diff --quiet` only sees the working tree, so once
      the task commit exists the guard short-circuits and the breadth bound is skipped rather than
      asserted. The content greps still run in that state, so the verify cannot pass on a wrong
      edit — only the breadth bound goes unchecked.
    task: 3
    fix_hint: "Use `git diff HEAD -- .planning/ROADMAP.md` in both the guard and the count so staged-and-committed-since-HEAD edits are still measured, keeping the existing skip when the file is genuinely untouched."
```

## Properties verified clean in BOTH checker runs — the revision must not break them

- Wave/dep graph 1→2→3; 48-02 ∥ 48-03 disjoint files
- `EXPECTED_PARTITIONS` single owner per wave: 4 → 8 → 9 → 11
- `pull_request.branches: [main, phase-*]` trigger (justified by real `eval-gate.yml:34`)
- No hard-coded test counts anywhere; the ROADMAP `666 Go 测试 + 84 FE 测试` figure is an anti-drift assertion, not a count gate
- Zero-waiver standard: no `continue-on-error` / `t.Skip` / `WAIVED` / `|| true` in any gate
- WSL-vs-Windows-host grounding: Go only under `wsl.exe -d Ubuntu-22.04`, FE on the Windows host
- `autonomous: false` + `checkpoint:decision` on 48-04 (required status checks rated one-way; branch protection currently 404s so it is not a phase-close blocker)
- Threat model in all four plans, ASVS L1 / block on `high`, no `high` left accepted

## Environment facts for the iteration-3 planner

- `golangci-lint` → `/c/CPIC/go/bin` (Go dev tools only) · `python` → `/c/Users/CPIC/AppData/Local/Programs/Python/Python313/python`
- Go build/test runs ONLY under WSL: `wsl.exe -d Ubuntu-22.04 -- bash -c "cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ..."`
- Measured suites (2026-10-19): `go test ./...` → exit 0, 47 packages ok / 8 no-test / 0 FAIL · `npx vitest run` → 85 files / 689 tests / 27.7s
- Probes on the current plans: `check verify-command-paths 48` → 14 `none`, 0 blocker/warning, no readError · `check verify-failure-directions 48` → 14 `none`, 0 blocker/warning, no readError

## Decisions locked this run (D-29 / D-30 were approved by the user, not derived)

- **D-29** — `permissions: contents: read` + `pull-requests: read` on the `go` job; still zero `write` anywhere. `only-new-issues` silently degrades to whole-repo lint on 403.
- **D-30** — `actions/checkout@v7` lives in each job, not in the composite. A local composite cannot bootstrap its own checkout.
- **Premise falsified** — "GHA cache needs `actions: write`" is false; cache access is scoped by event/ref, not by the `permissions` block. A `failed to reserve cache` is an event/ref question.

---

*Iteration 3 is the last. If BLOCKER/WARNING count does not reach 0, the loop stops at the cap and escalates — do not start an iteration 4.*
