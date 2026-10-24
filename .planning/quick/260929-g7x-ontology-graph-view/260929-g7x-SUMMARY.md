---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick 260929-g7x: 知识图谱「本体视图」Summary

用 KG ABox 本体数据(quick-260928-nzk 写入)投影出 note↔表/险种/机构 的真实关系图,作为知识图谱默认视图;原 tier→文档 树降级为「文档」第二模式。

**Duration:** ~50 min | **Date:** 2026-09-29 | **Branch:** phase-20-p10-6-admin-console

## Commits

| Task | Commit | Content |
| ---- | ------ | ------- |
| 1 BE | `bb5d3d5` | GET /api/admin/ontology/graph 投影端点 + ht pins |
| 2 FE | `93daf7d` | OntologyGraphView + 视图切换 + focus 贯通 + 测试 |
| 2 fix | `13d782e` | chunk_ids→notes 异步解析(focus 不再降级全库) |

## Task 1 — BE: 本体图投影端点

- `internal/server/admin_ontology.go` — `adminOntologyGraphHandler` (admin token 门,403 before 503,同 kg/query)。
  - `notes=note_032,note_033` 逗号分隔(容忍 skfNS 全 IRI 前缀与空白);无参 = 全量(`ontologyAllNoteIRIs`: rdf:type NoteChunk ∪ 三条 note 谓词的 subject,去重排序)。
  - 投影(`projectOntologyGraph` + `ontologyExpandNote` helper,handler 本体 ≤30 行):
    - note 节点(kind=note,label=rdfs:label 否则 source_idx 裸数字);referencesTable→table 节点(label 去 tbl_ 前缀);involvesInsuranceProduct→insurance_product;involvesOrganization→organization;
    - 每个被涉及机构的 parentOrganization 目标强制成节点 + parent_org 边(1-hop 邻域完整,即使 parent 不被 note 直连);
    - belongsToTier 一律作为节点 `tier` 字段,**不建 tier 节点**(避免大节点喧宾夺主);
    - 节点/边按 id / from|to|type 去重;不存在的 note → 200 + 空 nodes(不产 ghost 节点);
    - label 字面量按 store 约定剥 `"value"` 引号。
- 路由注册 `internal/server/server.go`(`h.GET("/api/admin/ontology/graph", ...)`)。
- 测试 `internal/server/admin_ontology_graph_test.go`(5 pins):投影形状(note label fallback/entity label/parent hop/tier 字段非节点)、跨 note 表节点去重、notes 过滤、未知 note 空响应、nil 层 503;并入了 403 token 门矩阵。

**验证:** WSL go build ./... + go vet + 新增 5 测试 + `go test ./internal/server/` 全绿 (15.6s)。

## Task 2 — FE: 本体视图 + 视图切换 + 聚焦贯通

