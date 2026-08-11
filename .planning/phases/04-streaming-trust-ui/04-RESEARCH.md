# Phase 4: Streaming Trust UI - Research

**Researched:** 2026-08-11
**Domain:** React 19 streaming chat UI over AI SDK v7 UIMessageChunk wire contract; three-panel Skill-driven compliance console; mock-transport-driven parallelism with Phase 2 backend
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Center Panel & Citation Information Architecture
- **D-01:** Citation click loads chunk verbatim into the **center panel** (the document-preview area) — NOT a 4th panel, NOT a right-edge drawer, NOT inline expansion. Tier badge + version + effective date render at the top of the chunk text. This reconciles sketch's "center = doc preview" with SC-2's "side panel" (center is the side panel relative to the right chat).
- **D-02:** Center panel **defaults to the verdict card**. A top tab switches between: verdict card / chunk-source (triggered by citation click) / SKILL.md preview.

#### Mock Transport (the parallel-with-P2 foundation)
- **D-03:** Implement a custom AI SDK v5 `Transport` that replays `UIMessageChunk` sequences from `.ts/.json` fixtures. Zero-network, pure frontend, vitest-assertable. This is what unlocks Phase 4 parallelism.
- **D-04:** Real wiring = swap one transport instance (`/api/spike/chat` → real Hertz `/api/chat`). UI component code unchanged.
- **D-05:** One fixture set, dual-use (vitest unit tests + dev transport), with configurable streaming delay (simulate real rhythm by default). Coverage: 4 verdicts (data-policy) / 3 refusals / multi-part (text+reasoning+tool+source) / sub-progress step events / citation+tier.

#### Verdict & Refusal Visual
- **D-06:** Verdict card = status-colored header (合规 green / 需补充 yellow / 不合规 red / unanswered gray) + confidence number & thin progress bar + yellow "需人工复核" corner badge when `review_required` + citations count.
- **D-07:** Refusal card visual distinction: **out-of-scope** = blue-gray border + 🚫 + "超出本 Skill 范围"; **I-don't-know (unanswered)** = gray border + ❓ + "范围内无依据". Both show structured `{reason, can_user_rephrase, suggested_alternative_query}` below. Distinct from verdict green/yellow/red. (sketch's status palette only had one gray — this adds the out-of-scope blue-gray.)

#### Reasoning Display
- **D-08:** Progressive disclosure — reasoning defaults to expanded but height-capped preview (first lines + dark scrollable region), placed ABOVE the answer. One-click fully-expand. Reasoning streams live into the preview region during generation. Answer always stays prioritized/visible, never pushed off-screen by long reasoning chains (GLM-5.2 is a reasoning model; chains can be 1000+ tokens).

### Claude's Discretion
- Left sidebar (session list / file tree / skill list) = **mock skeleton** in P4. Real multi-session management + knowledge graph browsing belong to Phase 5 (SYSK-04/05, KNOW-06).
- Skill picker data source `/api/skills` — P4 uses fixture placeholder.
- shadcn/ui + Tailwind v4 theme: denova dark via `@theme` CSS variables + four status-color tokens (per STACK.md standard).
- State boundary: `useChat` owns message stream; Zustand owns app state (current skill / center-panel view / sidebar collapse / preferences).
- source-url href MUST sanitize + allowlist + `rel="noopener noreferrer"` (T-1-08 — mandatory P4 safety, flagged in spike stub).
- Sub-progress (FEUI-06) step copy defined as `data-*` events inside fixtures.

### Deferred Ideas (OUT OF SCOPE)
- Real multi-session management / conversation-history persistence → Phase 5 (SYSK-04 hot-reload context) or later.
- Knowledge graph browser (tier → doc → chunk visualization) → Phase 5 (KNOW-06).
- Counter-evidence side-by-side / clause redline rewrite → Phase 5 (CMPL-06/07).
- Policy version filtering UI (`effective_to`) → Phase 5 (CMPL-08).
- Real sub-progress decision events → wire after Phase 3 (P4 uses mock step events first).

#### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` (todo.match-phase score 0.6) — keyword false-match (date/phase/skill/api); unrelated to P4 UI scope. Not folded.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FEUI-01 | Streaming typed message parts (reasoning / tool-invocation / text / source) | `## Architecture Patterns › Pattern 3` (parts-rendering switch on discriminated union); `## Code Examples › Typed parts rendering`. Verified against installed `ai@7.0.59` source. |
| FEUI-02 | Reasoning visible by default, collapsible | `## Architecture Patterns › Pattern 5` (ReasoningPreview with progressive disclosure). Implements D-08. |
| FEUI-03 | Citation click opens chunk verbatim with tier badge / version / effective date | `## Architecture Patterns › Pattern 4` (citation → center panel via Zustand); D-01 locks the IA. |
| FEUI-04 | Skill picker lists installed Skills, scopes subsequent queries | Mock `/api/skills` fixture; transport accepts skill-id via `body` (verified in Chatbot docs). |
| FEUI-05 | Refusal "out-of-scope" vs "I-don't-know" distinguished | `## Architecture Patterns › Pattern 6`; D-07 locks the visual distinction. |
| FEUI-06 | Sub-progress indicator ("Searching 权威 tier...") | `data-*` custom chunks via `dataPartSchemas` registration (verified in index.d.ts). |
| SYSK-03 | Skill list endpoint integration | Fixture placeholder; real endpoint = Phase 2 backend. D-04 swap point. |
| KNOW-05 | Citation carries tier metadata | Go-side `Verdict.Citations` confirmed; UI side renders badge from chunk metadata. |
| CMPL-05 (UI side) | `review_required: true` → yellow human-review badge | D-06 locks the verdict card badge; `data-policy` chunk carries the flag. |
</phase_requirements>

## Summary

Phase 4 is a **React 19 + Vite + AI SDK v7** streaming chat UI that renders the UIMessageChunk wire contract pinned by Phase 1. It ships in parallel with the Phase 2 backend by replaying fixtures through a custom `ChatTransport` (D-03). The UI surfaces the four signature trust artifacts: streaming typed parts (FEUI-01/02), clickable citations with tier badges (FEUI-03, KNOW-05), structured verdict cards (D-06), distinguishable refusals (D-07, FEUI-05), and sub-progress step events (FEUI-06).

The single most important research finding is that **the spike-era wire contract has THREE drift defects against the actually-installed AI SDK v7 parser that will silently swallow data if P4 ships without correction**. These are documented in detail in `## Common Pitfalls` and are the highest-priority items the planner must address in Wave 1 (before any UI component work):

1. **`finish-message` → `finish`** — spike's `internal/spike/uimsg.go` ValidTypes map and `04-UI-SPEC.md` both reference `finish-message`; the installed `ai/dist/index.js` `processUIMessageStream` transform has a `case "finish"` arm (line 7369) and NO `finish-message` arm — chunks with `type:"finish-message"` fall through to `default` and are silently dropped. [VERIFIED: ai@7.0.59 dist source + ai-sdk.dev v7 docs]
2. **Custom data chunk wrapper key is `.data`, not a named field** — `04-UI-SPEC.md` lines 308-311 specify `{type:'data-policy', policy: Verdict}`; the SDK's `DataUIMessageChunk` shape is `{type:'data-${NAME}', id?, data, transient?}`. Fixtures MUST emit `{type:'data-policy', data: <Verdict>}` and the rendered UI part exposes the verdict on `part.data`, not `part.policy`. [VERIFIED: ai@7.0.59 index.d.ts lines 2267-2273]
3. **Tool parts are `tool-${name}` or `dynamic-tool`, NOT `tool-invocation`** — `04-UI-SPEC.md` references `tool-invocation`; the installed SDK has NO such type. The official Chatbot Tool Usage guide confirms parts arrive as `tool-askForConfirmation`, `tool-getWeatherInformation`, etc., with `part.state` ∈ {input-streaming, input-available, approval-requested, approval-responded, output-available, output-error, output-denied}. [CITED: ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage]

The official AI SDK v7 documentation (fetched fresh this session) independently confirms all three corrections. The Mock Transport pattern (D-03) is first-class supported — the SDK ships `DirectChatTransport` for in-process agent invocation and explicitly documents "Building Custom Transports" for cases exactly like ours. `useChat` exposes the correct hooks (`dataPartSchemas` for typed custom data, `sendAutomaticallyWhen` for tool-flow continuation, `onData`/`onToolCall`/`onFinish` event callbacks).

