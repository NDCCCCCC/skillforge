---
phase: 48
review: 48-REVIEW.md
titles: json
findings:
  - id: CR-01
    severity: critical
    disposition: fixed
    title: "Gate job executes `curl | sh` from a mutable HEAD ref with no checksum verification — and 48-11's own supply-chain threat flag for this line was dropped from the security record"
    source: "48-12 Task 1 — module-proxy `go install @v2.12.2` + `[ -x ]` postcondition; pinned by 48-13's P2/P3 install-shape assertion and re-proven by the fetch/script reintroduction drill"
  - id: WR-01
    severity: warning
    disposition: fixed
    title: "The new `verify` job is invisible to every guard the phase built — its existence is unpinned and neither its job-level nor step-level `if:` is checked (48-07/48-11 seam)"
    source: "48-13 Task 2 — the shared P2/P3 probe now judges the verify job (existence, if-freeness, gate terminus, credential-free checkout, module-proxy install); M9/M10 drills red the gate"
  - id: WR-02
    severity: warning
    disposition: fixed
    title: "The install pipeline drops curl's exit status — the WR-03 class is reintroduced by the same phase that fixed it"
    source: "48-12 Task 1 — no pipe remains in the install surface (single `go install` command); pinned by 48-13's banned fetch/script tokens in P3"
  - id: WR-03
    severity: warning
    disposition: fixed
    title: "Begin-marker accounting does not bind a partition to its assertions — P8's `${TRUST_SURFACE_COUNT:-0}` default absorbs a deleted probe and keeps the gate green"
    source: "48-13 Task 1 — begin+ok requirement in `done_partition`, removal of the `:-0` default, and the M8 mutation drill (deleted P8 probe now aborts under `set -u`)"
  - id: WR-04
    severity: warning
    disposition: fixed
    title: "The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0"
    source: "48-12 — the `-list` existence guard; now carried by the materialised-list shape from 48-16 and pinned by 48-17's P2 assertion (a pipe after `-list` reds the gate)"
  - id: WR-05
    severity: warning
    disposition: fixed
    title: "`Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise"
    source: "48-12 — the three live mappings now occur only on the replay step; pinned by 48-13's P9 Gate-check-no-env and single-occurrence rules (D4/D7 drills red the gate)"
  - id: IN-01
    severity: info
    disposition: fixed
    title: "Header still says \"Three independent jobs\" while the file now ships four"
    source: "48-12 — the header is now the truthful four-job header; P1's per-file parse probe enumerates all four"
  - id: IN-02
    severity: info
    disposition: fixed
    title: "A fork PR's green `verify` check is fully self-attested, and the job persists the runner token — the header's \"strictly narrower surface\" claim omits this"
    source: "48-12 — `persist-credentials: false` on the verify checkout (asserted by 48-13's P2/P3 probe); the fork-PR self-attestation remains and is documented in the ci.yml header, not removed"
  - id: IN-03
    severity: info
    disposition: fixed
    title: "PyYAML is installed unpinned into the gate job"
    source: "48-12 — `pyyaml==6.0.3` pinned in the verify job's install step"
open: 0
total: 9
recorded: 2026-10-19T15:48:50.874Z
closed: 2026-10-20
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

**Round-3 closure (2026-10-20).** Plans 48-12 and 48-13 landed the fixes this ledger was waiting
for, and `48-VERIFICATION.md` round 3 re-proved every mutation closure independently (the
reintroduction drill for CR-01/WR-02, the verify-job M9/M10 drills for WR-01, the M8 drill for
WR-03, the D4/D7 drills for WR-05, and P2 for WR-04). This ledger was brought to its final state
on 2026-10-20: all nine findings are `fixed`, each with the closing plan and its proof in the
Source cell. The round-2 history below is retained unchanged — the ledger records what was open,
not only what ended closed.

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | fixed | 48-12 Task 1 — module-proxy `go install @v2.12.2` + `[ -x ]` postcondition; pinned by 48-13's P2/P3 install-shape assertion and re-proven by the fetch/script reintroduction drill |
| WR-01 | warning | fixed | 48-13 Task 2 — the shared P2/P3 probe now judges the verify job (existence, if-freeness, gate terminus, credential-free checkout, module-proxy install); M9/M10 drills red the gate |
| WR-02 | warning | fixed | 48-12 Task 1 — no pipe remains in the install surface (single `go install` command); pinned by 48-13's banned fetch/script tokens in P3 |
| WR-03 | warning | fixed | 48-13 Task 1 — begin+ok requirement in `done_partition`, removal of the `:-0` default, and the M8 mutation drill (deleted P8 probe now aborts under `set -u`) |
| WR-04 | warning | fixed | 48-12 — the `-list` existence guard; now carried by the materialised-list shape from 48-16 and pinned by 48-17's P2 assertion (a pipe after `-list` reds the gate) |
| WR-05 | warning | fixed | 48-12 — the three live mappings now occur only on the replay step; pinned by 48-13's P9 Gate-check-no-env and single-occurrence rules (D4/D7 drills red the gate) |
| IN-01 | info | fixed | 48-12 — the header is now the truthful four-job header; P1's per-file parse probe enumerates all four |
| IN-02 | info | fixed | 48-12 — `persist-credentials: false` on the verify checkout (asserted by 48-13's P2/P3 probe); the fork-PR self-attestation remains and is documented in the ci.yml header, not removed |
| IN-03 | info | fixed | 48-12 — `pyyaml==6.0.3` pinned in the verify job's install step |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
