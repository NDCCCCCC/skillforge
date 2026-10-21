---
phase: "48"
slug: "ci"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
block_on: high
created: "2026-10-19"
verified: "2026-10-19"
# threats_open corrected 2026-10-19 AFTER 48-VERIFICATION.md reproduced the code review's
# mutations. The earlier value (0) was written before that evidence existed and contradicted this
# file's own narrative section — frontmatter said "verified / 0 open" while the prose below said
# five closures were falsified. A machine-readable verdict must never contradict its own report.
# threats_open re-closed to 0 on 2026-10-19 by plan 48-10: the nine rows the falsifications held
# open are now re-closed on the per-file / normalized-expression / sentinel / begin-marker
# assertions landed by plans 48-05..48-09, each citing the mutation that proves it (see the
# register rows and "Why these closures changed" § Re-closure).
# threats_open corrected again 2026-10-20 by plan 48-14 (gap-closure round 3): the register had
# been silent on a shipped surface its own plan had flagged — 48-11-SUMMARY.md § Threat Flags
# carried `threat_flag: supply-chain` for the verify job's `curl | sh` bootstrap while this
# frontmatter said 0 open. The row (T-48-47) lands only now that the fix (48-12: module-proxy
# install) and its mutation-proof pin (48-13: P3 assertion + reintroduction drills) exist; 0 open
# is truthful again.
# threats_open re-confirmed 2026-10-21 by plan 48-25 (gap-closure round 6): T-48-78's Mitigation
# cell was corrected in place to the P3-INSTENV-OK exact-equality pin 48-23 landed — the round-5
# verification had falsified the cell's enforcement clause — rows T-48-80..T-48-82 were registered
# after their mitigations and mutation drills landed (48-23/48-24), and 0 remains the truthful
# open count: 34/34 closed, 0 open.
---

