---
phase: quick-260928-jxb
plan: 01
subsystem: admin-console
tags: [frontend, react, governance, platform, mcp, bugfix, null-guard]
requires:
  - Phase 40-07a governance API wrappers (governanceApi.ts)
  - Phase 41-05 PlatformTab lifted skillName state
  - Phase 43-01 forward-declared deps in cmd/server/main.go
provides:
  - null→[] normalization choke point in governanceApi.ts (warnings/quarantined/nodes/edges/entries)
  - shared Skill selector in PlatformTab (data-testid=platform-skill-select, auto-default)
  - deps.MCPServer wired at boot (admin MCP status/tools/invoke un-503'd)
affects:
  - /admin 治理 tab (no longer white-screens on Go nil-slice nulls)
  - /admin 平台 tab (SPARQL + Hybrid submit real X-Skill-Name)
  - GET /api/admin/platform/mcp/status (returns payload when MCP enabled)
tech-stack:
  added: []
  patterns:
    - normalize Go nil-slice nulls at the fetch-wrapper choke point (`?? []`), not at consumption sites
    - store-fed Skill selector with never-empty auto-default (useEffect on [skillName, skillNames])
key-files:
  created:
    - web/src/components/admin/governance/governanceApi.test.ts
    - web/src/components/admin/platform/PlatformTab.test.tsx
  modified:
    - web/src/components/admin/governance/governanceApi.ts
    - web/src/components/admin/platform/PlatformTab.tsx
    - cmd/server/main.go
    - .planning/quick/sf44-start.sh (untracked env helper, edited in place, intentionally NOT committed)
decisions:
  - normalize nulls in the API wrappers (single choke point) rather than guarding every consumption site — matches the walkthrough recommendation and the file's existing error-envelope-normalization role
  - one shared selector serves SPARQL + Hybrid views; MCP Inspector untouched (server-scoped)
  - failed skills excluded from selector options (absent from KG; BE would 401)
  - dead `if deps != nil` backref deleted rather than kept — the patch it promised (MCPServer wiring) is what this task lands
metrics:
  duration: ~12 min
  completed: 2026-09-28
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick Task 260928-jxb: Admin Console 3-Bug Fix Summary

**One-liner:** 治理 tab null-guard + PlatformTab shared Skill selector + deps.MCPServer boot wiring — 3 atomic commits, all gates green.

## What Was Built

### Task 1 — BUG-1 (P0): governance API null-array normalization — `99361f8`

Go nil slices serialize to JSON `null` while the FE types claim arrays; `warnings:null` / `edges:null` crashed ShaclReportView / ProvenanceGraphView via `.length`/`.some` on null, killing the whole AdminDashboard (no error boundary). Normalized at the fetch-wrapper choke point in `governanceApi.ts`: `warnings ?? []`, `quarantined ?? []`, `nodes ?? []`, `edges ?? []`, `entries ?? []`. Consumption sites untouched. 4 vitest regression tests pin the exact live payloads from the 2026-09-28 walkthrough (TDD: RED 3-fail → GREEN 4-pass).

### Task 2 — BUG-2 (P1): PlatformTab shared Skill selector — `e2738c9`

`skillName` was initialized `''` with NO selector, so SPARQL Run Query and Hybrid 搜索 always sent `X-Skill-Name: ''` → 401. Added a shared selector row (`data-testid="platform-skill-select"`, KGQueryPanel idiom) between the view switcher and view bodies; one control drives both views (MCP Inspector is server-scoped, untouched). Auto-defaults to the first `status === 'ok'` store skill via useEffect (`''` = guaranteed 401, must never persist); failed skills excluded; 加载中 placeholder when store empty. Existing SparqlPlaygroundView/HybridDebugView props and Ctrl+Enter deps re-attach unchanged. 3 vitest tests (TDD: RED 3-fail → GREEN 3-pass).

### Task 3 — BUG-3 (P1) + TASK-4: deps.MCPServer wiring + start-script GLM switch — `8390af1`

Phase 43-01 leftover: `if deps != nil { deps.MCPServer = mcpSrv }` (main.go ~993) ran before `deps` was assigned (~1059) and `mcpSrv` was branch-scoped — dead code; `/api/admin/platform/mcp/status|tools|invoke` always saw nil → 503. Fix: hoisted `var mcpSrv *mcp.Server` above the `mcpEnabled` branch, switched `NewServer` to assignment form, deleted the dead block, and wired `MCPServer: mcpSrv,` as the last field of the `deps = &server.Deps{...}` literal (nil-posture comment, same as ECL/Ontology). TASK-4 (env-only, untracked): `.planning/quick/sf44-start.sh` now `unset LOCAL_BASE_URL LOCAL_API_KEY LOCAL_MODEL` after sourcing `.env` so boot falls back to GLM_* while the LOCAL Unsloth endpoint is down (precedence LOCAL_* > GLM_* > default, main.go:302-304). Script verified still `??` untracked.

## Commits

| Commit | Task | Content |
|--------|------|---------|
| `99361f8` | 1 | fix(quick-260928-jxb): governance null-guard + 4 tests |
| `e2738c9` | 2 | fix(quick-260928-jxb): PlatformTab skill selector + 3 tests |
| `8390af1` | 3 | fix(quick-260928-jxb): deps.MCPServer wiring (message amended post-shell-mangle) |

## Verification Results

| Gate | Result |
|------|--------|
| `npx tsc --noEmit` | CLEAN |
| `npx vitest run src/components/admin/governance/governanceApi.test.ts` | 4/4 pass |
| `npx vitest run src/components/admin/platform/PlatformTab.test.tsx` | 3/3 pass |
| `npx vitest run src/components/admin` (regression net) | 55/57 — 2 failures are PRE-EXISTING stale pins (see Deferred Issues) |
| WSL `go build ./cmd/server` | PASS |
| WSL `CGO_ENABLED=1 go test ./internal/server/... -count=1` | ALL PASS (incl. full SPARQL + Hybrid suite) |
| `grep -n "MCPServer: mcpSrv" cmd/server/main.go` | line 1168 |
| New test files in commits | verified via `git show --stat` (both `create mode 100644`) |
| Tracked tree clean after commits | 0 modified/untracked under web/src + cmd/server |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Stale comment] Updated the `var deps` forward-declaration comment**
- **Found during:** Task 3
- **Issue:** The comment above `var deps *server.Deps` described the exact mechanism being deleted ("MCP setup block can stash a back-reference") — leaving it would document removed behavior.
- **Fix:** Rewrote to describe the actual flow (mcpSrv hoisted, wired into the literal's MCPServer field). Comment-only; no reordering.
- **Files modified:** cmd/server/main.go
- **Commit:** 8390af1

**2. [Process - commit hygiene] Task 3 commit message amended**
- **Issue:** Backticks in the `git commit -m` message were consumed by shell command substitution (message body lost two inline code spans).
- **Fix:** `git commit --amend -F -` with a quoted heredoc; final hash `8390af1`. No code change.

Otherwise: plan executed exactly as written.

## TDD Gate Compliance

Both tdd tasks ran true RED→GREEN (Task 1: 3/4 fail before implementation; Task 2: 3/3 fail before implementation). Per the orchestrator's explicit "3 atomic commits" instruction, test + implementation landed in ONE commit per task (`fix(...)` prefix) rather than separate `test(...)`/`feat(...)` commits — RED runs are reproduced in the transcripts above.

## Known Stubs

None. (The selector's 加载中 placeholder is a UI loading state driven by live `store.skills`, not a data stub.)

## Deferred Issues

**Pre-existing stale FE tab-count pins (NOT this plan's regression):** `OntologyTab.test.tsx:113` and `PipelineTab.test.tsx:121` assert `toHaveLength(9)` on the admin tab bar, but AdminDashboard has rendered **11** tabs since Phase 40-07a (治理) and Phase 41-04 (平台). Proven pre-existing: `git show 3322396:web/src/components/admin/AdminDashboard.tsx` (base commit) already contains both tabs — identical to HEAD. Out of scope per the scope boundary; full details + suggested fix in `deferred-items.md`.

## Threat Flags

None found — no new network endpoints, auth paths, file access patterns, or trust-boundary schema changes. (BUG-3 fix actually closes a surface: admin MCP routes previously degenerate-503'd; they now expose the already-token-gated Phase 41 handlers as designed.)

## Self-Check: PASSED

- All 6 files exist on disk (4 tracked artifacts + 2 new test files + untracked start script)
- All 3 commits present (`99361f8`, `e2738c9`, `8390af1`); both new test files confirmed inside their commits
- Tracked working tree clean for web/src + cmd/server
- sf44-start.sh remains untracked (`??`), edited in place per plan
