# Phase 45 — Deferred Items

## Discovered during 45-01 execution (2026-10-03)

| # | Item | Type | Evidence | Disposition |
|---|------|------|----------|-------------|
| D-45-DEF-01 | `internal/ontology/emitter` `TestEmitSubCategoryInstanceCount` fails: `SubCategory instances = 6, want 7` | Pre-existing test failure (out of 45-01 scope) | Reproduced identically at pre-plan commit `5bf9c35` in a throwaway worktree (`/d/CODE/ClaudeCode/p45-precheck`), 45-01's diff touches only `internal/ontology/store/*` and `internal/server/admin_platform_test.go`; emitter last modified in Phase 39 (`b789bfb`) | NOT fixed (scope boundary: only auto-fix issues caused by the current task's changes). Surface to phase verifier + 45-02: `verify-phase-45.sh` partition P2 (`go test ./internal/ontology/...`) will be red on this test until triaged — likely a stale count vs the v7 TBox (6 vs 7 SubCategory instances; STATE.md records TBox v3→v7 during 260928-nzk with "4 属性人工批准"). Candidate for a 45-02 assertion-choreography fix or a quick task. |
| D-45-DEF-02 | `internal/server/platform/verify_main.go` runtime-stale since Phase 44 (pre-44 fixture, owl:Class IRIs, raw counts, label-as-schema assertions) | Pre-existing stale harness | 45-RESEARCH Corrections + Pitfall 9; untracked in working tree | Deferred by plan (Open Question 2 RESOLVED): excluded from verify-phase-45.sh; BL-05 backlog candidate; do not fix in Phase 45. |

## Untracked working-tree noise (not produced by 45-01; left alone)

- `internal/server/skillforge-server-test-*` directories (5) — leftover hertz test temp dirs from earlier sessions.
- `internal/server/platform/verify_main.go` — see D-45-DEF-02.
