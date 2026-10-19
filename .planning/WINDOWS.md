---
schema_version: 1
open_count: 3
waived_count: 0
fixed_count: 0
total_count: 3
last_updated: 2026-10-19T14:44:27.226Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 48 | unrun-verify | verify-phase-48.sh |  | Task 3 CI-run proof (M1) never ran: no ci.yml run exists and none is triggerable from this host (git transport to github.com unreachable; push.branches=[main]; origin/main shares no history with this branch). CICD-01/02/03 remain Pending. | open |  | 2026-10-19T09:59:40.790Z |  |
| 2 | 48 | unmet-truth | .planning/phases/48-ci/48-04-SUMMARY.md |  | Plan 48-04 success criterion 3 (a real GitHub Actions run with go/fe/docker green) is unmet; CICD-01/02/03 stay Pending and the phase is not closable until a git-transport-capable host obtains the run. | open |  | 2026-10-19T09:59:54.045Z |  |
| 3 | 48 | unrun-verify | 48-VALIDATION.md M1 (first real GitHub Actions run of ci.yml) |  | M1 blocked on this host (no git transport to github.com; origin/main unrelated history) — recipe persisted in 48-11-SUMMARY.md; CICD-01/02/03 stay Pending until a transport-capable host produces go/fe/docker all success | open |  | 2026-10-19T14:44:27.226Z |  |

````json
[
  {
    "id": 1,
    "kind": "unrun-verify",
    "phase": "48",
    "file": "verify-phase-48.sh",
    "line": null,
    "description": "Task 3 CI-run proof (M1) never ran: no ci.yml run exists and none is triggerable from this host (git transport to github.com unreachable; push.branches=[main]; origin/main shares no history with this branch). CICD-01/02/03 remain Pending.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-10-19T09:59:40.790Z",
    "resolved_at": null,
    "milestone": "v1.7"
  },
  {
    "id": 2,
    "kind": "unmet-truth",
    "phase": "48",
    "file": ".planning/phases/48-ci/48-04-SUMMARY.md",
    "line": null,
    "description": "Plan 48-04 success criterion 3 (a real GitHub Actions run with go/fe/docker green) is unmet; CICD-01/02/03 stay Pending and the phase is not closable until a git-transport-capable host obtains the run.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-10-19T09:59:54.045Z",
    "resolved_at": null,
    "milestone": "v1.7"
  },
  {
    "id": 3,
    "kind": "unrun-verify",
    "phase": "48",
    "file": "48-VALIDATION.md M1 (first real GitHub Actions run of ci.yml)",
    "line": null,
    "description": "M1 blocked on this host (no git transport to github.com; origin/main unrelated history) — recipe persisted in 48-11-SUMMARY.md; CICD-01/02/03 stay Pending until a transport-capable host produces go/fe/docker all success",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-10-19T14:44:27.226Z",
    "resolved_at": null,
    "milestone": "v1.7"
  }
]
````