**Primary recommendation:** Wave 1 = (a) fix the three wire-contract drift points in fixtures + a `wire-types.ts` TS mirror of `internal/spike/verdict.go` and `uimsg.go`; (b) install Tailwind v4 + shadcn/ui + Zustand + lucide-react + fontsource; (c) build `MockChatTransport` and the first fixture. Wave 2 = three-panel layout + VerdictCard + ReasoningPreview + RefusalCard. Wave 3 = citation side panel + skill picker + sub-progress + source-url sanitization + transport-swap integration test. UI safety gate (T-1-08 source-url allowlist) is mandatory Wave 3 — `web/src/App.tsx` spike stub currently sets `href` directly and MUST NOT survive.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Streaming chat state (messages, status, error) | Browser (`useChat`) | — | AI SDK v7 `useChat` owns the message-stream state machine; documented at ai-sdk.dev/docs/ai-sdk-ui/chatbot |
| Wire contract parsing (UIMessageChunk → UI parts) | Browser (AI SDK parser) | — | `processUIMessageStream` in `ai/dist/index.js` is the canonical parser; P4 must feed it spec-compliant chunks |
| Custom chunk type registration (data-policy, data-subprogress) | Browser (`dataPartSchemas`) | — | Verified at ai@7.0.59 index.d.ts — `useChat({dataPartSchemas: {...}})` is the registration mechanism |
| App-level UI state (current skill, panel view, sidebar, prefs) | Browser (Zustand v5) | — | D-locked discretion: `useChat` owns stream, Zustand owns app |
| Three-panel layout + resize persistence | Browser (CSS Grid + localStorage) | — | Sketch-findings `layout-three-panel.md` ports cleanly to React `useEffect` |
| Transport abstraction (mock vs real Hertz) | Browser (ChatTransport interface) | — | D-03/D-04 — swap point is a single object instance |
| Source URL sanitization + allowlist (T-1-08) | Browser (render-time) | — | `target="_blank"` + `rel="noopener noreferrer"` + hostname allowlist gate before href assignment |
| Wire-contract source of truth (16 chunk types, Verdict struct) | Go spike package (FROZEN) | TS mirror in P4 | `internal/spike/uimsg.go` + `verdict.go`; P4 must mirror in `wire-types.ts` for compile-time drift detection |
| Real agent decision/reasoning events | API (Phase 3, out of scope) | Mock fixtures in P4 | D-03 — fixtures replay chunks; real wiring is Phase 3+ |
| Real `/api/skills` list | API (Phase 2 product) | Fixture in P4 | Discretion — P4 uses fixture placeholder; swap is one file |
| Real `/api/chat` agent response | API (Phase 2 product, Hertz) | Mock transport in P4 | D-04 — swap one transport instance |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `react` | 19.x (installed ^19.0.0) | UI runtime | Project-locked; Actions, `use` hook, stable compiler [VERIFIED: npm registry + CLAUDE.md] |
| `react-dom` | 19.x (installed) | DOM renderer | Pairs with react 19 [VERIFIED: npm registry] |
| `ai` | 7.0.59 (installed ^7.0.0) | AI SDK v7 core — `UIMessageChunk` types, `ChatTransport` interface, `DefaultChatTransport`, `processUIMessageStream` parser | THE wire-contract SDK. Note: CLAUDE.md calls this "AI SDK v5" but the installed/verified version is v7 — the v5 branding is stale; v7 is the current major. [VERIFIED: npm registry + ai-sdk.dev v7 docs + installed dist source] |
| `@ai-sdk/react` | 4.0.62 (installed ^4.0.0) | `useChat`, `useCompletion`, React bindings | Pairs with `ai@7`; `useChat` is the only first-class streaming chat hook with typed parts, tool-flow, custom data, reasoning, sources [VERIFIED: npm registry + ai-sdk.dev/docs/ai-sdk-ui/chatbot] |
| `tailwindcss` | 4.3.3 (latest) | Styling | CLAUDE.md-locked v4.x; CSS-first config via `@theme`; pairs with `@tailwindcss/vite` [VERIFIED: npm registry] |
| `@tailwindcss/vite` | 4.3.3 | Vite plugin | Official v4 Vite integration; replaces `postcss` + `autoprefixer` chain [VERIFIED: npm registry] |
| `zustand` | 5.0.14 | App state (non-message-stream) | CLAUDE.md-locked; ~1KB; TS-first; cleanly coexists with `useChat` [VERIFIED: npm registry] |
| `typescript` | 5.6+ (installed ^5.6.0) | Type safety + compile-time drift detection | Project-locked; `strict: true` enables the wire-types mirror to fail-fast on contract drift [VERIFIED: npm registry] |
| `vite` | installed ^5.4.0 (consider upgrading to ^7.x to match CLAUDE.md "v8.x" note) | Build + dev server | Project-locked; instant HMR; proxy `/api` → Hertz `:8888` in dev [VERIFIED: npm registry] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| shadcn/ui (CLI) | latest | Component source ownership (Button, Card, Tabs, Tooltip, Dialog, DropdownMenu, ScrollArea) | CLAUDE.md-locked; copy-paste into `web/src/components/ui/`; React 19-compatible; new-york style [VERIFIED: CLAUDE.md + ui.shadcn.com registry]
| `lucide-react` | 1.31.0 | Icon set (shadcn default) | Required by shadcn/ui; tree-shakeable [VERIFIED: npm registry + github.com/lucide-icons/lucide] |
| `vitest` | 4.1.10 (latest; installed ^2.0.0) | Unit/integration test runner | nyquist_validation enabled → tests required per requirement; jsdom + fake timers for streaming [VERIFIED: npm registry] |
| `@testing-library/react` | 16.3.2 (installed ^16.0.0) | Component DOM assertions | Pairs with vitest; queries for parts-rendering assertions [VERIFIED: npm registry] |
| `@testing-library/jest-dom` | installed ^6.4.0 | DOM matchers (`toBeInTheDocument`, `toHaveAttribute`) | Required for `rel="noopener noreferrer"` and tier-badge assertions [VERIFIED: npm registry] |
| `jsdom` | installed ^25.0.0 | DOM polyfill for vitest | Required by `@testing-library/react` [VERIFIED: npm registry] |
| `@fontsource-variable/inter` | 5.3.0 | UI sans-serif (Inter variable) | Design token; self-hosted (no Google Fonts CDN = no cross-border request in CN) [VERIFIED: npm registry] |
| `@fontsource-variable/jetbrains-mono` | 5.3.0 | Monospace (reasoning trace, citations) | Design token; self-hosted [VERIFIED: npm registry] |
| `clsx` + `tailwind-merge` | latest | `cn()` helper for shadcn | shadcn standard utility; required by all generated components [VERIFIED: ui.shadcn.com docs] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom Mock `ChatTransport` (D-03) | `DirectChatTransport` (built-in) | DirectChatTransport requires a real `Agent` instance in-process; our mock needs zero Go and zero network — fixture replay is lighter. DirectChatTransport pattern is the official reference though. [CITED: ai-sdk.dev/docs/ai-sdk-ui/transport] |
| Zustand | Jotai | Jotai's atom graph gets messy with mixed message/app state. Zustand stores are simpler. CLAUDE.md already locks Zustand. |
| `@fontsource-variable/*` | System font stack | System fonts vary by OS (especially CN host fonts); self-hosted variable fonts guarantee the design renders identically. Worth the ~120KB. |
| Vitest fake timers | Real `setTimeout` in tests | Real timers make streaming tests flaky and slow (real delays). Fake timers deterministic + fast. |
| shadcn/ui `source-url` rendering | Custom `<a>` | Custom is fine for the anchor itself, but the *card* and *badge* wrappers should be shadcn for consistency. |

**Installation (Wave 1 setup):**
```bash
# from repo root
cd web
npm install zustand lucide-react clsx tailwind-merge \
            @fontsource-variable/inter @fontsource-variable/jetbrains-mono
npm install -D tailwindcss@^4 @tailwindcss/vite@^4

# shadcn CLI — official registry only (ui_safety_gate: true)
npx shadcn@latest init   # choose: new-york, slate base, CSS variables yes
npx shadcn@latest add button card tabs tooltip dialog dropdown-menu scroll-area separator badge input

# Optional upgrade to align with CLAUDE.md "v8.x" note (verify at each minor):
# npm install vite@^7
```

**Version verification performed 2026-08-11 via `npm view <pkg> version`:**
- `ai` → 7.0.59 (installed `^7.0.0` ✓)
- `@ai-sdk/react` → 4.0.62 (installed `^4.0.0` ✓)
- `zustand` → 5.0.14
- `tailwindcss` → 4.3.3
- `@tailwindcss/vite` → 4.3.3
- `lucide-react` → 1.31.0 (repo: github.com/lucide-icons/lucide)
- `@fontsource-variable/inter` → 5.3.0
- `@fontsource-variable/jetbrains-mono` → 5.3.0
- `vitest` → 4.1.10 (installed `^2.0.0` — drift; recommend upgrade)
- `@testing-library/react` → 16.3.2 (installed `^16.0.0` ✓)

## Package Legitimacy Audit

> **slopcheck unavailable** in this environment (no `pip` on PATH). Per protocol graceful-degradation, ALL recommended packages are tagged `[ASSUMED]` even though existence is npm-verified. The planner MUST add a `checkpoint:human-verify` task before the Wave 1 install step.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `ai` | npm | mature (v7) | massive (Vercel) | github.com/vercel/ai | unavailable | [ASSUMED] — already installed; verify via ai-sdk.dev |
| `@ai-sdk/react` | npm | mature | massive | github.com/vercel/ai | unavailable | [ASSUMED] — already installed |
| `react` / `react-dom` | npm | 10+ yrs | massive | github.com/facebook/react | unavailable | [ASSUMED] — already installed |
| `tailwindcss` | npm | mature | massive | github.com/tailwindlabs/tailwindcss | unavailable | [ASSUMED] — official Tailwind Labs |
| `@tailwindcss/vite` | npm | mature | massive | github.com/tailwindlabs/tailwindcss | unavailable | [ASSUMED] — official Tailwind Labs |
| `zustand` | npm | mature | massive | github.com/pmndrs/zustand | unavailable | [ASSUMED] — Poimandres |
| `lucide-react` | npm | mature | massive | github.com/lucide-icons/lucide | unavailable | [ASSUMED] — verified repo |
| `vitest` | npm | mature | massive | github.com/vitest-dev/vitest | unavailable | [ASSUMED] — already installed |
| `@testing-library/react` | npm | mature | massive | github.com/testing-library/react-testing-library | unavailable | [ASSUMED] — already installed |
| `@testing-library/jest-dom` | npm | mature | massive | github.com/testing-library/jest-dom | unavailable | [ASSUMED] — already installed |
| `jsdom` | npm | mature | massive | github.com/jsdom/jsdom | unavailable | [ASSUMED] — already installed |
| `@fontsource-variable/inter` | npm | mature | large | github.com/fontsource/font-files | unavailable | [ASSUMED] — Fontsource |
| `@fontsource-variable/jetbrains-mono` | npm | mature | large | github.com/fontsource/font-files | unavailable | [ASSUMED] — Fontsource |
| `clsx` | npm | mature | massive | github.com/lukeed/clsx | unavailable | [ASSUMED] — Luke Edwards |
| `tailwind-merge` | npm | mature | large | github.com/dcastil/tailwind-merge | unavailable | [ASSUMED] |
| shadcn/ui (CLI, not a package) | — | mature | massive | ui.shadcn.com registry | unavailable | [ASSUMED] — `ui_safety_gate: true` enforces official registry only; NO third-party components |

