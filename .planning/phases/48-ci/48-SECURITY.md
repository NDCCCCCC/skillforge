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
   reports PASS.

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

Arithmetic: the register carries 20 rows; 0 rows are `open` at or above `block_on: high`
(`threats_open: 0`), 19 are `closed`, and 1 is `closed (accepted)` (T-48-14, gated decision A).
Because the open count is 0, the frontmatter sets `status: verified`.

---

## What this audit does NOT establish

Every `closed` above is a **declaration-level** result (`workflow.security_asvs_level = 1`): the
re-closures rest on partition assertions in `verify-phase-48.sh`, each proven non-vacuous by the
mutation probe named in its Mitigation cell. That establishes the gates bite on the committed
files; it does not establish runner-side runtime behavior. Two classes of proof remain open and
are tracked as Manual-Only M1–M5 in `48-VALIDATION.md`:

- **No CI runner has executed these workflows — M1: BLOCKED (recorded by plan 48-11).** `ci.yml`
  has never been on the remote default branch, and git transport to github.com is unreachable from
  the executing host (connect timeout on 443; a TLS handshake failure on a second attempt), so
  `go` / `fe` / `docker` (and the new `verify` job) have never reported a conclusion. Every
  "mitigated" claim above is a claim about *declaration*, not about *a run*. `origin/main` is also
  an unrelated single commit (`369b08a`) with an empty merge-base, so no PR can be opened from
  this branch. The verbatim run recipe is persisted in `48-11-SUMMARY.md`; until a host with git
  transport produces one run with all jobs `success`, CICD-01/02/03 stay Pending.
- **No image was built.** Docker is absent from this host, so `T-48-09`'s `push: false` and the two
  per-image cache scopes are statically asserted only.

Deferred decisions carried from plan 48-11:

- **WR-06 — deferred (option D).** Whether `golangci-lint-action@v9` falls back to whole-repo
  linting on a `push` to `main` is **unmeasured**: the action cannot be exercised on a push event
  from this host, so the push-path result is missing. Both candidate fixes are recorded in
  `48-11-SUMMARY.md` (candidate A: explicit `--new-from-rev` diff base; candidate B: PR-gating the
  lint step — recorded as D-06-reversing and NOT chosen). No lint-step code change was made; the
  decision re-opens on the first real run's evidence. Owner: the first `ci.yml` run.
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

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed — the nine re-opened rows are re-closed on the mutation-proven assertions of plans 48-05…48-09
- [x] `status: verified` set in frontmatter — matches the register (0 open at or above `block_on: high`)

**Approval:** APPROVED — 20/20 closed, 0 open. Class A (no real CI run, no image build) remains honestly recorded under "What this audit does NOT establish" and in `48-VALIDATION.md` M1–M5; it is a recorded environment block, not an open threat.

## Security Audit 2026-10-19

| Metric | Count |
|---|---|
| Threats found | 20 |
| Closed | 20 |
| Open | 0 |
