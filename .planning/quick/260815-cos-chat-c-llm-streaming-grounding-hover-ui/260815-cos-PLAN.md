---
quick_id: 260815-cos
title: chat C-方案 LLM streaming draft + grounding hover + 置信度隐藏 + HistoricalRefutationCard 仅冲突时显示
type: execute
depends_on: []
files_modified:
  - internal/wire/chunk.go
  - internal/server/server.go
  - internal/server/pipeline.go
  - internal/server/pipeline_default_test.go
  - internal/server/chat_test.go
  - cmd/server/main.go
  - web/src/types/wire-types.ts
  - web/src/lib/chunk-lookup.ts
  - web/src/transport/MockChatTransport.ts
  - web/src/components/chat/parts/DraftStreamChip.tsx
  - web/src/components/chat/parts/DraftStreamChip.test.tsx
  - web/src/components/center/CitationCluster.tsx
  - web/src/components/center/CitationCluster.test.tsx
  - web/src/components/center/VerdictCard.tsx
  - web/src/components/center/VerdictCard.test.tsx
  - web/src/components/verdict/CounterEvidenceCard.tsx
  - web/src/components/verdict/CounterEvidenceCard.test.tsx
autonomous: true
user_setup: []

must_haves:
  truths:
    - "Chat 流式输出期间用户看到 LLM 草稿文本流式到达（draft-1 chip 🟡 待校验）"
    - "grounding 通过后 chip 切换为「✅ 已验证」并保留草稿文本作为正式结论"
    - "grounding 失败时草稿消失，RefusalCard 替换（不留 chip）"
    - "引用角标 `[1]` `[2]` 悬浮在结论正文右上角，hover 弹层显示 chunk 全文"
    - "VerdictCard 不再渲染绿色置信度条（DOM testid 保留以 display:none 兜底）"
    - "CounterEvidenceCard 仅当 conflictChunks.length > 0 时渲染（无冲突时完全消失）"
    - "现有 stubRunner 测试无回归（PipelineChatModel 为 nil 时回退 Runner.Query + drainFinalContent）"
  artifacts:
    - path: "internal/wire/chunk.go"
      provides: "ValidTypes 白名单追加 text-delta-draft + text-delta-draft-end"
      contains: "text-delta-draft"
    - path: "internal/server/server.go"
      provides: "Deps.PipelineChatModel 字段（独立于 Deps.ChatModel）"
      contains: "PipelineChatModel"
    - path: "internal/server/pipeline.go"
      provides: "Step 8 流式分支：PipelineChatModel 非 nil → Stream + 每个 token emit draft；nil → Runner.Query 回退"
      contains: "PipelineChatModel.Stream"
    - path: "internal/server/pipeline_default_test.go"
      provides: "TestVerdictPipelineStreaming_DraftChunksEmitted（RED → GREEN）"
      contains: "TestVerdictPipelineStreaming_DraftChunksEmitted"
    - path: "web/src/types/wire-types.ts"
      provides: "SkillForgeChunk 联合追加 text-delta-draft + text-delta-draft-end"
      contains: "text-delta-draft"
    - path: "web/src/components/chat/parts/DraftStreamChip.tsx"
      provides: "新组件：根据 status（pending/verified）渲染 🟡 待校验 / ✅ 已验证 chip"
      contains: "data-testid=\"draft-chip\""
    - path: "web/src/components/center/CitationCluster.tsx"
      provides: "chip 改 `<sup>[N]</sup>` 角标 + hover popover（React 19 onMouseEnter/Leave + useState + fetchChunkContent）"
      contains: "data-testid=\"citation-superscript\""
    - path: "web/src/components/center/VerdictCard.tsx"
      provides: "移除绿色置信度条 + 「置信度 N%」文本；保留 data-testid=\"confidence-bar\" DOM 节点 display:none"
      contains: "data-testid=\"confidence-bar\""
      min_lines: 40
    - path: "web/src/components/verdict/CounterEvidenceCard.tsx"
      provides: "组件顶部 `if (rows.length === 0) return null` 短路"
      contains: "if (rows.length === 0) return null"
  key_links:
    - from: "internal/server/pipeline.go Step 8"
      to: "wire.EmitChunk(text-delta-draft)"
      via: "Stream.Recv() 循环内每 token emit"
      pattern: "wire\\.EmitChunk.*text-delta-draft"
    - from: "internal/server/pipeline.go Step 8"
      to: "verdict.GroundingChecker"
      via: "buf.String() 走原 parse/validate/grounding 路径（草稿通道对 GroundingChecker 透明）"
      pattern: "verdict\\.GroundingChecker"
    - from: "web/src/components/chat/parts/DraftStreamChip.tsx"
      to: "SkillForgeChunk (text-delta-draft / text-delta-draft-end / data-policy)"
      via: "transport 监听器维护 draftBuffer + status（pending → verified → null）"
      pattern: "text-delta-draft"
    - from: "web/src/components/center/CitationCluster.tsx"
      to: "fetchChunkContent (web/src/lib/chunk-lookup.ts)"
      via: "hover 时 fetchChunkContent(chunkId) → popover 渲染"
      pattern: "fetchChunkContent"
