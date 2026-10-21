---
phase: "48"
slug: "ci"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-10-19"
validated: "2026-10-19"
---

# Phase 48 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `48-RESEARCH.md` § Validation Architecture (23 concrete predicates).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (existing) + Vitest 2.x (existing) + golangci-lint v2.12.2 (new) + `verify-phase-48.sh` (new harness) + GitHub Actions |
| **Config file** | `web/vitest.config.ts`; `.golangci.yml` (new, Wave 0); no Go test config |
| **Quick run command** | `bash verify-phase-48.sh` |
| **Full suite command** | `wsl.exe -d Ubuntu-22.04 -- bash -c "cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./... -count=1"` and `cd web && npx tsc --noEmit && npx vitest run` |
| **Estimated runtime** | `verify-phase-48.sh` ~10s (static + sentinel) · Go `./...` ~146s · FE ~28s · CI Go job ~5–10 min (cold Docker build) |

---

## Sampling Rate

- **After every task commit:** `bash verify-phase-48.sh`
- **After every plan wave:** `verify-phase-48.sh` + the local Go `./...` and FE suite
- **Before `/gsd-verify-work`:** Full suite green **and** a real GitHub Actions run where `go`, `fe`, `docker` are all green
- **Max feedback latency:** ~15s for the quick gate

---

## Per-Task Verification Map

Task IDs are provisional — the planner assigns them. Predicates P1–P23 are the
authoritative list from `48-RESEARCH.md` § "Concrete verifiable predicates".

