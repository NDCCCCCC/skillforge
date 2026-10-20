---
phase: 48
review: 48-REVIEW.md
reviewed: 2026-10-19
disposition_by: gsd execute-phase → code-review gate (advisory); final dispositions by plan 48-10
disposition_completed: 2026-10-19
---

# Phase 48 — Code Review Disposition

> Per-finding record written by the `code_review_gate` step, closed out by gap-closure plan 48-10.
> Every row now carries a final disposition: `fixed — <plan> Task <n>` (with the assertion or
> mutation probe that proves it) or `deferred — <rationale + owner>`. Nothing was waived, and no
> finding was silently dropped.

| Severity | Count |
|----------|-------|
| Critical | 1 |
| Warning  | 8 |
| Info     | 10 |
| **Total**| **19** |
| Fixed    | 18 |
| Deferred | 1 |

---

## Disposition table

| ID | Severity | One-line claim | Disposition | Notes |
|----|----------|----------------|-------------|-------|
| CR-01 | Critical | P8's path-filter and secret rules run only against a file named `ci.yml` while the partition reports directory-wide coverage — a later workflow with `paths:`/`secrets.`/`docker push` returns `TRUST-SURFACE-OK` | fixed — 48-05 T1/T2: the `paths:`/`paths-ignore:`, secret and publication-token rules all moved into the per-file loop of `scan_trust_surface` over `*.yml` + `*.yaml`; `zz-p8-paths.yml` (`paths: ["internal/**"]`) and `zz-p8-secret.yml` (`secrets.DEPLOY_KEY`) each fail the gate | Re-closes `T-48-03`, `T-48-04` and `T-48-10` in `48-SECURITY.md`; the P11 half-2 fixtures (`two-files`, `yaml-ext`, `lowercase-registry`, `stray-secret`) re-prove it through the shared function (48-05 T3). |
| WR-01 | Warning | P2/P3 reject a job-level `if:` but not a step-level one; `if: false` on the gate steps leaves both partitions green | fixed — 48-07 T1/T2: P2/P3 reject any step-level `if` key on the `go` job's steps (naming step and condition, no allowlist) and P6/P7 carry the identical rule for the `fe`/`docker` jobs; injected `if: false` on a gate step reds the gate | Key-presence rejection, not value matching — `always()`/`success()`/negations decide execution too. Re-closes `T-48-07`. |
| WR-02 | Warning | `eval-gate-skip-notice` is not the exact complement of the gate; a Dependabot push to `main` skips the live gate with no notice | fixed — 48-08 T2: `eval-gate-skip-notice.if` is now the verbatim negation of the gate expression, and P9 asserts the relationship `notice == "!(" + gate + ")"` structurally | The previously-skipped region (Dependabot-attributed push) is now announced with `::notice::`. Re-closes `T-48-15`. |
| WR-03 | Warning | The live replay step pipes `./bin/eval` through `tee`, dropping its exit code under `bash -e` | fixed — 48-06 T1: the replay step declares `shell: bash` and begins its run body with `set -o pipefail` before the unchanged pipeline | Mechanism probe: `( exit 3 ) \| tee` → rc=3 under pipefail, rc=0 without. Re-closes the Repudiation half of the SC-4 fake-green gap (T-48-26). |
| WR-04 | Warning | No positive `revive` sentinel, so deleting `enable: [revive]` passes `config verify` and leaves P4 green | fixed — 48-09 T1: P4's sentinel D requires a `package-comments` violation (reported only by `revive`) in a two-finding fixture, so removing `linters.enable`/`settings` reds the gate with the failure naming `revive` | The second (errcheck) finding keeps the log non-empty so the branch that fires is guaranteed to be the one naming revive. Re-closes `T-48-11`'s revive arm. |
| WR-05 | Warning | `find -name '*.yml'` misses `.yaml` workflows, so a `.yaml` workflow escapes P1 and P8 entirely | fixed — 48-05 T1/T2: P1 and P8 enumerate both extensions everywhere a workflow file is counted or parsed; an invalid `.yaml` fails P1 by name and `zz-p8-registry.yaml` fails P8 | The verifier refuted the sub-claim that `.yaml` escapes P1's `pull_request_target` scan entirely (it is `grep -rn` over the directory); the real gap — the parse loop and P8 enumeration — is closed anyway. |
| WR-06 | Warning | On `push` to `main` the lint step has no PR diff; measured, this config exits 1 with 45 findings | fixed — 48-15 T2 evidence (closed on run-3 head `a2a4c86`, fast-forward push with the recipe-revert commit restoring the pre-plan lint step `version: v2.12.2` + `only-new-issues: true` and no manual `--new*` args). Run-1 (force-push with empty merge-base) reproduced the documented whole-repo fallback (11+ legacy findings); run-2 tested the recipe's candidate A (`args: --new-from-rev` on the lint step) and the action rejected it as incompatible with `only-new-issues: true` ("please, don't specify manually --new* args when requesting only new issues"); run-3 confirmed the action's `only-new-issues: true` alone, on a fast-forward push, produces a green lint check. Candidate B (PR-gating the lint step) remains D-06-reversing and was not chosen. No lint-step code change was applied to production; cumulative workflow diff vs the pre-plan BASE_SHA = 0. | Lint step verified green on run-3; recipe's "if green, record that the action did not fall back to whole-repo lint" branch hit. Also recorded in `48-SECURITY.md` § WR-06 paragraph and `48-15-SUMMARY.md` § Task 2 Run-3 findings. |
| WR-07 | Warning | P9's three fork-gate clauses are asserted as independent substrings, so appending `\|\| true` still yields `EVALGATE-OK` | fixed — 48-08 T1: P9 compares the whitespace-normalized `eval-gate.if:` to the exact D-09 literal with no substring fallback; `|| true`, an outer negation, and clause removal each red the gate printing both expressions | Re-closes `T-48-02` and `T-48-18`. |
| WR-08 | Warning | The three live secrets are job-level, so every step — including mutable-major-tag third-party actions — receives them | fixed — 48-06 T2: `GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` moved to step-level env on exactly the two consuming steps (frozen-replay and gate check); job-level env carries configuration only | Re-closes `T-48-27`; all eight non-secret job-env keys retained. |
| IN-01 | Info | P4's sentinel-A evidence check is a bare `grep -q 'errcheck'` | fixed — 48-09 T2: the evidence check now requires the linter-attribution string `is not checked (errcheck)`, so an incidental occurrence of the word cannot satisfy the sentinel | Re-closes `T-48-38`. |
| IN-02 | Info | P8's raw-substring rules produce false failures and a case-sensitive hole (`ghcr.io` slips past `"GHCR"`) | fixed — 48-05 T1: publication tokens are matched case-insensitively against comment-stripped text and the raw `"write" in src` scan was replaced by a value-shaped, line-anchored check | `ghcr.io` now bites (the `lowercase-registry` fixture proves it) and prose like "this workflow writes the PR comment" no longer false-fires. |
| IN-03 | Info | P8 rejects the valid `read-all` shorthand and silently skips job-level string permissions | fixed — 48-05 T1: workflow- and job-level string permissions are judged instead of skipped — `none` accepted, `write-all` and `read-all` rejected | `read-all` stays rejected deliberately: D-29 locks the permission ceiling at exactly `{contents: read, pull-requests: read}`, so accepting it would widen the verifier beyond the recorded decision. The silent-skip defect is closed without widening that ceiling. |
| IN-04 | Info | The FE count-literal token `"84 "` can match unrelated text | fixed — 48-07 T3: the token loop is replaced by an anchored, case-insensitive regex requiring the gap between count word and number, covering both orders (`689 tests`, `Tests  84 passed`) plus `\b666\b`/`\b689\b` | `8084`/`1984` and the file's own step names do not match; an injected `689 tests` comment does. The loose `[^\\n]{0,40}` form was rejected because it reds the untouched baseline. |
| IN-05 | Info | P2's hermeticity assertion inspects only the job-level `env:` map, not step-level `env` | fixed — 48-07 T3: P2 computes the union of the job-level env map and every step-level env map and requires exactly `{CGO_ENABLED}` | Injected `PROBE_EXTRA_VAR` on a step fails P2 naming the key. Re-closes `T-48-30`. |
| IN-06 | Info | P11's registry check counts textual call sites, not executed partitions | fixed — 48-09 T3: each partition body writes its own begin marker as its first statement; `done_partition` refuses without it, and P11 half 1a requires registry == call sites == begin markers == 11 | A commented-out body with a live `done_partition` call now fails naming the partition and "did not run". Re-closes `T-48-37` and `T-48-20`'s accounting arm. |
| IN-07 | Info | `verify-phase-48.sh` is not wired into any workflow, so its assertions can rot silently | fixed — 48-11 T2 (IN-07 option A, implemented): a fourth published `verify` job checks out, calls the composite with `go: "true"`, installs pinned golangci-lint v2.12.2 from the official script, and runs `bash verify-phase-48.sh` — no `continue-on-error`, no step-level `if:`, no widened permissions | Verifier drift is now a red check on every push/PR instead of a maintainer-memory step. |
| IN-08 | Info | The `pull_request.branches` filter leaves the same "Pending forever" hole the file declares an invariant against | fixed — 48-11 T1 (IN-08 option F, implemented): the closed PR base set `[main, 'phase-*']` is documented in the `ci.yml` header next to invariant 1; the trigger block is unchanged | Option E (dropping the filter) was the more-inclusive alternative and was not chosen. Widening the set is now a documented header+trigger decision. |
| IN-09 | Info | The `std-error-handling` preset absorbs more than its stated rationale (also silences `os.Remove`) | fixed — 48-09 T1: the `.golangci.yml` preset rationale now names `os.Remove` and the std-lib cleanup scope (deferred `Close`, `fmt.Fprint*`, …) | Comment-only change; `golangci-lint config verify` rc=0 after. Re-closes `T-48-39`. |
| IN-10 | Info | The job that executes fork-controlled code does not disable credential persistence in checkout | fixed — 48-07 T3: the `go` job's `actions/checkout@v7` sets `persist-credentials: false`, and P3 asserts the parsed value is exactly `False` | Scoped to the one job carrying a non-read-only permission surface (`pull-requests: read`); `fe`/`docker` remain on the default. Re-closes `T-48-31`. |

