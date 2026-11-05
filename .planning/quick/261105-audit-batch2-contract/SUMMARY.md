---
type: quick-task
status: complete
date: 2026-11-05
plan: .planning/quick/261105-audit-batch2-contract/PLAN.md
audit_source: .planning/reports/CODE-AUDIT-2026-11-05.md
fixes: [A-03, A-04, A-05, A-06, A-10, B-01, B-02, B-03, B-07, C-02, C-04, C-06, D-01, D-03, D-05, D-08, D-09]
---

# SUMMARY — 批次 2 契约对齐

## 后端

### A-03 — 上游装配收敛为唯一工厂
- 新增 `internal/llm/upstream.go`：`BuildUpstream(UpstreamEnv)` 是文件配置 →
  plan 回落 → LocalThinkOffModel、无文件 → env preset 回落的**唯一**装配点。
- `cmd/server/main.go` 的 buildUpstream 闭包与 `llm_admin.go` 的
  applyLLMConfig resolver 都改为调用它——面板保存一次就顶掉启动语义
  （丢 env 回落、丢 LOCAL_LLM_THINK）的分叉在构造上不可能再发生。
- `errNoFileConfig` / `errUnknownVendorPlan` 哨兵与 openai 导入随之清理。

### A-04 — keyless 切模型端点门槛修正
- `llmModelHandler` 删除 `cfg.BaseURL == ""` 硬门槛（合法的目录厂商配置
  不再被 400「先去面板保存」）；配置损坏 → 500 + 原始错误，配置缺失 → 400，
  两种语义分开。

### A-05/A-06/A-10 — thinking 策略按实际端点判定（能力白名单）
- `llmconfig.EffectiveBaseURL()`：文件配置（含 plan 回落）优先，env preset
  （LOCAL_/GLM_BASE_URL）兜底——实际拨号端点的单一事实来源。
- `llmconfig.SupportsThinkingDisabled()`：能力白名单（当前仅
  open.bigmodel.cn），替代 `!= "minimax"` 声明值黑名单。
- `PipelineExtraFields()` 改用能力判定；`DefaultGLMBaseURL` 由 llmconfig
  导出，main.go / upstream.go 共用（消除三处字面量漂移）。
- 新增 `current_test.go` 锁定：minimax 经 GLM_* env 部署不再全量 400/2013；
  vendor 声明与端点不一致时以端点为准；空配置回落默认 GLM。

### B-01 — RETRIEVER_BACKEND=none 上传路径 panic
- `retriever.DisabledIndexer`（Insert 恒返回 `ErrIndexerDisabled`）替代
  nil 接口装配；`upload.go` STEP 8 加 nil 守卫 + 禁用错误归入 503 分类。

### B-02 — 维度错配启动期断言
- milvus 分支在拨号前断言 `emb.Dimensions() == milvus.DefaultEmbedDim`，
  不匹配 Fatalf 并给出可操作提示——"启动通过、每条检索都死"的静默故障
  不再可能。

### B-03/B-07 — 日志脱敏
- main.go 新增 `redactURL`（剥 userinfo 与 query，解析失败降级占位符）；
  MONGO_URI 的 Fatalf 与 backendLabel、BGE_M3_URL 的 WARN 全部走脱敏。

### C-06 — span 原文子串兜底
- `extract.go` renderVerdict：`firstStatementSpan` 结果仅在
  `strings.Contains(content, s)` 时才覆盖 firstSpan——多围栏跨拼接点的
  非原文 span 不再导致无谓的 grounding 降级。

### C-04 — 降级答案不再冒充「结论」
- `chat.go` conclusionText：Status != "ok"（grounding 失败降级）时标题
  改为「**未经核验的候选答案**」；「**结论**」保留给通过核验的 verdict。

### C-02 — [ref-N] 分帧头伪造（两层防御）
- `stubchat.extractRefBlocks` 只扫证据区（`Retrieved chunks (DATA` 标记行
  之后）——query 区的伪造头不再可能成为 blocks[0] 劫持答案；
- `pipeline.go neutralizeFramingPrefix`：chunk 正文里处于行首且形状匹配
  块头的行注入零宽空格，伪造头不再满足行首匹配（LLM 与 stub 双受益）；
  `chunksToBlock`（classifier 输入）同源处理。
- 新增防回归测试：query 区伪造头不产生块。

## 前端

### D-03/D-08/D-09 — ModelPicker 加固
- localStorage 读/写全部 try/catch（禁存储浏览器降级为内存态，不再白屏）；
  load() 成功时清 stale error；两个加载入口共享一个取消 ref（卸载保护）。

### D-05 — DataSubprogressPart 空值防御
- `part.data` 改为可选读取 + `if (!data) return null`（hook 仍无条件运行）；
  双重 `as` 断言收敛。

### D-01 — 附件静默丢弃的最小修复
- 有附件时禁用发送按钮、芯片栏明示「附件通道尚未接入，暂不会随消息发送」；
  阻止"用户以为文档已提交"的静默数据丢失（真实上传通道另行立项）。

## 验证
- `go build ./...` ✅ exit 0
- `go test` llm / llmconfig / retriever{embedded,milvus,mongo} / wire /
  verdict / stubchat ✅ 全 ok；`go test ./internal/server/ -skip <存量
  duckdb flake 族>` ✅ exit=0
- `cd web && npx tsc --noEmit` ✅；chat 组件 7 文件 31 测试 ✅

## 遗留（批次 3）
A-11（读盘热路径/TOCTOU）、A-12..A-18、B-04..B-09（除已修）、C-03/C-05/
C-07..C-11、D-02/D-04/D-06/D-07/D-10..D-12、E-01..E-16、G-01..G-23
（见审计报告 §5.3）。
