---
status: ready_for_execution
task: 261113-lhu
created: 2026-11-13
---

# 261113-lhu PLAN — composer 头部交互改造（SkillPicker 下沉 + ModelPicker 可交互）

## 背景（勘察已锁定）

- 输入框左上角的当前 Skill 徽章（ChatInput.tsx:275-293，`composer-skill-badge` 只读）不可点击换 Skill；可交互的 SkillPicker 下拉在 TopBar（:63-65、:71），离使用场景远。
- 输入框右上 ModelPicker 在 `models.length===0` 时是非交互 span（ModelPicker.tsx:227-229，env 模式拉不到模型列表时显示"未设置模型 live"），用户无法设置模型。
- `SkillPicker`（chat/parts/SkillPicker.tsx）自包含（无必填 props，默认模式），可直接移植。
- TopBar 仅被 ChatLayout.tsx:240 渲染；TopBar.test.tsx:48 断言 `skill-picker-trigger` 存在，需同步改。

## Task 1 — ChatInput：只读徽章 → SkillPicker 下拉

**files:** `web/src/components/chat/ChatInput.tsx`

**action:**
1. :271-293 行（非 modeLabel 分支的头部行）：左侧 `<div style={{minWidth:0}}>` 内的 `currentSkill` 只读徽章与 `未选择 Skill` 提示**整体替换**为 `<SkillPicker />`，外包定宽容器 `<div className="w-44">`（与 TopBar 原容器一致）。modeLabel（向导模式）分支不动。
2. import SkillPicker（`./parts/SkillPicker` 相对路径按 ChatInput 在 `components/chat/` 位置推：`./parts/SkillPicker`）。
3. 斜杠菜单逻辑**保留不动**（输入 / 仍可开菜单；两者是独立浮层不冲突）。
4. `currentSkill`/`skills`/`currentSkillId` 若因此无引用则清理 import（注意 stale-skill effect :137-141 仍用 currentSkillId/skills——保留该 effect 与 slash 菜单用的 skills）。

## Task 2 — TopBar：移除 SkillPicker

**files:** `web/src/components/layout/TopBar.tsx`, `web/src/__tests__/TopBar.test.tsx`

**action:**
1. 删 :63-65（`w-44` 容器）与 :71 的 `<SkillPicker />` 及对应 import；左区注释同步。
2. TopBar.test.tsx:47-48 断言 `skill-picker-trigger` 存在 → 改为断言**不存在**（`queryByTestId('skill-picker-trigger')).not.toBeInTheDocument()`），并在注释注明 Skill 切换入口已下沉 composer（261113-lhu）。

## Task 3 — ModelPicker：空模型状态可交互

**files:** `web/src/components/chat/ModelPicker.tsx`

**action:**
1. :227-229 非列表分支：`<span>` → `<button type="button">`，`data-testid="model-picker-set"`；点击 → `const next = window.prompt('输入模型名（如 glm-5.3-flash）后回车切换：'); if (next && next.trim()) void switchModel(next.trim());`。
2. 文案与兜底保留：显示 `model || (mode === 'stub' ? 'stub' : '未设置模型')`；title 组合现有指引 + "点击手动输入模型名切换"。
3. 样式与 badge 协调（`cursor-pointer`、hover 不刺眼即可），aria-label="设置模型"。

## Task 4 — ChatInput 测试同步

**files:** `web/src/__tests__/ChatInput.test.tsx`（以及断言了 `chat-input-current-skill` 只读文案的其他测试，先 grep `chat-input-current-skill` 确认波及面）

**action:** 断言只读徽章文案/存在性的用例改为断言 `skill-picker-trigger` 存在；`data-testid="chat-input-current-skill"` 若被 wire-contract 或 E2E 引用，可在 SkillPicker 外层容器保留该 testid 挂到新结构上（优先保 testid 稳定，其次改断言）。

**verify:**
- `cd web && npx vitest run src/__tests__/ChatInput.test.tsx src/__tests__/TopBar.test.tsx src/__tests__/SkillPicker.test.tsx` exit 0
- `npm run typecheck` exit 0
- 若 App.test.tsx / ChatLayout.test.tsx / wire-contract.test.ts 断言波及，同步最小修改后跑 `npx vitest run`（全量）确认无回归

## 收口

- 单 commit，message 简体中文 conventional 风格
- backlog 注记：ModelPicker models 列表为空的根因（env 模式无凭据，`/api/admin/llm/models` 需后端支持 env 凭据解析）留后端增强
