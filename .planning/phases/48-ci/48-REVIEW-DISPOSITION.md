---
phase: 48
review: 48-REVIEW.md
titles: json
findings:
  - id: CR-01
    severity: critical
    disposition: open
    title: "The P5a determinism guard SIGPIPEs deterministically on every Linux runner — the root cause of the tracked M1c red lives in `ci.yml`"
  - id: CR-02
    severity: critical
    disposition: open
    title: "The success-token accounting does not prove a probe ran — a neutered partition still prints `ALL PASSED`"
  - id: WR-01
    severity: warning
    disposition: open
    title: "The P11 else-detector uses the same SIGPIPE-prone idiom and can silently stop detecting"
  - id: WR-02
    severity: warning
    disposition: open
    title: "The trust-surface secret and path-filter rules are regex-scanned over text and miss three evasion shapes"
  - id: WR-03
    severity: warning
    disposition: open
    title: "`eval-gate.yml` persists the runner token in the one job that holds the three live secrets"
  - id: IN-01
    severity: info
    disposition: open
    title: "The linter install's integrity guarantees are inherited from the runner image, not declared — the \"no network fetch-and-execute surface\" comment is overstated"
  - id: IN-02
    severity: info
    disposition: open
    title: "The pinned linter version `v2.12.2` is duplicated in four places with no single source of truth"
  - id: IN-03
    severity: info
    disposition: open
    title: "P1's `pull_request_target` scan matches comment text, unlike the trust-surface rules"
  - id: IN-04
    severity: info
    disposition: open
    title: "`scan_trust_surface` reuses one log path for P8 and the P11 fixtures"
  - id: IN-05
    severity: info
    disposition: open
    title: "The `verify` check is self-attested on fork PRs (already documented — noted for branch-protection purposes)"
  - id: WR-04
    severity: warning
    disposition: open
    title: "The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0"
  - id: WR-05
    severity: warning
    disposition: open
    title: "`Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise"
open: 12
total: 12
recorded: 2026-10-20T02:25:11.610Z
---

# Phase 48: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | open | - |
| CR-02 | critical | open | - |
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |
| IN-03 | info | open | - |
| IN-04 | info | open | - |
| IN-05 | info | open | - |
| WR-04 | warning | open | - (not in the current review) |
| WR-05 | warning | open | - (not in the current review) |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
