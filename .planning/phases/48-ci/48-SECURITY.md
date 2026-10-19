---
phase: "48"
slug: "ci"
status: gaps_found
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 9
asvs_level: 1
block_on: high
created: "2026-10-19"
verified: "2026-10-19"
# threats_open corrected 2026-10-19 AFTER 48-VERIFICATION.md reproduced the code review's
# mutations. The earlier value (0) was written before that evidence existed and contradicted this
# file's own narrative section — frontmatter said "verified / 0 open" while the prose below said
# five closures were falsified. A machine-readable verdict must never contradict its own report.
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
| T-48-02 | Information Disclosure / Availability | `.github/workflows/eval-gate.yml` | high | mitigate | P9 asserts each of the three job-level `if:` clauses individually and names the missing one | **open** — WR-07: three independent substrings, so an `if:` neutralized with `\|\| true` still reports `EVALGATE-OK` |
| T-48-03 | Spoofing | `.github/workflows/ci.yml` | high | mitigate | P8 asserts zero `paths:` / `paths-ignore:` | **open** — CR-01: P8 reads only a file literally named `ci.yml` while claiming directory-wide coverage |
| T-48-04 | Information Disclosure | `.github/workflows/ci.yml` | high | mitigate | P8 asserts `ci.yml` references nothing but `secrets.GITHUB_TOKEN` | **open** — CR-01: same single-file scope; a later workflow's `${{ secrets.* }}` passes |
| T-48-05 | Repudiation | `go-version` sourcing | medium | mitigate | P9 asserts the directory-wide ban on `go-version:` literals and that every `setup-go` carries `go-version-file: go.mod` | closed |
| T-48-06 | Information Disclosure | `ci.yml` job `env:` | high | mitigate | P2 asserts `MILVUS_ADDRESS` / `BGE_M3_URL` / `GLM_API_KEY` occur nowhere in `ci.yml`; job env carries only `CGO_ENABLED: "1"` | closed |
| T-48-07 | Repudiation | lint gate | high | mitigate | P3 asserts zero `continue-on-error` and a pinned action + `version: v2.12.2` | **open** — WR-01: P2/P3 reject a job-level `if:` but not a step-level one, so the gate steps can be skipped while both partitions stay green |
| T-48-08 | Repudiation | `pull-requests: read` scope | high | mitigate | P3 asserts the `go` job `permissions` map is exactly `{contents: read, pull-requests: read}`; P8 asserts every value is `read`/`none` | closed |
| T-48-09 | Tampering | `docker` job | high | mitigate | P7 asserts exactly two `push: false` and zero `load: true` / `tags:` | closed |
| T-48-10 | Tampering | registry surface | high | mitigate | P8 scans "the entire workflow directory" for `docker/login-action` / `registry:` / `GHCR` / `packages: write` | **open** — CR-01 + IN-02: the scan is single-file, and `ghcr.io` (lowercase) slips the case-sensitive `GHCR` pattern |
| T-48-11 | Repudiation | `verify-phase-48.sh` | high | mitigate | P1's empty-file-list guard, P4's `LINT NOT VERIFIED` + exit 1, P11's `RAN` counter | **open** — WR-05 + WR-04: `.yaml` workflows are invisible to P1's parse loop, and P4 has no positive `revive` sentinel, so it reports green with `revive` deleted |
| T-48-12 | Denial of Service | `fe` / `docker` jobs | medium | mitigate | `timeout-minutes: 15` (`fe`) and `45` (`docker`) — verified present at `ci.yml:97` and `ci.yml:129`; no self-hosted runner | closed |
| T-48-13 | Spoofing | first-run "fixes" | high | mitigate | Task 3 forbids `continue-on-error` / `paths:` / `\|\| true` by name; P8/P10/P11 catch two | closed — but **coupled to T-48-03**: its `paths:` arm inherits CR-01's single-file blind spot. Closing that threat re-opens this one. |
| T-48-14 | Tampering | required status checks | medium | accept (gated) | **Decision: A** — leave branch protection off. Recorded verbatim in `48-04-SUMMARY.md` as a conscious refusal, not an oversight; nothing was configured | closed (accepted) |
| T-48-15 | Spoofing | `eval-gate` skip | high | mitigate | The `eval-gate-skip-notice` job with the exactly-inverted `if:` and a literal `::notice::` | **open** — WR-02: the notice job is not the complement of the gate; a Dependabot push to `main` skips the live gate *and* the notice, showing green with no `::notice::` |
| T-48-16 | Elevation of Privilege | this plan's own hardening edit | critical | mitigate | Banned by name in 48-03 Task 1; both verify commands fail if the string appears | closed |
| T-48-17 | Denial of Service | `eval-gate.yml` | medium | mitigate | Migrated to `go-version-file: go.mod`; the ban is asserted directory-wide | closed |
| T-48-18 | Information Disclosure / Availability | later fork-gate edits | high | mitigate | P9 asserts each clause individually and names the missing one | **open** — WR-07: an overall negation of the `if:` also passes |
| T-48-19 | Denial of Service | unrelated later edits | medium | mitigate | P9's directory-wide `go-version:` ban plus the trigger-shape assertion | closed |
| T-48-20 | Repudiation | the partitions this phase adds | high | mitigate | P11 half 1 recomputes the partition registry against the `done_partition` call sites; half 2 pushes empty fixtures through **the same `scan_trust_surface` P8 runs** | **open** — CR-01: half 2's vacuity proof inherits P8's single-file scope, so it cannot detect the fail-open it was built to catch |

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

