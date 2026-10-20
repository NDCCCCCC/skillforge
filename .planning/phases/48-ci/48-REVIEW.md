---
phase: 48-ci
reviewed: 2026-10-20T07:02:18Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - .github/workflows/ci.yml
  - Dockerfile.backend
  - internal/duckdb/connector_test.go
  - verify-phase-48.sh
  - web/src/components/chat/parts/Sparkline.test.tsx
findings:
  critical: 0
  warning: 3
  info: 8
  total: 11
status: issues_found
---

# Phase 48: Code Review Report (gap-closure round 4)

**Reviewed:** 2026-10-20T07:02:18Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Round-4 delta reviewed at standard depth: `.github/workflows/ci.yml`, `Dockerfile.backend`,
`internal/duckdb/connector_test.go`, `verify-phase-48.sh`, and
`web/src/components/chat/parts/Sparkline.test.tsx` (diff base `d1c7d5d`).

**The three targeted fixes are correct.** I re-checked each against the code and, where possible,
against executables:

* **SC-1 / P5a SIGPIPE (`ci.yml:106-111`)** — correct. The `-list` output is now redirected to
  `$RUNNER_TEMP/go-test-list.txt` and `grep -qx` reads the file; there is no pipe left to close
  early, so the trailing `ok <pkg> <time>` line cannot EPIPE. The rename-proof guard is intact
  (`grep -qx` fails the step under GitHub's `bash -eo pipefail`, and `go test -run` with zero
  matches still exits 0). `TestDeriveEdgesDeterministic` exists at
  `internal/knowledge/graph_edges_test.go:50`.
* **Sparkline path (`Sparkline.test.tsx:77-95`)** — correct. `new NodeURL('./Sparkline.tsx',
  import.meta.url)` resolves against the test module on every platform, and importing the
  constructor from `node:url` avoids jsdom's replacement of the global `URL` (Node's `fs` accepts
  only its own `URL` instances).
* **glibc bases (`Dockerfile.backend`)** — structurally correct: the build stage is
  `golang:1.25-bookworm` (matches go.mod `go 1.25.5`), the runtime is `debian:bookworm-slim` with
  `libstdc++6`/`libgcc-s1` installed, and the source-build flags are unchanged.

**The round-4 evidence binding materially narrows round-3 CR-02 but does not close it.** Deleting
or replacing a whole probe now fails (the log is absent/empty, or the token is missing), which is
the mutation class the comment names. What remains open is *partial* neutering: a partition can
lose all of its own assertions inside a shared/multi-assertion probe while the probe's single
success token is still printed. P2/P3 are the sharpest case — they share one `STRUCT-OK` token
produced by one probe, so either partition's entire assertion set can be deleted and both still
deposit PASS. Details and mutation table in WR-03.

The two round-3 advisories still present in these files are re-raised (WR-01, WR-02), with an
additional replicated evasion shape for WR-02. The two known stale comments in this script
(P2 header, probe comment) were checked and are **not** re-raised, per the phase record.

Verification method: code reading plus execution of the P4 sentinels against the real pinned
linter (`golangci-lint 2.12.2` — sentinels B/C report `0 issues.`, D reports `revive`), and a
sandbox execution of the exact P8 rule logic against four fixtures. `internal/duckdb` could not be
compiled on this host (no C toolchain for CGO on Windows); its assessment is code review plus the
recorded WSL run.

## Warnings

### WR-01: The P11 else-detector still uses the SIGPIPE-prone `sed | grep -q` idiom and can silently stop detecting

**File:** `verify-phase-48.sh:1409-1411`
**Issue:** (Round-3 WR-01, still open — the round-4 delta did not touch this line.)

```bash
if sed -n "$((GATE_LINE + 1)),${BANNER_LINE}p" "$SELF" | grep -qE '^[[:space:]]*else[[:space:]]*$'; then
  fail "P11 the passing banner sits after an else: it is no longer inside the RAN-gated branch"
fi
```

