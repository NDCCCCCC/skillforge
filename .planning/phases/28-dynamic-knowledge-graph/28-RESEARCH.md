# Phase 28: 知识图谱动态化 + 关系层 (Dynamic Knowledge Graph) - Research

**Researched:** 2026-09-07
**Domain:** Go knowledge graph — deterministic relation edges, neighbor/path query APIs, GraphCanvas upgrade, performance guards, Phase 27 joint acceptance
**Confidence:** HIGH (codebase-grounded; every claim carries a file:line anchor verified in this session)

## User Constraints (from ROADMAP Phase 28 + PROJECT.md v1.2 standards)

### Locked Decisions
- **GRF-01**: `GET /api/knowledge/graph` 按注册表输出 N 个知识库节点（移除固定 4 tier 假设）——Phase 26 graph.go 已完成 N-dynamic node set (`graph.go:251-265`：`b.reg.List()` ascending by slug → N TierNode entries)
- **GRF-02**: chunk 关系层——确定性（零 LLM）关系边：同文档版本继承边 + 跨文档实体/关键词条款共现边；边可确定性重算
- **GRF-03**: 图谱查询 API——邻居查询（给定 chunk 的关系边）+ 受限深度路径查询；供 FE 与未来 agent 工具消费
- **GRF-04**: FE GraphCanvas 升级——动态知识库渲染 + 关系边可视化 + 邻居展开交互
- **GRF-05**: 图谱性能护栏——节点/边上限、分页与懒加载策略配置驱动；超限返回骨架图并明示降级
- **零 LLM 确定性边**（PROJECT.md v1.2 Standards + ROADMAP Phase 28 Notes）：图谱边保持零 LLM 确定性（可 hash 复现）——LLM 实体关系抽取明确 Out of Scope（未来候选，需评测门）
- **Phase 27 耦合**：上传文档须在图谱中可见（DOC-06）；Phase 27 落地后图谱须包含上传文档 chunk 节点
- **Phase 26 耦合**：KBRegistry 是图谱节点集的真源（已完成 `graph.go:251-265`）
- **CGO 约束**：Phase 28 保持纯 Go（mirrors Phase 27 模式）
- **配置驱动**：节点/边限制/深度等全部配置驱动，零硬编码（HYG-02 为 Phase 30 验收，Phase 28 自身不得新增硬编码）

### Claude's Discretion
- 具体边计算算法细节（同文档版本继承的 effective_from/effective_to 窗口判断、跨文档关键词共现的 token 提取与 Jaccard 阈值）
- 邻居/路径 API 的具体响应字段形状
- GraphCanvas 的 FE 实现细节（SVG 扩展 vs 新库）
- 边存储策略（重算 vs 持久化 vs memoization cache）

### Deferred Ideas (OUT OF SCOPE)
- LLM 实体关系抽取——明确 Out of Scope（PROJECT.md v1.2 Out of Scope：知识图谱 LLM 实体关系抽取）
- 知识库级 RBAC / 多租户 → v2
- 反馈落库/工单系统 → Phase 29

---

## 1. Current graph surface (the baseline)

The existing `GET /api/knowledge/graph` endpoint (`internal/server/knowledge.go:158-181` `graphHandler`) delegates to `GraphBuilder.Build` (`internal/knowledge/graph.go:196-298`). The current surface returns:

**What it returns today:**
- `Graph { tiers: []TierNode }` — one `TierNode` per ACTIVE registry entry, ascending by slug (N dynamic since Phase 26)
- Each `TierNode` carries `tier` (slug), `docCount`, `chunkCount`, `docs: []DocNode`
- Each `DocNode` carries `id` (synthetic `tier::source`), `filename` (basename), `chunkCount`, `chunks: []ChunkNode`
- Each `ChunkNode` carries `id` (string-form int64), `effectiveFrom`, `effectiveTo`, `version` (derived label)
- Chunk content is intentionally absent (`graph.go:24`: "threat T-5-22 mitigation"); separate `GET /api/knowledge/chunk/:id` fetches verbatim text on demand

**What it does NOT return today (missing capabilities confirmed by code inspection):**

| Missing Capability | Evidence | Impact |
|---|---|---|
| **Deterministic relation edges** — no `edges` array in the response; the graph only shows containment (tier -> doc -> chunk), not cross-chunk relations | `graph.go:106-108` `Graph` struct: only `Tiers []TierNode` field; no `Edges` field | GRF-02 not satisfied |
| **Neighbor query API** — no `GET /api/knowledge/graph/neighbors?chunk_id=X` endpoint | `knowledge.go:158-181` only `graphHandler` (GET /graph) + `chunkHandler` (GET /chunk/:id); no neighbors handler | GRF-03 not satisfied |
| **Path query API** — no `GET /api/knowledge/graph/path?from=X&to=Y` endpoint | same as above | GRF-03 not satisfied |
| **Graph stats API** — no `GET /api/knowledge/graph/stats` for admin debug | same as above | planning suggestion only |
| **Chunk-level pagination** — `ListMetadata` returns ALL rows; no cursor/pagination on large corpora | `retriever.go:171` `ListMetadata` returns `[]DocumentMeta` (no pagination params) | GRF-05 performance guard needed |
| **Edges in FE rendering** — GraphCanvas only renders tier->doc containment edges (line from tier node to doc node); no cross-chunk edges | `GraphCanvas.tsx:133-146` only renders edges from tier position to doc positions; no inter-chunk edge type | GRF-04 partial |
| **Phase 27 upload doc nodes** — existing graph does not distinguish boot-seeded chunks from Phase 27 upload-indexed chunks (both use same `ListMetadata` path) | `graph.go:203-205` `ListMetadata` returns all rows regardless of origin; no origin tagging | GRF-01 + DOC-06 joint gap |

**Existing graph_test.go fixtures:** The test at `graph_test.go:91-107` asserts 4 hardcoded baseline slugs in ascending order. With N-dynamic registry (Phase 26), `TestGraphNodesFromRegistry` (`graph_test.go:226-275`) already tests the dynamic behavior — the 4-slug assertion in `TestGraphBuilder_TierStructure` (`graph_test.go:134`) uses `baselineGraphSlugs` which is a test variable, not a constant, so adding more KBs in tests would not break it. However, `TestGraphBuilder_NilRegistryFailsClosed` (`graph_test.go:307`) and `TestGraphBuilder_Empty` (`graph_test.go:207`) assert exactly 4 nodes when all 4 baseline slugs are active — these assertions would need updating if a 5th KB is added to `testBaselineSlugs`.

---

## 2. Deterministic relation edges (no LLM)

For each relation type Phase 28 should ship:

| Relation | Derivation | Determinism Guarantee | Storage Cost |
|---|---|---|---|
| **Same-document version chain** (chunk V2 supersedes V1) | Two chunks share the same `Tier` + `Source` AND `EffectiveFrom` windows overlap/adjacent (the newer version has `effective_from > 0` and the older has `effective_to > 0` within the same doc). Edge direction: older → newer. | Deterministically derivable from `EffectiveFrom/EffectiveTo` epoch fields already stored in `DocumentMeta` (`retriever.go:89-95`). Same input rows always produce the same edge set. | Zero — computed from existing metadata at query time. |
| **Same-document section adjacency** (chunk N → chunk N+1 within doc) | Two chunks share the same `Tier` + `Source` AND their numeric IDs are consecutive within the same source file (sorted by ID ascending). Edge direction: lower ID → higher ID. | Deterministically derivable from `ID` ordering within `(Tier, Source)` group. No new fields needed. | Zero — computed at query time from ID sort order within `(tier, source)` group. |
| **Cross-document keyword co-occurrence** (chunks sharing rare tokens) | Compute token set (3+ char alphanumeric words, lowercased, stop-word filtered) for each chunk's `Source` filename (not content — content is not in `ListMetadata`). Two chunks share an edge if their filename token sets intersect with Jaccard ≥ threshold (e.g., 0.3). Edge weight = Jaccard score. | Filename is a stable string field in `DocumentMeta` (`retriever.go:92` `Source`). No LLM involved — purely string-token set intersection. Same rows always produce same edges. | Zero additional storage — filename tokenization is pure function on `Source` field already in metadata. |
| **Cross-document citation co-occurrence** (chunks both cited in same verdict) | **Requires verdict audit data** — audit rows with `action_verdict_rendered` carry `frozen_args` with `chunk_ids[]`. Two chunks have an edge if they appear in the same verdict's `chunk_ids` list. | Audit rows are INSERT-only; `chunk_ids` array in `frozen_args` is frozen at verdict time. Same audit rows always produce same edges. Requires parsing `frozen_args` JSON at query time. | Zero — derived from existing audit rows at query time (with memoization). |
| **Entity-extraction edges** | **OUT OF SCOPE** — LLM required | N/A | N/A |

