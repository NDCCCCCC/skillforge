# Phase 45: SPARQL Schema Visibility (D-SPARQL-05) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-03
**Phase:** 45 — SPARQL Schema Visibility (D-SPARQL-05)
**Mode:** Advisor（4 个并行 gsd-advisor-researcher 基于代码事实研究，table-first 呈现）
**Areas discussed:** Subject 判定范围、schemaSubjects 生命周期、SC3 单点化形态、断言更新与回归范围

---

## Subject 判定范围

| Option | Description | Selected |
|--------|-------------|----------|
| β subject∩allowlist | schemaSubjects ∩ 13 个 emitter 元数据谓词（含 hasReadAccess，排除 label/comment）；D-SPARQL-03/06 pinned 测试原样绿；conformance 测试防漂移 | ✓ |
| α 全量 subject 升格 | ROADMAP SC1 字面方案；需重写 2 个 D-SPARQL-03 pinned 测试（显式 supersede）；emitter mis-type 静默升级为全实体 bypass | |
| α + class-XOR 不变量 | α + boot 时 XOR 不变量；误类型变硬错误，但仍需重写 pinned 测试 + 平行清单维护 | |

**User's choice:** β（推荐采纳）
**Notes:** 研究发现 ROADMAP 字面 α 与 Phase 44 pinned 测试（store_label_passthrough_test.go:64/:175-208，编码 D-SPARQL-03 裁决）直接冲突——β 是唯一不动既有 fail-closed 决策且达成 SC1 的路径。

---

## schemaSubjects 生命周期

| Option | Description | Selected |
|--------|-------------|----------|
| B 查询时派生 | QueryScoped 入口每次从 byP[rdfType] 派生局部集合，无持久状态；写路径零改动，漂移类 bug 构造性消除 | ✓ |
| A Eager set + 4 mutator hook | Insert/InsertUnique/DeleteSubject/Quarantine 全部 hook；查询 O(1) 但漏 hook 即静默违规（α 原文已示范典型遗漏：restore.go:86 InsertUnique 路径） | |
| C 缓存 + gen 计数器 | Lazy 缓存 + generation bump；当前规模下被 B 严格支配 | |

**User's choice:** B（推荐采纳）
**Notes:** 对 ROADMAP SC1 括号"Insert 时填充"为实现策略偏离（语义等价），已要求 planner 在 PLAN.md 记录偏离 rationale。

---

## SC3 单点化形态

| Option | Description | Selected |
|--------|-------------|----------|
| A 单点+parity 测试 | isSchemaTriple 已是全仓唯一实现；nil-tier early-return 与 admin 裸 Query() 维持 D-TIER-01 刻意语义；SC3 用端到端 parity 测试验证 | ✓ |
| B 统一入口 | admin 无 skill 也走 QueryScoped(nil)，消除 early-return；dedup 语义变化可能扰动 Phase 44 计数 fixture | |
| C 差分测试折中 | fast-path 保留，差分测试钉死 ≡ walk；等价性测试需精确编码现有 dedup quirk | |

**User's choice:** A（推荐采纳）
**Notes:** 判定的现实演化方向（增删 schemaClasses/allowlist 条目）对 nil 路径恒为 no-op；B 的构造保证收益接近零、回归风险真实。

---

## 断言更新与回归范围

| Option | Description | Selected |
|--------|-------------|----------|
| A 双层回归 | admin fixture 计数更新（TierFilter 15→17 + 正向断言）+ store 包新文件一测双断言（hasReadAccess 可见且 skill label 仍不可见）+ verify-phase-45.sh 新建 | ✓ |
| B 仅 admin fixture | 2 处计数，红线靠既有套件；单元层无 hasReadAccess 正向覆盖 | |
| C 谓词门追加 | hasReadAccess 进 OWL 四谓词门；回归最小但未交付 subject-based 泛化，与 SPARQL-01 字面冲突 | |

**User's choice:** A（推荐采纳）
**Notes:** D-SPARQL-08 政策延续但 45 的计数变化是预期行为变更——godoc rationale 引用 D-SPARQL-05 而非 miscount。FailClosed Path B 8→10 依赖 D-45-01+02 组合的 isSchemaTriple 升格落点（visibleSubjects 注入变体会给出 8 不变，已排除）。

---

## Claude's Discretion

- schemaSubjects 派生形态（内联 vs helper）
- 新测试文件命名与 conformance 测试断言写法
- parity 测试落点（store 级 / handler 级）
- allowlist 常量定义位置（判定在 store、枚举真源在 emitter，conformance 测试钉住）

## Claude's Discretion（决策补充）

- SC1 措辞澄清：ROADMAP "admin baseline 不可见" 与代码事实不符（nil-tier early-return 本就全可见）；实际受益者是 skill-scoped 视角。CONTEXT.md 以代码事实为准，已向用户呈现。

## Deferred Ideas

- schemaSubjects 持久缓存 + gen 计数器（KG 100k+ triples 时再考虑）
- class-XOR 不变量（emitter mis-type 成为现实问题再立项）
- D-SPARQL-07 comment hardening（已是 SPARQL-F01）
- SPARQL 1.1 parser（Phase 41 backlog）
