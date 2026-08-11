# Phase 4: Streaming Trust UI - Context

**Gathered:** 2026-08-11
**Status:** Ready for planning
**Parallelizable:** YES — Phase 4 is nearly independent of Phase 2 (runs in a separate worktree). UI is driven by mock UIMessageChunk fixtures during development; real agent wiring is the last step (swap transport instance only). Mock policy confirmed: **mock/fixture allowed during development; product demo + Ragas eval use real agent/data.**

<domain>
## Phase Boundary

Deliver a React chat experience where every claim is auditable: streaming typed message parts (reasoning / tool-invocation / text / source), clickable citations opening chunk verbatim with tier badge, structured compliance verdict card, distinguishable refusal UX (out-of-scope vs I-don't-know), and a sub-progress indicator during agent work. Built on the Phase-1-locked UIMessageChunk wire contract, driven by mock fixtures so it can ship in parallel with the Phase 2 backend.

**In scope:** FEUI-01..06, SYSK-03, KNOW-05 (citation tier badge), CMPL-05 UI side (review_required badge rendering — logic itself is Phase 2).
**Out of scope:** real multi-session management, knowledge graph browser, counter-evidence/redline, policy-version filtering UI — all Phase 5. Real agent decision/reasoning events — Phase 3 (P4 mocks them).

</domain>

<decisions>
## Implementation Decisions

### Center Panel & Citation Information Architecture
- **D-01:** Citation click loads chunk verbatim into the **center panel** (the document-preview area) — NOT a 4th panel, NOT a right-edge drawer, NOT inline expansion. Tier badge + version + effective date render at the top of the chunk text. This reconciles sketch's "center = doc preview" with SC-2's "side panel" (center is the side panel relative to the right chat).
- **D-02:** Center panel **defaults to the verdict card**. A top tab switches between: verdict card / chunk-source (triggered by citation click) / SKILL.md preview.

### Mock Transport (the parallel-with-P2 foundation)
- **D-03:** Implement a custom AI SDK v5 `Transport` that replays `UIMessageChunk` sequences from `.ts/.json` fixtures. Zero-network, pure frontend, vitest-assertable. This is what unlocks Phase 4 parallelism.
- **D-04:** Real wiring = swap one transport instance (`/api/spike/chat` → real Hertz `/api/chat`). UI component code unchanged.
- **D-05:** One fixture set, dual-use (vitest unit tests + dev transport), with configurable streaming delay (simulate real rhythm by default). Coverage: 4 verdicts (data-policy) / 3 refusals / multi-part (text+reasoning+tool+source) / sub-progress step events / citation+tier.

### Verdict & Refusal Visual
- **D-06:** Verdict card = status-colored header (合规 green / 需补充 yellow / 不合规 red / unanswered gray) + confidence number & thin progress bar + yellow "需人工复核" corner badge when `review_required` + citations count.
- **D-07:** Refusal card visual distinction: **out-of-scope** = blue-gray border + 🚫 + "超出本 Skill 范围"; **I-don't-know (unanswered)** = gray border + ❓ + "范围内无依据". Both show structured `{reason, can_user_rephrase, suggested_alternative_query}` below. Distinct from verdict green/yellow/red. (sketch's status palette only had one gray — this adds the out-of-scope blue-gray.)

### Reasoning Display
- **D-08:** Progressive disclosure — reasoning defaults to expanded but height-capped preview (first lines + dark scrollable region), placed ABOVE the answer. One-click fully-expand. Reasoning streams live into the preview region during generation. Answer always stays prioritized/visible, never pushed off-screen by long reasoning chains (GLM-5.2 is a reasoning model; chains can be 1000+ tokens).

