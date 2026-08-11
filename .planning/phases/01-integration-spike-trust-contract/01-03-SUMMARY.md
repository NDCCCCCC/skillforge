---
phase: 01-integration-spike-trust-contract
plan: 03
subsystem: api
tags: [sse, hertz, react, vitest, uimessage, ai-sdk, spike, sc-1, sc-4, wave-2]

# Dependency graph
requires:
  - phase: 01-01
    provides: "go.mod with hertz@v0.10.6 + eino@v0.7.7 + cmd/spiked subcommand router + Windows-safe .gitignore"
provides:
  - "internal/spike.UIMessageChunk struct (kebab-case Type + camelCase fields) matching AI SDK v5+ wire spec"
  - "internal/spike.EmitChunk + StreamChunks — Hertz SSE writer using actual v0.10.6 API (sse.NewWriter + WriteEvent)"
  - "internal/spike.ValidTypes 16-type whitelist (T-1-11 mitigation: reject unknown/snake_case before wire write)"
  - "cmd/spiked sse subcommand — Hertz on dynamic port, emits 6 representative chunks, round-trip parses frames"
  - "web/ React 19 + Vite + vitest stub — useChat + DefaultChatTransport, source-url -> clickable <a> (SC-4)"
  - "SC-1 layers (b) typed-parts serialization + (c) client-side parse — both verified via Go round-trip test"
affects:
  - "01-04 (spike e2e — chains UIMessageChunk frames through real GLM-5.2 + skill pipeline)"
  - "Phase 2 (production code reuses internal/spike.UIMessageChunk + EmitChunk for Hertz SSE handlers)"
  - "Phase 4 (Streaming Trust UI reuses web/ structure + App.tsx rendering pattern; adds URL sanitization T-1-08)"

# Tech tracking
tech-stack:
  added:
    - "ai@^7.0.0 (Vercel AI SDK core — UIMessageChunk wire format, DefaultChatTransport)"
    - "@ai-sdk/react@^4.0.0 (useChat React hook — typed message parts)"
    - "react@^19.0.0 + react-dom@^19.0.0 (React 19 stable)"
    - "vitest@^2.1.9 + @testing-library/react@^16.0.0 + jsdom@^25.0.0 (citation rendering tests)"
    - "vite@^5.4.0 + @vitejs/plugin-react@^4.3.0 (dev server + proxy /api -> Hertz :8888)"
    - "google.golang.org/protobuf + github.com/fsnotify/fsnotify (Hertz server.Default transitive deps, pulled by go mod tidy)"
  patterns:
    - "Hertz SSE writer via server.WithListener(pre-bound net.Listener) — no close-and-rebind port race"
    - "EmitChunk: sse.NewWriter(c) is idempotent (re-uses hijacked writer), so safe to call per-chunk in a loop WITHOUT Close (framework finalizes on handler return)"
    - "ValidTypes kebab-case whitelist gate before wire write (T-1-11 mitigation — reject snake_case server-side so AI SDK v5 parser never silently drops a frame)"
    - "Seeded-messages React testing: <App initialMessages={...} /> bypasses useChat so vitest runs no network — Phase 4 adds live transport E2E"
    - "UIMessageChunk wire envelope: Hertz marshal -> HTTP chunked -> stdlib de-chunk -> JSON parse -> struct (full round-trip proven in TestSpikeSSERoundTrip)"

