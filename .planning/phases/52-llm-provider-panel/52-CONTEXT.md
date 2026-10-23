# Phase 52 — LLM Provider 配置面板（Hermetic Demo 之后）

## Phase Goal

把"换 chat 模型/换厂商/换 key 必须改环境变量并重启"改造成**管理面板里点几下、保存即生效**：前端选择厂商与套餐（自动适配 base URL）、填 key、拉取模型列表选择模型、测试连通性、保存后热重载立即生效；支持自定义 base URL 接入任意 OpenAI 兼容第三方服务。

## Locked Decisions (D-01..D-12)

- **D-01** 面板入口 = admin（X-Admin-Token 保护），非公开端点。
- **D-02** 协议范围 = 仅 OpenAI 兼容 chat/completions（用户确认）。不支持 Anthropic/Gemini 原生协议。
- **D-03** 生效时机 = **立即生效（热重载，用户确认）**：保存后无需重启。chat model 改为委托式包装（每次调用解析当前上游），`WithTools` 惰性重绑。
- **D-04** 配置持久化 = `~/.skillforge/llm.json`（0600，`SKILLFORGE_LLM_CONFIG` 可覆盖路径）；**不入仓库**。文件配置优先于 env（面板保存是显式意图）；面板显示 effective 值与来源。
- **D-05** key 安全：GET 响应永不回传明文（masked `…last4`）；文件 0600；支持 `api_key_env` 间接引用（env 名优先于明文）。
- **D-06** 厂商/套餐目录内置（zhipu Coding Plan/标准、opencode Go/Go Plus/Zen、openai、deepseek、moonshot、siliconflow、ollama 本地、custom 自定义）；每个 plan 提供 base URL + 可选 models URL；支持自定义 URL。
- **D-07** 模型列表 = OpenAI 兼容 `GET {modelsURL}`（Authorization: Bearer），宽松解析（`data[].id` / `models[].id` / 裸数组）；拉取失败不阻塞——模型字段是自由文本 combobox，列表只是便利。
- **D-08** 连通性测试 = 最小 chat completion（max_tokens=1），返回 ok/延迟/模型数/错误分类（401/403 = key 无效；404 = URL 错；超时/网络错误区分）。
- **D-09** nil 语义必须保持：stub 模式（无上游）下 redline skip、approval/plan 503 的行为不变。实现 = `ChatModelAvailable()` helper（DynamicChatModel ⇒ `Available()`；其它实现/测试替身 ⇒ `!= nil`）。
- **D-10** mode 标注自动切换：stubchat extractor 写 `mode="stub"`，热重载切真模型后由 `omitempty` 自动消失——不需要额外处理。
- **D-11** PipelineChatModel 动态化：upstream 有 ⇒ 真模型；无 ⇒ stubChat。这样热重载 stub→real 对流式 Verdict 也生效。
- **D-12** 测试替身兼容：`Deps.ChatModel`/`PipelineChatModel` 保持接口类型不变；仅生产装配改为委托式包装。

## Claude's Discretion（实现者裁量）

- JSON 字段命名、catalog 结构、错误文案；模型列表缓存策略；前端组件拆分方式；gate 脚本分区组织。

## Deferred

- Anthropic/Gemini 原生协议（D-02 明确排除）。
- 多配置 profile（一次只保存一份 active 配置）。
- opencode 本地网关模式（上游未实现，见 issue #31724）。
