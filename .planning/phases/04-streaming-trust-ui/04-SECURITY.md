---
phase: "04"
slug: streaming-trust-ui
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-11
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
>
> **Retroactive-STRIDE mode.** The PLAN files for Phase 04 do not carry a
> `<threat_model>` block. The threat register below was reconstructed from
> (a) the `## Threat Flags` sections in 04-01/02/03-SUMMARY.md and
> (b) the STRIDE table in 04-RESEARCH.md (~line 975). Every mitigation
> was then verified against the implementation code (not the SUMMARY
> claims) — evidence column cites the file:line where the actual pattern
> lives.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser ↔ MockTransport (dev / test) | `useChat` ↔ `MockChatTransport` is a pure in-process ReadableStream. No network egress. Crossed by `UIMessageChunk` sequences replayed from JSON fixtures under `web/src/__fixtures__/`. | `UIMessageChunk` wire data (text / reasoning / tool I/O / Verdict / Refusal / source-url). Fixture-controlled, but treated as untrusted at the renderer boundary (see T-04-01). |
| Browser ↔ `/api/chat` Hertz SSE (prod) | `useChat` ↔ `DefaultChatTransport({ api: '/api/chat' })` is the prod-only network boundary. Activated by `createTransport('prod')` (transport/index.ts:24-26). | Same `UIMessageChunk` wire data, but sourced from the Phase 2 Hertz server (agent output). Treated as untrusted — every chunk renderer applies the same sanitization that runs in dev. |
| Agent reasoning stream → UI | `reasoning-delta` chunks render verbatim into `ReasoningPart` `<pre>`. Content is agent-generated, not user-PII, but is treated as untrusted text (React escapes; no `dangerouslySetInnerHTML`). | Reasoning prose. Information-disclosure risk is accepted (see Accepted Risks Log — risk R-04-01). |
| Fixture JSON → chunk-lookup | `chunks.json` / `skills.json` are build-time Vite JSON imports. Verbatim text is rendered via `{result.text}` in `<pre>` (React auto-escapes); no `eval`, no `dangerouslySetInnerHTML`. | Chunk verbatim text + tier + version + effective_date. Trusted at dev time (fixture-controlled); same render path is safe for prod-sourced chunks because React escapes. |
| Browser localStorage ↔ ThreePanel | `sf-left-width` / `sf-right-width` panel-width values. `parseInt` + clamp to [180, 600] on read; only numeric ASCII written. No untrusted-string consumption. | Panel width integers only. No attack surface. |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-04-01 | Tampering / Spoofing (XSS via `javascript:` or non-allowlisted hostname on `source-url` chunks) | `SourceUrlPart` href rendering | mitigate | Protocol allowlist (`https:`/`http:`) + hostname allowlist (`localhost`, `127.0.0.1`, `::1`, `spike.local`, `skillforge.local`) enforced in `sanitizeSourceUrl()`; the only `<a href={...}>` in the chat tree (`SourceUrlPart.tsx:37`) is gated through it; rejected URLs render `data-testid="citation-blocked"` span + `console.warn` (NOT a clickable link); accepted URLs carry `target="_blank" rel="noopener noreferrer"`. 7-assertion gate in `sanitize-url.test.ts`; regression guard in `citations.test.tsx`. **Verified**: full-tree grep for `href=` returns exactly one match (the gated one). | closed |
| T-04-02 | Spoofing (Mock transport leaking to production build) | Transport factory selection | mitigate | `createTransport(env)` factory branches on `env === 'prod'` → `DefaultChatTransport({ api: '/api/chat' })`; `App.tsx:43-44` derives `env` from `import.meta.env.PROD` (cast-through-unknown for tsc strict) and memoizes the transport. `MockChatTransport` is only constructable from the `dev`/`test` branch. `transport-swap.test.tsx` (6 assertions) pins the D-04 invariant that Mock and Default surface byte-identical chunk sequences from the same fixture. | closed |
| T-04-03 | Tampering (supply-chain via shadcn registry or third-party component) | UI component sourcing | mitigate | All UI primitives (`button.tsx`, `badge.tsx`, `card.tsx`, `tabs.tsx`) are hand-authored using `class-variance-authority` + `forwardRef`; `package.json` declares no shadcn CLI, no `@radix-ui`, no `@iconify`, no `cmdk`, no `react-resizable-panels`; full-`web/` grep for `shadcn\|@radix-ui\|@iconify\|cmdk\|components.json` returns no source-code matches (only npm registry URLs in `package-lock.json`). | closed |
| T-04-04 | Information Disclosure (agent reasoning chain content) | `ReasoningPart` | accept | See Accepted Risks Log R-04-01. | closed |
| T-04-05 | Tampering (AI SDK discriminator drift / wire integrity) | Wire types | mitigate | Compile-time `type AssertAssignable<A,B>` bidirectional check at `wire-types.ts:79-84` — `_driftCheckTrue` exported constant surfaces the constraint to the compiler; if AI SDK upstream renames a discriminator, this file fails to compile. Runtime-asserted by `wire-contract.test.ts:29-33` (SkillForgeChunk assignable to UIMessageChunk) plus Pitfall 1/2/3 fixture assertions (lines 38-44, 49-65, 71-83). | closed |
| T-04-06 | Integrity / state-boundary (Zustand store vs `useChat` message stream — double source of truth) | `app-store.ts` | mitigate | File-level comment at `app-store.ts:6-12` explicitly BANS a `messages` field (Pitfall 7); the `AppState` interface (lines 30-38) contains only `currentSkillId`, `centerView`, sidebar flags, `preferences`, `leftWidth`, `rightWidth` — no `messages`. `useChat` is the sole owner of the message stream. | closed |
| T-04-07 | DoS / browser perf (bundle-size bloat from umbrella `ai` / `@ai-sdk/react` imports) | Build / imports | mitigate | All type symbols are imported via `import type` (erased at compile); runtime symbols use named imports (`useChat` from `@ai-sdk/react` in App.tsx:12; `DefaultChatTransport` from `ai` in transport/index.ts:14) — Vite/Rollup tree-shakes named imports. `npm run build` emits 405 KB gzipped JS + 27 KB CSS across 1962 modules (per 04-03-SUMMARY Verification). No `import * as` umbrella pattern; no third-party heavy deps introduced. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Unregistered Flags (informational — not blockers)

