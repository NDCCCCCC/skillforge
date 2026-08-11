---
phase: 04-streaming-trust-ui
plan: 01
type: execute
wave: 1
autonomous: true
completed: 2026-08-11T08:19:00Z
duration_minutes: 75
tasks_completed: 4
requirements_covered: [FEUI-01, FEUI-02, FEUI-06, T-1-08]
dependency_graph:
  requires: []
  provides: [wire-types, mock-transport, fixture-set, parts-renderer, sanitize-url]
  affects: [phase-04]
tags: [foundation, drift-detection, mock-transport, sanitization]
---

# Phase 4 Plan 01 — Foundation: Wire Types + Mock Transport + Parts Renderer

## One-Liner

P4 Wave 1 foundation: TS mirror of Go wire contract (Pitfalls 1/2/3 pinned), MockChatTransport zero-network harness, 10 UIMessageChunk fixtures, T-1-08 source-URL sanitization gate, and 5 typed part renderers. Establishes the parallel-with-Phase-2 dev loop.

## Tasks Completed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | MockChatTransport + factory + Zustand store | `946c043` | transport/, store/, __tests__/MockChatTransport.test.ts |
| 2 | Wire types + fixtures + parts + sanitization | `5339858` | types/, __fixtures__/, components/chat/, lib/sanitize-url.ts, styles/theme.css |

(Task 0 dep-install and Task 1 wire-types were initially committed but reverted by an external worktree reset mid-execution; the affected files were re-applied and committed together in `5339858` to preserve the test-passing state. The package.json / tsconfig.json / vite.config.ts / main.tsx files remained at the spike-stub state per the locked-files constraint; the runtime deps were still installed in node_modules and the tests run against them.)

## Files Created (38 net new)

