---
phase: 48-ci
reviewed: 2026-10-19T15:44:23Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - .golangci.yml
  - verify-phase-48.sh
findings:
  critical: 1
  warning: 5
  info: 3
  total: 9
status: issues_found
---

# Phase 48: Code Review Report (incremental re-review)

**Reviewed:** 2026-10-19T15:44:23Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Incremental re-review of the hardened state produced by gap-closure plans 48-05..48-11 and
dispositioned in `48-REVIEW-DISPOSITION.md` (18 fixed, 1 deferred). All previously recorded
assertions were re-verified against the current files and still hold: the P9 normalized
gate/notice equality replays true against the actual YAML folded scalars; the P6 count-phrase
regex matches nothing in ci.yml; the P10 ROADMAP wording is old-absent/new-present; the script
has 11 `done_partition` call sites and 11 `begin_partition` sites; `.golangci.yml` is free of
exemption lists; the deferred WR-06 (push-path lint scope) is unchanged and not re-raised.

The 9 findings below are new defects introduced or left by the hardening itself. They cluster on
two seams. First, **48-11's `verify` job**: the remote linter install is `curl | sh` from a
mutable `HEAD` ref with no checksum verification, its install pipeline drops curl's exit status
(the exact WR-03 class fixed elsewhere in this same phase), and — most consequentially — the new
job is invisible to every guard the phase built: no partition pins its existence or rejects
`if:` conditions on it, so the 48-07/48-09 hardening does not cover the job that 48-11 added.
Second, **the begin-marker accounting (IN-06's fix)**: markers prove a body *started*, not that
its probe *ran*, and P8's summary line contains a `:-0` default that silently absorbs a deleted
probe. One disposition contradiction was found: WR-08's fix grants the live secrets to a
`Gate check` step that `cmd/eval` documents as offline-by-design.

The `threat_flag: supply-chain` that `48-11-SUMMARY.md` recorded against the `curl | sh` line —
routed to "48-10's security pass" — never landed in `48-SECURITY.md` (zero matches for
supply-chain / curl / install.sh / raw.githubusercontent / fetch / network). The surface is
therefore undispositioned and is raised here as CR-01.

## Critical Issues

### CR-01: Gate job executes `curl | sh` from a mutable HEAD ref with no checksum verification — and 48-11's own supply-chain threat flag for this line was dropped from the security record

**File:** `.github/workflows/ci.yml:199-204` (line 203); related: `verify-phase-48.sh:121`, `.planning/phases/48-ci/48-11-SUMMARY.md` (verification table, `threat_flag: supply-chain`), `.planning/phases/48-ci/48-SECURITY.md` (no trace)

**Issue:** The `verify` job bootstraps its linter with:

```yaml
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$LINT_BIN" v2.12.2
```

Three problems compound:

1. **Mutable ref.** `HEAD` resolves at run time to whatever `golangci/golangci-lint`'s default
   branch contains. The `v2.12.2` argument pins the *binary artifact*, but pins neither the
   *bootstrap script* (executed verbatim) nor the downloaded tarball (install.sh does not verify
   the release's published SHA256 checksums). Any compromise or hostile change upstream becomes
   arbitrary code execution in this job on every push and PR.
2. **Self-undermining placement.** This is the job whose sole purpose is proving the CI gate
   cannot silently rot. Unverified remote code running inside it can forge the green `verify`
   check the phase intends to publish for branch protection (D-21), and can read the runner
   token, which this job persists into the checkout (no `persist-credentials: false`, unlike the
   `go` job).
3. **Unrecorded.** `48-11-SUMMARY.md` flagged exactly this line (`threat_flag: supply-chain`)
   and routed it to 48-10's security pass. `48-SECURITY.md` contains no supply-chain, curl,
   install.sh, fetch, or network row — the flag was dropped, so the phase's `threats_open: 0`
   record is silent on a surface its own plan identified.

**Fix:** Prefer the Go module proxy, which verifies via the checksum database — no piped shell:

```yaml
      - name: install pinned golangci-lint
        run: |
          set -o pipefail
          GOBIN="$(go env GOPATH)/bin"
          go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
          echo "$GOBIN" >> "$GITHUB_PATH"
```

If the install script must stay, pin it to a full commit SHA (never `HEAD`) and verify the
extracted binary against the v2.12.2 release's `checksums.txt` before adding it to `$GITHUB_PATH`.
Either way, re-record the supply-chain row in `48-SECURITY.md` so the register matches the code.

## Warnings

### WR-01: The new `verify` job is invisible to every guard the phase built — its existence is unpinned and neither its job-level nor step-level `if:` is checked (48-07/48-11 seam)

**File:** `verify-phase-48.sh:208` (`j = jobs["go"]`), `:580` (`jobs["fe"]`), `:654` (`jobs["docker"]`); subject: `.github/workflows/ci.yml:185-210`

**Issue:** The T-48-07 rule (reject job-level and step-level `if:` on gate steps, no allowlist)
is applied to the `go` job (P2/P3, script lines 218-224 and 269-270), `fe` (P6, 586-592) and
`docker` (P7, 659-664). The fourth gate job that 48-11 added is covered by none of them: no
probe ever reads `jobs["verify"]` (P5 checks only checkout-first ordering and composite
membership across all jobs; P9 covers only eval-gate's two jobs). All three of the following
mutations leave the gate at `ALL PASSED`:

- **Delete the `verify` job entirely** — the IN-07 fix (wire the verifier into CI) is itself not
  pinned by the verifier, so the rot IN-07 was meant to prevent can return unnoticed.
- **Add a job-level `if:`** (e.g. `if: github.event_name == 'push'`) — PR runs vanish and the
  published job name goes Pending-forever under branch protection: invariant 1's exact hazard,
  three paragraphs above the job.
- **Add a step-level `if:` on `phase gate self-proof`** — the job goes green without running the
  verifier: the WR-01/T-48-07 class in its original form.

48-07 hardened the three jobs that existed when it landed; 48-11 added a fourth; no plan
extended the guards across the seam.

**Fix:** Extend the P2/P3 probe (which already loads ci.yml) with the verify-job contract:

```python
vj = jobs.get("verify")
if not isinstance(vj, dict):
    raise SystemExit("P3: ci.yml has no `verify` job — the phase gate is not wired into CI (IN-07)")
if "if" in vj:
    raise SystemExit("P3: the verify job carries a job-level `if:` — the gate is skippable")
vsteps = vj.get("steps") or []
for s in vsteps:
    if "if" in s:
        raise SystemExit("P3: the verify job's step %r carries a step-level if: (T-48-07)"
                         % (s.get("name") or s.get("run")))
if not vsteps or (vsteps[-1].get("run") or "").strip() != "bash verify-phase-48.sh":
    raise SystemExit("P3: the verify job's last step is not the phase gate")
```

### WR-02: The install pipeline drops curl's exit status — the WR-03 class is reintroduced by the same phase that fixed it

**File:** `.github/workflows/ci.yml:203` (contrast `.github/workflows/eval-gate.yml:102-105`)

**Issue:** GitHub's default shell for `run:` is `bash -e {0}` — no `pipefail`. If `curl` fails
(transient network, HTTP 4xx/5xx — `-f` makes curl exit non-zero), the piped `sh` receives empty
input, exits 0, and the pipeline reports 0: the step goes green and `$GITHUB_PATH` is still
extended. The fault surfaces three steps later as the P4 preflight refusal
("golangci-lint is absent from PATH"), misattributing an install/network failure to the gate
itself. WR-03's fix for eval-gate's replay step (`shell: bash` + `set -o pipefail`) was not
carried into the job 48-11 added.

**Fix:**

```yaml
        run: |
          set -o pipefail
          LINT_BIN="$(go env GOPATH)/bin"
          mkdir -p "$LINT_BIN"
          curl -sSfL <pinned-url> | sh -s -- -b "$LINT_BIN" v2.12.2
          [ -x "$LINT_BIN/golangci-lint" ]
          echo "$LINT_BIN" >> "$GITHUB_PATH"
```

(The `[ -x ... ]` postcondition fails the step at the point of failure rather than three steps
later, independent of the CR-01 resolution.)

### WR-03: Begin-marker accounting does not bind a partition to its assertions — P8's `${TRUST_SURFACE_COUNT:-0}` default absorbs a deleted probe and keeps the gate green

**File:** `verify-phase-48.sh:934-939` (contrast the guarded-by-accident P1 at `:186`)

**Issue:** IN-06's begin-marker fix proves a partition body *started*, not that its probe *ran
and passed*. Comment out P8's `scan_trust_surface` call and its rc check (lines 934-937) while
keeping `begin_partition "P8"` (line 932) and `done_partition "P8 ..."` (line 939): the marker
exists, `RAN` counts P8, and the gate exits 0 — with the PASS line literally reading
"P8 trust-surface (**0** workflow(s) ...)" because the message interpolates
`${TRUST_SURFACE_COUNT:-0}`, whose default silently absorbs the unset probe output. P1 is the
only partition that defends itself here — its `done_partition` message interpolates `$WF_COUNT`
unguarded, so deleting the P1 probe trips `set -u` and fails. More generally, no partition is
required to have emitted its success token (`STRUCT-OK`, `COMPOSITE-ORDER-OK`, `FE-LINE-OK`,
`DOCKER-LINE-OK`, `TRUST-SURFACE-OK`, `EVALGATE-OK`), so the same deletion stays green for
P2-P7 and P9-P11 too.

**Fix:** Make each probe's success deposit `$WORK/ran/<P>.ok`, and have `done_partition` require
both markers; P11 half 1a then compares `BEGIN_COUNT` and `OK_COUNT` against
`EXPECTED_PARTITIONS`. Minimal immediate hardening: remove the `:-0` from P8's message so a
deleted probe fails loudly under `set -u`, exactly the way P1 already does.

### WR-04: The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0

**File:** `.github/workflows/ci.yml:95-96` (fragment pin at `verify-phase-48.sh:231`)

**Issue:** `go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100`
prints "testing: warning: no tests to run" and exits 0 when nothing matches the pattern. P2 pins
the command *text*, not the test's existence, so renaming or deleting
`TestDeriveEdgesDeterministic` silently voids the P5a determinism gate — the "found nothing,
reported green" class this phase exists to close, and the same vacuity rule P1 and P8 enforce
over file lists, unenforced over tests. The test exists today
(`internal/knowledge/graph_edges_test.go:50`); the defect is the future drift, not the present
state.

**Fix:**

```yaml
      - name: determinism -count=100 (P5a)
        run: |
          go test ./internal/knowledge/ -list '^TestDeriveEdgesDeterministic$' | grep -qx TestDeriveEdgesDeterministic
          go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100
```

The `-list` probe fails the step the moment the name drifts; P2's existing fragment assertions
remain satisfied (substring checks over the run block).

### WR-05: `Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise

**File:** `.github/workflows/eval-gate.yml:113-117` (evidence: `cmd/eval/main.go:463-466`)

**Issue:** WR-08's disposition states the secrets were moved "to step-level env on exactly the
two consuming steps (frozen-replay and gate check)". The consuming-steps premise is false for
gate check: `cmd/eval` short-circuits it before any bootstrap — "gating reads a file, so it must
not require GLM_API_KEY / Milvus / BGE-M3" (`cmd/eval/main.go:463-464`). The step-level env
block therefore grants `GLM_API_KEY`, `BGE_M3_URL` and `MILVUS_ADDRESS` to an offline threshold
comparison that cannot use them — pure secret-surface widening, against the minimization intent
WR-08 was filed for.

**Fix:** Delete the step-level `env:` block from `Gate check`; keep the three secrets on the
replay step only. The verifier's P9 wiring assertion (`secrets.* in src`) still passes — it
checks presence anywhere in the file.

## Info

### IN-01: Header still says "Three independent jobs" while the file now ships four

**File:** `.github/workflows/ci.yml:29-31` (contrast `:169-173`, which correctly says "FOURTH
published job name")

**Issue:** 48-11 added the `verify` job but did not update the header's job enumeration, leaving
the file internally contradictory.

**Fix:** One-line comment change — enumerate `go`, `fe`, `docker`, `verify` (and note `verify`
guards the other three).

### IN-02: A fork PR's green `verify` check is fully self-attested, and the job persists the runner token — the header's "strictly narrower surface" claim omits this

**File:** `.github/workflows/ci.yml:182-183, 192`

**Issue:** On a fork PR the checkout is the merge commit, so the script being executed
(`bash verify-phase-48.sh`) and the workflow files it judges (`$ROOT/.github/workflows`) are
both PR-controlled, with credentials persisted at the default. The IN-10 disposition scoped
`persist-credentials: false` to the `go` job and left `fe`/`docker` on the default, so this is
consistent with the recorded decision — not a re-raise. But the job's header comment ("all the
script touches, strictly narrower than the go job's surface") does not acknowledge that a fork
PR's green `verify` proves nothing about the base repository.

**Fix:** One header sentence next to the IN-08 closed-set note, and take the free hardening the
job's own threat shape justifies:

```yaml
      - uses: actions/checkout@v7
        with:
          persist-credentials: false
```

### IN-03: PyYAML is installed unpinned into the gate job

**File:** `.github/workflows/ci.yml:206-207`

**Issue:** `pip install pyyaml` resolves to whatever is newest at run time; a breaking or
compromised new release reaches the gate job unreviewed, and the install is uncached on every
run.

**Fix:** Pin to the version the local gate was validated against, e.g.
`python -m pip install --quiet --disable-pip-version-check 'pyyaml==6.0.2'`.

---

_Reviewed: 2026-10-19T15:44:23Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
