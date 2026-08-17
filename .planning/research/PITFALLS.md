# Pitfalls Research — v0.8 Harness Milestone (可信×行动融合)

**Domain:** Adding harness-class capabilities (multi-turn memory, HITL write tools, DeepAgent planning, sub-agent fan-out) to an existing trusted-Q&A agent whose moat is trust-floor refusal + verbatim citations + INSERT-only audit
**Researched:** 2026-08-17
**Confidence:** HIGH for Eino / AI SDK facts (Context7-verified 2026-08-17); MEDIUM-HIGH for integration pitfalls (synthesized from system invariants + framework docs)

**Phase naming note:** v0.8 roadmap phases are not yet numbered. Labels below are proposals for the roadmap author:
- **P8.0** — 前置：全量依赖升级 (eino v0.7.7 → v0.8.x/v0.10 era; summarization middleware + new HITL APIs live here)
- **P8.S** — 前置 spike：历史注入对 trust floor 的影响（seed: `spike-history-injection-trust-floor`，H1–H3）
- **P8.1** — 多轮对话记忆 (sqlite history → agent + summarization)
- **P8.2** — 写工具 + 审批门 (ADK Interrupt/Resume + 审计行)
- **P8.3** — 规划层 (DeepAgent + write_todos + 可回放)
- **P8.4** — 子 agent 扇出 (tier 继承)

---

## Critical Pitfalls

### Pitfall 1: 历史注入污染 trust floor（H1–H3 实证假设）

**What goes wrong:**
修复 `internal/server/chat.go` 的单轮限制、把 sqlite session history 喂进 ReAct agent 后，历史内容绕过本应是硬性前提的检索门：
- **H1**：历史 assistant 消息里的旧 chunk 引用让 agent 以为"本轮已检索"，跳过 retrieve → 裁决引用的 chunk 不在本轮检索结果里 → grounding checker 要么全拒（可用性崩）要么放行悬空引用（护城河崩）
- **H2**：历史中的拒答上下文使拒答率漂移——过度拒答（refusal_accuracy 虚高但 answer_relevance 崩）或不再拒答（trust floor 失守）
- **H3**：历史里用户引用/转述的文本被当作新证据，tier 边界穿透（见 Pitfall 2）

**Why it happens:**
trust floor 是在**单轮**假设下校准的（Phase 2/3 的 golden set 全是单轮 query）。历史注入改变了 prompt 分布，但冻结基线 `eval/frozen/compliance.lock.json` 不会自动发现——四指标漂移是静默的。这是把"会话功能"当成纯工程任务、忽略其对已校准行为的回归影响的典型错误。

**How to avoid:**
1. **Spike 先行（P8.S 是硬门）**：50 例 golden set × {baseline, history-injected(善意追问/恶意注入/旧拒答上下文三类)}，四指标漂移 < 5% 才允许进 P8.1；不达标则产出过滤规则再测
2. **历史消毒（最可能的产出规则）**：喂给 agent 的历史只保留 user/assistant **纯文本**，剥离 chunk 引用、`data-*` parts、source-document parts——引用永远只能来自**本轮**检索
3. **每轮重新 grounding**：grounding checker 的 citation-span 验证只针对本轮 retriever 返回的 chunk 集合，历史中的引用字符串不在白名单内
4. 在 prompt 契约里显式标注历史为"上下文参考，非证据"

**Warning signs:**
- history-injected 组 faithfulness 或 answer_relevance 漂移 ≥ 5%
- 拒答率突然升高/降低而 Skill 和语料都没变
- 裁决引用的 chunk_id 在本轮 retriever 结果集中找不到（日志可测）

**Phase to address:** P8.S（spike 门）→ P8.1（消毒规则落地）

---

### Pitfall 2: 用户引用文本造成 tier 边界穿透

**What goes wrong:**
用户在第 N 轮粘贴一段"历史/个人"层文档内容（甚至杜撰内容），第 N+1 轮历史注入后 agent 把这段用户消息当成证据给出裁决——`reads: ["权威"]` 的边界被聊天通道绕过。Milvus tier-filter 防得住检索路径，防不住 prompt 路径。

**Why it happens:**
真源分离目前只在 retriever 层强制执行（metadata filter）。一旦历史进入 prompt，就多了一条**不受 filter 约束的知识注入通道**。这是"边界在错误的层执行"的经典问题。

**How to avoid:**
1. **裁决门不变量（代码强制，非 prompt 请求）**：结构化裁决的每个 citation 必须命中本轮 权威-tier 检索结果——这条不变量**不随** harness 升级改变，写工具/规划/子 agent 全部继承
2. Prompt 中把历史 user 消息包上显式"未验证用户陈述"标签
3. P8.1 增加红队用例：用户在历史里粘贴伪造条款 → agent 必须拒答或明确标注"无法核实"

