# Requirements — v1.7 Open-Source Readiness

**Milestone Goal:** 把"核心引擎产品级、门面个人项目级"的落差补齐——外部用户 clone→跑通→看懂→信任，确立信任边界赛道的开源门面。

**Source:** 2026-10-13 缺口审查（vs Dify/RAGFlow/FastGPT 等明星项目 + 商业标准）。

---

## 开源门面 (FACE)

- [x] **FACE-01**: README 与实际一致——`docs/demo.gif` 破图修复（真实运行素材）、示例 skill 数量更正为 7、badge 链接全部有效
- [x] **FACE-02**: 新访客可理解架构——`docs/architecture.md` 组件图（agent 内核/知识层/治理层/平台层）+ README 中 vs Dify/RAGFlow 差异化对比表
- [x] **FACE-03**: 社区设施齐备——SECURITY.md、CODE_OF_CONDUCT.md、GitHub issue/PR 模板

## CI 矩阵 (CICD)

- [x] **CICD-01**: Go CI 进 GitHub Actions——golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet，PR/push 触发
- [x] **CICD-02**: FE CI——`tsc --noEmit` + `vitest run` 全量
- [x] **CICD-03**: Docker 镜像构建验证——backend/frontend 双 Dockerfile 在 CI 中可构建

## Hermetic Demo (DEMO)

- [x] **DEMO-01**: 无内网依赖的检索链——embedded 向量兜底模式（无需外部 Milvus/BGE 即可检索）
- [x] **DEMO-02**: 无 API key 路径——stub LLM chat（既有 stub 抽取器延伸），全链可跑
- [x] **DEMO-03**: quickstart 实测通过——hermetic 模式下 README quickstart 逐步验证（产出 demo.gif 素材，反哺 FACE-01）

## 清债与发布 (REL)

- [ ] **REL-01**: D-46-DEF-02 修复——`TestDeriveEdgesDeterministic` **边方向**确定性化（根因 = `internal/knowledge/graph_edges.go::ComputeEdges` 中 `sources` 由 `sourceTokens` 的 map 迭代序构造，`i<j` 配对使 `From`/`To` 随迭代序翻转，而 `EdgeGraph.Neighbors` 为有向遍历；修复 = 配对前确定性排序 + `Weight` 作第四排序键）。实测：`-count=200` 两次分别 34 败 / 50 败（17–25%），失败全落在 `From`/`To`/`EdgeType` 断言，`Weight` 断言 0 败；验收强度 = `-count=100` 全绿 + 一次 `-shuffle=on`（原「连续 6 次」在 ~20% flake 率下置信度不足，仅作冒烟门槛）
- [ ] **REL-02**: audit WARNING-1 修复——rollback 成功路径刷新 SHACL 报告缓存（或修正注释），FE 死分支恢复
- [ ] **REL-03**: 发布工程——goreleaser 配置 + tag 触发 GitHub Release（二进制 + release notes）
- [ ] **REL-04**: BL-01..04 补录进 REQUIREMENTS Traceability 表

## Future Requirements（v1.8+ 商业分期）

- 身份体系（用户/RBAC/SSO，替代单共享 X-Admin-Token）
- 多租户隔离
- OTel 可观测（eino-ext callbacks 接线）
- OpenAPI 契约
- rate limiting

## Out of Scope

- **可视化编排 UI**（Dify/Flowise 赛道）——Skill 文件即契约是设计选择，不卷编排赛道
- **插件/工具市场**——MCP 5 tools 已覆盖平台层展示
- **provider 注册矩阵 UI**——openai-compatible 适配已够 demo
- **模型微调/训练**——PROJECT.md 既有否决，成本与定位不符

---

## Backlog Items Carried From v1.6

v1.6 遗留 backlog（继承进 v1.7 Traceability 表，REL-04 补录）。定义逐字引自 `.planning/milestones/v1.6-REQUIREMENTS.md` § Accumulated Backlog（第 40-43 行）——归档只读，本文件仅引用不改动归档。

- **BL-01**: 双 resolver 去重（`resolveSkillTiers` vs `resolveSkillTiersFromKG`） — 来源 `.planning/milestones/v1.6-REQUIREMENTS.md:40`
- **BL-02**: server 包 lint 清扫 — 来源 `.planning/milestones/v1.6-REQUIREMENTS.md:41`
- **BL-03**: live 抽取 prompt 对齐 writes-only 的残余微调 — 来源 `.planning/milestones/v1.6-REQUIREMENTS.md:42`
- **BL-04**: DetailPanel/emitter 两个 deferred 测试 — 来源 `.planning/milestones/v1.6-REQUIREMENTS.md:43`

去向（与 `47-DEBT-DISPOSITION.md` 主表一致）：BL-04 = v1.7 修复（plans 47-03 + 47-05）；BL-01/BL-02/BL-03 = 延期+理由（分别是行为语义重构需单独立项、需 Phase 48 的 `golangci-lint` 报告才有可定义范围、needs-live 内网 LLM/BGE）。

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| FACE-01 | Phase 50 | Complete |
| FACE-02 | Phase 50 | Complete |
| FACE-03 | Phase 50 | Complete |
| CICD-01 | Phase 48 | Complete (run 39380373930) |
| CICD-02 | Phase 48 | Complete (run 39380373930) |
| CICD-03 | Phase 48 | Complete (run 39380373930) |
| DEMO-01 | Phase 49 | Complete |
| DEMO-02 | Phase 49 | Complete |
| DEMO-03 | Phase 49 | Complete |
| REL-01 | Phase 47 | Complete |
| REL-02 | Phase 47 | Complete |
| REL-03 | Phase 51 | Pending |
| REL-04 | Phase 47 | Complete |
| BL-01 | Backlog（无 owner phase） | Deferred |
| BL-02 | Backlog（Phase 48 lint 落地后立项） | Deferred |
| BL-03 | Backlog（needs-live） | Deferred |
| BL-04 | Phase 47 | Complete |

Coverage: 17/17 mapped items（13 项 v1.7 requirements：FACE-01..03 / CICD-01..03 / DEMO-01..03 / REL-01..04，+ 4 项 v1.6 遗留 backlog BL-01..04，由 REL-04 补录；BL-01..03 维持 Deferred 并附理由，BL-04 在 Phase 47 闭合 = Complete）。其余 v1.6 遗留项 D-46-DEF-02 / WARNING-1 由 REL-01/02 吸收；D-45-DEF-01/02/03、D-46-DEF-01、WARNING-2 在 Phase 47 逐项落处置记录（`47-DEBT-DISPOSITION.md`）。
