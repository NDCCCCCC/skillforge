---
phase: 48-ci
reviewed: 2026-10-19T10:23:38Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - .github/actions/setup/action.yml
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - .golangci.yml
  - verify-phase-48.sh
findings:
  critical: 1
  warning: 8
  info: 10
  total: 19
status: issues_found
---

# Phase 48: Code Review Report

**Reviewed:** 2026-10-19T10:23:38Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 48's deliverable is the CI policy surface: a composite toolchain action, two workflows, a
golangci-lint v2 config, and `verify-phase-48.sh` (990 lines, 11 partitions) that statically asserts
the properties of all of them. I reviewed all five files and, because the verifier's whole claim is
"the gate will really run and really bite", I **executed** it and then ran adversarial mutations
against copies of the workflows/config in a temp directory (no repository file was modified).

Baseline observation: `bash verify-phase-48.sh` exits 0 and prints `ALL PASSED` on the current tree
with golangci-lint 2.12.2 present. P4's three sentinels behave as advertised (errcheck fires, the
deferred-Close and unused-parameter classes stay quiet). `.golangci.yml` is principled in the D-03
sense — noise classes are absorbed by presets and one narrow, justified rule disable, with no
hand-written finding allowlist, and the config is accepted by `config verify` under the pinned
binary.

The verifier, however, has a coverage surface considerably narrower than what it prints, and I was
able to produce a green `ALL PASSED` in several states where the property the partition asserts is
no longer true. The strongest of these (CR-01) is a *present-tense* claim/coverage mismatch: P8
prints "directory-wide" but its path-filter and secret rules are hard-wired to one filename. Two
further classes of the same defect are prospective (a future edit disables a gate and the verifier
still reports green) — WR-01, WR-07 — and the phase's own framing asks for exactly this discipline.

On the workflow side, the YAML is largely well-built: read-only permissions with one narrowly
justified extra scope, no `pull_request_target`, no mutable `paths:` trigger filters, no secrets in
`ci.yml`, and a real (if not quite exact — WR-02) fork/Dependabot gate on the live workflow. Two
gate-integrity defects stand out: the eval-gate replay step swallows the eval binary's exit code
through `tee` (WR-03), and the live secrets are mounted job-wide, i.e. visible to tag-pinned
third-party actions that do not need them (WR-08).

The phase context's stated non-defects were respected: the statically-asserted-only Docker
declarations, the absence of any executed CI run, and the pre-existing/hardened status of
`eval-gate.yml` are not counted against the phase. Note that the ROADMAP itself records CICD-01/02/03
as still Pending (no `ci.yml` run has ever existed), so every workflow finding below is
reasoning-from-text, not from an observed red run.

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01: P8's path-filter and secret rules only ever run against a file literally named `ci.yml`, while the partition reports directory-wide coverage

**File:** `verify-phase-48.sh:580-584, 636-644, 659-675` (claim sites: `verify-phase-48.sh:36-37, 984`)
**Issue:** `scan_trust_surface` reads `ci="$dir/ci.yml"` (line 590) and then:

- runs the path-filter rules (`paths:`, `paths-ignore:`) against `ci_src` only (lines 663-669), and
- runs the secret regex against `ci_src` only (lines 671-675),

while every other rule in the same function is per-file (`permissions`, `write` scope, registry
tokens) or directory-wide (`find` enumeration). The partition header (line 36-37) and the closing
banner (line 984) both advertise this as directory-wide: "the whole workflow directory: read-only
permissions, zero registry surface, **zero path filters, zero secrets beyond the runner token**".

Demonstrated (temp fixtures, exit 0 = pass):

