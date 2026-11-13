---
status: complete
task: 261113-ixo
completed: 2026-11-13
---

# 261113-ixo SUMMARY — LLM Provider 面板 trim 崩溃修复

## 改动

### Task 1 — `web/src/components/admin/llm/LLMProviderPanel.tsx`

1. **loadAll 防御回填**（`cfg.config` 分支）：五个 setter 全部加 `?? ''` 兜底
   （`vendor` / `plan` / `base_url` / `model` / `api_key_env`），并附注释说明
   后端 omitempty / env 推导可使任意字段运行时缺省、`LLMConfigView` 类型是乐观假设。
2. **buildRequest 兜底**：`const envName = apiKeyEnv ?? '';` `const plainKey = apiKey ?? '';`
   trim 判断与赋值改用这两个局部量（行为不变，仅消 undefined）。

### Task 2 — `web/src/components/admin/llm/LLMProviderPanel.test.tsx`

新增回归测试：`config missing api_key_env (env/omitempty) must not crash trim on 拉取模型列表`
- mock `GET /api/admin/llm/config` 返回 `source=env` 且 config **缺 `api_key_env` 字段**（含注释：runtime contract ≠ LLMConfigView 类型）
- 挂载面板后点击「拉取模型列表」
- 断言：msg 显示「拉到 2 个模型」（fetch mock 返回两个模型）、`/api/admin/llm/models` 请求确已发出、msg 文案不含 `trim`（修复前 TypeError 文案特征）

## 测试执行结果

**BLOCKED（环境权限）**：`cd web && npx vitest run src/components/admin/llm/` 两种形式（带/不带管道）均被本会话 Bash "don't ask mode" 权限拒绝（`npx vitest` 命令级拒绝；同会话 `ls` 可执行）。按执行纪律跳过运行，**测试门禁未验证，需编排方在验证阶段执行**。文件编辑照常完成。

## 偏差

- 无计划外文件改动。未提交 commit（按指令留给编排方统一提交）。
- 类型契约问题（`llmApi.ts` 的 `LLMConfigView.api_key_env: string` 乐观假设）按计划未改动，留 backlog：
  > `llmApi.ts` LLMConfigView 各 string 字段应对齐后端 omitempty 契约改为可选（`api_key_env?: string` 等），属独立重构，未纳入本 quick 任务。
