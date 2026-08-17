---
slug: verdict-stream-hang
status: awaiting_human_verify
trigger: "问答在出结论之前就不动了 — 后端日志显示 streamAndAssemble 进入 Recv 循环后再无输出"
created: 2026-08-17
updated: 2026-08-17
---

# Debug: verdict-stream-hang

## Symptoms

- **Expected behavior**: 提交问题（如"年假未休完的部分如何折算或结转？"，skill=compliance）后应流式输出结论，前端"生成结论"节点推进并展示 verdict。
- **Actual behavior**: 前端停在"生成结论"节点不再推进。后端日志显示 `streamAndAssemble` 调用 `PipelineChatModel.Stream` 返回 OK 并进入 Recv 循环后，再无任何输出（没有 "about to call first Recv"、没有 completed）。
- **Error messages**: 无报错日志。同一服务 11:33 的请求以 `streamAndAssemble completed total_tokens=0 buf_bytes=0` 结束（流空跑）；11:34 的请求卡在 Recv 循环。
- **Timeline**: vsm 平台解耦改动（commits 0273d10 verdict package platform-decoupling、b63d43e、e4124f0、502b5f1）之前正常，改动后每个请求都卡死。改动前是好的。
- **Reproduction**: 任意问题、skill=compliance，每个请求都卡死。

## Key Log Evidence

```
11:34:04 chatHandler enter query="年假未休完的部分如何折算或结转？" skill=compliance
11:34:04 chatHandler routing to runAgentPath (USE_AGENT_PATH=true)
11:34:04 chatHandler routing to Registry+Router
11:34:04 chatHandler routing to VerdictPipeline (Loader)
11:34:04 chatHandler dispatching to VerdictPipeline
11:34:04 VerdictPipeline enter query="..." skill_name="simple-qa" skill_id=compliance
11:34:04 streamAndAssemble entering system_bytes=2970 user_bytes=2182 opts_count=3
11:34:04 streamAndAssemble calling PipelineChatModel.Stream model=*openai.ChatModel opts=3
11:34:05 streamAndAssemble PipelineChatModel.Stream returned OK, entering Recv loop
[再无输出]
```

## Suspicious Observations

- `skill_name="simple-qa" skill_id=compliance` — Loader 解析出的 skill_name 与 skill_id 不一致，可能加载了错误的 SKILL 配置（与已完成的 verdict-subject-mismatch 会话主题相关）。
- 11:33 请求 total_tokens=0 buf_bytes=0 — 上游模型流返回为空或帧解析后全部被丢弃。
- 回归点明确指向 verdict package platform-decoupling（0273d10）及其后续 3 个提交。

## Current Focus

- hypothesis: CONFIRMED + FIX APPLIED（见 reasoning_checkpoint 与 Resolution）
- test: go build ./... 绿；go test ./... 全仓绿（含 3 个新回归测试）；go test -tags=contract ./... 绿；race 不可用（Windows 无 gcc，环境问题非代码问题）
- expecting: 用户重启 server 后发送同一查询 → (a) 正常情况结论流式出现；(b) 若 GLM 仍失败，日志出现 retry WARN + 最终 [ERROR] chatHandler verdict pipeline failed，FE chat 流显示"结论生成失败：模型服务暂时不可用，请稍后重试。"而非冻结
- next_action: 等待用户 live 验证（GLM key 在用户 shell，debugger 无法自行活体验证）