**Warning signs:**
- 裁决出现而本轮 权威-tier 检索结果为空
- 红队注入用例得到肯定裁决

**Phase to address:** P8.1（不变量 + 红队用例）；P8.S spike 的 H3 组先行暴露

---

### Pitfall 3: 审批门破坏"每步可审计"——部分写无审计行

**What goes wrong:**
写工具（memory:facts 写入、报告生成）的执行路径是：interrupt → 用户批准 → resume → 执行写。三条典型翻车路径：
1. **批准后执行失败**：审计行在写成功后 INSERT → 写成功但审计 INSERT 失败 → 有一次无审计的写（T-3-AUDIT 不变量破）
2. **审计先行但无状态**：审计行先 INSERT 但不含 approved/pending 状态 → 审批被拒的操作在审计里看起来像是执行了
3. **resume 后上下文错位**：审批时的工具参数与 resume 后实际执行的参数不一致（见 Pitfall 5 的 HistoryModifier 风险）→ 审计记录的不是实际发生的写

**Why it happens:**
现有审计是"事后记录"模型（Phase 2：响应流完后写审计行，失败仅 log——`chat.go` 头注释明确写了 swallow 策略）。对**读**操作这是可接受的降级；对**写**操作这是护城河破口。"可信×行动"的第一次正面验证如果第一笔写就无审计，差异化声明直接破产。

**How to avoid:**
1. **写前审计意图行（write-ahead）**：interrupt 触发时先 INSERT `status=pending` 审计行（含 tool_call_id、参数快照、skill 版本、prompt_hash）；批准→resume→执行成功后在**同一逻辑流**内 INSERT `status=executed` 行。拒绝则 INSERT `status=denied`。保持 INSERT-only（用状态字段的行追加而非 UPDATE，或允许对 audit 表仅此一例 UPDATE——需在 Phase 决策中显式记录）
2. **审计行绑定 checkpoint ID**：审计行记录 `WithCheckPointID` 的 ID，使审计 ↔ 可回放状态 1:1 关联
3. **审计写失败 = 写不执行**（fail-closed）：与读路径的 swallow 策略相反，写路径必须 fail-closed。这是 D-级决策，写进 Key Decisions

**Warning signs:**
- 审计表出现 tool_call 有 executed 行但无对应 pending 行（或反之）
- 审计参数快照与实际副作用不一致（抽查回放）

**Phase to address:** P8.2

---

### Pitfall 4: SSE 断连 + 内存 checkpoint = 中断态丢失

**What goes wrong:**
agent interrupt 等待审批时，用户刷新页面 / 网络断开 / 服务器重启：
- `store.NewInMemoryStore()`（官方 HITL 示例的默认）→ checkpoint 全丢，审批请求变成孤儿，前端永远停在 `approval-requested`
- 用户重新发同一条消息 → 写工具可能执行两次（第一次的 checkpoint 其实还在，只是前端丢了 handle）
- 审批决定已发出但 SSE 已断 → 后端执行了写，前端从未收到结果，用户以为没执行又批准一次

**Why it happens:**
官方 quickstart 用 InMemoryStore 演示（"In the real world, you can use a distributed store like Redis"——docs 原话），照抄示例代码就把这个生产陷阱原样搬进来。SSE 是无状态单向流，HITL 把"等待用户"这段长时间窗口插进了流中间，放大了断连概率。

**How to avoid:**
1. **P8.0/P8.2 起就用持久 CheckPointStore**（sqlite 表即可，与现有 session 存储同库；Redis 是过度工程）：实现 `CheckPointStore` 接口（`Set/Get`，[]byte，接口极小）
2. **gob.RegisterName 注册所有自定义类型**（ADK 用 gob 序列化 checkpoint；未注册类型在 resume 时才炸，属于延迟引爆）
3. **幂等键**：每个写工具调用携带 `tool_call_id` 作为幂等键，执行前查审计表——已 executed 则直接返回上次结果
4. **前端 resume 协议**：ai SDK `useChat({ resume: true })` + 服务端 GET resume 端点（`DefaultChatTransport` 支持自定义 resume URL）；页面刷新后重新拉到 `approval-requested` 状态而非丢失
5. checkpoint 设 TTL + 显式过期态 UI（"该审批已过期，请重新发起"）

**Warning signs:**
- 用户报告"我点了批准但什么都没发生"
- 审计表出现同一业务动作的重复 executed 行（幂等键缺失的直接证据）
- 重启服务后出现永远 pending 的审批

**Phase to address:** P8.0（持久 store 骨架）→ P8.2（幂等 + 前端 resume）

---

### Pitfall 5: Interrupt/Resume 重构迁移陷阱（legacy error 无地址 + HistoryModifier 改历史）

