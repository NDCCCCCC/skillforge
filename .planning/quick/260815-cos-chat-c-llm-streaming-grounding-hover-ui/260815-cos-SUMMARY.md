---
quick_id: 260815-cos
title: chat C-方案 LLM streaming draft + grounding hover + 置信度隐藏 + HistoricalRefutationCard 仅冲突时显示
type: execute
date: 2026-08-15
branch: worktree-agent-aa824c93a30e64c0e
base_commit: 3e60db920b5ba3131c7438391434bc0bb34c677f
commits:
  - c348294 — feat(260815-cos): BE LLM streaming draft emit + PipelineChatModel seam
  - 46b9fad — feat(260815-cos): FE SkillForgeChunk + DraftStreamChip + transport hook
  - f02476d — feat(260815-cos): FE CitationCluster superscript + hover popover + VerdictCard 置信度隐藏 + CounterEvidenceCard 短路
files_modified: 20
lines_added: 1203
lines_removed: 185
duration: ~30m
tags: [streaming, draft-channel, hover-popover, trust-ui, superscript, confidence-bar, short-circuit]
---

# Quick Plan 260815-cos Summary

## One-liner

BE LLM streaming draft emit (PipelineChatModel.Stream + 2 new wire types) + FE DraftStreamChip status state machine + CitationCluster 角标 hover popover + VerdictCard 置信度条隐藏 + CounterEvidenceCard 仅冲突时渲染 — 3 TDD cycles, 240/240 vitest + all server tests green.

## Tasks Completed

### Task 1 — BE LLM streaming draft emit (`c348294`)

**Files modified (6):**
- `internal/wire/chunk.go` — ValidTypes whitelist +2: `text-delta-draft` + `text-delta-draft-end` (count 20→22). Draft channel ID namespace `draft-1` isolates from `msg-1` text part so useChat cannot pollute `message.parts[].text`.
- `internal/wire/chunk_test.go` — TestValidTypes_Twenty updated: asserts 22 entries + snake_case regression for both new types.
- `internal/server/server.go` — `Deps.PipelineChatModel model.BaseChatModel` (independent field from ChatModel; keeps D5-14 redline cost-control invariant isolated from the draft channel).
- `internal/server/pipeline.go` — Step 8 streaming branch: `PipelineChatModel.Stream(ctx, []*schema.Message{System: prompt}, chatModelOpts...)` → for-loop `Recv()` → emit `text-delta-draft` per token + `text-delta-draft-end` terminator + assemble buffer. The assembled buffer walks the same `json.Unmarshal → verdict.Validate → verdict.GroundingChecker → emitVerdictStream` path — draft channel is purely additive observability. `PipelineChatModel == nil` falls back to Phase 2's `Runner.Query + drainFinalContent` (byte-identical).
- `internal/server/pipeline_stream_test.go` — NEW (428 lines): 3 TDD tests.
- `cmd/server/main.go` — `PipelineChatModel: cm` at startup.

**New test types:**
- `stubStreamingChatModel` implements `model.BaseChatModel` (Stream returns `schema.StreamReader` via `schema.Pipe`).
- `draftFrames(t, body)` extractor for text-delta-draft* frames.

**Test outcomes:**
1. `TestVerdictPipelineStreaming_DraftChunksEmitted` — 3 token Stream → 3 text-delta-draft + 1 text-delta-draft-end + grounded Verdict on data-policy + audit row + 2 session messages. PASS.
2. `TestVerdictPipelineDefault_PipelineChatModelNilFallback` — `PipelineChatModel == nil` → Runner.Query path, NO draft frames, Phase 2 byte-identical behavior. PASS.
3. `TestVerdictPipelineStreaming_RefusalReplacesDraft` — streamed tokens parse to `status="unanswered"` → emits Refusal on data-policy, audit row carries `no_authorized_source`. PASS.

### Task 2 — FE DraftStreamChip + transport hook (`46b9fad`)

**Files modified (8):**
- `web/src/types/wire-types.ts` — `SkillForgeChunk` union +2 variants (text-delta-draft, text-delta-draft-end). D15 drift alarm stays green: required fields `{type, id, delta}` are a subset of AI SDK v5's optional `providerMetadata` arm.
- `web/src/components/chat/parts/DraftStreamChip.tsx` — NEW (51 lines): `{ status: 'pending'|'verified'|'refused'; draftText }` props. Returns `null` on `refused` (RefusalCard replaces). Renders 🟡 待校验 on `pending`, ✅ 已验证 on `verified`.
- `web/src/components/chat/parts/DraftStreamChip.test.tsx` — NEW (61 lines): 4 tests (pending/verified/refused/streaming-append).
- `web/src/transport/MockChatTransport.ts` — per-stream draft buffer + status state machine. `onDraftUpdate` callback fires on every transition. Draft frames are NOT enqueued to useChat (filtered out of `controller.enqueue`) so they cannot pollute `message.parts[].text` — the load-bearing wire-contract invariant.
- `web/src/transport/index.ts` — `createTransport` factory forwards `onDraftUpdate` hook.
- `web/src/store/app-store.ts` — `draftStatus + draftText` slice + `setDraft` writer.
- `web/src/App.tsx` — transport constructed with `onDraftUpdate: ({status, draftText}) => setDraft(status, draftText)`.
- `web/src/components/layout/RightChat.tsx` — renders `<DraftStreamChip status={...} draftText={...} />` below the last message bubble when `draftStatus !== 'idle'`.

### Task 3 — FE CitationCluster 角标 + VerdictCard 置信度隐藏 + CounterEvidenceCard 短路 (`f02476d`)

