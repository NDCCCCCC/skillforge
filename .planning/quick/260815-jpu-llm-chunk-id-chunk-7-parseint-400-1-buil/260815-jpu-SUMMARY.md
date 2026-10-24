---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick 260815-jpu Summary

## Goal

Fix the LLM-output drift on `chunk_id` end-to-end: when the verdict JSON emits
`"CHUNK 7"` / `"ref-7"` / `"[ref-7]"` (prefixed forms), the FE chunk hover popover
+ ChunkSourceView fetch must NOT 400. Four defense layers — all shipped.

## Layers Shipped

| Layer | Where | Change |
|-------|-------|--------|
| 1 (prevention) | `internal/server/pipeline.go buildPrompt` | `[CHUNK %d]` → `[ref-%d]` data-tag prefix |
| 2 (constraint) | `systemInstructionsTemplate` JSON schema | chunk_id description: `"<the literal numeric chunk ID from [ref-N] — output digits only, no prefix>"` |
| 3 (BE tolerant) | `internal/server/knowledge.go` | new `parseLenientChunkID(string) (int64, bool)`; `chunkHandler` calls it instead of bare `strconv.ParseInt`; `[WARN]` log on prefix-strip hits |
| 4 (FE normalize) | `web/src/lib/chunk-lookup.ts` | new `normalizeChunkID(string) string` pure fn; `lookupChunk` / `fetchChunkContent` / `extractChunkMetadata` route every input through it |

## Files Changed

| File | Change |
|------|--------|
| `internal/server/pipeline.go` | buildPrompt loop tag + schema description (2 text edits + godoc comment) |
| `internal/server/pipeline_stream_test.go` | TestStreamAndAssemble assertion pins on `tier=` (schema now legitimately references `[ref-N]` in prose) |
| `internal/server/knowledge.go` | new `parseLenientChunkID` helper + `chunkHandler` rewire + WARN log |
| `internal/server/knowledge_test.go` | 14-case `TestParseLenientChunkID` + 6-case `TestChunkHandler_TolerantIDAcceptsPrefix` + 3-case `TestChunkHandler_GarbageIDStillReturns400`; `invokeChunkHandler` URL-escapes the idPath |
| `web/src/lib/chunk-lookup.ts` | new `normalizeChunkID` export + wiring at 3 entry points |
| `web/src/__tests__/chunk-lookup.test.ts` | 14-case `normalizeChunkID` parameterised + 2-case `lookupChunk with normalize` + 1-case `fetchChunkContent with normalize` |
| `web/src/__fixtures__/chunks.json` | additive `"7"` entry (年假标准 chunk matching the buildPrompt corpus) so `lookupChunk('CHUNK 7')` hits the fixture, not NOT_FOUND_TEXT |

## Test Results

| Suite | Before | After |
|-------|--------|-------|
| `go test ./internal/server/ -count=1` | 100% | 100% (3 new test funcs + 23 new subtests; all 5 existing `TestChunkHandler_*` stay green) |
| `go vet ./internal/server/` | clean | clean |
| `web npx tsc --noEmit` | 0 errors | 0 errors |
| `web npx vitest run src/__tests__/chunk-lookup.test.ts` | 4 tests | 22 tests PASS |
| `web npx vitest run` (full) | 244 tests | 262 tests PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Pipeline-stream test assertion `NotContains("[ref-")` no longer unique**

The plan said to swap `NotContains("[CHUNK ")` → `NotContains("[ref-")`. But the new
schema description legitimately contains `[ref-N]` to clarify "the literal numeric
chunk ID" — so the assertion would trip on the schema prose, not the data block.

- **Fix:** Pin the assertion on the literal `tier=` token (the schema uses `tier:`,
  data blocks use `tier=... source=...`). One-line change with explanatory comment.

**2. [Rule 1 — Bug] `invokeChunkHandler` path needs URL encoding for prefixed IDs**