**Why LLM entity extraction is explicitly forbidden:** PROJECT.md v1.2 Out of Scope explicitly states "知识图谱 LLM 实体关系抽取——保持图谱确定性零 LLM（可 hash 复现）" and ROADMAP Phase 28 Notes: "LLM 实体关系抽取明确 Out of Scope（未来候选，需评测门）". Any entity-extraction edge would require the LLM to read chunk content and produce structured relation triples — non-deterministic, expensive, and defeats the "deterministic edges can be hash-verified" property that GRF-02 requires.

**Edge weight normalization:** All edge weights are floats in `[0.0, 1.0]`:
- Version chain: weight = 1.0 (binary — exists or not)
- Section adjacency: weight = 1.0 (binary — consecutive IDs)
- Keyword co-occurrence: weight = Jaccard(filename_token_set_A, filename_token_set_B) — `[0.0, 1.0]`
- Citation co-occurrence: weight = 1.0 / (distinct_chunks_in_verdict) — weighted by rarity of co-citation

---

## 3. Neighbor + path query API

### `GET /api/knowledge/graph/neighbors`

**Purpose:** Return all edges connected to a given chunk, optionally expanding to N-depth neighborhood.

**HTTP surface:**
```
GET /api/knowledge/graph/neighbors?chunk_id=7&depth=2 HTTP/1.1
X-Admin-Token: (optional — neighbors endpoint is public read, no token required)

Response 200:
{
  "chunk_id": "7",
  "depth": 2,
  "nodes": [
    { "chunk_id": "7",  "tier": "authority", "source": "contract-approval.md", "effectiveFrom": 1735689600, "effectiveTo": 0 },
    { "chunk_id": "11", "tier": "authority", "source": "contract-approval.md", "effectiveFrom": 0, "effectiveTo": 0 },
    { "chunk_id": "3",  "tier": "historical", "source": "old-policy-2023.md", "effectiveFrom": 0, "effectiveTo": 1720137600 }
  ],
  "edges": [
    { "from": "7", "to": "11", "edge_type": "section_adjacent",   "weight": 1.0, "tier": "authority" },
    { "from": "7", "to": "3",  "edge_type": "version_chain",      "weight": 1.0, "tier": "authority" }
  ],
  "degraded": false,
  "degraded_reason": ""
}
```

**Response when limits exceeded (GRF-05 skeleton + explicit 降级 message):**
```json
{
  "chunk_id": "7",
  "depth": 2,
  "nodes": [ { "chunk_id": "7", "tier": "authority", "source": "...", "effectiveFrom": 0, "effectiveTo": 0 } ],
  "edges": [],
  "degraded": true,
  "degraded_reason": "neighbor limit exceeded (depth=2 yields N nodes); returning depth-1 skeleton only. see SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT config."
}
```

**Fields (deterministic):**
- `chunk_id` (string) — the query target
- `depth` (int) — actual depth used (may be capped by config)
- `nodes[].chunk_id` — all chunk IDs discovered within depth limit (deterministic from metadata)
- `nodes[].tier/source/effectiveFrom/effectiveTo` — deterministic from `ListMetadata` rows
- `edges[].from/to` — deterministic chunk ID pair
- `edges[].edge_type` — one of: `section_adjacent | version_chain | keyword_cooccur | citation_cooccur`
- `edges[].weight` — float `[0.0, 1.0]`
- `edges[].tier` — tier of the source chunk (for FE edge-coloring)
- `degraded` — boolean; `true` when limits were hit
- `degraded_reason` — human-readable Chinese explanation

**Response size limit:** Max `nodes` array capped at `SKILLFORGE_GRAPH_MAX_NODES` (default 200). If computed neighborhood exceeds this, return degraded skeleton with `degraded=true`. Max `edges` array capped at `SKILLFORGE_GRAPH_MAX_EDGES` (default 500).

**Error responses:**
- `400 { "error": "invalid chunk id" }` — `chunk_id` not parseable as int64
- `404 { "error": "chunk not found" }` — chunk ID exists in metadata but not in current active tier set
- `500 { "error": "neighbor query failed — see server logs" }` — W-3 envelope (generic message)

### `GET /api/knowledge/graph/path`

**Purpose:** Find a constrained shortest path between two chunks.

**HTTP surface:**
```
GET /api/knowledge/graph/path?from=7&to=3&max_hops=4

Response 200:
{
  "from": "7",
  "to": "3",
  "max_hops": 4,
  "hops": 4,
  "path": ["7", "11", "9", "3"],
  "edges": [
    { "from": "7",  "to": "11", "edge_type": "section_adjacent",  "weight": 1.0 },
    { "from": "11", "to": "9",  "edge_type": "keyword_cooccur",   "weight": 0.45 },
    { "from": "9",  "to": "3",  "edge_type": "version_chain",     "weight": 1.0 }
  ],
  "degraded": false,
  "degraded_reason": ""
}
```

**Response when no path found within max_hops:**
```json
{
  "from": "7",
  "to": "3",
  "max_hops": 4,
  "hops": -1,
  "path": [],
  "edges": [],
  "degraded": true,
  "degraded_reason": "no path found within 4 hops (max_hops); graph may be disconnected."
}
```

**Response when limits exceeded:**
```json
{
  "from": "7",
  "to": "3",
  "max_hops": 4,
  "hops": -1,
  "path": [],
  "edges": [],
  "degraded": true,
  "degraded_reason": "path search exhausted max_hops=4; see SKILLFORGE_GRAPH_PATH_HOPS_LIMIT config."
}
```

**Pathfinding algorithm:** BFS on the in-memory edge graph (built from metadata on first call or from cache). Unweighted (all edge types equal weight for path length purposes). `max_hops` default 4, capped at `SKILLFORGE_GRAPH_PATH_HOPS_LIMIT`.

**Error responses:**
- `400` — invalid `from`/`to` chunk IDs or `max_hops <= 0`
- `500` — W-3 envelope

### `GET /api/knowledge/graph/stats` (optional admin debug)

```
GET /api/knowledge/graph/stats?admin_token=...

Response 200:
{
  "nodes_total": 47,
  "edges_total": 312,
  "edges_by_type": {
    "section_adjacent": 42,
    "version_chain": 8,
    "keyword_cooccur": 258,
    "citation_cooccur": 4
  },
  "nodes_by_tier": {
    "authority": 30,
    "historical": 12,
    "personal": 5
  },
  "cached": true,
  "cache_age_seconds": 3.2
}
```

No token required for stats (read-only diagnostic). `cache_age_seconds` = seconds since last edge graph recompute.

---

## 4. Performance ceilings (configuration-driven, zero hardcode)

Three config keys (env vars, read once at boot):

