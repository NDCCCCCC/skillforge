# Phase 16: KPI 聚合通道（P10.2）- Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-24
**Phase:** 16-p10-2 (KPI 聚合通道)
**Areas discussed:** KPI 定义载体与 DSL, kpi_call 工具形态, 通道守卫执法, k_min 防线+差分门

---

## KPI 定义载体与 DSL

### Q1: KPI 定义的真源载体用什么？

| Option | Description | Selected |
|--------|-------------|----------|
| YAML 受控文件 | 镜像 datarules/loans.yaml 先例（数据人员可写 + loader fail-closed）；文件=唯一真源；签名=内容哈希 | ✓ |
| Go 代码函数 | internal/kpi/registry.go 每 KPI 一函数；每 KPI 要开发者介入 | |
| sqlite 表 + admin 管理 | ARCHITECTURE Q2 原案；admin 可写 = 注册后门面 | |

**User's choice:** YAML 受控文件
**Notes:** KPI-01 原文即「受控文件定义」；ARCHITECTURE Q2 的 sqlite 表形态被 ROADMAP SC-1 覆盖——表只能是快照非真源。

### Q2: 定义文件里的查询怎么表达？

| Option | Description | Selected |
|--------|-------------|----------|
| SQL 模板+参数占位 | SELECT + :param 占位；注册期静态校验（SELECT-only/参数全覆盖/数据集已注册/禁跨数据集 join） | ✓ |
| 声明式度量/维度 | metricflow 模式引擎拼 SQL；表达力受限且引擎也是 SQL 生成面 | |
| 混合（声明式+SQL 逃生舱） | 两套校验多兼容一种类型 | |

**User's choice:** SQL 模板+参数占位
**Notes:** SQL 生成面收敛在 loader 预编译层，agent 只见 name+params。

### Q3: KPI 参数 schema 怎么声明？

| Option | Description | Selected |
|--------|-------------|----------|
| 类型+枚举值域声明 | params: 声明类型+值域，loader 编译成 JSON schema 挂工具边界；枚举=差分门声明面 | ✓ |
| 仅类型不约束值域 | 枚举校验变运行时行为 | |
| period 一等参数特殊化 | period 本质是 pattern 约束的 string | |

**User's choice:** 类型+枚举值域声明
**Notes:** 越域参数在进 SQL 绑定前 fail-closed 拒绝。

### Q4: 注册期预编译成什么？

| Option | Description | Selected |
|--------|-------------|----------|
| 内存 registry+启动期编译 | 校验后编译 {name, sql, param schema, 数据集, 元数据, 签名}；结构性测试锁运行时不可变 | ✓ |
| sqlite kpi_function 快照表 | 文件改了表没同步 = 两真源漂移 | |
| 运行时按需读文件 | 每次调用多 IO 且不可变量难锁 | |

**User's choice:** 内存 registry+启动期编译
**Notes:** admin 只读视图 = 文件加载结果 + 审计行组合，无需新表。

---

## kpi_call 工具形态

### Q1: kpi_call 的工具粒度？

| Option | Description | Selected |
|--------|-------------|----------|
| 单一 kpi_call 工具 | ARCHITECTURE Q2 形态 Call(ctx, name, paramsJSON)；目录内嵌 description；与 retrieve 同构 | ✓ |
| 每 KPI 一个工具 | KPI 数=工具数，20+ 时 tool-calling 精度反降 | |

**User's choice:** 单一 kpi_call 工具

### Q2: 无匹配 KPI 时的结构化拒答走哪条路径？

| Option | Description | Selected |
|--------|-------------|----------|
| 工具层结构化错误 | {reason_code: no_matching_kpi, human_explanation, available_kpis}；agent 转述 | ✓ |
| 短路 verdict 拒答管线 | 需 callback 层拦截工具结果；丢失可用目录信息 | |
| query 前置分类拒答 | 匹配判断与参数解析强耦合，前置层不准 | |

**User's choice:** 工具层结构化错误
**Notes:** 无匹配是工具调用结果而非 query 分类问题——信息最准的层拒答。

### Q3: KPI 结果怎么流回 agent 与前端？

| Option | Description | Selected |
|--------|-------------|----------|
| 结构化快照+data-reference | {func, args, as_of, rows, k_min_applied} + D16 通道扩 data-reference part（不扩 wire union） | ✓ |
| 纯文本结果 | SSE 流无结构化引用，FE 无法 hover 溯源 | |
| 仅审计行，wire 层留 P17 | wire 契约无 hermetic 锁，P17 改动面反大 | |

**User's choice:** 结构化快照+data-reference
**Notes:** Phase 17 GEVL-10（答案数字与快照一致）与 sparkline 以此为地基。

### Q4: as_of 语义从哪里来？

| Option | Description | Selected |
|--------|-------------|----------|
| 系统自动生成 | datasets 注册表最新 period 水位 + 调用时刻；agent 不可伪造（铁律⑦结构性落地） | ✓ |
| agent 参数提供 | 可伪造「新鲜数据」话术；supersede 后旧 period 已删 | |

**User's choice:** 系统自动生成
**Notes:** 历史回放走审计行快照，不重新查询。

---

## 通道守卫执法

### Q1: data:aggregated_only 与 tier reads 的关系？

