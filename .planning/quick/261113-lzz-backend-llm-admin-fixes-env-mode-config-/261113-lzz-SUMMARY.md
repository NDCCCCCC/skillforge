---
status: complete
task: 261113-lzz
created: 2026-11-13
completed: 2026-11-13
---

# 261113-lzz SUMMARY — 后端 LLM admin：env 模式报告真实模型 + 探针凭据兜底

## 根因（勘察结论）

用户报「模型显示这里看不见名称」「点击拉取模型没有响应」，前端兜底只是表象，**根因在后端两处**：

1. **面板报告的模型 ≠ 实际生效模型**：`llm_admin.go` env 分支 `Model: envOr("LOCAL_MODEL", envOr("GLM_MODEL", ""))` **无默认值**；而 `internal/llm/upstream.go` 与 `cmd/server/main.go` 的实际生效模型是 `envOr("LOCAL_MODEL", envOr("GLM_MODEL", "glm-5.3-flash"))` **有默认值**。两个 env 都没设时，服务实际在跑 `glm-5.3-flash`，面板却报 `model: ""` → 前端只能显示"未设置模型"。
2. **env preset 模式拉不到模型列表**：`resolveProbeKey` 只认白名单 env 名与已存文件配置；env preset（无 `~/.skillforge/llm.json`）两者皆无 → key 为空 → 退化成 `"anonymous"` 占位 → 真实网关拒绝 → 502。

## 改动

| 文件 | 改动 |
|------|------|
| `internal/llmconfig/current.go` | 新增 `DefaultGLMModel = "glm-5.3-flash"` 常量（照抄同文件 `DefaultGLMBaseURL` 的单一事实来源模式） |
| `internal/llm/upstream.go` | env 模型字面量 → `llmconfig.DefaultGLMModel` |
| `cmd/server/main.go` | 同上 |
| `internal/server/llm_admin.go` | ① env 分支 Model/BaseURL 补默认值（与实际拨号端点一致）；② `resolveProbeKey` 新增第 (3) 条来源：env preset 凭据兜底 —— 仅当 `baseURL == llmconfig.EffectiveBaseURL()`（env 自己配的端点）才回落 `LOCAL_API_KEY`/`GLM_API_KEY`，**发明端点拿不到 key** |
| `internal/server/llm_admin_test.go` | 新增 2 条测试：env preset 下 config 报告真实 model/base_url（含 LOCAL_MODEL 覆盖子用例）；env preset 下 models 列表可拉取（假上游强校验 Bearer key）+ 发明端点不外泄 env key 的安全回归 |

## 偏差

1. **子代理写的安全断言口径错误**（编排方修正）：原断言"发明端点不得收到任何 Authorization 头"，但后端在无凭据时**按设计**发 `Bearer anonymous` 占位（`llm_admin.go:414` —— 部分网关允许无 key 列模型，如 ollama）。改为断言"env 真 key 不得出现"（`strings.Contains(auth, envKey)`），语义正确且不与设计冲突。实现本身无需改动。
2. 子代理补了 `resolveProbeKey` 的 doc 第 (3) 条约束——不补则注释失真。

## 验证

- `go build ./...` + `go vet ./internal/... ./cmd/...` → exit 0
- `go test ./internal/server/ ./internal/llmconfig/ ./internal/llm/ -count=1` → **三包全绿**
- `go test ./internal/... -count=1` → 仅 `internal/eval` 的 `TestShippedFrozenLockIsInSync` FAIL，**已用 HEAD 状态临时 worktree 复跑证明是既有问题**（compliance Skill 的 prompt_hash 冻结锁陈旧，与本次改动零关联）；临时 worktree 已安全移除

## 提交

- 待编排方提交
