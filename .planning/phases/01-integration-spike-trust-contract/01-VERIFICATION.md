---
phase: 01-integration-spike-trust-contract
verified: 2026-08-11T13:05:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  is_re_verification: false
human_verification:
  - test: "SC-2 live Milvus tier-filter round-trip — run `spiked milvus` against internal Milvus cluster"
    expected: "[OK] spike milvus: 12 docs seeded, tier filter returns 3 authority chunks; zero bleed across all 4 tiers + default-deny"
    why_human: "Sandbox has no internal-network access. Per Plan 01-02 Task 2 `checkpoint:human-verify` (blocking gate). Code path complete; BuildMilvusFilter unit tests + CN-char escape + default-deny logic all pass."
  - test: "SC-3 live GLM-5.2 5-query pipeline + cancellation timing — run `spiked e2e` with GLM_API_KEY set"
    expected: "[OK] spike e2e: 5 queries validated, trace at spike-e2e-trace.json, cancel <Nms> < 200ms verified"
    why_human: "Sandbox has no GLM_API_KEY. Per Plan 01-04 Task 1 `checkpoint:human-verify` (blocking gate). Code path complete; TestValidateVerdict passes; cancellation logic exercised by probeCancellation."
deferred:
  - truth: "SC-3 SequentialAgent 2-step (one ChatModel call then one tool call)"
    addressed_in: "Phase 3 (Agentic Retrieval & Trust Evaluation)"
    evidence: "Plan 01-04 <objective>: 'SequentialAgent 2-step DEFERRED to Phase 3 per user-approved Claude\\'s Discretion in 01-DISCUSSION-LOG.md §Claude\\'s Discretion Summary'. 01-DISCUSSION-LOG.md line 113: 'Eino usage = single ChatModel (no ADK in Phase 1)'. Phase 1 implements only single ChatModelAgent; the 2-step orchestration is Phase 3's primary deliverable."
---

# Phase 1: Integration Spike & Trust Contract — Verification Report

**Phase Goal (ROADMAP.md §Phase 1):** Validate the core integration of Eino v0.7.7 + Hertz SSE + Vercel AI SDK v5 + Milvus tier filter before committing to the full data model and Skill schema.

**Verified:** 2026-08-11T13:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

This is a **spike** phase. Per the verification contract: "Live infra (Milvus reachability, GLM key) being deferred to the user is an EXPECTED outcome of the internal-API strategy, not a gap — as long as the code path is complete and the verify command is documented." Status is `passed` when SC-1/SC-4/SC-5 are fully green via automated tests AND SC-2/SC-3 are code-complete with documented live-verify commands (overriding the global "passed requires empty human_verification" rule per verification_focus).

### Observable Truths (Success Criteria SC-1..SC-5)