---

<objective>
实现 chat 端到端流式 C 方案：BE LLM streaming 草稿 emit + FE 草稿 chip 状态切换 + 引用角标 hover 弹层（拉 chunk 全文）+ VerdictCard 置信度条隐藏 + CounterEvidenceCard 仅冲突时渲染。保留 SkillForge 「无未验证结论」信任契约（结论只在 grounding 通过后渲染真文本；草稿阶段 chip 标记 🟡 待校验；grounding 失败时 RefusalCard 整体替换草稿）。

Per locked CONTEXT.md: A2（草稿流式 + 待校验/已验证 chip）/ B1（grounding 失败 → RefusalCard 替换）/ C3（角标 + hover 弹层 + 复用 GET /api/knowledge/chunk/:id）/ E1（CounterEvidenceCard 仅冲突渲染）/ D 决策（VerdictCard 移除置信度条，后端 verdict JSON 不动）。

Per RESEARCH.md 推荐：新 wire 类型 `text-delta-draft` + `text-delta-draft-end`（不复用 text-delta 避免污染 AI SDK v5 text part 状态）；Deps.PipelineChatModel 与 Deps.ChatModel 分离；raw React 19 hover（不引 Radix）；CounterEvidenceCard 顶部 `if (rows.length === 0) return null`；VerdictCard 保留 data-testid="confidence-bar" 节点 display:none 兜底兼容现有断言。
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/quick/260815-cos-chat-c-llm-streaming-grounding-hover-ui/260815-cos-CONTEXT.md
@.planning/quick/260815-cos-chat-c-llm-streaming-grounding-hover-ui/260815-cos-RESEARCH.md

# Source files（已读）
- internal/wire/chunk.go:231 — ValidTypes 白名单（20 类型）
- internal/server/server.go:140-190 — Deps 结构（含 ChatModel ToolCallingChatModel）
- internal/server/pipeline.go:170-320 — VerdictPipeline Step 5-11（Step 8 = LLM 调用）
- internal/server/chat.go:619,697,783 — emitVerdictStream / emitRefusal / drainFinalContent 范式
- internal/server/knowledge.go:124 — GetChunk 路由（260813-dno 落地）
- internal/server/pipeline_default_test.go — 现有 stubRunner + TestVerdictPipelineDefault
- web/src/components/center/CitationCluster.tsx — 现有 chip（FEUI-03）
- web/src/components/center/VerdictCard.tsx:36-75 — 置信度条 + 「置信度 N%」文本
- web/src/components/verdict/CounterEvidenceCard.tsx:41-115 — 现状空状态卡片
- web/src/lib/chunk-lookup.ts:103 — fetchChunkContent（hover 弹层直接复用）
- web/src/types/wire-types.ts:153-192 — SkillForgeChunk 联合 + D15 drift alarm
- web/src/transport/MockChatTransport.ts — useChat transport 入口
</context>

<interfaces>
<!-- Key contracts the executor needs. Extracted from codebase. -->

From internal/wire/chunk.go:
```go
type UIMessageChunk struct {
    Type      string          `json:"type"`
    ID        string          `json:"id,omitempty"`
    Delta     string          `json:"delta,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
    // ... other fields
}

var ValidTypes = map[string]bool{...}  // 白名单守门