**Files modified (6):**
- `web/src/components/center/CitationCluster.tsx` — `<button class="citation-chip">` → `<sup data-testid="citation-superscript">[N]</sup>`. Hover opens popover (raw React 19 `onMouseEnter/Leave` + `useState` + 150ms `setTimeout` debounce; NO Radix). Sync `lookupChunk` first (typical path); async `fetchChunkContent` on fixture miss. Popover carries verbatim chunk text + TierBadge + version + effective date. `onClick` retained on `<sup>` for the deeper drill-down via `setCenterView({ kind: 'chunk-source', ... })`.
- `web/src/components/center/VerdictCard.tsx` — removed green confidence bar + 「置信度 N%」 text (locked decision D). Kept `data-testid="confidence-bar"` as Tailwind `hidden` (display:none) with `data-confidence={pct}` attribute so existing assertions / scrapers don't regress.
- `web/src/components/verdict/CounterEvidenceCard.tsx` — top-of-component `if (rows.length === 0) return null` (locked decision E1). Disclaimer + header chrome now ONLY appear when there's actual counter-evidence to annotate.
- `web/src/__tests__/citations.test.tsx` — 5 tests (superscript render / click navigation / hover opens popover / mouseLeave closes / T-1-08 SourceUrlPart blocked URL regression).
- `web/src/__tests__/VerdictCard.test.tsx` — confidence text REMOVED assertion; confidence-bar testid retained + `data-confidence` + `hidden` class assertions.
- `web/src/__tests__/CounterEvidenceCard.test.tsx` — 7 tests (empty/undef return null / non-empty disclaimer verbatim / rows render / click dispatches setCenterView / zero-effective-date case / D5-13 prop isolation moat).

## Locked CONTEXT Decisions Honored

| Decision | Implementation |
|----------|---------------|
| A2 (草稿流式 + 待校验/已验证 chip) | Task 1 BE streaming emit + Task 2 DraftStreamChip status state machine |
| B1 (grounding 失败 → RefusalCard 替换) | MockChatTransport clears draft buffer on `data-policy` carrying `Refusal`; DraftStreamChip returns null on `refused` |
| C3 (角标 + hover 弹层 + 复用 GET /api/knowledge/chunk/:id) | Task 3 `<sup>` superscript + popover with `lookupChunk` sync + `fetchChunkContent` async fallback |
| D (VerdictCard 移除置信度条) | Task 3 removed bar + text; kept hidden DOM node for testid compatibility |
| E1 (CounterEvidenceCard 仅冲突渲染) | Task 3 `if (rows.length === 0) return null` short-circuit |

## Research Recommendations Honored

- New wire types `text-delta-draft` + `text-delta-draft-end` (not reusing text-delta) — prevents AI SDK v5 text part state pollution
- `Deps.PipelineChatModel` as separate field (not reusing ChatModel) — keeps D5-14 redline cost-control invariant isolated
- Raw React 19 hover (no Radix) — minimal surface area in CitationCluster
- `CounterEvidenceCard` top `if (rows.length === 0) return null` — clean short-circuit
- `VerdictCard` retains `data-testid="confidence-bar"` DOM node — testid-compat preservation

## Verification

| Check | Result |
|-------|--------|
| `go test ./internal/server/... -count=1` | ok (2.109s) — all 40+ tests including new 3 streaming tests PASS |
| `go test ./internal/wire/... -count=1` | ok (1.054s) — TestValidTypes_Twenty updated count PASS |
| `go vet ./...` | clean |
| `cd web && npx tsc --noEmit` | exit 0 |
| `cd web && npx vitest run` | 240 passed (baseline 209 + 4 new DraftStreamChip + 12 new/updated across 36 files) |
| `cd web && npx vite build` | exit 0 (850.43 KB / 248.23 KB gzip — node_modules drift; production build clean) |

## Deviations from Plan

None — plan executed as written. Three minor implementation notes:

1. **chatModelOpts extraction** — Initial implementation attempted `adk.GetChatModelOptions(opts)`, which does not exist. Resolved by extracting the underlying `[]model.Option` slice (`model.WithTemperature/WithMaxTokens/openai.WithExtraFields`) as a named local and passing it directly to both branches. Phase 2 fallback wraps the same slice in `adk.WithChatModelOptions` for `Runner.Query`.
2. **CitationCluster hover test timer strategy** — Initially used `vi.useFakeTimers() + vi.advanceTimersByTime(200)`, but `findByTestId`'s waitFor loop doesn't flush with fake timers cleanly. Switched to real timers + `findByTestId` (which has its own retry loop).
3. **VerdictCard confidence-bar test assertion** — Initially used `getComputedStyle().display === 'none'`, but jsdom doesn't resolve Tailwind's `hidden` utility chain without PostCSS at test time. Switched to `toHaveClass('hidden')` + absence-of-text-content assertion.

## Threat Flags

None introduced by this plan. The draft channel is purely additive wire observability; the trust boundary (`emitVerdictStream` on grounded verdict) is preserved unchanged. The FE popover reads the same `fetchChunkContent` already guarded by the existing T-1-08 sanitization gate.

## Known Stubs

None — all wire types are concrete, all components have data-testid surfaces, all tests are deterministic.

## Self-Check

```
Files exist: all 20 modified files exist on disk ✓
Commits exist: c348294, 46b9fad, f02476d all in git log ✓
go vet clean: yes ✓
npx tsc --noEmit: exit 0 ✓
npx vitest run: 240 passed ✓
npx vite build: exit 0 ✓
```

## Self-Check: PASSED