---
phase: 4
slug: streaming-trust-ui
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-11
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `04-RESEARCH.md` §Validation Architecture (verified HIGH confidence).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest ^2.0.0 (installed; latest 4.1.10 optional upgrade deferred) |
| **Config file** | `web/vitest.config.ts` (verify in Wave 0; create with `environment: 'jsdom'`, `setupFiles: ['./src/__tests__/setup.ts']` if missing) |
| **Quick run command** | `npm test -- --run src/__tests__/<file>` |
| **Full suite command** | `npm test` (alias: `vitest run`) |
| **Estimated runtime** | Quick: < 10s · Full: < 60s · Manual visual: ~2 min |

---

## Sampling Rate

- **After every task commit:** `npm test -- --run src/__tests__/<changed-area>` (target < 10s)
- **After every plan wave:** `npm test` (full suite; target < 60s)
- **Before `/gsd:verify-work`:** Full suite green + manual visual check of all 10 fixtures rendering correctly in the dev server
- **Max feedback latency:** ~10s per task, ~60s per wave

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-W1-T1 | TBD | 1 | FEUI-01 | — | Streaming typed parts render text/reasoning/tool/source | unit | `npm test -- --run src/__tests__/MessageParts.test.tsx` | ❌ W1 | ⬜ pending |
| 04-W1-T2 | TBD | 1 | FEUI-01 | Pitfall 1/2/3 | Wire contract has no drift (kebab-case `data-policy` for Verdict; camelCase fields) | unit | `npm test -- --run src/__tests__/wire-contract.test.ts` | ❌ W1 | ⬜ pending |
| 04-W2-T1 | TBD | 2 | FEUI-02 | — | Reasoning preview default-capped, expandable, doesn't bury answer | unit + visual | `npm test -- --run src/__tests__/ReasoningPart.test.tsx` | ❌ W2 | ⬜ pending |
| 04-W2-T2 | TBD | 2 | FEUI-05 | — | Refusal out-of-scope vs I-don't-know visually distinct (D-07) | unit | `npm test -- --run src/__tests__/RefusalCard.test.tsx` | ❌ W2 | ⬜ pending |
| 04-W2-T3 | TBD | 2 | CMPL-05 (UI) | — | `review_required: true` → yellow human-review badge (D-06) | unit | `npm test -- --run src/__tests__/VerdictCard.test.tsx` | ❌ W2 | ⬜ pending |
| 04-W2-T4 | TBD | 2 | D-08 | — | Reasoning always positioned above answer; answer never off-screen | integration + manual visual | `npm test -- --run src/__tests__/ReasoningPart.test.tsx` + manual | ❌ W2 | ⬜ pending |
| 04-W3-T1 | TBD | 3 | FEUI-03 | — | Citation click → center panel chunk view | integration | `npm test -- --run src/__tests__/CenterPanel.test.tsx` | ❌ W3 | ⬜ pending |
| 04-W3-T2 | TBD | 3 | FEUI-03, KNOW-05 | — | Chunk view shows tier badge (`[权威]`/`[历史]`/`[个人]`/`[草稿]`) + version + effective date | unit | `npm test -- --run src/__tests__/ChunkSourceView.test.tsx` | ❌ W3 | ⬜ pending |
| 04-W3-T3 | TBD | 3 | FEUI-04, SYSK-03 | — | Skill picker lists fixtures from `/api/skills` mock; switches skill scopes query | unit | `npm test -- --run src/__tests__/SkillPicker.test.tsx` | ❌ W3 | ⬜ pending |
| 04-W3-T4 | TBD | 3 | FEUI-06 | — | Sub-progress steps render in order ("Searching 权威 tier…", "Found N candidates…", "Re-ranking…") | unit | `npm test -- --run src/__tests__/DataSubprogressPart.test.tsx` | ❌ W3 | ⬜ pending |
| 04-W3-T5 | TBD | 3 | T-1-08 | XSS via `source-url` | Source URL sanitization: protocol allowlist (`https:`,`http:`) + hostname allowlist + `rel="noopener noreferrer"` | unit | `npm test -- --run src/__tests__/sanitize-url.test.ts` | ❌ W3 (mandatory gate) | ⬜ pending |
| 04-W3-T6 | TBD | 3 | T-1-08 | Tampering | SourceUrlPart uses sanitizer; rejected URLs render blocked message (not clickable) | integration | covered by `sanitize-url.test.ts` + a part-render test | ❌ W3 | ⬜ pending |
| 04-W3-T7 | TBD | 3 | D-04 | — | Mock ↔ Default transport produce identical parts for same fixture | integration | `npm test -- --run src/__tests__/transport-swap.test.ts` | ❌ W3 | ⬜ pending |

*Status legend: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/vitest.config.ts` — confirm exists with `environment: 'jsdom'`; create if missing (jsdom for DOM-rendering parts, citation click events, localStorage persistence)
- [ ] `web/src/__tests__/setup.ts` — imports `@testing-library/jest-dom/vitest` matchers, sets up fake timers for streaming-delay tests
- [ ] Framework install: `npm install -D @tailwindcss/vite tailwindcss@^4` plus runtime deps (zustand@^5, lucide-react@^1.31, @fontsource-variable/inter@^5, @fontsource-variable/jetbrains-mono@^5, clsx, tailwind-merge) — verify with `npm ls` before Wave 1
- [ ] `web/tsconfig.json` strict-mode verification (needed for `wire-types.ts` drift detection to fail-fast on Pitfall 1/2/3 type corrections)
- [ ] Initial fixture files: `web/src/__tests__/fixtures/{verdict-policy.jsonl, refusal-out-of-scope.jsonl, refusal-i-dont-know.jsonl, multi-part.jsonl, sub-progress.jsonl, citation-tiered.jsonl, …}` (≥10 fixtures covering SC-1..6)

*If none: not the case here — Wave 0 has concrete setup work.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 3-column draggable resize persists across reload | FEUI-04-adjacent | localStorage round-trip + visual | Drag the resize handle; reload browser; verify widths restored. Repeat with cleared localStorage (default 260/flex/420). |
| GLM-5.2 long reasoning chain (1000+ tokens) doesn't push answer off-screen | D-08 | Streaming visual rhythm | Load `multi-part.jsonl` with 1500-token reasoning stream; scroll during streaming; verify answer stays in viewport. |
| Citation click → center panel loads chunk verbatim with tier badge | FEUI-03, KNOW-05 | Visual + integration | Open `citation-tiered.jsonl` fixture; click first citation; verify center panel swaps from verdict card to chunk view with `[权威]` badge + version + effective date rendered at top. |
| Skill picker selection scopes subsequent query | FEUI-04, SYSK-03 | Visual + integration | Open skill picker; select "compliance"; submit query; verify request body / fixture lookup uses scoped Skill. |
| Refusal visual distinction (blue-gray vs gray borders + 🚫/❓ icons) | FEUI-05 | Visual | Load both refusal fixtures side-by-side; verify border color + icon differ. |

*If none: All phase behaviors have automated verification.* (Not the case — visual + drag-resize are inherently manual.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (vitest.config, setup.ts, fixture files, strict TS)
- [ ] No watch-mode flags in CI commands
- [ ] Feedback latency < 10s per task, < 60s per wave
- [ ] Manual visual checklist for FEUI-02/03/04/05 + D-08 passes
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending