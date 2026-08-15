# Quick 260815-j4y: RealChatTransport 包装 DefaultChatTransport 过滤 draft-* Summary

## One-liner

Shipped `RealChatTransport` as the prod-env SSE→UIMessageChunk wrapper: filters `text-delta-draft` / `text-delta-draft-end` from the AI SDK v7 parser surface, routes them through the existing `onDraftUpdate` callback (same shape as `MockChatTransport`), and forwards all other frames verbatim — fixes the `UIMessageStreamError` → `wsasend` abort caused by Phase 2's Hertz backend emitting draft frames the upstream parser does not recognize.

## Files

| Path | Action | Lines |
|------|--------|-------|
| `web/src/transport/RealChatTransport.ts` | created | +214 |
| `web/src/transport/RealChatTransport.test.ts` | created | +216 |
| `web/src/transport/index.ts` | modified | +18 / −8 |

## Commits

| Hash | Type | Description |
|------|------|-------------|
| `af9bb59` | test | RED — `RealChatTransport.test.ts` (4 tests: draft filter, pending→verified, pending→refused, reconnect→null) |
| `be7bd62` | feat | GREEN — `RealChatTransport.ts` + factory wiring (`createTransport('prod')` now wraps SSE filter) |

## Verification Results

| Check | Result |
|-------|--------|
| `cd web && npx tsc --noEmit` | exit 0 |
| `cd web && npx vitest run src/transport/RealChatTransport.test.ts` | 4 / 4 PASS |
| `cd web && npx vitest run src/__tests__/transport-swap.test.tsx` | 6 / 6 PASS (D-04 invariant intact) |
| `cd web && npx vitest run` | 244 / 244 PASS (baseline 240 + 4 new RealChatTransport tests) |

## TDD Discipline

- **RED** (`af9bb59`): test file fails with `Failed to resolve import "./RealChatTransport" from "src/transport/RealChatTransport.test.ts"` — module does not exist yet.
- **GREEN** (`be7bd62`): 4/4 RealChatTransport tests pass; 6/6 transport-swap tests pass; 244/244 full suite green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `this.onDraftUpdate` lost inside `ReadableStream.start()`**
- **Found during:** Task 2 GREEN (first vitest run after impl)
- **Issue:** `chunk.type === 'text-delta-draft'` and `emitDraft(...)` references to `this.onDraftUpdate` failed because `ReadableStream.start()` binds `this` to its underlying source object, NOT the `RealChatTransport` instance.
- **Fix:** Captured `this.onDraftUpdate` (and `api`, `headers`) into local consts at the top of `sendMessages`; the inner `start()` and arrow `emitDraft` reference the closures instead.
- **Files modified:** `web/src/transport/RealChatTransport.ts`
- **Commit:** `be7bd62`

**2. [Rule 1 - Bug] UIMessageChunk type-narrowing rejected SkillForge extensions**
- **Found during:** Task 2 GREEN (TS check after first impl)
- **Issue:** `UIMessageChunk` (upstream AI SDK v7) does NOT include `text-delta-draft` or `text-delta-draft-end` discriminators — comparing `chunk.type === 'text-delta-draft'` triggered `TS2367: This comparison appears to be unintentional because the types have no overlap`. The wire shape is a SUPERSET of `UIMessageChunk`.
- **Fix:** Cast `chunk as unknown as { type: string; delta?: string }` for the discriminator check; structural shape (data-policy .status / .reason) already cast.
- **Files modified:** `web/src/transport/RealChatTransport.ts`
- **Commit:** `be7bd62`