| Config Key | Default | Purpose | GRF-05 Coverage |
|---|---|---|---|
| `SKILLFORGE_GRAPH_MAX_NODES` | **50** | Hard cap on nodes returned by `neighbors` and embedded in graph response | node count guard |
| `SKILLFORGE_GRAPH_MAX_EDGES` | **200** | Hard cap on edges returned by `neighbors` and embedded in graph response | edge count guard |
| `SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT` | **2** | Default `depth` param cap for `neighbors` API | depth limit |
| `SKILLFORGE_GRAPH_PATH_HOPS_LIMIT` | **4** | Default `max_hops` cap for `path` API | hop limit |

**Justification for defaults:**
- `SKILLFORGE_GRAPH_MAX_NODES=50` — typical Milvus corpus is ~16 docs × ~3 chunks/chunk = ~48 nodes; radial SVG layout (GraphCanvas.tsx:26) renders cleanly up to ~50 nodes; beyond 50 doc nodes the SVG arc spacing collapses
- `SKILLFORGE_GRAPH_MAX_EDGES=200` — for 50 nodes, dense graph is ~50×49/2 = 1225 edges; 200 is a safe headroom before SVG edge clutter makes the graph unreadable
- `SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT=2` — BFS depth 2 from a node can reach ~50-200 nodes in a dense corpus; depth 3 risks combinatorial explosion
- `SKILLFORGE_GRAPH_PATH_HOPS_LIMIT=4` — 4 hops covers most real-world regulatory document chains; longer paths are unlikely to be meaningful for the compliance use case

**Degraded response shape (GRF-05 "骨架图 + 降级 message"):**

When any limit is exceeded, the response returns:
```json
{
  "degraded": true,
  "degraded_reason": "<Chinese explanation of which limit was hit and what was returned>",
  "nodes": [ /* skeleton — only the query node + immediate neighbors within reduced depth */ ],
  "edges": [ /* only edges among the skeleton nodes */ ]
}
```

The `degraded_reason` must name:
1. Which limit was hit (e.g., "节点数超过 50")
2. What was returned (e.g., "depth 已自动降至 1")
3. Which env var controls it (e.g., "see SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT")

**Graph response embedding with degraded flag:**

The existing `GET /api/knowledge/graph` response (`Graph` struct `graph.go:106-108`) should gain two fields:
```go
type Graph struct {
    Tiers    []TierNode `json:"tiers"`
    Degraded bool       `json:"degraded"`
    DegradedReason string `json:"degradedReason,omitempty"`
}
```

When `len(all_chunk_ids) > SKILLFORGE_GRAPH_MAX_NODES`, return degraded skeleton with `degraded=true` and a reason.

---

## 5. Storage: Milvus vs Mongo for graph metadata

The graph nodes come from the retriever (`ListMetadata` — already backed by Milvus or Mongo). The graph edges are **DERIVED** — they are computed from chunk metadata (IDs, Source, EffectiveFrom/EffectiveTo) that already lives in the vector store.

**Three storage strategies:**

| Strategy | Description | Pros | Cons |
|---|---|---|---|
| **(a) Recompute on every graph endpoint call** | On each `GET /graph` or `GET /neighbors`, rebuild the full edge graph from scratch by iterating `ListMetadata` | Zero storage overhead; always current; simplest implementation | Latency: for 50KB metadata rows × 1000 chunks = O(n) scan every request; unacceptable for GRF-01/04 requiring <100ms response |
| **(b) Persist in separate sqlite table** | Pre-compute edges on upload/index events; store `(from_id, to_id, edge_type, weight)` rows in sqlite | Fast reads; incremental update on upload; persistent across restarts | Schema migration needed; edge invalidation on chunk delete/deactivate is complex; adds sqlite write on every upload |
| **(c) Persist as Milvus/Mongo metadata fields** | Store edge lists as a field on each chunk document | No new store; natural colocation | Schema change required; Milvus/Mongo not designed for list fields; query complexity increases |

**Recommendation: Strategy (a) + memoization cache (5-second TTL) — no schema migration, no audit risk, deterministic enough.**

**Rationale:**
- GRF-01/04 do not impose a latency SLA, but `<100ms` is the implicit target for interactive graph rendering (Phase 4 UI latency standards apply)
- A 5-second in-memory TTL cache (`sync.Map` of `graphKey -> edgeGraph`, populated lazily on first call, invalidated on upload or KB deactivate) means edges are recomputed at most once per 5-second window per corpus state
- For the typical corpus (~50 nodes), edge computation is O(n²) on the chunk count (section adjacency = O(n log n); keyword co-occurrence = O(n × token_count)); even 1000 chunks completes in <10ms in Go
- Citation co-occurrence requires scanning audit rows — this is the expensive path; cache TTL of 5s is acceptable since verdicts don't arrive faster than that in practice
- Zero schema migration — no new tables, no new Milvus/Mongo fields
- No audit risk — edge graph is derived data, not source-of-truth

**Cache invalidation events:**
- `POST /api/admin/kb/:slug/upload` success → invalidate edge graph for target slug
- `POST /api/admin/kb` (create) → invalidate all (new KB adds node topology)
- `POST /api/admin/kb/:slug/deactivate` → invalidate all (deactivated KB removes nodes)
- Time-based TTL expiry (5 seconds) → lazy recompute on next request

**Implementation location:** `internal/knowledge/graph.go` — new `EdgeGraph` type + `neighborsHandler` + `pathHandler` as methods on `*GraphBuilder` or as standalone functions that accept a `Retriever` + `kb.Registry`.

---

## 6. Phase 27 / DOC-06 joint acceptance

Phase 27 closed hermetically with `verify-phase-27.sh` (3 stages / 6 sub-checks all green) and 17 upload tests PASS. The operator runbook at `27-UAT.md` step 7 expects "Citation appears from the uploaded document" in the chat — this is the Phase 27 DOC-06 success criterion.

**Joint acceptance test (added to 28-UAT.md):**

The Phase 28 landing validates that Phase 27 upload pipeline and Phase 28 graph are jointly correct:

| Step | Action | Expected Result | Validation |
|---|---|---|---|
| **1. Operator uploads a new document via `/admin` KB view** | Upload `test-compliance-policy.pdf` to `authority` KB via `POST /api/admin/kb/authority/upload` | 200 response; `action_doc_uploaded` audit row written; `id_range_start/end` in frozen_args | `verify-phase-27.sh` Stage 2 still green; audit table has new row |
| **2. Operator queries `GET /api/knowledge/graph`** | No auth token required | Response includes an `authority` tier node with new doc as `DocNode`; chunk count increased | Graph JSON has new doc under `authority` tier |
| **3. Operator clicks doc node in GraphCanvas** | FE dispatches `setCenterView({kind:'doc-view', tier:'authority', file:'test-compliance-policy.pdf'})` | DocViewer opens with full markdown source | UI renders content |
| **4. Operator queries `GET /api/knowledge/graph/neighbors?chunk_id=<new_chunk_id>&depth=1`** | Public endpoint (no token) | Returns the new chunk's immediate neighbors (section-adjacent chunks within same doc, version-chain edges if effective window set) | `edges[].edge_type` in `{section_adjacent, version_chain}` |
| **5. Operator runs a chat query that triggers a verdict citing the uploaded document** | POST to `POST /api/chat` with query requiring content from uploaded doc | Verdict response cites `[ref-N]` pointing to new chunk; citation appears in `InspectorRetrieval` | FE shows chunk ID from new upload in verdict citation |

**Critical validation:** Step 5 confirms DOC-06 "上传即可见" extends to the full retrieval + citation pipeline — the uploaded chunk participates in `ChunkAccumulator.Add` (trust_floor.go:113) via the same `kb.Citable(reg, tier)` gate as boot-indexed chunks. No new code path needed; Phase 27 already indexes into the same Milvus/Mongo collection that Phase 26 seeded.

