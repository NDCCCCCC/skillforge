---
phase: 260817-hck-verdictcard-dedupe-answer-vs-chat-conclu
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/components/center/VerdictCard.tsx
  - web/src/components/chat/parts/DataPolicyPart.tsx
  - web/src/components/chat/MessageBubble.tsx
  - web/src/__tests__/VerdictCard.test.tsx
  - web/src/__tests__/MessageBubble.test.tsx
autonomous: true
requirements: []

must_haves:
  truths:
    - "聊天流正文（text part）已包含 verdict.answer 时，VerdictCard 不再渲染 verdict-answer 区块（用户只看到一份结论）"
    - "聊天流正文为空或不包含 verdict.answer 时（refusal 旁路、文本缺失等异常路径），verdict-answer 照常渲染"
    - "CenterPanel 独立使用的 VerdictCard（不传 chatText prop）行为不变，verdict-answer 照常渲染"
    - "聊天流正文 text part 保持原样渲染（后端 text-delta 是 useChat 消息锚点，不可动）"
    - "confidence-bar hidden-node 模式（data-testid + hidden class + data-confidence）不受影响"
  artifacts:
    - path: "web/src/components/center/VerdictCard.tsx"
      provides: "可选 chatText prop + 重复检测逻辑（substring 匹配）"
      contains: "chatText"
    - path: "web/src/components/chat/parts/DataPolicyPart.tsx"
      provides: "chatText prop 透传到 VerdictCard"
      contains: "chatText"
    - path: "web/src/components/chat/MessageBubble.tsx"
      provides: "assistant 分支聚合 text parts 为 chatText 并传给 DataPolicyPart"
      contains: "chatText"
    - path: "web/src/__tests__/VerdictCard.test.tsx"
      provides: "重复隐藏 / 不重复保留 / 无 prop 保留 三组断言"
    - path: "web/src/__tests__/MessageBubble.test.tsx"
      provides: "端到端集成断言：text part 含结论时卡片内 verdict-answer 消失"
  key_links:
    - from: "web/src/components/chat/MessageBubble.tsx"
      to: "web/src/components/chat/parts/DataPolicyPart.tsx"
      via: "renderPart 'data-policy' 分支传入 chatText prop"
      pattern: "chatText={chatText}"
    - from: "web/src/components/chat/parts/DataPolicyPart.tsx"
      to: "web/src/components/center/VerdictCard.tsx"
      via: "isVerdict 分支透传 chatText prop"
      pattern: "chatText={chatText}"
    - from: "web/src/components/center/VerdictCard.tsx"
      to: "verdict-answer 条件渲染"
      via: "chatText?.includes(answer.trim()) 判定"
      pattern: "chatText.*includes"
---

<objective>
当 verdict.answer 与聊天流正文结论重复时，隐藏 VerdictCard 内的 verdict-answer 区块。卡片保留：StatusBadge（+ 可选 ComplianceStatusBadge）、「N 条引用」计数、CitationCluster。聊天流 text part 不动（它是 AI SDK v5 useChat 的消息锚点 + 主阅读面）。

Purpose: 消除助手消息里结论渲染两次的视觉重复（chat.go emitVerdictStream 的 text-delta 结论 + 卡片内 verdict-answer 区块）。
Output: VerdictCard 新增可选 chatText prop；MessageBubble → DataPolicyPart → VerdictCard 三级 prop 透传；重复时条件隐藏 verdict-answer；VerdictCard.test.tsx + MessageBubble.test.tsx 测试覆盖。
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./CLAUDE.md

@web/src/components/center/VerdictCard.tsx
@web/src/components/chat/parts/DataPolicyPart.tsx
@web/src/components/chat/MessageBubble.tsx
@web/src/__tests__/VerdictCard.test.tsx
@web/src/__tests__/MessageBubble.test.tsx

<interfaces>
<!-- 重复判定的事实依据（来自 internal/server/chat.go conclusionText，chat.go:793-828）： -->
<!-- factual query（v.Answer != ""）时后端发出的 text-delta 为：                    -->
<!--   "结论：" + v.Answer + "。" + [可选 "置信度未达自动采信阈值，需人工复核。"]      -->
<!--        + [可选 "依据 {tier} tier 条款：「{span}」。"]                           -->
<!-- answer 被 VERBATIM 嵌入 → FE 侧 substring 匹配即可，无需在 TS 复刻 Go 推导逻辑。  -->

关键既有契约（executor 直接使用，无需探索）：

VerdictCard 现状（VerdictCard.tsx:41-78）：
  - export interface VerdictCardProps { verdict: Verdict }
  - const isFactualQuery = !!verdict.answer
  - verdict-answer 区块：{isFactualQuery && <div data-testid="verdict-answer" ...>{verdict.answer}</div>}
  - confidence-bar hidden node（VerdictCard.tsx:86-91）：data-testid="confidence-bar" + className="hidden" + data-confidence={pct} — 不可回归

