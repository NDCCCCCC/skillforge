---
phase: 26-kb-registry-foundation
plan: 08
subsystem: web-admin-kb
tags: [kb-registry, admin-surface, kb-m-02, tier-badge, registry-projection, n-tier, zero-hardcode, tdd, fe]
requires:
  - "GET /api/knowledge/bases public projection (26-05) — the FE's single display-name source"
  - "GET/POST /api/admin/kb + POST /api/admin/kb/:slug/deactivate (26-05) — the token-gated wire the admin view drives"
  - "internal/kb/slug.go slugPattern (`^[a-z][a-z0-9-]{0,19}$`) — the rule the FE form pre-flights"
provides:
  - "/admin 'Knowledge Bases' tab — the repo's first admin view with mutation verbs (create + confirm-gated deactivate), every request X-Admin-Token'd, backend verdict rendered verbatim"
  - "web/src/lib/kb-bases.ts — module-cached registry projection client: fetchKnowledgeBases (never throws, [] on failure/503/malformed), displayNameFor (bracketed), firstSlugByRole, useTierLabel hook, TIER_FALLBACK_LABELS"
  - "TierBadge + graph node labels resolve registry display names; unknown/legacy-CN slugs degrade to `[slug]`, empty tier hides the badge"
  - "Tier = string across the FE type layer (TIERS kept as the 4-slug seed fixture) — N knowledge bases are first-class"
  - "FE residual tier hardcode zeroed: chunk-lookup fallback, TITLE_DIR_TO_TIER, `?? '权威'`, `defaultTier = '权威'`, TIER_COLOR CN keys"
affects:
  - web (26-10 zero-hardcode audit: the CN whitelist now has exactly two annotated homes — kb-bases TIER_FALLBACK_LABELS and the SC-6 disclaimer copy)
  - Phase 28 (graph layout upgrade inherits N-tier safety + registry labels; layout() untouched here)
tech-stack:
  added: [] # zero new npm dependencies (fireEvent, not user-event — not installed)
  patterns:
    - "static seed-fallback map + async registry override: sync first paint from the offline map, projection display_name wins once loaded"
    - "only-set-state-when-the-visible-label-changed guard — keeps the async override act-clean in the 60+ pre-existing suites"
    - "slug + role vocabulary in the FE mirrors internal/kb (slug regex literal cited to slug.go; roles from Roles())"
    - "proxy-gate honesty: recorded when a grep's intent holds but its letter conflicts with another requirement in the same plan"
key-files:
  created:
    - web/src/lib/kb-bases.ts
    - web/src/components/admin/KnowledgeBaseList.tsx
    - web/src/components/admin/KnowledgeBaseList.test.tsx
    - web/src/components/admin/AdminDashboard.test.tsx
    - web/src/components/common/TierBadge.test.tsx
    - web/src/lib/kb-bases.test.ts
    - web/src/components/graph/GraphCanvas.test.tsx
    - web/src/components/graph/GraphTree.test.tsx
    - web/src/components/chat/parts/SourceDocumentPart.test.tsx
    - web/src/components/inspector/InspectorRetrieval.test.tsx
  modified:
    - web/src/types/verdict.ts
    - web/src/types/knowledge.ts
    - web/src/types/wire-types.ts
    - web/src/types/wizard.ts
    - web/src/lib/chunk-lookup.ts
    - web/src/components/common/TierBadge.tsx
    - web/src/components/admin/AdminDashboard.tsx
    - web/src/store/app-store.ts
    - web/src/components/graph/GraphCanvas.tsx
    - web/src/components/graph/GraphTree.tsx
    - web/src/components/graph/KnowledgeGraphBrowser.tsx
    - web/src/components/chat/parts/SourceDocumentPart.tsx
    - web/src/components/chat/parts/DataSubprogressPart.tsx
    - web/src/components/inspector/InspectorRetrieval.tsx
    - web/src/components/verdict/CounterEvidenceCard.tsx
    - web/src/__fixtures__/index.ts
    - web/src/__tests__/DataSubprogressPart.test.tsx
    - web/src/__tests__/MessageBubble.test.tsx
    - web/src/__tests__/useWizard.test.tsx