func EmitChunk(c *app.RequestContext, chunk UIMessageChunk) error  // best-effort emit
```

From internal/server/server.go:
```go
type Deps struct {
    Loader    *skill.Loader
    Embedder  Embedder
    Retriever Retriever
    Runner    Runner
    Agent     *agent.Agent
    Store     store.Store
    ChatModel model.ToolCallingChatModel  // SC-7 redline seam (D5-14)
    // ... PipelineChatModel 在本 plan 新增
}
```

From internal/server/pipeline.go (现有 Step 8):
```go
opts := []adk.AgentRunOption{
    adk.WithChatModelOptions([]model.Option{
        model.WithTemperature(temp),
        model.WithMaxTokens(600),
        openai.WithExtraFields(extraFields),
    }),
}
emitSubprogress(c, wire.SubprogressStep{Step: "generate"})
iter := s.Runner.Query(ctx, prompt, opts...)
raw, err := drainFinalContent(iter)  // → buf.String() → JSON parse → Validate → GroundingChecker
```

From Eino v0.7.7 components/model/interface.go:
```go
type BaseChatModel interface {
    Generate(ctx, msgs, opts...) (*schema.Message, error)
    Stream(ctx, msgs, opts...) (*schema.StreamReader[*schema.Message], error)
}
// StreamReader: Recv() (T, error) + Close()
// Eino 的 BaseChatModel 已被 model.ToolCallingChatModel 嵌入（ToolCallingChatModel 接口嵌 BaseChatModel）
```

From web/src/lib/chunk-lookup.ts:
```typescript
export async function fetchChunkContent(chunkId: string): Promise<ChunkContent | null>
export function lookupChunk(chunkId: string): ChunkContent | null
export function extractChunkMetadata(chunkId: string): { version: string | null; effectiveDate: string | null }
```

From web/src/types/wire-types.ts:
```typescript
export type SkillForgeChunk =
  | { type: 'text-start'; id: string; ... }
  | { type: 'text-delta'; id: string; delta: string; ... }
  | { type: 'text-end'; id: string; ... }
  // ... 17 more variants

