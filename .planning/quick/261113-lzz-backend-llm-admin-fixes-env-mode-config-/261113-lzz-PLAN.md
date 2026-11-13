---
status: ready_for_execution
task: 261113-lzz
created: 2026-11-13
---

# 261113-lzz PLAN — 后端 LLM admin：env 模式报告真实生效模型 + 探针凭据 env 兜底

## 背景（勘察已锁定，勿重复勘察）

用户报「模型显示这里看不见名称」「点击拉取模型没有响应」。前端已做兜底（261113-luu/lhu），但**根因在后端**：

### 缺陷 1：env 模式报告的模型名与实际生效模型不一致
- `internal/server/llm_admin.go:148` env 分支：`Model: envOr("LOCAL_MODEL", envOr("GLM_MODEL", ""))` —— **无默认值**
- 实际生效（`internal/llm/upstream.go:98`、`cmd/server/main.go:346`）：`envOr("LOCAL_MODEL", envOr("GLM_MODEL", "glm-5.3-flash"))` —— **有默认值**
- 于是 LOCAL_MODEL/GLM_MODEL 都未设时：服务实际在跑 `glm-5.3-flash`，而 `/api/admin/llm/config` 报 `model: ""` → 前端只能显示"未设置模型"。**面板报告的事实 ≠ 系统实际状态**。

### 缺陷 2：env preset 模式拉不到模型列表
- `resolveProbeKey`（llm_admin.go:371-389）只认两种来源：白名单 env 名（`api_key_env`）、已存文件配置
- env preset 模式（无 `~/.skillforge/llm.json`）两者皆无 → key 为空 → 退化成 `"anonymous"`（:414）→ 真实网关拒绝 → 502 "model list failed"
- 且 `resolveProbeURLs` 对 `vendor="env"`（不在 catalog）直接回传请求值，`models_url` 为空 → `ListModels` 需从 baseURL 推导

## Task 1 — 单一事实来源：默认模型常量

**files:** `internal/llmconfig/current.go`, `internal/llm/upstream.go`, `cmd/server/main.go`

**action:**
1. `internal/llmconfig/current.go` 在 `DefaultGLMBaseURL` 旁新增：
```go
// DefaultGLMModel 是 env preset 路径（无文件配置）的默认模型 id，
// 与 upstream.go / cmd/server 的 env 回落语义一致。导出它是为了让
// 「实际生效模型」与「面板报告模型」共用同一事实来源，避免字面量漂移。
const DefaultGLMModel = "glm-5.3-flash"
```
2. `internal/llm/upstream.go:98` 字面量 → `envOr("LOCAL_MODEL", envOr("GLM_MODEL", llmconfig.DefaultGLMModel))`
3. `cmd/server/main.go:346` 字面量 → 同上（确认 main.go 已 import llmconfig，未 import 则加）

## Task 2 — env 分支报告真实生效模型

**files:** `internal/server/llm_admin.go`

**action:** :148 改为 `Model: envOr("LOCAL_MODEL", envOr("GLM_MODEL", llmconfig.DefaultGLMModel))`。
同时 `BaseURL`（:147）也补默认 `llmconfig.DefaultGLMBaseURL`（env preset 下 catalog 回落之外的兜底），使报告的端点同样等于实际拨号端点。

## Task 3 — 探针凭据 env preset 兜底

**files:** `internal/server/llm_admin.go`

**action:** `resolveProbeKey` 增加第三条来源（**在文件配置之后、白名单 env 之后**）：
```go
// env preset 兜底：无文件配置时，用 env 里的凭据，但只发给 env 自己
// 配的那个端点（baseURL 必须逐字等于 env base）——不扩大 key 的外泄面。
if baseURL != "" && baseURL == llmconfig.EffectiveBaseURL() {
    if v := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY")); v != "" {
        return v
    }
}
```
注意：`EffectiveBaseURL()` 在有文件配置时返回文件配置端点 —— 此时上面的文件配置分支已先命中并 return，所以本分支只会在 env preset 路径生效。

## Task 4 — 测试

**files:** `internal/server/llm_admin_test.go`（必要时 `internal/llmconfig/current_test.go`）

**action:** 新增/补齐：
1. env preset 模式（清空 LOCAL_*/GLM_* 文件配置路径、设 GLM_API_KEY）下 GET /api/admin/llm/config 的 `config.model` == `glm-5.3-flash`（或 env 里显式设的模型名）——用 `t.Setenv` 隔离环境
2. env preset 模式 + httptest 上游，GET /api/admin/llm/models 带 base_url=env base → 返回模型列表（凭据来自 env），不再是 502
3. 回归：文件配置模式行为不变（现有测试全绿即可）

**verify:**
- `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go build ./... && CGO_ENABLED=1 /usr/local/go/bin/go vet ./internal/... ./cmd/... && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/server/ ./internal/llmconfig/ ./internal/llm/ -count=1'`

## 收口

- 单 commit（后端 + 测试 + 规划文档），message 简体中文 conventional 风格
- 前端不动（前端兜底已就位，后端修好后前端直接显示真实模型名）
- backlog 注记：env preset 模式下 POST /api/admin/llm/model（切模型）仍要求先有文件配置（llm_admin.go:463-466）——本轮不扩范围
