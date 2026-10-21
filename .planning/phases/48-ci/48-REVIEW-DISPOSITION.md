---
phase: 48
review: 48-REVIEW.md
titles: json
# hand-reconciled on 2026-10-21 by round-6 plan 48-25: all 15 round-4 rows carry their final
# disposition sourced to the fixing plan/summary, and the 7 round-5 findings (WR-R5-01,
# WR-R5-02, IN-R5-01..05) have their own rows with dispositions. A future mechanical re-run
# that resets rows to `open` is a parser-schema limitation (the WR-R5-NN ids do not match this
# ledger's CR|BL|WR|IN-NN row schema), NOT a re-triage.
findings:
  - id: WR-01
    severity: warning
    disposition: fixed
    title: "The P11 else-detector still uses the SIGPIPE-prone `sed | grep -q` idiom and can silently stop detecting"
    source: "48-21 Task 3 - the gate-to-banner range is captured into BANNER_RANGE before a non-quiet grep reads it to EOF; re-proven by round-5 drill R8 (banner transposed after the else reds P11)"
  - id: WR-02
    severity: warning
    disposition: fixed
    title: "The trust-surface path-filter and secret rules are still text scans and accept four valid-YAML evasion shapes"
    source: "48-22 Task 1 - the parsed `on:` walk and the recursive parsed-string secret walk are primary over the retained textual rules; re-proven by round-5 drills R9/R10 (deleting either walk reds P11 naming quoted-paths resp. tojson-secrets)"
  - id: WR-03
    severity: warning
    disposition: fixed
    title: "Success tokens are per-probe, not per-assertion — a partition can be fully neutered inside a shared probe and still deposit PASS (residual of round-3 CR-02)"
    source: "48-20 Tasks 1-2 + 48-21 Tasks 1-2 - per-assertion-group success tokens across P1/P2/P3/P4 and P6/P7/P9, all required at ok_partition; re-proven by round-5 drills R1-R7 (every deletion exits 1 naming the partition and the missing token)"
  - id: IN-01
    severity: info
    disposition: fixed
    title: "The linter install's integrity settings are inherited from the runner image, not declared (round-3 IN-01, still open)"
    source: "Declaration 48-22 Task 2 (GOTOOLCHAIN/GOSUMDB/GOPROXY declared on the install step); the enforcement pin the declaration lacked landed as 48-23 Task 3's P3-INSTENV-OK exact-equality assertion. Note: the round-5 verification falsified the original claim that the declaration alone reds the gate (R13/R14 kept exit 0); 48-SECURITY.md's T-48-78 cell was corrected accordingly by 48-25 Task 2"
  - id: IN-02
    severity: info
    disposition: fixed
    title: "`ok_partition` dereferences `$1` before its arity check and labels every failure as \"P11\""
    source: "48-20 Task 1 - ok_partition upgraded to a token-list gate (arity >= 3, set -u safe, per-partition prefix) so the arity check precedes any label deref"
  - id: IN-03
    severity: info
    disposition: fixed
    title: "P1's `pull_request_target` scan still matches comment text (round-3 IN-03, still open)"
    source: "48-24 Task 2 - the banned-event scan judges comment-stripped text per enumerated workflow file (carried as round-5 IN-R5-03); a comment naming the event no longer false-fires while a real trigger still reds with file:line"
  - id: IN-04
    severity: info
    disposition: deferred
    title: "`Dockerfile.backend` calls its base images \"pinned\" while every reference is a mutable tag, and the image ships as root"
    source: "Deferred 2026-10-21 (48-25): not re-raised at the recorded severity - the round-5 review's scope was the gate script and ci.yml - no fixing plan is recorded, and no live defect is proven; the wording and root-user observations stay as written and reopen explicitly if they become load-bearing"
  - id: IN-05
    severity: info
    disposition: deferred
    title: "The runtime half of the glibc migration is not exercised anywhere in CI"
    source: "Deferred 2026-10-21 (48-25): not re-raised at the recorded severity - the round-5 review's scope was the gate script and ci.yml - no fixing plan is recorded, and no live defect is proven; the observation stays as written and reopens explicitly if it becomes load-bearing"
  - id: IN-06
    severity: info
    disposition: deferred
    title: "The best-of-5 ratio is not a controlled comparison, and the env override accepts malformed values"
    source: "Deferred 2026-10-21 (48-25): not re-raised at the recorded severity - the round-5 review's scope was the gate script and ci.yml - no fixing plan is recorded, and no live defect is proven; the sampling-shape and env-override observations stay as written and reopen explicitly if they become load-bearing"
  - id: IN-07
    severity: info
    disposition: fixed
    title: "The push-to-main trigger's `only-new-issues` reasoning does not follow, and the push path appears unexercised"
    source: "48-25 Task 1 - the push-trigger comment corrected to the mitigation that actually exists (run-3 fast-forward evidence, force-push whole-repo fallback hazard, the 48-15 history decision as a one-time recorded exception; the trigger limit alone is not the mitigation); carried as round-5 IN-R5-04; comment-only edit, job spans byte-identical, gate exit 0"
  - id: IN-08
    severity: info
    disposition: fixed
    title: "`TestSparklineNoDeps` only inspects the component's own source text"
    source: "48-16, fixed with note - the hazard the test guarded changed: TestSparklineNoDeps now resolves Sparkline.tsx from import.meta.url (node:url), so the hardcoded Windows absolute path no longer exists to guard against (proven green on the Linux runner, run 39380373930); the source-text inspection method itself is unchanged by design"
  - id: CR-01
    severity: critical
    disposition: fixed
    title: "The P5a determinism guard SIGPIPEs deterministically on every Linux runner — the root cause of the tracked M1c red lives in `ci.yml`"
    source: "48-16 - the determinism list is materialised to $RUNNER_TEMP/go-test-list.txt and grepped from the file, so no pipeline remains to SIGPIPE the producer; re-proven by round-5 drill G3 (reintroducing the piped shape reds P2) and by the four-green run 39380373930"
  - id: CR-02
    severity: critical
    disposition: fixed
    title: "The success-token accounting does not prove a probe ran — a neutered partition still prints `ALL PASSED`"
    source: "48-17 + 48-20/48-21 - 48-17 bound every partition's deposit to a probe-written log containing a probe-emitted token (the begin+ok accounting), then 48-20/48-21 required per-assertion-group tokens; re-proven by round-5 drills R1-R7 (all nine deletion drills exit 1 naming partition + missing token)"
  - id: WR-04
    severity: warning
    disposition: fixed
    title: "The determinism step passes vacuously if the pinned test is renamed — `-run` with zero matches exits 0"
    source: "48-16 - the materialised -list shape; pinned by 48-17's P2 assertion, whose drill proves the piped shape reds P2, so a renamed test can no longer pass vacuously"
  - id: WR-05
    severity: warning
    disposition: fixed
    title: "`Gate check` is granted all three live secrets although `--gate-check` is documented to run offline — the code contradicts WR-08's recorded premise"
    source: "48-12 - the three live secret mappings removed from Gate check (replay step only); pinned by 48-13's P9 Gate-check-no-env and single-occurrence rules; re-proven by round-5 regression drill G4 (re-adding the three-secret env reds P9)"
  - id: WR-R5-01
    severity: warning
    disposition: fixed
    title: "The P11 fixture loop cannot attribute rejections — a deleted or emptied fixture counts as a pin, so the 12 evasion fixtures can be silently removed while P11 stays green"
    source: "48-23 Task 1 - per-fixture registered rejection reason with a refusing default arm, a file-presence guard, and five per-class deposit tokens; the round-5 reproductions R11/R12 now red naming the fixture"
  - id: WR-R5-02
    severity: warning
    disposition: fixed
    title: "The 48-22 install-surface env declarations are pinned by nothing — removing `GOTOOLCHAIN: local` (or `GOSUMDB`/`GOPROXY`) keeps every partition green"
    source: "48-23 Task 3 - the parsed exact-equality pin on the verify job's linter install-step env map with its own P3-INSTENV-OK token; R13/R14 now red; the 48-SECURITY.md T-48-78 register cell corrected by 48-25 Task 2"
  - id: IN-R5-01
    severity: info
    disposition: fixed
    title: "`PUBLICATION_TOKENS` got no parsed layer in the 48-22 conversion — `push: 'true'` and a private-registry `tags:` value evade the publication rule everywhere outside P7's two judged steps"
    source: "48-23 Task 2 - the quoted publication variants (push: 'true' / push: \"true\") and the attributing quoted-push fixture; the parsed with:-mapping registry half is deferred with the review's own rationale: judging with.tags registries needs a D-26 registry-allowlist decision, and a real publish names a registry token the existing rule catches"
  - id: IN-R5-02
    severity: info
    disposition: fixed
    title: "Two assertion groups in the reviewed delta have no evidence binding of their own"
    source: "48-23 Task 2 (the secrets[ branch carries its own column-0 marker range and the dyn-secrets fixture only that branch catches) + 48-24 Task 1 (P4-CONFIG-OK appended after the config-verify rc refusal and required first at the P4 deposit)"
  - id: IN-R5-03
    severity: info
    disposition: fixed
    title: "P1's `pull_request_target` scan still judges raw source (carried from round 4 IN-03, still present)"
    source: "48-24 Task 2 - the comment-stripped per-file scan with a live-probe canary and an asserted decisive success (scanned == enumerated, hits == 0)"
  - id: IN-R5-04
    severity: info
    disposition: fixed
    title: "The push-to-main trigger comment still claims a mitigation it does not provide (carried from round 4 IN-07, still present)"
    source: "48-25 Task 1 - the corrected push-trigger comment: run-3 fast-forward evidence, the force-push whole-repo fallback hazard, the 48-15 history decision as a one-time recorded exception, and the explicit note that the trigger limit alone is not the mitigation"
  - id: IN-R5-05
    severity: info
    disposition: fixed
    title: "Dead code left by the round-5 edits"
    source: "48-24 Task 3 - the P2/P3 probe import reduced to sys, yaml; exactly one drill-survivable basename binding in scan_trust_surface (hoisted above the walk marker range the R10 drill deletes)"