| Flag | File | Notes |
|------|------|-------|
| `tsc-strict-baseline` | `web/src/components/chat/parts/ToolPart.tsx:26` | Pre-existing TS strict-mode error (`Property 'toolName' does not exist on type ...`) rooted in AI SDK v7 tool-part generic tightening. Documented Out-of-Scope in 04-02-SUMMARY and 04-03-SUMMARY. **Not security-relevant**: the line narrows a tool name for display only; the worst-case behavior is `part.type` falling through as the displayed name. `npm run build` succeeds (esbuild is permissive); `tsc --noEmit` fails on this line but also on pre-existing Wave 1 baseline errors in `wire-contract.test.ts` / `MessageBubble.test.tsx`. Recorded for visibility; does not block ship. |

No other unregistered attack surface was introduced by Phase 04. The
auditor explicitly searched for the following patterns in `web/src/` and
found NO matches (or only the gated, expected match):

| Pattern | Result |
|---------|--------|
| `dangerouslySetInnerHTML` | No matches. |
| `eval(` / `new Function(` | No matches. |
| `postMessage(` | No matches. |
| `<a href={...}>` (raw, ungated) | One match: `SourceUrlPart.tsx:37`, gated through `sanitizeSourceUrl` (T-04-01). No bypass anywhere in `web/src/components/chat/parts/`. |
| `localStorage` misuse | Two files use `localStorage`: `ThreePanel.tsx` (panel widths, `parseInt` + clamp) and its test. Numeric only; no untrusted-string consumption. |
| shadcn / third-party registry imports | None. UI primitives hand-authored. |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Deferral Target | Accepted By | Date |
|---------|------------|-----------|-----------------|-------------|------|
| R-04-01 | T-04-04 | Agent reasoning-chain content is treated as a UX concern, not a security control. The `ReasoningPart` collapsed-by-default height cap (160px) is a usability affordance against GLM-5.2's 1000+ token chains burying the answer (D-08). The reasoning text is agent-generated prose derived from retrieved documents; **real PII redaction is deferred to Phase 6** (per 04-RESEARCH.md STRIDE row "Information disclosure via reasoning chain"). Phase 4 ships no PII-handling logic on the reasoning path; the agent operates on regulation/clause text in the demo Skill (`企业合规/制度审查`), not user-PII corpora. React auto-escapes the rendered `<pre>` content (`reasoningPart.tsx:81-90`) — no HTML injection risk. The accepted risk is the *content visibility* (a reasoning chain could surface retrieved-quotation text the operator would rather hide), not an injection vector. | Phase 6 (PII redaction layer) | gsd-security-auditor | 2026-08-11 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-11 | 7 | 7 | 0 | gsd-security-auditor |

