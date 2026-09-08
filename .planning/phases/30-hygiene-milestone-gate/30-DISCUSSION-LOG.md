# Phase 30: 工程卫生收尾 (Hygiene & Milestone Gate) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-08
**Phase:** 30-hygiene-milestone-gate
**Areas discussed:** CR-01+审查发现处置, UAT 债口径, 信任语义回归证明口径, 魔数抽查门形态

---

## 灰区选择（multiSelect）

| Option | Selected |
|--------|----------|
| CR-01+审查发现处置 | ✓ |
| UAT 债口径 | ✓ |
| 信任语义回归证明口径 | ✓ |
| 魔数抽查门形态 | ✓ |

**User's choice:** 全部四个灰区都讨论
**Notes:** 30 从「纯收尾门」扩为「加固 + 收尾门」，planner 按此排 wave（加固修复先行于门与证据汇编）。

---

## CR-01+审查发现处置

| Option | Description | Selected |
|--------|-------------|----------|
| 纳入 30 修复 | CR-01 作为 30 的一个 plan task：修复 + 回归测试锁 + STRIDE-30 登记，30 验证门证明关闭 | ✓ |
| 现在 --fix | 启动 30 前 `/gsd:code-review 29 --fix` 自动处置，让 30 从干净基线开始 | |
| 留 issue 不修 | 不建议：critical 且影响 FBK 指标口径，与里程碑卫生门目标直接冲突 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 安全类必修+小修 | WR-03/05 必修（真实安全洞）；WR-01/02/04/06 逐条裁定，结构性大改 deferred | |
| 全部修 | 6 个 warning 全部 30 内修完，含结构性改动（事务化/sweep 机制） | ✓ |
| 只修 critical | 只修 CR-01，6 warning 登记 deferred；与「信任语义不削弱」主题冲突 | |

**User's choice:** CR-01 纳入 30 修复 + 6 warning 全部修
**Notes:** 8 个 info 逐条裁定记录（trivial 顺手修，其余 accept/defer 带理由）。

---

## UAT 债口径

| Option | Description | Selected |
|--------|-------------|----------|
| 29 必清 27/28 维持 | 29 的 9 项关账前 operator 亲自跑；27/28 维持 deferred | |
| 全部清偿 | 27+28+29 三份 runbook 全部项在关账前 operator 亲自跑完 | ✓ |
| 全部 deferred | 与「信任语义可证」主题有张力（29 联合验收 = FBK-05 实机确认） | |

**User's choice:** 全部清偿
**Notes:** UAT 清偿成为 30 关账的硬门；plan 落成显式 operator checkpoint 任务（autonomous: false），AI 备环境与步骤、operator 执行并记录。

---

## 信任语义回归证明口径

| Option | Description | Selected |
|--------|-------------|----------|
| hermetic+live 双层 | hermetic 结构门 + 冻结 golden set + live `--frozen --gate-check`；断线纪律遵循 | ✓ |
| 仅 hermetic+引用 | 零风险零成本；但 26 注册表重构后无新鲜 live 证明 | |
| 仅 live | 证据最强但单点（GLM 配额 + 内网间歇断） | |

**User's choice:** hermetic+live 双层
**Notes:** live 前探测内网服务（Milvus/BGE/GLM），断线 = 环境 STOP 记原因，不刷分、不掩盖；两层证据都进 30 验收记录。

---

## 魔数抽查门形态

| Option | Description | Selected |
|--------|-------------|----------|
| 常驻结构门 | verify-phase-30.sh + Makefile phase-30 target；字段可配验证 + diff 字面量扫描 | ✓ |
| 一次性 audit | 30 内跑一次写进验收文档，无常驻门 | |

**User's choice:** 常驻结构门

---

## Claude's Discretion

- REQUIREMENTS traceability 补 12 个 PLAT/SCALE/DOMAIN 旧 ID
- `.env.example` 补 `SKILLFORGE_ADMIN_TOKEN`
- STRIDE-28 从 phase 目录归位 `docs/threat-model/`
- STRIDE-30 威胁面登记（无新攻击面）
- 30 验收文档结构与位置
- 里程碑关账仪式（CHANGELOG/RELEASE-NOTES）属 `/gsd:complete-milestone` 职责

## Deferred Ideas

- 里程碑关账仪式（CHANGELOG/RELEASE-NOTES/milestone-summary）
- WR-05 完整用户鉴权体系（真实登录态）
- WR-02 持久化 outbox 模式（跨重启强投递保证）