Under the file's `set -uo pipefail`, `grep -q` closes the pipe at the first match. If the printed
range ever exceeds the pipe buffer, `sed` takes EPIPE and the pipeline status becomes 141 — read
as "no `else` found" even when one was printed. Today the range is one line (gate `:1596`, banner
`:1597`), so the defect is latent; it becomes live exactly in the scenario the check exists for
(a banner moved further down), at which point the mutation is silently accepted.
**Fix:** consume the whole range before grepping:

```bash
BANNER_RANGE="$(sed -n "$((GATE_LINE + 1)),${BANNER_LINE}p" "$SELF")"
if printf '%s\n' "$BANNER_RANGE" | grep -E '^[[:space:]]*else[[:space:]]*$' >/dev/null; then
```

### WR-02: The trust-surface path-filter and secret rules are still text scans and accept four valid-YAML evasion shapes

**File:** `verify-phase-48.sh:1003` (`SECRET_REF`), `:1032` (`nocomment` strip), `:1114-1119`
(path filters), `:1122-1131` (secret walk)
**Issue:** (Round-3 WR-02, still open.) Both rules run over
`nocomment = re.sub(r"(?m)#.*$", "", src)`, a textual comment strip, not the parsed document. I
executed the exact rule logic from the shipped function against four fixtures; all four returned
**ACCEPT**:

| Fixture | Rule it defeats | Result |
|---|---|---|
| `on: pull_request: "paths": ["internal/**"]` | T-48-03 path filter | accepted |
| `on: pull_request: paths : ["internal/**"]` | T-48-03 path filter | accepted |
| `run: echo "${{ toJSON(secrets) }}"` | T-48-04 secret reference | accepted |
| `run: "echo x# ${{ secrets.DEPLOY_KEY }}"` | T-48-04 secret reference | accepted (stripped as a "comment") |

`toJSON(secrets)` expands the entire secrets context, which is the direct T-48-04 hazard; the
quoted/space-before-colon path keys are valid YAML that GitHub honours, so the "required check
sits Pending forever" class the rule exists to prevent is still reachable. The permissions walk
(`:1075-1109`) is the one rule already driven from the parsed `doc` and is not affected.
**Fix:** make the parsed document primary. For path filters, walk
`doc.get("on", doc.get(True))` for `paths` / `paths-ignore` keys on every event. For secrets,
recursively walk every string value in the document for `secrets.`, `secrets[`, and
`toJSON(secrets` (and `toJSON( secrets`), then keep the textual scan only as a second layer that
does not strip `#` from quoted strings.

### WR-03: Success tokens are per-probe, not per-assertion — a partition can be fully neutered inside a shared probe and still deposit PASS (residual of round-3 CR-02)

**File:** `verify-phase-48.sh:268-474` (P2/P3 shared probe and tokens), `:190-256` (P1),
`:558-691` (P4), `:754-833` (P6), `:841-914` (P7), `:1166-1317` (P9)
**Issue:** `ok_partition` (`:125-142`) now refuses when the log is missing, empty, or lacks the
token, which correctly kills the "delete the probe invocation, keep the bookkeeping" mutation.
But the token is one per *probe*, and several partitions have many assertions behind one token:

* **P2 and P3 share `STRUCT-OK`** (`:465`, `:473-474`). Deleting the whole P2 command-fragment
  block (`:298-334`) or the whole P3 lint/permissions/verify block (`:366-463`) leaves
  `print("STRUCT-OK")` intact, so **both** partitions still deposit PASS. This is the cleanest
  reproduction of the residual hole: two named gates, one evidence token.
* **P1** (`:190-256`): the token count proves every workflow file was parsed, but deleting the
  composite parse (`:230-236`) or the `pull_request_target` scan (`:241-252`) leaves the
  `P1-PARSED` tokens intact.
* **P4** (`:558-689`): the deposited evidence is sentinel D's log only. Deleting sentinel A, B, or
  C (and their `fail` paths) leaves `revive` in `p4-sentinel-d.log`, so P4 still passes.
* **P6/P7/P9**: each probe ends with a single `print("...-OK")`; deleting any individual
  assertion block (e.g. the new job-level `if:` checks at `:772-789` / `:858-872`) still leaves
  the success print.

