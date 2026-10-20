---
phase: 48-ci
verified: 2026-10-20T16:04:09Z
status: gaps_found
score: 13/15 must-haves verified
covered_files:
  - .github/actions/setup/action.yml
  - .github/workflows/ci.yml
  - .github/workflows/eval-gate.yml
  - .golangci.yml
  - .planning/REQUIREMENTS.md
  - .planning/phases/48-ci/48-01-PLAN.md
  - .planning/phases/48-ci/48-01-SUMMARY.md
  - .planning/phases/48-ci/48-02-PLAN.md
  - .planning/phases/48-ci/48-02-SUMMARY.md
  - .planning/phases/48-ci/48-03-PLAN.md
  - .planning/phases/48-ci/48-03-SUMMARY.md
  - .planning/phases/48-ci/48-04-PLAN.md
  - .planning/phases/48-ci/48-04-SUMMARY.md
  - .planning/phases/48-ci/48-05-PLAN.md
  - .planning/phases/48-ci/48-05-SUMMARY.md
  - .planning/phases/48-ci/48-06-PLAN.md
  - .planning/phases/48-ci/48-06-SUMMARY.md
  - .planning/phases/48-ci/48-07-PLAN.md
  - .planning/phases/48-ci/48-07-SUMMARY.md
  - .planning/phases/48-ci/48-08-PLAN.md
  - .planning/phases/48-ci/48-08-SUMMARY.md
  - .planning/phases/48-ci/48-09-PLAN.md
  - .planning/phases/48-ci/48-09-SUMMARY.md
  - .planning/phases/48-ci/48-10-PLAN.md
  - .planning/phases/48-ci/48-10-SUMMARY.md
  - .planning/phases/48-ci/48-11-PLAN.md
  - .planning/phases/48-ci/48-11-SUMMARY.md
  - .planning/phases/48-ci/48-12-PLAN.md
  - .planning/phases/48-ci/48-12-SUMMARY.md
  - .planning/phases/48-ci/48-13-PLAN.md
  - .planning/phases/48-ci/48-13-SUMMARY.md
  - .planning/phases/48-ci/48-14-PLAN.md
  - .planning/phases/48-ci/48-14-SUMMARY.md
  - .planning/phases/48-ci/48-15-PLAN.md
  - .planning/phases/48-ci/48-15-SUMMARY.md
  - .planning/phases/48-ci/48-16-PLAN.md
  - .planning/phases/48-ci/48-16-SUMMARY.md
  - .planning/phases/48-ci/48-17-PLAN.md
  - .planning/phases/48-ci/48-17-SUMMARY.md
  - .planning/phases/48-ci/48-18-PLAN.md
  - .planning/phases/48-ci/48-18-SUMMARY.md
  - .planning/phases/48-ci/48-19-PLAN.md
  - .planning/phases/48-ci/48-19-SUMMARY.md
  - .planning/phases/48-ci/48-20-PLAN.md
  - .planning/phases/48-ci/48-20-SUMMARY.md
  - .planning/phases/48-ci/48-21-PLAN.md
  - .planning/phases/48-ci/48-21-SUMMARY.md
  - .planning/phases/48-ci/48-22-PLAN.md
  - .planning/phases/48-ci/48-22-SUMMARY.md
  - .planning/phases/48-ci/48-REVIEW-DISPOSITION-R1.md
  - .planning/phases/48-ci/48-SECURITY.md
  - Dockerfile.backend
  - Dockerfile.frontend
  - internal/duckdb/connector_test.go
  - verify-phase-48.sh
  - web/src/components/chat/parts/Sparkline.test.tsx
