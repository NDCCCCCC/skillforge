---
phase: 51-release-engineering
plan: 03
status: complete
subsystem: release, tooling
requires:
  - "51-01 (goreleaser config + snapshot)"
  - "51-02 (release workflow + README Releases)"
provides:
  - "verify-phase-51.sh — five partitions + four mutation drills, all proven red"
  - "make verify-phase-51"
affects:
  - "Makefile"
tech-stack:
  - bash
  - python (drill driver)
key-files:
  - verify-phase-51.sh
key-decisions:
  - "Drill env crosses into the scratch copy via .gate-drill-env file (WSLENV forwarding unreliable)."
  - "D5 (snapshot ELF swap) intentionally omitted: P-SNAPSHOT deposits an honest SKIPPED token when no snapshot exists, so a fake-binary drill has no load-bearing anchor on this host; the ELF check itself is executed in CI (release.yml build-linux)."
actuals:
  tokens: 20000
  tasks: 2
  commits: 1
---

## Summary

`verify-phase-51.sh` gates release engineering with five partitions and
four mutation drills, each proven to bite:

- **P-CONFIG** — `.goreleaser.yaml` is v2, builds cmd/server, targets
  linux+windows/amd64 with CGO_ENABLED=1; `goreleaser check` runs when
  goreleaser is on PATH. Drill D1 flips `version: 2` -> `version: 1`.
- **P-WORKFLOW** — release.yml triggers on v* tags only, three jobs, the
  release job is the sole `contents: write` workflow, generate_release_notes
  set. Drills D2 (add a branch trigger) and D3 (flatten the permission
  gradient) both bite here.
- **P-README** — `## Releases` section with per-platform asset names and
  the stub-mode caveat. Drill D4 renames the heading.
- **P-SNAPSHOT** — when a local snapshot binary exists it must open with
  ELF magic; when absent the partition deposits an honest SKIPPED token
  (the ELF check runs authoritatively in CI).
- **P-SELF** — banner gated behind the RAN check, five begin markers,
  four predecessor ok markers.

`make verify-phase-51` runs it; healthy tree is green and D1..D4 all red.

## Deviations

1. The gate file was corrupted twice during development (a bad
   `str.replace` duplicated content; an interactive patch inserted a DEL
   byte into YAML). Both were caught: the corruption by `bash -n`/YAML
   parse, the DEL byte by the workflow YAML parse error. Rewrote the
   drills section as one canonical python block and deduplicated three
   accidental partition/banner copies (found by grepping `# --- P-CONFIG`
   three times).
2. Drill D5 (snapshot ELF swap) recorded as intentionally omitted —
   see key-decisions; honest-skipped design makes it non-load-bearing
   locally.