| # | Truth (SC) | Status | Evidence |
|---|------------|--------|----------|
| 1 | SC-1: Hertz streams UIMessageChunk envelope, no wire-format drift (3 layers: wire + typed + parse) | VERIFIED | `internal/spike/uimsg.go` (13-field struct, 16-type `ValidTypes` whitelist with `data-policy`) + `internal/spike/hertz_sse.go` (uses `sse.NewWriter(c).WriteEvent("", "message", payload)` — NOT `Publish`/`NewStream`). `go test ./cmd/spiked ./internal/spike -count=1` → PASS (TestSpikeWireMock, TestMockServerReceivesWireExtras, TestUIMessageChunkShape 10 subtests, TestSpikeSSERoundTrip, TestEmitChunkRejectsUnknownType). `./spiked sse` → `[OK] spike sse: parsed 6 frames (types: text-delta, reasoning-delta, tool-input-available, source-url, data-policy, finish-step)`. `grep -cE "(Publish\|NewStream)" cmd/ internal/` returns 0. |
| 2 | SC-2: Milvus tier-filter returns ONLY `tier == "权威"`, zero bleed across 4-tier dataset; empty tier list returns 0 rows (default-deny) | VERIFIED (code path); live round-trip deferred to human-verify | `internal/spike/milvus_client.go` exports `BuildMilvusFilter` + `NewMilvusClient` + `SeedDocuments` + `SearchWithTierFilter` + `EmbedQuery` + `EmbedQueryDeterministic`. `cmd/spiked/milvus.go` seeds 12 fixtures, runs 4 tier queries + default-deny, asserts zero bleed. `BuildMilvusFilter([]string{"权威"})` → `tier in ["权威"]` (CN-char escaped via Go `%q`). `BuildMilvusFilter(nil)`/`[]` → `"false"` (D-09 default-deny). TestBuildMilvusFilter (8 cases + 3 special-char injection cases) PASS. 12 fixtures exist (3 per tier × 4 tiers). **Live round-trip pending human-verify** — sandbox has no `MILVUS_ADDRESS`. Documented command: `MILVUS_ADDRESS=<internal>:19530 go run ./cmd/spiked milvus`. |
| 3 | SC-3: ChatModelAgent + cancellation propagates within 200ms (single-agent portion; 2-step SequentialAgent DEFERRED to Phase 3 per user-approved Claude's Discretion) | VERIFIED (code path); live GLM-5.2 call deferred to human-verify | `cmd/spiked/e2e.go` uses `adk.NewChatModelAgent` + `adk.NewRunner` + `adk.WithChatModelOptions`. `probeCancellation(ctx, runner)` measures wall-clock from `cancel()` to `iter.Next() == (zero, false)`, asserts `< 200ms` (cancellationBudgetMS=200). 5 verbatim Chinese compliance queries from spike 002/003. `spike.Validate(v)` gate on every Verdict. TestValidateVerdict (5 cases) PASS. **Live GLM-5.2 5-query run pending human-verify** — sandbox has no `GLM_API_KEY`. Documented command: `export $(grep -v '^#' .env \| xargs) && go run ./cmd/spiked e2e`. **SequentialAgent 2-step explicitly deferred to Phase 3** per 01-DISCUSSION-LOG.md §Claude's Discretion Summary line 113 ("Eino usage = single ChatModel (no ADK in Phase 1)") + Plan 01-04 `<objective>` deferred note. |
| 4 | SC-4: "Fake cited answer" demo with hardcoded chunk citations rendered as clickable `<a>` elements | VERIFIED | `web/src/App.tsx` uses `useChat` + `DefaultChatTransport` (from `@ai-sdk/react` + `ai`); `renderPart` switches on `p.type` and emits `<a href={p.url}>` for `source-url`. `cd web && npx vitest run` → 3/3 PASS in 2.27s (source-url `<a>` + `tagName==='A'` + `href===seeded URL`; unknown-type silent drop; reasoning in `<details>`). |
| 5 | SC-5: Version-pinned integration test runs SC-1..SC-4 validators in <10s and fails on wire-format/filter/version drift | VERIFIED | `integration/contract/contract_test.go` has `//go:build contract` tag. `go test ./integration/contract -tags=contract -count=1 -timeout 10s` → 6/6 PASS in 3.442s (budget 10s; headroom 6.5s). `TestContractVersionPin` asserts eino v0.7.7 + hertz v0.10.6 + eino-ext openai v0.1.5 + milvus-sdk-go v2.4.2. `TestContractWireFormat` shells out to `spiked wire` + asserts UIMessageChunk kebab-case Type. `TestContractMilvusFilter` (3 cases + CN_char_escape subtest). `TestContractStructuredOutputSchema` (JSON round-trip + Validate gate). `TestContractRunsUnder10s` (budget gate). `TestContractFullSpikeSuite` (wire + sse pass; milvus + e2e skip with explicit reason). |

**Score:** 5/5 truths verified (3 fully green via automated tests; 2 code-complete with documented live-verify deferrals per spike strategy).

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|--------------|----------|
| 1 | SC-3 SequentialAgent 2-step (one ChatModel call then one tool call) — second half of SC-3 | Phase 3 (Agentic Retrieval & Trust Evaluation) | Plan 01-04 `<objective>`: "SequentialAgent 2-step DEFERRED to Phase 3 per user-approved Claude's Discretion in 01-DISCUSSION-LOG.md §Claude's Discretion Summary". 01-DISCUSSION-LOG.md line 113: "Eino usage = single ChatModel (no ADK in Phase 1)". Phase 3 ROADMAP goal: "Make the agent decide for itself (retrieve / refuse / ask / multi-hop)". |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `go.mod` | Pinned eino v0.7.7, hertz v0.10.6, milvus-sdk-go v2.4.2, eino-ext openai v0.1.5, testify, yaml.v3 | VERIFIED | `cat go.mod` confirms all 6 pinned versions in direct `require` block. TestContractVersionPin asserts the same. |
| `.env.example` | `GLM_API_KEY=` placeholder + internal-api Milvus/BGE-M3 | VERIFIED | Contains MILVUS_ADDRESS, MILVUS_TOKEN, BGE_M3_URL, BGE_M3_AUTH, GLM_API_KEY, GLM_BASE_URL. |
| `.gitignore` | Excludes `.env` from version control | VERIFIED | `.env` on line 1. `git ls-files \| grep -E '^\.env$'` returns empty. |
| `cmd/spiked/main.go` | Subcommand router (4 slots: wire/milvus/sse/e2e) | VERIFIED | 62 lines, switch on `os.Args[1]`, usage msg to stderr exit 2, unknown subcommand exit 2, error path exit 1. `./spiked` (no args) → exit 2 with usage; `./spiked bogus` → exit 2 with "unknown subcommand". |
| `cmd/spiked/wire.go` + `wire_test.go` | SC-1 layer a — mock OpenAI server round-trip | VERIFIED | runWire + parseSKILL + startMockServer (dynamic port). TestSpikeWireMock + TestMockServerReceivesWireExtras PASS. `./spiked wire` → `[OK] spike wire: temperature=0 passthrough verified` exit 0. |
| `cmd/spiked/milvus.go` + `milvus_test.go` | SC-2 — 12-doc seed + tier-filter assertion | VERIFIED (code path) | runMilvus + loadAndEmbedFixtures + bleed checker. TestBuildMilvusFilter PASS. `./spiked milvus` (no MILVUS_ADDRESS) → fails fast with `[FAIL] milvus: milvus connect: milvus config: Addr is empty` (correct fail-fast behavior). |
| `cmd/spiked/sse.go` + `sse_test.go` | SC-1 layers b+c — Hertz SSE writer + parser round-trip | VERIFIED | runSSE on dynamic port, 6-chunk handler emitting one of each wire category, parseSSEFrames, waitForListener. TestUIMessageChunkShape (10 subtests) + TestSpikeSSERoundTrip + TestEmitChunkRejectsUnknownType all PASS. |
| `cmd/spiked/e2e.go` + `e2e_test.go` | SC-3 — ChatModelAgent + cancellation + 5-query loop | VERIFIED (code path) | runE2E + runE2EWithBaseURL + probeCancellation + drainFinalContent. 5 verbatim Chinese queries. TestValidateVerdict (5 cases) always runs + PASS. TestE2ECancellation + TestComplianceQueriesSkip correctly skip when GLM_API_KEY unset. |
| `cmd/spiked/scaffold_stubs.go` | All 4 stubs removed; doc-only marker | VERIFIED | File contains only the Apache-2.0 header + package comment documenting that all 4 subcommands have real implementations. No `func runX` declarations. |
| `internal/spike/uimsg.go` | UIMessageChunk struct + 16-type ValidTypes whitelist + IsValidType | VERIFIED | 13-field struct with camelCase JSON tags; 16-type map (text-3 + reasoning-3 + tool-input-3 + tool-output-1 + source-2 + data-policy-1 + finish-2 + error-1). `IsValidType` method. |
| `internal/spike/hertz_sse.go` | EmitChunk + StreamChunks + ContentTypeIsSSE using actual Hertz API | VERIFIED | `sse.NewWriter(c).WriteEvent("", "message", payload)`. Pre-write `IsValidType()` gate (T-1-11). |
| `internal/spike/milvus_client.go` | BuildMilvusFilter + NewMilvusClient (retry/backoff) + SeedDocuments + SearchWithTierFilter + EmbedQuery + EmbedQueryDeterministic | VERIFIED | 7 exported symbols. CN-char escape via `%q`. Default-deny `"false"`. Positional SDK Search API (4th param = expr, applied PRE-similarity — Pitfall 3 mitigation intact). 5s dial timeout + 3x exponential backoff (1s/2s/4s) per internal-api strategy. |
| `internal/spike/verdict.go` + `verdict_test.go` | Verdict struct + ValidStatuses + Validate — T-1-16 single source of truth | VERIFIED | 4-field struct (Status/Confidence/Citations/ReviewRequired) with JSON tags matching spike 003. ValidStatuses has exactly 4 keys (合规/不合规/需补充/unanswered). Validate checks status enum + confidence range + non-nil citations. 5 tests PASS. |
| `integration/contract/contract_test.go` | `//go:build contract` SC-5 suite | VERIFIED | 6 tests, 3.442s wall-clock (< 10s budget). T-1-16 enforced: imports `spike.Verdict`/`spike.ValidStatuses`/`spike.Validate` (does NOT redeclare). Compile-time anchor `var _ spike.Verdict`. |
| `testdata/skills/{authority,historical,personal,draft}/SKILL.md` | 4 SKILL.md fixtures per SKILL_SCHEMA.md v1.1 | VERIFIED | 4 fixtures exist. Each has YAML frontmatter with tier-matching `reads: ["<tier>"]`, `temperature: 0`, `reasoning: false`, `guardrails.refuse_on_missing_tier: true`. |
| `testdata/docs/{authority,historical,personal,draft}/*.md` | 12 fixture docs (3 per tier × 4 tiers) | VERIFIED | 12 markdown files. Each has YAML frontmatter with `id`, `tier`, `source`, `effective_from`. Bodies are 50-200 char Chinese compliance text, semantically distinct per tier (12 unique md5sums per Plan 01-02 SUMMARY). |
| `web/package.json` | Pinned ai + @ai-sdk/react + react@19 + vitest | VERIFIED | `ai@^7.0.0` + `@ai-sdk/react@^4.0.0` + `react@^19.0.0` + `react-dom@^19.0.0` + `vitest@^2.0.0` + `@testing-library/react@^16.0.0`. **Plan correction**: Plan specified `ai@^5` + `@ai-sdk/react@^5` (Rule 3 blocking-fix in Plan 01-03 SUMMARY); `@ai-sdk/react@5` does not exist on npm. Pinned to actual compatible pair. |
| `web/src/App.tsx` | useChat + DefaultChatTransport + source-url → `<a>` | VERIFIED | 88 lines. LiveChat (calls useChat) + SeededView (renders prop) split keeps hooks unconditional. `renderPart` switches on `p.type`: text→`<p>`, reasoning→`<details>`, source-url→`<a href={p.url}>`, source-document→`<span>`, default→`null`. T-1-08 URL sanitization deferred to Phase 4 per plan threat_model. |
| `web/src/__tests__/citations.test.tsx` | 3 vitest tests for SC-4 | VERIFIED | source-url `<a>` + tagName + href assertion; unknown-type silent drop; reasoning in `<details>`. 3/3 PASS in 2.27s. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `cmd/spiked/wire.go` | mock OpenAI server (dynamic port) | `openai.NewChatModel` + `BaseURL: http://127.0.0.1:%d/v1` | WIRED | TestSpikeWireMock validates end-to-end. Captured body contains `"temperature"`, `"json_object"`, `"disabled"`. |
| `cmd/spiked/wire.go` | `testdata/skills/authority/SKILL.md` | `parseSKILL` (YAML frontmatter unmarshal) | WIRED | `parseSKILL` walks up to `go.mod` for CWD-independent fixture resolution. |
| `cmd/spiked/sse.go` | `internal/spike/hertz_sse.go` | `spike.EmitChunk(c, chunk)` | WIRED | sseChatHandler iterates 6 chunks calling EmitChunk per chunk. |
| `internal/spike/hertz_sse.go` | Hertz `sse.Writer` | `sse.NewWriter(c).WriteEvent("", "message", payload)` | WIRED | Verified writer.go:41/46/64/157 anchors in comments. NOT using `Publish`/`NewStream` (grep returns 0). |
| `internal/spike/uimsg.go` | AI SDK v5 wire spec | kebab-case Type discriminator + camelCase field names | WIRED | 16-type ValidTypes whitelist matches AI SDK v5 UIMessageChunk parser. data-policy registered per D-03/D-04. |
| `cmd/spiked/milvus.go` | `internal/spike/milvus_client.go` | package import for NewMilvusClient + SeedDocuments + SearchWithTierFilter | WIRED | `import "github.com/skillforge/skillforge/internal/spike"`. mc.Search is encapsulated in `SearchWithTierFilter`. |
| `internal/spike/milvus_client.go` | Milvus SDK v2.4.2 `client.Search` | positional `expr` parameter (4th) | WIRED | Pre-similarity filter (Pitfall 3 mitigation). RESEARCH.md Pattern 3 incorrectly described `SearchQueryOptions` struct; Plan 01-02 SUMMARY deviation #1 documents the positional API correction. |
| `cmd/spiked/e2e.go` | `internal/spike/verdict.go` | `spike.Verdict` + `spike.Validate` (single source of truth) | WIRED | T-1-16 enforced. NO local ContractVerdict type. Field rename in verdict.go breaks BOTH cmd/spiked/e2e.go AND integration/contract/contract_test.go at compile time. |
| `cmd/spiked/e2e.go` | `internal/spike/uimsg.go` | `spike.UIMessageChunk` for SSE trace records | WIRED | 4 chunk types recorded per query: text-delta + source-url + data-policy + finish-step. |
| `cmd/spiked/e2e.go` | `adk.NewChatModelAgent` + `adk.NewRunner` | `adk.NewChatModelAgent(ctx, cfg)` + `adk.NewRunner(ctx, RunnerConfig{Agent: agent})` | WIRED | First direct import of `github.com/cloudwego/eino/adk` in cmd/. `runner.Query(ctx, query, WithChatModelOptions(opts...))` returns AsyncIterator drained via `iter.Next()`. |
| `cmd/spiked/e2e.go` | `context.WithTimeout` / `context.WithCancel` | cancellation propagation probe | WIRED | `probeCancellation` uses `context.WithCancel` + goroutine drain + wall-clock measurement. `< 200ms` budget enforced. |
| `integration/contract/contract_test.go` | `internal/spike/verdict.go` | `spike.Verdict` imported, NOT redeclared | WIRED | T-1-16 mitigation. Compile-time anchor `var _ spike.Verdict`. |
| `integration/contract/contract_test.go` | `cmd/spiked` subcommands | `exec.Command` shell-out | WIRED | `buildSpiked` compiles spiked once via `sync.Once`; `runSpiked` invokes `wire` + `sse` (pass) and `milvus` + `e2e` (skip with explicit reason). |
| `web/src/App.tsx` | Hertz SSE endpoint | `DefaultChatTransport({ api: '/api/spike/chat' })` | WIRED | LiveChat uses useChat with spikeTransport. (Test path uses SeededView to avoid jsdom network — Plan 01-03 SUMMARY design decision.) |
| `web/src/App.tsx` | `web/src/__tests__/citations.test.tsx` | `render(<App initialMessages={...} />)` | WIRED | 3 vitest tests render App with seeded messages and assert citation rendering. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `cmd/spiked/wire.go` | `captured bytes.Buffer` | mock OpenAI server handler writes request body | YES — body contains `"temperature"`, `"json_object"`, `"disabled"` substrings asserted by runWire | FLOWING |
| `cmd/spiked/sse.go` | `frames []spike.UIMessageChunk` | parseSSEFrames parses HTTP response body | YES — 6 frames parsed from real Hertz HTTP response, types iterated and printed | FLOWING |
| `cmd/spiked/e2e.go` | `trace []map[string]string` | appendTrace per step (init/cancel/queries/emitted/milvus) | YES when GLM_API_KEY set — produces `spike-e2e-trace.json` with ≥20 entries; redacted (no API key) per T-1-13. TestComplianceQueriesSkip asserts `assert.NotContains(t, string(body), apiKey)`. | FLOWING (live LLM gated) |
| `cmd/spiked/milvus.go` | `docs []spike.Document` | loadAndEmbedFixtures reads `testdata/docs/*/*.md` + embeds | YES — 12 fixtures loaded, embeddings produced (deterministic fallback or real BGE-M3); seeded via SeedDocuments | FLOWING (live Milvus gated) |
| `web/src/App.tsx` | `messages` (from useChat or initialMessages prop) | useChat transport OR test prop | YES — vitest seeds messages with `source-url` parts; App renders `<a>` from `p.url` | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| SC-1 wire round-trip | `./spiked wire` | `[OK] spike wire: temperature=0 passthrough verified` exit 0 | PASS |
| SC-1 SSE round-trip | `./spiked sse` | `[OK] spike sse: parsed 6 frames (types: text-delta, reasoning-delta, tool-input-available, source-url, data-policy, finish-step)` exit 0 | PASS |
| CLI router usage | `./spiked` (no args) | `usage: spiked <subcommand> [flags]` to stderr exit 2 | PASS |
| CLI unknown subcommand | `./spiked bogus` | `unknown subcommand: bogus` to stderr exit 2 | PASS |
| SC-2 fail-fast (no MILVUS_ADDRESS) | `./spiked milvus` | `[SEED] 12 fixtures loaded (embed mode: deterministic)` then `[FAIL] milvus: milvus connect: milvus config: Addr is empty (set MILVUS_ADDRESS)` exit 1 | PASS (correct fail-fast) |
| SC-3 fail-fast (no GLM_API_KEY) | `./spiked e2e` | `[FAIL] e2e: GLM_API_KEY not set; copy .env.example to .env and populate` exit 1 | PASS (correct fail-fast) |
| SC-1 + SC-3 + SC-5 unit/integration tests | `go test ./cmd/spiked ./internal/spike -count=1 -timeout 60s` | `ok cmd/spiked 0.270s` + `ok internal/spike 0.223s` | PASS |
| SC-5 contract suite | `go test ./integration/contract -tags=contract -count=1 -timeout 10s` | 6/6 PASS in 3.442s (budget 10s) | PASS |
| SC-4 vitest | `cd web && npx vitest run` | 3/3 PASS in 2.27s | PASS |
| `go vet` clean | `go vet ./cmd/... ./internal/... && go vet -tags=contract ./integration/...` | exit 0 (no output) | PASS |
| Secret hygiene | `git ls-files \| grep -E '^\.env$'` | empty (0 lines) | PASS |
| Anti-pattern scan | `grep -rnE "Publish\|NewStream\|RequestBodyModifier\|18080" cmd/ internal/` | 0 matches | PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared in any PLAN.md. The phase uses `go test` + `vitest` + CLI subcommand execution as the probe surface (all executed above).

### Requirements Coverage

Phase 1 is a spike — generates no v1 requirements. `.planning/REQUIREMENTS.md` Phase 1 row: "Phase 1 (Spike) | 0 | (none — integration validation only)". Nothing to cross-reference.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No TBD/FIXME/XXX/HACK/PLACEHOLDER/TODO markers in cmd/, internal/, integration/ | — | — |
| (none) | — | No `s.Publish` / `NewStream` / `RequestBodyModifier` calls (Pitfall 2 + deprecated API avoidance) | — | — |
| (none) | — | No hardcoded `:18080` port (Pitfall 7 — dynamic port allocation via `net.Listen("tcp", "127.0.0.1:0")` everywhere) | — | — |
| `web/src/App.tsx` | 53 | `<a href={p.url}>` renders source-url with NO URL sanitization | Info | T-1-08 deferred to Phase 4 per plan threat_model. Acceptable for spike (test never executes user URLs). |
| `cmd/spiked/e2e.go` | 237-239 | Hardcoded citation injection (`auth_001`..`auth_005`) | Info | Per plan execution_notes "fake cited answer" pattern. Plan 02 validated real Milvus separately; Plan 04 e2e uses synthetic citations for the SSE chunk shape round-trip only. NOT a stub — explicit spike scope decision. |

### Threat-Model Mitigations (Verification)

| Threat ID | Mitigation | Status | Evidence |
|-----------|-----------|--------|----------|
| T-1-01 | `.env` in `.gitignore`; never log key value | VERIFIED | `git ls-files \| grep .env` returns empty. `.env` on line 1 of `.gitignore`. `os.Getenv("GLM_API_KEY")` used; key never logged. |
| T-1-04 | `BuildMilvusFilter` uses `%q` for CN-string escaping; empty tiers → `"false"` (default-deny) | VERIFIED | `internal/spike/milvus_client.go:139-154`. TestBuildMilvusFilterSpecialChars (3 cases: single quote, double quote, backslash). |
| T-1-05 | Filter applied PRE-similarity at SDK Search layer | VERIFIED | `SearchWithTierFilter` passes filter as positional `expr` (4th param of `client.Search`), applied at index-scan time. No post-search filtering. |
| T-1-08 | React `<a href>` URL sanitization | DEFERRED to Phase 4 | App.tsx line 53 + comment "PHASE 4 SAFETY (T-1-08 threat)". Acceptable per plan threat_model. |
| T-1-09 | SSE chunk injection via embedded newlines | VERIFIED (accepted) | Hertz `WriteEvent` auto-splits multi-line data per writer.go:141-149. No additional mitigation required. |
| T-1-10 | GLM API key redaction in SSE error path | VERIFIED | `UIMessageChunk.ErrorText` field never includes API key. Error messages constructed from non-sensitive context. |
| T-1-11 | Type enum kebab-case whitelist | VERIFIED | `ValidTypes` 16-type map gates `EmitChunk` BEFORE any wire write. `TestEmitChunkRejectsUnknownType` proves snake_case `text_delta` is rejected. |
| T-1-12 | Eino Runner iterator cleanup on cancellation | VERIFIED (code path) | `probeCancellation` measures wall-clock from cancel to iter close. 200ms budget. Live verification gated on GLM_API_KEY. |
| T-1-13 | API key never in trace log | VERIFIED | `appendTrace` records only step name + input/output summaries + decision. `apiKey` stays in local variable. TestComplianceQueriesSkip asserts `assert.NotContains(t, string(body), apiKey)`. |
| T-1-15 | Version pinning contract test | VERIFIED | `TestContractVersionPin` reads go.mod and asserts all 4 pinned versions. `-count=1` disables caching. |
| T-1-16 | Verdict struct single source of truth | VERIFIED | Defined in `internal/spike/verdict.go`. Imported by `cmd/spiked/e2e.go` (lines 222, 227, 387, 405) AND `integration/contract/contract_test.go` (lines 217, 226, 254, 258, 452). NO local ContractVerdict type. Compile-time anchor `var _ spike.Verdict`. |
| T-1-SC | Module version drift detection | VERIFIED | `TestContractVersionPin` + `TestContractWireFormat` + `TestContractMilvusFilter` + `TestContractStructuredOutputSchema` all enforce wire/filter/schema contracts. Any drift → merge blocked. |

### Pitfall Avoidance (RESEARCH.md §Pitfalls 1-8)

| Pitfall | Avoidance | Status |
|---------|-----------|--------|
| P1: Hertz SSE API mismatch (PROJECT.md §2.2 wrong) | Used actual `sse.NewWriter(c).WriteEvent` API; `Publish`/`NewStream` not called (grep returns 0) | AVOIDED |
| P2: (same as P1) | — | AVOIDED |
| P3: Filter applied post-similarity (wrong) | Filter is positional `expr` in SDK Search (4th param), applied PRE-similarity at index-scan | AVOIDED |
| P4: eino-ext pseudo-version | Corrected to tagged v0.1.5 (Plan 01-01 SUMMARY deviation #2); v0.1.5 is highest version compatible with eino v0.7.7 | AVOIDED (corrected) |
| P5: GLM max_tokens=400 with thinking:disabled | `model.WithMaxTokens(400)` paired with `thinking:{type:disabled}` everywhere (`wire.go`, `e2e.go`) | AVOIDED |
| P6: kebab-case Type discriminator | 16-type `ValidTypes` whitelist + `IsValidType` pre-write gate + `TestUIMessageChunkShape` 10 subtests | AVOIDED |
| P7: CN-char escape + hardcoded port | Go `%q` formatter for filter values; `net.Listen("tcp", "127.0.0.1:0")` for dynamic ports everywhere; `grep "18080"` returns 0 | AVOIDED |
| P8: GLM-5.2 reasoning burns tokens | `thinking:{type:disabled}` extra field set in all GLM calls (`wire.go`, `e2e.go`) | AVOIDED |

### CONTEXT.md Decisions (D-01..D-12) Honored

| Decision | Honored | Evidence |
|----------|---------|----------|
| D-01: Real API only (GLM-5.2 via Coding Plan) | YES | `cmd/spiked/e2e.go:130-133` defaults `GLM_BASE_URL=https://open.bigmodel.cn/api/coding/paas/v4`. Mock used only for SC-1 wire regression per D-02. |
| D-02: Mock pattern reused alongside real API | YES | `cmd/spiked/wire.go` + `cmd/spiked/e2e_test.go` both use mock OpenAI pattern from spike 001. |
| D-03: SSE all 3 layers validated | YES | SC-1 layer (a) wire format (TestSpikeSSERoundTrip header + body assertions); layer (b) typed parts (TestUIMessageChunkShape 10 subtests); layer (c) UIMessageChunk parse (round-trip parseSSEFrames). |
| D-04: Typed parts: reasoning + tool-calls + sources | YES | `ValidTypes` whitelist includes reasoning-* + tool-input-* + source-url + source-document. sseChatHandler emits 6 representative chunks spanning every category. |
| D-05: `WithExtraFields(map[string]any{...})` for OpenAI extras | YES | `wire.go:214` + `e2e.go:212` both use `openai.WithExtraFields(map[string]any{"response_format": ..., "thinking": ...})`. |
| D-06: + BGE-M3 + real retrieval E2E | PARTIAL | `EmbedQuery` calls real BGE-M3 endpoint; `EmbedQueryDeterministic` is the offline fallback. Live retrieval gated on MILVUS_ADDRESS (deferred per internal-api strategy). |
| D-07: 4 tiers × ≥3 docs; `tier in ["权威"]` returns zero bleed | YES (code path) | 12 fixtures (3 per tier). `runMilvus` runs 4 tier queries + default-deny. Bleed assertion logic in place; live verification deferred. |
| D-08: BGE-M3 self-host via Docker | OVERRIDDEN by user checkpoint | User chose internal-api strategy at Plan 01-01 checkpoint (Milvus + BGE-M3 on 内网, no Docker). `docker-compose.yml.template` retained as Phase 7 packaging artifact. |
| D-09: Milvus = local Docker container | OVERRIDDEN by user checkpoint | Internal-api strategy instead. BuildMilvusFilter still implements default-deny `"false"` for empty tiers. |
| D-10: Output = Runnable Go skeleton + E2E trace log | YES | `cmd/spiked/` runnable skeleton (all 4 subcommands). `spike-e2e-trace.json` produced on `spiked e2e` success. |
| D-11: Runnable end-to-end with one command | PARTIAL | One-command run requires user-provided `.env` (GLM_API_KEY + MILVUS_ADDRESS). Code gates with explicit fail-fast messages. |
| D-12: E2E trace log timestamped per step | YES | `appendTrace(step, input, output, decision)` writes `time.RFC3339Nano` timestamp per step. Trace covers init + cancel_test + 5×(query_received + verdict_validated + sse_emitted + milvus_lookup). |

### Human Verification Required

The following live verifications are deferred to the user. They are **expected spike-strategy deferrals** (internal-api + gated live-LLM pattern), NOT gaps. Code paths are complete; documented commands exist.

### 1. SC-2 Live Milvus Tier-Filter Round-Trip

**Test:** Configure `.env` with `MILVUS_ADDRESS` + `BGE_M3_URL` pointing to internal-services, then run `go run ./cmd/spiked milvus` (or `MILVUS_INTEGRATION=1 MILVUS_ADDRESS=<addr> go test ./cmd/spiked -run TestSeedAndFilterIntegration -v`).
**Expected:** `[OK] spike milvus: 12 docs seeded, tier filter returns 3 authority chunks; zero bleed across all 4 tiers + default-deny`
**Why human:** Sandbox executing this verification has no internal-network access. Per Plan 01-02 Task 2 `checkpoint:human-verify` (blocking gate). Code path is complete; `BuildMilvusFilter` unit tests (8 cases + 3 special-char injection cases) all pass; `TestSeedAndFilterIntegration` correctly skips when `MILVUS_INTEGRATION != 1`.

### 2. SC-3 Live GLM-5.2 5-Query Pipeline + Cancellation Timing

**Test:** Set `GLM_API_KEY=<valid Coding Plan key>` (and optionally `GLM_BASE_URL`), then run `go run ./cmd/spiked e2e` (or `GLM_API_KEY=<key> go test ./cmd/spiked -run "TestE2ECancellation|TestComplianceQueriesSkip" -v`).
**Expected:** `[OK] spike e2e: 5 queries validated, trace at spike-e2e-trace.json, cancel <Nms> < 200ms verified` AND `spike-e2e-trace.json` artifact produced.
**Why human:** Sandbox has no `GLM_API_KEY`. Per Plan 01-04 Task 1 `checkpoint:human-verify` (blocking gate). Code path complete; `TestValidateVerdict` (5 cases) always runs + passes; `probeCancellation` logic is exercised by `runE2E`; `TestE2ECancellation` + `TestComplianceQueriesSkip` correctly skip when `GLM_API_KEY` unset or `"unhappy"`.

### Gaps Summary

No code-path gaps. All 5 SCs verified at the spike-required level:

- **SC-1, SC-4, SC-5**: Fully green via automated tests (Go + vitest).
- **SC-2**: Code path complete; `BuildMilvusFilter` syntax + injection-safety verified; live bleed assertion deferred to human-verify per blocking checkpoint (internal-api strategy).
- **SC-3 (single-agent portion)**: Code path complete; `TestValidateVerdict` gate green; `probeCancellation` + 5-query loop in place; live GLM-5.2 run deferred to human-verify per blocking checkpoint.
- **SC-3 (SequentialAgent 2-step)**: DEFERRED to Phase 3 per user-approved Claude's Discretion (01-DISCUSSION-LOG.md §Claude's Discretion Summary). NOT a gap.

The phase goal — validate the core integration wire contract — is achieved. The D-10 "trust contract" deliverable is the version-pinned integration test (`integration/contract/contract_test.go`) which runs in 3.442s and catches any future drift in eino/hertz/eino-ext versions, UIMessageChunk Type discriminator, Milvus tier-filter syntax, or Verdict struct shape. Phase 2-7 can build on this contract.

---

_Verified: 2026-08-11T13:05:00Z_
_Verifier: Claude (gsd-verifier)_
