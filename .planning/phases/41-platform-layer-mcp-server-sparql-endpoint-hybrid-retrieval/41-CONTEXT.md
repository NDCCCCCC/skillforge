# Phase 41: Platform Layer — MCP Server + SPARQL Endpoint + Hybrid Retrieval - Context

**Gathered:** 2026-09-23
**Status:** Ready for planning

<domain>
## Phase Boundary

给 v1.4 Ontology Layer 加 Platform Layer。本 phase 交付 **3 个外部消费面 + 1 个调试面板**，全部强制经过 QueryScoped tier 信任隔离（Phase 40 SC-5 复用）：

- **MCP server (stdio)**：用 `mark3labs/mcp-go` 暴露 4 类工具，让外部 agent (Claude Desktop / Cline / IDE 插件) 直接 query SkillForge KG
- **SPARQL-like REST `POST /api/sparql`**：接受三元 pattern subset 请求，返回 JSON triples；零外部 SPARQL parser 依赖
- **Hybrid Retrieval**：query → KG 1-2 跳 subgraph expansion → Milvus hybrid search → BGE-reranker-base rerank；Q1/Q4 类查询召回 ≥ 0.85（绝对门槛）
- **`/admin/platform` UI**：单 page 三 tab — SPARQL Playground (CodeMirror 6) + MCP Inspector + Hybrid 调试面板（KG 展开 + Milvus 命中列表）
- **`verify-phase-41.sh`**：MCP stdio 子进程调用 + SPARQL endpoint curl + Hybrid Q1/Q4 召回跑分

**SC-1..6 (ROADMAP)**：
1. MCP server 启动并暴露 4 类工具（min: skf_query/skf_skills/skf_chunk_for；扩展: schema/neighbors）
2. SPARQL-like `POST /api/sparql` 接受三元 pattern，response 为 JSON triples
3. KG query 自动 tier 隔离：MCP（service_skill）+ SPARQL（X-Skill-Name header）强制按 caller 上下文过滤，不可绕过
4. Hybrid Retrieval：KG 1-2 跳 + Milvus hybrid + BGE-reranker-base；Q1/Q4 召回 ≥ 0.85（绝对门槛，复用现有 golden set）
5. Admin UI 三个 tab：SPARQL Playground + MCP Inspector + Hybrid 调试
6. `verify-phase-41.sh` 全绿：MCP tools 可调用 + SPARQL 正常 + Hybrid Q1 ≥ 0.85

**不包含**：可写 SPARQL（UPDATE/INSERT 仅 admin console 已具备；本 phase 端点 read-only）；OAS 3.0 / OpenAPI 自动生成（手动维护即可）；按 user/role 维度的 ACL（Phase 21 已做 auth channel，platform 层只按 skill 维度隔离）；Milvus schema 变更（hybrid search 已支持，不动 schema）。

**UI 一致性 hard constraint**: `/admin/platform` 涉及 web — **plan-phase 前先跑 `/gsd:ui-phase 41` 产 UI-SPEC.md** (v1.4 Standards, STATE.md 36-41 行)。

</domain>

<decisions>
## Implementation Decisions

### MCP server transport
- **D-MCP-01:** Transport = **stdio only**（非 Streamable HTTP）。理由：本地 IDE / Claude Desktop 集成是 v1.4 最常见消费场景；零端口冲突；与现有 Hertz :8888 隔离；v1.4 远程 agent 不在 SC-1..6 scope。
- **D-MCP-02:** Server framework = **`github.com/mark3labs/mcp-go`**（社区标准；eino-ext 仅提供 client 端 adapter，server 侧无 eino 原生）。stdio transport 由 mark3labs/mcp-go 提供；无需自实现 JSON-RPC。

### MCP tools 集（min 3 + 3 扩展 = 6 个）
- **D-MCP-03:** 必含 min 集：
  - `skf_query(pattern)` — `{s?, p?, o?, limit?, offset?}` 三元 pattern，server 内部走 QueryScoped
  - `skf_skills()` — 返回所有 Skill 元数据（list of `{name, description, reads_tiers}`）
  - `skf_chunk_for(entity_iri)` — 返回该实体相关的 ECL chunks（source_path + line range + text snippet），用于 agentic 上下文注入
