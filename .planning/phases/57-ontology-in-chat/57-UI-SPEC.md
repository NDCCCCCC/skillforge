# 57-UI-SPEC — 本体进 chat 的可见面（ONT-02 / ONT-03）

## 1. 数据源（全部已存在或本 phase 已新增，零发明）

| 展示 | 数据源 | 说明 |
|------|--------|------|
| 实体锚定行 | `data-decision` decisions.ontology_anchor 的 `anchorEntities` / `entitiesMatched` / `kgAvailable` | 57-02 新增（D16 overload，无新 chunk 类型）|
| KG 降级徽标 | 同上 `kgAvailable:false` | 显式降级，绝不静默 |
| 实体 grounding 报告 | verdict payload 的 `entities:{verified,unverified}` | 57-03 新增，omitempty |
| review 徽标 | `review_required` | 既有字段；unverified 非空时 BE 已强制 true |

## 2. 组件（渲染在既有 `AgentInspector` 的 step 树内，不新增 tab）

### 2.1 锚定行 `data-testid="kg-anchor-line"`
- 位置：AgentInspector 的 step 树 header 之下、树节点之上。
- 文案：`本体锚定 · {anchorEntities}`（多个用 ` / ` 分隔）。
- `entitiesMatched === 0` 且 `kgAvailable === true` → `本体锚定 · 未识别到实体`（
  **不是**「无本体」）。
- `kgAvailable === false` → `本体未接入（本次仅向量检索）` + `data-testid="kg-degraded-badge"`
  （amber tone，`aria-label="知识图谱不可用"`）。

### 2.2 实体 grounding 报告 `data-testid="entity-grounding-report"`
- 仅在 verdict `entities` 存在时渲染（absent = 未执行，**不渲染**，不显示「无」）。
- `unverified` 非空 → 区块标题「实体校验未通过」+ 每条 `data-testid="entity-unverified-item"`
  （text-status-non-compliant）+ 说明行「这些实体在本 Skill 可读语料中不存在，
  回答已降级为需复核」。
- `unverified` 为空但 `verified` 非空 → 折叠行「实体校验通过（N 个）」。
- 空数组 × 2 → 「实体校验：未识别到实体名称」。

## 3. 交互契约
- 全部只读（**零按钮、零 onClick**——与 55-02 矩阵同一 Honesty 姿态）。
- 锚定行与内容均为 verbatim 渲染，FE 不解析、不推导、不聚合。
- 未知字段值（如未来出现 `entities` 内新键）→ 忽略，不崩。

## 4. 空态/降级文案（逐字）
- `kgAvailable:false`：「本体未接入（本次仅向量检索）」+ 徽标「知识图谱不可用」
- `entitiesMatched:0` + available：「本体锚定 · 未识别到实体」
- `entities` absent：「（不渲染任何实体区块）」
- `unverified` 非空：「实体校验未通过」+「这些实体在本 Skill 可读语料中不存在，回答已降级为需复核」

## 5. Accessibility
- 降级徽标：`role="status"` + `aria-label="知识图谱不可用"`（不被读屏忽略）。
- unverified 列表：`<ul>` + 每条 `role` 默认（列表语义），标题 `<h3>`。
- 锚定行 `aria-live="polite"`（流式抵达时礼貌播报一次）。

## 6. Standards（继承）
零新依赖；零硬编码状态词（一律经既有 mapping）；data-testid 前缀 `kg-*` / `entity-*`；
文案逐字锁定（本 SPEC 的中文即实现文案）。
