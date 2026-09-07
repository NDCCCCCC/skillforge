# Phase 29: 反馈闭环 + 工单预留 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-08
**Phase:** 29-feedback-loop-ticket-interface
**Areas discussed:** 消息身份与投票幂等, 复核队列处置语义, 仪表盘指标口径, 工单创建交互

---

## 灰区选择（multiSelect）

| Option | Selected |
|--------|----------|
| 消息身份与投票幂等 | ✓ |
| 复核队列处置语义 | ✓ |
| 仪表盘指标口径 | ✓ |
| 工单创建交互 | ✓ |

**User's choice:** 全部四个灰区都讨论
**Notes:** 侦察发现服务端无消息 ID（FE `message.id` 是客户端生成的、`session_turn` 无稳定行 id 暴露）但表本身有 `id` 列——这成为第一灰区的根基。

---

## 消息身份与投票幂等

| Option | Description | Selected |
|--------|-------------|----------|
| 暴露既有 turn id | chat 流 + history 各加 additive 字段发 `session_turn.id`；零 schema 变更，服务端真相 | ✓ |
| turn ts 当消息标识 | 复用 UNIQUE(session_id, role, ts)，但 epoch ms 有碰撞风险、ts 是时间不是身份 | |
| FE uuid 上报 | wire 不动但服务端无法验证，幂等最弱，回显不可靠 | |

| Option | Description | Selected |
|--------|-------------|----------|
| history 内嵌回显 | historyEntry 加 voted_verdict；恢复会话零额外请求 | ✓ |
| 独立查询端点 | GET /api/feedback?session_id=；多一跳 + 一致性窗口 | |
| 不回显 | 违反 FBK-04，不达标 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 失败不锁定可重试 | 错误 toast + 按钮可点；服务端幂等兜底 | ✓ |
| 失败锁定 | 用户无自救路径，且与刷新回显自相矛盾 | |
| 自动重试 2-3 次 | 首次延迟高，幂等已使重试无害，收益有限 | |

**User's choice:** 每题均选 Recommended
**Notes:** chunk ids 来源（服务端从 audit 行复制）未单独提问——由 FBK-01 需求文本「反馈表含被引 chunk ids」+ 服务端真相原则直接锁定为 D-29-05。

---

## 复核队列处置语义

| Option | Description | Selected |
|--------|-------------|----------|
| 可选说明文本 | 选中后展开输入框（可空、默认直接提交），文本进队列条目 | ✓ |
| 不带说明 | 一键投票但管理员只能自己读 chunk 原文 | |
| 必填说明 | 线索最全但投票摩擦上升，压低帮助率 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 仅状态标记 | 已更新 = 人工确认 + 审计行 + 关单；变更走 Phase 27 上传 | ✓ |
| 联动上传流程 | 跳转 DocUpload 预填，上传成功自动关单；跨面板状态复杂 | |
| 强制关联新文档 | 审计链最强但对「别处已更新」场景过硬 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 按 chunk 聚合展示 | 存储每票一行；一行 = chunk + 次数 + 展开说明；一处处置关闭该 chunk 全部待复核 | ✓ |
| 逐条报告展示 | 同 chunk 报 5 次点 5 次，处置意图碎片化 | |
| 聚合 + 批量处置 | 同左但处置语义为批量（审计行形态需另定） | |

| Option | Description | Selected |
|--------|-------------|----------|
| 三处置均终态 | 已更新/误报/忽略都离开待复核（历史可筛）；误报 = 假阳性数据点 | ✓ |
| 忽略 = 搁置不终态 | 队列积压 + 需延后/唤起机制，复杂度上升 | |
| 误报忽略合并 | UI 最简但丢掉假阳性数据点 | |

**User's choice:** 每题均选 Recommended

---

## 仪表盘指标口径

| Option | Description | Selected |
|--------|-------------|----------|
| 全票分母 | helpful / (helpful + unhelpful + outdated)；outdated 率单独展示 | ✓ |
| 二票分母 | NPS 式，但跨 Skill/知识库比较因 outdated 占比失真 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 日粒度可切周 | 后端按日序列；早期数据量小，日粒度才能看到变更后波动 | ✓ |
| 周粒度 | 点少但早期每周几票，趋势噪声大且滞后 | |
| 无趋势 | 违反 FBK-03「趋势」要求 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 新「反馈」tab | AdminDashboard 第 6 个 tab；与评测 KPI 管理、EvalDashboard 语义分离 | ✓ |
| 顶部摘要横幅 | 最省但趋势图没地方放 | |
| EvalDashboard 里 | 受众混淆（评测门禁 vs 知识库运营） | |

**User's choice:** 每题均选 Recommended
**Notes:** 用户为金融 IT 数据支撑方向，明确关心指标口径的可比性——全票分母写入 CONTEXT specifics 提醒 planner 不得改回。

---

## 工单创建交互

| Option | Description | Selected |
|--------|-------------|----------|
| 展开式小表单 | 描述必填（预填摘要可改）+ 自动引用展示；提交后展示受理结果 | ✓ |
| 一击直建 | 零摩擦但自动描述对下游 IT 服务台没用处，易产生垃圾工单 | |
| 独立工单页 | 预留阶段过度设计 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 一消息一单 | UNIQUE(session_id, message_turn_id)；重复点击返回既有编号；FE 回显已建单态 | ✓ |
| 允许多张 | 对「问题持续」宽容但无防重机制，FE 无法锁定 | |

**User's choice:** 每题均选 Recommended

---

## Claude's Discretion

- webhook provider 细节（payload 格式、成功判定、重试次数/退避/超时）——配置驱动
- per-Skill / per-知识库 维度切换与时间范围筛选的具体 UI
- `sf-toast` 监听器全局挂载位置（参照 `wizard-success` 先例）
- 复核视图/反馈 tab 的组件拆分与状态管理
- 工单受理编号格式与本地 id 生成方式

## Deferred Ideas

- 工单列表的 admin 可见视图（本 phase 只落库 + 审计 + 外发）
- 误报率统计（数据基础已由误报/忽略区分保留）
- 「已更新」处置联动 Phase 27 上传的深度闭环