| Predicate | Req | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-----------|-----|------------|-----------------|-----------|-------------------|-------------|--------|
| P1 all workflow YAML parses | — | — | N/A | static | `python -c "import yaml,glob;[yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yml')]"` | ✅ | ✅ green |
| P2 no `pull_request_target` | D-11 | T-48-01 | fork code never runs with repo secrets | static | `! grep -rn 'pull_request_target' .github/workflows/` | ✅ | ✅ green |
| P3 eval-gate `if` has all three gates | D-09 | T-48-02 | fork/Dependabot PR cannot reach live eval | static | `grep -c "head.repo.full_name == github.repository" .github/workflows/eval-gate.yml` | ✅ | ✅ green |
| P4 skip-notice job exists with `::notice::` | D-10 | T-48-15 | a skipped job is never reported as PASS | static | `grep -c '::notice::' .github/workflows/eval-gate.yml` | ✅ | ✅ green |
| P5 `ci.yml` has zero `secrets.*` beyond `GITHUB_TOKEN` | D-14 | T-48-04 | CI cannot leak or require a secret | static | `! grep -n 'secrets\.' .github/workflows/ci.yml \| grep -v GITHUB_TOKEN` | ✅ | ✅ green |
| P6 `go-version-file: go.mod` everywhere, zero `go-version:` literal | D-17 | T-48-05 | one Go version source of truth | static | `! grep -rn "go-version:" .github/workflows/` | ✅ | ✅ green |
| P7 `go` job has `CGO_ENABLED: "1"`, no live-service env | D-05/D-16 | T-48-06 | go job is hermetic | static | `grep -c 'CGO_ENABLED' .github/workflows/ci.yml` | ✅ | ✅ green |
| P8 Go commands match `verify-phase-47.sh` P1/P4/P5 | D-20/D-15/D-18 | — | local gate and CI gate say the same thing | static | `bash verify-phase-48.sh` (internal diff) | ✅ | ✅ green |
| P9 shuffle step carries `-v` | D-19 | — | a failing seed is recoverable from CI logs | static | `grep -c 'shuffle=on.*-v\|-v.*shuffle=on' .github/workflows/ci.yml` | ✅ | ✅ green |
| P10 lint is blocking + pinned | D-04/D-06 | T-48-07 | lint cannot be silently skipped | static | `grep -c 'golangci-lint-action@v9' .github/workflows/ci.yml && ! grep -rn 'continue-on-error' .github/workflows/ci.yml` | ✅ | ✅ green |
| P11 `go` job grants `pull-requests: read`, no write scope | D-29 | T-48-08 | `only-new-issues` cannot silently degrade to whole-repo | static | `bash verify-phase-48.sh` (permissions partition) | ✅ | ✅ green |
| P12 FE job runs `npm ci` + `tsc --noEmit` + `vitest run` | CICD-02 | — | type + test gates both fire | static | `grep -c 'npx vitest run' .github/workflows/ci.yml` | ✅ | ✅ green |
| P13 both Dockerfiles build, `push: false`, no `load: true` | CICD-03/D-24 | T-48-09 | CI never publishes an image | static | `grep -c 'push: false' .github/workflows/ci.yml` | ✅ | ✅ green |
| P14 cache scopes differ per image | D-23 | — | second image does not evict the first's cache | static | `grep -c 'scope=backend' .github/workflows/ci.yml && grep -c 'scope=frontend' .github/workflows/ci.yml` | ✅ | ✅ green |
| P15 zero registry surface | D-26 | T-48-10 | no publish path exists in CI | static | `! grep -rnE 'docker/login-action|registry:|GHCR|DOCKERHUB|packages: write' .github/workflows/` | ✅ | ✅ green |
| P16 every permissions value is read/none | D-26/D-29 | T-48-08 | CI holds no write capability | static | `bash verify-phase-48.sh` (permissions partition) | ✅ | ✅ green |
| P17 no `paths` / `paths-ignore` filters | D-24 | T-48-03 | a required check never sits Pending forever | static | `! grep -rn 'paths' .github/workflows/ci.yml` | ✅ | ✅ green |
| P18 composite action exists + callers checkout first | D-22/D-30 | — | local action resolves; toolchain versions stay single-sourced | static | `test -f .github/actions/setup/action.yml && grep -c 'uses: ./.github/actions/setup' .github/workflows/ci.yml` | ✅ | ✅ green |
| P19 `golangci-lint config verify` exit 0 | D-01/D-02 | — | the v2 config schema is real | static | `golangci-lint config verify --config .golangci.yml` | ✅ | ✅ green |
| P20 sentinel unchecked `json.Unmarshal` exits non-zero naming `errcheck` | D-01/D-03 | — | the lint gate has teeth | sentinel | `bash verify-phase-48.sh` (sentinel partition) | ✅ | ✅ green |
| P21 sentinel `defer f.Close()` exits 0; unused parameter not reported | D-03 | — | noise is absorbed, not waived by suppression hacks | sentinel | `bash verify-phase-48.sh` (sentinel partition) | ✅ | ✅ green |
| P22 SC-4 old wording gone, new wording present | D-12 | — | docs match build reality | static | `! grep -q '既有 hermetic 先例' .planning/ROADMAP.md && grep -q '既有 \*\*live\*\* workflow' .planning/ROADMAP.md` | ✅ | ✅ green |
| P23 verifier is honest: empty partition ⇒ NOT VERIFIED + non-zero | D-06/WR-06 | T-48-11 | an unrun check never folds into PASS | sentinel | `bash verify-phase-48.sh` (self-probe partition) | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Nyquist audit 2026-10-19 — every predicate has automated verification; none MISSING.**
`verify-phase-48.sh` runs 11 partitions covering all 23 predicates (P1→part.1, P2/P8/P9→part.2,
P10/P11→part.3, P19/P20/P21→part.4, P18→part.5, P12→part.6, P13/P14→part.7, P5/P15/P16/P17→part.8,
P3/P4/P6→part.9, P22→part.10, P23→part.11). All 11 report PASS on a clean HEAD, so the ✅ marks
above are observed results, not projections. **No test files were generated**: this phase adds no
runtime Go or FE code — its verification surface *is* the shell gate plus the three CI jobs — so
the Nyquist gap axis (a requirement with no automated check) is empty by construction rather than
by omission. No requirement is PARTIAL either.