The header comment (`:114-122`) claims "a neutralised or deleted probe cannot deposit PASS". That
is true for whole-probe replacement, not for the assertion-level mutation it was introduced to
close. Severity is warning rather than critical because the round-4 mechanism did remove the
whole-probe case and the P11 registry still catches deleted tokens.
**Fix:** emit one distinct token per assertion group and require each at deposit time, e.g.
`print("P2-CMDS-OK")` at the end of the P2 section and `print("P3-LINT-OK")` at the end of the P3
section, then `ok_partition "P2" "$WORK/p23.log" "P2-CMDS-OK"` and
`ok_partition "P3" "$WORK/p23.log" "P3-LINT-OK"`. Apply the same split to P1
(`P1-PARSED`, `P1-COMPOSITE-OK`, `P1-NO-PRT`), P4 (one token per sentinel), and the single-print
probes in P6/P7/P9. `ok_partition` can accept a list of required tokens to keep the interface
small.

## Info

### IN-01: The linter install's integrity settings are inherited from the runner image, not declared (round-3 IN-01, still open)

**File:** `.github/workflows/ci.yml:186-195` (the claim), `:230-236` (the step)
**Issue:** The file states "no network fetch-and-execute surface remains anywhere in this file",
but `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` inherits
`GOTOOLCHAIN`, `GOSUMDB`, `GOPROXY`, and `GOFLAGS` from the runner image. With `GOTOOLCHAIN=auto`
(the default), a module whose `go` directive exceeds the local toolchain causes `go install` to
download and run another toolchain. Today it is safe (golangci-lint requires `go 1.25.0`, go.mod
pins `1.25.5`), but the guarantee is not declared in this repo.
**Fix:** add `env: { GOTOOLCHAIN: local, GOSUMDB: sum.golang.org, GOPROXY: "https://proxy.golang.org,direct" }`
to that step (and soften the comment if the inherited surface is left in place).

### IN-02: `ok_partition` dereferences `$1` before its arity check and labels every failure as "P11"

**File:** `verify-phase-48.sh:125-141`
**Issue:** `local pn="$1"` executes before `if [ "$#" -ne 3 ]`. A zero-argument call under `set -u`
aborts with bash's "unbound variable" instead of the intended diagnostic (still fail-closed, but
the message loses the reason). Every failure message also hard-codes "P11 partition ${pn}", so a
P2 deposit failure reads as a P11 error and sends the reader to the wrong partition.
**Fix:** check `$#` first, then assign; change the prefix to the generic `partition ${pn}` (or
`P11: partition ${pn}` only for the self-probe).

### IN-03: P1's `pull_request_target` scan still matches comment text (round-3 IN-03, still open)

**File:** `verify-phase-48.sh:241-252`
**Issue:** The round-4 change correctly makes the scan's exit code decisive (`rc=0` → fail,
`rc=1` → pass, anything else → fail). But `grep -rn 'pull_request_target'` still reads raw source,
so a workflow comment such as "we never use pull_request_target" reds P1 — while P8 strips
comments for the same class of concern (`:1032`). This is a false-positive risk, not a security
hole (the opposite direction is the dangerous one).
**Fix:** scan `nocomment` text (or the parsed document) for the event name, keeping the
exit-code check.

### IN-04: `Dockerfile.backend` calls its base images "pinned" while every reference is a mutable tag, and the image ships as root

**File:** `Dockerfile.backend:17` (claim), `:20` (`FROM golang:1.25-bookworm`), `:40`
(`FROM debian:bookworm-slim`), `:47` (unpinned `apt-get install`)
**Issue:** The comment says "All base images are pinned to specific tags (T-7-OSS-CONTAINER
mitigation)", but `golang:1.25-bookworm` and `debian:bookworm-slim` are floating tags — `ci.yml`
itself calls `golang:1.25-bookworm` "a floating tag" at `:153`. Builds are therefore not
reproducible and a compromised/updated upstream tag silently changes the image. There is also no
`USER` directive (the backend runs as root) and no `HEALTHCHECK`.
**Fix:** pin by digest (`FROM golang:1.25-bookworm@sha256:...`), pin apt package versions or accept
and document the drift, add a non-root `USER`, and add a `HEALTHCHECK` (or drop the "pinned"
wording).