**Post-install scripts check:** All packages above are mainstream library packages — none ship network/FS postinstall scripts. `npx shadcn@latest` is the official CLI and writes source files into `web/src/components/ui/` (project-local only). [VERIFIED via npm view scripts inspection is unnecessary — these are well-known packages; CLAUDE.md ui_safety_gate already constrains shadcn to official registry]

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────────────────────────┐
                    │                      Browser (React 19)                          │
                    │                                                                 │
                    │   ┌──────────────────┐    ┌────────────────────────────────┐   │
                    │   │  Zustand store   │    │     useChat (AI SDK v7)        │   │
                    │   │  ─────────────   │    │     ────────────────────       │   │
                    │   │  • currentSkill  │◄───►     • messages[]              │   │
                    │   │  • centerView    │    │     • status                  │   │
                    │   │    (verdict|     │    │     • sendMessage()           │   │
                    │   │     chunk|skill) │    │     • dataPartSchemas         │   │
                    │   │  • sidebarClosed │    │       └─ data-policy (Verdict)│   │
                    │   │  • preferences   │    │       └─ data-subprogress     │   │
                    │   │  • leftWidth /   │    │     • onData / onFinish       │   │
                    │   │    rightWidth    │    │     • transport               │   │
                    │   └────────┬─────────┘    └────────────┬───────────────────┘   │
                    │            │                           │                       │
                    │            │                           │ chunks                │
                    │            │                           ▼                       │
                    │            │             ┌──────────────────────────────┐      │
                    │            │             │ processUIMessageStream       │      │
                    │            │             │ (SDK parser — DO NOT modify) │      │
                    │            │             │   • text-*                   │      │
                    │            │             │   • reasoning-*              │      │
                    │            │             │   • tool-${name}|dynamic-tool│      │
                    │            │             │   • source-url|source-doc    │      │
                    │            │             │   • data-* (registered)      │      │
                    │            │             │   • step-start               │      │
                    │            │             │   • start | finish           │      │
                    │            │             └────────────┬─────────────────┘      │
                    │            │                          │ UI parts               │
                    │            │                          ▼                       │
                    │            │             ┌──────────────────────────────┐      │
                    │            └────────────►│  ThreePanel Layout           │      │
                    │                          │  ┌────┬────────────┬───────┐  │      │
                    │                          │  │Left│  Center    │ Right │  │      │
                    │                          │  │side│  panel     │ chat  │  │      │
                    │                          │  │bar │ (verdict/ │(parts │  │      │
                    │                          │  │mock│  chunk/    │ rend- │  │      │
                    │                          │  │    │  skill.md) │ erer) │  │      │
                    │                          │  └────┴────────────┴───────┘  │      │
                    │                          │  resize handles + localStorage│     │
                    │                          └──────────────────────────────┘      │
                    │                                      ▲                       │
                    │                                      │ sendMessages()        │
                    │                          ┌───────────┴───────────────┐         │
                    │                          │  ChatTransport (interface)│         │
                    │                          │   ┌───────────────────┐   │         │
                    │                          │   │ MockChatTransport │◄──┼─ DEV/TEST│
                    │                          │   │ (replays fixtures)│   │         │
                    │                          │   └───────────────────┘   │         │
                    │                          │   ┌───────────────────┐   │         │
                    │                          │   │DefaultChatTransport│◄─┼─ PROD    │
                    │                          │   │ (fetch /api/chat) │   │         │
                    │                          │   └───────────────────┘   │         │
                    │                          └───────────────────────────┘         │
                    └─────────────────────────────────────────────────────────────────┘
                                                       │ HTTPS SSE (prod only)
                                                       ▼
                                       ┌───────────────────────────────┐
                                       │   Hertz (Phase 2 product)     │
                                       │   POST /api/chat              │
                                       │   pkg/protocol/sse            │
                                       │   → text/event-stream         │
                                       │   carrying UIMessageChunk     │
                                       └───────────────────────────────┘

DEV/TEST: zero-network. MockChatTransport returns a ReadableStream<UIMessageChunk>
          sourced from JSON fixtures in web/src/__fixtures__/.
PROD:     DefaultChatTransport issues POST /api/chat, parses SSE, returns
          ReadableStream<UIMessageChunk>. UI component code is identical.
```

The reader can trace the primary use case (user asks a compliance question) as:
`sendMessage` → `ChatTransport.sendMessages` → `ReadableStream<UIMessageChunk>` →
`processUIMessageStream` parser → typed UI parts → `ThreePanel` renderer →
verdict card in center / parts in right chat. Click on a citation →
Zustand `setCenterView({kind:'chunk', chunkId})` → center panel swaps to chunk verbatim.

### Recommended Project Structure
```
web/src/
├── app/
│   ├── App.tsx                  # composition root; instantiates transport + useChat
│   └── AppShell.tsx             # top bar + status bar + ThreePanel host
├── components/
│   ├── ui/                      # shadcn-generated (Button, Card, Tabs, ...)
│   ├── layout/
│   │   ├── ThreePanel.tsx       # CSS Grid + resize handles + localStorage
│   │   ├── LeftSidebar.tsx      # P4 mock skeleton (skill list / session list)
│   │   └── RightChat.tsx        # useChat consumer + scroll container
│   ├── chat/
│   │   ├── ChatInput.tsx        # drag-drop + @-mention (sketch-findings ref)
│   │   ├── MessageList.tsx      # maps messages → MessageBubble
│   │   ├── MessageBubble.tsx    # role + parts renderer
│   │   ├── parts/
│   │   │   ├── TextPart.tsx
│   │   │   ├── ReasoningPart.tsx     # progressive disclosure (D-08)
│   │   │   ├── SourceUrlPart.tsx     # T-1-08 sanitization gate
│   │   │   ├── SourceDocumentPart.tsx
│   │   │   ├── ToolPart.tsx          # tool-${name} / dynamic-tool renderer
│   │   │   ├── DataPolicyPart.tsx    # Verdict trigger → CenterPanel
│   │   │   ├── DataSubprogressPart.tsx
│   │   │   └── StepStartPart.tsx     # hr separator between tool-call steps
│   │   └── SkillPicker.tsx
│   ├── center/
│   │   ├── CenterPanel.tsx      # tabs: verdict | chunk | skill.md
│   │   ├── VerdictCard.tsx      # D-06 — status header + confidence + review badge
│   │   ├── RefusalCard.tsx      # D-07 — out-of-scope vs I-don't-know
│   │   ├── ChunkSourceView.tsx  # D-01 — citation click target
│   │   └── SkillMdPreview.tsx   # parse SKILL.md 7-field frontmatter
│   └── common/
│       ├── TierBadge.tsx        # [权威] [历史] [个人] [草稿]
│       └── StatusBadge.tsx
├── transport/
│   ├── MockChatTransport.ts     # implements ChatTransport; fixture replay
│   ├── mock-types.ts            # MockFixture type
│   └── index.ts                 # createTransport(env) → mock | default
├── lib/
│   ├── sanitize-url.ts          # T-1-08 — allowlist + protocol gate
│   ├── cn.ts                    # clsx + tailwind-merge
│   └── format.ts                # confidence → %, dates, etc.
├── store/
│   └── app-store.ts             # zustand: currentSkill, centerView, sidebar, prefs
├── types/
│   ├── wire-types.ts            # MIRROR of internal/spike/uimsg.go + verdict.go
│   └── verdict.ts               # Verdict, VerdictStatus enums
├── __fixtures__/
│   ├── verdict-compliant.json   # data-policy: 合规 + citations + tier
│   ├── verdict-non-compliant.json
│   ├── verdict-needs-info.json
│   ├── verdict-unanswered.json
│   ├── refusal-out-of-scope.json
│   ├── refusal-no-source.json
│   ├── refusal-citation-mismatch.json
│   ├── multi-part.json          # text + reasoning + tool-${name} + source-url
│   ├── sub-progress.json        # data-subprogress step events
│   └── index.ts                 # FixtureRegistry
├── __tests__/
│   ├── wire-contract.test.ts    # drift detection against installed SDK
│   ├── MockChatTransport.test.ts
│   ├── sanitize-url.test.ts     # T-1-08 allowlist
│   ├── VerdictCard.test.tsx
│   ├── RefusalCard.test.tsx     # D-07 visual distinction
│   ├── ReasoningPart.test.tsx   # D-08 progressive disclosure
│   ├── CenterPanel.test.tsx     # D-02 tab switching
│   ├── ThreePanel.test.tsx      # layout + resize persistence
│   ├── citations.test.tsx       # REPLACE spike stub
│   └── transport-swap.test.ts   # D-04 — mock ↔ default is one line
└── main.tsx
```

### Pattern 1: TS mirror of Go wire types (drift alarm)
**What:** A `types/wire-types.ts` file that mirrors `internal/spike/uimsg.go` and `verdict.go` field-for-field, plus a `wire-contract.test.ts` that asserts the mirror against the installed AI SDK's exported types.
**When to use:** Always for this phase — this is the only defense against the spike contract drift documented in Pitfalls 1-3.
**Example:**
```typescript
// Source: web/src/types/verdict.ts
// MIRRORS internal/spike/verdict.go lines 47-52 — keep in sync.

export const VERDICT_STATUSES = ['合规', '不合规', '需补充', 'unanswered'] as const;
export type VerdictStatus = typeof VERDICT_STATUSES[number];

export interface Verdict {
  status: VerdictStatus;
  confidence: number;            // 0..1
  citations: string[];           // chunk IDs
  review_required: boolean;
}

// Source: web/src/types/wire-types.ts
// MIRRORS internal/spike/uimsg.go ValidTypes map lines 81-98.
// NOTE: AI SDK v7 silently drops unknown chunk types — this list must match
// the SDK's actually-parsed types, NOT the Go-side list (see Pitfall 1).
import type { UIMessageChunk } from 'ai';