**What ✅ does NOT mean here (read before closing CICD-01/02/03):** the green marks are
*static and sentinel* proofs. P7/P13 prove the `docker` job is declared with `push: false`, both
Dockerfiles referenced and per-image cache scopes — **the images were built by the runner, not by
this host**: Docker is absent locally. P12 proves the `fe` job *declares* `npm ci` /
`tsc --noEmit` / `vitest run`; the suites themselves were run separately and passed (Go
build+test exit 0; Vitest 85 files / 689 tests exit 0). **A real GitHub Actions runner has
executed `ci.yml` six times** (recorded by plans 48-15 and 48-19 in `48-15-SUMMARY.md` and
`48-19-SUMMARY.md`; event `push` to `main`). The earlier runs (runs 1-5) were all red. **Run 6
of 6 — run id 39380373930 on head `630f875` — is the first all-green**, with
`go=fe=docker=verify=success`. Canonical fragment: `ci.yml total runs: 6; first all-green:
39380373930`. The earlier runs exposed the three classified build causes fixed by 48-16 (P5a
workflow pipeline shape, Sparkline Windows path, alpine-musl duckdb linkage — see M1a-M1c) and
one load-sensitive DuckDB reader-block ratio flake fixed by 48-19 (best-of-5 minimum sampling).
M1 is closed on run 6 of 6. M2-M5 remain open: a push run alone cannot exercise a fork PR, a
Dependabot PR, or a second-run cache comparison. **Round-6 update (2026-10-21, plan 48-26):** a
seventh run exists — run `39525757252` on head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67` (the
round-6 tree, fast-forward push `630f875..c060c84`, no force) — and is four-green, the **first
public run of the round-6 gate** (48-23's attributing fixture loop + install-env pin, 48-24's
config-verify evidence binding + comment-stripped trigger scan, 48-25's record corrections): the
round-6 gate script and its `verify` job have now executed publicly, not only locally. go/fe/docker
job spans are byte-identical between `630f875` and `c060c84`, so CICD-01/02/03 stay Complete
against run 39380373930 without re-flipping. Canonical fragment: `ci.yml total runs: 7; first
all-green: 39380373930; run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252,
event push) is the first public run of the round-6 gate (four-green: go=success fe=success
docker=success verify=success)`. See M6 below.

### Round-7 strengthening (2026-10-21, plan 48-27)

No predicate row was added and none was retired. Round 7 did not change what CI must
satisfy — it changed how the local gate PROVES three of the rows above, so those rows get
stronger evidence instead of new rows:

| Row | What round 7 added | Mutation evidence |
|-----|--------------------|-------------------|
| P1 (workflow parse / D-11 trigger) | trigger judgment moved to the PARSED `on:` document (mapping key, sequence element, bare scalar — read as `doc.get("on", doc.get(True))`, because PyYAML 1.1 loads a bare `on:` as the boolean `True`), and both comment strips now share one quote-aware helper, so a `#` inside a quoted scalar can no longer hide a real `pull_request_target`; four canaries + `P1-NO-PRT-PARSED` | R15 / R15b / R16 / R17 / D1-new / D2 — each red names its canary or its missing token; D1-old shows the pre-round-7 script green on the same file |
| P11 / P16 (permission rules, D-29 ceiling) | one isolating fixture and one class token per rule (`perm-write-line` / `perm-job-write` / `perm-job-readall` / `yaml-ext`), fixtures 14→17, class tokens 5→8 | R18 reds naming `perm-write-line`, R19 naming `perm-job-write`, R19b naming `perm-job-readall`, EXT naming `yaml-ext` |
| P19 (`golangci-lint config verify` exit 0) | the config-verify evidence line reports the tool's measured output size, sampled before any append, instead of describing itself | R20 keeps the gate green while reporting `tool output: 0 byte(s)`; R20b shows the pre-round-7 wording reporting `present` for that same 0-byte run |

The 37-drill matrix in `48-27-SUMMARY.md` is the adversarial evidence that these assertions
bite. It exercises the gate script rather than a new test target, so the quick-run command
for every row above remains `bash verify-phase-48.sh`.

---

## Wave 0 Requirements