**What goes wrong:**
Eino v0.7.0+ 重构了 HITL（`compose.Interrupt` / `ExtractInterruptInfo` / `ResumeWithParams` + address-aware interrupt contexts），旧 API（`NewInterruptAndRerunErr`）仍兼容但有坑：
1. **legacy interrupt 无地址**：旧式 error 不带 address，`ResumeWithParams` 的 `Targets` map 匹配不上 → resume 数据送不到目标 interrupt 点，或多个 interrupt 点时分错。必须用 `compose.WrapInterruptAndRerunIfNeeded(ctx, segment, err)` 显式包地址
2. **新旧混用**：迁移期一部分工具用新 API、一部分用旧 API → 同一 run 里两种 interrupt 语义并存，resume 行为不可预测
3. **HistoryModifier 滥用**：`ChatModelAgentResumeData.HistoryModifier` 允许 resume 时改写历史——这正好撞上 Pitfall 1/2：审批环节如果有人用它"修正"历史，trust floor 的输入在审批前后不一致，审计和实际执行脱节

**Why it happens:**
官方明确说"existing code continues to function without automatic modification"——渐进采用是 feature，但对一个有信任不变量的系统，"静默双语义并存"是 bug 温床。迁移文档（v0.8 breaking changes：`ShellBackend→Shell`、`ReadRequest.Offset` 0-based→1-based、`GrepRequest.Pattern` literal→regex、`WriteRequest` 改为覆盖写）也说明大版本升级的 breakage 是多点、细微的。

**How to avoid:**
1. **P8.0 一次性切干净**：全仓禁止 `NewInterruptAndRerunErr`（lint 规则或 code review checklist），新写工具只用 address-aware API；若必须保留 legacy 路径，强制 `WrapInterruptAndRerunIfNeeded` 包装
2. 按 v0.8 upgrade guide 逐项核对（Offset 1-based、Grep regex 转义、WriteRequest 覆盖语义、FileInfo.Path 不再保证绝对路径——最后一项影响 skill/knowledge 文件路径逻辑）
3. **禁止在 resume 数据里改历史**：`HistoryModifier` 只允许注入"批准/拒绝 + 批注"，不允许删改既有消息；把这条写成 code review checkpoint
4. v0.7.7 → v0.8+ 的依赖升级单独成一个 phase 做，不与功能开发混合（混合 = 回归归因困难）

**Warning signs:**
- resume 后工具拿到 nil/默认参数
- 多个 interrupt 点场景下审批数据串台
- 升级后既有 contract test（`integration/contract`）或 eval 冻结基线变红

**Phase to address:** P8.0（迁移 + 禁令）→ P8.2（HITL 落地时复检）

---

### Pitfall 6: checkpoint 跨版本失效（skill 热加载 / prompt 漂移 / 部署重启）

**What goes wrong:**
checkpoint 序列化的是 interrupt 那一刻的运行状态（输入、历史、interrupt info）。但 SkillForge 的 Skill 是**热加载**的：interrupt 后、resume 前，有人改了 `SKILL.md` 或部署了新版本 → resume 出来的 run 用的是旧 prompt 语义 + 新代码（或反之），审计里记的 prompt_hash 和实际执行的对不上，SC-6 可复现性破。

**Why it happens:**
checkpoint 的 durability 和代码/配置版本是正交的，框架不管这个。热加载是 SkillForge 的核心卖点，HITL 引入了第一个"运行状态存活时间 > 配置版本存活时间"的场景。

**How to avoid:**
1. checkpoint 元数据里记录 `skill_name + skill_version/prompt_hash`（复用 `internal/reprohash` 的 3 参数 hash）
2. resume 前比对：版本不一致 → 拒绝 resume，返回结构化"审批已失效（Skill 已更新），请重新发起"——这与 trust 叙事一致（宁可作废也不模糊执行）
3. checkpoint 与 `compliance.lock.json` 的 pin 字段对齐记录（model_ver / seed）

**Warning signs:**
- resume 后输出风格/拒答行为与 interrupt 前明显不同
- 审计行 prompt_hash 与当前 skill 渲染 hash 不匹配

**Phase to address:** P8.2

---

### Pitfall 7: 摘要中间件侵蚀 trust floor + token 计数口径不一致

**What goes wrong:**
`adk/middlewares/summarization`（v0.8.x 时代能力，需 P8.0 升级才可用）在 token 超阈值时调 LLM 生成摘要替换历史：
1. **摘要改写引用**：摘要 LLM 会 paraphrase——历史里逐字引用过的条款被改写后，grounding checker 的逐字 span 验证对历史引用全部失效；更糟的是摘要被后续轮当证据
2. **拒答上下文丢失**：摘要有损压缩会把"我之前拒答过这个，因为缺权威来源"压成"我们讨论过 X" → 下一轮直接答（trust floor 破）或重复检索（成本翻倍）
3. **token 计数口径**：`TriggerCondition.ContextTokens` 的计数器若与 GLM 实际 tokenizer 不一致（中文字符/tok 比率、tool 消息的开销），要么触发太晚（context overflow 直接 500）要么触发太早（每次 LLM 调用前先付一次摘要 LLM 调用，成本翻倍）

