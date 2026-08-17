---
phase: 260817-hck-verdictcard-dedupe-answer-vs-chat-conclu
plan: 01
subsystem: frontend/chat-render
tags: [verdict-card, dedupe, chat-text, prop-drilling, tdd]
requires:
  - web/src/types/verdict.ts (Verdict.answer, isVerdict/isRefusal)
  - internal/server/chat.go:793-828 (conclusionText embeds answer verbatim in text-delta)
provides:
  - VerdictCard optional chatText prop with verbatim-substring dedupe of verdict-answer
  - MessageBubble -> DataPolicyPart -> VerdictCard three-level chatText pass-through
affects:
  - web/src/components/center/VerdictCard.tsx
  - web/src/components/chat/parts/DataPolicyPart.tsx
  - web/src/components/chat/MessageBubble.tsx
tech-stack:
  added: []
  patterns:
    - "verbatim substring dedupe (no fuzzy/regex) against single-atomic text-delta"
    - "optional-prop drilling for chat-context-aware cards (D14: no messages in Zustand)"
key-files:
  created: []
  modified:
    - web/src/components/center/VerdictCard.tsx
    - web/src/components/chat/parts/DataPolicyPart.tsx
    - web/src/components/chat/MessageBubble.tsx
    - web/src/__tests__/VerdictCard.test.tsx
    - web/src/__tests__/MessageBubble.test.tsx
decisions:
  - "Dedupe = chatText.includes(answer.trim()) only; FE does NOT re-derive Go conclusionText (D15 anti-drift)"
  - "Duplicate case does not render the verdict-answer node at all (no hidden-class pattern; verdict-answer has no scraper dependency, unlike confidence-bar)"
metrics:
  duration: 12m
  completed: 2026-08-17
---

# Phase 260817-hck Plan 01: VerdictCard dedupe answer vs chat conclusion — Summary

**One-liner:** VerdictCard gains an optional `chatText` prop; when the assistant message's concatenated text parts verbatim contain `verdict.answer`, the in-card `verdict-answer` block is not rendered, so the conclusion appears exactly once (chat body keeps it, card keeps badges/citations).

## What Was Built

- `VerdictCard.tsx`: optional `chatText?: string` prop; `answerDuplicatedInChat = chatText !== undefined && answerText !== '' && chatText.includes(answerText)`; render gate changed to `{isFactualQuery && !answerDuplicatedInChat && ...}`. StatusBadge / ComplianceStatusBadge / 「N 条引用」/ CitationCluster / confidence-bar hidden node / review-required-badge all untouched.
- `DataPolicyPart.tsx`: optional `chatText?: string` prop forwarded to `<VerdictCard verdict={data} chatText={chatText} />` in the `isVerdict` branch; refusal and unknown-payload branches unchanged.
- `MessageBubble.tsx`: assistant role aggregates `message.parts.filter(type==='text').map(.text).join('')` into `chatText`; `renderPart(part, i, chatText)` passes it only to the `data-policy` branch. User role passes `undefined`.
- Tests: 4 new VerdictCard unit tests (duplicate-hidden / non-duplicate-kept / no-prop CenterPanel-path-kept with compliance_status / empty-string-kept) + 2 new MessageBubble integration tests (text contains conclusion -> `verdict-answer` gone, `verdict-card` + `citation-cluster` remain; control: text without answer -> `verdict-answer` renders).

## Verification

| Gate | Result |
|------|--------|
| `npx vitest run src/__tests__/VerdictCard.test.tsx` | 14/14 PASS (10 existing + 4 new) |
| `npx vitest run src/__tests__/MessageBubble.test.tsx` | 10/10 PASS (8 existing + 2 new) |
| Full regression `npx vitest run` | **272/272 PASS (37 files)** |
| `grep -n "chatText"` 3-file structural gate | hits in all 3 files |
| `npx tsc --noEmit` | exit 2 — **1 pre-existing error in user's uncommitted WIP** (`DataSubprogressPart.tsx:47` TS2339 `state` on `SubprogressStep`); zero errors in any file this plan touched. See Deferred Issues. |

## TDD Gate Compliance

- RED `e41a2cd` (VerdictCard tests, 1 failing as expected) -> GREEN `ec0b1b2` (VerdictCard impl)
- RED `edfcd72` (MessageBubble integration tests, 1 failing as expected) -> GREEN `0da6e17` (pass-through impl)
- No refactor commit needed (minimal diffs).

## Deviations from Plan

**1. [Rule 3 - Blocking / commit-hygiene recovery] Rebased Task 2 GREEN commit onto HEAD-only MessageBubble**
- **Found during:** Task 2 commit
- **Issue:** The working tree held PRE-EXISTING uncommitted user modifications to `MessageBubble.tsx` (chat-layout modernization). The first Task 2 GREEN commit (`4620171`) swept those unrelated changes into the commit (104-line diff instead of ~40), violating the working-tree isolation constraint.
- **Fix:** `git reset --mixed HEAD~1`; checked out HEAD's `MessageBubble.tsx`; re-applied only this plan's 4 edits (header note, `renderPart` 3rd arg, `data-policy` branch prop, assistant-role `chatText` aggregation — expressed as `role === 'assistant' ? ... : undefined` since HEAD's component has a single shared return); recommitted as `0da6e17` (40+/4-, plan-scoped only); then restored the user's WIP + this plan's changes to the working tree. Re-ran both test files after each state change (24/24 green both times).
- **Net effect on working tree:** `MessageBubble.tsx` is modified again (user WIP remains uncommitted, now with this plan's feature on top, semantically identical to the committed version).
- **Commit:** `0da6e17` (replaces discarded `4620171`)

**2. [Scope boundary] `tsc --noEmit` not exit-0 due to pre-existing user WIP error**
- Logged to `deferred-items.md`; NOT fixed (error is in `DataSubprogressPart.tsx`, a file with pre-existing uncommitted user modifications, unrelated to this plan). The plan's own files type-check clean — the sole error is TS2339 at `DataSubprogressPart.tsx:47`.

## Known Stubs

None. No placeholders, no empty-value render paths introduced.

## Threat Flags

None. No new network endpoints, auth paths, or trust-boundary surface. Threat register dispositions (T-hck-01/T-hck-02, both `accept`) hold: the dedupe judgment is display-only; worst-case misjudgment is the pre-change behavior (answer shown twice) or card-only omission while the body still shows the text.

## Self-Check

- `web/src/components/center/VerdictCard.tsx` — FOUND (chatText prop + dedupe logic)
- `web/src/components/chat/parts/DataPolicyPart.tsx` — FOUND (chatText pass-through)
- `web/src/components/chat/MessageBubble.tsx` — FOUND (chatText aggregation, working tree = committed feature + user WIP)
- `web/src/__tests__/VerdictCard.test.tsx` — FOUND (4 new tests)
- `web/src/__tests__/MessageBubble.test.tsx` — FOUND (2 new tests)
- Commits `e41a2cd`, `ec0b1b2`, `edfcd72`, `0da6e17` — all FOUND in `git log`
- Contaminated commit `4620171` — confirmed ABSENT from history

## Self-Check: PASSED
