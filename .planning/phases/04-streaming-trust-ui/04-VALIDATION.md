---
phase: 4
slug: streaming-trust-ui
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-11
validated: 2026-08-11
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `04-RESEARCH.md` §Validation Architecture (verified HIGH confidence).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 2.1.9 (declared `^2.0.0`; installed ahead of spec) + jsdom 25 + @testing-library/react 16 |
| **Config file** | `web/vitest.config.ts` — confirmed: `environment: 'jsdom'`, `globals: true`, `setupFiles: ['./vitest.setup.ts']`, `include: ['src/**/*.{test,spec}.{ts,tsx}']`, shares `@vitejs/plugin-react` |
| **Quick run command** | `cd web && npm test -- --run src/__tests__/<file>` |
| **Full suite command** | `cd web && npm test` (alias: `vitest run`) |
| **Estimated / measured runtime** | Quick: < 10s · Full (measured): **39.21s** for 14 files / 83 tests (< 60s wave target ✓) |

---

## Sampling Rate

- **After every task commit:** `npm test -- --run src/__tests__/<changed-area>` (target < 10s)
- **After every plan wave:** `npm test` (full suite; target < 60s)
- **Before `/gsd:verify-work`:** Full suite green + manual visual check of all 10 fixtures rendering correctly in the dev server
- **Max feedback latency:** ~10s per task, ~60s per wave

---

## Per-Task Verification Map

> Audit 2026-08-11: every planned task now has a green automated test on disk. Two planned filenames were renamed during execution (noted in **Actual file** column). Counts from `npm test -- --run` (83/83 green).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-W1-T1 | 04-01 | 1 | FEUI-01 | — | Streaming typed parts render text/reasoning/tool/source | unit | `cd web && npm test -- --run src/__tests__/MessageBubble.test.tsx` (7) | ✅ | ✅ green |
| 04-W1-T2 | 04-01 | 1 | FEUI-01 | Pitfall 1/2/3 | Wire contract has no drift (kebab-case `data-policy` for Verdict; camelCase fields) | unit | `cd web && npm test -- --run src/__tests__/wire-contract.test.ts` (5) | ✅ | ✅ green |
| 04-W2-T1 | 04-02 | 2 | FEUI-02 | — | Reasoning preview default-capped, expandable, doesn't bury answer | unit + visual | `cd web && npm test -- --run src/__tests__/ReasoningPart.test.tsx` (7) | ✅ | ✅ green |
| 04-W2-T2 | 04-02 | 2 | FEUI-05 | — | Refusal out-of-scope vs I-don't-know visually distinct (D-07) | unit | `cd web && npm test -- --run src/__tests__/RefusalCard.test.tsx` (8) | ✅ | ✅ green |
| 04-W2-T3 | 04-02 | 2 | CMPL-05 (UI) | — | `review_required: true` → yellow human-review badge (D-06) | unit | `cd web && npm test -- --run src/__tests__/VerdictCard.test.tsx` (6) | ✅ | ✅ green |
| 04-W2-T4 | 04-02 | 2 | D-08 | — | Reasoning always positioned above answer; answer never off-screen | integration + manual visual | `ReasoningPart.test.tsx` + `MessageBubble.test.tsx` (D-08 ordering) + UAT #5 | ✅ | ✅ green |
| 04-W3-T1 | 04-03 | 3 | FEUI-03 | — | Citation click → center panel chunk view | integration | `cd web && npm test -- --run src/__tests__/CenterPanel.test.tsx` (7) + `citations.test.tsx` (4) | ✅ | ✅ green |
| 04-W3-T2 | 04-03 | 3 | FEUI-03, KNOW-05 | — | Chunk view shows tier badge (`[权威]`/`[历史]`/`[个人]`/`[草稿]`) + version + effective date | unit | `cd web && npm test -- --run src/__tests__/ChunkSourceView.test.tsx` (5) | ✅ | ✅ green |
| 04-W3-T3 | 04-03 | 3 | FEUI-04, SYSK-03 | — | Skill picker lists fixtures from `/api/skills` mock; switches skill scopes query | unit | `cd web && npm test -- --run src/__tests__/SkillPicker.test.tsx` (5) | ✅ | ✅ green |
| 04-W3-T4 | 04-03 | 3 | FEUI-06 | — | Sub-progress steps render in order ("Searching 权威 tier…", "Found N candidates…", "Re-ranking…") | unit | `cd web && npm test -- --run src/__tests__/DataSubprogressPart.test.tsx` (5) | ✅ | ✅ green |
| 04-W3-T5 | 04-03 | 3 | T-1-08 | XSS via `source-url` | Source URL sanitization: protocol allowlist (`https:`,`http:`) + hostname allowlist + `rel="noopener noreferrer"` | unit | `cd web && npm test -- --run src/__tests__/sanitize-url.test.ts` (7) | ✅ | ✅ green |
| 04-W3-T6 | 04-03 | 3 | T-1-08 | Tampering | SourceUrlPart uses sanitizer; rejected URLs render blocked message (not clickable) | integration | `MessageBubble.test.tsx` T-1-08 block assertions + `citations.test.tsx` regression guard | ✅ | ✅ green |
| 04-W3-T7 | 04-03 | 3 | D-04 | — | Mock ↔ Default transport produce identical parts for same fixture | integration | `cd web && npm test -- --run src/__tests__/transport-swap.test.tsx` (6) | ✅ | ✅ green |
| 04-W3-T8 (bonus) | 04-03 | 3 | FEUI-04 | — | Three-panel drag-resize clamps + collapses + persists via localStorage | unit | `cd web && npm test -- --run src/__tests__/ThreePanel.test.tsx` (6) | ✅ | ✅ green |
| 04-W1-T3 (bonus) | 04-01 | 1 | D-03 | — | MockChatTransport emits deterministic fixture-order chunk stream | unit | `cd web && npm test -- --run src/__tests__/MockChatTransport.test.ts` (5) | ✅ | ✅ green |