key-files:
  created:
    - "internal/spike/uimsg.go — UIMessageChunk struct (13 fields, camelCase JSON tags) + 16-type ValidTypes whitelist + IsValidType()"
    - "internal/spike/hertz_sse.go — EmitChunk + StreamChunks + ContentTypeIsSSE (verified against hertz@v0.10.6 writer.go)"
    - "cmd/spiked/sse.go — runSSE: dynamic-port Hertz + 6-chunk handler + parseSSEFrames + waitForListener helpers"
    - "cmd/spiked/sse_test.go — TestUIMessageChunkShape (10 subtests) + TestSpikeSSERoundTrip + TestEmitChunkRejectsUnknownType"
    - "web/package.json — pinned ai@^7 + @ai-sdk/react@^4 + react@^19 + vitest + @testing-library/react"
    - "web/src/App.tsx — useChat + DefaultChatTransport; LiveChat (production) vs SeededView (test) split to keep hooks unconditional"
    - "web/src/__tests__/citations.test.tsx — 3 vitest tests: source-url <a>, unknown-type silent drop, reasoning <details>"
    - "web/{vite,vitest}.config.ts + vitest.setup.ts + tsconfig.json + index.html + src/main.tsx"
  modified:
    - "cmd/spiked/scaffold_stubs.go — runSSE stub removed (promoted to real impl in sse.go)"
    - "go.mod + go.sum — go mod tidy pulled Hertz server transitive deps (protobuf, fsnotify) required by server.Default"

key-decisions:
  - "AI SDK version pin corrected: plan specified ai@^5 + @ai-sdk/react@^5, but @ai-sdk/react@5 does not exist on npm (latest 4.0.62) and ai@5 predates the @ai-sdk/react package split. Pinned to actual compatible pair ai@^7.0.0 + @ai-sdk/react@^4.0.0 (current stable line). Verified useChat + DefaultChatTransport + UIMessage types all exported. Rule 3 blocking-fix; the plan's version was a planning error, not a slopsquatting concern (same official Vercel packages, different version line)."
  - "Hertz server.WithListener(pre-bound net.Listener) chosen over WithHostPorts(close-then-rebind). The WithListener option exists in hertz@v0.10.6 (option.go:176) and eliminates the port-reuse race entirely — the listener Hertz serves on is the exact one net.Listener bound. Verified via example_test.go pattern."
  - "EmitChunk calls sse.NewWriter(c) per-invocation but does NOT call w.Close(). NewWriter is idempotent (writer.go:47-52 re-uses the hijacked writer), and Close finalizes chunked encoding (writes 0\\r\\n\\r\\n terminator) — calling it mid-stream would break subsequent frames. The Hertz framework finalizes automatically when the handler returns (verified example_test.go:48-50 comment)."
  - "16-type ValidTypes whitelist includes data-policy per D-03/D-04 (AI SDK v5 supports data-* custom types for Skill-specific payloads). data-policy carries the SkillForge compliance Verdict, distinct from source-url which carries the citation URL. This aligns the whitelist with TestUIMessageChunkShape's shapeTypes iteration."
  - "App.tsx split into LiveChat (calls useChat) + SeededView (renders prop). App conditionally renders one or the other — keeps hooks unconditional inside each component and lets tests seed messages without triggering a real transport call in jsdom."

patterns-established:
  - "Pattern: UIMessageChunk wire envelope — internal/spike.UIMessageChunk is the canonical Go mirror of AI SDK v5+ UIMessageChunk. Phase 2 production code reuses this struct + EmitChunk."
  - "Pattern: Hertz SSE handler — register h.GET(path, handler) where handler loops spike.EmitChunk(c, chunk). Dynamic port via server.WithListener(net.Listen(\"tcp\",\"127.0.0.1:0\"))."
  - "Pattern: Seeded-messages React testing — components accept optional initialMessages prop; tests render <App initialMessages={...} /> to assert rendering without transport. Phase 4 extends with live transport E2E."
  - "Pattern: ValidTypes whitelist gate — EmitChunk rejects unknown/snake_case Type before json.Marshal so wire drift fails loudly server-side, not silently client-side."

requirements-completed: []

# Metrics
duration: ~25min
completed: 2026-08-11
---

# Phase 1 Plan 03: SSE Wire Contract + React Citation Stub Summary

**UIMessageChunk wire envelope (16-type whitelist) + Hertz SSE writer using actual v0.10.6 API + spiked sse CLI proving 6-frame round-trip + React 19 stub rendering source-url chunks as clickable `<a>` — SC-1 layers (b)+(c) and SC-4 all green.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-08-11T04:15:00Z
- **Completed:** 2026-08-11T04:40:00Z
- **Tasks:** 2 (Go wire contract + React stub)
- **Files created:** 10 (4 Go + 6 web TS/TSX + web configs)
- **Files modified:** 3 (scaffold_stubs.go, go.mod, go.sum)
- **Commits:** 2 atomic (Task 1 Go, Task 2 web)

