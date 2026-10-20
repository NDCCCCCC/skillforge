---
phase: 48
review: 48-REVIEW.md
titles: json
findings:
  - id: CR-01
    severity: critical
    disposition: open
    title: "Gate job executes `curl | sh` from a mutable HEAD ref with no checksum verification — and 48-11's own supply-chain threat flag for this line was dropped from the security record"
  - id: WR-01
    severity: warning
    disposition: open
    title: "The new `verify` job is invisible to every guard the phase built — its existence is unpinned and neither its job-level nor step-level `if:` is checked (48-07/48-11 seam)"
  - id: WR-02
    severity: warning
    disposition: open
    title: "The install pipeline drops curl's exit status — the WR-03 class is reintroduced by the same phase that fixed it"
  - id: WR-03
    severity: warning
    disposition: open
    title: "Begin-marker accounting does not bind a partition to its assertions — P8's `${TRUST_SURFACE_COUNT:-0}` default absorbs a deleted probe and keeps the gate green"
  - id: WR-04
    severity: warning
    disposition: open
    title: "The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0"
  - id: WR-05
    severity: warning
    disposition: open
    title: "`Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise"
  - id: IN-01
    severity: info
    disposition: open
    title: "Header still says \"Three independent jobs\" while the file now ships four"
  - id: IN-02
    severity: info
    disposition: open
    title: "A fork PR's green `verify` check is fully self-attested, and the job persists the runner token — the header's \"strictly narrower surface\" claim omits this"
  - id: IN-03
    severity: info
    disposition: open
    title: "PyYAML is installed unpinned into the gate job"
open: 9
total: 9
recorded: 2026-10-19T15:48:50.874Z
---

# Phase 48: Code Review Disposition

> **Round 2 (2026-10-19, post gap-closure re-review).** This file is the gate-format ledger for the
> INCREMENTAL re-review that ran after plans 48-05…48-11 landed. The ROUND-1 review's 19 findings
> and their final dispositions (18 fixed by 48-05…48-09, 1 closed-by-evidence on plan 48-15 =
> WR-06 lint step green on run-3 — recorded by plan 48-10 as the option-D deferral and reconciled
> by plan 48-15 in `48-15-SUMMARY.md` § Task 2 Run-3) live in [48-REVIEW-DISPOSITION-R1.md](48-REVIEW-DISPOSITION-R1.md) — that round's
> ledger was hand-authored in a different table format the gate parser cannot read, so its rows
> could not be carried forward mechanically; it was restored from git (`6aa3391`) rather than lost.
> **Finding IDs are REUSED across rounds and name different findings**: round-2 CR-01/WR-01…05/IN-01…03
> below are NOT the round-1 findings of the same ids (those are dispositioned in the R1 file).

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | open | - |
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| WR-04 | warning | open | - |
| WR-05 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |
| IN-03 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
