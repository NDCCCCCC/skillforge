---
phase: quick-260929-d5w3
plan: d5w3
subsystem: ui
tags: [knowledge-graph, svg, viewBox, pan-zoom, citations, focus, react, vitest]

# Dependency graph
requires:
  - phase: quick-260929-d5w2
    provides: graph fullscreen overlay (portal + shared graphBody), prettySourcePath display layer
  - phase: phases 29/28
    provides: data-policy UIMessage part carrying Verdict.citations; radial tier/doc graph canvas
provides:
  - lib/graph-focus — citation→graph focus derivation (extractGraphFocus / hasAnsweredTurn / computeFocusMatch / basenameOf)
  - GraphCanvas 仅当前问题 focus mode + persisted 仅当前问题/全库 toggle (graph-focus-toggle)
  - GraphCanvas dynamic content-fitted viewBox + wheel zoom (0.4x–4x) + pointer-drag pan + 适应 reset (graph-fit-btn)
affects: [knowledge-graph UX, admin graph consumers, future citation-source wire enrichment]

# Tech tracking
tech-stack:
  added: []  # zero new dependencies — hand-rolled zoom/pan per constraint
  patterns:
    - "view=null state means 'fit to measured bbox' — the viewBox auto-tracks layout changes until the user pins an explicit zoom/pan"
    - "non-passive wheel listener via ref useEffect (React root wheel is passive; preventDefault needs manual subscribe)"
    - "localStorage-persisted UI mode preference with try/catch fallback"

key-files:
  created:
    - web/src/lib/graph-focus.ts
    - web/src/components/graph/GraphCanvasFocus.test.tsx
  modified:
    - web/src/components/graph/GraphCanvas.tsx
    - web/src/components/graph/KnowledgeGraphBrowser.tsx
    - web/src/components/layout/DetailPanel.tsx

key-decisions:
  - "Focus source of truth = latest assistant verdict's citations only (no fallback to older turns) — a citation-less newer verdict means 'no focus', never resurrect the previous answer's citations"
  - "Wire Citation has NO source field today (internal/verdict/citation.go = chunk_id/span/tier) — source basename match implemented tolerantly and activates when the BE enriches citations (plan brief assumed source exists; documented as deviation)"
  - "Non-focus tiers 淡显 (opacity 0.15) instead of collapse — layout stays stable so focus toggling never re-flows the graph"
  - "view=null sentinel for fit-mode keeps filter changes/corpus growth auto-fitting without effect wiring"

patterns-established:
  - "data-focus attr on graph nodes marks focus membership for tests (true/false when focus mode active, absent otherwise)"

requirements-completed: []

# Metrics
duration: 15min
completed: 2026-09-29
---

# Quick 260929-d5w3: Graph Focus + Canvas Fit Summary

**知识图谱两个 UX 缺陷修复：回答后图谱默认聚焦本次引用文档（仅当前问题/全库可切换），画布 viewBox 按真实内容包围盒自适应 + 滚轮缩放/拖拽平移，全屏底弧不再被裁。**

## Performance

- **Duration:** 15 min (03:13–03:27 UTC)
- **Started:** 2026-09-29T03:13:12Z
- **Completed:** 2026-09-29T03:27:15Z
- **Tasks:** 2/2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- **Task 1 问题聚焦** (`9416b3b`): DetailPanel extracts the latest assistant verdict's citations from `messages` (tolerant parse mirroring MessageBubble.tsx:312-317) → `focus` prop flows KnowledgeGraphBrowser → GraphCanvas (same props through the fullscreen portal). Cited docs render opacity 1, enlarged (r=11) with a dashed tier-coloured highlight ring + `data-focus="true"`; non-focus docs fade to 0.12 (hover reveals at 0.35); focus tiers stay lit, others 淡显 0.15. Toggle `data-testid="graph-focus-toggle"` (仅当前问题/全库, aria-pressed) persists via localStorage `skillforge.graph.focusMode`.
- **Task 2 画布自适应** (`dafe594`): `measureContentBBox` measures every positioned tier/doc ± label margins (±60/±40 tier, ±80/±40 doc) + 24 pad → dynamic viewBox with `preserveAspectRatio="xMidYMid meet"`; svg `h-full w-full` inside a flex column so the fullscreen overlay fills the viewport and the bottom arc is always visible. Wheel zoom 0.4x–4x anchored at cursor (non-passive listener), pointer-drag pan from the background pan-surface rect (inflated 3× so empty space stays grabbable zoomed in), double-click or `data-testid="graph-fit-btn"` 适应 resets to fit. Zero new dependencies.

## Verification

- `npx tsc --noEmit` — clean (exit 0)
- vitest: 12 new tests in GraphCanvasFocus.test.tsx (① focus opacities/data-focus/ring ② toggle + localStorage persistence + remount ③ refusal empty note 当前回答无引用原文 — 显示全库 ③b source-basename match ④ 100-doc viewBox coverage — bottom arc y+40+24 inside viewBox, height > 640)
- No regression: GraphCanvas (10), GraphTree (5), KnowledgeGraphBrowser (8), AdminFeedbackTabs — all green; full suite 654/655 (1 pre-existing failure, see Deferred Issues)
- `npm run build` (vite) — green in 15.9s
- Browser live acceptance (fullscreen bottom arc, wheel zoom, drag pan, post-answer focus) — left to orchestrator per task spec

## Deviations from Plan

**1. [Rule 3 - Blocking/Spec mismatch] Citation wire has no `source` field**
- **Found during:** Task 1
- **Issue:** Task brief states citations carry `source(路径)`; the actual wire (internal/verdict/citation.go, pinned by contract test) carries only `{ chunk_id, span, tier }`.
- **Fix:** Implemented chunk_id matching as primary + tolerant optional `source` basename matching (reads `citation.source` when present, adds nothing to the wire). Doc matching also covers the graph's own DocNode shape (`id` = "tier::source", `filename` = basename).
- **Files modified:** web/src/lib/graph-focus.ts
- **Commit:** 9416b3b

## Known Stubs

None — all data paths are wired to real message/citation state.

## Deferred Issues

- **D-d5w3-01 (pre-existing, out of scope):** `src/__tests__/chunk-lookup.test.ts > fetchChunkContent > resolves { text, tier } on 200` fails — commit `dfc5231` (quick-260929-d5w2) added `source` to the `fetchChunkContent` return without updating its test. Not caused by d5w3 (verified: no d5w3 commit touches either file). Details + suggested fix in `deferred-items.md`.

## Commits

- `9416b3b` fix(quick-260929-d5w3): focus knowledge graph on the current answer's citations
- `dafe594` fix(quick-260929-d5w3): dynamic viewBox fit + wheel zoom / drag pan on graph canvas

## Self-Check: PASSED

- All 5 source files + 2 planning files verified on disk (FOUND)
- Both commit hashes verified in git log (9416b3b, dafe594)
- web/src working tree clean after commits (0 modified; planning docs intentionally uncommitted per task constraints)

