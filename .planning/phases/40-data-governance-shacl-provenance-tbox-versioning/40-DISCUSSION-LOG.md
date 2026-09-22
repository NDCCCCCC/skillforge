# Phase 40: Data Governance — SHACL + Provenance + TBox Versioning - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-22
**Phase:** 40-Data Governance — SHACL + Provenance + TBox Versioning
**Areas discussed:** SHACL 校验器与违规处置 / Provenance 数据模型 / Tier 过滤落地范围 (SC-5) / 演进日志 + rollback UX

---

## SHACL 校验器与违规处置

### Q1: SHACL 校验器怎么实现？（单 binary 硬约束下）

| Option | Description | Selected |
|--------|-------------|----------|
| 自写轻量子集 | shapes 用标准 SHACL TTL + Go 校验引擎约 7 个 constraint type，零新依赖 | ✓ |
| 第三方 Go SHACL 库 | Go 生态 SHACL 引擎几乎空白，大概率要自己 fork 维护 | |
| JSON/Go struct shapes | 实现最简但丢 SHACL 标准兼容性，违反零硬编码精神 | |

**User's choice:** 自写轻量子集（D-SHACL-01）
**Notes:** goRDFlib v0.1.0 纯解析库无 SHACL 能力是决策前提；护城河排除外部进程（pySHACL sidecar 出局）。

### Q2: SHACL violation 时 server boot 怎么处置？

| Option | Description | Selected |
|--------|-------------|----------|
| 分级隔离 | error 级实体 triples 进隔离区不进主索引，boot 继续；warning 放行+记录 | ✓ |
| Error 阻断启动 | fail-fast 但一个坏 chunk 能让服务起不来（内网环境风险） | |
| 全放行 + 报告 | 最宽松但违反 ONT-14 "does not silently corrupt" 精神 | |

**User's choice:** 分级隔离（D-SHACL-02）
**Notes:** 决策考虑了 memory 里内网服务间歇断的教训（Phase 39 D-Run-01 已因此把 ECL 改成 boot 不自动跑）。

### Q3: /admin/governance SHACL 报告的绿/黄/红怎么定义？

| Option | Description | Selected |
|--------|-------------|----------|
| 映射隔离状态 | 红=有实体被隔离 / 黄=仅 warning / 绿=0 violation | ✓ |
| 按数量阈值分级 | 用数量而非严重度，"黄但已被隔离"组合语义模糊 | |
| 按 constraint 类型配色 | 同类型可能 error 也可能 warning，颜色不反映处置状态 | |

**User's choice:** 映射隔离状态（D-SHACL-03）

### Q4: 初始 SHACL shapes 覆盖范围？

| Option | Description | Selected |
|--------|-------------|----------|
| 全 TBox + ABox 关系 | 8 Class NodeShape + referencesTable/belongsToTier 指向存在 + note 必带 provenance | ✓ |
| 只 ECL 产出链 | TBox emitter 生成物信任不验，"boot 验 TBox+ABox" 名不副实 | |
| 最小示范 shapes | verify 验收点缺乏说服力 | |

**User's choice:** 全 TBox + ABox 关系（D-SHACL-04）

---

## Provenance 数据模型

### Q1: Provenance 怎么挂在 ABox triple 上？

| Option | Description | Selected |
|--------|-------------|----------|
| Triple 第4字段 | 可选 Provenance struct，snapshot JSON 向后兼容，零膨胀 | ✓ |
| RDF reification triples | 标准形态但 KG 规模 ~4× 膨胀，全部链路处理元 triples | |
| 旁路 provenance store | 零侵入但双写一致性维护负担 | |

**User's choice:** Triple 第 4 字段（D-PROV-01）

### Q2: 不同来源的 ABox triple，provenance 怎么标？

| Option | Description | Selected |
|--------|-------------|----------|
| 三分类 source_kind | file（SKILL.md/sql-guides 真路径）/ emitter（纯生成物）/ induction（proposal id） | ✓ |
| emitter 物全标 generated | 实现最简但 Skill 明明来自 SKILL.md 却不记 | |
| 只 ECL 物挂 provenance | ONT-15 "Every ABox statement" 打折 | |

**User's choice:** 三分类 source_kind（D-PROV-02）

### Q3: KG 查询结果的 provenance 返回策略？

| Option | Description | Selected |
|--------|-------------|----------|
| 默认返回 | struct 层零成本，无开关参数，admin 面板直接可见 | ✓ |
| 按需参数返回 | 响应更瘦但多分支、FE 两套渲染 | |
| 仅 governance 页展示 | 溯源体验割裂 | |

**User's choice:** 默认返回（D-PROV-03）

### Q4: provenance graph 展示边界？