### Claude's Discretion
- Left sidebar (session list / file tree / skill list) = **mock skeleton** in P4. Real multi-session management + knowledge graph browsing belong to Phase 5 (SYSK-04/05, KNOW-06).
- Skill picker data source `/api/skills` — P4 uses fixture placeholder.
- shadcn/ui + Tailwind v4 theme: denova dark via `@theme` CSS variables + four status-color tokens (per STACK.md standard).
- State boundary: `useChat` owns message stream; Zustand owns app state (current skill / center-panel view / sidebar collapse / preferences).
- source-url href MUST sanitize + allowlist + `rel="noopener noreferrer"` (T-1-08 — mandatory P4 safety, flagged in spike stub).
- Sub-progress (FEUI-06) step copy defined as `data-*` events inside fixtures.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Wire contract & schema (FROZEN — P2/P4 shared boundary)
- `internal/spike/uimsg.go` — UIMessageChunk struct + 16 kebab-case `ValidTypes` (incl. `data-policy` carrying Verdict); camelCase fields. AI SDK v5 parser silently drops snake_case — fixtures MUST comply.
- `internal/spike/verdict.go` — `Verdict` struct `{status, confidence, citations[], review_required}` + `ValidStatuses` {合规/不合规/需补充/unanswered}. Source of truth for verdict card rendering.
- `.planning/SKILL_SCHEMA.md` — SKILL.md 7-field schema; basis for center-panel SKILL.md preview parsing.

### Locked design decisions (from sketches/spikes)
- `.claude/skills/sketch-findings-ai-agent/references/layout-three-panel.md` — three-column 260/flex/420 + draggable resize + localStorage persistence.
- `.claude/skills/sketch-findings-ai-agent/references/chat-input-attach-skill.md` — drag-drop + @-mention unified dropdown.
- `.claude/skills/sketch-findings-ai-agent/SKILL.md` — dark denova theme + four status colors (compliant green / needs-info yellow / non-compliant red / unanswered gray).
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — temperature passthrough + structured-output + refusal-by-construction findings (informs refusal UX semantics).

### Planning
- `.planning/ROADMAP.md` §Phase 4 — success criteria SC-1..6.
- `.planning/REQUIREMENTS.md` — FEUI-01..06, SYSK-03, KNOW-05.
- `web/src/App.tsx` — spike stub (`renderPart`: text/reasoning/source-url/source-document); P4 replacement starting point.
- `integration/contract/contract_test.go` — contract drift detection; P4 must not break it.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/App.tsx` `renderPart()` — already branches text/reasoning/source-url/source-document; expand into full component tree.
- `useChat` + `DefaultChatTransport` (`@ai-sdk/react` v4 + `ai` v7) — wire contract validated; P4 mock transport implements the same Transport interface.
- `internal/spike.EmitChunk` + `hertz_sse.go` — Go-side SSE emission; fixture chunk-sequence format mirrors this.
- `testdata/skills/*/SKILL.md` + `testdata/docs/{authority,historical,personal,draft}/*.md` — real 4-tier fixtures; reusable as mock data for SKILL.md preview + citation chunk content.

### Established Patterns
- UIMessageChunk kebab-case type + camelCase field (AI SDK v5 parser drops snake_case silently) — fixtures must obey.
- `data-policy` custom type carries Verdict (distinct from `source-url` which carries citation URL).
- Compile-time contract lock via `spike.Verdict` import graph — P4 frontend TS types should mirror the Go struct to catch drift.

### Integration Points
- Transport swap point: `web/src/App.tsx` `spikeTransport` (mock ↔ real).
- `/api/spike/chat` → P4 `/api/chat` (Hertz, Phase 2 product).
- `/api/skills` → skill picker data source (Phase 2 product; P4 fixture placeholder).

</code_context>

<specifics>
## Specific Ideas

- denova's three-column "center = document preview" pattern is the core reference (NOT cursor's "center = chat"). Agent operates on the center doc; user asks in the right chat.
- Compliance verdict is the demo's soul — verdict card is the center panel's default protagonist.
- Reasoning progressive disclosure specifically guards against GLM-5.2's long reasoning chains burying the answer.

</specifics>

<deferred>
## Deferred Ideas

- Real multi-session management / conversation-history persistence → Phase 5 (SYSK-04 hot-reload context) or later.
- Knowledge graph browser (tier → doc → chunk visualization) → Phase 5 (KNOW-06).
- Counter-evidence side-by-side / clause redline rewrite → Phase 5 (CMPL-06/07).
- Policy version filtering UI (`effective_to`) → Phase 5 (CMPL-08).
- Real sub-progress decision events → wire after Phase 3 (P4 uses mock step events first).

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` (todo.match-phase score 0.6) — keyword false-match (date/phase/skill/api); unrelated to P4 UI scope. Not folded.

</deferred>

---

*Phase: 4-Streaming Trust UI*
*Context gathered: 2026-08-11*
