---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 05
subsystem: skill-platform
tags: [inspector, knowledge-graph, sc-4, sc-5, sc-8, version-badges, filtered-out-notice]
requires: [05-01, 05-02, 05-03]
provides: [agent-inspector-tree, knowledge-graph-endpoint, knowledge-graph-browser, version-badge-ui, filtered-out-notice-ui]
---

# Plan 05-05 — SC-4 Agent Inspector + SC-5 Knowledge Graph Browser + SC-8 Version Filtering UI

## Objective

Fill in the SC-4 Agent State Inspector + SC-5 Knowledge Graph Browser placeholders from 05-03 and extend the ChunkSourceView with the SC-8 version badges + effective_* metadata + filtered-out notice. The Inspector is a derived view of useChat.messages (no Zustand state per D14). The Graph Browser consumes a new GET /api/knowledge/graph endpoint backed by retriever metadata. The ChunkSourceView gains the 当前有效 / 已失效 badge + effective_from / effective_to rows + filtered-out notice for superseded chunks navigated via the graph.

## What shipped

### Backend (Go)

| File | Provides |
|------|----------|
| `internal/knowledge/graph.go` | `GraphBuilder.Build(ctx)` → `(*Graph, error)` — reads retriever metadata via the new `ListMetadata` method (NO chunk content surfaced — threat T-5-22 mitigation). Groups by tier → source → chunks in deterministic `skill.TierEnumValues()` order. Always emits 4 tiers (权威 / 历史 / 个人 / 草稿) even when the corpus is empty. `versionFromEffective` produces human-readable labels (自 YYYY-MM-DD / YYYY-MM-DD 起失效). |
| `internal/knowledge/graph_test.go` | 8 tests covering tier structure, effective fields round-trip, empty corpus, deterministic order, nil retriever, error propagation, unknown tier rejection, empty source rejection, source grouping. |
| `internal/server/knowledge.go` | `graphHandler` returns 503 (nil builder) / 500 (build error) / 200 (success) envelopes. Route: GET /api/knowledge/graph. |
| `internal/server/knowledge_test.go` | 4 tests using `ut.ResponseRecorder` (Code field + Body.Bytes()). |
| `internal/server/server.go` | `KnowledgeBuilder` interface (`Build(ctx) (*knowledge.Graph, error)`) + `Knowledge` field on `Deps` + the new route registered in `New()`. |
| `internal/retriever/retriever.go` | `DocumentMeta` type + `ListMetadata(ctx) ([]DocumentMeta, error)` method on the `Retriever` interface — metadata-only projection (id / tier / source / effective_from / effective_to). No chunk content. |
| `internal/retriever/milvus/client.go` | `ListMetadata` implementation using `m.mc.Query()` returning `ResultSet` (`[]entity.Column`). Iterates columns by `Name()` and uses `GetAsInt64` / `GetAsString` to extract values. |
| `internal/retriever/mongo/mongo.go` | `ListMetadata` reusing `c.finder.FindRows(ctx, bson.D{})` (empty filter matches all). Returns metadata-only projection; deliberately surfaces superseded chunks (graph is for browse). |
| `internal/server/testhelpers_test.go` + `internal/agent/agent_test.go` | `ListMetadata` method added to `stubRetriever` to satisfy the interface. |
| `cmd/server/main.go` | Wires `Knowledge: knowledge.New(r)` into Deps. |

### Frontend (TypeScript / React 19)

