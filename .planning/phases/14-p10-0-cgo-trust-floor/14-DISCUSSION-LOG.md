# Phase 14: CGO 地基 + 平台信任地板 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-22
**Phase:** 14-CGO 地基 + 平台信任地板（P10.0）
**Areas discussed:** 平台地板文案边界, 地板版本化+审计携带, platform-floor 冻结集口径, DuckDB 数据目录布局

---

## Area 选择

全部 4 个灰区被选中讨论（multiSelect 全选）。

## 平台地板文案边界

### Q1: 地板铁律条目集

| Option | Description | Selected |
|--------|-------------|----------|
| 五条核心 | SQL 禁令/原始行禁令/as_of+引用/拒跨数据集推断/脱敏不可逆声明 | |
| 七条扩展 | 五条 + 去脱敏再识别禁令 + 实时数据声明 | ✓ |
| 三条最小 | SQL 禁令 + 原始行禁令 + 引用要求 | |

**User's choice:** 七条扩展
**Notes:** 接受更长提示词换取 ⑥⑦ 两道防线（去脱敏/再识别禁令含提示词诱导场景；实时数据声明保护 as_of 语义）。

### Q2: 地板语言

| Option | Description | Selected |
|--------|-------------|----------|
| 中文 | GLM 中文最强 / Skill 文件全中文 / 拒答引用原文一致 | ✓ |
| 英文 | 指令遵循略稳但混排 + 翻译 | |
| 中英双语 | 双保险但 token 翻倍 | |

**User's choice:** 中文

### Q3: 叠加形态

| Option | Description | Selected |
|--------|-------------|----------|
| 仅头部 prepend | 最简，与 D11-C2 兼容好 | |
| 头部+尾部提醒 | 头部全文 + 尾部一句重申，抗长对话衰减 | ✓ |
| 其他 | 用户自定形态 | |

**User's choice:** 头部+尾部提醒
**Notes:** 两处注入点都必须进 prompt_hash（D14-B3 联动）。

### Q4: 与现有模板信任条款关系

| Option | Description | Selected |
|--------|-------------|----------|
| 上提统一 | 通用信任条款上提为平台地板一处版本化，根治 Pitfall 17 | ✓ |
| 只加数据条款 | 现有条款不动，双路径漂移风险保留 | |

**User's choice:** 上提统一
**Notes:** `internal/agent/prompts.go` 现有模板的引用白名单/无 grounding 拒答条款上提；Skill 模板只留 persona/领域条款。

---

## 地板版本化+审计携带

### Q1: 版本形态

| Option | Description | Selected |
|--------|-------------|----------|
| semver 列+hash 覆盖 | 人可读常量列 + prompt_hash 自然覆盖内容 | ✓ |
| 内容 hash 列 | 自动防漂移但不可读 | |
| 双列 | 信息最全但冗余 | |

**User's choice:** semver 列+hash 覆盖

### Q2: 携带位置

| Option | Description | Selected |
|--------|-------------|----------|
| audit 主表加列 | 每行都带，SELECT DISTINCT 查漂移窗口 | ✓ |
| audit_meta 兄弟表 | 主表不动但查询多 JOIN | |

**User's choice:** audit 主表加列

### Q3: prompt_hash 语义（D11-C2 是否含地板）

| Option | Description | Selected |
|--------|-------------|----------|
| hash 覆盖合成全文 | 地板 bump = pending 审批全过期（语义正确） | ✓ |
| 只算 skill 部分 | 列不变但过期判定多一条规则 | |

**User's choice:** hash 覆盖合成全文
**Notes:** 「批的 A 不会执行成 B」的地板版。

### Q4: bump 纪律

| Option | Description | Selected |
|--------|-------------|----------|
| 全量重跑 | semver 递增 + 全部冻结集重跑绿 + 审计可查 | ✓ |
| 只跑地板集 | 快但其他冻结集行为可能受影响 | |
| 无特殊纪律 | 当普通代码变更 | |

**User's choice:** 全量重跑

---

## platform-floor 冻结集口径

### Q1: 判定方式