// D15 drift alarm (compile-time)
type _AssertAssignable<A, B> = A extends B ? B extends A ? true : never : never;
const _check: _AssertAssignable<SkillForgeChunk, UIMessageChunk> = true;
```

From web/src/components/verdict/CounterEvidenceCard.tsx:41:
```typescript
export default function CounterEvidenceCard({ conflictChunks }: CounterEvidenceCardProps) {
  const rows = conflictChunks ?? [];
  return (
    <div data-testid="counter-evidence-card" ...>
      ...
      {rows.length === 0 ? (<empty />) : (<ul>{rows.map(...)}</ul>)}
    </div>
  );
}
```
</interfaces>

<tasks>

<!-- ================================================================== -->
<!-- Task 1: BE pipeline Stream + wire ValidTypes + PipelineChatModel   -->
<!-- ================================================================== -->
<task type="auto" tdd="true">
  <name>Task 1 (TDD RED→GREEN): BE LLM streaming draft emit (PipelineChatModel.Stream + text-delta-draft wire type)</name>
  <files>
    internal/wire/chunk.go,
    internal/server/server.go,
    internal/server/pipeline.go,
    internal/server/pipeline_default_test.go,
    cmd/server/main.go,
    internal/server/chat_test.go
  </files>
  <behavior>
    - Test 1: TestVerdictPipelineStreaming_DraftChunksEmitted — PipelineChatModel 注入一个返回 3 个 token 的 stub StreamReader，stubRunner 仍然返回 canned verdict；POST /api/chat 后 drainBody 必须按顺序包含 `{"type":"text-delta-draft","id":"draft-1","delta":"..."}` × 3 + `{"type":"text-delta-draft-end","id":"draft-1"}` + 原 verdict 流（text-start/text-delta/source-document/data-policy/text-end/finish-step/finish）；audit 仍写一行；session turn 仍追加。
    - Test 2: TestVerdictPipelineDefault_PipelineChatModelNilFallback — Deps.PipelineChatModel 为 nil（既有 newVerdictPipelineDeps），行为与 TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict 字节一致（无 text-delta-draft 帧，回退 Runner.Query + drainFinalContent）。
    - Test 3: TestVerdictPipelineStreaming_RefusalReplacesDraft — PipelineChatModel 注入的 stub 返回 canned verdict 含 status="unanswered"（或 stubRunner 返回 refusal 路径），drainBody 不应残留 text-delta-draft（草稿被 emitRefusal 覆盖——实际 SSE 上草稿在 refusal 之前已 emit，FE 端按 Research Pitfall 2 清空 buffer；BE 侧仅验证 data-policy 是 Refusal 而非 Verdict）。
  </behavior>
  <action>
    **修改 1 — internal/wire/chunk.go (低风险)**
    在 ValidTypes map 追加 `"text-delta-draft": true` 和 `"text-delta-draft-end": true`（注释引用 plan 260815-cos + 解释 "draft 通道 ID 命名空间 draft-1 与 text part msg-1 隔离"）。同时更新计数注释 "Count: 20 types ... → Count: 22 types ..."。

    **修改 2 — internal/server/server.go**
    在 Deps struct 追加 `PipelineChatModel model.BaseChatModel`（**独立字段**，不复用 ChatModel——避免 redline 与 verdict 互扰，per Research "Open questions" 推荐）。`// Plan 260815-cos: separate from ChatModel (D5-14 redline seam) so streaming verdict generation does not interfere with SC-7 redline pass.` 注意 `model.ToolCallingChatModel` 已嵌入 `BaseChatModel` 接口（Stream 方法来自 BaseChatModel），所以 cmd/server/main.go 既有的 ChatModel 赋值可同时满足这两个字段。

    **修改 3 — internal/server/pipeline.go Step 8 (核心)**
    Step 8 拆成两个分支：
    1. 当 `s.PipelineChatModel != nil` 时：调 `s.PipelineChatModel.Stream(ctx, []*schema.Message{{Role: schema.System, Content: prompt}}, opts...)`；defer stream.Close()；for-loop Recv()：
       - `errors.Is(err, io.EOF)` → break
       - 其他 err → return wrapped error
       - msg 非 nil 且 Content 非空 → (a) `wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-delta-draft", ID: "draft-1", Delta: msg.Content})` (best-effort，仿 emitSubprogress log.Printf WARN)；(b) `buf.WriteString(msg.Content)`
    2. 循环结束后 emit `{"type":"text-delta-draft-end","id":"draft-1"}`
    3. `raw := buf.String()` 走原 parse/validate/grounding/emit 路径（**完全不动**）。
    4. 当 `s.PipelineChatModel == nil` 时：回退到 `iter := s.Runner.Query(ctx, prompt, opts...)` + `drainFinalContent(iter)`（**字节兼容现有测试**）。
    opts 复用现有 `adk.WithChatModelOptions(...)` 切片；openai.WithExtraFields 包装在 model.Option 内（Stream 也接受 model.Option 列表）。注意：opts 通过 `model.WithTemperature/WithMaxTokens/openai.WithExtraFields` 这些 model.Option 构造——Stream 与 Generate 共用同一组 Option。

    **修改 4 — cmd/server/main.go**
    在既有的 `ChatModel:` 赋值后追加 `PipelineChatModel: cm,`（同一 model 实例——ToolCallingChatModel 内嵌 BaseChatModel，所以 Stream 方法可用）。

    **修改 5 — 测试 (TDD RED → GREEN)**
    - 新建 `internal/server/pipeline_stream_test.go`：定义 `stubStreamingChatModel` (implements model.BaseChatModel) —— tokens []string 字段，Generate 返回拼好的 Message，Stream 返回 schema.StreamReader 自定义实现（用 channels 喂 token + io.EOF 终止）。PipelineChatModel 字段类型 model.BaseChatModel——确认 ToolCallingChatModel 嵌入 BaseChatModel 后赋值兼容。
    - Test 1：注入 stubStreamingChatModel + stubRunner(cannedVerdictJSON)，调用 newTestHertz + postChatRaw + drainBody，逐行断言 3 个 text-delta-draft + text-delta-draft-end + verdict 流顺序与计数。
    - Test 2：复用 newVerdictPipelineDeps（PipelineChatModel 字段未设置 = nil），断言 drainBody 无 text-delta-draft 帧、走原 verdict 路径。
    - Test 3：stubStreamingChatModel 注入后 stubRunner 改返回 "unanswered" canned verdict（或直接走 emitRefusal 路径——这取决于 pipeline Step 9b 现有逻辑，不需要新代码）。断言 drainBody 中 data-policy data 是 Refusal 而非 Verdict；FE 端的草稿 buffer 清空由 Task 2 的 transport 处理。
  </action>
  <verify>
    <automated>cd D:\CODE\ClaudeCode\ai-agent && go test ./internal/server/ -run 'TestVerdictPipeline' -count=1 -v 2>&1 | tail -60</automated>
  </verify>
  <done>
    - ValidTypes 白名单含 text-delta-draft + text-delta-draft-end
    - Deps.PipelineChatModel 字段存在且独立于 ChatModel
    - 既有 TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict + 其他 pipeline_default_test.go 测试 0 回归
    - 新增 TestVerdictPipelineStreaming_DraftChunksEmitted + TestVerdictPipelineDefault_PipelineChatModelNilFallback + TestVerdictPipelineStreaming_RefusalReplacesDraft 全部 PASS
    - `go vet ./...` clean
    - cmd/server/main.go 编译通过
  </done>