covered_digest: "v3:sha256:8a7b2858b06fada58b1df17d25ce4835043aab915701be8b5ecb9a0d16540336"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 13/15
  gaps_closed:
    - "Truth 12's round-4 failed class is closed: P2 and P3 no longer share a success token. Deleting P2's entire assertion block (the M13 mutation) now exits 1 naming P2 with all four of its tokens missing; the P3-section, P1-composite and P4-sentinel-A deletions red the same way — ok_partition takes one or more required tokens and 15 per-assertion tokens cover P1/P2/P3/P4"
    - "Truth 12's WR-03 residual phase-wide: P6 (4 tokens), P7 (6) and P9 (6) each require their own groups' evidence; the P6 job-if, P7 cache-scope and P9 notice-group deletion drills each exit 1 naming the partition and the missing token"
    - "Truth 15's PARTIAL half is closed: 48-REVIEW-DISPOSITION-R1.md's closing paragraph is preceded by the 'Historical note (superseded 2026-10-20)' annotation (line 62 < line 70), names run 39380373930 head 630f875 four-green and 'CICD-01/02/03 are `Complete (run 39380373930)`', labels the retained paragraph '**Superseded text (2026-10-19):**', and the file remains tracked"
    - "Round-4 advisory 1 (P11 SIGPIPE else-detector) is closed: the gate-to-banner range is materialised into BANNER_RANGE before a non-quiet grep reads it to EOF; moving the banner after an else still exits 1 naming P11"
    - "Round-4 advisory 2 (text-scan evasion) is closed at the rule layer: parsed `on:` walk and recursive parsed-string secret walk are primary with the textual rules retained unchanged as second layers; all four evasion fixtures (quoted-paths, space-paths, tojson-secrets, quoted-hash-secret) are rejected through the parsed paths in the live gate log; deleting either walk reds P11 naming the fixture only it caught"
    - "Round-4 advisory 3 (install surface declared) is closed as a declaration: ci.yml:242-245 declares GOTOOLCHAIN: local, GOSUMDB: sum.golang.org and GOPROXY on the install step, with the corrected module-proxy comment — but see gap 2: nothing pins it"
    - "Round-4 advisory 4 (coordinated version bump) is closed: verify-phase-48.sh:176 carries the exact 'Bump together with LINT_VERSION…' sentence naming both ci.yml locations, and ci.yml:201 names LINT_VERSION back"
    - "48-SECURITY.md registers T-48-70..T-48-79 exactly once each (31 rows total), 0 open, status verified, 31/31/0 audit-trail row naming plans 48-20..48-22, the 2026-10-20 metric table at 31/31/0, and the STRUCT-OK/EVALGATE-OK narrative marked historical"
  gaps_remaining:
    - "The P11 fixture loop counts any non-zero scan_trust_surface exit as a pin and deposits a single 'rejected' token, so deleting a fixture heredoc keeps exit 0 / ALL PASSED (reproduced); sharper, deleting the parsed path-filter walk TOGETHER with its two fixtures also keeps exit 0 / ALL PASSED (reproduced) — a security rule and its evidence can vanish silently (WR-R5-01, independently reproduced)"
    - "The 48-22 install-surface env declarations are pinned by nothing: flipping GOTOOLCHAIN to auto (reproduced) and deleting the whole env block (reproduced) both keep exit 0 / ALL PASSED; 48-SECURITY.md row T-48-78's enforcement clause ('re-narrowing or re-widening the declared env reds the gate') is therefore false (WR-R5-02, independently reproduced)"
  regressions: []
gaps:
  - truth: "The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20)"
    status: failed
    reason: "Round 4's named residual is genuinely closed — I re-ran the M13 mutation plus eight sibling deletion drills (P3 section, P1 composite, P4 sentinel A, P6 job-if, P7 cache, P9 notice, banner-in-else, both parsed walks) and every one reds with the partition and the missing token named. A new fold class remains in P11's fixture loop: its only success criteria are a non-zero scan_trust_surface exit per fixture, a count of 12, and the single 'rejected' token — but scan_trust_surface also returns non-zero when it FINDS NOTHING (zero-files branch), so deleting a fixture heredoc leaves an empty directory that is 'rejected' by vacuity, the count still reaches 12, and the gate exits 0 with ALL PASSED. Reproduced. The compound form is worse: deleting the parsed path-filter walk together with its two fixture heredocs also exits 0 / ALL PASSED (reproduced) — the quoted-`\"paths\":` evasion class 48-22 closed becomes unguarded with a green banner, in a file whose own comments name 'found nothing, reported green' as the class the phase exists to close."
    artifacts:
      - path: "verify-phase-48.sh"
        issue: "P11 fixture loop (lines 1786-1809) accepts any non-zero rc as a pin; the deposit at line 1814 requires only the one token 'rejected'; the zero-files rejection branch (lines 1011-1016) collides with the loop's success criteria; the mkdir list (1574-1576) still creates the directory of a deleted fixture"
    missing:
      - "Assert the rejection REASON per fixture, not just the exit code — pair each fixture name with a required substring of scan_trust_surface's first log line and require the zero-files reason to be absent for every non-vacuity fixture (48-REVIEW.md WR-R5-01's fix)"
      - "Emit one deposit token per fixture class instead of the single all-matching 'rejected' so a deleted fixture cannot hide behind its siblings' tokens"
      - "Add the compound drill (delete a parsed walk + its fixtures together) to the round's mutation evidence so the rule and its evidence are pinned jointly"
  - truth: "The gate's bootstrap opens no unrecorded supply-chain surface; register == code"
    status: partial
    reason: "The declaration half is real: ci.yml:242-245 declares GOTOOLCHAIN: local, GOSUMDB: sum.golang.org and GOPROXY: \"https://proxy.golang.org,direct\" on the install step, with an accurate module-proxy comment. But the surface is pinned by nothing and the record claims otherwise. GOSUMDB and GOPROXY appear nowhere in verify-phase-48.sh and GOTOOLCHAIN appears only inside a P9 failure message about `go-version:` literals (line 1440); the P3-VERIFYJOB assertion group (lines 444-499) checks the install LINE but never the step's env map. Reproduced twice: setting GOTOOLCHAIN: auto and deleting the whole env block both leave exit 0 / ALL PASSED. 48-SECURITY.md row T-48-78 (line 81) asserts 'P3's install-shape assertion still pins them, so re-narrowing or re-widening the declared env reds the gate' — falsified by both reproductions, so register != code for this row."
    artifacts:
      - path: ".github/workflows/ci.yml"
        issue: "install-step env (242-245) is asserted by no partition — the hardening shipped as a declaration without a pin"
      - path: "verify-phase-48.sh"
        issue: "P3-VERIFYJOB group (444-499) pins the install command line and the checkout's persist-credentials but never the install step's env map"
      - path: ".planning/phases/48-ci/48-SECURITY.md"
        issue: "T-48-78 row (line 81) claims the declared env 'reds the gate' when re-narrowed or re-widened — false by reproduction (status: closed rests partly on a false enforcement clause)"
    missing:
      - "Extend the P3-VERIFYJOB group with a parsed assertion that the install step's env map equals exactly {GOTOOLCHAIN: local, GOSUMDB: sum.golang.org, GOPROXY: \"https://proxy.golang.org,direct\"} (48-REVIEW.md WR-R5-02's fix), then re-run the two reproductions to show they red"
      - "Correct 48-SECURITY.md's T-48-78 mitigation cell to name the pin that actually exists after the assertion lands"