reasoning_checkpoint:
  hypothesis: "0273d10 将 VerdictPipeline system prompt 从 1546B 翻倍至 2970B（实测）。GLM-5.2 Coding Plan 端点对 verdict prompt 长度敏感（前序活体会话 agent-llm-query-rewrite.md 记录：偶发返回空字符串 → parse verdict: unexpected end of JSON input (raw: )，并明确建议 retry-with-backoff 但未实施）。prompt 翻倍把偶发失败放大为近乎必然：11:33 空流（total_tokens=0 buf_bytes=0，Recv 循环正常跑完但 0 token）、11:34 Stream() 直接返回错误（D-5 'about to call first Recv' 缺失即铁证 —— D-4/D-5 同 commit aec5392 引入，两者之间仅 err!=nil 分支可分流）。用户感知的『卡死』是静默错误链：chatHandler 错误分支（chat.go:271-284）不 log.Printf、只发 errorChunk+bare finish-step（缺 text-start/text-end/finish，本仓 Phase-7 已证明此形态消息不注册），且 App.tsx:115 未解构 useChat 的 error → 生成结论 subprogress 节点永远停住。"
  confirming_evidence:
    - "D-5 日志缺失证明 11:34 走了 err!=nil 早退（pipeline.go:506-508）—— 505-521 行之间无任何可阻塞代码"
    - "11:33 total_tokens=0 = 上游空流 → raw='' → parse 错误 → 同一静默错误分支"
    - "agent-llm-query-rewrite.md:121-135 活体记录 GLM-5.2 对 prompt 长度敏感、偶发返回空；建议的 retry-with-backoff 从未实现"
    - "实测 systemInstructionsTemplate：1adbbdb=1546B → HEAD=2970B；活体日志 system_bytes=2970 吻合"
    - "git diff 确认 0273d10..HEAD 唯一的 LLM 调用前运行时改动 = prompt 模板；go.mod/go.sum 无变化；model 配置无变化"
    - "FE：RealChatTransport 原样透传 error chunk；ai@7 parser(dist/index.js:7386) 对 error chunk 仅调 onError 后继续；App.tsx:115 不读 error → 无任何可见渲染"
  falsification_test: "若修复后 live 日志显示重试 N 次仍失败且错误为 401/429/quota 类上游错误，则 prompt 长度假设被证伪，真因是账号/网关侧 —— 新增的错误日志会捕获确切上游错误文本以判别"
  fix_rationale: "(1) 精简 prompt 回 ~1.7KB 消除长度触发器（保留 schema+决策规则+测试钉住的字符串）；(2) streamAndAssemble 加重试（4 attempts, 1s/2s/4s backoff，仅 0-token 失败可重试以避免 draft chip 重复）+ 每次 attempt 60s 超时（把真· hang 变成可重试错误）+ 空流显式报错 —— 即前序会话建议但从未实施的缓解；(3) chatHandler 错误分支补 log.Printf —— 消除『再无任何日志输出』盲区；(4) 错误分支补完整 text 生命周期（text-start/delta/end+finish-step+finish）让 FE 永远有终态可见"
  blind_spots: "无法自行活体调 GLM（key 仅在用户 shell）—— prompt长度→失败 的因果链来自前序活体记录+时间线相关，非我亲自复现；若 GLM 失败实为账号配额，prompt 精简无效，但重试+日志会暴露真相。mid-stream 部分 token 后的 recv 错误不重试（避免 draft 重复），若 GLM 失败模式是中途断流需后续扩展。"

## Evidence

- timestamp: 2026-08-17 (session 1)
  checked: pipeline.go:505-521（D-4 与 D-5 之间代码）+ git log -S 确认 D-4/D-5 同 commit aec5392 引入
  found: D-4 与 D-5 之间仅有 err!=nil 检查（506-508 早退）。11:34 日志有 D-4 无 D-5 → Stream() 返回了错误，函数从 508 行返回 "open LLM stream: %w"。任何含 D-4 的二进制必然含 D-5，排除 stale binary。
  implication: 11:34 不是 Recv 阻塞的真 hang，是 stream-open 错误静默传播。

- timestamp: 2026-08-17 (session 1)
  checked: chat.go:271-284（chatHandler 错误分支）
  found: 错误分支只 emit errorChunk + finish-step，**不 log.Printf**，缺 text-start/text-end/finish。这解释了"再无任何日志输出"。
  implication: 所有 pipeline 错误（LLM 失败/parse 失败/validate 失败）都是静默的 —— 用户只能看到 FE 停住。

- timestamp: 2026-08-17 (session 1)
  checked: App.tsx:115 + RealChatTransport.ts + ai@7 dist/index.js:7386
  found: useChat 未解构 error；transport 透传 error chunk；SDK 对 error chunk 仅调 onError 后继续处理后续 chunk。生成结论节点（data-subprogress）永不推进，无任何错误 UI。
  implication: FE 对任何后端错误表现为"卡死"。修复后端错误分支发完整 text 生命周期即可让 FE 有终态（FE 零改动）。

- timestamp: 2026-08-17 (session 1)
  checked: .planning/debug/agent-llm-query-rewrite.md:121-135
  found: 前序活体会话已记录 GLM-5.2 "偶尔返回空字符串或非 JSON 给 VerdictPipeline → parse verdict: unexpected end of JSON input (raw: ) → internal_error"，且明确"对 prompt 长度/chunk 数/temperature 敏感"，建议 retry-with-backoff（3次 1s/2s/4s）但从未实施。
  implication: 11:33 空流是已知 GLM 行为的复现；retry 是既定但未落地的缓解。

- timestamp: 2026-08-17 (session 1)
  checked: git show 1adbbdb vs HEAD 的 systemInstructionsTemplate 字节数（Go 程序实测）
  found: 1546B → 2970B（1.92x）。活体日志 system_bytes=2970 与新模板精确吻合。0273d10..HEAD 唯一 LLM 调用前改动 = 该模板；go.mod 无变化。
  implication: prompt 翻倍是唯一的代码侧触发候选，与已知长度敏感性叠加构成完整因果链。

- timestamp: 2026-08-17 (session 1)
  checked: git log -- go.mod go.sum + main.go:187-191 + pipeline.go:284-296
  found: eino v0.7.7 / eino-ext openai v0.1.5 未变；model=glm-5.2、extraFields(response_format=json_object, thinking=disabled)、max_tokens=600 均未变。
  implication: 排除依赖/配置漂移。

