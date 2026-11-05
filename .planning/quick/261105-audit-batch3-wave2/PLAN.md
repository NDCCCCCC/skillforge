---
type: quick-task
status: in-progress
date: 2026-11-05
scope: 批次 3 工程质量（第二波）— CODE-AUDIT-2026-11-05.md §5.3
fixes:
  - D-02 Admin Token localStorage key 双轨统一（sf_admin_token + 兼容 skillforge.adminToken）
  - E-07 LLMProviderPanel onSave 刷新失败不被覆盖
  - G-08 ProposalDrawer 跨 proposal 状态泄漏
  - G-10 DocUpload reset 清理 file input
  - G-04 / G-05 FeedbackMetricsPanel / ReviewQueuePanel 请求竞态
  - A-13 分类器置信度下限检查与 Valid() 冗余清理
  - A-18 ExtraFieldsModel 零值与 nil 保护
---

# 批次 3（第二波）：工程质量收尾

## 任务清单
1. **D-02 / E-07**: `web/src/components/admin/llm/LLMProviderPanel.tsx` 统一 token key 读取，保持向后兼容；onSave 时若 loadAll 失败保留失败信息。
2. **G-08**: `web/src/components/admin/ontology/SchemaProposalsTab.tsx` / `ProposalDrawer.tsx` 添加切换 proposal 时重置状态或 key 挂载。
3. **G-10**: `web/src/components/admin/DocUpload.tsx` 为 input 添加 ref，reset 时重置 file input 节点。
4. **G-04**: `web/src/components/admin/FeedbackMetricsPanel.tsx` 增加 requestId 防旧请求覆盖新维度。
5. **G-05**: `web/src/components/admin/ReviewQueuePanel.tsx` 增加 requestId 防旧请求覆盖新状态。
6. **A-13**: `internal/server/classifier/classifier.go` 简化置信度下限检查，避免与 `Valid()` 冗余。
7. **A-18**: `internal/llm/extrafields.go` 为 `ExtraFieldsModel` 方法增加 nil 保护。

## 验证
- `go test ./...`
- `cd web && npx tsc --noEmit`
- `npx vitest run` 相应测试