advisory:
  - finding: "48-REVIEW-DISPOSITION.md (the round-4 review's mechanical ledger) still records all 15 rows `open` and carries `unparsed: 7` — the round-5 findings (WR-R5-NN) do not match its CR|BL|WR|IN-NN row schema, so they never entered the ledger; rows fixed in round 5 (WR-01/02/03, IN-01, IN-02) read as untriaged"
    category: other
    reason: "Record-keeping inconsistency, not a false run/transport claim (truth 15's scope); the authoritative dispositions live in 48-VERIFICATION.md and the R1/R2 ledgers, and the round-5 review re-raised every carried finding with provenance so nothing is lost"
    evidence_status: "code read (48-REVIEW-DISPOSITION.md frontmatter + table)"
  - finding: "Round-5 review IN-R5-01: the publication-token rule gained no parsed layer in the 48-22 conversion — quoted `push: 'true'` and an off-allowlist `tags:` registry evade it outside P7's two judged steps"
    category: security
    reason: "Info-grade per the review (a real publish usually also names ghcr.io/docker.io); the same WR-02 asymmetry class, third rule"
    evidence_status: "code read (verify-phase-48.sh:1055-1065, :1112-1119); not reproduced"
  - finding: "Round-5 review IN-R5-02: the parsed `secrets[` branch and P4's config-verify group have no evidence binding of their own (no fixture for `secrets['NAME']`; no P4-CONFIG-OK token)"
    category: other
    reason: "Info-grade per the review; sentinel D's revive attribution still reds a config-verify deletion, but by luck of its design"
    evidence_status: "code read (verify-phase-48.sh:1251-1254, :586-593); not reproduced"
  - finding: "Round-5 review IN-R5-03 (carried from round-4 IN-03): P1's `pull_request_target` scan reads raw workflow text, so a comment mentioning the event name false-fires — the false-positive direction only"
    category: other
    reason: "Exit-code discipline intact; P8 strips comments for the same class; parsed-document conversion of this scan is the remaining asymmetry"
    evidence_status: "code read (verify-phase-48.sh:265-279); not reproduced"
  - finding: "Round-5 review IN-R5-04 (carried from round-4 IN-07): the push-to-main trigger comment's whole-repo-lint justification does not follow from limiting the trigger to main"
    category: other
    reason: "Unsound comment on an auditable contract file; no behavioral effect"
    evidence_status: "code read (.github/workflows/ci.yml:41-45); not reproduced"
  - finding: "Round-5 review IN-R5-05: dead code left by the round-5 edits — `import re, sys, yaml` never uses `re` in the P2/P3 probe; `basename` computed twice per file in scan_trust_surface"
    category: other
    reason: "Harmless drift, but exactly the kind the column-0 drill markers exist to make attributable"
    evidence_status: "code read (verify-phase-48.sh:296, :1225, :1267) — confirmed present"
---

# Phase 48: CI 矩阵 Verification Report (Re-verification, round 5)