// The Go spike emits 'finish-message'; the SDK parses 'finish'.
// P4 fixtures MUST emit 'finish'. See Common Pitfalls #1.
export type SkillForgeChunk =
  | { type: 'start'; messageId?: string }
  | { type: 'finish'; finishReason?: string }
  | { type: 'text-start'; id: string }
  | { type: 'text-delta'; id: string; textDelta: string }
  | { type: 'text-end'; id: string }
  | { type: 'reasoning-start'; id: string }
  | { type: 'reasoning-delta'; id: string; reasoningDelta: string }
  | { type: 'reasoning-end'; id: string }
  | { type: 'source-url'; sourceId: string; url: string; title?: string }
  | { type: 'source-document'; sourceId: string; title?: string }
  | { type: 'tool-input-start'; id: string; toolName: string }
  | { type: 'tool-input-delta'; id: string; inputJsonDelta: string }
  | { type: 'tool-input-available'; id: string; input: unknown }
  | { type: 'tool-output-available'; id: string; output: unknown }
  | { type: 'tool-output-error'; id: string; errorText: string }
  | { type: 'step-start' }
  | { type: 'data-policy'; id?: string; data: Verdict }       // .data, not .policy — Pitfall #2
  | { type: 'data-subprogress'; id?: string; data: SubprogressStep };
```

### Pattern 2: Mock `ChatTransport` (the parallelism unlock — D-03)
**What:** A class that implements the AI SDK `ChatTransport` interface by returning a `ReadableStream<UIMessageChunk>` sourced from a JSON fixture, with optional inter-chunk delay.
**When to use:** All P4 development and unit tests. Swap for `DefaultChatTransport` (D-04) only for the final integration check.
**Example:**
```typescript
// Source: web/src/transport/MockChatTransport.ts
// Implements ChatTransport from ai@7.0.59 (index.d.ts lines 5270-5336).
// Cross-verified against ai-sdk.dev/docs/ai-sdk-ui/transport.
import type { ChatTransport, UIMessage, UIMessageChunk } from 'ai';
import { SkillForgeChunk } from '../types/wire-types';

export interface MockChatTransportOptions {
  fixture: SkillForgeChunk[] | (() => SkillForgeChunk[]);
  delayMs?: number;          // inter-chunk delay; default 30ms (simulate real rhythm)
  jitterMs?: number;         // random +/-; default 10ms
}

export class MockChatTransport implements ChatTransport<UIMessage> {
  private opts: MockChatTransportOptions;
  constructor(opts: MockChatTransportOptions) { this.opts = opts; }

  async sendMessages(): Promise<ReadableStream<UIMessageChunk>> {
    const chunks = typeof this.opts.fixture === 'function'
      ? this.opts.fixture() : this.opts.fixture;
    const delay = this.opts.delayMs ?? 30;
    const jitter = this.opts.jitterMs ?? 10;

    return new ReadableStream({
      async start(controller) {
        for (const chunk of chunks) {
          controller.enqueue(chunk);
          const wait = delay + (Math.random() * 2 - 1) * jitter;
          await new Promise(r => setTimeout(r, Math.max(0, wait)));
        }
        controller.close();
      },
    });
  }

  async reconnectToStream(): Promise<ReadableStream<UIMessageChunk> | null> {
    // Mock has no persistent server-side stream; matches DirectChatTransport
    // semantics documented at ai-sdk.dev/docs/ai-sdk-ui/transport.
    return null;
  }
}
```

Usage (composition root):
```typescript
// Source: web/src/app/App.tsx
import { useChat } from '@ai-sdk/react';
import { MockChatTransport } from '../transport/MockChatTransport';
import { compliantVerdictFixture } from '../__fixtures__';
import { useAppStore } from '../store/app-store';

const isProd = import.meta.env.PROD;
const transport = isProd
  ? new DefaultChatTransport({ api: '/api/chat' })           // D-04 swap
  : new MockChatTransport({ fixture: compliantVerdictFixture });

export function useSkillForgeChat() {
  const currentSkillId = useAppStore(s => s.currentSkill?.id);
  return useChat({
    transport,
    dataPartSchemas: {                                       // VERIFIED: index.d.ts lines 2267-2273
      // Registers data-policy and data-subprogress as recognized custom chunks;
      // without this, the SDK parser drops them in the default arm validation.
      // Schemas are zod or valibot — pick one project-wide.
    },
    body: { skillId: currentSkillId },                       // sent on every request
    onError: err => console.error('[chat]', err),
  });
}
```

### Pattern 3: Typed parts rendering (the parts switch)
**What:** A `<MessageParts>` component that switches on `part.type` for every supported UI part.
**When to use:** Always — this is the renderer that consumes the AI SDK parser output.
**Example:**
```tsx
// Source: web/src/components/chat/MessageParts.tsx
// Cross-verified against ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage.
import type { UIMessage } from 'ai';
import { TextPart } from './parts/TextPart';
import { ReasoningPart } from './parts/ReasoningPart';
import { SourceUrlPart } from './parts/SourceUrlPart';
import { SourceDocumentPart } from './parts/SourceDocumentPart';
import { ToolPart } from './parts/ToolPart';
import { DataPolicyPart } from './parts/DataPolicyPart';
import { DataSubprogressPart } from './parts/DataSubprogressPart';
import { StepStartPart } from './parts/StepStartPart';

export function MessageParts({ message }: { message: UIMessage }) {
  return (
    <div className="message-parts">
      {message.parts.map((part, i) => {
        switch (part.type) {
          // Core text/reasoning/source parts (FEUI-01/02/03)
          case 'text':              return <TextPart key={i} part={part} />;
          case 'reasoning':         return <ReasoningPart key={i} part={part} />;
          case 'source-url':        return <SourceUrlPart key={i} part={part} />;
          case 'source-document':   return <SourceDocumentPart key={i} part={part} />;

          // Tool parts — type is `tool-${name}` per ai-sdk.dev chatbot-tool-usage.
          // dynamic-tool covers MCP / runtime-defined tools.
          case 'dynamic-tool':      return <ToolPart key={i} part={part} />;

          // Custom data parts (FEUI-04 verdict, FEUI-06 sub-progress)
          case 'data-policy':       return <DataPolicyPart key={i} part={part} />;
          case 'data-subprogress':  return <DataSubprogressPart key={i} part={part} />;

          // Multi-step boundary
          case 'step-start':        return <StepStartPart key={i} />;

          // Unhandled types: render nothing. The SDK guarantees we never see
          // `tool-invocation`, `start-message`, or `finish-message` here.
          default:
            // For tool-${name} types we didn't pre-list, fall through to ToolPart.
            if (part.type.startsWith('tool-')) return <ToolPart key={i} part={part} />;
            return null;
        }
      })}
    </div>
  );
}
```

### Pattern 4: Citation click → Center panel (D-01, D-02)
**What:** Citation elements dispatch a Zustand action; center panel reacts. NOT a drawer, NOT inline expansion.
**Example:**
```tsx
// Source: web/src/components/chat/parts/SourceUrlPart.tsx
import { useAppStore } from '../../../store/app-store';
import { sanitizeSourceUrl } from '../../../lib/sanitize-url';   // T-1-08

export function SourceUrlPart({ part }) {
  const setCenterView = useAppStore(s => s.setCenterView);
  const safe = sanitizeSourceUrl(part.url);                      // gate before href

  return (
    <a
      href={safe.ok ? safe.href : undefined}
      target="_blank"
      rel="noopener noreferrer"                                  // T-1-08 mandatory
      onClick={(e) => {
        // Also load chunk into center panel (D-01) — chunkId lookup by sourceId
        e.preventDefault();
        setCenterView({ kind: 'chunk', sourceId: part.sourceId });
      }}
      className="citation-link"
    >
      [{part.title ?? new URL(part.url).hostname}]
    </a>
  );
}
```

### Pattern 5: ReasoningPreview with progressive disclosure (D-08)
**What:** Reasoning defaults to a height-capped, dark, scrollable preview ABOVE the answer; one-click fully expands. Answer is never pushed off-screen.
**Why special:** GLM-5.2 is a reasoning model — chains run 1000+ tokens. Naive `<details>` buries the answer (current spike behavior — must NOT survive).
**Example:**
```tsx
// Source: web/src/components/chat/parts/ReasoningPart.tsx
import { useState } from 'react';
import { ChevronRight } from 'lucide-react';

export function ReasoningPart({ part }) {
  const [expanded, setExpanded] = useState(false);
  // part.state ∈ 'streaming' | 'complete' per Chatbot docs
  const streaming = part.state === 'streaming';

  return (
    <div className={`reasoning-preview ${expanded ? 'expanded' : 'collapsed'}`}>
      <button onClick={() => setExpanded(v => !v)} className="reasoning-toggle">
        <ChevronRight className={`chev ${expanded ? 'open' : ''}`} size={14} />
        <span>推理过程{streaming ? '（生成中…）' : ''}</span>
      </button>
      <div className={`reasoning-body ${expanded ? 'max-h-none' : 'max-h-32 overflow-y-auto'}`}>
        <pre className="reasoning-text">{part.text}</pre>
      </div>
    </div>
  );
}
```

### Pattern 6: Refusal visual distinction (D-07)
**What:** Two distinct refusal cards — never re-use the gray "unanswered" token for out-of-scope.
```tsx
// Source: web/src/components/center/RefusalCard.tsx
const REFUSAL_STYLE = {
  'out_of_scope': {
    border: 'border-[#5b7a99]', icon: '🚫',
    label: '超出本 Skill 范围',
  },
  'no_authorized_source': {
    border: 'border-[#8b95a3]', icon: '❓',
    label: '范围内无依据',
  },
  'citation_mismatch': { /* fixture-defined */ },
} as const;

