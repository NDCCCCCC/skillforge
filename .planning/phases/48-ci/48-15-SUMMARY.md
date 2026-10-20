---
phase: 48-ci
plan: 15
subsystem: ci-evidence
tags: [github-actions, ci-run, requirements-flip, history-decision, record-reconciliation]

requires:
  - phase: 48-ci
    plan: 11
    provides: "the M1 run recipe (history decision -> push -> gate-preserving trigger -> gh run watch -> red-run classification) this plan executes"
  - phase: 48-ci
    plan: 14
    provides: "transport-truth refresh (git ls-remote origin HEAD exit 0, 369b08a) that re-opens M1"

provides:
  - "The recorded repository-owner history decision (option a) and the executed push that made this branch the public main"
  - "One real GitHub Actions run of ci.yml with its four job conclusions, deciding CICD-01/02/03 and WR-06 option D on evidence"

affects: [phase-49-hermetic-demo, phase-51-release-engineering]

key-files:
  created:
    - .planning/phases/48-ci/48-15-SUMMARY.md
  modified: []

status: in-progress
---

# Phase 48 Plan 15: First Real CI Run — History Decision, Push, and Record Reconciliation Summary

**One-liner:** Executed the M1 recipe end-to-end — recorded the owner's option-a history decision, pushed this branch to public main over SSH, and captured the first real ci.yml run with all four job conclusions.

## Task 1 — The repository-history decision (M1 recipe step 1)

History decision: Option a

**Date:** 2026-10-20 (decision recorded by the repository owner; executor re-verified evidence immediately before recording)

**Decision:** Option a — replace public `main` with this branch (`phase-20-p10-6-admin-console`, force-push), WITH the v1.5-archive tag preservation the previous executor recommended: the outgoing single-commit `main` (`369b08a`, "SkillForge v1.5") is preserved as tag `v1.5-archive` before the rewrite, so the old public history stays reachable.

**Fresh evidence (re-verified 2026-10-20 ~00:55 UTC, this host, immediately before this record):**

| Check | Result |
|-------|--------|
| `git ls-remote git@github.com:NDCCCCCC/skillforge.git HEAD refs/heads/main refs/tags/v1.5-archive` | exit 0; `HEAD` and `refs/heads/main` = `369b08a6babf0cc97c7d5b6db3a3c89a2012b9de`; no `v1.5-archive` tag on the remote yet |
| `git merge-base HEAD 369b08a…` | exit 1 — merge-base empty (unrelated histories; GitHub cannot open a PR between them) |
| `git log --oneline 369b08a… -1` | `369b08a SkillForge v1.5 — Skill 驱动的企业知识问答平台` (single-commit default branch) |
| `git rev-list --count 369b08a…..HEAD` | 1891 commits ahead, no common ancestor |
| `gh auth status` | Logged in to github.com as NDCCCCCC, scopes include `workflow` |
| `gh run list --workflow ci.yml` | HTTP 404 — workflow never existed on the remote default branch (baseline: no run ever) |
| ci.yml trigger (lines 36-45) | `push.branches: [main]`, `pull_request.branches: [main, 'phase-*']` — a force-push to main directly triggers all four jobs; branch protection off (AR-48-01) |

**Exact push sequence Task 2 executes (verbatim, from the owner's decision):**

```bash
git push git@github.com:NDCCCCCC/skillforge.git 369b08a:refs/tags/v1.5-archive   # preserve v1.5 single commit
git push git@github.com:NDCCCCCC/skillforge.git +phase-20-p10-6-admin-console:main
```

SSH transport is used explicitly because HTTPS to github.com:443 was timing out at this host's network boundary in the prior session; the SSH path authenticated cleanly on re-verification.

**Reversibility (one-way):** rewriting the public default branch's history cannot be undone from this host once third parties may have cloned. Mitigation recorded with the decision: the outgoing `main` tip is preserved as tag `v1.5-archive` in the same sequence, before the rewrite lands.

**BASE_SHA for Task 3's no-weakening check:** `4a8e4c99427a491495e720743073ac0d3dc7f9ca` (pre-plan HEAD, before this plan's first commit — the diff base covers every commit this plan makes).

**No push has been executed by Task 1.**