**Why it happens:**
摘要中间件是为"长对话不丢连贯性"设计的通用件，它的默认行为不知道 SkillForge 有"引用必须逐字可验"的不变量。token 计数在 docs 里就是一个 int 阈值，计数器实现细节容易当成"框架会处理好"。

**How to avoid:**
1. **摘要不进证据链**：与 Pitfall 1 的消毒规则同构——摘要文本在 prompt 中标注为"对话梗概，非证据"；裁决 citations 仍只接受本轮检索
2. **保留窗口**：用 `TriggerCondition.ContextMessages` + 最近 N 条原文保留，确保最近的拒答/裁决上下文不被压缩；触发阈值在 P8.1 用真实 GLM 用量日志校准（对比 middleware 计数 vs API 返回 usage）
3. **摘要本身是额外 LLM 调用**：计入每会话成本预算（见 Pitfall 8），eval 里加"摘要触发后四指标"专项
4. 配置 `TranscriptFilePath` 保留压缩前原文——既是调试物证也是"可回放"叙事的素材

**Warning signs:**
- 长对话尾部轮次 faithfulness 系统性低于头部轮次
- 每会话 LLM 调用次数随轮次超线性增长
- 触发摘要的轮次上下文溢出 500

**Phase to address:** P8.0（拿到中间件）→ P8.1（阈值校准 + 证据隔离）

---

### Pitfall 8: DeepAgent 成本爆炸与无界循环

**What goes wrong:**
`adk/prebuilt/deep` 的 DeepAgent = 主 agent + write_todos + task（子 agent 调用）+ 文件系统中间件。成本敏感产品上的翻车路径：
1. **MaxIteration 默认值 50**（官方 LocalBackend 示例即 50）——每轮 iteration 是全量历史 + 工具结果进 LLM，50 轮 × 长历史 = 单次问答几十块人民币
2. **plan drift**：write_todos 写的计划在中途失效（检索结果显示前提错了），agent 不重规划而是硬执行剩余 todos → 又贵又错
3. **todo 状态机空转**：pending/in_progress/completed 转换条件靠 prompt 自觉，模型反复"更新计划"而不推进任务（observed failure mode of todo-style agents）
4. **历史随轮次二次增长**：每 iteration 的输入包含全部先前 iteration → token 成本是 iteration 数的平方级

**Why it happens:**
DeepAgent 是 prebuilt 通用件，默认预算假设是"开发者 demo"不是"成本敏感产品"。规划层的价值叙事（可回放轨迹）容易让人先跑起来再算账。

**How to avoid:**
1. **三层预算硬门**（P8.3 第一天就加）：`MaxIteration` 显式调小（建议 ≤10 起步）、每 run token 预算（超限 abort 并返回结构化"超出预算"）、每会话日预算
2. **回路检测**：连续 2 次 iteration 无新工具结果/无 todo 状态变化 → 强制收敛（输出当前结论或拒答）
3. **plan 变更要审计**：write_todos 的每次调用落审计行——这正好喂给"规划轨迹可回放"叙事，预算控制和可回放是同一份数据
4. eval 增加成本维度：golden set 跑规划层时记录 token/成本，回归门不止四指标还有 p95 成本

**Warning signs:**
- 单 run LLM 调用次数 p95 > 15
- todo 列表更新次数 > 实际工具调用次数
- 相同 query 两次运行的 plan 步数差异 > 2×（drift 信号）

**Phase to address:** P8.3

---

### Pitfall 9: 子 agent 扇出穿透 tier 边界 + 非确定性合并

**What goes wrong:**
1. **tier 泄漏**：DeepAgent 的 sub-agent "context-isolated"（官方架构说明）——隔离的是对话上下文，**不是能力配置**。如果子 agent 构造时拿到不带 tier filter 的 retriever（或父 agent 的检索结果以文本形式塞进子 agent prompt），`reads` 边界在扇出层被绕过
2. **合并非确定性**：并行子 agent 返回顺序不定，若合并逻辑依赖到达顺序 → 相同输入不同裁决 → `compliance.lock.json` 的冻结可复现性（SC-6）破
3. **重复检索**：多个子 agent 对相同/相似 query 各自调 retriever + embedding → 成本按扇出倍数线性放大
4. **子 agent 失败语义模糊**：一个子 agent 拒答、一个给出裁决，父 agent 如何合并？"部分证据下给肯定裁决"是 trust floor 最隐蔽的破口