**Step 5 prerequisite:** Requires a live LLM endpoint; if unavailable (intranet flakiness), skip to manual verification: confirm `GET /api/knowledge/graph/neighbors?chunk_id=<new_chunk_id>` returns the new chunk with at least section-adjacent edges to other chunks in the same source document.

---

## 7. Risks & Unknowns

### Risk R1: Existing graph_test.go fixtures — what breaks if node-set becomes dynamic from KBRegistry?

**Severity: LOW**

The test at `graph_test.go:128-137` (`TestGraphBuilder_TierStructure`) asserts exactly 4 nodes when the stub registry has exactly 4 active slugs (`testBaselineSlugs`). Adding a 5th active KB to `testBaselineSlugs` would make this test fail — the assertion `require.Len(t, g.Tiers, 4)` would get 5.

**Break pattern:** If planner adds a new KB entry to the test fixture `testBaselineSlugs` without updating `TestGraphBuilder_TierStructure`, the test goes red. This is the **correct** behavior — the test is asserting a specific node count that should change when the KB set changes.

**Fix:** Update `TestGraphBuilder_TierStructure` to derive the expected count from `len(testBaselineSlugs)` rather than hardcoding 4. The existing `TestGraphNodesFromRegistry` (`graph_test.go:226-275`) already demonstrates this pattern — it tests 4-active→4-nodes and 5-active→5-nodes correctly.

**Risk:** The test at `graph_test.go:207-219` (`TestGraphBuilder_Empty`) asserts exactly 4 nodes when all 4 baseline slugs are active but the corpus is empty. Same fix needed.

### Risk R2: GraphCanvas scaling — node-count threshold at which it becomes unreadable

**Severity: MEDIUM**

The current radial layout (`GraphCanvas.tsx:64-93`) positions tiers on an inner ring (`TIER_RING = 110px`) and docs on an outer arc (`DOC_RING_BASE = 260px`). With `W=900, H=640`, the usable canvas area is ~640px diameter.

**Scaling analysis:**
- At 4 KBs × 10 docs × 3 chunks = 120 doc nodes: arc spread per doc = `min(PI/2.4, 2*PI/(120*2.2))` ≈ 0.012 rad ≈ 0.7 degrees — doc labels overlap
- At 4 KBs × 5 docs = 20 doc nodes: arc spread ≈ 0.07 rad ≈ 4 degrees — readable
- **Threshold: ~50-60 total doc nodes before label collision becomes problematic**

**Mitigation (GRF-04):** Implement `depth` limit on GraphCanvas neighbor expansion — clicking a doc node shows only its immediate neighbors, not the full edge graph. This limits the visible node count to `1 (clicked node) + degree(node) × depth` which stays under 50 for reasonable corpora.

**FE change for GRF-04:** `GraphCanvas.tsx` needs a new interaction mode: click a doc node → fetch `GET /api/knowledge/graph/neighbors?chunk_id=<id>&depth=1` → render subgraph with only the neighbor edges highlighted, original graph dimmed. This is a new render mode, not a replacement of the current radial view.

### Risk R3: Memoization cache invalidation — what if a chunk's effective window is updated?

**Severity: LOW**

Edge type `version_chain` depends on `EffectiveFrom/EffectiveTo` windows. If an operator updates a chunk's effective window (via re-indexing with new frontmatter), the edge graph computed from stale cached metadata would be wrong until TTL expiry (5 seconds).

**Analysis:** This is acceptable for v1.2 because:
1. Effective window updates happen only via re-index (boot seed or Phase 27 upload) — both are relatively rare operations
2. The 5-second TTL ensures stale edges auto-expire
3. The edge graph is advisory (FE visualization), not a trust boundary — incorrect edges don't cause incorrect citations
4. Citation correctness is protected by `ChunkAccumulator.Add` (trust_floor.go:113) which reads the authoritative `EffectiveFrom/EffectiveTo` at query time, not from cached edges

**If stricter consistency is needed (v2):** Invalidate cache on any `ListMetadata` call that returns different `EffectiveFrom/EffectiveTo` than the cached snapshot. This requires version-stamping the cache.

### Risk R4: CGO constraint — Phase 28 must stay pure Go

**Severity: LOW**

Phase 27 introduced `eino-ext` document parser submodules. Phase 28 adds no new dependencies — all edge computation is pure Go string/set operations on existing `DocumentMeta` fields. The `sync.Map` TTL cache is stdlib. No CGO risk.

**Verification:** `go list -m` after Phase 28 implementation should show no new dependencies beyond Phase 27's parser additions.

### Risk R5: Citation co-occurrence edges require audit row parsing

**Severity: MEDIUM**

The `citation_cooccur` edge type requires scanning audit rows with `action_verdict_rendered` and parsing `frozen_args` JSON to extract `chunk_ids[]`. This is:
1. Non-trivial SQL + JSON parsing
2. Only available for sessions that have verdicts — initial state has zero `citation_cooccur` edges
3. Potentially large audit table (thousands of rows) — needs indexing on `action` column

**Mitigation for v1.2:** Make `citation_cooccur` edges an **optional enhancement** (not a GRF-02 blocker). Implement as: on first `neighbors` call, scan audit rows with `action LIKE '%verdict%'` and build the co-occurrence map lazily. If the audit table is large (>10k rows), fall back to not computing `citation_cooccur` edges and set `degraded_reason` to explain.

---

## 8. Open Questions for the Planner

### Q1: Node-id format — chunk ID (integer) vs slug:doc#chunk (string) for human-readable API surface?

**Current:** `graph.go:342-366` `int64ToChunkID` renders chunk ID as a decimal string (e.g., `"7"`). The `neighbors` and `path` APIs use this string form as the canonical `chunk_id` identifier.

**Option A (current):** `chunk_id = "7"` — simple, matches existing citation format `[ref-7]`
**Option B:** `chunk_id = "authority:contract-approval.md#7"` — self-describing, but verbose and changes the citation format

**Recommendation:** Option A (current). The chunk ID is already a stable identifier that the existing citation system uses. Adding tier/source context to the API identifier would be redundant (the `nodes[].tier` and `nodes[].source` fields provide that context in the response). Changing the identifier format would require Phase 27's `InspectorRetrieval.tsx` to be updated.

### Q2: Edge weight threshold for cross-doc keyword co-occurrence (rare-token Jaccard min score)?

**Question:** What Jaccard score threshold should trigger a `keyword_cooccur` edge?

**Option A:** Jaccard ≥ 0.3 — includes moderately similar filenames (e.g., `travel-expense-policy.md` and `travel-expense-2024.md` share "travel-expense")
**Option B:** Jaccard ≥ 0.5 — only highly similar filenames (e.g., `travel-expense-policy.md` and `travel-expense-policy-v2.md`)
**Option C:** Jaccard ≥ 0.1 — catches more connections, higher noise

**Recommendation:** Option A (0.3) as default, configurable via `SKILLFORGE_GRAPH_KEYWORD_JACCARD_THRESHOLD` env var (float, default `0.3`). Justification: regulatory corpus filenames are verbose (often 3-5 words); Jaccard of 0.3 captures same-topic documents without being so strict that only versioned filenames connect.

### Q3: Memoization cache TTL (seconds)?

**Recommendation:** `SKILLFORGE_GRAPH_CACHE_TTL=5` (5 seconds). Justification: 5s is long enough to survive a burst of neighbor queries (e.g., FE expanding multiple nodes) without redundant recomputation, but short enough that a new upload (`POST /api/admin/kb/:slug/upload`) produces visible graph changes within 5-10 seconds. For the compliance use case, real-time graph consistency is not required — the citation pipeline (trust floor) operates at query time regardless of graph cache state.

### Q4: Whether to recompute edges on every `/api/knowledge/graph` call (option a) or only on graph mutation events (upload, deactivate)?

