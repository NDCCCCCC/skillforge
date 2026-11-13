---
status: ready_for_execution
task: 261113-ixo
created: 2026-11-13
---

# 261113-ixo PLAN — LLM Provider 面板 trim 崩溃修复

## 背景（勘察已锁定，勿重复勘察）

- 现象：`source=env` 时面板报 `Cannot read properties of undefined (reading 'trim')`，点击"拉取模型列表"无网络请求（TypeError 被 onModels catch 显示为红字）。
- 根因：`web/src/components/admin/llm/LLMProviderPanel.tsx:93` `setApiKeyEnv(cfg.config.api_key_env)` —— 后端 config 的 `api_key_env` 可能缺省（omitempty / env 来源推导），state 变 `undefined`；`buildRequest()`（:131-132）`apiKeyEnv.trim()` 抛 TypeError。
- 前端类型 `LLMConfigView`（llmApi.ts:29）声明 `api_key_env: string` 是对后端契约的乐观假设，运行时不成立。

## Task 1 — loadAll 防御回填 + buildRequest 兜底

**files:**
- `web/src/components/admin/llm/LLMProviderPanel.tsx`

**action:**
1. `loadAll`（:88-94）所有来自 `cfg.config` 的回填加空值兜底：
   ```ts
   setVendor(cfg.config.vendor ?? '');
   setPlan(cfg.config.plan ?? '');
   setBaseUrl(cfg.config.base_url ?? '');
   setModel(cfg.config.model ?? '');
   setApiKeyEnv(cfg.config.api_key_env ?? '');
   ```
2. `buildRequest`（:129-134）兜底：`const envName = apiKeyEnv ?? '';` `const plainKey = apiKey ?? '';` 用它们做 trim 判断与赋值（行为不变，仅消 undefined）。

**verify:** `cd web && npx vitest run src/components/admin/llm/LLMProviderPanel.test.tsx`（存在则跑；exit 0）

## Task 2 — 回归测试锁定 undefined 契约

**files:**
- `web/src/components/admin/llm/LLMProviderPanel.test.tsx`（若无则新建，参考现有测试的 mock 手法）

**action:** 增加一条测试：mock `GET /api/admin/llm/config` 返回 `config` 对象**缺 `api_key_env` 字段**（模拟 omitempty），挂载面板后点击"拉取模型列表"，断言不抛 trim TypeError、面板发出 `/api/admin/llm/models` 请求（或 msg 显示非 TypeError 文案）。

**verify:** 同 Task 1 的 vitest 命令，新测试通过。

## 收口门禁

- `npx vitest run src/components/admin/llm/` 全绿
- 提交：单 commit（改动同属一个修复），message 简体中文 conventional 风格
- 不碰后端；不改 llmApi.ts 类型（类型契约问题留 backlog 注明即可）