</task>

<!-- ================================================================== -->
<!-- Task 2: FE wire mirror + DraftStreamChip + transport hook         -->
<!-- ================================================================== -->
<task type="auto" tdd="true">
  <name>Task 2 (TDD RED→GREEN): FE SkillForgeChunk 联合 + DraftStreamChip 组件 + useChat transport 草稿 buffer</name>
  <files>
    web/src/types/wire-types.ts,
    web/src/transport/MockChatTransport.ts,
    web/src/components/chat/parts/DraftStreamChip.tsx,
    web/src/components/chat/parts/DraftStreamChip.test.tsx,
    web/src/__tests__/MockChatTransport.test.ts
  </files>
  <behavior>
    - Test 1: TestDraftStreamChip_Pending — status="pending" + draftText="依据制度..." → 渲染 `<div data-testid="draft-chip" data-status="pending">🟡 待校验</div>` + 草稿文本块；getByText('待校验') 命中。
    - Test 2: TestDraftStreamChip_Verified — status="verified" → 渲染 ✅ 已验证；文本保留（draftText="依据制度..." 仍可见）。
    - Test 3: TestDraftStreamChip_Refused — status="refused" → return null（不渲染任何节点）；queryByTestId('draft-chip') === null。
    - Test 4: TestDraftStreamChip_StreamingAppend — 模拟多次渲染，draftText 从 "依" → "依据" → "依据制度"，每段都断言文本存在（验证渐进追加）。
    - Test 5: TestSkillForgeChunk_DraftTypeAssertion — `AssertAssignable<SkillForgeChunk, UIMessageChunk>` 编译通过；TS 类型 import { SkillForgeChunk } 包含 'text-delta-draft' 联合分支。
  </behavior>
  <action>
    **修改 1 — web/src/types/wire-types.ts**
    在 SkillForgeChunk 联合的 "Text streaming" 段后追加：
    ```ts
    // Plan 260815-cos: draft streaming channel (LLM tokens before grounding).
    // NOT routed through AI SDK v5 DefaultChatTransport parser — kept in a
    // dedicated buffer by the transport hook so it cannot pollute the
    // message.parts text state. ID namespace "draft-1" isolates from
    // "msg-1" text parts.
    | { type: 'text-delta-draft'; id: string; delta: string }
    | { type: 'text-delta-draft-end'; id: string }
    ```
    D15 drift alarm：text-delta-draft 的字段子集（{type,id,delta}）必须在 AI SDK v5 UIMessageChunk 的允许联合中。`UIMessageChunk` 的 text-delta 变体是 `{type:'text-delta', id, delta, providerMetadata?}`——SkillForgeChunk 的 text-delta-draft 是 `{type,id,delta}`，**id 和 delta 是 required**——TS 严格 assignability 检查时 optional `providerMetadata?` 在父类型有、在子类型没有，这违反 strict subtype 但不违反 assignability（A 是 B 的子集当 A 的 required 都是 B 的 optional 或同名 required）。验证 `cd web && npx tsc --noEmit` 0 错误。若编译失败，加 `// @ts-expect-error D15: draft variant uses narrower field set` 一行注释。

    **修改 2 — web/src/components/chat/parts/DraftStreamChip.tsx (新文件)**
    Props: `{ status: 'pending' | 'verified' | 'refused'; draftText: string }`。return null 当 status==='refused'。否则渲染：
    ```tsx
    <div data-testid="draft-chip" data-status={status}
         className="my-2 rounded-md border border-border-muted bg-surface-2 px-3 py-2 text-sm">
      <span className="mr-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs">
        {status === 'pending' ? '🟡 待校验' : '✅ 已验证'}
      </span>
      <span data-testid="draft-text">{draftText}</span>
    </div>
    ```
    沿用现有 token：`bg-surface-2`/`border-border-muted`/`text-sm`/`text-xs`——与 DataSubprogressPart 同套风格（查 `web/src/components/chat/parts/DataSubprogressPart.tsx` 现有 className 复用）。

    **修改 3 — web/src/transport/MockChatTransport.ts**
    新增 draftBuffer 状态 + status 状态机：
    - 在 transport 解析每个 SSE chunk 时（已有 parse loop）：
      - chunk.type === 'text-delta-draft' → draftBuffer[chunk.id] = (draftBuffer[chunk.id] ?? '') + chunk.delta；status = 'pending'；通过 `onDraftUpdate({status, draftText})` 回调
      - chunk.type === 'text-delta-draft-end' → 保持 status='pending'（等 data-policy 决定升级或清空）
      - chunk.type === 'data-policy' 且 data 是 Refusal（用 typeof 'Refusal' 区分——data 有 reason 字段）→ status='refused'，onDraftUpdate 后立即清空 buffer
      - chunk.type === 'data-policy' 且 data 是 Verdict（data 有 status/citations 字段）→ status='verified'，draftBuffer 升级为 useChat 正式 message part（**注意：useChat 不允许外部注入 part**——所以策略改为保留 draftBuffer + 渲染 DraftStreamChip 在 message bubble 之外；让 VerdictCard 渲染在草稿下方；草稿在 data-policy 后只是 status 切换，不再叠加 text part）
    - 暴露的回调：通过 transport 构造器注入 `onDraftUpdate` handler（默认 no-op）；App.tsx 或 RightChat 持有 React state 把 status+draftText 渲染 DraftStreamChip。
    - 与 MockChatTransport 现有 fixture 兼容：mock 在测试中默认不 emit text-delta-draft，所以现有 196+ 测试不受影响（默认行为 = 不渲染 chip）。
    - 在 `App.tsx` 或合适位置订阅 onDraftUpdate 并在 chat 流期间显示 DraftStreamChip（**这一行 wiring 在 plan 范围内**——放在 App.tsx 或 RightChat.tsx 已有的 chat 流渲染处）。

    **修改 4 — web/src/components/chat/parts/DraftStreamChip.test.tsx (新文件)**
    4 个 vitest + @testing-library/react 测试覆盖上述 Test 1-4。setup：从 `@testing-library/react` import render + screen。

    **修改 5 — Test 5 类型断言**
    不需要新文件——`cd web && npx tsc --noEmit` 退出 0 即覆盖 D15 drift alarm 通过。
  </action>
  <verify>
    <automated>cd D:\CODE\ClaudeCode\ai-agent\web && npx vitest run src/components/chat/parts/DraftStreamChip.test.tsx src/__tests__/MockChatTransport.test.ts 2>&1 | tail -30</automated>
  </verify>
  <done>
    - SkillForgeChunk 联合含 text-delta-draft + text-delta-draft-end
    - D15 drift alarm `AssertAssignable` 编译通过（`npx tsc --noEmit` exit 0）
    - DraftStreamChip 4 个测试 PASS（pending/verified/refused/streaming-append）
    - MockChatTransport 不破坏既有测试（vitest 全集 209+ baseline 不退化）
    - App.tsx 或 RightChat.tsx 在 chat 流期间订阅 onDraftUpdate 并渲染 DraftStreamChip
  </done>