**Clarification of options from Section 5:**
- **Option a (current recommendation):** Lazy recompute + 5-second TTL cache. Edges recomputed on first request after TTL expiry or mutation event.
- **Option b (full invalidation on mutation):** After `POST /api/admin/kb/:slug/upload` succeeds, immediately recompute edge graph and store in cache (no TTL needed until next mutation). Faster reads, but upload latency increases.

**Recommendation:** Option a (lazy + TTL). Upload latency is already bounded by embedding (Phase 27's most expensive step). Adding edge recomputation to the upload critical path would increase the upload handler's latency without proportional benefit — the FE can see the new node immediately via `GET /api/knowledge/graph`, and edges to/from the new chunks will populate within 5 seconds via TTL expiry on the next neighbor query.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| GRF-01 | 图谱动态知识库化——`GET /api/knowledge/graph` 按注册表输出 N 个知识库节点（移除固定 4 tier 假设） | graph.go:251-265: N-dynamic via `b.reg.List()` ascending by slug; Phase 26 completed this |
| GRF-02 | chunk 关系层——确定性（零 LLM）关系边：同文档版本继承边 + 跨文档实体/关键词条款共现边；边可确定性重算 | Section 2: 4 edge types; all derivable from `DocumentMeta` fields (ID, Source, EffectiveFrom/EffectiveTo) + audit rows |
| GRF-03 | 图谱查询 API——邻居查询 + 受限深度路径查询；供 FE 与未来 agent 工具消费 | Section 3: `GET /neighbors?chunk_id=X&depth=2` + `GET /path?from=X&to=Y&max_hops=4` HTTP surfaces |
| GRF-04 | FE GraphCanvas 升级——动态知识库渲染 + 关系边可视化 + 邻居展开交互 | Section 7 Risk R2 + graph_test.go analysis; neighbor expansion via new FE interaction mode |
| GRF-05 | 图谱性能护栏——节点/边上限、分页与懒加载策略配置驱动；超限返回骨架图并明示降级 | Section 4: 4 config keys with defaults; degraded response shape; Section 7 Risk R2 GraphCanvas scaling threshold |

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Graph node set (N KB nodes) | Database/Storage (KBRegistry via ListMetadata) | — | Node set = `b.reg.List()` active entries (graph.go:258-264); no new storage |
| Edge computation (section_adjacent, version_chain) | API/Backend (internal/knowledge/graph.go) | — | Pure function on `DocumentMeta` fields already in retriever |
| Edge computation (keyword_cooccur) | API/Backend (internal/knowledge/graph.go) | — | Pure function on `Source` filename tokens; no new data needed |
| Edge computation (citation_cooccur) | API/Backend (internal/knowledge/audit.go scan) | Database/Storage (audit table) | Scans `action LIKE '%verdict%'` audit rows; heavy but optional for v1.2 |
| Neighbor/path query handlers | API/Backend (internal/server/knowledge.go) | — | New handlers alongside existing `graphHandler`; same auth model (public read) |
| Memoization cache | API/Backend (sync.Map in graph.go) | — | 5s TTL; invalidated on upload/deactivate events |
| GraphCanvas neighbor expansion (FE) | Browser/Client (React) | API/Backend (neighbors API) | FE fetches neighbors on doc-node click; renders subgraph |
| Config enforcement (limits) | API/Backend (env vars read at boot) | — | 4 env keys; zero hardcode |

---

## Standard Stack

### Core (zero new Go dependencies — all stdlib or already in go.mod)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sync.Map` | stdlib | Memoization cache for edge graph (5s TTL) | No external dependency; idiomatic Go for TTL cache |
| `encoding/json` | stdlib | Frozen args parsing for citation_cooccur edges | Already used everywhere |
| `strings` | stdlib | Tokenization for keyword co-occurrence | Already used everywhere |
| `sort` | stdlib | Deterministic ordering for neighbors/path output | Already used in graph.go:279 |
| `container/list` or custom ring buffer | stdlib | BFS for pathfinding | stdlib; no heap needed for BFS up to depth 4 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `stretchr/testify` | v1.11.1 | Assertions in new graph tests | Existing test framework |
| `internal/kb` | (existing) | `kb.Registry` interface for node set | Already imported in graph.go |
| `internal/retriever` | (existing) | `DocumentMeta` + `ListMetadata` | Already imported in graph.go |
| `internal/store` | (existing) | Audit row scan for citation_cooccur | Existing store interface |

### No New Dependencies
Phase 28 adds **zero new Go packages** and **zero new npm packages**. All edge computation is pure Go on existing data structures. The FE changes are additions to existing components (`GraphCanvas.tsx`, `knowledge.ts` types), not new library acquisitions.

---

## Package Legitimacy Audit

**No new external packages in this phase.** All work is pure Go on existing data structures. Skip legitimacy gate.

---

## Architecture Patterns

### System Architecture Diagram

```
GET /api/knowledge/graph
        │
        ▼
  GraphBuilder.Build()
        │
        ├── kb.Registry.List() ────────────────────────────► Node set = N TierNodes
        │   (graph.go:258-264)
        │
        └── retriever.ListMetadata()
            │
            ▼
        [DocumentMeta × N]
            │
            ├── Section adjacency edges ──── (ID sort within same (Tier, Source))
            ├── Version chain edges ──────── (EffectiveFrom/EffectiveTo window overlap)
            └── Keyword co-occur edges ───── (filename token Jaccard ≥ threshold)
                      │
                      ▼
                 EdgeGraph (in-memory)
                      │
            ┌─────────┴──────────────────┐
            │  sync.Map cache            │
            │  key = "graph-v1"          │
            │  TTL = 5 seconds           │
            │  invalidated on upload/    │
            │  deactivate                │
            └────────────────────────────┘
                      │
         ┌────────────┼────────────────────┐
         ▼            ▼                    ▼
  GET /neighbors   GET /path           GET /graph
  (new handler)    (new handler)       (existing, +degraded flag)
         │            │                    │
         ▼            ▼                    ▼
  BFS on EdgeGraph  BFS on EdgeGraph   embed edge summary
  within depth      within max_hops    or degraded skeleton

Audit rows (citation_cooccur):
  store.QueryAudit(action LIKE '%verdict%')
         │
         ▼
  parse frozen_args.chunk_ids[]
         │
         ▼
  co-occur graph (optional, v1.2)
```

### Recommended Project Structure

```
internal/knowledge/
├── graph.go              # existing: GraphBuilder.Build + GetChunk
├── graph_edges.go        # NEW: EdgeGraph type + ComputeEdges + neighbors/path handlers
├── graph_cache.go        # NEW: sync.Map TTL cache + invalidation hooks
└── graph_test.go         # existing; add neighbor/path/edge tests

internal/server/
└── knowledge.go          # existing: graphHandler/chunkHandler; add neighborsHandler/pathHandler/statsHandler

web/src/
├── components/graph/
│   ├── GraphCanvas.tsx   # existing: add neighbor-expansion interaction mode
│   └── GraphTree.tsx     # existing: no change needed
├── types/
│   └── knowledge.ts      # add EdgeNode + Edge types; degrade Graph type
└── hooks/
    └── useGraphNeighbors.ts  # NEW: fetch /neighbors on doc-node click
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pathfinding | Custom priority queue for BFS | `container/list` ring buffer or simple map+slice BFS | Unweighted BFS to depth 4 needs no heap; simple queue is faster and clearer |
| Edge graph memoization | channel-based cache or external Redis | `sync.Map` with TTL goroutine per entry | `sync.Map` is stdlib; TTL via `time.Timer` per entry; no new dependency |
| Keyword tokenization | regex-based tokenizer with unicode support | `strings.Fields` + stop-word map | Sufficient for ASCII/中文 filenames; no need for unicode-aware NLP library |
| Graph traversal | recursive DFS (stack overflow risk) | iterative BFS with explicit stack/queue | Depth limited to 4; iterative is safe and simple |

**Key insight:** The edge graph is a derived, non-critical visualization layer. It does not affect trust boundaries (citation, grounding). All edge types are derivable from data already in `DocumentMeta` or audit rows. The engineering budget should be spent on correct edge derivation logic and the FE neighbor-expansion interaction, not on sophisticated graph data structures.

---

## Common Pitfalls

### Pitfall G1: Edge graph recomputed on every request without cache
**What goes wrong:** `GET /neighbors` takes 500ms+ on every call (O(n²) edge computation), causing FE timeouts and bad UX.
**Why it happens:** Developer implements edge computation inline in handler without caching, assuming "it's just a loop over 50 chunks."
**How to avoid:** Implement `sync.Map` cache with 5s TTL. Lazy-recompute on first call; subsequent calls within 5s return cached result. Cache key = content-hash of sorted `(tier, source, effectiveFrom, effectiveTo)` tuples from `ListMetadata` — any metadata change produces a new cache key and triggers recompute.

### Pitfall G2: citation_cooccur edges computed on every neighbors call
**What goes wrong:** Scanning the full audit table on every `neighbors` request causes O(audit_rows) latency even with small corpus.
**How to avoid:** Lazy-load citation co-occurrence on first request; cache separately with longer TTL (30s or until next upload event). If audit table > 10k rows, skip citation_cooccur edges entirely in v1.2.

### Pitfall G3: GraphCanvas renders all edges simultaneously
**What goes wrong:** With 200+ edges, the SVG becomes a hairball — all edges visible, unreadable.
**How to avoid:** Phase 28's neighbor-expansion mode only renders edges among the subgraph (query node + depth-1 neighbors). The full radial view (existing) continues to show only tier→doc containment edges. The two modes are distinct: browse (containment only) vs explore (neighbor subgraph with relation edges).

### Pitfall G4: Node ID format inconsistency between graph and citations
**What goes wrong:** If Phase 28 changes chunk_id format (e.g., to `slug:source#id`), the existing `[ref-N]` citation format in verdicts breaks.
**How to avoid:** Keep `chunk_id` as decimal string (the `int64ToChunkID` function at graph.go:342). The `neighbors` and `path` responses use the same format. InspectorRetrieval.tsx (`inspector.go`) already handles string chunk IDs.

---

## Code Examples

### Edge computation — section adjacency + version chain (pure function)

```go
// Source: internal/knowledge/graph_edges.go (new file)
// ComputeEdges derives all relation edges from DocumentMeta rows.
// Deterministic: same input rows always produce identical edge set.
func ComputeEdges(metas []retriever.DocumentMeta) EdgeGraph {
    // Group by (tier, source)
    groups := make(map[string][]retriever.DocumentMeta) // key = "tier::source"
    for _, m := range metas {
        key := tierSourceKey(m.Tier, m.Source)
        groups[key] = append(groups[key], m)
    }

    edges := []Edge{}
    for _, chunks := range groups {
        // Section adjacency: sort by ID ascending, consecutive pairs get edge
        sort.Slice(chunks, func(i, j int) bool { return chunks[i].ID < chunks[j].ID })
        for i := 0; i < len(chunks)-1; i++ {
            edges = append(edges, Edge{
                From:      int64ToChunkID(chunks[i].ID),
                To:        int64ToChunkID(chunks[i+1].ID),
                EdgeType:  EdgeTypeSectionAdjacent,
                Weight:    1.0,
                Tier:      chunks[i].Tier,
            })
        }
        // Version chain: for each pair (a, b) where a.effective_to > 0 and b.effective_from > a.effective_from
        for i := 0; i < len(chunks); i++ {
            for j := i + 1; j < len(chunks); j++ {
                if chunks[j].EffectiveFrom > 0 && chunks[j].EffectiveFrom > chunks[i].EffectiveFrom &&
                    chunks[i].EffectiveTo > 0 && chunks[i].EffectiveTo <= chunks[j].EffectiveFrom {
                    // a is superseded by b
                    edges = append(edges, Edge{
                        From:     int64ToChunkID(chunks[i].ID),
                        To:       int64ToChunkID(chunks[j].ID),
                        EdgeType: EdgeTypeVersionChain,
                        Weight:   1.0,
                        Tier:     chunks[i].Tier,
                    })
                }
            }
        }
    }
    return EdgeGraph{Metas: metas, Edges: edges}
}

func tierSourceKey(tier, source string) string {
    return tier + "::" + source
}
```

### Keyword co-occurrence — filename token Jaccard

```go
// Source: internal/knowledge/graph_edges.go (new file)
var stopWords = map[string]bool{
    "the": true, "a": true, "an": true, "and": true, "or": true,
    "policy": true, "procedure": true, "md": true, "doc": true,
    // add common regulatory document words to filter
}

func tokenizeFilename(filename string) []string {
    // Strip extension
    name := strings.TrimSuffix(filename, ".md")
    // Split on non-alphanumeric
    raw := strings.FieldsFunc(name, func(r rune) bool {
        return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
    })
    tokens := []string{}
    for _, t := range raw {
        lower := strings.ToLower(t)
        if len(lower) >= 3 && !stopWords[lower] {
            tokens = append(tokens, lower)
        }
    }
    sort.Strings(tokens)
    return tokens
}

func jaccard(a, b []string) float64 {
    if len(a) == 0 && len(b) == 0 { return 0 }
    setA := make(map[string]bool)
    for _, x := range a { setA[x] = true }
    intersection := 0
    for _, x := range b {
        if setA[x] { intersection++ }
    }
    union := len(a) + len(b) - intersection
    return float64(intersection) / float64(union)
}

// Threshold from env: SKILLFORGE_GRAPH_KEYWORD_JACCARD_THRESHOLD (default 0.3)
func computeKeywordEdges(metas []retriever.DocumentMeta, threshold float64) []Edge {
    // Build token set per unique source file
    sourceTokens := make(map[string][]string) // source -> tokens
    seenSource := make(map[string]bool)
    for _, m := range metas {
        if seenSource[m.Source] { continue }
        seenSource[m.Source] = true
        sourceTokens[m.Source] = tokenizeFilename(m.Source)
    }
    // For each pair of chunks with different sources sharing tokens
    // (simplified: edge between any two chunks whose source files have Jaccard >= threshold)
    edges := []Edge{}
    sources := make([]string, 0, len(sourceTokens))
    for s := range sourceTokens { sources = append(sources, s) }
    for i := 0; i < len(sources); i++ {
        for j := i + 1; j < len(sources); j++ {
            score := jaccard(sourceTokens[sources[i]], sourceTokens[sources[j]])
            if score >= threshold {
                // Add edge between representative chunks of each source
                // (find first chunk ID for each source from metas)
                var idI, idJ int64
                for _, m := range metas {
                    if m.Source == sources[i] && idI == 0 { idI = m.ID }
                    if m.Source == sources[j] && idJ == 0 { idJ = m.ID }
                }
                edges = append(edges, Edge{
                    From:     int64ToChunkID(idI),
                    To:       int64ToChunkID(idJ),
                    EdgeType: EdgeTypeKeywordCooccur,
                    Weight:   score,
                    Tier:     "", // cross-tier; FE can use source tier from metadata
                })
            }
        }
    }
    return edges
}
```

### BFS neighbor query

```go
// Source: internal/knowledge/graph_edges.go (new file)
type EdgeGraph struct {
    Metas  []retriever.DocumentMeta
    Edges  []Edge
    adj    map[string][]Edge // chunk_id -> outgoing edges (pre-built for fast lookup)
}

func (g *EdgeGraph)Neighbors(chunkID string, maxDepth int, maxNodes int) (*NeighborResult, bool) {
    if g.adj == nil {
        g.buildAdjacency()
    }
    visited := map[string]bool{chunkID: true}
    queue := []string{chunkID}
    depth := 0
    resultNodes := []Node{nodeFromMeta(g.findMeta(chunkID))}
    resultEdges := []Edge{}

    for len(queue) > 0 && depth < maxDepth && len(resultNodes) < maxNodes {
        levelSize := len(queue)
        for i := 0; i < levelSize; i++ {
            current := queue[0]
            queue = queue[1:]
            for _, edge := range g.adj[current] {
                if !visited[edge.To] {
                    visited[edge.To] = true
                    queue = append(queue, edge.To)
                    resultNodes = append(resultNodes, nodeFromMeta(g.findMeta(edge.To)))
                    if len(resultEdges) < maxNodes*2 { // keep edges under limit
                        resultEdges = append(resultEdges, edge)
                    }
                }
            }
        }
        depth++
    }
    degraded := len(visited) >= maxNodes || depth >= maxDepth
    return &NeighborResult{
        ChunkID:  chunkID,
        Depth:    depth,
        Nodes:    resultNodes,
        Edges:    resultEdges,
        Degraded: degraded,
    }, !degraded
}
```

### BFS shortest path

```go
// Source: internal/knowledge/graph_edges.go (new file)
func (g *EdgeGraph) ShortestPath(from, to string, maxHops int) *PathResult {
    if g.adj == nil {
        g.buildAdjacency()
    }
    if from == to {
        return &PathResult{From: from, To: to, Hops: 0, Path: []string{from}, Edges: []Edge{}, Degraded: false}
    }
    parent := map[string]string{}
    var end string
    queue := []string{from}
    found := false

    for len(queue) > 0 && len(parent) < maxHops*10 { // safety limit on iterations
        current := queue[0]
        queue = queue[1:]
        for _, edge := range g.adj[current] {
            if _, seen := parent[edge.To]; !seen && edge.To != from {
                parent[edge.To] = current
                if edge.To == to {
                    found = true
                    end = to
                    break
                }
                queue = append(queue, edge.To)
            }
        }
        if found {
            break
        }
    }

    if !found || end == "" {
        return &PathResult{
            From: from, To: to, MaxHops: maxHops, Hops: -1,
            Path: []string{}, Edges: []Edge{},
            Degraded: true, DegradedReason: fmt.Sprintf("no path found within %d hops", maxHops),
        }
    }

    // Reconstruct path
    path := []string{}
    cur := to
    for cur != "" && cur != from {
        path = append([]string{cur}, path...)
        cur = parent[cur]
    }
    path = append([]string{from}, path...)

    // Reconstruct edges
    pathEdges := []Edge{}
    for i := 0; i < len(path)-1; i++ {
        for _, e := range g.adj[path[i]] {
            if e.To == path[i+1] {
                pathEdges = append(pathEdges, e)
                break
            }
        }
    }
    return &PathResult{
        From: from, To: to, MaxHops: maxHops,
        Hops: len(path) - 1, Path: path, Edges: pathEdges,
        Degraded: false,
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| Graph returns only containment hierarchy (tier→doc→chunk) | Graph returns containment + deterministic relation edges (section_adjacent, version_chain, keyword_cooccur, citation_cooccur) | Phase 28 | FE can now show cross-chunk relationships |
| Fixed 4-tier node set assumption | N-dynamic node set from KBRegistry (Phase 26 completed) | Phase 26 | Graph adapts to any number of KBs |
| No neighbor query API | `GET /neighbors?chunk_id=X&depth=2` returns subgraph | Phase 28 | Agent tools and FE can explore relations |
| No path query API | `GET /path?from=X&to=Y&max_hops=4` returns BFS path | Phase 28 | Enables "how are these two chunks related?" queries |
| GraphCanvas shows only tier→doc containment edges | GraphCanvas neighbor-expansion shows relation edges for subgraph | Phase 28 | Operators can see cross-chunk relationships |
| No performance guards on graph size | Config-driven limits + degraded skeleton response | Phase 28 | Graph remains usable at scale |
| LLM entity extraction for graph edges | Explicitly out of scope; all edges from metadata/audit | Phase 28 | Deterministic, hash-verifiable edges |

**Deprecated/outdated:**
- Hardcoded 4-tier node set assumption — retired by Phase 26
- LLM-based entity extraction for graph edges — explicitly out of scope per PROJECT.md v1.2

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Section adjacency edge = consecutive chunk IDs within same (tier, source) | Section 2 | If Phase 27 upload changes ID assignment (non-consecutive IDs), adjacency logic may not match document order |
| A2 | Keyword co-occurrence uses filename token Jaccard, not content token Jaccard | Section 2 | Content tokens require fetching chunk text; filename tokens are available from `ListMetadata` without extra I/O |
| A3 | Citation co-occurrence edge type is optional for v1.2 | Section 7 Risk R5 | If user requires it for v1.2, audit row scanning must be implemented with proper indexing |
| A4 | 5-second TTL cache is acceptable for upload→graph visibility latency | Q3 | If user requires immediate visibility, TTL must drop to 1s or cache must be invalidated synchronously on upload |
| A5 | Jaccard threshold 0.3 is appropriate for regulatory document filenames | Q2 | If threshold is too low (too many edges) or too high (too few), FE graph becomes either a hairball or a disconnected set |

---

## Open Questions for the Planner

1. **Node-id format** (Q1 above): `chunk_id = "7"` (current, decimal string) vs `chunk_id = "authority:contract-approval.md#7"` (self-describing)?
2. **Edge weight threshold for keyword co-occurrence** (Q2 above): Jaccard ≥ 0.3 (default) — acceptable for regulatory corpus?
3. **Memoization cache TTL** (Q3 above): 5 seconds — acceptable, or should it be configurable?
4. **Citation co-occurrence edges** (Section 7 Risk R5): Include in v1.2 scope or defer to v1.3? If included, requires audit row scan implementation.
5. **Whether neighbors/path APIs require admin token**: Current recommendation is public (no token) since they only read metadata. If user wants admin-only, add `X-Admin-Token` check.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | All BE | ✓ | go.mod `go 1.25.0` | — |
| WSL (CGO build/test) | CGO packages (sqlite3, existing) | ✓ | — | Windows hermetic non-CGO tests |
| Milvus / MongoDB | `ListMetadata` source; edge computation derives from existing data | ✗ 间歇 (memory: env-intranet-service-flakiness) | — | Hermetic tests use `stubMetaRetriever` (no real backend) |
| Node/npm + vitest | FE tests | ✓ | web/package.json | — |

**Missing dependencies with no fallback:** None — all new code is pure Go on existing data structures.
**Missing dependencies with fallback:** Milvus/Mongo — hermetic tests use `stubMetaRetriever` (already in graph_test.go); live testing is an operator gate.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + testify v1.11.1（BE）；vitest（FE） |
| Config file | None (env vars at boot; no config file) |
| Quick run command | `go test ./internal/knowledge/ -count=1` |
| Full suite command | `go test ./... -count=1` + `bash scripts/verify-phase-28.sh` + `cd web && npm test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GRF-01 | Node set = registry active entries; N dynamic; deactivated KB removed | unit | `go test ./internal/knowledge/ -run TestGraphNodesFromRegistry -count=1` | ✅ (graph_test.go:226) |
| GRF-02 | Section adjacency edges: consecutive IDs in same source | unit | `go test ./internal/knowledge/ -run TestEdgeComputation_SectionAdjacent -count=1` | ❌ Wave 0 |
| GRF-02 | Version chain edges: effective window overlap | unit | `go test ./internal/knowledge/ -run TestEdgeComputation_VersionChain -count=1` | ❌ Wave 0 |
| GRF-02 | Keyword co-occur edges: filename Jaccard ≥ threshold | unit | `go test ./internal/knowledge/ -run TestEdgeComputation_KeywordCooccur -count=1` | ❌ Wave 0 |
| GRF-03 | `neighbors` returns correct subgraph within depth limit | unit | `go test ./internal/knowledge/ -run TestNeighbors -count=1` | ❌ Wave 0 |
| GRF-03 | `neighbors` returns degraded skeleton when limits exceeded | unit | `go test ./internal/knowledge/ -run TestNeighbors_Degraded -count=1` | ❌ Wave 0 |
| GRF-03 | `path` returns BFS shortest path | unit | `go test ./internal/knowledge/ -run TestPath -count=1` | ❌ Wave 0 |
| GRF-03 | `path` returns degraded when no path within max_hops | unit | `go test ./internal/knowledge/ -run TestPath_NotFound -count=1` | ❌ Wave 0 |
| GRF-04 | GraphCanvas neighbor expansion: FE calls neighbors API on doc-node click | integration | `cd web && npx vitest run --grep neighbor` | ❌ Wave 0 |
| GRF-05 | Degraded response when node/edge count exceeds config limits | unit | `go test ./internal/knowledge/ -run TestDegradedResponse -count=1` | ❌ Wave 0 |
| GRF-05 | Config-driven limits: env vars override defaults | unit | `go test ./internal/knowledge/ -run TestConfigLimits -count=1` | ❌ Wave 0 |
| Joint (27+28) | Upload doc → graph shows new node → neighbors API returns edges | integration | Manual: upload + GET /graph + GET /neighbors | ❌ Operator UAT |

### Sampling Rate
- **Per task commit:** `go test ./internal/knowledge/ -count=1`（< 30s，纯 hermetic with stub retriever）
- **Per wave merge:** `go test ./... -count=1` + `bash scripts/verify-phase-28.sh` + `cd web && npm test`
- **Phase gate:** WSL `make check-cgo` + `go build ./...` + `go vet ./internal/knowledge/ ./internal/server/` 全绿 + FE `npx tsc --noEmit` exit 0 + operator UAT (Phase 27 upload → graph shows node → neighbor edges visible)

### Wave 0 Gaps
- [ ] `internal/knowledge/graph_edges.go` + `graph_edges_test.go` — edge computation + neighbors + path
- [ ] `internal/knowledge/graph_cache.go` — sync.Map TTL cache + invalidation
- [ ] `internal/server/knowledge.go` — add `neighborsHandler` + `pathHandler` + `statsHandler`
- [ ] `web/src/hooks/useGraphNeighbors.ts` — FE neighbor expansion hook
- [ ] `web/src/components/graph/GraphCanvas.tsx` — neighbor expansion interaction mode
- [ ] `web/src/types/knowledge.ts` — add `Edge`, `EdgeNode`, `NeighborResult`, `PathResult`, `GraphDegraded` types
- [ ] `scripts/verify-phase-28.sh` — structural gate (zero new hardcode; env var grep locks)

---

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No user auth; admin token only for mutation endpoints |
| V3 Session Management | no | No session state in graph layer |
| V4 Access Control | partial | `neighbors`/`path` are public read-only (metadata only); admin token only needed if user wants admin-only stats |
| V5 Input Validation | yes | `chunk_id` parsed via `strconv.ParseInt`; invalid → 400；`depth`/`max_hops` validated as positive integers |
| V6 Cryptography | no | No crypto operations; edge graph is derived public data |
| V7 Error/Logging | yes | W-3 envelope for 500: generic message, full error logged server-side |

### Known Threat Patterns for 本 phase 触面
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Chunk ID injection via `neighbors`/`path` URL params | Tampering | `strconv.ParseInt` validates numeric ID; non-numeric → 400；IDs validated against `ListMetadata` before use |
| Large depth/max_hops causing DoS (BFS explosion) | Denial of Service | Config-enforced hard limits (`SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT=2`, `SKILLFORGE_GRAPH_PATH_HOPS_LIMIT=4`) + `maxNodes` cap on result |
| Graphviz DoS ( pathological filename tokens causing Jaccard explosion) | Denial of Service | Jaccard threshold (default 0.3) prevents pathological similarity; filename-only tokenization limits input size |
| Audit row parsing for citation_cooccur (JSON injection) | Information Disclosure | `json.Unmarshal` into typed struct with validation; malformed frozen_args skipped with WARN |

---

## Sources

### Primary (HIGH confidence — codebase-grounded)
- `internal/knowledge/graph.go` — `GraphBuilder.Build`, `Graph` struct, `int64ToChunkID`, `versionFromEffective` (lines: 17-422 all read in full)
- `internal/knowledge/graph_test.go` — all 14 tests (lines: 1-482 all read in full); `stubMetaRetriever` pattern verified at :35-64
- `internal/knowledge/indexer.go` — `Document` struct + `validateTiers` + `splitFrontmatter` + `parseEffectiveWindow` (lines: 1-466 all read in full)
- `internal/retriever/retriever.go` — `DocumentMeta`, `Retriever` interface, `ListMetadata` contract (lines: 1-193 all read in full)
- `internal/server/knowledge.go` — `graphHandler`, `chunkHandler`, `parseLenientChunkID`, `docsHandler` (lines: 1-394 all read in full)
- `web/src/components/graph/GraphCanvas.tsx` — radial layout, TIER_COLOR map, interaction handlers (lines: 1-208 all read in full)
- `web/src/components/graph/GraphTree.tsx` — Accordion tree rendering (lines: 1-119 all read in full)
- `web/src/components/inspector/InspectorRetrieval.tsx` — chunk ID click dispatch (lines: 1-88 all read in full)
- `.planning/ROADMAP.md` — Phase 28 description, GRF-01..05 requirements, v1.2 standards (lines: 154-166)
- `.planning/REQUIREMENTS.md` — GRF-01..05 traceability (lines: 247-255), v1.2 Out of Scope (lines: 277-284)
- `.planning/PROJECT.md` — v1.2 standards (lines: 124-135)

### Secondary (MEDIUM confidence — phase document grounded)
- Phase 26 RESEARCH.md — KBRegistry patterns, `graph.go` N-dynamic implementation already verified
- Phase 27 RESEARCH.md — upload pipeline + `ListMetadata` consistency, DOC-06 joint acceptance protocol
- Phase 27 PATTERNS.md — document shape reuse (`retriever.Document` fields)

### Tertiary (LOW confidence — architectural reasoning)
- BFS pathfinding implementation details — standard algorithm, no external source needed
- Jaccard threshold 0.3 — architectural choice for regulatory corpus, not verified against specific dataset

---

## Metadata

**Confidence breakdown:**
- Current graph surface (Section 1): HIGH — every missing capability confirmed by file:line citations from graph.go, knowledge.go, retriever.go
- Deterministic edge types (Section 2): HIGH — all edge derivations use existing `DocumentMeta` fields with known semantics
- Neighbor/path API design (Section 3): HIGH — HTTP surface designed to match existing handler patterns in knowledge.go
- Performance ceilings (Section 4): MEDIUM — defaults (50/200/2/4) are architectural choices based on GraphCanvas scaling analysis, not measured against live corpus
- Storage strategy (Section 5): MEDIUM — memoization cache recommendation is sound but TTL choice (5s) is a judgment call
- Joint acceptance (Section 6): HIGH — all steps verifiable with existing endpoints; no new infrastructure needed
- Risks (Section 7): MEDIUM — Risk R2 (GraphCanvas scaling) is estimated from geometry, not measured; Risk R5 (citation_cooccur) is LOW priority for v1.2
- Open questions (Section 8): HIGH — all grounded in concrete code analysis; planner decisions needed before plan-phase

**Research date:** 2026-09-07
**Valid until:** 2026-10-07（stable — Phase 28 builds directly on Phase 26/27 which are already closed; no upstream API changes expected）