- **D-MCP-04:** 加 **skill 过滤**（每个 tool 可选 `skill=` 参数；server 启动时配 `SKILLFORGE_MCP_SERVICE_SKILL` 作为默认 skill；未指定时使用 service_skill 过滤）。SC-3 信任隔离强保证。
- **D-MCP-05:** 加 **pagination + KG 跳数**：`skf_query` 接受 `limit` (default 50, max 500) / `offset`；`skf_chunk_for` 接受 `max_hops` (1|2, default 1)。
- **D-MCP-06:** 加 **KG 边架构**：`skf_schema()` 返回 TBox 类层次概览（classes + properties）；`skf_neighbors(uri, hops)` 返回某实体 N 跳内 neighbors + edges，给 agentic 探索用。

### SPARQL endpoint
- **D-SPARQL-01:** Shape = **三元 subset**（非完整 SPARQL 1.1 parser）。POST body = `{s?, p?, o?, limit?, offset?, filter?}`；filter 接受实体名子串匹配；response = `{results: [{s, p, o, provenance?}], count, truncated}`。零外部 SPARQL parser 依赖；与 MCP skf_query 参数同形（共享 server-side handler）。
- **D-SPARQL-02:** Read-only；任何 mutation 走 admin ontology routes（Phase 38/40 已具备）。
- **D-SPARQL-03:** 与 MCP 共享 caller identity 协议：`X-Skill-Name` header 携带 skill 名；未指定 → 401 (no caller context)，强制 SC-3 不可绕过。

### Hybrid Retrieval
- **D-HYB-01:** Pipeline = `query → KG 1-2 跳 subgraph expansion → Milvus hybrid (dense + sparse) top-50 → BGE-reranker-base → top-5`。
- **D-HYB-02:** SC-4 召回 ≥ 0.85 = **绝对门槛**（非相对 v1.3 Δ=+0.15）。Eval set 复用 `eval/calibration.json` 现有 golden set（Phase 6 已固化）；Q1/Q4 类查询子集。
- **D-HYB-03:** **启用 BGE-reranker-base**（不开可关）。`SKILLFORGE_RETRIEVAL_RERANK=true`（default）启用 rerank；false 走 Milvus 原生 top-k。
- **D-HYB-04:** KG expansion `max_hops=1|2`，default=1。SKILLFORGE_HYBRID_HOPS=2 实验性开启；v1.4 默认 1 跳（已足够；2 跳用于 eval 调优）。
- **D-HYB-05:** caller identity = skill（同 D-SPARQL-03）；hybrid retrieval 结果 KG 子图受 QueryScoped 过滤。

### Admin UI /admin/platform
- **D-UI-01:** **单 page 三 tab**（与 Phase 40 governance tab 同架构：`SubrouteTab` + view switcher）。三 view 共享 store，可从 SPARQL Playground 跳到 Hybrid 调试复用 query 输入。
- **D-UI-02:** Tab 顺序（与 Phase 40 admin tab 顺序一致，11th tab 在 10th 治理 tab 之后）：SPARQL Playground / MCP Inspector / Hybrid 调试
- **D-UI-03:** 代码编辑器 = **CodeMirror 6**（非 Monaco/textarea）。理由：~150KB，未来 SKILL.md/YAML 编辑可复用；React 19 兼容；Vite 原生 ESM 支持。
- **D-UI-04:** MCP Inspector = tools list（来自 MCP server 启动时 introspection）+ 每个 tool 的「测试调用」按钮（发 stdio 子进程 demo 一次；FE 只展示 tools list 与 JSON schema）
- **D-UI-05:** Hybrid 调试面板 = KG expansion 可视化（节点 + 边，可点开看 provenance）+ Milvus 命中列表（top-50 score + text snippet，rerank 后 top-5 高亮）
- **D-UI-06:** 全 tab 沿用 Phase 40 UI-SPEC 既有 token 集；保留 ThemeToggle 兼容；`/admin/platform` 11th SubrouteTab 入口加在 AdminFeedbackTabs order 数组末尾

