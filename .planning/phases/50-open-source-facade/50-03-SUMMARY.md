---
phase: 50-open-source-facade
plan: 03
status: complete
subsystem: docs, tooling
requires:
  - "50-01-PLAN.md (README/architecture truth sync — P-README/P-ARCH drill against it)"
  - "50-02-PLAN.md (community files — P-COMMUNITY drill against it)"
provides:
  - "verify-phase-50.sh — nine-assertion facade gate with four mutation drills"
  - "make verify-phase-50"
affects:
  - "Makefile (verify-phase-50 target)"
tech-stack:
  - bash
key-files:
  - verify-phase-50.sh
  - Makefile
key-decisions:
  - "Drill mutations must land on load-bearing comparisons (wrong threshold/path), never comment out the predicate or the || echo."
  - "Drill environment is handed through a .gate-drill-env file in the scratch copy (WSLENV cross-host forwarding is unreliable)."
actuals:
  tokens: 26000
  tasks: 2
  commits: 1
---

## Summary

`verify-phase-50.sh` gates the open-source facade with six partitions and
four mutation drills, each proven to bite:

- **P-README** — skill-count sync vs `examples/skills`, honest CI badge
  (no decorative green), WeKnora/Semantica lane wording, architecture link,
  no false CGO-optional claim. Drill D1 flips the directory-count
  comparison.
- **P-ARCH** — `docs/architecture.md` exists, has a Mermaid block, names
  all four layers, and every backticked repo path it mentions exists
  (zero-fabrication loop). Drill D2 points the existence check at a
  nonexistent file.
- **P-COMMUNITY** — README links SECURITY.md, CODE_OF_CONDUCT.md,
  CONTRIBUTING.md; the community files exist; issue/PR templates exist and
  contain no secret-eliciting prompts. Drill D3 breaks the SECURITY.md
  anchor inside the compound README condition.
- **P-DEMO** — `docs/demo.gif` exists and opens with a GIF magic number.
  Drill D4 drops GIF89a from the accepted magic set, so the healthy asset
  itself turns the partition red.
- **P-BADGE** — the README badge block contains no decorative static-green
  CI badge text.
- **P-SELF** — registry self-proof: partition count, single banner site,
  every drill went red and named its partition; the gate refuses to pass
  if any drill stayed green ("fix the SOURCE, never the drill").

`make verify-phase-50` runs it; the gate is healthy-green on the current
tree and all four drills go red.

## Deviations

1. (Rule 2) Drills D2/D3 as planned mutated the fatal *predicate*
   (`if grep ... ; then` -> `if false; then`) — that only skips the
   red-check, it does not create a red line, so the partitions stayed
   green and the drills were vacuous. Re-designed onto load-bearing
   comparisons (existence target / anchor string), predicates untouched.
2. (Rule 2) Drill D4 mutated `case "$magic"` to a constant — a no-op on a
   tree whose asset is already GIF89a. Re-designed to narrow the accepted
   magic set.
3. (Rule 3) The drill's partition-naming check now accepts the partitions'
   actual failure banners (`P-README: ...` style) in addition to the
   ok-deposit wording.
4. Drill environment crosses into the scratch copy via a `.gate-drill-env`
   file sourced at gate startup; WSLENV-based Win32->WSL env forwarding
   proved unreliable across the cygwin parent chain.
