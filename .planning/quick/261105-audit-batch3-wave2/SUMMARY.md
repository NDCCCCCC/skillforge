---
type: quick-task
status: complete
date: 2026-11-05
plan: .planning/quick/261105-audit-batch3-wave2/PLAN.md
audit_source: .planning/reports/CODE-AUDIT-2026-11-05.md
fixes: [D-02, E-07, G-08, G-10, G-04, G-05, A-13, A-18]
---

# SUMMARY — 批次 3（第二波）：工程质量收尾

## 修复内容

### D-02 — Admin Token localStorage key 双轨统一
- `LLMProviderPanel.tsx`: 优先读取 `sf_admin_token`，回退兼容 `skillforge.adminToken`；写入时同时保存双 key，彻底解决聊天页与后台面板跨页面授权割裂的问题，同时保证既有单测兼容。

### E-07 — LLMProviderPanel onSave 刷新失败不被覆盖
- `loadAll()` 返回执行状态布尔值；`onSave` 仅在重新加载成功后才覆盖展示 `savedMsg`，若重载失败则保留真实错误文案。

### G-08 — ProposalDrawer 跨 proposal 状态泄漏
- `ProposalDrawer.tsx`: 添加 `useEffect` 监听 `proposal?.id` 变化，切换 proposal 时重置 `renaming`、`customName`、`error` 与 `busy` 状态，防止 A 的自定义名称被误提交审批到 B。

### G-10 — DocUpload reset 清理 file input
- `DocUpload.tsx`: 增加 `fileInputRef`，在 `reset()` 中显式清空原生 `<input type="file">` 的 `value`，避免界面残留文件名且重新选择同名文件无法触发 change 的问题。

### G-04 / G-05 — 前端数据面板请求竞态防护
- `FeedbackMetricsPanel.tsx`: 引入递增 `reqIdRef`，异步回调中若请求已被更新的请求覆盖则静默丢弃旧数据，避免“表头切到新维度但内容显示旧维度”。
- `ReviewQueuePanel.tsx`: 引入递增 `reqIdRef`，防止待复核/处置历史页签或过滤条件快速切换时旧响应覆盖新状态。

### A-13 — 分类器置信度下限检查与 Valid() 冗余清理
- `classifier.go`: 移除冗余重复的 `out.Confidence < ClassifierConfidenceFloor` 检查，完全以 `types.go:Valid()` 作为单一事实来源。

### A-18 — ExtraFieldsModel 零值与 nil 保护
- `extrafields.go`: 为 `Generate`、`Stream`、`WithTools` 补充 nil 检查，在 `m == nil || m.inner == nil` 时返回清晰的错误，避免空指针 panic。

## 验证
- `go build ./...` ✅ BUILD_EXIT=0
- 后端单元测试（llm, llmconfig, classifier, rewrite, stubchat 等）全绿 ✅
- 前端 `npx tsc --noEmit` ✅ TSC_EXIT=0
- 前端测试全跑：**87 passed (87), 702 passed (702)** ✅