### Trust isolation (cross-cutting)
- **D-TRUST-01:** SPARQL endpoint + MCP tools + Hybrid Retrieval 全部强制经过 Phase 40 `QueryScoped`。SPARQL 通过 `X-Skill-Name` header 携带；MCP 通过启动 env `SKILLFORGE_MCP_SERVICE_SKILL` 配置；hybrid 通过 SPARQL/MCP 同一入口。未指定 caller skill → 401。
- **D-TRUST-02:** 不提供「admin baseline tier」旁路。所有 platform 入口都是「按 skill 过滤」。

### Claude's Discretion
- Milvus hybrid search 的 dense/sparse 权重（默认 0.7/0.3，planner 可调；BGE-M3 sparse 已内置）
- BGE-reranker-base 加载方式（self-host Docker vs API；v1.4 倾向 self-host 但若部署失败可退化）
- verify-phase-41.sh 的 sub-shell 用 `bash`（用户在 WSL 后端）或 `sh`（兼容 Windows MSYS2，但 stdio MCP 测试需 bash 子进程）
- 三 tab 的相对 tab 切换动效（沿用 Phase 40 governance tab 既有 idiom）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 40 (sister-phase, just shipped)
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-CONTEXT.md` — locked decisions D-SHACL-*, D-PROV-*, D-TIER-*, D-LOG-*; especially D-TIER-01/03/06/07 (QueryScoped contract reused here)
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md` — visual contract idiom (SubrouteTab + view switcher + three-color tokens); 11th admin tab pattern
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-VERIFICATION.md` — 17/17 must-haves, verify-phase-40.sh hermetic gate pattern (mirror for verify-phase-41.sh)

### Phase 38 (TBox/KG foundation)
- `.planning/phases/38-foundation-tbox-kg/38-CONTEXT.md` — KG store API surface, auto schema lift architecture
- `internal/ontology/store/store.go` — Triple / Query / QueryScoped signatures (PLATFORM-Phase consumes directly)

### Phase 39 (ECL Pipeline)
- `.planning/phases/39-ecl-pipeline/39-CONTEXT.md` — ECL chunk_for entity; skf_chunk_for MCP tool reuses this

### MCP / Eino framework docs (researched via Context7 2026-09-23)
- `https://modelcontextprotocol.io/specification/2025-11-25/basic/transports` — stdio + Streamable HTTP dual-track; stdio chosen for v1.4
- `https://github.com/cloudwego/eino-ext/blob/main/components/tool/mcp/README.md` — confirms eino-ext is **client-side** MCP only; server must use mark3labs/mcp-go

### Milvus hybrid search
- Phase 8 spike (`.planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/`) — confirmed Milvus 2.5+ native hybrid (dense+sparse), BGE-M3 multi-vector
- BGE-reranker-base: Phase 8 spike selected as CN C-MTEB leader

