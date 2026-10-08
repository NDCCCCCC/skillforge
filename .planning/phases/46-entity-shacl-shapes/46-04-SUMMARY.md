---
phase: 46-entity-shacl-shapes
plan: 04
subsystem: web-governance-ui
tags: [shacl, governance-report, layer-badges, dangling-edges, ui-spec-delta, honesty-red-lines, d-46-03, d-46-04]
requires:
  - 46-02 BE DTO 契约（Violation.layer/target_class omitempty + QuarantinedEntity.dangling_edge_count 无 omitempty 恒下发）
  - 46-UI-SPEC.md（approved 2026-10-04，5 PASS + 1 FLAG；冲突优先级 5 条取代 40-UI-SPEC 相应条款）
  - 40-UI-SPEC 基座（Design System / Honesty 红线 1-7 / ViolationBadge literal idiom / 行号判空惯例）
provides:
  - governance 报告分层呈现：LayerBadge 三态（tbox indigo / entity neutral / global dashed 无 chip 哨兵）+ 未知 layer 原值渲染
  - 约束 cell 三层结构（layer 徽章 → target_class neutral chip → constraint mono → +extra）；severity 徽章与 provenance-missing 特化分支零改动
  - 红/黄区头 per-layer 计数 chips（仅非零 layer、固定顺序 TBox · 实体 · 跨实体、data-testid 双锚点）
  - 隔离区 7 列表（悬挂边列：0 如实渲染 0、缺失渲染 —、header 带 title）+ SkeletonRows colSpan 7
  - 隔离区 hint 新文案（subject 位置精确语义 + 悬挂边指引 + 原恢复路径）
  - ShaclReportView.test.tsx 9 组件回归（legacy 兼容 / entity / global / 未知 layer / 悬挂边 0 与缺失 / 双区计数 / provenance-missing 非回归 / XSS 转义 / hint）
  - HealthBanner shapes meta join 列表 truncate + title 显示保护（max-w-[280px]）
affects:
  - SHACL-03 收口（46-02 语义层 + 46-03 装配 + 46-04 FE 呈现；checkbox 留收口 plan）
  - verify-phase-46.sh（若纳入 FE stage，ShaclReportView.test.tsx 的 9 测试为 anti-vacuous 锚点候选）
tech-stack:
  added: []   # 零新依赖（Registry Safety: none declared）
  patterns:
    - FE 零推导：layer 徽章/计数 chips 只读直渲 BE 盖戳字段，未知 layer 值渲染原值不吞不猜（Honesty 红线 9；OntologyGraphView 硬编码债反例不复刻）
    - Honesty 计数渲染：`!= null ? count : '—'` 复用行号列判空惯例，0 如实渲染 0（Honesty 红线 8）
    - XSS 面收敛：detail/constraint 全部 React 文本节点默认转义；组件测试以元素查询 + 无 title 复制列的 innerHTML 负断言双重钉死（T-46-02）
    - 注释措辞规避 grep 自伤（46-02 entities.ttl 先例）：XSS 注释避开 dangerouslySetInnerHTML 字面量使机器门零噪音
key-files:
  created:
    - web/src/components/admin/governance/ShaclReportView.test.tsx
  modified:
    - web/src/components/admin/governance/types.ts
    - web/src/components/admin/governance/ShaclReportView.tsx
    - web/src/components/admin/governance/GovernanceTab.tsx
decisions:
  - "46-04 FE 零推导落地：LAYER_BADGE 只按 BE 盖的 layer 字符串查表渲染；未知值走 `?? { entity 同色, label: 原值 }` fallback——console honesty 而非枚举收窄（46-UI-SPEC §Color 第 4 行）"
  - "TDD RED-first 对 Task 3 不适用（46-02 同款 process note）：plan 排序把实现（Task 1/2）放在钉子测试之前，behavior 全部是已落地语义的回归锚定；保持 test → feat 提交顺序满足 TDD gate 的 git log 序"
  - "XSS innerHTML 断言落点修正：detail td 把内容复制进 title 属性且 HTML 属性序列化不转义尖括号，字符串级 innerHTML 负断言在 detail 列必假阳；改为元素查询（无 b/i/img/script 节点）+ 约束列（无 title 复制）innerHTML 负断言——D-SPARQL-08 测试作者政策同款 godoc 理由写进测试头注释"
  - "ShaclReportView.test.tsx 提交时 makeViolation fixture 误带 dangling_edge_count（TS2353）——amend 修入同一 test commit（未推送前的原子性修复，非独立 fix commit）"