The existing helper does `"/api/knowledge/chunk/" + idPath`. For prefixed inputs
like `"CHUNK 7"` / `"[ref-7]"`, the raw space / brackets split the request line
and the handler never sees the variant.

- **Fix:** `url.PathEscape(idPath)` inside `invokeChunkHandler`. Hertz pre-decodes
  `c.Param("id")` so the handler still sees the original `idStr` for the WARN
  log. Existing single-word inputs (`"abc"`, `"1"`, `"999"`) encode to themselves.

**3. [Rule 1 — Bug] `parseLenientChunkID` rejects `"CHUNK7"` (no separator)**

Plan acceptance list: `"CHUNK7"` → `(7, true)`. Initial implementation only
stripped the prefix when followed by ` ` or `-`, so `"CHUNK7"` left the string
intact and ParseInt rejected it.

- **Fix:** Also accept `rest[0] >= '0' && rest[0] <= '9'` as a valid label-
  payload boundary. `"CHUNKabc"` still rejected (no digit boundary) so ParseInt
  gets a clean fail.

**4. [Rule 1 — Bug] `normalizeChunkID("CHUNK abc")` returned `"abc"`, plan wants `"CHUNK abc"`**

Initial implementation stripped the prefix whenever syntactically present. But
the plan's contract is: if the prefix-stripped remainder is not a valid int64,
return the input UNCHANGED so the BE `parseLenientChunkID` can still 400.

- **Fix:** After bracket / paren / quote strip, attempt prefix strip but verify
  the stripped remainder matches `/^-?\d+$/`. If yes, return the stripped form.
  If no, `return chunkId` (original) so BE handles the 400 path. Single-digit
  pure-numeric IDs and `"CHUNK 7"`-style inputs work; `"CHUNK abc"`, `"ref-foo"`,
  arbitrary garbage stay unchanged.

### Plan-level Adjustments

**5. [Rule 2 — Missing critical fixture] `chunks.json` had no `"7"` key**

The plan's `lookupChunk("CHUNK 7")` test expects fixture hit on key `"7"`, but
the existing fixture only carries `c-001`..`c-004` (no bare numeric IDs). Two
options: change the fixture OR change the test inputs. Adding a `"7"` entry
(年假标准 verbatim chunk) preserves the plan's verbatim test code.

- **Fix:** Single additive `"7"` entry in `web/src/__fixtures__/chunks.json`. No
  existing entries removed or modified. Note: plan constraint said
  `"1"/"2"/"7" 等保持不变` — those keys were claimed to exist but didn't, so the
  constraint is moot. The fixture schema/format is unchanged.

**6. [Rule 1 — Bug] `TestStreamAndAssembleSendsSystemAndUserMessages` needed a rewire**

The plan said "改前 / 改后" — the old `NotContains` pattern just swaps text. After
fix #1 above, the assertion uses `tier=` — the test file's surrounding comment
also had to update from `"messages[0] MUST NOT contain the [ref-...] data blocks"`
to `"messages[0] MUST NOT contain chunk data blocks (those belong on
messages[1])"` (no `[ref-...]` substring in the new message).

## Verification (Global)

```bash

# Backend

go test ./internal/server/ -count=1 -timeout 120s    # ok
go vet ./internal/server/                              # clean
grep "CHUNK %d" internal/server/pipeline.go           # only in godoc comment (line 579); code uses ref-%d (line 586)
grep "ref-%d" internal/server/pipeline.go             # matches code + godoc
grep "digits only, no prefix" internal/server/pipeline.go  # matches schema + comment
grep "\[CHUNK" internal/server/pipeline_stream_test.go     # no matches
grep "parseLenientChunkID" internal/server/knowledge.go     # helper + caller + godoc
grep "strconv.ParseInt(idStr" internal/server/knowledge.go # no matches (replaced)

# Frontend

cd web && npx tsc --noEmit                             # 0 errors
cd web && npx vitest run src/__tests__/chunk-lookup.test.ts   # 22/22 PASS
cd web && npx vitest run                               # 262/262 PASS
grep "export function normalizeChunkID" web/src/lib/chunk-lookup.ts   # line 73
```

