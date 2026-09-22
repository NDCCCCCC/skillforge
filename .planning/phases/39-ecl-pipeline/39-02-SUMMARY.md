---
phase: 39-ecl-pipeline
plan: 02
subsystem: database
tags: [milvus, schema, rag, delete-expr, watermark, ecl]

# Dependency graph
requires:
  - phase: 38-foundation-tbox-kg
    provides: ontology boot baseline (no direct code dep; 39-02 only touches the Milvus layer)
  - phase: 39-01 (same phase)
    provides: KG store delete primitives + TBox v2 contract (parallel wave-1 sibling; no shared files)
provides:
  - 11-field Milvus collection schema with 4 native ECL fields (source_idx/section/section_idx/section_count)
  - retriever.Document extended with 4 zero-value-defaulted ECL fields (D-Milvus-02 legacy contract)
  - SeedDocuments 11-column insert path (NewColumnVarChar discipline kept)
  - milvus.DeleteByExpr — caller-built marker-scoped purge primitive (Plans 05/06 seam)
  - MilvusClient.GetMaxIDCollection — collection-wide ID watermark (Plans 05 seam)
affects: [39-03, 39-05 (ECL chunk seeding), 39-06 (boot marker-scoped purge), 39-04 (Document field population)]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "Single-home width consts: schema VarChar widths derive from config.go consts, no literals at use sites"
    - "Ownership-marker purge: section_count > 0 marks ECL-owned rows; DeleteByExpr keeps mixed-tier purge scope exact"
    - "Mock-client hermetic tests: embed client.Client, unmocked methods panic (recreateMock/purgeMock discipline)"

key-files:
  created:
    - internal/retriever/milvus/delete_expr.go
    - internal/retriever/milvus/seed_test.go
  modified:
    - internal/retriever/retriever.go
    - internal/retriever/milvus/config.go
    - internal/retriever/milvus/seed.go
    - internal/retriever/milvus/client.go
    - internal/retriever/milvus/delete_tier_rows_test.go

key-decisions:
  - "39-02 Milvus schema 11 fields: +source_idx VarChar(64) / section VarChar(256) / section_idx Int64 / section_count Int64 appended after effective_to (D-Milvus-01 NATIVE fields, not JSON-in-source); widths single-homed in config consts; deploy requires SKILLFORGE_MILVUS_RECREATE=1 once"
  - "39-02 DeleteByExpr: caller-built expr passes through VERBATIM (helper never interpolates; %q escaping is caller duty per filter.go:55-59 precedent); empty/nil expr rejected before any DML; mechanics = DeleteTierRows verbatim (Has no-op -> Load -> Delete -> Flush)"
  - "39-02 GetMaxIDCollection: always-true `id >= 0` expr watermark — ECL chunk-ID allocation must start above the collection-wide max (per-tier GetMaxID under-estimates; Pitfall 5 duplicate-generation class)"
  - "39-02 truncateRunes applied to the two new VarChar columns (source_idx/section) so over-width values cannot fail the insert — same silent-cap discipline as content/source"

patterns-established:
  - "Pattern: marker-scoped purge — expr `tier in [X] and section_count > 0` (ECL) / `== 0` (legacy boot); helper never interpolates"
  - "Pattern: collection-wide watermark before ID allocation — never per-tier GetMaxID for cross-tier inserts"

requirements-completed: [ONT-13]

# Metrics
duration: 13 min
completed: 2026-09-22
---

# Phase 39 Plan 02: Milvus Layer ECL Extension Summary

**11-field Milvus schema with 4 native ECL section fields (D-Milvus-01), zero-value legacy seeding (D-Milvus-02), DeleteByExpr marker-scoped purge, and GetMaxIDCollection collection-wide watermark — the two seam functions Plans 05/06 inject**

## Performance

- **Duration:** 13 min
- **Started:** 2026-09-22T01:15:31Z
- **Completed:** 2026-09-22T01:29:01Z
- **Tasks:** 2
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments
- Collection schema extended 7 → 11 fields after `effective_to`: `source_idx` VarChar(64), `section` VarChar(256), `section_idx` Int64, `section_count` Int64 — all NATIVE columns (Phase 41 hybrid retrieval can filter on them), widths single-homed in `DefaultSourceIdxMaxLen` / `DefaultSectionMaxLen` consts
- `retriever.Document` gained the 4 zero-value-defaulted fields with the D-Milvus-02 contract documented: legacy producers seed `source_idx=""`, `section_idx=0`, `section_count=0`; `section_count > 0` is the ECL ownership marker (DocumentMeta untouched — graph contract unchanged)
- `SeedDocuments` now inserts 11 columns in schema order (NewColumnVarChar for strings, never NewColumnString); Insert → Flush → Load sequence untouched
- `milvus.DeleteByExpr(ctx, mc, cfg, expr)` — the mixed-tier purge escape hatch (RESEARCH Pitfall 1): DeleteTierRows mechanics verbatim, caller-built expr passed verbatim with a godoc-mandated %q escaping duty (T-39-01 mitigation)
- `MilvusClient.GetMaxIDCollection(ctx)` — always-true `id >= 0` watermark so ECL ID allocation can never collide with boot-seeded rows (RESEARCH Pitfall 5 / 2026-09-15 duplicate-generation incident class)