| Option | Description | Selected |
|--------|-------------|----------|
| 溯源链主视图 | 源文件→note→表分层 + 搜索定位 + 点击明细，首屏即有意义 | ✓ |
| 纯搜索驱动子图 | 首屏空画布，需先知道实体名 | |
| 全 KG 概览图 | 数千节点卡顿，明细海里捞针 | |

**User's choice:** 溯源链主视图（D-PROV-04）

### Q5: line range 存什么粒度？

| Option | Description | Selected |
|--------|-------------|----------|
| 行号区间 | 1-based {start_line, end_line}，与 ECL ChunkMeta 对齐 | ✓ |
| 字节偏移 | 精确但人不可读，ECL meta 要加字段 | |
| 行号 + 字节双存 | 为不确定需求付当前成本 | |

**User's choice:** 行号区间（D-PROV-05）

### Q6: ECL note 重跑后 extracted_at？

| Option | Description | Selected |
|--------|-------------|----------|
| 覆盖为最新 | provenance 永远描述当前 statement 的来源事实 | ✓ |
| 保留首次时间 | 需 diff 判断，replace 语义复杂化 | |
| 双时间戳 | struct 膨胀，两时间戳差异要 UI 解释 | |

**User's choice:** 覆盖为最新（D-PROV-06）

### Q7: provenance coverage 100% 断言范围？

| Option | Description | Selected |
|--------|-------------|----------|
| 主索引+隔离区 | 任何角落的 ABox statement 都可溯源 | ✓ |
| 仅主索引 | 与 SC-1 violation 报告要 source file 自相矛盾 | |
| 分 kind 计量 | 本质还是三个 100%，断言复杂化 | |

**User's choice:** 主索引+隔离区（D-PROV-07）

### Q8: provenance 要不要同步进 Milvus？

| Option | Description | Selected |
|--------|-------------|----------|
| KG-only 不动 Milvus | 39 刚 RECREATE 过，再迁移成本高收益零 | ✓ |
| Milvus 也加字段 | 冗余便利不值全量 re-embed | |
| 显式 defer 到 41 | 行为相同但变成 deferred 而非设计决策 | |

**User's choice:** KG-only 不动 Milvus（D-PROV-08）

### Q9: TBox rollback 后 SHACL 联动？

| Option | Description | Selected |
|--------|-------------|----------|
| 自动重跑 SHACL | dangling 走标准隔离流程，零特殊分支 | ✓ |
| 不联动等下次 boot | 治理空窗，dangling 潜伏主索引 | |
| rollback 前预检确认 | 预检本身就要跑一次完整校验 | |

**User's choice:** 自动重跑 SHACL（D-PROV-09）

### Q10: pre-40 存量 snapshot 无 provenance triples？

| Option | Description | Selected |
|--------|-------------|----------|
| warning 放行 + ECL 回填 | boot 不痪，governance 黄区引导 Run ECL 补全 | ✓ |
| boot 自动重跑 ECL | 违反 D-Run-01（内网断线把 boot 拖死） | |
| error 级全隔离 | 40 上线即大批隔离，服务看起来"坏了" | |

**User's choice:** warning 放行 + ECL 回填（D-PROV-10）

### Q11: extracted_at 时钟源？

| Option | Description | Selected |
|--------|-------------|----------|
| Server 本地 UTC | time.Now().UTC() RFC3339，简单单调 | ✓ |
| 源文件 mtime | git 操作/拷贝扰动，多 note 时间无区分度 | |
| 双时间戳 | mtime 运维噪声价值低 | |

**User's choice:** Server 本地 UTC（D-PROV-11）

---

## Tier 过滤落地范围 (SC-5)

### Q1: SC-5 tier 过滤在 Phase 40 怎么落地？

| Option | Description | Selected |
|--------|-------------|----------|
| 内部函数 + admin 演示 | QueryScoped + 单测 + admin 面板选择器，41 SPARQL/MCP 复用 | ✓ |
| 新增 skill-scoped API | 40 无消费方，endpoint 建了没人调 | |
| 仅 FE 过滤 | 隔离逻辑必须服务端强制，FE 过滤防不住泄漏 | |

**User's choice:** 内部函数 + admin 演示（D-TIER-01）

### Q2: ECL 产出实体的 tier 归属怎么标？

| Option | Description | Selected |
|--------|-------------|----------|
| ECL 写入时自动挂 | note 按 KB tier，tbl 继承 note（多 triple 任一匹配可见） | ✓ |
| 只挂 note，tbl 公共 | 表名本身可能是敏感信号（etl_customer_pii），隔离有洞 | |
| 查询时动态推 | 每次查询图遍历，SHACL/可视化无法直接断言 | |

**User's choice:** ECL 写入时自动挂（D-TIER-02）

### Q3: tier 过滤的豁免语义？

