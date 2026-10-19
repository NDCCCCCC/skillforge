---
phase: 47-debt-cleanup-reproducible-baseline
plan: 01
subsystem: knowledge-graph
tags: [go, determinism, graph, eino, testify, grf-02]

requires:
  - phase: 28
    provides: "ComputeEdges / Edge / EdgeGraph (internal/knowledge/graph_edges.go) and the GRF-02 determinism contract"
provides:
  - "Deterministic ComputeEdges: map-iteration order no longer decides keyword_cooccur edge direction"
  - "Total-order edge comparator (From, To, EdgeType, Weight) closing the unstable sort.Slice tie hazard"
  - "TestDeriveEdgesDeterministic hardened to 100-run whole-struct equality (REL-01 flake closed)"
  - "Measured pre/post flake evidence for the D-46-DEF-02 row cited by plan 47-06"
affects: [47-06, phase-48-ci-gate, phase-49-hermetic-suite, internal/knowledge, verify-phase-47.sh]

tech-stack:
  added: []
  patterns:
    - "map -> key slice -> sort.Strings -> consume, before any O(n^2) pairing that derives direction from slice order"
    - "sort.Slice comparators must be a total order over every field that can distinguish two entries"
    - "N-call whole-struct equality (N=100) instead of two-run per-field comparison for pure-function determinism gates"

key-files:
  created: []
  modified:
    - internal/knowledge/graph_edges.go
    - internal/knowledge/graph_edges_test.go

key-decisions:
  - "Sort `sources` by the `source` (filename) field rather than the representative chunkID — the source name is the map key and is unique per entry, so it is a strict total order with no tie to resolve (CONTEXT D-01 Claude's Discretion)."
  - "Extend the test's sort helper to the implementation's four-key comparator rather than keeping three — sorting on fewer keys than the implementation would let a Weight divergence slip past the equality check (the exact class of masking D-01c forbids)."
  - "Keep TestDeriveEdgesDeterministic a single top-level test (no t.Run) so the anti-vacuous `--- PASS` floor in verify-phase-47.sh P3/P5 stays calibrated."

patterns-established:
  - "Determinism gates on pure functions drive N independent calls against one reference result and compare the whole slice in a single assertion"
  - "Prohibition comments inside a test that name the forbidden API (InDelta / order-insensitive compare) so a future editor cannot launder a red run into green"

requirements-completed: [REL-01]

duration: 12min
completed: 2026-10-19
---

# Phase 47 Plan 01: Determinize ComputeEdges (REL-01) Summary

**Go map iteration order no longer decides `keyword_cooccur` edge direction in `ComputeEdges`; the GRF-02 determinism gate is now a 100-run whole-struct equality test instead of two lucky runs.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-10-19
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- **Root cause closed at the implementation site.** `sources` was built by `for src, tokens := range sourceTokens`; the `i < j` Jaccard pairing then took `From` from `sources[i]` and `To` from `sources[j]`, so slice order decided edge DIRECTION. Sorting `sources` by `source` before pairing removes the channel. `groups` got the same treatment (map -> key slice -> `sort.Strings` -> consume, mirroring the in-file `computeCitationEdges` precedent).
- **Comparator is now a total order.** `Weight` was added as the fourth key after `EdgeType`, closing the `sort.Slice`-is-unstable tie hazard for duplicate `(From, To, EdgeType)` tuples emitted by `citation_cooccur` across multiple audit rows.
- **The test now proves what it claims.** `TestDeriveEdgesDeterministic` computes one reference result and drives 100 further `ComputeEdges` calls, comparing the complete sorted `[]Edge` in a single `assert.Equal`. The four per-field assertions (including `assert.InDelta(..., 0)`) are gone from this test.
- **Flake measured, not assumed.** Pre-fix `-count=200` produced 34 failures on the first run and 50 on a second; zero of them were `Weight` mismatches. Post-fix `-count=200` exits 0.

## Evidence — the numbers plan 47-06 cites for D-46-DEF-02

All commands run in WSL (`wsl.exe -d Ubuntu-22.04`) at `/mnt/d/code/ClaudeCode/ai-agent` with `CGO_ENABLED=1` (go-duckdb requires cgo; Windows-native builds cannot link it).

### Pre-fix baseline (before any edit)

```
$ wsl.exe -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ -run "^TestDeriveEdgesDeterministic$" -count=200'
  -> FAIL   (grep -c -- "--- FAIL: TestDeriveEdgesDeterministic" = 34 on run 1, 50 on run 2)
  -> grep -c "Weight must match" = 0
```

Representative failure (direction flip, not a weight drift):

```
Error: Not equal:
       expected: "keyword_cooccur"
       actual  : "section_adjacent"
Messages: EdgeType must match at index 2
```

Every pre-fix failure landed on `From` / `To` / `EdgeType`. Zero on `Weight` — confirming the CONTEXT D-01 root cause (map order flipping direction) rather than float precision.

### Post-fix verification — exit codes