# Phase 48 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Source of truth: the four `48-*-PLAN.md` `<threat_model>` registers, renumbered pre-execution so
> each ID names exactly one threat (see `48-PLAN-REVISION-3.md` § Post-review threat-ID renumbering).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Fork PR → `eval-gate` live job | Fork-controlled trigger reaches a job whose `env:` carries `GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` | live-provider credentials (secret) |
| Dependabot → same job | A same-repo actor that nonetheless receives no secrets, so `cmd/eval` would `log.Fatalf` on an empty substitution | availability only |
| Workflow trigger shape → token scope | Introducing `pull_request_target` re-runs fork code with base-repo secrets and a write token | repository write capability |
| GitHub Actions → published CI contract | The first real run turns three job names into an external contract for contributors and future tooling | published check names |
| `.github/workflows/` → token permissions | Any `permissions:` value or registry surface added here widens what CI can do | `GITHUB_TOKEN` scopes; registry credentials |
| Verifier → the phase's claim of correctness | If `verify-phase-48.sh` can pass without having run something, every other green here is unfounded | the phase's own evidence |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-48-01 | Elevation of Privilege | `.github/workflows/` | critical | mitigate | P1 scans the whole workflow tree for `pull_request_target`, fails with `file:line`; an empty file list fails rather than passes | closed |
| T-48-02 | Information Disclosure / Availability | `.github/workflows/eval-gate.yml` | high | mitigate | P9 normalizes the parsed `eval-gate.if:` and requires exact equality with the D-09 literal — appending `\|\| true`, wrapping the expression in an outer negation, or removing any one clause all red the gate (48-08 Task 1) | closed |
| T-48-03 | Spoofing | `.github/workflows/ci.yml` | high | mitigate | P8's `paths:` / `paths-ignore:` rule runs per enumerated workflow file inside `scan_trust_surface`; a second `zz-p8-paths.yml` carrying `paths: ["internal/**"]` fails the gate (48-05 Task 1) | closed |
| T-48-04 | Information Disclosure | `.github/workflows/ci.yml` | high | mitigate | P8's secret rule is a per-file named allowlist — `GITHUB_TOKEN` anywhere; `GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` only in `eval-gate.yml`; a second `zz-p8-secret.yml` carrying `${{ secrets.DEPLOY_KEY }}` fails the gate (48-05 Task 1) | closed |
| T-48-05 | Repudiation | `go-version` sourcing | medium | mitigate | P9 asserts the directory-wide ban on `go-version:` literals and that every `setup-go` carries `go-version-file: go.mod` | closed |
| T-48-06 | Information Disclosure | `ci.yml` job `env:` | high | mitigate | P2 asserts `MILVUS_ADDRESS` / `BGE_M3_URL` / `GLM_API_KEY` occur nowhere in `ci.yml`; job env carries only `CGO_ENABLED: "1"` | closed |
| T-48-07 | Repudiation | lint gate | high | mitigate | P2/P3 reject any step-level `if` key on the `go` job's steps, naming the step and the condition; P6/P7 carry the identical rule for the `fe` and `docker` jobs — injecting `if: false` on a gate step reds the gate (48-07 Tasks 1–2) | closed |
| T-48-08 | Repudiation | `pull-requests: read` scope | high | mitigate | P3 asserts the `go` job `permissions` map is exactly `{contents: read, pull-requests: read}`; P8 asserts every value is `read`/`none` | closed |
| T-48-09 | Tampering | `docker` job | high | mitigate | P7 asserts exactly two `push: false` and zero `load: true` / `tags:` | closed |
| T-48-10 | Tampering | registry surface | high | mitigate | P8's publication-token scan runs per enumerated workflow file, case-insensitively over comment-stripped text (`ghcr.io`, `docker push`, `docker/login-action`, `registry:`, `packages: write`, `push: true`); `zz-p8-registry.yaml` carrying `docker push ghcr.io/...` fails the gate and also proves `.yaml` enumeration (48-05 Task 1) | closed |
| T-48-11 | Repudiation | `verify-phase-48.sh` | high | mitigate | P1 parses and P8 enumerates both `*.yml` and `*.yaml` (an invalid `.yaml` fails P1 by name); P4's sentinel D requires a `package-comments` violation reported as `revive`, so deleting `linters.enable` reds the gate (48-05 Task 2 + 48-09 Task 1) | closed |
| T-48-12 | Denial of Service | `fe` / `docker` jobs | medium | mitigate | `timeout-minutes: 15` (`fe`) and `45` (`docker`) — verified present at `ci.yml:97` and `ci.yml:129`; no self-hosted runner | closed |
| T-48-13 | Spoofing | first-run "fixes" | high | mitigate | Task 3 forbids `continue-on-error` / `paths:` / `\|\| true` by name; P8/P10/P11 catch two | closed — the `paths:` arm's CR-01 coupling is resolved: T-48-03's re-closure now rests on 48-05's per-file `paths:` rule, so the arm is no longer single-file-blind |
| T-48-14 | Tampering | required status checks | medium | accept (gated) | **Decision: A** — leave branch protection off. Recorded verbatim in `48-04-SUMMARY.md` as a conscious refusal, not an oversight; nothing was configured | closed (accepted) |
| T-48-15 | Spoofing | `eval-gate` skip | high | mitigate | `eval-gate-skip-notice.if` is the verbatim negation of the gate expression, and P9 asserts the relationship `notice == "!(" + gate + ")"` after normalization — a Dependabot-attributed push is now announced, and reverting the notice, dropping the negation, or appending `\|\| true` reds the gate (48-08 Task 2) | closed |
| T-48-16 | Elevation of Privilege | this plan's own hardening edit | critical | mitigate | Banned by name in 48-03 Task 1; both verify commands fail if the string appears | closed |
| T-48-17 | Denial of Service | `eval-gate.yml` | medium | mitigate | Migrated to `go-version-file: go.mod`; the ban is asserted directory-wide | closed |
| T-48-18 | Information Disclosure / Availability | later fork-gate edits | high | mitigate | P9's normalized-expression equality has no substring fallback — an overall negation of the `if:` changes the normalized string and fails with both expressions printed (48-08 Task 1) | closed |
| T-48-19 | Denial of Service | unrelated later edits | medium | mitigate | P9's directory-wide `go-version:` ban plus the trigger-shape assertion | closed |
| T-48-20 | Repudiation | the partitions this phase adds | high | mitigate | P11 half 2 pushes the adversarial fixtures (`two-files`, `yaml-ext`, `lowercase-registry`, `stray-secret`) through **the same per-file `scan_trust_surface` P8 runs**, each required to fail for its named rule; half 1a requires registry == call sites == body-written begin markers == 11, so a body that did not run cannot reach `ALL PASSED` (48-05 Task 3 + 48-09 Task 3) | closed |
| T-48-47 | Tampering | `.github/workflows/ci.yml` verify job install step | critical | mitigate | Module-proxy install, not a pipe: the verify job runs `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` with an in-step `[ -x "$LINT_BIN/golangci-lint" ]` postcondition before the gate runs (48-12 Task 1); the mechanism is pinned by the same P2/P3 probe that judges the `go` job — install shape, `[ -x ]`, and two banned fetch/script tokens asserted (48-13 Task 2) — and re-adding a curl-piped install reds P3 naming the tokens (48-13 Task 3 drill). Origin: `48-11-SUMMARY.md` § Threat Flags recorded `threat_flag: supply-chain` against the then-shipped `curl …/HEAD/install.sh \| sh` step while this register stayed silent — the flag finally lands here | closed |
| T-48-70 | Repudiation | P2/P3 shared probe — per-assertion evidence | high | mitigate | `ok_partition` requires every one of P2/P3's eight per-assertion-group tokens (`P2-CMDS-OK` / `P2-DET-OK` / `P2-SHUFFLE-OK` / `P2-HERMETIC-OK` / `P3-STEP-IF-OK` / `P3-BLOCKING-OK` / `P3-LINT-OK` / `P3-VERIFYJOB-OK`) at deposit (48-20 Task 1); deleting P2's entire assertion block reds the gate naming partition P2 and the four missing P2 tokens (48-20 M13 drill) | closed |
| T-48-71 | Repudiation | P1 composite-parse and `pull_request_target` groups | high | mitigate | P1 deposits `P1-PARSED` + `P1-COMPOSITE-OK` + `P1-NO-PRT` with `# P1-B:`/`# P1-C:` section markers (48-20 Task 2); deleting the composite-parse group reds the gate naming P1 and `P1-COMPOSITE-OK` (48-20 P1-composite drill) | closed |
| T-48-72 | Repudiation | P4 sentinel evidence binding | high | mitigate | P4 emits one token per sentinel (`P4-A-ERRCHECK` / `P4-B-QUIET` / `P4-C-QUIET` / `P4-D-REVIVE`) concatenated into `p4.log` missing-file-tolerantly (48-20 Task 2); deleting sentinel A reds the gate naming P4 and `P4-A-ERRCHECK` (48-20 P4-sentinel-A drill) | closed |
| T-48-73 | Repudiation | P6/P7 single-print probes | high | mitigate | P6 emits four and P7 six per-assertion-group tokens with `# P6-A:`..`# P6-D:` / `# P7-A:`..`# P7-F:` markers, all required at deposit (48-21 Task 1); deleting the P6 job-level `if:` group or the P7 cache-scope group reds the gate naming the partition and the missing token (48-21 drills) | closed |
| T-48-74 | Repudiation | P9 combined probe print | high | mitigate | P9's six per-assertion-group tokens (`P9-JOBS-OK` … `P9-GOVERSION-OK`) replace the combined `EVALGATE-OK`/`GOVERSION-OK` print (48-21 Task 2); deleting the notice group reds the gate naming P9 and `P9-NOTICE-OK` (48-21 drill) | closed |
| T-48-75 | Repudiation | P11 else-detector SIGPIPE | medium | mitigate | The P11 else-detector captures the gate-to-banner range into `BANNER_RANGE` before a non-quiet grep reads it to EOF, so `set -o pipefail` can no longer EPIPE the producer into a silent "no else found" (48-21 Task 3); the banner-in-else mutation still reds P11 with `the passing banner sits after an else` (48-21 drill). Severity `medium` per 48-21-PLAN.md's own threat table — the declaring plan and this register state the same severity for the same ID | closed |
| T-48-76 | Tampering | trust-surface path-filter rule | high | mitigate | The parsed `on:` walk in `scan_trust_surface` rejects `paths`/`paths-ignore` keys on every event configuration mapping — quoted `"paths":` and spaced `paths :` shapes included — with the textual substring rule retained unchanged as the second layer (48-22 Task 1); the `quoted-paths` and `space-paths` P11 fixtures are rejected by the same scan, and deleting the parsed walk (marker range `# T-48-03:`) reds P11 naming `quoted-paths` (48-22 drill) | closed |
| T-48-77 | Information Disclosure | trust-surface secret rule | high | mitigate | A recursive walk over every parsed string value rejects named refs outside the allowlist, `secrets[` dynamic references and `toJSON(secrets` expressions, with the textual `SECRET_REF` scan retained unchanged as the second layer (48-22 Task 1); the `tojson-secrets` and `quoted-hash-secret` P11 fixtures are rejected — the latter hides its ref after a `#` the comment strip eats — and deleting the parsed walk (marker range `# T-48-04:`) reds P11 naming `tojson-secrets` (48-22 drill) | closed |
| T-48-78 | Tampering | `ci.yml` linter install environment | medium | mitigate | The install step declares `GOTOOLCHAIN: local`, `GOSUMDB: sum.golang.org` and `GOPROXY: "https://proxy.golang.org,direct"`, so the inherited runner image no longer decides what `go install` downloads; the overstated no-network claim is corrected to the module-proxy statement with the satisfiability argument (48-22 Task 2); the install command, `@v2.12.2` pin and `[ -x ]` postcondition are pinned by P3's install-shape assertion. Corrected 2026-10-21 (48-25 Task 2, round 6) — this cell's former enforcement clause ("re-narrowing or re-widening the declared env reds the gate") was **falsified** by the round-5 verification: the `GOTOOLCHAIN: auto` flip and the whole env-block deletion both kept exit 0 under the passing banner, so that clause is superseded here, not silently rewritten. The pin that actually exists is 48-23 Task 3's parsed exact-equality assertion over the install step's env map, wrapped in the `# T-48-78: install env pin` marker range with its own `P3-INSTENV-OK` token required at P3's deposit: re-narrowing (the `GOTOOLCHAIN` flip), re-widening (an extra key or changed value) or deleting the block reds P3 naming the got/want env maps, and deleting the assertion itself reds P3 with the missing `P3-INSTENV-OK` token (48-23 drills, round-5 reproductions R13/R14) | closed |
| T-48-79 | Repudiation | `48-REVIEW-DISPOSITION-R1.md` tail | medium | mitigate | The closing paragraph is prefixed by a historical annotation carrying `Historical note (superseded 2026-10-20)`, run `39380373930` (head `630f875`) four-green and the `Complete (run 39380373930)` statuses, with the original paragraph retained and labelled `**Superseded text (2026-10-19):**` (48-22 Task 3); the annotation precedes the retained text and the phrase `remain Pending` stays unique to it, so a verify comparing the two line numbers reds on removal or reordering | closed |
| T-48-80 | Repudiation | P11 fixture-loop evidence binding | high | mitigate | Every P11 fixture registers a required first-line rejection reason (case statement with a refusing default arm), a file-presence guard runs before the scan so a deleted heredoc cannot be "rejected" by vacuity, and five per-class deposit tokens (`P11-VACUITY-OK` / `P11-PATHFILTER-OK` / `P11-PERMSWRITE-OK` / `P11-PUBLICATION-OK` / `P11-SECRET-OK`) are all required at ok_partition (48-23 Task 1); deleting a fixture heredoc reds the gate naming the fixture ("has no workflow file to scan" — the round-5 reproduction R11 now bites), and deleting the parsed path-filter walk together with both its path fixtures reds naming `quoted-paths` (the compound drill R12) | closed |
| T-48-81 | Repudiation | P4 config-verify evidence binding | medium | mitigate | `P4-CONFIG-OK` is appended to `p4-config.log` only after the `golangci-lint config verify` rc refusal passes, joins `p4.log` behind the same missing-file-tolerant guard as the sentinel logs, and is required first by ok_partition P4; the block is wrapped in the column-0 marker range `# T-48-81: config verify evidence binding` (48-24 Task 1); deleting the range reds the gate naming partition P4 and the missing `P4-CONFIG-OK` token — by the group's own evidence, not by luck of sentinel D's revive attribution (48-24 drill) | closed |
| T-48-82 | Denial of Service | P1 trigger-scan false-positive direction | low | mitigate | P1's banned-event scan judges comment-stripped text per enumerated workflow file (line numbers preserved), a positive canary proves the probe can report a hit before any clean verdict deposits, and the decisive success (scanned == enumerated, hits == 0) is asserted before `P1-NO-PRT` (48-24 Task 2); a comment naming the event no longer false-fires the gate (the comment-only fixture drill runs the full gate green), while a real trigger still reds with `file:line` evidence and the matched line (the real-trigger drill, 48-24) | closed |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-48-01 | T-48-14 | Branch protection stays off. The repository has **no** protection today (`GET .../branches/main/protection` → 404), so nothing is exposed while the gates run unenforced. Enabling required status checks is a one-way door: a later job rename or a `ci.yml` split would strand every open PR. Enforcement is a deliberate future decision, once `go`/`fe`/`docker` have been stable across a few merges — **not** an oversight to be "fixed" by a later phase. | user (checkpoint:decision, plan 48-04 Task 4) | 2026-10-19 |

