---
status: ready_for_execution
task: 261117-dvu
created: 2026-11-17
---

# 261117-dvu PLAN — 修 LLM 厂商目录的 JSON 字段名（下拉空白 bug）

## 背景（已实证，勿重复勘察）

用户报：admin「LLM Provider 配置」面板的**厂商下拉是空的**，且没有出现「连接 / 输入 Admin Token」卡片。

**实证根因**（不是 token 问题）：后端 `internal/llmconfig/catalog.go` 的 `Vendor`/`Plan` 结构体**没有 json tag**，Go 默认序列化为**大写字段名**。用 Go 实际跑 `json.Marshal(llmconfig.Catalog())` 得到：

```json
[{"ID":"zhipu","Label":"智谱 GLM","Plans":[{"ID":"coding-plan","Label":"...","BaseURL":"...","ModelsURL":"","Note":"..."}]}]
```

而前端 `web/src/components/admin/llm/llmApi.ts` 的 `LLMVendor`/`LLMPlan` 接口期望**小写** `id`/`label`/`plans`/`base_url`/`models_url`/`note`，组件里 `vendors.map(v => <option key={v.id}>{v.label}</option>)` 全部拿到 `undefined` → `<select>` 只有占位行「选择厂商…」，看起来就是空的。

**为什么测试没抓到**：`internal/server/llm_admin_test.go:89-96` 的 `TestLLMAdmin_ProvidersCatalog` 只断言 JSON 里含字符串 `"zhipu"`/`"opencode"`（值大小写无关），**没断言字段名**，所以字段全大写也能过。

**为什么没出现 Token 卡片**：面板的 `!token` 分支只在 localStorage 无 `sf_admin_token` 时渲染连接卡片；用户环境里曾存过（或 token 通过别的途径注入），所以卡片不出现。这与下拉空白无关。

## Task 1 — 给结构体加 json tag

**files:** `internal/llmconfig/catalog.go`

**action:** 给 `Plan` 与 `Vendor` 的每个字段加与前端契约一致的 json tag：

```go
type Plan struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	BaseURL   string `json:"base_url"`
	ModelsURL string `json:"models_url"`
	Note      string `json:"note"`
}

type Vendor struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Plans  []Plan `json:"plans"`
}
```

**注意**：`base_url`/`models_url` 是否 omitempty？前端 `LLMPlan` 声明为必填 `string`，且 `resolveProbeURLs` 依赖空串语义（custom 的 ModelsURL 为空串表示"用 BaseURL+/models"）→ **不要 omitempty**，保持空串也序列化出来。

## Task 2 — 补字段名回归测试（关键，防再漏）

**files:** `internal/server/llm_admin_test.go` 的 `TestLLMAdmin_ProvidersCatalog`

**action:** 把断言加强为**字段名级**：
```go
// 不能只断言值存在（"zhipu" 大小写无关）——必须断言 JSON 字段名确实是
// 前端契约的小写，否则结构体漏写 json tag 时全大写也能过这测。
var typed []struct {
    ID    string `json:"id"`
    Label string `json:"label"`
    Plans []struct {
        ID      string `json:"id"`
        BaseURL string `json:"base_url"`
    } `json:"plans"`
}
require.NoError(t, json.Unmarshal(rawVendors, &typed))
require.NotEmpty(t, typed[0].ID, "vendors[0].id 必须存在（当前若为空说明后端返回的是大写 ID）")
require.NotEmpty(t, typed[0].Label)
require.NotEmpty(t, typed[0].Plans)
require.NotEmpty(t, typed[0].Plans[0].ID)
```
（保留原有的字符串包含断言也行，但要新增上面这段。）

## Task 3 — 前端契约复核（不改代码，仅确认）

读 `web/src/components/admin/llm/llmApi.ts` 的 `LLMVendor`/`LLMPlan` 与 `LLMProviderPanel.tsx` 的 `pickVendor`/`pickPlan`（`v.id`/`v.plans[0].id` 等），确认后端加了 tag 后完全对齐。**不要改前端**（前端本来就是对的，错在后端）。

## verify

- `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go build ./... && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/server/ ./internal/llmconfig/ -count=1'` exit 0
- 用之前的 Go 探针再跑一次 `json.Marshal(llmconfig.Catalog())`，确认输出是小写 `id`/`label`/`plans`
- （可选，若浏览器可用）刷新 admin 面板，厂商下拉出现选项

## 收口

- 单 commit，message 简体中文 conventional 风格
- 只改这两处文件 + 规划文档