## Eliminated

- hypothesis: Recv 循环真阻塞（上游永不发 token 也永不 EOF）
  evidence: 11:34 缺 D-5 日志（D-5 在 Recv 循环之前），证明函数在 err!=nil 分支早退，根本没进 Recv 循环
  timestamp: 2026-08-17 (session 1)

- hypothesis: 依赖/模型配置变更导致（go.mod、extraFields、temperature 漂移）
  evidence: git diff 确认 0273d10..HEAD go.mod/go.sum 无变化；main.go 模型构造、pipeline.go extraFields 均未在 4 个 vsm commit 中触碰
  timestamp: 2026-08-17 (session 1)

- hypothesis: FE 单方面卡死（后端正常完成）
  evidence: 后端日志在 D-4 后停（11:34）/ total_tokens=0（11:33）—— 后端自身在 LLM 边界失败；11:33 空流后必走 parse 错误分支
  timestamp: 2026-08-17 (session 1)

- hypothesis: skill_name="simple-qa" 与 skill_id=compliance 不一致导致加载错误 SKILL 配置
  evidence: VerdictPipeline 的 prompt 来自 const systemInstructionsTemplate，不经 manifest；manifest.Name 仅用于日志。compliance/SKILL.md frontmatter 无 name 字段，Loader 默认行为 —— 与 hang 因果链无关
  timestamp: 2026-08-17 (session 1)

## Resolution

root_cause: 双重因果链。触发器：0273d10 把 VerdictPipeline system prompt 从 1546B 翻倍到 2970B，叠加 GLM-5.2 Coding Plan 端点已知的 prompt 长度敏感性（agent-llm-query-rewrite.md 活体记录：偶发返回空流/stream 错误），把偶发失败放大为近乎必然 —— 11:33 空流（total_tokens=0），11:34 Stream() 直接返回错误（D-5 日志缺失为铁证：D-4/D-5 同 commit aec5392，两者之间仅 err!=nil 分支可分流）。放大器（用户感知"卡死"的直接原因）：chatHandler 错误分支不 log.Printf 且只发 errorChunk+bare finish-step（缺 text-start/text-end/finish，本仓 Phase-7 已证明此形态消息不注册），App.tsx:115 又不读 useChat error → 生成结论节点永远冻结且后端零日志。

fix: 四处修复，对应因果链每一环：
1. **prompt 精简**（pipeline.go systemInstructionsTemplate 2970B→2030B）：压缩 STEP 0-4 冗长散文为 4 条紧凑规则，保留全部 schema/决策语义与测试钉住字符串；顺带删除了一处 prompt 与枚举不一致（原 STEP 0 让 LLM 可输出 compliance_status="needs_input"，不在 ComplianceStatuses 枚举内，会触发 Validate 失败）。
2. **重试 + 超时 + 空流显式报错**（pipeline.go streamAndAssemble 重构为 retry wrapper + streamOnce）：最多 4 attempts（1s/2s/4s backoff，package var streamBackoff 可测试注入）；仅 0-token 失败可重试（避免 draft chip 重复）；每次 attempt 60s ctx 超时（把真·hang 变可重试错误）；空流（0 tokens at EOF）显式报错进重试，不再以 "" 流入 parse 产生误导性 "unexpected end of JSON input"。此即 agent-llm-query-rewrite.md 建议但从未实施的缓解。
3. **错误日志**（chat.go chatHandler 错误分支 + streamOnce stream-open 失败处）：[ERROR] chatHandler verdict pipeline failed sess=... 全链错误 + [WARN] stream open failed / empty stream / retry attempt —— 消除"再无任何日志输出"盲区。
4. **错误路径完整 text 生命周期**（chat.go 错误分支）：errorChunk 后补 text-start/text-delta(pipelineErrorText="结论生成失败：模型服务暂时不可用，请稍后重试。")/text-end/finish-step/finish —— ai v7 对 error chunk 仅调 onError 继续处理（dist/index.js:7386），FE 零改动即可获得可见终态；内部错误细节只留服务端日志（T-1-13）。

verification: go build ./... + go vet 绿；go test ./... 全仓绿；go test -tags=contract ./... 绿；3 个新回归测试（pipeline_stream_test.go）：TestStreamAndAssemble_RetryOnEmptyStream（空流→重试→成功，StreamCount=2）、TestStreamAndAssemble_RetryOnStreamOpenError（stream-open 错误→重试→成功）、TestStreamAndAssemble_ExhaustsRetries（4 次全空→错误分支完整生命周期+internal_error audit 行+无 draft 泄漏）。race 检测不可用（Windows 无 gcc，环境问题）。live 验证待用户。

files_changed: [internal/server/pipeline.go, internal/server/chat.go, internal/server/pipeline_stream_test.go]