**Phase Goal:** 外部贡献者的每次 PR 与维护者的每次 push 都被自动化质量门守护——Go / FE / Docker 三线在 GitHub Actions 上公开绿灯，把"666 Go 测试 + 84 FE 测试"的可信度从本机 WSL 搬到公共可见的 CI。
**Verified:** 2026-10-20T16:04:09Z
**Status:** gaps_found
**Re-verification:** Yes — after gap-closure plans 48-20..48-22 (round 5)

---

## Goal Achievement

### The public outcome stands, and the record boundary is stated plainly

The four-green public run is unchanged and re-confirmed by independent fetch during this verification:

- **Run `39380373930`** (`head 630f875`, event `push`, branch `main`, 2026-10-20T06:26:30Z) concludes **`success`**. Per job, fetched from the Actions API: **`go=success`, `fe=success`, `docker=success`, `verify=success`**, and the per-step query returned **zero non-success steps in all four jobs** (a skipped step would have been listed).
- Six `ci.yml` runs exist (1–5 failure, 6 success). Remote `main` is still `630f8751ab65a07d71d5fa6f25fd8e13c3838ef7` — the green run's own head.
- **Record boundary (honesty, the round-4 standard):** since `630f875`, exactly two implementation files changed — `verify-phase-48.sh` (355 lines, plans 48-20/21/22) and the `verify` job of `ci.yml` (comment + install-step `env:`, plan 48-22). I diffed the `go`, `fe` and `docker` job spans between `630f875` and HEAD: **byte-identical**, so the green run's evidence for CICD-01/02/03 remains valid for the committed tree. The `verify` job definition and the new gate script have **not** run on a public runner: the current gate's green is local-only (my run below, plus the round-5 review's independent host run). Until the next push, the committed tree's own CI conclusion is unknown; no claim in this report relies on it.
- CICD-01/02/03 are `Complete (run 39380373930)` in both REQUIREMENTS.md places (requirement list and Traceability table).

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1 / CICD-01: the Go line runs on a public trigger and its published check is green | ✓ VERIFIED | Run 39380373930 `go=success`, zero non-success steps (re-fetched); the `go` job span is byte-identical to the green run's head. |
| 2 | SC-2 / CICD-02: the FE line runs on a public trigger and its published check is green | ✓ VERIFIED | Same run `fe=success`; `fe` job span byte-identical. |
| 3 | SC-3 / CICD-03: both Dockerfiles build in CI | ✓ VERIFIED | Same run `docker=success`; `docker` job span byte-identical. |
| 4 | A real GitHub Actions run of `ci.yml` exists on the public default branch through a gate-preserving trigger | ✓ VERIFIED | Six runs, all `push` to `main`; remote `main` = the green run's head `630f875`. |
| 5 | The repository-owner history decision is recorded and executed | ✓ VERIFIED | Round-4 evidence unchanged; remote `main` and the `v1.5-archive` tag state are untouched by round 5 (no non-`.planning` commit touches refs; `gh api` confirms `main` = `630f875`). |
| 6 | CICD-01/02/03 advance only against a corroborated four-green run | ✓ VERIFIED | Both REQUIREMENTS.md places cite `Complete (run 39380373930)`; that run's four job conclusions were fetched independently during this verification. |
| 7 | No gate was weakened to manufacture the run | ✓ VERIFIED | Re-scanned: ci.yml has no `continue-on-error`, no `paths:`/`paths-ignore:`, no `pull_request_target`, no `\|\| true`, no job- or step-level `if:`, no `secrets.` reference; the 48-22 delta is comment + install-step `env:` only (reversible class). eval-gate.yml's single `if: always()` is the report-artifact upload step, not a gate. Regression drills D10/D11/M12/M14 all still red. |
| 8 | WR-06 (push-path lint scope) is decided on run evidence | ✓ VERIFIED | Round-4 evidence unchanged: `fixed — closed-by-evidence on run-3` in both ledgers; run-3 id/head match. |
| 9 | The gate's bootstrap opens no unrecorded supply-chain surface; register == code | ⚠️ PARTIAL | The surface IS declared (ci.yml:242-245, three env keys, accurate comment; T-48-78 registered). But nothing pins it and the register row claims a pin: I reproduced both a `GOTOOLCHAIN: auto` flip and a full env-block deletion keeping exit 0 / ALL PASSED, while T-48-78 says re-narrowing or re-widening "reds the gate". Register != code for that row → gap 2. |
| 10 | The live gate grants secrets only to the step that consumes them | ✓ VERIFIED | P9 passes on the current tree (six tokens in the baseline log); re-adding the three-secret `env:` to `Gate check` reds the gate (reproduced, exit 1). |
| 11 | The workflow-side Class-B closure holds, including the rename-proof determinism guard | ✓ VERIFIED | Materialised-list shape present; reintroducing `go test -list \| grep -qx` reds P2/P3 (reproduced, exit 1). |
| 12 | The verifier refuses to fold a vacuous or neutralised check into PASS (P23 / T-48-11 / T-48-20) | ✗ FAILED | Round-4's class is closed: M13 (delete P2's whole assertion block) now exits 1 naming P2 and all four missing tokens, and eight sibling drills (P3, P1-composite, P4-sentinel-A, P6-job-if, P7-cache, P9-notice, banner-in-else, both parsed walks) red the same way. A new class remains: the P11 fixture loop counts any non-zero exit as a pin, so deleting a fixture heredoc keeps exit 0 / ALL PASSED (reproduced), and deleting the parsed path-filter walk together with its two fixtures also keeps exit 0 / ALL PASSED (reproduced). See gap 1. |
| 13 | SC-4: the live eval-gate never fakes green; its fork/Dependabot boundary, secrets and single Go-version source hold | ✓ VERIFIED | P9 passes on the current tree; the exact gate expression, complement notice, single-occurrence secrets, secret-free `Gate check` and `go-version-file: go.mod` are intact (P9's six per-assertion tokens all present in the baseline log). |
| 14 | SC-5: `bash verify-phase-48.sh` exits 0 | ✓ VERIFIED | Local run on the committed tree: exit 0, exactly one `ALL PASSED`, registry/begin/success tokens 11/11/11, all 12 P11 fixtures rejected. (The public `verify` job proved the 630f875 script; the current script is local-green only — see the record boundary above.) |
| 15 | Every record that asserts a run or transport state is reconciled to the same outcome | ✓ VERIFIED | R1 ledger's tail is annotated historical (annotation line 62 < retained paragraph line 70; names run 39380373930, head 630f875, four-green, `Complete (run 39380373930)`; `**Superseded text (2026-10-19):**` label; file tracked). The five 48-19 records stay reconciled; 48-SECURITY.md reads 31 rows / 0 open / verified with the token-names narrative marked historical. (The round-4 review's mechanical disposition ledger is stale but asserts no run/transport state — advisory.) |