| File | Provides |
|------|----------|
| `web/src/types/knowledge.ts` | TS mirror of Go Graph / TierNode / DocNode / ChunkNode + `isChunkActive(effectiveTo, nowMs)` helper. Threat T-5-22: ChunkNode carries NO content field. |
| `web/src/hooks/useKnowledgeGraph.ts` | Fetch + memoize; one-shot on mount; `refetch()` handle. 503 → friendly error. No subscription to skill-reload events (D5-09: knowledge graph stable across Skill reloads). |
| `web/src/lib/format-effective.ts` | `formatEffectiveDate(epochSec)` returning YYYY-MM-DD UTC or 未知. |
| `web/src/components/center/VersionBadge.tsx` | 当前有效 (BadgeCheck green) when effectiveTo is null/0/future; 已失效 (strikethrough --color-text-dim) when superseded. `data-status` mirrors the state for downstream test queries. |
| `web/src/components/center/FilteredOutNotice.tsx` | Banner rendered ABOVE ChunkSourceView body when navigating to a superseded chunk via the graph browser: 此片段已失效（{effectiveTo} 起），不会出现在检索结果中。仅可浏览。 Defensive null/0 guard keeps the component standalone-safe. |
| `web/src/components/center/ChunkSourceView.tsx` | Metadata row extended with VersionBadge + 生效日期 (always) + 失效日期 (only when effective_to is set). FilteredOutNotice mounted ABOVE the body when the chunk is superseded. |
| `web/src/components/graph/KnowledgeGraphBrowser.tsx` | Replaced 05-03 placeholder with full implementation: error / loading / empty / ready branches; GraphFilterInput + GraphTree in ready state. Error state has retry button. |
| `web/src/components/graph/GraphTree.tsx` | 3-level `<details>/<summary>` Accordion: tier `{tier}（{docCount} 文档 / {chunkCount} 片段）`, doc `{filename} · {chunkCount} 片段`, chunk rows. Tier text in --color-primary (NOT teal — teal reserved for SC-6 counter-evidence). Filter narrows visible docs. |
| `web/src/components/graph/GraphChunkNode.tsx` | Button row with chunkId + VersionBadge; click dispatches `setCenterView({kind:'chunk-source', chunkId, tier, version, effectiveDate, effectiveTo})`. |
| `web/src/components/graph/GraphFilterInput.tsx` | Search-style input with Search icon, placeholder 按文档名 / chunk ID 过滤. |
| `web/src/components/inspector/AgentInspector.tsx` | Replaced 05-03 placeholder with full Accordion tree. Walks `useChat.messages` (latest assistant turn), groups parts into hops using `data-decision` DecisionPayload.step. Empty / loading / hop tree branches. |
| `web/src/components/inspector/InspectorHop.tsx` | Per-hop Accordion item: 决策 row + 工具调用 list + 检索结果 row + 推理 row (collapsed). Active hop = `border-l-4 border-l-primary`. Counter-evidence hop = teal 历史层反证已检索 badge. |
| `web/src/components/inspector/InspectorToolCall.tsx` | Per-tool-call expandable. Renders args + output as JSON-stringified text in `<pre><code>`. NEVER `dangerouslySetInnerHTML` (T-5-21). |
| `web/src/components/inspector/InspectorRetrieval.tsx` | Chunk ID list; click dispatches `setCenterView({kind:'chunk-source', chunkId, tier})` — reuses the same dispatcher as the CitationCluster. |
| `web/src/components/inspector/hop-types.ts` | Derived `Hop` / `ToolCallRow` / `RetrievalRow` types built from useChat parts. |
| `web/src/components/center/CenterPanel.tsx` | Adds `messages` + `chatStatus` props; forwards to AgentInspector. D14 honored: messages stay out of Zustand. |
| `web/src/components/center/CitationCluster.tsx` | Passes `effectiveTo: null` through to setCenterView (verdict.citations chunks are always active per retriever PRE-filter). |
| `web/src/store/app-store.ts` | Adds `effectiveTo: number | null` to chunk-source CenterView kind for supersession signaling. |
| `web/src/App.tsx` | Passes `messages` + `chatStatus` (from `useChat`) to CenterPanel as props. |

### Tests

