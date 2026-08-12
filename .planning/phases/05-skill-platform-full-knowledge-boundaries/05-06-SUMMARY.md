---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 06
subsystem: skill-platform
tags: [counter-evidence, redline, sc-6, sc-7, cmpl-06, cmpl-07, d5-13, d5-14, trust-moat, cost-control]
requires: [05-01, 05-02, 05-05]
provides: [counter-evidence-annotation-pass, redline-rewrite-pass, counter-evidence-card-ui, redline-card-ui]
---

# Plan 05-06 — SC-6 Counter-Evidence (CMPL-06) + SC-7 Redline Rewrite (CMPL-07)

## Objective

Ship the two post-verdict annotation passes that turn a single-shot
compliance verdict into a multi-angle audit surface:

- **SC-6 / CMPL-06 / D5-13** — after a verdict is emitted, run an
  INDEPENDENT retrieval pass against the 历史 tier and surface the
  contradicting chunks as ANNOTATION. The trust moat (D5-13):
  counter-evidence chunks NEVER enter the verdict's `chunksByID`
  grounding map. This is enforced structurally —
  `emitCounterEvidence(ctx, c, qvec, now)` does not receive
  `chunksByID` as a parameter. The history-tier chunks are visible to
  the operator (for review) but cannot influence the verdict that was
  already emitted.

- **SC-7 / CMPL-07 / D5-14** — after a 不合规 / 需补充 verdict, call the
  chat model with a structured-output prompt that produces a clause-
  level rewrite proposal (original / proposed_rewrite / rationale).
  Emit it as ONE `data-redline` chunk (the 18th wire type added in
  Plan 05-01). The cost-control gate (D5-14) is the FIRST line of
  `emitRedline`: a 合规 verdict returns nil immediately — ZERO LLM
  calls, ZERO chunks emitted. Only 不合规 / 需补充 fire the pass.

Both passes are **best-effort**: errors are logged via `log.Printf`
and the pipeline returns nil so the verdict + any partial annotation
still ships. The operator sees what arrived; a failed annotation
pass never invalidates an already-grounded verdict.

## What shipped

### Backend (Go)

| File | Provides |
|------|----------|
| `internal/verdict/grounding.go` | `NonCompliantStatus = "不合规"` constant (typed redline gate — companion to the existing `DowngradeStatus = "需补充"`). |
| `internal/wire/chunk.go` | `ConflictChunk` struct (ChunkID / Tier / Source / Snippet / EffectiveFrom / EffectiveTo) + `ConflictChunks []ConflictChunk` field on `DecisionPayload`. The counter-evidence pass reuses `data-decision` with `decision='counter_evidence'` (NO new wire type — D5-04). |
| `internal/server/annotate.go` | `func (s *Deps) emitCounterEvidence(ctx, c, qvec, now) error` — calls `s.Retriever.Search(ctx, qvec, ["历史"], 5, now)`, builds `[]wire.ConflictChunk` via the `snippetOf(content, 200)` helper (text-only — never HTML), emits ONE data-decision chunk with `decision='counter_evidence'`. **Signature excludes `chunksByID` — this IS the D5-13 moat.** |
| `internal/server/redline.go` | `func (s *Deps) emitRedline(ctx, c, v, chunksByID) error` — **FIRST LINE is the D5-14 gate**: `if v.Status != NonCompliantStatus && v.Status != DowngradeStatus { return nil }`. Otherwise calls `s.ChatModel.Generate(ctx, msgs)`, parses with `wire.UnmarshalRedline`, emits ONE data-redline chunk. Includes `redlineSystemPrompt` + `buildRedlineUserPrompt(v, chunksByID)` helper. |
| `internal/server/pipeline.go` | Step 11 added after `emitVerdictStream` (step 10): `s.emitCounterEvidence(ctx, c, qvec, now)` then `s.emitRedline(ctx, c, v, chunksByID)`. Both wrapped in `if err != nil { log.Printf("[WARN] ...") }`. Order matters: counter-evidence renders BEFORE redline in the bubble (UI-SPEC §SC-6/7). |
| `internal/server/server.go` | `ChatModel model.ToolCallingChatModel` field added to Deps (the redline LLM seam). |
| `cmd/server/main.go` | Wires `ChatModel: cm` into Deps at startup. |

