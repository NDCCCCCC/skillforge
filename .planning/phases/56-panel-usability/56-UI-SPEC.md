# 56-UI-SPEC — 审批规则面板 + rule-matched 反馈（56-03 已落地实现契约）

> 本 SPEC 文档化 56-03 的已实现 FE 契约（Task 3/4/5/6 全部落地并测试通过），
> 并为 56-04（检查器 IA）锁定消费节。模式沿 55-UI-SPEC（Standards 章节）。

## 1. 审批弹窗记住范围（两级选择）

### State
- `matchType: 'exact' | 'prefix' | 'template'`（默认 exact）
- `lockedFields: Record<string, boolean>`（prefix 字段锁定，默认全锁）
- `scope: ApprovalScope`（session/skill/global，默认 session）

### 结构（data-testid 清单）
| 元素 | data-testid |
|------|-------------|
| match-type radio ×3 | `match-type-exact` / `match-type-prefix` / `match-type-template` |
| prefix 字段锁定容器 | `prefix-lock-fields`（条件渲染：prefix 选中 ∧ 候选 > 0）|
| 字段锁定 checkbox | `lock-field-<key>`（frozenArgs 顶层标量键逐键）|
| scope radio ×3 | `scope-session` / `scope-skill` / `scope-global` |
| 确认按钮 | `approve-remember-confirm` |

### 交互契约
- prefix 在 frozenArgs 无顶层标量键时**禁用** + title「参数无可锁定字段，无法构造参数模板」。
- prefix 选中展开锁定列表（frozenArgs 顶层标量键逐键 checkbox，默认全锁）→
  提交构造 `args_pattern = JSON.stringify({锁定键: 原值})`。
- 按钮文案随两级联动：`确认：批准并记住（{匹配方式中文} · {范围中文}）`。
- 提交 body：`{decision:'approve', target, frozen_args, approval_id, scope, match_type, args_pattern?}`。

## 2. rule-matched 反馈（chat 流 + 面板联动）

### 数据链
BE（56-02）data-decision 帧 `decision:'rule-matched'`（payload 含
ruleId/ruleMatchType/ruleScope/target/toolCallId）→
RealChatTransport `dispatchApprovalDecision` 分支 →
① `appendHistory`（auto-passed 行，durable record）
② `setHighlightApprovalRuleId(ruleId)`（UI-only 瞬时，不持久化——D14 只读习惯）

### 面板联动（ApprovalRulesPanel）
- `highlightApprovalRuleId` 命中的行加 `border-[var(--color-primary)] bg-primary/10`
  高亮带，**2.5s**（HIGHLIGHT_MS）后清除（setHighlightApprovalRuleId(null)）。
- SessionDrawer 关闭时高亮态暂存，打开时消费。
- 消息流内的系统事件行形态**留 56-04/UI-SPEC v2 拍板**（当前：审批历史
  AuditTimelineTab 已可见 auto-passed 行——durable record 已覆盖可见性）。

## 3. 规则面板（CRUD + hit 列 + 徽标）

### 行渲染列
| 列 | 渲染 | 状态样式 |
|----|------|---------|
| target | font-mono | — |
| scope 徽标 | SCOPE_LABEL（session/skill/global）| — |
| match_type 徽标 | MATCH_LABEL（仅此参数/同参数模板/全部调用）data-testid=`approval-rules-match-<type>` | — |
| global 徽标 | 「全局」（scope=global 时）| — |
| origin | manual 显示「手动」；approve_remember 静默 | — |
| hit_count | 「命中 N 次」（>0）/「未命中」（灰显）| — |
| last_hit_at | 上次命中 + relativeTimeZh / 「未命中」| — |
| enabled | 「已启用」/「已停用」（红）+ 停用/启用按钮（PATCH 乐观更新 + 回滚 toast）| disabled 行 opacity-60 |
| expires_at | 到期「已过期」灰显徽标（expiresAt*1000 < now）| — |

### 行操作
- 撤销 X（既有，DELETE + 乐观卸载 + fade）
- 停用/启用按钮（data-testid=`approval-rules-toggle-<ruleId>`，PATCH）

### 高亮样式
`border-[var(--color-primary)] bg-[var(--color-primary)]/10`（HIGHLIGHT_MS=2500ms）

### 空状态两段式（PANEL-03）
1. 引导：「还没有任何审批规则。批准一次时选择「记住」，或从下方建议一键创建：」
2. 建议卡（suggestions 非空）：data-testid=`approval-rule-suggestion-<target>`，
   文案「「{target}」已人工批准 {occurrences} 次」+ 一键创建按钮
   （POST template·session 规则 → toast + 刷新列表）；suggestions 空 →
   「暂无重复批准模式。」（data-testid=`approval-rule-suggestions-none`）

