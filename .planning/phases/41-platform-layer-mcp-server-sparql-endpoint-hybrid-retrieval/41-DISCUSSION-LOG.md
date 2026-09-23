# Phase 41 Discussion Log

**Date:** 2026-09-23
**Mode:** standard discuss-phase (no --auto/--chain/--text)
**Areas discussed:** 4 of 4 (user selected all)
**Questions asked:** 8

## Areas

### Area 1: MCP server transport 与 tools 集范围

**Q1 — MCP transport 选型（spec stdio + Streamable HTTP 双轨；mark3labs/mcp-go 两者都支持）**
- 仅 stdio（推荐）
- 仅 Streamable HTTP
- 两者都支持

**Selected:** 仅 stdio（推荐）
**Rationale:** 本地 IDE / Claude Desktop 集成是 v1.4 最常见消费场景；零端口冲突；与 Hertz :8888 隔离；部署简单（systemd / Docker exec）。

**Q2 — MCP tools 集范围（ROADMAP 提的 3 个 tool 是最低集，是否加 skill 过滤 / pagination / KG 跳数）**
- 最小集（推荐）
- 加 skill 过滤
- 加 pagination + KG 跳数
- 查 KG 边架构

**Selected:** 最小集 + 加 skill 过滤 + 加 pagination + KG 跳数 + 查 KG 边架构（全部 4 个）
**Rationale:** 用户在 area 1 一次选了所有 4 个 tool 选项 — 倾向"完整 MCP 表面暴露"而非"渐进式扩展"。6 个 tool 总数（3 min + 3 扩展）仍属 minimal surface area 但完整覆盖外部 agent 集成需求。

### Area 2: SPARQL endpoint 兼容深度 + caller identity

**Q3 — SPARQL endpoint 兼容深度**
- 三元 subset（推荐）
- 标准 SPARQL JSON SELECT
- 同形双轨（textual + JSON）

**Selected:** 三元 subset（推荐）
**Rationale:** 零外部 SPARQL parser 依赖；与 MCP skf_query 参数同形（共享 server-side handler）；admin playground 足够；完整 parser 学习曲线 + 上游变更风险与 v1.4 价值不匹配。

**Q4 — SPARQL/MCP 是否按 caller 上下文强制 tier 隔离（SC-3 原话：不可绕过）**
- HTTP-only + header 携带 skill（推荐）
- 双调用面各有 origin
- 单一 admin tier（锁差）

**Selected:** HTTP-only + header 携带 skill（推荐）
**Rationale:** SC-3 强信任隔离承诺；MCP server 启动时配 `SKILLFORGE_MCP_SERVICE_SKILL` 作为默认 skill；SPARQL 通过 `X-Skill-Name` header 携带；未指定 → 401。

### Area 3: Hybrid Retrieval 召回门槛（SC-4）+ Reranker

**Q5 — SC-4 召回 ≥ 0.85 怎么达成**
- 绝对门槛 + 现有 golden set 复测（推荐）
- Δ=+0.15 相对提升
- 样本 representative 补齐
- 仅 BGE-M3 dense 无 rerank

**Selected:** 绝对门槛 + 现有 golden set 复测（推荐）
**Rationale:** 0.85 是用户对 Phase 41 的承诺；复用 `eval/calibration.json` 现有 golden set（Phase 6 已固化）；Q1/Q4 类查询子集；非相对 delta 表达（防止"反正比 v1.3 好"自满）。

**Q6 — Hybrid retrieval 是否启用 BGE-reranker-base**
- BGE-reranker-base
- 不启用 reranker（推荐）
- 可开关 hybrid_retrieval.rerank

**Selected:** BGE-reranker-base
**Rationale:** 用户对召回质量有强要求（SC-4 0.85 绝对门槛）；BGE-reranker-base 在 C-MTEB 跑分已知；额外 100ms+ 延迟可接受（admin 调试面板非高频路径）。

### Area 4: Admin UI /admin/platform 结构 + 代码编辑器

**Q7 — /admin/platform 是单页三 tab 还是三独立 route**
- 单 page 三 tab（推荐）
- 三个独立 route

**Selected:** 单 page 三 tab（推荐）
**Rationale:** 与 Phase 40 governance tab 同架构（SubrouteTab + view switcher）；状态可跨面板共享（如输入 s/p/o 时同时跳到 SPARQL Playground）；UI 一致性 hard constraint（v1.4 Standards）。

**Q8 — SPARQL/MCP Inspector 中的代码编辑器选型**
- CodeMirror 6（推荐）
- Monaco Editor
- 纯 textarea + 等宽字体

**Selected:** CodeMirror 6（推荐）
**Rationale:** ~150KB，未来 SKILL.md/YAML 编辑可复用；React 19 兼容；Vite 原生 ESM 支持；不引入 Monaco 2MB bundle。

## Deferred ideas

- Streamable HTTP transport for MCP → v2/v1.5 远程 agent 场景
- 完整 SPARQL 1.1 parser → 若 v1.5+ 有外部消费者真正需要
- Monaco editor → 若未来需要 IDE 级编辑体验
- user/role ACL → 已在 Phase 21 (auth channel) 解决
- OAS 3.0 / OpenAPI 自动生成 → 手动维护 endpoint 文档即可
- 多语言 admin UI 翻译 → 跨 phase future work

## Claude's discretion

- Milvus hybrid search dense/sparse 权重（planner 可调）
- BGE-reranker-base 加载方式（self-host vs API；v1.4 倾向 self-host）
- verify-phase-41.sh 的 sub-shell 类型（bash vs sh）
- 三 tab 切换动效（沿用 Phase 40 既有 idiom）

## Frameworks researched

- **MCP spec (2025-11-25)** — stdio + Streamable HTTP dual-track; Streamable HTTP 是新增，旧 HTTP+SSE deprecated 但保留向后兼容
- **eino-ext MCP** — 仅 client 端 adapter (`GetTools` 消费外部 MCP server)；server 侧无 eino 原生实现；社区标准是 `github.com/mark3labs/mcp-go`
- **Milvus hybrid search** — Phase 8 spike 已确认 2.5+ native；dense+sparse weight 可调；BGE-reranker-base 是 Phase 8 spike 选定的 CN C-MTEB leader