metrics:
  duration: 16 min
  completed: 2026-10-08
  tasks: 3
  files: 4
---

# Phase 46 Plan 04: governance 报告分层呈现（SHACL-03 FE 侧）Summary

**One-liner:** 46-02 盖的 layer/target_class 戳与恒下发的 dangling_edge_count 在 /admin/governance 渲染为三态徽章（tbox indigo / entity neutral / global dashed 无 chip 哨兵）、红黄区头 per-layer 计数 chips 与隔离区第 7 列「悬挂边」（0 渲染 0 / 缺失渲染 —）——46-UI-SPEC 冲突优先级 5 条取代项全部落地，FE 零推导（Honesty 红线 8/9），governanceApi.ts 咽喉点零改动，legacy 报告无徽章不炸，9 组件回归全绿。

## What Was Built

### Task 1 — types.ts DTO 镜像扩展（46-UI-SPEC §Component 1 逐字）→ `5eeaa01`

- `ViolationLayer = 'tbox' | 'entity' | 'global'`（ViolationSeverity 旁；锁定 BE validate.go 常量字符串）。
- `ShaclViolation.layer?/target_class?` optional（BE omitempty 对偶；JSDoc 逐字采 UI-SPEC 中文注释）；`QuarantinedEntity.dangling_edge_count?` optional（BE 无 omitempty 恒下发，FE 容缺 legacy 兼容）。
- `ShaclReport` 零变化（shapes_path 保持单 string，D-46-02 BE join 显示）；governanceApi.ts 零改动（grep 证实：`git diff --name-only` 仅 types.ts）。
- 门：tsc exit 0；ViolationLayer 计数 2、三 optional 字段计数 3、shapes_path 计数不变（1）。

### Task 2 — ShaclReportView 分层徽章 + 区头计数 + 悬挂边列 + hint 新文案（46-UI-SPEC §Component 2/3/4 + Copywriting）→ `efc0f4a`

- `LAYER_BADGE` 查表 + `LayerBadge`（照 ViolationBadge `rounded px-1.5 py-0.5 text-[11px] font-semibold` idiom）：tbox→`bg-primary-dim text-primary`、entity→`bg-surface-3 text-text-muted`、global→`border border-dashed border-border text-text-muted`（透明 bg）；未知 layer 值渲染原字符串 + entity 同色（`??` fallback，console honesty）。
- `ConstraintCell` 共享组件：layer 徽章 → target_class neutral chip（`bg-surface-2` mono IRI）→ constraint mono → `+{extra}`；红区整体替换、黄区仅替换非 provenance-missing 分支（特化 badge 分支逐字保留）；字段缺省对应元素不渲染（global 行有 dashed 徽章无 chip——哨兵可视区分）。
- 红区 6→7 列：「悬挂边」th 插「三元组」后带 `title="指向该隔离实体的入边数量（在 KG 查询对象位仍可见）"`；cell `qe.dangling_edge_count != null ? count : '—'`（0 渲染 0，Honesty 红线 8）；SkeletonRows colSpan 7。
- 区头计数：`layerCounts()` helper 按 layer 分组、仅非零、固定顺序；`LayerCountChips` 渲染 `<span ml-2 font-mono text-xs font-normal text-text-muted data-testid=governance-{quarantine|warnings}-layer-counts>`；红区计数 = 各隔离实体全部 violations，黄区 = report.warnings；全零不渲染 span。
- hint 替换为 §Copywriting 原文（含「查询 subject 位置不可见」精确短语 + 悬挂边指引 + Run ECL 重建/下次启动重校验原恢复路径）。
- Forbidden 自查全零：零新分区/tab/token/交互控件/禁用字符串；detail/constraint 保持文本节点（grep 门 `dangerouslySetInnerHTML == 0`——注释措辞已规避字面量自伤）。