---

## Deferred findings

Exactly one of the 19 findings is deferred; everything else is fixed with a named assertion or
mutation probe above.

| ID | What is missing | Rationale | Owner |
|----|-----------------|-----------|-------|
| WR-06 | The push-path lint result: whether `golangci-lint-action@v9` falls back to whole-repo linting on a `push` to `main` (the review measured 45 findings under a whole-repo run of this tree, but cannot execute the action on a push event) | Closed on 2026-10-20 by plan 48-15 T2 evidence: run-1 reproduced the whole-repo fallback (force-push with empty merge-base, 11+ findings); run-2 tested candidate A (`args: --new-from-rev`) and the action rejected it as incompatible with `only-new-issues: true`; run-3 (fast-forward push with pre-plan lint step restored) confirmed `only-new-issues: true` alone is sufficient on a normal push — zero findings reported. Candidate B (PR-gating the lint step) was explicitly rejected as D-06-reversing and was not chosen. No lint-step code change was applied to production. | Run-1 (head `d168512`, id 39350901689); run-2 (head `62b6663`, id 39352480525); run-3 (head `a2a4c86`, id 39353593172). See `48-15-SUMMARY.md` § Task 2; row updated to `fixed`. |

Historical note (superseded 2026-10-20): the paragraph below records the state as it stood on
2026-10-19 and is superseded on 2026-10-20 — git transport to github.com succeeded, six `ci.yml`
runs exist on public `main`, run `39380373930` (head `630f875`) is four-green with
`go=fe=docker=verify=success`, and CICD-01/02/03 are `Complete (run 39380373930)` in both
`REQUIREMENTS.md` places. The text is retained as history, not as a current claim
(48-22 Task 3; see also `48-REVIEW-DISPOSITION-R2.md` for the closure convention).