| Option | Description | Selected |
|--------|-------------|----------|
| schema 豁免+数据强制 | TBox/Skill 元数据永不过滤；ABox 数据实体 fail-closed | ✓ |
| 全局 fail-closed | 查类定义都被滤掉，演示没法用 | |
| fail-open 宽松 | 漏标 = 泄漏，与护城河立场相悖 | |

**User's choice:** schema 豁免 + 数据强制 fail-closed（D-TIER-03）

### Q4: SC-5 verify 验收方式？

| Option | Description | Selected |
|--------|-------------|----------|
| 单测+集成双覆盖 | fixture KG 断言可见集 + boot→API 集成测试 | ✓ |
| 仅单元测试 | HTTP 层接线无覆盖 | |
| 仅手测演示 | 违反 SC-7 verify 自动化精神 | |

**User's choice:** 单测+集成双覆盖（D-TIER-04）

### Q5: allowedTiers 从哪读？

| Option | Description | Selected |
|--------|-------------|----------|
| SKILL.md 真源 | registry 热加载读参传入；KG hasReadAccess 仅可视化投影 | ✓ |
| KG hasReadAccess 查询 | 过滤器自引用循环，热改 reads 不生效 | |
| 双读对账 | 漂移是 emitter 一致性问题，不该查询路径兼负 | |

**User's choice:** SKILL.md 真源（D-TIER-05）

### Q6: QueryScoped 过滤单位？

| Option | Description | Selected |
|--------|-------------|----------|
| 实体级 | 实体可见则全部边可见，图不碎裂 | ✓ |
| triple 级 | 图碎成孤岛，admin 图谱演示支离破碎 | |

**User's choice:** 实体级（D-TIER-06）

### Q7: note replace 后 tbl 继承 tier 级联清理？

| Option | Description | Selected |
|--------|-------------|----------|
| 事务内重算+boot 兜底 | 运行期精确 + boot 幂等自愈，隔离语义永不残留 | ✓ |
| 仅 boot 重建 | 长期运行窗口内越权可见，隔离有时间洞 | |
| 不清理 | 残留=可见性放宽=越权风险，与 fail-closed 矛盾 | |

**User's choice:** 事务内重算 + boot 兜底（D-TIER-07）

---

## 演进日志 + rollback UX

### Q1: TBox 演进日志存哪？

| Option | Description | Selected |
|--------|-------------|----------|
| data_dir JSONL | tbox_changelog.jsonl 与版本文件家族同域同生命周期 | ✓ |
| SQLite 表 | SQL 查询能力强但日志与版本文件分家 | |
| 内嵌 TTL 注释 | 注释非结构化，rollback 日志归属混乱 | |

**User's choice:** data_dir JSONL（D-LOG-01）

### Q2: who/why/what 字段语义？

| Option | Description | Selected |
|--------|-------------|----------|
| 触发者枚举+变更清单 | who=admin/induction:llm/system:rollback；what=IRI 清单+计数；why=reasoning+备注 | ✓ |
| who 固定 admin | 丢失 LLM 自动化链路溯源，与 source_kind 精神不一致 | |
| 全文 TTL diff | 日志条目巨大，TTL 本身就是归档 | |

**User's choice:** 触发者枚举 + 变更清单（D-LOG-02）

### Q3: rollback 入口形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 任意版本可回 | 与 D-TBox-03 单步机制完全同构，零额外机制 | ✓ |
| 严格单步 | 回滚 5 步产生 5 个冗余版本+5 条日志 | |
| 独立危险区按钮 | rollback 本身可逆（写 v{N+1} 不覆盖历史），过度设防 | |

**User's choice:** 任意版本可回滚（D-LOG-03）

### Q4: 哪些事件写演进日志？

| Option | Description | Selected |
|--------|-------------|----------|
| 仅版本变化事件 | approve/rollback 写；reject 查 proposal 状态；boot 不写 | ✓ |
| 含 reject | timeline 语义从"版本演进"变"治理活动" | |
| 含 boot | 实质是启动日志不是演进日志 | |

**User's choice:** 仅版本变化事件（D-LOG-04）

---

## Claude's Discretion

- QueryScoped 具体签名、过滤实现、缓存策略
- SHACL 引擎包落位 + shapes 文件默认路径
- 隔离区 store 侧实现与恢复 API 细节
- Provenance struct 字段命名与 JSON 序列化细节
- tbox_changelog.jsonl 条目 struct 与读写 helper 落位
- verify-phase-40.sh 具体测试编组
- ECL 写入路径挂 provenance+belongsToTier 的具体改造点
- UI-SPEC 后的 /admin/governance 视觉与交互细节

## Deferred Ideas

- induction 产实例数据的 tier 挂载规则（未来 phase）
- Milvus provenance 字段（若 Phase 41 证明需要）
- shapes 热加载 / 运行时 Re-validate 按钮（有需求再加）
- 隔离区自动修复策略（auto-fix 属未来增强）
