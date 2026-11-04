---
type: quick-task
status: complete
date: 2026-11-05
plan: .planning/quick/261105-audit-batch1-security/PLAN.md
audit_source: .planning/reports/CODE-AUDIT-2026-11-05.md
fixes: [F-01, A-07, F-02, E-02, A-02, E-01, A-01, G-01, C-01]
---

# SUMMARY — 批次 1 安全止血

## 修复内容

### T1+T2 — llm_admin.go / probe.go（F-01 + A-07 + F-02 + E-02 + A-02）
- `llmModelsHandler`：**移除 `api_key` query 参数**（明文 key 不再经 URL 进 nginx
  access_log / 浏览器历史 / Referer）；凭据只从 `api_key_env`（白名单）或已存配置解析。
- `resolveProbeURLs`：catalog 厂商的 `base_url`/`models_url` 仅在与目录值一致时被采纳，
  否则强制目录值（SSRF 加固）；`vendor=custom` 保留请求值（ollama 等自建端点）。
- `resolveProbeKey`：已存配置的 key 仅在最终 base_url 与已存配置一致时回落——
  关闭"把服务端 key 发往任意主机 + 经错误回显读回"的闭环。
- `envWhitelistedEnvName`：`api_key_env` 仅接受 `SKILLFORGE_*` 前缀
  （防任意环境变量如 AWS_SECRET_ACCESS_KEY 外泄）。
- `resolveTestTarget`：显式自带 key 的测试请求允许自选端点（调用方自身凭据）；
  依赖服务端凭据时强制 URL 对齐 + env 白名单。
- `probe.go`：`probeClient` 加 CheckRedirect（同主机、最多 3 跳）；
  `parseModelList` 不再回显上游响应体（断掉 key 读回通道）。

### T3 — llmSaveHandler（E-01）
- 请求未携带任何 key 字段时保留已存配置的 `APIKey`/`APIKeyEnv`——
  "API Key 留空则不改动已存 key" 的 UI 承诺兑现；首次保存无 key 仍 400。

### T4 — extrafields.go（A-01）
- `ExtraFieldsModel.Generate/Stream`：包装字段放 opts **最前**（调用方优先），
  `generateOptions` 分配新切片（不复用调用方底层数组）。
- 库的 impl-specific 类型未导出（`openaiOptions`），逐键合并不可行；
  替代正确解：`PipelineExtraFields()` 在 `LOCAL_LLM_THINK=false` 时自带 `think:false`，
  使 pipeline/classifier/rewriter 的调用方集合自足——`response_format`/`thinking`
  在两种顺序下都不再丢失（分类器不再 fail-closed 拒答）。
- 新增 `extrafields_test.go` 锁定顺序语义与 think 标志（A-16 零覆盖缺口的开始）。

### T5 — KGQueryPanel.tsx（G-01）
- `runQuery(skillOverride?)`：Skill select 的 onChange 显式传新值，
  消除陈旧闭包导致的"全量查询被标成所选 Skill 视角"范围误报。

### T6 — server.go（C-01）
- 非导入路由 body 上限：`ContentLength()<0` 时拒绝**流式** body（chunked 向量），
  缓冲 body 直接量长——堵住公开 `/api/chat` 的 2 GiB 内存 OOM 通道。

## 验证
- `go build ./...` ✅
- `go test ./internal/llm/ ./internal/llmconfig/ ./internal/wire/` ✅ 全绿
- `go test ./internal/server/ -skip "TestImport|TestPreviewEndpointShape|TestCommitViaPreview|TestPushDryRunFlag|TestDeleteDataset"` ✅ **exit=0**
- `cd web && npx tsc --noEmit` ✅；`OntologyTab.test.tsx` 8/8 ✅

## 存量失败（与本批无关，已用干净树 stash 对照证实）
- `TestImportStreaming` 等整个 duckdb import 测试族在本机**全量运行时**崩溃于
  `go-duckdb` CGO（`duckdb_destroy_config` 访问冲突）；干净树全量同样 FAIL（exit=1），
  单测可过——顺序相关的本机 flake，建议另行跟进（记入审计报告附录）。

## 遗留（批次 2/3）
- A-03/A-04/A-05/A-06/A-08/A-09/A-10/A-11、B-01..B-08、C-02..C-11、D-01..D-12、
  E-03..E-16、G-02..G-23（见审计报告 §5.3）。
