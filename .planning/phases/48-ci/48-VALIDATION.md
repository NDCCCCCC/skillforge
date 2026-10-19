---
phase: "48"
slug: "ci"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-10-19"
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
| P1 all workflow YAML parses | — | — | N/A | static | `python -c "import yaml,glob;[yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yml')]"` | ❌ W0 | ⬜ pending |
| P2 no `pull_request_target` | D-11 | T-48-01 | fork code never runs with repo secrets | static | `! grep -rn 'pull_request_target' .github/workflows/` | ❌ W0 | ⬜ pending |
| P3 eval-gate `if` has all three gates | D-09 | T-48-02 | fork/Dependabot PR cannot reach live eval | static | `grep -c "head.repo.full_name == github.repository" .github/workflows/eval-gate.yml` | ❌ W0 | ⬜ pending |
| P4 skip-notice job exists with `::notice::` | D-10 | T-48-03 | a skipped job is never reported as PASS | static | `grep -c '::notice::' .github/workflows/eval-gate.yml` | ❌ W0 | ⬜ pending |
| P5 `ci.yml` has zero `secrets.*` beyond `GITHUB_TOKEN` | D-14 | T-48-04 | CI cannot leak or require a secret | static | `! grep -n 'secrets\.' .github/workflows/ci.yml \| grep -v GITHUB_TOKEN` | ❌ W0 | ⬜ pending |
| P6 `go-version-file: go.mod` everywhere, zero `go-version:` literal | D-17 | T-48-05 | one Go version source of truth | static | `! grep -rn "go-version:" .github/workflows/` | ❌ W0 | ⬜ pending |
| P7 `go` job has `CGO_ENABLED: "1"`, no live-service env | D-05/D-16 | T-48-06 | go job is hermetic | static | `grep -c 'CGO_ENABLED' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P8 Go commands match `verify-phase-47.sh` P1/P4/P5 | D-20/D-15/D-18 | — | local gate and CI gate say the same thing | static | `bash verify-phase-48.sh` (internal diff) | ❌ W0 | ⬜ pending |
| P9 shuffle step carries `-v` | D-19 | — | a failing seed is recoverable from CI logs | static | `grep -c 'shuffle=on.*-v\|-v.*shuffle=on' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P10 lint is blocking + pinned | D-04/D-06 | T-48-07 | lint cannot be silently skipped | static | `grep -c 'golangci-lint-action@v9' .github/workflows/ci.yml && ! grep -rn 'continue-on-error' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P11 `go` job grants `pull-requests: read`, no write scope | D-29 | T-48-08 | `only-new-issues` cannot silently degrade to whole-repo | static | `bash verify-phase-48.sh` (permissions partition) | ❌ W0 | ⬜ pending |
| P12 FE job runs `npm ci` + `tsc --noEmit` + `vitest run` | CICD-02 | — | type + test gates both fire | static | `grep -c 'npx vitest run' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P13 both Dockerfiles build, `push: false`, no `load: true` | CICD-03/D-24 | T-48-09 | CI never publishes an image | static | `grep -c 'push: false' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P14 cache scopes differ per image | D-23 | — | second image does not evict the first's cache | static | `grep -c 'scope=backend' .github/workflows/ci.yml && grep -c 'scope=frontend' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P15 zero registry surface | D-26 | T-48-10 | no publish path exists in CI | static | `! grep -rnE 'docker/login-action|registry:|GHCR|DOCKERHUB|packages: write' .github/workflows/` | ❌ W0 | ⬜ pending |
| P16 every permissions value is read/none | D-26/D-29 | T-48-08 | CI holds no write capability | static | `bash verify-phase-48.sh` (permissions partition) | ❌ W0 | ⬜ pending |
| P17 no `paths` / `paths-ignore` filters | D-24 | T-48-03 | a required check never sits Pending forever | static | `! grep -rn 'paths' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P18 composite action exists + callers checkout first | D-22/D-30 | — | local action resolves; toolchain versions stay single-sourced | static | `test -f .github/actions/setup/action.yml && grep -c 'uses: ./.github/actions/setup' .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| P19 `golangci-lint config verify` exit 0 | D-01/D-02 | — | the v2 config schema is real | static | `golangci-lint config verify --config .golangci.yml` | ❌ W0 | ⬜ pending |
| P20 sentinel unchecked `json.Unmarshal` exits non-zero naming `errcheck` | D-01/D-03 | — | the lint gate has teeth | sentinel | `bash verify-phase-48.sh` (sentinel partition) | ❌ W0 | ⬜ pending |
| P21 sentinel `defer f.Close()` exits 0; unused parameter not reported | D-03 | — | noise is absorbed, not waived by suppression hacks | sentinel | `bash verify-phase-48.sh` (sentinel partition) | ❌ W0 | ⬜ pending |
| P22 SC-4 old wording gone, new wording present | D-12 | — | docs match build reality | static | `! grep -q '既有 hermetic 先例' .planning/ROADMAP.md && grep -q '既有 \*\*live\*\* workflow' .planning/ROADMAP.md` | ❌ W0 | ⬜ pending |
| P23 verifier is honest: empty partition ⇒ NOT VERIFIED + non-zero | D-06/WR-06 | T-48-11 | an unrun check never folds into PASS | sentinel | `bash verify-phase-48.sh` (self-probe partition) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

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
| M1 | `go`, `fe`, `docker` jobs are all green on a real push | `gh run watch <run-id>` after pushing |
| M2 | fork PR triggers the approved `eval-gate` gate and the notice job is green | open a PR from a fork branch, read the check list |
| M3 | Dependabot PR is treated like a fork (no live eval) | read the Dependabot PR's check list |
| M4 | `only-new-issues` actually filters (lint output is scoped to changed lines, not the whole repo) | inspect the lint step's finding count on a PR touching one Go file |
| M5 | Docker cache hit on the second run for **both** images | compare `scope=backend` / `scope=frontend` cache entries in run 2 |

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
