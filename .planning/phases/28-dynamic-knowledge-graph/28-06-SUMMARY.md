# Phase 28 Plan 06: InspectorRetrieval neighbors panel Summary

## Plan Overview
**Phase:** 28 | **Plan:** 06 | **Type:** execute | **Wave:** 3
**Status:** COMPLETE | **Requirements:** GRF-04

---

## Commits

| Task | Hash | Message |
|------|------|---------|
| T1 | `41cebec` | 28-06-T1: add neighbors panel to InspectorRetrieval |
| T2 | `c654cd9` | 28-06-T2: add 5 InspectorRetrieval neighbor integration tests |

---

## Files Added/Modified

| Path | Lines | Description |
|------|-------|-------------|
| `web/src/components/inspector/InspectorRetrieval.tsx` | +64/-1 | Add neighbors panel state + inline fetch + panel UI with Chinese edge-type labels |
| `web/src/components/inspector/InspectorRetrieval.test.tsx` | +98 | Add 6 neighbor panel tests (renders, labels, loading, degraded, collapse, error) |

---

## Test Results

**Command:** `npm test -- --run`

```
Test Files  70 passed (70)
Tests       535 passed (535)
Duration    16.12s
```

**InspectorRetrieval tests (9 total — 3 original + 6 new):**
| File | Tests | Status |
|------|-------|--------|
| `InspectorRetrieval.test.tsx` — original (tier dispatch) | 3 | ALL PASS |
| `InspectorRetrieval.test.tsx` — neighbors panel (Phase 28-06) | 6 | ALL PASS |

---

## Verification Command Output

```bash
$ cd web && npm run build
✓ built in 9.34s  (chunk-size warning is pre-existing)

$ cd web && npm test -- --run --testNamePattern="InspectorRetrieval"
Test Files  1 passed (1)
Tests       9 passed (9)
Duration    15.53s
```

---

## Deviations from Plan

### Rule 1 — Auto-fixed: handleClick async signature broke existing AgentInspector test
- **Found during:** T1 first test run
- **Issue:** `handleClick` was written as `async function` with `await` before the `setCenterView` dispatch. The existing `AgentInspector.test.tsx` test does not await `handleClick`, so `centerView` was still `'empty'` when the assertion ran. Also, `neighborsResult.edges` access crashed when the test mock returned the KB bases JSON (no `edges` field).
- **Fix (T1):** (1) Reverted `handleClick` to synchronous `function` with `void IIFE` for the fire-and-forget neighbors fetch — keeps the dispatch synchronous so existing non-awaited callers still work. (2) Added `?? []` guard on `neighborsResult.edges` accesses throughout the panel.
- **Files modified:** `web/src/components/inspector/InspectorRetrieval.tsx`
- **Commit:** `41cebec`

### Rule 2 — Auto-added: panel guard for error degradation
- **Found during:** T2 test "hides panel silently when neighbors API returns an error"
- **Issue:** The outer panel container (`{expandedChunkId && ...}`) was rendering even when the fetch errored and `neighborsResult` was `null`. The plan says "don't render the panel" on error — not just an empty panel.
- **Fix:** Changed guard to `{expandedChunkId && (neighborsResult || neighborsLoading) && (...)}` — panel is only mounted when there is actual data to show OR a fetch is in flight.
- **Files modified:** `web/src/components/inspector/InspectorRetrieval.tsx`
- **Commit:** `41cebec`

---

## Non-Deviations (Plan Executed Exactly)
- Edge type Chinese labels: `邻接` / `版本链` / `共现` / `引用共现` — VERIFIED (EDGE_TYPE_LABELS map matches plan spec)
- Panel shows "常被一起引用" header — VERIFIED
- Loading state shows "加载中…" — VERIFIED
- Degraded reason shown when `degraded=true` — VERIFIED
- Collapse on second click — VERIFIED
- Silent error degradation (panel hidden) — VERIFIED
- Inline fetch to `/api/knowledge/graph/neighbors?chunk_id=X&depth=1` — VERIFIED (no useGraphNeighbors hook duplication)
- No new npm packages — VERIFIED
- Zero TODO/FIXME in any new file — VERIFIED
- Existing InspectorRetrieval tests unchanged — VERIFIED (3/3 pass)
- Existing AgentInspector test unchanged — VERIFIED (still passes)

---

## Key Decisions Made

| Decision ID | Decision | Rationale |
|-------------|----------|-----------|
| D-28-06-HANDLECLICK-SYNC | `handleClick` is synchronous; neighbors fetch is fire-and-forget via `void IIFE` | Existing callers (AgentInspector test) do not await `handleClick`; the dispatch must be synchronous. The neighbors fetch does not affect the store state and does not need to block. |
| D-28-06-PANEL-GUARD | Panel guard is `(neighborsResult \|\| neighborsLoading)` | Ensures panel is not mounted at all when fetch errors and there is no data — matching the plan's "graceful degradation" intent. |

---

## Threat Flags

None — only adds new inline fetch and UI expansion in an existing component; no new endpoints, auth paths, or trust-boundary changes.
