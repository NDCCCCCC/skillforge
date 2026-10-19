---
phase: 48
review: 48-REVIEW.md
reviewed: 2026-10-19
disposition_by: gsd execute-phase → code-review gate (advisory)
---

# Phase 48 — Code Review Disposition

> Per-finding record written by the `code_review_gate` step. Advisory throughout: the gate never
> blocked execution. Every row defaults to `open`; nothing here has been fixed, and nothing here
> was silently dropped.

| Severity | Count |
|----------|-------|
| Critical | 1 |
| Warning  | 8 |
| Info     | 10 |
| **Total**| **19** |

---

## Disposition table

| ID | Severity | One-line claim | Disposition | Notes |
|----|----------|----------------|-------------|-------|
| CR-01 | Critical | P8's path-filter and secret rules run only against a file named `ci.yml` while the partition reports directory-wide coverage — a later workflow with `paths:`/`secrets.`/`docker push` returns `TRUST-SURFACE-OK` | **open — contradicts a `closed` row** | Directly falsifies the `closed` status of `T-48-03`, `T-48-04` and `T-48-10` in `48-SECURITY.md`. The reviewer demonstrated it with a fixture, not by reading alone. Highest-priority follow-up. |
| WR-01 | Warning | P2/P3 reject a job-level `if:` but not a step-level one; `if: false` on the gate steps leaves both partitions green | open | Same class as CR-01 (assertion narrower than its stated claim). |
| WR-02 | Warning | `eval-gate-skip-notice` is not the exact complement of the gate; a Dependabot push to `main` skips the live gate with no notice | open | Violates locked decision D-10 and the `::notice::` guarantee asserted by P9/P4. |
| WR-03 | Warning | The live replay step pipes `./bin/eval` through `tee`, dropping its exit code under `bash -e` | open | A gate that cannot go red. Concrete failure path cited (`cmd/eval/main.go:648` writes a report, `:653` then `log.Fatalf`). |
| WR-04 | Warning | No positive `revive` sentinel, so deleting `enable: [revive]` passes `config verify` and leaves P4 green | open | Vacuity in the lint-teeth partition — the class the phase was specifically built to prevent (T-48-11). |
| WR-05 | Warning | `find -name '*.yml'` misses `.yaml` workflows, so a `.yaml` workflow escapes P1 and P8 entirely | open | Same fail-open shape as CR-01. |
| WR-06 | Warning | On `push` to `main` the lint step has no PR diff; measured, this config exits 1 with 45 findings | open | Either the workflow comment is wrong or `main` is permanently red. Needs a decision, not a code tweak. |
| WR-07 | Warning | P9's three fork-gate clauses are asserted as independent substrings, so appending `\|\| true` still yields `EVALGATE-OK` | open | Falsifies `T-48-02`/`T-48-18` closure for the `\|\| true` variant. |
| WR-08 | Warning | The three live secrets are job-level, so every step — including mutable-major-tag third-party actions — receives them | open | Pre-existing `eval-gate.yml` surface that this phase hardened but did not narrow. |
| IN-01 | Info | P4's sentinel-A evidence check is a bare `grep -q 'errcheck'` | open | — |
| IN-02 | Info | P8's raw-substring rules produce false failures and a case-sensitive hole (`ghcr.io` slips past `"GHCR"`) | open | — |
| IN-03 | Info | P8 rejects the valid `read-all` shorthand and silently skips job-level string permissions | open | — |
| IN-04 | Info | The FE count-literal token `"84 "` can match unrelated text | open | — |
| IN-05 | Info | P2's hermeticity assertion inspects only the job-level `env:` map, not step-level `env` | open | — |
| IN-06 | Info | P11's registry check counts textual call sites, not executed partitions | open | — |
| IN-07 | Info | `verify-phase-48.sh` is not wired into any workflow, so its assertions can rot silently | open | — |
| IN-08 | Info | The `pull_request.branches` filter leaves the same "Pending forever" hole the file declares an invariant against | open | — |
| IN-09 | Info | The `std-error-handling` preset absorbs more than its stated rationale (also silences `os.Remove`) | open | — |
| IN-10 | Info | The job that executes fork-controlled code does not disable credential persistence in checkout | open | — |

---

## What this changes about the phase's security record

`48-SECURITY.md` was written earlier in this run at ASVS **L1** (grep depth) and closed all 20
threats. CR-01, WR-05, WR-07 and WR-01 are all **mutation-proven** — the reviewer broke the input
and watched the partition stay green — which is stronger evidence than any L1 grep. Four threat
closures therefore do not hold:

| Threat | Closed on the strength of | Why it does not hold |
|--------|--------------------------|----------------------|
| `T-48-03` (no `paths:` filters) | P8 | P8 only reads `ci.yml`; a `.yml` or `.yaml` workflow added later is unchecked |
| `T-48-04` (no stray `secrets.*`) | P8 | same |
| `T-48-10` (no registry surface) | P8 | same — `ghcr.io` lowercase also slips the case-sensitive pattern |
| `T-48-02` / `T-48-18` (fork gate intact) | P9 | P9's substring checks pass an `if:` neutralized with `\|\| true` |
| `T-48-11` / `T-48-20` (no vacuous partition) | P4 sentinels | WR-04: P4 is itself vacuous with respect to `revive` |

This does **not** retroactively invalidate the phase — no shipped evidence is wrong; the gates are
narrower than their own headers claim. It does mean `48-SECURITY.md`'s "What this audit does NOT
establish" section needs the mutation evidence added, and these closures need gap-closure plans.

**Nothing was waived.** No finding was dismissed as out-of-scope, and no gate was relaxed to make
the review come out clean. `.golangci.yml` itself reviewed clean (preset/narrow-rule exclusions, no
finding allowlist).
