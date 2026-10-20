---
phase: 48-ci
reviewed: 2026-10-20T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - verify-phase-48.sh
findings:
  critical: 2
  warning: 3
  info: 5
  total: 10
status: issues_found
---

# Phase 48: Code Review Report (gap-closure round 3)

**Reviewed:** 2026-10-20
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Three files reviewed: `.github/workflows/ci.yml`, `.github/workflows/eval-gate.yml`,
`verify-phase-48.sh`. The phase's own gate (`bash verify-phase-48.sh`, 11 partitions, exit 0)
was executed locally as a baseline, and the script was then **mutation-tested** against its own
claims. The linter bootstrap was re-verified against the Go module proxy metadata.

**Headline: the tracked M1c red has a root cause inside `ci.yml`, and it is deterministic — not a
runner-environment flake.** `ci.yml:105` runs

```
go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -qx 'TestDeriveEdgesDeterministic'
```

under `set -o pipefail`. `grep -q` closes the pipe at the first match; the `go` command's
subsequent write (the `ok <pkg> <time>` line that `go test -list` always prints after the list)
gets `EPIPE` → the process dies of SIGPIPE → exit 141 → `pipefail` propagates 141 → the step dies.
Reproduced on Linux with go1.25.5 (the exact version the runner installs from `go.mod`): **exit
141, 0-byte step log** — the identical fingerprint recorded for all four runner runs
(`48-15-SUMMARY.md:209`: "exit 141 (SIGPIPE) … Captured step log: empty … ≈0.78 s"). The
`grep`-without-`-q` variant and the "capture to a variable" variant both exit 0. **The `go` job
can never be green with this step as written**, and the phase's disposition of M1c ("runner
environment … not a code-side bug", `48-VALIDATION.md` M1c) is wrong; the runner-side remedies it
proposes would not help. This is *not* a restatement of the tracked red — it is the cause, and the
cause is in one of the three in-scope files.

Second headline: **the gate's partition accounting does not establish that a probe ran.** The
`ok_partition` success token is deposited by the partition body *at a hand-chosen line*, so
deleting or neutralising a probe while leaving the token in place still prints
`=== Phase 48 Gate: ALL PASSED ===` with exit 0. Four such mutations were run and all four
survived (P9, P6, P5, P2/P3 probe neutered; plus P4's decisive `fail` downgraded to `info`).
Only P8 has a mechanical backstop (the unbound `TRUST_SURFACE_COUNT`, `verify-phase-48.sh:1064`),
and the recorded M8 drill covered only P8.

Answering the three focus questions directly:

* **Linter bootstrap (R2-CR-01)** — the supply-chain surface it replaced is materially gone:
  `go install …@v2.12.2` is checksum-DB verified, needs no fetched executable, and the failure
  surfaces as a **step failure, not a swallowed one** (`bash -e` aborts on a failed `go install`;
  the `[ -x … ]` postcondition catches a `GOBIN`-mismatch install with a loud `FATAL`). All four
  runner runs executed this exact shape and the `verify` job was green in every one. One residual:
  `GOTOOLCHAIN`/`GOSUMDB`/`GOPROXY` are inherited from the runner image rather than declared, so
  the file's "no network fetch-and-execute surface remains anywhere in this file" claim is
  overstated (IN-01).
* **Secret scoping in `eval-gate.yml`** — correct. The three live secrets are step-scoped to the
  only consumer, and `cmd/eval --gate-check` genuinely short-circuits before any bootstrap
  (`cmd/eval/main.go:463-466`, `runGateCheck` at `main.go:186-213` is a pure file comparison), so
  the `Gate check` step needs no credential. **No secret is reachable by a step that does not
  consume it.** One related gap: the job's checkout still persists the runner token (WR-03).
* **Partition accounting** — yes, a probe can be deleted or neutralised while the gate prints
  PASS (CR-02).
* **Shell correctness** — one *live* instance of the pipefail/`grep -q` SIGPIPE class (CR-01) and
  one *latent* instance in the gate's own else-detector (WR-01).

## Critical Issues

### CR-01: The P5a determinism guard SIGPIPEs deterministically on every Linux runner — the root cause of the tracked M1c red lives in `ci.yml`

**File:** `.github/workflows/ci.yml:103-106`
**Issue:** The step added by this phase (commit `05803d5`, "rename-proof P5a") is:

```yaml
      - name: determinism -count=100 (P5a)
        shell: bash
        run: |
          set -o pipefail
          go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -qx 'TestDeriveEdgesDeterministic'
          go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100
```

`grep -q` exits at the first match and closes the pipe's read end. `go test -list` always prints
the test list **and then** a second line `ok <pkg> <time>`, so the `go` command performs a second
write into a pipe whose reader is gone → `EPIPE` → SIGPIPE → the `go` command exits 141 →
`pipefail` makes the pipeline's status 141 → `bash -e` aborts the step. The step log is empty
because `grep -q` prints nothing and the `go` command's output was lost to the broken pipe.

Evidence (WSL Ubuntu 22.04, go1.25.5 linux/amd64 — the same Go version `go-version-file: go.mod`
installs on the runner):

```
--- current ci.yml P5a line 1 under pipefail (real package) ---
exit=141  log_bytes=0
--- with the proposed fix (grep without -q) ---
exit=0  log_bytes=0
--- P5a second command: go test -run ... -count=100 ---
ok  	github.com/skillforge/skillforge/internal/knowledge	0.170s
exit=0
```

6/6 iterations of a minimal reproduction (`go test -list` + `grep -qx` + `pipefail`) returned 141.
The fingerprint matches the runner exactly: **exit 141, empty log, ~0.78 s** (the second command
never ran — it would have printed an `ok` line). The guard was present in all four runs
(`git merge-base --is-ancestor 05803d5 <run-head>` is true for `d168512`, `62b6663`, `a2a4c86`,
`304dcc4`), so this single line explains the red in every one of them.

Consequences:
1. The `go` job is **permanently red** → the phase's CICD-01 conclusion is unreachable.
2. `48-VALIDATION.md` M1c and `48-15-SUMMARY.md:209` classify this as "runner environment …
   not a code-side bug" and propose runner-side remedies (capture step output, abort early on
   output-buffer close, add `-v`). None of those address the cause; the red is 100 % reproducible
   in 0.2 s on any Linux host.
