---
phase: 51-release-engineering
plan: 01
status: complete
subsystem: release
requires:
  - "51-CONTEXT.md D-51-01/02/05/06"
provides:
  - ".goreleaser.yaml (linux+windows/amd64, CGO_ENABLED=1, single server binary)"
  - "snapshot build proven: goreleaser check OK + linux/amd64 ELF + stub /api/skills=200"
affects:
  - ".goreleaser.yaml"
tech-stack:
  - goreleaser v2
  - CGO
key-files:
  - .goreleaser.yaml
key-decisions:
  - "Windows/amd64 target included but flagged: runner mingw-vs-duckdb linkability is an open risk (documented in config comment + context); if it fails on the real tag push, drop the windows target there and record it."
actuals:
  tokens: 14000
  tasks: 3
  commits: 1
---
## Accomplishments
- .goreleaser.yaml (v2) builds CGO_ENABLED=1 linux+windows/amd64 binaries of ./cmd/server; goreleaser check validates it and a WSL snapshot build produced a 98.6MB linux/amd64 ELF whose stub-mode smoke test served /api/skills=200 with a real 'mode':'stub' data-policy frame.


## Summary

`.goreleaser.yaml` (v2) defines the packaging: single server binary from
`./cmd/server`, linux+windows/amd64, CGO_ENABLED=1 (go-duckdb), tar.gz
archives with zip override for windows, checksums, snapshot template, and
a release body whose header carries the project positioning + stub-mode
caveat and whose footer is a four-line quick start.

Verified locally (WSL, go1.27.3 + goreleaser v2 installed via go install):

- `goreleaser check` — 1 configuration file(s) validated.
- `goreleaser build --snapshot --clean --single-target` — linux/amd64
  build succeeded (19s compile, 98,666,880 B, stripped, dynamically
  linked, ELF magic confirmed).
- Snapshot binary smoke (stub mode, real embedding endpoint): server up,
  `[WARN] ... STUB LLM MODE`, `/api/skills` = 200, one `data-policy` frame
  per chat, payload carries `"mode":"stub"` — the same path as Phase 49
  UAT test 1, now from the packaged binary.

## Deviations

1. (D-51-06 honesty note) The stub smoke uses the local real embedding
   endpoint (fastembed on the host) — the exact Phase 49 UAT path. The
   "download from the Releases page" leg remains manual-only (H-51-2).