- `web/src/components/graph/OntologyGraphView.tsx`(新):cytoscape plain npm(治理 tab 同款,零新依赖),cose 力导向(全库 200+ 节点由 cytoscape 扛),kind 配色 note=蓝(--color-primary)/table=橙(needs-info)/insurance_product=绿(compliant)/organization=紫(#a78bfa,主题无紫色 token,已注释),parent_org 虚线,节点 tap → 右侧明细栏(类型/名称/tier/IRI),zoom/pan cytoscape 原生,图例含四类计数。
- `web/src/components/graph/ontologyApi.ts`(新):X-Admin-Token fetch(localStorage `sf_admin_token`,governanceApi 同款),nil slice 归一,403 → 「需要管理员令牌」可读错误。
- `web/src/components/graph/KnowledgeGraphBrowser.tsx`:顶部 mode chips「本体」(默认)/「文档」(`data-testid="graph-mode-document"`);corpus loading/error/empty hero 全部退到文档模式;全屏 overlay 渲染当前激活模式(本体同样进 overlay);focus prop 双视图贯通。
- focus→notes(两路合并):
  - **chunk_ids 路径(线路实况)**:`resolveFocusNoteIds(chunkIds)`(graph-focus.ts 导出,带单测)对每个 chunk_id 调既有 `fetchChunkContent`(GET /api/knowledge/chunk/:id → source 路径)→ basename 前导数字 → note_XXX;单条 404/网络失败/无数字前缀跳过,全部失败 → [](降级全库);去重保序。解析期间投影 fetch 挂起,loading 态显示「正在解析引用来源…」。
  - **sources 路径(同步)**:`noteIdsFromFocus`(citation source 字段,BE 尚未下发,tolerated future field)。
  - 两路 id 合并后作为 wire 参数;toggle 仅当前问题/全库语义不变。
- 仅当前问题/全库 toggle:本体视图内新做,复用 GraphCanvas 的 `skillforge.graph.focusMode` localStorage key(两个视图共享一个偏好)。
- 文档视图原样保留(GraphCanvas 一行未动)。

**测试:** OntologyGraphView 8 例(cytoscape mock 断言真实 elements/style payload:四类节点/parent_org 边/cose/tap→明细/chunk_id 异步解析→?notes=/全部失败降级全库/toggle refetch/403 提示/空态);KnowledgeGraphBrowser 12 例(默认本体、空 corpus 下本体照常渲染、focus→?notes=note_032、双向切换、overlay 按模式渲染、文档模式全部旧 pin 保留);graph-focus 9 例(noteIdsFromFocus 4 + resolveFocusNoteIds 5,mock fetch)。`npx tsc --noEmit` 0 错;`vitest run src/components/admin src/components/graph src/lib` 146/146 绿。

## Decisions Made

1. **handler 硬编码 skfNS**(同 admin_governance.go 先例),不给 induction.Manager 加 NS() accessor —— 投影是 ABox 数据面,live ns 恒为 emitter.DefaultNS。
2. **测试夹具用 live ns(skf())而非 ontologyTestNS** —— 复用 governance 的 stripNS/skf helper,投影逻辑与 live 数据同构。
3. **未知 note 跳过而非产 ghost 节点**(测试驱动修正):note 无任何三元组时不输出 note 节点,严格满足「note 不存在时返回空 nodes」。
4. **organization 紫色硬编码 #a78bfa**:theme.css 无紫色 token,注释标明;其余三类全部走 token。
5. **admin token 门意味着本体视图依赖 localStorage 已存 sf_admin_token**(admin console 登录后写入)。聊天用户未登录 admin 时,本体视图显示「需要管理员令牌」错误而非白屏。活体验收时用 uat-local-admin。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 未知 note 请求产出了 ghost note 节点**
- **Found during:** Task 1(自写 ht 断言抓到)
- **Issue:** 初版实现无条件 acc.node(noteIRI,...),`notes=note_999` 返回 `{note_999, "999"}` 而非空 nodes,违反任务规格。
- **Fix:** 投影前 `len(kg.Query(noteIRI,"",""))==0` → skip。
- **Commit:** bb5d3d5(同一 commit 内修复)

**2. [Rule 1 - Bug] focus 聚焦永远降级全库(coordinator follow-up,13d782e)**
- **Found during:** 验收前审查 — 线路 verdict citations 只带 chunk_id/span/tier,`source` 字段不存在,sync 派生恒空;且初版 `fetchOntologyGraph` 内部只从 focus.sources 重推 notes,组件解析出的 chunk note ids 根本到不了 URL(测试 spy 抓到第 3 个 fetch 无 notes 参数)。
- **Fix:** FE-only——`resolveFocusNoteIds` 走既有 chunk-content 端点逐条解析(单条失败跳过,全失败降级全库);`fetchOntologyGraph(notes: string[])` 改收已解析 id;组件解析期挂起投影 fetch。
- **Commit:** 13d782e

其余按计划执行,无计划外文件。cytoscape 零新依赖确认(3.34.3 已在 package.json)。

## Known Stubs

无 stub。citation `source` 直传字段仍为 tolerated future wire field,但聚焦语义已由 chunk_id 解析路径兜住,不再依赖它。

## 活体验收(orchestrator)

需重启后端(WSL runbook,当前 binary 不含新端点)后:
1. 提问(如 保单支付方式)→ 知识图谱默认显示本体视图;若 citation 带 source,则聚焦被引 note 的 表/险种/机构 邻域,toggle 可切全库。
2. 全库模式显示实体关系网(note 蓝 / 表 橙 / 险种 绿 / 机构 紫,层级虚线)。
3. 快速验证端点:`curl -s "http://[::1]:8888/api/admin/ontology/graph?notes=note_032" -H "X-Admin-Token: uat-local-admin"`。

## Self-Check: PASSED

- 6/6 关键文件存在(BE handler + ht、FE view + api + focus helper + SUMMARY)
- 3/3 commits 在分支上:`bb5d3d5`(BE)、`93daf7d`(FE)、`13d782e`(chunk 解析 fix)
- 验证门全过:WSL `go test ./internal/server/` 全绿;`tsc --noEmit` 0 错;vitest admin+graph+lib 146/146(含 chunk 解析 5 pins + 降级 pin)