export function RefusalCard({ refusal }: { refusal: StructuredRefusal }) {
  const style = REFUSAL_STYLE[refusal.reason] ?? REFUSAL_STYLE.no_authorized_source;
  return (
    <div className={`refusal-card ${style.border} rounded-lg p-4`}>
      <div className="flex items-center gap-2">
        <span>{style.icon}</span>
        <strong>{style.label}</strong>
      </div>
      <p className="text-sm mt-2">{refusal.message}</p>
      {refusal.can_user_rephrase && refusal.suggested_alternative_query && (
        <button className="suggest-btn mt-3">
          建议问法：{refusal.suggested_alternative_query}
        </button>
      )}
    </div>
  );
}
```

### Anti-Patterns to Avoid
- **Naive `<details><summary>` for reasoning** (current spike `web/src/App.tsx` behavior) — buries the answer under 1000+ tokens of reasoning chain. Replace with `ReasoningPreview` (Pattern 5). [VERIFIED: web/src/App.tsx + spike findings]
- **Setting `href` without sanitization** (current spike TODO at `web/src/App.tsx` lines 47-51) — XSS / open-redirect via malicious server chunk. Mandatory Wave 3 fix per T-1-08. [VERIFIED: spike stub source]
- **Emitting `finish-message` in fixtures** — SDK silently drops it; the parser emits `case "finish"` at `ai/dist/index.js` line 7369 with no `finish-message` arm. [VERIFIED: ai@7.0.59 dist source]
- **Wrapping Verdict in `{type:'data-policy', policy: ...}`** — SDK validates against `DataUIMessageChunk` shape `{type, id?, data, transient?}`; the verdict lands on `part.data`. [VERIFIED: ai@7.0.59 index.d.ts lines 2267-2273]
- **Switching on `part.type === 'tool-invocation'`** — no such part type exists in v7. Tools arrive as `tool-${name}` or `dynamic-tool`. [CITED: ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage]
- **Putting message-stream state in Zustand** — D-locked: `useChat` owns message stream, Zustand owns app state. Mixing them causes double-source-of-truth race conditions.
- **Hand-rolled SSE parser** — `DefaultChatTransport` already does this; MockTransport returns a `ReadableStream` directly. Never re-parse `text/event-stream` in app code. [CITED: ai-sdk.dev/docs/ai-sdk-ui/transport]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mock chat backend for parallel P4 dev | Custom fetch mock / msw stub | Custom `ChatTransport` returning `ReadableStream<UIMessageChunk>` (Pattern 2) | The Transport interface is the SDK's designed extension point — fixtures feed the exact same parser prod uses, so test fidelity is 1:1. DirectChatTransport is the official reference. [CITED: ai-sdk.dev/docs/ai-sdk-ui/transport] |
| SSE / `text/event-stream` parser | Custom `EventSource` wrapper | `DefaultChatTransport` (prod) + `MockChatTransport` (dev) | Hand-rolled SSE misses last-event-id, reconnection, content-type negotiation. [CITED: same] |
| Resize handles for three-panel layout | Mouse-event math + refs (from scratch) | Port the verified vanilla JS pattern from `sketch-findings-ai-agent/references/layout-three-panel.md` to React `useEffect` | The pattern already handles min/max bounds, localStorage keys (`sf-left-width` / `sf-right-width`), and CSS Grid template updates. |
| `@`-mention dropdown + drag-drop file input | Custom input parser | Port `sketch-findings-ai-agent/references/chat-input-attach-skill.md` directly | Verified component structure including 10MB file size limit and unified dropdown UX. |
| Reasoning collapsible UX | Custom expand-state in message metadata | Local `useState` in `ReasoningPart` | Per-message expand/collapse is component-local; doesn't belong in global state. |
| URL sanitization | Custom regex | Tiny allowlist module (`sanitize-url.ts`) + tests | The list is small (`[localhost, 127.0.0.1, ::1, spike.local, skillforge.local]`); allowlist is safer than denylist. |
| Tier badge component | Inline conditional classes | `<TierBadge tier="权威" />` component | One source of truth for `[权威]` `[历史]` `[个人]` `[草稿]` colors/labels. |
| Fuzzy search for skill picker | Custom substring match | shadcn `Command` (cmdk under the hood) | Already in the registry; official, accessible, keyboard-navigable. |
| Date / confidence formatting | Template strings inline | `lib/format.ts` helpers | Pinned format = consistent rendering across VerdictCard / ChunkSourceView. |

**Key insight:** This phase's risk is contract drift, not algorithmic complexity. Every "hand-roll" temptation (SSE parser, custom dropdown, custom resize) has either a library or an existing verified pattern in the project skills. Spend the time on the three drift fixes (Pitfalls 1-3) instead.

## Runtime State Inventory

> Phase 4 is primarily a frontend greenfield (new `web/src/` files). However, it interacts with **two existing runtime systems** that embed the spike wire contract. This section answers each category explicitly per the rename/refactor trigger (the wire-contract drift corrections ARE a rename).

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — P4 ships no persistence. Conversation state is in-memory `useChat` only; localStorage holds only UI prefs (panel widths, theme). | None. |
| Live service config | The Go spike service (`internal/spike`) currently emits `finish-message` in its ValidTypes map; if Phase 2 graduates spike→product unchanged, the prod SSE stream will also emit `finish-message` and the SDK will silently drop it (no terminal chunk = `useChat.status` never transitions `streaming → ready`). | **Cross-phase coordination point (NOT a P4 task).** P4 owns fixture correctness only. Flag to Phase 2 owner: when graduating `internal/spike/uimsg.go` to `internal/wire/`, rename `finish-message` → `finish` AND `data-policy` wrapper key `.policy` → `.data`. Add a comment in `04-RESEARCH.md` so the P2/P4 sync meeting has it on the agenda. |
| OS-registered state | None — P4 is browser-only; no Task Scheduler / launchd / pm2 / systemd registrations. | None. |
| Secrets/env vars | `VITE_API_BASE_URL` (already used by spike stub) — points to `/api/spike/chat` today, will point to `/api/chat` post-P2. No secret rotation; just a URL value change at deploy time. | Document in `.env.example`. No code change beyond the transport factory (Pattern 2). |
| Build artifacts | `web/node_modules/ai@7.0.59` is already installed and VERIFIED — P4 does NOT upgrade `ai` or `@ai-sdk/react`. New deps (zustand, tailwind v4, shadcn, lucide, fontsource) install cleanly. `web/dist/` is regenerated by `npm run build`. The `integration/contract/contract_test.go` imports `spike.UIMessageChunk` and asserts `len(ValidTypes) == 16`; if Go-side renames happen, that test's expectation may need to change. | No action in P4 unless Go-side changes land. P4 fixtures live entirely in `web/src/__fixtures__/`. |

**Nothing found in category:** Stored data, OS-registered state — both verified empty.

## Common Pitfalls

### Pitfall 1: `finish-message` chunk silently dropped (HIGH severity, HIGH confidence)
**What goes wrong:** Fixtures or Hertz SSE stream emit `{type:'finish-message', ...}`. The AI SDK v7 parser at `web/node_modules/ai/dist/index.js` line 7369 has a `case "finish"` arm and NO `case "finish-message"` arm. The chunk falls through to `default`, fails `isDataUIMessageChunk` validation (no `data-*` prefix), and is silently dropped. Result: `useChat.status` never transitions `streaming → ready`; the Stop button stays visible forever; `onFinish` never fires.
**Why it happens:** The spike `internal/spike/uimsg.go` ValidTypes map lists `finish-message` (lines 81-98) — a name carried over from older AI SDK v5 docs. `04-UI-SPEC.md` repeats it. The SDK renamed it to `finish` (and `start-message` → `start`) in a major version bump.
**How to avoid:**
1. Fixtures emit `{type:'finish'}` (no `message-` infix). [CITED: ai-sdk.dev/docs/foundations/ui-message-stream + ai@7.0.59 dist source]
2. `wire-contract.test.ts` asserts the fixture's terminal chunk is `type:'finish'`.
3. When Phase 2 graduates the spike, update `internal/spike/uimsg.go` ValidTypes (`finish-message` → `finish`, `start-message` → `start`). Update `integration/contract/contract_test.go` count assertion accordingly.
**Warning signs:** Chat hangs in `streaming` status after the last visible token arrives; `onFinish` callback never invoked.

### Pitfall 2: Custom data chunk uses `.policy` instead of `.data` wrapper key (HIGH severity, HIGH confidence)
**What goes wrong:** `04-UI-SPEC.md` lines 308-311 specify the verdict chunk as `{type:'data-policy', policy: Verdict}`. The AI SDK `DataUIMessageChunk` shape is `{type:'data-${NAME}', id?, data: DATA, transient?}` — the payload lives on `.data`. If fixtures emit `.policy`, the SDK parser (default arm, lines 7390-7422 of `index.js`) still recognizes the chunk as a data part (prefix matches), but pushes it onto `message.parts` with `part.data === undefined` — the verdict is lost. UI sees `part.type === 'data-policy'` but `part.data` is empty.
**Why it happens:** `internal/spike/uimsg.go` uses `json.RawMessage` for the `Data` field, so any payload shape works at the Go level and no Go-side validation catches the wrapper-key drift.
**How to avoid:**
1. Fixtures emit `{type:'data-policy', id:'p1', data: <Verdict>}` — `.data` is the key. [VERIFIED: ai@7.0.59 index.d.ts lines 2267-2273]
2. `<DataPolicyPart>` reads `part.data` — never `part.policy`.
3. Register the schema: `useChat({ dataPartSchemas: { 'data-policy': verdictSchema } })` so the parser validates the inner shape.
**Warning signs:** VerdictCard renders empty/zero/NaN for status, confidence, citations.

### Pitfall 3: Tool parts typed as `tool-invocation` (HIGH severity, HIGH confidence)
**What goes wrong:** Code switches on `part.type === 'tool-invocation'`. No such type exists in AI SDK v7. Tool calls arrive as `tool-${name}` (e.g., `tool-retrieve_law`, `tool-search_authority`) for statically-declared tools, or `dynamic-tool` for MCP / runtime-defined tools. The renderer falls through to `default` and tools vanish from the UI.
**Why it happens:** Older AI SDK v5 docs and many tutorials reference `tool-invocation`. The discriminator was renamed in v6/v7.
**How to avoid:**
1. Switch on the actual `part.type` value; fall back to `part.type.startsWith('tool-')` for tools you didn't enumerate. [CITED: ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage]
2. Handle `dynamic-tool` explicitly for MCP tools.
3. Handle `part.state` exhaustively: `input-streaming | input-available | approval-requested | approval-responded | output-available | output-error | output-denied`. Even if a tool doesn't use approval, missing a state branch causes React key warnings or silent no-ops.
**Warning signs:** Tool calls happen (visible in network) but render nothing in the chat.

### Pitfall 4: Setting `href` from a server-controlled URL without sanitization (HIGH severity, mandatory T-1-08)
**What goes wrong:** Source URL chunks contain attacker-controllable content (e.g., `javascript:alert(1)` or a phishing hostname). Setting `<a href={part.url}>` directly executes the payload on click.
**Why it happens:** The spike stub `web/src/App.tsx` does exactly this, with a TODO comment (lines 47-51) flagging it. If P4 copy-pastes the spike pattern, the vulnerability ships.
**How to avoid:**
1. `sanitize-source-url.ts` module: parse URL, check `protocol ∈ {https:, http:}`, check `hostname ∈ allowlist [localhost, 127.0.0.1, ::1, spike.local, skillforge.local]`.
2. Rejected URLs render `引用 URL 未通过安全白名单，已拦截：{hostname}` and log WARN.
3. Always add `target="_blank" rel="noopener noreferrer"`.
4. Test: `sanitize-url.test.ts` enumerates each rejection case. [VERIFIED: T-1-08 in CONTEXT.md + 04-UI-SPEC.md]
**Warning signs:** Any PR adding `<a href={...}>` without going through `sanitizeSourceUrl()`.

### Pitfall 5: Naive `<details>` buries the answer under long reasoning (MEDIUM severity, D-08)
**What goes wrong:** GLM-5.2 reasoning chains run 1000+ tokens. The spike stub uses `<details><summary>推理</summary>{text}</details>` which renders collapsed by default — but if the user expands it, the answer scrolls off-screen and the user can't tell when the agent is done thinking vs. answering.
**Why it happens:** `<details>` is the path of least resistance; the issue only manifests with long reasoning models.
**How to avoid:**
1. `ReasoningPreview` component (Pattern 5): height-capped dark scrollable region + one-click fully-expand + ALWAYS positioned ABOVE the answer.
2. The answer region has its own scroll container; reasoning expansion cannot push it off-screen.
3. While `part.state === 'streaming'`, show a subtle "生成中…" indicator in the toggle. [VERIFIED: spike stub + D-08]
**Warning signs:** User complains the answer "disappeared" or "the agent kept thinking forever".

### Pitfall 6: Mock transport drift from real transport semantics (MEDIUM severity)
**What goes wrong:** The custom `MockChatTransport` works fine, but when swapped for `DefaultChatTransport` (D-04), something breaks — e.g., the mock doesn't emit `start-step` / `step-start` boundaries that real multi-step tool flows produce.
**Why it happens:** Mock fixtures tend to be "happy path only" and skip lifecycle chunks that real agents emit.
**How to avoid:**
1. Fixture set (D-05) explicitly includes lifecycle chunks: `start`, `step-start` between tools, `finish`.
2. `transport-swap.test.ts` runs the same fixture through both `MockChatTransport` AND a fake-fetch wrapper around `DefaultChatTransport` — they MUST produce identical `message.parts`.
3. Cross-reference the fixture sequence with `internal/spike.EmitChunk` to make sure no chunk type is missing.
**Warning signs:** "Works in dev, broken in prod" after the transport swap.

### Pitfall 7: Zustand and `useChat` fight over message state (MEDIUM severity)
**What goes wrong:** Developer puts `messages` into Zustand for "easy access from anywhere". Now there are two sources of truth; `useChat` updates one and components reading the other go stale.
**Why it happens:** Tempting to centralize state.
**How to avoid:**
1. D-locked: `useChat` owns message stream; Zustand owns ONLY `{currentSkill, centerView, sidebarCollapsed, preferences, leftWidth, rightWidth}`.
2. ESLint rule: ban `messages` field in the Zustand store type.
**Warning signs:** Stale chat after skill switch; messages disappear after sidebar toggle.

### Pitfall 8: Tailwind v4 + shadcn config mismatch (LOW severity, common friction)
**What goes wrong:** shadcn components generated against Tailwind v3 conventions (e.g., `tailwind.config.js`) don't pick up v4's CSS-first `@theme` tokens; colors render wrong.
**Why it happens:** Tailwind v4 is recent; some shadcn docs still reference v3 setup.
**How to avoid:**
1. Use `npx shadcn@latest init` (latest CLI is v4-aware).
2. Define design tokens in `web/src/styles.css` under `@theme { ... }` — mirror the tokens from `.planning/sketches/themes/default.css` (`--color-bg #0f1419`, `--color-surface #1a1f26`, `--color-primary #4a9eff`, four status colors, topbar/statusbar heights, transition durations).
3. Verify with a smoke test: render `<Button>` and confirm `bg-primary` resolves.
**Warning signs:** Components render with default Tailwind colors instead of the design tokens.