</task>

<!-- ================================================================== -->
<!-- Task 3: FE CitationCluster 角标 hover + VerdictCard 置信度隐藏 + CounterEvidenceCard 短路 -->
<!-- ================================================================== -->
<task type="auto" tdd="true">
  <name>Task 3 (TDD RED→GREEN): CitationCluster 角标 + hover popover + VerdictCard 置信度隐藏 + CounterEvidenceCard 仅冲突时渲染</name>
  <files>
    web/src/components/center/CitationCluster.tsx,
    web/src/components/center/CitationCluster.test.tsx,
    web/src/components/center/VerdictCard.tsx,
    web/src/components/center/VerdictCard.test.tsx,
    web/src/components/verdict/CounterEvidenceCard.tsx,
    web/src/components/verdict/CounterEvidenceCard.test.tsx,
    web/src/__tests__/citations.test.tsx
  </files>
  <behavior>
    - Test 1: TestCitationCluster_RendersSuperscript — 2 citations → 2 个 `<sup data-testid="citation-superscript">[1]</sup>` + `<sup>[2]</sup>`；TierBadge 不再显示在 chip 内（角标模式下保留为标题或 popover 内）。
    - Test 2: TestCitationCluster_HoverFetchesChunkContent — 模拟 hover superscript[0]（fireEvent.mouseEnter），advanceTimers 150ms；断言 `fetchChunkContent` 被调用一次（chunkId=citations[0].chunk_id），popover 渲染 fetchChunkContent 返回的 content + source + chunk_id；mouseLeave 后 popover 消失。
    - Test 3: TestVerdictCard_NoConfidenceBar — 渲染 VerdictCard（verdict.status="合规", confidence=0.9），queryByTestId('confidence-bar') 返回**存在但 display:none**（节点保留以兼容既有断言）；queryByText(/置信度 \d+%/) 失败（文本移除）。
    - Test 4: TestVerdictCard_ConfidenceBarTestidRetained — getByTestId('confidence-bar') 命中（DOM 节点存在），但 getComputedStyle(el).display === 'none'。
    - Test 5: TestCounterEvidenceCard_EmptyShortCircuit — conflictChunks=[] → queryByTestId('counter-evidence-card') === null；queryByTestId('counter-evidence-disclaimer') === null；queryByTestId('counter-evidence-empty') === null。
    - Test 6: TestCounterEvidenceCard_NonEmptyStillRenders — conflictChunks=[1 chunk] → getByTestId('counter-evidence-card') 命中；getByTestId('counter-evidence-disclaimer') 命中；list 渲染 1 row。
  </behavior>
  <action>
    **修改 1 — web/src/components/center/CitationCluster.tsx (FEUI-03 + C3)**
    重构：每个 citation 不再渲染 `<button class="citation-chip">`，改渲染：
    ```tsx
    <span className="relative inline"
          onMouseEnter={onEnter} onMouseLeave={onLeave}>
      <sup data-testid="citation-superscript"
           data-chunk-id={citation.chunk_id}
           data-tier={citation.tier}
           className="ml-0.5 cursor-pointer text-xs text-accent">
        [{index + 1}]
      </sup>
      {open && <CitationHoverPopover chunkId={citation.chunk_id} data={popoverData} />}
    </span>
    ```
    onEnter 用 setTimeout(150ms) 触发 fetchChunkContent（仅一次，data 缓存到 useState）；onLeave clearTimeout + setOpen(false)。Popover 内容：`<div data-testid="citation-popover" className="absolute ...">` 内含 chunk content + source + chunk_id 元数据。直接复用 `extractChunkMetadata + fetchChunkContent`（lookupChunk 同步 fallback + fetchChunkContent 异步兜底，**完全沿用 ChunkSourceView 已有 fetchChunkContent + lookupChunk 模式**——查 web/src/components/center/ChunkSourceView.tsx 的 chunk-lookup 调用点作为范式）。注意 onClick 仍保留 navigate to centerView（保留可点击性：`<sup>` 接受 onClick）。保留 `data-testid="citation-cluster"` 在外层 div 不变。

    **修改 2 — web/src/components/center/CitationCluster.test.tsx (更新) + web/src/__tests__/citations.test.tsx (更新)**
    - 现有断言 `getByTestId('citation-chip')` 改为 `getByTestId('citation-superscript')`。
    - 现有 span "{citation.span}" 移除（角标模式不显示 span）；改用 `[index + 1]` 文本。
    - 新增 hover 测试：vi.useFakeTimers() + fireEvent.mouseEnter + vi.advanceTimersByTime(150) + 断言 fetchChunkContent 被调 + popover 渲染。

    **修改 3 — web/src/components/center/VerdictCard.tsx (D 决策 + VerdictCard.test.tsx 更新)**
    - 移除 `<span className="font-mono text-xs">置信度 {pct}%</span>` (line 51)
    - 移除整个 `<div className="mt-2 h-0.5 w-full rounded-full bg-border-muted">` 块 (lines 53-59)
    - **但保留** inner `<div data-testid="confidence-bar" ...>` 节点，改成：
      ```tsx
      <div data-testid="confidence-bar" data-confidence={pct}
           className="hidden" aria-hidden="true" />
      ```
      `hidden` 是 Tailwind 的 `display:none` 等价类。这样既移除视觉渲染，又保留 DOM testid + data-confidence 属性以兼容既有测试断言（TestVerdictCard 既有测试可能断言 confidence-bar 存在但不再断言宽度——更新断言改为 `getComputedStyle().display === 'none'` 或仅保留 `getByTestId` 不变）。
    - 更新 `web/src/components/center/VerdictCard.test.tsx`：移除对「置信度 N%」文本的断言；新增 `TestVerdictCard_NoConfidenceText` + `TestVerdictCard_ConfidenceBarTestidRetained` (上述 Test 3-4)。

    **修改 4 — web/src/components/verdict/CounterEvidenceCard.tsx (E1 决策)**
    在组件函数顶部（第 42 行附近，const rows = conflictChunks ?? []; 之后）插入一行：
    ```ts
    if (rows.length === 0) return null;
    ```
    删除原 `{rows.length === 0 ? (<empty />) : (<ul />)}` 三元表达式（lines 67-74）——永远走 `<ul>` 分支（短路后 rows 必非空）。
    更新 CounterEvidenceCard.test.tsx：移除原空状态断言；新增 Test 5-6（empty short-circuit + non-empty render）。
  </action>
  <verify>
    <automated>cd D:\CODE\ClaudeCode\ai-agent\web && npx vitest run src/components/center/CitationCluster.test.tsx src/components/center/VerdictCard.test.tsx src/components/verdict/CounterEvidenceCard.test.tsx src/__tests__/citations.test.tsx 2>&1 | tail -40</automated>
  </verify>
  <done>
    - CitationCluster 角标 + hover popover：superscript DOM + fetchChunkContent 触发
    - VerdictCard 不渲染置信度文本/条（DOM testid 保留 hidden）
    - CounterEvidenceCard rows.length===0 → return null
    - 既有 CitationCluster / VerdictCard / CounterEvidenceCard / citations 测试更新后 PASS
    - vitest 全集（baseline 209+）不回归
    - `npx tsc --noEmit` exit 0
  </done>