## Acceptance — Plan `must_haves` Truths

| Truth | Status |
|-------|--------|
| Pipeline `[ref-<id>]` (not `[CHUNK <id>]`) | ✅ `internal/server/pipeline.go:586` |
| Schema `chunk_id` field description: "digits only, no prefix" | ✅ `internal/server/pipeline.go:98` |
| Backend accepts `"7"` / `"CHUNK 7"` / `"ref-7"` / `"[ref-7]"` / `"CHUNK7"` → same int64 | ✅ 14 cases in `TestParseLenientChunkID` |
| Backend rejects `"abc"` → 400 | ✅ 3 cases in `TestChunkHandler_GarbageIDStillReturns400` |
| Backend `[WARN]` log on prefix | ✅ `internal/server/knowledge.go:220-222` |
| Frontend `normalizeChunkID` strips prefixes (pure fn, exported) | ✅ `web/src/lib/chunk-lookup.ts:73` |
| Frontend normalize doesn't change bare numeric / fixture keys | ✅ 14 cases include `"7" → "7"` and `"c-1" → "c-1"` |
| TestStreamAndAssemble assertions all green | ✅ |
| All `TestChunkHandler_*` + new tolerance tests green | ✅ |

## Acceptance — `artifacts` Contracts

| Artifact | Required Content | Verified |
|----------|------------------|----------|
| `internal/server/pipeline.go` | contains `ref-%d` | ✅ |
| `internal/server/pipeline_stream_test.go` | contains `ref-1` | ✅ |
| `internal/server/knowledge.go` | contains `parseLenientChunkID` | ✅ (definition + 1 caller + 1 godoc ref) |
| `internal/server/knowledge_test.go` | contains `TolerantID` | ✅ (`TestChunkHandler_TolerantIDAcceptsPrefix`) |
| `web/src/lib/chunk-lookup.ts` | exports `normalizeChunkID` | ✅ |
| `web/src/__tests__/chunk-lookup.test.ts` | contains `normalizeChunkID` | ✅ (import + 14 cases) |

## Commits

| Hash | Subject |
|------|---------|
| `8eab17c` | feat(server): pipeline prompt — chunk tag [CHUNK N]→[ref-N] + schema digits-only hint (quick 260815-jpu Layer 1+2) |
| `e41a810` | feat(server): chunkHandler tolerant parse — parseLenientChunkID + WARN on prefix (quick 260815-jpu Layer 3) |
| `977333f` | feat(web): normalizeChunkID + lookup/fetch/extract entry-point wiring (quick 260815-jpu Layer 4) |

## Not Changed (Out of Scope, Per Plan)

- `internal/verdict/grounding.go canonicalChunkID` — verdict-layer handling still
  owns the `CHUNK ` strip for the in-process grounding map. New server-package
  helper is the URL-path analogue.
- `internal/agent/retrieve_tool.go` `[CHUNK N]` framing — Phase 3 deferred; this
  quick task deliberately does not introduce Phase-3 scope creep.
- `internal/server/pipeline.go` userQueryTemplate — only the schema prose
  description changed; nothing else in the system prompt template.
- `systemPromptTemplate` (the pre-260815-grw fallback path) — single-prompt
  fallback stays untouched.
- `ROADMAP.md` — explicitly excluded by task constraints.

## Live-stack smoke (suggested; NOT auto-run)

After restart of the dev server, trigger a query that produces a citation.
If the LLM still echoes a prefix into `chunk_id`:

- Server log should show `[WARN] server: chunkHandler lenient parse: "CHUNK 7" → 7 (LLM likely echoed label prefix)`
- CitationCluster hover popover should render the verbatim chunk text (no 400)
- ChunkSourceView should render normally (no 400)

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| (none new) | — | No new network endpoints, auth paths, file access patterns, or trust-boundary schema changes. The BE endpoint already exists; both helpers are package-private / pure functions. WARN log on LLM output is server-side stderr only. |