*Accepted risks do not resurface in future audit runs.*

---

## Why these closures changed

Two independent passes disagreed with the first pass of this file.

1. **This file was first written at ASVS L1** (grep depth): each row was closed because the string
   it forbids was absent. That establishes *absence*, not *coverage* — it cannot tell the
   difference between "the check ran over everything" and "the check only ever looked at one file".
2. **`48-REVIEW.md` then ran adversarial mutations** — broke the input, then watched whether the
   partition actually went red. **`48-VERIFICATION.md` independently reproduced four of them**
   using probe code extracted verbatim from `verify-phase-48.sh` and the real `golangci-lint 2.12.2`
   binary. Reproductions: a second `.yml` workflow carrying `paths:`, `${{ secrets.DEPLOY_KEY }}` and
   `docker push ghcr.io/...` → `TRUST-SURFACE-OK`, rc=0. `\|\| true` appended to the fork gate's
   `if:` → `EVALGATE-OK`, rc=0. `if: false` on all five `go`-job gate steps → `STRUCT-OK`, rc=0.
   `enable: [revive]` deleted → `config verify` rc=0, sentinel A still fires, sentinel C silent → P4
   reports PASS. Historical note (token names superseded 2026-10-20): STRUCT-OK and EVALGATE-OK were the verifier's shared per-probe success tokens on 2026-10-19; plans 48-20 and 48-21 split them into per-assertion-group tokens (P2-CMDS-OK, P3-STEP-IF-OK, P9-JOBS-OK and their siblings), so the mutation evidence recorded above still stands but the token names are historical.