| File | Coverage |
|------|----------|
| `web/src/__tests__/KnowledgeGraphBrowser.test.tsx` | Expanded from 3 placeholder assertions to 6: empty corpus hero, loading override, error state + retry, 3-level tree rendering, filter narrows by filename, chunk click dispatches setCenterView. |
| `web/src/__tests__/ChunkSourceView.test.tsx` | 12 assertions: original 5 + 7 new SC-8 cases (badge states, 失效日期 row conditional, FilteredOutNotice mount on superseded navigation). |
| `web/src/__tests__/AgentInspector.test.tsx` | Expanded from 3 placeholder assertions to 8: empty / loading / multi-hop grouping + active hop / 2px accent left-border / counter-evidence badge / tool-call expansion (no dangerouslySetInnerHTML) / reasoning collapsed / retrieval click dispatch. |
| `web/src/__tests__/VersionBadge.test.tsx` | 5 cases: perpetual null/0/future, superseded past, null-from edge. |
| `web/src/__tests__/FilteredOutNotice.test.tsx` | 2 cases: renders banner copy; suppresses on null/undefined effectiveTo. |
| `web/src/__tests__/CenterPanel.test.tsx` | knowledge-graph empty-state test mock-fetches the empty corpus (was: relied on the 05-03 placeholder). |

## Verification

```bash
$ go test ./internal/knowledge/... ./internal/server/... ./internal/retriever/... -count=1
ok      github.com/skillforge/skillforge/internal/knowledge    0.289s
ok      github.com/skillforge/skillforge/internal/server       0.738s
?       github.com/skillforge/skillforge/internal/retriever    [no test files]
ok      github.com/skillforge/skillforge/internal/retriever/milvus     0.159s
ok      github.com/skillforge/skillforge/internal/retriever/mongo      0.156s

$ cd web && npx tsc --noEmit
# (no output)

$ cd web && npx vitest run --reporter=basic
Test Files  27 passed (27)
     Tests  172 passed (172)
```

All automated checks green. SPEC §verification checklist:

- [x] `go test ./internal/knowledge/... ./internal/server/... -count=1` exits 0
- [x] `cd web && npx vitest run` exits 0
- [x] `cd web && npx tsc --noEmit` exits 0
- [x] GraphBuilder test covers tier structure + effective fields
- [x] AgentInspector walks useChat.messages via props (NO Zustand state for messages — D14)
- [x] No `dangerouslySetInnerHTML` in InspectorToolCall or InspectorRetrieval (grep confirms only comment-level mention of the mitigation)
- [x] VersionBadge + FilteredOutNotice test files exist + green
- [x] ChunkSourceView metadata row includes the new fields

## Acceptance Criteria — Results

| Criterion | Result |
|-----------|--------|
| GET /api/knowledge/graph serves the 4-tier tree with effective_* fields | PASS — TestGraphBuilder_TierStructure + TestGraphBuilder_EffectiveFields; graphHandler 200/500/503 envelopes |
| KnowledgeGraphBrowser renders 3-level Accordion with search filter + version badges; empty / loading / error states | PASS — 6-test suite covers all branches + filter + click dispatch |
| AgentInspector walks useChat.messages and groups parts by hop (test asserts hop count) | PASS — deriveHops() in AgentInspector; multi-hop test asserts 2 hops with correct data-decision values |
| Active hop has 2px left-border in --color-primary (test asserts style/class) | PASS — inspector-hop[data-active="true"] carries border-l-4 border-l-primary classes |
| Counter-evidence hop badge renders on the SC-6 hop (decision='counter_evidence') | PASS — inspector-hop[data-counter-evidence="true"] surfaces 历史层反证已检索 badge |
| Tool call expansion reveals args + output (test) | PASS — inspector-tool-call-args + inspector-tool-call-output test ids; args rendered as JSON.stringify text (no HTML injection) |
| Reasoning row is collapsed by default (test asserts collapsed state) | PASS — inspector-hop-reasoning <details> has no `open` attribute until summary click |
| ChunkSourceView metadata row includes version badge + 生效日期 + (conditional) 失效日期 | PASS — chunk-version + chunk-effective-date + chunk-effective-to-date test ids; 失效日期 row conditional on effectiveTo > 0 |
| FilteredOutNotice renders when a superseded chunk is navigated (test) | PASS — filtered-out-notice test id asserts banner copy + 2023-03-01 date |
| VersionBadge shows 当前有效 when active; 已失效 when superseded | PASS — 5-case test suite covers null/0/future/past/null-from |
| messages stays banned from Zustand (D14) | PASS — `grep messages web/src/store/app-store.ts` returns only the doc-comment ban note; messages flow via React props (App → CenterPanel → AgentInspector) |