3. The gate cannot catch it: the P2 probe (`verify-phase-48.sh:271-284`) only greps the *text* of
   the step for `"-list '^TestDeriveEdgesDeterministic$'"` and `"set -o pipefail"`. It never
   executes the pipeline, so a step that is textually correct and behaviourally guaranteed to fail
   passes the gate. The same applies to the "48-07 rule" probes — they are text checks.

**Fix:** remove the early pipe close. Either variant is verified exit 0:

```yaml
        run: |
          set -o pipefail
          go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -x 'TestDeriveEdgesDeterministic' >/dev/null
          go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100
```

or, with no pipe at all (most robust — nothing can SIGPIPE a file):

```yaml
        run: |
          set -o pipefail
          go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' > "$RUNNER_TEMP/testlist.txt"
          grep -qx 'TestDeriveEdgesDeterministic' "$RUNNER_TEMP/testlist.txt"
          go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100
```

Note `grep -x` without `-q` reads the whole input, so the producer never sees a closed pipe, and
its exit status is still 0-on-match / 1-on-no-match, so the "renamed test must red the gate"
intent is preserved. The P2 probe's textual assertions still pass with either form (the
`-list '…'` fragment and `set -o pipefail` both remain), so no gate change is required for the fix
itself — but the P2 probe should be extended with a *behavioural* assertion so this class is
caught in future (see CR-02).

### CR-02: The success-token accounting does not prove a probe ran — a neutered partition still prints `ALL PASSED`

**File:** `verify-phase-48.sh:103-136` (the mechanism), and every partition body it is meant to
protect (`:214`, `:423-424`, `:525-624`, `:637-673`, `:688-749`, `:764-820`, `:882-1047`,
`:1079-1223`)
**Issue:** The header (`:103-115`) states the design goal: "A `done_partition` call site is not
evidence that its partition body ran … every probe deposits a success token — `ok_partition` —
immediately after its own decisive exit-code assertion, and `done_partition` refuses without
begin AND ok. The deposit never precedes the assertion it certifies."

The token is deposited by a **hand-placed line in the partition body**, not derived from the
probe's outcome. `done_partition` therefore proves only that a line reading `ok_partition "P9"`
was reached — not that P9's probe ran, and not that P9's assertion fired. The M8 mutation class
("delete the probe, keep the bookkeeping") is closed only where a probe-owned *variable* happens
to be interpolated afterwards: P8 (`TRUST_SURFACE_COUNT`, `:1064-1068`) and, incidentally, P1
(`WF_COUNT`). The recorded drill (48-13 Task 1) exercised **P8 only**.

Mutation tests run against the shipped script (each mutation applied to a copy, the copy
executed, the copy deleted):