One review sub-claim was **refuted** by the verifier and is recorded here so it is not re-raised:
WR-05 held that `.yaml` workflows escape P1 entirely. P1's `pull_request_target` scan is `grep -rn`
and does cover `.yaml`; only P1's parse loop and P8's enumeration are extension-blind. The finding
is narrowed, not dismissed.

**The gates were narrower than their own headers claimed.** No shipped evidence was wrong and none
of this was a live vulnerability — but a gate that reports green over less than it says it covers
is exactly the defect class this phase exists to eliminate, so those rows were recorded `open`
rather than waived. Per-finding detail: `48-REVIEW-DISPOSITION.md` (19 findings, final
dispositions recorded by plan 48-10 — fixed or deferred, none waived). `.golangci.yml` itself
reviewed clean.

### Re-closure (plan 48-10, 2026-10-19)

Plans 48-05…48-09 landed the hardened assertions and their mutation probes; the nine rows the
falsifications held open are re-closed above, each Mitigation cell naming the enforcing assertion
and the mutation that proves it. The five falsified closures, and what now proves each:

| Falsified closure | Rows | Now proven by | Mutation that bites |
|-------------------|------|---------------|---------------------|
| P8 single-file `paths:`/`secrets.` scope (CR-01) | T-48-03, T-48-04, T-48-10, T-48-20 | 48-05 Task 1/3: every rule moved into the per-file `scan_trust_surface` loop over `*.yml` + `*.yaml`, case-insensitive, comment-stripped | `zz-p8-paths.yml` (`paths: ["internal/**"]`), `zz-p8-secret.yml` (`secrets.DEPLOY_KEY`), `zz-p8-registry.yaml` (`docker push ghcr.io/...`) each fail the gate; the four P11 half-2 fixtures each fail through the same function |
| P9 three independent `if:` substrings (WR-07) | T-48-02, T-48-18 | 48-08 Task 1: P9 compares the whitespace-normalized `if:` to the exact D-09 literal, no substring fallback | `|| true` appended, outer negation wrapped, or one clause removed → P9 rc=1 printing both expressions |
| Step-level `if:` unchecked (WR-01) | T-48-07 | 48-07 Tasks 1–2: P2/P3 (and P6/P7) reject any `if` key on any gate step, no allowlist | `if: false` injected on build+vet / lint / go test / vitest / image-build steps → rc=1 naming the step |
| P4 vacuous with respect to `revive` (WR-04) | T-48-11, T-48-20 | 48-09 Task 1: sentinel D requires a `package-comments` violation reported as `revive`; 48-09 Task 3: begin-marker accounting | Deleting `linters.enable`/`settings` → sentinel D fails naming `revive`; deleting a `begin_partition` call → rc=1 naming the partition "did not run" |
| Skip notice not the complement (WR-02) | T-48-15 | 48-08 Task 2: the notice is the verbatim negation, and P9 asserts `notice == "!(" + gate + ")"` structurally | Old PR-only notice, bare un-negated gate, or notice + `|| true` → P9 rc=1 naming `complement` |