## Threat-Model Disposition

| Threat | Mitigation | Test |
|--------|------------|------|
| T-5-21 Inspector renders tool-call args via dangerouslySetInnerHTML | InspectorToolCall renders args as text via React's default text-node rendering. JSON args stringified with indentation via `safeStringify`. NEVER inject HTML. | Grep: only the mitigation comment mentions `dangerouslySetInnerHTML`; test asserts args textContent contains the input |
| T-5-22 Graph browser loads full chunk text into the browser via the metadata endpoint | Graph endpoint returns metadata only (id / filename / effective_* / version). Chunk verbatim opens separately in ChunkSourceView. `types/knowledge.ts` ChunkNode carries no content field. | TestGraphBuilder_* assertions check no content fields; types file inspection |
| T-5-23 Version badge reveals the effective_to date (could leak internal policy timing) | Accept — effective_to is operator-authored metadata per KNOW-02 + indexer validation. Surfacing it is the explicit SC-8 requirement. (No additional gating.) | (acceptance — no test) |
| T-5-24 Inspector hop border could be applied to ANY hop (visual lie) | Active hop detection uses the actual in-flight state (`status === 'streaming'` AND the hop is the last one). Border is a UI hint, not a security boundary. | InspectorHop border assertion test asserts the LAST hop with streaming carries the accent border; earlier hops carry border-l-transparent |

## Atomic Commit Trail

```
d86d5bf feat(05-05): SC-4 Agent Inspector full Accordion tree (Task 2)
e839663 feat(05-05): SC-5 graph browser + SC-8 version badges + filtered-out notice
```

## Notes for Future Work

- `useKnowledgeGraph` is a one-shot fetch on mount + `refetch()` handle. No polling. The plan defers auto-refresh on skill-reload events (D5-09 says reload scope is Skill definition only — the graph is stable). If a future plan wires knowledge-graph invalidation, the hook is the single integration point.
- `deriveHops()` in `AgentInspector` walks parts in array order and groups by `data-decision` step. Leading non-decision parts (e.g. initial `start` chunks) are skipped until the first decision arrives. Multi-turn assistant messages are not collapsed — the inspector reflects the latest assistant turn only.
- `InspectorHop` uses native `<details>/<summary>` for the Accordion (consistent with `GraphTree`) — Phase 5 deliberately avoids a heavy Radix Accordion dependency for the 3-level browse surface per CONTEXT §Claude's Discretion.
- `InspectorRetrieval` defaults the chunk tier to 权威 when dispatching setCenterView (the wire source-document part doesn't carry tier metadata). If future Plans tag retrieved chunks with their tier, the dispatcher should be updated to pass the real tier through.
- `FilteredOutNotice` has a defensive null/0 guard (returns null) in addition to the ChunkSourceView mount-time `isSuperseded` check. The double-guard keeps the component standalone-safe for tests + future surfaces.
- `CenterPanel` accepts `messages` + `chatStatus` as optional props (defaults: `[]` and `'ready'`) — this preserves backward compatibility with any caller that doesn't supply them, and means the empty-state hero still renders for test setups that don't seed messages.
- The graph endpoint deliberately surfaces superseded chunks (no `effective_to` filter) — the graph is a browse surface, not a retrieval surface. The retriever PRE-filter (D5-15) is what guarantees verdict.citations never includes superseded chunks.