### Project-level
- `.planning/PROJECT.md` — Tech stack locked: Go + Eino + Hertz; Phase 41 must follow
- `.planning/REQUIREMENTS.md` — PLAT-ONT-18, PLAT-ONT-19, PLAT-ONT-20 requirement texts
- `.planning/STATE.md` — v1.4 Standards (UI consistency, zero hardcoded IRI, no cgo, handler ≤ 200 lines, Tier trust isolation as moat)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ontology/store/store.go::QueryScoped(pattern, allowedTiers)` — Phase 40 SC-5; **all three Phase 41 surfaces (MCP/SPARQL/Hybrid) MUST call this** for trust isolation
- `internal/ontology/store/store.go::Query(pattern)` — admin baseline (no tier filter); only used by internal admin routes
- `internal/ontology/store/store.go::Provenance` struct — `source_kind/path/line_start/line_end/extracted_at`; hybrid debug panel + skf_chunk_for consume this
- `internal/server/admin_ontology.go` — existing admin handler pattern (Hertz + DI + JSON DTOs); new sparql/mcp/hybrid handlers mirror this
- `internal/server/admin_governance.go` (just shipped) — get-route pattern with role-gated return shapes; SPARQL/MCP handlers reuse idiom
- `web/src/components/admin/governance/GovernanceTab.tsx` (Phase 40) — SubrouteTab + view-switcher idiom; **/admin/platform is a near-clone of this** with three different views

### Established Patterns
- v1.4 admin console = hand-rolled (no shadcn registry); uses Tailwind v4 + theme.css tokens + lucide-react icons
- Phase 40 introduced Cytoscape.js as the lone sanctioned 3rd-party npm dep for FE graph viz; **Phase 41 likely introduces CodeMirror 6** as another sanctioned npm dep (planner must pin exact version + no extra plugins)
- Each phase adds 1 FE subroute tab; AdminFeedbackTabs order array has 10 entries today (治理 = 10th), platform = 11th
- verify-phase-*.sh hermetic gate pattern from Phase 40 — 6 sections (build/vet/unit/integration/latency/full-suite); Phase 41 follows same shape with MCP stdio test added

### Integration Points
- `cmd/server/main.go` (Phase 40 added SHACL boot) — **Phase 41 adds MCP stdio subprocess boot** alongside Hertz :8888
- `internal/server/server.go` — registers admin governance/ontology routes; Phase 41 adds `POST /api/sparql` here
- Milvus client (Phase 8) — Phase 41 hybrid retrieval adds BGE-reranker-base client; consider config-driven enable/disable
- ECL ABox (`internal/knowledge/ecl/abox.go`) — skf_chunk_for reads from here

</code_context>

<specifics>
## Specific Ideas

- 用户对 **stdio transport 简化部署** 强烈偏好 — 不引入端口冲突 / Origin 校验的运维负担
- 用户对 **CodeMirror 6** 的"未来可复用"价值认可 — 不是当前最小可用解，但 ROADMAP 后续 phase（如 skill editor）会复用
- 用户对 **BGE-reranker-base** 的召回质量有强要求 — SC-4 绝对门槛 0.85 是用户的承诺，不是建议
- 用户倾向 **最小可行 surface** + **强信任隔离**：MCP 6 tools 已超出 ROADMAP SC-1 的 3 min 集（多了 schema/neighbors/分页），但都在"调试可见性"范围内
- 用户在 area 1 一次选了所有 4 个 tool 选项 — 表明倾向"完整 MCP 表面暴露"而非"渐进式扩展"
- 与 Phase 40 governance tab **架构同形**（单 page 三 tab + view switcher + SubrouteTab）是用户的明确偏好 — UI 一致性 hard constraint

</specifics>

<deferred>
## Deferred Ideas

- **Streamable HTTP transport for MCP** — v2/v1.5 远程 agent 场景；当前 v1.4 stdio only 即可
- **完整 SPARQL 1.1 parser** — 若 v1.5+ 有外部消费者真正需要，可引入 apache/jena-arq 或类似；当前三元 subset 足够 admin playground + 调试
- **Monaco editor** — 若未来需要 IDE 级编辑体验（多文件、IntelliSense）；当前 CodeMirror 6 轻量起点
- **user/role ACL** — Phase 21 已做 auth channel，platform 层只按 skill 维度隔离；user/role ACL 不在 v1.4 scope
- **OAS 3.0 / OpenAPI 自动生成** — 手动维护 endpoint 文档即可；自动生成是 nice-to-have
- **多语言 admin UI 翻译** — 平台文本中文优先；i18n 是 cross-cutting future phase

</deferred>

---

*Phase: 41-platform-layer-mcp-server-sparql-endpoint-hybrid-retrieval*
*Context gathered: 2026-09-23 via standard discuss-phase (4 areas discussed)*