DataPolicyPart 现状（DataPolicyPart.tsx:31-51）：
  - 签名：({ part }: { part: DataPolicyPartType | (UIMessage['parts'][number] & Record<string, unknown>) })
  - isVerdict(data) → <VerdictCard verdict={data} />
  - isRefusal(data) → <RefusalCard refusal={data} />（本任务不动）

MessageBubble 现状（MessageBubble.tsx:38-91, 98-133）：
  - renderPart(part, i) 自由函数，'data-policy' 分支 → <DataPolicyPart key={i} part={...} />
  - assistant 分支：{message.parts.map((part, i) => renderPart(part, i))}
  - text part 类型为 UIMessage['parts'][number] 的 'text' 变体，文本在 .text 字段

VerdictCard 的另一消费者：web/src/components/center/CenterPanel.tsx:191
  - <VerdictCard verdict={verdict} /> — 无 chat 流上下文，不传新 prop → 行为必须不变

流时序事实（chat.go:674-733 emitVerdictStream）：text-delta 单次原子发出，先于
  data-policy chunk → VerdictCard 渲染时 text part 已含完整结论，无部分文本竞态。
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: VerdictCard 新增 chatText prop + 重复隐藏逻辑 + 单元测试</name>
  <files>web/src/components/center/VerdictCard.tsx, web/src/__tests__/VerdictCard.test.tsx</files>
  <behavior>
    - 既有测试全部保持绿色：所有现有断言不传 chatText，行为不变（尤其 "renders the factual-answer text in verdict-answer when answer is populated" 与 confidence-bar hidden-node 两个测试）
    - 新增测试 1（重复隐藏）：verdict 带 answer="由部门负责人审批"，传 chatText="结论：由部门负责人审批。依据 权威 tier 条款：「采购合同金额 50 万元以下由部门负责人审批」。" → queryByTestId('verdict-answer') 为 null；verdict-card / citation-cluster / StatusBadge 仍渲染
    - 新增测试 2（不重复保留）：传 chatText="一些不包含答案的正文" → getByTestId('verdict-answer') 存在且含 answer 文本
    - 新增测试 3（无 prop 保留 = CenterPanel 路径）：不传 chatText → getByTestId('verdict-answer') 存在（既有测试已覆盖，新增测试可显式断言带 compliance_status 的 verdict 不受影响）
    - 边界：chatText 为空字符串 '' → verdict-answer 仍渲染（'' 不含 answer）
  </behavior>
  <action>
    RED：先在 VerdictCard.test.tsx 追加上述新测试（文件头注释按项目 test-migration 惯例补一行 "260817-hck: verdict-answer 在 chatText 已含结论时隐藏" 说明），运行 vitest 确认新测试失败。

    GREEN：VerdictCard.tsx 做最小改动：
    1. VerdictCardProps 增加可选字段 chatText?: string，注释说明语义："调用方（MessageBubble 经 DataPolicyPart）传入本条消息全部 text parts 的拼接文本；当该文本已逐字包含 verdict.answer 时（后端 conclusionText 把 answer verbatim 嵌入 '结论：…。'，chat.go:801-809），卡片隐藏 verdict-answer 区块避免双份渲染。CenterPanel 等无 chat 流上下文的使用方不传此 prop，行为不变。"
    2. 判定逻辑放在组件体内 isFactualQuery 旁：
       const answerText = verdict.answer?.trim() ?? '';
       const answerDuplicatedInChat =
         chatText !== undefined && answerText !== '' && chatText.includes(answerText);
       渲染条件改为 {isFactualQuery && !answerDuplicatedInChat && (...)}。
    3. 不删除 DOM 节点改用 hidden class —— 与 confidence-bar 的"保留节点"模式不同：这里的诉求是消除视觉重复且 verdict-answer 没有外部 scraper 依赖，直接不渲染最干净（既有测试的 getByTestId('verdict-answer') 全部在无 chatText 场景下，不受影响）。
    4. 不修改 confidence-bar hidden node、StatusBadge、CitationCluster、「N 条引用」计数、review-required-badge 的任何逻辑。
    5. 文件头注释补充一行本次变更说明（遵循文件已有的注释编年体例，引用 260817-hck）。

    禁止：不要在 FE 复刻 Go conclusionText 的整句推导（跨语言逻辑重复 = 漂移面，违背 D15 精神）；不要动后端 chat.go（text-delta 结论必须保留）；不要给 chatText 做正则/模糊匹配（verbatim substring 足够，后端是原子单次 text-delta）。
  </action>
  <verify>
    <automated>cd web && npx vitest run src/__tests__/VerdictCard.test.tsx</automated>
  </verify>
  <done>VerdictCard.test.tsx 全部通过（既有 10 个 + 新增 ≥3 个）；重复场景 verdict-answer 不渲染，其余场景行为与改动前完全一致；confidence-bar hidden node 断言仍绿</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: MessageBubble → DataPolicyPart prop 透传 + 集成测试 + 全量验证</name>
  <files>web/src/components/chat/MessageBubble.tsx, web/src/components/chat/parts/DataPolicyPart.tsx, web/src/__tests__/MessageBubble.test.tsx</files>
  <behavior>
    - 集成测试（MessageBubble.test.tsx 新增）：构造 assistant UIMessage，parts = [text("结论：由部门负责人审批。"), data-policy(verdict with answer="由部门负责人审批")] → 渲染后 text 正文可见且 queryByTestId('verdict-answer') 为 null，getByTestId('verdict-card') 存在
    - 对照集成测试：同一 data-policy verdict 但 text part 正文不含 answer → verdict-answer 渲染
    - 既有 MessageBubble 测试全部保持绿色（reasoning 在 text 之前的 DOM 顺序、7 种 part 渲染、unknown part 静默丢弃等）
    - npx tsc --noEmit 零错误（D15 编译期告警链不断）
  </behavior>
  <action>
    RED：先在 MessageBubble.test.tsx 追加上述两个集成测试（fixtures 参考该文件已有的 data-policy part 构造方式；verdict payload 结构同 VerdictCard.test.tsx 的 factualQuery fixture），运行确认新测试失败（当前 verdict-answer 总渲染）。

    GREEN：
    1. DataPolicyPart.tsx：props 类型增加可选 chatText?: string；isVerdict 分支改为 <VerdictCard verdict={data} chatText={chatText} />。refusal 分支与 unknown-payload 分支不动。更新文件头注释一行说明透传职责。
    2. MessageBubble.tsx assistant 分支：在 return 前计算
       const chatText = message.parts
         .filter((p) => p.type === 'text')
         .map((p) => (p as { text: string }).text)
         .join('');
       renderPart 签名扩为 renderPart(part, i, chatText)，仅 'data-policy' 分支使用第三个实参：<DataPolicyPart key={i} part={...} chatText={chatText} />；其余分支忽略该参数。user 分支不动。
    3. 不要把 chatText 放进 Zustand store（D14：useChat 拥有消息流，store 禁 messages 字段；prop 下钻是正解，三级且类型可选，成本极低）。

    全量验证：跑整个前端受影响面 + 类型检查。
  </action>
  <verify>
    <automated>cd web && npx vitest run src/__tests__/MessageBubble.test.tsx src/__tests__/VerdictCard.test.tsx && npx tsc --noEmit</automated>
  </verify>
  <done>MessageBubble 集成测试 pin 住"text 含结论 → 卡片 verdict-answer 隐藏"的端到端行为；全部既有测试绿；tsc --noEmit exit 0；DataPolicyPart 的 refusal/unknown 分支行为不变</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| backend SSE → FE renderer | verdict JSON 与 text-delta 均来自后端 SSE 流（既有边界，本任务不新增） |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-hck-01 | Tampering | chatText substring 判定 | accept | 判定仅影响视觉展示（隐藏/显示重复文案），不影响数据完整性；误判的最坏后果是答案文本多显示一次（现状）或卡片内少显示一次（正文仍在）。无安全语义。 |