**Why it happens:**
真源分离目前在**单 agent 的 retriever 配置**上执行。扇出把"一个 agent 一条检索路径"变成"N 个 agent N 条路径"，边界执行点必须随之复制——但代码上最容易的路径是共享一个 retriever 实例或干脆在父层检索完传文本，两条路都漏。

**How to avoid:**
1. **tier 边界代码级继承**：子 agent 构造器强制要求 `reads` 白名单参数（类型层面必填，不是 prompt），子 agent 的 retriever 由工厂按白名单包 tier filter——子 agent 无法声明父 agent 没有的 tier（只能收窄不能放宽）
2. **父→子传证据 = 传 chunk 引用 ID，不传自由文本**；子 agent 用 ID 回查（复用 grounding 白名单机制）
3. **确定性合并**：合并按键排序（doc_id / 子 agent name），不依赖到达顺序；合并逻辑写成纯函数，进 contract test
4. **检索去重缓存**：run 级（非全局）retrieval cache，key = (query hash, tier filter)
5. **合并保守原则**：任一子 agent 拒答 → 合并结果至多是"需补充"，绝不在部分拒答下合成肯定裁决；把这条写成合并函数的测试断言

**Warning signs:**
- 同一 frozen query 两次运行产出不同 chunk 集合或不同裁决
- 扇出场景 embedding API 调用次数 ≈ 子 agent 数 × 单 agent 基线
- 子 agent 日志出现父 Skill `reads` 之外的 tier

**Phase to address:** P8.4（依赖 P8.1 的引用 ID 机制 + P8.3 的预算门）

---

### Pitfall 10: 前端 ai@7 审批 UI 与流恢复的状态错位

**What goes wrong:**
1. **审批状态机漏态**：tool part 的 `approval-requested` 态需要 `addToolApprovalResponse({id, approved})` + `sendAutomaticallyWhen: lastAssistantMessageIsCompleteWithApprovalResponses` 才闭环。漏配 `sendAutomaticallyWhen` → 用户点了批准但消息不发回后端，UI 看起来"卡死"
2. **resume 后审批态丢失**：`useChat({resume:true})` 需要服务端 GET resume 端点重放流；若后端只支持 POST /api/chat（当前形态），刷新页面后 `approval-requested` 永远悬挂
3. **D-04 transport-swap 不变量被审批流打破**：现有 `transport-swap.test.tsx` 保证 Mock↔Default 字节级一致，但审批是**双向**交互（前端要回发），Mock transport 若不实现审批回环，测试绿而生产挂
4. **Zustand 无 messages 不变量的新诱惑**：审批状态（哪个 toolCallId 待批）看起来该进全局 store——一旦进 store 就违反 Pitfall 7（Phase 4 已固化的边界），审批态必须留在 `useChat` 的 message parts 里

**Why it happens:**
ai SDK 的 HITL 是"消息 parts 状态机 + 自动重发"模型，与传统"按钮调 REST"直觉相反。当前前端只有单向流，审批是第一条**回写**通道。

**How to avoid:**
1. 审批 UI 组件按 part.state 全枚举渲染（`approval-requested` / `approval-responded` / `output-available` / `output-error` / `output-denied`），缺态渲染显式 fallback 而非静默 null
2. P8.2 给后端加 GET resume 端点（与 Pitfall 4 的持久 checkpoint 配套），`DefaultChatTransport` 配自定义 resume URL
3. 扩展 `transport-swap.test.tsx`：Mock transport 必须能模拟完整 approve→resume→output 回环，fixture 覆盖 deny 分支
4. 文件级注释禁令延续：审批相关任何状态不入 `app-store.ts`

**Warning signs:**
- 点批准后 Network 面板无后续 POST
- 刷新页面后待审批卡片消失或永远 spinner
- Mock 测试绿但 staging 审批流程挂