```
# wf3/ci.yml = a byte-copy of the real ci.yml ; wf3/release.yml = a NEW workflow containing
#   on.push.paths: ['internal/**']      <- T-48-03 hazard (required check Pending forever)
#   ${{ secrets.DEPLOY_KEY }}           <- T-48-04 hazard (secret in a public repo)
#   docker push ghcr.io/org/img:latest  <- T-48-10 hazard (publication capability)
$ python scan.py <wf3/ci.yml> <wf3/ci.yml> <wf3/release.yml>
TRUST-SURFACE-OK (2 workflow file(s))
rc=0
```

Two distinct consequences:

1. **Silent fail-open.** The next workflow added to `.github/workflows/` — for example a release
   workflow in Phase 51, which the composite's own header says will reuse these files — escapes the
   path-filter and secret guards entirely, while the partition still prints a green line that asserts
   they were checked directory-wide. This is the "reported green but did not run" class the script's
   own header (lines 10-11) names as the reason it exists.
2. The printed evidence is false today: `TRUST-SURFACE-OK (2 workflow file(s))` is adjacent to a
   banner claiming directory-wide path/secret coverage that was computed from one file.

**Fix:** hoist both rules out of the `ci_src` block and apply them per-file inside the existing
`for p in paths:` loop, e.g.

```python
for p in paths:
    src = open(p, encoding="utf-8").read()
    ...
    # T-48-03: a path-filtered required check sits Pending forever.
    for bad in ("paths:", "paths-ignore:"):
        if bad in src:
            raise SystemExit("trust-surface: %s contains %s (T-48-03)" % (p, bad))
    # T-48-04 / D-14: only the runner token may be referenced.
    for name in re.findall(r"secrets\.([A-Za-z_][A-Za-z0-9_]*)", src):
        if name != "GITHUB_TOKEN" and "eval-gate" not in os.path.basename(p):
            raise SystemExit("trust-surface: %s references secrets.%s (T-48-04)" % (p, name))
```

The eval-gate exemption must be an explicit, named allowlist (that file legitimately references three
secrets, and P9 already asserts those three are intact), not the current "whatever is in ci.yml is
fine" accident. Keep `ci_src` only if some rule genuinely needs the ci.yml subject, and if so, say so
in the header instead of claiming directory-wide coverage.

### Warnings

#### WR-01: P2/P3 reject a job-level `if:` but not a step-level one — the gate steps can be disabled while the partitions still pass

**File:** `verify-phase-48.sh:228-229` (and the step selection at `198-210`, `233-240`)
**Issue:** Line 228 checks `if "if" in j` — the **job** mapping only. The step-level `if:` is never
inspected, and the command fragments are only substring-matched against the concatenated `run`
strings, so a step can keep its text and never execute.

Demonstrated (mutated copy of `ci.yml` in a temp dir; the real file was not touched):

```yaml
- name: go test ./... (CICD-01 / D-15)
  if: false                      # added
  run: go test ./... -count=1
- name: lint (changed lines)
  if: false                      # added
  uses: golangci/golangci-lint-action@v9
```

Result: `P2 all fragments still present: True`, `P3 lint step count: 1 version: v2.12.2`,
`P3 job-level 'if' present: False` → P2 and P3 both PASS. The lint/test gates are dead and the
verifier prints `P3 lint-line (pinned v2.12.2, blocking, ...)`.