</task>

</tasks>

<verification>
## Plan-level checks

After all 3 tasks complete:

```bash
# BE — pipeline + wire + tests
cd D:\CODE\ClaudeCode\ai-agent && go test ./internal/server/... -count=1 2>&1 | tail -20
cd D:\CODE\ClaudeCode\ai-agent && go vet ./... 2>&1 | tail -5
# Frontend — types + parts + components
cd D:\CODE\ClaudeCode\ai-agent\web && npx tsc --noEmit 2>&1 | tail -5
cd D:\CODE\ClaudeCode\ai-agent\web && npx vitest run 2>&1 | tail -10
cd D:\CODE\ClaudeCode\ai-agent\web && npx vite build 2>&1 | tail -10
```

All 4 must exit 0; vitest baseline 209+ 不退化；vite build 输出 bundle 不超过 baseline +5%。
</verification>

<success_criteria>
1. BE: ValidTypes 含 text-delta-draft + text-delta-draft-end；Deps.PipelineChatModel 独立于 ChatModel；Step 8 流式分支在 PipelineChatModel 非 nil 时 emit 草稿，nil 时回退 Runner.Query 既有路径；3 个新测试 PASS；既有 pipeline_default_test.go 0 回归。
2. FE TS wire: SkillForgeChunk 含 2 个新 variant；D15 drift alarm 编译通过。
3. FE DraftStreamChip: 4 测试 PASS（pending/verified/refused/streaming-append）。
4. FE CitationCluster: 角标 [N] superscript DOM + 150ms hover popover 拉 fetchChunkContent。
5. FE VerdictCard: 移除置信度文本 + 视觉条；data-testid="confidence-bar" 节点 hidden 兜底。
6. FE CounterEvidenceCard: rows.length===0 → return null；rows 非空时完整渲染 disclaimer + list。
7. `go test ./internal/server/...` 全部 PASS。
8. `go vet ./...` clean。
9. `npx tsc --noEmit` exit 0。
10. `npx vitest run` 全集 PASS（baseline 209+ 不退化）。
11. `npx vite build` 退出 0，bundle 大小在 baseline +5% 内。
12. `gofmt -l` + `prettier --check` 干净（按 260814-wre EOL 规范）。
</success_criteria>

<output>
Create `.planning/quick/260815-cos-chat-c-llm-streaming-grounding-hover-ui/260815-cos-SUMMARY.md` when done.
</output>