---
phase: 48
review: 48-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "The workflow-level `permissions:` mapping-value branch has no fixture and no class token — deleting it keeps the gate green while a top-level write scope is accepted"
    source: "round-7 review (48-REVIEW.md, Round 7) — same class of hole round 6 was raised for: the branch at verify-phase-48.sh:1501-1506 is deletable with no fixture change while a flow- or quoted-key top-level write scope is accepted"
  - id: WR-02
    severity: warning
    disposition: open
    title: "The new quote-aware stripper falsifies the two `quoted-hash-secret` isolation comments — the fixture no longer isolates the parsed secret walk"
    source: "round-7 review (48-REVIEW.md, Round 7) — the retained textual SECRET_REF scan now sees the quoted reference itself (matches the R21/R21-counter rows already recorded in 48-27-SUMMARY.md); the parsed walk is still pinned by tojson-secrets / dyn-secrets"
  - id: IN-01
    severity: info
    disposition: open
    title: "The fixture-count comment still says \"fourteen rejections\" while the loop now runs 17"
    source: "round-7 review (48-REVIEW.md, Round 7) — stale comment at verify-phase-48.sh:2411; every authoritative count site and all 8 class tokens agree on 17"
  - id: IN-02
    severity: info
    disposition: open
    title: "The stripper's quote states are unexercised — every canary and fixture passes with SINGLE/DOUBLE/escape handling deleted, so the recorded R21 proof over-claims"
    source: "round-7 review (48-REVIEW.md, Round 7) — R21 distinguishes the whitespace rule from the old quote-blind regex, not quote-awareness from whitespace-awareness"
  - id: IN-03
    severity: info
    disposition: open
    title: "The P1 text layer is still a red-capable judge with a whole-line literal test, and the quote-aware stripper widens what counts as a non-comment line"
    source: "round-7 review (48-REVIEW.md, Round 7) — a workflow that merely names the event inside a quoted scalar now reds where the quote-blind strip was green; no canary covers that shape"
  - id: IN-04
    severity: info
    disposition: open
    title: "The P4 byte count is measured but never asserted, and a failed measurement degrades to an empty number instead of a refusal"
    source: "round-7 review (48-REVIEW.md, Round 7) — a failing wc leaves the evidence line reading `tool output:  byte(s)` with the gate still green (no set -e)"
  - id: CR-01
    severity: critical
    disposition: fixed
    title: "P1 comment-stripped pull_request_target probe is not quote-aware — a '#' inside a quoted YAML scalar truncates a real trigger line (reproduced)"
    source: "plan 48-27 Task 1 — the P1 trigger judgment now runs on the PARSED on: document (doc.get(\"on\", doc.get(True)), covering mapping key / sequence element / bare scalar) with a shared quote-aware stripper (NORMAL/SINGLE/DOUBLE) used by both the P1 text layer and scan_trust_surface; the flow-style positive canary reproduces the old blind spot. Commit a4cb04c. The round-7 review independently confirms the closure."
open: 6
total: 7
recorded: 2026-10-21T09:04:38.113Z
---

# Phase 48: Code Review Disposition

Round 7 (delta `80990b1..HEAD`, scope `verify-phase-48.sh`) is the current review — see
`48-REVIEW.md`. The round-6 ledger this table supersedes is archived verbatim as
`48-REVIEW-DISPOSITION-R6.md`; the round-6 review body is preserved in `48-REVIEW-R6.md`.
`48-REVIEW-DISPOSITION-R1.md` and `48-REVIEW-DISPOSITION-R2.md` are the older rounds' ledgers and
are left untouched.

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | round-7 review — the workflow-level `permissions:` mapping-value branch has no fixture and no class token, so deleting it keeps the gate green |
| WR-02 | warning | open | round-7 review — the quote-aware stripper falsifies the two `quoted-hash-secret` isolation comments |
| IN-01 | info | open | round-7 review — the fixture-count comment still says "fourteen rejections" while the loop runs 17 |
| IN-02 | info | open | round-7 review — the stripper's quote states are unexercised, so the recorded R21 proof over-claims |
| IN-03 | info | open | round-7 review — the P1 text layer is still a red-capable whole-line literal judge and the failure message claims both layers reported |
| IN-04 | info | open | round-7 review — the P4 byte count is measured but never asserted and a failed measurement degrades to an empty number |
| CR-01 | critical | **fixed** | plan 48-27 Task 1 — the P1 trigger judgment now runs on the parsed `on:` document with a shared quote-aware stripper; commit a4cb04c; confirmed closed by the round-7 review |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.

## Round-6 findings — dispositioned by plan 48-27

The round-6 ids WR-01 / WR-02 / IN-01 … IN-04 were REUSED by the round-7 review for different
findings, so the gate's ledger could not keep the earlier rows under the same ids. Their
dispositions are recorded here (full text in `48-REVIEW-DISPOSITION-R6.md`).

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | fixed | 48-27 Task 1 — parsed `on:` judge + shared quote-aware stripper, four canaries, new token `P1-NO-PRT-PARSED`; commit a4cb04c |
| WR-01 | warning | fixed | 48-27 Task 2 — one isolating fixture and one class token per permission rule (`P11-PERMSWRITE-OK`, `P11-JOBPERMS-OK`, `P11-JOBPERMS-STR-OK`, `P11-YAMLEXT-OK`), fixtures 14 → 17, class tokens 5 → 8; commit 09d1444 |
| WR-02 | warning | fixed | 48-27 Task 3 — the P4 evidence line now reports the tool's measured output size (`p4_cfg_bytes=$(wc -c < …)` sampled before any append) instead of a self-fulfilling word; commit 6bbdcf8 |
| IN-01 | info | fixed | 48-27 Task 2 — the publication-token fixture `yaml-ext` now carries its own class token `P11-YAMLEXT-OK`; commit 09d1444 |
| IN-02 | info | open | out of scope for 48-27 — probe rc 1 is still overloaded between hit and crash; not addressed by the round-7 delta |
| IN-03 | info | open | out of scope for 48-27 — the literal secret matchers still miss spaced spellings; not addressed by the round-7 delta |
| IN-04 | info | open | out of scope for 48-27 — `.github/workflows/ci.yml` was not changed in the round-7 delta, so the push-trigger comment is unchanged |