Note the asymmetry that shows the intent: step-level `continue-on-error` **is** caught, because
`soft_key in src` (line 226) scans the whole file — the same technique is simply not applied to
`if:`. This is exactly the negative-probe failure the phase brief calls out ("a check that only ever
passes because the thing it guards against is absent by default is not a check").

**Fix:** add a step-condition rule to the P2/P3 probe, allowing only the one deliberate exception:

```python
for s in steps:
    cond = s.get("if")
    if cond is None:
        continue
    if any(frag in (s.get("run") or "") for frag in (
        "go build ./...", "go vet ", "go test ./... -count=1", "-shuffle=on",
    )) or "golangci/golangci-lint-action" in (s.get("uses") or ""):
        raise SystemExit("P2/P3: gate step %r carries `if: %r` — the gate is skippable"
                         % (s.get("name"), cond))
```

Apply the same rule to P6's three `fe` steps and P7's two build steps (both currently have the same
hole).

#### WR-02: `eval-gate-skip-notice` is not the complement of the gate — a Dependabot push skips the live gate with no notice

**File:** `.github/workflows/eval-gate.yml:56-59, 127-132` (claim at `123-126`)
**Issue:** The comment claims "This job runs on exactly the complement of the gate above". It does
not. Gate: `(event != 'pull_request' || same_repo) && actor != 'dependabot[bot]'`. Notice:
`event == 'pull_request' && (fork || dependabot)`. Evaluating the omitted region — a non-PR event
whose actor is `dependabot[bot]`:

| event, actor | gate job | notice job | GitHub shows |
|---|---|---|---|
| pull_request, fork | skipped | runs (`::notice::`) | notice visible |
| push to main, `dependabot[bot]` | **skipped** | **skipped** | **green, no notice** |

The gate's own comment (lines 50-53) states that secrets are withheld from "fork PRs **and
Dependabot runs**", i.e. the skip is intended for both events — but the notice only covers the PR
half. The D-10 rationale ("a skipped job still reports Success, so the skip is indistinguishable from
a real live-gate pass") therefore still holds for Dependabot-attributed pushes to `main`, which is
precisely the state the sibling job was added to make visible. I did not verify on a runner whether
Dependabot auto-merge attributes the resulting push to `dependabot[bot]`; regardless, the comment's
"exactly the complement" claim is provably wrong and the coverage gap is real for whatever actor
value produces it.

**Fix:** make the notice the true complement by dropping the `pull_request` requirement, or
negating the gate condition verbatim:

```yaml
  eval-gate-skip-notice:
    name: eval-gate skipped (fork / Dependabot)
    if: >
      !((github.event_name != 'pull_request' ||
         github.event.pull_request.head.repo.full_name == github.repository) &&
        github.actor != 'dependabot[bot]')
```

Then assert complementarity mechanically in P9 (parse both expressions, or at minimum require the
notice's expression to contain `!(`) so the two cannot drift apart again.

#### WR-03: the live replay step swallows the eval binary's exit code through `tee`

**File:** `.github/workflows/eval-gate.yml:93-101`
**Issue:**

```yaml
        run: |
          mkdir -p eval/reports/ci
          ./bin/eval \
            --frozen compliance \
            ... \
            2>&1 | tee eval/reports/ci/run.log
```

GitHub Actions' default shell for `run` on Linux is `bash -e {0}`; `pipefail` is only included when
`shell: bash` is set explicitly. With no `shell:` key on this step, the pipeline's status is `tee`'s
status, so **every** non-zero exit of `./bin/eval --frozen compliance` is discarded and the step
named "Run frozen compliance replay" reports success.

This is not theoretical. `cmd/eval/main.go` has real post-report fatal paths:

- `cmd/eval/main.go:648` — `eval.WriteReport(...)` succeeds, writing a complete, valid report;
- `cmd/eval/main.go:653` — a subsequent `writeCalibration` failure calls `log.Fatalf`, exiting 1;
- the "Gate check" step then reads the already-written report and can pass on it.

So a run whose grader crashed after scoring, or whose calibration/archive write failed, yields a
green gate built on a report the tool itself considered a failed run. The same masking applies to any
future `log.Fatalf` added after the report write.

**Fix:**

```yaml
      - name: Run frozen compliance replay
        shell: bash          # bash -eo pipefail
        working-directory: ${{ github.workspace }}
        run: |
          set -o pipefail
          mkdir -p eval/reports/ci
          ./bin/eval ... 2>&1 | tee eval/reports/ci/run.log
```

(add `set -o pipefail` even with `shell: bash`, so the intent survives a later shell-key removal.)

#### WR-04: P4 cannot detect that `revive` was dropped from the config — sentinel C passes vacuously

**File:** `verify-phase-48.sh:394-417` (config under test: `.golangci.yml:38-48`)
**Issue:** Sentinel A is a positive probe (errcheck must fire) and B/C are negative probes. Nothing
in P4 is a positive probe for `revive`. If the `linters.enable: [revive]` entry vanished, an
unused parameter stays quiet for the trivial reason that revive is not running — sentinel C still
passes — and D-03's entire justification ("revive's `unused-parameter` is off because it false-fires
on MCP-shaped handlers") would describe a rule disable that no longer means anything.

Demonstrated with a temp config identical to `.golangci.yml` minus the `enable: [revive]` entry and
its settings:

```
config verify            -> rc=0            (accepted)
sentinel A (unchecked)   -> rc=1, errcheck named   (P4 sentinel A passes)
sentinel C (unused param)-> rc=0            (P4 sentinel C passes)
=> P4 would report "lint-teeth" PASSED with revive absent
```

**Fix:** add a positive revive sentinel alongside A, using a violation that cannot collide with the
intentional disable (the config's own note at lines 25-28 shows the intended one):

```bash
SD="$WORK/sentinel-d"   # missing package comment -> revive `package-comments` must fire
cat > "$SD/main.go" <<'GOEOF'
package main

func main() {}
GOEOF
lint_run "p4-sentinel-d.log" "$SD" run --config "$ROOT/.golangci.yml" ./...
[ "$p4_d_rc" -ne 0 ] || fail "P4 sentinel D: revive reported nothing on a file with no package comment — revive is not running"
grep -q 'revive' "$WORK/p4-sentinel-d.log" || { tail -20 "$WORK/p4-sentinel-d.log"; fail "P4 sentinel D: the run failed but did not name revive"; }
```

This also makes the `.golangci.yml:25-28` note ("P4 asserts the current tree still configures
cleanly under this rule") true instead of aspirational.

#### WR-05: `.yaml` workflows are invisible to P1 and P8

**File:** `verify-phase-48.sh:132, 592` (contrast: `702-808` P9, which does include `.yaml`)
**Issue:** `find ... -name '*.yml'` — GitHub Actions accepts both `.yml` and `.yaml`, and P9's own
`os.walk` filter (line 775) uses `(".yml", ".yaml")`, so the script is internally inconsistent about
what a workflow is. A `.yaml` workflow escapes P1 (YAML parse, trigger-block read,
`pull_request_target` scan) and P8 (permissions ceiling, write-scope, registry tokens, path filters,
secrets) and is never counted by the "a partition that finds zero files must not pass" guards,
because those guards only prove that *at least one `.yml`* exists.

Demonstrated: in a fixture directory containing `ci.yml` and `other.yaml`, `find … -name '*.yml'`
returns `['ci.yml']` and a Python `glob('*.yml')` reports `other.yaml enumerated: False`.

**Fix:** widen both enumerations to match P9:

```bash
find "$dir" -maxdepth 1 -type f \( -name '*.yml' -o -name '*.yaml' \) -print | sort > "$list"
```

#### WR-06: on `push` to main the lint step has no PR diff to filter by; the repository currently yields 45 findings under this config

**File:** `.github/workflows/ci.yml:33-37, 74-78`
**Issue:** The workflow comment states the mechanism itself: "the lint action has no diff on a
manual/push run and would fall back to whole-repo lint over ~175 legacy findings". Measured on this
tree:

```
$ golangci-lint run --config .golangci.yml ./...   # (CGO_ENABLED unset locally; one package failed typechecking)
45 issues:
* errcheck: 4  * ineffassign: 1  * revive: 27  * staticcheck: 8  * unused: 5
LINT_EXIT=1
```

So on every `push` to `main` the `go` job runs the same configuration that reports these findings and
exits 1. Either the comment is wrong (then it should be corrected, and the invariant reviewers rely
on is not what the file says), or the required `go` check is red on `main` by construction — CI
greening only on the PR path while `main` carries a permanent failure. I could not execute
`golangci-lint-action@v9` to observe its push-event behaviour, so I am reporting the two branches
rather than asserting which one holds.

**Fix:** pick one and make it explicit:

- give the push run a diff base — pass `args: --new-from-rev=${{ github.event.before }}` (or a
  `--new-from-rev=origin/main` equivalent), so the whole-repo fallback never happens; or
- gate the lint step to PRs (`if: github.event_name == 'pull_request'`) and let the push run cover
  build/vet/test only; or
- if permanent red on `main` is actually accepted, say so in the header — currently the file reads
  as if it is a defect someone already decided not to have.

#### WR-07: P9's fork gate is asserted by three independent substrings, so an expression that neutralizes the gate still passes

**File:** `verify-phase-48.sh:721-730`
**Issue:** The comment above the check says a substring search for the whole expression would pass on
two clauses out of three; the implemented fix is three separate substring searches, which has the
same weakness in the other direction — the clauses can all be present while the expression no longer
gates anything. Demonstrated by appending `|| true` to the gate job's `if:` and running the P9 probe:

```
EVALGATE-OK (2 jobs, 3-clause gate, notice present) + GOVERSION-OK (3 file(s))
P9-exit-with-||-true=0
```

The same holds for a wrapping negation (`!((…) && …)`) or an injected `|| github.actor == 'x'`; P9
still reports "3-clause gate". Since P9 is the only guard on the fork/secret boundary, the guard
should not be satisfiable by text that no longer implements the boundary.

**Fix:** assert on the normalized expression instead of its pieces — parse the folded scalar, strip
whitespace, and compare against the exact expected string:

```python
expected = ("(github.event_name != 'pull_request' || "
            "github.event.pull_request.head.repo.full_name == github.repository) "
            "&& github.actor != 'dependabot[bot]'")
actual = " ".join(str(gate_if).split())
if actual != expected:
    raise SystemExit("P9 the eval-gate job's if: is %r, want exactly %r (D-09/P3)" % (actual, expected))
```

If exact equality is too brittle for future legitimate edits, at minimum require that the expression
contains no `||` at top level beyond the one inside the negated fork clause, and reject `true`/`!`
tokens.

#### WR-08: live secrets are mounted at job level, i.e. into every step including tag-pinned third-party actions

**File:** `.github/workflows/eval-gate.yml:62-78`
**Issue:** `GLM_API_KEY`, `BGE_M3_URL` and `MILVUS_ADDRESS` are declared under the job's `env:`, so
they are in the environment of every step of that job — `actions/checkout@v7`, `actions/setup-go@v7`
and `actions/upload-artifact@v7` included. Those actions are pinned by mutable major tags (per the
project's stated convention), not by SHA, and none of them needs a live-service credential. A
compromised or re-pointed `v7` tag inherits the three secrets for free. The comment on line 63
("only place that reads them; not echoed in logs") addresses log leakage, not process exposure.

**Fix:** move the three secrets to step-level `env:` on the two steps that actually need them (the
replay and the gate check), leaving the job level free of credentials:

```yaml
      - name: Run frozen compliance replay
        env:
          GLM_API_KEY: ${{ secrets.GLM_API_KEY }}
          BGE_M3_URL: ${{ secrets.BGE_M3_URL }}
          MILVUS_ADDRESS: ${{ secrets.MILVUS_ADDRESS }}
```

P9's "secrets intact" assertion (lines 750-755) matches `secrets.<NAME>` anywhere in the file, so it
keeps working unchanged.

### Info

#### IN-01: P4's sentinel-A evidence check is a bare word search

**File:** `verify-phase-48.sh:365`
**Issue:** `grep -q 'errcheck'` is satisfied by any occurrence of the word — a config diagnostic, a
deprecation warning, or the linter-name header of a different failure would satisfy "the run failed
and named errcheck". The intent is "a finding attributed to errcheck was emitted".
**Fix:** `grep -qE '\((errcheck)\)$|\(errcheck\)' "$WORK/p4-sentinel-a.log"` (or match
`Error return value .* is not checked \(errcheck\)`).

#### IN-02: P8's raw-substring rules produce false failures and have a case-sensitive hole

**File:** `verify-phase-48.sh:631, 659`
**Issue:** `if "write" in src` scans the whole file text; demonstrated that a workflow whose *comment*
reads "this workflow writes the PR comment summary" fails with the misleading message "contains a
write scope (D-26/D-29)". The registry token list is likewise raw-text and case-sensitive: the same
probe's `docker push ghcr.io/org/img:latest` slipped past `"GHCR"` (uppercase only) and past
`"registry:"`; only the genuinely-correct `packages: write` form is reliably covered (and it is
caught by the `write` rule anyway).
**Fix:** keep the raw scan as a coarse net but pair it with a parsed check
(`value in ("read", "none")`, already present) and add the lowercase/most-likely publication tokens
(`ghcr.io`, `docker push`, `push: true`) to the forbidden list — the current list cannot see a
publication capability expressed in a `uses: docker/build-push-action` step.

#### IN-03: P8 rejects the valid read-only shorthand and silently skips job-level string permissions

**File:** `verify-phase-48.sh:636-644, 648`
**Issue:** `permissions: read-all` (valid GitHub Actions syntax, and strictly read-only) is rejected
— demonstrated: `declares no permissions: mapping — an undeclared ceiling is not a read-only
ceiling`. Conversely, the job-level branch at line 648 uses `isinstance(job.get("permissions"),
dict)`, so a job-level `permissions: read-all` / `write-all` string is skipped by the per-scope loop
(the `write` raw check catches `write-all`, but the skip is silent in general).
**Fix:** normalize the string forms before checking: treat `"read-all"`/`"none"` as read-only and
`"write-all"` as a violation, then run the existing per-scope loop over the mapping form only.

#### IN-04: the FE count-literal token `"84 "` can match unrelated text

**File:** `verify-phase-48.sh:497`
**Issue:** `for literal in ("666", "689", "84 ")` — the trailing-space token matches "1984 ",
"port 8084 ", "184 cases " and similar, producing a failure whose message ("hard-codes an FE
test-count literal") misdiagnoses the cause. The other two tokens are equally substring-based but
less collision-prone.
**Fix:** anchor the assertion to how a count would actually appear, e.g. regex
`r'\b(test|tests|passed|suites)\b[^\n]*\b\d{3,}\b'`, or drop the token list in favour of asserting
that no `run:` step contains a bare 3+-digit standalone number.

#### IN-05: P2's hermeticity assertion inspects only the job-level `env:` map

**File:** `verify-phase-48.sh:212-217`
**Issue:** `extra = [k for k in env if k != "CGO_ENABLED"]` covers `jobs.go.env` only. A step-level
`env:` block adds variables invisibly to this check; the three D-16 names are caught separately by
the whole-file scan at lines 221-223, but anything else (e.g. `RETRIEVER_BACKEND: milvus`,
`JUDGE_MODEL`, `BGE_M3_MODEL` — all of which the eval path reads) could be introduced without any
partition noticing. The banner's "CGO_ENABLED=1 only" is therefore broader than the check.
**Fix:** walk `steps[*].env` too, and assert the union of keys across the whole `go` job is exactly
`{CGO_ENABLED}`.

#### IN-06: P11's registry check counts textual call sites, not executed partitions

**File:** `verify-phase-48.sh:869-872`
**Issue:** `grep -cE '^done_partition "'` proves that 11 call sites exist in the file, not that 11
partition bodies ran. Commenting out the body of a partition while leaving its `done_partition` call
satisfies both this check and the `RAN` counter, so `ALL PASSED` is still printed. The half-1b
banner check has the same shape (it proves a print site is after the gate, not that the gate can
fail).
**Fix:** derive `RAN` from observable effects rather than from call sites — e.g. have each partition
emit a marker line to a temp file that only the last statement of its body writes, and count markers
instead of source text; or assert that each `done_partition` call is the last statement of its
section between two `# P<n>:` banners.

#### IN-07: the verifier is not wired into any workflow, so its assertions can rot silently

**File:** `verify-phase-48.sh` (whole file); absence verifiable across `.github/`
**Issue:** No workflow invokes `verify-phase-48.sh` — the only in-repo references are a comment in
`.golangci.yml:27`, the ROADMAP, and the script itself. Combined with its hard requirement on
`golangci-lint` exactly `2.12.2` on `PATH` (lines 280-293), the "gate self-proof" is a manual,
maintainer-host-only step: drift between `verify-phase-48.sh` and the workflows it guards is
detected only when a human remembers to run it. The ROADMAP already records that the CI-side proof
was never obtained (`CICD-01/02/03 remain Pending; no ci.yml run exists`).
**Fix:** add a `verify` job (or a step in the `go` job) that installs the pinned linter with
`golangci/golangci-lint-action` and runs `bash verify-phase-48.sh`; the false-positive surface noted
in IN-02/IN-03 should be narrowed first so the gate does not block on cosmetic edits.

#### IN-08: the `pull_request.branches` filter leaves the same "Pending forever" hole the file declares an invariant against

**File:** `.github/workflows/ci.yml:28-32`
**Issue:** Invariant 1 forbids path filters because they leave a required check Pending on a PR that
matches no path. A base-branch filter has the identical effect on a PR whose *base* is outside
`[main, 'phase-*']` — no run is created at all, so `go`/`fe`/`docker` stay Pending if branch
protection requires them there. The widening from `main` to `phase-*` shows the hazard is
understood; any other long-lived base (release/hotfix branches) still hits it.
**Fix:** drop the `branches:` key (no filter = every base branch) or document the closed set of
supported base branches in the header next to invariant 1.

#### IN-09: the `std-error-handling` preset absorbs more than its stated rationale

**File:** `.golangci.yml:49-51` (rationale at `.golangci.yml:14-16`)
**Issue:** The comment justifies the preset as absorbing "the ~102 bare `defer X.Close()` sites and
the print noise". Measured against the same config, the preset also silences genuinely actionable
unchecked errors:

```
defer f.Close()        -> silent   (intended)
os.Remove("x.tmp")     -> silent   (not mentioned in the rationale)
fmt.Fprintf(f, "hi")   -> silent   (intended: "print noise")
io.Copy(os.Stdout, f)  -> errcheck reports it
f.Read(buf)            -> errcheck reports it
```

The exclusion is still principled (a named upstream noise class, not a hand-written allowlist), but
a reader of the comment would not expect `os.Remove` to be covered.
**Fix:** extend the comment to name the preset's real scope ("errcheck's std-error-handling preset:
deferred Close, `fmt.Fprint*`, `os.Remove` and the other std-lib cleanup calls"), or add a targeted
`rules:` entry re-enabling errcheck for the non-defer half if those sites should be reviewed.

#### IN-10: the job that executes fork-controlled code does not disable credential persistence in checkout

**File:** `.github/workflows/ci.yml:63-67`
**Issue:** `actions/checkout@v7` defaults to `persist-credentials: true`, leaving the runner token in
the local git config; the `go` job then executes `go test ./...` — arbitrary fork-authored code — and
is the one job carrying an extra scope (`pull-requests: read`). Impact is bounded (public repo,
read-only token, no push capability), but the default is unnecessary for a job that never
authenticates.
**Fix:** add `with: { persist-credentials: false }` to the `go` job's checkout step. (No write token
reaches fork code anywhere in this workflow — that part of the design holds as documented.)

---

_Reviewed: 2026-10-19T10:23:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