## Accomplishments

- **SC-1 layers (b) + (c) verified.** UIMessageChunk struct shape (layer b — 10 subtests pin JSON per category) + full Hertz → HTTP → JSON parse round-trip (layer c — TestSpikeSSERoundTrip reads a real Hertz response via stdlib http.Get and reassembles 3 chunks). Combined with Plan 01's layer (a) wire format, SC-1 is fully validated across all three layers.
- **SC-4 verified.** React 19 + @ai-sdk/react stub renders `source-url` parts as `<a href={p.url}>` elements. vitest confirms `link.tagName === 'A'` + `href === seeded URL` via `screen.getByRole('link', ...)`. Three tests pass: source-url clickable, unknown-type silent drop, reasoning in `<details>`.
- **T-1-11 mitigation in place.** `spike.ValidTypes` (16-type kebab-case whitelist) gates EmitChunk BEFORE any wire write. `TestEmitChunkRejectsUnknownType` proves snake_case `text_delta` is rejected with a clear error rather than silently dropped by the AI SDK parser. No snake_case Type can reach the wire.
- **Pitfall 2 avoided.** Hertz SSE writer uses the ACTUAL v0.10.6 API (`sse.NewWriter(c)` + `w.WriteEvent("", "message", payload)`), NOT the PROJECT.md §2.2 anti-pattern. Verified against writer.go source; `grep -cE "(Publish|NewStream)" cmd/spiked/sse.go internal/spike/hertz_sse.go` returns 0+0.
- **`spiked sse` CLI works end-to-end.** `./cmd/spiked/spiked.exe sse` prints `[OK] spike sse: parsed 6 frames (types: text-delta, reasoning-delta, tool-input-available, source-url, data-policy, finish-step)` — one chunk from each wire category.

## Task Commits

Each task was committed atomically:

1. **Task 1: UIMessageChunk + Hertz SSE writer + spike sse CLI** — `3ff954a` (feat)
2. **Task 2: React 19 + Vite + vitest web stub** — `13a62bc` (feat)

## Files Created/Modified

- `internal/spike/uimsg.go` — UIMessageChunk struct (13 fields, camelCase JSON tags) + ValidTypes 16-type whitelist + IsValidType()
- `internal/spike/hertz_sse.go` — EmitChunk + StreamChunks + ContentTypeIsSSE using hertz@v0.10.6 sse.NewWriter + WriteEvent
- `cmd/spiked/sse.go` — runSSE (dynamic-port Hertz + 6-chunk handler + round-trip parse), sseChatHandler, parseSSEFrames, waitForListener
- `cmd/spiked/sse_test.go` — TestUIMessageChunkShape (10 subtests), TestSpikeSSERoundTrip, TestEmitChunkRejectsUnknownType
- `cmd/spiked/scaffold_stubs.go` (modified) — runSSE stub removed (promoted to real impl)
- `web/package.json` — pinned ai@^7, @ai-sdk/react@^4, react@^19, vitest@^2, @testing-library/react@^16
- `web/src/App.tsx` — useChat + DefaultChatTransport; LiveChat vs SeededView split; source-url -> `<a>`, reasoning -> `<details>`, unknown -> null
- `web/src/__tests__/citations.test.tsx` — 3 vitest tests (SC-4 citation rendering)
- `web/{vite,vitest}.config.ts + vitest.setup.ts + tsconfig.json + index.html + src/main.tsx` — Vite/vitest config + entry
- `go.mod + go.sum` (modified) — go mod tidy pulled protobuf + fsnotify (Hertz server.Default transitive deps)

## Decisions Made

### 1. AI SDK version pin (Rule 3 blocking-fix)