**Wire types (compile-time drift alarm)**
- `web/src/types/wire-types.ts` — `SkillForgeChunk` discriminated union mirroring `internal/wire/chunk.go` with Pitfall 1 (`finish` not `finish-message`), Pitfall 2 (`.data` not `.policy`), Pitfall 3 (`tool-${name}` / `dynamic-tool` not `tool-invocation`) corrections baked in. Compile-time `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails build if SDK upstream renames any discriminator.
- `web/src/types/verdict.ts` — TS mirror of `internal/verdict/*` structs (Verdict / Citation / Refusal / SubprogressStep). Snake_case preserved on wire fields (`review_required` / `can_user_rephrase` / `suggested_alternative_query`) per `internal/verdict/citation.go` JSON tags. Includes `isVerdict`/`isRefusal` type guards.

**Mock transport (zero-network dev harness — D-03)**
- `web/src/transport/MockChatTransport.ts` — implements `ChatTransport<UIMessage>` per `ai@7.0.59/dist/index.d.ts` lines 5270-5336. Configurable `delayMs`/`jitterMs` for streaming rhythm simulation. `reconnectToStream` returns `null` (matches DirectChatTransport).
- `web/src/transport/index.ts` — `createTransport(env, fixtureName)` factory. `'prod'` → `DefaultChatTransport` against `/api/chat` (D-04 swap point). `'dev'`/`'test'` → `MockChatTransport` against `FIXTURE_REGISTRY[fixture]`.
- `web/src/transport/mock-types.ts` — `MockFixture`/`MockFixtureName` aliases.

**Zustand store (Claude's Discretion locks state boundary)**
- `web/src/store/app-store.ts` — CenterView discriminated union (`empty | verdict | chunk-source | skill-md`). File-level comment bans `messages` field — `useChat` owns the message stream (Pitfall 7).

**Fixtures (10 — D-05 coverage)**
- `web/src/__fixtures__/verdict-compliant.json`
- `web/src/__fixtures__/verdict-needs-info.json` (review_required=true; D-06)
- `web/src/__fixtures__/verdict-non-compliant.json`
- `web/src/__fixtures__/verdict-unanswered.json`
- `web/src/__fixtures__/refusal-out-of-scope.json` (D-07)
- `web/src/__fixtures__/refusal-no-source.json`
- `web/src/__fixtures__/refusal-citation-mismatch.json`
- `web/src/__fixtures__/multi-part.json` (text + reasoning + 2 tool calls + source-url + data-policy)
- `web/src/__fixtures__/sub-progress.json` (5-step data-subprogress sequence)
- `web/src/__fixtures__/citation-tiered.json` (4 tiers: 权威/历史/个人/草稿 — KNOW-05)
- `web/src/__fixtures__/index.ts` — `FIXTURE_REGISTRY` exporting all 10 fixtures.

**Part renderers (FEUI-01)**
- `web/src/components/chat/parts/TextPart.tsx`
- `web/src/components/chat/parts/SourceUrlPart.tsx` (T-1-08 gate)
- `web/src/components/chat/parts/SourceDocumentPart.tsx`
- `web/src/components/chat/parts/ToolPart.tsx` (handles `dynamic-tool` + `tool-${name}` fallthrough)
- `web/src/components/chat/parts/DataPolicyPart.tsx` (reads `part.data` per Pitfall 2; reads Verdict OR Refusal)
- `web/src/components/chat/parts/StepStartPart.tsx`
- `web/src/components/chat/MessageBubble.tsx` — parts switch dispatcher; reasoning renders BEFORE text in DOM order (D-08 invariant)

**Sanitization (T-1-08 hard gate)**
- `web/src/lib/sanitize-url.ts` — protocol+hostname allowlist (`https:`/`http:` + `localhost`/`127.0.0.1`/`::1`/`spike.local`/`skillforge.local`). Rejected URLs render `citation-blocked` span + `console.warn`.

**UI primitives (shadcn-style, hand-authored)**
- `web/src/components/ui/button.tsx`, `card.tsx`, `badge.tsx`, `tabs.tsx` — tuned to SkillForge design tokens (Label size, surface variants).

**Styling (Tailwind v4)**
- `web/src/styles/theme.css` — `@theme` block with denova dark, 5 status tokens (compliant/needs-info/non-compliant/unanswered + out-of-scope), spacing scale, typography (4 sizes × 2 weights), topbar/statusbar heights.

**Utility**
- `web/src/lib/cn.ts` — shadcn standard `cn()` helper (clsx + tailwind-merge).

**App composition root**
- `web/src/App.tsx` — replaces spike stub. `createTransport(env)` factory selects Mock vs Default transport; uses `useAppStore` for `centerView`.

**Tests (21 passing across 4 files)**
- `web/src/__tests__/wire-contract.test.ts` — 5 drift-detection assertions (Pitfalls 1/2/3 + lifecycle).
- `web/src/__tests__/MockChatTransport.test.ts` — 5 transport assertions (fixture order, function fixture, reconnect null, deterministic snapshot, delay honored).
- `web/src/__tests__/MessageBubble.test.tsx` — 7 parts-rendering assertions (all 7 part types, reasoning-before-text, T-1-08 block, rel attrs, .data vs .policy, tool parts, unknown drop).
- `web/src/__tests__/citations.test.tsx` — 4 spike-stub assertions (allowlisted URL renders `<a>`, non-allowlisted blocked, unknown drops, reasoning in `<details>`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Spike-stub App.tsx + citations.test.tsx imports locked at Phase 1 state**
- **Found during:** Task 3 — build failed after reverts; spike `App.tsx` and `citations.test.tsx` referenced spike-stub paths.
- **Issue:** The locked-file constraint (package.json / tsconfig.json / vite.config.ts / main.tsx) prevented the planned P4 changes to those 4 root files. The existing spike `App.tsx` still used `spikeTransport = new DefaultChatTransport({api:'/api/spike/chat'})` and the `citations.test.tsx` test used a non-allowlisted URL that the new T-1-08 sanitizer correctly blocks.
- **Fix:** Updated `web/src/App.tsx` to use `createTransport(env)` factory (in scope — not in the locked list). Updated `web/src/__tests__/citations.test.tsx` to use an allowlisted URL (`skillforge.local`) per T-1-08 contract + added an explicit block-malicious-host assertion.
- **Files modified:** `web/src/App.tsx`, `web/src/__tests__/citations.test.tsx`.
- **Commit:** `5339858`.

**2. [Rule 1 - Bug] Missing `isVerdict`/`isRefusal` type guards for DataPolicyPart**
- **Found during:** Task 3 — DataPolicyPart needed to discriminate `Verdict` vs `Refusal` payload.
- **Issue:** The plan referenced "discriminated union `Verdict | Refusal`" but didn't ship the discriminator helpers.
- **Fix:** Added `isVerdict(value)` and `isRefusal(value)` type guards to `web/src/types/verdict.ts` checking presence of `status` (Verdict) vs `reason` (Refusal).
- **Files modified:** `web/src/types/verdict.ts`.
- **Commit:** `5339858`.

### Pre-existing Issues

**Locked-file constraint:** The 4 root-level config files (`package.json`, `tsconfig.json`, `vite.config.ts`, `web/src/main.tsx`) were externally reverted mid-execution and remained at the Phase 1 spike-stub state per the "intentional" file change. This means:
- `package.json` does NOT list `zustand`/`tailwindcss`/`lucide-react`/`clsx`/`tailwind-merge`/`@fontsource-variable/*`/`class-variance-authority` deps (they are still in `node_modules` as `extraneous`).
- `tsconfig.json` does NOT have the `@/*` path alias (so all imports use relative paths).
- `vite.config.ts` does NOT include `@tailwindcss/vite` (so `theme.css` is not processed by Tailwind at build time — but the CSS variables in `@theme` are still valid CSS).
- `web/src/main.tsx` does NOT import fontsource or `theme.css` (so the design tokens are not applied globally).

These are tracked in **Known Stubs** below. The Wave 2 / Wave 3 follow-up plan should reconcile the locked files before the visual contract is materialized.

## Verification

| Check | Result |
| ----- | ------ |
| `cd web && npm test -- --run src/__tests__/wire-contract.test.ts` | PASS (5/5) |
| `cd web && npm test -- --run src/__tests__/MockChatTransport.test.ts` | PASS (5/5) |
| `cd web && npm test -- --run src/__tests__/MessageBubble.test.tsx` | PASS (7/7) |
| `cd web && npm test -- --run src/__tests__/citations.test.tsx` | PASS (4/4) |
| `cd web && npm test` (full suite) | PASS (21/21 across 4 files) |
| `cd web && npm run build` | PASS (164 modules, 352KB gzipped) |

## Pitfalls Addressed

| Pitfall | Severity | Resolution |
| ------- | -------- | ---------- |
| 1: `finish-message` → `finish` | HIGH | `wire-contract.test.ts` asserts every fixture ends with `type:'finish'`; `_driftCheck` assertion at compile time |
| 2: `.policy` → `.data` | HIGH | `wire-contract.test.ts` asserts every `data-policy` has `.data` defined and `.policy` undefined |
| 3: `tool-invocation` → `tool-${name}` | HIGH | `wire-contract.test.ts` forbids `tool-invocation`; `ToolPart` accepts dynamic-tool + `tool-${name}` fallthrough via `startsWith('tool-')` |
| 4: XSS via `source-url` href (T-1-08) | HIGH | `sanitizeSourceUrl()` allowlist gate; rejected URLs render blocked-citation span + `console.warn`; `<a>` always carries `target="_blank" rel="noopener noreferrer"` |
| 5: Reasoning buries answer | MEDIUM | D-08 ordering invariant pinned by MessageBubble test |
| 6: Mock ↔ real transport drift | MEDIUM | Fixtures include lifecycle chunks (`start`, `step-start`, `finish`); D-04 swap point isolated in `createTransport` factory |
| 7: Zustand ↔ useChat double source of truth | MEDIUM | `app-store.ts` file-level comment bans `messages` field |
| 8: Tailwind v4 / shadcn theme collision | LOW | Hand-authored shadcn-style components tuned to SkillForge design tokens (no shadcn CLI required) |

## Known Stubs

| Stub | File | Reason | Resolved In |
| ---- | ---- | ------ | ----------- |
| `theme.css` Tailwind plugin not active | `web/src/styles/theme.css` + `vite.config.ts` | `vite.config.ts` is locked at spike-stub state; `@tailwindcss/vite` plugin not registered | Wave 2 (or before) — needs locked-file reconciliation |
| Fontsource imports not active | `web/src/main.tsx` | File is locked at spike-stub state; no fontsource import | Wave 2 — needs locked-file reconciliation |
| Path alias `@/*` not active | `web/tsconfig.json` | File is locked at spike-stub state; all imports use relative paths | Wave 2 — needs locked-file reconciliation |
| Runtime deps not declared in `package.json` | `web/package.json` | File is locked at spike-stub state; deps are `extraneous` in node_modules | Wave 2 — needs locked-file reconciliation |
| DataPolicyPart placeholder (renders JSON, not VerdictCard/RefusalCard) | `web/src/components/chat/parts/DataPolicyPart.tsx` | Full VerdictCard/RefusalCard ships in Wave 2 (D-06/D-07); data attribute `data-policy-status` exposed for Wave 2 tests | Wave 2 |
| ReasoningPart placeholder (`<details>`) | `web/src/components/chat/MessageBubble.tsx` `ReasoningPartPlaceholder` | Wave 2 ships ReasoningPreview progressive disclosure (D-08); current rendering preserves the D-08 ordering invariant but doesn't have the height-cap/expand UX | Wave 2 |

## Threat Flags

| Flag | File | Description |
| ---- | ---- | ----------- |
| threat_flag: drift | `web/src/types/wire-types.ts` | Compile-time AssertAssignable on `SkillForgeChunk ↔ UIMessageChunk` will fail if AI SDK upstream renames a discriminator. The TS compiler IS the drift alarm. |
| threat_flag: sanitization | `web/src/lib/sanitize-url.ts` | T-1-08 allowlist gate at the SourceUrlPart boundary. Rejected URLs render a blocked-citation span and log WARN. The hard-gate comment in the file prohibits any `<a href={...}>` from bypassing this module. |
| threat_flag: state-boundary | `web/src/store/app-store.ts` | File-level comment bans `messages` field (Pitfall 7). useChat owns the message stream. |

## Next Plan

Wave 2 (`04-02`): VerdictCard + RefusalCard + ReasoningPreview + ReasoningPart replacement. Builds on the parts dispatcher + DataPolicyPart placeholder + sanitization gate. Also reconciles the 4 locked config files so `theme.css` becomes active and fontsource variables load.
