---
phase: 48
review: 48-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "The P11 else-detector still uses the SIGPIPE-prone `sed | grep -q` idiom and can silently stop detecting"
  - id: WR-02
    severity: warning
    disposition: open
    title: "The trust-surface path-filter and secret rules are still text scans and accept four valid-YAML evasion shapes"
  - id: WR-03
    severity: warning
    disposition: open
    title: "Success tokens are per-probe, not per-assertion — a partition can be fully neutered inside a shared probe and still deposit PASS (residual of round-3 CR-02)"
  - id: IN-01
    severity: info
    disposition: open
    title: "The linter install's integrity settings are inherited from the runner image, not declared (round-3 IN-01, still open)"
  - id: IN-02
    severity: info
    disposition: open
    title: "`ok_partition` dereferences `$1` before its arity check and labels every failure as \"P11\""
  - id: IN-03
    severity: info
    disposition: open
    title: "P1's `pull_request_target` scan still matches comment text (round-3 IN-03, still open)"
  - id: IN-04
    severity: info
    disposition: open
    title: "`Dockerfile.backend` calls its base images \"pinned\" while every reference is a mutable tag, and the image ships as root"
  - id: IN-05
    severity: info
    disposition: open
    title: "The runtime half of the glibc migration is not exercised anywhere in CI"
  - id: IN-06
    severity: info
    disposition: open
    title: "The best-of-5 ratio is not a controlled comparison, and the env override accepts malformed values"
  - id: IN-07
    severity: info
    disposition: open
    title: "The push-to-main trigger's `only-new-issues` reasoning does not follow, and the push path appears unexercised"
  - id: IN-08
    severity: info
    disposition: open
    title: "`TestSparklineNoDeps` only inspects the component's own source text"
  - id: CR-01
    severity: critical
    disposition: open
    title: "The P5a determinism guard SIGPIPEs deterministically on every Linux runner — the root cause of the tracked M1c red lives in `ci.yml`"
  - id: CR-02
    severity: critical
    disposition: open
    title: "The success-token accounting does not prove a probe ran — a neutered partition still prints `ALL PASSED`"
  - id: WR-04
    severity: warning
    disposition: open
    title: "The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0"
  - id: WR-05
    severity: warning
    disposition: open
    title: "`Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise"
open: 15
total: 15
unparsed: 7
recorded: 2026-10-20T07:07:19.325Z---

# Phase 48: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - (not in the current review) |
| WR-02 | warning | open | - (not in the current review) |
| WR-03 | warning | open | - (not in the current review) |
| IN-01 | info | open | - (not in the current review) |
| IN-02 | info | open | - (not in the current review) |
| IN-03 | info | open | - (not in the current review) |
| IN-04 | info | open | - (not in the current review) |
| IN-05 | info | open | - (not in the current review) |
| IN-06 | info | open | - (not in the current review) |
| IN-07 | info | open | - (not in the current review) |
| IN-08 | info | open | - (not in the current review) |
| CR-01 | critical | open | - (not in the current review) |
| CR-02 | critical | open | - (not in the current review) |
| WR-04 | warning | open | - (not in the current review) |
| WR-05 | warning | open | - (not in the current review) |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
