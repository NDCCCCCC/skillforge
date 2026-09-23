---
phase: "41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval"
plan: "01"
subsystem: platform
tags: [mcp, stdio, mark3labs/mcp-go, kg, tier-filtering, PLAT-ONT-18]

# Dependency graph
requires:
  - phase: "40-data-governance-shacl-provenance-tbox-versioning"
    provides: "QueryScoped tier filtering; OntologyDeps struct; admin handler pattern"
affects:
  - "41-02"
  - "41-06"

# Tech tracking
tech-stack:
  added:
    - "github.com/mark3labs/mcp-go v1.1.0 (MCP server framework, stdio transport)"
  patterns:
    - "MCP stdio subprocess spawned at Go server boot as background goroutine"
    - "All tool calls route through QueryScoped for tier trust isolation (D-TRUST-01)"
    - "Admin API handlers mirror governance handler idiom (token gate + nil check + JSON envelope)"

key-files:
  created:
    - "internal/server/platform/mcp/server.go"
    - "internal/server/admin_platform.go"
  modified:
    - "cmd/server/main.go"
    - "internal/server/server.go"
    - "go.mod"
    - "go.sum"

key-decisions:
  - "mark3labs/mcp-go v1.1.0 — community standard MCP server; eino-ext only has client-side adapter (D-MCP-02)"
  - "5 tools exposed: skf_query, skf_skills, skf_chunk_for, skf_schema, skf_neighbors (D-MCP-03..06)"
  - "SKILLFORGE_MCP_SERVICE_SKILL mandatory at startup — no admin baseline bypass (D-TRUST-02)"
  - "MCP stdio subprocess runs in background goroutine; Go server communicates with MCP via admin HTTP routes only"

patterns-established:
  - "MCP stdio server as background goroutine alongside Hertz HTTP server"
  - "QueryScoped tier filtering on every KG read path (MCP tools + admin routes)"
  - "Admin platform route triad: status / tools / invoke — mirroring governance route patterns"

requirements-completed: [PLAT-ONT-18]

# Metrics
duration: ~15min
completed: 2026-09-23
---

# Phase 41 Plan 01: MCP Server with 5 KG Tools (PLAT-ONT-18)

**MCP stdio server spawned at Go boot, exposing 5 KG tools (skf_query/skf_skills/skf_chunk_for/skf_schema/skf_neighbors) with mandatory tier filtering via QueryScoped**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-23T05:42Z
- **Completed:** 2026-09-23
- **Tasks:** 3
- **Commits:** 3 (ff5e4b5, cb28d52, 9a3e54a)

## Accomplishments
- MCP stdio subprocess wired in cmd/server/main.go alongside SHACL boot validation (after ontologyKG restore)
- 5 tools registered via mark3labs/mcp-go: skf_query (tier-filtered triple pattern), skf_skills (Skill enumeration), skf_chunk_for (ECL chunk lookup), skf_schema (TBox classes/properties), skf_neighbors (N-hop graph expansion)
- SKILLFORGE_MCP_SERVICE_SKILL required at startup (D-TRUST-02: no admin baseline bypass); SKILLFORGE_MCP_ENABLED controls enable/disable
- Admin platform API: GET /status, GET /tools, POST /invoke — all token-gated, nil-safe (503 when MCP disabled)
- go vet clean on internal/server/platform/mcp/... and internal/server/

## Task Commits

1. **Task 1: MCP server package** - `ff5e4b5` (feat)
2. **Task 2: Wire subprocess boot in main.go** - `cb28d52` (feat)
3. **Task 3: Admin platform API handlers** - `9a3e54a` (feat)

## Files Created/Modified

- `internal/server/platform/mcp/server.go` — MCP server with 5 tools; NewServer/Serve/ListTools/CallTool/ServiceSkill methods; resolveSkillTiers tier resolution
- `internal/server/admin_platform.go` — Three admin handlers for MCP inspector panel; token-gated JSON envelope
- `cmd/server/main.go` — MCP subprocess boot after ontologyKG + SHACL validation; env var parsing for SKILLFORGE_MCP_SERVICE_SKILL + SKILLFORGE_MCP_ENABLED
- `internal/server/server.go` — MCPServer field added to Deps; three routes registered (/api/admin/platform/mcp/{status,tools,invoke})
- `go.mod` — Added github.com/mark3labs/mcp-go v1.1.0
- `go.sum` — Updated with transitive dependencies (jsonschema-go, spf13/cast, uritemplate, jsonschema v6)

## Decisions Made

- Used `mark3labs/mcp-go v1.1.0` over eino-ext MCP (client-only per eino-ext/components/tool/mcp/README)
- stdio transport (not Streamable HTTP) — local IDE/Claude Desktop integration is v1.4 primary consumer (D-MCP-01)
- SKILLFORGE_MCP_SERVICE_SKILL mandatory — D-TRUST-02 no admin baseline bypass enforced at startup
- MCP server runs in goroutine; admin API routes use MCPServer.CallTool for tool invocation (not in-process handler calls to avoid threading complexity)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `go build ./cmd/server/...` fails with `runtime/cgo: exit status 2` on Windows MSYS2 due to DuckDB CGO bindings — known deferred issue (gcc linking env_issue). Verified via `go vet` only (acceptable per existing project pattern).
- mark3labs/mcp-go transitive deps not in go.sum — resolved with `go mod tidy`.

## Next Phase Readiness

- MCP stdio server package and boot wiring complete (41-01 done)
- Next: 41-02 (SPARQL-like POST /api/sparql endpoint) can reuse the admin_platform.go pattern and the same QueryScoped tier filtering
- FE MCP Inspector tab (41-03..05) can now consume GET /tools and POST /invoke endpoints

---
*Phase: 41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval*
*Plan: 01*
*Completed: 2026-09-23*
