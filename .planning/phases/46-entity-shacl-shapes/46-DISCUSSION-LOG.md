# Phase 46: Entity SHACL Shapes - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-04
**Phase:** 46-entity-shacl-shapes
**Areas discussed:** 约束强度, Shapes 文件组织, 违规分层呈现, 隔离的悬挂边
**Mode:** advisor（4 并行 gsd-advisor-researcher，standard 校准档，全部选项经 file:line 取证 + live 快照核对）

---

## 约束强度

| Option | Description | Selected |
|--------|-------------|----------|
| C2 安静全绿 | 引擎加 sh:severity 解析（默认 error 不变现有行为）；label/type 必填；entityCode/parentOrganization 仅 if-present 检查；sourcePath 特例退役进 shapes。首跑安静全绿 | ✓ |
| C1 黄灯可见 | 同 C2 但 entityCode minCount 1 @ warning —— 112 条常驻警告，治理健康永黄，淹没 provenance-missing 可行动警告 | |
| A 纯 shapes | 零引擎改动，severity 政策继续硬编码（D-PROV-10 特例保持）；推荐属性语义不可表达 | |

**User's choice:** C2 安静全绿（推荐项）
**Notes:** 研究者数据实证：entityCode live 缺失 88-89%、parentOrganization 缺失 98%（根机构语义性无父级）→ 业务理想必填 = 首跑隔离 112/127 合法实体。B（业务理想必填@error）因数据直接否决，未列入选项。

---

## Shapes 文件组织

| Option | Description | Selected |
|--------|-------------|----------|
| b 路径列表 | 实体 shapes 独立 entities.ttl；SKILLFORGE_SHACL_SHAPES 延伸为路径列表；induction.go:383 rollback 硬编码（data/shacl/shapes.ttl 不存在→静默 no-op）一并改读同一 env | ✓ |
| b2 第二 env | 独立第二 env SKILLFORGE_SHACL_ENTITY_SHAPES（空=关）；偏离 ROADMAP「延伸既有机制」字面 | |
| a 单文件追加 | 单文件追加实体 shapes；零代码但无法独立禁用实体校验（rollback 硬编码仍需单独修） | |

**User's choice:** b 路径列表（推荐项）
**Notes:** 研究者挖出潜伏 bug：D-PROV-09 rollback 重校验今天在静默 no-op 且无视 env——四个选项下都同意顺手修复；b 同时满足 ROADMAP SC1 字面 + 修复 parity。用户确认「无论选哪个，rollback 硬编码都应顺手修复」。

---

## 违规分层呈现

| Option | Description | Selected |
|--------|-------------|----------|
| A 后端盖戳 | 后端校验时盖 target_class+layer 戳（global 哨兵）；FE 徽章+区头计数；精简版 46-UI-SPEC.md | ✓ |
| C shapes 标注 | shapes 文件写 skf:layer 标注由解析器透传 —— 仅当 layer 分类学预期增长才值得 | |

**User's choice:** A 后端盖戳（推荐项）
**Notes:** FE-only 分组方案因零硬编码违反（OntologyGraphView.tsx:21-25 硬编码债同款）在呈现阶段即被否决，未列入选项。UI 改动范围锁定为标签问题（徽章+区头计数），不加新分区/tab。

---

## 隔离的悬挂边

| Option | Description | Selected |
|--------|-------------|----------|
| a+c 保持+计数 | subject-only quarantine 保持不动；报告加只读 dangling_edge_count 字段供治理展示；级联留作未来项 | ✓ |
| a 仅现状 | 零改动 —— 悬挂暴露对运营者完全不可见 | |
| b 级联隔离 | 级联隔离入边 —— 需全新 object-keyed 机制，误标健康实体+破坏恢复路径（研究者明确不推荐） | |

**User's choice:** a+c 保持+计数（推荐项）
**Notes:** 研究者机制实证：store.Quarantine 按 subject 键控 → 级联会误把健康 note 记入隔离清单；Run ECL 恢复路径（二次父解析要求父级 rdf:type 在主索引）会被破坏；debounced snapshot 会把删除永久化。SC2 验收语义正式解读为 subject 位置不可见（记录于 CONTEXT specifics）。

---

## Claude's Discretion

- env 默认列表值形态、多文件 parse 错误聚合结构（约束：不静默吞缺失路径）
- entities.ttl 内部组织与 PropertyShape 命名
- dangling_edge_count 字段落点与入边谓词枚举实现位置
- layer 枚举字符串、徽章样式、区头文案（46-UI-SPEC / ui-phase 域）
- SC2 注入测试形态、verify-phase-46.sh 编组、首跑全绿的 hermetic 断言方式

## Deferred Ideas

- 级联隔离（object-keyed 抑制机制）—— SC2 正式重释为对象位不可见时再立项
- C1（entityCode 黄灯）—— 语料编码覆盖率提升后
- skf:layer shapes 标注 —— layer 分类学增长时
- 图谱空心节点渲染修复 —— FE 打磨候选
- 入边谓词枚举 shapes 化 —— dangling 计数泛化需求出现时

## 讨论范围说明

用户在 Area 2 决策确认中主动确认「rollback 硬编码应顺手修复」——这是范围内既有违规修复（零硬编码标准），非 scope creep。全程无新增能力提议。