One review sub-claim was **refuted** by the verifier and is recorded here so it is not re-raised:
WR-05 held that `.yaml` workflows escape P1 entirely. P1's `pull_request_target` scan is `grep -rn`
and already covered `.yaml`; only P1's parse loop and P8's enumeration were extension-blind. That
finding is narrowed — and the narrowed part is now fixed anyway (both extensions are enumerated
everywhere a workflow file is counted or parsed, 48-05 Task 2).

### Re-closure (plan 48-14, 2026-10-20 — round-2 gap closure)

The round-2 gap-closure wave reintroduced the defect class it was fixing: 48-11's new `verify` job
bootstrapped its linter with `curl …/HEAD/install.sh | sh` (R2-CR-01), and `threats_open: 0` had
been recorded while `48-11-SUMMARY.md` § Threat Flags carried `threat_flag: supply-chain` against
that same step — the same frontmatter-vs-prose contradiction class this file's header comment block
already documents once. 48-12 replaced the install shape and 48-13 pinned it; the row lands here,
now that the fix and its mutation proof both exist:

| Falsified closure | Rows | Now proven by | Mutation that bites |
|-------------------|------|---------------|---------------------|
| Round-2 gap-closure wave reintroduced the defect class it was fixing (R2-CR-01) | T-48-47 | 48-12 Task 1 (fix) + 48-13 Tasks 2-3 (pin + drill) | re-adding a curl-piped install or any network fetch-and-execute step to ci.yml reds P3 with the fetch/script tokens named |

Arithmetic: the register carries 31 rows; 0 rows are `open` at or above `block_on: high`
(`threats_open: 0`), 30 are `closed`, and 1 is `closed (accepted)` (T-48-14, gated decision A) —
31/31 closed, 0 open. Because the open count is 0, the frontmatter sets `status: verified`. The
21st row (T-48-47) was added by plan 48-14 on 2026-10-20, after 48-12 fixed the surface and 48-13
pinned it. Rows 22-31 (T-48-70..T-48-79) were added by plan 48-22 on 2026-10-20: T-48-70..T-48-75
register the per-assertion-token and consumed-range mitigations shipped by plans 48-20 and 48-21,
and T-48-76..T-48-79 register the parsed-document trust-surface walks, the declared linter install
environment and the R1 historical annotation shipped by plan 48-22 — each Mitigation cell naming
the enforcing assertion/token and the drill that bites.