**Phase to address:** P8.2（与后端 HITL 同 phase 交付，不可拆）

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| InMemoryStore 做 checkpoint | 零新表零代码，demo 立刻跑通 | 重启/断连丢全部中断态；与"可审计可回放"叙事直接矛盾 | **Never**（demo 也不行——这恰好是被演示的场景） |
| 历史全量注入（不消毒） | 代码最少，多轮"能聊" | H1–H3 污染 trust floor；护城河静默失效 | Never；P8.S spike 的产物就是消毒规则 |
| 审批只靠 prompt 让模型"先问再做" | 不写 interrupt 代码 | 模型自觉不可靠；无审计锚点；与 Eino ADK 原生 HITL 重复造轮子 | Never（ADK Interrupt/Resume 是一等公民，没理由绕开） |
| 摘要阈值拍脑袋（如 100k） | 不校准就上线 | 触发太晚 overflow 500 / 太早成本翻倍 | 仅 P8.1 开发期；上线前必须用真实 usage 日志校准 |
| DeepAgent MaxIteration=50 默认 | 官方示例照抄 | 单 run 成本失控 | Never；v0.8 任何 deep agent 必须显式预算三件套 |
| 子 agent 共享父 retriever 实例 | 构造简单 | tier 边界单点失守（Pitfall 9） | Never |
| 审计写失败沿用读路径的 swallow | 不改既有策略 | 写操作无审计 = 差异化声明破产 | Never；写路径必须 fail-closed（需 Key Decision 记录） |
| skill 热加载不校验 checkpoint 版本 | resume 总能成功 | 旧语义执行新审批，可复现性破 | Never（拒绝 resume 的"作废"体验反而强化信任叙事） |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Eino ADK Interrupt/Resume | 沿用 v0.7.0 前 `NewInterruptAndRerunErr` 不包地址 | 新代码只用 address-aware API；legacy 路径必须 `WrapInterruptAndRerunIfNeeded` |
| Eino CheckPointStore | 照抄官方示例用 `store.NewInMemoryStore()` | P8.0 实现 sqlite-backed store（接口只有 Set/Get，工作量小）；`gob.RegisterName` 注册全部自定义类型 |
| Eino summarization middleware | 信任 `ContextTokens` 默认计数口径 | 用 GLM 真实 usage 日志校准阈值；配 `ContextMessages` 保留窗口 + `TranscriptFilePath` 留证 |
| Eino v0.8 升级 | 升级与功能开发混在一个 phase | P8.0 独立升级 phase；逐项核对 breaking changes（Offset 1-based / Grep regex / WriteRequest 覆盖 / FileInfo.Path 非绝对） |
| DeepAgent (`prebuilt/deep`) | 默认 MaxIteration + 无 token 预算 | 显式 MaxIteration ≤10 起步 + run 级 token 预算 + 回路检测 |
| ai SDK `useChat` 审批 | 只渲染 `approval-requested`，漏 `sendAutomaticallyWhen` | `lastAssistantMessageIsCompleteWithApprovalResponses` + 全 part.state 枚举渲染 |
| ai SDK 流恢复 | 以为 `resume:true` 开箱即用 | 需服务端 GET resume 端点 + 持久 checkpoint；否则刷新即丢审批态 |
| Hertz SSE ↔ HITL | interrupt 期间 SSE 挂着等用户 | interrupt 即结束当前 SSE 帧（发 `data-*` 审批请求 chunk 后 close），审批回执走新请求——不要在 SSE 上长挂 |
| sqlite session history → agent | 直接把 DB 行塞 messages | 消毒管线（剥引用/剥 data parts）→ prompt 标注非证据 → 每轮重新 grounding |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| 历史线性增长 × 每轮全量注入 | 第 20 轮后每轮延迟/成本翻倍 | 摘要中间件 + 保留窗口（Pitfall 7） | ~15-30 轮长对话 |
| 摘要 LLM 调用叠加主调用 | 触发轮延迟毛刺 +2-5s | 阈值校准；摘要走小模型/异步预取 | 每次触发时 |
| DeepAgent iteration 二次成本 | 单 run token = O(iterations²) | MaxIteration 硬帽 + token 预算 abort | 复杂多跳 query（>5 跳） |
| 子 agent 重复检索 | embedding 调用 = 扇出数倍 | run 级 retrieval cache | multi-hop 扇出 ≥3 |
| checkpoint gob 序列化大历史 | resume 延迟随历史线性涨 | 摘要先行再 interrupt；checkpoint 只存压缩后状态 | 长对话 + 审批组合 |
| 50×5 会话隔离回归 | 历史注入后 `TestSessionIsolation50x5` 变慢/变红 | P8.1 必须重跑该测试（-race） | 历史加载进热路径时 |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| 历史中的用户引用被当证据 | tier 边界穿透（权威层被聊天通道绕过） | 裁决 citation 白名单 = 本轮检索结果（代码强制）；红队用例进 eval |
| 审批请求 chunk 伪造 | 恶意响应伪造 `data-approval` chunk 骗用户点批准 | 审批卡片校验 tool_call_id 必须存在于当前 run 的服务端记录；沿用 T-1-08 白名单消毒门思路 |
| 审批决定跨会话重放 | 拿别人的 approval id 批准自己的写 | approval id 绑定 session + checkpoint + 用户；`addToolApprovalResponse` 服务端校验 |
| checkpoint 含敏感历史被越权 Get | CheckPointID 可猜测 → 他人会话状态泄露 | checkpoint ID 用不可猜测 ID（复用 session ID 生成策略）+ 服务端鉴权绑定 session |
| resume 数据注入 | `ResumeWithParams` 的 data 直接进 prompt/工具参数 | resume data 校验 schema；HistoryModifier 禁用删改（Pitfall 5） |
| 写工具参数未在 interrupt 时冻结 | 审批看的是 A 参数，执行用 B 参数 | interrupt 时参数快照进审计 pending 行；resume 后执行前 diff 校验 |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| 审批疲劳（每个写都弹窗） | 用户无脑点批准 = 审批门形同虚设 | 分级：低风险写（memory:facts）可会话级"本次会话不再询问"；高风险（报告生成/外发）必须逐次；RiskBadge 区分 |
| 审批卡片不说"会发生什么" | 用户批了一个看不懂的操作 | 审批卡 = 工具名 + 参数 diff 预览 + 影响说明 + 关联条款引用（信任叙事延伸到审批 UX） |
| 断连/刷新后审批消失 | 用户不知道批了没有，重复操作 | 持久 checkpoint + `resume:true` + 重新进入会话时恢复 `approval-requested` 卡片 |
| checkpoint 过期/版本失效静默 | 用户点批准没反应 | 显式"审批已失效（Skill 已更新/已过期），重新发起"态——把作废也做成信任叙事 |
| DeepAgent 规划过程黑盒 | 用户看到长时间 spinner 不知道在干嘛 | write_todos 状态映射到子进度指示器（复用 FEUI-06 五步文案机制），todos 流式可见 |
| 拒答语义在多轮中混乱 | 用户不知道"它是不知道还是不能答" | RefusalCard 的超范围🚫/不知道❓区分在多轮场景保持；历史里的拒答不在 UI 重复计数 |