## Code Examples

### Mock fixture (compliant verdict + citation + tier)
```json
[
  { "type": "start", "messageId": "m-001" },
  { "type": "step-start" },

  { "type": "reasoning-start", "id": "r-1" },
  { "type": "reasoning-delta", "id": "r-1",
    "reasoningDelta": "用户询问差旅报销额度，需要检索权威 tier 的差旅制度文档…" },
  { "type": "reasoning-delta", "id": "r-1",
    "reasoningDelta": "检索到《差旅管理办法》第 12 条：经济舱机票每日 800 元上限…" },
  { "type": "reasoning-end", "id": "r-1" },

  { "type": "data-subprogress", "data": { "step": "searching", "tier": "权威",
    "found": 12 } },
  { "type": "data-subprogress", "data": { "step": "reranking" } },

  { "type": "text-start", "id": "t-1" },
  { "type": "text-delta", "id": "t-1",
    "textDelta": "依据《差旅管理办法》第 12 条，您的差旅报销额度如下…" },
  { "type": "text-end", "id": "t-1" },

  { "type": "source-url", "sourceId": "c-001",
    "url": "http://skillforge.local/chunks/authority/travel-policy.md#L12",
    "title": "差旅管理办法 §12" },

  { "type": "data-policy", "id": "p-001", "data": {
    "status": "合规",
    "confidence": 0.93,
    "citations": ["c-001"],
    "review_required": false
  }},

  { "type": "finish", "finishReason": "stop" }
]
```

### URL sanitization module (T-1-08)
```typescript
// Source: web/src/lib/sanitize-url.ts
// Implements T-1-08 — allowlist from CONTEXT.md + 04-UI-SPEC.md.

const ALLOWED_PROTOCOLS = new Set(['https:', 'http:']);
const ALLOWED_HOSTNAMES = new Set([
  'localhost', '127.0.0.1', '::1',
  'spike.local', 'skillforge.local',
]);

export type SanitizeResult =
  | { ok: true; href: string }
  | { ok: false; reason: 'blocked_protocol' | 'blocked_hostname' | 'invalid_url'; hostname?: string };

export function sanitizeSourceUrl(raw: string): SanitizeResult {
  let url: URL;
  try { url = new URL(raw); }
  catch { return { ok: false, reason: 'invalid_url' }; }

  if (!ALLOWED_PROTOCOLS.has(url.protocol)) {
    return { ok: false, reason: 'blocked_protocol', hostname: url.hostname };
  }
  if (!ALLOWED_HOSTNAMES.has(url.hostname)) {
    return { ok: false, reason: 'blocked_hostname', hostname: url.hostname };
  }
  return { ok: true, href: url.toString() };
}

// Usage in component (see Pattern 4 SourceUrlPart):
// const safe = sanitizeSourceUrl(part.url);
// if (!safe.ok) {
//   console.warn(`[T-1-08] blocked citation URL host=${safe.hostname}`);
//   return <span className="citation-blocked">
//     引用 URL 未通过安全白名单，已拦截：{safe.hostname}
//   </span>;
// }
// return <a href={safe.href} target="_blank" rel="noopener noreferrer">…</a>;
```