**3. [Rule 1 - Bug] `reconnectToStream` parameter signature missing**
- **Found during:** Task 2 GREEN (TS check after first impl)
- **Issue:** The plan's interface pin requires `reconnectToStream(options: {chatId, abortSignal?, ...ChatRequestOptions})`, but the initial impl declared `async reconnectToStream()` (0-arg). Test call site `transport.reconnectToStream({chatId:'chat-1'})` triggered `TS2554: Expected 0 arguments, but got 1`.
- **Fix:** Declared the options parameter with `& Record<string, unknown>` to satisfy `ChatRequestOptions` from `ai`; underscore-prefixed because unused (parity with MockChatTransport's `_options`).
- **Files modified:** `web/src/transport/RealChatTransport.ts`
- **Commit:** `be7bd62`

**4. [Rule 1 - Bug] Test fixture `finish` chunk shape mismatch with AI SDK v7**
- **Found during:** Task 2 GREEN (TS check on test file)
- **Issue:** Test fixture used `{ type:'finish', messageId:'msg-1', isContinued:false, finishReason:'stop' }` — but AI SDK v7's `finish` chunk only carries `{ type, finishReason?, messageMetadata? }`. `messageId` / `isContinued` are v4-style fields that triggered `TS2353`.
- **Fix:** Removed `messageId` and `isContinued` from `finish` chunks in all 3 fixture bodies; kept `{ type:'finish', finishReason:'stop' }`.
- **Files modified:** `web/src/transport/RealChatTransport.test.ts`
- **Commit:** `af9bb59` (amended)

**5. [Rule 1 - Bug] Test chunk literals not assignable to `UIMessageChunk`**
- **Found during:** Task 2 GREEN (TS check on test file)
- **Issue:** Test fixtures constructed `text-delta-draft` / `text-delta-draft-end` literals with `type` strings outside the `UIMessageChunk` union. Passed through `sseFrame(chunk: UIMessageChunk)` triggered `TS2322`.
- **Fix:** Loosened `sseFrame` to accept a structural `WireChunk = { type: string } & Record<string, unknown>`; the SSE serializer works on JSON shape, the parser in `RealChatTransport` re-narrows via the runtime discriminator.
- **Files modified:** `web/src/transport/RealChatTransport.test.ts`
- **Commit:** `af9bb59` (amended)

## Auth Gates

None — no auth flows touched.

## Known Stubs

None. The `RealChatTransport` SSE parser is fully implemented (line-buffered frame split, multi-line data join, malformed-frame skip, draft buffer + status state machine); `reconnectToStream` returns `null` per MockChatTransport parity (intentional, not a stub — v1 has no persistent server-side stream handle).

## Threat Flags

None. `RealChatTransport.fetch(opts.api)` uses the same endpoint (`/api/chat`) that `DefaultChatTransport` already exposed — no new network surface, no new auth headers, no new file access. The factory injection point remains the only path to the backend.

## Status

- All tasks complete
- All success criteria met:
  - [x] RealChatTransport implements `ChatTransport<UIMessage>` (verified by 4 unit tests + `tsc --noEmit`)
  - [x] createTransport('prod') returns RealChatTransport (verified by factory edit + TS contract preservation)
  - [x] env='dev'|'test' still returns MockChatTransport (transport-swap D-04 still passes)
  - [x] AI SDK v7 parser will no longer see draft-* chunks (filter at SSE→UIMessageChunk layer; `useChat` abort path closed)
  - [x] 244/244 frontend tests pass (zero regression; baseline 240 + 4 new)
  - [ ] Manual end-to-end verification of `VITE_REAL_BACKEND=1` dev server (user restart required — outside agent scope per `autonomous:true` + no checkpoint gate)

## Manual Verification (user step)

Once the user restarts the dev server with `VITE_REAL_BACKEND=1`, the chat endpoint's SSE stream will emit `text-delta-draft` frames during grounding parse. The `RealChatTransport` wrapper now intercepts them at the SSE layer:

1. `DraftStreamChip` transitions `🟡 待校验` → `✅ 已验证` (or unmounts on refusal)
2. Browser console no longer shows `UIMessageStreamError` aborts
3. The verdict message renders cleanly through the AI SDK v7 parser

## Self-Check: PASSED

- `web/src/transport/RealChatTransport.ts` exists
- `web/src/transport/RealChatTransport.test.ts` exists
- `.planning/quick/260815-j4y-transport-text-delta-draft-ai-sdk-parser/260815-j4y-SUMMARY.md` exists
- Commit `af9bb59` (RED test) present in `git log`
- Commit `be7bd62` (GREEN impl + factory) present in `git log`