**The gates are narrower than their own headers claim.** No shipped evidence is wrong and none of
this is a live vulnerability today — but a gate that reports green over less than it says it covers
is exactly the defect class this phase exists to eliminate, so these rows are recorded `open`
rather than waived. Per-finding detail: `48-REVIEW-DISPOSITION.md` (19 findings, all `open`, none
waived). `.golangci.yml` itself reviewed clean.

---

## What this audit does NOT establish

Every `closed` above is an **L1 grep-depth** result (`workflow.security_asvs_level = 1`): it
establishes that the asserted string, key or counter is present or absent in the committed files.
It does not establish runtime behavior. Two classes of proof remain open and are tracked as
Manual-Only M1–M5 in `48-VALIDATION.md`:

- **No CI runner has executed these workflows.** `ci.yml` has never been on the remote default
  branch, and git transport to github.com failed from the executing host (3/3 attempts), so
  `go` / `fe` / `docker` have never reported a green conclusion. Every "mitigated" claim above is a
  claim about *declaration*, not about *a run*. `origin/main` is also an unrelated single commit
  with an empty merge-base, so no PR can be opened from this branch.
- **No image was built.** Docker is absent from this host, so `T-48-09`'s `push: false` and the two
  per-image cache scopes are statically asserted only.

The local substitutes that *were* observed green: `bash verify-phase-48.sh` → 11/11 partitions PASS,
exit 0; Go `build ./...` + `test ./...` exit 0; Vitest 85 files / 689 tests exit 0; D-19 confirmed
that the `-test.shuffle` seed prints only under `-v`.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-19 | 20 | 11 | 9 | gsd execute-phase → secure-phase (ASVS L1, register authored at plan time; auditor skipped per the L1 short-circuit) |
| 2026-10-19 | 20 | 11 | 9 | corrected after `48-REVIEW.md` + `48-VERIFICATION.md` reproduced the mutations — the first row's `0 open` verdict is superseded, not deleted |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [ ] `threats_open: 0` confirmed — **NOT met**: 9 high-severity threats open pending gap closure
- [ ] `status: verified` set in frontmatter — **NOT met**: phase is `gaps_found` (3/9 must-haves)

**Approval:** NOT APPROVED — 9 high-severity threats open (see `48-VERIFICATION.md`, `48-REVIEW.md`)
