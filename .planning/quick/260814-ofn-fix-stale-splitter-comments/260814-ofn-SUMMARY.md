---
phase: 260814-ofn-fix-stale-splitter-comments
plan: 01
subsystem: knowledge/verdict
tags: [docs, comments, cleanup]
requirements:
  - QUICK-260814-OFN
dependency-graph:
  requires: []
  provides:
    - accurate splitter-status comments in indexer.go and verdict package
  affects: []
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - internal/knowledge/indexer.go
    - internal/verdict/grounding.go
    - internal/verdict/grounding_test.go
decisions:
  - "Splitter status recorded as: never scheduled by any phase; open backlog decision pending user input"
metrics:
  duration: "~5 min"
  completed: 2026-08-14
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick Task 260814-ofn: Fix Stale Splitter Comments Summary

**One-liner:** Corrected 5 stale comments/strings that falsely claimed a markdown hierarchical splitter was "deferred to Phase 3 (CMPL-06)" — the splitter was never scheduled, and CMPL-06 is actually the completed Phase 5 counter-evidence highlighting feature.

## What Was Done

Pure comment/operator-string corrections, zero behavior change:

| # | File | Location | Change |
|---|------|----------|--------|
| 1 | `internal/knowledge/indexer.go` | package doc (~line 33) | "deferred to Phase 3 (CMPL-06)" → splitter never scheduled by any phase; CMPL-06 is the Phase 5 counter-evidence feature completed in Plan 05-06 |
| 2 | `internal/knowledge/indexer.go` | maxDocBytes comment (~line 61) | "opt-in via the splitter in Phase 3" → no automatic splitter exists; open backlog decision |
| 3 | `internal/knowledge/indexer.go` | WARN string (~line 181) | "wait for the Phase 3 hierarchical splitter (CMPL-06)" → "Split the doc manually (no automatic splitter is scheduled)." — format preserved (single fmt.Fprintf arg, trailing `\n`, indentation unchanged) |
| 4 | `internal/verdict/grounding.go` | Normalize doc (~lines 29-36) | "chunker (markdown header splitter)" → chunks are the verbatim .md body (one-doc-one-chunk, no chunker exists); whitespace drift rationale reassigned to the LLM |
| 5 | `internal/verdict/grounding_test.go` | fixture comment (~line 107) | "markdown chunker passed them through verbatim" → "chunks are stored verbatim (one-doc-one-chunk, no chunker)" |

Untouched as required: `indexer.go:91` "KNOW-02 (Phase 3)" comment (correct), maxDocBytes value and skip logic, all other "Phase 3" references unrelated to the splitter, `.claude/worktrees/` copies.

## Verification

- `go build ./...` — passes
- `go test ./internal/knowledge/ ./internal/verdict/` — both pass (ok)
- `grep "markdown chunker|markdown header splitter|hierarchical splitter|Phase 3 (CMPL-06)" internal/ cmd/` — no matches
- `grep "CMPL-06" internal/knowledge/` — single remaining hit is the new corrected line explicitly stating "CMPL-06 is unrelated" (per plan-mandated replacement text)
- `git diff --stat` — 3 files, comment/string lines only, no logic changes

## Deviations from Plan

None — plan executed exactly as written.

## Commits

| Commit | Message |
|--------|---------|
| 70cd1e5 | docs(260814-ofn-01): correct stale splitter comments in indexer.go |
| 01fd467 | docs(260814-ofn-01): correct stale chunker comments in verdict package |

## Self-Check: PASSED

- internal/knowledge/indexer.go, internal/verdict/grounding.go, internal/verdict/grounding_test.go modified and verified present
- Commits 70cd1e5 and 01fd467 verified in git log