### Frontend (TypeScript / React 19)

| File | Provides |
|------|----------|
| `web/src/types/wire-types.ts` | Mirrored `ConflictChunk` interface + `conflictChunks?: ConflictChunk[]` on `DecisionPayload`. D15 drift alarm stays green (tsc exits 0). |
| `web/src/styles/theme.css` | 4 new tokens: `--color-annotation` (#2dd4bf teal-400), `--color-annotation-dim` (#134e4a teal-900), `--color-redline-removed` (rgba red 10%), `--color-redline-added` (rgba green 10%). Reserved SCOPED palette — the ONLY teal surface in the app (the 05-05 GraphTree deliberately does NOT use teal so this line stays bright). |
| `web/src/lib/clipboard.ts` | `copyToClipboard(text): Promise<boolean>` (navigator.clipboard.writeText wrapped — never throws) + `downloadMarkdown(filename, body)` (Blob + `<a download>` click pattern). No sonner dep — uses the wizard-success-toast inline pattern. |
| `web/src/components/verdict/CounterEvidenceCard.tsx` | Visible trust-moat marker. Renders AFTER VerdictCard. DISCLAIMER constant is a hardcoded JSX literal (T-5-27 — `以下内容来自「历史」知识层，仅供人工参考，不参与本次结论的生成与校验。`). Empty state copy `未检索到与本次结论冲突的历史片段。`. Conflict rows are buttons with the 历史 tier badge in teal; click dispatches `setCenterView({kind:'chunk-source', chunkId, tier, version:null, effectiveDate, effectiveTo})`. **Props: `{ conflictChunks?: ConflictChunk[] }` — receives ONLY conflictChunks, never verdict.citations (D5-13 FE moat).** |
| `web/src/components/verdict/RedlineCard.tsx` | Diff view: 原条款 (`bg-redline-removed line-through text-status-non-compliant font-mono`) + 建议改写 (`bg-redline-added text-status-compliant font-mono`) + 修改理由 (2px teal left-border, `border-l-annotation`). Copy CTA fires `copyToClipboard(proposedRewrite)` + shows `已复制到剪贴板。` toast. Download CTA builds a markdown Blob (filename from `clauseId` or fallback `redline.md`) and triggers an anchor click. **ABSENT on 合规 verdicts — the BE emits no data-redline chunk for them (D5-14).** |
| `web/src/components/chat/MessageBubble.tsx` | `renderPart` switch extended: `data-decision` with `decision='counter_evidence'` → `<CounterEvidenceCard conflictChunks={dp.conflictChunks} />` (REPLACES the prior RoutingTracePart route for this value — RoutingTracePart now only handles `decision='route'` from the dispatcher); `data-redline` → `<RedlineCard data={data} />`. Other decision values still drop silently (forward-compat with Phase 3 inspector data). |

### Tests

| File | Coverage |
|------|----------|
| `internal/server/annotate_test.go` | **5 tests** including the load-bearing `Test_emitCounterEvidence_NotGrounded` (D5-13 regression — asserts the counter-evidence pass cannot reach `chunksByID`). Also: emitted-as-annotation (decision='counter_evidence'), empty conflict chunks, retriever error is non-fatal (best-effort), 历史 tier only (never fetches 权威). |
| `internal/server/redline_test.go` | **5 tests** including the load-bearing `Test_emitRedline_Conditional` (D5-14 regression — 合规 → 0 LLM calls, 不合规 → 1 call, 需补充 → 1 call). Also: payload structure, LLM error is non-fatal (best-effort), LLM returns invalid JSON (wire V8 — UnmarshalRedline rejects). Uses a `stubChatModel` satisfying `model.ToolCallingChatModel` (Generate / Stream / WithTools) with a compile-time `var _ model.ToolCallingChatModel = (*stubChatModel)(nil` assertion. |
| `web/src/__tests__/CounterEvidenceCard.test.tsx` | **9 tests**: disclaimer verbatim (T-5-27 load-bearing — strict equality on the hardcoded literal), empty state copy, row with teal tier badge, click dispatches setCenterView, null effectiveDate path (effectiveFrom=0), D5-13 prop isolation (data-conflict-count mirrors ONLY conflictChunks length). |
| `web/src/__tests__/RedlineCard.test.tsx` | **9 tests**: original-clause line-through + red-tinted bg, proposed-rewrite green-tinted bg + status-compliant, rationale teal left-border, copy CTA fires navigator.clipboard.writeText (mocked), copy error toast on rejection, download CTA fires URL.createObjectURL + anchor click (mocked), download body includes all three fields, download filename derives from clauseId, download filename falls back to `redline.md`. |

## Verification

```bash
$ go test ./internal/server/... ./internal/verdict/... ./internal/wire/... -count=1
ok      github.com/skillforge/skillforge/internal/server       0.738s
ok      github.com/skillforge/skillforge/internal/verdict      0.045s
ok      github.com/skillforge/skillforge/internal/wire         0.087s

$ cd web && npx tsc --noEmit
# (no output — exit 0)

$ cd web && npx vitest run --reporter=basic
Test Files  29 passed (29)
     Tests  190 passed (190)
```

All automated checks green. SPEC §verification checklist:

- [x] `go test ./internal/server/... -count=1` exits 0 (10 new tests + all existing server tests)
- [x] `cd web && npx vitest run` exits 0 (18 new FE tests + all 172 existing tests)
- [x] `cd web && npx tsc --noEmit` exits 0 (D15 drift alarm intact)
- [x] counter-evidence regression test asserts the pass does NOT touch `chunksByID` (D5-13)
- [x] redline conditional test asserts 合规 verdict makes 0 LLM calls (D5-14)
- [x] counter-evidence disclaimer test asserts the exact literal verbatim (T-5-27)
- [x] no `dangerouslySetInnerHTML` in CounterEvidenceCard / RedlineCard (T-5-28)
- [x] D-04 / D12 transport-swap invariant unchanged (FE-only changes; the swap suite still green)

## Acceptance Criteria — Results

| Criterion | Result |
|-----------|--------|
| Counter-evidence retrieval pass runs against 历史 tier only (test) | PASS — `Test_emitCounterEvidence_PassesHistoryTierOnly` asserts the retriever was called with `[]string{"历史"}` |
| Counter-evidence chunks NEVER enter verdict's chunksByID (D5-13 regression test) | PASS — `Test_emitCounterEvidence_NotGrounded` snapshot-asserts the grounding map before + after the pass; identical. The function signature structurally forbids access (no `chunksByID` parameter) |
| Counter-evidence emits ONE data-decision chunk with decision='counter_evidence' carrying conflictChunks | PASS — `Test_emitCounterEvidence_EmittedAsAnnotation` parses the SSE frame and asserts the wire shape |
| Counter-evidence retriever error is non-fatal (verdict still stands) | PASS — `Test_emitCounterEvidence_RetrieverErrorIsNonFatal` returns an error from the stub and asserts emitVerdictStream still completed (the verdict chunk was already emitted in step 10) |
| Redline pass fires ONLY on 不合规 / 需补充 (D5-14 regression test) | PASS — `Test_emitRedline_Conditional`: 合规 → `stub.Generate` call count == 0; 不合规 → 1; 需补充 → 1 |
| 合规 verdict emits NO data-redline chunk and makes NO LLM call (cost control) | PASS — same test asserts ZERO frames on the 合规 path |
| Redline emits ONE data-redline chunk with original/proposed_rewrite/rationale | PASS — `Test_emitRedline_PayloadStructure` parses the wire frame |
| Redline LLM error / invalid JSON is non-fatal (best-effort) | PASS — `Test_emitRedline_LLMErrorIsNonFatal` + `Test_emitRedline_LLMReturnsInvalidJSON` assert the pipeline returns nil and the verdict stands |
| CounterEvidenceCard renders the disclaimer sentence verbatim on every render (T-5-27) | PASS — `CounterEvidenceCard.test.tsx` strict-equals the literal `以下内容来自「历史」知识层，仅供人工参考，不参与本次结论的生成与校验。` (both with and without conflictChunks) |
| CounterEvidenceCard click row dispatches setCenterView({kind:'chunk-source'}) | PASS — store-spy test asserts the dispatched view's kind / chunkId / tier / version / effectiveDate / effectiveTo |
| RedlineCard renders 原条款 strikethrough red-tinted + 建议改写 green-tinted + 修改理由 teal left-border | PASS — class-based assertions on the three test ids (`redline-original`, `redline-rewrite`, `redline-rationale`) |
| RedlineCard copy CTA fires clipboard.writeText (mocked) | PASS — `navigator.clipboard.writeText` spy asserted with `proposedRewrite`; success toast appears |
| RedlineCard download CTA triggers Blob + anchor click (mocked) | PASS — `URL.createObjectURL` + `HTMLAnchorElement.prototype.click` spies asserted; filename derives from clauseId (with `redline.md` fallback) |
| data-redline chunk reuses the 18th wire type from Plan 05-01 (NOT redefined) | PASS — no new wire type added; the `data-redline` variant in `SkillForgeChunk` already exists. Only the FE renderer was wired in this plan |
| D15 wire drift alarm: TS mirror mirrors Go shape | PASS — `tsc --noEmit` exits 0; `ConflictChunk` interface mirrored in wire-types.ts |

## Threat-Model Disposition

| Threat | Mitigation | Test |
|--------|------------|------|
| T-5-25 Counter-evidence chunks leak into verdict.citations and re-ground the verdict (the trust moat fails) | D5-13 — structural: `emitCounterEvidence(ctx, c, qvec, now)` does NOT receive `chunksByID`. The pass builds its own ConflictChunk slice and emits via a separate wire path. FE moat: `CounterEvidenceCard` props accept ONLY `conflictChunks` (no `citations`). | BE: `Test_emitCounterEvidence_NotGrounded` snapshots the map. FE: CounterEvidenceCardProps shape assertion. |
| T-5-26 Redline pass fires on 合规 verdicts and burns an LLM call for every successful review (cost control regression) | D5-14 — the FIRST line of `emitRedline` short-circuits on any status that is not 不合规 / 需补充. The 合规 path returns nil BEFORE the ChatModel is touched. | `Test_emitRedline_Conditional` asserts the stub's Generate call count for each status. |
| T-5-27 Operator-customizable disclaimer lets a future Skill weaken the trust-moat framing | The DISCLAIMER constant is a hardcoded JSX literal in CounterEvidenceCard. No prop, no Skill manifest field, no BE-driven override. The test strict-equals the exact bytes. | `CounterEvidenceCard.test.tsx` disclaimer-verbatim test (load-bearing). |
| T-5-28 Redline diff text injects HTML (the rewrite rationale contains `<script>...</script>`) | No `dangerouslySetInnerHTML` anywhere in CounterEvidenceCard / RedlineCard / clipboard.ts. All text rendered via React's default text-node rendering. The diff body is built as a template literal string and handed to a Blob — the Blob's `text/markdown` MIME is consumed by the operator's markdown viewer, not the browser DOM. | Grep asserts zero `dangerouslySetInnerHTML` mentions outside comments. |
| V8 Redline wire payload truncated to a single field (e.g. rationale omitted) still renders a blank card | `wire.RedlinePayload.UnmarshalJSON` uses pointer-fields to distinguish absent from empty and rejects missing required fields. The TS mirror types originalClause / proposedRewrite / rationale as required (no `?:`) so a truncated payload fails the `(p: RedlinePayload) => void` consumer signature at the dispatcher. | BE: `Test_emitRedline_LLMReturnsInvalidJSON` exercises the parse-reject path. FE: tsc enforces the required fields at the cast site in MessageBubble. |
| (Best-effort regression) A failed annotation pass invalidates the already-emitted verdict | Step 11 in `pipeline.go` logs the error with `[WARN]` and the pipeline returns nil. The verdict chunk was emitted in step 10 BEFORE the annotation passes run; SSE is append-only so the verdict is already in the client's buffer. | BE: `Test_emitCounterEvidence_RetrieverErrorIsNonFatal` + `Test_emitRedline_LLMErrorIsNonFatal`. |

## Atomic Commit Trail

```
3ecb8ab feat(05-06): BE post-verdict passes — SC-6 counter-evidence + SC-7 redline
792636a feat(05-06): FE annotation cards — SC-6 CounterEvidence + SC-7 Redline
```

## Notes for Future Work

- **`emitCounterEvidence` uses the same qvec as the main retrieval pass** — this is deliberate. The query embedding is the user's verbatim question; re-embedding would only add latency without changing the semantic match. The 历史 tier filter is what isolates the search space. If a future Plan introduces query-rewriting for the counter-evidence angle (e.g. "find docs that CONTRADICT the verdict" rather than "find docs that match the query"), the hook is the `qvec` parameter — swap in a re-derived vector at the call site.
- **The redline pass uses `ChatModel.Generate` directly** (not the agent runner) — the rewrite is a single-shot structured-output call, not an agentic loop. This is the cost-control minimum: one LLM call per non-compliant verdict. If a future Plan wants multi-turn refinement (e.g. critique → revise → emit), the integration point is `emitRedline` — wrap Generate in a loop and emit multiple `data-redline` chunks (the FE MessageBubble dispatcher already keys by array index, so multiple redline cards would render in order).
- **The `stubChatModel` in redline_test.go is reusable** for any future test that needs a `model.ToolCallingChatModel` stand-in. It exposes a `calls` counter and a `response` string, plus no-op Stream / WithTools stubs to satisfy the interface. Compile-time assertion `var _ model.ToolCallingChatModel = (*stubChatModel)(nil)` catches interface drift.
- **CounterEvidenceCard's `data-conflict-count` attribute** is the FE-visible mirror of `conflictChunks.length`. It's used by the D5-13 prop-isolation test to assert the row count derives ONLY from conflictChunks (not verdict.citations). If a future refactor feeds verdict.citations into the card, the count would diverge from `conflictChunks.length` and the test would fail red.
- **RedlineCard copy/download CTAs are scoped to `proposedRewrite` only** — the operator copies the SUGGESTED text, not the diff. The diff is downloadable as a markdown file for offline review. This matches the UI-SPEC §SC-7 contract: the rewrite is a proposal; the operator decides whether to apply it (v1 does NOT write back to the authority doc).
- **No sonner dependency** — the inline toast pattern (`{toast && <div>{toast}</div>}`) matches the wizard-success-toast shape already in App.tsx. Adding sonner just for two CTAs would be a heavy new dep; the existing pattern covers the UX need with zero bundle cost.
- **RoutingTracePart still has the `decision === 'counter_evidence'` branch** in its `isRouted` guard. This is now dead code from the MessageBubble dispatcher's perspective (counter_evidence routes to CounterEvidenceCard), but the component is still callable standalone and the existing `RoutingTracePart.test.tsx` "routes counter_evidence" case still exercises that branch. Leaving the guard in place is the conservative choice — removing it would be scope creep beyond Plan 05-06.