decisions:
  - "D26-08-A: TIER_FALLBACK_LABELS lives in lib/kb-bases.ts (keyed by SLUG), not TierBadge — the hook + three graph/chat consumers need it and a lib→component import would cycle; TierBadge ends up CN-free, which the plan's whitelist permits (allowed, not required)"
  - "D26-08-B: TierBadge always consults the projection asynchronously (the offline map is the first paint, the registry display_name overrides) — the 'registry display_name wins' must-have outranks the 'only unknown slugs go async' reading, and a changed seed display_name still reaches the UI"
  - "D26-08-C: TIERS re-keys to the four seed SLUGS (not the CN display names) — after 26-05 the CN strings are display names, so a CN seed fixture would pin identifiers that no longer exist; `web/src/types/` ends at zero CN literals"
  - "D26-08-D: SourceDocumentPart dispatches the SLUG derived from the title's directory prefix (fixture fallback) instead of the fixture's tier value — the dispatch target is a registry identity; the bracketed display stays in the render layer"
  - "D26-08-E: the SC-6 disclaimer 「历史」 and the counter-evidence hop badge stay as locked copy (T-5-27, UI-SPEC §SC-6, test-pinned verbatim) with a dated whitelist comment — they are the historical ROLE in user-facing copy, not tier identifiers, and the plan's gate greps for 权威 only"
metrics:
  duration: ~40 min
  completed: 2026-09-07
  tasks: 3
  commits: 5
  test_functions_added: 47
  files: 29
  insertions: 1891
  deletions: 116
  baseline_tests: 458
  final_tests: 497
status: complete
---

# Phase 26 Plan 08: FE Admin KB View + Registry-Aligned Badge/Graph Summary