| T-hck-02 | Info Disclosure | verdict-answer 条件隐藏 | accept | 隐藏仅发生在同一消息正文已逐字包含同一文本时，无信息泄露面变化；CenterPanel 独立路径不传 prop，行为不变。 |

本任务无 npm/pip/cargo 安装，无 T-hck-SC 供应链项。
</threat_model>

<verification>
- `cd web && npx vitest run src/__tests__/VerdictCard.test.tsx src/__tests__/MessageBubble.test.tsx` — 全绿
- `cd web && npx tsc --noEmit` — exit 0
- 全量回归：`cd web && npx vitest run` — 全绿（基线 263/263 + 本任务新增用例）
- 结构抽查：`grep -n "chatText" web/src/components/chat/MessageBubble.tsx web/src/components/chat/parts/DataPolicyPart.tsx web/src/components/center/VerdictCard.tsx` — 三文件均有命中
</verification>

<success_criteria>
- 助手消息中结论只出现一次：聊天正文保留「结论：…」，VerdictCard 在重复时只显示 StatusBadge(+ComplianceStatusBadge) + 「N 条引用」+ CitationCluster
- CenterPanel 的 结论 tab 中 VerdictCard（无 chatText）仍完整显示 verdict-answer
- refusal 路径（RefusalCard）与 compliance 审查 badge 逻辑零变化
- confidence-bar hidden-node 测试与全部既有测试保持绿色，tsc 零错误
</success_criteria>

<output>
Create `.planning/quick/260817-hck-verdictcard-dedupe-answer-vs-chat-conclu/260817-hck-01-SUMMARY.md` when done
</output>