| Mutation | Result |
|---|---|
| M-A: P9 probe invocation neutered (`python … <<'PY'` → `: <<'PY'`) | **exit 0, `ALL PASSED` printed** |
| M-C: P6 probe invocation neutered | **exit 0, `ALL PASSED` printed** |
| M-F: P5 probe invocation neutered | **exit 0, `ALL PASSED` printed** |
| M-G: P2/P3 probe invocation neutered | **exit 0, `ALL PASSED` printed** |
| M-E: P4's decisive `fail` (sentinel A) downgraded to `info` | **exit 0, `ALL PASSED` printed** |
| M-B: P8's `scan_trust_surface` call deleted (the recorded M8 drill) | exit 1 — caught by the unbound `TRUST_SURFACE_COUNT` |
| M-D: P9's `ok_partition` token deleted | exit 1 — caught by the P11 registry count |

So: deleting the *token* is caught; deleting or neutering the *probe* is not, for 8 of the 11
partitions. Because the neutered probe's log file is then empty, the partition prints its normal
`PASS P9 eval-gate-gate (…)` line with no evidence behind it — the exact "found nothing, reported
green" class this phase exists to close (Phase 47 HIGH-1), reproduced inside the phase's own gate.

This matters beyond the script itself: the `verify` job in `ci.yml` runs this script, and the
script's purpose is to be the evidence that the CI gate "will really run and really bite"
(header `:4-6`). A green `verify` check currently does not carry that meaning for P2/P3, P4, P5,
P6, P7, P9, P10, P11.

**Fix:** make the token's deposit a function of the probe's exit status, so the bookkeeping cannot
outlive the evidence. A minimal, mechanical shape:

```bash
# probe <partition-name> <command...> — runs the probe, deposits the token only on rc 0,
# and never lets a failure be swallowed.
probe() {
  local pn="$1"; shift
  local log="$WORK/${pn}.log"
  "$@" > "$log" 2>&1
  local rc=$?
  [ "$rc" -eq 0 ] || { cat "$log"; fail "$pn probe failed (rc=$rc)"; }
  ok_partition "$pn"
}
```

and route every partition through it (the P2/P3 shared probe already has a single decisive
assertion at `:419`; P4's four sentinels each have one at `:527/:533/:559/:584/:616/:619`; P5/P6/P7/P9
each have one at `:676/:752/:823/:1226`). The additional belt-and-braces that P8 already has —
a probe-owned variable interpolated into the `done_partition` message — is worth adding to the
other partitions, because it converts "the probe was deleted" from a silent green into an
unbound-variable abort. Concretely: have each probe set a partition-owned variable
(`P9_EVIDENCE="…"`) and interpolate it in the `done_partition` message, exactly as P8 does with
`TRUST_SURFACE_COUNT`.

## Warnings

### WR-01: The P11 else-detector uses the same SIGPIPE-prone idiom and can silently stop detecting

**File:** `verify-phase-48.sh:1312`
**Issue:**

```bash
if sed -n "$((GATE_LINE + 1)),${BANNER_LINE}p" "$SELF" | grep -qE '^[[:space:]]*else[[:space:]]*$'; then
  fail "P11 the passing banner sits after an else: it is no longer inside the RAN-gated branch"
fi
```

Under the file's `set -uo pipefail`, `grep -q` closes the pipe early; if the printed range is
larger than the pipe buffer, `sed` is SIGPIPE'd and the pipeline's status becomes 141 — a
*non-match* result, even though the `else` was found. Demonstrated on Linux:

```
--- A: sed | grep -q  (current P11 Half-1 shape), LARGE range ---
  exit=141  (0 = else-detector FIRED)
--- B: sed | grep (no -q), LARGE range ---
  exit=0    (0 = else-detector FIRED)
--- C: sed | grep -q, 1-line range (shape live today) ---
  exit=1    (1 = correctly not found)
```

Today the range is one line (gate at 1490, banner at 1491), so the defect is latent. It becomes
active precisely in the scenario the check exists for: the range grows when the banner is moved
down, which is the mutation the check is meant to catch — at which point the check silently
reports "no else found" and the mutation is not caught.

**Fix:** drop `-q` so grep consumes the whole range, or materialise the range first:

```bash
BANNER_RANGE="$(sed -n "$((GATE_LINE + 1)),${BANNER_LINE}p" "$SELF")"
if printf '%s\n' "$BANNER_RANGE" | grep -E '^[[:space:]]*else[[:space:]]*$' >/dev/null; then
```

### WR-02: The trust-surface secret and path-filter rules are regex-scanned over text and miss three evasion shapes

