# Requirements — v1.7 Open-Source Readiness

**Milestone Goal:** 把"核心引擎产品级、门面个人项目级"的落差补齐——外部用户 clone→跑通→看懂→信任，确立信任边界赛道的开源门面。

**Source:** 2026-10-13 缺口审查（vs Dify/RAGFlow/FastGPT 等明星项目 + 商业标准）。

---

## 开源门面 (FACE)

- [ ] **FACE-01**: README 与实际一致——`docs/demo.gif` 破图修复（真实运行素材）、示例 skill 数量更正为 7、badge 链接全部有效
- [ ] **FACE-02**: 新访客可理解架构——`docs/architecture.md` 组件图（agent 内核/知识层/治理层/平台层）+ README 中 vs Dify/RAGFlow 差异化对比表
- [ ] **FACE-03**: 社区设施齐备——SECURITY.md、CODE_OF_CONDUCT.md、GitHub issue/PR 模板

## CI 矩阵 (CICD)

- [ ] **CICD-01**: Go CI 进 GitHub Actions——golangci-lint + `go test ./...` + CGO_ENABLED=1 build+vet，PR/push 触发
- [ ] **CICD-02**: FE CI——`tsc --noEmit` + `vitest run` 全量
- [ ] **CICD-03**: Docker 镜像构建验证——backend/frontend 双 Dockerfile 在 CI 中可构建

## Hermetic Demo (DEMO)

- [ ] **DEMO-01**: 无内网依赖的检索链——embedded 向量兜底模式（无需外部 Milvus/BGE 即可检索）
- [ ] **DEMO-02**: 无 API key 路径——stub LLM chat（既有 stub 抽取器延伸），全链可跑
- [ ] **DEMO-03**: quickstart 实测通过——hermetic 模式下 README quickstart 逐步验证（产出 demo.gif 素材，反哺 FACE-01）

## 清债与发布 (REL)

- [ ] **REL-01**: D-46-DEF-02 修复——`TestDeriveEdgesDeterministic` 浮点 Weight 确定性化，6/6 稳定
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

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| *(由 roadmap 填充)* | | |