### Task 3 — GovernanceTab 显示保护 + ShaclReportView 组件测试 → `3969a38` (test) + `a440ffe` (feat)

- `ShaclReportView.test.tsx`（colocated，vitest + @testing-library，顶部两行版权头）：9 测试覆盖 legacy 无徽章不炸（且计数 span 不渲染）、entity 徽章+chip、global 哨兵无 chip、未知 layer 原值、悬挂边 0→'0'/缺失→'—'（`getAllByRole('cell')[2]` 列序断言 + header title 断言）、红区 TBox 2 · 实体 1 无跨实体 + 黄区计数、provenance-missing 特化分支非回归、XSS 元字符文本化（元素查询零 b/i/img/script + 约束列 innerHTML 负断言）、hint 三短语。
- `GovernanceTab.tsx` :81-84 唯一触碰点：shapes 值包 `max-w-[280px] truncate align-bottom` + `title`（§Component 5 显示保护；防 D-46-02 join 列表撑折 banner）；Tab 注册/switcher/CTA 零改动（git diff 逐行核实仅该区域）。
- 门：tsc exit 0；governance vitest 13/13（9 新 + 4 既有 governanceApi）；`max-w-[280px]` 计数 1。

## Verification

- 每 task 提交前 gate：`cd web && npx tsc --noEmit` exit 0 + `npx vitest run src/components/admin/governance` 全绿（Windows 侧 node，WSL 无 node 红线）。
- 最终 gate：全量 FE 套件 `npx vitest run` 685 测试中 684 绿；唯一失败 `DetailPanel.test.tsx` SC-5 为**先存失败**（因果排除：DetailPanel 零 import governance、测试文件最后修改 2026-08-20、隔离运行确定性失败、STATE.md v1.6 积压已列名）→ 记 D-46-DEF-01（deferred-items.md），不顺手修。
- 计划验收 grep 全过：悬挂边 ≥2（3）、双 testid ==2、colSpan={7} ==1、`查询 subject 位置不可见` ==1、`dangerouslySetInnerHTML` ==0、禁用字符串全零、`max-w-[280px]` ==1。
- 红线核实：governanceApi.ts 零改动（plan 全程 `git diff --name-only` 无它）；四 commit 零文件删除（`--diff-filter=D` 空）。
- 46-UI-SPEC Visual Checklist 逐条可检：7 列悬挂边 ✓、三态徽章+chip ✓、双区计数 chips ✓、legacy 无徽章 ✓、hint 新文案 ✓、banner truncate ✓、types optional + governanceApi 未动 ✓、零新分区/tab/token/依赖/禁用串 ✓。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 测试 fixture 类型错误（TS2353）**
- **Found during:** Task 3
- **Issue:** `makeViolation` overrides 误带 `dangling_edge_count`（属 QuarantinedEntity 非 ShaclViolation），tsc 报 2 处 TS2353；vitest 不跑 tsc 故测试先绿后 gate 才暴露。
- **Fix:** 两处移到 `makeQuarantined` overrides；`git commit --amend` 修入同一 test commit（未推送，保持原子性）。
- **Files modified:** web/src/components/admin/governance/ShaclReportView.test.tsx
- **Commit:** 3969a38（amend 后）

### Process Notes（非代码偏差）

1. **Task 3 `tdd="true"` 的 RED-first 不适用**（46-02 同款）：plan 排序把实现（Task 1/2）放在钉子测试之前，`<behavior>` 全部是已落地语义的回归锚定——强行 RED 需回滚前序 task，与 plan 排序自相矛盾。按 `<action>` 原文执行，保持 test(46-04) → feat(46-04) 提交顺序，TDD gate 的 git log 序（test 先于 feat）成立。
2. **XSS innerHTML 断言落点修正**：behavior 原文「container.innerHTML 中无 '<b>'」在 detail 列不可满足——detail td 把内容复制进 `title` 属性且 HTML 属性序列化不转义尖括号，字符串级负断言必假阳。改为元素查询（更强：证明无元素物化）+ 约束列（无 title 复制）innerHTML 负断言，理由按 D-SPARQL-08 政策写进测试头注释。
3. **grep 自伤规避**：ConstraintCell 注释初稿含 `dangerouslySetInnerHTML` 字面量使验收 grep ==0 翻红；改写为「no raw-HTML sink」（46-02 entities.ttl 注释措辞先例）。
4. **gsd-sdk CLI 不在本机 PATH**：STATE.md / ROADMAP.md 按 SDK handler 等价语义手工更新（46-01/46-02/46-03 先例）。
5. **REQUIREMENTS.md 不动**：SHACL-03 由 46-02+46-03+46-04 跨 plan 收口，checkbox 留收口 plan（45-01/46-02 先例）。