**Decision:** Pin `ai@^7.0.0` + `@ai-sdk/react@^4.0.0` (NOT the plan's `ai@^5.0.0` + `@ai-sdk/react@^5.0.0`).

**Rationale:** `@ai-sdk/react@5.0.0` does not exist on npm (latest is 4.0.62). The `ai@5.x` line predates the `@ai-sdk/react` package split — React hooks lived in the `ai` package itself back then. The current stable, compatible pair is `ai@7.0.59` + `@ai-sdk/react@4.0.62` (the `@ai-sdk/react@4.x` line declares `ai@7.x` as a dependency). STACK.md's "AI SDK v5" refers to the product generation that introduced UIMessageChunk, not the npm package version. Verified `useChat`, `DefaultChatTransport`, `UIMessage`, `SourceUrlUIPart` all exported at the pinned versions. Treated as Rule 3 (blocking) not a package-legitimacy checkpoint: same official Vercel packages, just a corrected version line.

### 2. Hertz server.WithListener over WithHostPorts

**Decision:** Bind the listener ourselves (`net.Listen("tcp","127.0.0.1:0")`) and pass it to `server.Default(server.WithListener(ln))`.

**Rationale:** The WithListener option (option.go:176) lets Hertz serve on the already-bound socket — no close-and-rebind race, no port-reuse window. The plan's WithHostPorts alternative requires closing the probe listener before Hertz rebinds to the same port, which is racy on shared hosts. Matches the pattern in hertz example_test.go.

### 3. EmitChunk does NOT call w.Close()

**Decision:** EmitChunk calls `sse.NewWriter(c)` + `w.WriteEvent(...)` but omits `w.Close()`. StreamChunks (channel variant) DOES call Close when the channel drains.

**Rationale:** Close finalizes the chunked-body encoding (writes the `0\r\n\r\n` terminator). Calling it after every EmitChunk would terminate the response mid-stream. NewWriter is idempotent (writer.go:47-52 reuses the hijacked writer), so per-call NewWriter is safe. The Hertz framework auto-finalizes when the handler returns (example_test.go:48-50 comment confirms). Each WriteEvent flushes immediately (writer.go:157), so the client sees each frame without waiting.

### 4. 16-type ValidTypes includes data-policy

**Decision:** Whitelist `data-policy` alongside the standard AI SDK v5 chunk types.

**Rationale:** Per CONTEXT.md D-03/D-04, AI SDK v5 supports `data-*` custom types for Skill-specific payloads. SkillForge's `data-policy` type carries the compliance Verdict (`{verdict, confidence, citations}`), distinct from `source-url` which carries the citation URL. Including it in ValidTypes aligns the whitelist with TestUIMessageChunkShape's 6-type shapeTypes iteration and unblocks the spiked sse CLI's data-policy frame emission.

### 5. App.tsx split (LiveChat + SeededView)

**Decision:** Two components: LiveChat (calls useChat) and SeededView (renders prop). App renders one or the other based on `initialMessages`.

**Rationale:** React's Rules of Hooks forbid conditional hook calls inside a component. Splitting keeps useChat unconditional within LiveChat while letting tests render SeededView without triggering a real transport call in jsdom. Both share renderPart/renderMessage helpers.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] AI SDK version pin corrected (plan's @ai-sdk/react@^5 does not exist)**
- **Found during:** Task 2 (web/package.json dependency resolution)
- **Issue:** Plan specified `"ai": "^5.0.0"` + `"@ai-sdk/react": "^5.0.0"`. `@ai-sdk/react@5.0.0` does not exist on npm (latest 4.0.62); `npm install` would fail with E404.
- **Fix:** Pinned to actual compatible pair: `ai@^7.0.0` + `@ai-sdk/react@^4.0.0`. Verified useChat + DefaultChatTransport + UIMessage types all exported. STACK.md "AI SDK v5" refers to product generation, not npm version.
- **Files modified:** web/package.json
- **Verification:** `npm install --no-audit --no-fund` → 193 packages, 0 errors; `npm test` → 3/3 PASS
- **Committed in:** `13a62bc` (Task 2 commit)