## 4. 检查器 IA 骨架（56-04 消费节）

### 默认 tab 生命周期矩阵（DetailPanel 5 tabs 现状）
| 触发 | tab 切换 | 空态文案 |
|------|---------|---------|
| mount | verdict（结论）| 无 pending 审批时 |
| 点引用角标 | chunk-source（引用原文）| — |
| 点 Skill 名 | skill-md | — |
| 审批 pending | verdict + 审批卡 | — |
| rule-matched 事件 | **不切 tab**（高亮态 + 审批历史可见即可）| — |

### 空 tab 文案（逐字）
- audit-timeline 空：「本会话还没有审计记录。」
- plan-replay 空：「本会话没有规划轨迹。」
- 引用原文空：「点击回答中的引用角标查看原文。」

### step 树分组结构（56-04 实现）
检索（hits × 各通道分数）→ rerank → 工具调用 → 规划步骤——执行中步骤
与已完成步骤视觉区分（进行中 spinner / 完成 ✓）。

### 写入洞排查（PANEL-03 的 audit/plan_todos 洞）
56-04 的排查矩阵：runner 中断场景的 audit 行写入条件逐点核查
（drainResumeEvents 首要嫌疑：resume 后最终回答可能绕过 writeAuditAndTurn）。
结论见 `AUDIT-WRITE-MATRIX.md`：唯一真洞（#5/#6 resume 路径）已闭合，
写入 `action_resumed_answer` INSERT-only 标记行；plan_todos 的空是
「预期空」（deep archetype 且 agent 调用 write_todos 才有行），归空态引导侧。

### 数据源与实现对照（56-04 Task 1 确认结论）

| 展示 | 数据源（已存在的唯一来源） | 实现 |
|------|--------------------------|------|
| 默认 tab | `useChat.status`（props `chatStatus`）| DetailPanel useEffect：streaming/submitted 且当前在 verdict → agent-inspector；用户已选 tab 不被抢 |
| step 树 — 检索 | `source-document` parts → `Hop.retrievals` | `inspector/step-tree.ts buildStepTree()` |
| step 树 — 工具 | `dynamic-tool` / `tool-*` parts → `Hop.toolCalls` | 同上 |
| step 树 — 规划 | plan slice `todos`（data-plan-trace）｜`write_todos` 合成回退 | 同上 |
| step 树 — 决策 | `data-decision` parts → `Hop` | 同上 |
| 执行中/已完成 | `chatStatus` + todo `in_progress` | `StepNodeStatus = active \| done` |
| 各通道分数 | **无（Phase 58 PROV-01/02 才有）** | `StepTreeNode.channels` 仅声明槽位，不渲染 |
| 引用跳转 | `Citation` 对象自身（chunk_id/tier/version/effective_from） | `CitationCluster.handleClick(citation)`，fixture 仅作缺字段回退 |
| 引用原文缺失 | `fetchChunkContent` 404 → null | 「（片段不存在）…原文已不可定位（文档可能已更新）。」不白屏 |

**裁决规则**：spec 不得要求无数据源支撑的展示 —— 上表每一行都指向
一个已存在的字段；`channels` 是唯一的显式空槽（Phase 58 填充）。

### 空 tab 文案（逐字 · 56-04 实现值）
- audit-timeline 空：「本会话还没有审计记录——发起一次提问后，检索/审批/回答轨迹会出现在这里。」
- plan-replay 空：「暂无规划轨迹」+「规划轨迹只在 deep 模式下由 agent 写入 todo 时记录；普通问答不产生规划步骤。」
- 引用原文空（未选中）：「尚未选择引用片段」+「在结论的引用列表里点击一段引用，或从知识图谱中选择一个 chunk，这里会显示原文。也可以直接点击结论里的引用角标跳到这里。」
- Agent 状态空（无 hop，ready）：「暂无 Agent 状态」；streaming 但无 parts：「等待 Agent 首个事件...」
- step 树空：「等待第一个执行步骤…」（streaming）/「本回合没有可聚合的步骤（普通问答不产生检索/工具/规划节点）。」（idle）

## Standards（继承）
data-testid 命名对齐既有前缀（approval-rules-*/match-type-*/lock-field-*）；
文案逐字锁定（本 SPEC 的中文即实现文案）；零新依赖；CLAUDE.md 红线
（FE 零推导/Honesty）全适用。