| Verification (plan §`<verification>`) | Exit code |
|---|---|
| `CGO_ENABLED=1 go test ./internal/knowledge/ -count=1` | **0** |
| `CGO_ENABLED=1 go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=200` | **0** (was 34 failures) |
| `CGO_ENABLED=1 go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=100` | **0** |
| `CGO_ENABLED=1 go test ./internal/knowledge/ -count=1 -shuffle=on` | **0** |
| `CGO_ENABLED=1 go test ./internal/knowledge/ ./internal/ontology/... -count=1 -shuffle=on` | **0** |
| `CGO_ENABLED=1 go build ./...` | **0** |
| `CGO_ENABLED=1 go vet ./internal/knowledge/` | **0** |

The `-count=1 -v` run emits exactly one `--- PASS: TestDeriveEdgesDeterministic` line (verified by `grep -c` = 1), satisfying the anti-vacuous `pass_lines` floor that plan 47-02's Task 1 filter is calibrated against.

### Acceptance-criteria checks

- `git diff --stat internal/knowledge/graph_edges.go` -> 1 file, +25/-2; no change to the `Edge` struct literal fields or the `ComputeEdges` signature line.
- `grep "sort.Slice(sources"` -> present; `sort.Strings(groupKeys)` -> present; comparator closure has four keys, last comparing `Weight`.
- `awk '/^func TestDeriveEdgesDeterministic/,/^}/ {print NR ":" $0}' internal/knowledge/graph_edges_test.go | grep -vE '[0-9]+:[[:space:]]*//' | grep -c "InDelta"` -> **0**.
- File-wide `grep -c "InDelta"` -> **4**: three legitimate sibling assertions (`section_adjacent` weight, `version_chain` weight, the 1/2-weight citation case — now at lines 158, 208, 308) plus the one *comment* inside the target test that names `InDelta` as forbidden, as required by action item 4.
- `git status --short` over the four plan-relevant paths shows no modification to `.planning/STATE.md` or `.planning/ROADMAP.md` (orchestrator-owned).

## Task Commits

Each task committed atomically:

1. **Task 1: Determinize source/group iteration and complete the edge comparator** - `0ecd9c3` (fix)
2. **Task 2: Harden TestDeriveEdgesDeterministic to N-run whole-struct equality** - `a05929a` (test)

**Plan metadata:** this SUMMARY.md, committed separately as `docs(47-01): complete plan 47-01 summary` (hash recorded at commit time — a doc commit cannot embed its own hash).

## Files Created/Modified

- `internal/knowledge/graph_edges.go` - `ComputeEdges` root-cause fix: sorted `groupKeys` slice for phase 1, `sort.Slice(sources, ...)` before the Jaccard pairing loop, `Weight` tie-break in the final comparator. `Edge` struct, `ComputeEdges` signature, GRF-02 doc comment, and `EdgeGraph.Neighbors` untouched.
- `internal/knowledge/graph_edges_test.go` - `TestDeriveEdgesDeterministic` rewritten: four-key sort helper, one reference computation, 100-call loop, single whole-slice `assert.Equal`, root-cause comment + forbidden-fix comment.

No files created. No new imports, no new dependencies, no package-manager installs.

## Decisions Made

1. **Sort `sources` by `source`, not by `chunkID`.** CONTEXT left the comparator to Claude's discretion. The source name is the `sourceTokens` map key and is unique per `srcInfo`, so ordering on it is a strict total order with no tie to break; ordering on `chunkID` would require a second key for multi-document sources and would entangle direction with corpus layout rather than with the identity being deduplicated.
2. **Extended the test's sort helper to four keys.** The plan's action item 1 permitted "the same four keys as the implementation", which is what was done. A three-key helper would still have passed pre-fix (the flake was direction, not Weight) but would have left a real Weight regression invisible to the new whole-slice equality — the same masking failure D-01c forbids.
3. **Kept a single top-level test (no `t.Run`).** Per action item 3: the gate's `pass_lines` floor is calibrated against top-level test names, so 100 subtests would inflate the count and defeat the anti-vacuous check.

## Deviations from Plan

None - plan executed exactly as written. No auto-fixes were needed; no blockers were hit.

## Issues Encountered

- The plan's `<verify>` blocks spell the WSL path as `/mnt/d/CODE/ClaudeCode/ai-agent` while the project rules spell it `/mnt/d/code/ClaudeCode/ai-agent`. Used the lowercase form throughout (it is the verified-working form; WSL drvfs resolves either, but the lowercase path is the documented convention).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **REL-01 is closed** and its evidence is recorded above for the D-46-DEF-02 row in `47-DEBT-DISPOSITION.md` (consumed by plan 47-06 / the REL-04 traceability work).
- **47-02** (verification-script hardening) can rely on: `-count=100` green, one `-shuffle=on` green, and exactly one `--- PASS: TestDeriveEdgesDeterministic` line — the inputs its anti-vacuous `pass_lines` threshold needs.
- **47-05 / 47-06** can build the P5 determinism-strength segment (`-count=100` + `-shuffle=on`) on a green command; note the `-count=200` pre-fix failure rate was 17–25%, so any *future* determinism threshold should be set with that failure-rate context in mind rather than assuming a single run is informative.
- **Phase 48's public CI gate** is no longer blocked by this flake: `./internal/knowledge/` is green under both plain and shuffled ordering.
- No blockers.

---
*Phase: 47-debt-cleanup-reproducible-baseline*
*Completed: 2026-10-19*