**Score:** 13/15 truths verified (0 present-but-behavior-unverified, 1 partial, 1 failed)

---

## Mutation Evidence (independent, sandbox copies — the repository was not modified)

Every mutation was applied to a fresh `mktemp` copy of `verify-phase-48.sh` + `.github/` + `.golangci.yml` + `.planning/ROADMAP.md`; the mutated text was rewritten by a script whose anchors `assert` the pre-mutation shape, so a no-op could not fake a result. The healthy baseline (exit 0, one `ALL PASSED`) was run once on the committed tree itself.

| # | Mutation | Result | Verdict |
|---|----------|--------|---------|
| D0 | baseline (committed tree, unmodified) | exit 0, one `ALL PASSED`, 11/11 registry, 12/12 fixtures rejected | control |
| R1 | delete P2's whole assertion block (round-4 M13) | exit 1 — `partition P2 … does not contain the probe-emitted token(s) 'P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK'`, no banner | **round-4 gap CLOSED** |
| R2 | delete the P3 section | exit 1 — `partition P3 … 'P3-BLOCKING-OK P3-LINT-OK P3-VERIFYJOB-OK'` | BITES |
| R3 | delete the P1 composite group (`# P1-B:`..`# P1-C:`) | exit 1 — `partition P1 … 'P1-COMPOSITE-OK'` | BITES |
| R4 | delete P4 sentinel A | exit 1 — `partition P4 … 'P4-A-ERRCHECK'` | BITES |
| R5 | delete the P6 job-if group | exit 1 — `partition P6 … 'P6-JOB-IF-OK'` | BITES |
| R6 | delete the P7 cache group | exit 1 — `partition P7 … 'P7-CACHE-OK'` | BITES |
| R7 | delete the P9 notice group | exit 1 — `partition P9 … 'P9-NOTICE-OK'` | BITES |
| R8 | move the banner after the `else` | exit 1 — `FAIL P11 the passing banner sits after an else` | BITES (WR-01 closed) |
| R9 | delete the parsed path-filter walk | exit 1 — `scan_trust_surface returned success on the 'quoted-paths' fixture` | BITES |
| R10 | delete the parsed secret walk | exit 1 — `scan_trust_surface returned success on the 'tojson-secrets' fixture` | BITES |
| **R11** | **delete the `quoted-paths` fixture heredoc (WR-R5-01)** | **exit 0, `ALL PASSED`** | **VACUOUS — new gap** |
| **R12** | **delete the parsed path walk AND the quoted-paths + space-paths fixtures together** | **exit 0, `ALL PASSED`** | **VACUOUS — new gap (sharpest form: a security rule and its evidence vanish with a green banner)** |
| **R13** | **`GOTOOLCHAIN: local` → `auto` in ci.yml (WR-R5-02)** | **exit 0, `ALL PASSED`** | **UNPINNED — new gap** |
| **R14** | **delete the whole install-step `env:` block (WR-R5-02)** | **exit 0, `ALL PASSED`** | **UNPINNED — new gap** |
| G1 | regression: job-level `if:` on the `fe` job (round-3 D10) | exit 1 | no regression |
| G2 | regression: job-level `if:` on the `docker` job (round-3 D11) | exit 1 | no regression |
| G3 | regression: reintroduce `go test -list \| grep -qx` (round-4 M12) | exit 1 | no regression |
| G4 | regression: re-add the three-secret `env:` to `Gate check` (round-4 M14) | exit 1 | no regression |