| Option | Description | Selected |
|--------|-------------|----------|
| 互斥（二选一） | at most ONE data:* 延伸；两套存储/证据体系不混读；D14-D1 物理隔离同构 | ✓ |
| 可并存（混合 Skill） | 既查制度又查数；两套引用体系双轨，v1 复杂 | |

**User's choice:** 互斥（二选一）
**Notes:** 混合 Skill 记 deferred，业务明确后再开。

### Q2: 加载期通道守卫校验什么？

| Option | Description | Selected |
|--------|-------------|----------|
| 加载期交叉验证+拒绝 | reads 模式 × 工具集交叉验证，违规 = Skill 加载失败（启动期+热重载期） | ✓ |
| 仅运行时静默过滤 | 违规 Skill 无声存在；与 DATA-08 原文相悖 | |
| （见下一问） | 合并到下一问处理 | |

**User's choice:** 加载期交叉验证+拒绝
**Notes:** DATA-08 原文「跨通道违规 = 加载期拒绝而非运行时行为」直接落地。

### Q3: 运行时是否再加一层 Tool 拼装兜底？

| Option | Description | Selected |
|--------|-------------|----------|
| 双层镜像 D11-B1 | 加载期 fail + 运行时按 reads 拼 Tool 集（LLM 看不到越通道工具） | ✓ |
| 仅加载期一层 | 热重载窗口内已激活 session 无兜底；与 writes: 双层不对称 | |

**User's choice:** 双层镜像 D11-B1
**Notes:** 与 writes: 执法（D11-B1+B3）完全同构，先例代码直接镜像。

### Q4: KPI 目录对不同 Skill 的可见性？

| Option | Description | Selected |
|--------|-------------|----------|
| 平台级共享目录 | singleton registry 无 per-Skill 子集；同 KPI 不同 Skill 必同结果 | ✓ |
| per-Skill KPI 子集 | manifest 声明 allowlist；定义层差异面，审计对账复杂化 | |

**User's choice:** 平台级共享目录
**Notes:** FEATURES differentiator：定制在调用层（Skill steps 选哪些 KPI）不在定义层。

---

## k_min 防线+差分门

### Q1: KPI 结果行数 < k_min 时的行为？

| Option | Description | Selected |
|--------|-------------|----------|
| 行级抑制+标记 | 该行替换为 suppressed 标记，其余正常；快照如实记录 k_min_applied | ✓ |
| 整体拒答 | 一个小桶拖死整个查询；agent 学会避开敏感参数丢失审计面 | |
| 动态桶合并 | 桶边界漂移与 D15-D1 跨期稳定冲突 | |

**User's choice:** 行级抑制+标记
**Notes:** Phase 17 再识别 eval 断言以此行为为准。

### Q2: 差分成本门的形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 枚举值域即白名单 | params 枚举声明控制粒度，注册期拒细粒度组合；改粒度=改文件=git 评审 | ✓ |
| 白名单+运行时差分计数 | 跨调用状态+检测算法 v1 复杂度高；agent 可换 session 绕过 | |

**User's choice:** 枚举值域即白名单
**Notes:** 运行时差分计数留 Phase 17 eval 验证枚举白名单是否足够后决策。

### Q3: result_snapshot 冻结多大范围？

| Option | Description | Selected |
|--------|-------------|----------|
| 全量结果集 | rows + suppressed + as_of + k_min_applied；聚合结果天然有界，回放无损失 | ✓ |
| top-N 截断 | 回放不完整，KPI-02 打折 | |
| 仅存查询引用 | 数据清理后无法重查，违背 KPI-02 原文 | |

**User's choice:** 全量结果集

### Q4: KPI 调用的审计行落哪张表？

| Option | Description | Selected |
|--------|-------------|----------|
| 独立 kpi_calls 表 | 镜像 D15-02-1 import_batches 先例；ADM-03 直查无需 JOIN；大 JSON 不混主表 | ✓ |
| audit 主表+meta 兄弟表 | 跨表 JOIN 查视图；大快照与轻量 meta 混用 | |

**User's choice:** 独立 kpi_calls 表

---

## Claude's Discretion

- KPI 定义文件目录约定与 YAML schema 具体字段形态
- SQL 模板静态校验的具体规则集（SELECT-only 判定、占位符语法、方言校验深度、聚合函数要求）
- kpi_call 工具 description 目录生成口径（截断/排序/参数摘要）
- data-reference part 具体 wire payload 字段
- k_min 抑制实现位置（SQL HAVING vs Go 后过滤）
- kpi_calls 表具体 DDL 列集
- 通道守卫违规报错文案
- registry 签名哈希口径（单文件 vs 目录聚合）
- KPI 定义文件热重载行为（镜像 SYSK-04 fsnotify vs 仅启动期加载；热重载=新 registry 原子替换非原地增改）
- admin 只读 API 端点是否本 phase 交付
- period 数组参数（跨期比较一次调用）

## Deferred Ideas

- 混合 Skill（tier + data 并读）— v1 互斥后业务明确再开
- 运行时差分计数防线 — Phase 17 eval 验证后决策
- KPI 仪表盘 Skill + 再识别 eval 冻结集 — Phase 17
- admin KPI 只读视图 — Phase 20 ADM-03
- 声明式度量 DSL — v1.1+
- per-Skill KPI 子集 — 拒绝（定制在调用层）
- KPI 结果缓存 TTL — 不做（金融新鲜性默认 always live）
- data.lock.json 收录范围 — Phase 20