### Verification Methodology

Retroactive-STRIDE audit. The seed register supplied in the task prompt
was treated as **non-authoritative**; each mitigation was re-verified
by reading the actual implementation code at the cited file paths. A
SUMMARY.md claim was not accepted as evidence — only a code match at
the right location was.

| Threat | Evidence Verified (file:line) |
|--------|-------------------------------|
| T-04-01 | `web/src/lib/sanitize-url.ts:22-29` (allowlists); `:39-54` (gate); `web/src/components/chat/parts/SourceUrlPart.tsx:19,37-39` (single gated `<a>`); `web/src/__tests__/sanitize-url.test.ts:18-78` (7 assertions); `web/src/__tests__/citations.test.tsx:79-113` (regression guard). Full-tree grep for `href=` confirms one match. |
| T-04-02 | `web/src/transport/index.ts:24-33` (env-gated factory); `web/src/App.tsx:43-44` (env from `import.meta.env.PROD`); `web/src/__tests__/transport-swap.test.tsx:50-148` (D-04 invariant). |
| T-04-03 | `web/package.json:13-39` (no shadcn / radix / iconify deps); `web/src/components/ui/{button,badge,card,tabs}.tsx` (hand-authored); full-`web/` grep returns no source matches for `shadcn\|@radix-ui\|@iconify\|cmdk\|components.json`. |
| T-04-04 | Accepted Risks Log entry R-04-01 above. |
| T-04-05 | `web/src/types/wire-types.ts:79-84` (`AssertAssignable` + `_driftCheckTrue`); `web/src/__tests__/wire-contract.test.ts:29-33,38-44,49-65,71-83` (runtime + Pitfall 1/2/3 fixture assertions). |
| T-04-06 | `web/src/store/app-store.ts:6-12` (file-level comment banning `messages`); `:30-38` (interface has no `messages` field). |
| T-04-07 | `web/src/App.tsx:12-13` (`import type` for type symbols); `web/src/transport/index.ts:14` (named runtime import for `DefaultChatTransport`); `web/src/components/chat/parts/SourceUrlPart.tsx:15`, `SourceDocumentPart.tsx:11`, `TextPart.tsx:8`, `ToolPart.tsx:16-20`, `DataPolicyPart.tsx:16`, `DataSubprogressPart.tsx:11` — all type-only imports. Build size 405 KB gzipped JS / 27 KB CSS (04-03-SUMMARY). |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer) — 6 mitigate, 1 accept, 0 transfer.
- [x] Accepted risks documented in Accepted Risks Log — R-04-01 recorded with rationale, deferral target, accepted-by, date.
- [x] `threats_open: 0` confirmed.
- [x] `status: verified` set in frontmatter.

**Approval:** verified 2026-08-11 by gsd-security-auditor (retroactive-STRIDE audit, ASVS Level 1).