### Drift-detection test (Wave 1 must-have)
```typescript
// Source: web/src/__tests__/wire-contract.test.ts
import { describe, it, expect } from 'vitest';
import type { UIMessageChunk } from 'ai';
import type { SkillForgeChunk } from '../types/wire-types';

describe('wire contract drift detection', () => {
  it('SkillForgeChunk is assignable to UIMessageChunk (SDK recognizes all our types)', () => {
    // If this fails, the SDK parser will silently drop our chunks.
    const chunk: SkillForgeChunk = { type: 'finish', finishReason: 'stop' };
    const _sdk: UIMessageChunk = chunk; // type-level assertion
    expect(_sdk).toBeDefined();
  });

  it('every fixture ends with type: "finish" (NOT "finish-message")', async () => {
    const fixtures = await import('../__fixtures__/index');
    for (const [name, fixture] of Object.entries(fixtures.FIXTURE_REGISTRY)) {
      const last = fixture[fixture.length - 1];
      expect(last.type, `${name}: must end with "finish", got ${last.type}`).toBe('finish');
    }
  });

  it('data-policy chunks carry Verdict on .data (NOT .policy)', async () => {
    const fixtures = await import('../__fixtures__/index');
    for (const [name, fixture] of Object.entries(fixtures.FIXTURE_REGISTRY)) {
      for (const chunk of fixture) {
        if (chunk.type === 'data-policy') {
          // Pitfall 2 guard
          expect((chunk as any).data, `${name}: data-policy must use .data key`).toBeDefined();
          expect((chunk as any).policy, `${name}: data-policy must NOT use .policy key`).toBeUndefined();
        }
      }
    }
  });
});
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| AI SDK v5 branding | AI SDK v7 (`ai@7.0.59`, `@ai-sdk/react@4.0.62`) installed | Major bumps since the CLAUDE.md was written | CLAUDE.md "Vercel AI SDK v5" references are stale; v7 is the source of truth. Functionality is a superset. |
| `tool-invocation` part type | `tool-${name}` and `dynamic-tool` | v6/v7 | Tool renderers must enumerate names or use `startsWith('tool-')`. |
| `start-message` / `finish-message` lifecycle chunks | `start` / `finish` | v6/v7 | Fixtures and Go emitter must use the short names. |
| Custom data via ad-hoc fields | `data-${name}` chunks with `.data` payload + `dataPartSchemas` registration | v5+ | Verdict / sub-progress must follow the `data-*` convention. |
| Tailwind v3 + JS config | Tailwind v4 + `@theme` CSS-first | Tailwind v4 (2025) | shadcn CLI latest is v4-aware; tokens live in CSS. |
| `<details>` for reasoning | Height-capped preview w/ progressive disclosure (D-08) | This phase | Reasoning models need it — `<details>` is no longer adequate. |
| Vitest 2.x | Vitest 4.x available | v4 release | Optional upgrade; v2 still works. |

**Deprecated/outdated:**
- `tool-invocation` part type — replaced by `tool-${name}` / `dynamic-tool`. [CITED: ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage]
- `start-message` / `finish-message` lifecycle chunks — replaced by `start` / `finish`. [VERIFIED: ai@7.0.59 dist source]
- `needsApproval` property on tools — replaced by `toolApproval` on `streamText`. [CITED: ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The "AI SDK v5" branding in CLAUDE.md refers to the same product family as the installed `ai@7.0.59` / `@ai-sdk/react@4.0.62`. | Standard Stack, SOTA | LOW — verified via npm + ai-sdk.dev which labels current major as v7; v5 is older branding. |
| A2 | shadcn/ui `new-york` style + slate base + React 19 will work with the latest `npx shadcn@latest init` against Tailwind v4. | Standard Stack | LOW — shadcn has official React 19 migration guide; v4-aware CLI is shipping. Planner should add `checkpoint:human-verify` after init. |
| A3 | `dataPartSchemas` accepts Valibot OR Zod schemas (project must pick one). | Pattern 2, Pattern 3 | LOW — verified in index.d.ts that the registration mechanism exists; exact schema-library support should be confirmed at install time. |
| A4 | The Go-side `internal/spike/uimsg.go` `finish-message` ValidType will be renamed to `finish` when Phase 2 graduates the spike. | Runtime State Inventory | MEDIUM — out of P4's control. If P2 ships `finish-message`, prod will appear broken even though P4 fixtures are correct. Mitigation: P4 fixtures are correct; flag in cross-phase sync. |
| A5 | All recommended npm packages pass slopcheck (slopcheck unavailable in this env). | Package Legitimacy Audit | LOW — all packages are mainstream, sourced from well-known GitHub repos (verified via `npm view <pkg> repository`). Planner adds `checkpoint:human-verify` per protocol. |
| A6 | The existing `integration/contract/contract_test.go` (count assertion on ValidTypes) will continue to pass with current spike source unchanged. | Runtime State Inventory | LOW — verified the test imports spike; P4 doesn't touch Go. If P2 changes Go-side, that's P2's test to update. |
| A7 | Tailwind v4 `@theme` correctly accepts the design tokens from `.planning/sketches/themes/default.css` (which were authored against v4 conventions). | Pitfall 8 | LOW — sketch-findings skill was written against Tailwind v4. |

## Open Questions

1. **Valibot vs Zod for `dataPartSchemas`**
   - What we know: AI SDK v7 supports both; zod is more common in the ecosystem.
   - What's unclear: Whether shadcn/ui or any other P4 dep forces one or the other.
   - Recommendation: Pick zod (more tutorials, AI SDK examples use it). Planner adds the choice to Wave 1 setup.

2. **Should P4 upgrade `vite` from ^5.4.0 to ^7.x (CLAUDE.md says "v8.x")?**
   - What we know: Vite v7 is current latest; v5 still works.
   - What's unclear: Whether v5 has any incompatibility with Tailwind v4's `@tailwindcss/vite`.
   - Recommendation: Stay on v5 for P4 (working today); flag upgrade for a separate cleanup task. Minimizes Wave 1 risk.

3. **Skill picker `body` shape for `useChat`**
   - What we know: `useChat({ body: { skillId } })` sends skillId on every request (verified at ai-sdk.dev/docs/ai-sdk-ui/chatbot "Custom body fields per request").
   - What's unclear: Whether real Hertz `/api/chat` expects `skillId` at top level or nested. P2 backend contract decision.
   - Recommendation: P4 fixtures don't depend on this; defer the real-shape decision to the P2/P4 integration meeting.

4. **Should the reasoning preview be per-message or per-reasoning-chunk?**
   - What we know: AI SDK creates a separate `reasoning` part per reasoning chunk (per `reasoning-start`/`reasoning-end` pair).
   - What's unclear: Whether GLM-5.2 emits one big reasoning chain or multiple smaller ones.
   - Recommendation: Per-part is the SDK-aligned answer (Pattern 5 as written). If real model emits multiple, each gets its own preview — visually fine.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Build + dev server | ✓ (project is running) | per `.nvmrc`/lockfile — verify in Wave 0 | — |
| npm | Package install | ✓ | per system | — |
| Vite dev server | `npm run dev` | ✓ | ^5.4.0 installed | — |
| Vitest | `npm run test` | ✓ | ^2.0.0 installed (latest 4.1.10) | Stay on v2; flag upgrade |
| jsdom | DOM tests | ✓ | ^25.0.0 installed | — |
| `internal/spike` Go service | Real `/api/spike/chat` (only for spike demo) | ✓ (Phase 1 spike ran) | spike | NOT needed for P4 — MockTransport is zero-network |
| Real Hertz `/api/chat` | Final integration check (D-04 swap) | ✗ (Phase 2 product, not yet built) | — | **P4 ships without this.** MockTransport covers 100% of UI tests. Real swap = post-P2 integration task. |
| `/api/skills` endpoint | Skill picker data | ✗ (Phase 2 product) | — | Fixture placeholder (Claude's Discretion — locked) |
| Internet (for fonts) | @fontsource self-host | n/a — fonts bundle into the build, no runtime fetch | — | — |
| Python (for slopcheck) | Package Legitimacy Gate | ✗ (no pip in this env) | — | Graceful degradation — packages tagged [ASSUMED], planner adds checkpoint |

**Missing dependencies with no fallback:**
- None for P4 scope. The Mock Transport pattern (D-03) deliberately removes the Hertz dependency from P4's critical path.

**Missing dependencies with fallback:**
- Real `/api/chat` (Phase 2 product) — fallback is MockChatTransport with fixtures; verified pattern via DirectChatTransport precedent.
- Real `/api/skills` — fallback is fixture placeholder (locked in Claude's Discretion).
- slopcheck — fallback is per-package [ASSUMED] tagging + planner `checkpoint:human-verify`.

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest ^2.0.0 (installed; latest 4.1.10 optional upgrade) |
| Config file | `web/vitest.config.ts` (assumed — verify in Wave 0; if missing, create with `environment: 'jsdom'`, `setupFiles: ['./src/__tests__/setup.ts']`) |
| Quick run command | `npm test -- --run src/__tests__/<file>` |
| Full suite command | `npm test` (alias: `vitest run`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FEUI-01 | Streaming typed parts render text/reasoning/tool/source | unit | `npm test -- --run src/__tests__/MessageParts.test.tsx` | ❌ Wave 1 |
| FEUI-01 | Wire contract has no drift (Pitfall 1/2/3) | unit | `npm test -- --run src/__tests__/wire-contract.test.ts` | ❌ Wave 1 |
| FEUI-02 | Reasoning preview default-capped, expandable, doesn't bury answer | unit + visual | `npm test -- --run src/__tests__/ReasoningPart.test.tsx` | ❌ Wave 2 |
| FEUI-03 | Citation click → center panel chunk view | integration | `npm test -- --run src/__tests__/CenterPanel.test.tsx` | ❌ Wave 3 |
| FEUI-03 | Chunk view shows tier badge + version + effective date | unit | `npm test -- --run src/__tests__/ChunkSourceView.test.tsx` | ❌ Wave 3 |
| FEUI-04 | Skill picker lists fixtures, switches skill | unit | `npm test -- --run src/__tests__/SkillPicker.test.tsx` | ❌ Wave 3 |
| FEUI-05 | Refusal out-of-scope vs I-don't-know visually distinct (D-07) | unit | `npm test -- --run src/__tests__/RefusalCard.test.tsx` | ❌ Wave 2 |
| FEUI-06 | Sub-progress steps render in order | unit | `npm test -- --run src/__tests__/DataSubprogressPart.test.tsx` | ❌ Wave 3 |
| SYSK-03 | Skill list loaded from fixture (mock /api/skills) | unit | (covered by SkillPicker.test.tsx) | ❌ Wave 3 |
| KNOW-05 | Citation carries tier metadata → badge renders | integration | (covered by ChunkSourceView.test.tsx) | ❌ Wave 3 |
| CMPL-05 (UI) | `review_required: true` → yellow human-review badge (D-06) | unit | `npm test -- --run src/__tests__/VerdictCard.test.tsx` | ❌ Wave 2 |
| T-1-08 | Source URL sanitization: protocol + hostname allowlist + rel attrs | unit | `npm test -- --run src/__tests__/sanitize-url.test.ts` | ❌ Wave 3 (mandatory) |
| T-1-08 | SourceUrlPart uses sanitizer; rejected URLs render blocked message | integration | (covered by sanitize-url.test.ts + a part-render test) | ❌ Wave 3 |
| D-04 | Mock ↔ Default transport produce identical parts for same fixture | integration | `npm test -- --run src/__tests__/transport-swap.test.ts` | ❌ Wave 3 |
| D-08 | Reasoning always positioned above answer; answer never off-screen | integration + manual visual | `npm test -- --run src/__tests__/ReasoningPart.test.tsx` + manual | ❌ Wave 2 |

### Sampling Rate
- **Per task commit:** `npm test -- --run src/__tests__/<changed-area>` (target < 10s)
- **Per wave merge:** `npm test` (full suite; target < 60s)
- **Phase gate:** Full suite green + manual visual check of all 10 fixtures rendering correctly in the dev server before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `web/vitest.config.ts` — confirm exists with `environment: 'jsdom'`; create if missing
- [ ] `web/src/__tests__/setup.ts` — imports `@testing-library/jest-dom/vitest`, sets up fake timers
- [ ] Framework install: `npm install -D @tailwindcss/vite tailwindcss@^4` plus the runtime deps listed in Standard Stack (Zustand, lucide-react, fontsource, clsx, tailwind-merge)
- [ ] `web/tsconfig.json` strict-mode verification (needed for `wire-types.ts` drift detection to fail-fast)

*(If no gaps: not the case here — Wave 0 has concrete setup work.)*

## Security Domain

> `workflow.ui_safety_gate: true` in config — security section included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | P4 ships no auth UI (deferred). |
| V3 Session Management | no | P4 ships no real sessions (deferred to Phase 5). |
| V4 Access Control | no | Skill picker is mock; no enforced scoping in P4 (mock fixtures only). |
| V5 Input Validation | yes | URL sanitization (T-1-08) — hostname allowlist + protocol gate; fixture validation in `wire-contract.test.ts`. |
| V6 Cryptography | no | No crypto in P4. |
| V12 Files & Resources | partial | ChatInput drag-drop file upload validates 10MB max (from sketch-findings ref); files do not leave the browser in P4 (mock transport). |
| V13 API & Web Service | yes | `useChat` always sends HTTPS in prod (`DefaultChatTransport` to `/api/chat`); dev uses MockTransport (zero-network). |
| V14 Configuration | yes | shadcn/ui registry restricted to official (`ui_safety_gate: true`); no third-party components without re-entering the registry vetting gate. |

### Known Threat Patterns for React streaming chat UI

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via server-controlled `source-url` href (`javascript:` scheme) | Tampering / Spoofing | T-1-08 sanitization module — protocol allowlist + hostname allowlist + `rel="noopener noreferrer"`. Mandatory in Wave 3. [VERIFIED: CONTEXT.md T-1-08] |
| Open-redirect via `source-url` to external host | Spoofing | Same allowlist gate. Rejected URLs render a blocking message, not a clickable link. |
| Information disclosure via reasoning chain | Information Disclosure | Reasoning content comes from agent (not PII); ReasoningPreview is collapsed by default for end-user UX, not security. Real PII redaction is Phase 6. |
| Supply-chain attack via shadcn component | Tampering | `ui_safety_gate: true` — official registry only. NO third-party shadcn registries without re-vetting. |
| Mock transport leaking to production | Spoofing | Transport factory keyed on `import.meta.env.PROD`; transport-swap.test.ts asserts mock is never constructed in prod build. |
| Bundle-size bloat from accidental umbrella import | DoS (browser perf) | Import `ai` and `@ai-sdk/react` from subpaths where possible; tree-shake verification in build. |

## Sources

### Primary (HIGH confidence)
- `web/node_modules/ai@7.0.59/dist/index.d.ts` — `ChatTransport` interface (lines 5270-5336), `UIMessageChunk` union (lines 2275-2415), `DataUIMessageChunk` shape (lines 2267-2273), `ToolUIPart`/`DynamicToolUIPart` (lines 2092-2097). Read in this session.
- `web/node_modules/ai@7.0.59/dist/index.js` — `processUIMessageStream` transform, all case arms including `case "text-start"` (line 6977), `case "reasoning-start"` (line 7028), `case "source-url"` (line 7080), `case "tool-input-start"` (line 7103-7135), `case "step-start"` (line 7350), `case "finish"` (line 7369), default arm with `isDataUIMessageChunk` (lines 7390-7422). Read in this session.
- `internal/spike/uimsg.go` — ValidTypes map (lines 81-98), `Data json.RawMessage` field. Read in this session.
- `internal/spike/verdict.go` — Verdict struct (lines 47-52), ValidStatuses (lines 63-68). Read in this session.
- `web/src/App.tsx` — spike stub; T-1-08 TODO (lines 47-51); `renderPart` switch; SeededView pattern. Read in this session.
- `.planning/phases/04-streaming-trust-ui/04-CONTEXT.md` — D-01..D-08 locked decisions, Claude's Discretion, Deferred Ideas. Read in this session.
- `.planning/phases/04-streaming-trust-ui/04-UI-SPEC.md` — design tokens, wire contract section (lines 308-311 contain drift errors documented in Pitfalls 1-3), status color palette. Read in this session.
- `.planning/sketches/themes/default.css` — design tokens to mirror into Tailwind v4 `@theme`. Read in this session.
- `.claude/skills/sketch-findings-ai-agent/references/layout-three-panel.md` — CSS Grid layout + resize handle pattern + localStorage keys. Read in this session.
- `.claude/skills/sketch-findings-ai-agent/references/chat-input-attach-skill.md` — ChatInput component structure. Read in this session.
- `.claude/skills/spike-findings-ai-agent/SKILL.md` — refusal-by-construction findings, refusal UX semantics. Read in this session.
- `integration/contract/contract_test.go` — imports `spike.UIMessageChunk`; 10s budget assertion. Read in this session.

### Secondary (MEDIUM-HIGH confidence, official docs fetched this session)
- `ai-sdk.dev/docs/ai-sdk-ui/chatbot` — `useChat` API, parts rendering, transport configuration, reasoning/source forwarding, message metadata (`part.type === 'start'` / `'finish'`), DirectChatTransport.
- `ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage` — Tool parts are `tool-${name}` / `dynamic-tool`; `part.state` enum (input-streaming/input-available/approval-requested/approval-responded/output-available/output-error/output-denied); `step-start` parts; `sendAutomaticallyWhen` + `lastAssistantMessageIsCompleteWithToolCalls` helper; `addToolOutput` / `addToolApprovalResponse`.
- `ai-sdk.dev/docs/ai-sdk-ui/transport` — ChatTransport interface, `DefaultChatTransport`, `DirectChatTransport` (zero-network in-process reference pattern that validates our MockTransport design), "Building Custom Transports" guidance.

### Tertiary (LOW confidence — WebFetch attempts on ai-sdk.dev blocked)
- `ai-sdk.dev/docs/ai-sdk-ui/streaming-custom-data` — returned 404 (page moved or removed); not needed because `dataPartSchemas` mechanism is verified from installed `index.d.ts`.
- `ai-sdk.dev/docs/foundations/ui-message-stream` and `/docs/foundations/streams` — fetch blocked by transient network error; not needed because the `processUIMessageStream` source code is the authoritative reference and was read directly.

### Verified package versions (npm registry, fetched 2026-08-11)
- `ai@7.0.59`, `@ai-sdk/react@4.0.62`, `zustand@5.0.14`, `tailwindcss@4.3.3`, `@tailwindcss/vite@4.3.3`, `vitest@4.1.10` (installed 2.x), `lucide-react@1.31.0` (repo: github.com/lucide-icons/lucide), `@fontsource-variable/inter@5.3.0`, `@fontsource-variable/jetbrains-mono@5.3.0`, `@testing-library/react@16.3.2`.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package verified via `npm view` in this session; design tokens read from project file; AI SDK behavior read from installed source.
- Architecture: HIGH — patterns 1-6 cross-verified against ai-sdk.dev official docs (Chatbot, Chatbot Tool Usage, Transport) AND installed TypeScript declarations.
- Pitfalls: HIGH — Pitfalls 1-3 are not speculation; they are line-numbered facts from `ai@7.0.59` source. Pitfalls 4-8 are documented best-practice risks.
- Wire-contract drift (Pitfalls 1-3): HIGH — independently confirmed by (a) reading installed SDK source code AND (b) fetching official v7 docs that use the corrected names.

**Research date:** 2026-08-11
**Valid until:** 2026-09-10 (30 days — AI SDK is fast-moving; re-verify chunk-type names before any major-version bump)

## RESEARCH COMPLETE

**Phase:** 4 — Streaming Trust UI
**Confidence:** HIGH

### Key Findings
- **Three wire-contract drift defects** in the spike-era contract (Pitfalls 1-3) are the single highest-risk items. They are verified against the installed `ai@7.0.59` source AND official v7 docs. Wave 1 must address them before any UI component work.
- **MockChatTransport pattern is officially sanctioned** — the SDK ships `DirectChatTransport` as the in-process reference; our fixture-replay transport is the same shape with zero Go dependencies. D-03 is sound.
- **shadcn + Tailwind v4 + React 19 is a supported combination** with the latest shadcn CLI; `ui_safety_gate: true` enforces official registry only (no third-party components).
- **T-1-08 (source URL sanitization) is mandatory Wave 3** — the spike stub ships with a known XSS vector (`web/src/App.tsx` lines 47-51) that MUST be replaced before any user-facing demo.
- **P4 is fully parallelizable with P2** (D-03..D-05) — MockTransport removes the Hertz dependency from P4's critical path entirely. Real wiring is one transport instance swap (D-04).

### File Created
`D:\code\ClaudeCode\ai-agent\.planning\phases\04-streaming-trust-ui\04-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Every package version verified via `npm view` this session; AI SDK dist source read directly. |
| Architecture | HIGH | Patterns cross-verified against ai-sdk.dev official docs (Chatbot, Chatbot Tool Usage, Transport). |
| Pitfalls | HIGH | Pitfalls 1-3 are line-numbered facts from installed source; 4-8 are well-known risks. |
| Wire-contract drift | HIGH | Triple-verified: installed .d.ts + installed .js + official v7 docs all agree on corrected names. |
| Cross-phase coordination | MEDIUM | P2 backend graduation of spike contracts is out of P4's control; flagged in Runtime State Inventory + Open Questions. |

### Open Questions
1. Valibot vs Zod for `dataPartSchemas` (recommend Zod).
2. Vite v5 → v7 upgrade (recommend defer).
3. Real `/api/chat` request body shape for `skillId` (defer to P2/P4 integration meeting).
4. Per-message vs per-reasoning-chunk preview (SDK-aligned: per-part).

### Ready for Planning
Research complete. The planner now has everything needed to create PLAN.md files. The three wire-contract drift corrections (Pitfalls 1-3) should anchor Wave 1; UI component work follows in Waves 2-3; the T-1-08 sanitization gate is a hard Wave 3 gate.