**File:** `verify-phase-48.sh:940-1044` (specifically `:945` comment stripping, `:1027-1032` path
filter, `:1036-1044` secret reference)
**Issue:** The P8 rules run over `nocomment = re.sub(r"(?m)#.*$", "", src)` — a textual strip, not
a YAML-aware one — and the path-filter rule is a plain substring test (`"paths:" in nocomment`).
The already-parsed `doc` is loaded at `:946` but is not used for either rule. Three shapes are
therefore accepted by the partition (each fixture run through the real `scan_trust_surface`
function, extracted from the shipped script, against a temp directory):

| Fixture | `scan_trust_surface` result |
|---|---|
| `on: pull_request: "paths": ["internal/**"]` (quoted key) | **rc=0 — TRUST-SURFACE-OK** |
| `run: echo "${{ toJSON(secrets) }}"` | **rc=0 — TRUST-SURFACE-OK** |
| `run: "audit note # ${{ secrets.DEPLOY_KEY }}"` | **rc=0 — TRUST-SURFACE-OK** |

Concretely:
* `toJSON(secrets)` expands the whole `secrets` context — i.e. **every repository secret** — and
  the `SECRET_REF` regex (`secrets\.([A-Za-z_][A-Za-z0-9_]*)`) requires a dot, so the bare context
  is not matched. This is the T-48-04 hazard in its most direct form.
* A path filter written with a quoted key (`"paths":`) or with a space before the colon
  (`paths :`) is valid YAML and is honoured by GitHub, but neither contains the literal `paths:`,
  so the T-48-03 "required check sits Pending forever" rule does not fire.
* A secret reference after a `#` **inside a quoted string** is stripped by the textual
  comment-stripper, even though it is not a comment.

(A quoted write scope — `"packages": write` — is *not* a gap: it is caught by the structural
permissions walk at `:988-993` / `:1012-1017`, which reads the parsed mapping. That is the
strength the two rules above lack.)

**Fix:** drive both rules from the parsed document instead of the raw text. The secret rule
should walk the parsed structure for any string containing a `secrets` context reference
(`secrets.`, `toJSON(secrets`, `secrets[`) rather than regexing the source; the path-filter rule
should inspect `doc["on"]` (and `doc[True]` under PyYAML 1.1) for `paths` / `paths-ignore` keys
on any event. The textual scan can stay as a belt-and-braces second layer, but the parsed check
must be the primary one, and the `nocomment` strip must not be used to justify skipping strings.

### WR-03: `eval-gate.yml` persists the runner token in the one job that holds the three live secrets

**File:** `.github/workflows/eval-gate.yml:83-84`
**Issue:**

```yaml
      - uses: actions/checkout@v7
```

No `persist-credentials: false`. `actions/checkout` defaults to persisting the job token into
the repository's local git config (`.git/config` `http.https://github.com/.extraheader`), so the
token is readable by anything the job executes for the whole job lifetime — and this job
(a) builds `./bin/eval` from the checked-out tree and (b) runs it in the step that has
`GLM_API_KEY`, `BGE_M3_URL` and `MILVUS_ADDRESS` in its environment.

The phase's own IN-10 rule is applied in `ci.yml` for both jobs that execute checked-out content
(`ci.yml:74-76` for `go`, `ci.yml:216-218` for `verify`), but neither P8 nor P9 checks it, so
`eval-gate.yml` — the only workflow that holds live secrets — is the one workflow where the rule
is silently absent. Impact is bounded (same-repo PRs and pushes only, `contents: read`), but the
inconsistency is invisible to the gate: a reviewer reading P8/P9 would reasonably conclude the
token is never persisted anywhere.

**Fix:**

```yaml
      - uses: actions/checkout@v7
        with:
          persist-credentials: false
```

and extend the P8 or P9 probe to assert that every workflow in `.github/workflows/` that
executes checked-out content uses `persist-credentials: false`, so the rule is enforced rather
than documented.

## Info

### IN-01: The linter install's integrity guarantees are inherited from the runner image, not declared — the "no network fetch-and-execute surface" comment is overstated

**File:** `.github/workflows/ci.yml:186-189` (the claim), `.github/workflows/ci.yml:225-231` (the step)
**Issue:** The R2-CR-01 fix is a real improvement and I could not find a fetch-and-execute
surface in it: `go install …@v2.12.2` resolves through the module proxy, verifies every module
against the checksum database, compiles from source (no fetched executable is run), and the
failure path is loud — `bash -e` aborts the step on a failed `go install`, and the
`[ -x "$LINT_BIN/golangci-lint" ] || { …; exit 1; }` postcondition catches the `GOBIN`-mismatch
case with a `FATAL` message rather than a silent skip. All four runner runs used this exact shape
and the `verify` job was green in every one, so the path is empirically proven on a real runner.