### Re-closure (round 6, plans 48-23..48-26, 2026-10-21)

The round-5 verification reproduced the falsification of T-48-78's enforcement clause: flipping
`GOTOOLCHAIN: local` to `auto` and deleting the whole install-step env block both kept exit 0
under the passing banner, while the cell claimed the gate reds on re-narrowing or re-widening.
48-23 landed the pin the clause claimed; the cell is corrected above and the falsification is
recorded here rather than silently rewritten:

| Falsified closure | Rows | Now proven by | Mutation that bites |
|-------------------|------|---------------|---------------------|
| T-48-78's enforcement clause — the register claimed "re-narrowing or re-widening the declared env reds the gate" while the round-5 verification reproduced the `GOTOOLCHAIN: auto` flip (R13) and the whole env-block deletion (R14) both keeping exit 0 under the passing banner (WR-R5-02) | T-48-78 | 48-23 Task 3: the P2/P3 probe's parsed exact-equality assertion over the install step's env map, wrapped in the `# T-48-78: install env pin` marker range with its own `P3-INSTENV-OK` token required at P3's deposit | the `GOTOOLCHAIN` flip (R13) and the env-block deletion (R14) each red P3 naming the got/want env maps; an extra-key widening reds the same way; deleting the assertion itself reds P3 with the missing `P3-INSTENV-OK` token (48-23 drills) |