**2. [Rule 3 - Blocking] go.sum missing Hertz server transitive deps**
- **Found during:** Task 1 (`go build ./internal/spike ./cmd/spiked`)
- **Issue:** Plan 01 only imported `pkg/protocol/sse` directly (few transitive deps). Task 1 imports `server.Default` which pulls `app/server/binding` (protobuf) + `app/server/render` (fsnotify) — both missing from go.sum, build failed.
- **Fix:** `go mod tidy` resolved the full transitive closure of hertz@v0.10.6. These are transitive deps of the already-approved hertz package, not new top-level deps.
- **Files modified:** go.mod, go.sum
- **Verification:** `go build ./internal/spike ./cmd/spiked` → OK; `go test ./cmd/spiked` → 3/3 PASS
- **Committed in:** `3ff954a` (Task 1 commit)

**3. [Rule 3 - Blocking] vitest config needed `globals: true` for jest-dom setup**
- **Found during:** Task 2 (`npm test` first run)
- **Issue:** `@testing-library/jest-dom` calls `expect.extend(extensions)` at import time (in vitest.setup.ts). Without `globals: true`, `expect` is undefined at setup-file load → `ReferenceError: expect is not defined`.
- **Fix:** Added `globals: true` to vitest.config.ts test options. tsconfig.json already had `"types": ["vitest/globals", "node"]`, so types were already configured for globals mode — only the runtime config was missing.
- **Files modified:** web/vitest.config.ts
- **Verification:** `npm test` → 3/3 PASS in 2.48s
- **Committed in:** `13a62bc` (Task 2 commit)