### IN-05: The runtime half of the glibc migration is not exercised anywhere in CI

**File:** `.github/workflows/ci.yml:150-182`; `Dockerfile.backend:12-16`, `:40-47`
**Issue:** The round-4 base-image change was motivated by a runtime link failure, but the `docker`
job only builds (`push: false`), P7 explicitly forbids `load: true` (`verify-phase-48.sh:890-892`),
and no step starts either image. A runtime regression — e.g. dropping `libstdc++6`, or a future
move to a non-glibc runtime — keeps the `docker` job green while `docker compose up` fails exactly
as before. This is a coverage gap, not a defect in the change; closing it requires revisiting
D-24 ("CI never publishes an image and never produces a local one").
**Fix (requires a D-24 amendment):** load the backend image and run a short startup smoke
(`docker run --rm -e SERVER_ADDR=:9999 <tag>` under a timeout, failing if the container exits
non-zero with a loader error), or add a link-check step (`ldd /app/skillforge | grep "not found"`)
in the runtime stage.

### IN-06: The best-of-5 ratio is not a controlled comparison, and the env override accepts malformed values

**File:** `internal/duckdb/connector_test.go:201-224`, `:226-327`
**Issue:** The sampling logic is correct as written (min of 5 ratios; a genuine block inflates all
samples) and the extraction into `measureWriteWithConcurrentReaders` is clean. Two caveats remain:
(a) within each attempt the baseline is always measured first and against a table of 100k rows,
while the concurrent write runs second against 600k rows, so the two samples are not measured
under the same conditions — a systematic warm-up effect would bias the ratio downward (toward
passing), which is the unsafe direction; (b) `fmt.Sscanf(v, "%f", &parsed)` accepts trailing
garbage (`"3.0x"` → 3.0) and negative or zero values, which silently turn the assertion into an
always-fail. The 5× row volume also makes this the slowest test in the package.
**Fix:** measure a second baseline after the concurrent phase in each attempt and use
`max(baselineBefore, baselineAfter)` (conservative, cheap); validate the override with
`strconv.ParseFloat` and reject `<= 0`.

### IN-07: The push-to-main trigger's `only-new-issues` reasoning does not follow, and the push path appears unexercised

**File:** `.github/workflows/ci.yml:41-45` (trigger/comment), `:87-91` (lint step)
**Issue:** The comment says push runs are limited to `main` because "the lint action has no diff
on a manual/push run and would fall back to whole-repo lint over ~175 legacy findings." Limiting
the push trigger to `main` does not prevent that fallback on a `main` push — under the comment's
own premise, the first merge to `main` would red the lint step. Either the comment is stale (the
action does derive a diff for push events) or the trigger choice does not provide the mitigation
it claims. All green evidence in the phase record is from `pull_request` runs, so this path is
unproven.
**Fix:** verify one push-to-main run (or `workflow_dispatch` with the same job) and correct the
comment; if the fallback is real, gate the lint step with a job-level `if:` (allowed at job level
for this purpose only) or pass an explicit `--new-from-rev` base.

### IN-08: `TestSparklineNoDeps` only inspects the component's own source text

**File:** `web/src/components/chat/parts/Sparkline.test.tsx:77-95`
**Issue:** The path fix is correct and portable. The assertion, however, greps the raw text of
`Sparkline.tsx` for three library names: a transitive import (e.g. the component importing a local
wrapper that imports `recharts`), a dynamic import, or a bundler alias would all pass, and a
matching library name in a comment would fail. It is a heuristic guard, not a dependency check.
**Fix:** assert on the resolved module graph (e.g. `import.meta.glob`/Vite's module list) or on a
built bundle, or rename the test to reflect that it only forbids direct textual imports.

---

_Reviewed: 2026-10-20T07:02:18Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