**Superseded text (2026-10-19):** Not a review finding, but recorded here so nothing lives only in a plan's prose: **CICD-01/02/03
remain Pending.** `48-11-SUMMARY.md` records `M1: BLOCKED` — git transport to github.com is
unreachable from this host and `origin/main` shares no history with this branch, so no `ci.yml`
run can exist yet. The three requirements advance only against a real run with
`go`/`fe`/`docker` (and `verify`) all `success`.

---

## What this changes about the phase's security record

`48-SECURITY.md` was first written at ASVS **L1** (grep depth) and closed all 20 threats. CR-01,
WR-05, WR-07 and WR-01 were **mutation-proven** — the reviewer broke the input and watched the
partition stay green — and `48-VERIFICATION.md` independently reproduced them. The verification
marked the affected rows **open** (`threats_open: 9`) and the phase `gaps_found`.

Gap-closure plans 48-05…48-09 then hardened every falsified assertion (per-file trust-surface
scan, both-extension enumeration, step-level `if:` rejection, normalized fork-gate equality,
complement notice, positive `revive` sentinel, begin-marker accounting) and 48-11 implemented the
two decision findings (IN-07 option A, IN-08 option F) while deferring WR-06 as option D. Plan
48-10 re-closed the nine affected rows on the new mutation evidence; `threats_open: 0` and
`status: verified` now match the register row-for-row.

**Nothing was waived.** No finding was dismissed as out-of-scope, and no gate was relaxed to make
the review come out clean. `.golangci.yml` itself reviewed clean (preset/narrow-rule exclusions, no
finding allowlist).