Two residual gaps, both currently latent:
1. `GOTOOLCHAIN` is left at its `auto` default. If the target module's `go` directive ever
   exceeds the local toolchain's version, `go install` will **download a different Go toolchain**
   from the module proxy and run it — a network fetch-and-execute surface that the file's comment
   ("no network fetch-and-execute surface remains anywhere in this file") does not account for.
   It is safe to close today: golangci-lint v2.12.2's `go` directive is `1.25.0` and the repo
   pins `go 1.25.5` in `go.mod`, so `GOTOOLCHAIN=local` is satisfied without a download.
2. `GOSUMDB` / `GOPROXY` / `GOFLAGS` are inherited from the runner image rather than declared, so
   the checksum guarantee rests on an image default nobody in this repo controls.

**Fix:** declare the guarantees on the step:

```yaml
      - name: install pinned golangci-lint
        env:
          GOTOOLCHAIN: local
          GOFLAGS: -mod=mod
          GOSUMDB: sum.golang.org
          GOPROXY: https://proxy.golang.org,direct
        run: |
          ...
```

### IN-02: The pinned linter version `v2.12.2` is duplicated in four places with no single source of truth

**File:** `.github/workflows/ci.yml:90`, `.github/workflows/ci.yml:229`,
`.github/workflows/verify-phase-48.sh:140`, `.github/workflows/verify-phase-48.sh:404`
**Issue:** `v2.12.2` appears in the `go` job's lint-action `version:`, in the `verify` job's
`go install` line, as a literal inside the P3 probe's expected string, and as `LINT_VERSION` in
the script. A partial bump fails loudly rather than silently (the runtime preflight at `:446-458`
refuses any binary whose `--version` is not `LINT_VERSION`, and P3 compares the install line
against its own literal), so this is not a correctness hole — but a bump requires four coordinated
edits across three files, and the failure mode is a confusing red rather than a pointed message.
Consider having the P3 probe derive its expected version from a single literal in the script (or
from the `go` job's action `version:`) so there is one place to change.

### IN-03: P1's `pull_request_target` scan matches comment text, unlike the trust-surface rules

**File:** `verify-phase-48.sh:209-212`
**Issue:** `grep -rn 'pull_request_target' "$WF_DIR"` runs over raw source, so a workflow that
merely *documents* "we do not use pull_request_target" in a comment reds the gate — while the
P8 rules for the same class of concern explicitly strip comments first (`:945`). Applying the
same comment-stripping (or restricting the match to non-comment lines) would remove the
false-positive risk without weakening the check.

### IN-04: `scan_trust_surface` reuses one log path for P8 and the P11 fixtures

**File:** `verify-phase-48.sh:851`, `:1057-1060`, `:1445-1460`
**Issue:** The function always writes to `$WORK/trust-surface.log`, so the eight P11 fixture runs
overwrite P8's evidence file. It is harmless today (P8's log is printed at `:1059`, before the
fixture loop), but a partition that later wants to re-inspect P8's evidence would find the
fixture output instead. Parameterising the log path per caller would remove the coupling.

### IN-05: The `verify` check is self-attested on fork PRs (already documented — noted for branch-protection purposes)

**File:** `.github/workflows/ci.yml:199-203`
**Issue:** On a `pull_request` event the workflow and the script both come from the PR's merge
ref, so a fork PR can edit `verify-phase-48.sh` to `exit 0` and the `verify` check still reports
green. The file's header states this plainly, so it is a documented limitation rather than a
defect — but it means the branch-protection rule on `verify` carries no assurance for fork PRs
and should not be counted as one when the fork boundary is discussed.

## Scope notes

* M1a (`fe` Sparkline hardcoded Windows path) and M1b (`docker` alpine-musl vs duckdb) are
  source defects in files outside this review's scope and are **not** restated here. M1c is
  reported as **CR-01** because its root cause is in `ci.yml`, an in-scope file — the review was
  asked to report contributing causes, and the cause is deterministic and reproducible.
* No other file was modified by this review; the mutant copies used for the mutation drills were
  written to the repository root, executed, and deleted. `git status` shows no new tracked or
  untracked artefacts from this review.
* `bash verify-phase-48.sh` was executed on the review host as a baseline: **11/11 partitions,
  exit 0** — i.e. the shipped gate does pass on a clean tree, and the findings above are about
  what that pass is and is not evidence of.

---

_Reviewed: 2026-10-20_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