**One-liner:** the admin console gains its first mutation-bearing view (KB create/deactivate, token-gated, backend-verdict-rendered) while every tier label, badge, and graph node in the FE learns to read the KB registry projection — and the last hardcoded tier fallbacks (chunk-lookup's `权威`, the title-dir CN table, the subprogress and inspector defaults) are deleted rather than renamed.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `lib/kb-bases.ts` (projection client + `useTierLabel` + seed fallback map); `Tier = string` (TIERS → 4-slug seed fixture); TierBadge registry-driven; chunk-lookup's two hardcoded tier fallbacks deleted (`tier: null` + badge hidden) | `f668d8e` (RED), `eb232a6` (GREEN) |
| 2 | `KnowledgeBaseList.tsx` — list/create/deactivate over `/api/admin/kb*`; `AdminDashboard` 'bases' tab + store union widen | `3385dfc` (RED), `6369e6b` (GREEN) |
| 3 | Graph N-tier evidence (3-KB + 6-KB fixtures, registry labels, layout untouched) + the residual sweep across SourceDocumentPart / DataSubprogressPart / InspectorRetrieval / stale comments | `e6c806a` |

Trust semantics locked by the 47 new tests:

- **Registry display name wins** — `TierBadge` renders the projection's `display_name` for a registered slug (`[新库]` for `kb-2026`), with the seed fallback as the *synchronous first paint*; a renamed base reaches the UI without a code change.
- **Degradation, never fabrication (T-26-08-2)** — an unregistered slug renders `[slug]`; a pre-migration CN value replayed from an audit row renders bracketed and cannot masquerade as a live tier; a *failed* projection fetch keeps the same fallback; an empty tier renders **nothing** (badge-hidden) instead of `[]`.
- **Projection client posture** — one `GET /api/knowledge/bases` per session (module promise cache, asserted by array identity); 503 (registry unwired, D26-05-B), network failure, and a malformed body all degrade to `[]` without throwing.
- **The mutation surface (T-26-08-1 / T-26-08-3)** — create and deactivate both carry `X-Admin-Token`; the client-side slug regex (`^[a-z][a-z0-9-]{0,19}$`, cited to `internal/kb/slug.go`) rejects uppercase / underscore / empty / leading-hyphen **before any request** (`fetch` called exactly once = the initial GET); the role `<select>` offers exactly `authority|historical|personal|draft`; a 400 renders the backend rule text verbatim (the FE does not gate), 409 renders `slug 已存在`, 403 names the token, 503 names the unwired registry; deactivate asks once, flips only the confirmed row, and a refused mutation flips nothing locally; **no delete / re-enable control exists** because the backend has none.
- **N-dynamic evidence** — 3-KB and 6-KB graph fixtures render 3 and 6 tier nodes respectively, every doc node stays inside the 900×640 viewBox, the 6 tier ring positions are pairwise distinct, and labels resolve through the projection (`[权威]` / `[ghost-kb]`).
- **Residual sweep pinned** — a tier-less `retrieve` subprogress step renders `检索知识层` (no bracket anywhere in the row); the doc chip pill hides when no tier is derivable and dispatches the derived slug; the inspector dispatches the projection's first authority-role slug (surfaced as `data-default-tier` so the test proves resolution before clicking).

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| `npx vitest run src/components/common/TierBadge.test.tsx src/lib --reporter=basic` (Task 1) | PASS 41/41 |
| `npx vitest run src/components/admin --reporter=basic` (Task 2) | PASS 18/18 |
| `npx vitest run` (final, full FE) | **PASS 497/497 (67 files)** — baseline 458, +39 |
| `npx tsc --noEmit` | **exit 0** |
| `grep -n "tier: '权威'\|tier: \"权威\"" web/src/lib/chunk-lookup.ts` | **0** |
| `grep -c "X-Admin-Token" web/src/components/admin/KnowledgeBaseList.tsx` | **6** (≥2 required) |
| `grep -c "DELETE\|activate:" web/src/components/admin/KnowledgeBaseList.tsx` | **0** (no fake capability) |
| `grep -c "'bases'" AdminDashboard.tsx` / `app-store.ts` | 4 / 2 (both present) |
| `grep -rn "权威" chat/parts/ inspector/ verdict/ graph/ types/ (non-test)` | **0** |
| `grep -rn "tier: '权威'" web/src/ (non-test)` | **0** (27 remaining hits are all `*.test.*` wire fixtures carrying legacy payload data) |
| `git diff --name-only 8cbe270..HEAD` outside `web/` | none — **BE untouched** |

## Deviations from Plan

**1. [Rule 3 - Blocking] `web/src/__fixtures__/index.ts` tier union widened to `string`**
- **Found during:** Task 1 (tsc)
- **Issue:** `tier: '权威' | '历史' | '个人' | '草稿'` in `ChunkFixtureEntry` is (a) a stale fixed-4-CN-tier type and (b) the only non-test hit of the plan-level gate `grep -rn "tier: '权威'" web/src/`. Not in `files_modified`, but the gate cannot pass without it.
- **Fix:** widened to `string` with a comment stating the fixture *data* keeps legacy CN values deliberately (they exercise the degradation paths). Data files untouched.
- **Commit:** `e6c806a`

**2. [Rule 1 - Bug] Two pre-existing tests pinned the pre-sweep behavior and had to move with it**
- **Found during:** Task 3
- **Issue:** `MessageBubble.test` asserted the doc-chip dispatch tier equals the fixture's CN value; `useWizard.test` asserted the reads prompt contains `知识层`. Both encode exactly the vocabulary this plan retires, so the sweep reds them.
- **Fix:** the dispatch assertion now expects the registry slug (`authority`) and the prompt assertion the registry vocabulary (`知识库` + `authority`). No source code bent to satisfy either test.
- **Commit:** `e6c806a`

**3. [Form] The GraphCanvas ≤20-changed-line gate reads 24 insertions / 11 deletions**
- The gate's substance holds: `layout()`, both ring radii, all trig, and every positioned-node computation are byte-identical (verified by diffing for layout tokens — the only hit is a comment). The letter is exceeded because the *same plan's* CN-literal grep over `src/components/graph/` forces re-keying `TIER_COLOR`'s four CN keys (+ fallback for unregistered slugs, + the `useTierLabel` component that the label-routing requirement needs). Recorded rather than stripping the rationale comments to hit a proxy number.

**4. [Form] TIER_FALLBACK_LABELS homed in `lib/kb-bases.ts` instead of `TierBadge.tsx`**
- The plan locates the offline map in TierBadge; the hook and three graph/chat consumers need the same map, and a lib→component import would cycle. TierBadge now contains **zero** CN literals — the Task 1 whitelist ("允许保留") permits that, it does not require it. (`eb232a6`)

**5. [Form] Three new test files beyond `files_modified`**
- `SourceDocumentPart.test.tsx`, `InspectorRetrieval.test.tsx` (Task 3 behaviors with no existing harness), and `AdminDashboard.test.tsx` (the plan's own behavior line demands "'bases' tab 可点击激活并渲染 KnowledgeBaseList" and no AdminDashboard test existed). Also extended `DataSubprogressPart.test.tsx` with the no-tier pin.

**6. [Form] `@testing-library/user-event` not used**
- Not installed, and installs are out of an executor's scope — the admin-view tests drive forms with `fireEvent` + `waitFor` from the already-present `@testing-library/react`.

## Notes for Downstream Plans

- **26-09 (kb.Open wiring):** the FE surfaces are live against the wire as shipped — until the registry is wired, `GET /api/knowledge/bases` answers 503, the projection degrades to `[]`, and every badge shows the seed fallback or `[slug]`. No FE change is needed when the registry comes up.
- **26-10 (zero-hardcode audit):** the FE CN whitelist is now exactly two annotated homes — `TIER_FALLBACK_LABELS` in `lib/kb-bases.ts` (4 seed display names, slug-keyed) and the SC-6 disclaimer / hop-badge copy in `CounterEvidenceCard.tsx` + `InspectorHop.tsx` (locked user-facing copy, dated comment added). `src/components/layout/KnowledgeModal.tsx` still carries a CN `TIER_ORDER` + dir map; it is outside this plan's file list and grep gates and is logged below for the audit to adjudicate.
- **Phase 28 (graph):** `GraphCanvas.layout()` is byte-identical; the only graph-canvas changes are label resolution, palette keys, and the removed cast. The 3-KB/6-KB fixtures and the bounds/distinct-position assertions in `GraphCanvas.test.tsx` are the regression harness for the layout upgrade.

## Known Stubs

None. A `TODO|FIXME|placeholder|coming soon` scan over the 19 touched files matches only prose (the historical "05-03 placeholder" reference, `placeholder=` input attributes, and the pre-resolution `''` placeholder described in the inspector test) — no unfinished work, no unwired data surface.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-08-1 (EoP: the FE's only mutation verbs) | every create/deactivate request carries `X-Admin-Token` (asserted per-call in tests); the server's 403/400/409/503 verdict is what renders; no delete/re-enable UI exists because the API has no such endpoint |
| T-26-08-2 (Info disclosure: unknown tier render) | unknown and legacy-CN slugs degrade to a bracketed label; a missing tier hides the badge; `chunk-lookup` returns `tier: null` instead of inventing one — a replayed CN value can never impersonate a live tier |
| T-26-08-3 (Tampering: FE validation bypass) | the slug regex and role whitelist are documented in-component as UX-only; the 400 test proves the backend rule text is surfaced verbatim, i.e. the FE renders the server's decision rather than its own |
| T-26-SC (npm dependency legitimacy) | zero new dependencies — `package.json` untouched |

## Self-Check: PASSED

- Commits exist on the branch: `f668d8e`, `eb232a6`, `3385dfc`, `6369e6b`, `e6c806a` — all FOUND in `git log 8cbe270..HEAD` (29 files, +1891/−116).
- TDD gate sequence: a `test(26-08)` RED commit precedes each `feat(26-08)` GREEN commit for Tasks 1 and 2 (Task 3 is `type="auto"` and lands as one refactor commit).
- Files exist: all ten created files and nineteen modified files appear in the diff range; `web/src/lib/kb-bases.ts`, `web/src/components/admin/KnowledgeBaseList.tsx` (253 lines, ≥ the 80-line artifact floor), `web/src/components/common/TierBadge.tsx`, and the widened `app-store.ts` union all verified on disk.
- Gates re-verified after the final commit: `npx vitest run` 497/497, `npx tsc --noEmit` exit 0, CN-literal greps zero over non-test source, BE untouched.