| Option | Description | Selected |
|--------|-------------|----------|
| 双层正则+judge | hermetic 正则断言 + live LLM judge 扫部分服从 | ✓ |
| 仅正则断言 | 完全 hermetic 但语义浅 | |
| 仅 LLM judge | 语义深但依赖 live key | |

**User's choice:** 双层正则+judge

### Q2: 用例规模

| Option | Description | Selected |
|--------|-------------|----------|
| 20 例 | 每铁律 2-3 变体，与 fanout 20-case 同量级 | ✓ |
| 35 例 | Phase 9/10 spike 口径 | |
| 50 例 | GEVL-09 KPI golden 量级（Phase 17 的事） | |

**User's choice:** 20 例

### Q3: 攻击面覆盖

| Option | Description | Selected |
|--------|-------------|----------|
| 四类基础 | 诱导 SQL / 越通道 / 要原始行 / 去脱敏 join | |
| 六类含越狱/衰减 | 四类 + 角色扮演越狱 + 长对话稀释 | ✓ |

**User's choice:** 六类含越狱/衰减
**Notes:** 第⑥类（20 轮闲聊后诱导）是 D14-A3 尾部提醒的验收对象。

### Q4: 锁文件归属

| Option | Description | Selected |
|--------|-------------|----------|
| 本 phase 即锁 | platform-floor.lock.json 第 4 冻结锁，data.lock 顺延第 5 | ✓ |
| 并入 Phase 20 | 少一锁文件但 14-19 无防线 | |

**User's choice:** 本 phase 即锁
**Notes:** ROADMAP Phase 20 SC-5「第 4 冻结锁」表述需随 plan 期微调为第 5。

---

## DuckDB 数据目录布局

### Q1: 文件组织

| Option | Description | Selected |
|--------|-------------|----------|
| 每数据集一文件 | 物理隔离即防线 / TTL=DETACH+删文件 / 锁边界清晰 | ✓ |
| 单库多 schema | 跨库查询原生但不还空间 + 全库锁 | |
| 冷热混合 | 多一层管理，v1 过度设计 | |

**User's choice:** 每数据集一文件

### Q2: 元数据真源

| Option | Description | Selected |
|--------|-------------|----------|
| sqlite 新表 | 与 audit/checkpoint 同库事务一致 | ✓ |
| DuckDB catalog | 跨引擎事务一致性难保 | |

**User's choice:** sqlite 新表

### Q3: ATTACH 时机

| Option | Description | Selected |
|--------|-------------|----------|
| 懒 ATTACH+LRU | 复用 Phase 13 fanout LRU 模式 | ✓ |
| 启动全量 ATTACH | 简单但启动慢 + 全文件锁 | |

**User's choice:** 懒 ATTACH+LRU

### Q4: 导入写入路径

| Option | Description | Selected |
|--------|-------------|----------|
| staging+原子改名 | 失败无半成品；Phase 19 TOCTOU 模式前置 | ✓ |
| 直接写目标 | 少一步但中断留半成品 | |

**User's choice:** staging+原子改名

---

## Claude's Discretion

- 七条铁律的具体措辞（条目集已锁）
- admin 预览端点响应形态（建议全文+版本+hash）
- mattn 迁移 DSN 兼容层细节
- Makefile 构建门形态（gcc 检测 + 报错信息）
- DuckDB 微基准门断言阈值拆分
- LRU 上限默认值

## Deferred Ideas

- 数据导入管线本体 → Phase 15
- KPI 注册表 + 通道守卫 → Phase 16
- admin 管理台 UI → Phase 20
- data.lock.json 第 5 冻结锁 → Phase 20 收尾
- 跨数据集 join 应用层防线 → Phase 16（本 phase 靠物理隔离）
- 审计体量增长监控面 → Phase 20（Pitfall 20）
- VACUUM INTO 热备策略 → 里程碑 wrap

---

*Log generated: 2026-08-22 — 16 decisions across 4 areas, all recommended options or user-selected extensions (Area 1 Q1 七条扩展 / Q3 头+尾 / Area 3 Q3 六类攻击面 were user picks beyond the recommended baseline where applicable).*
