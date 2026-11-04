---
type: quick-task
status: in-progress
date: 2026-11-05
scope: 批次 1 安全止血 — CODE-AUDIT-2026-11-05.md §5.2
fixes:
  - F-01/A-07 SSRF + 任意环境变量外泄 + key 读回闭环
  - F-02/E-02/A-02 明文 API Key 进 URL query
  - E-01 明文 key 配置下保存必然 400
  - A-01 ExtraFieldsModel 整体覆盖调用方 extra fields
  - G-01 KG 查询陈旧闭包
  - C-01 chunked body 绕过非导入路由 8MiB 上限
---

# 批次 1：安全止血

## 任务清单

### T1 — llm_admin.go：关闭 SSRF / env 外泄 / key 读回（F-01 + A-07）
- `llmModelsHandler` / `resolveTestTarget`：`base_url`/`models_url` 不再接受任意 URL，
  仅当与 catalog 中 `FindPlan(vendor, plan)` 的 `BaseURL`/`ModelsURL` 一致时才接受；
  自定义端点必须 `vendor=custom`（沿用目录语义）。
- `api_key_env` 只接受 `SKILLFORGE_` 前缀白名单。
- `probe.go:215` 停止回显上游响应体（只回显状态码/形状名）。
- `probeClient` 加 `CheckRedirect`（禁止跨主机重定向）。

### T2 — 明文 key 不再走 query（F-02 + E-02 + A-02）
- 服务端：`llmModelsHandler` 移除 `c.Query("api_key")` 读取；key 一律从
  `api_key_env`（白名单）或已存配置解析。
- 前端：`llmApi.ts fetchLLMModels` 不再序列化 `api_key` 进 URL。
- 更新 `llm_admin_test.go` 中使用 `?api_key=` 的用例。

### T3 — 保存接口"未提供即不改"（E-01）
- `llmSaveHandler`：`req.APIKey == "" && req.APIKeyEnv == ""` 时保留已存配置的
  `APIKey`/`APIKeyEnv`；无已存配置时维持 400。

### T4 — ExtraFieldsModel 改为逐键合并（A-01）
- `Generate`/`Stream`：提取调用方已有 extra fields（经由包装自身实现不可行——
  option 是闭包；改为在调用前合并不可行时，退而求其次：包装层持有唯一 fields 来源，
  调用方 extra fields 场景重新评估）。**实现方式**：不再依赖 option 追加，
  改为在 wrap 时把 fields 固化进 inner model 的构造（对 openai model 用其构造参数不可行）——
  采用最小正确解：Generate/Stream 内先探测调用方是否传了 WithExtraFields 不现实，
  故改为"调用方优先"语义：包装器在 `append` 前记录自己 option 的位置，依赖"后者胜"顺序
  反转为"调用方胜"：把 `m.fields` 放在 opts **前面**、并在文档中声明覆盖顺序。
  简单且正确：`WithExtraFields` 是赋值语义 ⇒ 谁在后面谁赢 ⇒ 调用方 fields 放后面即可。

### T5 — KGQueryPanel 陈旧闭包（G-01）
- `runQuery(skillOverride?: string)`，onChange 传 `e.target.value`。

### T6 — chunked body 上限（C-01）
- server.go 中间件：`ContentLength() < 0`（chunked / 无 CL）的非导入请求 → 413 拒绝。

## 验证
- `go build ./...`、`go test ./internal/server/... ./internal/llm/... ./internal/llmconfig/... -count=1`
- `cd web && npx tsc --noEmit`
- 前端相关组件测试若存在则跑