## "Looks Done But Isn't" Checklist

- [ ] **多轮记忆：** demo 里连续追问流畅 ≠ 完成 —— 验证 P8.S spike 四指标漂移 <5% + 红队注入用例拒答 + 历史消毒规则有测试
- [ ] **审批门：** 点击批准能执行 ≠ 完成 —— 验证审计表有 pending→executed 双行、幂等键防重、刷新页面审批态恢复、deny 分支也有审计行
- [ ] **checkpoint：** 单次 interrupt/resume 跑通 ≠ 完成 —— 验证服务器重启后 resume（持久 store）、gob 注册覆盖所有自定义类型、skill 版本变更后 resume 被拒
- [ ] **摘要中间件：** 长对话不溢出 ≠ 完成 —— 验证摘要后 faithfulness 不掉、触发阈值经真实 usage 校准、TranscriptFilePath 留证
- [ ] **DeepAgent：** 能产出 plan 并执行 ≠ 完成 —— 验证 MaxIteration/token 预算被真实触发过（有 abort 路径测试）、每次 write_todos 有审计行、回放轨迹可还原
- [ ] **子 agent 扇出：** 并行返回结果 ≠ 完成 —— 验证子 agent tier 白名单类型强制、合并纯函数确定性测试（同输入 10 次同输出）、部分拒答不合成肯定裁决
- [ ] **前端审批 UI：** 批准按钮能点 ≠ 完成 —— 验证 `sendAutomaticallyWhen` 配置、part.state 全枚举渲染、transport-swap 测试覆盖 approve/deny 回环
- [ ] **依赖升级：** `go build` 过 ≠ 完成 —— 验证 v0.8 breaking changes 清单逐项核对 + `integration/contract` 全绿 + 冻结 eval 基线不变

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| 历史污染已上线（拒答率漂移） | MEDIUM | 回滚到单轮（chat.go 特性开关）；用线上日志定位毒历史类别；收紧消毒规则；重跑 spike 门再放行 |
| 写操作无审计行 | HIGH | 立即下线写工具；从 checkpoint 存储 + 应用日志重建 pending/executed 记录补 INSERT；audit 表加对账 job（tool_call 有副作用日志但无审计行 → 报警） |
| checkpoint 丢失（内存 store 事故） | MEDIUM | 用户侧重新发起（体验损失）；工程侧换持久 store；已丢的中断态无法恢复——接受并记录 |
| Interrupt 迁移双语义并存 | MEDIUM | 全仓 grep legacy API；统一包 `WrapInterruptAndRerunIfNeeded`；加 lint 禁令；补 resume 目标匹配测试 |
| 摘要侵蚀信任（长对话 faithfulness 崩） | MEDIUM | 调大保留窗口 / 提高阈值；摘要标注非证据；最坏情况：摘要对 compliance Skill 整体关闭（per-Skill 配置） |
| DeepAgent 成本爆炸 | LOW（发现早）/ HIGH（上账单才发现） | token 预算 abort 是止血阀；事后：调小 MaxIteration、回路检测、eval 加成本回归门 |
| 子 agent tier 泄漏 | HIGH | 下线扇出；审计泄漏窗口内的裁决（哪些引用来自越权 tier）；工厂白名单改造后重跑 frozen eval |
| 审批 UI 状态卡死 | LOW | 修 `sendAutomaticallyWhen` / part.state 枚举；补 transport-swap 审批回环测试 |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. 历史污染 trust floor | P8.S（门）→ P8.1 | spike 四指标漂移 <5%；消毒规则单测；红队注入用例进 golden set |
| 2. 用户引用 tier 穿透 | P8.1 | 红队用例（历史粘贴伪造条款）必须拒答；citation 白名单断言 |
| 3. 部分写无审计 | P8.2 | 审计对账测试：每笔写必有 pending+终态行；fail-closed 路径测试 |
| 4. SSE 断连丢中断态 | P8.0（store）→ P8.2 | 重启服务后 resume 成功测试；刷新页面审批态恢复 UAT |
| 5. Interrupt 迁移陷阱 | P8.0 | legacy API lint 禁令；`integration/contract` 全绿；多 interrupt 点 resume 测试 |
| 6. checkpoint 跨版本失效 | P8.2 | skill 变更后 resume 被拒测试；审计行 prompt_hash 比对 |
| 7. 摘要侵蚀 + token 口径 | P8.1 | 摘要触发轮四指标专项 eval；阈值校准记录（middleware 计数 vs GLM usage） |
| 8. DeepAgent 成本/循环 | P8.3 | 预算 abort 路径测试；golden set p95 成本回归门 |
| 9. 子 agent tier 泄漏 + 合并不确定性 | P8.4 | tier 白名单类型强制；合并纯函数 10× 确定性测试；部分拒答合并断言 |
| 10. 前端审批/流恢复状态 | P8.2 | transport-swap 覆盖 approve/deny 回环；刷新恢复 UAT；part.state 全枚举渲染测试 |

