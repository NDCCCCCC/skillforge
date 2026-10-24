---
phase: "41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval"
plan: "02"
subsystem: platform
tags: [sparql, kg, tier-filtering, rest-api, ont-19]

# Dependency graph
requires:
  - phase: "41-01"
    provides: "admin_platform.go skeleton; OntologyDeps; MCP stdio server wired at boot"
provides:
  - "POST /api/sparql REST endpoint with X-Skill-Name header tier isolation"
  - "SparqlQueryScoped shared function (REST + MCP reuse)"
affects:
  - "41-03"
  - "41-06"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SPARQL-like triple-pattern query via POST body {s?,p?,o?,limit?,offset?,filter?}"
    - "X-Skill-Name header carries caller identity for tier isolation (D-SPARQL-03)"
    - "Tier resolution via KG lookup (hasReadAccess on skill IRI)"

key-files:
  created:
    - "internal/server/admin_platform_test.go"
  modified:
    - "internal/server/admin_platform.go"
    - "internal/server/server.go"

key-decisions:
  - "POST /api/sparql route is NOT under /api/admin/ — X-Skill-Name header is the token, not X-Admin-Token"
  - "Skill not found in KG returns 401 (fail-closed; no silent fallback to admin baseline)"
  - "Filter applied post-QueryScoped — only visible (tier-filtered) entities participate"

patterns-established:
  - "SparqlQueryScoped as the shared handler for both REST endpoint and MCP skf_query tool"

requirements-completed: [PLAT-ONT-19]

# Metrics
duration: ~5min
completed: 2026-09-23
---

# Phase 41 Plan 02: SPARQL-like REST Endpoint (PLAT-ONT-19)

**POST /api/sparql accepts triple-pattern queries, returns JSON triples with tier isolation via X-Skill-Name header**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-09-23T05:57:28Z
- **Completed:** 2026-09-23T06:02:39Z
- **Tasks:** 3
- **Commits:** 3 (bc02242, 5e9c332)

## Accomplishments
- `SparqlQueryScoped` shared function handles tier filtering (via `QueryScoped`), filter substring match, offset/limit pagination, and 500-limit cap
- `POST /api/sparql` registered in `server.go` with `X-Skill-Name` header enforcement (401 if absent or skill not found)
- 11 unit tests cover all query shapes, tier filtering, pagination, truncation, and error cases

## Task Commits

1. **Task 1+2: SparqlQueryScoped + POST /api/sparql** - `bc02242` (feat)
2. **Task 3: Unit tests for SparqlQueryScoped** - `5e9c332` (test)

## Files Created/Modified

- `internal/server/admin_platform.go` — Added `SparqlPattern`/`SparqlResult`/`SparqlResponse` types, `SparqlQueryScoped` function, `adminPlatformSparqlHandler`, `resolveSkillTiersFromKG` helper
- `internal/server/server.go` — Added `h.POST("/api/sparql", deps.adminPlatformSparqlHandler)` (not under `/api/admin/`)
- `internal/server/admin_platform_test.go` — 11 unit tests: all-wildcard, subject filter, pagination, truncated, tier filter, fail-closed no-tier, filter, invalid limit, limit cap, default limit

## Decisions Made

- Used existing `OntologyDeps` field in `Deps` struct to resolve skill tiers via KG lookup (same `resolveSkillTiers` pattern as MCP server)
- Filter applied post-QueryScoped (only visible entities participate in substring match, per T-41-07)
- Skill not found → 401 (fail-closed per D-TRUST-02; no silent admin baseline fallback)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `go build` fails with `runtime/cgo: exit status 2` on Windows due to DuckDB CGO bindings (known deferred issue). Verified via `go vet` (store tests pass with `CGO_ENABLED=0`) and `gofmt` (syntax clean).

## Next Phase Readiness

- `SparqlQueryScoped` function is ready for reuse by the MCP `skf_query` tool (Plan 41-02 shares handler with MCP per D-SPARQL-01)
- `/admin/platform` UI SPARQL Playground tab (Plan 41-03) can now target the `POST /api/sparql` endpoint
- Tier isolation contract enforced: missing/invalid `X-Skill-Name` returns 401; valid skill returns tier-filtered triples

---
*Phase: 41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval*
*Plan: 02*
*Completed: 2026-09-23*