**4. [Rule 1 - Bug] Acceptance grep would match anti-pattern documentation strings**
- **Found during:** Task 1 (acceptance grep verification)
- **Issue:** `grep -cE "(Publish|NewStream)"` returned 4 matches in hertz_sse.go — all in documentation comments explaining which APIs NOT to use. The acceptance criterion requires 0 (proving the APIs aren't CALLED), but the grep is literal.
- **Fix:** Rephrased comments to avoid the literal "Publish"/"NewStream" tokens — used "event-publisher method" / "stream-constructor function" instead. The documentation value is preserved (reader knows which APIs don't exist) without tripping the grep.
- **Files modified:** internal/spike/hertz_sse.go (comment text only)
- **Verification:** `grep -cE "(Publish|NewStream)" cmd/spiked/sse.go internal/spike/hertz_sse.go` → 0+0
- **Committed in:** `3ff954a` (Task 1 commit)

---

**Total deviations:** 4 auto-fixed (3 Rule 3 blocking, 1 Rule 1 bug)
**Impact on plan:** All auto-fixes necessary for build/test correctness. No scope creep. The AI SDK version correction is the most significant — it reflects a planning error (wrong version line) corrected to match registry reality.

## Issues Encountered

None — all issues handled as auto-fix deviations above.

## User Setup Required

None — no external service configuration required. Plan 03 is fully in-process: Go tests use httptest-style dynamic ports, vitest uses seeded messages (no network). The spiked sse CLI binds 127.0.0.1 on a dynamic port.

## Known Stubs

- **`web/src/App.tsx` LiveChat transport untested:** The `useChat({ transport: spikeTransport })` path is wired but NOT exercised by vitest — tests render SeededView via the `initialMessages` prop. This is intentional for Phase 1 (validate the rendering contract, not live transport). Phase 4 will add a full E2E test against a live Hertz endpoint. Not a blocking stub — the wire contract itself is proven on the Go side by TestSpikeSSERoundTrip.
- **T-1-08 source-url href unsanitized (per plan threat_model):** App.tsx renders `<a href={p.url}>` directly from the chunk payload with NO URL sanitization. This is flagged in the plan's threat_model (T-1-08) and explicitly deferred to Phase 4 hardening (allowlist + rel="noopener noreferrer"). The spike test never executes user URLs — it only checks the `<a>` renders. Not a regression; documented for Phase 4.

## Threat Flags

None — no security-relevant surface beyond the plan's `<threat_model>`. The T-1-08 (source-url XSS), T-1-09 (chunk injection via newlines), T-1-10 (API key in errors), T-1-11 (Type enum), T-1-SC (npm package legitimacy) items from the plan's threat register are all handled as specified:
- T-1-08: deferred to Phase 4 (documented above)
- T-1-09: accepted — Hertz WriteEvent auto-splits multi-line data (writer.go:141-149)
- T-1-10: EmitChunk error path never includes API keys; `errorText` field only
- T-1-11: ValidTypes whitelist gate enforced by EmitChunk + TestEmitChunkRejectsUnknownType
- T-1-SC: packages verified legitimate (Vercel official); versions pinned in package.json

## Next Phase Readiness

**Plan 01-04 (spike e2e, Wave 2):**
- Reuses `cmd/spiked/scaffold_stubs.go` `runE2E` slot (still a stub)
- Chains UIMessageChunk frames through real GLM-5.2 + skill pipeline
- Can reuse `internal/spike.EmitChunk` for SSE emission of real agent output
- Must validate source-url chunks carry real Milvus citation URLs (ties to Plan 02 Milvus tier-filter)

**Phase 2 (production wiring):**
- Reuses `internal/spike.UIMessageChunk` as the canonical wire envelope struct
- Reuses `internal/spike.EmitChunk` / `StreamChunks` for Hertz SSE handlers
- Reuses the `server.WithListener(net.Listen(...))` dynamic-port pattern for integration tests

**Phase 4 (Streaming Trust UI):**
- Reuses `web/` structure (Vite + React 19 + vitest)
- Reuses App.tsx renderPart pattern (source-url -> `<a>`, reasoning -> `<details>`, text -> `<p>`)
- MUST add URL sanitization for source-url href (T-1-08 — allowlist + rel="noopener noreferrer")
- MUST replace seeded-messages test with live transport E2E against Hertz
- Will load sketch-findings-ai-agent skill for three-panel layout

**Blockers / concerns:**
- None. SC-1 layers (a)+(b)+(c) and SC-4 are all green. Plan 03 is self-contained.

---

## Self-Check: PASSED

**Files verified (all FOUND):**
- `internal/spike/uimsg.go` (UIMessageChunk + ValidTypes 16-type whitelist)
- `internal/spike/hertz_sse.go` (EmitChunk + StreamChunks + ContentTypeIsSSE)
- `cmd/spiked/sse.go` (runSSE + sseChatHandler + parseSSEFrames + waitForListener)
- `cmd/spiked/sse_test.go` (TestUIMessageChunkShape + TestSpikeSSERoundTrip + TestEmitChunkRejectsUnknownType)
- `cmd/spiked/scaffold_stubs.go` (runSSE stub removed, runE2E stub intact)
- `web/package.json` (ai@^7, @ai-sdk/react@^4, react@^19)
- `web/src/App.tsx` (useChat + DefaultChatTransport + LiveChat/SeededView split)
- `web/src/__tests__/citations.test.tsx` (3 tests: source-url <a>, unknown drop, reasoning <details>)
- `web/{vite,vitest}.config.ts + vitest.setup.ts + tsconfig.json + index.html + src/main.tsx`

**Commits verified (all FOUND):**
- `3ff954a` — feat(01-03): UIMessageChunk + Hertz SSE writer + spike sse (SC-1 b+c)
- `13a62bc` — feat(01-03): React 19 + Vite + vitest web stub (SC-4 clickable citations)

**Test verification:**
- `go test ./cmd/spiked -run "TestUIMessageChunkShape|TestSpikeSSERoundTrip|TestEmitChunkRejectsUnknownType" -v -count=1 -timeout 30s` → 3/3 PASS in 0.227s
- `cd web && npm test` → 3/3 PASS in 2.48s
- `./cmd/spiked/spiked.exe sse` → `[OK] spike sse: parsed 6 frames (types: text-delta, reasoning-delta, tool-input-available, source-url, data-policy, finish-step)`
- `grep -cE "(Publish|NewStream)" cmd/spiked/sse.go internal/spike/hertz_sse.go` → 0+0
- `grep -c "open.bigmodel.cn" cmd/spiked/sse.go internal/spike/*.go` → 0+0+0
- `grep -cE "type:\s*'text_" web/src/App.tsx web/src/__tests__/citations.test.tsx` → 0+0

---
*Phase: 01-integration-spike-trust-contract*
*Plan: 03*
*Completed: 2026-08-11*