open: 0
total: 22
# 22 rows = 15 round-4 findings + 7 round-5 findings. unparsed: 0 by hand-reconciliation
# (2026-10-21, plan 48-25): the WR-R5-NN ids are the round-5 review's own recorded ids
# (48-REVIEW.md), kept verbatim for cross-referencing that file; they do not match this ledger's
# CR|BL|WR|IN-NN row schema, which is why a mechanical run reported them unparsed. A future
# mechanical re-run resetting rows to `open` is a parser-schema limitation, not a re-triage.
unparsed: 0
recorded: 2026-10-20T07:07:19.325Z
hand_reconciled: 2026-10-21 (round-6 plan 48-25)
---

# Phase 48: Code Review Disposition

> **Hand-reconciled 2026-10-21 (round-6 plan 48-25).** This ledger was left by its round-4
> mechanical run with every row `open` and `unparsed: 7`. Every row now carries its final
> disposition, sourced to the fixing plan/summary (verified against each cited summary before
> writing): 19 fixed, 3 deferred (IN-04/05/06 — never re-raised, no fix on record). The
> round-5 findings keep their own recorded ids (WR-R5-NN / IN-R5-NN) verbatim so they
> cross-reference [48-REVIEW.md](48-REVIEW.md) directly. A future mechanical re-run that resets
> rows to `open` is a parser-schema limitation, not a re-triage. The round-5 verification
> report (48-VERIFICATION.md) independently reproduced the mutations cited in the re-proven
> rows; the round-6 mechanism fixes and their drill matrix are recorded in
> [48-23-SUMMARY.md](48-23-SUMMARY.md) and [48-24-SUMMARY.md](48-24-SUMMARY.md).

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | fixed | 48-21 Task 3 — the gate-to-banner range is captured into BANNER_RANGE before a non-quiet grep reads it to EOF; re-proven by round-5 drill R8 (banner transposed after the else reds P11) |
| WR-02 | warning | fixed | 48-22 Task 1 — the parsed `on:` walk and the recursive parsed-string secret walk are primary over the retained textual rules; re-proven by round-5 drills R9/R10 (deleting either walk reds P11 naming quoted-paths resp. tojson-secrets) |
| WR-03 | warning | fixed | 48-20 Tasks 1-2 + 48-21 Tasks 1-2 — per-assertion-group success tokens across P1/P2/P3/P4 and P6/P7/P9, all required at ok_partition; re-proven by round-5 drills R1-R7 (every deletion exits 1 naming the partition and the missing token) |
| IN-01 | info | fixed | Declaration 48-22 Task 2 (GOTOOLCHAIN/GOSUMDB/GOPROXY declared on the install step); the enforcement pin the declaration lacked landed as 48-23 Task 3's P3-INSTENV-OK exact-equality assertion — note: round 5 falsified the original claim that the declaration alone reds the gate (R13/R14 kept exit 0), and 48-SECURITY.md's T-48-78 cell was corrected accordingly by 48-25 Task 2 |
| IN-02 | info | fixed | 48-20 Task 1 — ok_partition upgraded to a token-list gate (arity >= 3, set -u safe, per-partition prefix) so the arity check precedes any label deref |
| IN-03 | info | fixed | 48-24 Task 2 — the banned-event scan judges comment-stripped text per enumerated workflow file (carried as round-5 IN-R5-03); a comment naming the event no longer false-fires while a real trigger still reds with file:line |
| IN-04 | info | deferred | Deferred 2026-10-21 (48-25): not re-raised at the recorded severity — the round-5 review's scope was the gate script and ci.yml; no fixing plan is recorded and no live defect is proven; the base-image wording and root-user observations stay as written and reopen explicitly if they become load-bearing |
| IN-05 | info | deferred | Deferred 2026-10-21 (48-25): not re-raised at the recorded severity — the round-5 review's scope was the gate script and ci.yml; no fixing plan is recorded and no live defect is proven; the observation stays as written and reopens explicitly if it becomes load-bearing |
| IN-06 | info | deferred | Deferred 2026-10-21 (48-25): not re-raised at the recorded severity — the round-5 review's scope was the gate script and ci.yml; no fixing plan is recorded and no live defect is proven; the sampling-shape and env-override observations stay as written and reopen explicitly if they become load-bearing |
| IN-07 | info | fixed | 48-25 Task 1 — the push-trigger comment corrected to the mitigation that actually exists (run-3 fast-forward evidence, force-push whole-repo fallback hazard, the 48-15 history decision as a one-time recorded exception; the trigger limit alone is not the mitigation); carried as round-5 IN-R5-04; comment-only edit, job spans byte-identical, gate exit 0 |
| IN-08 | info | fixed | 48-16, fixed with note — the hazard the test guarded changed: TestSparklineNoDeps now resolves Sparkline.tsx from import.meta.url (node:url), so the hardcoded Windows absolute path no longer exists to guard against (green on the Linux runner, run 39380373930); the source-text inspection method itself is unchanged by design |
| CR-01 | critical | fixed | 48-16 — the determinism list is materialised to $RUNNER_TEMP/go-test-list.txt and grepped from the file, so no pipeline remains to SIGPIPE the producer; re-proven by round-5 drill G3 (reintroducing the piped shape reds P2) and by the four-green run 39380373930 |
| CR-02 | critical | fixed | 48-17 + 48-20/48-21 — 48-17 bound every partition's deposit to a probe-written log containing a probe-emitted token (the begin+ok accounting), then 48-20/48-21 required per-assertion-group tokens; re-proven by round-5 drills R1-R7 (all nine deletion drills exit 1 naming partition + missing token) |
| WR-04 | warning | fixed | 48-16 — the materialised -list shape; pinned by 48-17's P2 assertion, whose drill proves the piped shape reds P2, so a renamed test can no longer pass vacuously |
| WR-05 | warning | fixed | 48-12 — the three live secret mappings removed from Gate check (replay step only); pinned by 48-13's P9 Gate-check-no-env and single-occurrence rules; re-proven by round-5 regression drill G4 (re-adding the three-secret env reds P9) |
| WR-R5-01 | warning | fixed | 48-23 Task 1 — per-fixture registered rejection reason with a refusing default arm, a file-presence guard, and five per-class deposit tokens; the round-5 reproductions R11/R12 now red naming the fixture |
| WR-R5-02 | warning | fixed | 48-23 Task 3 — the parsed exact-equality pin on the verify job's linter install-step env map with its own P3-INSTENV-OK token; R13/R14 now red; the 48-SECURITY.md T-48-78 register cell corrected by 48-25 Task 2 |
| IN-R5-01 | info | fixed | 48-23 Task 2 — the quoted publication variants (push: 'true' / push: "true") and the attributing quoted-push fixture; the parsed with:-mapping registry half is deferred with the review's own rationale: judging with.tags registries needs a D-26 registry-allowlist decision, and a real publish names a registry token the existing rule catches |
| IN-R5-02 | info | fixed | 48-23 Task 2 (the secrets[ branch carries its own column-0 marker range and the dyn-secrets fixture only that branch catches) + 48-24 Task 1 (P4-CONFIG-OK appended after the config-verify rc refusal and required first at the P4 deposit) |
| IN-R5-03 | info | fixed | 48-24 Task 2 — the comment-stripped per-file scan with a live-probe canary and an asserted decisive success (scanned == enumerated, hits == 0) |
| IN-R5-04 | info | fixed | 48-25 Task 1 — the corrected push-trigger comment: run-3 fast-forward evidence, the force-push whole-repo fallback hazard, the 48-15 history decision as a one-time recorded exception, and the explicit note that the trigger limit alone is not the mitigation |
| IN-R5-05 | info | fixed | 48-24 Task 3 — the P2/P3 probe import reduced to sys, yaml; exactly one drill-survivable basename binding in scan_trust_surface (hoisted above the walk marker range the R10 drill deletes) |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