## Task Commits

Each task was committed atomically:

1. **Task 1: Schema +4 native fields, Document +4 fields, SeedDocuments +4 columns** - `68b44e0` (feat)
2. **Task 2: DeleteByExpr helper + GetMaxIDCollection watermark** - `4a55a9b` (feat)

## Files Created/Modified
- `internal/retriever/milvus/delete_expr.go` - NEW: DeleteByExpr marker-scoped purge helper (HasCollection no-op → LoadCollection → Delete → Flush)
- `internal/retriever/milvus/seed_test.go` - NEW: hermetic seedMock suite — 11-column insert contract, legacy zero-values, populated-section + truncation, fresh-deploy schema shape
- `internal/retriever/retriever.go` - Document +SourceIdx/+Section/+SectionIdx/+SectionCount with D-Milvus-02 ownership-marker contract
- `internal/retriever/milvus/config.go` - +DefaultSourceIdxMaxLen=64, +DefaultSectionMaxLen=256 single-home consts
- `internal/retriever/milvus/seed.go` - EnsureCollection 11-field schema + RECREATE gate godoc; SeedDocuments +4 column slices
- `internal/retriever/milvus/client.go` - +GetMaxIDCollection with Pitfall 5 godoc
- `internal/retriever/milvus/delete_tier_rows_test.go` - +5 DeleteByExpr tests (verbatim expr, no-op, empty-expr, nil-client, error propagation) +2 GetMaxIDCollection tests

## Decisions Made
- **Empty-expr guard in DeleteByExpr** (plan-specified mechanics plus one defensive guard): an empty/whitespace expr is rejected before any backend call and the error names the helper duty. Mirrors DeleteTierRows' empty-tiers guard; Milvus would reject it server-side anyway, but the failure must read as a caller-contract error, not an SDK error. A "delete everything" convenience is deliberately not offered.
- **truncateRunes on the new VarChar columns**: over-width `source_idx`/`section` values are rune-capped to the single-home widths (same silent-cap discipline as content/source), so an over-width value degrades to truncation instead of failing the whole insert batch.
- **seed_test.go created new**: the plan's file list names seed_test.go but no seed test existed in the suite — created with the established hermetic mock pattern (recreateMock/purgeMock discipline: embed client.Client, unmocked methods panic) and the schema assertions the plan asked to "update for 11 fields" now live there.
- **SectionIdx semantics pinned in Document godoc**: 1-based within its note, first-seen order (matches the spike ChunkMeta contract); 0 = legacy.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `gofmt -l` flags pre-existing formatting drift in `internal/retriever/milvus/get_max_id_test.go` and the WIP-dirty `internal/retriever/mongo/*` files — left untouched (out of scope; scope boundary rule). Only this plan's files were formatted.

## Verification

- `go test ./internal/retriever/... -count=1` → ok (milvus + mongo), 12/12 purge/watermark tests pass, the 5 pre-existing DeleteTierRows tests untouched and green
- `go build ./...` → clean (Document struct change compiles across indexer / mongo / graph builder consumers, including the pre-existing WIP files)
- `go test ./... -count=1` (full repo) → all packages ok, zero regressions
- Grep confirmation: seed.go carries `source_idx`/`section_idx`/`section_count` fields + columns; delete_expr.go carries `DeleteByExpr`
- VarChar landmine honored: all string columns via `entity.NewColumnVarChar` (spike-findings: DataType String=20 vs FieldTypeVarChar=21)

## Deployment Note (operator-facing)

The schema change is NOT backward compatible with pre-39 collections. Deploy sequence (existing door, no new migration code): boot once with `SKILLFORGE_MILVUS_RECREATE=1` (drop + re-create + full re-embed of the 202-file corpus), then unset. Plan 06 documents this in `.env.example`.

## User Setup Required

None - no external service configuration required (code + tests are hermetic; live Milvus re-embed happens via the documented RECREATE boot).

## Next Phase Readiness
- Plans 05 (ECL chunk seeding) and 06 (boot marker-scoped purge) can consume `DeleteByExpr` / `GetMaxIDCollection` / the 11-column `SeedDocuments` without re-touching milvus package internals
- Plan 04 (indexer routing) populates the new Document fields on the ECL path; legacy path compiles unchanged against the extended struct
- ONT-13's Milvus half ("indexes chunks to Milvus with consistent metadata") is now writable; ONT-13 completes with the phase's ECL pipeline plans

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*

## Self-Check: PASSED

- All 7 key-files verified on disk (`[ -f ]` checks)
- Both task commits verified in git log (`68b44e0`, `4a55a9b`)
- `func DeleteByExpr` present in delete_expr.go; `GetMaxIDCollection` present in client.go
- Plan-level verification re-run: `go test ./internal/retriever/... -count=1` ok, `go build ./...` clean, full `go test ./...` ok