**Ordering rationale:** P8.0（升级）必须先于一切——summarization、新 HITL API、deep agent 全在 v0.8+。P8.S spike 是 P8.1 的硬门。P8.1 的引用 ID / 消毒机制被 P8.4 复用。P8.2 的审计+checkpoint 基建被 P8.3（todo 审计、预算 abort 审计）复用。P8.3 先于 P8.4，因为 DeepAgent 自带 task 子 agent 工具，扇出的预算/回路门在 P8.3 已建好。

## Sources

- **Context7 `/websites/cloudwego_io_eino`（HIGH，2026-08-17 取）**：
  - `core_modules/eino_adk/agent_hitl` — Runner.ResumeWithParams、CheckPointStore 接口、InMemoryStore 警告（"In the real world... Redis"）、legacy `NewInterruptAndRerunErr` 与 `WrapInterruptAndRerunIfNeeded`、迁移收益说明
  - `core_modules/eino_adk/agent_extension` — gob 序列化 + `gob.RegisterName` 要求、`WithCheckPointID`
  - `release_notes_and_migration/eino_v0.8._-adk_middlewares` — v0.8 breaking changes 清单（ShellBackend→Shell / Offset 1-based / Grep regex / WriteRequest 覆盖 / FileInfo.Path）
  - `eino_adk_chatmodelagentmiddleware/middleware_summarization` — TriggerCondition{ContextTokens, ContextMessages}、BeforeModelRewriteState 钩子、TranscriptFilePath、Retry/Failover
  - `agent_implementation/deepagents` — deep.New / NewTyped、MaxIteration=50（LocalBackend 示例值）、子 agent context-isolated、write_todos + task 内建
- **Context7 `/vercel/ai`（HIGH，2026-08-17 取）**：
  - `chatbot-resume-streams.mdx` — `useChat({resume:true})` 需服务端 resume 端点 + chat id
  - `tool-approvals.mdx` — `addToolApprovalResponse`、`approval-requested`/`approval-responded` 状态、`sendAutomaticallyWhen: lastAssistantMessageIsCompleteWithApprovalResponses`、`needsApproval`（静态/条件）
  - `workflow-chat-transport.mdx` — `reconnectToStream({chatId, startIndex})`
- **本仓代码与决策（HIGH）**：`internal/server/chat.go`（单轮限制 + 读路径审计 swallow 策略）、`.planning/seeds/spike-history-injection-trust-floor.md`（H1–H3）、`.planning/notes/harness-roadmap-decision.md`（支柱顺序 + Eino 能力事实）、PROJECT.md Key Decisions（D-04 transport-swap、T-3-AUDIT INSERT-only、SC-6 冻结 pin）
- **未独立验证（LOW-MEDIUM）**：子 agent tier 继承的具体 API 形态（docs 只确认 context-isolated，未确认配置隔离机制——P8.4 需 plan 级 research）；summarization 中间件的 token 计数器实现细节（是否可调 tokenizer——P8.1 需验证）

---
*Pitfalls research for: SkillForge v0.8 Harness milestone — 可信×行动融合*
*Researched: 2026-08-17*