Reading: every drill the round-5 plans claim is real — I reproduced all nine with the partition and the missing token named. The two review warnings are also real: I reproduced WR-R5-01 in two forms (R11, R12) and WR-R5-02 in two forms (R13, R14), and could not refute either. Per this verification's standard (a warning the verifier independently reproduces is a gap), both become gaps below.

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `verify-phase-48.sh` | 11 partitions, zero-waiver, per-assertion evidence tokens | ⚠️ PARTIAL | 15 per-assertion tokens (P1/P2/P3/P4) + 16 more (P6/P7/P9) verified by drill; `ok_partition` requires every token. Two residuals: the P11 fixture loop's single `rejected` token (gap 1) and the missing install-env assertion (gap 2). |
| `.github/workflows/ci.yml` | four jobs, gate-preserving, install surface declared | ✓ VERIFIED | go/fe/docker byte-identical to the green run's head; verify job gains the declared env + accurate comment; no weakening token, no secrets. The env declarations are unpinned (gap 2's artifact, not the artifact's failure). |
| `.github/workflows/eval-gate.yml` | gated live workflow, complement notice, secrets on the consumer only | ✓ VERIFIED | P9 passes; the M14 regression still reds. |
| `.github/actions/setup/action.yml` | checkout-free composite, three pinned majors | ✓ VERIFIED | P5 passes in the baseline run. |
| `.golangci.yml` | v2 schema, no baseline, revive positively assured | ✓ VERIFIED | P4 passes; sentinel D still attributes `revive`. |
| `web/src/components/chat/parts/Sparkline.test.tsx` | FE test that passes on a Linux runner | ✓ VERIFIED | Unchanged since the green run. |
| `Dockerfile.backend` | glibc backend image linking the static duckdb archive | ✓ VERIFIED | Unchanged since the green run; built green in run 39380373930. |
| `48-REVIEW-DISPOSITION-R1.md` | tail paragraph annotated historical | ✓ VERIFIED | Annotation line 62 precedes the retained paragraph (line 70); run id + Complete status present; file tracked. |
| `48-SECURITY.md` | T-48-70..79 once each, 31 rows, 0 open, verified | ✓ VERIFIED with one false clause | All ten IDs exactly once; 31 `T-48-` rows; `threats_open: 0`; `status: verified`; 31/31/0 audit-trail row naming plans 48-20..48-22; 2026-10-20 metric table 31/31/0; token-names historical marker. Exception: T-48-78's enforcement clause is false (gap 2). |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` debt markers exist in any deliverable — the debt-marker gate does not fire.

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `ci.yml` push to `main` | GitHub Actions run | `on.push.branches: [main]` | WIRED (observed) | Six runs, event `push`, branch `main`; remote `main` = run head. |
| `go`/`fe`/`docker` jobs | their gate steps | job definitions | WIRED (observed green) | Byte-identical to the four-green run's head; zero non-success steps. |
| `verify` job | `bash verify-phase-48.sh` | last step | WIRED (green at 630f875) | The current job definition (env + comment added) has not run publicly — record boundary stated above. |
| P2/P3 assertion groups | their own tokens → `p23.log` → deposit | 8 tokens | WIRED — drill-proven | R1/R2 red with the partition and missing tokens named. |
| P1/P4 groups | their tokens → `p1.log`/`p4.log` | 3 + 4 tokens | WIRED — drill-proven | R3/R4 red correctly; P4's concatenation is missing-file-tolerant. |
| P6/P7/P9 groups | their tokens → own logs | 4 + 6 + 6 tokens | WIRED — drill-proven | R5/R6/R7 red correctly. |
| P11 fixtures | scan_trust_surface rules | 12-fixture loop | PARTIALLY WIRED | Rule→fixture direction drill-proven (R9/R10); fixture→evidence direction unpinned (R11/R12 green — gap 1). |
| ci.yml install env | gate assertion | — | NOT WIRED | No partition reads the env map (gap 2). |
| run conclusions | `REQUIREMENTS.md` CICD statuses | blocked-branch rule | WIRED | Three `Complete (run 39380373930)` statuses match the corroborated run. |
| R1 tail | historical annotation | run id + Complete status | WIRED | Annotation precedes the retained text; verify-grade line-order property holds. |

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `REQUIREMENTS.md` CICD rows | run id `39380373930` | Actions API job conclusions | Yes — re-fetched this round | ✓ FLOWING |
| `verify-phase-48.sh` tokens | probe logs | each probe's own output | Yes — per assertion group now (32 tokens), drill-proven | ✓ FLOWING |
| P11 fixture loop | per-fixture rejections | scan_trust_surface rc only | Partly — the rc does not distinguish "rejected by a rule" from "found nothing" | ⚠️ PARTIAL (gap 1) |
| `48-SECURITY.md` T-48-78 | enforcement claim | no assertion exists | No — the claimed enforcement has no source | ✗ DISCONNECTED (gap 2) |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase gate green on the committed tree | `bash verify-phase-48.sh` | exit 0, one `ALL PASSED`, 11/11, 12 fixtures rejected | ✓ PASS |
| Public runs exist for this workflow | `gh run list --workflow ci.yml` | 6 runs: 5 failure, 1 success | ✓ PASS |
| Per-job conclusions of the green run | `gh api …/runs/39380373930/jobs` | four jobs `success`, zero non-success steps | ✓ PASS |
| Remote default branch | `gh api …/git/ref/heads/main` | `630f875…` (the green run's head) | ✓ PASS |
| go/fe/docker job immutability since the green run | `diff` of job spans `630f875`..HEAD | identical | ✓ PASS |
| Non-`.planning` commits after the green run | `git log 630f875..HEAD -- . ':(exclude).planning'` | 7 commits, touching only `verify-phase-48.sh` + `ci.yml` | ✓ PASS (boundary stated) |
| Assertion-level neutering (M13) | sandbox R1 | exit 1, P2 named | ✓ PASS (round-4 gap closed) |
| Fixture-deletion neutering (WR-R5-01) | sandbox R11/R12 | exit 0, `ALL PASSED` | ✗ FAIL (gap) |
| Install-env unpinned (WR-R5-02) | sandbox R13/R14 | exit 0, `ALL PASSED` | ✗ FAIL (gap) |

## Probe Execution

The phase's probes are the 11 partitions of `verify-phase-48.sh` plus the mutation drills above. The full gate was run once on the committed tree; every drill ran in its own sandbox against a mutated copy.

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| P1–P11 on the committed tree | `bash verify-phase-48.sh` | exit 0, one `ALL PASSED` | PASS |
| Nine per-assertion/walk/banner drills (R1–R10) | sandbox mutations | all exit 1 with partition + token/fixture named | PASS |
| Fixture-deletion drills (R11/R12) | sandbox mutations | exit 0, `ALL PASSED` | FAIL (gap 1) |
| Install-env drills (R13/R14) | sandbox mutations | exit 0, `ALL PASSED` | FAIL (gap 2) |
| Round-3/4 regression drills (G1–G4) | sandbox mutations | all exit 1 | PASS — no regression |

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| CICD-01 | 48-01, 03–23 (all plans declare it) | Go CI in GitHub Actions: golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet, PR/push triggered | ✓ SATISFIED | Run 39380373930 `go=success`; `go` job byte-identical since; `Complete (run 39380373930)` in both REQUIREMENTS places. |
| CICD-02 | 48-02, 04–23 | FE CI: `tsc --noEmit` + full `vitest run` | ✓ SATISFIED | Same run `fe=success`; `fe` job byte-identical since. |
| CICD-03 | 48-02, 04–23 | Both Dockerfiles build in CI | ✓ SATISFIED | Same run `docker=success`; `docker` job byte-identical since. |

**Cross-reference:** the union of every plan's frontmatter `requirements:` (checked for all 22 plans) is exactly `{CICD-01, CICD-02, CICD-03}` — the same set REQUIREMENTS.md maps to Phase 48 in both places. **No orphaned requirement, no unmapped ID.** `BL-02` remains a Deferred backlog item, not a Phase 48 requirement.

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `verify-phase-48.sh` | 1786-1814 | P11 fixture loop accepts any non-zero rc as a pin; one `rejected` token covers all 12 fixtures; deleting a fixture (or a rule + its fixtures) keeps exit 0 / ALL PASSED (R11/R12) | 🛑 Blocker | The evasion-shape evidence can silently disappear — the "found nothing, reported green" class in the loop that certifies the trust surface. |
| `.github/workflows/ci.yml` + `verify-phase-48.sh` | 242-245 / 444-499 | Install-step env declared but asserted by no partition; T-48-78's register row claims the pin exists (R13/R14) | 🛑 Blocker (record + pin) | The IN-01 hardening can be reverted invisibly, and the security record asserts an enforcement that does not exist. |
| `.planning/phases/48-ci/48-SECURITY.md` | 81 | T-48-78 mitigation cell: "re-narrowing or re-widening the declared env reds the gate" — falsified by reproduction | ⚠️ Warning | `status: closed` for T-48-78 rests partly on a false enforcement claim (register != code). |
| `.planning/phases/48-ci/48-REVIEW-DISPOSITION.md` | frontmatter | All 15 round-4 rows still `open`; `unparsed: 7` — the WR-R5-NN ids do not match the ledger's row schema | ℹ️ Info | Stale mechanical ledger; authoritative dispositions live elsewhere; round-5 findings have no disposition rows. |
| `verify-phase-48.sh` | 296, 1225/1267 | Dead code from the round-5 edits: unused `re` import; duplicate `basename` assignment | ℹ️ Info | Review IN-R5-05; harmless drift. |
| `verify-phase-48.sh` | 265-279 | P1 `pull_request_target` scan reads raw text (comment false-positive direction) | ℹ️ Info | Carried IN-03 / IN-R5-03. |
| `.github/workflows/ci.yml` | 41-45 | Push-trigger comment's whole-repo-lint justification does not follow from the trigger limit | ℹ️ Info | Carried IN-07 / IN-R5-04. |

## Manual-Only Verifications (acknowledged, unchanged)

Documented in `48-VALIDATION.md` as manual-only; none blocks CICD-01/02/03:

| # | Verification | Why manual | Current state |
|---|--------------|------------|---------------|
| M2 | A fork PR triggers the live `eval-gate` and the notice job is green | Requires an actual fork PR | Open (by design) |
| M3 | A Dependabot PR is treated like a fork | Requires an actual Dependabot PR | Open (by design) |
| M5 | Docker cache hit on the second green run for both images | Requires the next green `docker` run | Open — the first green docker job is run 39380373930 |

## Human Verification Required

None beyond the acknowledged manual-only rows above. Every truth in this report was decided by evidence obtained directly during this verification: public run ids and job/step conclusions via the Actions API, the local gate run on the committed tree, and fourteen independent sandbox mutations with asserted pre-mutation anchors.

## Gaps Summary

**The phase goal's public outcome remains achieved, and both round-4 gaps are genuinely closed.** The four-green run `39380373930` still stands on the public default branch, the go/fe/docker job definitions are byte-identical to its head, and CICD-01/02/03 are `Complete` on that run id in both REQUIREMENTS.md places. Truth 12's round-4 failed class is closed and I proved it by re-running the M13 mutation (exit 1, P2 named with all four tokens) plus eight sibling drills; truth 15's PARTIAL half is closed (the R1 ledger tail is annotated historical with the run id and Complete statuses, verified by line order). Score moves through 13/15 with a different composition than round 4: truth 15 flips to verified, truth 9 drops to partial.

**Two new must-have failures keep the phase at `gaps_found` — both are the round-5 review's warnings, independently reproduced with evidence rather than accepted on the review's word:**

1. **The P11 fixture loop folds fixture-deletion into PASS** (truth 12, new class — gap 1). The loop's only criteria are a non-zero `scan_trust_surface` exit, a count of 12, and the single `rejected` token; the scan also returns non-zero when it finds nothing, so deleting a fixture heredoc leaves an empty directory "rejected" by vacuity and the gate stays green (reproduced). The compound form deletes the parsed path-filter walk together with its two fixtures and still exits 0 / ALL PASSED (reproduced) — a security rule and its evidence vanish behind a green banner. The minimum fix is the review's: require the rejection reason per fixture and emit per-class tokens.
2. **The 48-22 install-surface declarations are pinned by nothing, and the register claims otherwise** (truth 9 partial — gap 2). Flipping `GOTOOLCHAIN` to `auto` and deleting the whole env block both keep exit 0 / ALL PASSED (reproduced twice), while 48-SECURITY.md's T-48-78 row asserts the declared env "reds the gate" when re-narrowed or re-widened. The minimum fix is the review's: a parsed assertion in the P3-VERIFYJOB group pinning the env map exactly, then correcting the T-48-78 cell.

Neither gap touches the public CI outcome or the requirement flips, but both are in the trust mechanism this phase exists to guarantee. The minimum path to `passed` is (1) per-fixture reason assertions + per-class tokens in the P11 loop (plus the compound drill), and (2) the install-env assertion plus the T-48-78 correction. The round-5 review's five info findings and the stale round-4 disposition ledger are recorded as advisory/non-blocking.

---

_Verified: 2026-10-20T16:04:09Z_
_Verifier: Claude (gsd-verifier)_