*Status legend: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Planned vs actual filename drift (renamed during execution, content equivalent):*
- `MessageParts.test.tsx` (planned) → `MessageBubble.test.tsx` (actual — the parts dispatcher test)
- `transport-swap.test.ts` (planned) → `transport-swap.test.tsx` (actual — renamed for JSX support per 04-03 deviation #2)

---

## Wave 0 Requirements

- [x] `web/vitest.config.ts` — confirmed exists with `environment: 'jsdom'`, `globals`, `setupFiles: ['./vitest.setup.ts']` (reconciled in 04-02 Task 1)
- [x] `web/vitest.setup.ts` — imports `@testing-library/jest-dom/vitest` matchers (actual path is `vitest.setup.ts`, not the speculated `src/__tests__/setup.ts`)
- [x] Framework install: tailwindcss@^4 + @tailwindcss/vite + zustand@^5 + lucide-react + clsx + tailwind-merge + @fontsource-variable/* + class-variance-authority — declared in `package.json` (04-02 Task 1 reconciliation; `node_modules` confirmed present)
- [x] `web/tsconfig.json` strict-mode + `@/*` path alias (needed for `wire-types.ts` drift detection to fail-fast) — `strict: true` + `paths` added in 04-02 Task 1
- [x] Fixture files: `web/src/__fixtures__/` — 10 message fixtures (verdict-{compliant,needs-info,non-compliant,unanswered} + refusal-{out-of-scope,no-source,citation-mismatch} + multi-part + sub-progress + citation-tiered) covering SC-1..6, plus `skills.json` (2) + `chunks.json` (4)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions | UAT result |
|----------|-------------|------------|-------------------|------------|
| 3-column draggable resize persists across reload | FEUI-04-adjacent | localStorage round-trip + visual | Drag the resize handle; reload browser; verify widths restored. Repeat with cleared localStorage (default 260/flex/420). | UAT #9 pass (collapse + clamp verified in-browser; drag+persistence unit-pinned in `ThreePanel.test.tsx`) |
| GLM-5.2 long reasoning chain (1000+ tokens) doesn't push answer off-screen | D-08 | Streaming visual rhythm | Load `multi-part.jsonl` with 1500-token reasoning stream; scroll during streaming; verify answer stays in viewport. | UAT #5 pass (D-08 ordering invariant + height cap verified; reasoning above answer + `展开全部` toggle) |
| Citation click → center panel loads chunk verbatim with tier badge | FEUI-03, KNOW-05 | Visual + integration | Open `citation-tiered.jsonl` fixture; click first citation; verify center panel swaps from verdict card to chunk view with `[权威]` badge + version + effective date rendered at top. | UAT #6 pass (clicked chip → `[权威]` + 版本: v3.2 + 生效日期: 2026-01-15 + verbatim) |
| Skill picker selection scopes subsequent query | FEUI-04, SYSK-03 | Visual + integration | Open skill picker; select "compliance"; submit query; verify request body / fixture lookup uses scoped Skill. | UAT #8 pass (selecting skill → `Skill 定义` tab active, 7 fields render, header/statusbar sync) |
| Refusal visual distinction (blue-gray vs gray borders + 🚫/❓ icons) | FEUI-05 | Visual | Load both refusal fixtures side-by-side; verify border color + icon differ. | UAT #7 pass via unit (`RefusalCard.test.tsx` 8 assertions); not UI-triggerable (single-fixture mock) — component correctness unit-pinned |

*All manual-only behaviors were exercised in `04-UAT.md` (12/12 passed, 0 issues, 2026-08-11) using chrome-devtools MCP against the live dev server. Non-UI-triggerable behaviors (refusal variants, sanitization, sub-progress) are unit-pinned and noted in UAT evidence.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (vitest.config, setup.ts, fixture files, strict TS)
- [x] No watch-mode flags in CI commands (`vitest run` / `--run`, never watch)
- [x] Feedback latency < 10s per task, < 60s per wave (measured full suite 39.21s)
- [x] Manual visual checklist for FEUI-02/03/04/05 + D-08 passes (UAT 12/12)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved — automated 83/83 green + UAT 12/12 green

---

## Validation Audit 2026-08-11

> Reconstructed from execution artifacts (`04-01/02/03-SUMMARY.md`, `04-UAT.md`) and live `npm test -- --run` against the real `web/` tree. The doc had remained in `status: draft` after execution; this audit reconciles it with as-built reality.

| Metric | Count |
|--------|-------|
| Planned tasks audited | 13 |
| Gaps found (MISSING/PARTIAL) | 0 |
| Resolved by auditor | 0 (none needed) |
| Escalated to manual-only | 0 |
| Bonus tests found beyond plan | 2 (`ThreePanel.test.tsx`, `MockChatTransport.test.ts`) |
| Test files on disk | 14 |
| Tests passing (full suite) | 83/83 |
| Full-suite runtime | 39.21s |
| UAT scenarios | 12/12 pass |

**Filename drift (content-equivalent, no coverage loss):**
- `MessageParts.test.tsx` → `MessageBubble.test.tsx` (the parts dispatcher; covers the same FEUI-01 typed-parts behavior)
- `transport-swap.test.ts` → `transport-swap.test.tsx` (renamed for JSX support; covers the same D-04 invariant)

**Note on `npx tsc --noEmit`:** exits non-zero due to pre-existing AI-SDK-v7 generic-narrowing errors in `ToolPart.tsx` + several test files (documented as Out-of-Scope in 04-01/02/03-SUMMARY). These do NOT block `npm run build` (esbuild is permissive) and do NOT affect test execution. Tracked separately; not a Nyquist coverage gap.