### Out-of-scope Discoveries（SCOPE BOUNDARY，未修）

- **D-46-DEF-01**：先存 `DetailPanel.test.tsx` SC-5 失败（knowledge-graph 空态 hero 不挂载；隔离运行确定性复现）→ 记入 `.planning/phases/46-entity-shacl-shapes/deferred-items.md`，归属 v1.6 积压既有条目「DetailPanel/emitter 两个 deferred 测试」。

## Known Stubs

None. 全部渲染直读 46-02 契约字段（layer/target_class/dangling_edge_count），无 mock 数据源、无占位文案；legacy 缺省路径为如实渲染而非 stub。

## Threat Posture

- **T-46-02（XSS）mitigate 兑现**：detail/constraint/entity/target_class 全部 React 文本节点默认转义；组件测试以 HTML 元字符 fixture 钉死（元素查询 + innerHTML 双层负断言）。
- **T-46-03（悬挂边披露）accept-as-designed 落地**：0 渲染 0、缺失渲染 —、header title 解释语义；禁用字符串 grep 门全零防「已处理」类虚假文案。
- **T-46-05（FE 伪造 layer）mitigate 兑现**：LAYER_BADGE 纯查表渲染 BE 戳，零 IRI/类名启发式；未知值渲染原值不吞不猜；legacy 缺省如实无徽章。
- **T-46-SC**：零新 npm 依赖、零安装动作。
- 无计划外新增攻击面 → 无 Threat Flags。

## Commits

| Task | Commit | Files |
| ---- | ------ | ----- |
| 1 | `5eeaa01` feat(46-04): mirror layer/target_class/dangling_edge_count DTO fields in governance types (46-UI-SPEC §Component 1) | types.ts |
| 2 | `efc0f4a` feat(46-04): render layer badges, zone count chips, dangling-edge column in ShaclReportView (46-UI-SPEC §Component 2/3/4) | ShaclReportView.tsx |
| 3 | `3969a38` test(46-04): pin ShaclReportView layered rendering — badges, count chips, dangling edges, legacy compat, XSS escaping | ShaclReportView.test.tsx |
| 3 | `a440ffe` feat(46-04): guard HealthBanner shapes meta against join-list overflow (46-UI-SPEC §Component 5) | GovernanceTab.tsx |

## Self-Check: PASSED

- [x] web/src/components/admin/governance/types.ts 含 ViolationLayer（2 处）+ 三 optional 字段（计数 3）+ shapes_path 计数不变（1）
- [x] web/src/components/admin/governance/ShaclReportView.tsx 含 LAYER_BADGE / governance-quarantine-layer-counts / governance-warnings-layer-counts（2 处）/ colSpan={7}（1 处）/ 悬挂边（3 处）/ subject 位置短语（1 处）/ dangerouslySetInnerHTML（0 处）
- [x] web/src/components/admin/governance/GovernanceTab.tsx 含 max-w-[280px]（1 处）；diff 仅 :81-84 区域
- [x] web/src/components/admin/governance/ShaclReportView.test.tsx 存在，9 测试全绿（governance 套件 13/13）
- [x] commits 5eeaa01 / efc0f4a / 3969a38 / a440ffe 均在 git log；零文件删除；governanceApi.ts 零改动
- [x] 最终 gate：tsc exit 0 + 全量 FE 套件 684/685（唯一失败为先存 D-46-DEF-01，已记 deferred）