- [ ] `.github/workflows/ci.yml` — CICD-01 / CICD-02 / CICD-03 (P1–P18)
- [ ] `.github/actions/setup/action.yml` — D-22 / D-30 (P18)
- [ ] `.golangci.yml` — D-01 / D-02 / D-03 / D-08 (P19)
- [ ] `verify-phase-48.sh` — SC-5 + D-06 / D-09 / D-11 / D-14 / D-20 / D-26 (P2–P5, P8, P11, P16, P20, P21, P23)
- [ ] `.github/workflows/eval-gate.yml` edits — D-09 / D-10 / D-17 (P3, P4, P6)
- [ ] `.planning/ROADMAP.md` SC-4 wording — D-12 (P22)
- [ ] No framework install needed: existing Go and Vitest suites already cover CICD-01/02's subject matter.
- [ ] **CI-run-only proofs** (cannot close locally — the first real CI run is the proof): CICD-01a/b/c/d, CICD-02, CICD-03; the fork-PR / Dependabot `eval-gate` skip observation.

---

## Manual-Only Verifications

| # | What | How |
|---|------|-----|
| M1 | `go`, `fe`, `docker` jobs are all green on a real push | **closed** on run 39380373930 (head `630f875`): `go=success`, `fe=success`, `docker=success` (and `verify=success`); canonical fragment `ci.yml total runs: 6; first all-green: 39380373930`. The earlier runs (runs 1-5) were red; see M1a-M1c for the three classified build causes fixed by 48-16 and `48-19-SUMMARY.md` for the reader-block flake fixed between run 5 and run 6 |
| M1a | `fe` job's `Sparkline.test.tsx:79` `TestSparklineNoDeps` passes on a Linux runner | **closed** — fixed by 48-16 Task 2 (resolve `Sparkline.tsx` via `import.meta.url` instead of a hardcoded Windows path); green on run 39380373930 |
| M1b | `docker` job's backend image build links against the static duckdb archive on the runner | **closed** — fixed by 48-16 Task 3 (`golang:1.25-bookworm` builder, `debian:bookworm-slim` runtime with libstdc++6 / libgcc-s1); both images built on run 39380373930 |
| M1c | `go` job's `determinism -count=100 (P5a)` guard is green on the runner | **closed** — fixed by 48-16 Task 1 (materialise the `-list` output to `$RUNNER_TEMP/go-test-list.txt` before grep); green on run 39380373930. Root-cause analysis retained: **workflow pipeline shape, not runner environment** — `go test … -list … \| grep -qx …` under `set -o pipefail`: `grep -q` exits at the first match and closes the pipe, `go test -list` still writes its trailing `ok <pkg> <time>` line, the producer takes EPIPE → SIGPIPE → exit 141, and pipefail propagates it. Reproduced on WSL Ubuntu 22.04 with go1.25.5 linux/amd64 (the version `go-version-file: go.mod` installs): exit 141 with the exact `ci.yml` line (6/6), exit 0 once the list is materialised or `grep -q` is removed. The same line exits 0 on this Windows host, which is why 48-12's local check missed it. Remedy actually applied in 48-16: materialise the list to `$RUNNER_TEMP/go-test-list.txt` before grep. The former prescription (runner-side capture of step output, aborting early on output-buffer close, or adding `-v`) is withdrawn — it cannot fix a pipeline in the workflow. Recorded in 48-15-SUMMARY.md and re-proven by 48-VERIFICATION.md round 3 |
| M2 | fork PR triggers the approved `eval-gate` gate and the notice job is green | open a PR from a fork branch, read the check list |
| M3 | Dependabot PR is treated like a fork (no live eval) | read the Dependabot PR's check list |
| M4 | `only-new-issues` actually filters (lint output is scoped to changed lines, not the whole repo) | closed-by-evidence on plan 48-15 run-3: lint step reported zero findings on the fast-forward push (head `a2a4c86`); the diff between `62b6663..a2a4c86` is `.github/workflows/ci.yml` revert + `.planning/phases/48-ci/48-15-SUMMARY.md` line additions — no Go content — confirming the action's `only-new-issues: true` alone is sufficient |
| M5 | Docker cache hit on the second run for **both** images | still open: compare `scope=backend` / `scope=frontend` cache entries across two consecutive green `docker` runs; the first green docker job is run 39380373930, so the comparison needs the next green run |
| M6 | the round-6 gate script (P11 fixture-loop attribution + `P3-INSTENV-OK` install-env pin from 48-23, `P4-CONFIG-OK` evidence binding + comment-stripped trigger scan from 48-24, 48-25's record corrections) and the `verify` job that runs it have executed on a public runner | **closed** (2026-10-21, plan 48-26) — run 7 of 7: id `39525757252`, head `c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67`, event `push`, fast-forward push `630f875..c060c84` (no force), 2026-10-21T01:10:28Z→01:13:50Z: `verify=success` alongside `go=success`, `fe=success`, `docker=success`; derived total 7 read live from `gh run list`; canonical fragment `ci.yml total runs: 7; first all-green: 39380373930; run 7 (head c060c84a0a9e2a73fbf5134adbbf2b7d45eefc67, id 39525757252, event push) is the first public run of the round-6 gate (four-green: go=success fe=success docker=success verify=success)` — this closes the round-5 local-green-only record boundary |
| M7 | the **round-7** gate script (parsed `on:` judge + shared quote-aware stripper from 48-27 Task 1, the three per-rule permission fixtures from Task 2, the measured config-verify evidence line from Task 3) has executed on a public runner | **open** — round 7 changed only `verify-phase-48.sh`; `git diff --stat fd6f082..HEAD` touches no `.github/**`, no `.golangci.yml` and no Dockerfile, so CICD-01/02/03 stay Complete against run 7 and no requirement flips. What is still unproven is that the round-7 assertions pass on a Linux runner: locally they are green plus a 37-drill matrix, but the parsed judge depends on the runner's PyYAML and `strip_yaml_comments` on its quoting, so the first green `verify` job on a pushed round-7 head closes this. `grep -cF 'P1-NO-PRT-PARSED' .github/workflows/ci.yml` is irrelevant here — the token lives in the script, not the workflow; read it from the `verify` job's log |

---

## Feedback Loop

| Signal | Action |
|--------|--------|
| `verify-phase-48.sh` exits non-zero | Fix before commit. A red quick gate is never waived (zero-waiver standard inherited from Phase 47). |
| `verify-phase-48.sh` prints `LINT NOT VERIFIED` | Environment problem, not a pass. Install `golangci-lint v2.12.2` or record the skip explicitly. |
| CI `go` job red on a pre-existing flake | Run the deterministic-strength commands locally with `-count=100` + `-shuffle=on -v` before touching the gate. |
| CI `docker` job red on cold build only | Expected for `golang:1.25-alpine` floating tag (D-28). Re-run to distinguish flake from real break. |

---

## Blocked / Environment-Limited

| Item | Blocker | Unblocked by |
|------|---------|--------------|
| CICD-03 local proof | Docker absent on this host | first real CI run |
| M2 / M3 | requires an actual fork / Dependabot PR | contributor PR |
| golangci-lint sentinels (P20/P21) | needs `golangci-lint` v2.12.2 on PATH; verifier must print `LINT NOT VERIFIED` rather than PASS when absent | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` |

---

## Notes

- **Do not hard-code test counts.** ROADMAP line 113 says "666 Go 测试 + 84 FE 测试"; the measured 2026-10-19 state is 85 FE test files / 689 tests and `go test ./...` → 47 packages ok / 8 no-test / 0 FAIL. D-12 mandates only the SC-4 hermetic-wording correction (P22); count corrections are out of scope and must not expand it.
- **`-shuffle=on` seed** only prints with `-v` (measured). P9 exists so a red CI run is reproducible.
- **`type=gha` does not preserve `RUN --mount=type=cache`** (D-27). Expect lower cache value than a naive reading of D-23 suggests; not a gate failure.

---

*Phase: 48-CI 矩阵*
*Seeded from 48-RESEARCH.md § Validation Architecture, 2026-10-19*

## Validation Audit 2026-10-19

| Metric | Count |
|---|---|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 5 |

## Validation Audit 2026-10-20

| Metric | Count |
|---|---|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 3 |

## Validation Audit 2026-10-21

| Metric | Count |
|---|---|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 1 |