T-48-80, T-48-81 and T-48-82 are NEW surfaces recorded as closed on their own landed evidence
(48-23 Task 1's attributing fixture loop, 48-24 Task 1's config-verify evidence binding and
48-24 Task 2's comment-stripped trigger scan), not re-closures — each row registered by plan
48-25 only after its mitigation and its mutation drill both existed.

Arithmetic (round 6, 2026-10-21): the register carries 34 rows; 33 are `closed`, 1 is
`closed (accepted)` (T-48-14, gated decision A), and 0 are `open` at or above `block_on: high` —
34/34 closed, 0 open. Row 32 (T-48-80) and the T-48-78 cell correction were recorded by plan
48-25; rows 33-34 (T-48-81, T-48-82) register the 48-24 mitigations. No existing row was deleted
or renumbered, and the row count arithmetic is truthful at 34. Because the open count is 0, the
frontmatter keeps `status: verified`.

---

## What this audit does NOT establish

Every `closed` above is a **declaration-level** result (`workflow.security_asvs_level = 1`): the
re-closures rest on partition assertions in `verify-phase-48.sh`, each proven non-vacuous by the
mutation probe named in its Mitigation cell. That establishes the gates bite on the committed
files; it does not establish runner-side runtime behavior. Two classes of proof remain open and
are tracked as Manual-Only M1–M5 in `48-VALIDATION.md`:

- **A CI runner has executed these workflows — six times; the sixth is the first all-green
  (recorded by plans 48-15 and 48-19).** `ci.yml` has run on the public default branch six
  times since `plan 48-15` force-pushed the local branch to public `main` on 2026-10-20. The
  earlier runs (runs 1-5) were all red; **run 6 of 6 — run id 39380373930 on head `630f875` —
  concluded `go=fe=docker=verify=success`**. Canonical fragment: `ci.yml total runs: 6; first
  all-green: 39380373930`. Run evidence for the earlier runs is recorded in `48-15-SUMMARY.md`
  (run-1 head `d168512`, run-2 head `62b6663`, run-3 head `a2a4c86`, run-4 head `304dcc4`) and
  `48-19-SUMMARY.md` (run-5 head `e161436`, run id 39378733451). Every "mitigated" claim above
  is now a claim about a run, not about a declaration — that is the production difference
  between "declared green" and "executed green". The history-decision constraint recorded by
  plan 48-11 (`origin/main` was a single unrelated commit with empty merge-base) was resolved by
  the owner's option-a decision: tag `v1.5-archive` preserves the original tip; force-push
  rewrote `main` to this branch. The "git transport unreachable" cause recorded by plan 48-11
  was already falsified by 48-VERIFICATION.md round 2. The three classified persistent reds
  recorded against the earlier runs were cleared at the source: **(a)** the go job's
  `determinism -count=100 (P5a)` was a **workflow pipeline shape** — `go test … -list … | grep
  -qx …` under `set -o pipefail` let `grep -q` close the pipe at the first match while `go test
  -list` still wrote its trailing `ok <pkg> <time>` line, so the producer took EPIPE →
  SIGPIPE → exit 141 (reproduced on WSL Ubuntu 22.04 / go1.25.5: 6/6 exit 141, exit 0 once the
  list is materialised); 48-16 materialised the list to `$RUNNER_TEMP/go-test-list.txt`; **(b)**
  the fe job's `Sparkline.test.tsx:79` hardcoded a Windows absolute path absent on the Linux
  runner (ENOENT); 48-16 resolved it through `import.meta.url`; **(c)** the docker job's
  `Dockerfile.backend` built on an alpine-musl base lacking the glibc symbols (`backtrace`,
  `malloc_trim`, `__res_init`) the static duckdb archive needs; 48-16 moved builder and runtime
  to glibc. A fifth red (run 5, head `e161436`) was a load-sensitive DuckDB reader-block ratio
  flake, fixed at the source by 48-19 (best-of-5 minimum sampling). The three build causes
  remain recorded in `48-VALIDATION.md`'s `Manual-Only Verifications` table as M1a-M1c (now
  closed), and none of this invalidates the existing `threats_open: 0` register — that
  register's `mitigate` dispositions are about the workflow's *declarations*, which the
  runner-side test surface confirmed. WR-06 closes positively on run-3 (the push-path lint
  scope: with `only-new-issues: true` alone, on a fast-forward push, the action's diff-scoped
  output reported zero findings — recorded fixed-by-evidence in `48-REVIEW-DISPOSITION-R1.md`
  and `48-REVIEW-DISPOSITION.md`). CICD-01/02/03 are Complete against run 39380373930.
  **Round-6 update (2026-10-21, plan 48-26):** the round-6 tree — head
  `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, carrying 48-23's attributing fixture loop and
  `P3-INSTENV-OK` install-env pin, 48-24's `P4-CONFIG-OK` evidence binding and comment-stripped
  trigger scan, and 48-25's record corrections (the T-48-80..T-48-82 mitigations and the corrected
  T-48-78 pin) — was pushed to public `main` as a fast-forward (`630f875..c060c84`, no force), and
  run 7 of 7 (id `39525757252`, event `push`, 2026-10-21) concluded
  `go=fe=docker=verify=success` — the **first public run of the round-6 gate**, closing the
  round-5 record boundary: the current gate script and verify job have now executed publicly, not
  only locally. The go, fe and docker job spans are byte-identical between `630f875` and
  `c060c84` — the ci.yml delta is the push-trigger comment block (48-25's IN-R5-04 correction,
  comments only), the verify-job header comment block, and the verify install step's declared env
  map (values unchanged) — so the `Complete (run 39380373930)` statuses stand for the pushed tree
  without re-flipping. Canonical fragment: `ci.yml total runs: 7; first all-green: 39380373930;
  run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252, event push) is the first
  public run of the round-6 gate (four-green: go=success fe=success docker=success
  verify=success)`.
- **Images were built by the runner but never published.** Run 39380373930's `docker` job built
  both images (backend on the glibc base, frontend) with `push: false`, so `T-48-09`'s
  non-publication and the two per-image cache scopes are now runner-observed. Docker is absent
  from this host, so the local build remains unproven. M5 (second-run cache comparison) stays
  open.

Deferred decisions carried from plan 48-11:

- **WR-06 — closed-by-evidence on run-3 (plan 48-15).** Whether `golangci-lint-action@v9` falls back to whole-repo
  linting on a `push` to `main` is **measured**: run-1 (head `d168512`,
  force-push with no common ancestor) revealed the action's push-patch fetch errored
  with "No common ancestor" and the action fell back to whole-repo lint with
  11+ legacy findings; run-2 (head `62b6663`, fast-forward) tested candidate A
  (`args: --new-from-rev` on the lint step) and the action rejected it as
  incompatible with `only-new-issues: true` ("please, don't specify manually
  --new* args when requesting only new issues"); run-3 (head `a2a4c86`,
  fast-forward with the recipe-revert commit, pre-plan lint step restored)
  confirmed the action's `only-new-issues: true` alone is sufficient on a normal
  fast-forward push — diff was fetched, scoped, and reported zero findings.
  Candidate B (PR-gating the lint step) remains D-06-reversing and was not chosen.
  No lint-step code change was applied to production. WR-06 closes
  fixed-by-evidence in both disposition ledgers.
- **IN-08 and IN-07 — implemented, not deferred.** IN-08 landed as option F (the closed PR base
  set `[main, 'phase-*']` documented in the `ci.yml` header, no behavior change); IN-07 landed as
  option A (a fourth published `verify` job running `bash verify-phase-48.sh` with the pinned
  linter). Both are recorded `fixed` in `48-REVIEW-DISPOSITION.md`.

The local substitutes that *were* observed green: `bash verify-phase-48.sh` → 11/11 partitions PASS,
exit 0; Go `build ./...` + `test ./...` exit 0; Vitest 85 files / 689 tests exit 0; D-19 confirmed
that the `-test.shuffle` seed prints only under `-v`.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-19 | 20 | 11 | 9 | gsd execute-phase → secure-phase (ASVS L1, register authored at plan time; auditor skipped per the L1 short-circuit) |
| 2026-10-19 | 20 | 11 | 9 | corrected after `48-REVIEW.md` + `48-VERIFICATION.md` reproduced the mutations — the first row's `0 open` verdict is superseded, not deleted |
| 2026-10-19 | 20 | 20 | 0 | gsd gap-closure (plans 48-05…48-09) — re-closed on mutation evidence; see 48-10-SUMMARY.md |
| 2026-10-20 | 21 | 21 | 0 | gsd gap-closure round 3 (plans 48-12..48-13) — supply-chain row recorded after fix+pin landed |
| 2026-10-20 | 31 | 31 | 0 | gsd gap-closure rounds 4-5 (plans 48-20..48-22) — per-assertion-group tokens (T-48-70..T-48-75), parsed-document trust-surface walks and declared linter install env (T-48-76..T-48-78), R1 ledger historical annotation (T-48-79); register rows recorded by plan 48-22 after each mitigation and its mutation drill landed |
| 2026-10-21 | 34 | 34 | 0 | gsd gap-closure round 6 (plans 48-23..48-26) — T-48-78's Mitigation cell corrected in place to the `P3-INSTENV-OK` exact-equality pin 48-23 landed (round 5 had falsified its enforcement clause; recorded in the round-6 re-closure section), and round-6 rows T-48-80 (48-23 Task 1), T-48-81 (48-24 Task 1) and T-48-82 (48-24 Task 2) registered by plan 48-25 after each mitigation and its drill landed |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed — 34/34 closed, 0 open: the nine re-opened rows are re-closed on the mutation-proven assertions of plans 48-05…48-09, the supply-chain surface flagged by `48-11-SUMMARY.md` § Threat Flags is received as row T-48-47 (closed; 48-12 fix + 48-13 pin/drill), the round-4/5 rows T-48-70..T-48-79 (48-20..48-22) are closed on their per-assertion tokens, parsed-document walks, declared install env and R1 annotation, and the round-6 rows T-48-80..T-48-82 (48-23/48-24, registered by 48-25 on 2026-10-21) are closed on the attributing fixture loop, the config-verify evidence binding and the comment-stripped trigger scan; T-48-78's Mitigation cell was corrected on 2026-10-21 (48-25) to the `P3-INSTENV-OK` pin that actually exists after round 5 falsified its enforcement clause
- [x] `status: verified` set in frontmatter — matches the register (0 open at or above `block_on: high`)

**Approval:** APPROVED — 34/34 closed, 0 open (2026-10-21, plan 48-25 round-6 reconciliation: T-48-78's cell corrected to the pin that actually exists and rows T-48-80..T-48-82 registered on 48-23/48-24's landed evidence; earlier approvals: 31/31 at the 48-22 round-5 reconciliation, 21/21 at the 48-19 reconciliation). Class A (no real CI run, no image build) was the recorded environment block through plan 48-14. On plan 48-15 the runner executed ci.yml on the public default branch; on plan 48-19 the derived total is six runs (`gh run list`), and **run 6 of 6 — run id 39380373930 on head `630f875` — is four-green: `go=fe=docker=verify=success`** (canonical fragment: `ci.yml total runs: 6; first all-green: 39380373930`; run evidence in `48-15-SUMMARY.md`, `48-19-SUMMARY.md` and `48-VALIDATION.md`). WR-06 closes positively on run-3 evidence (lint step green, fast-forward push with `only-new-issues: true` alone). The three classified persistent real reds from the earlier runs (P5a SIGPIPE on go, Sparkline test path on fe, alpine-musl vs duckdb on docker) were cleared by 48-16's source fixes, and the run-5 reader-block ratio flake was cleared by 48-19; they are recorded in the same "What this audit does NOT establish" section above and in the Manual-Only rows. Round-6 addendum (2026-10-21, plan 48-26): run 7 of 7 (id `39525757252`, head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, fast-forward push `630f875..c060c84`) is four-green — the first public run of the round-6 gate, so the T-48-80..T-48-82 mitigations and the corrected T-48-78 pin are now runner-executed; canonical fragment: `ci.yml total runs: 7; first all-green: 39380373930; run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252, event push) is the first public run of the round-6 gate (four-green: go=success fe=success docker=success verify=success)`.

## Security Audit 2026-10-19

| Metric | Count |
|---|---|
| Threats found | 20 |
| Closed | 20 |
| Open | 0 |

## Security Audit 2026-10-20

| Metric | Count |
|---|---|
| Threats found | 31 |
| Closed | 31 |
| Open | 0 |

## Security Audit 2026-10-21

| Metric | Count |
|---|---|
| Threats found | 34 |
| Closed | 34 |
| Open | 0 |
